package buddy

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

var profileScopeGroupIDRe = regexp.MustCompile(`<computer_group><id>(\d+)</id></computer_group>`)

// profileClassicPutRaw PUTs profile XML and returns HTTP status (409 does not error).
func profileClassicPutRaw(credName string, profileID int, xml []byte) (status int, respBody []byte, err error) {
	path := fmt.Sprintf("%s/%d", uriClassicOSXProfiles, profileID)
	resp, err := JamfRequest(credName, JamfRequestOpts{
		Method:      http.MethodPut,
		Path:        path,
		Body:        xml,
		ContentType: "application/xml",
	})
	if err != nil {
		return 0, nil, err
	}
	return resp.Status, resp.Body, nil
}

// profileClassicPostRaw POSTs profile XML to id/0 and returns HTTP status.
func profileClassicPostRaw(credName string, xml []byte) (status int, respBody []byte, err error) {
	resp, err := JamfRequest(credName, JamfRequestOpts{
		Method:      http.MethodPost,
		Path:        uriClassicOSXProfiles + "/0",
		Body:        xml,
		ContentType: "application/xml",
	})
	if err != nil {
		return 0, nil, err
	}
	return resp.Status, resp.Body, nil
}

func parseScopeFromProfileXML(profileXML []byte) (scopeAll bool, groupIDs []int) {
	s := string(profileXML)
	scopeAll = strings.Contains(s, "<all_computers>true</all_computers>")
	for _, m := range profileScopeGroupIDRe.FindAllStringSubmatch(s, -1) {
		if len(m) < 2 {
			continue
		}
		if id, err := strconv.Atoi(m[1]); err == nil {
			groupIDs = append(groupIDs, id)
		}
	}
	return scopeAll, groupIDs
}

func scopeFromPostOpts(xml []byte, postOpts profileClassicPostOpts) (scopeAll bool, groupIDs []int) {
	scopeAll, groupIDs = parseScopeFromProfileXML(xml)
	if postOpts.ScopeAll != nil {
		scopeAll = *postOpts.ScopeAll
	}
	if len(postOpts.ScopeGroupIDs) > 0 {
		groupIDs = postOpts.ScopeGroupIDs
	}
	return scopeAll, groupIDs
}

// profileClassicRedeployOnce GETs current profile, sets redeploy_on_update=All, PUTs (payloads unchanged).
func profileClassicRedeployOnce(credName string, profileID int, logger *slog.Logger) error {
	if logger != nil {
		logger.Debug("profile write: Classic redeploy PUT", "profile_id", profileID)
	}
	return profileClassicMetadataPut(credName, profileID, profileClassicMetadataOpts{Redeploy: true}, logger)
}

// profileClassicMetadataOpts controls GET-based Classic metadata sync after ACS payload writes.
type profileClassicMetadataOpts struct {
	Redeploy      bool
	Name          string // empty = leave unchanged
	ScopeAll      *bool  // nil = leave scope unchanged
	ScopeGroupIDs []int
}

// profileClassicMetadataPut GETs current profile and patches name/scope/redeploy without replacing payloads.
func profileClassicMetadataPut(credName string, profileID int, opts profileClassicMetadataOpts, logger *slog.Logger) error {
	needScope := opts.ScopeAll != nil || len(opts.ScopeGroupIDs) > 0
	if !opts.Redeploy && strings.TrimSpace(opts.Name) == "" && !needScope {
		return nil
	}
	if logger != nil {
		logger.Debug("profile write: Classic metadata PUT", "profile_id", profileID, "redeploy", opts.Redeploy, "name", opts.Name != "")
	}
	xml, err := profileFetchRaw(credName, profileID)
	if err != nil {
		return err
	}
	if name := strings.TrimSpace(opts.Name); name != "" {
		xml, err = setProfileName(xml, name)
		if err != nil {
			return err
		}
	}
	if needScope {
		scopeAll := false
		if opts.ScopeAll != nil {
			scopeAll = *opts.ScopeAll
		}
		xml, err = replaceProfileScopeBlock(xml, scopeAll, opts.ScopeGroupIDs)
		if err != nil {
			return err
		}
	}
	if opts.Redeploy {
		xml = setRedeployAll(xml)
	}
	status, body, err := profileClassicPutRaw(credName, profileID, xml)
	if err != nil {
		return err
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return fmt.Errorf("metadata PUT profile status %d: %s", status, string(body))
	}
	return nil
}

func syncClassicMetadataAfterACSCreate(credName string, classicID int, profileName string, xml []byte, postOpts profileClassicPostOpts, logger *slog.Logger) error {
	scopeAll, groupIDs := scopeFromPostOpts(xml, postOpts)
	meta := profileClassicMetadataOpts{
		Name:          profileName,
		ScopeAll:      &scopeAll,
		ScopeGroupIDs: groupIDs,
	}
	return profileClassicMetadataPut(credName, classicID, meta, logger)
}

