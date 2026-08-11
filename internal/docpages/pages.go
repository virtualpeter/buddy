// Package docpages loads buddy documentation pages for serve and MCP.
//
// Two catalogs share curated docs (hand-authored and generated):
//   - CatalogHuman — curated pages for the buddy serve HTML UI (neutral tone)
//   - CatalogAgent — MCP knowledge (agent-tone generated pages, examples, help)
//
// AGENTS.md and .cursor/rules stay in the repo for humans/Cursor; they are not
// embedded or exposed as buddy://docs resources.
//
// Pages come from the embedded docfs by default; --docs-root / BUDDY_DOCS_ROOT
// (or a discovered repo docs/ when PreferDisk) override with live files.
package docpages

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"buddy/docfs"
	"buddy/internal/help"
)

// Catalog selects which page set to load.
type Catalog int

const (
	// CatalogHuman is the curated set shown in buddy serve.
	CatalogHuman Catalog = iota
	// CatalogAgent is the full knowledge set for MCP agents.
	CatalogAgent
)

// Page is a navigable documentation / knowledge page.
type Page struct {
	ID       string // e.g. "docs/reference-manual", "agents", "rules/buddy-cli.mdc"
	Title    string
	Section  string // nav grouping for the human UI (Start here, Guides, …)
	Markdown string
	Plain    bool     // true = render as <pre> (CLI help)
	Source   string   // "embedded:…" or filesystem path
	Keywords []string // search-only terms for MCP docs list filter (not shown in nav)
}

// LoadOptions controls where pages are loaded from.
type LoadOptions struct {
	// Catalog selects human UI pages vs full agent knowledge (default: CatalogAgent).
	Catalog Catalog
	// DocsRoot is an optional filesystem docs/ directory (--docs-root / BUDDY_DOCS_ROOT).
	DocsRoot string
	// PreferDisk forces filesystem discovery when DocsRoot is empty.
	PreferDisk bool
}

// LoadResult is the page catalog plus where it was loaded from.
type LoadResult struct {
	Pages  []Page
	Source string // filesystem docs root, or "embedded"
}

// Load returns the documentation catalog for the requested audience.
func Load(opts LoadOptions) (LoadResult, error) {
	root := resolveOverrideRoot(opts)
	var (
		pages []Page
		err   error
		src   string
	)
	if root != "" {
		src = root
		switch opts.Catalog {
		case CatalogHuman:
			pages, err = loadHumanFromDisk(root)
		default:
			pages, err = loadAgentFromDisk(root)
		}
	} else {
		src = "embedded"
		switch opts.Catalog {
		case CatalogHuman:
			pages, err = loadHumanFromEmbedded()
		default:
			pages, err = loadAgentFromEmbedded()
		}
	}
	if err != nil {
		return LoadResult{}, err
	}
	return LoadResult{Pages: pages, Source: src}, nil
}

func resolveOverrideRoot(opts LoadOptions) string {
	if opts.DocsRoot != "" {
		if root, err := validateDocsRoot(opts.DocsRoot); err == nil {
			return root
		}
	}
	if env := os.Getenv("BUDDY_DOCS_ROOT"); env != "" {
		if root, err := validateDocsRoot(env); err == nil {
			return root
		}
	}
	if opts.PreferDisk {
		if root, err := findDocsRootFromCwd(); err == nil {
			return root
		}
	}
	return ""
}

func validateDocsRoot(path string) (string, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !fi.IsDir() {
		return "", fmt.Errorf("docs root is not a directory: %s", path)
	}
	return filepath.Clean(path), nil
}

func findDocsRootFromCwd() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := cwd
	for {
		candidate := filepath.Join(dir, "docs")
		if fi, err := os.Stat(filepath.Join(candidate, "index.md")); err == nil && !fi.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("docs/ not found near cwd")
}

