# Design: after v0.3.0

| | |
|---|---|
| **Status** | Proposed, 2026-09-27. Nothing here is decided until its section says so |
| **Decided** | 2026-09-27, with changkun: the release with end-to-end encryption is v0.3.0, not 1.0, since Android, iOS, and more than text and images are still to come |
| **Builds on** | [redesign.md](./redesign.md), built through its §12 |

This records what changkun asked for next (§2) and what else the work so far
suggests (§3), in the order proposed (§4), with the decisions each needs (§5).
Each section goes as far as a decision, not as far as an implementation.

## 1. Where v0.3.0 leaves us

The Mac has an app, and Linux and Windows have `mg daemon`. Agents use `mg`,
and a phone uses the web page, the Shortcuts, or Tasker. Copies are sealed
end to end (redesign §11). Six limits shape everything below:

- **A copy is plain text or a PNG image**, one format per copy. The envelope
  already lists formats, each with its type and size (`wire.Envelope.Formats`),
  and a frame already carries a part per format; devices send one.
- **A copy travels inside a websocket frame**, up to 32 MB
  (`wire.MaxPayload`).
- **It waits in the server's memory**, up to 64 MB and 7 days per person
  (redesign §6), and a restart loses what waits (§7).
- **A person has one key, for good.** Forgetting a device does not take the
  key from it (§11, *Not now*).
- **Phones are clients, not devices.** The web page and the Shortcuts read
  through an online computer, and have no history of their own.
- **The iPhone's Shortcuts and Tasker** reach an encrypted clipboard only
  through the bridge, in the clear.

## 2. Asked for

### 2.1 More than text and images

A copy on a computer is rarely one thing:

- rich text copied from a page is HTML, RTF and plain text at once;
- a copy in Finder or Explorer is a list of files;
- a screenshot is PNG and TIFF.

midgard keeps plain text or a PNG only. So rich text arrives plain, and a
copied file arrives as its path, which means nothing on another device.
Sending a file to one's other devices is also what people reach for a
clipboard tool to do.

**Formats.** A copy carries each representation its source offered that
another device can put back: `text/plain`, `text/html`, `text/rtf`,
`image/png`. A device writes each one its platform takes, and passes over the
rest.

- The wire barely changes: frames carry several parts already, and the seal's
  additional data covers every format (§11), so the server cannot drop one
  unseen.
- *Each copy once* (§6) compares all of a copy's formats.
- The history shows the plain text, or the image.
- The formats together count toward the 32 MB and the hold.
- The Mac app reads `NSPasteboard` itself (`LocalClipboard.swift`), so the Mac
  can do this on its own.
- `mg daemon` reads through golang.design/x/clipboard, which knows text and
  images only. Formats there wait for that library to learn HTML, RTF and file
  lists on each platform, a change upstream.

*Size: small on the Mac; upstream work for Linux and Windows.*

