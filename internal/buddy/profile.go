package buddy

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
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
func ProfileFetchXML(credName string, profileID int) ([]byte, error) {
	return profileFetchRaw(credName, profileID)
}

// profileFetchRaw fetches the full profile XML and returns it as bytes.
func profileFetchRaw(credName string, profileID int) ([]byte, error) {
	url, err := ResolveEndpoint(credName, fmt.Sprintf("/JSSResource/osxconfigurationprofiles/id/%d", profileID))
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/xml")
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return nil, fmt.Errorf("fetch profile %d: %w", profileID, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fetch profile %d status %d: %s", profileID, resp.StatusCode, string(body))
	}
	return io.ReadAll(resp.Body)
}

// ExtractPayloadsFromProfileXML finds the <payloads> element in profile XML, unescapes its content, and returns it.
// Returns an error if the payloads element is missing.
func ExtractPayloadsFromProfileXML(profileXML []byte) (string, error) {
	return extractPayloadsFromProfileXML(profileXML)
}

// extractPayloadsFromProfileXML finds the <payloads> element in profile XML, unescapes its content, and returns it.
func extractPayloadsFromProfileXML(profileXML []byte) (string, error) {
	s := string(profileXML)
	const openTag = "<payloads>"
	const closeTag = "</payloads>"
	openIdx := strings.Index(s, openTag)
	closeIdx := strings.Index(s, closeTag)
	if openIdx < 0 || closeIdx < 0 || closeIdx < openIdx {
		return "", fmt.Errorf("could not find <payloads> element in profile XML")
	}
	contentStart := openIdx + len(openTag)
	contentEnd := closeIdx
	return XMLUnescape(s[contentStart:contentEnd]), nil
}

// PlistPayloadIdentifiers parses the payloads plist and returns PayloadIdentifier values from PayloadContent array.
// Returns nil slice if the plist does not have a PayloadContent array (e.g. MCX-style flat dict).
func PlistPayloadIdentifiers(payloadsRaw string) ([]string, error) {
	return plistPayloadIdentifiers(payloadsRaw)
}

