package buddy

import (
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
		data, err := prepareProfileXMLForClassicWrite(profileXML)
		if err != nil {
			return err
		}
		payloadsInner, err := extractPayloadsFromProfileXML(data)
		if err != nil {
			return err
		}
		current, err := profileFetchRaw(credName, classicID)
		if err != nil {
			return err
		}
		updated, err := replacePayloadsContent(current, payloadsInner)
		if err != nil {
			return err
		}
		// Re-validate after merge into current Classic shell (outer encoding).
		if _, err := prepareProfileXMLForClassicWrite(updated); err != nil {
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
	if normalized, err := prepareProfileXMLForClassicWrite(desiredProfileXML); err == nil {
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
	if requireAmp {
		return fmt.Errorf("Classic payloads sync could not retain MCX ampersand settings for profile %d (Jamf ACS UI needs ExampleURL with &amp; encoding, not stripped keys): %s", classicID, strings.Join(errs, "; "))
	}
	// Non-amp MCX: last-resort stripped stub is a no-op for amp keys; keep for empty-shell recovery.
	acsStub, err := profileXMLClassicACSStubFromACS(credName, classicID, acsPayloadUUID, desired, true)
	if err != nil {
		return err
	}
	if err := tryPayloads("acs-stub", acsStub); err == nil {
		return nil
	} else if logger != nil {
		logger.Debug("profile acs: Classic ACS-aligned stub sync failed; trying fixture stub", "profile_id", classicID, "error", err.Error())
	}
	stub, err := profileXMLWithoutMCXAmpersandSettings(desired)
	if err != nil {
		return err
	}
	if err := tryPayloads("stub", stub); err == nil {
		return nil
	}
	return fmt.Errorf("Classic payloads sync failed for profile %d after ACS attach: %s", classicID, strings.Join(errs, "; "))
}

// profileClassicPut writes profile XML via Classic, falling back to ACS API when eligible.
func profileClassicPut(credName string, opts Opts, profileID int, xml []byte, redeploy bool, logger *slog.Logger) error {
	if err := rejectBareMCXAmpersandSettings(xml); err != nil {
		return err
	}
	data := xml
	if redeploy {
		data = SetRedeployAll(data)
	}
	eligible, reason := ProfileACSWritePreferred(xml)
	if !eligible {
		// Refactored ACS (UI edits ACS Forced.plist; Classic MCX often empty until hydrate):
		// when an ACS document is already linked, prefer ACS PUT so roundtrips don't wipe ACS.
		if entries, err := listMCXPayloadEntries(xml); err == nil && len(entries) > 0 {
			if u, err := resolveACSPayloadUUID(credName, profileID, logger); err == nil && strings.TrimSpace(u) != "" {
				eligible = true
				reason = "ACS document linked for Classic id"
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
		if err := syncClassicPayloadsAfterACSAttach(credName, profileID, payloadUUID, xml, logger); err != nil {
			return fmt.Errorf("ACS payload save ok but Classic payloads sync failed: %w", err)
		}
		meta := profileClassicMetadataOpts{Redeploy: redeploy}
		return profileClassicMetadataPut(credName, profileID, meta, logger)
	}
	if normalized, err := prepareProfileXMLForClassicWrite(data); err != nil {
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
		if err := syncClassicPayloadsAfterACSAttach(credName, profileID, "", xml, logger); err != nil {
			return fmt.Errorf("ACS payload save ok but Classic payloads sync failed: %w", err)
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
	if err := rejectBareMCXAmpersandSettings(xml); err != nil {
		return nil, err
	}
	profileName = strings.TrimSpace(profileName)
	data := xml
	if prepared, err := prepareProfileXMLForClassicWrite(xml); err != nil {
		return nil, fmt.Errorf("payloads encoding: %w", err)
	} else {
		data = prepared
	}
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
	if status == http.StatusConflict {
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
		if logger != nil {
			logger.Debug("profile create: Classic POST 409; using Classic shell + ACS payload attach", "name", profileName, "reason", reason2)
		}
		created, err := profileACSPost(credName, xml, profileName, logger)
		if err != nil {
			return nil, err
		}
		if err := syncClassicMetadataAfterACSCreate(credName, created.ClassicProfileID, profileName, xml, postOpts, logger); err != nil {
			return nil, fmt.Errorf("ACS create ok (id %d) but metadata sync failed: %w", created.ClassicProfileID, err)
		}
		if err := syncClassicPayloadsAfterACSAttach(credName, created.ClassicProfileID, created.PayloadUUID, xml, logger); err != nil {
			return nil, fmt.Errorf("ACS create ok (id %d) but Classic payloads sync failed: %w", created.ClassicProfileID, err)
		}
		fmt.Printf("Created configuration profile %q (ID %d)\n", profileName, created.ClassicProfileID)
		return &ProfileCreateOutcome{ProfileID: created.ClassicProfileID, Name: profileName}, nil
	}
	return nil, fmt.Errorf("create profile status %d: %s", status, string(body))
}
