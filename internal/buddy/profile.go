package buddy

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"howett.net/plist"
)

// ProfileRecord holds id and name for a configuration profile from the list endpoint.
type ProfileRecord struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// profileListXML parses the response from GET /JSSResource/osxconfigurationprofiles.
type profileListXML struct {
	XMLName  xml.Name `xml:"os_x_configuration_profiles"`
	Profiles []struct {
		ID      int    `xml:"id"`
		Name    string `xml:"name"`
		General *struct {
			Name string `xml:"name"`
		} `xml:"general"`
	} `xml:"os_x_configuration_profile"`
}

// ProfileFetchXML fetches the full profile XML and returns it as bytes (for script/component use).
// Empty Classic MCX domains may be hydrated from Jamf's ACS API when resolvable.
func ProfileFetchXML(credName string, profileID int) ([]byte, error) {
	return profileFetchRaw(credName, profileID)
}

// profileFetchRaw fetches the full profile XML and returns it as bytes.
// When Classic MCX preference settings are empty but ACS API has Forced.plist for the
// same domain, those settings are merged in (see hydrateProfileXMLFromACS).
func profileFetchRaw(credName string, profileID int) ([]byte, error) {
	body, err := profileFetchClassicUnhydrated(credName, profileID)
	if err != nil {
		return nil, err
	}
	return hydrateProfileXMLFromACS(credName, profileID, body, nil)
}

// profileFetchClassicUnhydrated is Classic XML only (no ACS Forced merge).
// Used when writing Classic payloads so a hydrated ACS view cannot overwrite '&' keys.
func profileFetchClassicUnhydrated(credName string, profileID int) ([]byte, error) {
	body, err := jamfGETWithAccept(credName, fmt.Sprintf("%s/%d", uriClassicOSXProfiles, profileID), "application/xml")
	if err != nil {
		return nil, fmt.Errorf("fetch profile %d: %w", profileID, err)
	}
	return body, nil
}

// ExtractPayloadsFromProfileXML finds the <payloads> element in profile XML, unescapes its content, and returns it.
// Returns an error if the payloads element is missing.
func ExtractPayloadsFromProfileXML(profileXML []byte) (string, error) {
	return extractPayloadsFromProfileXML(profileXML)
}

const uriClassicOSXProfiles = "/JSSResource/osxconfigurationprofiles/id"
const profilePayloadsOpen = "<payloads>"
const profilePayloadsClose = "</payloads>"
const errDecodePayloadsPlist = "decode payloads plist: %w"

func profilePayloadsContentBounds(s string) (contentStart, contentEnd int, err error) {
	openIdx := strings.Index(s, profilePayloadsOpen)
	closeIdx := strings.Index(s, profilePayloadsClose)
	if openIdx < 0 || closeIdx < 0 || closeIdx < openIdx {
		return 0, 0, fmt.Errorf("could not find <payloads> element in profile XML")
	}
	return openIdx + len(profilePayloadsOpen), closeIdx, nil
}

// extractPayloadsEncodedFromProfileXML returns the raw entity-encoded <payloads> inner XML (no unescape).
func extractPayloadsEncodedFromProfileXML(profileXML []byte) (string, error) {
	contentStart, contentEnd, err := profilePayloadsContentBounds(string(profileXML))
	if err != nil {
		return "", err
	}
	return string(profileXML)[contentStart:contentEnd], nil
}

// extractPayloadsFromProfileXML finds the <payloads> element in profile XML, unescapes its content, and returns it.
func extractPayloadsFromProfileXML(profileXML []byte) (string, error) {
	contentStart, contentEnd, err := profilePayloadsContentBounds(string(profileXML))
	if err != nil {
		return "", err
	}
	return XMLUnescape(string(profileXML)[contentStart:contentEnd]), nil
}

// PlistPayloadIdentifiers parses the payloads plist and returns PayloadIdentifier values from PayloadContent array.
// Returns nil slice if the plist does not have a PayloadContent array (e.g. MCX-style flat dict).
func PlistPayloadIdentifiers(payloadsRaw string) ([]string, error) {
	return plistPayloadIdentifiers(payloadsRaw)
}

func plistPayloadIdentifiers(payloadsRaw string) ([]string, error) {
	rows, err := plistPayloadContentRows(payloadsRaw)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, r := range rows {
		if r.Identifier != "" {
			ids = append(ids, r.Identifier)
		}
	}
	return ids, nil
}

// PlistGetPayloadByDomain returns the PayloadContent dict selected by identifier=
// (PayloadIdentifier, unique PayloadType, or unique PayloadUUID) as plist XML.
func PlistGetPayloadByDomain(payloadsRaw, domain string) ([]byte, error) {
	return plistGetPayloadByDomain(payloadsRaw, domain)
}

func plistGetPayloadByDomain(payloadsRaw, domain string) ([]byte, error) {
	_, arr, err := decodePayloadContent(payloadsRaw)
	if err != nil {
		return nil, err
	}
	idx, err := matchPayloadContentIndex(arr, domain)
	if err != nil {
		return nil, err
	}
	dict, ok := arr[idx].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("payload %q is not a dict", domain)
	}
	var buf bytes.Buffer
	if err := plist.NewEncoder(&buf).Encode(dict); err != nil {
		return nil, fmt.Errorf("encode payload: %w", err)
	}
	return buf.Bytes(), nil
}

// PlistReplacePayloadByDomain replaces the PayloadContent payload selected by identifier=.
// newPayload should be a decoded plist dict (map[string]interface{} or struct). Returns the updated plist as XML string.
// This is a full replace (advanced). Prefer PlistMergePayloadByDomain / PatchProfilePayload for partial updates.
func PlistReplacePayloadByDomain(payloadsRaw, domain string, newPayload interface{}) (string, error) {
	return plistReplacePayloadByDomain(payloadsRaw, domain, newPayload)
}

func plistReplacePayloadByDomain(payloadsRaw, domain string, newPayload interface{}) (string, error) {
	m, arr, err := decodePayloadContent(payloadsRaw)
	if err != nil {
		return "", err
	}
	newDict, err := coercePayloadDict(newPayload)
	if err != nil {
		return "", err
	}
	idx, err := matchPayloadContentIndex(arr, domain)
	if err != nil {
		return "", err
	}
	existing, _ := arr[idx].(map[string]interface{})
	canonical := payloadDictString(existing, "PayloadIdentifier")
	preserveCanonicalIdentifier(newDict, canonical, strings.TrimSpace(domain))
	arr[idx] = newDict
	m["PayloadContent"] = arr
	return encodePayloadsRoot(m)
}

// PlistMergePayloadByDomain merges newPayload keys onto the PayloadContent entry selected by identifier=
// (new keys win; omitted keys are preserved, including PayloadUUID). Does not rewrite PayloadIdentifier
// to a Type/UUID selector.
func PlistMergePayloadByDomain(payloadsRaw, domain string, newPayload interface{}) (string, error) {
	return plistMergePayloadByDomain(payloadsRaw, domain, newPayload)
}

func plistMergePayloadByDomain(payloadsRaw, domain string, newPayload interface{}) (string, error) {
	m, arr, err := decodePayloadContent(payloadsRaw)
	if err != nil {
		return "", err
	}
	newDict, err := coercePayloadDict(newPayload)
	if err != nil {
		return "", err
	}
	idx, err := matchPayloadContentIndex(arr, domain)
	if err != nil {
		return "", err
	}
	dict, ok := arr[idx].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("payload %q is not a dict", domain)
	}
	canonical := payloadDictString(dict, "PayloadIdentifier")
	merged := make(map[string]interface{}, len(dict)+len(newDict))
	for k, v := range dict {
		merged[k] = v
	}
	for k, v := range newDict {
		merged[k] = v
	}
	preserveCanonicalIdentifier(merged, canonical, strings.TrimSpace(domain))
	arr[idx] = merged
	m["PayloadContent"] = arr
	return encodePayloadsRoot(m)
}

