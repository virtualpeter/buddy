package buddy

import (
	"encoding/json"
	"strings"
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

func TestParseSmartGroupMembership(t *testing.T) {
	got, err := parseSmartGroupMembership([]byte(`{"members":[1,2,3]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].ID != "1" || got[1].ID != "2" || got[2].ID != "3" {
		t.Fatalf("ids = %#v", got)
	}
	if got[0].Name != "" || got[0].Serial != "" {
		t.Fatalf("expected empty name/serial, got %#v", got[0])
	}

	empty, err := parseSmartGroupMembership([]byte(`{"members":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if empty == nil || len(empty) != 0 {
		t.Fatalf("empty members: %#v", empty)
	}

	missing, err := parseSmartGroupMembership([]byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if missing == nil || len(missing) != 0 {
		t.Fatalf("missing members: %#v", missing)
	}

	skipZero, err := parseSmartGroupMembership([]byte(`{"members":[0,-1,5]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(skipZero) != 1 || skipZero[0].ID != "5" {
		t.Fatalf("skip non-positive: %#v", skipZero)
	}

	if _, err := parseSmartGroupMembership([]byte(`not-json`)); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestSmartGroupSetCriteriaRejectsNonArray(t *testing.T) {
	err := SmartGroupSetCriteria("unused", Opts{SkipConfirm: true}, "1", []byte(`{"not":"array"}`), nil)
	if err == nil || !strings.Contains(err.Error(), "JSON array") {
		t.Fatalf("expected JSON array error, got %v", err)
	}
	err = SmartGroupSetCriteria("unused", Opts{SkipConfirm: true}, "1", []byte(`  `), nil)
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("expected empty error, got %v", err)
	}
}

