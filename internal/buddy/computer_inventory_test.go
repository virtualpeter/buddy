package buddy

import "testing"

func TestNormalizeComputerSections(t *testing.T) {
	got := NormalizeComputerSections([]string{"general", "HARDWARE,storage", " GENERAL "})
	want := []string{"GENERAL", "HARDWARE", "STORAGE"}
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v want %#v", got, want)
		}
	}
}

func TestComputerInventoryPath(t *testing.T) {
	path, err := computerInventoryPath("42", nil)
	if err != nil {
		t.Fatal(err)
	}
	if path != "/api/v3/computers-inventory/42" {
		t.Fatalf("path = %q", path)
	}
	path, err = computerInventoryPath("42", []string{"hardware", "GENERAL"})
	if err != nil {
		t.Fatal(err)
	}
	if path != "/api/v3/computers-inventory/42?section=HARDWARE&section=GENERAL" {
		t.Fatalf("path = %q", path)
	}
	if _, err := computerInventoryPath("0", nil); err == nil {
		t.Fatal("expected invalid id error")
	}
}
