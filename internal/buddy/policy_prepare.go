package buddy

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// PolicyPrepareSpec controls offline transform before Classic policy POST create/copy/migrate.
type PolicyPrepareSpec struct {
	Name          string
	Trigger       *string // nil = leave source trigger_other; non-nil replaces (may be empty)
	ScopeAll      *bool   // nil + empty ScopeGroupIDs => unassigned (not all computers)
	ScopeGroupIDs []int
}

// PolicyCopySpec is input for cross-session policy copy (always POST create).
type PolicyCopySpec struct {
	SourcePolicyID int
	Name           string
	Trigger        *string
	ScopeAll       *bool
	ScopeGroupIDs  []int
	CategoryID     string // optional dest category id override; empty = resolve by source name
}

// PolicyMigrateSpec is input for cross-session policy migrate (upsert by name or trigger).
type PolicyMigrateSpec struct {
	SourcePolicyID int
	Name           string // empty => keep source name
	Trigger        *string
	MatchTrigger   bool // if true, find destination by trigger_other; else by exact name
	ScopeAll       *bool
	ScopeGroupIDs  []int
	CategoryID     string
}

// PolicyCreateOutcome is returned after a successful policy create/copy.
type PolicyCreateOutcome struct {
	PolicyID int
	Name     string
	Trigger  string
}

// PolicyMigrateOutcome is returned after policy_migrate upsert.
type PolicyMigrateOutcome struct {
	PolicyID int
	Name     string
	Trigger  string
	Created  bool
}

var policyGeneralIDRe = regexp.MustCompile(`(?i)<id>\s*\d+\s*</id>\s*`)

// PreparePolicyXMLForCreate transforms existing Classic policy XML for POST (id/0):
// normalize root, strip general id, set name/trigger, default unassigned scope,
// clear date_time_limitations.
func PreparePolicyXMLForCreate(policyXML []byte, spec PolicyPrepareSpec) ([]byte, error) {
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		return nil, fmt.Errorf("policy name is required")
	}
	if !bytes.Contains(policyXML, []byte("<policy")) && !bytes.Contains(policyXML, []byte("<ResourcePolicy")) {
		return nil, fmt.Errorf(errNotPolicyXML)
	}
	xml := normalizePolicyXMLRoot(policyXML)
	xml = stripFirstPolicyID(xml)
	var err error
	xml, err = setPolicyGeneralName(xml, name)
	if err != nil {
		return nil, err
	}
	if spec.Trigger != nil {
		xml, err = setPolicyTriggerOther(xml, *spec.Trigger)
		if err != nil {
			return nil, err
		}
	}
	scopeAll := false
	if spec.ScopeAll != nil {
		scopeAll = *spec.ScopeAll
	}
	xml, err = replacePolicyScopeBlock(xml, scopeAll, spec.ScopeGroupIDs)
	if err != nil {
		return nil, err
	}
	xml = dateTimeLimitationsBlockRegex.ReplaceAll(xml, []byte(dateTimeLimitationsEmptyXML))
	return xml, nil
}

// PolicyPostXML POSTs prepared policy XML to Classic id/0 and resolves the new policy ID.
func PolicyPostXML(credName string, opts Opts, xml []byte, policyName string, logger *slog.Logger) (*PolicyCreateOutcome, error) {
	policyName = strings.TrimSpace(policyName)
	if policyName == "" {
		return nil, fmt.Errorf("policy name is required")
	}
	if !bytes.Contains(xml, []byte("<policy")) && !bytes.Contains(xml, []byte("<ResourcePolicy")) {
		return nil, fmt.Errorf(errNotPolicyXML)
	}
	data := dateTimeLimitationsBlockRegex.ReplaceAll(normalizePolicyXMLRoot(xml), []byte(dateTimeLimitationsEmptyXML))
	if !opts.SkipConfirm {
		msg := fmt.Sprintf("\nPOST policy %q?", policyName)
		if !ConfirmPrompt(msg, logger) {
			return nil, nil
		}
	}
	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "\n--- POST payload (JSSResource/policies/id/0) ---\n%s\n--- end payload ---\n\n", data)
	}
	body, err := jamfPOST(credName, uriPolicies+"/id/0", data, "application/xml")
	if err != nil {
		return nil, err
	}
	invalidatePolicyListCache(credName)
	id := parsePolicyIDFromPostBody(body)
	if id <= 0 {
		ids, findErr := PolicyFindByKeywords(credName, opts, []string{policyName}, nil, logger)
		if findErr != nil {
			return nil, fmt.Errorf("created policy but could not resolve ID: %w", findErr)
		}
		if len(ids) == 0 {
			return nil, fmt.Errorf("created policy %q but could not find it in policy list", policyName)
		}
		id = ids[0]
	}
	trigger := extractPolicyTriggerOther(data)
	fmt.Printf("Created policy %q (ID %d)\n", policyName, id)
	return &PolicyCreateOutcome{PolicyID: id, Name: policyName, Trigger: trigger}, nil
}

