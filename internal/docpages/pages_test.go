package docpages

import (
	"strings"
	"testing"
)

func TestLoadHumanEmbedded(t *testing.T) {
	res, err := Load(LoadOptions{Catalog: CatalogHuman})
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != "embedded" {
		t.Fatalf("source=%q", res.Source)
	}
	for _, id := range []string{
		"docs/index", "docs/reference-manual", "docs/api-endpoints",
		"docs/mcp", "builtins", "examples/bump-release.star",
	} {
		if ByID(res.Pages, id) == nil {
			t.Errorf("missing human page %q", id)
		}
	}
	for _, id := range []string{"agents", "readme", "rules/buddy-cli.mdc", "docs/builtins-v3", "help/root"} {
		if ByID(res.Pages, id) != nil {
			t.Errorf("agent-only page %q should not appear in human catalog", id)
		}
	}
	secs := NavSections(res.Pages)
	if len(secs) < 3 {
		t.Fatalf("sections=%v", secs)
	}
}

func TestLoadAgentEmbedded(t *testing.T) {
	res, err := Load(LoadOptions{Catalog: CatalogAgent})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{
		"docs/index", "docs/builtins-v3", "readme", "agents", "builtins",
		"rules/buddy-cli.mdc", "rules/buddy-auth-secrets.mdc", "rules/buddy-builtins.mdc",
		"help/root", "examples/bump-release.star",
	} {
		if ByID(res.Pages, id) == nil {
			t.Errorf("missing agent page %q", id)
		}
	}
	a := ByID(res.Pages, "agents")
	if a == nil || !strings.Contains(a.Markdown, "Token caching") {
		t.Fatal("agents page missing expected content")
	}
}

func TestURI(t *testing.T) {
	if got := URI(Page{ID: "builtins"}); got != "buddy://docs/builtins" {
		t.Fatalf("got %q", got)
	}
}

func TestStripFrontMatter(t *testing.T) {
	in := "---\nalwaysApply: true\n---\n\n# Title\n"
	got := stripFrontMatter(in)
	if !strings.HasPrefix(got, "# Title") {
		t.Fatalf("got %q", got)
	}
}
