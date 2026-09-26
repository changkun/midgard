# Midgard 的安装

[English](./install.md) | 中文

## 架构

midgard 由一个服务端（只需运行一个）和每台设备上的程序组成。历史记录保存在设备上；服务端负责在设备之间传递复制内容，自身不保存任何内容。[midgard 的工作方式](./architecture.cn.md)有带图示的完整说明。就安装而言，需要了解的是：

- **服务端**需要一个公开地址、一个负责 TLS 的反向代理（见下文），以及存放数据库的 `data` 目录：其中保存你的设备、分享和应用令牌，不保存复制内容。
- **每台设备**以当前用户身份、在桌面会话中运行 `mg daemon`，因为剪贴板就在那里。历史记录保存在该用户的数据目录中。

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

midgard 从 `config.yml` 读取配置。请以 [config.example.yml](../config.example.yml)（列出了全部选项）为模板，并且不要把自己的配置提交到 git：设备的配置中可能包含应用令牌。midgard 按以下顺序使用找到的第一个 `config.yml`：

1. 环境变量 `MIDGARD_CONF` 指定的文件，例如 `MIDGARD_CONF=/path/to/your/config.yml`；
2. 运行 `mg` 时所在目录下的 `config.yml`；
3. 用户配置目录下的 `midgard/config.yml`：Linux 为 `~/.config/midgard/config.yml`，
   macOS 为 `~/Library/Application Support/midgard/config.yml`，Windows 为 `%AppData%\midgard\config.yml`。

随系统启动的 daemon 请使用第三个位置，因为系统服务没有可用的工作目录。`mg version` 等命令不需要任何配置。

## Midgard 服务端

用户通过 [auth.latere.ai](https://auth.latere.ai) 登录。服务端从环境变量读取设置：
`AUTH_ALLOWED_PRINCIPALS` 列出允许使用的人（邮箱或主体 ID），`AUTH_URL` 指定签发方（默认 auth.latere.ai）。
将 [.env.template](../.env.template) 复制为 `.env`，`docker-compose.yml` 会读取它。
`/midgard/` 的网页需要在 auth.latere.ai 注册的客户端（`midgard-web`，回调地址
`https://<你的域名>/midgard/.auth/callback`），由 `AUTH_CLIENT_ID` 指定，
并用 `AUTH_COOKIE_KEY`（`openssl rand -hex 32`）加密会话 Cookie。未设置时网页会提示未开启登录，其余功能不受影响。

从 Docker 启动 midgard 服务端可以使用下列命令:

```
$ make up
```

> 提示: 需要安装 [docker-compose](../docker-compose.yml).

或直接运行二进制文件:

```sh
$ mg server
```

### 从旧版服务端迁移

旧版服务端将分享以文件形式保存在 `data/repo` 下。新版服务端将分享保存在数据库中，
不再从磁盘提供文件，因此需要将它们一次性导入为某人的分享；原有链接保持不变。使用 Docker：

```sh
$ docker compose run --rm midgard import --owner <owner> --dry-run   # 预览
$ docker compose run --rm midgard import --owner <owner>
```

直接运行时，在服务端的运行目录下执行 `mg server import --owner <owner>`。
owner 是主体 ID，与应用令牌相同。隐藏文件（例如旧 git 备份的 `.git`）不会导入。
重复执行只会导入新增的文件。确认旧链接可用后即可删除 `data/repo`。

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