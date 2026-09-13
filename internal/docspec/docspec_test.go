package docspec_test

import (
	"crypto/sha256"
	"fmt"
	"io"
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
		if strings.Contains(h, "## Happy path") || strings.Contains(h, "## Overview") {
			t.Errorf("%s: human render still uses Happy path/Overview", topic.ID)
		}
		if !strings.Contains(h, "## Surfaces") && !strings.Contains(h, "## Patterns") {
			t.Errorf("%s: human render missing Surfaces or Patterns", topic.ID)
		}
		if strings.Contains(h, "## Surfaces") && !strings.Contains(h, "| Do this | BuddyScript | CLI | Notes |") {
			t.Errorf("%s: Surfaces section missing split BuddyScript/CLI table header", topic.ID)
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
		"docs/categories", "docs/macapps", "docs/computers", "docs/computer-searches", "docs/mdm",
		"docs/mobile-devices", "docs/mobile-searches", "docs/mobile-groups", "docs/mobile-profiles",
		"docs/extension-attributes", "docs/mobile-extension-attributes",
		"docs/api-roles", "docs/api-clients", "docs/blueprints", "docs/devices",
		"docs/device-groups", "docs/ddm", "docs/benchmarks", "docs/licenses", "docs/icons",
	}
	for _, id := range want {
		if !got[id] {
			t.Errorf("missing registered topic %s", id)
		}
	}
}

func TestGendocsMatchesCommitted(t *testing.T) {
	root := findRepoRoot(t)
	before, err := generatedTreeDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", "./cmd/gendocs")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gendocs: %v\n%s", err, out)
	}
	after, err := generatedTreeDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Errorf("gendocs rewrote docs/generated or internal/help/*_gen.go; run 'make gendocs' and keep the output\n%s", out)
	}
}

// generatedTreeDigest fingerprints audience markdown and help fragments gendocs writes.
func generatedTreeDigest(root string) (string, error) {
	h := sha256.New()
	helpDir := filepath.Join(root, "internal", "help")
	dirs := []string{
		filepath.Join(root, "docs", "generated"),
		helpDir,
	}
	for _, dir := range dirs {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			if strings.HasPrefix(path, helpDir) && !strings.HasSuffix(path, "_gen.go") {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			fmt.Fprintf(h, "%s\n", filepath.ToSlash(rel))
			if _, err := io.Copy(h, f); err != nil {
				return err
			}
			h.Write([]byte{0})
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
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
