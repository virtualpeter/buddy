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

func TestParseDoesNotesAndWrappedParams(t *testing.T) {
	src := `SAMPLE
  sample_put(group_id, body, tenant?, surface?, auth?)
      Does: Replace a smart group's full JSON document.
      → None
      Params: group_id: str; body: dict (same shape as
        smart_group(id=…)); tenant?/surface?/auth?: session overrides.
      Notes: PUT /api/v3/computer-groups/smart-groups/{id}.
        SkipConfirm.
`
	_, flat := parseBuiltinsCatalog(src)
	if len(flat) != 1 {
		t.Fatalf("got %d funcs", len(flat))
	}
	fn := flat[0]
	if fn.Name != "sample_put" {
		t.Fatalf("name = %q", fn.Name)
	}
	if fn.Summary != "Replace a smart group's full JSON document." {
		t.Fatalf("summary = %q", fn.Summary)
	}
	if fn.Returns != "None" {
		t.Fatalf("returns = %q", fn.Returns)
	}
	if !strings.Contains(fn.Params, "same shape as smart_group") {
		t.Fatalf("params should join wrap: %q", fn.Params)
	}
	if !strings.Contains(fn.Notes, "SkipConfirm") {
		t.Fatalf("notes should join wrap: %q", fn.Notes)
	}
	help := FormatBuiltinHelp(fn)
	for _, want := range []string{"Does:", "→ None", "Params:", "Notes:"} {
		if !strings.Contains(help, want) {
			t.Fatalf("FormatBuiltinHelp missing %q:\n%s", want, help)
		}
	}
}

func TestBuiltinsMarkdownRendersDoesNotCodeFence(t *testing.T) {
	src := `SAMPLE
  sample_put(group_id, body)
      Does: Replace a smart group's full JSON document.
      → None
      Params: group_id: str; body: dict.
      Notes: PUT the full document.
`
	md := formatBuiltinsMarkdown(src)
	if !strings.Contains(md, "Replace a smart group's full JSON document.") {
		t.Fatalf("missing Does paragraph:\n%s", md)
	}
	if !strings.Contains(md, "<dt>Returns</dt>") || !strings.Contains(md, "<dd>None</dd>") {
		t.Fatalf("missing returns:\n%s", md)
	}
	if !strings.Contains(md, "<dt>Params</dt>") ||
		!strings.Contains(md, "<code>group_id</code>") ||
		!strings.Contains(md, "<code>body</code>") {
		t.Fatalf("missing params:\n%s", md)
	}
	if !strings.Contains(md, "<dt>Notes</dt>") || !strings.Contains(md, "PUT the full document.") {
		t.Fatalf("missing notes:\n%s", md)
	}
	if !strings.Contains(md, `<article class="fn-entry">`) {
		t.Fatalf("missing fn-entry card:\n%s", md)
	}
	if strings.Contains(md, "```\n      Does:") || strings.Contains(md, "```\n      Notes:") {
		t.Fatalf("Does/Notes dumped as code fence:\n%s", md)
	}
}

func TestSmartGroupPutHasDoes(t *testing.T) {
	fn, ok := LookupBuiltin("smart_group_put")
	if !ok {
		t.Fatal("smart_group_put missing")
	}
	if fn.Summary == "" || fn.Summary == "None" {
		t.Fatalf("expected Does sentence, summary=%q returns=%q", fn.Summary, fn.Returns)
	}
	if !strings.Contains(strings.ToLower(fn.Summary), "smart group") {
		t.Fatalf("summary = %q", fn.Summary)
	}
	if fn.Returns != "None" {
		t.Fatalf("returns = %q", fn.Returns)
	}
	help := FormatBuiltinHelp(fn)
	if !strings.Contains(help, "Does:") || !strings.Contains(help, "Params:") {
		t.Fatalf("help = %q", help)
	}
}

func TestEveryBuiltinHasDoes(t *testing.T) {
	_, flat := parseBuiltinsCatalog(Builtins)
	var missing []string
	for _, f := range flat {
		if f.Summary == "" || f.Summary == f.Returns {
			missing = append(missing, f.Name+" summary="+f.Summary+" returns="+f.Returns)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("%d builtins missing Does: (summary fell back to returns):\n  %s", len(missing), strings.Join(missing, "\n  "))
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

func TestFormatBuiltinHoverMarkdown(t *testing.T) {
	fn, ok := LookupBuiltin("policy_xml")
	if !ok {
		t.Fatal("missing policy_xml")
	}
	md := FormatBuiltinHoverMarkdown(fn)
	for _, want := range []string{
		"**policy_xml**(id, tenant?, surface?, auth?)",
		"Fetch the raw Classic policy XML document.",
		"**Returns:** str (raw policy XML)",
		"**Params:**",
		"`id` — int/str (policy ID)",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("missing %q in:\n%s", want, md)
		}
	}
	if strings.Contains(md, "```") {
		t.Fatalf("hover should not fence the signature:\n%s", md)
	}
	if strings.Contains(md, "value: str") {
		t.Fatalf("returns still has value: prefix:\n%s", md)
	}
}
