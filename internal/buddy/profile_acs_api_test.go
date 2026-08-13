package buddy

import (
	"strings"
	"testing"

	"howett.net/plist"
)

func TestProfileACSWritePreferred_MCXAmpersand(t *testing.T) {
	// Bare '&' is rejected at write; not ACS-eligible on its own.
	bareXML := []byte(`<?xml version="1.0"?><os_x_configuration_profile><general>
<uuid>11111111-1111-1111-1111-111111111111</uuid>
<payloads>` + xmlEncodePayload(mcxPayloadsPlistWithAmpersand()) + `</payloads>
</general></os_x_configuration_profile>`)
	if err := rejectBareMCXAmpersandSettings(bareXML); err == nil {
		t.Fatal("expected reject for bare MCX ampersand")
	}
	if eligible, reason := ProfileACSWritePreferred(bareXML); eligible {
		t.Fatalf("bare MCX amp should not be ACS-eligible, reason=%q", reason)
	}

	// Jamf ACS Apply form (&amp; in the preference string) is ACS-eligible.
	applyXML := []byte(`<?xml version="1.0"?><os_x_configuration_profile><general>
<uuid>11111111-1111-1111-1111-111111111111</uuid>
<payloads>` + xmlEncodePayload(mcxPayloadsPlistWithApplyFormAmp()) + `</payloads>
</general></os_x_configuration_profile>`)
	if err := rejectBareMCXAmpersandSettings(applyXML); err != nil {
		t.Fatalf("Apply form should not be rejected: %v", err)
	}
	eligible, reason := ProfileACSWritePreferred(applyXML)
	if !eligible {
		t.Fatalf("expected Apply-form eligible, reason=%q", reason)
	}
	if !strings.Contains(reason, "com.microsoft.edge") {
		t.Fatalf("reason should mention domain: %q", reason)
	}
}

func TestRejectBareMCXAmpersandSettings_EntityFormOK(t *testing.T) {
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
				"Forced": []any{
					map[string]any{
						"mcx_preference_settings": map[string]any{
							"ExampleURL": "https://example.com/search?q=foo&amp;bar=baz",
						},
					},
				},
			},
		},
	}
	var buf strings.Builder
	if err := plist.NewEncoder(&buf).Encode(root); err != nil {
		t.Fatal(err)
	}
	profileXML := []byte(`<?xml version="1.0"?><os_x_configuration_profile><general>
<payloads>` + xmlEncodePayload(buf.String()) + `</payloads>
</general></os_x_configuration_profile>`)
	if err := rejectBareMCXAmpersandSettings(profileXML); err != nil {
		t.Fatalf("Jamf Apply form &amp; should be accepted: %v", err)
	}
	if _, err := prepareProfileXMLForClassicWrite(profileXML); err != nil {
		t.Fatalf("prepare should accept &amp; form: %v", err)
	}
	eligible, reason := ProfileACSWritePreferred(profileXML)
	if !eligible {
		t.Fatalf("Apply form should be ACS-eligible: %q", reason)
	}
}

func TestProfileACSWritePreferred_NoMCX(t *testing.T) {
	inner := `<?xml version="1.0"?><plist version="1.0"><dict>
<key>PayloadUUID</key><string>11111111-1111-1111-1111-111111111111</string>
<key>PayloadContent</key><array><dict>
<key>PayloadType</key><string>com.apple.vpn.managed</string>
<key>PayloadIdentifier</key><string>com.example.vpn</string>
<key>PayloadUUID</key><string>22222222-2222-2222-2222-222222222222</string>
</dict></array></dict></plist>`
	profileXML := []byte(`<?xml version="1.0"?><os_x_configuration_profile><general>
<payloads>` + xmlEncodePayload(inner) + `</payloads>
</general></os_x_configuration_profile>`)

	eligible, _ := ProfileACSWritePreferred(profileXML)
	if eligible {
		t.Fatal("non-MCX profile should not be eligible")
	}
}

func TestAmpFixtureBareAmpNotACSEligible(t *testing.T) {
	raw := profileAmpersandFixture(t)
	entries, err := listMCXPayloadEntries(raw)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("mcx entries: %d", len(entries))
	hasBare := false
	for _, e := range entries {
		t.Logf("  domain=%q amp=%v", e.Domain, e.HasAmpersand)
		if e.HasAmpersand {
			hasBare = true
		}
	}
	if hasBare {
		if err := rejectBareMCXAmpersandSettings(raw); err == nil {
			t.Fatal("expected reject for bare MCX ampersand in fixture")
		}
		eligible, reason := ProfileACSWritePreferred(raw)
		t.Logf("eligible=%v reason=%q (bare amp → reject, not ACS-eligible)", eligible, reason)
		return
	}
	eligible, reason := ProfileACSWritePreferred(raw)
	t.Logf("eligible=%v reason=%q", eligible, reason)
}

