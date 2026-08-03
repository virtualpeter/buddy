package buddy

import (
	"strings"
	"testing"
)

func TestEncodeDictMapToXML(t *testing.T) {
	xml, err := EncodeDictMapToXML(map[string]any{
		"PayloadIdentifier": "com.google.chrome",
		"PayloadType":       "com.google.chrome",
		"Key":               "v",
	})
	if err != nil {
		t.Fatal(err)
	}
	if xml == "" || xml[:5] != "<dict" {
		t.Fatalf("want bare dict, got %q", xml)
	}
}

func TestPatchProfilePayload_Offline(t *testing.T) {
	payloadsInner := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>PayloadContent</key>
<array>
<dict>
<key>PayloadIdentifier</key><string>com.example.test</string>
<key>PayloadType</key><string>com.example.test</string>
<key>PayloadUUID</key><string>keep-uuid</string>
<key>SomeKey</key><string>Old</string>
</dict>
</array>
</dict></plist>`
	escaped := strings.ReplaceAll(payloadsInner, "&", "&amp;")
	escaped = strings.ReplaceAll(escaped, "<", "&lt;")
	escaped = strings.ReplaceAll(escaped, ">", "&gt;")
	profileXML := `<?xml version="1.0"?><os_x_configuration_profile>
<general><id>1</id><name>t</name></general>
<payloads>` + escaped + `</payloads>
</os_x_configuration_profile>`

	dictXML, err := EncodeDictMapToXML(map[string]any{
		"SomeKey": "New",
	})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := PatchProfilePayload([]byte(profileXML), "com.example.test", dictXML)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := ExtractPayloadsFromProfileXML(updated)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw, "New") || strings.Contains(raw, ">Old<") {
		t.Fatalf("patch failed: %s", raw)
	}
	if !strings.Contains(raw, "keep-uuid") {
		t.Fatalf("PayloadUUID not preserved: %s", raw)
	}
	ids, err := PlistPayloadIdentifiers(raw)
	if err != nil || len(ids) != 1 || ids[0] != "com.example.test" {
		t.Fatalf("list identifiers: %v %v", ids, err)
	}
}

func TestSmartGroupResolveID_Xor(t *testing.T) {
	if _, err := SmartGroupResolveID("", "", "", nil); err == nil {
		t.Fatal("want error when both empty")
	}
	if _, err := SmartGroupResolveID("", "1", "n", nil); err == nil {
		t.Fatal("want error when both set")
	}
	id, err := SmartGroupResolveID("", "42", "", nil)
	if err != nil || id != "42" {
		t.Fatalf("id=%q err=%v", id, err)
	}
}
