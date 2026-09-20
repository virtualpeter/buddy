#!/usr/bin/env bash
# Create a GitHub release from regular files in dist/ (or $DIST).
# Args: <tag>   must match git describe --tags --exact-match
#
# The macOS installer (buddy-<version>.pkg) must be stapled (`stapler validate`).
#
# Env:
#   DIST          artefact directory (default dist)
#   REPO          owner/name (default: remote named public, else origin)
#   TITLE         release title (default the tag)
#   DRAFT=1       create a draft release
#   DRY_RUN=1     print the gh command and notes; do not create the release
#   PUBLISH_ARGS  extra flags passed to `gh release create`
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

tag="${1:?tag (e.g. v1.2.3)}"
dist="${DIST:-dist}"
title="${TITLE:-$tag}"
repo="${REPO:-}"
notes_file="${NOTES_FILE:-$dist/buddy-${tag#v}.md}"
dry_run=0
case "${DRY_RUN:-}${PREVIEW:-}" in
1*|true*|yes*) dry_run=1 ;;
esac

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
	.*|buddy|buddy-lite|*.md) continue ;;
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

# A shipped macOS install is the notarized pkg (ticket stapled). Refuse to
# publish a GitHub release whose installer would still need Apple's servers.
command -v xcrun >/dev/null || { echo "make publish: xcrun is not on PATH (need stapler to verify the pkg)" >&2; exit 1; }
pkgs=()
for a in "${assets[@]}"; do
	case "$a" in
	*.pkg) pkgs+=("$a") ;;
	esac
done
if [ ${#pkgs[@]} -eq 0 ]; then
	echo "make publish: no .pkg in $dist (run make release / make notarize first)" >&2
	exit 1
fi
want_pkg="$dist/buddy-${tag#v}.pkg"
found_want=0
for pkg in "${pkgs[@]}"; do
	echo "make publish: verifying notarization of $pkg"
	if ! xcrun stapler validate "$pkg"; then
		echo "make publish: $pkg is not notarized (run make notarize, or make staple if the wait was interrupted)" >&2
		exit 1
	fi
	if [ "$pkg" = "$want_pkg" ]; then
		found_want=1
	fi
done
if [ "$found_want" -ne 1 ]; then
	echo "make publish: missing $want_pkg (pkg version must match tag $tag)" >&2
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
	echo "make publish: notes file $notes_file missing (run make package first)" >&2
	exit 1
fi

echo "GitHub release $tag on $repo"
echo "  title: $title"
if [ "${DRAFT:-}" = 1 ] || [ "${DRAFT:-}" = true ]; then
	echo "  draft: yes"
fi
echo "  notes: $notes_file"
for a in "${assets[@]}"; do
	echo "  $a"
done

gh_args=(release create "$tag" --repo "$repo" --title "$title" --notes-file "$notes_file")
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

if [ "$dry_run" -eq 1 ]; then
	echo "make publish: dry run (not creating the release)"
	printf '  gh'
	for a in "${gh_args[@]}"; do
		printf ' %q' "$a"
	done
	printf '\n'
	echo "----- notes -----"
	cat "$notes_file"
	echo "-----"
	exit 0
fi

gh "${gh_args[@]}"
