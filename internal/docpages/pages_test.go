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
		"docs/index", "docs/getting-started", "docs/releases", "docs/policies", "docs/smart-groups", "docs/static-groups",
		"docs/profiles", "docs/packages", "docs/icons", "docs/scripts", "docs/categories",
		"docs/extension-attributes", "docs/mobile-extension-attributes",
		"docs/computers", "docs/computer-searches", "docs/mdm", "docs/mobile-devices", "docs/mobile-searches", "docs/mobile-groups", "docs/mobile-profiles",
		"docs/api-roles", "docs/api-clients",
		"docs/blueprints", "docs/devices", "docs/device-groups", "docs/ddm", "docs/benchmarks",
		"docs/licenses",
		"docs/auth", "docs/reference-manual", "docs/change-batches", "docs/api-endpoints",
		"docs/mcp", "docs/mcp-tools", "docs/profile-payloads", "docs/editor",
		"docs/apps", "docs/macapps",
		"docs/buddyscript", "docs/buddyscript-syntax", "docs/buddyscript-stdlib", "builtins", "cli",
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
	if NavParent("Platform") != "Resources" {
		t.Fatal("Platform should nest under Resources in the human docs nav")
	}
	if NavParent("Resources") != "" || NavParent("Guides") != "" {
		t.Fatal("only Platform should have a nav parent")
	}
	secs := NavSections(res.Pages)
	wantSec := map[string]bool{"Start here": true, "Resources": true, "Platform": true, "BuddyScript": true, "CLI": true, "Guides": true}
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
	cl := ByID(res.Pages, "cli")
	if cl == nil || cl.Title != "Command index" || cl.Section != "CLI" {
		t.Fatalf("cli page want Command index / CLI, got %+v", cl)
	}
	if !strings.Contains(cl.Markdown, `id="policy"`) || !strings.Contains(cl.Markdown, `id="auth"`) {
		t.Fatal("cli page missing command anchors")
	}
}

func TestLoadAgentEmbedded(t *testing.T) {
	res, err := Load(LoadOptions{Catalog: CatalogAgent})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{
		"docs/index", "docs/getting-started", "docs/releases", "docs/policies", "docs/packages", "docs/blueprints",
		"docs/change-batches", "docs/mcp", "docs/mcp-tools", "docs/profile-payloads", "docs/editor",
		"docs/apps", "readme", "builtins", "cli",
		"help/root", "examples/bump-release.buddy",
	} {
		if ByID(res.Pages, id) == nil {
			t.Errorf("missing agent page %q", id)
		}
	}
	for _, id := range []string{"agents", "rules/buddy-cli.mdc", "rules/buddy-auth-secrets.mdc", "rules/buddy-builtins.mdc"} {
		if ByID(res.Pages, id) != nil {
			t.Errorf("agent catalog must not include %q", id)
		}
	}
	apps := ByID(res.Pages, "docs/apps")
	if apps == nil || !strings.Contains(apps.Markdown, "## Agent rules") {
		t.Fatal("docs/apps agent page missing Agent rules")
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

func TestFilterByQueryFuzzy(t *testing.T) {
	res, err := Load(LoadOptions{Catalog: CatalogAgent})
	if err != nil {
		t.Fatal(err)
	}
	pages := res.Pages

	all := FilterByQuery(pages, "")
	if len(all) != len(pages) {
		t.Fatalf("empty query len=%d want %d", len(all), len(pages))
	}

	for _, q := range []string{
		"smart_groups", "smart-groups", "smart groups", "smart group", "Smart_Groups", "smartgroups",
	} {
		hits := FilterByQuery(pages, q)
		found := false
		for _, p := range hits {
			if p.ID == "docs/smart-groups" {
				found = true
				break
			}
		}
		if !found {
			ids := make([]string, len(hits))
			for i, p := range hits {
				ids[i] = p.ID
			}
			t.Errorf("query %q: expected docs/smart-groups in %v", q, ids)
		}
	}

	polHits := FilterByQuery(pages, "policy_set")
	foundPol := false
	for _, p := range polHits {
		if p.ID == "docs/policies" {
			foundPol = true
			break
		}
	}
	if !foundPol {
		t.Fatal("policy_set should match docs/policies")
	}

	// AND preference: smart groups must not OR-fallback to static-only pages.
	andHits := FilterByQuery(pages, "smart groups")
	for _, p := range andHits {
		if p.ID == "docs/static-groups" {
			t.Fatalf("smart groups AND match must not include static-groups")
		}
	}
}

func TestStripFrontMatter(t *testing.T) {
	in := "---\nalwaysApply: true\n---\n\n# Title\n"
	got := stripFrontMatter(in)
	if !strings.HasPrefix(got, "# Title") {
		t.Fatalf("got %q", got)
	}
}
