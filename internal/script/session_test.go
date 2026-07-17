package script

import (
	"log/slog"
	"strings"
	"testing"

	"buddy/internal/buddy"
)

func TestSessionEvalPrintAndExpr(t *testing.T) {
	s, err := NewSession("default", buddy.Opts{SkipConfirm: true}, slog.Default(), false)
	if err != nil {
		t.Fatal(err)
	}
	r := s.Eval("cell-1", `print("hi")`)
	if r.Error != "" {
		t.Fatalf("print: %s", r.Error)
	}
	if !strings.Contains(r.Stdout, "hi") {
		t.Fatalf("stdout=%q", r.Stdout)
	}
	r2 := s.Eval("cell-2", `1 + 2`)
	if r2.Error != "" {
		t.Fatalf("expr: %s", r2.Error)
	}
	if r2.Result != "3" {
		t.Fatalf("result=%q", r2.Result)
	}
}

func TestSessionPersistence(t *testing.T) {
	s, err := NewSession("default", buddy.Opts{SkipConfirm: true}, slog.Default(), false)
	if err != nil {
		t.Fatal(err)
	}
	r := s.Eval("c1", `x = 10`)
	if r.Error != "" {
		t.Fatal(r.Error)
	}
	r2 := s.Eval("c2", `x * 2`)
	if r2.Error != "" {
		t.Fatal(r2.Error)
	}
	if r2.Result != "20" {
		t.Fatalf("result=%q want 20", r2.Result)
	}
}

func TestSessionErrorBuiltinCapture(t *testing.T) {
	s, err := NewSession("default", buddy.Opts{SkipConfirm: true}, slog.Default(), false)
	if err != nil {
		t.Fatal(err)
	}
	r := s.Eval("c1", `error("warn")`)
	if r.Error != "" {
		t.Fatal(r.Error)
	}
	if !strings.Contains(r.Stderr, "warn") {
		t.Fatalf("stderr=%q", r.Stderr)
	}
}

func TestSessionMainReturn(t *testing.T) {
	s, err := NewSession("default", buddy.Opts{SkipConfirm: true}, slog.Default(), false)
	if err != nil {
		t.Fatal(err)
	}
	r := s.EvalOpts("c1", "def main():\n    return 1 + 2\n", EvalOptions{CallMain: true})
	if r.Error != "" {
		t.Fatal(r.Error)
	}
	if r.Result != "3" {
		t.Fatalf("result=%q", r.Result)
	}
}

func TestSessionReset(t *testing.T) {
	s, err := NewSession("default", buddy.Opts{SkipConfirm: true}, slog.Default(), false)
	if err != nil {
		t.Fatal(err)
	}
	if r := s.Eval("c1", `x = 1`); r.Error != "" {
		t.Fatal(r.Error)
	}
	if err := s.Reset(); err != nil {
		t.Fatal(err)
	}
	r := s.Eval("c2", `x`)
	if r.Error == "" {
		t.Fatal("expected undefined x after reset")
	}
}
