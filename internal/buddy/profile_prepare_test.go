package buddy

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"howett.net/plist"
)

const testProfileXMLForPrepare = `<?xml version="1.0" encoding="UTF-8"?>
<os_x_configuration_profile>
  <general>
    <id>99</id>
    <name>Source Profile</name>
    <uuid>11111111-1111-1111-1111-111111111111</uuid>
    <payloads>` + "&lt;plist version=&quot;1.0&quot;&gt;&lt;dict&gt;&lt;key&gt;PayloadUUID&lt;/key&gt;&lt;string&gt;11111111-1111-1111-1111-111111111111&lt;/string&gt;&lt;key&gt;PayloadContent&lt;/key&gt;&lt;array&gt;&lt;dict&gt;&lt;key&gt;PayloadIdentifier&lt;/key&gt;&lt;string&gt;com.example.a&lt;/string&gt;&lt;key&gt;PayloadUUID&lt;/key&gt;&lt;string&gt;22222222-2222-2222-2222-222222222222&lt;/string&gt;&lt;/dict&gt;&lt;dict&gt;&lt;key&gt;PayloadIdentifier&lt;/key&gt;&lt;string&gt;com.example.b&lt;/string&gt;&lt;key&gt;PayloadUUID&lt;/key&gt;&lt;string&gt;33333333-3333-3333-3333-333333333333&lt;/string&gt;&lt;/dict&gt;&lt;/array&gt;&lt;/dict&gt;&lt;/plist&gt;" + `</payloads>
  </general>
  <scope>
    <all_computers>true</all_computers>
    <all_jss_users>false</all_jss_users>
    <computers/>
    <buildings/>
    <departments/>
    <computer_groups/>
    <jss_users/>
    <jss_user_groups/>
    <limitations><users/><user_groups/><network_segments/><ibeacons/></limitations>
    <exclusions>
      <computers/><buildings/><departments/><computer_groups/>
      <users/><user_groups/><network_segments/><ibeacons/>
      <jss_users/><jss_user_groups/>
    </exclusions>
  </scope>
</os_x_configuration_profile>`

func TestRandomizeProfileUUIDs_AllPayloadUUIDsChange(t *testing.T) {
	out, err := RandomizeProfileUUIDs([]byte(testProfileXMLForPrepare))
	if err != nil {
		t.Fatal(err)
	}
	newGeneral, err := extractGeneralUUID(out)
	if err != nil {
		t.Fatal(err)
	}
	if newGeneral == "11111111-1111-1111-1111-111111111111" {
		t.Fatal("general uuid should change")
	}
	payloads, err := extractPayloadsFromProfileXML(out)
	if err != nil {
		t.Fatal(err)
	}
	uuids, err := PlistCollectPayloadUUIDs(payloads)
	if err != nil {
		t.Fatal(err)
	}
	if len(uuids) != 3 {
		t.Fatalf("want 3 payload uuids, got %v", uuids)
	}
	for _, u := range uuids {
		switch u {
		case "11111111-1111-1111-1111-111111111111",
			"22222222-2222-2222-2222-222222222222",
			"33333333-3333-3333-3333-333333333333":
			t.Fatalf("old uuid still present: %s", u)
		}
	}
	// Root PayloadUUID should match general when it previously matched.
	if uuids[0] != newGeneral {
		t.Fatalf("root PayloadUUID should stay synced with general uuid: root=%s general=%s", uuids[0], newGeneral)
	}
	ids, err := plistPayloadIdentifiers(payloads)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != "com.example.a" || ids[1] != "com.example.b" {
		t.Fatalf("PayloadIdentifier changed: %v", ids)
	}
}

