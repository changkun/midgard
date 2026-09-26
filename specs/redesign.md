# Design: midgard, redesigned

| | |
|---|---|
| **Status** | Proposed |
| **Decided** | 2026-09-26, with changkun: clipboards belong to individuals, behind a hard barrier; history is shared across one's own devices; login through auth.latere.ai; code2img and the GitHub backup go |
| **Builds on** | #35–#53 (Phase 0 and 1: safe, and easy to run) |

## 1. What midgard is for

Copy on one of your devices, paste on another, and find what you copied
yesterday from any of them. Turn a copy into a link to share. It runs on your
own server, at `changkun.de/midgard`, for the people you let sign in, and no
one sees anyone else's clipboard.

## 2. What stays and what goes

| | Today | After |
|---|---|---|
| Sign-in | one user name and password in `config.yml`, on every device | auth.latere.ai, plus an allowlist |
| Whose clipboard | one, shared by everyone who can log in | one per person, isolated |
| History | every text ever copied, plaintext YAML on disk, never read back | per person, bounded, shown on every device; sensitive copies never stored |
| Local RPC | `mg` → gRPC → daemon → HTTP → server | `mg` → HTTP → server |
| Backup | the store is a git clone, pushed to GitHub hourly | a data volume with one SQLite file; host backups cover it |
| code2img | server-side Chrome | removed |
| Share links | files under `data/repo`, forever | rows with an owner, optional expiry, revocable |
| Phones | iOS Shortcuts / Tasker with the password | a web page, or Shortcuts with a revocable app token |

## 3. Architecture

```
  device (edge)                              server (coordination)
  ┌──────────────────────┐                   ┌────────────────────────────┐
  │ mg daemon            │  wss, per person  │ sync: one room per person  │
  │  watch / apply clip  │ ◀───────────────▶ │ history: last N per person │
  │  hotkey → share      │                   │ shares: /midgard/s/<id>    │
  ├──────────────────────┤  https           │ web page (browser login)   │
  │ mg share/history/... │ ─────────────────▶│ SQLite + blobs, one volume │
  └──────────────────────┘                   └─────────────▲──────────────┘
          │ device grant, actor tokens                    │ JWKS
          ▼                                               │
     auth.latere.ai ──────────────────────────────────────┘
```

Two programs, one binary:

- **The server** is the only thing that knows more than one device. It
  authenticates every request, relays a person's copies between that person's
  devices, keeps their history and shares, and serves a small web page.
