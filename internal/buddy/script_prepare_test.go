package buddy

import (
	"testing"
)

func TestPrepareScriptForCreate_stripsIDSetsName(t *testing.T) {
	src := &ScriptDetail{
		ID:             "42",
		Name:           "Old",
		CategoryID:     "3",
		CategoryName:   "Apps",
		ScriptContents: "echo hi",
		Priority:       "BEFORE",
	}
	// Same-session: keep category id without needing live lookup.
	out, err := PrepareScriptForCreate(src, ScriptPrepareSpec{
		Name:             "New",
		SameSession:      true,
		DestCred:         "",
		SourceCategoryID: "3",
		SourceCategoryName: "Apps",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.ID != "" {
		t.Fatalf("id should be cleared, got %q", out.ID)
	}
	if out.Name != "New" {
		t.Fatalf("name: %q", out.Name)
	}
	if out.CategoryID != "3" {
		t.Fatalf("same-session should keep category id, got %q", out.CategoryID)
	}
	if out.ScriptContents != "echo hi" || out.Priority != "BEFORE" {
		t.Fatalf("contents/priority not preserved: %+v", out)
	}
}

func TestPrepareScriptForCreate_requiresName(t *testing.T) {
	_, err := PrepareScriptForCreate(&ScriptDetail{Name: "x"}, ScriptPrepareSpec{})
	if err == nil {
		t.Fatal("expected error")
	}
}
