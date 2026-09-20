#!/usr/bin/env bash
# Confirm a docs/releases.md section, commit it, and annotated-tag HEAD.
# Args: [tag]   vMAJOR.MINOR.PATCH; omit to prompt (default last tag + patch)
#
# Must be on main, with a clean tree, and HEAD must not already be an exact tag.
# Does not push.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

src="${NOTES_SRC:-docs/releases.md}"
want_branch="main"

die() {
	echo "make tag: $*" >&2
	exit 1
}

need_tty() {
	if [ ! -r /dev/tty ] || [ ! -w /dev/tty ]; then
		die "run from a terminal"
	fi
}

normalize_tag() {
	local t="$1"
	case "$t" in
	v*) ;;
	*) t="v$t" ;;
	esac
	if ! [[ "$t" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
		die "tag must be vMAJOR.MINOR.PATCH (got \"$1\")"
	fi
	printf '%s\n' "$t"
}

version_gt() {
	local a1 a2 a3 b1 b2 b3
	IFS=. read -r a1 a2 a3 <<<"${1#v}"
	IFS=. read -r b1 b2 b3 <<<"${2#v}"
	if [ "$a1" -gt "$b1" ]; then return 0; fi
	if [ "$a1" -lt "$b1" ]; then return 1; fi
	if [ "$a2" -gt "$b2" ]; then return 0; fi
	if [ "$a2" -lt "$b2" ]; then return 1; fi
	[ "$a3" -gt "$b3" ]
}

latest_version_tag() {
	local t
	while IFS= read -r t; do
		if [[ "$t" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
			printf '%s\n' "$t"
			return 0
		fi
	done < <(git tag -l 'v*.*.*' --sort=-v:refname)
}

patch_bump() {
	local maj min pat
	IFS=. read -r maj min pat <<<"${1#v}"
	printf 'v%s.%s.%s\n' "$maj" "$min" "$((pat + 1))"
}

extract_section() {
	local file="$1" tag="$2"
	awk -v tag="$tag" -v bare="${tag#v}" '
		/^## / {
			heading = $0
			sub(/^## /, "", heading)
			if (started) exit
			if (heading == tag || heading == bare || heading == "v" bare) {
				started = 1
			} else {
				next
			}
		}
		started { print }
	' "$file"
}

trim_section() {
	printf '%s\n' "$1" | sed -e 's/[[:space:]]*$//' | sed -e '/./,$!d' | sed -e :a -e '/^[[:space:]]*$/{$d;N;ba' -e '}'
}

write_section() {
	local file="$1" tag="$2" section="$3"
	local tmp extra
	tmp="$(mktemp "${TMPDIR:-/tmp}/buddy-tag.XXXXXX")"
	extra="$(mktemp "${TMPDIR:-/tmp}/buddy-tag.XXXXXX")"
	printf '%s\n' "$section" >"$extra"
	if [ -n "$(extract_section "$file" "$tag")" ]; then
		awk -v extra="$extra" -v tag="$tag" -v bare="${tag#v}" '
			/^## / {
				heading = $0
				sub(/^## /, "", heading)
				if (in_sec) in_sec = 0
				if (heading == tag || heading == bare || heading == "v" bare) {
					while ((getline line < extra) > 0) print line
					close(extra)
					print ""
					in_sec = 1
					next
				}
			}
			in_sec { next }
			{ print }
		' "$file" >"$tmp"
	else
		awk -v extra="$extra" '
			/^## / && !done {
				while ((getline line < extra) > 0) print line
				close(extra)
				print ""
				done = 1
			}
			{ print }
			END {
				if (!done) {
					print ""
					while ((getline line < extra) > 0) print line
					close(extra)
				}
			}
		' "$file" >"$tmp"
	fi
	mv "$tmp" "$file"
	rm -f "$extra"
}

ensure_heading() {
	local tag="$1" section="$2"
	local first
	first="$(printf '%s\n' "$section" | sed -n '1p')"
	case "$first" in
	"## $tag" | "## ${tag#v}" | "## v${tag#v}")
		printf '%s\n' "$section"
		;;
	*)
		printf '## %s\n\n%s\n' "$tag" "$section"
		;;
	esac
}

