#!/bin/sh
# Builds the Midgard Mac app (specs/redesign.md §3), into apple/build: the Go
# sync engine as libmidgard.a, then the Swift app linking it, then
# Midgard.app, and Midgard.dmg to hand it out.
#
#   apple/build.sh            for this Mac's architecture
#   apple/build.sh universal  for Apple silicon and Intel both, as a download is
#
# It signs with $MIDGARD_SIGN, a Developer ID Application identity, when set,
# with the hardened runtime a notarization needs; otherwise for this Mac only
# (ad hoc), and another Mac asks once, in System Settings, before opening it.
set -e
cd "$(dirname "$0")/.."
MIN=14.0 # what Package.swift and Info.plist say
VERSION=$(git describe --tags --always 2>/dev/null | sed 's/^v//')
if [ "$1" = universal ]; then ARCHS="arm64 x86_64"; else ARCHS=$(uname -m); fi

# engine builds libmidgard for one architecture, as clang names it.
engine() {
	goarch=$( [ "$1" = x86_64 ] && echo amd64 || echo arm64 )
	MACOSX_DEPLOYMENT_TARGET=$MIN CGO_ENABLED=1 GOARCH=$goarch \
		CGO_CFLAGS="-arch $1 -mmacos-version-min=$MIN" CGO_LDFLAGS="-arch $1 -mmacos-version-min=$MIN" \
		go build -trimpath -buildmode=c-archive -o "apple/build/libmidgard-$1.a" ./apple/engine
}

# dmgbuild lays out the disk image's window (apple/Midgard/DMG). It is set
# up once, in apple/build/dmgbuild, from a Python of 3.10 or later; 1.6.7 is
# the first whose background Finder still finds on macOS 15 and later.
dmgbuild() {
	venv=apple/build/dmgbuild
	if [ ! -x "$venv/bin/dmgbuild" ]; then
		for py in python3.14 python3.13 python3.12 python3.11 python3.10 python3; do
			command -v "$py" >/dev/null && "$py" -c 'import sys; sys.exit(sys.version_info < (3, 10))' && break
			py=""
		done
		[ -n "$py" ] || return 1
		{ "$py" -m venv "$venv" && "$venv/bin/pip" install -q "dmgbuild==1.6.7"; } >&2 || { rm -rf "$venv"; return 1; }
	fi
	echo "$venv/bin/dmgbuild"
}

echo "building the engine for $ARCHS"
mkdir -p apple/build
libs=""
for a in $ARCHS; do
	engine "$a"
	libs="$libs apple/build/libmidgard-$a.a"
done
# one library for them all; the header is the same for each
lipo -create $libs -output apple/build/libmidgard.a
mv "apple/build/libmidgard-${ARCHS%% *}.h" apple/build/libmidgard.h
rm -f apple/build/libmidgard-*.h $libs

echo "building the app"
flags=""
for a in $ARCHS; do flags="$flags --arch $a"; done
(cd apple/Midgard && swift build -c release $flags)
bin=$(cd apple/Midgard && swift build -c release $flags --show-bin-path)

app=apple/build/Midgard.app
rm -rf "$app" apple/build/midgard.app # the name it had, which a Mac's case-blind disk keeps
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
cp "$bin/Midgard" "$app/Contents/MacOS/Midgard"
# the icon, and the menu bar's, drawn in apple/Midgard/Icon/*.svg
cp apple/Midgard/Icon/AppIcon.icns apple/Midgard/Icon/MenuBarIcon.png apple/Midgard/Icon/MenuBarIcon@2x.png "$app/Contents/Resources/"
sed "s/VERSION/${VERSION:-0}/" apple/Midgard/Info.plist > "$app/Contents/Info.plist"
if [ -n "$MIDGARD_SIGN" ]; then
	codesign --force --options runtime --timestamp --sign "$MIDGARD_SIGN" "$app"
else
	codesign --force --sign - "$app"
fi

echo "packing the disk image"
dmg=apple/build/Midgard.dmg
rm -f "$dmg"
if dmgbuild=$(dmgbuild); then
	# the window's picture, for both kinds of screen, in the TIFF Finder shows
	tiffutil -cathidpicheck apple/Midgard/DMG/background.png apple/Midgard/DMG/background@2x.png \
		-out apple/build/background.tiff >/dev/null
	"$dmgbuild" -s apple/Midgard/DMG/settings.py -D app="$app" -D background=apple/build/background.tiff Midgard "$dmg" >/dev/null
else
	echo "no dmgbuild: the disk image has no background, and its icons stand where Finder puts them" >&2
	stage=$(mktemp -d)
	cp -R "$app" "$stage/"
	ln -s /Applications "$stage/Applications" # to drag it to
	hdiutil create -quiet -volname Midgard -srcfolder "$stage" -format UDZO -ov "$dmg"
	rm -rf "$stage"
fi
if [ -n "$MIDGARD_SIGN" ]; then codesign --force --sign "$MIDGARD_SIGN" "$dmg"; fi
echo "$app ($(lipo -archs "$app/Contents/MacOS/Midgard"))"
echo "$dmg"
