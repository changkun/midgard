# Midgard 的安装

[English](./install.md) | 中文

## 架构

了解 midgard 的架构有助于理解其安装所需的必要步骤。Midgard 服务包含三个组件：

- 命令行 CLI
- 守护进程 Daemon
- 服务端 Server

Midgard 的用户可以通过命令行与 midgard 本地守护进程进行通信，守护进程会将
操作转发给 midgard 服务端，并与其他设备进行同步和通信。

```
                            HTTPS
移动端 <-----------------------------------------------┐
                                                      |
命令行 <-------> 守护进程 <-----┐  安全的 Websocket      v     HTTPS
          RPC                 ├--------------------> 服务端 <------> 公开服务
命令行 <-------> 守护进程 <-----┘
```

因为 midgard 旨在作为个人服务运行，没有设计并支持协作访问。任何拥有访问凭据的用户都能访问 midgard 的私有接口。因此它被设计为中心化的访问方式：所有设备都将由个人服务器承担中间代理进行通信：

1. 集中式备份（midgard 服务器将备份剪贴板历史，当前将备份 code2img 和链接历史到指定的 GitHub 仓库）
2. 单个连接的广播（每个设备都需要与服务端建立连接，服务端将通过该连接广播消息）
3. 分布式一致（服务端为 leader）

进而这也存在少许缺陷：

1. 服务端必须保持在线，如果服务端离线，则所有设备将失去同步
2. 不能备份大型文件，基于 GitHub 的备份机制将存在文件上传限制，并且向服务端推送大型文件也会耗尽带宽和服务端磁盘开销

## 依赖

- macOS 端需要安装 Xcode 开发套件：

  ```sh
  $ xcode-select --install
  ```

- Linux 端需要安装 X11 开发文件和 git 工具：

  ```sh
  $ sudo apt install -y git libx11-dev
  ```

- Windows 端需要安装 git 工具：

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

### 容器镜像

容器镜像的构建需要设置环境变量 `SSH_KEY_PATH`，该变量用于指向一个私钥文件（用于同步 GitHub 仓库，例如 RSA, ED25519, 等）。例如：

```sh
$ echo $SSH_KEY_PATH
~/.ssh/id_ed25519
```

构建镜像可以使用下列命令：

```sh
$ make build
```

## 配置

Midgard 的配置文件可以通过下面两种方式进行

1. 默认配置路径：在仓库附属的配置文件 [config.yml](../config.yml)
2. 自定义配置路径：使用环境变量 `MIDGARD_CONF=/path/to/your/config.yml` 修改 [config.yml](../config.yml) 的文件位置

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

midgard 守护进程运行在**本地**（而非服务端），若正确安装为系统进程，则将在开机时自启：

```sh
$ mg daemon install
$ mg daemon start
$ mg daemon stop
$ mg daemon uninstall
```

> Linux 用户需要 `sudo` 权限, Windows 用户则需要将 PowerShell 以管理员身份运行。

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