- **The daemon** is the edge: it watches the local clipboard, applies what
  arrives, drops copies marked sensitive (#53), and turns the hotkey into a
  share. It runs as its user (#49, #50) and reconnects on its own (#43).
- **`mg` commands** call the server directly. The gRPC service between `mg` and
  the daemon goes: every call it carried ended as an HTTP request to the server
  or a round trip over the daemon's websocket, so it only relayed. `mg` writes
  the local clipboard itself where it needs to. This also retires the socket
  from #51.

## 4. Identity

**People** sign in with auth.latere.ai. A signature proves *who* is calling,
not that they may use this server, so `AUTH_ALLOWED_PRINCIPALS` (emails or
principal ids, as `main` and `redir` use) decides who may. The principal id
(`sub`) is the owner key for everything they store.

**Devices** — the daemon and `mg`, which are one program — sign in once with
`mg login`: the device grant (RFC 8628, `latere.ai/x/pkg/authkit/cli`) prints
a code, the person approves it in a browser, and the login token is kept in
`<UserConfigDir>/midgard/token.json`. For each server call the device mints a
short-lived actor token for audience `midgard` (`oidc.MintActorToken`) and
reuses it until it expires. The server verifies signature, issuer, expiry and
audience from the JWKS (`authkit.JWT`, `Audiences: [midgard]`), then the
allowlist.

**The web page** uses browser code + PKCE with a session cookie
(`latere.ai/x/pkg/oidc`), as `redir` does.

**Shortcuts and Tasker** cannot run a device grant, so the web page issues
*app tokens*: named, shown once, revocable, owned by the person who issued them
(the token store from #48, keyed by owner). They reach only their owner's data.

The password in `server.auth` goes, and with it basic auth.

**Registration** (in `latere-ai/auth`, `deploy/base/clients.yaml`):

- `midgard-cli`: public; grants `device_code`, `refresh_token`; scopes `openid
  email profile offline_access`; `actor_audiences: [midgard]`. Its own client,
  not a shared one, so its token file and refreshes are its own.
- `midgard-web`: public; code + PKCE; `redirect_uris:
  [https://changkun.de/midgard/.auth/callback, http://localhost:8080/midgard/.auth/callback]`.

## 5. The hard barrier

Every stored row has an `owner` (the `sub`), and every query takes it from the
authenticated request, never from the request body. A device's websocket
joins its owner's room only, and a broadcast never leaves the room. Share links
are public by design, but listing, revoking and creating them are owner-only.

The tests for this are part of each change that touches data: two principals,
each shown to see nothing of the other's clipboard, history, devices, shares or
tokens, through every endpoint.

## 6. History across devices

The server keeps each person's recent copies — text, images and file lists,
with every format a copy carried — newest first. The newest is the current
clipboard. Defaults: the last 200 copies or 30 days, whichever is fewer;
images count against a per-person byte budget (64 MB). A person can delete an
entry or all of it.

- `mg history` lists it; `mg history copy <n>` puts an entry back on the local
  clipboard. The web page shows it with a copy button per entry.
- Copies marked sensitive never reach the server (#53), so history holds none.
- At rest the database is plaintext, under the server's user. This is a
  personal server whose operator is its user; end-to-end encryption, where the
  server could not read a clip at all, is §11.

## 7. Storage

One SQLite file (`modernc.org/sqlite`, pure Go, so the server stays one static
binary) and a `blobs/` directory for large payloads, both in the `data`
volume:

- `devices(id, owner, name, last_seen)`
- `clips(id, owner, device, created, size)` and `clip_formats(clip, mime, bytes | blob)`
- `shares(slug, owner, created, expires, mime, blob, legacy_path)`
- `app_tokens(owner, name, hash, created)`

No git. The host's backup of the volume is the backup, and one file copies
consistently with `sqlite3 .backup`.

## 8. Protocol

A new `/midgard/api/v2`, since no v1 client survives the change of sign-in:

- `GET/PUT /clipboard`, `GET /history`, `DELETE /history/{id}`
- `POST /shares`, `GET /shares`, `DELETE /shares/{slug}`
- `GET /devices`
- `GET /sync`: the websocket. Typed JSON messages carrying a version; payloads
  go as binary frames, not base64 inside JSON inside base64 as today.

v1 and its endpoints are removed. Existing Shortcuts are recreated with app
tokens, and daemons reinstalled with `mg login`.

## 9. Shares

`POST /shares` stores a copy or a file and returns `/midgard/s/<id>` (random,
22 characters); an optional expiry; revocable. The 74 existing shares on the
server move into the table with `legacy_path` set, so every link already out
there — `/midgard/random/…`, `/midgard/img/…`, `/midgard/code/…`, and the
custom paths — keeps working.

## 10. Deployment

As `main` and `redir` are: `/root/changkun.de/midgard` with a
`docker-compose.yml` on the `traefik_proxy` network (the route to
`http://midgard` already exists), an `.env` holding `AUTH_*` and the
allowlist, and `./data` as the volume. Without Chrome the image is a static
binary on a minimal base; it fits the 2 GB host.

Migration: import the shares; the plaintext clipboard history in
`data/logs` (44 MB, 2020–2025) is deleted, not imported — it may hold
passwords copied before sensitive copies were marked [pending changkun's
confirmation]; retire the old checkout.

## 11. Not now

- End-to-end encryption (#31): the devices would share a key, the server would
  store ciphertext, and a share would be decrypted on the device before it is
  published.
- A native mobile app, and a tray icon (#13). The web page covers phones.
- Peer-to-peer sync on a LAN without the server.

## 12. Plan

Each step is its own PR, with its tests, merged when green.

1. Remove code2img.
2. Remove the gRPC service; `mg` commands call the server over HTTP.
3. Storage: the SQLite schema and store, with the isolation tests.
4. Identity: JWT verification with the allowlist on the server; `mg login` and
   actor tokens on the device; basic auth removed. Needs §4's registration.
5. Sync v2: rooms per person, history, `mg history`.
6. Shares v2, and `mg server import` for the existing ones.
7. The web page, with browser login and app tokens.
8. Deploy on changkun.de, migrate, and retire the old checkout.
