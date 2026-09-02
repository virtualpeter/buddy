#!/usr/bin/env bash
# Build a branded macOS product archive from the signed Darwin/arm64 binary.
# Args: <signed-binary> <pkg-output> <version> <identifier> [installer-sign-identity]
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

bin="${1:?signed binary}"
out="${2:?output pkg}"
version="${3:?version}"
ident="${4:?identifier}"
sign_id="${5:-}"

test "$(uname -s)" = Darwin || { echo "build-pkg: macOS only" >&2; exit 1; }
test -f "$bin" || { echo "build-pkg: missing $bin" >&2; exit 1; }
logo="internal/serve/buddy.png"
test -f "$logo" || { echo "build-pkg: missing $logo" >&2; exit 1; }

work="$(mktemp -d /tmp/buddy-pkg.XXXXXX)"
cleanup() { rm -rf "$work"; }
trap cleanup EXIT

mkdir -p "$work/root" "$work/resources" "$work/buddy.iconset"
cp "$bin" "$work/root/buddy"
chmod 755 "$work/root/buddy"
codesign --verify --verbose=2 "$work/root/buddy"

go run ./scripts/genbrand \
	-src "$logo" \
	-background "$work/resources/background.png" \
	-background-dark "$work/resources/background-darkAqua.png"
cp "$root/packaging/welcome.html" "$work/resources/welcome.html"

# Icon set from the 512×512 logo (512@2x is an upscale; Finder still prefers a full set).
while read -r size name; do
	sips -z "$size" "$size" "$logo" --out "$work/buddy.iconset/$name" >/dev/null
done <<'EOF'
16 icon_16x16.png
32 icon_16x16@2x.png
32 icon_32x32.png
64 icon_32x32@2x.png
128 icon_128x128.png
256 icon_128x128@2x.png
256 icon_256x256.png
512 icon_256x256@2x.png
512 icon_512x512.png
1024 icon_512x512@2x.png
EOF
xattr -cr "$work/buddy.iconset" 2>/dev/null || true
iconutil -c icns "$work/buddy.iconset" -o "$work/buddy.icns"

sed \
	-e "s|@IDENTIFIER@|${ident}|g" \
	-e "s|@VERSION@|${version}|g" \
	"$root/packaging/distribution.xml.in" >"$work/distribution.xml"

pkgbuild \
	--root "$work/root" \
	--identifier "$ident" \
	--version "$version" \
	--install-location /usr/local/bin \
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
