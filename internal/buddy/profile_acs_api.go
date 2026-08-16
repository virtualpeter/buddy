package buddy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"howett.net/plist"
)

const (
	acsConfigProfilesPath        = "/api/config-profiles/macos"
	acsConfigProfilesPlistPath   = "/api/config-profiles/plist/can-convert-to-plist"
	managedClientPreferencesType = "com.apple.ManagedClient.preferences"
	acsConfigProfileLevelSystem  = "SYSTEM"
)

// ACSConfigProfileDoc is the JSON document for GET/PUT/POST /api/config-profiles/macos/….
type ACSConfigProfileDoc struct {
	PayloadUUID             *string             `json:"payloadUUID,omitempty"`
	Level                   string              `json:"level,omitempty"`
	Name                    string              `json:"name,omitempty"`
	ClassicProfileID        *int                `json:"classicProfileId,omitempty"`
	ExternalConfigProfileID *int                `json:"externalConfigProfileId,omitempty"`
	ProfileID               *int                `json:"profileId,omitempty"`
	PayloadContent          []ACSPayloadContent `json:"payloadContent,omitempty"`
}

// ACSPayloadContent is one ACS/MCX payload entry in the ACS API.
type ACSPayloadContent struct {
	PayloadType         string       `json:"payloadType,omitempty"`
	PayloadVersion      int          `json:"payloadVersion,omitempty"`
	PayloadIdentifier   string       `json:"payloadIdentifier,omitempty"`
	PayloadUUID         string       `json:"payloadUUID,omitempty"`
	PayloadOrganization string       `json:"payloadOrganization,omitempty"`
	PayloadDisplayName  string       `json:"payloadDisplayName,omitempty"`
	PreferenceDomain    string       `json:"preferenceDomain,omitempty"`
	Forced              *ACSForced   `json:"forced,omitempty"`
	SetOnce             *acsJSONNull `json:"setOnce,omitempty"`
}

// ACSForced holds raw plist XML for MCX settings (single-layer &amp; in strings).
type ACSForced struct {
	Plist string `json:"plist"`
}

// acsJSONNull marshals as JSON null (Jamf UI sends "setOnce": null on ACS payloads).
type acsJSONNull struct{}

func (acsJSONNull) MarshalJSON() ([]byte, error) { return []byte("null"), nil }

var acsSetOnceNull = acsJSONNull{}

// MCXPayloadEntry describes one MCX preference domain extracted from a profile payloads plist.
type MCXPayloadEntry struct {
	Domain         string
	SettingsDict   map[string]any
	HasAmpersand   bool
	ForcedPlistRaw string // optional Jamf ACS forced.plist XML; preferred on ACS PUT when set
}

// ProfileACSWritePreferred reports whether the profile should use the ACS API
// instead of (or after) Classic POST/PUT for payload writes.
//
// Eligible when MCX preference strings use Jamf ACS Apply form (&amp; in the value) —
// Classic often 409s on those — or when outer <payloads> encoding is incompatible with Classic.
// Bare '&' in preference strings is not eligible: rejectBareMCXAmpersandSettings fails those first.
func ProfileACSWritePreferred(profileXML []byte) (bool, string) {
	entries, err := listMCXPayloadEntries(profileXML)
	if err != nil {
		return false, ""
	}
	var ampDomains []string
	for _, e := range entries {
		if e.HasAmpersand && !plistMapHasBareAmpersand(e.SettingsDict) {
			ampDomains = append(ampDomains, e.Domain)
		}
	}
	if len(ampDomains) > 0 {
		return true, fmt.Sprintf("MCX Apply-form ampersands in %v", ampDomains)
	}
	if enc, err := extractPayloadsEncodedFromProfileXML(profileXML); err == nil {
		if err := validateJamfEncodedPayloads(enc); err != nil {
			if len(entries) > 0 {
				return true, "payloads encoding incompatible with Classic: " + err.Error()
			}
		}
	}
	return false, ""
}

func listMCXPayloadEntries(profileXML []byte) ([]MCXPayloadEntry, error) {
	payloadsRaw, err := extractPayloadsFromProfileXML(profileXML)
	if err != nil {
		return nil, err
	}
	return listMCXPayloadEntriesFromPayloadsPlist(payloadsRaw)
}

func listMCXPayloadEntriesFromPayloadsPlist(payloadsRaw string) ([]MCXPayloadEntry, error) {
	if strings.TrimSpace(payloadsRaw) == "" {
		return nil, nil
	}
	var root map[string]any
	if err := plist.NewDecoder(strings.NewReader(payloadsRaw)).Decode(&root); err != nil {
		return nil, fmt.Errorf(errDecodePayloadsPlist, err)
	}
	return listMCXPayloadEntriesFromRoot(root), nil
}

