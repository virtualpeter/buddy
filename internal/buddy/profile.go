package buddy

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"os"
	"regexp"
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
	XMLName   xml.Name `xml:"os_x_configuration_profiles"`
	Profiles  []struct {
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
	baseURL, err := GetBaseURL(credName)
	if err != nil {
		return nil, err
	}
	url := baseURL + fmt.Sprintf("/JSSResource/osxconfigurationprofiles/id/%d", profileID)
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
	return html.UnescapeString(s[contentStart:contentEnd]), nil
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
func PlistReplacePayloadByDomain(payloadsRaw, domain string, newPayload interface{}) (string, error) {
	return plistReplacePayloadByDomain(payloadsRaw, domain, newPayload)
}

func plistReplacePayloadByDomain(payloadsRaw, domain string, newPayload interface{}) (string, error) {
	var root interface{}
	dec := plist.NewDecoder(strings.NewReader(payloadsRaw))
	if err := dec.Decode(&root); err != nil {
		return "", fmt.Errorf("decode payloads plist: %w", err)
	}
	m, ok := root.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("payloads root is not a dict")
	}
	content, ok := m["PayloadContent"]
	if !ok {
		return "", fmt.Errorf("payloads plist has no PayloadContent array")
	}
	arr, ok := content.([]interface{})
	if !ok {
		return "", fmt.Errorf("PayloadContent is not an array")
	}
	var newDict map[string]interface{}
	switch v := newPayload.(type) {
	case map[string]interface{}:
		newDict = v
	default:
		var decoded interface{}
		// Re-encode and decode to get map[string]interface{}
		var buf bytes.Buffer
		if err := plist.NewEncoder(&buf).Encode(newPayload); err != nil {
			return "", fmt.Errorf("encode new payload: %w", err)
		}
		if err := plist.NewDecoder(bytes.NewReader(buf.Bytes())).Decode(&decoded); err != nil {
			return "", fmt.Errorf("decode new payload: %w", err)
		}
		newDict, ok = decoded.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("new payload is not a dict")
		}
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
	var out bytes.Buffer
	if err := plist.NewEncoder(&out).Encode(root); err != nil {
		return "", fmt.Errorf("encode payloads plist: %w", err)
	}
	return out.String(), nil
}

// ProfileSearch fetches all configuration profiles and returns records matching the optional name filter.
// If nameFilter is empty, returns all profiles. Otherwise filters by name (case-insensitive contains).
func ProfileSearch(credName string, nameFilter string, logger *slog.Logger) ([]ProfileRecord, error) {
	baseURL, err := GetBaseURL(credName)
	if err != nil {
		return nil, err
	}
	url := baseURL + "/JSSResource/osxconfigurationprofiles"
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
	redeployBase, err := GetBaseURL(credName)
	if err != nil {
		return err
	}
	url := redeployBase + fmt.Sprintf("/JSSResource/osxconfigurationprofiles/id/%d", profileID)
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
	baseURL, err := GetBaseURL(credName)
	if err != nil {
		return err
	}
	url := baseURL + fmt.Sprintf("/JSSResource/osxconfigurationprofiles/id/%d", profileID)
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
	setBase, err := GetBaseURL(credName)
	if err != nil {
		return err
	}
	url := setBase + fmt.Sprintf("/JSSResource/osxconfigurationprofiles/id/%d", profileID)
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
	setBase, err := GetBaseURL(credName)
	if err != nil {
		return err
	}
	url := setBase + fmt.Sprintf("/JSSResource/osxconfigurationprofiles/id/%d", profileID)
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

// ProfileSetPayload replaces the mcx_preference_settings <dict> for a specific MCX domain
// within a profile's entity-encoded payloads plist. It fetches the current profile from
// Jamf, patches the target dict in place, and PUTs the result back.
//
// Usage: buddy profile set payload <profile-id> <domain> <dict-file.xml>
//
// The dict file should be a standalone <dict>...</dict> XML fragment (the
// mcx_preference_settings content, NOT the full plist).
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

	profileBase, err := GetBaseURL(credName)
	if err != nil {
		return err
	}
	url := profileBase + fmt.Sprintf("/JSSResource/osxconfigurationprofiles/id/%d", profileID)

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
// all_computers=false and all_jss_users=false with empty target lists = unassigned.
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

// ProfileClone creates a copy of a configuration profile with the given name and no scope.
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
	xml = bytes.Join([][]byte{xml[:nameStart], []byte("<name>" + newName + "</name>"), xml[nameEnd:]}, nil)

	// Replace entire <scope>...</scope> block with empty scope
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
		msg := fmt.Sprintf("\nClone configuration profile (ID %d) as %q with no scope?", sourceProfileID, newName)
		if !ConfirmPrompt(msg, logger) {
			return nil
		}
	}

	baseURL, err := GetBaseURL(credName)
	if err != nil {
		return err
	}
	postURL := baseURL + "/JSSResource/osxconfigurationprofiles/id/0"
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
	fmt.Printf("Cloned profile (ID %d) as %q with no scope\n", sourceProfileID, newName)
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
// entity-encodes it, and returns the updated profile XML.
func ReplacePayloadsContent(profileXML []byte, newPayloadsRaw string) []byte {
	return replacePayloadsContent(profileXML, newPayloadsRaw)
}

