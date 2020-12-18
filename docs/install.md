# Midgard Installation

## Dependencies

macOS:

```
xcode-select --install
```

Linux:

```
sudo apt install -y git xclip libx11-dev
```

## Midgard server

Docker:

```
make build
make up
```

Native:

```sh
$ mg server install
$ mg server start
$ mg server stop
$ mg server uninstall
```

or

```sh
$ mg server run
```

## Midgard daemon

`midgard` daemon process **runs on your local machine**:

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

## Configuration

- `MIDGARD_CONF=/path/to/your/config.yml`, or
- [config.yml](../config.yml)

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