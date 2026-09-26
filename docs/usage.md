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

## Sign In

Each device signs in once, through auth.latere.ai:

```sh
$ mg login    # prints a link and a code; approve in any browser
$ mg logout
```

The server lets in only the people on its `AUTH_ALLOWED_PRINCIPALS`, and each
person reaches only their own clipboard.

## Web Page

Open `https://<your domain>/midgard/` and sign in. There you see your clipboard
and can send text to your devices, go through your history, share a file or
your clipboard at a link and revoke shares, and issue app tokens. It works on a
phone too, where no daemon runs.

## Copy and Paste from the Command Line

```sh
$ mg copy hello world          # to the clipboard of all your devices
$ git log -1 | mg copy         # what comes on stdin, as it is
$ mg copy < shot.png           # a PNG image
$ mg paste                     # your clipboard, the newest copy from any device
$ mg paste > shot.png          # an image goes to a file
```

A copy reaches devices that are off when they come back. `mg paste` needs
one of your devices online, unless the copy is still on its way to them.

## For Agents and Scripts

`mg` is how an agent or a script uses midgard: give it an app token (see App
Tokens) or run `mg login` once where it runs. Add `--json` to any command
that prints results, and they come as JSON on stdout, while messages go to
stderr:

```sh
$ echo "the build is green" | mg copy --json
{"seq": 58, "type": "text", "size": 19}
$ mg history --json            # also: devices, queue, shares, share, paste, history show
```

Every command ends with one of these exit codes:

| Code | Means |
|---|---|
| 0 | it worked |
| 1 | the server or the network failed, or refused |
| 2 | the command was used wrongly |
| 3 | this device is not signed in: `mg login`, or a token in `config.yml` |
| 4 | none of your devices is online to answer (your clipboard and history are on them) |
| 5 | there is no such copy, share, device or token |

## History

Each of your devices keeps your recent copies, from all your devices: the last
200, for 30 days, up to 64 MB, the same on every device. The newest is your
clipboard. The server keeps none; `mg history` asks one of your devices that
is online.

```sh
$ mg history              # newest first
number  copied               device  type  size
42      2026-09-26 13:02:11  laptop  text  14
41      2026-09-26 12:58:40  phone   text  31
$ mg history show 41      # print it
$ mg history copy 41      # make copy 41 your clipboard again, on every device
$ mg history rm 41
$ mg history clear
```

Copies a password manager marks as secret are not kept, and never leave the
device they were made on. Your history is yours alone; no one else who signs
in can see it. Deleting a copy, or clearing the history, reaches every device,
and does not change what is on anyone's clipboard.

## What Is on Its Way

A copy waits on the server, in memory, until each of your devices has it. See
what waits, and for which devices, and take back a copy that has not reached
any:

```sh
$ mg queue
number  copied               from  type  size  waiting for
57      2026-09-26 18:02:11  web   text  14    desktop
$ mg queue rm 57          # only while none of your devices is online
$ mg devices              # your devices, online or not
$ mg devices forget <id>  # one you no longer use: copies stop waiting for it
```

## App Tokens

For a client that cannot sign in, such as an iOS Shortcut or a device without a
browser, issue an app token. A token acts for one person, its owner, and reaches only their data;
revoke one and only that client loses access. Issue one on the web page, under
**App tokens**, or on the server's machine, in the directory the server runs
from:

```sh
$ mg server token add laptop --owner <owner>   # prints the token, once
$ mg server token ls --owner <owner>           # the owner's tokens
$ mg server token rm laptop --owner <owner>    # revoke it; takes effect at once
```

On the device, put the token in its `config.yml` instead of running
`mg login`:

```yaml
domain: example.com
token: mgt_...
```

Anything else that talks to the server, such as an iOS Shortcut, can send it
as `Authorization: Bearer mgt_...`. A token only works while its owner is on
the server's `AUTH_ALLOWED_PRINCIPALS`; issue it with `--email` if the list
names people by email.

## Your Devices

```sh
$ mg devices
name     online  last seen            has up to  id
laptop   yes     2026-09-26 18:02:40  57         4f1c…
desktop  no      2026-09-26 09:12:03  52         9a0e…
```

A device is one install of midgard, known by an id it keeps in its
configuration directory. `mg daemon ls` prints the same list.

## Share a Link

Share a file, or your clipboard, at a link anyone can open:

```sh
$ mg share                          # your clipboard, at a random link
https://changkun.de/midgard/s/fboVP8u4xNMHfvsv2EeLzL

$ mg share -f report.pdf            # a file
$ mg share notes/today -f a.txt     # with a name: its link is notes/today.txt
https://changkun.de/midgard/notes/today.txt

$ mg share --expires 24h            # gone after a day
```

The link lands on your clipboard, ready to paste. A name is first come, first
served; each share also keeps its random link. Links are public, but only you
can list your shares or revoke one:

```sh
$ mg shares                         # what you have shared
$ mg shares rm fboVP8u4xNMHfvsv2EeLzL   # revoke it; its links stop working
```

The daemon's hotkey shares your clipboard the same way:

- Linux: **Ctrl+Mod4+s**
- macOS: **Ctrl+Option+s**
- Windows: **Ctrl+Shift+s**

An iOS Shortcut, or anything else with an app token, can share by sending
`POST /midgard/api/v1/shares` with `{"data": "<base64>", "type": "image/png"}`
(no data shares your clipboard). The older "midgard-alloc" Shortcuts used an
API that is gone, and no longer work.

## Shared Clipboard

`midgard` daemon watches system clipboard and automatically sync with the
midgard server. Thus, a possible use case of `midgard` is:

1. Take a screenhot
2. Use `mg share` or press **Ctrl+Option+s**
3. **Ctrl+v**

This returns a public accessible URL and write back into local clipboard
so that one can immediately paste to anywhere.

Furthermore, with the built-in universal clipboard, one can even share
clipboard cross platforms (e.g. between Mac and Linux).

### iOS, iPadOS, macOS Shortcut - Clipboard

These Shortcuts were made for the old user name and password, which the server
no longer accepts. To use one, edit its **Get Contents of URL** action so that
the `Authorization` header is `Bearer mgt_...`, an [app token](#app-tokens),
instead of the encoded user name and password.

- midgard-getclipboard
  + iOS 14, iPadOS 14: https://www.icloud.com/shortcuts/66c475e013e94dbf9f3714365d6c3f95
  + iOS 15+, iPadOS 15+, macOS 12+: https://www.icloud.com/shortcuts/c88e44b318e74eedb20201e4f513dabf
- midgard-putclipboard
  + iOS 14, iPadOS 14: https://www.icloud.com/shortcuts/c1b98b1ae59045e59c1f302a634e5633
  + iOS 15+, iPadOS 15+, macOS 12+: https://www.icloud.com/shortcuts/e875c142389e4fe6b45bbed4a517f8c8


## License

Copyright 2020-2021 [Changkun Ou](https://changkun.de). All rights reserved.