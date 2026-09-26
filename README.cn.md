# midgard [![midgard](https://github.com/changkun/midgard/actions/workflows/midgard.yml/badge.svg)](https://github.com/changkun/midgard/actions/workflows/midgard.yml) ![](https://changkun.de/urlstat?mode=github&repo=changkun/midgard)

[English](./README.md) | 中文

米德加德（midgard）是一个跨平台的全局剪贴板服务，支持 macOS/Linux/Windows/iOS。

在一台设备上复制，在另一台上粘贴；把复制的内容变成可分享的链接。一切都运行在你自己的服务器上。

## 工作方式

你运行一个**服务端**，每台设备运行一个**守护进程**，负责把本机剪贴板与服务端同步。`mg` 命令与本机守护进程通信；手机则通过 iOS 快捷指令或 Android 的 Tasker 直接访问服务端。

## 快速开始

**1. 服务端。** 在一台可公开访问的机器上：

```sh
$ cp config.example.yml config.yml   # 设置 domain 与 server.auth.pass
$ make build && make up              # 或直接运行：mg server
```

`docker-compose.yml` 会加入一个已有的 traefik 网络；如需使用自己的反向代理，请参阅[安装](./docs/install.cn.md)。

**2. 为每台设备签发令牌。** 在服务端运行目录下：

```sh
$ mg server token add laptop --owner you
mgt_...
```

**3. 每台设备。** 从 [releases](https://github.com/changkun/midgard/releases) 下载 `mg`（或使用 `go install changkun.de/x/midgard@latest`，生成的程序名为 `midgard`），然后将以下内容写入配置文件：Linux 为 `~/.config/midgard/config.yml`，macOS 为 `~/Library/Application Support/midgard/config.yml`，Windows 为 `%AppData%\midgard\config.yml`：

```yaml
domain: example.com   # 或 http://your-server:8456
token: mgt_...        # 第 2 步中得到的令牌
```

```sh
$ mg daemon install   # 以当前用户身份，无需 sudo
$ mg daemon start
$ mg status
server status: OK
daemon status: OK
```

现在就可以在一台设备上复制、在另一台上粘贴了。`mg alloc` 可将剪贴板内容变成公开链接，详见[使用](./docs/usage.cn.md)。

## 文档

- [安装](./docs/install.cn.md)
- [使用](./docs/usage.cn.md)

## 贡献

参与贡献最简单的方式就是提供反馈。我非常希望听到你认为这个服务缺少些什么。
欢迎提交 [Issue](https://github.com/changkun/midgard/issues/new) 和
[PRs](https://github.com/changkun/midgard/pulls) 提供反馈。

## 致谢

我希望感谢[杨文](https://maiyang.me)和[饶全成](https://qcrao.com)在项目早期进行的有启发性的讨论及测试。

## 许可

版权所有 2020-2021 [欧长坤](https://changkun.de)。保留所有权利。