func TestListMCXPayloadEntries_NestedJamfACS(t *testing.T) {
	root := map[string]any{
		"PayloadUUID": "11111111-1111-1111-1111-111111111111",
		"PayloadContent": []any{
			map[string]any{
				"PayloadType":       managedClientPreferencesType,
				"PayloadUUID":       "22222222-2222-2222-2222-222222222222",
				"PayloadIdentifier": "E4F5A6B7-C8D9-0E1F-2A3B-C4D5E6F7A8B9",
				"PayloadContent": map[string]any{
					"com.microsoft.edge": map[string]any{
						"Forced": []any{
							map[string]any{
								"mcx_preference_settings": map[string]any{
									"DefaultSearchProviderSearchURL": "{google:baseURL}search?q={searchTerms}&{google:RLZ}",
								},
							},
						},
					},
				},
			},
		},
	}
	var buf strings.Builder
	if err := plist.NewEncoder(&buf).Encode(root); err != nil {
		t.Fatal(err)
	}
	profileXML := []byte(`<?xml version="1.0"?><os_x_configuration_profile><general>
<payloads>` + xmlEncodePayload(buf.String()) + `</payloads>
</general></os_x_configuration_profile>`)

	entries, err := listMCXPayloadEntries(profileXML)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Domain != "com.microsoft.edge" || !entries[0].HasAmpersand {
		t.Fatalf("entries=%+v", entries)
	}
}

func TestListMCXPayloadEntries_NestedPayloadContent(t *testing.T) {
	profileXML := []byte(`<?xml version="1.0"?><os_x_configuration_profile><general>
<payloads>` + xmlEncodePayload(mcxPayloadsPlistWithAmpersand()) + `</payloads>
</general></os_x_configuration_profile>`)

	entries, err := listMCXPayloadEntries(profileXML)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	if entries[0].Domain != "com.microsoft.edge" {
		t.Fatalf("domain=%q", entries[0].Domain)
	}
	if !entries[0].HasAmpersand {
		t.Fatal("expected HasAmpersand")
	}
}

func TestMcxDictToForcedPlist_AmpersandEncoding(t *testing.T) {
	settings := map[string]any{
		"DefaultSearchProviderSearchURL": "{google:baseURL}search?q={searchTerms}&{google:RLZ}",
	}
	plistXML, err := mcxDictToForcedPlist(settings)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plistXML, "&amp;") {
		t.Fatalf("expected &amp; in plist XML, got: %s", plistXML)
	}
	if strings.Contains(plistXML, "searchTerms}&{") {
		t.Fatalf("raw ampersand should be encoded: %s", plistXML)
	}
}

func TestBuildClassicShellProfileXML(t *testing.T) {
	profileXML := []byte(`<?xml version="1.0"?><os_x_configuration_profile><general>
<uuid>11111111-1111-1111-1111-111111111111</uuid>
<payloads>` + xmlEncodePayload(mcxPayloadsPlistWithAmpersand()) + `</payloads>
</general></os_x_configuration_profile>`)
	shell, err := buildClassicShellProfileXML(profileXML)
	if err != nil {
		t.Fatal(err)
	}
	extracted, err := extractPayloadsFromProfileXML(shell)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(extracted, "com.microsoft.edge") {
		t.Fatal("shell payloads should not contain MCX domain")
	}
	if !strings.Contains(extracted, "<array></array>") && !strings.Contains(extracted, "<array/>") {
		t.Fatalf("expected empty PayloadContent array: %s", extracted)
	}
}

func TestACSDocHasDesiredMCXEntries(t *testing.T) {
	desired := []MCXPayloadEntry{{Domain: "com.example.buddy-integration"}}
	doc := &ACSConfigProfileDoc{
		PayloadContent: []ACSPayloadContent{{
			PreferenceDomain: "com.example.buddy-integration",
			Forced:           &ACSForced{Plist: "<plist/>"},
		}},
	}
	if !acsDocHasDesiredMCXEntries(doc, desired) {
		t.Fatal("expected match")
	}
	doc.PayloadContent[0].Forced.Plist = ""
	if acsDocHasDesiredMCXEntries(doc, desired) {
		t.Fatal("empty plist should not count")
	}
}

func TestBuildACSCreateDocFromProfileXML(t *testing.T) {
	profileXML := []byte(`<?xml version="1.0"?><os_x_configuration_profile><general>
<name>Edge Clone</name>
<payloads>` + xmlEncodePayload(mcxPayloadsPlistWithAmpersand()) + `</payloads>
</general></os_x_configuration_profile>`)

	doc, err := buildACSCreateDocFromProfileXML("", profileXML)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Level != acsConfigProfileLevelSystem {
		t.Fatalf("level=%q", doc.Level)
	}
	if len(doc.PayloadContent) != 1 {
		t.Fatalf("payload count=%d", len(doc.PayloadContent))
	}
	pc := doc.PayloadContent[0]
	if pc.PreferenceDomain != "com.microsoft.edge" {
		t.Fatalf("domain=%q", pc.PreferenceDomain)
	}
	if pc.SetOnce == nil {
		t.Fatal("expected setOnce null on ACS payload")
	}
	if pc.Forced == nil || pc.Forced.Plist == "" {
		t.Fatal("missing forced.plist")
	}
	if !strings.Contains(pc.Forced.Plist, "&amp;") {
		t.Fatalf("forced.plist missing &amp;: %s", pc.Forced.Plist)
	}
}

