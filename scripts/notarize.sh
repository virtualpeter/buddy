#!/usr/bin/env bash
# Submit an artefact to Apple's notary service and staple the ticket.
# Args: <app-or-pkg> <notarytool-keychain-profile>
#
# Stapling matters more than the submission: the ticket travels with the file,
# so a Mac with no network still sees a notarized artefact rather than an
# unidentified developer.
#
# Create the profile once:
#   xcrun notarytool store-credentials <profile> --apple-id … --team-id …
#
# BUDDY_NOTARY_WAIT bounds the wait (default 30m). Apple's queue is unpredictable
# — a pkg can sit for hours while the app inside it clears in seconds — so the
# wait is capped and the run is resumable with 'make staple' rather than being
# lost when someone gives up on a blocked terminal.
set -euo pipefail

target="${1:?app bundle or pkg to notarize}"
profile="${2:?notarytool keychain profile}"

test "$(uname -s)" = Darwin || { echo "notarize: macOS only" >&2; exit 1; }
test -e "$target" || { echo "notarize: missing $target" >&2; exit 1; }
xcrun notarytool --help >/dev/null 2>&1 || { echo "notarize: notarytool not available (install Xcode)" >&2; exit 1; }

work="$(mktemp -d /tmp/buddy-notarize.XXXXXX)"
cleanup() { rm -rf "$work"; }
trap cleanup EXIT

case "$target" in
*.pkg)
	# A pkg is submitted as-is; an unsigned one is rejected outright.
	if ! pkgutil --check-signature "$target" >/dev/null 2>&1; then
		echo "notarize: $target is not signed; set BUDDY_PKG_SIGN_IDENTITY (Developer ID Installer)" >&2
		exit 1
	fi
	submission="$target"
	;;
*.app)
	# An app bundle has to be zipped, and only ditto preserves the symlinks and
	# extended attributes the signature covers.
	submission="$work/$(basename "$target").zip"
	ditto -c -k --keepParent "$target" "$submission"
	;;
*)
	echo "notarize: expected a .app or .pkg, got $target" >&2
	exit 1
	;;
esac

echo "Notarizing $(basename "$target")..."
wait_for="${BUDDY_NOTARY_WAIT:-30m}"
log="$work/submit.log"
xcrun notarytool submit "$submission" \
	--keychain-profile "$profile" --wait --timeout "$wait_for" 2>&1 | tee "$log" || true

# Judge the outcome by what the service said, not by notarytool's exit code:
# a timeout and a rejection are different problems with different fixes.
id="$(sed -nE 's/^ *id: ([0-9a-f-]{36}).*/\1/p' "$log" | head -1)"
if grep -q 'status: Invalid' "$log"; then
	echo >&2
	echo "notarize: Apple rejected $(basename "$target")" >&2
	if [[ -n "$id" ]]; then
		echo "notarize: reasons follow (submission $id)" >&2
		xcrun notarytool log "$id" --keychain-profile "$profile" >&2 || true
	fi
	exit 1
fi
if ! grep -q 'status: Accepted' "$log"; then
	# The service keeps processing after notarytool stops polling, so this is
	# "not yet", not "no". The submitted file is still valid to staple later.
	echo >&2
	echo "notarize: stopped waiting after $wait_for; Apple is still processing $(basename "$target")" >&2
	test -n "$id" && echo "notarize: submission $id" >&2
	echo "notarize: check with  xcrun notarytool history --keychain-profile $profile" >&2
	echo "notarize: finish with  make staple  — do not rebuild first, that invalidates the ticket" >&2
	echo "notarize: set BUDDY_NOTARY_WAIT (e.g. 2h) to wait longer next time" >&2
	exit 1
fi

xcrun stapler staple "$target"
xcrun stapler validate "$target"

case "$target" in
*.pkg) spctl -a -vvv -t install "$target" ;;
*.app) spctl -a -vvv -t exec "$target" ;;
esac

echo "Notarized and stapled: $target"