func listMCXPayloadEntriesFromRoot(root map[string]any) []MCXPayloadEntry {
	var out []MCXPayloadEntry
	if content, ok := root["PayloadContent"].([]any); ok {
		for _, item := range content {
			payload, ok := item.(map[string]any)
			if !ok {
				continue
			}
			pt, _ := payload["PayloadType"].(string)
			if pt != managedClientPreferencesType {
				continue
			}
			out = append(out, collectMCXEntriesFromManagedClientPayload(payload)...)
		}
	}
	for k, v := range root {
		if k == "PayloadContent" || k == "PayloadUUID" || k == "PayloadType" || k == "PayloadVersion" {
			continue
		}
		domainDict, ok := v.(map[string]any)
		if !ok {
			continue
		}
		settings, ok := domainDict["mcx_preference_settings"].(map[string]any)
		if !ok {
			continue
		}
		out = append(out, MCXPayloadEntry{
			Domain:       k,
			SettingsDict: settings,
			HasAmpersand: plistMapHasMCXAmpersandIssue(settings),
		})
	}
	return out
}

func collectMCXEntriesFromManagedClientPayload(payload map[string]any) []MCXPayloadEntry {
	var out []MCXPayloadEntry
	if nested, ok := payload["PayloadContent"].(map[string]any); ok {
		for domain, val := range nested {
			domainDict, ok := val.(map[string]any)
			if !ok {
				continue
			}
			settings := extractMCXSettingsFromDomainDict(domainDict)
			if settings == nil {
				continue
			}
			out = append(out, MCXPayloadEntry{
				Domain:       domain,
				SettingsDict: settings,
				HasAmpersand: plistMapHasMCXAmpersandIssue(settings),
			})
		}
	}
	domain, _ := payload["PayloadIdentifier"].(string)
	if settings := extractMCXSettingsFromPayloadItem(payload); settings != nil {
		if domain == "" {
			domain, _ = payload["PreferenceDomain"].(string)
		}
		out = append(out, MCXPayloadEntry{
			Domain:       domain,
			SettingsDict: settings,
			HasAmpersand: plistMapHasMCXAmpersandIssue(settings),
		})
	}
	return out
}

func extractMCXSettingsFromDomainDict(domainDict map[string]any) map[string]any {
	if forced, ok := domainDict["Forced"].([]any); ok && len(forced) > 0 {
		if item, ok := forced[0].(map[string]any); ok {
			if s, ok := item["mcx_preference_settings"].(map[string]any); ok {
				return s
			}
		}
	}
	if s, ok := domainDict["mcx_preference_settings"].(map[string]any); ok {
		return s
	}
	return nil
}

func extractMCXSettingsFromPayloadItem(payload map[string]any) map[string]any {
	return extractMCXSettingsFromDomainDict(payload)
}

func mcxSettingsMapToDictFragment(settings map[string]any) (string, error) {
	if settings == nil {
		return "", fmt.Errorf("MCX settings dict is nil")
	}
	var buf bytes.Buffer
	if err := plist.NewEncoder(&buf).Encode(settings); err != nil {
		return "", err
	}
	return normaliseDictFile(buf.Bytes())
}

// replaceMCXSettingsInPayloadsPlist replaces mcx_preference_settings for domain in a payloads plist
// (flat domain keys or Jamf ACS ManagedClient.preferences nested PayloadContent).
func replaceMCXSettingsInPayloadsPlist(payloadsRaw, domain string, settings map[string]any) (string, bool, error) {
	if strings.TrimSpace(payloadsRaw) == "" {
		return "", false, nil
	}
	var root map[string]any
	if err := plist.NewDecoder(strings.NewReader(payloadsRaw)).Decode(&root); err != nil {
		return "", false, fmt.Errorf(errDecodePayloadsPlist, err)
	}
	if !replaceMCXSettingsInRoot(root, domain, settings) {
		return "", false, nil
	}
	updated, err := encodePayloadsRootAny(root)
	if err != nil {
		return "", false, err
	}
	return updated, true, nil
}

func encodePayloadsRootAny(root map[string]any) (string, error) {
	rootIface := make(map[string]interface{}, len(root))
	for k, v := range root {
		rootIface[k] = v
	}
	return encodePayloadsRoot(rootIface)
}

