#!/usr/bin/env bash
# Build a branded macOS product archive from Buddy.app.
# Installs /Applications/Buddy.app and a postinstall symlink
# /usr/local/bin/buddy → Contents/Helpers/buddy.
# Args: <buddy.app> <pkg-output> <version> <identifier> [installer-sign-identity]
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

app="${1:?Buddy.app}"
out="${2:?output pkg}"
version="${3:?version}"
ident="${4:?identifier}"
sign_id="${5:-}"

test "$(uname -s)" = Darwin || { echo "build-pkg: macOS only" >&2; exit 1; }
test -d "$app" || { echo "build-pkg: missing app bundle $app" >&2; exit 1; }
test -x "$app/Contents/Helpers/buddy" || { echo "build-pkg: missing $app/Contents/Helpers/buddy" >&2; exit 1; }
logo="internal/serve/buddy.png"
test -f "$logo" || { echo "build-pkg: missing $logo" >&2; exit 1; }

work="$(mktemp -d /tmp/buddy-pkg.XXXXXX)"
cleanup() { rm -rf "$work"; }
trap cleanup EXIT

mkdir -p "$work/root/Applications" "$work/resources" "$work/scripts"
ditto "$app" "$work/root/Applications/Buddy.app"
codesign --verify --verbose=2 "$work/root/Applications/Buddy.app"

cp "$root/packaging/pkg-scripts/postinstall" "$work/scripts/postinstall"
chmod 755 "$work/scripts/postinstall"

go run ./scripts/genbrand \
	-src "$logo" \
	-background "$work/resources/background.png" \
	-background-dark "$work/resources/background-darkAqua.png"
cp "$root/packaging/welcome.html" "$work/resources/welcome.html"

if [[ -f "$app/Contents/Resources/AppIcon.icns" ]]; then
	cp "$app/Contents/Resources/AppIcon.icns" "$work/buddy.icns"
else
	echo "build-pkg: missing AppIcon.icns in app bundle" >&2
	exit 1
fi

sed \
	-e "s|@IDENTIFIER@|${ident}|g" \
	-e "s|@VERSION@|${version}|g" \
	"$root/packaging/distribution.xml.in" >"$work/distribution.xml"

pkgbuild \
	--root "$work/root" \
	--identifier "$ident" \
	--version "$version" \
	--install-location / \
	--scripts "$work/scripts" \
	"$work/component.pkg"

unsigned="$work/unsigned.pkg"
productbuild \
	--distribution "$work/distribution.xml" \
	--resources "$work/resources" \
	--package-path "$work" \
	"$unsigned"

if [[ -n "$sign_id" ]]; then
	productsign --sign "$sign_id" "$unsigned" "$out"
else
	cp "$unsigned" "$out"
fi

# Finder icon is a resource-fork / xattr; productsign does not cover it.
swift "$root/scripts/set-file-icon.swift" "$work/buddy.icns" "$out"

ls -lh "$out"
echo "Install: sudo installer -pkg $out -target /"
echo "Layout: /Applications/Buddy.app + /usr/local/bin/buddy → Contents/Helpers/buddy"
