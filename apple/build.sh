#!/bin/sh
# Builds the midgard Mac app (specs/redesign.md §3), into apple/build:
# the Go sync engine as libmidgard.a, then the Swift app linking it, then
# Midgard.app. It is signed for this Mac only (ad hoc); a Developer ID and
# notarization are for distributing it.
set -e
cd "$(dirname "$0")/.."
MIN=13.0 # what Package.swift and Info.plist say
VERSION=$(git describe --tags --always 2>/dev/null | sed 's/^v//')
ARCH=$(uname -m)

echo "building the engine"
MACOSX_DEPLOYMENT_TARGET=$MIN CGO_ENABLED=1 GOARCH=$( [ "$ARCH" = x86_64 ] && echo amd64 || echo arm64 ) \
	CGO_CFLAGS="-mmacos-version-min=$MIN" CGO_LDFLAGS="-mmacos-version-min=$MIN" \
	go build -trimpath -buildmode=c-archive -o apple/build/libmidgard.a ./apple/engine

echo "building the app"
(cd apple/Midgard && swift build -c release --arch "$ARCH")
bin=$(cd apple/Midgard && swift build -c release --arch "$ARCH" --show-bin-path)

app=apple/build/Midgard.app
rm -rf "$app" apple/build/midgard.app # the name it had, which a Mac's case-blind disk keeps
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
cp "$bin/Midgard" "$app/Contents/MacOS/Midgard"
# the icon, and the menu bar's, drawn in apple/Midgard/Icon/*.svg
cp apple/Midgard/Icon/AppIcon.icns apple/Midgard/Icon/MenuBarIcon.png apple/Midgard/Icon/MenuBarIcon@2x.png "$app/Contents/Resources/"
sed "s/VERSION/${VERSION:-0}/" apple/Midgard/Info.plist > "$app/Contents/Info.plist"
codesign --force --sign - "$app"
echo "$app"