func replaceMCXSettingsInRoot(root map[string]any, domain string, settings map[string]any) bool {
	if domainDict, ok := root[domain].(map[string]any); ok {
		domainDict["mcx_preference_settings"] = settings
		return true
	}
	content, ok := root["PayloadContent"].([]any)
	if !ok {
		return false
	}
	for _, item := range content {
		payload, ok := item.(map[string]any)
		if !ok {
			continue
		}
		pt, _ := payload["PayloadType"].(string)
		if pt != managedClientPreferencesType {
			continue
		}
		if nested, ok := payload["PayloadContent"].(map[string]any); ok {
			if domainDict, ok := nested[domain].(map[string]any); ok {
				if setMCXSettingsOnDomainDict(domainDict, settings) {
					return true
				}
			}
		}
		if pid, _ := payload["PayloadIdentifier"].(string); pid == domain || pid == "" {
			if pref, _ := payload["PreferenceDomain"].(string); pref == domain || pref == "" {
				if setMCXSettingsOnDomainDict(payload, settings) {
					return true
				}
			}
		}
	}
	return false
}

func setMCXSettingsOnDomainDict(domainDict map[string]any, settings map[string]any) bool {
	if forced, ok := domainDict["Forced"].([]any); ok && len(forced) > 0 {
		if item, ok := forced[0].(map[string]any); ok {
			item["mcx_preference_settings"] = settings
			return true
		}
	}
	domainDict["mcx_preference_settings"] = settings
	return true
}

func plistMapHasMCXAmpersandIssue(v any) bool {
	switch t := v.(type) {
	case string:
		if strings.Contains(t, "%26") {
			return false
		}
		// Jamf ACS Apply form stores &amp; entity text in preference strings (e.g. URL query params).
		// Classic often 409s on those; ACS path + Classic retention handle them.
		if strings.Contains(t, "&amp;") {
			return true
		}
		// Bare '&' is rejected at write time; still flagged for analysis / strip helpers.
		return hasBareAmpersandInXML(t)
	case map[string]any:
		for _, val := range t {
			if plistMapHasMCXAmpersandIssue(val) {
				return true
			}
		}
	case []any:
		for _, val := range t {
			if plistMapHasMCXAmpersandIssue(val) {
				return true
			}
		}
	}
	return false
}

// plistMapHasBareAmpersand reports a raw '&' in preference strings (not part of a known entity).
func plistMapHasBareAmpersand(v any) bool {
	switch t := v.(type) {
	case string:
		if strings.Contains(t, "%26") {
			return false
		}
		return hasBareAmpersandInXML(t)
	case map[string]any:
		for _, val := range t {
			if plistMapHasBareAmpersand(val) {
				return true
			}
		}
	case []any:
		for _, val := range t {
			if plistMapHasBareAmpersand(val) {
				return true
			}
		}
	}
	return false
}

// plistMapHasLiteralAmpersand kept for tests referencing the name.
func plistMapHasLiteralAmpersand(v any) bool {
	return plistMapHasMCXAmpersandIssue(v)
}

