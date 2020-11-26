# Architecture

The midgard service contains three parts:

- CLI
- Daemon
- Server

A user uses midgard CLI talks to the midgard daemon on their local device,
and the daemon process talks to the midgard server for resource sharing
and global resource namespace allocation.

<!-- https://en.wikipedia.org/wiki/Box-drawing_character -->
```
cli <-------> daemon <-----┐  via HTTP/Websocket             via HTTP 
       RPC                 ├-------- /~/ --------> server <------------> public
cli <-------> daemon <-----┘                         ^
 |                                               |
 └- - - - - - - - - - - - - - - - - - - - - - - -┘
                    via RPC/Websocket
```

## License

GNU GPL-3.0 Copyright &copy; 2020 The [golang.design](https://golang.design) Initiative Authors.