// PlistSetPayloadContent sets the PayloadContent array on a payloads plist (or creates a new plist).
// payloadsRaw may be empty to start from scratch. content is a list of payload dicts
// (each typically including PayloadIdentifier).
func PlistSetPayloadContent(payloadsRaw string, content []interface{}) (string, error) {
	return plistSetPayloadContent(payloadsRaw, content)
}

func plistSetPayloadContent(payloadsRaw string, content []interface{}) (string, error) {
	var root map[string]interface{}
	if strings.TrimSpace(payloadsRaw) == "" {
		root = map[string]interface{}{}
	} else {
		var decoded interface{}
		if err := plist.NewDecoder(strings.NewReader(payloadsRaw)).Decode(&decoded); err != nil {
			return "", fmt.Errorf(errDecodePayloadsPlist, err)
		}
		m, ok := decoded.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("payloads root is not a dict")
		}
		root = m
	}
	normalized := make([]interface{}, 0, len(content))
	for i, item := range content {
		dict, err := coercePayloadDict(item)
		if err != nil {
			return "", fmt.Errorf("content[%d]: %w", i, err)
		}
		normalized = append(normalized, dict)
	}
	if normalized == nil {
		normalized = []interface{}{}
	}
	root["PayloadContent"] = normalized
	return encodePayloadsRoot(root)
}

// PlistAddPayload appends (or replaces) a payload dict selected by identifier=.
// If payloadsRaw is empty, creates a new payloads plist with a single PayloadContent entry.
func PlistAddPayload(payloadsRaw, domain string, newPayload interface{}) (string, error) {
	return plistAddPayload(payloadsRaw, domain, newPayload)
}

func plistAddPayload(payloadsRaw, domain string, newPayload interface{}) (string, error) {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return "", fmt.Errorf("identifier is required")
	}
	newDict, err := coercePayloadDict(newPayload)
	if err != nil {
		return "", err
	}

	if strings.TrimSpace(payloadsRaw) == "" {
		if payloadDictString(newDict, "PayloadIdentifier") == "" {
			newDict["PayloadIdentifier"] = domain
		} else if id := payloadDictString(newDict, "PayloadIdentifier"); id != domain {
			return "", fmt.Errorf("new payload PayloadIdentifier %q does not match identifier %q", id, domain)
		}
		return plistSetPayloadContent("", []interface{}{newDict})
	}

	m, arr, err := decodePayloadContentOrEmpty(payloadsRaw)
	if err != nil {
		return "", err
	}
	idx, matchErr := matchPayloadContentIndex(arr, domain)
	if matchErr == nil {
		existing, _ := arr[idx].(map[string]interface{})
		canonical := payloadDictString(existing, "PayloadIdentifier")
		preserveCanonicalIdentifier(newDict, canonical, domain)
		arr[idx] = newDict
	} else {
		if payloadDictString(newDict, "PayloadIdentifier") == "" {
			newDict["PayloadIdentifier"] = domain
		} else if id := payloadDictString(newDict, "PayloadIdentifier"); id != domain {
			return "", fmt.Errorf("new payload PayloadIdentifier %q does not match identifier %q", id, domain)
		}
		arr = append(arr, newDict)
	}
	m["PayloadContent"] = arr
	return encodePayloadsRoot(m)
}

// PlistRemovePayload removes the PayloadContent payload selected by identifier=.
func PlistRemovePayload(payloadsRaw, domain string) (string, error) {
	return plistRemovePayload(payloadsRaw, domain)
}

func plistRemovePayload(payloadsRaw, domain string) (string, error) {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return "", fmt.Errorf("identifier is required")
	}
	m, arr, err := decodePayloadContent(payloadsRaw)
	if err != nil {
		return "", err
	}
	idx, err := matchPayloadContentIndex(arr, domain)
	if err != nil {
		return "", err
	}
	out := make([]interface{}, 0, len(arr)-1)
	for i, item := range arr {
		if i == idx {
			continue
		}
		out = append(out, item)
	}
	m["PayloadContent"] = out
	return encodePayloadsRoot(m)
}

func decodePayloadContent(payloadsRaw string) (map[string]interface{}, []interface{}, error) {
	var root interface{}
	if err := plist.NewDecoder(strings.NewReader(payloadsRaw)).Decode(&root); err != nil {
		return nil, nil, fmt.Errorf(errDecodePayloadsPlist, err)
	}
	m, ok := root.(map[string]interface{})
	if !ok {
		return nil, nil, fmt.Errorf("payloads root is not a dict")
	}
	content, ok := m["PayloadContent"]
	if !ok {
		return nil, nil, fmt.Errorf("payloads plist has no PayloadContent array")
	}
	arr, ok := content.([]interface{})
	if !ok {
		return nil, nil, fmt.Errorf("PayloadContent is not an array")
	}
	return m, arr, nil
}

func decodePayloadContentOrEmpty(payloadsRaw string) (map[string]interface{}, []interface{}, error) {
	var root interface{}
	if err := plist.NewDecoder(strings.NewReader(payloadsRaw)).Decode(&root); err != nil {
		return nil, nil, fmt.Errorf(errDecodePayloadsPlist, err)
	}
	m, ok := root.(map[string]interface{})
	if !ok {
		return nil, nil, fmt.Errorf("payloads root is not a dict")
	}
	content, ok := m["PayloadContent"]
	if !ok {
		return m, []interface{}{}, nil
	}
	arr, ok := content.([]interface{})
	if !ok {
		return nil, nil, fmt.Errorf("PayloadContent is not an array")
	}
	return m, arr, nil
}

func coercePayloadDict(newPayload interface{}) (map[string]interface{}, error) {
	switch v := newPayload.(type) {
	case map[string]interface{}:
		return v, nil
	case nil:
		return nil, fmt.Errorf("payload is nil")
	default:
		var buf bytes.Buffer
		if err := plist.NewEncoder(&buf).Encode(newPayload); err != nil {
			return nil, fmt.Errorf("encode payload: %w", err)
		}
		var decoded interface{}
		if err := plist.NewDecoder(bytes.NewReader(buf.Bytes())).Decode(&decoded); err != nil {
			return nil, fmt.Errorf("decode payload: %w", err)
		}
		m, ok := decoded.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("payload is not a dict")
		}
		return m, nil
	}
}

func encodePayloadsRoot(root map[string]interface{}) (string, error) {
	var out bytes.Buffer
	if err := plist.NewEncoder(&out).Encode(root); err != nil {
		return "", fmt.Errorf("encode payloads plist: %w", err)
	}
	return out.String(), nil
}

// NormaliseDictXML accepts a bare <dict>…</dict> fragment or a full plist and returns the dict fragment.
func NormaliseDictXML(data []byte) (string, error) {
	return normaliseDictFile(data)
}

