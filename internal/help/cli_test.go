package help

import (
	"strings"
	"testing"

	"buddy/internal/clispec"
)

func TestCommandCatalogMatchesClispec(t *testing.T) {
	cat := CommandCatalog()
	spec := clispec.CatalogEntries()
	if len(cat) != len(spec) {
		t.Fatalf("catalog len %d vs spec %d", len(cat), len(spec))
	}
	for i := range spec {
		if cat[i].ID != spec[i].ID || cat[i].Title != spec[i].Title || cat[i].Level != spec[i].Level {
			t.Errorf("entry %d: catalog=%+v spec=%+v", i, cat[i], spec[i])
		}
		if cat[i].Body == "" {
			t.Errorf("empty body for %s", spec[i].ID)
		}
	}
}

func TestCLIMarkdownHasCommands(t *testing.T) {
	md := CLIMarkdown()
	for _, id := range []string{"overview", "auth", "token", "cred", "policy", "policy-set", "group", "setup", "serve", "mcp", "lsp"} {
		want := `id="` + id + `"`
		if !strings.Contains(md, want) {
			t.Errorf("CLIMarkdown missing %s", want)
		}
	}
	if !strings.Contains(md, `<article class="fn-entry">`) {
		t.Fatal("CLIMarkdown missing command cards")
	}
	if !strings.Contains(md, `<b class="fn-name">policy</b>`) {
		t.Fatal("CLIMarkdown missing policy name")
	}
	if !strings.Contains(md, "Manage Jamf policies") {
		t.Fatal("CLIMarkdown missing policy Does")
	}
	if strings.Contains(md, "```\nbuddy policy -") {
		t.Fatal("CLIMarkdown should not dump raw help as a fence")
	}
	ids := CommandAnchorIDs()
	if !ids["device-group"] || !ids["script-get"] {
		t.Fatalf("CommandAnchorIDs missing expected keys: %v", ids)
	}
	seen := map[string]bool{}
	for _, c := range CommandCatalog() {
		if c.ID == "" || c.Body == "" {
			t.Fatalf("empty catalog entry %+v", c)
		}
		if seen[c.ID] {
			t.Fatalf("duplicate command id %q", c.ID)
		}
		seen[c.ID] = true
	}
}

func TestParseCLIHelpPolicyList(t *testing.T) {
	p := parseCLIHelp(PolicyList)
	if p.Does != "List policies" {
		t.Fatalf("Does = %q", p.Does)
	}
	labels := map[string]string{}
	for _, s := range p.Sections {
		labels[s.Label] = s.Body
	}
	if !strings.Contains(labels["Usage"], "buddy [global-flags] policy list") {
		t.Fatalf("usage = %q", labels["Usage"])
	}
	if _, ok := labels["Flags (list)"]; !ok {
		t.Fatalf("sections = %v", keys(labels))
	}
	if !strings.Contains(labels["Examples"], "buddy policy list --filter=chrome") {
		t.Fatalf("examples = %q", labels["Examples"])
	}
}

func TestCLIMarkdownPolicyListCard(t *testing.T) {
	md := CLIMarkdown()
	idx := strings.Index(md, `<h3 id="policy-list">`)
	if idx < 0 {
		t.Fatal("missing policy-list")
	}
	art := strings.LastIndex(md[:idx], `<article class="fn-entry">`)
	end := strings.Index(md[idx:], `</article>`)
	chunk := md[art : idx+end+len("</article>")]
	for _, want := range []string{
		`<b class="fn-name">policy list</b> — List policies`,
		`<dt>Usage</dt>`,
		`<dt>Flags (list)</dt>`,
		`<code>--filter=&lt;keyword&gt;</code>`,
		`<dt>Examples</dt>`,
	} {
		if !strings.Contains(chunk, want) {
			t.Fatalf("missing %q in:\n%s", want, chunk)
		}
	}
}

func TestParseCLIHelpRootCommands(t *testing.T) {
	p := parseCLIHelp(Root)
	if p.Does != "Jamf Pro management CLI" {
		t.Fatalf("Does = %q", p.Does)
	}
	var cmds cliSection
	for _, s := range p.Sections {
		if s.Label == "Commands" {
			cmds = s
		}
	}
	items := splitCLIDefItems(cmds.Body)
	found := false
	for _, it := range items {
		if it.Key == "device-group" && strings.Contains(it.Desc, "Platform") {
			found = true
		}
	}
	if !found {
		t.Fatalf("device-group item missing: %+v", items)
	}
}

func TestParseCLIHelpPolicySetKeepsNestedFlagsInUsage(t *testing.T) {
	p := parseCLIHelp(PolicySet)
	var usage string
	for _, s := range p.Sections {
		if s.Label == "Usage" {
			usage = s.Body
		}
		if strings.HasPrefix(s.Label, "Flags") {
			t.Fatalf("unexpected flags section %q", s.Label)
		}
	}
	if !strings.Contains(usage, "[--display-name=") {
		t.Fatalf("nested Flags: line should stay in Usage:\n%s", usage)
	}
}

func TestParseCLIHelpCredLeadAndNotes(t *testing.T) {
	p := parseCLIHelp(Cred)
	if !strings.Contains(p.Lead, "Config is split into tenants") {
		t.Fatalf("lead = %q", p.Lead)
	}
	foundNotes := false
	var subcommands, examples string
	for _, s := range p.Sections {
		switch s.Label {
		case "Notes":
			if strings.Contains(s.Body, "platform clientId") {
				foundNotes = true
			}
		case "Subcommands":
			subcommands = s.Body
		case "Examples":
			examples = s.Body
		}
	}
	if !foundNotes {
		var labels []string
		for _, s := range p.Sections {
			labels = append(labels, s.Label)
		}
		t.Fatalf("expected Notes from indented Notes: header, sections=%v", labels)
	}
	if !strings.HasPrefix(strings.TrimSpace(subcommands), "add ") {
		t.Fatalf("full cred Subcommands should start with add:\n%s", subcommands)
	}
	if !strings.HasPrefix(strings.TrimSpace(examples), "buddy cred add ") {
		t.Fatalf("full cred Examples should start with cred add:\n%s", examples)
	}
}

func TestParseCLIHelpCredShort(t *testing.T) {
	p := parseCLIHelp(CredShort)
	if p.Does != "Manage tenants, auths, and secrets" {
		t.Fatalf("Does = %q", p.Does)
	}
	var subcommands, notes, examples string
	for _, s := range p.Sections {
		switch {
		case s.Label == "Subcommands":
			subcommands = s.Body
		case s.Label == "Notes":
			notes = s.Body
		case s.Label == "Examples":
			examples = s.Body
		case strings.HasPrefix(s.Label, "Flags"):
			t.Fatalf("short cred help should not include %q", s.Label)
		}
	}
	if !strings.HasPrefix(strings.TrimSpace(subcommands), "add ") {
		t.Fatalf("short Subcommands should start with add:\n%s", subcommands)
	}
	if !strings.Contains(notes, "buddy cred help") {
		t.Fatalf("short Notes should point at cred help:\n%s", notes)
	}
	if !strings.Contains(examples, "buddy cred add --tenant=") {
		t.Fatalf("short Examples missing setup add:\n%s", examples)
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
