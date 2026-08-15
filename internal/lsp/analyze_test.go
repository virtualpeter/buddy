package lsp

import (
	"strings"
	"testing"

	"buddy/internal/help"

	"go.starlark.net/starlark"
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

func TestAnalyzeValidReLoad(t *testing.T) {
	diags := Analyze("t.buddy", `
load("re", "re")

def main():
    return re
`, ModeFile)
	for _, d := range diags {
		if d.Severity == severityError {
			t.Fatalf("unexpected error: %s", d.Message)
		}
	}
}

func TestAnalyzeMissingStdlibLoad(t *testing.T) {
	diags := Analyze("t.buddy", `load("stdlib/release.buddy", "bump_release")
`, ModeFile)
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "unknown module") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected unknown stdlib module diagnostic, got %#v", diags)
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

func TestHoverMarkdownMatchesFunctionIndexCard(t *testing.T) {
	md := hoverMarkdown(Symbol{Name: "policy_xml", Kind: "builtin"})
	for _, want := range []string{
		"**policy_xml**(",
		"Fetch the raw Classic policy XML document.",
		"**Returns:**",
		"**Params:**",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("missing %q in:\n%s", want, md)
		}
	}
	if strings.Contains(md, "```starlark") {
		t.Fatalf("builtin hover should match the Function index card, not a code fence:\n%s", md)
	}
}

func TestHoverMarkdownStarlarkLen(t *testing.T) {
	md := hoverMarkdown(Symbol{Name: "len", Kind: "universe"})
	for _, want := range []string{
		"**len**(x)",
		"Return the number of elements in x.",
		"**Returns:** int",
		"**Params:**",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("missing %q in:\n%s", want, md)
		}
	}
	if strings.Contains(md, "Starlark universe") {
		t.Fatalf("len hover still generic:\n%s", md)
	}
}

func TestUniverseHelpCoversStarlarkUniverse(t *testing.T) {
	cat, err := getCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for name := range starlark.Universe {
		if _, ok := help.LookupStarlarkUniverse(name); !ok {
			t.Errorf("missing help for Universe %q", name)
		}
		sym, ok := cat.lookup(name)
		if !ok {
			t.Errorf("catalog missing %q", name)
			continue
		}
		md := hoverMarkdown(sym)
		if strings.TrimSpace(md) == "" || strings.Contains(md, "Starlark universe\n") {
			t.Errorf("generic/empty hover for %q (%s):\n%s", name, sym.Kind, md)
		}
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
