#!/usr/bin/env bash
# Assemble and sign Buddy.app: CLI at Contents/Helpers/buddy, GUI stub at Contents/MacOS/Buddy.
# Args: <helper-binary> <out-app> <version> <codesign-identity> [agent-app]
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

helper="${1:?signed or unsigned helper Mach-O}"
out="${2:?output .app path}"
version="${3:?version}"
identity="${4:?Developer ID Application identity}"
agent="${5:-}"

cli_ident="${BUDDY_CODESIGN_IDENTIFIER:-virtualpete.buddy.cli}"
app_ident="${BUDDY_APP_IDENTIFIER:-virtualpete.buddy}"

test "$(uname -s)" = Darwin || { echo "build-buddy-app: macOS only" >&2; exit 1; }
test -f "$helper" || { echo "build-buddy-app: missing $helper" >&2; exit 1; }
test -n "$identity" || { echo "build-buddy-app: empty codesign identity" >&2; exit 1; }

logo="internal/serve/buddy.png"
test -f "$logo" || { echo "build-buddy-app: missing $logo" >&2; exit 1; }

work="$(mktemp -d /tmp/buddy-app.XXXXXX)"
cleanup() { rm -rf "$work"; }
trap cleanup EXIT

app="$work/Buddy.app"
macos="$app/Contents/MacOS"
helpers="$app/Contents/Helpers"
res="$app/Contents/Resources"
mkdir -p "$macos" "$helpers" "$res"

cp "$helper" "$helpers/buddy"
chmod 755 "$helpers/buddy"

# The Agent ships already signed, with its own entitlements and provisioning
# profile. Nest it at the SMAppService location and leave the signature alone:
# `codesign --force` here would re-sign it without those entitlements and
# quietly cost it the keychain presence ACL. The outer signature seals nested
# bundles by hash, so copying is enough.
if [[ -n "$agent" ]]; then
	test -d "$agent" || { echo "build-buddy-app: missing agent bundle $agent" >&2; exit 1; }
	agent_info="$(codesign -dv --verbose=2 "$agent" 2>&1)"
	case "$agent_info" in *Signature=adhoc*) echo "build-buddy-app: agent is ad-hoc signed" >&2; exit 1;; esac
	test -f "$agent/Contents/embedded.provisionprofile" || {
		echo "build-buddy-app: agent has no embedded.provisionprofile (build it with make agent-app)" >&2
		exit 1
	}
	codesign --verify --strict --verbose=2 "$agent"
	mkdir -p "$app/Contents/Library/LoginItems"
	ditto "$agent" "$app/Contents/Library/LoginItems/BuddyAgent.app"
fi

echo "Building Buddy.app stub (version $version)..."
# CGO + AppKit so Finder launch finishes (Dock bounce stops) while serve runs.
CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -o "$macos/Buddy" ./cmd/buddy-app
chmod 755 "$macos/Buddy"

sed \
	-e "s|@APP_IDENTIFIER@|${app_ident}|g" \
	-e "s|@VERSION@|${version}|g" \
	"$root/packaging/Info.plist.in" >"$app/Contents/Info.plist"

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
iconutil -c icns "$work/buddy.iconset" -o "$res/AppIcon.icns"

# Hardened Runtime; no extra entitlements (Keychain Sharing is not in this cut).
# The --timestamp=none fallback exists for signing with no network. Notarization
# rejects a signature without a secure timestamp, so a notarizing build must not
# take that path silently: BUDDY_NOTARY_PROFILE turns it into a failure.
sign() {
	if codesign --force --sign "$identity" --options runtime --timestamp "$@" 2>/dev/null; then
		return 0
	fi
	if [[ -n "${BUDDY_NOTARY_PROFILE:-}" ]]; then
		echo "build-buddy-app: could not reach Apple's timestamp server; notarization needs one" >&2
		codesign --force --sign "$identity" --options runtime --timestamp "$@"
		exit 1
	fi
	echo "build-buddy-app: signing without a secure timestamp (not notarizable)" >&2
	codesign --force --sign "$identity" --options runtime --timestamp=none "$@"
}

sign --identifier "$cli_ident" "$helpers/buddy"
team="$(codesign -dv --verbose=2 "$helpers/buddy" 2>&1 | awk -F= '/^TeamIdentifier=/{print $2; exit}')"
if [[ -z "$team" || "$team" == "not set" ]]; then
	echo "build-buddy-app: TeamIdentifier not set after signing helper" >&2
	exit 1
fi

sign --identifier "$app_ident" "$macos/Buddy"
sign --identifier "$app_ident" "$app"

info="$(codesign -dv --verbose=2 "$app" 2>&1)"
printf '%s\n' "$info"
case "$info" in *Signature=adhoc*) echo "Buddy.app: ad-hoc signature rejected; use a Developer ID" >&2; exit 1;; esac
case "$info" in *TeamIdentifier=not\ set*) echo "Buddy.app: TeamIdentifier not set" >&2; exit 1;; esac

rm -rf "$out"
mkdir -p "$(dirname "$out")"
ditto "$app" "$out"
codesign --verify --verbose=2 "$out" "$out/Contents/Helpers/buddy" "$out/Contents/MacOS/Buddy"
ls -lh "$out/Contents/Helpers/buddy" "$out/Contents/MacOS/Buddy"
if [[ -n "$agent" ]]; then
	nested="$out/Contents/Library/LoginItems/BuddyAgent.app"
	# --deep: the outer signing must not have disturbed the Agent's own seal.
	codesign --verify --deep --strict --verbose=2 "$nested"
	codesign -d --entitlements - "$nested" 2>/dev/null | grep -q keychain-access-groups || {
		echo "Buddy.app: nested agent lost keychain-access-groups" >&2
		exit 1
	}
	echo "Buddy.app: agent at Contents/Library/LoginItems/BuddyAgent.app"
fi
echo "Buddy.app: $out (CLI Helpers/buddy, stub MacOS/Buddy)"
