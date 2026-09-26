# Midgard Installation

English | [中文](./install.cn.md)

## Architecture

Before start installing/using midgard, it is necessary to
understand how midgard works. The midgard service contains three parts:

- CLI
- Daemon
- Server

Each device runs the daemon, which keeps the device's clipboard in sync with
the server over a websocket. The CLI talks to the server directly, over HTTPS,
and phones do the same.

```
Mobile ──────────────── HTTPS ────────────────┐
CLI    ──────────────── HTTPS ────────────────┤
                                              ▼
daemon ◀──────── secure websocket ─────────▶ server ◀── HTTPS ── public links
daemon ◀──────── secure websocket ─────────▶
```

Since midgard serves as a personal service, which does not need to address trust/privacy issue for other customers, it is designed and implemented in a centralized way: everything communicates to a central proxy. This brings several benefits:

1. One place to back up: everything the server keeps is in its `data` folder
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
- `./data` is everything the server keeps. Back it up with the rest of the
  host; there is nothing else to back up.

## Configuration

midgard reads its settings from a `config.yml`. Start from
[config.example.yml](../config.example.yml), which lists every option, and keep
your copy out of git, since a device's may hold an app token. midgard uses the first
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

People sign in through [auth.latere.ai](https://auth.latere.ai). The server
takes its settings from the environment, as changkun.de's other services do:
`AUTH_ALLOWED_PRINCIPALS` lists who may use it, by email or principal id, and
`AUTH_URL` names the issuer (auth.latere.ai by default). Copy
[.env.template](../.env.template) to `.env`, which `docker-compose.yml` reads.
A token is accepted only if it was minted for midgard, by that issuer, for
someone on the list; app tokens stop working when their owner leaves it.

Docker:

```
$ make up
```

> Hint: You need understand how [docker-compose](../docker-compose.yml) works.

Native:

```sh
$ mg server
```

### Moving from an Older Server

An older server kept its shares as files, under `data/repo`. This server
keeps them in its database and serves nothing from disk, so import them once,
as someone's shares; they keep their links. With Docker:

```sh
$ docker compose run --rm midgard import --owner <owner> --dry-run   # what it would do
$ docker compose run --rm midgard import --owner <owner>
```

Natively, `mg server import --owner <owner>`, from the directory the server
runs in. The owner is a principal id, as for app tokens. Hidden files, such as
the old git backup's `.git`, stay behind. Running it again imports only what
is new. Once the old links work, `data/repo` can go.

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
- **Windows:** added to the programs Windows starts when you log in
  (`HKCU\...\CurrentVersion\Run`); no administrator needed. An older
  install registered a Windows service instead, which runs in a session of its
  own and cannot see your clipboard; `mg daemon uninstall`, in a PowerShell
  run as administrator, removes it.

An older `mg daemon install` put a system-wide service in `/etc`, which ran as
root and could not reach anyone's clipboard. `sudo mg daemon uninstall`
removes it.

`mg` commands talk to the server directly, not to the daemon, so they work
whether or not the daemon is running. Older configurations have a `daemon:`
section with `addr: localhost:9125`; it is no longer read, and can go.

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