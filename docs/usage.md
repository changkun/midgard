# Midgard Usage

English | [中文](./usage.cn.md)

Midgard CLI offers several command to interact
with the midgard server and daemon.

## Status Check

Check if everything is setup correctly:

```sh
$ mg status
server status: OK
daemon status: OK
```

## List Active Daemons

Check all connected daemon users:

```sh
$ mg daemon ls
id      name
1       changkun-perflock
2       changkun-air-arm
3       changkun-pro-intel
4       changkun-ubuntu
5       changkun-win
```

## Backup Data using Git

Midgard uses Git to backup all the data. All data are stored in the `./data` folder with some naming convention. Midgard server will sync with the configured Git repository,
see settings in [../config.yml](../config.yml)

Note, to sync the data, use git instead of https protocol:

```
git config --global url."git@github.com:".insteadOf "https://github.com/"
```

## Allocate Global URL

Allocate a global url to persist the data:

```sh
$ mg alloc /awesome/filename -f /path/to/the/file # alloc link for file
https://changkun.de/midgard/awesome/filename

$ mg alloc /awesome/clipboard/content             # alloc for clipboard data
https://changkun.de/midgard/awesome/clipboard/content

$ mg alloc                          # alloc a random link for clipboard data
https://changkun.de/midgard/random/fboVP8u4xNMHfvsv2EeLzL.txt
```

Keyboard hotkey:

- Linux: **Ctrl+Mod4+s**
- macOS: **Ctrl+Option+s**

Hint: The allocated link will be write back to the clipboard and ready for paste.

### iOS, iPadOS, macOS Shortcut - Alloc

- iOS 14, iPadOS 14: https://www.icloud.com/shortcuts/0964c0a651544604bd995cf1e723c573
- iOS 15+, iPadOS 15+, macOS 12+: https://www.icloud.com/shortcuts/a440412d0f12454cb4676e0ded72a9f1

## Shared Clipboard

`midgard` daemon watches system clipboard and automatically sync with the
midgard server. Thus, a possible use case of `midgard` is:

1. Take a screenhot
2. Use `mg alloc` or Use **Ctrl+Option+s**
3. **Ctrl+v**

This returns a public accessible URL and write back into local clipboard
so that one can immediately paste to anywhere.

Furthermore, with the built-in universal clipboard, one can even share
clipboard cross platforms (e.g. between Mac and Linux).

### iOS, iPadOS, macOS Shortcut - Clipboard

- midgard-getclipboard
  + iOS 14, iPadOS 14: https://www.icloud.com/shortcuts/66c475e013e94dbf9f3714365d6c3f95
  + iOS 15+, iPadOS 15+, macOS 12+: https://www.icloud.com/shortcuts/c88e44b318e74eedb20201e4f513dabf
- midgard-putclipboard
  + iOS 14, iPadOS 14: https://www.icloud.com/shortcuts/c1b98b1ae59045e59c1f302a634e5633
  + iOS 15+, iPadOS 15+, macOS 12+: https://www.icloud.com/shortcuts/e875c142389e4fe6b45bbed4a517f8c8


## Code2image

Convert copied code to an image:

```sh
$ mg code2img
https://changkun.de/midgard/code/201218-204010
https://changkun.de/midgard/code/201218-204010.png
```

Or convert specify a given file:

```sh
$ mg code2img /path/to/your/file
https://changkun.de/midgard/code/201218-204010
https://changkun.de/midgard/code/201218-204010.png
```

Or convert specify a given file with line numbers:

```sh
$ mg code2img /path/to/your/file/ -l 5:10 # line 5 to 10
https://changkun.de/midgard/code/201218-204010
https://changkun.de/midgard/code/201218-204010.png
```

Summary page at https://changkun.de/midgard/code.

### iOS, iPadOS, macOS Shortcut - code2img

- iOS 14, iPadOS 14: https://www.icloud.com/shortcuts/73f978c0179642b5bc2c31aba300b25a
- iOS 15+, iPadOS 15+, macOS 12+: https://www.icloud.com/shortcuts/cec5afc61b01476e87b888163de6e39b

## License

Copyright 2020-2021 [Changkun Ou](https://changkun.de). All rights reserved.