// PatchProfilePayload patches one payload domain inside a profile's <payloads> plist
// (PayloadContent by PayloadIdentifier with key merge, or MCX mcx_preference_settings fallback).
// newDictXML may be a bare <dict> fragment or a full plist.
// For PayloadContent, new keys merge onto the existing payload (PayloadUUID and other omitted keys are kept).
// For MCX, newDictXML is the settings dict under mcx_preference_settings only.
func PatchProfilePayload(profileXML []byte, domain, newDictXML string) ([]byte, error) {
	return patchProfilePayload(profileXML, domain, newDictXML)
}

// EncodeDictMapToXML encodes a JSON-like map as a bare plist <dict>…</dict> fragment
// suitable for PatchProfilePayload / MCX replace helpers.
func EncodeDictMapToXML(m map[string]any) (string, error) {
	if m == nil {
		return "", fmt.Errorf("dict map is nil")
	}
	var buf bytes.Buffer
	if err := plist.NewEncoder(&buf).Encode(m); err != nil {
		return "", fmt.Errorf("encode plist dict: %w", err)
	}
	return normaliseDictFile(buf.Bytes())
}

// ProfileSearch fetches all configuration profiles and returns records matching the optional name filter.
// If nameFilter is empty, returns all profiles. Otherwise filters by name (case-insensitive contains).
func ProfileSearch(credName string, nameFilter string, logger *slog.Logger) ([]ProfileRecord, error) {
	body, err := jamfGETWithAccept(credName, "/JSSResource/osxconfigurationprofiles", "application/xml")
	if err != nil {
		return nil, fmt.Errorf("list profiles: %w", err)
	}
	var list profileListXML
	if err := xml.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("parse profile list: %w", err)
	}
	var out []ProfileRecord
	filterLower := strings.ToLower(strings.TrimSpace(nameFilter))
	for _, p := range list.Profiles {
		name := p.Name
		if name == "" && p.General != nil {
			name = p.General.Name
		}
		if filterLower != "" && !strings.Contains(strings.ToLower(name), filterLower) {
			continue
		}
		out = append(out, ProfileRecord{ID: p.ID, Name: name})
	}
	return out, nil
}

// ProfileList fetches and prints configuration profiles, with an optional name filter.
func ProfileList(credName string, nameFilter string, asJSON bool, logger *slog.Logger) error {
	records, err := ProfileSearch(credName, nameFilter, logger)
	if err != nil {
		return err
	}
	if asJSON {
		return printJSON(records)
	}
	fmt.Printf("%-8s  %s\n", "ID", "Name")
	fmt.Printf("%-8s  %s\n", "--------", "----------------------------------------")
	for _, r := range records {
		fmt.Printf("%-8d  %s\n", r.ID, truncate(r.Name, 60))
	}
	fmt.Printf("\n%d profile(s)\n", len(records))
	return nil
}

// ProfileFindID finds a profile by name (case-insensitive contains match).
// Returns the first matching profile's ID and name, or 0 and empty string if not found.
func ProfileFindID(credName string, name string, logger *slog.Logger) (id int, profileName string, err error) {
	records, err := ProfileSearch(credName, name, logger)
	if err != nil {
		return 0, "", err
	}
	if len(records) == 0 {
		return 0, "", nil
	}
	// Prefer exact name match.
	nameLower := strings.ToLower(strings.TrimSpace(name))
	for _, r := range records {
		if strings.EqualFold(strings.TrimSpace(r.Name), nameLower) {
			return r.ID, r.Name, nil
		}
	}
	return records[0].ID, records[0].Name, nil
}

// ProfileRedeploy triggers redeployment of a configuration profile to all currently-assigned devices
// by setting redeploy_on_update to "All" and PUTting the profile back. No content is changed.
func ProfileRedeploy(credName string, opts Opts, profileID int, logger *slog.Logger) error {
	profileXML, err := profileFetchRaw(credName, profileID)
	if err != nil {
		return err
	}
	updatedXML := setRedeployAll(profileXML)
	if !opts.SkipConfirm {
		msg := fmt.Sprintf("\nRedeploy configuration profile (ID %d) to all assigned devices", profileID)
		if !ConfirmPrompt(msg, logger) {
			return nil
		}
	}
	if _, err := jamfPUT(credName, fmt.Sprintf("%s/%d", uriClassicOSXProfiles, profileID), updatedXML, "application/xml"); err != nil {
		return fmt.Errorf("redeploy profile: %w", err)
	}
	fmt.Printf("Redeployed configuration profile (ID %d)\n", profileID)
	return nil
}

// ProfileGetXML fetches a macOS configuration profile and writes its XML to w (typically os.Stdout).
// Empty Classic MCX domains may be hydrated from ACS ACS (same as ProfileFetchXML).
func ProfileGetXML(credName string, profileID int, w io.Writer, logger *slog.Logger) error {
	profileXML, err := profileFetchRaw(credName, profileID)
	if err != nil {
		return err
	}
	_, err = w.Write(profileXML)
	return err
}

// ProfileGetPayloads fetches the profile and writes only the payloads section (decoded plist XML) to w.
// The payloads are pretty-printed as plist XML. If the profile has no payloads or parsing fails, returns an error.
func ProfileGetPayloads(credName string, profileID int, w io.Writer, logger *slog.Logger) error {
	profileXML, err := profileFetchRaw(credName, profileID)
	if err != nil {
		return err
	}
	payloadsRaw, err := extractPayloadsFromProfileXML(profileXML)
	if err != nil {
		return err
	}
	if payloadsRaw == "" {
		return fmt.Errorf("profile %d has empty payloads", profileID)
	}
	// Decode and re-encode to get consistent formatting (pretty plist XML)
	var root interface{}
	dec := plist.NewDecoder(strings.NewReader(payloadsRaw))
	if err := dec.Decode(&root); err != nil {
		// If plist decode fails, write raw so user still gets content
		_, _ = w.Write([]byte(payloadsRaw))
		return nil
	}
	enc := plist.NewEncoder(w)
	return enc.Encode(root)
}

// ProfileGetPayloadByDomain fetches the profile and writes the PayloadContent payload selected by identifier= to w.
func ProfileGetPayloadByDomain(credName string, profileID int, domain string, w io.Writer, logger *slog.Logger) error {
	profileXML, err := profileFetchRaw(credName, profileID)
	if err != nil {
		return err
	}
	payloadsRaw, err := extractPayloadsFromProfileXML(profileXML)
	if err != nil {
		return err
	}
	out, err := plistGetPayloadByDomain(payloadsRaw, domain)
	if err != nil {
		return err
	}
	_, err = w.Write(out)
	return err
}

// ProfileListPayloads fetches the profile and returns PayloadContent rows {identifier, type, uuid}.
// Returns a nil slice if the payloads plist does not use a PayloadContent array (e.g. MCX-style).
func ProfileListPayloads(credName string, profileID int, logger *slog.Logger) ([]PayloadContentRow, error) {
	profileXML, err := profileFetchRaw(credName, profileID)
	if err != nil {
		return nil, err
	}
	payloadsRaw, err := extractPayloadsFromProfileXML(profileXML)
	if err != nil {
		return nil, err
	}
	return plistPayloadContentRows(payloadsRaw)
}

