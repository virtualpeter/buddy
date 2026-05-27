package script

import (
	"log/slog"
	"testing"

	"buddy/internal/buddy"

	"go.starlark.net/starlark"
)

func TestConfirmBuiltin_SkipConfirmReturnsTrue(t *testing.T) {
	old := runnerOpts
	t.Cleanup(func() { runnerOpts = old })
	runnerOpts = buddy.Opts{SkipConfirm: true}

	dict := makeBuiltins("", slog.Default(), false)
	fn := dict["confirm"].(*starlark.Builtin)
	thread := &starlark.Thread{Name: "test"}
	v, err := starlark.Call(thread, fn, starlark.Tuple{starlark.String("Delete everything?")}, nil)
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if v != starlark.True {
		t.Fatalf("with -y (SkipConfirm), want True, got %v", v)
	}
}

func TestConfirmBuiltin_SkipConfirmNoArgs(t *testing.T) {
	old := runnerOpts
	t.Cleanup(func() { runnerOpts = old })
	runnerOpts = buddy.Opts{SkipConfirm: true}

	dict := makeBuiltins("", slog.Default(), false)
	fn := dict["confirm"].(*starlark.Builtin)
	thread := &starlark.Thread{Name: "test"}
	v, err := starlark.Call(thread, fn, nil, nil)
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if v != starlark.True {
		t.Fatalf("want True, got %v", v)
	}
}
