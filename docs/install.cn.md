# 安装 midgard

[English](./install.md) | 中文

midgard 由两部分组成：**一个服务端**，运行在可公开访问的地方；以及**你的设备**，保存你的剪贴板和历史记录。服务端负责在你的设备之间传递复制内容，自身不保存任何内容，详见[midgard 的工作方式](./architecture.cn.md)。

1. [运行服务端](#运行服务端)，只需一次。
2. [设置每台设备](#设置你的设备)：Mac 上使用 Midgard 应用，Linux 和 Windows 上使用 `mg daemon`，手机上使用网页。

## 运行服务端

你需要一台有公网地址并安装了 Docker 的机器、一个域名，以及一个在 midgard 前面提供 HTTPS 的反向代理。

**1. 获取代码并构建镜像。**

```sh
$ git clone https://github.com/changkun/midgard && cd midgard
$ make build              # 构建 midgard:latest 镜像
```

发布版本也会以 `ghcr.io/changkun/midgard` 发布镜像。

**2. 配置。** 两个文件，都不要提交到 git：

```sh
$ cp config.example.yml config.yml   # 设置 domain：你的域名
$ cp .env.template .env              # 设置允许登录的人
```

在 `.env` 中：

- `AUTH_ALLOWED_PRINCIPALS`：允许使用该服务端的人，邮箱或主体 ID，以逗号分隔。所有人都通过 [auth.latere.ai](https://auth.latere.ai) 登录；由这份名单决定谁能进入，每个人只能访问自己的剪贴板。
- `AUTH_CLIENT_ID` 和 `AUTH_COOKIE_KEY`：用于网页登录。客户端为 `midgard-web`，在 auth.latere.ai 注册的回调地址是 `https://changkun.de/midgard/.auth/callback`；在其他域名上，请注册你自己的客户端和回调地址。密钥用于加密会话 Cookie：`openssl rand -hex 32`。两者都留空则不启用网页登录，其余功能不受影响。

**3. 启动。**

```sh
$ make up                 # docker compose up -d
$ curl https://your.domain/midgard/ping
{"version":"…","go_version":"…","build_time":"…"}
```

[docker-compose.yml](../docker-compose.yml) 在网络 `traefik_proxy` 上运行名为 `midgard` 的容器，以只读方式挂载 `config.yml`，并把服务端保存的一切放在 `./data` 中：你的设备、分享和应用令牌，不包括任何复制内容。备份 `./data` 即可，没有别的需要备份。

**4. 放到反向代理后面**，路径为 `/midgard`，需要允许 websocket，并且请求体上限要大于 32 MB（单次复制或分享的最大大小）。

使用 traefik，与容器处于同一网络（changkun.de 自己的完整配置，包括 traefik，见 [changkun/web](https://github.com/changkun/web)）：

```yaml
http:
  routers:
    to-midgard:
      rule: "Host(`your.domain`) && PathPrefix(`/midgard`)"
      tls:
        certResolver: yourResolver
      service: midgard
  services:
    midgard:
      loadBalancer:
        servers:
          - url: http://midgard
```

使用 nginx，需要在主机上发布 midgard 的端口，例如在 `docker-compose.yml` 的 `ports:` 下写 `127.0.0.1:8456:80`：

```nginx
location /midgard {
    proxy_pass              http://127.0.0.1:8456;
    proxy_set_header        Host              $host;
    proxy_set_header        X-Forwarded-For   $proxy_add_x_forwarded_for;
    proxy_set_header        X-Forwarded-Proto $scheme;
    proxy_http_version      1.1;
    proxy_set_header        Upgrade           $http_upgrade;
    proxy_set_header        Connection        "upgrade";
    proxy_read_timeout      120s;   # 设备每 30 秒 ping 一次
    client_max_body_size    48m;
}
```

midgard 只信任来自本机回环地址和私有网络的 `X-Forwarded-For`（反向代理通常位于这些地址）；否则请设置 `server.trusted_proxies`。登录失败次数按地址统计，依据的就是它。

### 提供 Mac 应用下载

服务端的 `data/downloads/Midgard.dmg` 存在时，网页就会提供 Mac 应用下载。在装有 Go 和 Xcode 命令行工具的 Mac 上构建同时支持 Apple 芯片和 Intel 的磁盘映像，复制过去即可，网页随即给出下载链接：

```sh
$ make dmg
$ scp apple/build/Midgard.dmg your.server:midgard/data/downloads/
```

新的构建以同名文件替换旧的即可。

自己构建的磁盘映像只为你自己的 Mac 签名，其他 Mac 会拒绝打开，直到有人在“隐私与安全性”中点击**仍要打开**。要让 Apple 公证它，你需要自己的 Developer ID：把 `MIDGARD_SIGN` 设为它的 Developer ID Application 身份，把 `MIDGARD_NOTARY` 设为 `xcrun notarytool store-credentials` 保存的钥匙串配置名称，`make dmg` 就会签名并让 Apple 公证。

### 从旧版服务端迁移

旧版服务端把分享以文件形式保存在 `data/repo` 下，并在 `data/logs` 下保存每一次复制的明文日志。把分享一次性导入为某人的分享，链接保持不变。日志不导入：服务端现在不保存复制内容，日志可以删除。

```sh
$ docker compose run --rm -v /path/to/old/data/repo:/app/old:ro \
    midgard import --owner <主体 ID> --from /app/old --dry-run   # 预览
$ docker compose run --rm -v /path/to/old/data/repo:/app/old:ro \
    midgard import --owner <主体 ID> --from /app/old
```

你的主体 ID 就是你首次登录时服务端记录的所有者。隐藏文件（例如旧 git 备份的 `.git`）不会导入。重复执行只会导入新增的文件。

### 升级到加密版本

从这个版本起，复制内容用你的设备共享的密钥加密，服务端无法读取（[midgard 的工作方式](./architecture.cn.md#加密服务端能看到什么)）。升级服务端之后：

- 第一台以此版本连接的设备会生成你的密钥；其他设备各与它配对一次（[为设备配对](./usage.cn.md#为设备配对)）；
- 旧版应用或守护进程会被拒绝，并提示更新；请更新它们；
- iPhone 快捷指令需要从网页重新添加，并且只能经由你允许它们进入的 Mac，以明文访问你的复制内容。

你的设备保留各自原有的历史记录。

## 设置你的设备

每台设备都通过 auth.latere.ai 登录一次，并且必须在服务端的白名单上。第一台连接的设备会生成加密复制内容的密钥；其他设备各用一个配对码与它配对一次：[为设备配对](./usage.cn.md#为设备配对)。

### Mac：Midgard 应用

Midgard 常驻菜单栏，需要 macOS 14 或更高版本。在服务端的网页上点 **Download for Mac** 下载，或从[发布页](https://github.com/changkun/midgard/releases)下载 `Midgard.dmg`；打开磁盘映像，把 Midgard 拖到“应用程序”。

它已签名并经过 Apple 公证，像其他应用一样直接打开即可。v0.2.0 及更早版本的磁盘映像没有公证：第一次打开时 Mac 会拒绝，需要在“系统设置”的“隐私与安全性”中点击**仍要打开**。

首次启动时，它会询问你的服务端，并在浏览器中引导你登录。之后菜单中有：最近的复制内容，可放回剪贴板；历史记录窗口；**将剪贴板分享为链接**，快捷键 **Ctrl+Option+S**，无需辅助功能权限；**暂停同步**；以及**登录时启动**。密码管理器标记为机密的内容不会被同步，也不会被保存。

应用与 `mg daemon` 在服务端看来是同一台设备，同一时间只能运行其中一个：如果这台 Mac 装了守护进程，请先停止它（`mg daemon stop`，然后 `mg daemon uninstall`）。

### Linux 和 Windows：`mg daemon`

**1. 获取 `mg`**：从 [releases](https://github.com/changkun/midgard/releases) 下载对应系统的版本并放入 `PATH`；压缩包旁的 `checksums.txt` 可用于校验。或者自行构建：`go install changkun.de/x/midgard@latest`，生成的程序名为 `midgard`。

**2. 指定你的服务端**：Linux 写入 `~/.config/midgard/config.yml`，Windows 写入 `%AppData%\midgard\config.yml`：

```yaml
domain: your.domain   # 或 http://your-server:8456
```

**3. 登录并安装守护进程**，以当前用户身份，无需 `sudo`：它同步的是你桌面会话的剪贴板，系统服务无法访问。

```sh
$ mg login
$ mg daemon install
$ mg daemon start
$ mg status
server status: OK
daemon status: OK
```

之后它会在你登录时启动：Linux 上作为随图形会话启动的 systemd 用户单元，没有 systemd 时作为自启动项；Windows 上加入登录时启动的程序，无需管理员权限。

- 在不启动图形会话的合成器（如 sway 或 Hyprland）下，先把 `WAYLAND_DISPLAY` 导入 systemd，再在其配置中加入 `exec systemctl --user start midgard-daemon`。
- 旧版 `mg daemon install` 安装的是系统级服务，无法访问任何人的剪贴板：Linux 上用 `sudo mg daemon uninstall`，Windows 上在以管理员身份运行的 PowerShell 中执行 `mg daemon uninstall` 即可移除。
- `mg daemon run` 则在终端中运行守护进程。

### 手机

打开 `https://your.domain/midgard/` 并登录：可以查看剪贴板、向设备发送文本、浏览历史记录和分享。iOS 快捷指令使用应用令牌，详见[使用](./usage.cn.md)。

### 没有浏览器的机器，或智能体

在网页的 **App tokens** 中签发应用令牌，或在服务端执行：

```sh
$ docker compose run --rm midgard token add build-box --owner <主体 ID>
```

然后在那台机器的 `config.yml` 中写入令牌，代替 `mg login`：

```yaml
domain: your.domain
token: mgt_...
```

## 参考

### `config.yml` 的查找顺序

midgard 使用以下位置中第一个存在的文件：

1. `MIDGARD_CONF` 指定的文件；
2. 运行 `mg` 的目录下的 `config.yml`；
3. 用户配置目录下的 `midgard/config.yml`：Linux 为 `~/.config/midgard/`，macOS 为 `~/Library/Application Support/midgard/`，Windows 为 `%AppData%\midgard\`。

随系统启动的守护进程没有有意义的工作目录，请使用第三个位置。[config.example.yml](../config.example.yml) 列出了所有设置。

### 设备在哪里保存数据

在上述配置目录中：`config.yml`、登录信息（`token.json`）和设备 ID（`device`）。历史记录在 Linux 上位于 `~/.local/share/midgard/history.db`，在 macOS 和 Windows 上位于配置目录中，仅你本人可读。

### 从源码构建

需要 `go.mod` 中指定的 Go 版本。守护进程的剪贴板和快捷键在 Linux 上需要 `sudo apt install -y libx11-dev`，在 macOS 上需要 `xcode-select --install`。然后 `make` 构建 `mg`，`make build` 构建服务端镜像，`make mac` 把 Mac 应用构建到 `apple/build`，`make dmg` 构建同时支持两种 Mac 的磁盘映像。

## 许可

版权所有 2020-2026 [欧长坤](https://changkun.de)。保留所有权利。
