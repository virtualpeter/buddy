// Package docfs holds documentation and agent knowledge embedded in the buddy binary.
//
// Content under content/ is produced by scripts/sync-docfs.sh from the repo’s
// docs/, README.md, AGENTS.md, examples/, and .cursor/rules/. Run
// `make sync-docfs` (or `make buddy`) after editing those sources.
package docfs

import "embed"

// Content is the embedded knowledge tree (paths relative to content/).
//
//	docs/*.md
//	README.md
//	AGENTS.md
//	examples/*
//	rules/*.mdc
//
//go:embed all:content
var Content embed.FS