// syncClassicPayloadsAfterACSAttach embeds the Jamf ACS payloads plist into Classic XML after a
// shell create + ACS attach. The Jamf UI reads ACS structure from Classic payloads (see production
// profiles with nested ManagedClient PayloadContent), not from an empty PayloadContent shell alone.
// When desired XML has MCX ampersand settings, Classic must retain them with Jamf &amp;/&amp;amp;
// encoding — stripping those keys is not treated as success.
func syncClassicPayloadsAfterACSAttach(credName string, classicID int, acsPayloadUUID string, desiredProfileXML []byte, logger *slog.Logger) error {
	if logger != nil {
		logger.Debug("profile acs: syncing Classic payloads after ACS attach", "profile_id", classicID)
	}
	requireAmp := profileXMLRequiresAmpersandClassicRetention(desiredProfileXML)
	tryPayloads := func(label string, profileXML []byte) error {
		data, err := prepareProfileXMLForClassicSync(profileXML)
		if err != nil {
			return err
		}
		payloadsInner, err := extractPayloadsFromProfileXML(data)
		if err != nil {
			return err
		}
		current, err := profileFetchClassicUnhydrated(credName, classicID)
		if err != nil {
			return err
		}
		if profileXMLDropsBareAmpKeys(current, data) {
			return fmt.Errorf("Classic payloads sync (%s): refusing to drop MCX keys that contain '&'", label)
		}
		updated, err := replacePayloadsContent(current, payloadsInner)
		if err != nil {
			return err
		}
		// Re-validate after merge into current Classic shell (outer encoding).
		if _, err := prepareProfileXMLForClassicSync(updated); err != nil {
			return err
		}
		status, body, err := profileClassicPutRaw(credName, classicID, updated)
		if err != nil {
			return err
		}
		if status == http.StatusOK || status == http.StatusCreated {
			if logger != nil {
				logger.Debug("profile acs: Classic payloads sync succeeded", "profile_id", classicID, "mode", label)
			}
			return nil
		}
		if status == http.StatusConflict {
			return fmt.Errorf("Classic payloads sync (%s) status 409: %s", label, string(body))
		}
		return fmt.Errorf("Classic payloads sync (%s) status %d: %s", label, status, string(body))
	}
	desired := desiredProfileXML
	if normalized, err := prepareProfileXMLForClassicSync(desiredProfileXML); err == nil {
		desired = normalized
	}
	var errs []string
	if err := tryPayloads("full", desired); err == nil {
		return nil
	} else {
		errs = append(errs, err.Error())
		if logger != nil {
			logger.Debug("profile acs: Classic full payloads sync failed; trying ACS-aligned Classic stub (all settings)", "profile_id", classicID, "error", err.Error())
		}
	}
	acsStubFull, err := profileXMLClassicACSStubFromACS(credName, classicID, acsPayloadUUID, desired, false)
	if err != nil {
		return err
	}
	if err := tryPayloads("acs-stub-full", acsStubFull); err == nil {
		return nil
	} else {
		errs = append(errs, err.Error())
		if logger != nil {
			logger.Debug("profile acs: Classic ACS stub (all settings) sync failed", "profile_id", classicID, "error", err.Error())
		}
	}
	if applyXML, err := profileXMLBareAmpToApplyForm(desired); err == nil && !bytes.Equal(applyXML, desired) {
		if err := tryPayloads("apply-form", applyXML); err == nil {
			return nil
		} else {
			errs = append(errs, err.Error())
			if logger != nil {
				logger.Debug("profile acs: Classic Apply-form payloads sync failed", "profile_id", classicID, "error", err.Error())
			}
		}
	}
	if requireAmp {
		return fmt.Errorf("Classic payloads sync could not retain MCX ampersand settings for profile %d (Jamf ACS UI needs ExampleURL with &amp; encoding, not stripped keys): %s", classicID, strings.Join(errs, "; "))
	}
	// Never PUT a Classic stub that omits '&' keys. That 200s and profile_xml then
	// drops AmpersandString / search URLs because GET /profile/{id} has no Forced.
	return fmt.Errorf("Classic payloads sync failed for profile %d after ACS attach: %s", classicID, strings.Join(errs, "; "))
}

func profileXMLMissingDesiredMCXDomains(got, desired []byte) bool {
	want, err := listMCXPayloadEntries(desired)
	if err != nil || len(want) == 0 {
		return false
	}
	have, err := listMCXPayloadEntries(got)
	if err != nil {
		return true
	}
	seen := map[string]bool{}
	for _, e := range have {
		if mcxSettingsNonEmpty(e.SettingsDict) {
			seen[e.Domain] = true
		}
	}
	for _, e := range want {
		if !seen[e.Domain] {
			return true
		}
	}
	return false
}

