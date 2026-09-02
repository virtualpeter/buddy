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

func TestResultOKIsBare(t *testing.T) {
	bare := resultOK(starlark.String("x"))
	if _, ok := bare.(starlark.String); !ok {
		t.Fatalf("resultOK should be bare, got %T", bare)
	}
}

func TestUnwrapBareJSONPrint(t *testing.T) {
	sess, err := NewSession("default", buddy.Opts{}, slog.Default(), false)
	if err != nil {
		t.Fatal(err)
	}
	res := sess.Eval("unwrap_bare", `
v = unwrap({"hello": "world"}, pretty=False)
v
`)
	if res.Error != "" {
		t.Fatal(res.Error)
	}
	got := strings.TrimSpace(res.Stdout)
	if got != `{"hello":"world"}` {
		t.Fatalf("stdout=%q", res.Stdout)
	}
}

func TestUnwrapPrintsDictAsJSON(t *testing.T) {
	sess, err := NewSession("default", buddy.Opts{}, slog.Default(), false)
	if err != nil {
		t.Fatal(err)
	}
	res := sess.Eval("unwrap_dict", `
unwrap({"ok": False, "error": "boom"}, pretty=False)
`)
	if res.Error != "" {
		t.Fatal(res.Error)
	}
	got := strings.TrimSpace(res.Stdout)
	if !strings.Contains(got, `"error":"boom"`) {
		t.Fatalf("stdout=%q", res.Stdout)
	}
}

func TestAttemptSoft(t *testing.T) {
	sess, err := NewSession("default", buddy.Opts{}, slog.Default(), false)
	if err != nil {
		t.Fatal(err)
	}
	res := sess.Eval("attempt_ok", `
def ok():
    return 42
a = attempt(ok)
a["ok"] == True and a["value"] == 42
`)
	if res.Error != "" {
		t.Fatal(res.Error)
	}
	if strings.TrimSpace(res.Result) != "true" {
		t.Fatalf("Result=%q", res.Result)
	}
	res = sess.Eval("attempt_err", `
def boom():
    fail("nope")
a = attempt(boom)
a["ok"] == False and "nope" in a["error"]
`)
	if res.Error != "" {
		t.Fatal(res.Error)
	}
	if strings.TrimSpace(res.Result) != "true" {
		t.Fatalf("Result=%q", res.Result)
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
