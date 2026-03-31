# v2node
A v2board backend base on moddified xray-core.
一个基于修改版xray内核的V2board节点服务端。

**注意： 本项目需要搭配[修改版V2board](https://github.com/EchoHS/v2board)**

## 软件安装

### 一键安装

```
wget -N https://raw.githubusercontent.com/EchoHS/v2node/master/script/install.sh && bash install.sh
```

## 构建
``` bash
GOEXPERIMENT=jsonv2 go build -v -o build_assets/v2node -trimpath -ldflags "-X 'github.com/EchoHS/v2node/cmd.version=$version' -s -w -buildid="
```