// PolicyCopy fetches a policy from src, prepares it for create, remaps deps on dst, and POSTs.
func PolicyCopy(src, dst SessionRef, opts Opts, spec PolicyCopySpec, logger *slog.Logger) (*PolicyCreateOutcome, error) {
	if spec.SourcePolicyID <= 0 {
		return nil, fmt.Errorf("policy_id must be positive")
	}
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		return nil, fmt.Errorf("policy name is required")
	}
	src, err := ResolveSessionRef(src, OpClassic)
	if err != nil {
		return nil, err
	}
	dst, err = ResolveSessionRef(dst, OpClassicWrite)
	if err != nil {
		return nil, err
	}
	var policyXML []byte
	err = WithSessionRef(src, func(pin string) error {
		var fetchErr error
		policyXML, fetchErr = PolicyFetchXML(pin, spec.SourcePolicyID)
		return fetchErr
	})
	if err != nil {
		return nil, err
	}
	prepared, err := PreparePolicyXMLForCreate(policyXML, PolicyPrepareSpec{
		Name:          name,
		Trigger:       spec.Trigger,
		ScopeAll:      spec.ScopeAll,
		ScopeGroupIDs: spec.ScopeGroupIDs,
	})
	if err != nil {
		return nil, err
	}
	prepared, err = RemapPolicyDepsOnDest(src, dst, prepared, spec.CategoryID, logger)
	if err != nil {
		return nil, err
	}
	if !opts.SkipConfirm {
		msg := fmt.Sprintf("\nCopy policy (ID %d) as %q to destination tenant?", spec.SourcePolicyID, name)
		if !ConfirmPrompt(msg, logger) {
			return nil, nil
		}
	}
	optsCopy := opts
	optsCopy.SkipConfirm = true
	var out *PolicyCreateOutcome
	err = WithSessionRef(dst, func(pin string) error {
		var postErr error
		out, postErr = PolicyPostXML(pin, optsCopy, prepared, name, logger)
		return postErr
	})
	return out, err
}

// PolicyMigrate fetches from src and upserts on dst by name (or trigger_other).
func PolicyMigrate(src, dst SessionRef, opts Opts, spec PolicyMigrateSpec, logger *slog.Logger) (*PolicyMigrateOutcome, error) {
	if spec.SourcePolicyID <= 0 {
		return nil, fmt.Errorf("policy_id must be positive")
	}
	src, err := ResolveSessionRef(src, OpClassic)
	if err != nil {
		return nil, err
	}
	dst, err = ResolveSessionRef(dst, OpClassicWrite)
	if err != nil {
		return nil, err
	}
	var policyXML []byte
	err = WithSessionRef(src, func(pin string) error {
		var fetchErr error
		policyXML, fetchErr = PolicyFetchXML(pin, spec.SourcePolicyID)
		return fetchErr
	})
	if err != nil {
		return nil, err
	}
	prepared, name, err := PreparePolicyMigrateXML(policyXML, spec)
	if err != nil {
		return nil, err
	}
	prepared, err = RemapPolicyDepsOnDest(src, dst, prepared, spec.CategoryID, logger)
	if err != nil {
		return nil, err
	}
	if !opts.SkipConfirm {
		msg := fmt.Sprintf("\nMigrate policy (ID %d) as %q to destination (upsert by %s)?",
			spec.SourcePolicyID, name, migrateMatchLabel(spec.MatchTrigger))
		if !ConfirmPrompt(msg, logger) {
			return nil, nil
		}
	}
	optsCopy := opts
	optsCopy.SkipConfirm = true
	var out *PolicyMigrateOutcome
	err = WithSessionRef(dst, func(pin string) error {
		var upErr error
		out, upErr = PolicyUpsertPrepared(pin, optsCopy, prepared, name, spec.MatchTrigger, logger)
		return upErr
	})
	return out, err
}

