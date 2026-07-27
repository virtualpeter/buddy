package serve

import (
	"regexp"
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

func TestSearchBuiltinsFragment(t *testing.T) {
	pages := []DocPage{
		{ID: "builtins", Title: "Builtins", Markdown: `<h3 id="policies"><code>policies</code></h3>` + "\n" + `<h3 id="policy_set"><code>policy_set</code></h3>`},
	}
	hits := searchPages(pages, "policy", 10)
	if len(hits) < 1 {
		t.Fatalf("hits=%+v", hits)
	}
	found := false
	for _, h := range hits {
		if h.Fragment == "policy_set" && h.PageID == "builtins" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected policy_set fragment in %+v", hits)
	}
}

func TestExtractTOCHTMLHeadings(t *testing.T) {
	md := "# Title\n\n<h2 id=\"section-policy\">POLICY</h2>\n\n<h3 id=\"policies\"><code>policies</code></h3>\n"
	toc := extractTOC(md)
	ids := map[string]bool{}
	for _, e := range toc {
		ids[e.ID] = true
	}
	if !ids["section-policy"] || !ids["policies"] {
		t.Fatalf("toc=%+v", toc)
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
	if pageByID(pages, "docs/change-batches") == nil {
		t.Fatal("missing docs/change-batches")
	}
	if pageByID(pages, "docs/builtins-v3") == nil {
		t.Fatal("missing docs/builtins-v3")
	}
	if pageByID(pages, "examples/README.md") == nil {
		t.Fatal("missing examples/README.md")
	}
	builtins := pageByID(pages, "builtins")
	if !strings.Contains(builtins.Markdown, `<h3 id="policy"><code>policy</code></h3>`) {
		t.Fatal("builtins page not structured")
	}
	if pageByID(pages, "agents") != nil {
		t.Fatal("AGENTS must not appear in the human docs UI")
	}
	if pageByID(pages, "rules/buddy-cli.mdc") != nil {
		t.Fatal("Cursor rules must not appear in the human docs UI")
	}
}

func TestRewriteDocLinks(t *testing.T) {
	pages := []DocPage{
		{ID: "docs/change-batches", Title: "Change batches"},
		{ID: "docs/builtins-v3", Title: "v3"},
		{ID: "examples/change-batch-santa.buddy", Title: "santa"},
		{ID: "examples/README.md", Title: "Examples index"},
	}
	in := `[a](change-batches.md) [b](builtins-v3.md) [c](../examples/change-batch-santa.buddy) [d](../examples/README.md) [e](https://example.com) [f](#anchor) [g](../README.md#configuration) [h](../scripts/integration_test.buddy)`
	got := rewriteDocLinks(in, pages)
	for _, want := range []string{
		"](/docs/docs/change-batches)",
		"](/docs/docs/builtins-v3)",
		"](/docs/examples/change-batch-santa.buddy)",
		"](/docs/examples/README.md)",
		"](https://example.com)",
		"](#anchor)",
		"`README.md#configuration`",
		"`scripts/integration_test.buddy`",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "/docs/scripts/") || strings.Contains(got, "](../README.md") {
		t.Fatalf("should not leave clickable ../ repo links:\n%s", got)
	}
}

func TestHumanDocRelativeLinksResolve(t *testing.T) {
	pages, _, err := loadDocPages("")
	if err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{}
	for _, p := range pages {
		known[p.ID] = true
	}
	linkRE := regexp.MustCompile(`\]\(([^)]+)\)`)
	var broken []string
	for _, p := range pages {
		rewritten := rewriteDocLinks(p.Markdown, pages)
		for _, m := range linkRE.FindAllStringSubmatch(rewritten, -1) {
			dest := m[1]
			if i := strings.IndexByte(dest, '#'); i >= 0 {
				dest = dest[:i]
			}
			if dest == "" || strings.HasPrefix(dest, "http") || strings.HasPrefix(dest, "mailto:") {
				continue
			}
			if strings.HasPrefix(dest, "/docs/") {
				id := strings.TrimPrefix(dest, "/docs/")
				if !known[id] {
					broken = append(broken, p.ID+" -> "+dest+" (unknown page)")
				}
				continue
			}
			// Repo-root escapes must be flattened (not left as ../ clickable links).
			if strings.HasPrefix(dest, "../") {
				broken = append(broken, p.ID+" dangling ../ link "+dest)
				continue
			}
			if strings.HasSuffix(dest, ".md") || strings.HasSuffix(dest, ".buddy") ||
				strings.HasSuffix(dest, ".json") || strings.Contains(dest, "examples/") {
				broken = append(broken, p.ID+" unresolved "+dest)
			}
		}
	}
	if len(broken) > 0 {
		t.Fatalf("unresolved docs UI links:\n  %s", strings.Join(broken, "\n  "))
	}
}
