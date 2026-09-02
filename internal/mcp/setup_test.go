package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWellKnownMCPConfigTargets(t *testing.T) {
	targets := WellKnownMCPConfigTargets()
	if len(targets) < 2 {
		t.Fatalf("expected cursor + claude at least, got %d", len(targets))
	}
	var sawCursorUser, sawClaude bool
	for _, tgt := range targets {
		if strings.Contains(tgt.Host, "cursor (user)") {
			sawCursorUser = true
			if !strings.HasSuffix(tgt.Path, filepath.Join(".cursor", "mcp.json")) &&
				!strings.Contains(tgt.Path, ".cursor"+string(filepath.Separator)+"mcp.json") {
				t.Fatalf("cursor user path = %q", tgt.Path)
			}
		}
		if tgt.Host == "claude-desktop" {
			sawClaude = true
			switch runtime.GOOS {
			case "darwin":
				if !strings.Contains(tgt.Path, filepath.Join("Application Support", "Claude")) {
					t.Fatalf("claude mac path = %q", tgt.Path)
				}
			case "windows":
				if !strings.Contains(strings.ToLower(tgt.Path), "claude") {
					t.Fatalf("claude win path = %q", tgt.Path)
				}
			default:
				if !strings.Contains(tgt.Path, filepath.Join(".config", "Claude")) {
					t.Fatalf("claude linux path = %q", tgt.Path)
				}
			}
		}
	}
	if !sawCursorUser || !sawClaude {
		t.Fatalf("missing targets: cursorUser=%v claude=%v (%+v)", sawCursorUser, sawClaude, targets)
	}
}

func TestMergeBuddyMCPConfig_PreservesOthers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")
	initial := []byte(`{
  "mcpServers": {
    "other": {"command": "echo", "args": ["hi"]}
  }
}
`)
	if err := os.WriteFile(path, initial, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := MergeBuddyMCPConfig(path, "/abs/buddy")
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(out, &root); err != nil {
		t.Fatal(err)
	}
	servers := root["mcpServers"].(map[string]any)
	if _, ok := servers["other"]; !ok {
		t.Fatal("other server missing")
	}
	buddy := servers["buddy"].(map[string]any)
	if buddy["command"] != "/abs/buddy" {
		t.Fatalf("command = %v", buddy["command"])
	}
	args, _ := buddy["args"].([]any)
	if len(args) != 1 || args[0] != "mcp" {
		t.Fatalf("args = %v", args)
	}
}

func TestWriteBuddyMCPConfig_Creates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "mcp.json")
	if err := WriteBuddyMCPConfig(path, "/bin/buddy"); err != nil {
		t.Fatal(err)
	}
	st := InspectMCPConfig(path)
	if !st.Exists || !st.HasBuddy || st.BuddyPath != "/bin/buddy" {
		t.Fatalf("%+v", st)
	}
}

func TestInspectMCPConfig_Missing(t *testing.T) {
	st := InspectMCPConfig(filepath.Join(t.TempDir(), "nope.json"))
	if st.Exists || st.HasBuddy {
		t.Fatalf("%+v", st)
	}
}