func plistPayloadIdentifiers(payloadsRaw string) ([]string, error) {
	var root interface{}
	dec := plist.NewDecoder(strings.NewReader(payloadsRaw))
	if err := dec.Decode(&root); err != nil {
		return nil, fmt.Errorf("decode payloads plist: %w", err)
	}
	m, ok := root.(map[string]interface{})
	if !ok {
		return nil, nil
	}
	content, ok := m["PayloadContent"]
	if !ok {
		return nil, nil
	}
	arr, ok := content.([]interface{})
	if !ok {
		return nil, nil
	}
	var ids []string
	for _, item := range arr {
		dict, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if id, _ := dict["PayloadIdentifier"].(string); id != "" {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// PlistGetPayloadByDomain returns the single payload dict with PayloadIdentifier == domain as plist XML (dict fragment).
// If the plist uses PayloadContent array, finds that payload; otherwise returns error.
func PlistGetPayloadByDomain(payloadsRaw, domain string) ([]byte, error) {
	return plistGetPayloadByDomain(payloadsRaw, domain)
}

func plistGetPayloadByDomain(payloadsRaw, domain string) ([]byte, error) {
	var root interface{}
	dec := plist.NewDecoder(strings.NewReader(payloadsRaw))
	if err := dec.Decode(&root); err != nil {
		return nil, fmt.Errorf("decode payloads plist: %w", err)
	}
	m, ok := root.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("payloads root is not a dict")
	}
	content, ok := m["PayloadContent"]
	if !ok {
		return nil, fmt.Errorf("payloads plist has no PayloadContent array")
	}
	arr, ok := content.([]interface{})
	if !ok {
		return nil, fmt.Errorf("PayloadContent is not an array")
	}
	for _, item := range arr {
		dict, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if id, _ := dict["PayloadIdentifier"].(string); id == domain {
			var buf bytes.Buffer
			enc := plist.NewEncoder(&buf)
			if err := enc.Encode(dict); err != nil {
				return nil, fmt.Errorf("encode payload: %w", err)
			}
			return buf.Bytes(), nil
		}
	}
	return nil, fmt.Errorf("payload with PayloadIdentifier %q not found", domain)
}

// PlistReplacePayloadByDomain replaces the payload with PayloadIdentifier == domain in the payloads plist with newPayload.
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
	found := false
	for i, item := range arr {
		dict, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if id, _ := dict["PayloadIdentifier"].(string); id == domain {
			arr[i] = newDict
			found = true
			break
		}
	}
	if !found {
		return "", fmt.Errorf("payload with PayloadIdentifier %q not found", domain)
	}
	m["PayloadContent"] = arr
	return encodePayloadsRoot(m)
}

// PlistMergePayloadByDomain merges newPayload keys onto the existing PayloadContent entry for domain
// (new keys win; omitted keys are preserved, including PayloadUUID). Sets PayloadIdentifier to domain if missing.
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
	found := false
	for i, item := range arr {
		dict, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if id, _ := dict["PayloadIdentifier"].(string); id == domain {
			merged := make(map[string]interface{}, len(dict)+len(newDict))
			for k, v := range dict {
				merged[k] = v
			}
			for k, v := range newDict {
				merged[k] = v
			}
			if _, ok := merged["PayloadIdentifier"]; !ok || merged["PayloadIdentifier"] == "" {
				merged["PayloadIdentifier"] = domain
			}
			arr[i] = merged
			found = true
			break
		}
	}
	if !found {
		return "", fmt.Errorf("payload with PayloadIdentifier %q not found", domain)
	}
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
			return "", fmt.Errorf("decode payloads plist: %w", err)
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

// PlistAddPayload appends (or replaces) a payload dict keyed by PayloadIdentifier == domain.
// If payloadsRaw is empty, creates a new payloads plist with a single PayloadContent entry.
func PlistAddPayload(payloadsRaw, domain string, newPayload interface{}) (string, error) {
	return plistAddPayload(payloadsRaw, domain, newPayload)
}

func plistAddPayload(payloadsRaw, domain string, newPayload interface{}) (string, error) {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return "", fmt.Errorf("domain is required")
	}
	newDict, err := coercePayloadDict(newPayload)
	if err != nil {
		return "", err
	}
	if id, _ := newDict["PayloadIdentifier"].(string); id == "" {
		newDict["PayloadIdentifier"] = domain
	} else if id != domain {
		return "", fmt.Errorf("new payload PayloadIdentifier %q does not match domain %q", id, domain)
	}

	if strings.TrimSpace(payloadsRaw) == "" {
		return plistSetPayloadContent("", []interface{}{newDict})
	}

	m, arr, err := decodePayloadContentOrEmpty(payloadsRaw)
	if err != nil {
		return "", err
	}
	found := false
	for i, item := range arr {
		dict, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if id, _ := dict["PayloadIdentifier"].(string); id == domain {
			arr[i] = newDict
			found = true
			break
		}
	}
	if !found {
		arr = append(arr, newDict)
	}
	m["PayloadContent"] = arr
	return encodePayloadsRoot(m)
}

// PlistRemovePayload removes the payload with PayloadIdentifier == domain from PayloadContent.
func PlistRemovePayload(payloadsRaw, domain string) (string, error) {
	return plistRemovePayload(payloadsRaw, domain)
}

func plistRemovePayload(payloadsRaw, domain string) (string, error) {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return "", fmt.Errorf("domain is required")
	}
	m, arr, err := decodePayloadContent(payloadsRaw)
	if err != nil {
		return "", err
	}
	out := make([]interface{}, 0, len(arr))
	found := false
	for _, item := range arr {
		dict, ok := item.(map[string]interface{})
		if !ok {
			out = append(out, item)
			continue
		}
		if id, _ := dict["PayloadIdentifier"].(string); id == domain {
			found = true
			continue
		}
		out = append(out, item)
	}
	if !found {
		return "", fmt.Errorf("payload with PayloadIdentifier %q not found", domain)
	}
	m["PayloadContent"] = out
	return encodePayloadsRoot(m)
}