// PolicyUpsertPrepared upserts already-prepared policy XML on credName (PUT if match exists, else POST).
// Match is exact name (case-insensitive), or trigger_other when matchTrigger is true.
func PolicyUpsertPrepared(credName string, opts Opts, prepared []byte, name string, matchTrigger bool, logger *slog.Logger) (*PolicyMigrateOutcome, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("policy name is required")
	}
	trigger := extractPolicyTriggerOther(prepared)

	var destID int
	var found bool
	var err error
	if matchTrigger {
		matchTrig := trigger
		if matchTrig == "" {
			return nil, fmt.Errorf("policy_migrate: match_trigger requires a non-empty trigger_other (pass trigger= or copy a policy that has one)")
		}
		id, findErr := PolicyFind(credName, opts, matchTrig, logger)
		if findErr == nil {
			destID = id
			found = true
		} else if !strings.Contains(findErr.Error(), "no policy found") {
			return nil, findErr
		}
	} else {
		destID, found, err = PolicyFindIDByExactName(credName, opts, name, logger)
		if err != nil {
			return nil, err
		}
	}

	if found {
		if !opts.SkipConfirm {
			msg := fmt.Sprintf("\nUpdate destination policy %q (ID %d)?", name, destID)
			if !ConfirmPrompt(msg, logger) {
				return nil, nil
			}
		}
		putXML, err := setPolicyGeneralID(prepared, destID)
		if err != nil {
			return nil, err
		}
		optsPut := opts
		optsPut.SkipConfirm = true
		if err := PolicyPutXML(credName, optsPut, destID, putXML, logger); err != nil {
			return nil, err
		}
		fmt.Printf("Updated policy %q (ID %d)\n", name, destID)
		return &PolicyMigrateOutcome{PolicyID: destID, Name: name, Trigger: trigger, Created: false}, nil
	}

	if !opts.SkipConfirm {
		msg := fmt.Sprintf("\nCreate policy %q on destination?", name)
		if !ConfirmPrompt(msg, logger) {
			return nil, nil
		}
	}
	optsPost := opts
	optsPost.SkipConfirm = true
	out, err := PolicyPostXML(credName, optsPost, prepared, name, logger)
	if err != nil {
		return nil, err
	}
	if out == nil {
		return nil, nil
	}
	return &PolicyMigrateOutcome{PolicyID: out.PolicyID, Name: out.Name, Trigger: out.Trigger, Created: true}, nil
}

// PreparePolicyMigrateXML prepares Classic policy XML for migrate upsert (name defaulted from source).
func PreparePolicyMigrateXML(policyXML []byte, spec PolicyMigrateSpec) (prepared []byte, name string, err error) {
	name = strings.TrimSpace(spec.Name)
	if name == "" {
		name = extractPolicyGeneralName(policyXML)
	}
	if name == "" {
		return nil, "", fmt.Errorf("policy name is required (pass name= or ensure source XML has <name>)")
	}
	prepared, err = PreparePolicyXMLForCreate(policyXML, PolicyPrepareSpec{
		Name:          name,
		Trigger:       spec.Trigger,
		ScopeAll:      spec.ScopeAll,
		ScopeGroupIDs: spec.ScopeGroupIDs,
	})
	if err != nil {
		return nil, "", err
	}
	return prepared, name, nil
}

func migrateMatchLabel(matchTrigger bool) string {
	if matchTrigger {
		return "trigger"
	}
	return "name"
}

// PolicyFindIDByExactName returns the policy ID whose name equals name (case-insensitive).
func PolicyFindIDByExactName(credName string, opts Opts, name string, logger *slog.Logger) (int, bool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, false, fmt.Errorf("name is required")
	}
	summaries, err := PolicyFindSummariesByKeywords(credName, opts, nil, nil, logger)
	if err != nil {
		return 0, false, err
	}
	for _, s := range summaries {
		if strings.EqualFold(s.Name, name) {
			return s.ID, true, nil
		}
	}
	return 0, false, nil
}

// PolicyDetailsMap converts PolicyDetails to a JSON-friendly map (MCP / structured output).
func PolicyDetailsMap(d *PolicyDetails) map[string]any {
	if d == nil {
		return map[string]any{}
	}
	pkgs := make([]map[string]any, 0, len(d.Packages))
	for _, p := range d.Packages {
		pkgs = append(pkgs, map[string]any{"id": p.ID, "name": p.Name, "action": p.Action})
	}
	scripts := make([]map[string]any, 0, len(d.Scripts))
	for _, s := range d.Scripts {
		params := make(map[string]any, len(s.Params))
		for k, v := range s.Params {
			params[k] = v
		}
		scripts = append(scripts, map[string]any{
			"id": s.ID, "name": s.Name, "priority": s.Priority, "params": params,
		})
	}
	groupIDs := d.Scope.ComputerGroupIDs
	if groupIDs == nil {
		groupIDs = []int{}
	}
	return map[string]any{
		"id":            d.ID,
		"name":          d.Name,
		"enabled":       d.Enabled,
		"trigger":       d.Trigger,
		"trigger_other": d.TriggerOther,
		"frequency":     d.Frequency,
		"scope": map[string]any{
			"all_computers":      d.Scope.AllComputers,
			"all_jss_users":      d.Scope.AllJSSUsers,
			"computer_group_ids": groupIDs,
		},
		"packages": pkgs,
		"scripts":  scripts,
	}
}

