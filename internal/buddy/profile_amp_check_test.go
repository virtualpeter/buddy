package buddy

import (
	"os"
	"strings"
	"testing"
)

func TestProfile18PreparePayloadsAmpEntityCounts(t *testing.T) {
	raw, err := os.ReadFile("/tmp/p18-live.xml")
	if err != nil {
		t.Skip("no fixture")
	}
	prepared, err := PrepareProfileXMLForCreate(raw, ProfilePrepareSpec{Name: "clone-test"})
	if err != nil {
		t.Fatal(err)
	}
	srcEnc, _ := extractPayloadsEncodedFromProfileXML(raw)
	dstEnc, _ := extractPayloadsEncodedFromProfileXML(prepared)
	t.Logf("source: &amp;amp;=%d under-encoded=%v", strings.Count(srcEnc, "&amp;amp;"), hasUnderEncodedAmpersandsInEncodedPayloads(srcEnc))
	t.Logf("prepared: &amp;amp;=%d under-encoded=%v", strings.Count(dstEnc, "&amp;amp;"), hasUnderEncodedAmpersandsInEncodedPayloads(dstEnc))
	eligible, reason := ProfileACSWritePreferred(prepared)
	if !eligible {
		t.Skip("fixture not MCX ampersand profile:", reason)
	}
	if strings.Contains(dstEnc, "<string>") || strings.Contains(dstEnc, "<plist") {
		t.Fatal("encoded payloads contains raw inner plist tags (not entity-encoded)")
	}
}

func TestHasUnderEncodedAmpersandsInEncodedPayloads(t *testing.T) {
	cases := []struct {
		s    string
		want bool
	}{
		{"searchTerms}&amp;amp;{google", false},
		{"searchTerms}&amp;{google", true},
		{"&lt;string&gt;upload&amp;amp;FORM&lt;/string&gt;", false},
		{"&lt;string&gt;upload&amp;FORM&lt;/string&gt;", true},
		{"&amp;lt;plist", false},
	}
	for _, tc := range cases {
		if got := hasUnderEncodedAmpersandsInEncodedPayloads(tc.s); got != tc.want {
			t.Fatalf("%q: got %v want %v", tc.s, got, tc.want)
		}
	}
}

func TestHasBareAmpersandInXML(t *testing.T) {
	if !hasBareAmpersandInXML(`<string>a&b</string>`) {
		t.Fatal("expected bare amp")
	}
	if hasBareAmpersandInXML(`<string>a&amp;b</string>`) {
		t.Fatal("entity amp should be ok")
	}
	if hasBareAmpersandInXML(`&lt;string&gt;a&amp;amp;b&lt;/string&gt;`) {
		t.Fatal("outer double-encoded should be ok for bare-amp scan")
	}
}

func TestPrepareProfileXMLForClassicWrite_RejectsUnderEncoded(t *testing.T) {
	// Inner plist with bare & embedded once → outer &amp; only (under-encoded for Jamf).
	innerRaw := `<?xml version="1.0"?><plist version="1.0"><dict><key>u</key><string>a&b</string></dict></plist>`
	under := []byte(`<?xml version="1.0"?><os_x_configuration_profile><general><payloads>` + xmlEmbedPayloadsPlist(innerRaw) + `</payloads></general></os_x_configuration_profile>`)
	_, err := prepareProfileXMLForClassicWrite(under)
	if err == nil {
		t.Fatal("expected reject under-encoded payloads")
	}
}

func TestPrepareProfileXMLForClassicWrite_RejectsBareMCXAmpersand(t *testing.T) {
	profileXML := []byte(`<?xml version="1.0"?><os_x_configuration_profile><general>
<payloads>` + xmlEncodePayload(mcxPayloadsPlistWithAmpersand()) + `</payloads>
</general></os_x_configuration_profile>`)
	_, err := prepareProfileXMLForClassicWrite(profileXML)
	if err == nil {
		t.Fatal("expected reject bare MCX ampersand before Classic write")
	}
	if !strings.Contains(err.Error(), "bare '&'") {
		t.Fatalf("error should mention bare amp: %v", err)
	}
}
