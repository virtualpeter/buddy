package buddy

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/google/uuid"
	"howett.net/plist"
)

// ProfilePrepareSpec controls offline transform before Classic POST create/clone/copy.
type ProfilePrepareSpec struct {
	Name          string
	ScopeAll      *bool // nil + empty ScopeGroupIDs => unassigned (not all computers)
	ScopeGroupIDs []int
}

// ProfileCopySpec is input for cross-session profile copy.
type ProfileCopySpec struct {
	SourceProfileID int
	Name            string
	ScopeAll        *bool
	ScopeGroupIDs   []int
}

// NewUUID returns a random RFC 4122 UUID string (offline helper).
func NewUUID() string {
	return uuid.New().String()
}

var generalUUIDRe = regexp.MustCompile(`(?i)<uuid>\s*([a-fA-F0-9\-]{36})\s*</uuid>`)

var payloadUUIDInTextRe = regexp.MustCompile(`[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}`)

// PayloadUUID keys in entity-encoded payloads plist (Jamf embeds plist XML escaped inside <payloads>).
var encodedPayloadUUIDKeyValueRe = regexp.MustCompile(
	`(&lt;key&gt;PayloadUUID&lt;/key&gt;&lt;string&gt;)([a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12})(&lt;/string&gt;)`)

func normalizeUUID(u string) string {
	return strings.ToLower(strings.TrimSpace(u))
}


func extractGeneralUUID(profileXML []byte) (string, error) {
	m := generalUUIDRe.FindSubmatch(profileXML)
	if m == nil {
		return "", fmt.Errorf("profile XML has no <uuid> in general")
	}
	return string(m[1]), nil
}

func setGeneralUUID(profileXML []byte, newUUID string) ([]byte, error) {
	loc := generalUUIDRe.FindIndex(profileXML)
	if loc == nil {
		return nil, fmt.Errorf("profile XML has no <uuid> in general")
	}
	replacement := []byte("<uuid>" + newUUID + "</uuid>")
	out := append(append([]byte{}, profileXML[:loc[0]]...), replacement...)
	out = append(out, profileXML[loc[1]:]...)
	return out, nil
}

// plistRemapPayloadUUIDs replaces PayloadUUID values in unescaped payloads plist XML.
// UUID strings are substituted in-place so Jamf entity encoding and payload formatting are preserved
// (decode/re-encode would normalize &amp; and other escapes and can break Classic POST).
func plistRemapPayloadUUIDs(payloadsRaw, oldGeneral, newGeneral string) (string, error) {
	uuids, err := PlistCollectPayloadUUIDs(payloadsRaw)
	if err != nil {
		return "", err
	}
	mapping := map[string]string{oldGeneral: newGeneral}
	for _, u := range uuids {
		if _, ok := mapping[u]; !ok {
			mapping[u] = uuid.New().String()
		}
	}
	oldIDs := make([]string, 0, len(mapping))
	for old := range mapping {
		oldIDs = append(oldIDs, old)
	}
	sort.Strings(oldIDs)
	out := payloadsRaw
	for _, old := range oldIDs {
		out = strings.ReplaceAll(out, old, mapping[old])
	}
	return out, nil
}

func collectPayloadUUIDsFromText(s string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, u := range payloadUUIDInTextRe.FindAllString(s, -1) {
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		out = append(out, u)
	}
	return out
}

func collectPayloadUUIDsFromEncodedKeys(encoded string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, m := range encodedPayloadUUIDKeyValueRe.FindAllStringSubmatch(encoded, -1) {
		if len(m) < 3 {
			continue
		}
		u := m[2]
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		out = append(out, u)
	}
	return out
}

func buildPayloadUUIDMapping(encodedPayloads, oldGeneral, newGeneral string) map[string]string {
	normToNew := map[string]string{normalizeUUID(oldGeneral): newGeneral}
	for _, u := range collectPayloadUUIDsFromEncodedKeys(encodedPayloads) {
		norm := normalizeUUID(u)
		if _, ok := normToNew[norm]; !ok {
			normToNew[norm] = uuid.New().String()
		}
	}
	mapping := map[string]string{oldGeneral: normToNew[normalizeUUID(oldGeneral)]}
	for _, u := range collectPayloadUUIDsFromEncodedKeys(encodedPayloads) {
		mapping[u] = normToNew[normalizeUUID(u)]
	}
	return mapping
}