branch="$(git rev-parse --abbrev-ref HEAD)"
if [ "$branch" != "$want_branch" ]; then
	die "must be on $want_branch (got $branch)"
fi

if [ -n "$(git status --porcelain)" ]; then
	echo "make tag: working tree is dirty; commit or stash first" >&2
	git status --porcelain >&2
	exit 1
fi

if git describe --tags --exact-match >/dev/null 2>&1; then
	die "HEAD is already tagged $(git describe --tags --exact-match); nothing to tag"
fi

if [ ! -f "$src" ]; then
	die "$src: not found"
fi

need_tty

latest="$(latest_version_tag || true)"
default=""
if [ -n "$latest" ]; then
	default="$(patch_bump "$latest")"
fi

tag="${1:-}"
if [ -z "$tag" ]; then
	if [ -z "$default" ]; then
		die "no existing vMAJOR.MINOR.PATCH tag to bump; set TAG=vX.Y.Z"
	fi
	printf 'Tag [%s]: ' "$default" >/dev/tty
	read -r tag </dev/tty
	if [ -z "$tag" ]; then
		tag="$default"
	fi
fi
tag="$(normalize_tag "$tag")"

if git rev-parse -q --verify "refs/tags/$tag" >/dev/null; then
	die "tag $tag already exists"
fi
if [ -n "$latest" ] && ! version_gt "$tag" "$latest"; then
	die "tag $tag is not greater than latest tag $latest"
fi

prev="$(git describe --tags --abbrev=0 2>/dev/null || true)"
existing="$(extract_section "$src" "$tag" || true)"
existing="$(trim_section "$existing")"
if [ -n "$existing" ]; then
	section="$existing"
else
	if [ -n "$prev" ]; then
		log="$(git log --no-merges --pretty=tformat:'- %s' "${prev}..HEAD")"
	else
		log="$(git log --no-merges --pretty=tformat:'- %s')"
	fi
	if [ -n "$log" ]; then
		section="$(printf '## %s\n\n%s\n' "$tag" "$log")"
	else
		section="$(printf '## %s\n' "$tag")"
	fi
fi
section="$(trim_section "$section")"

while true; do
	{
		echo "----- proposed notes for $tag -----"
		printf '%s\n' "$section"
		echo "-----"
		printf 'Accept these notes? [a]ccept / [e]dit / [q]uit: '
	} >/dev/tty
	read -r ans </dev/tty
	case "$ans" in
	a | A | accept | y | Y | yes)
		break
		;;
	e | E | edit)
		tmp="$(mktemp "${TMPDIR:-/tmp}/buddy-tag.XXXXXX")"
		printf '%s\n' "$section" >"$tmp"
		"${VISUAL:-${EDITOR:-vi}}" "$tmp" </dev/tty >/dev/tty 2>/dev/tty || {
			rm -f "$tmp"
			die "editor failed"
		}
		section="$(trim_section "$(cat "$tmp")")"
		rm -f "$tmp"
		if [ -z "$section" ]; then
			die "notes are empty"
		fi
		section="$(trim_section "$(ensure_heading "$tag" "$section")")"
		;;
	q | Q | quit | n | N | no | "")
		die "aborted"
		;;
	*)
		echo "make tag: enter a, e, or q" >/dev/tty
		;;
	esac
done

if [ "$section" != "$existing" ]; then
	write_section "$src" "$tag" "$section"
fi

if git diff --quiet -- "$src"; then
	echo "make tag: $src already has $tag; tagging HEAD"
else
	git add -- "$src"
	git commit -m "Add $tag release notes"
fi

git tag -a "$tag" -m "$tag"
echo "make tag: tagged $tag"
echo "Next: make release"
echo "Then: make publish-preview TAG=$tag"
echo "Then: make publish TAG=$tag"