// ProfileSetXML reads XML from r (typically os.Stdin) and PUTs it to update the configuration profile.
// Sends raw XML bytes directly to avoid struct round-trip that can corrupt mobileconfig payload data.
// If redeploy is true, <redeploy_on_update> is set to "All" before uploading.
func ProfileSetXML(credName string, opts Opts, profileID int, r io.Reader, redeploy bool, logger *slog.Logger) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read XML: %w", err)
	}
	if !bytes.Contains(data, []byte("<os_x_configuration_profile")) {
		return fmt.Errorf("input does not appear to be macOS configuration profile XML (missing <os_x_configuration_profile> root)")
	}
	if redeploy {
		data = setRedeployAll(data)
	}
	if !opts.SkipConfirm {
		msg := fmt.Sprintf("\nPUT configuration profile (ID %d) with XML from stdin", profileID)
		if redeploy {
			msg += " [redeploy_on_update=All]"
		}
		if !ConfirmPrompt(msg, logger) {
			return nil
		}
	}
	if _, err := jamfPUT(credName, fmt.Sprintf("%s/%d", uriClassicOSXProfiles, profileID), data, "application/xml"); err != nil {
		return fmt.Errorf("update profile: %w", err)
	}
	fmt.Printf("Updated configuration profile (ID %d)\n", profileID)
	return nil
}

// ProfilePutXML writes the given profile XML to Jamf for the given profile ID (for script/component use).
// If redeploy is true, sets redeploy_on_update to All. Uses opts.SkipConfirm to skip confirmation when called from scripts.
func ProfilePutXML(credName string, opts Opts, profileID int, xml []byte, redeploy bool, logger *slog.Logger) error {
	if !bytes.Contains(xml, []byte("<os_x_configuration_profile")) {
		return fmt.Errorf("input does not appear to be macOS configuration profile XML (missing <os_x_configuration_profile> root)")
	}
	if !opts.SkipConfirm {
		msg := fmt.Sprintf("\nPUT configuration profile (ID %d) with XML", profileID)
		if redeploy {
			msg += " [redeploy_on_update=All]"
		}
		if !ConfirmPrompt(msg, logger) {
			return nil
		}
	}
	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "\n--- PUT payload (JSSResource/osxconfigurationprofiles/id/%d) ---\n%s\n--- end payload ---\n\n", profileID, xml)
	}
	return profileClassicPut(credName, opts, profileID, xml, redeploy, logger)
}

// ProfileSetPayload patches a payload domain inside a profile (PayloadContent merge by
// PayloadIdentifier, or MCX mcx_preference_settings settings dict). It fetches the current
// profile from Jamf, patches in place, and PUTs the result back.
//
// Usage: buddy profile set payload <profile-id> <identifier> <dict-file.xml>
//
// For PayloadContent, the dict file may be a partial payload (keys merge; PayloadUUID kept).
// For MCX, the dict file is the mcx_preference_settings content only.
// If redeploy is true, <redeploy_on_update> is set to "All" before uploading.
func ProfileSetPayload(credName string, opts Opts, profileID int, domain string, dictFile string, redeploy bool, logger *slog.Logger) error {
	dictData, err := os.ReadFile(dictFile)
	if err != nil {
		return fmt.Errorf("read dict file %q: %w", dictFile, err)
	}
	newDict, err := normaliseDictFile(dictData)
	if err != nil {
		return fmt.Errorf("dict file %q: %w", dictFile, err)
	}

	profileXML, err := jamfGETWithAccept(credName, fmt.Sprintf("%s/%d", uriClassicOSXProfiles, profileID), "application/xml")
	if err != nil {
		return fmt.Errorf("fetch profile %d: %w", profileID, err)
	}

	updatedXML, err := patchProfilePayload(profileXML, domain, newDict)
	if err != nil {
		return err
	}

	if !opts.SkipConfirm {
		msg := fmt.Sprintf("\nUpdate %q payload in configuration profile (ID %d)", domain, profileID)
		if redeploy {
			msg += " [redeploy_on_update=All]"
		}
		if !ConfirmPrompt(msg, logger) {
			return nil
		}
	}

	if err := profileClassicPut(credName, opts, profileID, updatedXML, redeploy, logger); err != nil {
		return err
	}
	fmt.Printf("Updated %q payload in configuration profile (ID %d)\n", domain, profileID)
	return nil
}

// emptyScopeXML is the scope block for an unassigned profile (no targets).
// Jamf quirk: omitting <scope> or omitting <all_computers> on Classic POST/PUT can
// default to all computers. Always send all_computers=false (and empty target lists)
// when the intent is unassigned — never a bare/missing scope.
const emptyScopeXML = `<scope>
  <all_computers>false</all_computers>
  <all_jss_users>false</all_jss_users>
  <computers/>
  <buildings/>
  <departments/>
  <computer_groups/>
  <jss_users/>
  <jss_user_groups/>
  <limitations>
    <users/><user_groups/><network_segments/><ibeacons/>
  </limitations>
  <exclusions>
    <computers/><buildings/><departments/><computer_groups/>
    <users/><user_groups/><network_segments/><ibeacons/>
    <jss_users/><jss_user_groups/>
  </exclusions>
</scope>`

// ProfileCreateSpec describes a greenfield macOS configuration profile POST.
type ProfileCreateSpec struct {
	Name             string
	PayloadsPlist    string // unescaped payloads plist XML
	Category         string
	ScopeAll         bool
	ScopeGroupIDs    []int
	RedeployOnUpdate string // "Newly Assigned" (default) or "All"
}

// ProfileCreateOutcome is returned after a successful profile create.
type ProfileCreateOutcome struct {
	ProfileID int
	Name      string
}

// BuildProfileCreateXML assembles Classic os_x_configuration_profile XML for POST (no API call).
func BuildProfileCreateXML(spec ProfileCreateSpec) ([]byte, error) {
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		return nil, fmt.Errorf("profile name is required")
	}
	payloads := strings.TrimSpace(spec.PayloadsPlist)
	if payloads == "" {
		return nil, fmt.Errorf("payloads plist is required")
	}
	redeploy := strings.TrimSpace(spec.RedeployOnUpdate)
	if redeploy == "" {
		redeploy = "Newly Assigned"
	}
	scopeAll := spec.ScopeAll
	if !scopeAll && len(spec.ScopeGroupIDs) == 0 {
		scopeAll = true
	}
	puuid := uuid.New().String()
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	b.WriteString(`<os_x_configuration_profile><general>`)
	b.WriteString("<name>" + xmlEncodePayload(name) + "</name>")
	b.WriteString("<uuid>" + puuid + "</uuid>")
	b.WriteString(`<site><id>-1</id><name>None</name></site>`)
	if strings.TrimSpace(spec.Category) != "" {
		b.WriteString("<category><name>" + xmlEncodePayload(spec.Category) + "</name></category>")
	} else {
		b.WriteString(`<category><id>-1</id><name>No category assigned</name></category>`)
	}
	b.WriteString(`<distribution_method>Install Automatically</distribution_method>`)
	b.WriteString(`<user_removable>false</user_removable>`)
	b.WriteString(`<level>computer</level>`)
	b.WriteString("<redeploy_on_update>" + xmlEncodePayload(redeploy) + "</redeploy_on_update>")
	b.WriteString("<payloads>" + xmlEncodePayload(payloads) + "</payloads>")
	b.WriteString(`</general>`)
	b.WriteString(buildProfileScopeXML(scopeAll, spec.ScopeGroupIDs))
	b.WriteString(`</os_x_configuration_profile>`)
	return []byte(b.String()), nil
}

