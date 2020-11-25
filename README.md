# midgard

`midgard` is a lightweight solution for managing personal resource namespace.

## Setup

In order to setup `midgard`, you must configure the two `midgard` components:
`midgard` server and `midgard` daemon.

### `midgard` Server

`midgard` server should be deployed on a server, one can use the following command:

```sh
$ midgard server
```

### `midgard` Daemon

`midgard` daemon process runs on your local machine, it responsible for
listening the clipboard, hotkey, and server push events.

```sh
$ midgard daemon
```

### Configuration

```yaml
---
title: "The golang.design Initiative"

# server includes all midgard server side settings
# these settings are only used in server mode (run under `midgard serve`)
server:
  http: :8080
  rpc: :8081
  mode: debug # or debug/release/test
  store:
    prefix: /fs  # this will be your namespace prefix, i.e. golang.design/midgard/fs/*
    path: ./data # this is where your data stored on your server
  auth:
    # the following two configures your midgard credentials
    user: golang-design
    pass: aBWJnteJbt!j3G!qehLnJmbcgLqkkXuEusz9m4@JeqUqwZD*Dc

# daemon includes all midgard daemon settings
# these settings are only used in daemon mode (run under `midgard daemon`)
daemon:
  server_addr: https://golang.design
```

## Usage

`midgard` command line interface (CLI) offers several command to interact
with the midgard server and daemon.

### Allocate A Global URL

You can use `midgard new` to allocate a global url to persist your data,
for example:

```sh
$ midgard new /awesome/filename -f /path/to/your/file
DONE: https://golang.design/midgard/fs/awesome/filename
```

The first argument of `new` subcommand indicates the desired URI,
and `-f` flag indicates the file you want to put to your server.

You can omit the `-f` flag and leave it empty, then the `new` subcommand
will request the server to use your universal clipboard data.

You can even omit the argument of `new`, then the `midgard` server will
create a random path under `/wild`. For instance:

```sh
$ midgard new
DONE: https://golang.design/midgard/fs/wild/fboVP8u4xNMHfvsv2EeLzL.txt
```

It automatically writes to your clipboard and you can directly paste
it to anywhere else that you want.

### Universal Clipboard

`midgard` daemon watches your system clipboard and automatically sync
your clipboard with the `midgard` server. Thus, a possible usage of
`midgard` is:

1. Take a screenhot of your desktop,
2. Use `midgard new`

This will return you a public accessible URL and write back into your local
clipboard so that you can immediately paste to anywhere you want.

Furthermore, with the built-in universal clipboard, you can even share
your clipboard cross platforms (e.g. between Mac and Linux).

## Contributes

Easiest way to contribute is to provide feedback! We would love to hear
what you like and what you think is missing.
[Issue](https://github.com/golang-design/midgard/issues/new) and
[PRs](https://github.com/golang-design/midgard/pulls) are also welcome.

## License

GNU GPL-3.0 Copyright &copy; 2020 The [golang.design](https://golang.design) Initiative Authors.