func decodePayloadContent(payloadsRaw string) (map[string]interface{}, []interface{}, error) {
	var root interface{}
	if err := plist.NewDecoder(strings.NewReader(payloadsRaw)).Decode(&root); err != nil {
		return nil, nil, fmt.Errorf("decode payloads plist: %w", err)
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
		return nil, nil, fmt.Errorf("decode payloads plist: %w", err)
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
	url, err := ResolveEndpoint(credName, "/JSSResource/osxconfigurationprofiles")
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/xml")
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return nil, fmt.Errorf("list profiles: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list profiles status %d: %s", resp.StatusCode, string(body))
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
func ProfileList(credName string, nameFilter string, logger *slog.Logger) error {
	records, err := ProfileSearch(credName, nameFilter, logger)
	if err != nil {
		return err
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
	url, err := ResolveEndpoint(credName, fmt.Sprintf("/JSSResource/osxconfigurationprofiles/id/%d", profileID))
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(updatedXML))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/xml")
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return fmt.Errorf("redeploy profile: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("redeploy profile status %d: %s", resp.StatusCode, string(body))
	}
	fmt.Printf("Redeployed configuration profile (ID %d)\n", profileID)
	return nil
}

// ProfileGetXML fetches a macOS configuration profile and writes its XML to w (typically os.Stdout).
// Uses a raw HTTP GET to preserve the exact Jamf Pro XML, including base64-encoded mobileconfig payloads,
// which can be corrupted if round-tripped through Go struct marshaling.
func ProfileGetXML(credName string, profileID int, w io.Writer, logger *slog.Logger) error {
	url, err := ResolveEndpoint(credName, fmt.Sprintf("/JSSResource/osxconfigurationprofiles/id/%d", profileID))
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/xml")
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return fmt.Errorf("fetch profile %d: %w", profileID, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("fetch profile %d status %d: %s", profileID, resp.StatusCode, string(body))
	}
	_, err = io.Copy(w, resp.Body)
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

// ProfileGetPayloadByDomain fetches the profile and writes the single payload with PayloadIdentifier == domain to w
// as plist XML (a single dict). Use for extracting one payload for editing or re-use with set payload.
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

// ProfileListPayloads fetches the profile and returns the list of PayloadIdentifier values (domains) in the profile.
// Returns nil slice if the payloads plist does not use PayloadContent array (e.g. MCX-style).
func ProfileListPayloads(credName string, profileID int, logger *slog.Logger) ([]string, error) {
	profileXML, err := profileFetchRaw(credName, profileID)
	if err != nil {
		return nil, err
	}
	payloadsRaw, err := extractPayloadsFromProfileXML(profileXML)
	if err != nil {
		return nil, err
	}
	return plistPayloadIdentifiers(payloadsRaw)
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
	url, err := ResolveEndpoint(credName, fmt.Sprintf("/JSSResource/osxconfigurationprofiles/id/%d", profileID))
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/xml")
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return fmt.Errorf("update profile: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("update profile status %d: %s", resp.StatusCode, string(body))
	}
	fmt.Printf("Updated configuration profile (ID %d)\n", profileID)
	return nil
}

// ProfilePutXML writes the given profile XML to Jamf for the given profile ID (for script/component use).
// If redeploy is true, sets redeploy_on_update to All. Uses opts.SkipConfirm to skip confirmation when called from scripts.
func ProfilePutXML(credName string, opts Opts, profileID int, xml []byte, redeploy bool, logger *slog.Logger) error {
	data := xml
	if !bytes.Contains(data, []byte("<os_x_configuration_profile")) {
		return fmt.Errorf("input does not appear to be macOS configuration profile XML (missing <os_x_configuration_profile> root)")
	}
	if redeploy {
		data = SetRedeployAll(data)
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
	url, err := ResolveEndpoint(credName, fmt.Sprintf("/JSSResource/osxconfigurationprofiles/id/%d", profileID))
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/xml")
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return fmt.Errorf("update profile: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("update profile status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// ProfileSetPayload patches a payload domain inside a profile (PayloadContent merge by
// PayloadIdentifier, or MCX mcx_preference_settings settings dict). It fetches the current
// profile from Jamf, patches in place, and PUTs the result back.
//
// Usage: buddy profile set payload <profile-id> <domain> <dict-file.xml>
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

	url, err := ResolveEndpoint(credName, fmt.Sprintf("/JSSResource/osxconfigurationprofiles/id/%d", profileID))
	if err != nil {
		return err
	}

	getReq, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	getReq.Header.Set("Accept", "application/xml")
	getResp, err := PerformRequest(nil, getReq, credName)
	if err != nil {
		return fmt.Errorf("fetch profile %d: %w", profileID, err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(getResp.Body)
		return fmt.Errorf("fetch profile %d status %d: %s", profileID, getResp.StatusCode, string(body))
	}
	profileXML, err := io.ReadAll(getResp.Body)
	if err != nil {
		return fmt.Errorf("read profile response: %w", err)
	}

	updatedXML, err := patchProfilePayload(profileXML, domain, newDict)
	if err != nil {
		return err
	}
	if redeploy {
		updatedXML = setRedeployAll(updatedXML)
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

	putReq, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(updatedXML))
	if err != nil {
		return fmt.Errorf("create put request: %w", err)
	}
	putReq.Header.Set("Content-Type", "application/xml")
	putResp, err := PerformRequest(nil, putReq, credName)
	if err != nil {
		return fmt.Errorf("update profile: %w", err)
	}
	defer putResp.Body.Close()
	if putResp.StatusCode != http.StatusOK && putResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(putResp.Body)
		return fmt.Errorf("update profile status %d: %s", putResp.StatusCode, string(body))
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
	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "\n--- POST payload (JSSResource/osxconfigurationprofiles/id/0) ---\n%s\n--- end payload ---\n\n", xmlBody)
	}
	postURL, err := ResolveEndpoint(credName, "/JSSResource/osxconfigurationprofiles/id/0")
	if err != nil {
		return nil, err
	}
	postReq, err := http.NewRequest(http.MethodPost, postURL, bytes.NewReader(xmlBody))
	if err != nil {
		return nil, fmt.Errorf("create post request: %w", err)
	}
	postReq.Header.Set("Content-Type", "application/xml")
	postResp, err := PerformRequest(nil, postReq, credName)
	if err != nil {
		return nil, fmt.Errorf("create profile: %w", err)
	}
	defer postResp.Body.Close()
	body, _ := io.ReadAll(postResp.Body)
	if postResp.StatusCode != http.StatusOK && postResp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("create profile status %d: %s", postResp.StatusCode, string(body))
	}
	id, err := resolveCreatedProfileID(credName, opts, body, spec.Name, logger)
	if err != nil {
		return nil, err
	}
	fmt.Printf("Created configuration profile %q (ID %d)\n", spec.Name, id)
	return &ProfileCreateOutcome{ProfileID: id, Name: spec.Name}, nil
}

// ProfileClone creates a copy of a configuration profile with the given name and
// unassigned scope (all_computers=false, empty targets — not "all computers").
func ProfileClone(credName string, opts Opts, sourceProfileID int, newName string, logger *slog.Logger) error {
	profileXML, err := profileFetchRaw(credName, sourceProfileID)
	if err != nil {
		return err
	}
	xml := profileXML

	// Extract current UUID from <general><uuid>
	uuidRe := regexp.MustCompile(`<uuid>([a-fA-F0-9\-]{36})</uuid>`)
	uuidMatch := uuidRe.FindSubmatch(xml)
	if uuidMatch == nil {
		return fmt.Errorf("profile XML has no <uuid> in general")
	}
	oldUUID := string(uuidMatch[1])
	newUUID := uuid.New().String()

	// Replace old UUID with new UUID throughout
	xml = bytes.ReplaceAll(xml, []byte(oldUUID), []byte(newUUID))

	// Replace <general><name> with new name (first <name> in doc is profile name)
	nameStart := bytes.Index(xml, []byte("<name>"))
	if nameStart < 0 {
		return fmt.Errorf("profile XML has no <name> in general")
	}
	nameEnd := bytes.Index(xml[nameStart:], []byte("</name>"))
	if nameEnd < 0 {
		return fmt.Errorf("profile XML has malformed <name>")
	}
	nameEnd += nameStart + len("</name>")
	xml = bytes.Join([][]byte{xml[:nameStart], []byte("<name>" + xmlEncodePayload(newName) + "</name>"), xml[nameEnd:]}, nil)

	// Replace entire <scope>...</scope> with explicit unassigned scope (see emptyScopeXML).
	scopeStart := bytes.Index(xml, []byte("<scope>"))
	if scopeStart < 0 {
		return fmt.Errorf("profile XML has no <scope>")
	}
	relEnd := bytes.Index(xml[scopeStart:], []byte("</scope>"))
	if relEnd < 0 {
		return fmt.Errorf("profile XML has malformed <scope>")
	}
	scopeEnd := scopeStart + relEnd + len("</scope>")
	xml = bytes.Join([][]byte{xml[:scopeStart], []byte(emptyScopeXML), xml[scopeEnd:]}, nil)

	// Remove <id>...</id> from general so Jamf creates a new profile (only the first occurrence—the profile id).
	idRe := regexp.MustCompile(`<id>\d+</id>\s*`)
	if idx := idRe.FindIndex(xml); idx != nil {
		xml = append(xml[:idx[0]], xml[idx[1]:]...)
	}

	if !opts.SkipConfirm {
		msg := fmt.Sprintf("\nClone configuration profile (ID %d) as %q with unassigned scope (not all computers)?", sourceProfileID, newName)
		if !ConfirmPrompt(msg, logger) {
			return nil
		}
	}

	postURL, err := ResolveEndpoint(credName, "/JSSResource/osxconfigurationprofiles/id/0")
	if err != nil {
		return err
	}
	postReq, err := http.NewRequest(http.MethodPost, postURL, bytes.NewReader(xml))
	if err != nil {
		return fmt.Errorf("create post request: %w", err)
	}
	postReq.Header.Set("Content-Type", "application/xml")
	postResp, err := PerformRequest(nil, postReq, credName)
	if err != nil {
		return fmt.Errorf("clone profile: %w", err)
	}
	defer postResp.Body.Close()
	if postResp.StatusCode != http.StatusOK && postResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(postResp.Body)
		return fmt.Errorf("clone profile status %d: %s", postResp.StatusCode, string(body))
	}
	fmt.Printf("Cloned profile (ID %d) as %q with unassigned scope\n", sourceProfileID, newName)
	return nil
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
	s := string(profileXML)
	const openTag = "<payloads>"
	const closeTag = "</payloads>"
	openIdx := strings.Index(s, openTag)
	closeIdx := strings.Index(s, closeTag)
	if openIdx < 0 || closeIdx < 0 || closeIdx < openIdx {
		return nil, fmt.Errorf("profile XML has no <payloads> element")
	}
	contentStart := openIdx + len(openTag)
	contentEnd := closeIdx
	reEncoded := xmlEncodePayload(newPayloadsRaw)
	var buf strings.Builder
	buf.WriteString(s[:contentStart])
	buf.WriteString(reEncoded)
	buf.WriteString(s[contentEnd:])
	return []byte(buf.String()), nil
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

// patchProfilePayload locates the entity-encoded plist inside <payloads>, decodes it,
// merges/replaces the payload for the given domain (by PayloadIdentifier in PayloadContent, or MCX mcx_preference_settings),
// re-encodes it, and returns the reconstructed profile XML. Preserves the rest of the profile including UUID.
// PayloadContent path merges keys; MCX path replaces the settings dict only.
func patchProfilePayload(profileXML []byte, domain, newDictXML string) ([]byte, error) {
	payloadsRaw, err := extractPayloadsFromProfileXML(profileXML)
	if err != nil {
		return nil, err
	}

	// Try plist-based merge (PayloadContent array, PayloadIdentifier).
	newPayloadMap, mapErr := dictFileToPlistMap([]byte(newDictXML))
	if mapErr == nil {
		updatedRaw, replaceErr := plistMergePayloadByDomain(payloadsRaw, domain, newPayloadMap)
		if replaceErr == nil {
			return replacePayloadsContent(profileXML, updatedRaw)
		}
	}

	// Fall back to MCX-style replace (domain key + mcx_preference_settings dict).
	mcxDict := newDictXML
	if normalized, normErr := normaliseDictFile([]byte(newDictXML)); normErr == nil {
		mcxDict = normalized
	}
	updatedPlist, err := replaceMCXDict(payloadsRaw, domain, mcxDict)
	if err != nil {
		return nil, err
	}

	return replacePayloadsContent(profileXML, updatedPlist)
}

// GetMCXDict returns the bare <dict>…</dict> fragment for mcx_preference_settings under domain.
func GetMCXDict(plistXML, domain string) (string, error) {
	return getMCXDict(plistXML, domain)
}

func getMCXDict(plistXML, domain string) (string, error) {
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
	// Accept bare <dict> or full plist_encode output; re-encode so & < > are entity-escaped.
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

	// Keys in XML are entity-encoded; search with the same escaping.
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