func TestPrepareProfileXMLForCreate_ThenCategoryRemap(t *testing.T) {
	src := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<os_x_configuration_profile>
  <general>
    <id>99</id>
    <name>Source Profile</name>
    <category><id>-1</id><name>No category assigned</name></category>
    <uuid>11111111-1111-1111-1111-111111111111</uuid>
    <payloads>` + "&lt;plist version=&quot;1.0&quot;&gt;&lt;dict&gt;&lt;key&gt;PayloadUUID&lt;/key&gt;&lt;string&gt;11111111-1111-1111-1111-111111111111&lt;/string&gt;&lt;/dict&gt;&lt;/plist&gt;" + `</payloads>
  </general>
  <scope>
    <all_computers>true</all_computers>
    <all_jss_users>false</all_jss_users>
    <computers/>
    <computer_groups/>
  </scope>
</os_x_configuration_profile>`)
	prepared, err := PrepareProfileXMLForCreate(src, ProfilePrepareSpec{Name: "Cloned"})
	if err != nil {
		t.Fatal(err)
	}
	out, err := RemapPolicyCategoryOnDest("", prepared, "", true)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Contains(s, "<id>99</id>") {
		t.Fatal("profile id should be stripped before remap")
	}
	if strings.Count(s, "<category>") != 1 || strings.Count(s, "</category>") != 1 {
		t.Fatalf("clone path must keep one category block: %s", s)
	}
	if strings.Contains(s, "</category><id>") {
		t.Fatalf("category remap leftover id: %s", s)
	}
	if !strings.Contains(s, "<name>Cloned</name>") {
		t.Fatal("name")
	}
	if !strings.Contains(s, "<all_computers>false</all_computers>") {
		t.Fatal("unassigned scope")
	}
}

func TestPrepareProfileXMLForCreate_UnassignedScopeAndName(t *testing.T) {
	out, err := PrepareProfileXMLForCreate([]byte(testProfileXMLForPrepare), ProfilePrepareSpec{Name: "Cloned"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Contains(s, "<id>99</id>") {
		t.Fatal("profile id should be stripped")
	}
	if !strings.Contains(s, "<name>Cloned</name>") {
		t.Fatal("name not set")
	}
	if !strings.Contains(s, "<all_computers>false</all_computers>") {
		t.Fatal("expected unassigned scope")
	}
	if strings.Contains(s, "<all_computers>true</all_computers>") {
		t.Fatal("should not keep all computers scope")
	}
	general, err := extractGeneralUUID(out)
	if err != nil {
		t.Fatal(err)
	}
	if general == "11111111-1111-1111-1111-111111111111" {
		t.Fatal("general uuid should be randomized")
	}
}

func TestPrepareProfileXMLForCreate_ScopeGroups(t *testing.T) {
	scopeFalse := false
	out, err := PrepareProfileXMLForCreate([]byte(testProfileXMLForPrepare), ProfilePrepareSpec{
		Name:          "Grouped",
		ScopeAll:      &scopeFalse,
		ScopeGroupIDs: []int{10, 20},
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "<id>10</id>") || !strings.Contains(s, "<id>20</id>") {
		t.Fatalf("missing group scope:\n%s", s)
	}
	if strings.Contains(s, "<all_computers>true</all_computers>") {
		t.Fatal("should not scope all computers")
	}
}

func TestPlistCollectPayloadUUIDs_MCXRoot(t *testing.T) {
	plist := `<?xml version="1.0"?><plist version="1.0"><dict>
<key>PayloadUUID</key><string>aaaa-bbbb</string>
<key>com.example.prefs</key><dict><key>mcx_preference_settings</key><dict/></dict>
</dict></plist>`
	uuids, err := PlistCollectPayloadUUIDs(plist)
	if err != nil {
		t.Fatal(err)
	}
	if len(uuids) != 1 || uuids[0] != "aaaa-bbbb" {
		t.Fatalf("got %v", uuids)
	}
	updated, err := plistRemapPayloadUUIDs(plist, "aaaa-bbbb", "cccc-dddd")
	if err != nil {
		t.Fatal(err)
	}
	uuids2, err := PlistCollectPayloadUUIDs(updated)
	if err != nil {
		t.Fatal(err)
	}
	if len(uuids2) != 1 || uuids2[0] != "cccc-dddd" {
		t.Fatalf("got %v", uuids2)
	}
}

func TestPlistRemapPayloadUUIDs_PreservesAmpersandEncoding(t *testing.T) {
	const oldGeneral = "11111111-1111-1111-1111-111111111111"
	const oldPayload = "22222222-2222-2222-2222-222222222222"
	// Jamf-style plist bytes: entity-escaped ampersands and angle brackets inside strings.
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>PayloadUUID</key><string>` + oldGeneral + `</string>
<key>PayloadContent</key><array><dict>
<key>PayloadIdentifier</key><string>com.example&amp;co</string>
<key>PayloadUUID</key><string>` + oldPayload + `</string>
<key>URL</key><string>https://example.com?a=1&amp;b=2</string>
<key>Note</key><string>&lt;tag&gt; &amp; more</string>
</dict></array></dict></plist>`
	newGeneral := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	updated, err := plistRemapPayloadUUIDs(plist, oldGeneral, newGeneral)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(updated, oldGeneral) || strings.Contains(updated, oldPayload) {
		t.Fatalf("old UUIDs still present:\n%s", updated)
	}
	for _, want := range []string{
		`<string>com.example&amp;co</string>`,
		`<string>https://example.com?a=1&amp;b=2</string>`,
		`<string>&lt;tag&gt; &amp; more</string>`,
	} {
		if !strings.Contains(updated, want) {
			t.Fatalf("lost encoded fragment %q in:\n%s", want, updated)
		}
	}
}

