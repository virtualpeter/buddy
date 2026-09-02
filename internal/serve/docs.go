package serve

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"buddy/internal/help"
)

// DocPage is a navigable documentation page.
type DocPage struct {
	ID       string // e.g. "docs/reference-manual"
	Title    string
	Path     string // filesystem path if from disk; empty for virtual
	Markdown string
	Plain    bool // true = render as <pre> (CLI help)
}

func resolveDocsRoot(explicit string) (string, error) {
	if explicit != "" {
		fi, err := os.Stat(explicit)
		if err != nil {
			return "", err
		}
		if !fi.IsDir() {
			return "", fmt.Errorf("docs root is not a directory: %s", explicit)
		}
		return filepath.Clean(explicit), nil
	}
	if env := os.Getenv("BUDDY_DOCS_ROOT"); env != "" {
		return resolveDocsRoot(env)
	}
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
	return "", fmt.Errorf("could not find docs/ (set --docs-root or BUDDY_DOCS_ROOT, or run from the buddy repo)")
}

func loadDocPages(docsRoot string) ([]DocPage, error) {
	repoRoot := filepath.Dir(docsRoot)
	var pages []DocPage

	addMD := func(id, title, path string) error {
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		pages = append(pages, DocPage{ID: id, Title: title, Path: path, Markdown: string(b)})
		return nil
	}

	entries := []struct{ id, title, rel string }{
		{"docs/index", "Documentation index", filepath.Join(docsRoot, "index.md")},
		{"docs/reference-manual", "Reference manual", filepath.Join(docsRoot, "reference-manual.md")},
		{"docs/api-endpoints", "API endpoints", filepath.Join(docsRoot, "api-endpoints.md")},
		{"readme", "README", filepath.Join(repoRoot, "README.md")},
		{"agents", "AGENTS", filepath.Join(repoRoot, "AGENTS.md")},
	}
	for _, e := range entries {
		if _, err := os.Stat(e.rel); err != nil {
			continue
		}
		if err := addMD(e.id, e.title, e.rel); err != nil {
			return nil, err
		}
	}

	// First-class HTML browseable builtins (not a preformatted dump).
	pages = append(pages, DocPage{
		ID:       "builtins",
		Title:    "Starlark builtins",
		Markdown: help.BuiltinsMarkdown(),
	})

	examplesDir := filepath.Join(repoRoot, "examples")
	if ents, err := os.ReadDir(examplesDir); err == nil {
		for _, ent := range ents {
			name := ent.Name()
			if ent.IsDir() || (!strings.HasSuffix(name, ".md") && !strings.HasSuffix(name, ".star")) {
				continue
			}
			path := filepath.Join(examplesDir, name)
			b, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			id := "examples/" + name
			title := "Example: " + name
			if strings.HasSuffix(name, ".star") {
				pages = append(pages, DocPage{
					ID: id, Title: title, Path: path,
					Markdown: "```python\n" + string(b) + "\n```",
				})
			} else {
				pages = append(pages, DocPage{ID: id, Title: title, Path: path, Markdown: string(b)})
			}
		}
	}

	for _, p := range help.CLIPages() {
		if p.ID == "builtins" {
			// Prefer the structured /docs/builtins page; skip raw dump.
			continue
		}
		pages = append(pages, DocPage{
			ID:       "help/" + p.ID,
			Title:    "Help: " + p.Title,
			Markdown: "```\n" + p.Body + "\n```",
			Plain:    true,
		})
	}
	return pages, nil
}

func pageByID(pages []DocPage, id string) *DocPage {
	for i := range pages {
		if pages[i].ID == id {
			return &pages[i]
		}
	}
	return nil
}
