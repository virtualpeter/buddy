package script

import (
	"log/slog"
	"strings"
	"testing"

	"buddy/internal/buddy"
	"buddy/stdlib"

	"go.starlark.net/starlark"
)

func TestStdlibValidModule(t *testing.T) {
	if !stdlib.ValidModule("stdlib/release.buddy") {
		t.Fatal("expected well-formed stdlib/release.buddy path")
	}
	if stdlib.HasModule("stdlib/release.buddy") {
		t.Fatal("expected no embedded modules")
	}
	for _, bad := range []string{
		"stdlib/../release.buddy",
		"stdlib/foo/bar.buddy",
		"stdlib/Release.buddy",
		"stdlib/release.txt",
		"release.buddy",
		"re",
	} {
		if stdlib.ValidModule(bad) {
			t.Errorf("ValidModule(%q) = true, want false", bad)
		}
	}
}

func TestStdlibListEmpty(t *testing.T) {
	mods, err := stdlib.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(mods) != 0 {
		t.Fatalf("List() = %v, want empty", mods)
	}
}

func TestLoadMissingStdlibModule(t *testing.T) {
	thread, predeclared, err := newThread("default", buddy.Opts{}, slog.Default(), false, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = starlark.ExecFile(thread, "test_missing.buddy", `
load("stdlib/release.buddy", "bump_release")
`, predeclared)
	if err == nil {
		t.Fatal("expected error loading missing stdlib module")
	}
	if !strings.Contains(err.Error(), "unknown module") {
		t.Fatalf("error = %v, want unknown module", err)
	}
}

func TestLoadUnknownModule(t *testing.T) {
	thread, predeclared, err := newThread("default", buddy.Opts{}, slog.Default(), false, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = starlark.ExecFile(thread, "test_unk.buddy", `load("nope.buddy", "x")`, predeclared)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unknown module") {
		t.Fatalf("error = %v, want unknown module", err)
	}
}

func TestFilterStdlibExports(t *testing.T) {
	pre := starlark.StringDict{
		"policy": starlark.None,
		"auth":   starlark.String("x"),
	}
	globals := starlark.StringDict{
		"policy": starlark.None,
		"auth":   starlark.String("x"),
		"_ok":    starlark.None,
		"helper": starlark.True,
	}
	got := filterStdlibExports(globals, pre)
	if _, ok := got["helper"]; !ok {
		t.Fatal("missing helper")
	}
	if len(got) != 1 {
		t.Fatalf("exports = %v, want only helper", got)
	}
}