// curatedDoc is one docs/*.md entry shared by human and agent catalogs.
// file is relative to docs/ (or content/docs/ when embedded). When humanFile /
// agentFile are set, each catalog loads that path instead (generated dual docs).
type curatedDoc struct {
	id, title, section, file string
	humanFile, agentFile     string // optional; relative to docs/
	keywords                 []string
}

func (e curatedDoc) humanPath() string {
	if e.humanFile != "" {
		return e.humanFile
	}
	return e.file
}

func (e curatedDoc) agentPath() string {
	if e.agentFile != "" {
		return e.agentFile
	}
	return e.file
}

// curatedDocEntries is the curated docs/*.md order for serve + MCP.
// Keywords are search-only (MCP docs list); they are not shown in the UI.
var curatedDocEntries = []curatedDoc{
	{id: "docs/index", title: "Getting started", section: "Start here", file: "index.md"},

	// Resources — Classic / Pro object cookbooks
	{id: "docs/policies", title: "Policies", section: "Resources",
		humanFile: "generated/human/policies.md", agentFile: "generated/agent/policies.md",
		keywords: []string{"policy", "policies", "policy_set", "policy_copy", "policy_migrate", "trigger", "classic"}},
	{id: "docs/smart-groups", title: "Smart computer groups", section: "Resources",
		humanFile: "generated/human/smart-groups.md", agentFile: "generated/agent/smart-groups.md",
		keywords: []string{"smart", "smart_group", "smart_groups", "criteria", "membership", "smart_group_list_computers", "group", "groups"}},
	{id: "docs/static-groups", title: "Static computer groups", section: "Resources",
		humanFile: "generated/human/static-groups.md", agentFile: "generated/agent/static-groups.md",
		keywords: []string{"static", "static_group", "static_groups", "membership", "assignments", "group", "groups"}},
	{id: "docs/profiles", title: "Configuration profiles", section: "Resources",
		humanFile: "generated/human/profiles.md", agentFile: "generated/agent/profiles.md",
		keywords: []string{"profile", "profiles", "clone", "copy", "redeploy", "acs"}},
	{id: "docs/profile-payloads", title: "Profile payloads", section: "Resources",
		humanFile: "generated/human/profile-payloads.md", agentFile: "generated/agent/profile-payloads.md",
		keywords: []string{"profile", "profiles", "payload", "payloads", "mcx", "acs", "plist"}},
	{id: "docs/packages", title: "Packages / JCDS", section: "Resources",
		humanFile: "generated/human/packages.md", agentFile: "generated/agent/packages.md",
		keywords: []string{"package", "packages", "jcds", "ready", "download", "upload", "cloudTransferStatus"}},
	{id: "docs/scripts", title: "Scripts", section: "Resources",
		humanFile: "generated/human/scripts.md", agentFile: "generated/agent/scripts.md",
		keywords: []string{"script", "scripts", "script_copy", "script_migrate", "scriptContents"}},
	{id: "docs/categories", title: "Categories", section: "Resources",
		humanFile: "generated/human/categories.md", agentFile: "generated/agent/categories.md",
		keywords: []string{"category", "categories", "category_copy", "category_migrate"}},
	{id: "docs/macapps", title: "Mac App Store apps", section: "Resources",
		humanFile: "generated/human/macapps.md", agentFile: "generated/agent/macapps.md",
		keywords: []string{"macapp", "macapps", "macapplications", "mac-app-store", "mac app store", "app store"}},
	{id: "docs/apps", title: "Policy apps", section: "Resources",
		humanFile: "generated/human/apps.md", agentFile: "generated/agent/apps.md",
		keywords: []string{"app", "apps", "policy app", "policy apps", "self service", "app upgrade", "install-next", "cache policy"}},
	{id: "docs/computers", title: "Computers", section: "Resources",
		humanFile: "generated/human/computers.md", agentFile: "generated/agent/computers.md",
		keywords: []string{"computer", "computers", "computer_detail", "inventory", "section", "reboot", "lock", "erase", "mdm"}},
	{id: "docs/mdm", title: "MDM command history", section: "Resources",
		humanFile: "generated/human/mdm.md", agentFile: "generated/agent/mdm.md",
		keywords: []string{"mdm", "mdm_commands", "command", "commands", "history"}},
	{id: "docs/api-roles", title: "API roles", section: "Resources",
		humanFile: "generated/human/api-roles.md", agentFile: "generated/agent/api-roles.md",
		keywords: []string{"role", "roles", "api role", "privileges", "privilege"}},
	{id: "docs/api-clients", title: "API clients", section: "Resources",
		humanFile: "generated/human/api-clients.md", agentFile: "generated/agent/api-clients.md",
		keywords: []string{"client", "clients", "integration", "integrations", "oauth", "api client"}},

	// Platform — Platform API Gateway cookbooks
	{id: "docs/blueprints", title: "Blueprints", section: "Platform",
		humanFile: "generated/human/blueprints.md", agentFile: "generated/agent/blueprints.md",
		keywords: []string{"blueprint", "blueprints", "deploy", "undeploy", "platform"}},
	{id: "docs/devices", title: "Platform devices", section: "Platform",
		humanFile: "generated/human/devices.md", agentFile: "generated/agent/devices.md",
		keywords: []string{"device", "devices", "platform", "device_action", "check-in", "restart"}},
	{id: "docs/device-groups", title: "Platform device groups", section: "Platform",
		humanFile: "generated/human/device-groups.md", agentFile: "generated/agent/device-groups.md",
		keywords: []string{"device_group", "device_groups", "device-group", "platform", "membership"}},
	{id: "docs/ddm", title: "DDM reporting", section: "Platform",
		humanFile: "generated/human/ddm.md", agentFile: "generated/agent/ddm.md",
		keywords: []string{"ddm", "declaration", "declarations", "declarative"}},
	{id: "docs/benchmarks", title: "Compliance benchmarks", section: "Platform",
		humanFile: "generated/human/benchmarks.md", agentFile: "generated/agent/benchmarks.md",
		keywords: []string{"benchmark", "benchmarks", "compliance", "baseline", "baselines"}},

	// BuddyScript — language guide (function index appended as builtins page)
	{id: "docs/buddyscript", title: "Overview", section: "BuddyScript", file: "buddyscript.md",
		keywords: []string{"buddyscript", "starlark", "script", "main", "shell", "notebook", "ide"}},
	{id: "docs/buddyscript-syntax", title: "Syntax", section: "BuddyScript", file: "buddyscript-syntax.md",
		keywords: []string{"syntax", "starlark", "python", "types", "load", "spec", "while"}},
	{id: "docs/buddyscript-stdlib", title: "Stdlib", section: "BuddyScript", file: "buddyscript-stdlib.md",
		keywords: []string{"stdlib", "load", "bump_release", "setup_app_classic", "release"}},

	// Guides — cross-cutting
	{id: "docs/reference-manual", title: "Reference manual", section: "Guides", file: "reference-manual.md",
		keywords: []string{"config", "credentials", "cli", "starlark", "buddyscript", "install"}},
	{id: "docs/change-batches", title: "Change batches", section: "Guides", file: "change-batches.md",
		keywords: []string{"change", "batch", "batches", "plan_change", "run_change", "step_", "release", "bump"}},
	{id: "docs/api-endpoints", title: "API endpoints", section: "Guides", file: "api-endpoints.md",
		keywords: []string{"endpoint", "endpoints", "http", "api", "rsql", "jcds"}},
	{id: "docs/mcp", title: "MCP for AI tools", section: "Guides", file: "mcp.md",
		keywords: []string{"mcp", "cursor", "claude", "buddyscript"}},
	{id: "docs/mcp-tools", title: "MCP tools scorecard", section: "Guides", file: "mcp-tools.md",
		keywords: []string{"mcp", "scorecard", "pathfinding", "agent", "lean"}},
	{id: "docs/editor", title: "Editor / LSP", section: "Guides", file: "editor.md",
		keywords: []string{"editor", "lsp", "vscode", "extension", "vsix"}},
}