func buildProfileScopeXML(scopeAll bool, groupIDs []int) string {
	if scopeAll || len(groupIDs) == 0 {
		return `<scope>
  <all_computers>true</all_computers>
  <all_jss_users>false</all_jss_users>
  <computers/>
  <buildings/>
  <departments/>
  <computer_groups/>
  <jss_users/>
  <jss_user_groups/>
  <limitations>
    <users/><user_groups/><network_segments/><ibeacons/>
  </limitations>
  <exclusions>
    <computers/><buildings/><departments/><computer_groups/>
    <users/><user_groups/><network_segments/><ibeacons/>
    <jss_users/><jss_user_groups/>
  </exclusions>
</scope>`
	}
	var groups strings.Builder
	groups.WriteString("<computer_groups>")
	for _, id := range groupIDs {
		groups.WriteString(fmt.Sprintf("<computer_group><id>%d</id></computer_group>", id))
	}
	groups.WriteString("</computer_groups>")
	return `<scope>
  <all_computers>false</all_computers>
  <all_jss_users>false</all_jss_users>
  <computers/>
  <buildings/>
  <departments/>
  ` + groups.String() + `
  <jss_users/>
  <jss_user_groups/>
  <limitations>
    <users/><user_groups/><network_segments/><ibeacons/>
  </limitations>
  <exclusions>
    <computers/><buildings/><departments/><computer_groups/>
    <users/><user_groups/><network_segments/><ibeacons/>
    <jss_users/><jss_user_groups/>
  </exclusions>
</scope>`
}

func parseProfileIDFromPostBody(body []byte) int {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return 0
	}
	var wrap struct {
		ID int `xml:"id"`
	}
	if err := xml.Unmarshal(body, &wrap); err == nil && wrap.ID > 0 {
		return wrap.ID
	}
	re := regexp.MustCompile(`(?i)<id>\s*(\d+)\s*</id>`)
	if m := re.FindSubmatch(body); len(m) == 2 {
		id, _ := strconv.Atoi(string(m[1]))
		return id
	}
	var j map[string]interface{}
	if err := json.Unmarshal(body, &j); err == nil {
		if id := jsonIntID(j["id"]); id > 0 {
			return id
		}
		if p, ok := j["os_x_configuration_profile"].(map[string]interface{}); ok {
			if id := jsonIntID(p["id"]); id > 0 {
				return id
			}
			if g, ok := p["general"].(map[string]interface{}); ok {
				if id := jsonIntID(g["id"]); id > 0 {
					return id
				}
			}
		}
	}
	return 0
}

func resolveCreatedProfileID(credName string, opts Opts, postBody []byte, profileName string, logger *slog.Logger) (int, error) {
	_ = opts
	if id := parseProfileIDFromPostBody(postBody); id > 0 {
		return id, nil
	}
	id, foundName, err := ProfileFindID(credName, profileName, logger)
	if err != nil {
		return 0, fmt.Errorf("created profile %q but could not resolve ID: %w", profileName, err)
	}
	if id > 0 && (foundName == "" || strings.EqualFold(foundName, profileName)) {
		return id, nil
	}
	if id > 0 {
		return id, nil
	}
	return 0, fmt.Errorf("created profile %q but could not resolve its ID", profileName)
}

// ProfileCreate POSTs a new macOS configuration profile and returns its ID.
func ProfileCreate(credName string, opts Opts, spec ProfileCreateSpec, logger *slog.Logger) (*ProfileCreateOutcome, error) {
	xmlBody, err := BuildProfileCreateXML(spec)
	if err != nil {
		return nil, err
	}
	if !opts.SkipConfirm {
		scopeMsg := "all computers"
		if !spec.ScopeAll && len(spec.ScopeGroupIDs) > 0 {
			scopeMsg = fmt.Sprintf("groups %v", spec.ScopeGroupIDs)
		}
		msg := fmt.Sprintf("\nCreate configuration profile %q (scope: %s)?", spec.Name, scopeMsg)
		if !ConfirmPrompt(msg, logger) {
			return nil, nil
		}
	}
	optsPost := opts
	optsPost.SkipConfirm = true
	return ProfilePostXML(credName, optsPost, xmlBody, spec.Name, logger)
}

// ProfileClone creates a copy of a configuration profile with the given name and
// unassigned scope (all_computers=false, empty targets — not "all computers").
func ProfileClone(credName string, opts Opts, sourceProfileID int, newName string, logger *slog.Logger) (*ProfileCreateOutcome, error) {
	return ProfileCloneWithCategory(credName, opts, sourceProfileID, newName, "", logger)
}

// ProfileCloneWithCategory is ProfileClone with an optional destination category_id override.
func ProfileCloneWithCategory(credName string, opts Opts, sourceProfileID int, newName, categoryID string, logger *slog.Logger) (*ProfileCreateOutcome, error) {
	profileXML, err := profileFetchRaw(credName, sourceProfileID)
	if err != nil {
		return nil, err
	}
	prepared, err := PrepareProfileXMLForCreate(profileXML, ProfilePrepareSpec{Name: newName})
	if err != nil {
		return nil, err
	}
	prepared, err = RemapPolicyCategoryOnDest(credName, prepared, categoryID, categoryID == "")
	if err != nil {
		return nil, err
	}
	if !opts.SkipConfirm {
		msg := fmt.Sprintf("\nClone configuration profile (ID %d) as %q with unassigned scope (not all computers)?", sourceProfileID, newName)
		if !ConfirmPrompt(msg, logger) {
			return nil, nil
		}
	}
	optsCopy := opts
	optsCopy.SkipConfirm = true
	out, err := ProfilePostXML(credName, optsCopy, prepared, newName, logger)
	if err != nil {
		return nil, err
	}
	if out != nil {
		fmt.Printf("Cloned profile (ID %d) as %q with unassigned scope\n", sourceProfileID, newName)
	}
	return out, nil
}

// SetRedeployAll replaces the <redeploy_on_update> value with "All" in a profile XML,
// causing Jamf to redeploy the profile to all currently-assigned devices (not just new ones).
func SetRedeployAll(profileXML []byte) []byte {
	return setRedeployAll(profileXML)
}

func setRedeployAll(profileXML []byte) []byte {
	s := string(profileXML)
	const open = "<redeploy_on_update>"
	const close = "</redeploy_on_update>"
	openIdx := strings.Index(s, open)
	closeIdx := strings.Index(s, close)
	if openIdx < 0 || closeIdx < 0 || closeIdx < openIdx {
		return profileXML
	}
	return []byte(s[:openIdx+len(open)] + "All" + s[closeIdx:])
}

// normaliseDictFile accepts either a bare <dict>…</dict> fragment or a full plist
// file (with optional XML declaration, DOCTYPE, and <plist> wrapper) and returns
// just the <dict>…</dict> content ready for embedding in the MCX payload.
func normaliseDictFile(data []byte) (string, error) {
	s := strings.TrimSpace(string(data))

	// Strip XML declaration: <?xml ... ?>
	if strings.HasPrefix(s, "<?xml") {
		end := strings.Index(s, "?>")
		if end < 0 {
			return "", fmt.Errorf("malformed XML declaration")
		}
		s = strings.TrimSpace(s[end+2:])
	}

	// Strip DOCTYPE declaration: <!DOCTYPE ... >
	// The closing > may be preceded by a quoted system identifier, so scan carefully.
	if strings.HasPrefix(s, "<!DOCTYPE") {
		end := strings.Index(s, ">")
		if end < 0 {
			return "", fmt.Errorf("malformed DOCTYPE declaration")
		}
		s = strings.TrimSpace(s[end+1:])
	}

	// Strip <plist> wrapper, leaving just the inner <dict>.
	if strings.HasPrefix(s, "<plist") {
		dictStart := strings.Index(s, "<dict>")
		if dictStart < 0 {
			return "", fmt.Errorf("plist file contains no <dict> element")
		}
		plistClose := strings.LastIndex(s, "</plist>")
		if plistClose < 0 {
			return "", fmt.Errorf("plist file has no </plist> closing tag")
		}
		s = strings.TrimSpace(s[dictStart:plistClose])
	}

	if !strings.HasPrefix(s, "<dict>") {
		return "", fmt.Errorf("must contain a <dict> element at the root (got %q...)", truncate(s, 40))
	}
	return s, nil
}

