#!/usr/bin/env bash
# Build buddy with feature-tag combinations and print on-disk sizes.
# Tags are exclusion-style: noserve, nomcp, nolsp (default build = all features).
# Matches Makefile default: CGO_ENABLED=0 (override with env).
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

export CGO_ENABLED="${CGO_ENABLED:-0}"

outdir="${TMPDIR:-/tmp}/buddy-size-report-$$"
mkdir -p "$outdir"
cleanup() { rm -rf "$outdir"; }
trap cleanup EXIT

ver="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"
ldflags="-X main.Version=${ver}"

build_one() {
	local label="$1"
	local tags="$2"
	local out="$outdir/buddy-${label}"
	if [[ -n "$tags" ]]; then
		go build -tags "$tags" -buildvcs=false -ldflags "$ldflags" -o "$out" ./cmd/buddy
	else
		go build -buildvcs=false -ldflags "$ldflags" -o "$out" ./cmd/buddy
	fi
	local bytes
	bytes=$(wc -c <"$out" | tr -d ' ')
	printf '%s\t%s\t%s\n' "$label" "$bytes" "$(ls -lh "$out" | awk '{print $5}')"
}

echo "Building size variants (version $ver, CGO_ENABLED=$CGO_ENABLED)…"
echo ""
printf '%s\t%s\t%s\n' "variant" "bytes" "size"
printf '%s\t%s\t%s\n' "-------" "-----" "----"

full=$(build_one full "")
echo "$full"
full_bytes=$(echo "$full" | awk '{print $2}')

noserve=$(build_one noserve "noserve")
echo "$noserve"
nomcp=$(build_one nomcp "nomcp")
echo "$nomcp"
nolsp=$(build_one nolsp "nolsp")
echo "$nolsp"
lite=$(build_one lite "noserve,nomcp,nolsp")
echo "$lite"

# CGO on/off comparison for the full feature set (always runs both).
cgo1_bytes=""
if [[ "$CGO_ENABLED" == "0" ]]; then
	echo ""
	echo "CGO comparison (full features):"
	CGO_ENABLED=1 go build -buildvcs=false -ldflags "$ldflags" -o "$outdir/buddy-cgo1" ./cmd/buddy
	cgo1_bytes=$(wc -c <"$outdir/buddy-cgo1" | tr -d ' ')
	printf '  CGO_ENABLED=0  %s (%s)\n' "$full_bytes" "$(ls -lh "$outdir/buddy-full" | awk '{print $5}')"
	printf '  CGO_ENABLED=1  %s (%s)\n' "$cgo1_bytes" "$(ls -lh "$outdir/buddy-cgo1" | awk '{print $5}')"
	printf '  delta (0−1)    %+d bytes\n' "$((full_bytes - cgo1_bytes))"
fi

echo ""
echo "Delta vs full (negative = smaller):"
delta() {
	local label="$1" bytes="$2"
	local d=$((bytes - full_bytes))
	local human
	if (( d <= -1048576 )) || (( d >= 1048576 )); then
		human=$(awk -v d="$d" 'BEGIN{printf "%.1f MiB", d/1048576}')
	elif (( d <= -1024 )) || (( d >= 1024 )); then
		human=$(awk -v d="$d" 'BEGIN{printf "%.0f KiB", d/1024}')
	else
		human="${d} B"
	fi
	printf '  %-12s %+d bytes (%s)\n' "$label" "$d" "$human"
}
delta noserve "$(echo "$noserve" | awk '{print $2}')"
delta nomcp "$(echo "$nomcp" | awk '{print $2}')"
delta nolsp "$(echo "$nolsp" | awk '{print $2}')"
delta lite "$(echo "$lite" | awk '{print $2}')"

echo ""
echo "Notes:"
echo "  - Makefile defaults to CGO_ENABLED=0 (buddy does not need CGO)."
echo "  - Embeds on disk: docfs ~304KiB, platformschema ~268KiB, stdlib ~5KiB (not the bulk)."
echo "  - noserve mainly drops goldmark + chroma (markdown UI for buddy serve)."
echo "  - nomcp drops modelcontextprotocol/go-sdk."
echo "  - nolsp is small; starlark remains via buddyscript."
echo "  - Remaining size is Go runtime + internal/buddy + script + crypto/net + deps."
