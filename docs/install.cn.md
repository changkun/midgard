# Midgard 的安装

[English](./install.md) | 中文

## 架构

了解 midgard 的架构有助于理解其安装所需的必要步骤。Midgard 服务包含三个组件：

- 命令行 CLI
- 守护进程 Daemon
- 服务端 Server

每台设备运行守护进程，通过 websocket 与服务端保持剪贴板同步。命令行 CLI 与手机一样，通过 HTTPS 直接访问服务端。

```
手机   ──────────────── HTTPS ────────────────┐
CLI    ──────────────── HTTPS ────────────────┤
                                              ▼
守护进程 ◀────── secure websocket ─────────▶ 服务端 ◀── HTTPS ── 公开链接
守护进程 ◀────── secure websocket ─────────▶
```

因为 midgard 旨在作为个人服务运行，没有设计并支持协作访问。任何拥有访问凭据的用户都能访问 midgard 的私有接口。因此它被设计为中心化的访问方式：所有设备都将由个人服务器承担中间代理进行通信：

1. 集中备份：服务端保存的一切都在其 `data` 目录中
2. 单个连接的广播（每个设备都需要与服务端建立连接，服务端将通过该连接广播消息）
3. 分布式一致（服务端为 leader）

进而这也存在少许缺陷：

1. 服务端必须保持在线，如果服务端离线，则所有设备将失去同步
2. 不能备份大型文件，基于 GitHub 的备份机制将存在文件上传限制，并且向服务端推送大型文件也会耗尽带宽和服务端磁盘开销

## 依赖

- macOS 端（仅客户端）需要安装 Xcode 开发套件：

  ```sh
  $ xcode-select --install
  ```

- Linux 端（仅客户端）需要安装 X11 开发文件和 git 工具：

  ```sh
  $ sudo apt install -y git libx11-dev
  ```

- Windows 端（仅客户端）需要安装 git 工具：

  ```sh
  $ choco install git
  ```

## 构建

### 二进制文件

```sh
# 克隆 midgard 代码仓库
$ git clone https://github.com/changkun/midgard

# 编译二进制文件
$ make

# 将 midgard 命令行文件安装到系统
$ ln "$(pwd)/mg" /usr/local/bin/mg

# 使用 help 子命令验证安装是否成功
$ mg help
midgard is a universal clipboard service.
See https://changkun.de/s/midgard for more details.

Usage:
  mg [command]
```

### 容器镜像（推荐）

使用 `make build` 构建服务端镜像；每次发布也会以 `ghcr.io/changkun/midgard` 发布该镜像。镜像中不包含任何配置或密钥，[docker-compose.yml](../docker-compose.yml) 会在容器启动时挂载：

- `./config.yml` 为你的配置文件，只读挂载；
- `./data` 为服务端保存的全部数据，随主机一同备份即可，没有其他需要备份的内容。

## 配置

midgard 从 `config.yml` 读取配置。请以 [config.example.yml](../config.example.yml)（列出了全部选项）为模板，并且不要把自己的配置提交到 git：其中包含服务端密码。midgard 按以下顺序使用找到的第一个 `config.yml`：

1. 环境变量 `MIDGARD_CONF` 指定的文件，例如 `MIDGARD_CONF=/path/to/your/config.yml`；
2. 运行 `mg` 时所在目录下的 `config.yml`；
3. 用户配置目录下的 `midgard/config.yml`：Linux 为 `~/.config/midgard/config.yml`，
   macOS 为 `~/Library/Application Support/midgard/config.yml`，Windows 为 `%AppData%\midgard\config.yml`。

随系统启动的 daemon 请使用第三个位置，因为系统服务没有可用的工作目录。`mg version` 等命令不需要任何配置。

## Midgard 服务端

从 Docker 启动 midgard 服务端可以使用下列命令:

```
$ make up
```

> 提示: 需要安装 [docker-compose](../docker-compose.yml).

或直接运行二进制文件:

```sh
$ mg server
```

## Midgard 守护进程

midgard 守护进程运行在**每一台设备上**（而非服务端），登录后自动启动。请以当前用户身份安装，不要使用 `sudo`：它同步的是你的桌面剪贴板，系统服务无法访问。

```sh
$ mg daemon install
$ mg daemon start
$ mg daemon stop
$ mg daemon uninstall
```

- **macOS：** 安装为 `~/Library/LaunchAgents` 中的 LaunchAgent。
- **Linux：** 安装为 `~/.config/systemd/user` 中的 systemd 用户单元，随图形会话启动；没有 systemd 时，安装为 `~/.config/autostart` 中的自启动项。GNOME 与 KDE 会自动启动图形会话；sway、Hyprland 等合成器需要在导入 `WAYLAND_DISPLAY` 之后，于其配置中加入 `exec systemctl --user start midgard-daemon`。
- **Windows：** 加入登录时启动的程序（`HKCU\...\CurrentVersion\Run`），无需管理员权限。旧版安装会注册为 Windows 服务，它运行在独立的会话中，看不到你的剪贴板；以管理员身份运行 PowerShell 执行 `mg daemon uninstall` 即可移除。

旧版 `mg daemon install` 会在 `/etc` 中安装以 root 运行的系统服务，它无法访问任何人的剪贴板。`sudo mg daemon uninstall` 可将其移除。

`mg` 命令直接与服务端通信，而不经过守护进程，因此无论守护进程是否运行都可以使用。旧配置中的 `daemon:` 部分（`addr: localhost:9125`）已不再读取，可以删除。

若不需要安装为系统进程，则可直接使用下列命令运行在终端中（使用 Ctrl+C 退出）：

```sh
$ mg daemon run
```

## 反向代理

如果 midgard 部署在一个 nginx 服务后，可以使用下面的配置来支持 `/midgard` 路由：

```conf
location /midgard {
    proxy_pass          http://0.0.0.0:80;
    proxy_set_header    Host             $host;
    proxy_set_header    X-Real-IP        $remote_addr;
    proxy_set_header    X-Forwarded-For  $proxy_add_x_forwarded_for;
    proxy_set_header    X-Client-Verify  SUCCESS;
    proxy_set_header    X-Client-DN      $ssl_client_s_dn;
    proxy_set_header    X-SSL-Subject    $ssl_client_s_dn;
    proxy_set_header    X-SSL-Issuer     $ssl_client_i_dn;

    # websocket support
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    client_max_body_size 2M;
}
```

如果使用 traefik，可以参考下面的配置文件（或参见 [changkun/proxy](https://changkun.de/s/proxy) 作为一个完整的示例）：

- **静态配置**:

  ```yaml
  entryPoints:
    web:
      address: :80
      http:
        redirections:
          entryPoint:
            to: websecure
            scheme: https
    websecure:
      address: :443

  certificatesResolvers:
    changkunResolver:
      acme:
        email: your@email.com
        storage: /path/to/your/acme.json
        httpChallenge:
          entryPoint: web
  ```

- **动态配置**:

  ```yaml
  http:
    routers:
      to-midgard:
        rule: "Host(`example.com`)&&PathPrefix(`/midgard`)"
        tls:
          certResolver: yourCertResolver
        service: midgard
    services:
      midgard:
        loadBalancer:
          servers:
          - url: http://midgard
  ```

## 许可

版权所有 2020-2021 [欧长坤](https://changkun.de)。保留所有权利。