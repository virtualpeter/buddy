package buddy

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// PPPCFDA builds a SystemPolicyAllFiles (Full Disk Access) service entry for a PPPC payload.
// codeRequirement is the designated requirement string (often from codesign -d -r-).
func PPPCFDA(bundleID, codeRequirement, comment string) map[string]interface{} {
	entry := map[string]interface{}{
		"Identifier":       bundleID,
		"IdentifierType":   "bundleID",
		"CodeRequirement":  codeRequirement,
		"Allowed":          true,
		"StaticCode":       false,
	}
	if strings.TrimSpace(comment) != "" {
		entry["Comment"] = comment
	}
	return entry
}

// PayloadMetaOpts controls optional PayloadIdentifier / UUID / display name on payload builders.
type PayloadMetaOpts struct {
	PayloadIdentifier  string
	PayloadUUID        string
	PayloadDisplayName string
}

// PPPCPayload builds a full PPPC payload dict (PayloadType com.apple.TCC.configuration-profile-policy).
// services maps service names (e.g. "SystemPolicyAllFiles", "Accessibility") to lists of app entries
// (see PPPCFDA and similar).
func PPPCPayload(services map[string]interface{}, opts PayloadMetaOpts) (map[string]interface{}, error) {
	if len(services) == 0 {
		return nil, fmt.Errorf("pppc_payload requires a non-empty services map")
	}
	id := strings.TrimSpace(opts.PayloadIdentifier)
	if id == "" {
		id = "com.apple.TCC.configuration-profile-policy." + uuid.New().String()
	}
	puuid := strings.TrimSpace(opts.PayloadUUID)
	if puuid == "" {
		puuid = uuid.New().String()
	}
	out := map[string]interface{}{
		"PayloadType":       "com.apple.TCC.configuration-profile-policy",
		"PayloadIdentifier": id,
		"PayloadUUID":       puuid,
		"PayloadVersion":    1,
		"PayloadEnabled":    true,
		"Services":          services,
	}
	if strings.TrimSpace(opts.PayloadDisplayName) != "" {
		out["PayloadDisplayName"] = opts.PayloadDisplayName
	}
	return out, nil
}

// PrefsPayload builds a simple preference-domain payload dict (PayloadType == domain).
// settings are merged into the payload alongside the standard Payload* keys.
func PrefsPayload(domain string, settings map[string]interface{}, opts PayloadMetaOpts) (map[string]interface{}, error) {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return nil, fmt.Errorf("prefs_payload requires domain")
	}
	id := strings.TrimSpace(opts.PayloadIdentifier)
	if id == "" {
		id = domain
	}
	puuid := strings.TrimSpace(opts.PayloadUUID)
	if puuid == "" {
		puuid = uuid.New().String()
	}
	out := map[string]interface{}{
		"PayloadType":       domain,
		"PayloadIdentifier": id,
		"PayloadUUID":       puuid,
		"PayloadVersion":    1,
		"PayloadEnabled":    true,
	}
	if strings.TrimSpace(opts.PayloadDisplayName) != "" {
		out["PayloadDisplayName"] = opts.PayloadDisplayName
	}
	for k, v := range settings {
		if _, reserved := out[k]; reserved {
			continue
		}
		out[k] = v
	}
	return out, nil
}
