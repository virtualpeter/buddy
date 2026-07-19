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
		t.Fatal("expected release.buddy valid")
	}
	for _, bad := range []string{
		"stdlib/../release.buddy",
		"stdlib/foo/bar.buddy",
		"stdlib/Release.buddy",
		"stdlib/release.star",
		"release.buddy",
		"re",
	} {
		if stdlib.ValidModule(bad) {
			t.Errorf("ValidModule(%q) = true, want false", bad)
		}
	}
}

func TestStdlibListContainsRelease(t *testing.T) {
	mods, err := stdlib.List()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, m := range mods {
		if m == "stdlib/release.buddy" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("List() = %v, want stdlib/release.buddy", mods)
	}
}

func TestLoadStdlibReleaseExports(t *testing.T) {
	thread, predeclared, err := newThread("default", buddy.Opts{}, slog.Default(), false, nil)
	if err != nil {
		t.Fatal(err)
	}
	src := `
load("stdlib/release.buddy", "bump_release")
result = type(bump_release)
`
	globals, err := starlark.ExecFile(thread, "test_load.buddy", src, predeclared)
	if err != nil {
		t.Fatalf("ExecFile: %v", err)
	}
	got := globals["result"].(starlark.String)
	if string(got) != "function" {
		t.Fatalf("type(bump_release) = %q, want function", got)
	}
	// Private helpers must not be loadable.
	_, err = starlark.ExecFile(thread, "test_priv.buddy", `
load("stdlib/release.buddy", "_ok")
`, predeclared)
	if err == nil {
		t.Fatal("expected error loading private _ok")
	}
}

func TestLoadStdlibCache(t *testing.T) {
	thread, predeclared, err := newThread("default", buddy.Opts{}, slog.Default(), false, nil)
	if err != nil {
		t.Fatal(err)
	}
	src := `
load("stdlib/release.buddy", "bump_release")
load("stdlib/release.buddy", bump2 = "bump_release")
same = bump_release == bump2
`
	globals, err := starlark.ExecFile(thread, "test_cache.buddy", src, predeclared)
	if err != nil {
		t.Fatalf("ExecFile: %v", err)
	}
	if globals["same"] != starlark.True {
		t.Fatal("expected cached module to return same function object")
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
		"cred":   starlark.String("x"),
	}
	globals := starlark.StringDict{
		"policy":       starlark.None,
		"cred":         starlark.String("x"),
		"_ok":          starlark.None,
		"bump_release": starlark.True,
	}
	got := filterStdlibExports(globals, pre)
	if _, ok := got["bump_release"]; !ok {
		t.Fatal("missing bump_release")
	}
	if len(got) != 1 {
		t.Fatalf("exports = %v, want only bump_release", got)
	}
}
