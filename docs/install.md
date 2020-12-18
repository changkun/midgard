# Midgard Installation

**Warning: Midgard is not yet suitable for non-technical users.**

In order to setup `midgard`, you must configure the two `midgard` components:
`midgard` server and `midgard` daemon.

## TL;DR

```sh
$ git clone https://github.com/changkun/midgard
$ make
$ ln -s "$(pwd)/mg" /usr/local/bin/mg
$ mg version
Vrsion:      v0.0.2-35-ga1d6205
Go version:  go1.15.6
Build time:  2020-12-09T17:33:05+0100
$ mg help
midgard is a mind palace developed by Changkun Ou.
See https://changkun.de/s/midgard for more details.

Usage:
  mg [command]

Available Commands:
  help        Help about any command
  ...

Flags:
  -h, --help   help for mg

Use "mg [command] --help" for more information about a command.
```

## Dependencies

Midgard tries to minimize the number of dependencies, but we cannot build
everything from absolute nothing, the current dependent softwares are:

- `git` for version control
- `xclip` for clipboard on linux
- `chromium-browser` for `code2img`, see more details in https://github.com/chromedp/docker-headless-shell
- `libx11-dev` on Linux

## Build

A single make builds a single command `mg` for you to use midgard:

```sh
$ make
```

## Midgard server

`midgard` server should be **deployed on a server**, to enable midgard
server, one can:

```sh
$ mg server install   # install midgard server as system service
$ mg server start     # start midgard server after installation
$ mg server stop      # stop the running midgard server
$ mg server uninstall # uninstall midgard from system service
```

Or, if you just want run midgard server directly:

```sh
$ mg server run            # run midgard server directly
```

## Midgard daemon

`midgard` daemon process **runs on your local machine**, it responsible for
listening the clipboard, hotkey, and server push events.

```sh
$ mg daemon install   # install midgard daemon as system service
$ mg daemon start     # start midgard daemon after installation
$ mg daemon stop      # stop the running midgard daemon
$ mg daemon uninstall # uninstall midgard from system service
```

> Linux requires `sudo`

Or, if you just want run midgard daemon directly:

```sh
$ mg daemon run       # run midgard daemon directly
```

## Configuration

By default, midgard reads configuration from `./config.yml`, but
you can always override this behavior by environment variable `MIDGARD_CONF`
to specify your customized configuration. For the detailed configuration
items, see [config.yml](./config.yml).


### Backup

Midgard will backup the data folder to a remote Git VCS regularly.
You need provide a link to an repository and specify it in the configuration file.

The first time it will try to initialize the data repo, and later runs will only backup it regularly.

## License

Copyright 2020 [Changkun Ou](https://changkun.de). All rights reserved.