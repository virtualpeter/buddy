package buddy

import (
	"reflect"
	"testing"
)

func TestExtractVersion(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"1.2.3", "1.2.3"},
		{"130.0.6723.59", "130.0.6723.59"},
		{"GoogleChrome-130.0.6723.59.pkg", "130.0.6723.59"},
		{"App-1.2.3-arm64.pkg", "1.2.3"},
		{"App-1.2.3-amd64.pkg", "1.2.3"},
		{"App 1.2.3.pkg", "1.2.3"},
		{"App_1.2.3.pkg", "1.2.3"},
		{"Thing-2.0.dmg", "2.0"},
		{"no version here", ""},
		{"", ""},
		{"  9.8.7  ", "9.8.7"},
		{"install-chrome-121.0", "121.0"},
	}
	for _, tt := range tests {
		if got := ExtractVersion(tt.in); got != tt.want {
			t.Errorf("ExtractVersion(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.2.0", "1.2.0", 0},
		{"1.2", "1.2.0", 0},
		{"1.9", "1.10", -1},
		{"1.10", "1.9", 1},
		{"130.0.6723.59", "130.0.6723.58", 1},
		{"Chrome-1.2.0.pkg", "Chrome-1.10.0.pkg", -1},
		{"", "1.0", -1},
		{"1.0", "", 1},
		{"", "", 0},
		{"nope", "1.0", -1},
	}
	for _, tt := range tests {
		if got := CompareVersions(tt.a, tt.b); got != tt.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestSortByVersion(t *testing.T) {
	in := []string{"App-1.10.0.pkg", "App-1.2.0.pkg", "noversion", "App-1.9.0.pkg"}
	asc, err := SortByVersion(in, "asc")
	if err != nil {
		t.Fatal(err)
	}
	wantAsc := []string{"App-1.2.0.pkg", "App-1.9.0.pkg", "App-1.10.0.pkg", "noversion"}
	if !reflect.DeepEqual(asc, wantAsc) {
		t.Errorf("asc = %v, want %v", asc, wantAsc)
	}

	desc, err := SortByVersion(in, "desc")
	if err != nil {
		t.Fatal(err)
	}
	// Unversioned before versioned for desc.
	wantDesc := []string{"noversion", "App-1.10.0.pkg", "App-1.9.0.pkg", "App-1.2.0.pkg"}
	if !reflect.DeepEqual(desc, wantDesc) {
		t.Errorf("desc = %v, want %v", desc, wantDesc)
	}

	if _, err := SortByVersion(in, "sideways"); err == nil {
		t.Error("expected error for invalid order")
	}
}

func TestSortPackageRecordsByVersion(t *testing.T) {
	pkgs := []PackageRecord{
		{ID: "1", PackageName: "Chrome-120.0.pkg"},
		{ID: "2", PackageName: "Chrome-121.0.pkg"},
		{ID: "3", PackageName: "other"},
	}
	if err := SortPackageRecordsByVersion(pkgs, "desc"); err != nil {
		t.Fatal(err)
	}
	// empty first on desc, then highest version
	if pkgs[0].ID != "3" || pkgs[1].ID != "2" || pkgs[2].ID != "1" {
		t.Errorf("got order %v, %v, %v", pkgs[0].ID, pkgs[1].ID, pkgs[2].ID)
	}
}