func mcxDictToForcedPlist(settings map[string]any) (string, error) {
	if settings == nil {
		return "", fmt.Errorf("MCX settings dict is nil")
	}
	var buf bytes.Buffer
	if err := plist.NewEncoder(&buf).Encode(settings); err != nil {
		return "", fmt.Errorf("encode MCX settings plist: %w", err)
	}
	fragment, err := normaliseDictFile(buf.Bytes())
	if err != nil {
		return "", err
	}
	// Wrap bare dict in plist document (matches Jamf UI forced.plist shape).
	if strings.HasPrefix(strings.TrimSpace(fragment), "<?xml") {
		return fragment, nil
	}
	return `<?xml version="1.0" encoding="UTF-8"?>` + "\n" +
		`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n" +
		`<plist version="1.0">` + "\n" + fragment + "\n</plist>", nil
}

func fetchACSGeneratedUUIDs(credName string, count int) ([]string, error) {
	if count <= 0 {
		count = 1
	}
	path := fmt.Sprintf("%s/generated-uuids?count=%d", acsConfigProfilesPath, count)
	body, err := jamfGET(credName, path)
	if err != nil {
		return nil, err
	}
	var uuids []string
	if err := json.Unmarshal(body, &uuids); err != nil {
		return nil, fmt.Errorf("parse generated-uuids: %w", err)
	}
	return uuids, nil
}

func fetchACSConfigProfileByClassicID(credName string, classicProfileID int) (*ACSConfigProfileDoc, error) {
	path := fmt.Sprintf("%s/profile/%d", acsConfigProfilesPath, classicProfileID)
	body, err := jamfGET(credName, path)
	if err != nil {
		return nil, err
	}
	return parseACSConfigProfileDoc(body)
}

func fetchACSConfigProfile(credName, payloadUUID string) (*ACSConfigProfileDoc, error) {
	payloadUUID = strings.TrimSpace(payloadUUID)
	if payloadUUID == "" {
		return nil, fmt.Errorf("payload uuid is required")
	}
	path := fmt.Sprintf("%s/%s", acsConfigProfilesPath, payloadUUID)
	body, err := jamfGET(credName, path)
	if err != nil {
		return nil, err
	}
	return parseACSConfigProfileDoc(body)
}

func parseACSConfigProfileDoc(body []byte) (*ACSConfigProfileDoc, error) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return &ACSConfigProfileDoc{}, nil
	}
	var doc ACSConfigProfileDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("parse ACS config profile JSON: %w", err)
	}
	return &doc, nil
}

func validateACSPlist(credName, plistXML string) error {
	body, err := json.Marshal(plistXML)
	if err != nil {
		return err
	}
	_, err = jamfPOST(credName, acsConfigProfilesPlistPath, body, "application/json")
	return err
}

func putACSConfigProfile(credName, payloadUUID string, doc *ACSConfigProfileDoc) error {
	payloadUUID = strings.TrimSpace(payloadUUID)
	if payloadUUID == "" {
		return fmt.Errorf("payload uuid is required")
	}
	body, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("%s/%s", acsConfigProfilesPath, payloadUUID)
	return jamfPUT(credName, path, body, "application/json")
}

func putACSConfigProfileByClassicID(credName string, classicProfileID int, doc *ACSConfigProfileDoc) error {
	if classicProfileID <= 0 {
		return fmt.Errorf("classic profile id is required")
	}
	body, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("%s/profile/%d", acsConfigProfilesPath, classicProfileID)
	return jamfPUT(credName, path, body, "application/json")
}

func validateACSDocPlists(credName string, doc *ACSConfigProfileDoc) error {
	for _, pc := range doc.PayloadContent {
		if pc.Forced != nil && pc.Forced.Plist != "" {
			if err := validateACSPlist(credName, pc.Forced.Plist); err != nil {
				return fmt.Errorf("validate plist for %q: %w", pc.PreferenceDomain, err)
			}
		}
	}
	return nil
}

func enrichACSAttachDoc(doc *ACSConfigProfileDoc, classicProfileID int, profileXML []byte) {
	if doc == nil {
		return
	}
	if classicProfileID > 0 {
		doc.ClassicProfileID = &classicProfileID
	}
	if name := extractProfileNameFromXML(profileXML); name != "" {
		doc.Name = name
	}
}

func acsDocHasDesiredMCXEntries(doc *ACSConfigProfileDoc, desired []MCXPayloadEntry) bool {
	if doc == nil || len(desired) == 0 {
		return false
	}
	want := make(map[string]bool, len(desired))
	for _, e := range desired {
		if d := strings.TrimSpace(e.Domain); d != "" {
			want[d] = true
		}
	}
	for _, pc := range doc.PayloadContent {
		d := strings.TrimSpace(pc.PreferenceDomain)
		if d == "" || pc.Forced == nil || strings.TrimSpace(pc.Forced.Plist) == "" {
			continue
		}
		delete(want, d)
	}
	return len(want) == 0
}

// profileXMLWithoutMCXAmpersandSettings returns profile XML whose MCX settings omit values containing
// literal ampersands so Classic PUT is less likely to 409. ACS API keeps full settings.
func profileXMLWithoutMCXAmpersandSettings(profileXML []byte) ([]byte, error) {
	entries, err := listMCXPayloadEntries(profileXML)
	if err != nil {
		return nil, err
	}
	payloadsRaw, err := extractPayloadsFromProfileXML(profileXML)
	if err != nil {
		return nil, err
	}
	var root map[string]any
	if err := plist.NewDecoder(strings.NewReader(payloadsRaw)).Decode(&root); err != nil {
		return nil, fmt.Errorf(errDecodePayloadsPlist, err)
	}
	for _, entry := range entries {
		if !entry.HasAmpersand {
			continue
		}
		stripAmpersandKeysFromMCXRoot(root, entry.Domain)
	}
	updatedPlist, err := encodePayloadsRoot(root)
	if err != nil {
		return nil, err
	}
	return replacePayloadsContent(profileXML, updatedPlist)
}

// profileXMLClassicACSStubFromACS builds Classic payloads XML that mirrors Jamf UI ACS layout
// (PayloadIdentifier == PayloadUUID on each ManagedClient payload) using ACS API UUIDs.
// When stripAmpersandSettings is true, MCX keys whose values contain literal '&' are omitted
// so Classic PUT is less likely to 409 (ACS API still holds the full settings).
func profileXMLClassicACSStubFromACS(credName string, classicID int, acsPayloadUUID string, desiredProfileXML []byte, stripAmpersandSettings bool) ([]byte, error) {
	acsPayloadUUID = strings.TrimSpace(acsPayloadUUID)
	if acsPayloadUUID == "" {
		if stripAmpersandSettings {
			return profileXMLWithoutMCXAmpersandSettings(desiredProfileXML)
		}
		return desiredProfileXML, nil
	}
	acsDoc, err := fetchACSConfigProfile(credName, acsPayloadUUID)
	if err != nil || len(acsDoc.PayloadContent) == 0 {
		if stripAmpersandSettings {
			return profileXMLWithoutMCXAmpersandSettings(desiredProfileXML)
		}
		return desiredProfileXML, nil
	}
	classicXML, err := profileFetchRaw(credName, classicID)
	if err != nil {
		return nil, err
	}
	generalUUID, err := extractGeneralUUID(classicXML)
	if err != nil {
		return nil, err
	}
	var acsPayloads []any
	for _, pc := range acsDoc.PayloadContent {
		domain := strings.TrimSpace(pc.PreferenceDomain)
		if domain == "" {
			continue
		}
		payloadID := strings.TrimSpace(pc.PayloadUUID)
		if payloadID == "" {
			payloadID = strings.TrimSpace(pc.PayloadIdentifier)
		}
		if payloadID == "" {
			continue
		}
		settings := map[string]any{}
		if pc.Forced != nil && strings.TrimSpace(pc.Forced.Plist) != "" {
			var decoded map[string]any
			if err := plist.NewDecoder(strings.NewReader(pc.Forced.Plist)).Decode(&decoded); err == nil {
				for k, v := range decoded {
					if stripAmpersandSettings && plistMapHasMCXAmpersandIssue(v) {
						continue
					}
					settings[k] = v
				}
			}
		}
		displayName := strings.TrimSpace(pc.PayloadDisplayName)
		if displayName == "" {
			displayName = "Custom Settings"
		}
		acsPayloads = append(acsPayloads, map[string]any{
			"PayloadType":        managedClientPreferencesType,
			"PayloadVersion":     1,
			"PayloadIdentifier":  payloadID,
			"PayloadUUID":        payloadID,
			"PayloadDisplayName": displayName,
			"PayloadContent": map[string]any{
				domain: map[string]any{
					"Forced": []any{
						map[string]any{
							"mcx_preference_settings": settings,
						},
					},
				},
			},
		})
	}
	if len(acsPayloads) == 0 {
		if stripAmpersandSettings {
			return profileXMLWithoutMCXAmpersandSettings(desiredProfileXML)
		}
		return desiredProfileXML, nil
	}
	root := map[string]any{
		"PayloadUUID":       generalUUID,
		"PayloadType":       "Configuration",
		"PayloadVersion":    1,
		"PayloadIdentifier": generalUUID,
		"PayloadContent":    acsPayloads,
	}
	plistStr, err := encodePayloadsRoot(root)
	if err != nil {
		return nil, err
	}
	return replacePayloadsContent(classicXML, plistStr)
}

func stripAmpersandKeysFromMCXRoot(root map[string]any, domain string) {
	domain = strings.TrimSpace(domain)
	if domain == "" || root == nil {
		return
	}
	stripMap := func(settings map[string]any) {
		for k, v := range settings {
			if plistMapHasMCXAmpersandIssue(v) {
				delete(settings, k)
			}
		}
	}
	if content, ok := root["PayloadContent"].([]any); ok {
		for _, item := range content {
			payload, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if pt, _ := payload["PayloadType"].(string); pt != managedClientPreferencesType {
				continue
			}
			if nested, ok := payload["PayloadContent"].(map[string]any); ok {
				if domainDict, ok := nested[domain].(map[string]any); ok {
					if settings := extractMCXSettingsFromDomainDict(domainDict); settings != nil {
						stripMap(settings)
					}
				}
			}
		}
	}
	if domainDict, ok := root[domain].(map[string]any); ok {
		if settings, ok := domainDict["mcx_preference_settings"].(map[string]any); ok {
			stripMap(settings)
		}
	}
}

func verifyACSAttach(credName string, classicProfileID int, desired []MCXPayloadEntry, logger *slog.Logger) (payloadUUID string, ok bool) {
	doc, err := fetchACSConfigProfileByClassicID(credName, classicProfileID)
	if err != nil || !acsDocHasDesiredMCXEntries(doc, desired) {
		return "", false
	}
	if u, err := resolveACSPayloadUUID(credName, classicProfileID, logger); err == nil {
		return u, true
	}
	if logger != nil {
		logger.Debug("profile acs: ACS payloadContent verified (external payload UUID not yet linked)", "profile_id", classicProfileID)
	}
	return "", true
}

func profileACSCreatePayload(credName string, classicProfileID int, desiredProfileXML []byte, logger *slog.Logger) (payloadUUID string, contentWritten bool, err error) {
	desiredEntries, err := listMCXPayloadEntries(desiredProfileXML)
	if err != nil {
		return "", false, err
	}
	doc, err := buildACSCreateDocFromProfileXML(credName, desiredProfileXML)
	if err != nil {
		return "", false, err
	}
	enrichACSAttachDoc(doc, classicProfileID, desiredProfileXML)
	if err := validateACSDocPlists(credName, doc); err != nil {
		return "", false, err
	}
	if logger != nil {
		logger.Debug("profile acs: attach ACS payload on Classic profile", "profile_id", classicProfileID, "name", doc.Name)
	}
	if err := putACSConfigProfileByClassicID(credName, classicProfileID, doc); err == nil {
		if u, ok := verifyACSAttach(credName, classicProfileID, desiredEntries, logger); ok {
			return u, true, nil
		}
		if logger != nil {
			logger.Debug("profile acs: profile-scoped PUT returned OK but ACS not visible; trying ACS POST attach", "profile_id", classicProfileID)
		}
	} else if logger != nil {
		logger.Debug("profile acs: profile-scoped PUT unavailable; using ACS POST attach", "profile_id", classicProfileID, "error", err.Error())
	}
	respBody, err := postACSConfigProfile(credName, doc)
	if err != nil {
		return "", false, err
	}
	if logger != nil {
		logger.Debug("profile acs: ACS POST attach response", "body", strings.TrimSpace(string(respBody)))
	}
	payloadUUID = parseACSPostResponse(respBody)
	if payloadUUID == "" {
		return "", false, fmt.Errorf("ACS POST attach returned no payload UUID")
	}
	cacheACSPayloadUUID(classicProfileID, payloadUUID)
	if doc, err := fetchACSConfigProfile(credName, payloadUUID); err == nil && acsDocHasDesiredMCXEntries(doc, desiredEntries) {
		if logger != nil {
			logger.Debug("profile acs: ACS POST attach verified via payload UUID", "profile_id", classicProfileID, "payload_uuid", payloadUUID)
		}
		return payloadUUID, true, nil
	}
	if u, ok := verifyACSAttach(credName, classicProfileID, desiredEntries, logger); ok {
		if u != "" {
			return u, true, nil
		}
		return payloadUUID, true, nil
	}
	return "", false, fmt.Errorf("ACS POST attach for Classic id %d did not persist ACS payloadContent (check Jamf UI ACS tab / API permissions)", classicProfileID)
}

func postACSConfigProfile(credName string, doc *ACSConfigProfileDoc) ([]byte, error) {
	body, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	return jamfPOST(credName, acsConfigProfilesPath, body, "application/json")
}

func buildACSPayloadContentFromEntry(entry MCXPayloadEntry) (ACSPayloadContent, error) {
	if strings.TrimSpace(entry.ForcedPlistRaw) != "" {
		return ACSPayloadContent{
			PayloadType:      managedClientPreferencesType,
			PreferenceDomain: entry.Domain,
			Forced:           &ACSForced{Plist: entry.ForcedPlistRaw},
			SetOnce:          &acsSetOnceNull,
		}, nil
	}
	plistXML, err := mcxDictToForcedPlist(entry.SettingsDict)
	if err != nil {
		return ACSPayloadContent{}, err
	}
	return ACSPayloadContent{
		PayloadType:      managedClientPreferencesType,
		PreferenceDomain: entry.Domain,
		Forced:           &ACSForced{Plist: plistXML},
		SetOnce:          &acsSetOnceNull,
	}, nil
}

type macOSPostResponse struct {
	UUID        string `json:"uuid"`
	PayloadUUID string `json:"payloadUUID"`
}

func parseACSPostResponse(body []byte) (payloadUUID string) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return ""
	}
	var wrap macOSPostResponse
	if err := json.Unmarshal(body, &wrap); err == nil {
		if u := strings.TrimSpace(wrap.UUID); u != "" {
			return u
		}
		if u := strings.TrimSpace(wrap.PayloadUUID); u != "" {
			return u
		}
	}
	if doc, err := parseACSConfigProfileDoc(body); err == nil {
		if doc.PayloadUUID != nil {
			return strings.TrimSpace(*doc.PayloadUUID)
		}
	}
	return ""
}

func buildACSCreateDocFromProfileXML(credName string, profileXML []byte) (*ACSConfigProfileDoc, error) {
	entries, err := listMCXPayloadEntries(profileXML)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no MCX payloads found in profile XML")
	}
	doc := &ACSConfigProfileDoc{
		PayloadUUID: nil,
		Level:       acsConfigProfileLevelSystem,
	}
	for _, e := range entries {
		pc, err := buildACSPayloadContentFromEntry(e)
		if err != nil {
			return nil, err
		}
		doc.PayloadContent = append(doc.PayloadContent, pc)
	}
	return doc, nil
}

func mergeACSDocWithDesiredEntries(current *ACSConfigProfileDoc, desired []MCXPayloadEntry) (*ACSConfigProfileDoc, error) {
	if current == nil {
		current = &ACSConfigProfileDoc{Level: acsConfigProfileLevelSystem}
	}
	out := *current
	out.PayloadContent = make([]ACSPayloadContent, len(current.PayloadContent))
	copy(out.PayloadContent, current.PayloadContent)

	for _, entry := range desired {
		newPC, err := buildACSPayloadContentFromEntry(entry)
		if err != nil {
			return nil, err
		}
		replaced := false
		for i := range out.PayloadContent {
			if out.PayloadContent[i].PreferenceDomain == entry.Domain {
				// Preserve server-assigned identifiers.
				if out.PayloadContent[i].PayloadIdentifier != "" {
					newPC.PayloadIdentifier = out.PayloadContent[i].PayloadIdentifier
				}
				if out.PayloadContent[i].PayloadUUID != "" {
					newPC.PayloadUUID = out.PayloadContent[i].PayloadUUID
				}
				if out.PayloadContent[i].SetOnce != nil {
					newPC.SetOnce = out.PayloadContent[i].SetOnce
				}
				// Keep raw Forced.plist when settings are unchanged (preserves Jamf <value> tags etc.).
				if entry.ForcedPlistRaw == "" && out.PayloadContent[i].Forced != nil {
					if curSettings, err := decodeForcedPlistSettings(out.PayloadContent[i].Forced.Plist); err == nil &&
						settingsMapsEqual(curSettings, entry.SettingsDict) {
						newPC.Forced = out.PayloadContent[i].Forced
					}
				}
				out.PayloadContent[i] = newPC
				replaced = true
				break
			}
		}
		if !replaced {
			out.PayloadContent = append(out.PayloadContent, newPC)
		}
	}
	if out.Level == "" {
		out.Level = acsConfigProfileLevelSystem
	}
	return &out, nil
}

func profileACSPut(credName string, classicProfileID int, desiredProfileXML []byte, logger *slog.Logger) (acsPayloadUUID string, err error) {
	eligible, reason := ProfileACSWritePreferred(desiredProfileXML)
	desiredEntries, err := listMCXPayloadEntries(desiredProfileXML)
	if err != nil {
		return "", err
	}
	if !eligible && len(desiredEntries) == 0 {
		return "", fmt.Errorf("ACS write not preferred: %s", reason)
	}
	if !eligible {
		reason = "Classic 409 / MCX payloads"
	}
	payloadUUID, err := resolveACSPayloadUUID(credName, classicProfileID, logger)
	if err != nil {
		if logger != nil {
			logger.Debug("profile acs: no external payload UUID yet", "profile_id", classicProfileID, "reason", err.Error())
		}
		u, written, createErr := profileACSCreatePayload(credName, classicProfileID, desiredProfileXML, logger)
		if createErr != nil {
			return "", createErr
		}
		if written {
			if logger != nil {
				logger.Debug("profile acs: attach wrote payload content; skipping redundant ACS PUT", "profile_id", classicProfileID, "payload_uuid", u)
			}
			return u, nil
		}
		payloadUUID = u
		if payloadUUID == "" {
			return "", nil
		}
	}
	if logger != nil {
		logger.Debug("profile acs: PUT payload via ACS API", "profile_id", classicProfileID, "payload_uuid", payloadUUID, "reason", reason)
	}
	var current *ACSConfigProfileDoc
	if strings.HasPrefix(payloadUUID, "profile:") {
		current, err = fetchACSConfigProfileByClassicID(credName, classicProfileID)
	} else {
		current, err = fetchACSConfigProfile(credName, payloadUUID)
	}
	if err != nil {
		return "", err
	}
	doc, err := mergeACSDocWithDesiredEntries(current, desiredEntries)
	if err != nil {
		return "", err
	}
	if doc.PayloadUUID == nil || (doc.PayloadUUID != nil && *doc.PayloadUUID == "") {
		if !strings.HasPrefix(payloadUUID, "profile:") {
			doc.PayloadUUID = &payloadUUID
		}
	}
	if err := validateACSDocPlists(credName, doc); err != nil {
		return "", err
	}
	if strings.HasPrefix(payloadUUID, "profile:") {
		if err := putACSConfigProfileByClassicID(credName, classicProfileID, doc); err != nil {
			return "", err
		}
	} else {
		if err := putACSConfigProfile(credName, payloadUUID, doc); err != nil {
			return "", err
		}
	}
	if logger != nil {
		logger.Debug("profile acs: PUT succeeded", "profile_id", classicProfileID, "payload_uuid", payloadUUID)
	}
	return payloadUUID, nil
}

type acsCreateOutcome struct {
	ClassicProfileID int
	PayloadUUID      string
}

// buildClassicShellProfileXML replaces entity-encoded payloads with an empty PayloadContent shell
// so Classic POST succeeds; MCX content is written afterward via ACS PUT.
func buildClassicShellProfileXML(preparedProfileXML []byte) ([]byte, error) {
	generalUUID, err := extractGeneralUUID(preparedProfileXML)
	if err != nil {
		return nil, err
	}
	shellInner := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>PayloadUUID</key><string>%s</string>
<key>PayloadType</key><string>Configuration</string>
<key>PayloadVersion</key><integer>1</integer>
<key>PayloadIdentifier</key><string>%s</string>
<key>PayloadContent</key><array></array>
</dict></plist>`, generalUUID, generalUUID)
	return replacePayloadsEncodedContent(preparedProfileXML, xmlEncodePayload(shellInner))
}

