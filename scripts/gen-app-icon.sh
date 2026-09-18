#!/usr/bin/env bash
# Build AppIcon.icns from internal/serve/buddy.png into an output path.
# Args: <out.icns>
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
out="${1:?output .icns path}"
logo="${BUDDY_LOGO:-$root/internal/serve/buddy.png}"
test -f "$logo" || { echo "gen-app-icon: missing $logo" >&2; exit 1; }
command -v iconutil >/dev/null || { echo "gen-app-icon: iconutil not found" >&2; exit 1; }

work="$(mktemp -d /tmp/buddy-icon.XXXXXX)"
cleanup() { rm -rf "$work"; }
trap cleanup EXIT

mkdir -p "$work/buddy.iconset"
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
mkdir -p "$(dirname "$out")"
iconutil -c icns "$work/buddy.iconset" -o "$out"
