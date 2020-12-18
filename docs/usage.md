# Midgard Usage

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

### iOS Shortcut Support

TODO:

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

### iOS Shortcuts Support

See
[midgard-getclipboard](https://www.icloud.com/shortcuts/501fe001ebcc444aad1517fdccdbd740)
and
[midgard-putclipboard](https://www.icloud.com/shortcuts/587ae52bb5b447e699eb8876107b2e31).

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

### iOS Shortcuts Support

See [midgard-code2img](https://www.icloud.com/shortcuts/f5ed10ceb8fa40f393dfc4ebadb0dd89).

## Reverse Proxy

If midgard is deployed behind an nginx server, then the following
configuration could help:

```
location /midgard {
    proxy_pass          http://0.0.0.0:9124;
    proxy_set_header    Host             $host;
    proxy_set_header    X-Real-IP        $remote_addr;
    proxy_set_header    X-Forwarded-For  $proxy_add_x_forwarded_for;
    proxy_set_header    X-Client-Verify  SUCCESS;
    proxy_set_header    X-Client-DN      $ssl_client_s_dn;
    proxy_set_header    X-SSL-Subject    $ssl_client_s_dn;
    proxy_set_header    X-SSL-Issuer     $ssl_client_i_dn;

    # websocket support
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";

    proxy_read_timeout 1800;
    proxy_connect_timeout 1800;
}
```

## License

Copyright 2020 [Changkun Ou](https://changkun.de). All rights reserved.