func profileACSPost(credName string, preparedProfileXML []byte, profileName string, logger *slog.Logger) (*acsCreateOutcome, error) {
	eligible, reason := ProfileACSWritePreferred(preparedProfileXML)
	if !eligible {
		entries, err := listMCXPayloadEntries(preparedProfileXML)
		if err != nil || len(entries) == 0 {
			return nil, fmt.Errorf("ACS write not preferred: %s", reason)
		}
		reason = "Classic 409 / MCX payloads"
	}
	if logger != nil {
		logger.Debug("profile acs: Classic shell POST + ACS payload save", "name", profileName, "reason", reason)
	}
	shell, err := buildClassicShellProfileXML(preparedProfileXML)
	if err != nil {
		return nil, err
	}
	status, body, err := profileClassicPostRaw(credName, shell)
	if err != nil {
		return nil, fmt.Errorf("Classic shell POST: %w", err)
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return nil, fmt.Errorf("Classic shell POST status %d: %s", status, string(body))
	}
	classicID := parseProfileIDFromPostBody(body)
	if classicID <= 0 {
		classicID, _, _ = ProfileFindID(credName, profileName, logger)
	}
	if classicID <= 0 {
		if generalUUID, err := extractGeneralUUID(preparedProfileXML); err == nil {
			classicID, _ = profileFindIDByGeneralUUID(credName, generalUUID, logger)
		}
	}
	if classicID <= 0 {
		return nil, fmt.Errorf("Classic shell POST succeeded but could not resolve Classic profile id for %q", profileName)
	}
	acsUUID, err := profileACSPut(credName, classicID, preparedProfileXML, logger)
	if err != nil {
		return nil, fmt.Errorf("Classic shell create ok (id %d) but ACS payload save failed: %w", classicID, err)
	}
	out := &acsCreateOutcome{ClassicProfileID: classicID, PayloadUUID: acsUUID}
	if out.PayloadUUID == "" {
		if u, err := resolveACSPayloadUUID(credName, classicID, logger); err == nil {
			out.PayloadUUID = u
		}
	}
	if logger != nil {
		logger.Debug("profile acs: create complete", "new_profile_id", out.ClassicProfileID, "payload_uuid", out.PayloadUUID)
	}
	return out, nil
}

