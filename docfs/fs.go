// Package docfs holds documentation embedded in the buddy binary.
//
// Content under content/ is produced by scripts/sync-docfs.sh from the repo’s
// docs/ (including docs/generated/{human,agent}/), README.md, and examples/.
// AGENTS.md and .cursor/rules are not embedded. Run `make sync-docfs` (or
// `make buddy`) after editing those sources; for policy apps edit
// internal/buddy/app_doc.go then `make gendocs`.
package docfs

import "embed"

// Content is the embedded knowledge tree (paths relative to content/).
//
//	docs/*.md
//	docs/generated/human/*.md
//	docs/generated/agent/*.md
//	README.md
//	examples/*
//
//go:embed all:content
var Content embed.FS
