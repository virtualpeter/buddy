package buddy

import (
	"encoding/xml"
	"io"
	"strings"
	"testing"

	"howett.net/plist"
)

func assertWellFormedXML(t *testing.T, s string) {
	t.Helper()
	dec := xml.NewDecoder(strings.NewReader(s))
	for {
		_, err := dec.Token()
		if err == io.EOF {
			return
		}
		if err != nil {
			t.Fatalf("malformed XML: %v\n---\n%s", err, s)
		}
	}
}

func TestXMLEscape_DelimiterOrderAndRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"amp_first", "a&b", "a&amp;b"},
		{"lt_gt", "a<b>c", "a&lt;b&gt;c"},
		{"all", `<&">'`, `&lt;&amp;"&gt;'`},
		{"already_amp", "a&amp;b", "a&amp;amp;b"}, // encode again doubles — intentional for raw text
		{"empty", "", ""},
		{"no_special", "plain", "plain"},
		{"nested_xml", `<plist><dict><key>a&b</key></dict></plist>`,
			`&lt;plist&gt;&lt;dict&gt;&lt;key&gt;a&amp;b&lt;/key&gt;&lt;/dict&gt;&lt;/plist&gt;`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := XMLEscape(tc.raw)
			if got != tc.want {
				t.Fatalf("XMLEscape: got %q want %q", got, tc.want)
			}
			// Escaped form must be safe as element text inside a wrapper.
			wrapped := "<root>" + got + "</root>"
			assertWellFormedXML(t, wrapped)
			if XMLUnescape(got) != tc.raw {
				t.Fatalf("unescape round-trip: got %q want %q", XMLUnescape(got), tc.raw)
			}
		})
	}
}

func TestReplacePayloadsContent_EscapesDelimiters(t *testing.T) {
	profile := []byte(`<?xml version="1.0"?><os_x_configuration_profile><general/><payloads>OLD</payloads></os_x_configuration_profile>`)
	var buf strings.Builder
	if err := plist.NewEncoder(&buf).Encode(map[string]interface{}{
		"k": "a&b<c>",
	}); err != nil {
		t.Fatal(err)
	}
	inner := buf.String()
	assertWellFormedXML(t, inner)

	out, err := replacePayloadsContent(profile, inner)
	if err != nil {
		t.Fatal(err)
	}
	assertWellFormedXML(t, string(out))
	// payloads body must be entity-encoded, not raw nested tags
	if !strings.Contains(string(out), "&lt;") || !strings.Contains(string(out), "&amp;") {
		t.Fatalf("expected entity-encoded payloads, got:\n%s", out)
	}
	if strings.Contains(string(out), "<plist") {
		t.Fatalf("raw <plist> leaked into profile XML:\n%s", out)
	}
	extracted, err := extractPayloadsFromProfileXML(out)
	if err != nil {
		t.Fatal(err)
	}
	assertWellFormedXML(t, extracted)
	var decoded interface{}
	if err := plist.NewDecoder(strings.NewReader(extracted)).Decode(&decoded); err != nil {
		t.Fatal(err)
	}
	m := decoded.(map[string]interface{})
	if m["k"] != "a&b<c>" {
		t.Fatalf("payload value = %v", m["k"])
	}
}

func TestReplaceMCXDict_AcceptsFullPlistEncodeOutput(t *testing.T) {
	// Outer payloads plist with MCX-style domain + empty settings dict.
	base := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>PayloadContent</key><array><dict>
<key>PayloadType</key><string>com.apple.ManagedClient.preferences</string>
<key>PayloadContent</key><dict>
<key>com.example.app</key><dict>
<key>mcx_preference_settings</key><dict/>
</dict>
</dict>
</dict></array>
</dict></plist>`
	assertWellFormedXML(t, base)

	// Simulate plist_encode(dict) — full document, not a bare <dict>.
	var enc strings.Builder
	if err := plist.NewEncoder(&enc).Encode(map[string]interface{}{
		"SomeKey":   "value & <special>",
		"OtherKey":  "a<b>c",
		"AmpEntity": "keep &amp; literal",
	}); err != nil {
		t.Fatal(err)
	}
	fullPlist := enc.String()
	if !strings.Contains(fullPlist, "<plist") {
		t.Fatal("expected full plist document from encoder")
	}

	updated, err := ReplaceMCXDict(base, "com.example.app", fullPlist)
	if err != nil {
		t.Fatal(err)
	}
	assertWellFormedXML(t, updated)
	if strings.Contains(updated, "<?xml") && strings.Count(updated, "<?xml") > 1 {
		t.Fatalf("full plist declaration was spliced into payloads:\n%s", updated)
	}
	if strings.Contains(updated, "<plist") && strings.Count(updated, "<plist") > 1 {
		t.Fatalf("nested <plist> left in payloads (not normalised):\n%s", updated)
	}
	if !strings.Contains(updated, "&amp;") || !strings.Contains(updated, "&lt;") {
		t.Fatalf("expected escaped special chars in settings dict:\n%s", updated)
	}

	var root interface{}
	if err := plist.NewDecoder(strings.NewReader(updated)).Decode(&root); err != nil {
		t.Fatalf("updated payloads not a valid plist: %v\n%s", err, updated)
	}
}

func TestReplaceMCXDict_DomainWithAmpersand(t *testing.T) {
	base := `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict>
<key>com.example&amp;co</key><dict>
<key>mcx_preference_settings</key><dict><key>old</key><string>1</string></dict>
</dict>
</dict></plist>`
	fragment := `<dict><key>new</key><string>a&amp;b</string></dict>`
	updated, err := ReplaceMCXDict(base, "com.example&co", fragment)
	if err != nil {
		t.Fatal(err)
	}
	assertWellFormedXML(t, updated)
	if !strings.Contains(updated, "<key>new</key>") {
		t.Fatalf("settings not replaced:\n%s", updated)
	}
}

func TestNormaliseDictFile_PreservesEscapedContent(t *testing.T) {
	full := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict><key>a&amp;b</key><string>&lt;hi&gt;</string></dict></plist>`
	got, err := normaliseDictFile([]byte(full))
	if err != nil {
		t.Fatal(err)
	}
	want := `<dict><key>a&amp;b</key><string>&lt;hi&gt;</string></dict>`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	assertWellFormedXML(t, got)
}