func TestRandomizeProfileUUIDs_SkipsUUIDInPayloadValue(t *testing.T) {
	// Config value contains a UUID-shaped string that must not be rewritten (not a PayloadUUID key).
	configUUID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	inner := `<?xml version="1.0"?><plist version="1.0"><dict>
<key>PayloadUUID</key><string>11111111-1111-1111-1111-111111111111</string>
<key>PayloadContent</key><array><dict>
<key>PayloadIdentifier</key><string>com.example.edge</string>
<key>PayloadUUID</key><string>22222222-2222-2222-2222-222222222222</string>
<key>PolicyID</key><string>` + configUUID + `</string>
</dict></array></dict></plist>`
	encodedInner := xmlEncodePayload(inner)
	profileXML := []byte(`<?xml version="1.0"?><os_x_configuration_profile><general>
<uuid>11111111-1111-1111-1111-111111111111</uuid>
<payloads>` + encodedInner + `</payloads>
</general></os_x_configuration_profile>`)

	out, err := RandomizeProfileUUIDs(profileXML)
	if err != nil {
		t.Fatal(err)
	}
	gotEncoded, err := extractPayloadsEncodedFromProfileXML(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotEncoded, configUUID) {
		t.Fatalf("config UUID in payload value was rewritten:\n%s", gotEncoded)
	}
	if strings.Contains(gotEncoded, "11111111-1111-1111-1111-111111111111") ||
		strings.Contains(gotEncoded, "22222222-2222-2222-2222-222222222222") {
		t.Fatalf("PayloadUUID keys were not remapped:\n%s", gotEncoded)
	}
}

