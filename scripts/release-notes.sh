#!/usr/bin/env bash
# Write GitHub release notes for the exact git tag on HEAD.
# Body is docs/releases.md from ## <tag> through ## <major>.0.0.
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
	function parse_ver(s,    n, a) {
		sub(/^v/, "", s)
		n = split(s, a, ".")
		if (n < 1 || a[1] !~ /^[0-9]+$/) return 0
		maj = a[1] + 0
		min = (n >= 2 && a[2] ~ /^[0-9]+$/ ? a[2] + 0 : 0)
		pat = (n >= 3 && a[3] ~ /^[0-9]+$/ ? a[3] + 0 : 0)
		return 1
	}
	BEGIN {
		if (!parse_ver(bare)) {
			print "release-notes: tag is not dotted-numeric (got \"" tag "\")" > "/dev/stderr"
			failed = 1
			exit 2
		}
		want_maj = maj
		started = 0
		saw_floor = 0
	}
	/^## / {
		heading = $0
		sub(/^## /, "", heading)
		is_tag = (heading == tag || heading == bare || heading == "v" bare)
		ok = parse_ver(heading)
		if (!started) {
			if (!is_tag) next
			started = 1
		} else if (!ok || maj != want_maj) {
			exit
		}
		if (ok && maj == want_maj && min == 0 && pat == 0) {
			saw_floor = 1
		}
	}
	started { print }
	END {
		if (failed) exit 2
		if (!started) {
			print "release-notes: " ARGV[1] ": no \"## " tag "\" section" > "/dev/stderr"
			exit 2
		}
		if (!saw_floor) {
			print "release-notes: " ARGV[1] ": missing \"## v" want_maj ".0.0\" (notes go back to the major .0.0)" > "/dev/stderr"
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
echo "release-notes: $out (tag $tag through v${bare%%.*}.0.0)"
