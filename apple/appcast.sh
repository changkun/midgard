#!/bin/sh
# Writes the appcast Sparkle updates the Mac app from (apple/Midgard/
# Updates.swift): one item, the release, with its disk image signed with the
# update key, which Sparkle's generate_keys keeps in this Mac's Keychain.
#
#   apple/appcast.sh <version> <url of Midgard.dmg> <Midgard.dmg> [<release notes url>] > appcast.xml
#
# Sign the disk image that is published, byte for byte: a release attaches it
# at <url>, and appcast.xml beside it, where the app looks for the newest,
# https://github.com/changkun/midgard/releases/latest/download/appcast.xml.
set -e
[ $# -ge 3 ] || { echo "usage: $0 <version> <dmg url> <dmg> [<notes url>]" >&2; exit 2; }
version=$1 url=$2 notes=$4
dmg=$(cd "$(dirname "$3")" && pwd)/$(basename "$3")
cd "$(dirname "$0")/.."

# Sparkle's tools, as its release has them, for the version Package.swift
# resolved
tools=apple/build/sparkle
sparkle=$(python3 -c 'import json; print([p["state"]["version"] for p in json.load(open("apple/Midgard/Package.resolved"))["pins"] if p["identity"] == "sparkle"][0])')
if [ ! -x "$tools/bin/sign_update" ] || [ "$(cat "$tools/version" 2>/dev/null)" != "$sparkle" ]; then
	rm -rf "$tools" && mkdir -p "$tools"
	curl -sfL "https://github.com/sparkle-project/Sparkle/releases/download/$sparkle/Sparkle-$sparkle.tar.xz" | tar -xJ -C "$tools"
	echo "$sparkle" > "$tools/version"
fi

# sparkle:edSignature="…" length="…"
signature=$("$tools/bin/sign_update" "$dmg")
cat <<EOF
<?xml version="1.0" encoding="utf-8"?>
<rss version="2.0" xmlns:sparkle="http://www.andymatuschak.org/xml-namespaces/sparkle">
  <channel>
    <title>Midgard</title>
    <item>
      <title>Midgard $version</title>
      <pubDate>$(LC_ALL=C date -u '+%a, %d %b %Y %H:%M:%S +0000')</pubDate>
      <sparkle:version>$version</sparkle:version>
      <sparkle:shortVersionString>$version</sparkle:shortVersionString>
      <sparkle:minimumSystemVersion>14.0</sparkle:minimumSystemVersion>${notes:+
      <sparkle:fullReleaseNotesLink>$notes</sparkle:fullReleaseNotesLink>}
      <enclosure url="$url" type="application/octet-stream" $signature/>
    </item>
  </channel>
</rss>
EOF