func TestRandomizeProfileUUIDs_SyncsPayloadIdentifierWhenUUID(t *testing.T) {
	// MCX-style payloads often use the same UUID for PayloadIdentifier and PayloadUUID.
	const oldGeneral = "93cf2036-b7b6-45b0-a68c-fa652f4232ff"
	const oldNested = "E4F5A6B7-C8D9-0E1F-2A3B-C4D5E6F7A8B9"
	inner := `<?xml version="1.0"?><plist version="1.0"><dict>
<key>PayloadUUID</key><string>` + oldGeneral + `</string>
<key>PayloadContent</key><array><dict>
<key>PayloadIdentifier</key><string>` + oldNested + `</string>
<key>PayloadUUID</key><string>` + oldNested + `</string>
</dict></array></dict></plist>`
	encodedInner := xmlEncodePayload(inner)
	profileXML := []byte(`<?xml version="1.0"?><os_x_configuration_profile><general>
<uuid>` + oldGeneral + `</uuid>
<payloads>` + encodedInner + `</payloads>
</general></os_x_configuration_profile>`)

	out, err := RandomizeProfileUUIDs(profileXML)
	if err != nil {
		t.Fatal(err)
	}
	gotEncoded, err := extractPayloadsEncodedFromProfileXML(out)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(gotEncoded, oldGeneral) || strings.Contains(gotEncoded, oldNested) {
		t.Fatalf("old UUIDs still present:\n%s", gotEncoded)
	}
	extracted, err := extractPayloadsFromProfileXML(out)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]interface{}
	if err := plist.NewDecoder(strings.NewReader(extracted)).Decode(&root); err != nil {
		t.Fatal(err)
	}
	content, _ := root["PayloadContent"].([]interface{})
	if len(content) != 1 {
		t.Fatalf("want 1 payload, got %d", len(content))
	}
	payload, _ := content[0].(map[string]interface{})
	id, _ := payload["PayloadIdentifier"].(string)
	puid, _ := payload["PayloadUUID"].(string)
	if id != puid {
		t.Fatalf("nested PayloadIdentifier %q != PayloadUUID %q", id, puid)
	}
	if id == oldNested {
		t.Fatal("nested payload UUID not remapped")
	}
}

func TestAmpFixtureRandomizePreservesEncoding(t *testing.T) {
	raw := profileAmpersandFixture(t)
	srcEnc, err := extractPayloadsEncodedFromProfileXML(raw)
	if err != nil {
		t.Fatal(err)
	}
	out, err := RandomizeProfileUUIDs(raw)
	if err != nil {
		t.Fatal(err)
	}
	dstEnc, err := extractPayloadsEncodedFromProfileXML(out)
	if err != nil {
		t.Fatal(err)
	}
	strip := func(s string) string { return payloadUUIDInTextRe.ReplaceAllString(s, "") }
	if strip(srcEnc) != strip(dstEnc) {
		t.Fatalf("encoded payloads changed beyond UUID strings\nlen before=%d after=%d", len(srcEnc), len(dstEnc))
	}
}

func TestRandomizeProfileUUIDs_EntityEncodedAmpersandsRoundTrip(t *testing.T) {
	inner := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>PayloadUUID</key><string>11111111-1111-1111-1111-111111111111</string>
<key>PayloadContent</key><array><dict>
<key>PayloadIdentifier</key><string>com.microsoft.edge</string>
<key>PayloadUUID</key><string>22222222-2222-2222-2222-222222222222</string>
<key>Homepages</key><string>https://example.com?foo=1&amp;bar=2</string>
</dict></array></dict></plist>`
	encodedInner := xmlEncodePayload(inner)
	profileXML := []byte(`<?xml version="1.0"?><os_x_configuration_profile><general>
<uuid>11111111-1111-1111-1111-111111111111</uuid>
<payloads>` + encodedInner + `</payloads>
</general></os_x_configuration_profile>`)

	out, err := RandomizeProfileUUIDs(profileXML)
	if err != nil {
		t.Fatal(err)
	}
	gotEncoded, err := extractPayloadsEncodedFromProfileXML(out)
	if err != nil {
		t.Fatal(err)
	}
	stripUUIDs := func(s string) string {
		return payloadUUIDInTextRe.ReplaceAllString(s, "")
	}
	if stripUUIDs(gotEncoded) != stripUUIDs(encodedInner) {
		t.Fatalf("encoded payloads changed beyond UUID remap\nbefore: %s\nafter:  %s", encodedInner, gotEncoded)
	}
	extracted, err := extractPayloadsFromProfileXML(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(extracted, `https://example.com?foo=1&amp;bar=2`) {
		t.Fatalf("ampersand encoding not preserved after UUID remap:\n%s", extracted)
	}
}

