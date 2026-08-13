package buddy

import (
	"os"
	"path/filepath"
	"testing"
)

// profileAmpersandFixture loads testdata/profile_ampersand.xml: a stripped Classic
// profile whose nested payloads plist has one Edge MCX search URL with an inner
// &amp; (a real "&" after plist decode). The outer payloads blob must keep &amp;amp;.
func profileAmpersandFixture(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "profile_ampersand.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := extractPayloadsEncodedFromProfileXML(raw); err != nil {
		t.Fatalf("testdata/profile_ampersand.xml: %v", err)
	}
	return raw
}
