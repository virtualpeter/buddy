#!/usr/bin/env bash
# Sync documentation and agent knowledge into docfs/content for go:embed.
# Source of truth remains docs/, README.md, AGENTS.md, examples/, .cursor/rules/.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
dest="$root/docfs/content"
rm -rf "$dest"
mkdir -p "$dest/docs" "$dest/examples" "$dest/rules"

cp "$root"/docs/*.md "$dest/docs/"
# Skip scratch scripts under docs/
if [[ -f "$root/docs/test.star" ]] || [[ -f "$root/docs/test.buddy" ]]; then
	: # intentionally not embedded
fi

cp "$root/README.md" "$dest/README.md"
cp "$root/AGENTS.md" "$dest/AGENTS.md"

shopt -s nullglob
for f in "$root"/examples/*.md "$root"/examples/*.buddy "$root"/examples/*.star "$root"/examples/*.json; do
	cp "$f" "$dest/examples/"
done
for f in "$root"/.cursor/rules/*.mdc; do
	cp "$f" "$dest/rules/"
done

# Marker so embed never sees an empty tree.
printf 'embedded knowledge for buddy CLI/MCP/serve\n' >"$dest/README.embedded"

echo "synced docfs/content from repo knowledge sources"
