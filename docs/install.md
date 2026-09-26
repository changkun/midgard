# Midgard Installation

English | [中文](./install.cn.md)

## Architecture

midgard has a server, which you run once, and a program on each of your
devices. The devices keep the clipboard history; the server passes copies
between them and keeps none. [How midgard works](./architecture.md) explains
it, with diagrams. For installing, what matters is:

- **The server** needs a public address, a TLS-terminating reverse proxy in
  front of it (below), and a `data` folder for its database: your devices,
  shares and app tokens, and no copies.
- **Each device** runs `mg daemon`, as its user, in its desktop session: that
  is where the clipboard is. It keeps its history in its user's data
  directory.

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

The web page at `/midgard/` signs people in from a browser. It needs a client
registered with auth.latere.ai (`midgard-web`, with the redirect URI
`https://<your domain>/midgard/.auth/callback`), named by `AUTH_CLIENT_ID`,
and an `AUTH_COOKIE_KEY` to encrypt its session cookie
(`openssl rand -hex 32`). Without them the page says sign-in is not set up,
and everything else works as before.

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

## The Mac App

On a Mac, midgard is an app in the menu bar. It keeps the Mac's clipboard in
sync and its history, as `mg daemon` does elsewhere, and runs the same sync
engine: the app is Swift, for the menu, the window, the clipboard and the
hotkey, and links midgard's Go engine as a library.

```sh
$ make mac                   # needs Go and Xcode's command line tools
$ open apple/build/midgard.app
```

The first time, it asks for your server and signs you in, in the browser. It
then offers, in its menu:

- your recent copies, to put one back on the clipboard;
- the history, in a window, to search, look at, copy back and delete;
- **Share Clipboard at a Link**, also on **Ctrl+Option+S**, which needs no
  Accessibility permission;
- **Pause Syncing**, which stops it reading or writing the Mac's clipboard;
- **Start at Login**.

Copies a password manager marks as secret are not synced, nor kept. The app
and `mg daemon` are the same device to the server, and one of them runs at a
time: stop the daemon first (`mg daemon stop`, then `mg daemon uninstall`).
`mg` commands work beside the app.

The build is signed for your Mac only; to give the app to others, sign it
with a Developer ID and notarize it.

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