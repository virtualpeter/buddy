package docspec_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"buddy/internal/buddy"
	"buddy/internal/docspec"
)

func TestRenderHumanVsAgent(t *testing.T) {
	topics := []docspec.Topic{buddy.AppsTopic, buddy.PoliciesTopic, buddy.ProfilesTopic, buddy.BlueprintsTopic}
	for _, topic := range topics {
		h := docspec.RenderHuman(topic)
		a := docspec.RenderAgent(topic)
		if strings.Contains(h, "## Agent rules") {
			t.Errorf("%s: human render leaked agent rules", topic.ID)
		}
		if !strings.Contains(a, "## Agent rules") {
			t.Errorf("%s: agent render missing agent rules", topic.ID)
		}
		if !strings.Contains(a, "audience: agent") {
			t.Errorf("%s: agent render missing frontmatter", topic.ID)
		}
		if len(topic.Builtins) > 0 && !strings.Contains(a, "```json") {
			t.Errorf("%s: agent render missing JSON Schema fence", topic.ID)
		}
		if strings.Contains(h, "## Happy path") {
			t.Errorf("%s: human render still uses Happy path", topic.ID)
		}
		if !strings.Contains(h, "## Overview") && !strings.Contains(h, "## Patterns") {
			t.Errorf("%s: human render missing Overview or Patterns", topic.ID)
		}
	}
}

func TestAllTopicsRegistered(t *testing.T) {
	_ = buddy.AppsTopic // ensure package init
	got := map[string]bool{}
	for _, tpc := range docspec.All() {
		got[tpc.ID] = true
	}
	want := []string{
		"docs/apps", "docs/policies", "docs/smart-groups", "docs/static-groups",
		"docs/profiles", "docs/profile-payloads", "docs/packages", "docs/scripts",
		"docs/categories", "docs/macapps", "docs/computers", "docs/mdm",
		"docs/api-roles", "docs/api-clients", "docs/blueprints", "docs/devices",
		"docs/device-groups", "docs/ddm", "docs/benchmarks",
	}
	for _, id := range want {
		if !got[id] {
			t.Errorf("missing registered topic %s", id)
		}
	}
}

func TestGendocsMatchesCommitted(t *testing.T) {
	root := findRepoRoot(t)
	cmd := exec.Command("go", "run", "./cmd/gendocs")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gendocs: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "diff", "--exit-code", "--", "docs/generated", "internal/help")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		// Only fail on generated artifacts; ignore unrelated help edits outside *_gen.go
		cmd2 := exec.Command("git", "diff", "--exit-code", "--", "docs/generated/", "internal/help/*_gen.go")
		cmd2.Dir = root
		if out2, err2 := cmd2.CombinedOutput(); err2 != nil {
			t.Errorf("gendocs output stale:\n%s\n%s", out, out2)
		}
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
