# Midgard Installation

In order to setup `midgard`, you must configure the two `midgard` components:
`midgard` server and `midgard` daemon.

## Build

A single make builds a single command `mg` for you to use midgard:

```sh
$ make
```

## `midgard` Server

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

## `midgard` Daemon

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
$ mg daemon run            # run midgard daemon directly
```

## Configuration

By default, midgard reads configuration from `./config.yml`, but
you can always override this behavior by environment variable `MIDGARD_CONF`
to specify your customized configuration. For the detailed configuration
items, see [config.yml](./config.yml).


### Repository Backup

**Midgard will backup the data folder to GitHub regularly.
You need create an empty repository on GitHub and specify it in the configuration file.**

The first time it will try to initialize the data repo, and later runs will only backup it regularly.

## License

Copyright 2020 [Changkun Ou](https://changkun.de). All rights reserved.