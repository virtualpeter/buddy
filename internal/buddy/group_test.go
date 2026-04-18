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
