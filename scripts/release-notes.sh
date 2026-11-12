#!/usr/bin/env bash
# Write GitHub release notes for the exact git tag on HEAD.
# Body is the ## <tag> section of docs/releases.md, through the next heading.
# Args: <out.md>
# Env: NOTES_SRC  (default docs/releases.md)
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

out="${1:?output markdown path}"
src="${NOTES_SRC:-docs/releases.md}"

tag="$(git describe --tags --exact-match 2>/dev/null || true)"
if [ -z "$tag" ]; then
	echo "release-notes: HEAD is not an exact git tag" >&2
	exit 1
fi
if [ ! -f "$src" ]; then
	echo "release-notes: $src: not found" >&2
	exit 1
fi

bare="${tag#v}"
notes="$(awk -v tag="$tag" -v bare="$bare" '
	function dotted(s) {
		sub(/^v/, "", s)
		return s ~ /^[0-9]+(\.[0-9]+)+$/
	}
	BEGIN {
		if (!dotted(bare)) {
			print "release-notes: tag is not dotted-numeric (got \"" tag "\")" > "/dev/stderr"
			failed = 1
			exit 2
		}
	}
	/^## / {
		heading = $0
		sub(/^## /, "", heading)
		is_tag = (heading == tag || heading == bare || heading == "v" bare)
		if (!started) {
			if (!is_tag) next
			started = 1
		} else {
			exit
		}
	}
	started { print }
	END {
		if (failed) exit 2
		if (!started) {
			print "release-notes: " ARGV[1] ": no \"## " tag "\" section" > "/dev/stderr"
			exit 2
		}
	}
' "$src")"
notes="$(printf '%s\n' "$notes" | sed -e 's/[[:space:]]*$//' | sed -e '/./,$!d' | sed -e :a -e '/^[[:space:]]*$/{$d;N;ba' -e '}')"
if [ -z "$notes" ]; then
	echo "release-notes: $src: no notes for $tag" >&2
	exit 1
fi

mkdir -p "$(dirname "$out")"
printf '%s\n' "$notes" >"$out"
echo "release-notes: $out (tag $tag)"