// ReplacePayloadsContent replaces the entire content of <payloads> with newPayloadsRaw (unescaped plist XML),
// entity-encodes it, and returns the updated profile XML. Returns an error if <payloads> is missing.
func ReplacePayloadsContent(profileXML []byte, newPayloadsRaw string) ([]byte, error) {
	return replacePayloadsContent(profileXML, newPayloadsRaw)
}

func replacePayloadsContent(profileXML []byte, newPayloadsRaw string) ([]byte, error) {
	return replacePayloadsInner(profileXML, xmlEmbedPayloadsPlist(newPayloadsRaw))
}

// replacePayloadsEncodedContent swaps <payloads> inner text without re-entity-encoding (clone/copy preserve Jamf bytes).
func replacePayloadsEncodedContent(profileXML []byte, encodedInner string) ([]byte, error) {
	return replacePayloadsInner(profileXML, encodedInner)
}

func replacePayloadsInner(profileXML []byte, inner string) ([]byte, error) {
	s := string(profileXML)
	contentStart, contentEnd, err := profilePayloadsContentBounds(s)
	if err != nil {
		return nil, fmt.Errorf("profile XML has no <payloads> element")
	}
	var buf strings.Builder
	buf.WriteString(s[:contentStart])
	buf.WriteString(inner)
	buf.WriteString(s[contentEnd:])
	return []byte(buf.String()), nil
}

// reembedPayloadsForJamfAPI decodes the entity-encoded <payloads> blob once,
// validates/re-serializes the inner plist, and re-embeds with Jamf double-encoding.
// Use before Classic POST/PUT so nested &amp; in plist strings become &amp;amp; in
// the outer profile XML (see xmlEmbedPayloadsPlist).
func reembedPayloadsForJamfAPI(profileXML []byte) ([]byte, error) {
	encoded, err := extractPayloadsEncodedFromProfileXML(profileXML)
	if err != nil {
		return nil, err
	}
	if err := validateJamfEncodedPayloads(encoded); err == nil {
		return profileXML, nil
	}
	inner, err := extractPayloadsFromProfileXML(profileXML)
	if err != nil {
		return nil, err
	}
	var root interface{}
	if err := plist.NewDecoder(strings.NewReader(inner)).Decode(&root); err != nil {
		repaired := repairJamfEncodedPayloadsAmps(encoded)
		if repaired != encoded {
			xml2, err2 := replacePayloadsEncodedContent(profileXML, repaired)
			if err2 != nil {
				return nil, err2
			}
			return reembedPayloadsForJamfAPI(xml2)
		}
		return nil, fmt.Errorf(errDecodePayloadsPlist, err)
	}
	rootMap, ok := root.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("payloads root is not a dict")
	}
	reencoded, err := encodePayloadsRoot(rootMap)
	if err != nil {
		return nil, err
	}
	out, err := replacePayloadsContent(profileXML, reencoded)
	if err != nil {
		return nil, err
	}
	gotEnc, err := extractPayloadsEncodedFromProfileXML(out)
	if err != nil {
		return nil, err
	}
	if err := validateJamfEncodedPayloads(gotEnc); err != nil {
		return nil, err
	}
	return out, nil
}

func validateJamfEncodedPayloads(encoded string) error {
	if strings.Contains(encoded, "<plist") || strings.Contains(encoded, "<string>") {
		return fmt.Errorf("payloads content is not entity-encoded (raw plist tags in outer XML)")
	}
	if hasBareAmpersandInXML(encoded) {
		return fmt.Errorf("payloads contain bare '&' (use &amp; / &lt; / &gt; entities; buddy will not auto-convert)")
	}
	if hasUnderEncodedAmpersandsInEncodedPayloads(encoded) {
		return fmt.Errorf("payloads under-encode '&' in nested plist strings (need &amp;amp; in outer XML, not &amp; alone; coder should supply &amp; — buddy will not auto-convert)")
	}
	return nil
}

// errBareOrUnderEncodedAmp is returned when Classic write payloads have bare or under-encoded '&'.
var errBareOrUnderEncodedAmp = fmt.Errorf("bare or under-encoded '&' in profile <payloads>")

// errBareMCXAmpersandSetting is returned when decoded MCX preference strings contain a bare '&'.
// Jamf ACS Apply stores entity text (&amp;) in the string value; a raw '&' character 409s Classic.
var errBareMCXAmpersandSetting = fmt.Errorf("MCX preference strings contain bare '&' (use &amp; like Jamf ACS Apply; buddy will not auto-convert)")

// rejectBareMCXAmpersandSettings fails fast when MCX settings have a bare '&' in preference strings
// (not part of &amp;/&lt;/…). Encoding-only checks can miss this after plist escape + Classic embed.
// Jamf ACS Apply form (&amp; in the string value) is allowed and may use the ACS write path.
func rejectBareMCXAmpersandSettings(profileXML []byte) error {
	entries, err := listMCXPayloadEntries(profileXML)
	if err != nil || len(entries) == 0 {
		return nil
	}
	var domains []string
	for _, e := range entries {
		if plistMapHasBareAmpersand(e.SettingsDict) {
			domains = append(domains, e.Domain)
		}
	}
	if len(domains) == 0 {
		return nil
	}
	return fmt.Errorf("%w in domains %v", errBareMCXAmpersandSetting, domains)
}

// prepareProfileXMLForClassicWrite validates entity-encoded <payloads> for Classic POST/PUT.
// Raw (unescaped) payloads are re-embedded once. Bare or under-encoded '&' is refused — no silent conversion.
func prepareProfileXMLForClassicWrite(profileXML []byte) ([]byte, error) {
	return prepareProfileXMLForClassicWriteOpts(profileXML, true)
}

// prepareProfileXMLForClassicSync validates outer <payloads> encoding after an ACS write.
// Decoded MCX URL '&' is allowed (ACS already stored it); under-encoded outer XML is still rejected.
func prepareProfileXMLForClassicSync(profileXML []byte) ([]byte, error) {
	return prepareProfileXMLForClassicWriteOpts(profileXML, false)
}

func prepareProfileXMLForClassicWriteOpts(profileXML []byte, rejectBareMCX bool) ([]byte, error) {
	if rejectBareMCX {
		if err := rejectBareMCXAmpersandSettings(profileXML); err != nil {
			return nil, err
		}
	}
	enc, err := extractPayloadsEncodedFromProfileXML(profileXML)
	if err != nil {
		return nil, err
	}
	if err := validateJamfEncodedPayloads(enc); err == nil {
		return profileXML, nil
	} else if hasBareAmpersandInXML(enc) || hasUnderEncodedAmpersandsInEncodedPayloads(enc) {
		return nil, fmt.Errorf("%w: %v", errBareOrUnderEncodedAmp, err)
	}
	// Structural: payloads not entity-encoded yet — encode once (does not invent &amp; for bare URL amps in decoded strings beyond normal XML escaping).
	return reembedPayloadsForJamfAPI(profileXML)
}