func pageFromCurated(e curatedDoc, markdown, source string) Page {
	return Page{
		ID: e.id, Title: e.title, Section: e.section,
		Markdown: markdown, Source: source, Keywords: e.keywords,
	}
}

func loadHumanFromDisk(docsRoot string) ([]Page, error) {
	repoRoot := filepath.Dir(docsRoot)
	var pages []Page
	for _, e := range curatedDocEntries {
		path := filepath.Join(docsRoot, e.humanPath())
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		pages = append(pages, pageFromCurated(e, string(b), path))
	}
	pages = append(pages, builtinsPage())
	for _, p := range loadExamplesDir(filepath.Join(repoRoot, "examples")) {
		if humanExampleID(p.ID) {
			p.Section = "Examples"
			pages = append(pages, p)
		}
	}
	return pages, nil
}

func humanExampleID(id string) bool {
	return strings.HasSuffix(id, ".buddy") ||
		strings.HasSuffix(id, ".star") ||
		strings.HasSuffix(id, ".md") ||
		strings.HasSuffix(id, ".json")
}

func loadHumanFromEmbedded() ([]Page, error) {
	var pages []Page
	for _, e := range curatedDocEntries {
		path := "content/docs/" + e.humanPath()
		body, err := fs.ReadFile(docfs.Content, path)
		if err != nil {
			return nil, fmt.Errorf("embedded %s: %w", path, err)
		}
		pages = append(pages, pageFromCurated(e, string(body), "embedded:"+path))
	}
	pages = append(pages, builtinsPage())
	ents, err := fs.ReadDir(docfs.Content, "content/examples")
	if err == nil {
		for _, ent := range ents {
			if ent.IsDir() || !humanExampleID("examples/"+ent.Name()) {
				continue
			}
			name := ent.Name()
			path := "content/examples/" + name
			body, err := fs.ReadFile(docfs.Content, path)
			if err != nil {
				return nil, err
			}
			p := examplePage("examples/"+name, name, string(body), "embedded:"+path)
			p.Section = "Examples"
			pages = append(pages, p)
		}
	}
	return pages, nil
}

