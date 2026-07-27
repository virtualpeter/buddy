package script

import (
	"log/slog"
	"strings"
	"testing"

	"buddy/internal/buddy"

	"go.starlark.net/starlark"
)

func TestFormatDisplayValueBuiltinHelp(t *testing.T) {
	dict := makeBuiltins("default", slog.Default(), false)
	b, ok := dict["policies"].(*starlark.Builtin)
	if !ok {
		t.Fatal("policies not a builtin")
	}
	got := FormatDisplayValue(b)
	if !strings.Contains(got, "policies(") || !strings.Contains(got, "Params:") {
		t.Fatalf("got %q", got)
	}
}

func TestFormatDisplayValueJSON(t *testing.T) {
	d := new(starlark.Dict)
	_ = d.SetKey(starlark.String("a"), starlark.MakeInt(1))
	got := FormatDisplayValue(d)
	if !strings.Contains(got, "\n") || !strings.Contains(got, `"a"`) {
		t.Fatalf("got %q", got)
	}
	// Must not be Starlark repr with single quotes.
	if strings.Contains(got, "'a'") {
		t.Fatalf("looks like Starlark repr: %q", got)
	}
}

func TestFormatDisplayValueStringRaw(t *testing.T) {
	got := FormatDisplayValue(starlark.String("line1\nline2"))
	if got != "line1\nline2" {
		t.Fatalf("got %q", got)
	}
}

func TestResultEnvelope(t *testing.T) {
	env := resultOK(starlark.String("x"))
	ok, val, _, isEnv := resultEnvelope(env)
	if !isEnv || !ok {
		t.Fatal("expected ok envelope")
	}
	if string(val.(starlark.String)) != "x" {
		t.Fatalf("val %v", val)
	}
	plain := new(starlark.Dict)
	_ = plain.SetKey(starlark.String("api"), starlark.String("pro"))
	_, _, _, isEnv = resultEnvelope(plain)
	if isEnv {
		t.Fatal("plain dict should not be envelope")
	}
}

func TestUnwrapOK(t *testing.T) {
	sess, err := NewSession("default", buddy.Opts{}, slog.Default(), false)
	if err != nil {
		t.Fatal(err)
	}
	res := sess.Eval("unwrap_ok", `
v = unwrap({"ok": True, "value": {"hello": "world"}, "error": ""}, pretty=False)
v
`)
	if res.Error != "" {
		t.Fatal(res.Error)
	}
	got := strings.TrimSpace(res.Stdout)
	if got != `{"hello":"world"}` {
		t.Fatalf("stdout=%q", res.Stdout)
	}
	if !strings.Contains(res.Result, `"hello"`) {
		t.Fatalf("Result=%q", res.Result)
	}
}

func TestUnwrapSoftError(t *testing.T) {
	sess, err := NewSession("default", buddy.Opts{}, slog.Default(), false)
	if err != nil {
		t.Fatal(err)
	}
	res := sess.Eval("unwrap_err", `
v = unwrap({"ok": False, "error": "boom", "value": None})
v == None
`)
	if res.Error != "" {
		t.Fatalf("should not abort: %v", res.Error)
	}
	if !strings.Contains(res.Stderr, "boom") {
		t.Fatalf("stderr=%q", res.Stderr)
	}
	// Bool last-expr is shown as JSON true/false.
	if strings.TrimSpace(res.Result) != "true" {
		t.Fatalf("Result=%q want true (v == None)", res.Result)
	}
	if strings.TrimSpace(res.Stdout) != "" {
		t.Fatalf("stdout should be empty on error, got %q", res.Stdout)
	}
}

func TestNotebookOutIsJSON(t *testing.T) {
	sess, err := NewSession("default", buddy.Opts{}, slog.Default(), false)
	if err != nil {
		t.Fatal(err)
	}
	res := sess.Eval("out_test", `{"a": 1}`)
	if res.Error != "" {
		t.Fatal(res.Error)
	}
	if !strings.Contains(res.Result, "\n") || !strings.Contains(res.Result, `"a"`) {
		t.Fatalf("Result=%q", res.Result)
	}
}
