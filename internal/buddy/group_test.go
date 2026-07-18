package buddy

import (
	"encoding/json"
	"testing"
)

func TestNormalizeAssignmentIDs_EmptyIsNonNilSlice(t *testing.T) {
	out := normalizeAssignmentIDs(nil)
	if out == nil {
		t.Fatal("expected non-nil empty slice so JSON encodes as [] not null")
	}
	if len(out) != 0 {
		t.Fatalf("len = %d", len(out))
	}
	raw, err := json.Marshal(map[string][]string{"assignments": out})
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"assignments":[]}` {
		t.Fatalf("got %s", raw)
	}
}

func TestParseGroupCreateResult(t *testing.T) {
	got, err := parseGroupCreateResult([]byte(`{"id":"42","href":"https://example/api/v3/computer-groups/smart-groups/42"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "42" {
		t.Fatalf("id = %q", got.ID)
	}
	if got.Href == "" {
		t.Fatal("expected href")
	}
	if _, err := parseGroupCreateResult([]byte(`{"href":"/x"}`)); err == nil {
		t.Fatal("expected error for missing id")
	}
}
