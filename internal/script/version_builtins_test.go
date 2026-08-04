package script

import (
	"log/slog"
	"strings"
	"testing"

	"buddy/internal/buddy"
	"go.starlark.net/starlark"
)

func TestVersionBuiltins(t *testing.T) {
	s, err := NewSession("default", buddy.Opts{SkipConfirm: true}, slog.Default(), false)
	if err != nil {
		t.Fatal(err)
	}
	src := `
def check(cond, msg):
    if not cond:
        fail(msg)

r = version_extract("GoogleChrome-130.0.6723.59.pkg")
check(r == "130.0.6723.59", "extract value")

r = version_compare("1.9", "1.10")
check(r == -1, "compare 1.9 vs 1.10")

r = version_compare("Chrome-1.2.0.pkg", "Chrome-1.10.0.pkg")
check(r == -1, "compare pkg names")

items = ["App-1.10.0.pkg", "App-1.2.0.pkg", "noversion", "App-1.9.0.pkg"]
r = version_sort(items, order="asc")
check(r == ["App-1.2.0.pkg", "App-1.9.0.pkg", "App-1.10.0.pkg", "noversion"], "sort asc")

dicts = [
    {"packageName": "App-1.10.0.pkg", "id": "a"},
    {"packageName": "App-1.2.0.pkg", "id": "b"},
]
r = version_sort(dicts, key="packageName", order="desc")
check(r[0]["id"] == "a", "dict sort first")
check(r[1]["id"] == "b", "dict sort second")
`
	r := s.Eval("v", src)
	if r.Error != "" {
		t.Fatal(r.Error)
	}

	dict := makeBuiltins("", nil, false)
	sortFn := dict["version_sort"].(*starlark.Builtin)
	th := &starlark.Thread{Name: "t"}
	dicts := starlark.NewList([]starlark.Value{
		func() *starlark.Dict {
			d := new(starlark.Dict)
			_ = d.SetKey(starlark.String("packageName"), starlark.String("App-1.10.0.pkg"))
			return d
		}(),
	})
	_, err = starlark.Call(th, sortFn, starlark.Tuple{dicts}, nil)
	if err == nil {
		t.Fatal("expected version_sort(dicts) without key to raise")
	}
	if !strings.Contains(err.Error(), "key") {
		t.Fatalf("unexpected error: %v", err)
	}
}
