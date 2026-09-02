package help

import (
	"strings"
	"testing"
)

func TestBuiltinsMarkdownHasSectionsAndFuncs(t *testing.T) {
	md := BuiltinsMarkdown()
	if !strings.Contains(md, "# Function index") {
		t.Fatal("missing title")
	}
	if !strings.Contains(md, `<h2 id="section-policy">POLICY</h2>`) {
		t.Fatalf("missing policy section:\n%s", md[:800])
	}
	if !strings.Contains(md, `<h3 id="policies">`) || !strings.Contains(md, `<b class="fn-name">policies</b>`) {
		t.Fatal("missing policies")
	}
	if !strings.Contains(md, `<h3 id="policy">`) || !strings.Contains(md, `<b class="fn-name">policy</b>`) {
		t.Fatal("missing policy")
	}
	if !strings.Contains(md, `<h3 id="smart_groups">`) || !strings.Contains(md, `<b class="fn-name">smart_groups</b>`) {
		t.Fatal("missing smart_groups")
	}
	if !strings.Contains(md, `<h3 id="smart_group_create">`) || !strings.Contains(md, `<b class="fn-name">smart_group_create</b>`) {
		t.Fatal("missing smart_group_create")
	}
	if strings.Contains(md, "```\nbuddy built-in") {
		t.Fatal("should not dump raw help as one code block")
	}
}

func TestFormatSignatureHTML(t *testing.T) {
	got := formatSignatureHTML("policy_set_ss", "policy_set_ss(id, version?, group_ids?)")
	want := `<b class="fn-name">policy_set_ss</b>(id, version?, group_ids?)`
	if got != want {
		t.Fatalf("got %s", got)
	}
	got = formatSignatureHTML("policy", "policy(id=?) | policy(trigger=?, tenant?, surface?, auth?)")
	want = `<b class="fn-name">policy</b>(id=?) | <b class="fn-name">policy</b>(trigger=?, tenant?, surface?, auth?)`
	if got != want {
		t.Fatalf("got %s", got)
	}
}

func TestSplitParamItems(t *testing.T) {
	got := splitParamItems("id: int/str (policy ID).")
	if len(got) != 1 || got[0] != "id: int/str (policy ID)." {
		t.Fatalf("single param: %#v", got)
	}
	got = splitParamItems("group_id: str; body: dict.")
	if len(got) != 2 || got[0] != "group_id: str" || got[1] != "body: dict." {
		t.Fatalf("semicolon params: %#v", got)
	}
	got = splitParamItems("keywords?: str. exclude_ids?: list of int. ids?: bool.")
	if len(got) != 3 {
		t.Fatalf("period params: %#v", got)
	}
	got = splitParamItems("pass either id= (int/str) or trigger= (str), not both.")
	if len(got) != 1 {
		t.Fatalf("prose params: %#v", got)
	}
	got = splitParamItems("policy_id: int/str; at least one of enable?/name?/trigger?/frequency?.")
	if len(got) != 1 {
		t.Fatalf("trailing prose after param: %#v", got)
	}
}

func TestBuiltinsMarkdownHasDoesAndNotesLabels(t *testing.T) {
	md := BuiltinsMarkdown()
	if !strings.Contains(md, "<dt>Params</dt>") {
		t.Fatal("expected Params definition term")
	}
	if !strings.Contains(md, `<article class="fn-entry">`) {
		t.Fatal("expected fn-entry cards")
	}
	if !strings.Contains(md, "Replace a smart group's full JSON document") {
		t.Fatal("expected smart_group_put Does paragraph")
	}
	if strings.Contains(md, "```\n      Does:") || strings.Contains(md, "```\n      Notes:") {
		t.Fatal("Does/Notes dumped as code fence")
	}
}

func TestPolicyXMLCardLayout(t *testing.T) {
	md := BuiltinsMarkdown()
	idx := strings.Index(md, `<h3 id="policy_xml">`)
	if idx < 0 {
		t.Fatal("missing policy_xml")
	}
	art := strings.LastIndex(md[:idx], `<article class="fn-entry">`)
	end := strings.Index(md[idx:], `</article>`)
	if art < 0 || end < 0 {
		t.Fatal("policy_xml not wrapped in fn-entry")
	}
	chunk := md[art : idx+end+len("</article>")]
	for _, want := range []string{
		`<h3 id="policy_xml"><code class="fn-sig"><b class="fn-name">policy_xml</b>(id, tenant?, surface?, auth?)</code></h3>`,
		`Fetch the raw Classic policy XML document.`,
		`<dt>Returns</dt>`,
		`<dd>str (raw policy XML)</dd>`,
		`<dt>Params</dt>`,
		`<code>id</code> — int/str (policy ID).`,
	} {
		if !strings.Contains(chunk, want) {
			t.Fatalf("missing %q in:\n%s", want, chunk)
		}
	}
	if strings.Contains(chunk, "```") {
		t.Fatalf("signature should not be a fence:\n%s", chunk)
	}
	if strings.Contains(chunk, "value: str") {
		t.Fatalf("returns still has value: prefix:\n%s", chunk)
	}
}
