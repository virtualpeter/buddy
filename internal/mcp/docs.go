package mcp

import (
	"fmt"
	"strings"

	"buddy/internal/docpages"
)

// docsCatalogEntry is one row from docs(action=list).
type docsCatalogEntry struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Section string `json:"section,omitempty"`
	URI     string `json:"uri"`
}

// docsPageValue is the payload for docs(action=get).
type docsPageValue struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Section  string `json:"section,omitempty"`
	URI      string `json:"uri"`
	Markdown string `json:"markdown"`
}

func loadAgentDocs() ([]docpages.Page, error) {
	res, err := docpages.Load(docpages.LoadOptions{Catalog: docpages.CatalogAgent})
	if err != nil {
		return nil, err
	}
	return res.Pages, nil
}

func docsList(pages []docpages.Page, query string) []docsCatalogEntry {
	q := strings.ToLower(strings.TrimSpace(query))
	out := make([]docsCatalogEntry, 0, len(pages))
	for _, p := range pages {
		if q != "" && !strings.Contains(docpages.SearchHaystack(p), q) {
			continue
		}
		out = append(out, docsCatalogEntry{
			ID:      p.ID,
			Title:   p.Title,
			Section: p.Section,
			URI:     docpages.URI(p),
		})
	}
	return out
}

func docsGet(pages []docpages.Page, idOrURI string) (*docsPageValue, error) {
	id := strings.TrimSpace(idOrURI)
	if id == "" {
		return nil, fmt.Errorf("id is required for action=get")
	}
	id = strings.TrimPrefix(id, "buddy://docs/")
	// Legacy aliases (former disk filenames).
	switch id {
	case "index.md":
		id = "docs/index"
	case "reference-manual.md":
		id = "docs/reference-manual"
	case "change-batches.md":
		id = "docs/change-batches"
	case "api-endpoints.md":
		id = "docs/api-endpoints"
	case "mcp.md":
		id = "docs/mcp"
	}

	p := docpages.ByID(pages, id)
	if p == nil {
		return nil, fmt.Errorf("unknown docs id %q (use docs action=list)", idOrURI)
	}
	return &docsPageValue{
		ID:       p.ID,
		Title:    p.Title,
		Section:  p.Section,
		URI:      docpages.URI(*p),
		Markdown: p.Markdown,
	}, nil
}
