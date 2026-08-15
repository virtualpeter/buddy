package serve

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

func TestDocsNavGroups(t *testing.T) {
	pages := []DocPage{
		{ID: "docs/index", Title: "Getting started", Section: "Start here"},
		{ID: "docs/policies", Title: "Policies", Section: "Resources"},
		{ID: "docs/packages", Title: "Packages", Section: "Resources"},
		{ID: "docs/blueprints", Title: "Blueprints", Section: "Platform"},
		{ID: "orphan", Title: "Orphan"},
	}
	got := docsNavGroups(pages)
	if len(got) != 3 {
		t.Fatalf("groups=%d want 3 (Platform nested): %+v", len(got), got)
	}
	if got[0].Name != "Start here" || len(got[0].Pages) != 1 {
		t.Fatalf("first group %+v", got[0])
	}
	if got[1].Name != "Resources" || len(got[1].Pages) != 2 {
		t.Fatalf("resources %+v", got[1])
	}
	if len(got[1].Children) != 1 || got[1].Children[0].Name != "Platform" || len(got[1].Children[0].Pages) != 1 {
		t.Fatalf("Platform should nest under Resources: %+v", got[1].Children)
	}
	if got[1].Count() != 3 {
		t.Fatalf("Resources count=%d want 3 (includes Platform)", got[1].Count())
	}
	if got[2].Name != "Docs" || got[2].Pages[0].ID != "orphan" {
		t.Fatalf("empty section should become Docs: %+v", got[2])
	}

	applyNavOpen(got, "docs/blueprints", "Platform")
	if !got[1].Open || !got[1].Children[0].Open {
		t.Fatal("active Platform page should open Resources and Platform")
	}
}

func TestDocsNavCollapsibleHTML(t *testing.T) {
	var buf bytes.Buffer
	resources := docsNavGroup{
		Name:  "Resources",
		Pages: []DocPage{{ID: "docs/policies", Title: "Policies"}},
		Children: []docsNavGroup{{
			Name:  "Platform",
			Pages: []DocPage{{ID: "docs/blueprints", Title: "Blueprints"}},
			Open:  true,
		}},
		Open: true,
	}
	err := docsTmpl.Execute(&buf, docsView{
		Groups: []docsNavGroup{
			{Name: "Start here", Pages: []DocPage{{ID: "docs/index", Title: "Getting started"}}, Open: true},
			resources,
		},
		ActiveID:      "docs/blueprints",
		ActiveSection: "Platform",
		Title:         "Blueprints",
	})
	if err != nil {
		t.Fatal(err)
	}
	html := buf.String()
	for _, want := range []string{
		`class="nav-group"`,
		`data-sec="Start here"`,
		`data-sec="Resources"`,
		`data-sec="Platform"`,
		`href="/docs/docs/blueprints"`,
		`class="active"`,
		`buddy-serve-docs-nav`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("docs HTML missing %q", want)
		}
	}
	// Active section is open on first paint; unused long sections stay closed.
	reOpen := regexp.MustCompile(`(?s)<details class="nav-group" data-sec="Resources"[^>]* open`)
	if !reOpen.MatchString(html) {
		t.Fatal("parent Resources group should be open when Platform is active")
	}
	rePlat := regexp.MustCompile(`(?s)<details class="nav-group" data-sec="Platform"[^>]* open`)
	if !rePlat.MatchString(html) {
		t.Fatal("active Platform group should be open")
	}
	if strings.Contains(html, `<div class="section">Resources</div>`) {
		t.Fatal("flat section headings should be replaced by details summaries")
	}
}

