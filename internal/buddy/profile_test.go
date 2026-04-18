package buddy

import (
	"bytes"
	"strings"
	"testing"
)

func TestExtractPayloadsFromProfileXML(t *testing.T) {
	// Minimal profile XML with entity-encoded payloads
	profileXML := []byte(`<?xml version="1.0"?>
<os_x_configuration_profile>
  <general>
    <id>99</id>
    <uuid>a1b2c3d4-e5f6-7890-abcd-ef1234567890</uuid>
    <payloads>&lt;plist version="1.0"&gt;&lt;dict&gt;&lt;key&gt;PayloadContent&lt;/key&gt;&lt;array&gt;&lt;dict&gt;&lt;key&gt;PayloadIdentifier&lt;/key&gt;&lt;string&gt;com.example.test&lt;/string&gt;&lt;/dict&gt;&lt;/array&gt;&lt;/dict&gt;&lt;/plist&gt;</payloads>
  </general>
</os_x_configuration_profile>`)
	got, err := extractPayloadsFromProfileXML(profileXML)
	if err != nil {
		t.Fatalf("extractPayloadsFromProfileXML: %v", err)
	}
	want := `<plist version="1.0"><dict><key>PayloadContent</key><array><dict><key>PayloadIdentifier</key><string>com.example.test</string></dict></array></dict></plist>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExtractPayloadsFromProfileXML_Missing(t *testing.T) {
	profileXML := []byte(`<os_x_configuration_profile><general><id>1</id></general></os_x_configuration_profile>`)
	_, err := extractPayloadsFromProfileXML(profileXML)
	if err == nil {
		t.Error("expected error when payloads element is missing")
	}
}

func TestPlistPayloadIdentifiers(t *testing.T) {
	plistRaw := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict><key>PayloadContent</key><array>
  <dict><key>PayloadIdentifier</key><string>com.apple.foo</string></dict>
  <dict><key>PayloadIdentifier</key><string>com.google.chrome</string></dict>
</array></dict></plist>`
	ids, err := plistPayloadIdentifiers(plistRaw)
	if err != nil {
		t.Fatalf("plistPayloadIdentifiers: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("got %d identifiers, want 2", len(ids))
	}
	if ids[0] != "com.apple.foo" || ids[1] != "com.google.chrome" {
		t.Errorf("got %v", ids)
	}
}

func TestPlistPayloadIdentifiers_NoPayloadContent(t *testing.T) {
	plistRaw := `<plist version="1.0"><dict><key>foo</key><string>bar</string></dict></plist>`
	ids, err := plistPayloadIdentifiers(plistRaw)
	if err != nil {
		t.Fatalf("plistPayloadIdentifiers: %v", err)
	}
	if ids != nil {
		t.Errorf("expected nil for non-PayloadContent plist, got %v", ids)
	}
}

func TestPlistGetPayloadByDomain(t *testing.T) {
	plistRaw := `<?xml version="1.0"?>
<plist version="1.0"><dict><key>PayloadContent</key><array>
  <dict><key>PayloadIdentifier</key><string>com.apple.foo</string><key>PayloadType</key><string>com.apple.foo</string></dict>
  <dict><key>PayloadIdentifier</key><string>com.google.chrome</string><key>PayloadType</key><string>com.google.chrome</string></dict>
</array></dict></plist>`
	out, err := plistGetPayloadByDomain(plistRaw, "com.google.chrome")
	if err != nil {
		t.Fatalf("plistGetPayloadByDomain: %v", err)
	}
	if !bytes.Contains(out, []byte("com.google.chrome")) {
		t.Errorf("output should contain PayloadIdentifier, got %s", out)
	}
}

func TestPlistGetPayloadByDomain_NotFound(t *testing.T) {
	plistRaw := `<plist version="1.0"><dict><key>PayloadContent</key><array><dict><key>PayloadIdentifier</key><string>com.other</string></dict></array></dict></plist>`
	_, err := plistGetPayloadByDomain(plistRaw, "com.missing")
	if err == nil {
		t.Error("expected error when domain not found")
	}
}

func TestPlistReplacePayloadByDomain(t *testing.T) {
	plistRaw := `<plist version="1.0"><dict><key>PayloadContent</key><array>
  <dict><key>PayloadIdentifier</key><string>com.old</string><key>Value</key><string>before</string></dict>
</array></dict></plist>`
	newPayload := map[string]interface{}{
		"PayloadIdentifier": "com.old",
		"Value":             "after",
	}
	updated, err := plistReplacePayloadByDomain(plistRaw, "com.old", newPayload)
	if err != nil {
		t.Fatalf("plistReplacePayloadByDomain: %v", err)
	}
	if !strings.Contains(updated, "after") {
		t.Errorf("updated plist should contain new value, got %s", updated)
	}
	if !strings.Contains(updated, "com.old") {
		t.Errorf("updated plist should contain PayloadIdentifier, got %s", updated)
	}
}

func TestReplacePayloadsContent(t *testing.T) {
	profileXML := []byte(`<root><payloads>old</payloads></root>`)
	newContent := "new & <plist/>"
	got := replacePayloadsContent(profileXML, newContent)
	if !bytes.Contains(got, []byte("new")) {
		t.Errorf("expected new content, got %s", got)
	}
	if !bytes.Contains(got, []byte("&lt;")) {
		t.Errorf("expected entity-encoded angle brackets, got %s", got)
	}
}

func TestDictFileToPlistMap_BareDict(t *testing.T) {
	data := []byte(`<dict><key>PayloadIdentifier</key><string>com.test</string></dict>`)
	m, err := dictFileToPlistMap(data)
	if err != nil {
		t.Fatalf("dictFileToPlistMap: %v", err)
	}
	if m["PayloadIdentifier"] != "com.test" {
		t.Errorf("got %v", m)
	}
}

func TestDictFileToPlistMap_FullPlist(t *testing.T) {
	data := []byte(`<?xml version="1.0"?><plist version="1.0"><dict><key>a</key><integer>1</integer></dict></plist>`)
	m, err := dictFileToPlistMap(data)
	if err != nil {
		t.Fatalf("dictFileToPlistMap: %v", err)
	}
	if _, ok := m["a"]; !ok {
		t.Errorf("missing key a, got %v", m)
	}
}

func TestSetRedeployAll_HasElement(t *testing.T) {
	profileXML := []byte(`<root><redeploy_on_update>New</redeploy_on_update></root>`)
	got := SetRedeployAll(profileXML)
	if !bytes.Contains(got, []byte("<redeploy_on_update>All</redeploy_on_update>")) {
		t.Errorf("expected redeploy_on_update to be All, got %s", got)
	}
	if bytes.Contains(got, []byte("New")) {
		t.Errorf("expected original value replaced, got %s", got)
	}
}

func TestSetRedeployAll_NoElement(t *testing.T) {
	profileXML := []byte(`<root><other>x</other></root>`)
	got := SetRedeployAll(profileXML)
	if !bytes.Equal(got, profileXML) {
		t.Errorf("expected unchanged when redeploy_on_update missing, got %s", got)
	}
}

func TestNormaliseDictFile_BareDict(t *testing.T) {
	data := []byte(`<dict><key>PayloadIdentifier</key><string>com.test</string></dict>`)
	got, err := normaliseDictFile(data)
	if err != nil {
		t.Fatalf("normaliseDictFile: %v", err)
	}
	if got != string(data) {
		t.Errorf("got %q, want %q", got, data)
	}
}

func TestNormaliseDictFile_FullPlist(t *testing.T) {
	data := []byte(`<?xml version="1.0"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict><key>a</key><integer>1</integer></dict></plist>`)
	got, err := normaliseDictFile(data)
	if err != nil {
		t.Fatalf("normaliseDictFile: %v", err)
	}
	want := `<dict><key>a</key><integer>1</integer></dict>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNormaliseDictFile_NoDict(t *testing.T) {
	data := []byte(`<plist version="1.0"><array><string>x</string></array></plist>`)
	_, err := normaliseDictFile(data)
	if err == nil {
		t.Error("expected error when no dict at root")
	}
	if !strings.Contains(err.Error(), "<dict>") {
		t.Errorf("error = %v", err)
	}
}

func TestNormaliseDictFile_MalformedXMLDecl(t *testing.T) {
	data := []byte(`<?xml version="1.0")
<dict></dict>`)
	_, err := normaliseDictFile(data)
	if err == nil {
		t.Error("expected error for malformed XML declaration")
	}
	if !strings.Contains(err.Error(), "XML declaration") {
		t.Errorf("error = %v", err)
	}
}
