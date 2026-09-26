# Midgard Installation

English | [中文](./install.cn.md)

## Architecture

Before start installing/using midgard, it is necessary to
understand how midgard works. The midgard service contains three parts:

- CLI
- Daemon
- Server

A user uses midgard CLI communicate with the midgard daemon on local device,
and the daemon process talks to the midgard server for synchornization/allocation
between devices.

```
                            HTTPS
Mobile <-----------------------------------------------┐
                                                       |
CLI    <-------> daemon <-----┐  Secure Websocket      v     HTTPS
          RPC                 ├--------------------> server <------> public
CLI    <-------> daemon <-----┘
```

Since midgard serves as a personal service, which does not need to address trust/privacy issue for other customers, it is designed and implemented in a centralized way: everything communicates to a central proxy. This brings several benefits:

1. Central backup (midgard server backups clipboard history, and currently backups code2img/link history to a GitHub repository)
2. Single connection broadcasting (a device only need a single connection, server broadcasts all messages)
3. Distributed synchronization consistency (server is the lead)

And more :-)

## Dependencies

- macOS (Daemon)

  ```
  $ xcode-select --install
  ```

- Linux (Daemon)

  ```
  $ sudo apt install -y git libx11-dev
  ```

- Windows

  ```
  $ choco install git
  ```

## Build

### Download

Each [release](https://github.com/changkun/midgard/releases) carries a ready
`mg` for macOS, Linux and Windows, on amd64 and arm64: unpack it and put `mg`
on your `PATH`. `checksums.txt` next to the archives lets you verify them.

### Binary Distribution

```
$ git clone https://github.com/changkun/midgard

$ make

$ ln "$(pwd)/mg" /usr/local/bin/mg

$ mg help
midgard is a universal clipboard service.
See https://changkun.de/s/midgard for more details.

Usage:
  mg [command]
```

### Docker Distribution (Recommended)

Build the server image with `make build`; each release also publishes it as
`ghcr.io/changkun/midgard`. The image holds no configuration and no keys.
[docker-compose.yml](../docker-compose.yml) mounts them when the container
starts:

- `./config.yml` is your configuration, read-only;
- `./data` is where published files and backups live;
- for backups over ssh, uncomment the key mount and `GIT_SSH_COMMAND`, and
  use an ssh URL for `server.store.backup.repo`, such as
  `git@github.com:you/midgard-data.git`. Set `GIT_AUTHOR_NAME` and friends to
  sign the backup commits as yourself.

## Configuration

midgard reads its settings from a `config.yml`. Start from
[config.example.yml](../config.example.yml), which lists every option, and keep
your copy out of git: it holds the server password. midgard uses the first
`config.yml` it finds:

1. the file named by the `MIDGARD_CONF` environment variable, for example
   `MIDGARD_CONF=/path/to/your/config.yml`;
2. `config.yml` in the directory you run `mg` from;
3. `midgard/config.yml` in your user configuration directory:
   `~/.config/midgard/config.yml` on Linux,
   `~/Library/Application Support/midgard/config.yml` on macOS, and
   `%AppData%\midgard\config.yml` on Windows.

For a daemon that starts with your machine, use the third location: a service
has no useful working directory. Commands such as `mg version` need no
configuration at all.

## Midgard Server

Docker:

```
$ make up
```

> Hint: You need understand how [docker-compose](../docker-compose.yml) works.

Native:

```sh
$ mg server
```

## Midgard Daemon

The `midgard` daemon **runs on each of your machines** and starts when you log
in. Install it as yourself, without `sudo`: it syncs your desktop's
clipboard, which a system service cannot reach.

```sh
$ mg daemon install
$ mg daemon start
$ mg daemon stop
$ mg daemon uninstall
```

- **macOS:** a LaunchAgent in `~/Library/LaunchAgents`.
- **Linux:** a systemd user unit in `~/.config/systemd/user`, started with
  your graphical session; without systemd, an autostart entry in
  `~/.config/autostart`. GNOME and KDE start the session for you. Under a
  compositor that does not, such as sway or Hyprland, add
  `exec systemctl --user start midgard-daemon` to its configuration after
  importing `WAYLAND_DISPLAY` into systemd.
- **Windows:** run PowerShell as administrator to install.

An older `mg daemon install` put a system-wide service in `/etc`, which ran as
root and could not reach anyone's clipboard. `sudo mg daemon uninstall`
removes it.

or

```sh
$ mg daemon run
```

## Reverse Proxy

If midgard is deployed behind an nginx server, then the following
configuration could help:

```
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

If you use traefik, then the following configuration could help (see [changkun/proxy](https://changkun.de/s/proxy) as a complete example):

- **Static configuration**:

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

- **Dynamic configuration**:

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

## License

Copyright 2020-2021 [Changkun Ou](https://changkun.de). All rights reserved.