package core

import (
	"encoding/json"
	"net"
	"strconv"
	"strings"

	panel "github.com/EchoHS/v2node/api/v2board"
	"github.com/EchoHS/v2node/common/format"
	"github.com/xtls/xray-core/app/dns"
	"github.com/xtls/xray-core/app/router"
	xnet "github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/core"
	coreConf "github.com/xtls/xray-core/infra/conf"
)

// hasPublicIPv6 checks if the machine has a public IPv6 address
func hasPublicIPv6() bool {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		ip := ipNet.IP
		// Check if it's IPv6, not loopback, not link-local, not private/ULA
		if ip.To4() == nil && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsPrivate() {
			return true
		}
	}
	return false
}

func hasOutboundWithTag(list []*core.OutboundHandlerConfig, tag string) bool {
	for _, o := range list {
		if o != nil && o.Tag == tag {
			return true
		}
	}
	return false
}

func appendRoutingRule(rules *[]json.RawMessage, rule map[string]interface{}) {
	rawRule, err := json.Marshal(rule)
	if err != nil {
		return
	}
	*rules = append(*rules, rawRule)
}

func ensureCustomOutbound(outbounds *[]*core.OutboundHandlerConfig, actionValue *string, tagOverride string) (string, bool) {
	if actionValue == nil {
		return "", false
	}
	outbound := &coreConf.OutboundDetourConfig{}
	if err := json.Unmarshal([]byte(*actionValue), outbound); err != nil {
		return "", false
	}
	if tagOverride != "" {
		outbound.Tag = tagOverride
	}
	if hasOutboundWithTag(*outbounds, outbound.Tag) {
		return outbound.Tag, true
	}
	customOutbound, err := outbound.Build()
	if err != nil {
		return "", false
	}
	*outbounds = append(*outbounds, customOutbound)
	return outbound.Tag, true
}

func routeUserOutboundTag(route panel.Route) string {
	if route.Remarks != "" {
		return route.Remarks
	}
	return "route_user_" + strconv.Itoa(route.Id)
}

func GetCustomConfig(infos []*panel.NodeInfo) (*dns.Config, []*core.OutboundHandlerConfig, *router.Config, error) {
	//dns
	queryStrategy := "UseIPv4v6"
	if !hasPublicIPv6() {
		queryStrategy = "UseIPv4"
	}
	coreDnsConfig := &coreConf.DNSConfig{
		Servers: []*coreConf.NameServerConfig{
			{
				Address: &coreConf.Address{
					Address: xnet.ParseAddress("localhost"),
				},
			},
		},
		QueryStrategy: queryStrategy,
	}
	//outbound
	defaultoutbound, _ := buildDefaultOutbound()
	coreOutboundConfig := append([]*core.OutboundHandlerConfig{}, defaultoutbound)
	block, _ := buildBlockOutbound()
	coreOutboundConfig = append(coreOutboundConfig, block)
	dns, _ := buildDnsOutbound()
	coreOutboundConfig = append(coreOutboundConfig, dns)

	//route
	domainStrategy := "AsIs"
	dnsRule, _ := json.Marshal(map[string]interface{}{
		"port":        "53",
		"network":     "udp",
		"outboundTag": "dns_out",
	})
	coreRouterConfig := &coreConf.RouterConfig{
		RuleList:       []json.RawMessage{dnsRule},
		DomainStrategy: &domainStrategy,
	}

	for _, info := range infos {
		if len(info.Common.Routes) == 0 {
			continue
		}
		// Xray routing is first-match wins, so route actions must be grouped
		// by priority instead of trusting the panel response order.
		blockRules := make([]json.RawMessage, 0)
		outboundRules := make([]json.RawMessage, 0)
		defaultOutRules := make([]json.RawMessage, 0)
		for _, route := range info.Common.Routes {
			switch route.Action {
			case "dns":
				if route.ActionValue == nil {
					continue
				}
				server := &coreConf.NameServerConfig{
					Address: &coreConf.Address{
						Address: xnet.ParseAddress(*route.ActionValue),
					},
				}
				if len(route.Match) != 0 {
					server.Domains = route.Match
					server.SkipFallback = true
				}
				coreDnsConfig.Servers = append(coreDnsConfig.Servers, server)
			case "block":
				appendRoutingRule(&blockRules, map[string]interface{}{
					"inboundTag":  info.Tag,
					"domain":      route.Match,
					"outboundTag": "block",
				})
			case "block_ip":
				appendRoutingRule(&blockRules, map[string]interface{}{
					"inboundTag":  info.Tag,
					"ip":          route.Match,
					"outboundTag": "block",
				})
			case "block_port":
				appendRoutingRule(&blockRules, map[string]interface{}{
					"inboundTag":  info.Tag,
					"port":        strings.Join(route.Match, ","),
					"outboundTag": "block",
				})
			case "protocol":
				appendRoutingRule(&blockRules, map[string]interface{}{
					"inboundTag":  info.Tag,
					"protocol":    route.Match,
					"outboundTag": "block",
				})
			case "route":
				outboundTag, ok := ensureCustomOutbound(&coreOutboundConfig, route.ActionValue, "")
				if !ok {
					continue
				}
				appendRoutingRule(&outboundRules, map[string]interface{}{
					"inboundTag":  info.Tag,
					"domain":      route.Match,
					"outboundTag": outboundTag,
				})
			case "route_ip":
				outboundTag, ok := ensureCustomOutbound(&coreOutboundConfig, route.ActionValue, "")
				if !ok {
					continue
				}
				appendRoutingRule(&outboundRules, map[string]interface{}{
					"inboundTag":  info.Tag,
					"ip":          route.Match,
					"outboundTag": outboundTag,
				})
			case "route_user":
				users := make([]string, 0, len(route.Match))
				for _, uuid := range route.Match {
					if uuid == "" {
						continue
					}
					users = append(users, format.UserTag(info.Tag, uuid))
				}
				if len(users) == 0 {
					continue
				}
				outboundTag, ok := ensureCustomOutbound(&coreOutboundConfig, route.ActionValue, routeUserOutboundTag(route))
				if !ok {
					continue
				}
				appendRoutingRule(&outboundRules, map[string]interface{}{
					"inboundTag":  info.Tag,
					"user":        users,
					"outboundTag": outboundTag,
				})
			case "default_out":
				outboundTag, ok := ensureCustomOutbound(&coreOutboundConfig, route.ActionValue, "")
				if !ok {
					continue
				}
				appendRoutingRule(&defaultOutRules, map[string]interface{}{
					"inboundTag":  info.Tag,
					"network":     "tcp,udp",
					"outboundTag": outboundTag,
				})
			default:
				continue
			}
		}
		coreRouterConfig.RuleList = append(coreRouterConfig.RuleList, blockRules...)
		coreRouterConfig.RuleList = append(coreRouterConfig.RuleList, outboundRules...)
		coreRouterConfig.RuleList = append(coreRouterConfig.RuleList, defaultOutRules...)
	}
	DnsConfig, err := coreDnsConfig.Build()
	if err != nil {
		return nil, nil, nil, err
	}
	RouterConfig, err := coreRouterConfig.Build()
	if err != nil {
		return nil, nil, nil, err
	}
	return DnsConfig, coreOutboundConfig, RouterConfig, nil
}
