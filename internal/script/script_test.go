package script

import (
	"log/slog"
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

	for _, key := range []string{"cred", "args", "re", "policy_get_id", "confirm", "exit"} {
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
