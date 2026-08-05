package buddy

import (
	"strings"
	"testing"
)

func TestParseMacAppDetail_Wrapped(t *testing.T) {
	body := []byte(`{
  "mac_application": {
    "general": {
      "id": 42,
      "name": "TextWrangler.app",
      "version": "5.5.2",
      "bundle_id": "com.barebones.textwrangler",
      "url": "https://itunes.apple.com/app/id404010395",
      "is_free": true
    },
    "scope": {"all_computers": false},
    "self_service": {"install_button_text": "Install"}
  }
}`)
	d, err := parseMacAppDetail(body)
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != 42 || d.Name != "TextWrangler.app" || d.Version != "5.5.2" {
		t.Fatalf("got id=%d name=%q version=%q", d.ID, d.Name, d.Version)
	}
	if d.BundleID != "com.barebones.textwrangler" || !d.IsFree {
		t.Fatalf("bundle/free: %q %v", d.BundleID, d.IsFree)
	}
	if len(d.General) == 0 || len(d.Scope) == 0 {
		t.Fatal("expected raw general/scope preserved")
	}
}

func TestParseMacAppDetail_Unwrapped(t *testing.T) {
	body := []byte(`{
  "general": {"id": 7, "name": "Foo", "version": "1.0", "bundle_id": "com.foo", "url": "https://example.com"}
}`)
	d, err := parseMacAppDetail(body)
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != 7 || d.Name != "Foo" || d.BundleID != "com.foo" {
		t.Fatalf("got %+v", d)
	}
}

func TestMacAppCreateSpecValidation(t *testing.T) {
	_, err := MacAppCreate("", Opts{SkipConfirm: true}, MacAppCreateSpec{Name: "x"}, nil)
	if err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("expected required-fields error, got %v", err)
	}
}

func TestMacAppSummaryMatches(t *testing.T) {
	s := MacAppSummary{ID: 1, Name: "TextWrangler.app", Version: "5.5.2", BundleID: "com.barebones.textwrangler"}
	if !macAppSummaryMatches(s, "") {
		t.Fatal("empty filter should match")
	}
	if !macAppSummaryMatches(s, "text") {
		t.Fatal("name substring")
	}
	if !macAppSummaryMatches(s, "5.5") {
		t.Fatal("version substring")
	}
	if !macAppSummaryMatches(s, "barebones") {
		t.Fatal("bundle_id substring")
	}
	if macAppSummaryMatches(s, "chrome") {
		t.Fatal("should not match")
	}
}