**Files.** A file, copied or sent ("Send to My Devices" from the menu bar, a
drop on it, a phone's share sheet), arrives on the other devices as a file,
not as bytes in a frame.

- **Too big for a frame or for memory**, its bytes go up sealed, in chunks, to
  a store on the server's disk. That is possible only once sealed bytes may be
  written there (§3.1).
- Each chunk is sealed with the person's key. Its additional data binds the
  file and the chunk's place, so the server cannot swap or reorder chunks.
- The copy event carries the file's manifest, sealed like any copy: its name,
  type, size and chunks.
- **Fetched when wanted.** On a Mac, a file copy puts a promise on the
  clipboard (`NSFilePromiseProvider`), fetched only when pasted. Or, by a
  setting, it saves to `~/Downloads/Midgard`. Either way, a 2 GB video never
  reaches a device that does not open it.
- The server keeps the chunks for a time (a day, say) and within a quota per
  person. The history keeps the entry, not the bytes.
- Later, between devices on one network, the bytes can go directly, without
  the server (§11, *Not now*).

*Open:* the largest file, how long the server keeps one, the quota, and
whether a folder goes as a zip. *Size: medium.*

### 2.2 An iPhone and iPad app

Today the iPhone has the web page on its Home Screen, encrypted, and the
Shortcuts through the bridge, in the clear.

**What iOS allows.** An app cannot watch the clipboard. From the
foreground, reading what another app copied asks the person first ("Allow
Paste"), unless they allow it for the app in Settings or tap a paste button
(`UIPasteControl`). The app does not run in the background, so it holds no
websocket; it catches up when opened.

**What the app does:**

- **History**: it shows the person's history and puts a copy back on the
  clipboard.
- **Send**: a share extension (text, images, files) and a paste button, with
  no prompt.
- **Shortcuts, encrypted**: App Intents, *Get from Midgard* and *Send to
  Midgard*, as Shortcuts actions that run with the key. The Action button and
  Back Tap can run them. This does what the bridge does, without the clear.
- **Code shared with the Mac**: SwiftUI views and the model are shared with
  the Mac app, and the Go engine is built for iOS as the Mac's is (a
  c-archive, in an xcframework).
- **The key** lives in the Keychain, shared with the extensions through an app
  group.

**A device or a client?** As a device, it keeps a history and the server
holds copies for it (7 days, 64 MB), so it works while every computer sleeps.
That needs held copies to outlive a restart (§3.1).

**New copies while closed** would need notifications, and those have a catch.
Pushing to an App Store app takes the publisher's APNs key. A self-hosted
server would push through a service changkun runs, or not at all. Whether a
notification's action can put a copy on the clipboard without opening the
app is to confirm in a spike.

**Distribution**: TestFlight, then the App Store, under the team the Mac is
notarized with. Review needs a way to sign in: a demo account on a server.

*Open:* device or client; notifications, and through what; whether App
Intents retire the bridge. *Size: large.*

### 2.3 An Android app (#11)

Today Android has the web page, and Tasker with an app token, through the
bridge.

**What Android allows.** Since Android 10, only the app in focus, or the
keyboard, may read the clipboard, so no app watches it in the background.

**What the app does:**

- **Send**: a share-sheet target, in-app paste, and a Quick Settings tile or
  notification action that comes to the front for a moment to read the
  clipboard and send it.
- **Receive**: a foreground service holding the websocket, which puts what
  arrives on the clipboard. To confirm in a spike: writing the clipboard from
  the background, and the time limits recent Android versions put on
  data-sync services. The fallback is pushes. FCM needs Google's services and
  the publisher's key, as APNs does; UnifiedPush is what a self-hosted server
  can use.

**The engine, for changkun to choose:**

- (a) *Recommended*: the Go engine as a shared library
  (`-buildmode=c-shared` for android/arm64), behind a thin JNI layer that
  mirrors `apple/engine`'s C API. One engine on every platform, and no
  gomobile.
- (b) Kotlin: wire, seal and history rewritten natively. It is a second
  implementation of the protocol to keep in step, though `internal/e2e`'s test
  vector helps.

**UI**: Kotlin and Jetpack Compose.

**Distribution**: the APK on the releases, and F-Droid (no Google services),
Play (a one-time fee), or both.

*Open:* the engine; F-Droid, Play, or both. *Size: large.*

## 3. Proposed

### 3.1 Held copies survive a restart

Redesign §7 kept copies in memory only, so nothing of a copy reached the
server's disk. A restart or a deploy loses what waits, a phone's paste while
every computer sleeps included. That was right while the server could read
copies. Since §11 it holds bytes it cannot read, and a sealed copy on disk
says no more than its envelope, which the server sees anyway.

**What**: the server writes held copies to its SQLite file, sealed ones only,
and deletes each once every device has it, within the same bounds (64 MB, 7
days). A person without a key keeps memory only. The pairing mailbox and the
bridge's copy stay in memory: one holds a key under a code, the other is in
the clear.

**Why first**: files (§2.1) need a store on disk, and phone apps need what
they send to outlive a deploy. *Needs:* changkun's call, as it reverses §7
for sealed copies. *Size: small.*

### 3.2 A new key when a device is lost

Forgetting a device leaves the key on it: a stolen laptop's `key.json` opens
every copy that passes after. Phones are lost more often than laptops, so
this comes before the phone apps (§4).

**What**: from a paired device, *Forget and Make a New Key*.

1. The server forgets the device and records the new `kid`. It also needs to
   refuse the lost device's sign-in, which lives at auth.latere.ai: revoking
   it there is part of the work.
2. The new key cannot reach the other devices sealed under the old one, which
   the thief has too. Two ways:
   - (a) re-pair each remaining device with a code. Simple, no new
     cryptography, tedious with many devices;
   - (b) each device gets a key pair of its own when paired, and a new key
     goes to each remaining device sealed to its public key, which the thief
     does not have.

   (a) first; (b) once there are phones.
3. Histories on the devices are in the clear already, so nothing is
   re-encrypted. The copies the server holds under the old `kid` go, as a
   device's `since` voids them. Browsers pair again.

*Size: medium for (a), large for (b).*

### 3.3 Windows and Linux in the tray (#13)

`mg daemon` has no face: no menu, no history window, no sign-in but the
terminal's. The Mac got a native app (redesign §12, step 9).

**What**: one Go app with a web view, Wails v3, for both. The engine is Go
already, and the web page's UI is there to reuse.

- **Linux**: the tray is StatusNotifierItem, and the clipboard reaches Wayland
  through golang.design/x/clipboard's ext-data-control backend.
- **Windows**: SmartScreen warns about an unsigned app, much as Gatekeeper
  did, and signing takes a certificate that costs money.

*Open:* Wails, or native per platform; Windows signing. *Size: large.*

### 3.4 The Mac: updates, and notarization in CI

A new version is a manual download. Notarizing needs changkun's Mac, so each
release's disk image is built there and put in place of the one CI built.

- **Updates**: Sparkle 2, with an EdDSA-signed appcast and *Check for
  Updates* in the menu, from the GitHub releases.
- **CI**: the Developer ID certificate and an App Store Connect API key, as
  GitHub secrets, let `release.yml`'s `mac_app` job sign and notarize. The
  API key is preferred over the app-specific password. changkun adds the
  secrets himself.

*Size: small each.*

### 3.5 Shares that stay encrypted

A share is published in the clear (§9, §11), so the server can read every
share, while many are meant for one person, not the world.

**What**: an encrypted link, as an option.

- The client seals the share under a fresh key, and puts the key in the
  link's fragment: `/midgard/s/<id>#k=…`.
- The share page opens it in the browser, and the server stores only sealed
  bytes.
- The page that opens it is the server's, with the same caveat as the web
  page (§11).
- Chat apps cannot preview such a link. Plain links stay the default.

*Size: small.*

### 3.6 Rules on copies (#30)

#30 asks for rules applied on the fly, such as a regular expression that
rewrites links copied on one device.

**What**: rules run on the device that makes the copy, since the server
cannot read copies. A rule matches the text and the device it came from, and
rewrites it. One comes built in: stripping tracking parameters (`utm_*`,
`fbclid`, …) from links. Rules are set in the Mac app's Settings, and in a
daemon's `config.yml`.

*Open:* whether the history keeps the original too. *Size: small.*

## 4. Order

Proposed:

1. Held copies survive a restart (§3.1). It is small, and files and phones
   wait on it.
2. Notarization in CI (§3.4), which ends a manual step in each release.
3. Formats on the Mac (§2.1).
4. A new key by re-pairing (§3.2a), before any phone is a device.
5. Files (§2.1).
6. The iPhone app (§2.2), with Sparkle for the Mac alongside (§3.4).
7. The Android app (§2.3).
8. Windows and Linux in the tray (§3.3).

Encrypted shares (§3.5) and rules (§3.6) fit anywhere.

1.0 is changkun's call. This proposes it once phones are devices and files
pass (steps 1–7).

## 5. Decisions for changkun

1. Held copies on the server's disk, sealed (§3.1): reverses redesign §7.
2. The phone apps as devices, with a history, or as clients (§2.2, §2.3).
3. The Android engine: Go as a shared library behind JNI, or Kotlin (§2.3).
4. Notifications on phones, and through whose push service (§2.2, §2.3).
5. Whether App Intents retire the Shortcuts bridge on the iPhone (§2.2).
6. The largest file, how long the server keeps one, and the quota (§2.1).
7. Wails or native for Windows and Linux, and signing on Windows (§3.3).
8. What 1.0 means (§4).