func seedClassicMCXAfterACSCreate(credName string, classicID int, desired []byte, logger *slog.Logger) error {
	hydrated, err := profileFetchRaw(credName, classicID)
	if err != nil {
		return err
	}
	if !profileXMLMissingDesiredMCXDomains(hydrated, desired) {
		return nil
	}
	applyXML, err := profileXMLBareAmpToApplyForm(desired)
	if err != nil {
		return err
	}
	data, err := prepareProfileXMLForClassicSync(applyXML)
	if err != nil {
		return err
	}
	payloadsInner, err := extractPayloadsFromProfileXML(data)
	if err != nil {
		return err
	}
	current, err := profileFetchClassicUnhydrated(credName, classicID)
	if err != nil {
		return err
	}
	updated, err := replacePayloadsContent(current, payloadsInner)
	if err != nil {
		return err
	}
	status, body, err := profileClassicPutRaw(credName, classicID, updated)
	if err != nil {
		return err
	}
	if status == http.StatusOK || status == http.StatusCreated {
		if logger != nil {
			logger.Debug("profile acs: Classic MCX seed after create used Apply-form '&amp;'", "profile_id", classicID)
		}
		return nil
	}
	return fmt.Errorf("Classic MCX seed status %d: %s", status, string(body))
}

func syncClassicAfterACSWrite(credName string, profileID int, payloadUUID string, xml []byte, logger *slog.Logger) error {
	if err := syncClassicPayloadsAfterACSAttach(credName, profileID, payloadUUID, xml, logger); err != nil {
		if profileXMLRequiresAmpersandClassicRetention(xml) {
			return fmt.Errorf("ACS payload save ok but Classic payloads sync failed: %w", err)
		}
		if logger != nil {
			logger.Debug("profile acs: Classic payloads sync failed after ACS write; ACS is source of truth", "profile_id", profileID, "error", err.Error())
		}
	}
	return nil
}

// profileClassicPut writes profile XML via Classic, falling back to ACS API when eligible.
func profileClassicPut(credName string, opts Opts, profileID int, xml []byte, redeploy bool, logger *slog.Logger) error {
	data := xml
	if redeploy {
		data = SetRedeployAll(data)
	}
	eligible, reason := ProfileACSWritePreferred(xml)
	if !eligible {
		// Linked ACS (document UUID or profile:{id} shell): write via ACS.
		// PUT /profile/{id} is 405 on some tenants; profileACSPut then POST-attaches.
		// Classic is left unstripped so '&' keys stay visible on profile_xml.
		if entries, err := listMCXPayloadEntries(xml); err == nil && len(entries) > 0 {
			if u, err := resolveACSPayloadUUID(credName, profileID, logger); err == nil && strings.TrimSpace(u) != "" {
				eligible = true
				if acsPayloadUUIDIsDocument(u) {
					reason = "ACS document linked for Classic id"
				} else {
					reason = "ACS shell; POST-attach (Classic 409s on '&')"
				}
			}
		}
	}
	if eligible {
		if logger != nil {
			logger.Debug("profile write: Classic skipped; ACS API path", "profile_id", profileID, "reason", reason)
			if profileHasNonEmptyScope(data) {
				logger.Debug("profile write: scoped profile ACS payload save may push to assigned devices", "profile_id", profileID)
			}
		}
		payloadUUID, err := profileACSPut(credName, profileID, xml, logger)
		if err != nil {
			return err
		}
		if err := syncClassicAfterACSWrite(credName, profileID, payloadUUID, xml, logger); err != nil {
			return err
		}
		meta := profileClassicMetadataOpts{Redeploy: redeploy}
		return profileClassicMetadataPut(credName, profileID, meta, logger)
	}
	if normalized, err := prepareProfileXMLForClassicWrite(data); err != nil {
		if errors.Is(err, errBareMCXAmpersandSetting) {
			return fmt.Errorf("payloads encoding: %w (Classic-only; bind ACS UUID with profile_acs_bind_uuid or BUDDY_ACS_PAYLOAD_UUID_%d to write via ACS)", err, profileID)
		}
		return fmt.Errorf("payloads encoding: %w", err)
	} else {
		data = normalized
	}
	if logger != nil {
		logger.Debug("profile write: Classic PUT", "profile_id", profileID, "path", fmt.Sprintf("%s/%d", uriClassicOSXProfiles, profileID))
	}
	status, body, err := profileClassicPutRaw(credName, profileID, data)
	if err != nil {
		return err
	}
	if status == http.StatusOK || status == http.StatusCreated {
		return nil
	}
	if status == http.StatusConflict {
		eligible2, reason2 := ProfileACSWritePreferred(xml)
		if !eligible2 {
			if entries, err := listMCXPayloadEntries(xml); err == nil && len(entries) > 0 {
				eligible2 = true
				reason2 = "Classic PUT 409 with MCX payloads"
			}
		}
		if !eligible2 {
			return fmt.Errorf("update profile status %d: %s", status, string(body))
		}
		if logger != nil {
			logger.Debug("profile write: Classic returned 409; trying ACS API", "profile_id", profileID, "reason", reason2)
			if redeploy {
				logger.Debug("profile write: Classic 409; redeploy not applied, deferring to final Classic PUT", "profile_id", profileID)
			}
		}
		if _, err := profileACSPut(credName, profileID, xml, logger); err != nil {
			return err
		}
		if err := syncClassicAfterACSWrite(credName, profileID, "", xml, logger); err != nil {
			return err
		}
		return profileClassicMetadataPut(credName, profileID, profileClassicMetadataOpts{Redeploy: redeploy}, logger)
	}
	return fmt.Errorf("update profile status %d: %s", status, string(body))
}