// RandomizeProfileUUIDs mints a new general profile UUID and remaps every PayloadUUID
// (and matching PayloadIdentifier UUID strings) in the profile XML via exact substring replace.
func RandomizeProfileUUIDs(profileXML []byte) ([]byte, error) {
	oldGeneral, err := extractGeneralUUID(profileXML)
	if err != nil {
		return nil, err
	}
	newGeneral := uuid.New().String()

	encodedPayloads, err := extractPayloadsEncodedFromProfileXML(profileXML)
	if err != nil {
		return nil, err
	}
	mapping := buildPayloadUUIDMapping(encodedPayloads, oldGeneral, newGeneral)

	out := profileXML
	oldIDs := make([]string, 0, len(mapping))
	for old := range mapping {
		oldIDs = append(oldIDs, old)
	}
	sort.Strings(oldIDs)
	for _, old := range oldIDs {
		out = bytes.ReplaceAll(out, []byte(old), []byte(mapping[old]))
	}
	return setGeneralUUID(out, newGeneral)
}

func stripSelfServiceBlock(profileXML []byte) []byte {
	const open = "<self_service>"
	const close = "</self_service>"
	s := string(profileXML)
	start := strings.Index(s, open)
	if start < 0 {
		return profileXML
	}
	end := strings.Index(s[start:], close)
	if end < 0 {
		return profileXML
	}
	end = start + end + len(close)
	return []byte(s[:start] + s[end:])
}

func stripFirstProfileID(profileXML []byte) []byte {
	idRe := regexp.MustCompile(`<id>\d+</id>\s*`)
	if idx := idRe.FindIndex(profileXML); idx != nil {
		return append(profileXML[:idx[0]], profileXML[idx[1]:]...)
	}
	return profileXML
}

func extractProfileNameFromXML(profileXML []byte) string {
	nameStart := bytes.Index(profileXML, []byte("<name>"))
	if nameStart < 0 {
		return ""
	}
	nameStart += len("<name>")
	nameEnd := bytes.Index(profileXML[nameStart:], []byte("</name>"))
	if nameEnd < 0 {
		return ""
	}
	return strings.TrimSpace(XMLUnescape(string(profileXML[nameStart : nameStart+nameEnd])))
}

func setProfileName(profileXML []byte, name string) ([]byte, error) {
	nameStart := bytes.Index(profileXML, []byte("<name>"))
	if nameStart < 0 {
		return nil, fmt.Errorf("profile XML has no <name> in general")
	}
	nameEnd := bytes.Index(profileXML[nameStart:], []byte("</name>"))
	if nameEnd < 0 {
		return nil, fmt.Errorf("profile XML has malformed <name>")
	}
	nameEnd += nameStart + len("</name>")
	return bytes.Join([][]byte{
		profileXML[:nameStart],
		[]byte("<name>" + xmlEncodePayload(name) + "</name>"),
		profileXML[nameEnd:],
	}, nil), nil
}

func replaceProfileScopeBlock(profileXML []byte, scopeAll bool, groupIDs []int) ([]byte, error) {
	scopeStart := bytes.Index(profileXML, []byte("<scope>"))
	if scopeStart < 0 {
		return nil, fmt.Errorf("profile XML has no <scope>")
	}
	relEnd := bytes.Index(profileXML[scopeStart:], []byte("</scope>"))
	if relEnd < 0 {
		return nil, fmt.Errorf("profile XML has malformed <scope>")
	}
	scopeEnd := scopeStart + relEnd + len("</scope>")
	var scopeXML string
	if !scopeAll && len(groupIDs) == 0 {
		scopeXML = emptyScopeXML
	} else {
		scopeXML = buildProfileScopeXML(scopeAll, groupIDs)
	}
	return bytes.Join([][]byte{profileXML[:scopeStart], []byte(scopeXML), profileXML[scopeEnd:]}, nil), nil
}

// PrepareProfileXMLForCreate transforms existing profile XML for Classic POST (id/0):
// randomize UUIDs, set name, strip profile id, scope (default unassigned).
// UUID remap runs first (Classic clone order) so entity-encoded payloads stay byte-stable.
func PrepareProfileXMLForCreate(profileXML []byte, spec ProfilePrepareSpec) ([]byte, error) {
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		return nil, fmt.Errorf("profile name is required")
	}
	xml, err := RandomizeProfileUUIDs(profileXML)
	if err != nil {
		return nil, err
	}
	xml, err = setProfileName(xml, name)
	if err != nil {
		return nil, err
	}
	xml = stripFirstProfileID(xml)
	xml = stripSelfServiceBlock(xml)

	scopeAll := false
	if spec.ScopeAll != nil {
		scopeAll = *spec.ScopeAll
	}
	xml, err = replaceProfileScopeBlock(xml, scopeAll, spec.ScopeGroupIDs)
	if err != nil {
		return nil, err
	}
	return xml, nil
}

