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
                            HTTP
Mobile <---------------------------------------------┐
                                                     |
CLI <-------> daemon <-----┐       Websocket         v         HTTP 
       RPC                 ├--------------------> server <------------> public
CLI <-------> daemon <-----┘                         ^
 |                                                   |
 └- - - - - - - - - - - - - - - - - - - - - - - - - -┘
                     via RPC/Websocket
```

## License

Copyright 2020 [Changkun Ou](https://changkun.de). All rights reserved.