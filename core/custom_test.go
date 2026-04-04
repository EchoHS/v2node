package core

import (
	"testing"

	panel "github.com/EchoHS/v2node/api/v2board"
)

func strPtr(v string) *string {
	return &v
}

func TestGetCustomConfigOrdersRouteRulesByActionPriority(t *testing.T) {
	info := &panel.NodeInfo{
		Tag: "test-inbound",
		Common: &panel.CommonNode{
			Routes: []panel.Route{
				{
					Id:          1,
					Action:      "default_out",
					ActionValue: strPtr(`{"protocol":"freedom","tag":"default_custom"}`),
				},
				{
					Id:          2,
					Action:      "route",
					Match:       []string{"domain:example.com"},
					ActionValue: strPtr(`{"protocol":"freedom","tag":"route_custom"}`),
				},
				{
					Id:     3,
					Action: "block",
					Match:  []string{"domain:blocked.example"},
				},
			},
		},
	}

	_, _, routerConfig, err := GetCustomConfig([]*panel.NodeInfo{info})
	if err != nil {
		t.Fatalf("GetCustomConfig() error = %v", err)
	}

	gotTags := make([]string, 0, len(routerConfig.GetRule()))
	for _, rule := range routerConfig.GetRule() {
		gotTags = append(gotTags, rule.GetTag())
	}

	wantTags := []string{"dns_out", "block", "route_custom", "default_custom"}
	if len(gotTags) != len(wantTags) {
		t.Fatalf("rule count = %d, want %d (%v)", len(gotTags), len(wantTags), gotTags)
	}
	for i := range wantTags {
		if gotTags[i] != wantTags[i] {
			t.Fatalf("rule order = %v, want %v", gotTags, wantTags)
		}
	}
}

func TestGetCustomConfigPlacesRouteUserWithOutboundRules(t *testing.T) {
	info := &panel.NodeInfo{
		Tag: "test-inbound",
		Common: &panel.CommonNode{
			Routes: []panel.Route{
				{
					Id:          1,
					Action:      "default_out",
					ActionValue: strPtr(`{"protocol":"freedom","tag":"default_custom"}`),
				},
				{
					Id:          2,
					Remarks:     "user_route_out",
					Action:      "route_user",
					Match:       []string{"user-uuid"},
					ActionValue: strPtr(`{"protocol":"freedom","tag":"ignored_by_route_user"}`),
				},
				{
					Id:     3,
					Action: "block",
					Match:  []string{"domain:blocked.example"},
				},
			},
		},
	}

	_, _, routerConfig, err := GetCustomConfig([]*panel.NodeInfo{info})
	if err != nil {
		t.Fatalf("GetCustomConfig() error = %v", err)
	}

	gotTags := make([]string, 0, len(routerConfig.GetRule()))
	for _, rule := range routerConfig.GetRule() {
		gotTags = append(gotTags, rule.GetTag())
	}

	wantTags := []string{"dns_out", "block", "user_route_out", "default_custom"}
	if len(gotTags) != len(wantTags) {
		t.Fatalf("rule count = %d, want %d (%v)", len(gotTags), len(wantTags), gotTags)
	}
	for i := range wantTags {
		if gotTags[i] != wantTags[i] {
			t.Fatalf("rule order = %v, want %v", gotTags, wantTags)
		}
	}

	routeUserRule := routerConfig.GetRule()[2]
	wantUser := "test-inbound|user-uuid"
	if len(routeUserRule.GetUserEmail()) != 1 || routeUserRule.GetUserEmail()[0] != wantUser {
		t.Fatalf("route_user match = %v, want [%s]", routeUserRule.GetUserEmail(), wantUser)
	}
}
