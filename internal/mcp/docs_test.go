package mcp

import (
	"strings"
	"testing"
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
		hay := strings.ToLower(e.ID + " " + e.Title + " " + e.Section)
		if !strings.Contains(hay, "builtin") {
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