// profileClassicPostOpts carries create metadata for ACS fallback scope sync.
type profileClassicPostOpts struct {
	ScopeAll      *bool
	ScopeGroupIDs []int
}

// profileClassicPost POSTs profile XML via Classic, falling back to ACS API create on 409 when MCX/ACS payloads need it.
func profileClassicPost(credName string, opts Opts, xml []byte, profileName string, postOpts profileClassicPostOpts, logger *slog.Logger) (*ProfileCreateOutcome, error) {
	profileName = strings.TrimSpace(profileName)
	data := xml
	useACS := false
	acsReason := ""
	if prepared, err := prepareProfileXMLForClassicWrite(xml); err != nil {
		if errors.Is(err, errBareMCXAmpersandSetting) {
			useACS = true
			acsReason = "MCX URL ampersands"
		} else {
			return nil, fmt.Errorf("payloads encoding: %w", err)
		}
	} else {
		data = prepared
	}
	if !useACS {
		eligible, reason := ProfileACSWritePreferred(xml)
		if logger != nil {
			if eligible {
				logger.Debug("profile create: trying Classic POST first (Jamf UI reads ACS from Classic payloads)", "name", profileName, "reason", reason)
			} else {
				logger.Debug("profile create: Classic POST", "name", profileName)
			}
		}
		status, body, err := profileClassicPostRaw(credName, data)
		if err != nil {
			return nil, fmt.Errorf("create profile: %w", err)
		}
		if status == http.StatusOK || status == http.StatusCreated {
			id, err := resolveCreatedProfileID(credName, opts, body, profileName, logger)
			if err != nil {
				return nil, err
			}
			fmt.Printf("Created configuration profile %q (ID %d)\n", profileName, id)
			return &ProfileCreateOutcome{ProfileID: id, Name: profileName}, nil
		}
		if status != http.StatusConflict {
			return nil, fmt.Errorf("create profile status %d: %s", status, string(body))
		}
		eligible2, reason2 := ProfileACSWritePreferred(xml)
		if !eligible2 {
			if entries, err := listMCXPayloadEntries(xml); err == nil && len(entries) > 0 {
				eligible2 = true
				reason2 = "Classic POST 409 with MCX payloads"
			}
		}
		if !eligible2 {
			return nil, fmt.Errorf("create profile status %d: %s", status, string(body))
		}
		useACS = true
		acsReason = reason2
	}
	if logger != nil {
		logger.Debug("profile create: Classic shell + ACS payload attach", "name", profileName, "reason", acsReason)
	}
	created, err := profileACSPost(credName, xml, profileName, logger)
	if err != nil {
		return nil, err
	}
	if err := syncClassicMetadataAfterACSCreate(credName, created.ClassicProfileID, profileName, xml, postOpts, logger); err != nil {
		return nil, fmt.Errorf("ACS create ok (id %d) but metadata sync failed: %w", created.ClassicProfileID, err)
	}
	if err := syncClassicAfterACSWrite(credName, created.ClassicProfileID, created.PayloadUUID, xml, logger); err != nil {
		return nil, fmt.Errorf("ACS create ok (id %d): %w", created.ClassicProfileID, err)
	}
	if err := seedClassicMCXAfterACSCreate(credName, created.ClassicProfileID, xml, logger); err != nil {
		return nil, fmt.Errorf("ACS create ok (id %d) but Classic MCX seed failed: %w", created.ClassicProfileID, err)
	}
	fmt.Printf("Created configuration profile %q (ID %d)\n", profileName, created.ClassicProfileID)
	return &ProfileCreateOutcome{ProfileID: created.ClassicProfileID, Name: profileName}, nil
}
