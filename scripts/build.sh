#!/bin/bash
# Builds Menubloom.app: the mygo app with the Finder Sync extension in
# Contents/PlugIns, which mygo build has no place for, signed inside out.
#
#   scripts/build.sh              build into build/darwin-universal
#   scripts/build.sh --install    and install into /Applications, enable the extension, launch
#   scripts/build.sh --release    and make a notarized disk image (needs MENUBLOOM_NOTARY,
#                                 a profile of `xcrun notarytool store-credentials`)
#                                 and the signed update (scripts/sign-update)
#
# SIGN picks the identity; the default is the first Developer ID Application
# in the keychain, else ad hoc ("-"), which runs on this Mac only.
set -euo pipefail
cd "$(dirname "$0")/.."

mode="${1:-}"
version=$(plutil -extract version raw mygo.json)
ext_id=io.github.lzhgus.menubloom.finder
sign="${SIGN:-$(security find-identity -v -p codesigning | sed -n 's/.*"\(Developer ID Application: [^"]*\)".*/\1/p' | head -1)}"
sign="${sign:--}"
echo "==> Menubloom $version, signed by: $sign"

# No update signing key here: mygo's update would lack the extension, so
# scripts/sign-update makes it from the finished app.
env -u MYGO_UPDATER_PRIVATE_KEY go run github.com/egoist/mygo/cmd/mygo build -platform darwin/universal -skip-dmg -skip-notarize -sign "$sign" .
app="build/darwin-universal/Menubloom.app"

echo "==> Finder extension"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
for arch in arm64 x86_64; do
  swiftc -O -target "$arch-apple-macos12.0" -application-extension -parse-as-library \
    -module-name MenubloomFinder -framework FinderSync \
    -Xlinker -e -Xlinker _NSExtensionMain \
    extension/FinderSync.swift -o "$work/MenubloomFinder-$arch"
done
appex="$app/Contents/PlugIns/MenubloomFinder.appex"
rm -rf "$appex"
mkdir -p "$appex/Contents/MacOS"
lipo -create "$work"/MenubloomFinder-* -output "$appex/Contents/MacOS/MenubloomFinder"
sed "s/VERSION/$version/g" extension/Info.plist > "$appex/Contents/Info.plist"

echo "==> Signing"
opts=(--force --sign "$sign")
[ "$sign" != "-" ] && opts+=(--options runtime --timestamp)
codesign "${opts[@]}" --entitlements extension/entitlements.plist "$appex"
codesign "${opts[@]}" --preserve-metadata=entitlements,requirements,flags "$app"
codesign --verify --deep --strict "$app"
echo "    $app"

case "$mode" in
--install)
  echo "==> Installing into /Applications"
  osascript -e 'quit app "Menubloom"' 2>/dev/null || true
  rm -rf "/Applications/Menubloom.app"
  ditto "$app" "/Applications/Menubloom.app"
  /System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister -f "/Applications/Menubloom.app"
  /System/Library/CoreServices/pbs -update   # the services of NSServices
  pluginkit -a "/Applications/Menubloom.app/Contents/PlugIns/MenubloomFinder.appex"
  pluginkit -e use -i "$ext_id"
  open "/Applications/Menubloom.app"
  pluginkit -m -i "$ext_id"
  ;;
--release)
  [ "$sign" = "-" ] && { echo "a release needs a Developer ID" >&2; exit 1; }
  : "${MENUBLOOM_NOTARY:?set MENUBLOOM_NOTARY to a notarytool keychain profile}"
  dmg="build/menubloom-$version.dmg"
  echo "==> $dmg"
  stage="$work/dmg"
  mkdir -p "$stage"
  ditto "$app" "$stage/Menubloom.app"
  ln -s /Applications "$stage/Applications"
  rm -f "$dmg"
  hdiutil create -volname "Menubloom" -srcfolder "$stage" -format UDZO "$dmg" >/dev/null
  codesign --sign "$sign" --timestamp "$dmg"
  xcrun notarytool submit "$dmg" --keychain-profile "$MENUBLOOM_NOTARY" --wait
  xcrun stapler staple "$dmg"
  xcrun stapler staple "$app"
  echo "==> Update"
  go run ./scripts/sign-update "$app" build
  ;;
esac
