# midgard [![midgard](https://github.com/changkun/midgard/actions/workflows/midgard.yml/badge.svg)](https://github.com/changkun/midgard/actions/workflows/midgard.yml) [![Go Reference](https://pkg.go.dev/badge/changkun.de/x/midgard.svg)](https://pkg.go.dev/changkun.de/x/midgard) ![visitors](https://changkun.de/urlstat?mode=github&repo=changkun/midgard)

English | [中文](./README.cn.md)

midgard is a universal clipboard service, it supports macOS/Linux/Windows/iOS.

Copy on one machine, paste on another. Turn what you copied into a link you can
share. Render code as an image. It all runs on a server you own.

## How it works

You run one **server**. Each of your machines runs a **daemon**, which syncs
that machine's clipboard with the server. The `mg` command talks to the local
daemon, and phones talk to the server directly, through iOS Shortcuts or
Android's Tasker.

## Quick start

**1. The server.** On a machine with a public address:

```sh
$ cp config.example.yml config.yml   # set domain and server.auth.pass
$ make build && make up              # or run: mg server
```

`docker-compose.yml` joins an existing traefik network; see
[Installation](./docs/install.md) to put midgard behind your own reverse proxy.

**2. A token for each device.** On the server, from the directory it runs in:

```sh
$ mg server token add laptop
mgt_...
```

**3. Each device.** Download `mg` from the
[releases](https://github.com/changkun/midgard/releases) (or
`go install changkun.de/x/midgard@latest`, which names it `midgard`), then
put this in `~/.config/midgard/config.yml` on Linux,
`~/Library/Application Support/midgard/config.yml` on macOS, or
`%AppData%\midgard\config.yml` on Windows:

```yaml
domain: example.com   # or http://your-server:8456
token: mgt_...        # from step 2
```

```sh
$ mg daemon install   # as yourself, no sudo
$ mg daemon start
$ mg status
server status: OK
daemon status: OK
```

Now copy something on one device and paste it on another. `mg alloc` turns the
clipboard into a public link, and `mg code2img` turns code into an image; see
[Usage](./docs/usage.md).

## Docs

- [Installation](./docs/install.md)
- [Usage](./docs/usage.md)

## Contributes

Easiest way to contribute is to provide feedback! I would love to hear
what you like and what you think is missing.
[Issue](https://github.com/changkun/midgard/issues/new) and
[PRs](https://github.com/changkun/midgard/pulls) are also welcome.

## Acknowledgment

The author of this project would like to thank
[Wen Yang](https://maiyang.me) and [Quancheng Rao](https://qcrao.com)
for their inspiring discussion and testing in the early stage of the midgard.

## License

Copyright 2020-2021 [Changkun Ou](https://changkun.de). All rights reserved.