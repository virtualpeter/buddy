package help

import (
	"strings"
	"testing"
)

func TestBuiltinsMarkdownHasSectionsAndFuncs(t *testing.T) {
	md := BuiltinsMarkdown()
	if !strings.Contains(md, "# Starlark builtins") {
		t.Fatal("missing title")
	}
	if !strings.Contains(md, "## POLICY") {
		t.Fatalf("missing policy section:\n%s", md[:500])
	}
	if !strings.Contains(md, "### `policies`") {
		t.Fatal("missing policies")
	}
	if !strings.Contains(md, "### `policy`") {
		t.Fatal("missing policy")
	}
	if !strings.Contains(md, "### `smart_groups`") {
		t.Fatal("missing smart_groups")
	}
	if !strings.Contains(md, "### `smart_group_create`") {
		t.Fatal("missing smart_group_create")
	}
	if strings.Contains(md, "```\nbuddy built-in") {
		t.Fatal("should not dump raw help as one code block")
	}
}