func TestAmpFixturePreparedUUIDSync(t *testing.T) {
	raw := profileAmpersandFixture(t)
	out, err := PrepareProfileXMLForCreate(raw, ProfilePrepareSpec{Name: "test-clone"})
	if err != nil {
		t.Fatal(err)
	}
	general, err := extractGeneralUUID(out)
	if err != nil {
		t.Fatal(err)
	}
	extracted, err := extractPayloadsFromProfileXML(out)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]interface{}
	if err := plist.NewDecoder(strings.NewReader(extracted)).Decode(&root); err != nil {
		t.Fatal(err)
	}
	rootUUID, _ := root["PayloadUUID"].(string)
	rootID, _ := root["PayloadIdentifier"].(string)
	if general != rootUUID {
		t.Fatalf("general %q != root PayloadUUID %q", general, rootUUID)
	}
	if rootUUID != rootID {
		t.Fatalf("root PayloadUUID %q != PayloadIdentifier %q", rootUUID, rootID)
	}
	content, _ := root["PayloadContent"].([]interface{})
	payload, _ := content[0].(map[string]interface{})
	nid, _ := payload["PayloadIdentifier"].(string)
	npuid, _ := payload["PayloadUUID"].(string)
	if nid != npuid {
		t.Fatalf("nested PayloadIdentifier %q != PayloadUUID %q", nid, npuid)
	}
}

func TestAmpFixturePreparedPayloadsParse(t *testing.T) {
	raw := profileAmpersandFixture(t)
	out, err := PrepareProfileXMLForCreate(raw, ProfilePrepareSpec{Name: "test-clone"})
	if err != nil {
		t.Fatal(err)
	}
	extracted, err := extractPayloadsFromProfileXML(out)
	if err != nil {
		t.Fatal(err)
	}
	var root interface{}
	if err := plist.NewDecoder(strings.NewReader(extracted)).Decode(&root); err != nil {
		t.Fatalf("decode prepared payloads: %v", err)
	}
}

func TestAmpFixturePreparePreservesJamfAmpersandEncoding(t *testing.T) {
	raw := profileAmpersandFixture(t)
	const needle = "search?q={searchTerms}"
	srcEnc, err := extractPayloadsEncodedFromProfileXML(raw)
	if err != nil {
		t.Fatal(err)
	}
	i := strings.Index(srcEnc, needle)
	if i < 0 {
		t.Fatal("needle not in source")
	}
	srcFrag := srcEnc[i : i+80]

	prepared, err := PrepareProfileXMLForCreate(raw, ProfilePrepareSpec{Name: "clone-test"})
	if err != nil {
		t.Fatal(err)
	}
	dstEnc, err := extractPayloadsEncodedFromProfileXML(prepared)
	if err != nil {
		t.Fatal(err)
	}
	j := strings.Index(dstEnc, needle)
	if j < 0 {
		t.Fatal("needle not in prepared")
	}
	dstFrag := dstEnc[j : j+80]
	if srcFrag != dstFrag {
		t.Fatalf("ampersand encoding changed:\nsource:   %q\nprepared: %q", srcFrag, dstFrag)
	}
}

func TestReplacePayloadsContent_JamfDoubleAmpEncoding(t *testing.T) {
	// Jamf stores nested plist XML inside <payloads> with double-encoded ampersands in URL strings.
	inner := `<?xml version="1.0"?><plist version="1.0"><dict><key>DefaultSearchProviderSearchURL</key><string>{google:baseURL}search?q={searchTerms}&amp;{google:RLZ}</string></dict></plist>`
	encodedInner := xmlEncodePayload(inner) // Jamf outer form: &amp; → &amp;amp;
	if !strings.Contains(encodedInner, "&amp;amp;") {
		t.Fatalf("expected Jamf-style double encoding, got: %s", encodedInner)
	}
	profileXML := []byte(`<?xml version="1.0"?><os_x_configuration_profile><general><payloads>` + encodedInner + `</payloads></general></os_x_configuration_profile>`)

	// Round-trip through extract + replacePayloadsContent (patch/edit path).
	raw, err := extractPayloadsFromProfileXML(profileXML)
	if err != nil {
		t.Fatal(err)
	}
	out, err := replacePayloadsContent(profileXML, raw)
	if err != nil {
		t.Fatal(err)
	}
	gotEnc, err := extractPayloadsEncodedFromProfileXML(out)
	if err != nil {
		t.Fatal(err)
	}
	if gotEnc != encodedInner {
		t.Fatalf("replacePayloadsContent changed Jamf amp encoding\nbefore: %q\nafter:  %q", encodedInner, gotEnc)
	}
}