func macOSDocClassicProfileID(doc *ACSConfigProfileDoc) int {
	if doc == nil {
		return 0
	}
	for _, id := range []*int{doc.ClassicProfileID, doc.ExternalConfigProfileID, doc.ProfileID} {
		if id != nil && *id > 0 {
			return *id
		}
	}
	return 0
}

func profileFindClassicIDByACSPayloadUUID(credName, payloadUUID string, logger *slog.Logger) (int, error) {
	payloadUUID = strings.TrimSpace(payloadUUID)
	if payloadUUID == "" {
		return 0, nil
	}
	records, err := ProfileSearch(credName, "", logger)
	if err != nil {
		return 0, err
	}
	for _, r := range records {
		doc, err := fetchACSConfigProfileByClassicID(credName, r.ID)
		if err != nil || doc == nil || doc.PayloadUUID == nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(*doc.PayloadUUID), payloadUUID) {
			return r.ID, nil
		}
	}
	return 0, nil
}

func profileFindIDByGeneralUUID(credName, generalUUID string, logger *slog.Logger) (int, error) {
	generalUUID = normalizeUUID(generalUUID)
	if generalUUID == "" {
		return 0, nil
	}
	records, err := ProfileSearch(credName, "", logger)
	if err != nil {
		return 0, err
	}
	for _, r := range records {
		xml, err := profileFetchRaw(credName, r.ID)
		if err != nil {
			continue
		}
		u, err := extractGeneralUUID(xml)
		if err != nil {
			continue
		}
		if normalizeUUID(u) == generalUUID {
			return r.ID, nil
		}
	}
	return 0, nil
}

func profileHasNonEmptyScope(profileXML []byte) bool {
	s := string(profileXML)
	if strings.Contains(s, "<all_computers>true</all_computers>") {
		return true
	}
	if strings.Contains(s, "<computer_group><id>") && !strings.Contains(s, "<computer_groups/>") {
		return true
	}
	return false
}
