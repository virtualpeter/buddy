package buddy

import (
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"howett.net/plist"
)

// classicToACSPayloadUUID caches classic profile id → ACS API payload UUID
// for the lifetime of the process (populated on create/resolve success).
var classicToACSPayloadUUID sync.Map // int → string

func cacheACSPayloadUUID(classicProfileID int, payloadUUID string) {
	payloadUUID = strings.TrimSpace(payloadUUID)
	if classicProfileID <= 0 || payloadUUID == "" {
		return
	}
	classicToACSPayloadUUID.Store(classicProfileID, payloadUUID)
}

// CacheACSPayloadUUID records classic profile id → ACS API payload UUID
// for this process (API tokens often cannot scrape FIELD_EXTERNAL from legacy HTML).
func CacheACSPayloadUUID(classicProfileID int, payloadUUID string) {
	cacheACSPayloadUUID(classicProfileID, payloadUUID)
}

func cachedACSPayloadUUID(classicProfileID int) string {
	if classicProfileID <= 0 {
		return ""
	}
	if v, ok := classicToACSPayloadUUID.Load(classicProfileID); ok {
		if s, _ := v.(string); strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

var (
	fieldExternalConfigProfileIDRe = regexp.MustCompile(`(?i)FIELD_EXTERNAL_CONFIGPROFILE_ID["'\s=:]+([a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12})`)
	acsConfigProfilesUUIDPathRe  = regexp.MustCompile(`(?i)/api/config-profiles/macos/([a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12})`)
	inputExternalConfigProfileIDRe = regexp.MustCompile(`(?i)name=["']FIELD_EXTERNAL_CONFIGPROFILE_ID["'][^>]*value=["']([a-fA-F0-9\-]{36})["']`)
)

// parseACSPayloadUUIDFromLegacyHTML extracts FIELD_EXTERNAL_CONFIGPROFILE_ID (or ACS API path UUID).
func parseACSPayloadUUIDFromLegacyHTML(html string) string {
	if html == "" {
		return ""
	}
	if m := inputExternalConfigProfileIDRe.FindStringSubmatch(html); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	if m := fieldExternalConfigProfileIDRe.FindStringSubmatch(html); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	if m := acsConfigProfilesUUIDPathRe.FindStringSubmatch(html); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func fetchLegacyOSXConfigurationProfileHTML(credName string, classicProfileID int) (string, error) {
	// Prefer edit view (o=u); some tenants only embed FIELD_EXTERNAL there. Fall back to read (o=r).
	var lastHTML string
	var lastErr error
	for _, mode := range []string{"u", "r"} {
		path := "/legacy/OSXConfigurationProfiles.html?id=" + strconv.Itoa(classicProfileID) + "&o=" + mode
		body, err := jamfGETWithAccept(credName, path, "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		if err != nil {
			lastErr = err
			continue
		}
		html := string(body)
		lastHTML = html
		if parseACSPayloadUUIDFromLegacyHTML(html) != "" {
			return html, nil
		}
	}
	if lastHTML != "" {
		return lastHTML, nil
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("legacy OSXConfigurationProfiles HTML empty for id %d", classicProfileID)
}

// resolveACSPayloadUUID finds the ACS API document UUID for a Classic profile id.
func resolveACSPayloadUUID(credName string, classicProfileID int, logger *slog.Logger) (string, error) {
	if u := cachedACSPayloadUUID(classicProfileID); u != "" {
		if logger != nil {
			logger.Debug("profile acs: resolved payload UUID from process cache", "profile_id", classicProfileID, "payload_uuid", u)
		}
		return u, nil
	}
	if u := strings.TrimSpace(os.Getenv(fmt.Sprintf("BUDDY_ACS_PAYLOAD_UUID_%d", classicProfileID))); u != "" {
		cacheACSPayloadUUID(classicProfileID, u)
		if logger != nil {
			logger.Debug("profile acs: resolved payload UUID from BUDDY_ACS_PAYLOAD_UUID_* env", "profile_id", classicProfileID, "payload_uuid", u)
		}
		return u, nil
	}
	// Legacy env name (pre-ACS rename); remove after one release cycle.
	if u := strings.TrimSpace(os.Getenv(fmt.Sprintf("BUDDY_MACOS_PAYLOAD_UUID_%d", classicProfileID))); u != "" {
		cacheACSPayloadUUID(classicProfileID, u)
		if logger != nil {
			logger.Debug("profile acs: resolved payload UUID from legacy BUDDY_MACOS_PAYLOAD_UUID_* env (prefer BUDDY_ACS_PAYLOAD_UUID_*)", "profile_id", classicProfileID, "payload_uuid", u)
		}
		return u, nil
	}
	doc, err := fetchACSConfigProfileByClassicID(credName, classicProfileID)
	if err == nil && doc != nil {
		if doc.PayloadUUID != nil && strings.TrimSpace(*doc.PayloadUUID) != "" {
			uuid := strings.TrimSpace(*doc.PayloadUUID)
			cacheACSPayloadUUID(classicProfileID, uuid)
			if logger != nil {
				logger.Debug("profile acs: resolved external payload UUID from profile/{id}", "profile_id", classicProfileID, "payload_uuid", uuid)
			}
			return uuid, nil
		}
		for _, pc := range doc.PayloadContent {
			if u := strings.TrimSpace(pc.PayloadUUID); u != "" && pc.Forced != nil && strings.TrimSpace(pc.Forced.Plist) != "" {
				// Some tenants return ACS content on profile/{id} with inner payload UUIDs only;
				// prefer document-level UUID when present. Inner UUIDs are ManagedClient ids, not
				// always valid as GET …/macos/{uuid} keys — only use when Forced is present and
				// later fetch succeeds (caller handles 404).
				if logger != nil {
					logger.Debug("profile acs: profile/{id} has Forced payloadContent", "profile_id", classicProfileID, "payload_uuid", u)
				}
			}
		}
		// If profile/{id} already carries Forced plists, no document UUID is required for hydrate.
		if acsDocHasAnyForcedPlist(doc) {
			cacheACSPayloadUUID(classicProfileID, "profile:"+strconv.Itoa(classicProfileID))
			return "profile:" + strconv.Itoa(classicProfileID), nil
		}
	}
	html, htmlErr := fetchLegacyOSXConfigurationProfileHTML(credName, classicProfileID)
	if htmlErr != nil {
		if logger != nil {
			logger.Debug("profile acs: legacy HTML fetch failed", "profile_id", classicProfileID, "error", htmlErr.Error())
		}
	} else if u := parseACSPayloadUUIDFromLegacyHTML(html); u != "" {
		cacheACSPayloadUUID(classicProfileID, u)
		if logger != nil {
			logger.Debug("profile acs: resolved payload UUID from legacy HTML", "profile_id", classicProfileID, "payload_uuid", u)
		}
		return u, nil
	}
	return "", fmt.Errorf("ACS config profile for Classic id %d has no external payload UUID (open the profile ACS tab in Jamf UI once, or use a profile without refactored ACS payloads)", classicProfileID)
}

func acsDocHasAnyForcedPlist(doc *ACSConfigProfileDoc) bool {
	if doc == nil {
		return false
	}
	for _, pc := range doc.PayloadContent {
		if pc.Forced != nil && strings.TrimSpace(pc.Forced.Plist) != "" && strings.TrimSpace(pc.PreferenceDomain) != "" {
			return true
		}
	}
	return false
}

func fetchACSConfigProfileForClassic(credName string, classicProfileID int, logger *slog.Logger) (*ACSConfigProfileDoc, string, error) {
	uuid, err := resolveACSPayloadUUID(credName, classicProfileID, logger)
	if err != nil {
		return nil, "", err
	}
	if strings.HasPrefix(uuid, "profile:") {
		doc, err := fetchACSConfigProfileByClassicID(credName, classicProfileID)
		return doc, uuid, err
	}
	doc, err := fetchACSConfigProfile(credName, uuid)
	return doc, uuid, err
}

// normalizeJamfForcedPlistForDecode rewrites Jamf UI <value> tags to <string> so howett can parse.
func normalizeJamfForcedPlistForDecode(plistXML string) string {
	s := plistXML
	s = strings.ReplaceAll(s, "<value>", "<string>")
	s = strings.ReplaceAll(s, "</value>", "</string>")
	s = strings.ReplaceAll(s, "<VALUE>", "<string>")
	s = strings.ReplaceAll(s, "</VALUE>", "</string>")
	return s
}

func decodeForcedPlistSettings(forcedPlist string) (map[string]any, error) {
	dictXML, err := normaliseDictFile([]byte(strings.TrimSpace(forcedPlist)))
	if err != nil {
		// Try after value→string normalization.
		dictXML, err = normaliseDictFile([]byte(normalizeJamfForcedPlistForDecode(strings.TrimSpace(forcedPlist))))
		if err != nil {
			return nil, err
		}
	}
	normalized := normalizeJamfForcedPlistForDecode(dictXML)
	var root any
	if err := plist.NewDecoder(strings.NewReader(normalized)).Decode(&root); err != nil {
		return nil, fmt.Errorf("decode forced.plist: %w", err)
	}
	m, ok := root.(map[string]any)
	if !ok {
		// howett may return map[string]interface{}
		if m2, ok2 := root.(map[string]interface{}); ok2 {
			out := make(map[string]any, len(m2))
			for k, v := range m2 {
				out[k] = v
			}
			return out, nil
		}
		return nil, fmt.Errorf("forced.plist root is not a dict")
	}
	return m, nil
}

func mcxSettingsNonEmpty(settings map[string]any) bool {
	return settings != nil && len(settings) > 0
}

func classicMCXSettingsForDomain(payloadsRaw, domain string) (map[string]any, error) {
	entries, err := listMCXPayloadEntriesFromPayloadsPlist(payloadsRaw)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.Domain == domain {
			return e.SettingsDict, nil
		}
	}
	return nil, nil
}

// hydrateClassicProfileXMLFromACSDoc merges ACS Forced plists into empty Classic MCX domains.
// Non-empty Classic domains are left unchanged. forced.plist is normalized for Classic embedding
// (<value>→<string>) so payloads remain howett-decodable; raw ACS Forced is preserved on write
// via mergeACSDocWithDesiredEntries when settings are unchanged.
func hydrateClassicProfileXMLFromACSDoc(classicXML []byte, doc *ACSConfigProfileDoc) ([]byte, error) {
	if doc == nil || !acsDocHasAnyForcedPlist(doc) {
		return classicXML, nil
	}
	payloadsRaw, err := extractPayloadsFromProfileXML(classicXML)
	if err != nil {
		return nil, err
	}
	changed := false
	for _, pc := range doc.PayloadContent {
		domain := strings.TrimSpace(pc.PreferenceDomain)
		if domain == "" || pc.Forced == nil || strings.TrimSpace(pc.Forced.Plist) == "" {
			continue
		}
		existing, err := classicMCXSettingsForDomain(payloadsRaw, domain)
		if err != nil {
			return nil, err
		}
		if mcxSettingsNonEmpty(existing) {
			continue
		}
		settings, err := decodeForcedPlistSettings(pc.Forced.Plist)
		if err != nil {
			return nil, fmt.Errorf("ACS forced.plist for domain %q: %w", domain, err)
		}
		if !mcxSettingsNonEmpty(settings) {
			continue
		}
		updated, ok, err := replaceMCXSettingsInPayloadsPlist(payloadsRaw, domain, settings)
		if err != nil {
			return nil, err
		}
		if !ok {
			payloadsRaw, ok, err = ensureMCXDomainAndSetSettings(payloadsRaw, domain, settings)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
		} else {
			payloadsRaw = updated
		}
		changed = true
	}
	if !changed {
		return classicXML, nil
	}
	return replacePayloadsContent(classicXML, payloadsRaw)
}

func ensureMCXDomainAndSetSettings(payloadsRaw, domain string, settings map[string]any) (string, bool, error) {
	var root map[string]any
	if err := plist.NewDecoder(strings.NewReader(payloadsRaw)).Decode(&root); err != nil {
		return "", false, fmt.Errorf("decode payloads plist: %w", err)
	}
	content, _ := root["PayloadContent"].([]any)
	var managed map[string]any
	for _, item := range content {
		payload, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if pt, _ := payload["PayloadType"].(string); pt == managedClientPreferencesType {
			managed = payload
			break
		}
	}
	if managed == nil {
		return "", false, nil
	}
	nested, _ := managed["PayloadContent"].(map[string]any)
	if nested == nil {
		nested = map[string]any{}
		managed["PayloadContent"] = nested
	}
	domainDict, _ := nested[domain].(map[string]any)
	if domainDict == nil {
		domainDict = map[string]any{
			"Forced": []any{
				map[string]any{"mcx_preference_settings": settings},
			},
		}
		nested[domain] = domainDict
	} else if !setMCXSettingsOnDomainDict(domainDict, settings) {
		return "", false, nil
	}
	updated, err := encodePayloadsRootAny(root)
	if err != nil {
		return "", false, err
	}
	return updated, true, nil
}

// hydrateProfileXMLFromACS fetches ACS API doc for classicProfileID and merges into Classic XML.
// On resolve failure, returns classicXML unchanged (Classic-only read).
func hydrateProfileXMLFromACS(credName string, classicProfileID int, classicXML []byte, logger *slog.Logger) ([]byte, error) {
	doc, _, err := fetchACSConfigProfileForClassic(credName, classicProfileID, logger)
	if err != nil {
		if logger != nil {
			logger.Debug("profile read: ACS hydrate skipped", "profile_id", classicProfileID, "reason", err.Error())
		}
		return classicXML, nil
	}
	out, err := hydrateClassicProfileXMLFromACSDoc(classicXML, doc)
	if err != nil {
		return nil, err
	}
	if logger != nil && string(out) != string(classicXML) {
		logger.Debug("profile read: hydrated empty Classic MCX from ACS API", "profile_id", classicProfileID)
	}
	return out, nil
}

func settingsMapsEqual(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	// Encode both for a stable comparison (avoids type noise from plist decode).
	as, err1 := mcxSettingsMapToDictFragment(a)
	bs, err2 := mcxSettingsMapToDictFragment(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return as == bs
}
