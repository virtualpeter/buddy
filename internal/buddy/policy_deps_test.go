package buddy

import (
	"fmt"
	"strings"
	"testing"
)

func TestRemapPolicyScriptsByName_rewritesIDs(t *testing.T) {
	xml := []byte(`<policy><scripts>
<script><id>1</id><name>Install App</name><priority>Before</priority></script>
<script><id>2</id><name>Cleanup</name><priority>After</priority></script>
</scripts></policy>`)
	out, err := remapNamedIDBlocks(xml, policyScriptBlockRe, "script", func(name string) (string, error) {
		switch name {
		case "Install App":
			return "100", nil
		case "Cleanup":
			return "200", nil
		default:
			return "", fmt.Errorf("missing")
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "<id>100</id>") || !strings.Contains(s, "<id>200</id>") {
		t.Fatalf("ids not remapped: %s", s)
	}
	if strings.Contains(s, "<id>1</id>") || strings.Contains(s, "<id>2</id>") {
		t.Fatalf("old ids remain: %s", s)
	}
}

func TestRemapPolicyScriptsByName_missingFails(t *testing.T) {
	xml := []byte(`<policy><scripts>
<script><id>1</id><name>Missing Script</name><priority>Before</priority></script>
</scripts></policy>`)
	_, err := remapNamedIDBlocks(xml, policyScriptBlockRe, "script", func(name string) (string, error) {
		return "", fmt.Errorf("no script found with name %q", name)
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Missing Script") {
		t.Fatalf("error should list name: %v", err)
	}
}

func TestRemapPolicyCategoryOnDest_sameSessionKeeps(t *testing.T) {
	xml := []byte(`<policy><general><name>P</name><category><id>3</id><name>Apps</name></category></general></policy>`)
	out, err := RemapPolicyCategoryOnDest("", xml, "", true)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "<id>3</id>") || !strings.Contains(s, "Apps") {
		t.Fatalf("same-session should keep category: %s", out)
	}
	if strings.Count(s, "<category>") != 1 || strings.Count(s, "</category>") != 1 {
		t.Fatalf("expected one well-formed category block: %s", s)
	}
}

func TestRemapPolicyCategoryOnDest_uncategorizedIsOneBlock(t *testing.T) {
	xml := []byte(`<os_x_configuration_profile><general><name>P</name><category><id>-1</id><name>No category assigned</name></category><uuid>u</uuid></general></os_x_configuration_profile>`)
	out, err := RemapPolicyCategoryOnDest("", xml, "", true)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Count(s, "<category>") != 1 || strings.Count(s, "</category>") != 1 {
		t.Fatalf("expected one category block, not a split leftover id: %s", s)
	}
	if strings.Contains(s, "</category><id>") {
		t.Fatalf("opening-tag-only replace leftover: %s", s)
	}
	want := "<category><id>-1</id><name>No category assigned</name></category>"
	if !strings.Contains(s, want) {
		t.Fatalf("missing uncategorized block: %s", s)
	}
}

func TestRemapPolicyCategoryOnDest_selfClosingCategory(t *testing.T) {
	xml := []byte(`<policy><general><name>P</name><category/><uuid>u</uuid></general></policy>`)
	out, err := RemapPolicyCategoryOnDest("", xml, "-1", false)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Contains(s, "<category/>") {
		t.Fatalf("self-closing category should be replaced: %s", s)
	}
	if strings.Count(s, "<category>") != 1 || strings.Count(s, "</category>") != 1 {
		t.Fatalf("expected one category block: %s", s)
	}
}

func TestRemapPolicyCategoryOnDest_overrideUncategorized(t *testing.T) {
	xml := []byte(`<policy><general><name>P</name><category><id>3</id><name>Apps</name></category></general></policy>`)
	out, err := RemapPolicyCategoryOnDest("", xml, "-1", false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "<id>-1</id>") {
		t.Fatalf("expected uncategorized: %s", out)
	}
}