func TestRepairJamfEncodedPayloadsAmps(t *testing.T) {
	const under = "searchTerms}&amp;{google:RLZ}"
	const fixed = "searchTerms}&amp;amp;{google:RLZ}"
	got := repairJamfEncodedPayloadsAmps(under)
	if got != fixed {
		t.Fatalf("got %q want %q", got, fixed)
	}
	if repairJamfEncodedPayloadsAmps(fixed) != fixed {
		t.Fatal("already-fixed input changed")
	}
}

func TestReembedPayloadsForJamfAPI_FixesUnderEncodedAmp(t *testing.T) {
	// Simulate buddy bug: inner plist with &amp; was embedded with only one XML encoding layer.
	inner := `<?xml version="1.0"?><plist version="1.0"><dict><key>DefaultSearchProviderSearchURL</key><string>{google:baseURL}search?q={searchTerms}&amp;{google:RLZ}</string></dict></plist>`
	brokenOuter := xmlEmbedPayloadsPlist(strings.ReplaceAll(inner, "&amp;", "&")) // raw & in inner → single &amp; outer
	if strings.Contains(brokenOuter, "&amp;amp;") {
		t.Fatal("setup: expected under-encoded outer")
	}
	profileXML := []byte(`<?xml version="1.0"?><os_x_configuration_profile><general><payloads>` + brokenOuter + `</payloads></general></os_x_configuration_profile>`)
	fixed, err := reembedPayloadsForJamfAPI(profileXML)
	if err != nil {
		t.Fatal(err)
	}
	gotEnc, err := extractPayloadsEncodedFromProfileXML(fixed)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateJamfEncodedPayloads(gotEnc); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotEnc, "searchTerms}&amp;amp;{") {
		t.Fatalf("expected Jamf double-encoded amp, got: %s", gotEnc)
	}
}

func TestPlistReencode_JamfDoubleAmpEncoding(t *testing.T) {
	inner := `<?xml version="1.0"?><plist version="1.0"><dict><key>DefaultSearchProviderSearchURL</key><string>{google:baseURL}search?q={searchTerms}&amp;{google:RLZ}</string></dict></plist>`
	encodedInner := xmlEncodePayload(inner)
	profileXML := []byte(`<?xml version="1.0"?><os_x_configuration_profile><general><payloads>` + encodedInner + `</payloads></general></os_x_configuration_profile>`)

	raw, err := extractPayloadsFromProfileXML(profileXML)
	if err != nil {
		t.Fatal(err)
	}
	var root interface{}
	if err := plist.NewDecoder(strings.NewReader(raw)).Decode(&root); err != nil {
		t.Fatal(err)
	}
	var buf strings.Builder
	if err := plist.NewEncoder(&buf).Encode(root); err != nil {
		t.Fatal(err)
	}
	reencoded := buf.String()
	t.Logf("reencoded inner: %q", reencoded)
	out, err := replacePayloadsContent(profileXML, reencoded)
	if err != nil {
		t.Fatal(err)
	}
	gotEnc, _ := extractPayloadsEncodedFromProfileXML(out)
	if !strings.Contains(gotEnc, "searchTerms}&amp;amp;{") {
		t.Fatalf("plist re-encode broke Jamf amp encoding, got: %s", gotEnc[strings.Index(gotEnc, "searchTerms"):strings.Index(gotEnc, "searchTerms")+60])
	}
}

