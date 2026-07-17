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

func TestResolveDocsRoot(t *testing.T) {
	root, err := resolveDocsRoot("")
	if err != nil {
		t.Skip("not in repo:", err)
	}
	if !strings.HasSuffix(root, "docs") {
		t.Fatalf("root=%q", root)
	}
	pages, err := loadDocPages(root)
	if err != nil {
		t.Fatal(err)
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
}
