package mcp

import (
	"strings"
	"testing"

	"buddy/internal/docpages"
)

func TestDocsListAndGet(t *testing.T) {
	pages, err := loadAgentDocs()
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) == 0 {
		t.Fatal("expected embedded agent docs")
	}

	all := docsList(pages, "")
	if len(all) != len(pages) {
		t.Fatalf("list len = %d, want %d", len(all), len(pages))
	}

	filtered := docsList(pages, "builtin")
	if len(filtered) == 0 {
		t.Fatal("expected query=builtin matches")
	}
	for _, e := range filtered {
		p := docpages.ByID(pages, e.ID)
		if p == nil || !strings.Contains(docpages.SearchHaystack(*p), "builtin") {
			t.Fatalf("unexpected entry %#v", e)
		}
	}

	got, err := docsGet(pages, "agents")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "agents" || got.URI != "buddy://docs/agents" || got.Markdown == "" {
		t.Fatalf("%+v", got)
	}

	viaURI, err := docsGet(pages, "buddy://docs/builtins")
	if err != nil {
		t.Fatal(err)
	}
	if viaURI.ID != "builtins" || !strings.Contains(viaURI.Markdown, "policy") {
		t.Fatalf("%+v", viaURI)
	}

	legacy, err := docsGet(pages, "buddy://docs/mcp.md")
	if err != nil {
		t.Fatal(err)
	}
	if legacy.ID != "docs/mcp" {
		t.Fatalf("legacy id = %q", legacy.ID)
	}

	if _, err := docsGet(pages, ""); err == nil {
		t.Fatal("expected error for empty id")
	}
	if _, err := docsGet(pages, "no-such-page"); err == nil {
		t.Fatal("expected error for unknown id")
	}
}

func TestDocsListKeywordNouns(t *testing.T) {
	pages, err := loadAgentDocs()
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		query string
		want  string
	}{
		{"policy", "docs/policies"},
		{"jcds", "docs/packages"},
		{"blueprint", "docs/blueprints"},
		{"static", "docs/static-groups"},
		{"smart", "docs/smart-groups"},
		{"ddm", "docs/ddm"},
	}
	for _, tc := range cases {
		hits := docsList(pages, tc.query)
		found := false
		for _, e := range hits {
			if e.ID == tc.want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("query %q: expected id %q in %v", tc.query, tc.want, hitIDs(hits))
		}
	}
}

func hitIDs(entries []docsCatalogEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.ID
	}
	return out
}