func loadAgentCuratedDocs(docsRoot string) ([]Page, error) {
	var pages []Page
	for _, e := range curatedDocEntries {
		path := filepath.Join(docsRoot, e.agentPath())
		b, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		pages = append(pages, pageFromCurated(e, string(b), path))
	}
	return pages, nil
}

func loadAgentFromDisk(docsRoot string) ([]Page, error) {
	repoRoot := filepath.Dir(docsRoot)
	pages, err := loadAgentCuratedDocs(docsRoot)
	if err != nil {
		return nil, err
	}

	readme := filepath.Join(repoRoot, "README.md")
	if b, err := os.ReadFile(readme); err == nil {
		pages = append(pages, Page{
			ID: "readme", Title: "README", Markdown: string(b), Source: readme,
			Keywords: []string{"readme", "install", "overview"},
		})
	}

	pages = append(pages, builtinsPage())
	pages = append(pages, loadExamplesDir(filepath.Join(repoRoot, "examples"))...)
	pages = append(pages, helpPages()...)
	return pages, nil
}

func loadAgentFromEmbedded() ([]Page, error) {
	var pages []Page
	read := func(name string) (string, error) {
		b, err := fs.ReadFile(docfs.Content, name)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}

	for _, e := range curatedDocEntries {
		path := "content/docs/" + e.agentPath()
		body, err := read(path)
		if err != nil {
			return nil, fmt.Errorf("embedded %s: %w", path, err)
		}
		pages = append(pages, pageFromCurated(e, body, "embedded:"+path))
	}

	if body, err := read("content/README.md"); err == nil {
		pages = append(pages, Page{
			ID: "readme", Title: "README", Markdown: body, Source: "embedded:content/README.md",
			Keywords: []string{"readme", "install", "overview"},
		})
	} else {
		return nil, fmt.Errorf("embedded content/README.md: %w", err)
	}

	pages = append(pages, builtinsPage())

	ents, err := fs.ReadDir(docfs.Content, "content/examples")
	if err == nil {
		for _, ent := range ents {
			if ent.IsDir() {
				continue
			}
			name := ent.Name()
			path := "content/examples/" + name
			body, err := read(path)
			if err != nil {
				return nil, err
			}
			pages = append(pages, examplePage("examples/"+name, name, body, "embedded:"+path))
		}
	}

	pages = append(pages, helpPages()...)
	return pages, nil
}