func TestAmpFixtureFullPlistRoundTripPreservesJamfAmp(t *testing.T) {
	raw := profileAmpersandFixture(t)
	srcEnc, err := extractPayloadsEncodedFromProfileXML(raw)
	if err != nil {
		t.Fatal(err)
	}
	inner, err := extractPayloadsFromProfileXML(raw)
	if err != nil {
		t.Fatal(err)
	}
	var root interface{}
	if err := plist.NewDecoder(strings.NewReader(inner)).Decode(&root); err != nil {
		t.Fatal(err)
	}
	reencoded, err := encodePayloadsRoot(root.(map[string]interface{}))
	if err != nil {
		t.Fatal(err)
	}
	out, err := replacePayloadsContent(raw, reencoded)
	if err != nil {
		t.Fatal(err)
	}
	gotEnc, err := extractPayloadsEncodedFromProfileXML(out)
	if err != nil {
		t.Fatal(err)
	}
	const needle = "search?q={searchTerms}"
	if !strings.Contains(srcEnc, needle) {
		t.Fatal("needle missing from fixture")
	}
	i, j := strings.Index(srcEnc, needle), strings.Index(gotEnc, needle)
	if srcEnc[i:i+60] != gotEnc[j:j+60] {
		t.Fatalf("full plist round-trip changed amp encoding\nbefore: %q\nafter:  %q", srcEnc[i:i+60], gotEnc[j:j+60])
	}
}

func TestAmpFixtureMCXDictRoundTripPreservesJamfAmp(t *testing.T) {
	raw := profileAmpersandFixture(t)
	srcEnc, _ := extractPayloadsEncodedFromProfileXML(raw)
	inner, err := extractPayloadsFromProfileXML(raw)
	if err != nil {
		t.Fatal(err)
	}
	mcx, err := getMCXDict(inner, "com.microsoft.edge")
	if err != nil {
		t.Fatal(err)
	}
	updated, err := replaceMCXDict(inner, "com.microsoft.edge", mcx)
	if err != nil {
		t.Fatal(err)
	}
	out, err := replacePayloadsContent(raw, updated)
	if err != nil {
		t.Fatal(err)
	}
	gotEnc, _ := extractPayloadsEncodedFromProfileXML(out)
	const needle = "search?q={searchTerms}"
	i, j := strings.Index(srcEnc, needle), strings.Index(gotEnc, needle)
	if srcEnc[i:i+60] != gotEnc[j:j+60] {
		t.Fatalf("MCX dict round-trip changed amp encoding\nbefore: %q\nafter:  %q", srcEnc[i:i+60], gotEnc[j:j+60])
	}
}

func TestPlistRemapPlusReplacePayloadsContent_BreaksJamfAmp(t *testing.T) {
	raw := profileAmpersandFixture(t)
	srcEnc, _ := extractPayloadsEncodedFromProfileXML(raw)
	inner, err := extractPayloadsFromProfileXML(raw)
	if err != nil {
		t.Fatal(err)
	}
	general, _ := extractGeneralUUID(raw)
	updated, err := plistRemapPayloadUUIDs(inner, general, "00000000-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatal(err)
	}
	out, err := replacePayloadsContent(raw, updated)
	if err != nil {
		t.Fatal(err)
	}
	gotEnc, _ := extractPayloadsEncodedFromProfileXML(out)
	const needle = "search?q={searchTerms}"
	if !strings.Contains(srcEnc, needle) {
		t.Fatal("needle missing from fixture")
	}
	if srcEnc[strings.Index(srcEnc, needle):strings.Index(srcEnc, needle)+60] == gotEnc[strings.Index(gotEnc, needle):strings.Index(gotEnc, needle)+60] {
		t.Skip("remap+replace preserved encoding in this case")
	}
	t.Logf("before: %q", srcEnc[strings.Index(srcEnc, needle):strings.Index(srcEnc, needle)+60])
	t.Logf("after:  %q", gotEnc[strings.Index(gotEnc, needle):strings.Index(gotEnc, needle)+60])
}

