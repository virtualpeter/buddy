package script

import (
	"log/slog"
	"strings"
	"testing"

	"buddy/internal/buddy"
)

func TestStripTrailingNoise(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"1+1", "1+1"},
		{"1+1\n", "1+1"},
		{"1+1\n\n# trailing\n", "1+1"},
		{"1+1\n\n# a\n# b\n", "1+1"},
		{"1+1  # eol\n", "1+1  # eol"},
		{"# only\n", ""},
		{"", ""},
	}
	for _, tc := range cases {
		got := stripTrailingNoise(tc.in)
		if got != tc.want {
			t.Errorf("stripTrailingNoise(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestSessionEvalTrailingComments(t *testing.T) {
	s, err := NewSession("default", buddy.Opts{SkipConfirm: true}, slog.Default(), false)
	if err != nil {
		t.Fatal(err)
	}
	for _, src := range []string{
		"1+1\n# trailing\n",
		"1+1\n\n# a\n# b\n",
		"1+1  # eol",
	} {
		r := s.Eval("cell", src)
		if r.Error != "" {
			t.Fatalf("%q: %s", src, r.Error)
		}
		if r.Result != "2" {
			t.Fatalf("%q: result=%q want 2", src, r.Result)
		}
	}
}

func TestSessionEvalTrailingPass(t *testing.T) {
	s, err := NewSession("default", buddy.Opts{SkipConfirm: true}, slog.Default(), false)
	if err != nil {
		t.Fatal(err)
	}
	r := s.Eval("cell", "1+1\npass\n")
	if r.Error != "" {
		t.Fatal(r.Error)
	}
	if r.Result != "2" {
		t.Fatalf("result=%q want 2", r.Result)
	}
}

func TestSessionEvalTrailingStringComment(t *testing.T) {
	s, err := NewSession("default", buddy.Opts{SkipConfirm: true}, slog.Default(), false)
	if err != nil {
		t.Fatal(err)
	}
	r := s.Eval("cell", "def foo():\n    return 7\nfoo()\n\"\"\"note\"\"\"\n")
	if r.Error != "" {
		t.Fatal(r.Error)
	}
	if r.Result != "7" {
		t.Fatalf("result=%q want 7 (string comment must not steal Out)", r.Result)
	}
}

func TestSessionEvalTrailingNone(t *testing.T) {
	s, err := NewSession("default", buddy.Opts{SkipConfirm: true}, slog.Default(), false)
	if err != nil {
		t.Fatal(err)
	}
	r := s.Eval("cell", "1+1\nNone\n")
	if r.Error != "" {
		t.Fatal(r.Error)
	}
	if r.Result != "2" {
		t.Fatalf("result=%q want 2", r.Result)
	}
}

func TestSessionMainReturnWithTrailingComment(t *testing.T) {
	s, err := NewSession("default", buddy.Opts{SkipConfirm: true}, slog.Default(), false)
	if err != nil {
		t.Fatal(err)
	}
	r := s.EvalOpts("c1", "def main():\n    return 7\n# note\n", EvalOptions{CallMain: true})
	if r.Error != "" {
		t.Fatal(r.Error)
	}
	if r.Result != "7" {
		t.Fatalf("result=%q", r.Result)
	}
}

func TestRunSourceLastExprTrailingNoise(t *testing.T) {
	logger := slog.Default()
	out, err := captureStdout(t, func() error {
		return RunSource("default", buddy.Opts{}, "1+1\n# done\npass\n", nil, logger, false)
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "2" {
		t.Fatalf("stdout=%q, want 2", out)
	}
}
