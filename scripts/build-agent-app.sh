#!/usr/bin/env bash
# Build and Developer ID sign the Buddy Agent (macos/BuddyAgent) into a .app.
# Args: <out-app> <version> [team-id]
#
# Archive + export rather than a plain `xcodebuild build`: with automatic
# signing only the export step picks the Developer ID identity and embeds the
# provisioning profile that authorizes keychain-access-groups. Without that
# profile the Agent keeps secrets in memory only and loses the presence ACL,
# so a build that silently drops it is a build worth failing.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

out="${1:?output .app path}"
version="${2:?version}"
team="${3:-}"

project="macos/BuddyAgent/BuddyAgent.xcodeproj"
scheme=BuddyAgent

test "$(uname -s)" = Darwin || { echo "build-agent-app: macOS only" >&2; exit 1; }
test -d "$project" || { echo "build-agent-app: missing $project" >&2; exit 1; }
command -v xcodebuild >/dev/null || { echo "build-agent-app: xcodebuild not found (install Xcode)" >&2; exit 1; }

if [[ -z "$team" ]]; then
	# Same source of truth as the CLI: the Developer ID in the keychain.
	team="$(security find-identity -v -p codesigning 2>/dev/null |
		sed -nE 's/.*Developer ID Application: .*\(([A-Z0-9]+)\).*/\1/p' | head -1)"
fi
test -n "$team" || {
	echo "build-agent-app: no Developer ID Application identity found; set BUDDY_TEAM_ID" >&2
	exit 1
}

work="$(mktemp -d /tmp/buddy-agent.XXXXXX)"
cleanup() { rm -rf "$work"; }
trap cleanup EXIT

cat >"$work/ExportOptions.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>method</key>
	<string>developer-id</string>
	<key>signingStyle</key>
	<string>automatic</string>
	<key>teamID</key>
	<string>${team}</string>
	<key>destination</key>
	<string>export</string>
</dict>
</plist>
PLIST

echo "Building BuddyAgent.app (version $version, team $team)..."
# Same logo as Buddy.app; generated into Resources so the archive picks it up.
"$root/scripts/gen-app-icon.sh" "$root/macos/BuddyAgent/Resources/AppIcon.icns"
xcodebuild \
	-project "$project" \
	-scheme "$scheme" \
	-configuration Release \
	-archivePath "$work/BuddyAgent.xcarchive" \
	-allowProvisioningUpdates \
	DEVELOPMENT_TEAM="$team" \
	MARKETING_VERSION="$version" \
	CURRENT_PROJECT_VERSION=1 \
	archive >"$work/archive.log" 2>&1 ||
	{ tail -40 "$work/archive.log" >&2; echo "build-agent-app: archive failed" >&2; exit 1; }

xcodebuild \
	-exportArchive \
	-archivePath "$work/BuddyAgent.xcarchive" \
	-exportOptionsPlist "$work/ExportOptions.plist" \
	-exportPath "$work/export" \
	-allowProvisioningUpdates >"$work/export.log" 2>&1 ||
	{ tail -40 "$work/export.log" >&2; echo "build-agent-app: export failed" >&2; exit 1; }

built="$work/export/BuddyAgent.app"
test -d "$built" || { echo "build-agent-app: export produced no BuddyAgent.app" >&2; exit 1; }

info="$(codesign -dv --verbose=2 "$built" 2>&1)"
printf '%s\n' "$info"
case "$info" in *Signature=adhoc*) echo "BuddyAgent.app: ad-hoc signature rejected; use a Developer ID" >&2; exit 1;; esac
case "$info" in *TeamIdentifier=not\ set*) echo "BuddyAgent.app: TeamIdentifier not set" >&2; exit 1;; esac
case "$info" in *"flags=0x10000(runtime)"*) ;; *) echo "BuddyAgent.app: hardened runtime missing" >&2; exit 1;; esac
case "$info" in *Timestamp=*) ;; *) echo "BuddyAgent.app: no secure timestamp; notarization would reject it" >&2; exit 1;; esac

# The whole point of the Xcode route: without these the Agent silently keeps
# secrets in memory only and presence stops gating the Keychain item.
test -f "$built/Contents/embedded.provisionprofile" || {
	echo "BuddyAgent.app: no embedded.provisionprofile; keychain-access-groups would be unauthorized" >&2
	exit 1
}
codesign -d --entitlements - "$built" 2>/dev/null | grep -q keychain-access-groups || {
	echo "BuddyAgent.app: keychain-access-groups entitlement missing" >&2
	exit 1
}

rm -rf "$out"
mkdir -p "$(dirname "$out")"
ditto "$built" "$out"
codesign --verify --strict --verbose=2 "$out"
echo "BuddyAgent.app: $out (Developer ID, presence ACL available)"
