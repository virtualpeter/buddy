package stdlib

import (
	"strings"
	"testing"
)

func TestCatalogEmpty(t *testing.T) {
	mods, err := Catalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(mods) != 0 {
		t.Fatalf("Catalog() = %#v, want empty", mods)
	}
}

func TestParseModuleFuncs(t *testing.T) {
	src := `# helper — does a thing
def helper(x, y = 1):
    """Does a thing."""
    return x

def _priv():
    pass
`
	fns := parseModuleFuncs("stdlib/demo.buddy", src)
	if len(fns) != 1 || fns[0].Name != "helper" {
		t.Fatalf("functions = %#v, want helper only", fns)
	}
	fn := fns[0]
	if fn.Summary != "does a thing" {
		t.Fatalf("summary = %q", fn.Summary)
	}
	if !strings.Contains(fn.Insert, `load("stdlib/demo.buddy", "helper")`) ||
		!strings.Contains(fn.Insert, "helper(") {
		t.Fatalf("insert = %q", fn.Insert)
	}
}
