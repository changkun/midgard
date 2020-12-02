# midgard

`midgard` is a lightweight solution for managing personal resource namespace.

## Installation

In order to setup `midgard`, you must configure the two `midgard` components:
`midgard` server and `midgard` daemon.

### `midgard` Server

`midgard` server should be **deployed on a server**, to enable midgard
server, one can:

```sh
$ midgard server install   # install midgard server as system service
$ midgard server start     # start midgard server after installation
$ midgard server stop      # stop the running midgard server
$ midgard server uninstall # uninstall midgard from system service
```

Or, if you just want run midgard server directly:

```sh
$ midgard server run            # run midgard server directly
```

### `midgard` Daemon

`midgard` daemon process **runs on your local machine**, it responsible for
listening the clipboard, hotkey, and server push events.

```sh
$ midgard daemon install   # install midgard daemon as system service
$ midgard daemon start     # start midgard daemon after installation
$ midgard daemon stop      # stop the running midgard daemon
$ midgard daemon uninstall # uninstall midgard from system service
```

> Linux requires `sudo`

Or, if you just want run midgard daemon directly:

```sh
$ midgard daemon run            # run midgard daemon directly
```

### Configuration

By default, midgard reads configuration from `./config.yml`, but
you can always override this behavior by environment variable `MIDGARD_CONF`
to specify your customized configuration. For the detailed configuration
items, see [config.yml](./config.yml).

## Usage

`midgard` command line interface (CLI) offers several command to interact
with the midgard server and daemon.

### Status check

Status check gives you option to check if everything is setup correctly:

```sh
$ midgard status
server status: OK
daemon status: OK
```

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

#### iOS Shortcuts support

- `midgard-getclipboard`: https://www.icloud.com/shortcuts/501fe001ebcc444aad1517fdccdbd740
- `midgard-putclipboard`: https://www.icloud.com/shortcuts/587ae52bb5b447e699eb8876107b2e31

With these shortcuts, you can create an automation that runs
the `midgard-getclipboard` when an application is opened (or multiples),
so that the clipboard is fetch from the midgard server automatically.

### Code2image

Whenever you want to convert your clipboard to an image:

```sh
$ midgard code2img
```

Or you can specify a given file:

```sh
$ midgard code2img /path/to/your/file
```

#### iOS Shortcuts support

- `midgard-code2img`: https://www.icloud.com/shortcuts/f5ed10ceb8fa40f393dfc4ebadb0dd89

With these shortcut, you can post your code on an iOS device.
The shortcut will read your clipboard then render it.

## Contributes

Easiest way to contribute is to provide feedback! We would love to hear
what you like and what you think is missing.
[Issue](https://github.com/golang-design/midgard/issues/new) and
[PRs](https://github.com/golang-design/midgard/pulls) are also welcome.

## License

GNU GPL-3.0 Copyright &copy; 2020 The [golang.design](https://golang.design) Initiative Authors.