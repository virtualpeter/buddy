package help

import (
	"strings"
	"testing"
)

func TestBuiltinsLibraryHasPolicy(t *testing.T) {
	sections, flat := BuiltinsLibrary()
	if len(sections) == 0 {
		t.Fatal("no sections")
	}
	if sections[0].Title != "Getting started" {
		t.Fatalf("first section = %q, want Getting started", sections[0].Title)
	}
	found := false
	for _, f := range flat {
		if f.Name == "policy" {
			found = true
			if f.Insert != "policy()" {
				t.Fatalf("insert = %q", f.Insert)
			}
			if f.Group != "getting-started" {
				t.Fatalf("group = %q", f.Group)
			}
			break
		}
	}
	if !found {
		t.Fatal("policy not found in builtins catalog")
	}
}

func TestCatalogInlineReturn(t *testing.T) {
	_, flat := BuiltinsLibrary()
	var del LibraryFunc
	for _, f := range flat {
		if f.Name == "policy_delete" {
			del = f
			break
		}
	}
	if del.Name == "" {
		t.Fatal("policy_delete missing")
	}
	if del.Returns == "" {
		t.Fatalf("expected returns from inline →, signature=%q summary=%q", del.Signature, del.Summary)
	}
	if strings.Contains(del.Signature, "→") {
		t.Fatalf("signature should not retain arrow: %q", del.Signature)
	}
}

func TestLookupBuiltinAndFormatHelp(t *testing.T) {
	fn, ok := LookupBuiltin("policies")
	if !ok {
		t.Fatal("policies not found")
	}
	help := FormatBuiltinHelp(fn)
	if !strings.Contains(help, "policies(") || !strings.Contains(help, "Params:") {
		t.Fatalf("help = %q", help)
	}
}

func TestChangeBatchInsertStub(t *testing.T) {
	fn, ok := LookupBuiltin("change_batch")
	if !ok {
		t.Fatal("missing change_batch")
	}
	if !strings.Contains(fn.Insert, "plan_change") {
		t.Fatalf("insert = %q", fn.Insert)
	}
}
