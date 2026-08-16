package buddy

import (
	"strings"
	"testing"
)

func TestAmpFixturePreparePayloadsAmpEntityCounts(t *testing.T) {
	raw := profileAmpersandFixture(t)
	prepared, err := PrepareProfileXMLForCreate(raw, ProfilePrepareSpec{Name: "clone-test"})
	if err != nil {
		t.Fatal(err)
	}
	srcEnc, err := extractPayloadsEncodedFromProfileXML(raw)
	if err != nil {
		t.Fatal(err)
	}
	dstEnc, err := extractPayloadsEncodedFromProfileXML(prepared)
	if err != nil {
		t.Fatal(err)
	}
	srcAmpAmp := strings.Count(srcEnc, "&amp;amp;")
	dstAmpAmp := strings.Count(dstEnc, "&amp;amp;")
	if srcAmpAmp < 1 {
		t.Fatal("fixture outer payloads missing &amp;amp;")
	}
	if srcAmpAmp != dstAmpAmp {
		t.Fatalf("prepare changed &amp;amp; count: source=%d prepared=%d", srcAmpAmp, dstAmpAmp)
	}
	if hasUnderEncodedAmpersandsInEncodedPayloads(srcEnc) || hasUnderEncodedAmpersandsInEncodedPayloads(dstEnc) {
		t.Fatal("payloads under-encode nested plist ampersands")
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

func TestApplyFormEncodeBareAmpInString(t *testing.T) {
	if got := applyFormEncodeBareAmpInString("Here &"); got != "Here &amp;" {
		t.Fatalf("got %q", got)
	}
	if got := applyFormEncodeBareAmpInString("a&amp;b"); got != "a&amp;b" {
		t.Fatalf("Apply-form should stay %q", got)
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
	if _, err := prepareProfileXMLForClassicSync(profileXML); err != nil {
		t.Fatalf("Classic sync after ACS should allow decoded URL '&': %v", err)
	}
	if profileXMLRequiresAmpersandClassicRetention(profileXML) {
		t.Fatal("literal URL '&' should not require Classic Apply-form retention")
	}
}

func TestProfileXMLDropsBareAmpKeys(t *testing.T) {
	withAmp := []byte(`<?xml version="1.0"?><os_x_configuration_profile><general>
<payloads>` + xmlEncodePayload(mcxPayloadsPlistWithAmpersand()) + `</payloads>
</general></os_x_configuration_profile>`)
	markerOnly := []byte(`<?xml version="1.0"?><os_x_configuration_profile><general>
<payloads>` + xmlEncodePayload(mcxPayloadsPlistWithURL("https://example.com")) + `</payloads>
</general></os_x_configuration_profile>`)
	// Same key, but the '&' value is replaced with a no-'&' URL — Classic 200s and hides the key.
	if !profileXMLDropsBareAmpKeys(withAmp, markerOnly) {
		t.Fatal("replacing an '&' value with a no-'&' string should count as drop")
	}
	if profileXMLDropsBareAmpKeys(withAmp, withAmp) {
		t.Fatal("same XML should not count as dropping amp keys")
	}
	keepAmp := []byte(`<?xml version="1.0"?><os_x_configuration_profile><general>
<payloads>` + xmlEncodePayload(mcxPayloadsPlistWithURL("https://example.com/q?a=1&b=2")) + `</payloads>
</general></os_x_configuration_profile>`)
	if profileXMLDropsBareAmpKeys(withAmp, keepAmp) {
		t.Fatal("same key still containing '&' should not count as drop")
	}
	emptyMCX := []byte(`<?xml version="1.0"?><os_x_configuration_profile><general>
<payloads>` + xmlEncodePayload(mcxPayloadsPlistWithSettings(map[string]any{"Buddy": "x"})) + `</payloads>
</general></os_x_configuration_profile>`)
	if !profileXMLDropsBareAmpKeys(withAmp, emptyMCX) {
		t.Fatal("omitting the '&' key should count as drop")
	}
}

func TestProfileXMLRequiresAmpersandClassicRetention_ApplyForm(t *testing.T) {
	profileXML := []byte(`<?xml version="1.0"?><os_x_configuration_profile><general>
<payloads>` + xmlEncodePayload(mcxPayloadsPlistWithApplyFormAmp()) + `</payloads>
</general></os_x_configuration_profile>`)
	if !profileXMLRequiresAmpersandClassicRetention(profileXML) {
		t.Fatal("Apply-form &amp; should require Classic retention")
	}
}
