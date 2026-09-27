# Installing midgard

English | [中文](./install.cn.md)

midgard has two parts: **one server**, which you run somewhere public, and
**your devices**, which keep your clipboard and its history. The server
passes copies between your devices and keeps none of them; see
[How midgard works](./architecture.md).

1. [Run the server](#run-the-server), once.
2. [Set up each device](#set-up-your-devices): the Midgard app on a Mac,
   `mg daemon` on Linux and Windows, the web page on a phone.

## Run the Server

You need a machine with a public address and Docker, a domain, and a reverse
proxy in front of midgard that serves it over HTTPS.

**1. Get the code and the image.**

```sh
$ git clone https://github.com/changkun/midgard && cd midgard
$ make build              # builds the midgard:latest image
```

Releases also publish the image as `ghcr.io/changkun/midgard`.

**2. Configure it.** Two files, both kept out of git:

```sh
$ cp config.example.yml config.yml   # set domain: your domain
$ cp .env.template .env              # set who may sign in
```

In `.env`:

- `AUTH_ALLOWED_PRINCIPALS`: who may use this server, by email or principal
  id, comma-separated. Everyone signs in through
  [auth.latere.ai](https://auth.latere.ai); the list decides who is let in,
  and each person reaches only their own clipboard.
- `AUTH_CLIENT_ID` and `AUTH_COOKIE_KEY`: for signing in to the web page.
  The client is `midgard-web`, registered with auth.latere.ai for
  `https://changkun.de/midgard/.auth/callback`; on another domain, register a
  client of your own with your callback. The key encrypts the session cookie:
  `openssl rand -hex 32`. Leave both empty to go without the web sign-in;
  everything else works.

**3. Start it.**

```sh
$ make up                 # docker compose up -d
$ curl https://your.domain/midgard/ping
{"version":"…","go_version":"…","build_time":"…"}
```

[docker-compose.yml](../docker-compose.yml) runs a container named `midgard`
on the network `traefik_proxy`, mounts `config.yml` read-only, and keeps
everything the server stores in `./data`: your devices, shares and app
tokens, and no copies. Back up `./data`; there is nothing else to back up.

**4. Put it behind your reverse proxy** at `/midgard`, with websockets
allowed, and a body limit above 32 MB, the size of the largest copy or
share.

With traefik, next to the container on its network (changkun.de's own
setup, traefik and all, is [changkun/web](https://github.com/changkun/web)):

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

With nginx, with midgard's port published on the host, for example as
`127.0.0.1:8456:80` under `ports:` in `docker-compose.yml`:

```nginx
location /midgard {
    proxy_pass              http://127.0.0.1:8456;
    proxy_set_header        Host              $host;
    proxy_set_header        X-Forwarded-For   $proxy_add_x_forwarded_for;
    proxy_set_header        X-Forwarded-Proto $scheme;
    proxy_http_version      1.1;
    proxy_set_header        Upgrade           $http_upgrade;
    proxy_set_header        Connection        "upgrade";
    proxy_read_timeout      120s;   # devices ping every 30 s
    client_max_body_size    48m;
}
```

midgard believes `X-Forwarded-For` from loopback and the private networks
only, where a proxy usually is; set `server.trusted_proxies` otherwise. It
is what failed sign-ins are counted by.

### Offering the Mac App

The web page offers the Mac app for download once the server has it, in
`data/downloads/Midgard.dmg`. Build the disk image, for Apple silicon and
Intel both, on a Mac with Go and Xcode's command line tools, and copy it
there; the page links to it from then on:

```sh
$ make dmg
$ scp apple/build/Midgard.dmg your.server:midgard/data/downloads/
```

A newer build replaces it under the same name.

### Moving from an Older Server

An older server kept its shares as files, under `data/repo`, and a
plaintext log of every copy under `data/logs`. Import the shares once, as
someone's, and they keep their links. The log is not imported: the server
keeps no copies now, and it may be deleted.

```sh
$ docker compose run --rm -v /path/to/old/data/repo:/app/old:ro \
    midgard import --owner <principal id> --from /app/old --dry-run   # what it would do
$ docker compose run --rm -v /path/to/old/data/repo:/app/old:ro \
    midgard import --owner <principal id> --from /app/old
```

Your principal id is the owner the server records when you first sign in.
Hidden files, such as the old git backup's `.git`, stay behind. Running it
again imports only what is new.

## Set Up Your Devices

Every device signs in once, through auth.latere.ai, and must be on the
server's allowlist.

### A Mac: the Midgard App

Midgard lives in the menu bar, on macOS 14 or later. Download it from your
server's web page, **Download for Mac**, or `Midgard.dmg` from the
[releases](https://github.com/changkun/midgard/releases); open the disk image,
and drag Midgard to Applications.

It is not notarized by Apple yet, so the first time you open it the Mac
refuses: in System Settings, under Privacy & Security, click **Open
Anyway**. Or, once, in a terminal:
`xattr -dr com.apple.quarantine /Applications/Midgard.app`.

The first time, it asks for your server and signs you in, in the browser.
Its menu then has your recent copies, to put one back on the clipboard; the
history, in a window; **Share Clipboard at a Link**, also on
**Ctrl+Option+S**, with no Accessibility permission needed; **Pause
Syncing**; and **Start at Login**. Copies a password manager marks as
secret are not synced, nor kept.

The app and `mg daemon` are the same device to the server, and one runs at a
time: on a Mac with the daemon installed, stop it first (`mg daemon stop`,
then `mg daemon uninstall`).

### Linux and Windows: `mg daemon`

**1. Get `mg`**, from the [releases](https://github.com/changkun/midgard/releases)
for your system, and put it on your `PATH`; `checksums.txt` next to the
archives verifies them. Or build it: `go install changkun.de/x/midgard@latest`,
which names it `midgard`.

**2. Point it at your server**, in `~/.config/midgard/config.yml` on Linux or
`%AppData%\midgard\config.yml` on Windows:

```yaml
domain: your.domain   # or http://your-server:8456
```

**3. Sign in, and install the daemon**, as yourself, without `sudo`: it syncs
your desktop's clipboard, which a system service cannot reach.

```sh
$ mg login
$ mg daemon install
$ mg daemon start
$ mg status
server status: OK
daemon status: OK
```

It then starts when you log in: on Linux as a systemd user unit, started
with your graphical session, or an autostart entry without systemd; on
Windows from the programs started at logon, with no administrator needed.

- Under a compositor that starts no graphical session, such as sway or
  Hyprland, add `exec systemctl --user start midgard-daemon` to its
  configuration, after importing `WAYLAND_DISPLAY` into systemd.
- An older `mg daemon install` made a system-wide service, which could not
  reach anyone's clipboard: `sudo mg daemon uninstall` on Linux, or
  `mg daemon uninstall` in a PowerShell run as administrator on Windows,
  removes it.
- `mg daemon run` runs the daemon in the terminal instead.

### A Phone

Open `https://your.domain/midgard/` and sign in: your clipboard, sending
text to your devices, your history, and your shares. iOS Shortcuts use an
app token; see [Usage](./usage.md#app-tokens).

### A Machine Without a Browser, or an Agent

Issue an app token on the web page, under **App tokens**, or on the server:

```sh
$ docker compose run --rm midgard token add build-box --owner <principal id>
```

and put it in that machine's `config.yml` instead of running `mg login`:

```yaml
domain: your.domain
token: mgt_...
```

## Reference

### Where `config.yml` Is Found

midgard uses the first of:

1. the file named by `MIDGARD_CONF`;
2. `config.yml` in the directory you run `mg` from;
3. `midgard/config.yml` in your configuration directory:
   `~/.config/midgard/` on Linux, `~/Library/Application Support/midgard/`
   on macOS, `%AppData%\midgard\` on Windows.

A daemon that starts with your machine has no useful working directory: use
the third. [config.example.yml](../config.example.yml) lists every setting.

### Where a Device Keeps Its Data

In the configuration directory above: `config.yml`, the sign-in
(`token.json`), and the device's id (`device`). The history is in
`~/.local/share/midgard/history.db` on Linux, and in the configuration
directory on macOS and Windows, readable by you alone.

### Building from Source

Go, as `go.mod` says. The daemon's clipboard and hotkey need
`sudo apt install -y libx11-dev` on Linux, and `xcode-select --install` on
macOS. Then `make` builds `mg`, `make build` the server image, `make mac` the
Mac app, into `apple/build`, and `make dmg` its disk image for both kinds
of Mac.

## License

Copyright 2020-2026 [Changkun Ou](https://changkun.de). All rights reserved.