// hasBareAmpersandInXML reports a '&' that is not the start of a known XML entity
// (&amp; &lt; &gt; &quot; &#… / &#x…).
func applyFormEncodeBareAmpInString(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 8)
	for i := 0; i < len(s); i++ {
		if s[i] != '&' {
			b.WriteByte(s[i])
			continue
		}
		rest := s[i+1:]
		switch {
		case strings.HasPrefix(rest, "amp;"),
			strings.HasPrefix(rest, "lt;"),
			strings.HasPrefix(rest, "gt;"),
			strings.HasPrefix(rest, "quot;"),
			strings.HasPrefix(rest, "#"):
			b.WriteByte('&')
		default:
			b.WriteString("&amp;")
		}
	}
	return b.String()
}

func applyFormEncodeBareAmpInAny(v any) any {
	switch t := v.(type) {
	case string:
		return applyFormEncodeBareAmpInString(t)
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = applyFormEncodeBareAmpInAny(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = applyFormEncodeBareAmpInAny(val)
		}
		return out
	default:
		return v
	}
}

// profileXMLBareAmpToApplyForm copies Jamf ACS Apply form (&amp; in preference strings)
// so Classic POST/PUT can retain MCX keys that 409 with a literal '&'.
func profileXMLBareAmpToApplyForm(profileXML []byte) ([]byte, error) {
	entries, err := listMCXPayloadEntries(profileXML)
	if err != nil {
		return nil, err
	}
	payloads, err := extractPayloadsFromProfileXML(profileXML)
	if err != nil {
		return nil, err
	}
	changed := false
	for _, e := range entries {
		if !plistMapHasBareAmpersand(e.SettingsDict) {
			continue
		}
		encoded, _ := applyFormEncodeBareAmpInAny(e.SettingsDict).(map[string]any)
		updated, ok, err := replaceMCXSettingsInPayloadsPlist(payloads, e.Domain, encoded)
		if err != nil {
			return nil, err
		}
		if ok {
			payloads = updated
			changed = true
		}
	}
	if !changed {
		return profileXML, nil
	}
	return replacePayloadsContent(profileXML, payloads)
}

func hasBareAmpersandInXML(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != '&' {
			continue
		}
		rest := s[i+1:]
		switch {
		case strings.HasPrefix(rest, "amp;"),
			strings.HasPrefix(rest, "lt;"),
			strings.HasPrefix(rest, "gt;"),
			strings.HasPrefix(rest, "quot;"),
			strings.HasPrefix(rest, "#"):
			continue
		default:
			return true
		}
	}
	return false
}

// hasUnderEncodedAmpersandsInEncodedPayloads reports &amp; in the outer <payloads> blob that is
// not part of a valid XML entity (&amp;amp; &amp;lt; &amp;gt; &amp;quot; &amp;#…). Jamf nests
// entity-encoded plist XML; inner plist &amp; must appear as &amp;amp; at this layer.
func hasUnderEncodedAmpersandsInEncodedPayloads(encoded string) bool {
	for i := 0; i < len(encoded); {
		j := strings.Index(encoded[i:], "&amp;")
		if j < 0 {
			return false
		}
		pos := i + j
		rest := encoded[pos+len("&amp;"):]
		switch {
		case strings.HasPrefix(rest, "amp;"),
			strings.HasPrefix(rest, "lt;"),
			strings.HasPrefix(rest, "gt;"),
			strings.HasPrefix(rest, "quot;"),
			strings.HasPrefix(rest, "#"):
			i = pos + len("&amp;")
			continue
		default:
			return true
		}
	}
	return false
}

// repairJamfEncodedPayloadsAmps is an offline migrate helper for known under-encoded patterns.
// Live Classic write paths must not call this — they refuse under-encoded payloads instead.
func repairJamfEncodedPayloadsAmps(encoded string) string {
	out := encoded
	for _, pair := range [][2]string{
		{"searchTerms}&amp;{google", "searchTerms}&amp;amp;{google"},
		{"upload&amp;FORM", "upload&amp;amp;FORM"},
	} {
		if strings.Contains(out, pair[1]) {
			continue
		}
		out = strings.ReplaceAll(out, pair[0], pair[1])
	}
	return out
}

// profileXMLRequiresAmpersandClassicRetention reports whether desired profile XML has
// Jamf ACS Apply-form MCX (&amp; in the preference string) that must remain visible
// in Classic after ACS attach. Literal URL '&' is stored on ACS and is not required
// in the Classic stub.
// profileXMLDropsBareAmpKeys reports that desired MCX would lose a current
// Classic value that contains a literal '&' — either by omitting the key or
// by replacing that value with a non-'&' string. Classic PUT of a no-'&'
// dict 200s and hides AmpersandString from the next process (ACS GET has no
// Forced; hydrate then shows Classic only).
func profileXMLDropsBareAmpKeys(current, desired []byte) bool {
	curEntries, err := listMCXPayloadEntries(current)
	if err != nil {
		return false
	}
	desEntries, err := listMCXPayloadEntries(desired)
	if err != nil {
		return false
	}
	desByDomain := make(map[string]MCXPayloadEntry, len(desEntries))
	for _, e := range desEntries {
		desByDomain[e.Domain] = e
	}
	for _, e := range curEntries {
		if !plistMapHasBareAmpersand(e.SettingsDict) {
			continue
		}
		d, ok := desByDomain[e.Domain]
		if !ok {
			return true
		}
		for k, v := range e.SettingsDict {
			if !plistMapHasBareAmpersand(v) {
				continue
			}
			dv, exists := d.SettingsDict[k]
			if !exists {
				return true
			}
			// Same key, but the '&' is gone (e.g. "Here &" → marker). Apply-form
			// (&amp; in the string) still counts as retaining the ampersand.
			if !plistMapHasBareAmpersand(dv) && !plistMapHasMCXAmpersandIssue(dv) {
				return true
			}
		}
	}
	return false
}

func profileXMLRequiresAmpersandClassicRetention(profileXML []byte) bool {
	entries, err := listMCXPayloadEntries(profileXML)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.HasAmpersand && !plistMapHasBareAmpersand(e.SettingsDict) {
			return true
		}
	}
	return false
}

// dictFileToPlistMap parses the dict file (full plist or bare <dict> fragment) and returns the root as a map.
// Returns nil, nil if the file is not a plist dict (e.g. MCX fragment only).
func dictFileToPlistMap(data []byte) (map[string]interface{}, error) {
	s := strings.TrimSpace(string(data))
	// If it looks like a bare dict, wrap in plist for decoding
	if strings.HasPrefix(s, "<dict>") || strings.HasPrefix(s, "<dict ") {
		s = "<?xml version=\"1.0\" encoding=\"UTF-8\"?><!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\"><plist version=\"1.0\">" + s + "</plist>"
	}
	var root interface{}
	if err := plist.NewDecoder(strings.NewReader(s)).Decode(&root); err != nil {
		return nil, err
	}
	m, ok := root.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("dict file root is not a dict")
	}
	return m, nil
}

