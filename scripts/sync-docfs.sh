#!/usr/bin/env bash
# Sync documentation into docfs/content for go:embed.
# Source of truth: docs/ (hand + docs/generated/{human,agent}/), README.md, examples/.
# AGENTS.md and .cursor/rules stay in the repo for humans/Cursor — not embedded.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
dest="$root/docfs/content"
rm -rf "$dest"
mkdir -p "$dest/docs/generated/human" "$dest/docs/generated/agent" "$dest/examples"

# Hand-authored cookbooks (exclude empty globs if only generated apps exist).
shopt -s nullglob
for f in "$root"/docs/*.md; do
	cp "$f" "$dest/docs/"
done
for f in "$root"/docs/generated/human/*.md; do
	cp "$f" "$dest/docs/generated/human/"
done
for f in "$root"/docs/generated/agent/*.md; do
	cp "$f" "$dest/docs/generated/agent/"
done

cp "$root/README.md" "$dest/README.md"

for f in "$root"/examples/*.md "$root"/examples/*.buddy "$root"/examples/*.json; do
	cp "$f" "$dest/examples/"
done

# Marker so embed never sees an empty tree.
printf 'embedded knowledge for buddy CLI/MCP/serve\n' >"$dest/README.embedded"

echo "synced docfs/content from docs/ + generated + README + examples"
