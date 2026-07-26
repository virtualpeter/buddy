// Package docpages loads buddy documentation pages for serve and MCP.
//
// Two catalogs share the same sources:
//   - CatalogHuman — curated pages for the buddy serve HTML UI
//   - CatalogAgent — full embedded knowledge for MCP (AGENTS, rules, migration notes, …)
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
	Section  string // nav grouping for the human UI (Overview, Guides, …)
	Markdown string
	Plain    bool   // true = render as <pre> (CLI help)
	Source   string // "embedded:…" or filesystem path
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

// humanDocEntries is the curated serve UI order (id, title, section, filename under docs/).
var humanDocEntries = []struct {
	id, title, section, file string
}{
	{"docs/index", "Overview", "Start here", "index.md"},
	{"docs/reference-manual", "Reference manual", "Guides", "reference-manual.md"},
	{"docs/change-batches", "Change batches", "Guides", "change-batches.md"},
	{"docs/api-endpoints", "API endpoints", "Guides", "api-endpoints.md"},
	{"docs/mcp", "MCP for AI tools", "Guides", "mcp.md"},
	{"docs/builtins-v3", "Builtins v3 rename map", "Guides", "builtins-v3.md"},
}

func loadHumanFromDisk(docsRoot string) ([]Page, error) {
	repoRoot := filepath.Dir(docsRoot)
	var pages []Page
	for _, e := range humanDocEntries {
		path := filepath.Join(docsRoot, e.file)
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		pages = append(pages, Page{
			ID: e.id, Title: e.title, Section: e.section,
			Markdown: string(b), Source: path,
		})
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
	for _, e := range humanDocEntries {
		path := "content/docs/" + e.file
		body, err := fs.ReadFile(docfs.Content, path)
		if err != nil {
			return nil, fmt.Errorf("embedded %s: %w", path, err)
		}
		pages = append(pages, Page{
			ID: e.id, Title: e.title, Section: e.section,
			Markdown: string(body), Source: "embedded:" + path,
		})
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

func loadAgentFromDisk(docsRoot string) ([]Page, error) {
	repoRoot := filepath.Dir(docsRoot)
	var pages []Page

	addMD := func(id, title, path string) error {
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		pages = append(pages, Page{ID: id, Title: title, Markdown: string(b), Source: path})
		return nil
	}

	entries := []struct{ id, title, path string }{
		{"docs/index", "Documentation index", filepath.Join(docsRoot, "index.md")},
		{"docs/reference-manual", "Reference manual", filepath.Join(docsRoot, "reference-manual.md")},
		{"docs/change-batches", "Change batches", filepath.Join(docsRoot, "change-batches.md")},
		{"docs/api-endpoints", "API endpoints", filepath.Join(docsRoot, "api-endpoints.md")},
		{"docs/builtins-v3", "Builtins v3 rename map", filepath.Join(docsRoot, "builtins-v3.md")},
		{"docs/mcp", "MCP server", filepath.Join(docsRoot, "mcp.md")},
		{"readme", "README", filepath.Join(repoRoot, "README.md")},
		{"agents", "AGENTS", filepath.Join(repoRoot, "AGENTS.md")},
	}
	for _, e := range entries {
		if _, err := os.Stat(e.path); err != nil {
			continue
		}
		if err := addMD(e.id, e.title, e.path); err != nil {
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

	entries := []struct{ id, title, path string }{
		{"docs/index", "Documentation index", "content/docs/index.md"},
		{"docs/reference-manual", "Reference manual", "content/docs/reference-manual.md"},
		{"docs/change-batches", "Change batches", "content/docs/change-batches.md"},
		{"docs/api-endpoints", "API endpoints", "content/docs/api-endpoints.md"},
		{"docs/builtins-v3", "Builtins v3 rename map", "content/docs/builtins-v3.md"},
		{"docs/mcp", "MCP server", "content/docs/mcp.md"},
		{"readme", "README", "content/README.md"},
		{"agents", "AGENTS", "content/AGENTS.md"},
	}
	for _, e := range entries {
		body, err := read(e.path)
		if err != nil {
			return nil, fmt.Errorf("embedded %s: %w", e.path, err)
		}
		pages = append(pages, Page{ID: e.id, Title: e.title, Markdown: body, Source: "embedded:" + e.path})
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
		ID:       "builtins",
		Title:    "Starlark builtins",
		Section:  "Starlark",
		Markdown: help.BuiltinsMarkdown(),
		Source:   "builtin:help.BuiltinsMarkdown",
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
