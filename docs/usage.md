# Using midgard

English | [中文](./usage.cn.md)

Copy on one of your devices, and paste on any other. Every copy goes into
your history, the same on every device, and any copy or file can become a
link to share. How to install each piece is in [Installing midgard](./install.md).

## On a Mac: the Midgard App

Midgard sits in the menu bar. Copy as you always do, and it reaches your
other devices; what you copy on them lands on the Mac's clipboard. Its menu
has:

- **your recent copies**: pick one to put it back on the clipboard, here and
  on every device;
- **Open History…**: every copy, from all your devices, to search, look at,
  copy back and delete;
- **Share Clipboard at a Link**, also on **Ctrl+Option+S**: the link lands on
  your clipboard;
- **Pause Syncing**: Midgard stops reading and writing the Mac's clipboard
  until you resume;
- **Start at Login**, and **Settings…** for the server and your sign-in.

## On Linux and Windows: `mg daemon`

The daemon runs in your desktop session and does the same without a window:
your copies reach your other devices, theirs reach your clipboard, and
**Ctrl+Mod4+S** on Linux (**Ctrl+Shift+S** on Windows) shares the clipboard
at a link. `mg history` and the web page show your history.

## In a Browser

Open `https://your.domain/midgard/` and sign in, on any computer or phone.
You see your clipboard and send text to your devices; go through your
history; see what is on its way to devices that are off, and your devices;
share a file or your clipboard at a link; and issue app tokens.

## On an iPhone: Shortcuts

Two Shortcuts sync the iPhone's clipboard, as an iPhone lets no app watch
it:

- **Get from Midgard** puts your newest copy on the iPhone's clipboard, text
  or an image.
- **Send to Midgard** sends what you share to it, from any app's share
  sheet, or else the clipboard, to your devices.

To add them, open the web page on the iPhone, and follow **Your iPhone**:
issue a token for it, add the two Shortcuts, and paste the token when
Shortcuts asks. Put them on the Home Screen, or on Back Tap under
Accessibility, to run them in a tap.

## Your History

Each of your devices keeps your recent copies, from all of them: the last
200, for 30 days, up to 64 MB, the same list in the same order on each. The
newest is your clipboard. The server keeps none of it; the web page and `mg`
ask one of your devices that is online.

```sh
$ mg history              # newest first, with the start of each text
$ mg history show 41      # print copy 41
$ mg history copy 41      # make copy 41 your clipboard again, on every device
$ mg history rm 41        # remove it, from every device
$ mg history clear        # remove all of it
```

A copy is in the history once: copying it again, or putting it back from
the history, moves it to the top. Copies a password manager marks as secret
are neither synced nor kept. Removing a copy from the history does not change what is on anyone's
clipboard.

## Sharing a Link

A share is a copy or a file published at a link anyone can open, until it
expires or you revoke it. It is the one thing the server keeps, as a link
must work while your devices are off.

```sh
$ mg share                          # your clipboard, at a random link
https://changkun.de/midgard/s/fboVP8u4xNMHfvsv2EeLzL

$ mg share -f report.pdf            # a file
$ mg share notes/today -f a.txt     # with a name: its link is notes/today.txt
$ mg share --expires 24h            # gone after a day

$ mg shares                         # what you have shared
$ mg shares rm fboVP8u4xNMHfvsv2EeLzL   # revoke it; its links stop working
```

The link lands on your clipboard. A name is first come, first served; each
share keeps its random link too. The web page shares files, and your
clipboard, the same way.

## Your Devices, and What Is on Its Way

A copy waits on the server, in memory, until each of your devices has it:
a laptop you closed gets what you copied meanwhile when you open it again.

```sh
$ mg devices              # your devices, online or not
name     online  last seen            has up to  id
laptop   yes     2026-09-26 18:02:40  57         4f1c…
desktop  no      2026-09-26 09:12:03  52         9a0e…
$ mg devices forget <id>  # one you no longer use: copies stop waiting for it

$ mg queue                # what waits, and for which devices
$ mg queue rm 57          # take back a copy no device has yet
```

A device you have not used for 30 days stops counting by itself.

## App Tokens

A client that cannot sign in with a browser, such as a Shortcut, a machine
without a display, or a script, uses an app token instead. A token acts for
you alone, and revoking it signs out only what uses it. Issue one on the web
page, under **App tokens**, or on the server:

```sh
$ docker compose run --rm midgard token add phone --owner <principal id>
$ docker compose run --rm midgard token ls --owner <principal id>
$ docker compose run --rm midgard token rm phone --owner <principal id>
```

Put it in a device's `config.yml` as `token: mgt_...` instead of running
`mg login`, or send it as `Authorization: Bearer mgt_...`. It works only
while you are on the server's allowlist.

## `mg` for Scripts and Agents

`mg` is also how a script or an agent uses midgard:

```sh
$ mg copy hello world          # to the clipboard of all your devices
$ git log -1 | mg copy         # what comes on stdin, as it is
$ mg copy < shot.png           # a PNG image
$ mg paste                     # your clipboard, the newest copy
$ mg paste > shot.png          # an image goes to a file
```

Add `--json` to any command that prints results, and they come as JSON on
stdout, while messages go to stderr:

```sh
$ echo "the build is green" | mg copy --json
{"seq": 58, "type": "text", "size": 19}
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

## Is It Working?

```sh
$ mg status
server status: OK
daemon status: OK
```

`mg login` signs a device in, `mg logout` out. On a Mac, the app's menu says
whether it syncs.

## License

Copyright 2020-2026 [Changkun Ou](https://changkun.de). All rights reserved.