func replacePayloadsContent(profileXML []byte, newPayloadsRaw string) []byte {
	s := string(profileXML)
	const openTag = "<payloads>"
	const closeTag = "</payloads>"
	openIdx := strings.Index(s, openTag)
	closeIdx := strings.Index(s, closeTag)
	if openIdx < 0 || closeIdx < 0 || closeIdx < openIdx {
		return profileXML
	}
	contentStart := openIdx + len(openTag)
	contentEnd := closeIdx
	reEncoded := xmlEncodePayload(newPayloadsRaw)
	var buf strings.Builder
	buf.WriteString(s[:contentStart])
	buf.WriteString(reEncoded)
	buf.WriteString(s[contentEnd:])
	return []byte(buf.String())
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
// replaces the payload for the given domain (by PayloadIdentifier in PayloadContent, or MCX mcx_preference_settings),
// re-encodes it, and returns the reconstructed profile XML. Preserves the rest of the profile including UUID.
func patchProfilePayload(profileXML []byte, domain, newDictXML string) ([]byte, error) {
	payloadsRaw, err := extractPayloadsFromProfileXML(profileXML)
	if err != nil {
		return nil, err
	}

	// Try plist-based replace (PayloadContent array, PayloadIdentifier).
	newPayloadMap, mapErr := dictFileToPlistMap([]byte(newDictXML))
	if mapErr == nil {
		updatedRaw, replaceErr := plistReplacePayloadByDomain(payloadsRaw, domain, newPayloadMap)
		if replaceErr == nil {
			return replacePayloadsContent(profileXML, updatedRaw), nil
		}
	}

	// Fall back to MCX-style replace (domain key + mcx_preference_settings dict).
	updatedPlist, err := replaceMCXDict(payloadsRaw, domain, newDictXML)
	if err != nil {
		return nil, err
	}

	const openTag = "<payloads>"
	const closeTag = "</payloads>"
	s := string(profileXML)
	openIdx := strings.Index(s, openTag)
	closeIdx := strings.Index(s, closeTag)
	if openIdx < 0 || closeIdx < 0 || closeIdx < openIdx {
		return nil, fmt.Errorf("could not find <payloads> element in profile XML")
	}
	contentStart := openIdx + len(openTag)
	contentEnd := closeIdx
	reEncoded := xmlEncodePayload(updatedPlist)
	var buf strings.Builder
	buf.WriteString(s[:contentStart])
	buf.WriteString(reEncoded)
	buf.WriteString(s[contentEnd:])
	return []byte(buf.String()), nil
}

// ReplaceMCXDict navigates the decoded plist XML to find the mcx_preference_settings
// <dict> under the given domain key and replaces it with newDictXML.
func ReplaceMCXDict(plist, domain, newDictXML string) (string, error) {
	return replaceMCXDict(plist, domain, newDictXML)
}

func replaceMCXDict(plist, domain, newDictXML string) (string, error) {
	domainKey := "<key>" + domain + "</key>"
	domainPos := strings.Index(plist, domainKey)
	if domainPos < 0 {
		return "", fmt.Errorf("domain %q not found in payloads plist", domain)
	}

	afterDomain := domainPos + len(domainKey)
	mcxKey := "<key>mcx_preference_settings</key>"
	mcxRelPos := strings.Index(plist[afterDomain:], mcxKey)
	if mcxRelPos < 0 {
		return "", fmt.Errorf("mcx_preference_settings not found for domain %q", domain)
	}
	afterMCX := afterDomain + mcxRelPos + len(mcxKey)

	dictRelPos := strings.Index(plist[afterMCX:], "<dict>")
	if dictRelPos < 0 {
		// Empty dict may be <dict/> or <dict />
		dictRelPos = strings.Index(plist[afterMCX:], "<dict/>")
		if dictRelPos < 0 {
			dictRelPos = strings.Index(plist[afterMCX:], "<dict />")
		}
		if dictRelPos < 0 {
			return "", fmt.Errorf("no <dict> found after mcx_preference_settings for domain %q", domain)
		}
	}
	dictOpenPos := afterMCX + dictRelPos

	var dictCloseEnd int
	if strings.HasPrefix(plist[dictOpenPos:], "<dict>") {
		dictCloseEnd = findMatchingCloseDict(plist, dictOpenPos)
		if dictCloseEnd < 0 {
			return "", fmt.Errorf("unmatched <dict> in mcx_preference_settings for domain %q", domain)
		}
	} else {
		// Self-closing <dict/> or <dict />
		dictCloseEnd = dictOpenPos + strings.Index(plist[dictOpenPos:], ">") + 1
	}

	return plist[:dictOpenPos] + newDictXML + plist[dictCloseEnd:], nil
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

// xmlEncodePayload escapes a string for embedding as XML element content.
// & must be replaced before < and > to avoid double-encoding.
func xmlEncodePayload(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