func builtinsPage() Page {
	return Page{
		ID:      "builtins",
		Title:   "Function index",
		Section: "BuddyScript",
		Markdown: help.BuiltinsMarkdown(),
		Source:  "builtin:help.BuiltinsMarkdown",
		Keywords: []string{
			"policy", "policies", "smart_group", "static_group", "profile", "package",
			"script", "category", "computer", "mdm", "role", "client",
			"blueprint", "device", "device_group", "ddm", "benchmark",
			"builtin", "builtins", "starlark", "buddyscript", "function", "index",
		},
	}
}

func helpPages() []Page {
	var pages []Page
	for _, p := range help.CLIPages() {
		if p.ID == "builtins" {
			continue
		}
		pages = append(pages, Page{
			ID:       "help/" + p.ID,
			Title:    "Help: " + p.Title,
			Markdown: "```\n" + p.Body + "\n```",
			Plain:    true,
			Source:   "builtin:help." + p.ID,
		})
	}
	return pages
}

func loadExamplesDir(dir string) []Page {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var pages []Page
	for _, ent := range ents {
		name := ent.Name()
		if ent.IsDir() || (!strings.HasSuffix(name, ".md") && !strings.HasSuffix(name, ".buddy") && !strings.HasSuffix(name, ".star") && !strings.HasSuffix(name, ".json")) {
			continue
		}
		path := filepath.Join(dir, name)
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		pages = append(pages, examplePage("examples/"+name, name, string(b), path))
	}
	return pages
}

func loadRulesDir(dir string) []Page {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var pages []Page
	for _, ent := range ents {
		name := ent.Name()
		if ent.IsDir() || !strings.HasSuffix(name, ".mdc") {
			continue
		}
		path := filepath.Join(dir, name)
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		pages = append(pages, rulePage(name, string(b), path))
	}
	return pages
}

func examplePage(id, name, body, source string) Page {
	title := name
	switch {
	case strings.HasSuffix(name, ".buddy"):
		title = strings.TrimSuffix(name, ".buddy")
	case strings.HasSuffix(name, ".star"):
		title = strings.TrimSuffix(name, ".star")
	}
	md := body
	switch {
	case strings.HasSuffix(name, ".buddy"), strings.HasSuffix(name, ".star"):
		md = "```python\n" + body + "\n```"
	case strings.HasSuffix(name, ".json"):
		md = "```json\n" + body + "\n```"
		title = "Example: " + name
	case strings.HasSuffix(name, ".md"):
		if name == "README.md" {
			title = "Examples index"
		} else {
			title = "Example: " + strings.TrimSuffix(name, ".md")
		}
	}
	return Page{ID: id, Title: title, Markdown: md, Source: source}
}

func rulePage(name, body, source string) Page {
	md := stripFrontMatter(body)
	return Page{
		ID:       "rules/" + name,
		Title:    "Rule: " + strings.TrimSuffix(name, ".mdc"),
		Markdown: md,
		Source:   source,
	}
}