func TestLinkBuiltinCodeSpans(t *testing.T) {
	in := strings.Join([]string{
		"| List | `policies(keywords=…)` / `policies(ids=True)` | `buddy policy list` |",
		"| Mutate | `policy_set*` / `policy_put_xml` |",
		"| Report | `ddm_*` builtins |",
		"Note: `true` and `name=` stay plain; already [`policy`](/docs/builtins#policy).",
		"",
		"```python",
		"p = policy(trigger=\"x\")",
		"```",
	}, "\n")
	got := linkBuiltinCodeSpans(in)
	for _, want := range []string{
		"[`policies(keywords=…)`](/docs/builtins#policies)",
		"[`policies(ids=True)`](/docs/builtins#policies)",
		"[`policy_set*`](/docs/builtins#policy_set)",
		"[`policy_put_xml`](/docs/builtins#policy_put_xml)",
		"[`ddm_*`](/docs/builtins#",
		"[`policy`](/docs/builtins#policy)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "[`true`]") || strings.Contains(got, "[`name=`]") {
		t.Fatalf("should not link non-builtin code spans:\n%s", got)
	}
	if strings.Contains(got, "[`policy(trigger") {
		t.Fatalf("should not rewrite fenced examples:\n%s", got)
	}
	if strings.Count(got, "[`policy`](/docs/builtins#policy)") != 1 {
		t.Fatalf("should not double-link existing anchors:\n%s", got)
	}
}

func TestPrepareDocMarkdownSkipsIndexPages(t *testing.T) {
	md := "See `policies(keywords=…)` and `buddy policy list`.\n"
	for _, id := range []string{"builtins", "cli"} {
		got := prepareDocMarkdown(md, nil, id)
		if strings.Contains(got, "/docs/builtins#") || strings.Contains(got, "/docs/cli#") {
			t.Fatalf("%s should not self-link:\n%s", id, got)
		}
	}
	got2 := prepareDocMarkdown(md, nil, "docs/policies")
	if !strings.Contains(got2, "[`policies(keywords=…)`](/docs/builtins#policies)") {
		t.Fatalf("resource pages should link builtins:\n%s", got2)
	}
	if !strings.Contains(got2, "[`buddy policy list`](/docs/cli#policy-list)") {
		t.Fatalf("resource pages should link CLI commands:\n%s", got2)
	}
}

func TestLinkCLICodeSpans(t *testing.T) {
	in := "| List | `buddy policy list` | `buddy group list smart` | `buddy computer reboot|lock|…` |"
	got := linkCLICodeSpans(in)
	for _, want := range []string{
		"[`buddy policy list`](/docs/cli#policy-list)",
		"[`buddy group list smart`](/docs/cli#group)",
		"[`buddy computer reboot|lock|…`](/docs/cli#computer)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

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

func TestExtractTOCFnNameNotFullSignature(t *testing.T) {
	md := `<h3 id="policy_set_ss"><code class="fn-sig"><b class="fn-name">policy_set_ss</b>(policy_id, version?)</code></h3>`
	toc := extractTOC(md)
	if len(toc) != 1 || toc[0].ID != "policy_set_ss" || toc[0].Text != "policy_set_ss" {
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
	if pageByID(pages, "cli") == nil {
		t.Fatal("missing cli")
	}
	if pageByID(pages, "docs/change-batches") == nil {
		t.Fatal("missing docs/change-batches")
	}
	if pageByID(pages, "examples/README.md") == nil {
		t.Fatal("missing examples/README.md")
	}
	builtins := pageByID(pages, "builtins")
	if !strings.Contains(builtins.Markdown, `<h3 id="policy">`) {
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
		{ID: "docs/mcp", Title: "MCP"},
		{ID: "examples/change-batch-santa.buddy", Title: "santa"},
		{ID: "examples/README.md", Title: "Examples index"},
	}
	in := `[a](change-batches.md) [b](mcp.md) [c](../examples/change-batch-santa.buddy) [d](../examples/README.md) [e](https://example.com) [f](#anchor) [g](../README.md#configuration) [h](../scripts/integration_test.buddy)`
	got := rewriteDocLinks(in, pages)
	for _, want := range []string{
		"](/docs/docs/change-batches)",
		"](/docs/docs/mcp)",
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
		rewritten := prepareDocMarkdown(p.Markdown, pages, p.ID)
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