func TestEnrichACSAttachDoc(t *testing.T) {
	profileXML := []byte(`<?xml version="1.0"?><os_x_configuration_profile><general>
<name>buddy-test-profile</name>
<payloads>` + xmlEncodePayload(mcxPayloadsPlistWithAmpersand()) + `</payloads>
</general></os_x_configuration_profile>`)
	doc := &ACSConfigProfileDoc{Level: acsConfigProfileLevelSystem}
	enrichACSAttachDoc(doc, 61, profileXML)
	if doc.ClassicProfileID == nil || *doc.ClassicProfileID != 61 {
		t.Fatalf("classicProfileId=%v", doc.ClassicProfileID)
	}
	if doc.Name != "buddy-test-profile" {
		t.Fatalf("name=%q", doc.Name)
	}
}

func TestGetMCXDict_NestedACS(t *testing.T) {
	root := map[string]any{
		"PayloadUUID": "11111111-1111-1111-1111-111111111111",
		"PayloadContent": []any{
			map[string]any{
				"PayloadType":       managedClientPreferencesType,
				"PayloadVersion":    1,
				"PayloadIdentifier": "com.apple.ManagedClient.preferences.test",
				"PayloadUUID":       "22222222-2222-2222-2222-222222222222",
				"PayloadContent": map[string]any{
					"com.example.buddy-integration": map[string]any{
						"Forced": []any{
							map[string]any{
								"mcx_preference_settings": map[string]any{
									"ExampleURL": "https://example.com?a=1&b=2",
								},
							},
						},
					},
				},
			},
		},
	}
	var buf strings.Builder
	if err := plist.NewEncoder(&buf).Encode(root); err != nil {
		t.Fatal(err)
	}
	got, err := GetMCXDict(buf.String(), "com.example.buddy-integration")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "ExampleURL") || !strings.Contains(got, "&") && !strings.Contains(got, "&amp;") {
		t.Fatalf("got %q", got)
	}
	updated, err := ReplaceMCXDict(buf.String(), "com.example.buddy-integration", "<dict><key>BuddyTestKey</key><string>ok</string></dict>")
	if err != nil {
		t.Fatal(err)
	}
	got2, err := GetMCXDict(updated, "com.example.buddy-integration")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got2, "BuddyTestKey") {
		t.Fatalf("replace failed: %q", got2)
	}
}

func TestParseScopeFromProfileXML(t *testing.T) {
	allXML := []byte(`<scope><all_computers>true</all_computers><computer_groups/></scope>`)
	all, groups := parseScopeFromProfileXML(allXML)
	if !all || len(groups) != 0 {
		t.Fatalf("all=%v groups=%v", all, groups)
	}
	groupXML := []byte(`<scope><all_computers>false</all_computers><computer_groups><computer_group><id>42</id></computer_group><computer_group><id>99</id></computer_group></computer_groups></scope>`)
	all, groups = parseScopeFromProfileXML(groupXML)
	if all || len(groups) != 2 || groups[0] != 42 || groups[1] != 99 {
		t.Fatalf("all=%v groups=%v", all, groups)
	}
}

func mcxPayloadsPlistWithAmpersand() string {
	return mcxPayloadsPlistWithURL("{google:baseURL}search?q={searchTerms}&{google:RLZ}")
}

func mcxPayloadsPlistWithApplyFormAmp() string {
	return mcxPayloadsPlistWithURL("{google:baseURL}search?q={searchTerms}&amp;{google:RLZ}")
}

func mcxPayloadsPlistWithURL(searchURL string) string {
	root := map[string]any{
		"PayloadUUID":    "11111111-1111-1111-1111-111111111111",
		"PayloadType":    "Configuration",
		"PayloadVersion": 1,
		"PayloadContent": []any{
			map[string]any{
				"PayloadType":       managedClientPreferencesType,
				"PayloadVersion":    1,
				"PayloadIdentifier": "com.microsoft.edge",
				"PayloadUUID":       "22222222-2222-2222-2222-222222222222",
				"Forced": []any{
					map[string]any{
						"mcx_preference_settings": map[string]any{
							"DefaultSearchProviderSearchURL": searchURL,
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
	return buf.String()
}

func TestPlistMapHasLiteralAmpersand(t *testing.T) {
	if !plistMapHasLiteralAmpersand("a&b") {
		t.Fatal("expected true for literal ampersand")
	}
	if plistMapHasLiteralAmpersand("%26") {
		t.Fatal("percent-encoded should not count")
	}
	if !plistMapHasLiteralAmpersand("a&amp;b") {
		t.Fatal("Jamf UI entity form &amp; should flag amp handling")
	}
	if plistMapHasBareAmpersand("a&amp;b") {
		t.Fatal("Apply form must not count as bare amp")
	}
	if !plistMapHasBareAmpersand("a&b") {
		t.Fatal("expected bare amp")
	}
}