func stripFrontMatter(s string) string {
	if !strings.HasPrefix(s, "---\n") {
		return s
	}
	rest := s[4:]
	if i := strings.Index(rest, "\n---\n"); i >= 0 {
		return strings.TrimSpace(rest[i+5:]) + "\n"
	}
	return s
}

// ByID returns the page with the given ID, or nil.
func ByID(pages []Page, id string) *Page {
	for i := range pages {
		if pages[i].ID == id {
			return &pages[i]
		}
	}
	return nil
}

// URI returns the MCP resource URI for a page.
func URI(p Page) string {
	return "buddy://docs/" + p.ID
}

// SearchHaystack returns the lowercased string used for docs list query matching.
// It includes separator-normalized and collapsed forms of id/title/keywords so
// queries like "smart_groups" / "smart groups" align with "smart-groups".
func SearchHaystack(p Page) string {
	base := []string{p.ID, p.Title, p.Section}
	base = append(base, p.Keywords...)
	parts := make([]string, 0, len(base)*3)
	parts = append(parts, base...)
	for _, s := range append([]string{p.ID, p.Title}, p.Keywords...) {
		parts = append(parts, sepNormalize(s), collapseAlnum(s))
	}
	return strings.ToLower(strings.Join(parts, " "))
}

func sepNormalize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '_', '-', '/', '.':
			b.WriteByte(' ')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func collapseAlnum(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// tokenizeQuery splits a lowercased query on non-alphanumeric separators.
// Tokens shorter than 2 characters are dropped unless they are all digits.
func tokenizeQuery(query string) []string {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	var tokens []string
	var b strings.Builder
	flush := func() {
		t := b.String()
		b.Reset()
		if t == "" {
			return
		}
		if len(t) >= 2 || isAllDigits(t) {
			tokens = append(tokens, t)
		}
	}
	for _, r := range q {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return tokens
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func haystackHasAllTokens(hay string, tokens []string) bool {
	if len(tokens) == 0 {
		return false
	}
	for _, t := range tokens {
		if !strings.Contains(hay, t) {
			return false
		}
	}
	return true
}

func haystackHasAnyToken(hay string, tokens []string) bool {
	for _, t := range tokens {
		if strings.Contains(hay, t) {
			return true
		}
	}
	return false
}

// MatchQuery reports whether a single page matches query under the strict
// rule (exact substring or all tokens present). Empty query matches.
// Prefer FilterByQuery for list filtering (AND then OR fallback).
func MatchQuery(p Page, query string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return true
	}
	hay := SearchHaystack(p)
	if strings.Contains(hay, q) {
		return true
	}
	return haystackHasAllTokens(hay, tokenizeQuery(q))
}

// FilterByQuery returns pages matching query. Empty/whitespace query returns
// all pages. Matching is case-insensitive and treats _, -, /, spaces, and
// other non-alphanumerics as token separators. Prefer exact/AND hits; if none,
// fall back to OR (any token) so agents get recall instead of an empty list.
func FilterByQuery(pages []Page, query string) []Page {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		out := make([]Page, len(pages))
		copy(out, pages)
		return out
	}
	tokens := tokenizeQuery(q)
	strict := make([]Page, 0)
	for _, p := range pages {
		hay := SearchHaystack(p)
		if strings.Contains(hay, q) || haystackHasAllTokens(hay, tokens) {
			strict = append(strict, p)
		}
	}
	if len(strict) > 0 || len(tokens) == 0 {
		return strict
	}
	loose := make([]Page, 0)
	for _, p := range pages {
		if haystackHasAnyToken(SearchHaystack(p), tokens) {
			loose = append(loose, p)
		}
	}
	return loose
}

// NavSections returns unique section names in page order (for the human UI).
func NavSections(pages []Page) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range pages {
		s := p.Section
		if s == "" {
			s = "Docs"
		}
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
