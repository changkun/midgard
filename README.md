# midgard

`midgard` is a lightweight cloud solution for data persistence and
synchronization between devices.

## Usage

In order to setup `midgard`, three components must be configured.

### `midgard` Server

`midgard` server is deployed on a server, one can use the following command:

```sh
$ midgard -s # run midgard server
```

### `midgard` Daemon

`midgard` daemon process runs on your local machine, it responsible for
listening the clipboard, hotkey, and server push events.

```sh
$ midgard -d # run midgard daemon
```

### `midgard` CLI

`midgard` command line tool offers several command for generating persistent
URLs. It automatically writes to your clipboard and you can directly paste
it to anywhere else that you want.

```sh
$ midgard # generate perminant URL for the current universal clipboard data
DONE: golang.design/midgard/fs/wild/H6e3G8rcjXVWxGK9jsSS57.txt

$ midgard -p /path/you/want # generate a specified URL for clipboard data
DONE: golang.design/midgard/fs/special.go

$ midgard -p /path/you/want/v2.txt -f path/to/the/file.txt # generate a specified URL for a given file
DONE: golang.design/midgard/fs/special2.go
```

where the `golang.design` hostname is configurable from configuration file.

## TODO

- [ ] installation script for daemon process
- [ ] register keyboard hotkey
- [ ] authenticated gRPC calls
- [ ] OAuth/JWT authentication?
- [ ] Better clipboard listener, implement X11 convension
- [ ] Webcoekt clipboard push registration/notification
- [ ] UPDATE/DELETE existing resource
- [ ] Search function?
- [ ] iOS shortcut for clipboard data fetching
- [ ] VCS backup
- [ ] list folder tree
- [ ] config initialization, both for client and server (can we use init for daemon/server installation?)

## Troubleshooting

- Linux user must: `sudo apt install protobuf-compiler xclip` in order to use `protoc` and `xclip` command.

## License

GNU GPL-3.0 Copyright &copy; 2020 The [golang.design](https://golang.design) Initiative Authors.