func TestAmpFixturePatchPayloadPreservesJamfAmp(t *testing.T) {
	raw := profileAmpersandFixture(t)
	srcEnc, _ := extractPayloadsEncodedFromProfileXML(raw)
	inner, err := extractPayloadsFromProfileXML(raw)
	if err != nil {
		t.Fatal(err)
	}
	mcx, err := getMCXDict(inner, "com.microsoft.edge")
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]interface{}
	if err := plist.NewDecoder(strings.NewReader(mcx)).Decode(&settings); err != nil {
		t.Fatal(err)
	}
	settings["BuddyTestKey"] = true
	dictXML, err := EncodeDictMapToXML(settings)
	if err != nil {
		t.Fatal(err)
	}
	out, err := PatchProfilePayload(raw, "com.microsoft.edge", dictXML)
	if err != nil {
		t.Fatal(err)
	}
	gotEnc, _ := extractPayloadsEncodedFromProfileXML(out)
	const needle = "search?q={searchTerms}"
	si := strings.Index(srcEnc, needle)
	gi := strings.Index(gotEnc, needle)
	if si < 0 || gi < 0 {
		t.Fatal("needle missing")
	}
	if srcEnc[si:si+60] != gotEnc[gi:gi+60] {
		t.Fatalf("patch changed amp encoding\nbefore: %q\nafter:  %q", srcEnc[si:si+60], gotEnc[gi:gi+60])
	}
}

func TestAmpFixtureReplacePayloadPreservesJamfAmp(t *testing.T) {
	raw := profileAmpersandFixture(t)
	srcEnc, _ := extractPayloadsEncodedFromProfileXML(raw)
	inner, err := extractPayloadsFromProfileXML(raw)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := plistPayloadIdentifiers(inner)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) == 0 {
		t.Fatal("fixture has no PayloadIdentifier values")
	}
	domain := ids[0]
	payloadXML, err := plistGetPayloadByDomain(inner, domain)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]interface{}
	if err := plist.NewDecoder(strings.NewReader(string(payloadXML))).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	payload["BuddyTestKey"] = true
	updatedInner, err := plistReplacePayloadByDomain(inner, domain, payload)
	if err != nil {
		t.Fatal(err)
	}
	out, err := replacePayloadsContent(raw, updatedInner)
	if err != nil {
		t.Fatal(err)
	}
	gotEnc, _ := extractPayloadsEncodedFromProfileXML(out)
	const needle = "search?q={searchTerms}"
	si, gi := strings.Index(srcEnc, needle), strings.Index(gotEnc, needle)
	if si < 0 || gi < 0 {
		t.Fatal("needle missing")
	}
	if srcEnc[si:si+60] != gotEnc[gi:gi+60] {
		t.Fatalf("replace payload changed amp encoding\nbefore: %q\nafter:  %q", srcEnc[si:si+60], gotEnc[gi:gi+60])
	}
}

func TestIsUUID(t *testing.T) {
	got := NewUUID()
	u, err := uuid.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	if u.Version() != 4 {
		t.Fatalf("uuid_new version = %s, want VERSION_4", u.Version())
	}
	if u.Variant() != uuid.RFC4122 {
		t.Fatalf("uuid_new variant = %s, want RFC4122", u.Variant())
	}
	if got != strings.ToLower(got) {
		t.Fatalf("uuid_new should be lowercase, got %q", got)
	}
	if !IsUUID(got) {
		t.Fatal("uuid_new form should parse")
	}
	if !IsUUID("330C2054-B496-494F-9EC1-846B04E789B6") {
		t.Fatal("uppercase hyphenated UUID")
	}
	if !IsUUID("00000000-0000-0000-0000-000000000000") {
		t.Fatal("nil UUID is still canonical text")
	}
	for _, s := range []string{
		"",
		"com.apple.ManagedClient.preferences",
		"330C2054B496494F9EC1846B04E789B6",
		"urn:uuid:330C2054-B496-494F-9EC1-846B04E789B6",
		"{330C2054-B496-494F-9EC1-846B04E789B6}",
		"not-a-uuid",
	} {
		if IsUUID(s) {
			t.Fatalf("IsUUID(%q) = true", s)
		}
	}
}
