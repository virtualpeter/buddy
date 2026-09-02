package buddy

import (
	"strings"
	"testing"

	"howett.net/plist"
)

func TestParseACSPayloadUUIDFromLegacyHTML(t *testing.T) {
	const want = "e0b7a3a0-1234-5678-9abc-def012345678"
	cases := []string{
		`<input type="hidden" name="FIELD_EXTERNAL_CONFIGPROFILE_ID" value="` + want + `"/>`,
		`FIELD_EXTERNAL_CONFIGPROFILE_ID=` + want + `&o=r`,
		`/api/config-profiles/macos/` + want + `/something`,
	}
	for _, html := range cases {
		got := parseACSPayloadUUIDFromLegacyHTML(html)
		if got != want {
			t.Fatalf("parse=%q want=%q html=%q", got, want, html)
		}
	}
	if got := parseACSPayloadUUIDFromLegacyHTML("<html>no uuid</html>"); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func classicShellWithEmptyMCXDomain(domain string) []byte {
	root := map[string]any{
		"PayloadUUID":    "11111111-1111-1111-1111-111111111111",
		"PayloadType":    "Configuration",
		"PayloadVersion": 1,
		"PayloadContent": []any{
			map[string]any{
				"PayloadType":       managedClientPreferencesType,
				"PayloadVersion":    1,
				"PayloadIdentifier": "com.example.buddy",
				"PayloadUUID":       "22222222-2222-2222-2222-222222222222",
				"PayloadContent": map[string]any{
					domain: map[string]any{
						"Forced": []any{
							map[string]any{
								"mcx_preference_settings": map[string]any{},
							},
						},
					},
				},
			},
		},
	}
	var buf strings.Builder
	if err := plist.NewEncoder(&buf).Encode(root); err != nil {
		panic(err)
	}
	return []byte(`<?xml version="1.0"?><os_x_configuration_profile><general>
<uuid>11111111-1111-1111-1111-111111111111</uuid>
<payloads>` + xmlEncodePayload(buf.String()) + `</payloads>
</general></os_x_configuration_profile>`)
}

func classicShellWithMCXSettings(domain string, settings map[string]any) []byte {
	root := map[string]any{
		"PayloadUUID":    "11111111-1111-1111-1111-111111111111",
		"PayloadType":    "Configuration",
		"PayloadVersion": 1,
		"PayloadContent": []any{
			map[string]any{
				"PayloadType":       managedClientPreferencesType,
				"PayloadVersion":    1,
				"PayloadIdentifier": "com.example.buddy",
				"PayloadUUID":       "22222222-2222-2222-2222-222222222222",
				"PayloadContent": map[string]any{
					domain: map[string]any{
						"Forced": []any{
							map[string]any{
								"mcx_preference_settings": settings,
							},
						},
					},
				},
			},
		},
	}
	var buf strings.Builder
	if err := plist.NewEncoder(&buf).Encode(root); err != nil {
		panic(err)
	}
	return []byte(`<?xml version="1.0"?><os_x_configuration_profile><general>
<uuid>11111111-1111-1111-1111-111111111111</uuid>
<payloads>` + xmlEncodePayload(buf.String()) + `</payloads>
</general></os_x_configuration_profile>`)
}

func TestHydrateClassicProfileXMLFromACSDoc_EmptyGetsFilled(t *testing.T) {
	const domain = "test.profile"
	classic := classicShellWithEmptyMCXDomain(domain)
	doc := &ACSConfigProfileDoc{
		PayloadContent: []ACSPayloadContent{{
			PreferenceDomain: domain,
			Forced: &ACSForced{
				// Jamf UI sometimes uses non-standard <value> tags.
				Plist: `<dict><key>fred</key><value>Nerk</value></dict>`,
			},
		}},
	}
	out, err := hydrateClassicProfileXMLFromACSDoc(classic, doc)
	if err != nil {
		t.Fatal(err)
	}
	payloads, err := extractPayloadsFromProfileXML(out)
	if err != nil {
		t.Fatal(err)
	}
	settings, err := classicMCXSettingsForDomain(payloads, domain)
	if err != nil {
		t.Fatal(err)
	}
	if settings["fred"] != "Nerk" {
		t.Fatalf("hydrated settings=%v", settings)
	}
}

func TestHydrateClassicProfileXMLFromACSDoc_ACSForcedWinsWhenDifferent(t *testing.T) {
	const domain = "test.profile"
	classic := classicShellWithMCXSettings(domain, map[string]any{"fred": "Classic"})
	doc := &ACSConfigProfileDoc{
		PayloadContent: []ACSPayloadContent{{
			PreferenceDomain: domain,
			Forced:           &ACSForced{Plist: `<dict><key>fred</key><string>MacOS</string></dict>`},
		}},
	}
	out, err := hydrateClassicProfileXMLFromACSDoc(classic, doc)
	if err != nil {
		t.Fatal(err)
	}
	payloads, err := extractPayloadsFromProfileXML(out)
	if err != nil {
		t.Fatal(err)
	}
	settings, err := classicMCXSettingsForDomain(payloads, domain)
	if err != nil {
		t.Fatal(err)
	}
	if settings["fred"] != "MacOS" {
		t.Fatalf("ACS Forced should win when settings differ, got %v", settings)
	}
}

func TestHydrateClassicProfileXMLFromACSDoc_EqualSettingsKeepClassic(t *testing.T) {
	const domain = "test.profile"
	classic := classicShellWithMCXSettings(domain, map[string]any{"fred": "Nerk"})
	doc := &ACSConfigProfileDoc{
		PayloadContent: []ACSPayloadContent{{
			PreferenceDomain: domain,
			Forced:           &ACSForced{Plist: `<dict><key>fred</key><string>Nerk</string></dict>`},
		}},
	}
	out, err := hydrateClassicProfileXMLFromACSDoc(classic, doc)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(classic) {
		t.Fatal("equal ACS/Classic settings should leave Classic XML unchanged")
	}
}

func TestMergeACSDocPreservesForcedPlistWhenUnchanged(t *testing.T) {
	raw := `<dict><key>fred</key><value>Nerk</value></dict>`
	current := &ACSConfigProfileDoc{
		PayloadContent: []ACSPayloadContent{{
			PreferenceDomain: "test.profile",
			PayloadUUID:      "aaaa",
			Forced:           &ACSForced{Plist: raw},
		}},
	}
	desired := []MCXPayloadEntry{{
		Domain:       "test.profile",
		SettingsDict: map[string]any{"fred": "Nerk"},
	}}
	merged, err := mergeACSDocWithDesiredEntries(current, desired)
	if err != nil {
		t.Fatal(err)
	}
	if merged.PayloadContent[0].Forced == nil || merged.PayloadContent[0].Forced.Plist != raw {
		t.Fatalf("expected raw Forced preserved, got %#v", merged.PayloadContent[0].Forced)
	}
}

func TestNormalizeJamfForcedPlistForDecode(t *testing.T) {
	in := `<dict><key>a</key><value>b</value></dict>`
	got := normalizeJamfForcedPlistForDecode(in)
	if strings.Contains(got, "<value>") || !strings.Contains(got, "<string>b</string>") {
		t.Fatalf("normalize failed: %s", got)
	}
}

func TestACSPayloadUUIDIsDocument(t *testing.T) {
	if acsPayloadUUIDIsDocument("") || acsPayloadUUIDIsDocument("profile:300") {
		t.Fatal("shell sentinel is not a writable ACS document UUID")
	}
	if !acsPayloadUUIDIsDocument("38FD9351-C4A7-4B8F-BF82-A0B6473BD112") {
		t.Fatal("real UUID should be a document id")
	}
}

func TestACSDocLinkedWithoutPayloadUUID(t *testing.T) {
	if acsDocLinkedWithoutPayloadUUID(nil) || acsDocLinkedWithoutPayloadUUID(&ACSConfigProfileDoc{}) {
		t.Fatal("empty doc should not count as linked")
	}
	if !acsDocLinkedWithoutPayloadUUID(&ACSConfigProfileDoc{Name: "Ampersand Trouble", Level: "SYSTEM"}) {
		t.Fatal("name/level shell should count as linked")
	}
	id := 300
	if !acsDocLinkedWithoutPayloadUUID(&ACSConfigProfileDoc{ClassicProfileID: &id}) {
		t.Fatal("classicProfileId should count as linked")
	}
}
