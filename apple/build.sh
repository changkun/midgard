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
# With $MIDGARD_NOTARY too, a notarytool keychain profile (as xcrun notarytool
# store-credentials keeps one), Apple notarizes the app and the disk image,
# and their tickets are stapled to them: a Mac then opens them without asking,
# offline too.
set -e
cd "$(dirname "$0")/.."
MIN=14.0 # what Package.swift and Info.plist say
# $MIDGARD_VERSION and $MIDGARD_BUNDLE_ID stand in for the version and the
# bundle's id in a test of updating one build to another, so that it
# touches nothing of the app's own
VERSION=${MIDGARD_VERSION:-$(git describe --tags --always 2>/dev/null | sed 's/^v//')}
BUNDLE_ID=${MIDGARD_BUNDLE_ID:-de.changkun.midgard}
if [ "$1" = universal ]; then ARCHS="arm64 x86_64"; else ARCHS=$(uname -m); fi
if [ -n "$MIDGARD_NOTARY" ] && [ -z "$MIDGARD_SIGN" ]; then
	echo "MIDGARD_NOTARY needs MIDGARD_SIGN: Apple notarizes only what a Developer ID signed" >&2
	exit 2
fi

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

# notarize has Apple check $1, a zip or a disk image, and waits for the
# verdict; on a refusal it shows Apple's log, which names what to fix.
notarize() {
	out=apple/build/notary.json
	xcrun notarytool submit "$1" --keychain-profile "$MIDGARD_NOTARY" --wait --output-format json >"$out" || true
	status=$(plutil -extract status raw -o - "$out" 2>/dev/null) || status=""
	[ "$status" = Accepted ] && return
	echo "Apple did not notarize $1: ${status:-see above}" >&2
	if id=$(plutil -extract id raw -o - "$out" 2>/dev/null); then
		xcrun notarytool log "$id" --keychain-profile "$MIDGARD_NOTARY" >&2
	fi
	return 1
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
# Sparkle, which updates the app, where the binary's rpath looks for it
mkdir -p "$app/Contents/Frameworks"
ditto "$bin/Sparkle.framework" "$app/Contents/Frameworks/Sparkle.framework"
# the icon, and the menu bar's, drawn in apple/Midgard/Icon/*.svg
cp apple/Midgard/Icon/AppIcon.icns apple/Midgard/Icon/MenuBarIcon.png apple/Midgard/Icon/MenuBarIcon@2x.png "$app/Contents/Resources/"
sed -e "s/VERSION/${VERSION:-0}/" -e "s/de\.changkun\.midgard/$BUNDLE_ID/" apple/Midgard/Info.plist > "$app/Contents/Info.plist"
# signed signs $1 as the app is: with a Developer ID, the hardened runtime
# and a timestamp; or ad hoc, without the runtime, whose library validation
# would refuse an ad hoc Sparkle.framework.
signed() {
	p=$1
	shift
	if [ -n "$MIDGARD_SIGN" ]; then
		codesign --force --options runtime --timestamp --sign "$MIDGARD_SIGN" "$@" "$p"
	else
		codesign --force --sign - "$@" "$p"
	fi
}
# inside out, as Sparkle's documentation has it: what the framework holds,
# the framework, then the app, never with --deep
fw="$app/Contents/Frameworks/Sparkle.framework/Versions/B"
signed "$fw/XPCServices/Installer.xpc"
signed "$fw/XPCServices/Downloader.xpc" --preserve-metadata=entitlements
signed "$fw/Autoupdate"
signed "$fw/Updater.app"
signed "$app/Contents/Frameworks/Sparkle.framework"
signed "$app"
if [ -n "$MIDGARD_SIGN" ]; then
	if [ -n "$MIDGARD_NOTARY" ]; then
		# the app on its own first, so that it carries its ticket out of the
		# disk image, into Applications
		echo "notarizing the app"
		ditto -c -k --keepParent "$app" apple/build/Midgard.zip
		notarize apple/build/Midgard.zip
		rm apple/build/Midgard.zip
		xcrun stapler staple -q "$app"
		spctl --assess --type execute "$app"
	fi
fi

echo "packing the disk image"
dmg=apple/build/Midgard.dmg
rm -f "$dmg"
if dmgbuild=$(dmgbuild); then
	# the window's picture, with the version under its title, for both kinds
	# of screen, in the TIFF Finder shows
	swift apple/Midgard/DMG/stamp.swift "Version ${VERSION:-0}" \
		apple/Midgard/DMG/background.png apple/build/background.png \
		apple/Midgard/DMG/background@2x.png apple/build/background@2x.png
	tiffutil -cathidpicheck apple/build/background.png apple/build/background@2x.png \
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
if [ -n "$MIDGARD_SIGN" ]; then codesign --force --timestamp --sign "$MIDGARD_SIGN" "$dmg"; fi
if [ -n "$MIDGARD_NOTARY" ]; then
	echo "notarizing the disk image"
	notarize "$dmg"
	xcrun stapler staple -q "$dmg"
fi
echo "$app ($(lipo -archs "$app/Contents/MacOS/Midgard"))"
echo "$dmg"
