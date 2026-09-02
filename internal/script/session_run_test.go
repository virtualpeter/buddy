package script

import (
	"log/slog"
	"strings"
	"testing"

	"buddy/internal/buddy"
)

func TestSessionRunScriptArgsAndMain(t *testing.T) {
	s, err := NewSessionArgs("default", buddy.Opts{}, slog.Default(), false, []string{"1.2.3"})
	if err != nil {
		t.Fatal(err)
	}
	src := `
seen = []

def main():
    seen.append(args[0])
    return args[0]
`
	out, globals := s.RunScript("t.buddy", src)
	if out.Error != "" {
		t.Fatal(out.Error)
	}
	if out.Result != "1.2.3" {
		t.Fatalf("result = %q", out.Result)
	}
	found := false
	for _, g := range globals {
		if g.Name == "seen" && strings.Contains(g.Value, "1.2.3") {
			found = true
		}
	}
	if !found {
		t.Fatalf("globals = %#v", globals)
	}
	if !s.HasLastValue() {
		t.Fatal("main return should be stashable")
	}
	foundUnderscore := false
	for _, g := range s.InspectGlobals() {
		if g.Name == "_" && g.Value == "1.2.3" {
			foundUnderscore = true
		}
	}
	if !foundUnderscore {
		t.Fatal("expected _ bound to main return")
	}
	if err := s.Stash("ver"); err != nil {
		t.Fatal(err)
	}
	foundVer := false
	for _, g := range s.InspectGlobals() {
		if g.Name == "ver" && g.Value == "1.2.3" {
			foundVer = true
		}
	}
	if !foundVer {
		t.Fatal("expected stashed ver")
	}
}

func TestSessionRunScriptLastExpr(t *testing.T) {
	s, err := NewSession("default", buddy.Opts{}, slog.Default(), false)
	if err != nil {
		t.Fatal(err)
	}
	out, _ := s.RunScript("t.buddy", "print(\"hi\")\n1 + 2\n")
	if out.Error != "" {
		t.Fatal(out.Error)
	}
	if !strings.Contains(out.Stdout, "hi") {
		t.Fatalf("stdout = %q", out.Stdout)
	}
	if out.Result != "3" {
		t.Fatalf("result = %q", out.Result)
	}
	if !s.HasLastValue() {
		t.Fatal("trailing expression should be stashable")
	}
	if err := s.Stash("1bad"); err == nil {
		t.Fatal("invalid identifier should fail")
	}
	if err := s.Stash("print"); err == nil {
		t.Fatal("builtin name should fail")
	}
	if err := s.Stash("n"); err != nil {
		t.Fatal(err)
	}
	if err := s.Drop("n"); err != nil {
		t.Fatal(err)
	}
	for _, g := range s.InspectGlobals() {
		if g.Name == "n" {
			t.Fatal("dropped n should be gone")
		}
	}
	if err := s.Drop("_"); err != nil {
		t.Fatal(err)
	}
	if s.HasLastValue() {
		t.Fatal("drop _ should clear last value")
	}
}

func TestSessionRunScriptStdoutOnlyNotStashable(t *testing.T) {
	s, err := NewSession("default", buddy.Opts{}, slog.Default(), false)
	if err != nil {
		t.Fatal(err)
	}
	out, _ := s.RunScript("t.buddy", "print(\"only\")\n")
	if out.Error != "" {
		t.Fatal(out.Error)
	}
	if s.HasLastValue() {
		t.Fatal("print-only run should not be stashable")
	}
	if err := s.Stash("x"); err == nil {
		t.Fatal("stash without value should fail")
	}
}

func TestSessionRunScriptShebangAndTrace(t *testing.T) {
	s, err := NewSession("default", buddy.Opts{}, slog.Default(), true)
	if err != nil {
		t.Fatal(err)
	}
	s.SetArgs([]string{"x"})
	src := "#!/usr/bin/env buddy\nprint(args[0])\nr = rsql_eq(\"name\", \"a\")\n"
	out, _ := s.RunScript("t.buddy", src)
	if out.Error != "" {
		t.Fatal(out.Error)
	}
	if !strings.Contains(out.Stdout, "x") {
		t.Fatalf("stdout = %q", out.Stdout)
	}
	if !strings.Contains(out.Stderr, "[trace]") || !strings.Contains(out.Stderr, "rsql_eq") {
		t.Fatalf("expected builtin call trace in stderr, got %q", out.Stderr)
	}
	if !strings.Contains(out.Stderr, "print:") {
		t.Fatalf("expected print trace in stderr, got %q", out.Stderr)
	}
}

func TestSessionInspectGlobalsSkipsBuiltins(t *testing.T) {
	s, err := NewSession("default", buddy.Opts{}, slog.Default(), false)
	if err != nil {
		t.Fatal(err)
	}
	out := s.Eval("c", `x = 1`)
	if out.Error != "" {
		t.Fatal(out.Error)
	}
	vars := s.InspectGlobals()
	names := map[string]bool{}
	for _, g := range vars {
		names[g.Name] = true
		if g.Name == "policies" {
			t.Fatal("builtin policies should be filtered")
		}
	}
	if !names["x"] || !names["tenant"] || !names["auth"] || !names["args"] {
		t.Fatalf("%v", names)
	}
}
