// Package docpages loads buddy documentation pages for serve and MCP.
//
// Two catalogs share the same sources:
//   - CatalogHuman — curated pages for the buddy serve HTML UI
//   - CatalogAgent — full embedded knowledge for MCP (AGENTS, rules, …)
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
type curatedDoc struct {
	id, title, section, file string
	keywords                 []string
}

// curatedDocEntries is the curated docs/*.md order for serve + MCP.
// Keywords are search-only (MCP docs list); they are not shown in the UI.
var curatedDocEntries = []curatedDoc{
	{"docs/index", "Getting started", "Start here", "index.md", nil},

	// Resources — Classic / Pro object cookbooks
	{"docs/policies", "Policies", "Resources", "policies.md",
		[]string{"policy", "policies", "policy_set", "policy_copy", "policy_migrate", "trigger", "classic"}},
	{"docs/smart-groups", "Smart computer groups", "Resources", "smart-groups.md",
		[]string{"smart", "smart_group", "smart_groups", "criteria", "group", "groups"}},
	{"docs/static-groups", "Static computer groups", "Resources", "static-groups.md",
		[]string{"static", "static_group", "static_groups", "membership", "assignments", "group", "groups"}},
	{"docs/profiles", "Configuration profiles", "Resources", "profiles.md",
		[]string{"profile", "profiles", "clone", "copy", "redeploy", "acs"}},
	{"docs/profile-payloads", "Profile payloads", "Resources", "profile-payloads.md",
		[]string{"profile", "profiles", "payload", "payloads", "mcx", "acs", "plist"}},
	{"docs/packages", "Packages / JCDS", "Resources", "packages.md",
		[]string{"package", "packages", "jcds", "ready", "download", "upload", "cloudTransferStatus"}},
	{"docs/scripts", "Scripts", "Resources", "scripts.md",
		[]string{"script", "scripts", "script_copy", "script_migrate", "scriptContents"}},
	{"docs/categories", "Categories", "Resources", "categories.md",
		[]string{"category", "categories", "category_copy", "category_migrate"}},
	{"docs/macapps", "Mac App Store apps", "Resources", "macapps.md",
		[]string{"macapp", "macapps", "macapplications", "mac-app-store", "mac app store", "app store"}},
	{"docs/apps", "Policy apps", "Resources", "apps.md",
		[]string{"app", "apps", "policy app", "policy apps", "self service", "app upgrade", "install-next", "cache policy"}},
	{"docs/computers", "Computers", "Resources", "computers.md",
		[]string{"computer", "computers", "inventory", "reboot", "lock", "erase", "mdm"}},
	{"docs/mdm", "MDM command history", "Resources", "mdm.md",
		[]string{"mdm", "mdm_commands", "command", "commands", "history"}},
	{"docs/api-roles", "API roles", "Resources", "api-roles.md",
		[]string{"role", "roles", "api role", "privileges", "privilege"}},
	{"docs/api-clients", "API clients", "Resources", "api-clients.md",
		[]string{"client", "clients", "integration", "integrations", "oauth", "api client"}},

	// Platform — Platform API Gateway cookbooks
	{"docs/blueprints", "Blueprints", "Platform", "blueprints.md",
		[]string{"blueprint", "blueprints", "deploy", "undeploy", "platform"}},
	{"docs/devices", "Platform devices", "Platform", "devices.md",
		[]string{"device", "devices", "platform", "device_action", "check-in", "restart"}},
	{"docs/device-groups", "Platform device groups", "Platform", "device-groups.md",
		[]string{"device_group", "device_groups", "device-group", "platform", "membership"}},
	{"docs/ddm", "DDM reporting", "Platform", "ddm.md",
		[]string{"ddm", "declaration", "declarations", "declarative"}},
	{"docs/benchmarks", "Compliance benchmarks", "Platform", "benchmarks.md",
		[]string{"benchmark", "benchmarks", "compliance", "baseline", "baselines"}},

	// BuddyScript — language guide (function index appended as builtins page)
	{"docs/buddyscript", "Overview", "BuddyScript", "buddyscript.md",
		[]string{"buddyscript", "starlark", "script", "main", "shell", "notebook", "ide"}},
	{"docs/buddyscript-syntax", "Syntax", "BuddyScript", "buddyscript-syntax.md",
		[]string{"syntax", "starlark", "python", "types", "load", "spec"}},
	{"docs/buddyscript-stdlib", "Stdlib", "BuddyScript", "buddyscript-stdlib.md",
		[]string{"stdlib", "load", "bump_release", "setup_app_classic", "release"}},

	// Guides — cross-cutting
	{"docs/reference-manual", "Reference manual", "Guides", "reference-manual.md",
		[]string{"config", "credentials", "cli", "starlark", "buddyscript", "install"}},
	{"docs/change-batches", "Change batches", "Guides", "change-batches.md",
		[]string{"change", "batch", "batches", "plan_change", "run_change", "step_", "release", "bump"}},
	{"docs/api-endpoints", "API endpoints", "Guides", "api-endpoints.md",
		[]string{"endpoint", "endpoints", "http", "api", "rsql", "jcds"}},
	{"docs/mcp", "MCP for AI tools", "Guides", "mcp.md",
		[]string{"mcp", "cursor", "claude", "buddyscript"}},
	{"docs/mcp-tools", "MCP tools scorecard", "Guides", "mcp-tools.md",
		[]string{"mcp", "scorecard", "pathfinding", "agent", "lean"}},
	{"docs/editor", "Editor / LSP", "Guides", "editor.md",
		[]string{"editor", "lsp", "vscode", "extension", "vsix"}},
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
		path := filepath.Join(docsRoot, e.file)
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
		path := "content/docs/" + e.file
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
		path := filepath.Join(docsRoot, e.file)
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

	addMD := func(id, title, path string, keywords []string) error {
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		pages = append(pages, Page{ID: id, Title: title, Markdown: string(b), Source: path, Keywords: keywords})
		return nil
	}

	extras := []struct {
		id, title, path string
		keywords        []string
	}{
		{"readme", "README", filepath.Join(repoRoot, "README.md"), []string{"readme", "install", "overview"}},
		{"agents", "AGENTS", filepath.Join(repoRoot, "AGENTS.md"), []string{"agents", "auth", "token", "secrets", "tenant"}},
	}
	for _, e := range extras {
		if _, err := os.Stat(e.path); err != nil {
			continue
		}
		if err := addMD(e.id, e.title, e.path, e.keywords); err != nil {
			return nil, err
		}
	}

	pages = append(pages, builtinsPage())
	pages = append(pages, loadExamplesDir(filepath.Join(repoRoot, "examples"))...)
	pages = append(pages, loadRulesDir(filepath.Join(repoRoot, ".cursor", "rules"))...)
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
		path := "content/docs/" + e.file
		body, err := read(path)
		if err != nil {
			return nil, fmt.Errorf("embedded %s: %w", path, err)
		}
		pages = append(pages, pageFromCurated(e, body, "embedded:"+path))
	}

	extras := []struct {
		id, title, path string
		keywords        []string
	}{
		{"readme", "README", "content/README.md", []string{"readme", "install", "overview"}},
		{"agents", "AGENTS", "content/AGENTS.md", []string{"agents", "auth", "token", "secrets", "tenant"}},
	}
	for _, e := range extras {
		body, err := read(e.path)
		if err != nil {
			return nil, fmt.Errorf("embedded %s: %w", e.path, err)
		}
		pages = append(pages, Page{ID: e.id, Title: e.title, Markdown: body, Source: "embedded:" + e.path, Keywords: e.keywords})
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

	ents, err = fs.ReadDir(docfs.Content, "content/rules")
	if err == nil {
		for _, ent := range ents {
			if ent.IsDir() {
				continue
			}
			name := ent.Name()
			path := "content/rules/" + name
			body, err := read(path)
			if err != nil {
				return nil, err
			}
			pages = append(pages, rulePage(name, body, "embedded:"+path))
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
func SearchHaystack(p Page) string {
	parts := []string{p.ID, p.Title, p.Section}
	parts = append(parts, p.Keywords...)
	return strings.ToLower(strings.Join(parts, " "))
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
