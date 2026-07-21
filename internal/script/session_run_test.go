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
	if !names["x"] || !names["cred"] || !names["args"] {
		t.Fatalf("%v", names)
	}
}
