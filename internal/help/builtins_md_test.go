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
	if !strings.Contains(md, "## POLICY — read") {
		t.Fatalf("missing policy section:\n%s", md[:500])
	}
	if !strings.Contains(md, "### `policy_get_id`") {
		t.Fatal("missing policy_get_id")
	}
	if !strings.Contains(md, "### `group_list_smart`") {
		t.Fatal("missing group_list_smart")
	}
	if !strings.Contains(md, "### `smart_group_create`") {
		t.Fatal("missing smart_group_create")
	}
	if strings.Contains(md, "```\nbuddy built-in") {
		t.Fatal("should not dump raw help as one code block")
	}
}
