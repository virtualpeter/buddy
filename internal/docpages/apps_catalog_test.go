package docpages_test

import (
	"strings"
	"testing"

	"buddy/internal/docpages"
)

var generatedDocIDs = []string{
	"docs/apps", "docs/policies", "docs/smart-groups", "docs/static-groups",
	"docs/profiles", "docs/profile-payloads", "docs/packages", "docs/icons", "docs/scripts",
	"docs/categories", "docs/macapps", "docs/computers", "docs/mdm",
	"docs/mobile-devices", "docs/mobile-groups", "docs/mobile-profiles",
	"docs/api-roles", "docs/api-clients", "docs/blueprints", "docs/devices",
	"docs/device-groups", "docs/ddm", "docs/benchmarks",
}

func TestGeneratedResourcesHumanVsAgentTone(t *testing.T) {
	human, err := docpages.Load(docpages.LoadOptions{Catalog: docpages.CatalogHuman, PreferDisk: true})
	if err != nil {
		t.Fatal(err)
	}
	agent, err := docpages.Load(docpages.LoadOptions{Catalog: docpages.CatalogAgent, PreferDisk: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range generatedDocIDs {
		h := docpages.ByID(human.Pages, id)
		a := docpages.ByID(agent.Pages, id)
		if h == nil || a == nil {
			t.Errorf("%s missing from human or agent catalog", id)
			continue
		}
		if h.Markdown == a.Markdown {
			t.Errorf("%s: human and agent bodies identical", id)
		}
		if strings.Contains(h.Markdown, "## Agent rules") {
			t.Errorf("%s: human page contains Agent rules", id)
		}
		if !strings.Contains(a.Markdown, "## Agent rules") {
			t.Errorf("%s: agent page missing Agent rules", id)
		}
		if !strings.Contains(a.Markdown, "audience: agent") {
			t.Errorf("%s: agent page missing YAML frontmatter", id)
		}
		if !strings.Contains(a.Markdown, "```json") || !strings.Contains(a.Markdown, `"$schema"`) {
			t.Errorf("%s: agent page missing JSON Schema", id)
		}
		if strings.Contains(h.Markdown, `"$schema"`) {
			t.Errorf("%s: human page should not include JSON Schema", id)
		}
		if strings.Contains(h.Markdown, "## Overview\n") {
			t.Errorf("%s: human page still has Overview bullets section", id)
		}
		// Action cookbooks should use Surfaces table, not **List:** bullet blobs.
		if strings.Contains(h.Markdown, "- **List:") || strings.Contains(h.Markdown, "- **Load:") {
			t.Errorf("%s: human page still uses bold-label Overview bullets", id)
		}
	}
}

func TestGeneratedHumanLacksAgentImperatives(t *testing.T) {
	human, err := docpages.Load(docpages.LoadOptions{Catalog: docpages.CatalogHuman, PreferDisk: true})
	if err != nil {
		t.Fatal(err)
	}
	// Imperative agent voice should not appear in human Summary/Concepts output.
	bad := []string{
		"Do not use",
		"Do not invent",
		"Do not: ",
		"Prefer an instance",
		"Prefer named",
		"Prefer `",
		"Prefer **",
		"— prefer ",
		"; prefer ",
		"(do not invent",
	}
	for _, id := range generatedDocIDs {
		h := docpages.ByID(human.Pages, id)
		if h == nil {
			t.Errorf("missing %s", id)
			continue
		}
		for _, needle := range bad {
			if strings.Contains(h.Markdown, needle) {
				t.Errorf("%s human page contains agent imperative %q", id, needle)
			}
		}
	}
}

func TestAgentCatalogOmitsAgentsAndRules(t *testing.T) {
	res, err := docpages.Load(docpages.LoadOptions{Catalog: docpages.CatalogAgent, PreferDisk: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"agents", "rules/buddy-cli.mdc", "rules/buddy-builtins.mdc"} {
		if docpages.ByID(res.Pages, id) != nil {
			t.Errorf("agent catalog must not include %q", id)
		}
	}
	if docpages.ByID(res.Pages, "readme") == nil {
		t.Error("expected readme in agent catalog")
	}
	for _, id := range []string{"docs/policies", "docs/profiles", "docs/blueprints"} {
		if docpages.ByID(res.Pages, id) == nil {
			t.Errorf("expected %s in agent catalog", id)
		}
	}
}
