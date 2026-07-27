package script

import (
	"log/slog"
	"testing"

	"buddy/internal/buddy"
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
check(r["ok"] == True, "extract ok")
check(r["value"] == "130.0.6723.59", "extract value")

r = version_compare("1.9", "1.10")
check(r["value"] == -1, "compare 1.9 vs 1.10")

r = version_compare("Chrome-1.2.0.pkg", "Chrome-1.10.0.pkg")
check(r["value"] == -1, "compare pkg names")

items = ["App-1.10.0.pkg", "App-1.2.0.pkg", "noversion", "App-1.9.0.pkg"]
r = version_sort(items, order="asc")
check(r["ok"] == True, "sort ok")
check(r["value"] == ["App-1.2.0.pkg", "App-1.9.0.pkg", "App-1.10.0.pkg", "noversion"], "sort asc")

dicts = [
    {"packageName": "App-1.10.0.pkg", "id": "a"},
    {"packageName": "App-1.2.0.pkg", "id": "b"},
]
r = version_sort(dicts, key="packageName", order="desc")
check(r["ok"] == True, "dict sort ok")
check(r["value"][0]["id"] == "a", "dict sort first")
check(r["value"][1]["id"] == "b", "dict sort second")

r = version_sort(dicts)
check(r["ok"] == False, "dicts require key")
`
	r := s.Eval("v", src)
	if r.Error != "" {
		t.Fatal(r.Error)
	}
}
