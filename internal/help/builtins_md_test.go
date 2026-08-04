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
	if !strings.Contains(md, `<h3 id="policies"><code>policies</code></h3>`) {
		t.Fatal("missing policies")
	}
	if !strings.Contains(md, `<h3 id="policy"><code>policy</code></h3>`) {
		t.Fatal("missing policy")
	}
	if !strings.Contains(md, `<h3 id="smart_groups"><code>smart_groups</code></h3>`) {
		t.Fatal("missing smart_groups")
	}
	if !strings.Contains(md, `<h3 id="smart_group_create"><code>smart_group_create</code></h3>`) {
		t.Fatal("missing smart_group_create")
	}
	if strings.Contains(md, "```\nbuddy built-in") {
		t.Fatal("should not dump raw help as one code block")
	}
}