func normalizePolicyXMLRoot(policyXML []byte) []byte {
	out := policyXML
	out = bytes.Replace(out, []byte("<ResourcePolicy>"), []byte("<policy>"), 1)
	out = bytes.Replace(out, []byte("</ResourcePolicy>"), []byte("</policy>"), 1)
	out = bytes.Replace(out, []byte("<resource_policy>"), []byte("<policy>"), 1)
	out = bytes.Replace(out, []byte("</resource_policy>"), []byte("</policy>"), 1)
	return out
}

func stripFirstPolicyID(policyXML []byte) []byte {
	if idx := policyGeneralIDRe.FindIndex(policyXML); idx != nil {
		return append(policyXML[:idx[0]], policyXML[idx[1]:]...)
	}
	return policyXML
}

func extractPolicyGeneralName(policyXML []byte) string {
	nameStart := bytes.Index(policyXML, []byte("<name>"))
	if nameStart < 0 {
		return ""
	}
	nameStart += len("<name>")
	nameEnd := bytes.Index(policyXML[nameStart:], []byte("</name>"))
	if nameEnd < 0 {
		return ""
	}
	return strings.TrimSpace(XMLUnescape(string(policyXML[nameStart : nameStart+nameEnd])))
}

func extractPolicyTriggerOther(policyXML []byte) string {
	re := regexp.MustCompile(`(?i)<trigger_other>([\s\S]*?)</trigger_other>`)
	if m := re.FindSubmatch(policyXML); len(m) == 2 {
		return strings.TrimSpace(XMLUnescape(string(m[1])))
	}
	return ""
}

func setPolicyGeneralName(policyXML []byte, name string) ([]byte, error) {
	nameStart := bytes.Index(policyXML, []byte("<name>"))
	if nameStart < 0 {
		return nil, fmt.Errorf("policy XML has no <name> in general")
	}
	nameEnd := bytes.Index(policyXML[nameStart:], []byte("</name>"))
	if nameEnd < 0 {
		return nil, fmt.Errorf("policy XML has malformed <name>")
	}
	nameEnd += nameStart + len("</name>")
	return bytes.Join([][]byte{
		policyXML[:nameStart],
		[]byte("<name>" + xmlEncodePayload(name) + "</name>"),
		policyXML[nameEnd:],
	}, nil), nil
}

func setPolicyTriggerOther(policyXML []byte, trigger string) ([]byte, error) {
	re := regexp.MustCompile(`(?i)<trigger_other(?:\s*/>|>[\s\S]*?</trigger_other>)`)
	replacement := []byte("<trigger_other>" + xmlEncodePayload(trigger) + "</trigger_other>")
	if re.Match(policyXML) {
		return re.ReplaceAll(policyXML, replacement), nil
	}
	// Insert after <name>...</name> inside general when missing.
	nameEnd := bytes.Index(policyXML, []byte("</name>"))
	if nameEnd < 0 {
		return nil, fmt.Errorf("policy XML has no <trigger_other> and no <name> to anchor insert")
	}
	nameEnd += len("</name>")
	return bytes.Join([][]byte{policyXML[:nameEnd], []byte("\n"), replacement, policyXML[nameEnd:]}, nil), nil
}

func setPolicyGeneralID(policyXML []byte, id int) ([]byte, error) {
	if id <= 0 {
		return nil, fmt.Errorf("policy id must be positive")
	}
	idXML := []byte("<id>" + strconv.Itoa(id) + "</id>")
	if idx := policyGeneralIDRe.FindIndex(policyXML); idx != nil {
		return bytes.Join([][]byte{policyXML[:idx[0]], idXML, []byte("\n"), policyXML[idx[1]:]}, nil), nil
	}
	general := bytes.Index(policyXML, []byte("<general>"))
	if general < 0 {
		return nil, fmt.Errorf("policy XML has no <general>")
	}
	insertAt := general + len("<general>")
	return bytes.Join([][]byte{policyXML[:insertAt], []byte("\n"), idXML, policyXML[insertAt:]}, nil), nil
}

func replacePolicyScopeBlock(policyXML []byte, scopeAll bool, groupIDs []int) ([]byte, error) {
	scopeStart := bytes.Index(policyXML, []byte("<scope>"))
	if scopeStart < 0 {
		return nil, fmt.Errorf("policy XML has no <scope>")
	}
	relEnd := bytes.Index(policyXML[scopeStart:], []byte("</scope>"))
	if relEnd < 0 {
		return nil, fmt.Errorf("policy XML has malformed <scope>")
	}
	scopeEnd := scopeStart + relEnd + len("</scope>")
	var scopeXML string
	if !scopeAll && len(groupIDs) == 0 {
		scopeXML = emptyScopeXML
	} else {
		scopeXML = buildProfileScopeXML(scopeAll, groupIDs)
	}
	return bytes.Join([][]byte{policyXML[:scopeStart], []byte(scopeXML), policyXML[scopeEnd:]}, nil), nil
}
