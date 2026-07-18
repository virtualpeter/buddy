package serve

import (
	"strings"
	"testing"
)

func TestSearchPages(t *testing.T) {
	pages := []DocPage{
		{ID: "a", Title: "Alpha", Markdown: "hello world pagination"},
		{ID: "b", Title: "Beta", Markdown: "nothing here"},
	}
	hits := searchPages(pages, "pagination", 10)
	if len(hits) != 1 || hits[0].PageID != "a" {
		t.Fatalf("hits=%+v", hits)
	}
}

func TestSlugify(t *testing.T) {
	if got := slugify("Hello World!"); got != "hello-world" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractTOC(t *testing.T) {
	md := "# Title\n\n## Section One\n\ntext\n"
	toc := extractTOC(md)
	if len(toc) < 2 {
		t.Fatalf("toc=%+v", toc)
	}
	if toc[1].Text != "Section One" {
		t.Fatalf("toc[1]=%+v", toc[1])
	}
}

func TestLoadDocPages(t *testing.T) {
	pages, source, err := loadDocPages("")
	if err != nil {
		t.Fatal(err)
	}
	if source == "" {
		t.Fatal("empty docs source")
	}
	if pageByID(pages, "docs/index") == nil {
		t.Fatal("missing docs/index")
	}
	if pageByID(pages, "builtins") == nil {
		t.Fatal("missing builtins")
	}
	builtins := pageByID(pages, "builtins")
	if !strings.Contains(builtins.Markdown, "### `policy`") {
		t.Fatal("builtins page not structured")
	}
	if pageByID(pages, "agents") != nil {
		t.Fatal("AGENTS must not appear in the human docs UI")
	}
	if pageByID(pages, "rules/buddy-cli.mdc") != nil {
		t.Fatal("Cursor rules must not appear in the human docs UI")
	}
}
