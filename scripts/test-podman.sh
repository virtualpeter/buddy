#!/usr/bin/env bash
# Run buddy unit tests on Linux inside Podman.
#
# Default image: docker.io/library/golang:1.27
# GOTOOLCHAIN=local uses that image's Go. Module downloads stay in the
# buddy-podman-home volume.
#
# Usage:
#   scripts/test-podman.sh
#   scripts/test-podman.sh ./internal/buddy/...
#   scripts/test-podman.sh -count=1 ./internal/script/...
#   scripts/test-podman.sh shell
#
# Env:
#   BUDDY_PODMAN_IMAGE   container image (default above)
#   BUDDY_PODMAN         podman binary (default podman)
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

image="${BUDDY_PODMAN_IMAGE:-docker.io/library/golang:1.27}"
podman="${BUDDY_PODMAN:-podman}"

if ! command -v "$podman" >/dev/null 2>&1; then
	echo "test-podman: $podman not found (install podman, or set BUDDY_PODMAN)" >&2
	exit 1
fi

mode=test
if [ "${1:-}" = "shell" ]; then
	mode=shell
	shift
fi
if [ "$mode" = "test" ] && [ "$#" -eq 0 ]; then
	set -- ./...
fi

# go:embed reads docfs/content, which is not in git. Same step as `make test`.
"$root/scripts/sync-docfs.sh"

# :Z relabels the mount for SELinux on Fedora/RHEL. Podman on macOS has no SELinux.
relabel=""
if [ "$(uname -s)" != Darwin ]; then
	relabel=":Z"
fi

echo "test-podman: $image"
run=(
	"$podman" run --rm
	--pull=missing
	--user 0:0
	--entrypoint /bin/bash
	-v "$root:/src${relabel}"
	-v "buddy-podman-home:/root${relabel}"
	-w /src
	-e CGO_ENABLED=0
	-e GOTOOLCHAIN=local
	-e GOTELEMETRY=off
)
if [ -n "${GOPROXY:-}" ]; then
	run+=(-e "GOPROXY=${GOPROXY}")
fi
if [ -n "${GOSUMDB:-}" ]; then
	run+=(-e "GOSUMDB=${GOSUMDB}")
fi
if [ "$mode" = "shell" ]; then
	run+=(-i)
	if [ -t 0 ]; then
		run+=(-t)
	fi
	run+=("$image")
else
	run+=("$image" -c 'exec go test -v "$@"' bash "$@")
fi
exec "${run[@]}"
