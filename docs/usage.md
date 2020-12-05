# Midgard Usage

Midgard CLI offers several command to interact
with the midgard server and daemon.

## Status check

Status check gives you option to check if everything is setup correctly:

```sh
$ mg status
server status: OK
daemon status: OK
```

## Allocate A Global URL

You can use `midgard alloc` to allocate a global url to persist your data,
for example:

```sh
$ mg alloc /awesome/filename -f /path/to/your/file
DONE: https://changkun.de/midgard/fs/awesome/filename
```

The first argument of `alloc` subcommand indicates the desired URI,
and `-f` flag indicates the file you want to put to your server.

You can omit the `-f` flag and leave it empty, then the `alloc` subcommand
will request the server to use your universal clipboard data.

You can even omit the argument of `alloc`, then the `midgard` server will
create a random path under `/random`. For instance:

```sh
$ mg alloc
DONE: https://changkun.de/midgard/random/fboVP8u4xNMHfvsv2EeLzL.txt
```

It automatically writes to your clipboard and you can directly paste
it to anywhere else that you want.

Moreover, to alloc a global URL, Midgard provides the following global
keyboard shortcut to trigger such an action:

- Linux: **Ctrl+Mod4+s**
- macOS: **Ctrl+Option+s**

## Universal Clipboard

`midgard` daemon watches your system clipboard and automatically sync
your clipboard with the `midgard` server. Thus, a possible usage of
`midgard` is:

1. Take a screenhot of your desktop,
2. Use `midgard new`

This will return you a public accessible URL and write back into your local
clipboard so that you can immediately paste to anywhere you want.

Furthermore, with the built-in universal clipboard, you can even share
your clipboard cross platforms (e.g. between Mac and Linux).

### iOS Shortcuts support

- `midgard-getclipboard`: https://www.icloud.com/shortcuts/501fe001ebcc444aad1517fdccdbd740
- `midgard-putclipboard`: https://www.icloud.com/shortcuts/587ae52bb5b447e699eb8876107b2e31

With these shortcuts, you can create an automation that runs
the `midgard-getclipboard` when an application is opened (or multiples),
so that the clipboard is fetch from the midgard server automatically.

## Code2image

Whenever you want to convert your clipboard to an image:

```sh
$ mg code2img
```

Or you can specify a given file:

```sh
$ mg code2img /path/to/your/file
```

### iOS Shortcuts support

- `midgard-code2img`: https://www.icloud.com/shortcuts/f5ed10ceb8fa40f393dfc4ebadb0dd89

With these shortcut, you can post your code on an iOS device.
The shortcut will read your clipboard then render it.

## News

A timeline based news page can be visisted from: `/midgard/news`. 

To create new news, one can use the following command:

```
$ mg news "documenting your life with plain text" 
(Ctrl+D to complete; Ctrl+C to cancel)
> ...
> ...
DONE.
```

## License

Copyright 2020 [Changkun Ou](https://changkun.de). All rights reserved.