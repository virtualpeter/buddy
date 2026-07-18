package help

import "testing"

func TestBuiltinsLibraryHasPolicy(t *testing.T) {
	sections, flat := BuiltinsLibrary()
	if len(sections) == 0 {
		t.Fatal("no sections")
	}
	found := false
	for _, f := range flat {
		if f.Name == "policy" {
			found = true
			if f.Insert != "policy()" {
				t.Fatalf("insert = %q", f.Insert)
			}
			if f.Section == "" {
				t.Fatal("missing section")
			}
			break
		}
	}
	if !found {
		t.Fatal("policy not found in builtins catalog")
	}
}