// PlistCollectPayloadUUIDs returns every PayloadUUID string in a payloads plist (depth-first).
func PlistCollectPayloadUUIDs(payloadsRaw string) ([]string, error) {
	var root interface{}
	dec := plist.NewDecoder(strings.NewReader(payloadsRaw))
	if err := dec.Decode(&root); err != nil {
		return nil, fmt.Errorf("decode payloads plist: %w", err)
	}
	var out []string
	collectPayloadUUIDs(root, &out)
	return out, nil
}

func collectPayloadUUIDs(v interface{}, out *[]string) {
	switch t := v.(type) {
	case map[string]interface{}:
		if s, ok := t["PayloadUUID"].(string); ok && strings.TrimSpace(s) != "" {
			*out = append(*out, s)
		}
		for _, val := range t {
			collectPayloadUUIDs(val, out)
		}
	case []interface{}:
		for _, item := range t {
			collectPayloadUUIDs(item, out)
		}
	}
}

// ProfilePostXML POSTs prepared profile XML to Classic id/0 and resolves the new profile ID.
func ProfilePostXML(credName string, opts Opts, xml []byte, profileName string, logger *slog.Logger) (*ProfileCreateOutcome, error) {
	profileName = strings.TrimSpace(profileName)
	if profileName == "" {
		return nil, fmt.Errorf("profile name is required")
	}
	if !opts.SkipConfirm {
		msg := fmt.Sprintf("\nPOST configuration profile %q?", profileName)
		if !ConfirmPrompt(msg, logger) {
			return nil, nil
		}
	}
	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "\n--- POST payload (JSSResource/osxconfigurationprofiles/id/0) ---\n%s\n--- end payload ---\n\n", xml)
	}
	scopeAll, groupIDs := parseScopeFromProfileXML(xml)
	postOpts := profileClassicPostOpts{ScopeGroupIDs: groupIDs}
	postOpts.ScopeAll = &scopeAll
	return profileClassicPost(credName, opts, xml, profileName, postOpts, logger)
}

// ProfileDelete deletes a macOS configuration profile by Classic API ID.
func ProfileDelete(credName string, opts Opts, profileID int, logger *slog.Logger) error {
	if profileID <= 0 {
		return fmt.Errorf("profile_id must be positive")
	}
	name := fmt.Sprintf("ID %d", profileID)
	if recs, err := ProfileSearch(credName, "", logger); err == nil {
		for _, r := range recs {
			if r.ID == profileID {
				name = r.Name
				break
			}
		}
	}
	if !opts.SkipConfirm {
		msg := fmt.Sprintf("\nDelete configuration profile %q (ID %d)?", name, profileID)
		if !ConfirmPrompt(msg, logger) {
			return nil
		}
	}
	if err := jamfDELETE(credName, fmt.Sprintf("/JSSResource/osxconfigurationprofiles/id/%d", profileID)); err != nil {
		return fmt.Errorf("delete profile %d: %w", profileID, err)
	}
	fmt.Printf("Deleted configuration profile %q (ID %d)\n", name, profileID)
	return nil
}

// ProfileCopy fetches a profile from srcCred, prepares it for create, and POSTs on dstCred.
func ProfileCopy(srcCred, dstCred string, opts Opts, spec ProfileCopySpec, logger *slog.Logger) (*ProfileCreateOutcome, error) {
	if spec.SourceProfileID <= 0 {
		return nil, fmt.Errorf("profile_id must be positive")
	}
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		return nil, fmt.Errorf("profile name is required")
	}
	profileXML, err := profileFetchRaw(srcCred, spec.SourceProfileID)
	if err != nil {
		return nil, err
	}
	prepared, err := PrepareProfileXMLForCreate(profileXML, ProfilePrepareSpec{
		Name:          name,
		ScopeAll:      spec.ScopeAll,
		ScopeGroupIDs: spec.ScopeGroupIDs,
	})
	if err != nil {
		return nil, err
	}
	if !opts.SkipConfirm {
		msg := fmt.Sprintf("\nCopy configuration profile (ID %d) as %q to destination tenant?", spec.SourceProfileID, name)
		if !ConfirmPrompt(msg, logger) {
			return nil, nil
		}
	}
	optsCopy := opts
	optsCopy.SkipConfirm = true // confirm once above
	return ProfilePostXML(dstCred, optsCopy, prepared, name, logger)
}
