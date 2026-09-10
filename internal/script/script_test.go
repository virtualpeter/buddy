package script

import (
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"buddy/internal/buddy"

	"go.starlark.net/starlark"
)

func TestNewThreadPredeclared(t *testing.T) {
	logger := slog.Default()
	thread, globals, err := newThread("default", buddy.Opts{}, logger, false, []string{"a", "b"})
	if err != nil {
		t.Fatalf("newThread: %v", err)
	}
	if thread == nil {
		t.Fatal("expected non-nil thread")
	}

	for _, key := range []string{"tenant", "auth", "args", "re", "policy", "policies", "confirm", "exit"} {
		if _, ok := globals[key]; !ok {
			t.Errorf("missing predeclared %q", key)
		}
	}

	list, ok := globals["args"].(*starlark.List)
	if !ok {
		t.Fatalf("args type = %T", globals["args"])
	}
	if list.Len() != 2 {
		t.Errorf("args len = %d, want 2", list.Len())
	}
}

func TestParseScriptInvocationEmpty(t *testing.T) {
	trace, path, scriptArgs := ParseScriptInvocation(nil)
	if trace || path != "" || scriptArgs != nil {
		t.Errorf("got trace=%v path=%q args=%v", trace, path, scriptArgs)
	}
}

func TestParseEvalArgs(t *testing.T) {
	if got := ParseEvalArgs([]string{"--", "a", "b"}); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("ParseEvalArgs(-- …) = %v", got)
	}
	if got := ParseEvalArgs([]string{"a"}); len(got) != 1 || got[0] != "a" {
		t.Fatalf("ParseEvalArgs(a) = %v", got)
	}
}

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	runErr := fn()
	_ = w.Close()
	os.Stdout = old
	b, readErr := io.ReadAll(r)
	_ = r.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	return string(b), runErr
}

func TestRunSourceLastExpr(t *testing.T) {
	logger := slog.Default()
	out, err := captureStdout(t, func() error {
		return RunSource("default", buddy.Opts{}, "1+1", nil, logger, false)
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "2" {
		t.Fatalf("stdout=%q, want 2", out)
	}
}

func TestRunSourceUnwrap(t *testing.T) {
	logger := slog.Default()
	src := `unwrap({"a": 1}, pretty=False)`
	out, err := captureStdout(t, func() error {
		return RunSource("default", buddy.Opts{}, src, nil, logger, false)
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != `{"a":1}` {
		t.Fatalf("stdout=%q, want {\"a\":1}", out)
	}
}

func TestRunSourceJQ(t *testing.T) {
	logger := slog.Default()
	out, err := captureStdout(t, func() error {
		return RunSource("default", buddy.Opts{}, `jq({"a": 1}, ".a")`, nil, logger, false)
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "1" {
		t.Fatalf("jq int: stdout=%q", out)
	}
	out, err = captureStdout(t, func() error {
		return RunSource("default", buddy.Opts{}, `jq({"n": "Ada"}, ".n")`, nil, logger, false)
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "Ada" {
		t.Fatalf("jq string should be unquoted: stdout=%q", out)
	}
	err = RunSource("default", buddy.Opts{}, `jq({"a": 1}, ".foo & .bar")`, nil, logger, false)
	if err == nil {
		t.Fatal("expected jq parse error")
	}
}

func TestRunSourceJSONFlatten(t *testing.T) {
	logger := slog.Default()
	out, err := captureStdout(t, func() error {
		return RunSource("default", buddy.Opts{}, `json_flatten({"current": {"place": "hobo"}}, prefix="h")["h.current.place"]`, nil, logger, false)
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "hobo" {
		t.Fatalf("json_flatten: stdout=%q", out)
	}
}

func TestRunSourceArgs(t *testing.T) {
	logger := slog.Default()
	out, err := captureStdout(t, func() error {
		return RunSource("default", buddy.Opts{}, "len(args)", []string{"foo", "bar"}, logger, false)
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "2" {
		t.Fatalf("stdout=%q, want 2", out)
	}
}
