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
		"docs/index", "docs/policies", "docs/smart-groups", "docs/static-groups",
		"docs/profiles", "docs/packages", "docs/scripts", "docs/categories",
		"docs/computers", "docs/mdm", "docs/api-roles", "docs/api-clients",
		"docs/blueprints", "docs/devices", "docs/device-groups", "docs/ddm", "docs/benchmarks",
		"docs/reference-manual", "docs/change-batches", "docs/api-endpoints",
		"docs/mcp", "docs/mcp-tools", "docs/profile-payloads", "docs/editor",
		"docs/buddyscript", "docs/buddyscript-syntax", "docs/buddyscript-stdlib", "builtins",
		"examples/bump-release.buddy", "examples/change-batch-santa.buddy",
		"examples/setup-app-classic.buddy",
		"examples/README.md", "examples/mcp.json",
	} {
		if ByID(res.Pages, id) == nil {
			t.Errorf("missing human page %q", id)
		}
	}
	for _, id := range []string{"agents", "readme", "rules/buddy-cli.mdc", "help/root"} {
		if ByID(res.Pages, id) != nil {
			t.Errorf("agent-only page %q should not appear in human catalog", id)
		}
	}
	// Every curated curatedDocEntries file must be present under docs/.
	for _, e := range curatedDocEntries {
		if ByID(res.Pages, e.id) == nil {
			t.Errorf("curatedDocEntries id %q missing from catalog", e.id)
		}
		p := ByID(res.Pages, e.id)
		if p != nil && len(e.keywords) > 0 && len(p.Keywords) == 0 {
			t.Errorf("page %q missing keywords", e.id)
		}
	}
	secs := NavSections(res.Pages)
	wantSec := map[string]bool{"Start here": true, "Resources": true, "Platform": true, "BuddyScript": true, "Guides": true}
	for s := range wantSec {
		found := false
		for _, got := range secs {
			if got == s {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing nav section %q in %v", s, secs)
		}
	}
	bi := ByID(res.Pages, "builtins")
	if bi == nil || bi.Title != "Function index" || bi.Section != "BuddyScript" {
		t.Fatalf("builtins page want Function index / BuddyScript, got %+v", bi)
	}
}

func TestLoadAgentEmbedded(t *testing.T) {
	res, err := Load(LoadOptions{Catalog: CatalogAgent})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{
		"docs/index", "docs/policies", "docs/packages", "docs/blueprints",
		"docs/change-batches", "docs/mcp", "docs/mcp-tools", "docs/profile-payloads", "docs/editor",
		"readme", "agents", "builtins",
		"rules/buddy-cli.mdc", "rules/buddy-auth-secrets.mdc", "rules/buddy-builtins.mdc",
		"help/root", "examples/bump-release.buddy",
	} {
		if ByID(res.Pages, id) == nil {
			t.Errorf("missing agent page %q", id)
		}
	}
	a := ByID(res.Pages, "agents")
	if a == nil || !strings.Contains(a.Markdown, "Token caching") {
		t.Fatal("agents page missing expected content")
	}
	pol := ByID(res.Pages, "docs/policies")
	if pol == nil || !strings.Contains(SearchHaystack(*pol), "policy") {
		t.Fatal("docs/policies should match keyword policy")
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
