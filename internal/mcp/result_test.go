package mcp

import (
	"encoding/json"
	"testing"

	"buddy/internal/buddy"
)

func TestToolResultAlwaysObject(t *testing.T) {
	items := []map[string]string{{"id": "1", "name": "a"}}
	n := len(items)
	_, structured, err := toolOK(items, &n)
	if err != nil {
		t.Fatal(err)
	}
	tr, ok := structured.(ToolResult)
	if !ok {
		t.Fatalf("structured type %T", structured)
	}
	if !tr.OK || tr.Count == nil || *tr.Count != 1 {
		t.Fatalf("%+v", tr)
	}
	b, err := json.Marshal(tr)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) == 0 || b[0] != '{' {
		t.Fatalf("want object, got %s", b)
	}
	var decoded map[string]any
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, ok := decoded["value"]; !ok {
		t.Fatal("missing value")
	}
}

func TestSummarizeAuth(t *testing.T) {
	info := map[string]interface{}{
		"authenticationType": "CLIENT_CREDENTIALS",
		"account": map[string]interface{}{
			"username":     "buddy-cli",
			"accessLevel":  "FullAccess",
			"privilegeSet": "CUSTOM",
			"privilegesBySite": map[string]interface{}{
				"-1": []interface{}{"Read Computers", "Update Policies"},
			},
		},
		"sites": []interface{}{map[string]interface{}{"id": "-1", "name": "NONE"}},
	}
	s := buddy.CompactAuthInfo(info)
	if s["username"] != "buddy-cli" {
		t.Fatalf("%v", s)
	}
	if s["privilegeCount"] != 2 {
		t.Fatalf("privilegeCount=%v", s["privilegeCount"])
	}
}