// patchProfilePayload locates the entity-encoded plist inside <payloads>,
// merges the payload selected by identifier= (PayloadIdentifier, unique Type, unique UUID)
// or falls back to MCX mcx_preference_settings for the same string as a preference domain.
func patchProfilePayload(profileXML []byte, domain, newDictXML string) ([]byte, error) {
	payloadsRaw, err := extractPayloadsFromProfileXML(profileXML)
	if err != nil {
		return nil, err
	}

	var pcErr error
	newPayloadMap, mapErr := dictFileToPlistMap([]byte(newDictXML))
	if mapErr == nil {
		updatedRaw, replaceErr := plistMergePayloadByDomain(payloadsRaw, domain, newPayloadMap)
		if replaceErr == nil {
			return replacePayloadsContent(profileXML, updatedRaw)
		}
		pcErr = replaceErr
	} else {
		pcErr = mapErr
	}

	mcxDict := newDictXML
	if normalized, normErr := normaliseDictFile([]byte(newDictXML)); normErr == nil {
		mcxDict = normalized
	}
	updatedPlist, mcxErr := replaceMCXDict(payloadsRaw, domain, mcxDict)
	if mcxErr == nil {
		return replacePayloadsContent(profileXML, updatedPlist)
	}

	rows, _ := plistPayloadContentRows(payloadsRaw)
	if len(rows) > 0 {
		entries, _ := listMCXPayloadEntriesFromPayloadsPlist(payloadsRaw)
		msg := pcErr.Error()
		if names := mcxDomainNames(entries); len(names) > 0 {
			msg += fmt.Sprintf("; MCX domains: %v", names)
		}
		return nil, fmt.Errorf("%s", msg)
	}
	if mcxErr != nil {
		return nil, mcxErr
	}
	return nil, pcErr
}

// GetMCXDict returns the bare <dict>…</dict> fragment for mcx_preference_settings under domain.
func GetMCXDict(plistXML, domain string) (string, error) {
	return getMCXDict(plistXML, domain)
}

func getMCXDict(plistXML, domain string) (string, error) {
	dict, err := getMCXDictFlat(plistXML, domain)
	if err == nil {
		return dict, nil
	}
	entries, listErr := listMCXPayloadEntriesFromPayloadsPlist(plistXML)
	if listErr != nil {
		return "", err
	}
	for _, e := range entries {
		if e.Domain == domain {
			return mcxSettingsMapToDictFragment(e.SettingsDict)
		}
	}
	rows, _ := plistPayloadContentRows(plistXML)
	if len(entries) == 0 && len(rows) > 0 {
		return "", fmt.Errorf("no MCX preference domain %q; this plist has PayloadContent — use payloads_list_identifiers / payloads_get_payload", domain)
	}
	return "", fmt.Errorf("MCX preference domain %q not found", domain)
}

func getMCXDictFlat(plistXML, domain string) (string, error) {
	domainKey := "<key>" + xmlEncodePayload(domain) + "</key>"
	domainPos := strings.Index(plistXML, domainKey)
	if domainPos < 0 {
		return "", fmt.Errorf("domain %q not found in payloads plist", domain)
	}
	afterDomain := domainPos + len(domainKey)
	mcxKey := "<key>mcx_preference_settings</key>"
	mcxRelPos := strings.Index(plistXML[afterDomain:], mcxKey)
	if mcxRelPos < 0 {
		return "", fmt.Errorf("mcx_preference_settings not found for domain %q", domain)
	}
	afterMCX := afterDomain + mcxRelPos + len(mcxKey)

	dictRelPos := strings.Index(plistXML[afterMCX:], "<dict>")
	if dictRelPos < 0 {
		if strings.Contains(plistXML[afterMCX:], "<dict/>") || strings.Contains(plistXML[afterMCX:], "<dict />") {
			return "<dict></dict>", nil
		}
		return "", fmt.Errorf("no <dict> found after mcx_preference_settings for domain %q", domain)
	}
	dictOpenPos := afterMCX + dictRelPos
	dictCloseEnd := findMatchingCloseDict(plistXML, dictOpenPos)
	if dictCloseEnd < 0 {
		return "", fmt.Errorf("unmatched <dict> in mcx_preference_settings for domain %q", domain)
	}
	return plistXML[dictOpenPos:dictCloseEnd], nil
}

// ReplaceMCXDict navigates the plist XML to find the mcx_preference_settings
// <dict> under the given domain key and replaces it with newDictXML.
// newDictXML may be a bare <dict> fragment or a full plist from plist_encode;
// it is normalised and re-encoded so XML delimiters (& < >) are entity-escaped.
func ReplaceMCXDict(plist, domain, newDictXML string) (string, error) {
	return replaceMCXDict(plist, domain, newDictXML)
}

func replaceMCXDict(plistXML, domain, newDictXML string) (string, error) {
	fragment, err := normaliseDictFile([]byte(newDictXML))
	if err != nil {
		return "", fmt.Errorf("dict XML: %w", err)
	}
	m, err := dictFileToPlistMap([]byte(fragment))
	if err != nil {
		return "", fmt.Errorf("dict XML: %w", err)
	}
	var encBuf bytes.Buffer
	if err := plist.NewEncoder(&encBuf).Encode(m); err != nil {
		return "", fmt.Errorf("encode dict: %w", err)
	}
	fragment, err = normaliseDictFile(encBuf.Bytes())
	if err != nil {
		return "", err
	}

	if updated, ok, err := replaceMCXSettingsInPayloadsPlist(plistXML, domain, m); err != nil {
		return "", err
	} else if ok {
		return updated, nil
	}

	// Flat domain-key layout (legacy / simple plists).
	domainKey := "<key>" + xmlEncodePayload(domain) + "</key>"
	domainPos := strings.Index(plistXML, domainKey)
	if domainPos < 0 {
		return "", fmt.Errorf("domain %q not found in payloads plist", domain)
	}

	afterDomain := domainPos + len(domainKey)
	mcxKey := "<key>mcx_preference_settings</key>"
	mcxRelPos := strings.Index(plistXML[afterDomain:], mcxKey)
	if mcxRelPos < 0 {
		return "", fmt.Errorf("mcx_preference_settings not found for domain %q", domain)
	}
	afterMCX := afterDomain + mcxRelPos + len(mcxKey)

	dictRelPos := strings.Index(plistXML[afterMCX:], "<dict>")
	if dictRelPos < 0 {
		// Empty dict may be <dict/> or <dict />
		dictRelPos = strings.Index(plistXML[afterMCX:], "<dict/>")
		if dictRelPos < 0 {
			dictRelPos = strings.Index(plistXML[afterMCX:], "<dict />")
		}
		if dictRelPos < 0 {
			return "", fmt.Errorf("no <dict> found after mcx_preference_settings for domain %q", domain)
		}
	}
	dictOpenPos := afterMCX + dictRelPos

	var dictCloseEnd int
	if strings.HasPrefix(plistXML[dictOpenPos:], "<dict>") {
		dictCloseEnd = findMatchingCloseDict(plistXML, dictOpenPos)
		if dictCloseEnd < 0 {
			return "", fmt.Errorf("unmatched <dict> in mcx_preference_settings for domain %q", domain)
		}
	} else {
		// Self-closing <dict/> or <dict />
		dictCloseEnd = dictOpenPos + strings.Index(plistXML[dictOpenPos:], ">") + 1
	}

	return plistXML[:dictOpenPos] + fragment + plistXML[dictCloseEnd:], nil
}

// findMatchingCloseDict returns the index immediately after the </dict> that closes
// the <dict> starting at openPos, counting nesting depth.
func findMatchingCloseDict(s string, openPos int) int {
	depth := 0
	i := openPos
	for i < len(s) {
		switch {
		case strings.HasPrefix(s[i:], "<dict>"):
			depth++
			i += len("<dict>")
		case strings.HasPrefix(s[i:], "</dict>"):
			depth--
			if depth == 0 {
				return i + len("</dict>")
			}
			i += len("</dict>")
		default:
			i++
		}
	}
	return -1
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
