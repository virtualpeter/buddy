package lsp

import (
	"strings"
	"testing"
)

func TestAnalyzeSyntaxError(t *testing.T) {
	diags := Analyze("t.buddy", "def main(:\n  pass\n", ModeFile)
	if len(diags) == 0 {
		t.Fatal("expected syntax diagnostic")
	}
	if diags[0].Severity != severityError {
		t.Fatalf("severity = %d", diags[0].Severity)
	}
}

func TestAnalyzeUndefinedName(t *testing.T) {
	diags := Analyze("t.buddy", `
def main():
    nope()
`, ModeFile)
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "nope") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected undefined name diagnostic, got %#v", diags)
	}
}

func TestAnalyzeKnownBuiltin(t *testing.T) {
	diags := Analyze("t.buddy", `
def main():
    r = policies()
    return r
`, ModeFile)
	for _, d := range diags {
		if d.Severity == severityError {
			t.Fatalf("unexpected error: %s", d.Message)
		}
	}
}

func TestAnalyzeInvalidLoad(t *testing.T) {
	diags := Analyze("t.buddy", `load("nope.buddy", "x")
`, ModeFile)
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "unknown module") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected invalid load diagnostic, got %#v", diags)
	}
}

func TestAnalyzeValidStdlibLoad(t *testing.T) {
	diags := Analyze("t.buddy", `
load("stdlib/release.buddy", "bump_release")

def main():
    return bump_release
`, ModeFile)
	for _, d := range diags {
		if d.Severity == severityError {
			t.Fatalf("unexpected error: %s", d.Message)
		}
	}
}

func TestAnalyzeUnknownLegacyAliasIsError(t *testing.T) {
	diags := Analyze("t.buddy", `
def main():
    package_list()
`, ModeFile)
	found := false
	for _, d := range diags {
		if d.Severity == severityError && strings.Contains(d.Message, "package_list") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected resolve error for removed alias, got %#v", diags)
	}
}

func TestCatalogHasPolicies(t *testing.T) {
	cat, err := getCatalog()
	if err != nil {
		t.Fatal(err)
	}
	sym, ok := cat.lookup("policies")
	if !ok {
		t.Fatal("missing policies")
	}
	if sym.Signature == "" {
		t.Fatal("empty signature")
	}
	found := false
	for _, s := range cat.filterCompletion("pol") {
		if s.Name == "policies" {
			found = true
		}
		if s.Name == "package_list" {
			t.Fatal("removed alias must not be in completions")
		}
	}
	if !found {
		t.Fatal("policies not in filtered completions")
	}
	if _, ok := cat.lookup("package_list"); ok {
		t.Fatal("removed alias must not be in catalog")
	}
}

func TestWordAt(t *testing.T) {
	prefix, start := wordAt("policies()", 0, 3)
	if prefix != "pol" || start != 0 {
		t.Fatalf("got prefix=%q start=%d", prefix, start)
	}
}

func TestCallAt(t *testing.T) {
	name, arg, ok := callAt("policy_set(1, enable=True)", 0, 14)
	if !ok || name != "policy_set" || arg != 1 {
		t.Fatalf("got name=%q arg=%d ok=%v", name, arg, ok)
	}
}

func TestLoadStringPrefix(t *testing.T) {
	p, in, ok := loadStringPrefix(`load("std`, 0, 9)
	if !ok || !in || p != "std" {
		t.Fatalf("got prefix=%q in=%v ok=%v", p, in, ok)
	}
}
