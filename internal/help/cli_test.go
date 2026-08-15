package help

import (
	"strings"
	"testing"
)

func TestCLIMarkdownHasCommands(t *testing.T) {
	md := CLIMarkdown()
	for _, id := range []string{"overview", "auth", "cred", "policy", "policy-set", "group", "setup", "serve", "mcp", "lsp"} {
		want := `id="` + id + `"`
		if !strings.Contains(md, want) {
			t.Errorf("CLIMarkdown missing %s", want)
		}
	}
	if !strings.Contains(md, "buddy policy - Manage Jamf policies") {
		t.Fatal("CLIMarkdown missing policy help body")
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
