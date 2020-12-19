# Midgard Installation
## Dependencies

macOS:

```
$ xcode-select --install
```

Linux:

```
$ sudo apt install -y git xclip libx11-dev
```

## Build

```
$ git clone https://github.com/changkun/midgard
$ make
$ ln -s "$(pwd)/mg" /usr/local/bin/mg
$ mg help
midgard is a universal clipboard service developed by Changkun Ou.
See https://changkun.de/s/midgard for more details.

Usage:
  mg [command]
```

## Configuration

- `MIDGARD_CONF=/path/to/your/config.yml`, or
- [config.yml](../config.yml)

## Midgard Server

Docker:

```
$ make build
$ make up
```

Native:

```sh
$ mg server
```

## Midgard Daemon

`midgard` daemon process **runs on your local machine**
(automatic start when machine boots):

```sh
$ mg daemon install
$ mg daemon start
$ mg daemon stop
$ mg daemon uninstall
```

> Linux requires `sudo`

or

```sh
$ mg daemon run
```

## Architecture

The midgard service contains three parts:

- CLI
- Daemon
- Server

A user uses midgard CLI talks to the midgard daemon on local device,
and the daemon process talks to the midgard server for synchornization
between devices.

```
                            HTTP
Mobile <---------------------------------------------┐
                                                     |
CLI <-------> daemon <-----┐       Websocket         v     HTTP
       RPC                 ├--------------------> server <------> public
CLI <-------> daemon <-----┘
```

## License

Copyright 2020 [Changkun Ou](https://changkun.de). All rights reserved.