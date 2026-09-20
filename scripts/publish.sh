#!/usr/bin/env bash
# Create a GitHub release from regular files in dist/ (or $DIST).
# Args: <tag>   must match git describe --tags --exact-match
#
# Env:
#   DIST          artefact directory (default dist)
#   REPO          owner/name (default: remote named public, else origin)
#   NOTES_FILE    markdown; notes are ## <tag> through ## <major>.0.0 (default docs/releases.md)
#   TITLE         release title (default the tag)
#   DRAFT=1       create a draft release
#   PUBLISH_ARGS  extra flags passed to `gh release create`
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

tag="${1:?tag (e.g. v1.2.3)}"
dist="${DIST:-dist}"
notes_file="${NOTES_FILE:-docs/releases.md}"
title="${TITLE:-$tag}"
repo="${REPO:-}"

if [ -n "$(git status --porcelain)" ]; then
	echo "make publish: working tree is dirty; commit or stash first" >&2
	git status --porcelain >&2
	exit 1
fi

head="$(git describe --tags --exact-match 2>/dev/null || true)"
if [ -z "$head" ]; then
	echo "make publish: HEAD is not an exact git tag (git describe --tags --exact-match)" >&2
	exit 1
fi
if [ "$tag" != "$head" ]; then
	echo "make publish: tag \"$tag\" does not match HEAD \"$head\"" >&2
	exit 1
fi

command -v gh >/dev/null || { echo "make publish: gh is not on PATH" >&2; exit 1; }

if [ ! -d "$dist" ]; then
	echo "make publish: $dist: missing (run make release first)" >&2
	exit 1
fi

assets=()
shopt -s nullglob
for p in "$dist"/*; do
	name="$(basename "$p")"
	case "$name" in
	.*|buddy|buddy-lite) continue ;;
	esac
	if [ -d "$p" ]; then
		echo "make publish: skipping $p (directory)" >&2
		continue
	fi
	if [ ! -f "$p" ]; then
		echo "make publish: skipping $p" >&2
		continue
	fi
	assets+=("$p")
done
if [ ${#assets[@]} -eq 0 ]; then
	echo "make publish: no regular files in $dist (run make release first)" >&2
	exit 1
fi

github_repo_from_url() {
	local s="$1"
	s="${s%.git}"
	s="${s%/}"
	local path=""
	case "$s" in
	git@github.com:*) path="${s#git@github.com:}" ;;
	ssh://git@github.com/*) path="${s#ssh://git@github.com/}" ;;
	https://github.com/*) path="${s#https://github.com/}" ;;
	http://github.com/*) path="${s#http://github.com/}" ;;
	*) return 1 ;;
	esac
	path="${path#/}"
	local owner="${path%%/*}"
	local rest="${path#*/}"
	local name="${rest%%/*}"
	if [ -z "$owner" ] || [ -z "$name" ] || [ "$owner" = "$path" ]; then
		return 1
	fi
	printf '%s/%s\n' "$owner" "$name"
}

if [ -z "$repo" ]; then
	for remote in public origin; do
		url="$(git remote get-url "$remote" 2>/dev/null || true)"
		if [ -n "$url" ] && parsed="$(github_repo_from_url "$url")"; then
			repo="$parsed"
			break
		fi
	done
else
	case "$repo" in
	*/*)
		if parsed="$(github_repo_from_url "$repo")"; then
			repo="$parsed"
		fi
		;;
	esac
fi
if [ -z "$repo" ]; then
	echo "make publish: no GitHub remote named public or origin; set REPO=owner/name" >&2
	exit 1
fi
case "$repo" in
*/*) ;;
*)
	echo "make publish: REPO must be owner/name (got \"$repo\")" >&2
	exit 1
	;;
esac

if [ ! -f "$notes_file" ]; then
	echo "make publish: notes file: $notes_file: not found" >&2
	exit 1
fi

bare="${tag#v}"
# GitHub notes are this tag's ## section plus every older one down to the
# major .0.0 (e.g. v12.1.1 includes v12.1.1, v12.1.0, and v12.0.0).
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
			print "make publish: tag is not dotted-numeric (got \"" tag "\")" > "/dev/stderr"
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
			print "make publish: " ARGV[1] ": no \"## " tag "\" section" > "/dev/stderr"
			exit 2
		}
		if (!saw_floor) {
			print "make publish: " ARGV[1] ": missing \"## v" want_maj ".0.0\" (notes go back to the major .0.0)" > "/dev/stderr"
			exit 2
		}
	}
' "$notes_file")"
notes="$(printf '%s\n' "$notes" | sed -e 's/[[:space:]]*$//' | sed -e '/./,$!d' | sed -e :a -e '/^[[:space:]]*$/{$d;N;ba' -e '}')"
if [ -z "$notes" ]; then
	echo "make publish: $notes_file: no notes for $tag" >&2
	exit 1
fi

notes_path="$(mktemp "${TMPDIR:-/tmp}/buddy-release-notes.XXXXXX.md")"
cleanup() { rm -f "$notes_path"; }
trap cleanup EXIT
printf '%s\n' "$notes" >"$notes_path"

echo "GitHub release $tag on $repo"
for a in "${assets[@]}"; do
	echo "  $a"
done

gh_args=(release create "$tag" --repo "$repo" --title "$title" --notes-file "$notes_path")
if [ "${DRAFT:-}" = 1 ] || [ "${DRAFT:-}" = true ]; then
	gh_args+=(--draft)
fi
# shellcheck disable=SC2206
if [ -n "${PUBLISH_ARGS:-}" ]; then
	# Intentionally word-split so PUBLISH_ARGS='--draft --verify-tag' works.
	extra=(${PUBLISH_ARGS})
	gh_args+=("${extra[@]}")
fi
gh_args+=("${assets[@]}")

gh "${gh_args[@]}"
