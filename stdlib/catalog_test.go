package stdlib

import (
	"strings"
	"testing"
)

func TestCatalogRelease(t *testing.T) {
	mods, err := Catalog()
	if err != nil {
		t.Fatal(err)
	}
	var release *ModuleInfo
	for i := range mods {
		if mods[i].ID == "stdlib/release" {
			release = &mods[i]
			break
		}
	}
	if release == nil {
		t.Fatalf("missing stdlib/release in %#v", mods)
	}
	if len(release.Functions) != 1 || release.Functions[0].Name != "bump_release" {
		t.Fatalf("functions = %#v, want bump_release only", release.Functions)
	}
	fn := release.Functions[0]
	if fn.Summary == "" {
		t.Fatal("expected summary")
	}
	if !strings.Contains(fn.Insert, `load("stdlib/release.star", "bump_release")`) ||
		!strings.Contains(fn.Insert, "bump_release(") {
		t.Fatalf("insert = %q", fn.Insert)
	}
}
