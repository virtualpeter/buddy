package script

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"buddy/internal/buddy"

	"go.starlark.net/starlark"
)

// stepType constants identify each supported workflow step.
const (
	stepTypeSetPackage      = "set_package"
	stepTypeClonePolicy     = "clone_policy"
	stepTypeSetSS           = "set_ss"
	stepTypeSetGroupVersion = "set_group_version"
	stepTypeSetInstall      = "set_install"
	stepTypeEnablePolicy    = "enable_policy"
	stepTypeDisablePolicy   = "disable_policy"
	stepTypeSetTrigger      = "set_trigger"
	stepTypeSetScope        = "set_scope"
	stepTypeSetScopeAll     = "set_scope_all"
	stepTypeDeletePolicy    = "delete_policy"
	stepTypeCreatePolicy    = "create_policy"
	stepTypeSetProfilePayload   = "set_profile_payload"
	stepTypeSetProfileXML       = "set_profile_xml"
	stepTypeCreatePackagePolicy = "create_package_policy"
	stepTypeCreateProfile       = "create_profile"
	stepTypeCreateSS            = "create_ss"
	stepTypeNote                = "note"
	stepTypeUpgradeApp          = "upgrade_app"
)

// Step is a validated, typed workflow step descriptor.
type Step struct {
	Type string
	// set_package / clone_policy / create_package_policy: package name keyword
	Name        string
	FromTrigger string
	// set_package, clone_policy, set_ss, set_group_version, set_install, create_package_policy
	Version string
	// set_ss, set_install, enable_policy, disable_policy, set_trigger, set_scope, set_scope_all
	PolicyID int
	// set_ss, set_scope
	GroupIDs []int
	// set_group_version
	GroupID string
	// set_install, clone_policy
	Params []int
	// clone_policy (package-carrying policies)
	OldVersion string
	// set_package, clone_policy, create_package_policy
	PackagePattern string
	// set_package / clone_policy disable-source control
	DisableMode buddy.DisableSourceMode
	// set_trigger / create_ss
	Trigger string
	// delete_policy
	PolicyIDs []int
	// create_policy (template-based) / create_ss display name / create_profile name
	PolicyName        string
	PolicyTrigger     string
	ScriptID          int
	ScriptPriority    string
	ScriptParams      map[int]string
	Frequency         string
	Category          string
	ScopeGroupIDs     []int
	ScopeAll          bool
	Recon             bool
	DistributionPoint string
	// create_package_policy: "install" or "cache"
	Action string
	// create_ss: optional single scope group
	ScopeGroupID int
	// create_ss
	DisplayName string
	// set_profile_payload / set_profile_xml / create_profile
	ProfileID int
	Domain    string
	DictFile  string
	XMLFile   string
	Redeploy  bool
	// create_profile
	PayloadsPlist    string
	PayloadDomains   []string // plan-time summary (optional)
	RedeployOnUpdate string
	// Human-readable note for change records (any step, including step_note)
	Note string
}

// resolvedStep pairs a validated step with a pre-resolved plan (when applicable).
type resolvedStep struct {
	step           *Step
	planLines      []string
	pkgPlan        *buddy.PolicyPackagePlan
	clonePlan      *buddy.PolicyClonePlan
	createPkgPlan  *buddy.PolicyCreatePackagePlan
	createProfile  *buddy.ProfileCreateSpec // filled at resolve for create_profile
}

// planLine returns a human-readable description of the step for the plan output.
func (s *Step) planLine() string {
	switch s.Type {
	case stepTypeSetPackage:
		return fmt.Sprintf("set_package        %s %s  (from %s)", s.Name, s.Version, s.FromTrigger)
	case stepTypeClonePolicy:
		if s.Name != "" {
			oldVer := s.OldVersion
			if oldVer == "" {
				oldVer = "?"
			}
			return fmt.Sprintf("clone_policy       %s  %s %s -> %s", s.FromTrigger, s.Name, oldVer, s.Version)
		}
		return fmt.Sprintf("clone_policy       %s -> %s", s.FromTrigger, s.Version)
	case stepTypeSetSS:
		if len(s.GroupIDs) > 0 {
			return fmt.Sprintf("set_ss             policy %d  version %s  groups %v", s.PolicyID, s.Version, s.GroupIDs)
		}
		return fmt.Sprintf("set_ss             policy %d  version %s", s.PolicyID, s.Version)
	case stepTypeSetGroupVersion:
		return fmt.Sprintf("set_group_version  group %s -> %s", s.GroupID, s.Version)
	case stepTypeSetInstall:
		return fmt.Sprintf("set_install        policy %d  version %s  params %v", s.PolicyID, s.Version, s.Params)
	case stepTypeEnablePolicy:
		return fmt.Sprintf("enable_policy      policy %d", s.PolicyID)
	case stepTypeDisablePolicy:
		return fmt.Sprintf("disable_policy     policy %d", s.PolicyID)
	case stepTypeSetTrigger:
		return fmt.Sprintf("set_trigger        policy %d -> %q", s.PolicyID, s.Trigger)
	case stepTypeSetScope:
		return fmt.Sprintf("set_scope          policy %d  groups %v", s.PolicyID, s.GroupIDs)
	case stepTypeSetScopeAll:
		return fmt.Sprintf("set_scope_all      policy %d", s.PolicyID)
	case stepTypeDeletePolicy:
		return fmt.Sprintf("delete_policy      ids %v", s.PolicyIDs)
	case stepTypeCreatePolicy:
		return fmt.Sprintf("create_policy      %q  trigger=%s", s.PolicyName, s.PolicyTrigger)
	case stepTypeCreatePackagePolicy:
		return fmt.Sprintf("create_package_policy  %s %s  action=%s", s.Name, s.Version, s.Action)
	case stepTypeCreateProfile:
		domains := strings.Join(s.PayloadDomains, ",")
		if domains == "" {
			domains = "?"
		}
		return fmt.Sprintf("create_profile      %q  domains=%s", s.PolicyName, domains)
	case stepTypeCreateSS:
		return fmt.Sprintf("create_ss          %q  trigger=%s", s.DisplayName, s.Trigger)
	case stepTypeNote:
		return fmt.Sprintf("note               %s", s.Note)
	case stepTypeUpgradeApp:
		pat := s.PackagePattern
		if pat == "" {
			pat = "(default package search)"
		}
		return fmt.Sprintf("upgrade_app        %s -> %s  %s", s.Name, s.Version, pat)
	case stepTypeSetProfilePayload:
		redeployStr := ""
		if s.Redeploy {
			redeployStr = "  [redeploy=All]"
		}
		return fmt.Sprintf("set_profile_payload  profile %d  %s  %s%s", s.ProfileID, s.Domain, s.DictFile, redeployStr)
	case stepTypeSetProfileXML:
		redeployStr := ""
		if s.Redeploy {
			redeployStr = "  [redeploy=All]"
		}
		return fmt.Sprintf("set_profile_xml      profile %d  %s%s", s.ProfileID, s.XMLFile, redeployStr)
	default:
		return fmt.Sprintf("unknown step %q", s.Type)
	}
}

// stepFromStarlark converts a Starlark dict (produced by a step constructor) into a typed Step.
func stepFromStarlark(v starlark.Value) (*Step, error) {
	d, ok := v.(*starlark.Dict)
	if !ok {
		return nil, fmt.Errorf("workflow step must be a dict, got %s", v.Type())
	}

	getStr := func(key string) (string, bool, error) {
		val, found, err := d.Get(starlark.String(key))
		if err != nil || !found {
			return "", found, err
		}
		s, ok := val.(starlark.String)
		if !ok {
			return "", true, fmt.Errorf("field %q must be a string", key)
		}
		return string(s), true, nil
	}

	getInt := func(key string) (int, bool, error) {
		val, found, err := d.Get(starlark.String(key))
		if err != nil || !found {
			return 0, found, err
		}
		n, ok := val.(starlark.Int)
		if !ok {
			return 0, true, fmt.Errorf("field %q must be an int", key)
		}
		v64, _ := n.Int64()
		return int(v64), true, nil
	}

	getBool := func(key string) (bool, bool, error) {
		val, found, err := d.Get(starlark.String(key))
		if err != nil || !found {
			return false, found, err
		}
		b, ok := val.(starlark.Bool)
		if !ok {
			return false, true, fmt.Errorf("field %q must be a bool", key)
		}
		return bool(b), true, nil
	}

	getIntStrMap := func(key string) (map[int]string, bool, error) {
		val, found, err := d.Get(starlark.String(key))
		if err != nil || !found {
			return nil, found, err
		}
		dm, ok := val.(*starlark.Dict)
		if !ok {
			return nil, true, fmt.Errorf("field %q must be a dict", key)
		}
		out := make(map[int]string, dm.Len())
		for _, item := range dm.Items() {
			k, ok := item[0].(starlark.Int)
			if !ok {
				return nil, true, fmt.Errorf("field %q: dict keys must be ints", key)
			}
			v, ok := item[1].(starlark.String)
			if !ok {
				return nil, true, fmt.Errorf("field %q: dict values must be strings", key)
			}
			ki, _ := k.Int64()
			out[int(ki)] = string(v)
		}
		return out, true, nil
	}

	getIntList := func(key string) ([]int, bool, error) {
		val, found, err := d.Get(starlark.String(key))
		if err != nil || !found {
			return nil, found, err
		}
		lst, ok := val.(*starlark.List)
		if !ok {
			return nil, true, fmt.Errorf("field %q must be a list", key)
		}
		var out []int
		for i := 0; i < lst.Len(); i++ {
			n, ok := lst.Index(i).(starlark.Int)
			if !ok {
				return nil, true, fmt.Errorf("field %q must be a list of ints", key)
			}
			v64, _ := n.Int64()
			out = append(out, int(v64))
		}
		return out, true, nil
	}

	typVal, found, err := getStr("type")
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("workflow step dict missing required field \"type\"")
	}

	s := &Step{Type: typVal}
	if s.Note, _, err = getStr("note"); err != nil {
		return nil, err
	}

	switch typVal {
	case stepTypeNote:
		if s.Note == "" {
			return nil, fmt.Errorf("step_note requires note text")
		}

	case stepTypeUpgradeApp:
		if s.Name, _, err = getStr("name"); err != nil {
			return nil, err
		}
		if s.Version, _, err = getStr("version"); err != nil {
			return nil, err
		}
		if s.Name == "" || s.Version == "" {
			return nil, fmt.Errorf("step_upgrade_app requires name, version")
		}
		s.PackagePattern, _, _ = getStr("package_pattern")

	case stepTypeSetPackage:
		if s.Name, _, err = getStr("name"); err != nil {
			return nil, err
		}
		if s.Version, _, err = getStr("version"); err != nil {
			return nil, err
		}
		if s.FromTrigger, _, err = getStr("from_trigger"); err != nil {
			return nil, err
		}
		if s.Name == "" || s.Version == "" || s.FromTrigger == "" {
			return nil, fmt.Errorf("step_set_package requires name, version, from_trigger")
		}
		s.PackagePattern, _, _ = getStr("package_pattern")
		s.DisableMode, err = disableModeFromStepDict(d, true)
		if err != nil {
			return nil, err
		}

	case stepTypeClonePolicy:
		if s.FromTrigger, _, err = getStr("from_trigger"); err != nil {
			return nil, err
		}
		if s.Version, _, err = getStr("version"); err != nil {
			return nil, err
		}
		if s.FromTrigger == "" || s.Version == "" {
			return nil, fmt.Errorf("step_clone_policy requires from_trigger, version")
		}
		if s.Name, _, err = getStr("name"); err != nil {
			return nil, err
		}
		if s.OldVersion, _, err = getStr("old_version"); err != nil {
			return nil, err
		}
		s.PackagePattern, _, _ = getStr("package_pattern")
		s.Params, _, err = getIntList("params")
		if err != nil {
			return nil, err
		}
		if len(s.Params) == 0 {
			s.Params = []int{4, 5, 6}
		}
		s.DisableMode, err = disableModeFromStepDict(d, false)
		if err != nil {
			return nil, err
		}

	case stepTypeSetSS:
		if s.PolicyID, _, err = getInt("policy_id"); err != nil {
			return nil, err
		}
		if s.Version, _, err = getStr("version"); err != nil {
			return nil, err
		}
		if s.PolicyID == 0 || s.Version == "" {
			return nil, fmt.Errorf("step_set_ss requires policy_id, version")
		}
		s.GroupIDs, _, err = getIntList("group_ids")
		if err != nil {
			return nil, err
		}

	case stepTypeSetGroupVersion:
		if s.GroupID, _, err = getStr("group_id"); err != nil {
			return nil, err
		}
		if s.Version, _, err = getStr("version"); err != nil {
			return nil, err
		}
		if s.GroupID == "" || s.Version == "" {
			return nil, fmt.Errorf("step_set_group_version requires group_id, version")
		}

	case stepTypeSetInstall:
		if s.PolicyID, _, err = getInt("policy_id"); err != nil {
			return nil, err
		}
		if s.Version, _, err = getStr("version"); err != nil {
			return nil, err
		}
		if s.PolicyID == 0 || s.Version == "" {
			return nil, fmt.Errorf("step_set_install requires policy_id, version")
		}
		s.Params, _, err = getIntList("params")
		if err != nil {
			return nil, err
		}
		if len(s.Params) == 0 {
			s.Params = []int{4, 5, 6}
		}

	case stepTypeEnablePolicy, stepTypeDisablePolicy, stepTypeSetScopeAll:
		if s.PolicyID, _, err = getInt("policy_id"); err != nil {
			return nil, err
		}
		if s.PolicyID == 0 {
			return nil, fmt.Errorf("step_%s requires policy_id", typVal)
		}

	case stepTypeSetTrigger:
		if s.PolicyID, _, err = getInt("policy_id"); err != nil {
			return nil, err
		}
		if s.Trigger, _, err = getStr("trigger"); err != nil {
			return nil, err
		}
		if s.PolicyID == 0 || s.Trigger == "" {
			return nil, fmt.Errorf("step_set_trigger requires policy_id, trigger")
		}

	case stepTypeSetScope:
		if s.PolicyID, _, err = getInt("policy_id"); err != nil {
			return nil, err
		}
		if s.GroupIDs, _, err = getIntList("group_ids"); err != nil {
			return nil, err
		}
		if s.PolicyID == 0 || len(s.GroupIDs) == 0 {
			return nil, fmt.Errorf("step_set_scope requires policy_id, group_ids")
		}

	case stepTypeDeletePolicy:
		if s.PolicyIDs, _, err = getIntList("policy_ids"); err != nil {
			return nil, err
		}
		if len(s.PolicyIDs) == 0 {
			return nil, fmt.Errorf("step_delete_policy requires policy_ids (non-empty list)")
		}

	case stepTypeCreatePolicy:
		// Read version first; used to substitute {version} in all string fields.
		var version string
		if version, _, err = getStr("version"); err != nil {
			return nil, err
		}
		sub := func(raw string) string {
			return strings.ReplaceAll(raw, "{version}", version)
		}

		// Required fields.
		var rawName, rawTrigger string
		if rawName, _, err = getStr("name"); err != nil {
			return nil, err
		}
		if rawTrigger, _, err = getStr("trigger"); err != nil {
			return nil, err
		}
		if s.ScriptID, _, err = getInt("script_id"); err != nil {
			return nil, err
		}
		s.PolicyName = sub(rawName)
		s.PolicyTrigger = sub(rawTrigger)
		if s.PolicyName == "" || s.PolicyTrigger == "" || s.ScriptID == 0 {
			return nil, fmt.Errorf("step_create_policy requires name, trigger, and script_id")
		}

		// Optional string fields with {version} substitution.
		var rawPriority, rawFreq, rawCat, rawDP string
		if rawPriority, _, err = getStr("script_priority"); err != nil {
			return nil, err
		}
		s.ScriptPriority = sub(rawPriority)
		if s.ScriptPriority == "" {
			s.ScriptPriority = "After"
		}
		if rawFreq, _, err = getStr("frequency"); err != nil {
			return nil, err
		}
		s.Frequency = sub(rawFreq)
		if s.Frequency == "" {
			s.Frequency = "Once per computer"
		}
		if rawCat, _, err = getStr("category"); err != nil {
			return nil, err
		}
		s.Category = sub(rawCat)
		if rawDP, _, err = getStr("distribution_point"); err != nil {
			return nil, err
		}
		s.DistributionPoint = sub(rawDP)
		if s.DistributionPoint == "" {
			s.DistributionPoint = "default"
		}

		// Params dict: apply {version} substitution to values.
		var rawParams map[int]string
		if rawParams, _, err = getIntStrMap("params"); err != nil {
			return nil, err
		}
		if len(rawParams) > 0 {
			s.ScriptParams = make(map[int]string, len(rawParams))
			for k, v := range rawParams {
				s.ScriptParams[k] = sub(v)
			}
		}

		// Scope fields.
		if s.ScopeGroupIDs, _, err = getIntList("scope_group_ids"); err != nil {
			return nil, err
		}
		if s.ScopeAll, _, err = getBool("scope_all"); err != nil {
			return nil, err
		}

		// Misc.
		if s.Recon, _, err = getBool("recon"); err != nil {
			return nil, err
		}

	case stepTypeSetProfilePayload:
		if s.ProfileID, _, err = getInt("profile_id"); err != nil {
			return nil, err
		}
		if s.Domain, _, err = getStr("domain"); err != nil {
			return nil, err
		}
		if s.DictFile, _, err = getStr("dict_file"); err != nil {
			return nil, err
		}
		if s.ProfileID == 0 || s.Domain == "" || s.DictFile == "" {
			return nil, fmt.Errorf("step_set_profile_payload requires profile_id, domain, dict_file")
		}
		if s.Redeploy, _, err = getBool("redeploy"); err != nil {
			return nil, err
		}

	case stepTypeSetProfileXML:
		if s.ProfileID, _, err = getInt("profile_id"); err != nil {
			return nil, err
		}
		if s.XMLFile, _, err = getStr("xml_file"); err != nil {
			return nil, err
		}
		if s.ProfileID == 0 || s.XMLFile == "" {
			return nil, fmt.Errorf("step_set_profile_xml requires profile_id, xml_file")
		}
		if s.Redeploy, _, err = getBool("redeploy"); err != nil {
			return nil, err
		}

	case stepTypeCreatePackagePolicy:
		if s.Name, _, err = getStr("name"); err != nil {
			return nil, err
		}
		if s.Version, _, err = getStr("version"); err != nil {
			return nil, err
		}
		if s.Action, _, err = getStr("action"); err != nil {
			return nil, err
		}
		if s.Action == "" {
			s.Action = "install"
		}
		if s.Name == "" || s.Version == "" {
			return nil, fmt.Errorf("step_create_package_policy requires name and version")
		}
		if s.PackagePattern, _, err = getStr("package_pattern"); err != nil {
			return nil, err
		}
		if s.Category, _, err = getStr("category"); err != nil {
			return nil, err
		}
		if s.ScopeGroupIDs, _, err = getIntList("scope_group_ids"); err != nil {
			return nil, err
		}
		if scopeAll, found, err := getBool("scope_all"); err != nil {
			return nil, err
		} else if found {
			s.ScopeAll = scopeAll
		} else {
			s.ScopeAll = len(s.ScopeGroupIDs) == 0
		}

	case stepTypeCreateProfile:
		if s.PolicyName, _, err = getStr("name"); err != nil {
			return nil, err
		}
		if s.PolicyName == "" {
			return nil, fmt.Errorf("step_create_profile requires name")
		}
		if s.PayloadsPlist, _, err = getStr("payloads_plist_str"); err != nil {
			return nil, err
		}
		if s.Category, _, err = getStr("category"); err != nil {
			return nil, err
		}
		if s.RedeployOnUpdate, _, err = getStr("redeploy_on_update"); err != nil {
			return nil, err
		}
		if s.ScopeGroupIDs, _, err = getIntList("scope_group_ids"); err != nil {
			return nil, err
		}
		if scopeAll, found, err := getBool("scope_all"); err != nil {
			return nil, err
		} else if found {
			s.ScopeAll = scopeAll
		} else {
			s.ScopeAll = len(s.ScopeGroupIDs) == 0
		}
		if s.PayloadsPlist == "" {
			return nil, fmt.Errorf("step_create_profile requires payloads_plist_str")
		}
		if ids, idErr := buddy.PlistPayloadIdentifiers(s.PayloadsPlist); idErr == nil {
			s.PayloadDomains = ids
		}

	case stepTypeCreateSS:
		if s.Trigger, _, err = getStr("trigger"); err != nil {
			return nil, err
		}
		if s.DisplayName, _, err = getStr("display_name"); err != nil {
			return nil, err
		}
		if s.Trigger == "" || s.DisplayName == "" {
			return nil, fmt.Errorf("step_create_ss requires trigger and display_name")
		}
		if s.Category, _, err = getStr("category"); err != nil {
			return nil, err
		}
		if s.ScopeGroupID, _, err = getInt("scope_group_id"); err != nil {
			return nil, err
		}
		if s.ScriptID, _, err = getInt("script_id"); err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("unknown workflow step type %q", typVal)
	}

	return s, nil
}

// disableModeFromStepDict reads disable_source / keep_source from a step dict.
// When allowDisableSource is false (clone), only keep_source is accepted.
func disableModeFromStepDict(d *starlark.Dict, allowDisableSource bool) (buddy.DisableSourceMode, error) {
	getOptBool := func(key string) (starlark.Value, error) {
		val, found, err := d.Get(starlark.String(key))
		if err != nil {
			return nil, err
		}
		if !found {
			return starlark.None, nil
		}
		return val, nil
	}
	keep, err := getOptBool("keep_source")
	if err != nil {
		return buddy.DisableSourceAuto, err
	}
	if !allowDisableSource {
		return disableModeFromKeepSource(keep)
	}
	disable, err := getOptBool("disable_source")
	if err != nil {
		return buddy.DisableSourceAuto, err
	}
	return disableModeFromPackageKwargs(disable, keep)
}

// runWorkflow validates all steps, resolves package/clone plans, prints a plan,
// optionally confirms, then executes (reusing resolved plans when present).
func runWorkflow(credName string, stepVals []starlark.Value, logger *slog.Logger) error {
	steps := make([]*Step, 0, len(stepVals))
	for i, v := range stepVals {
		s, err := stepFromStarlark(v)
		if err != nil {
			return fmt.Errorf("step %d: %w", i+1, err)
		}
		steps = append(steps, s)
	}
	if len(steps) == 0 {
		return fmt.Errorf("run_workflow: no steps provided")
	}

	opts := buddy.Opts{SkipConfirm: true, Verbose: scriptOpts.Verbose}

	resolved := make([]resolvedStep, 0, len(steps))
	for i, s := range steps {
		rs, err := resolveWorkflowStep(credName, s, opts, logger)
		if err != nil {
			return fmt.Errorf("step %d (%s): %w", i+1, s.Type, err)
		}
		resolved = append(resolved, rs)
	}

	fmt.Println("Workflow plan:")
	for i, rs := range resolved {
		if len(rs.planLines) == 0 {
			fmt.Printf("  %d. %s\n", i+1, rs.step.planLine())
			continue
		}
		fmt.Printf("  %d. %s\n", i+1, rs.planLines[0])
		for _, line := range rs.planLines[1:] {
			fmt.Printf("      %s\n", line)
		}
	}
	fmt.Println()

	if !runnerOpts.SkipConfirm {
		if !buddy.ConfirmPrompt("", logger) {
			return nil
		}
	}

	for i, rs := range resolved {
		if err := executeResolvedStep(credName, rs, opts, logger); err != nil {
			return fmt.Errorf("step %d (%s): %w", i+1, rs.step.Type, err)
		}
	}
	return nil
}

func resolveWorkflowStep(credName string, s *Step, opts buddy.Opts, logger *slog.Logger) (resolvedStep, error) {
	rs := resolvedStep{step: s}
	switch s.Type {
	case stepTypeSetPackage:
		srcID, err := buddy.PolicyFind(credName, opts, s.FromTrigger, logger)
		if err != nil {
			return rs, fmt.Errorf("find source policy %q: %w", s.FromTrigger, err)
		}
		plan, err := buddy.PlanPolicyPackage(credName, opts, s.Name, s.Version, srcID, "", s.PackagePattern, s.DisableMode, logger)
		if err != nil {
			return rs, err
		}
		rs.pkgPlan = plan
		rs.planLines = append([]string{fmt.Sprintf("set_package        %s %s", s.Name, s.Version)}, plan.Actions...)
		return rs, nil
	case stepTypeClonePolicy:
		srcID, err := buddy.PolicyFind(credName, opts, s.FromTrigger, logger)
		if err != nil {
			return rs, fmt.Errorf("find source policy %q: %w", s.FromTrigger, err)
		}
		plan, err := buddy.PlanPolicyClone(credName, opts, srcID, s.Name, s.OldVersion, s.Version, s.PackagePattern, s.Params, s.DisableMode, logger)
		if err != nil {
			return rs, err
		}
		rs.clonePlan = plan
		rs.planLines = append([]string{fmt.Sprintf("clone_policy       %s -> %s", s.FromTrigger, s.Version)}, plan.Actions...)
		return rs, nil
	case stepTypeCreatePackagePolicy:
		plan, err := buddy.PlanPolicyCreatePackage(credName, opts, s.Name, s.Version, s.Action, s.ScopeAll, s.ScopeGroupIDs, s.Category, s.PackagePattern, logger)
		if err != nil {
			return rs, err
		}
		rs.createPkgPlan = plan
		rs.planLines = append([]string{s.planLine()}, plan.Actions...)
		return rs, nil
	case stepTypeCreateProfile:
		spec := &buddy.ProfileCreateSpec{
			Name:             s.PolicyName,
			PayloadsPlist:    s.PayloadsPlist,
			Category:         s.Category,
			ScopeAll:         s.ScopeAll,
			ScopeGroupIDs:    s.ScopeGroupIDs,
			RedeployOnUpdate: s.RedeployOnUpdate,
		}
		rs.createProfile = spec
		domains := s.PayloadDomains
		if len(domains) == 0 {
			if ids, err := buddy.PlistPayloadIdentifiers(s.PayloadsPlist); err == nil {
				domains = ids
				s.PayloadDomains = ids
			}
		}
		scopeDesc := "all computers"
		if !s.ScopeAll && len(s.ScopeGroupIDs) > 0 {
			scopeDesc = fmt.Sprintf("groups %v", s.ScopeGroupIDs)
		}
		rs.planLines = []string{
			fmt.Sprintf("create_profile      %q", s.PolicyName),
			fmt.Sprintf("Payload domains: %s", strings.Join(domains, ", ")),
			fmt.Sprintf("Scope: %s", scopeDesc),
		}
		return rs, nil
	default:
		rs.planLines = []string{s.planLine()}
		return rs, nil
	}
}

func executeResolvedStep(credName string, rs resolvedStep, opts buddy.Opts, logger *slog.Logger) error {
	switch rs.step.Type {
	case stepTypeSetPackage:
		if rs.pkgPlan != nil {
			_, err := buddy.ApplyPolicyPackagePlan(credName, opts, rs.pkgPlan, logger)
			return err
		}
	case stepTypeClonePolicy:
		if rs.clonePlan != nil {
			_, err := buddy.ApplyPolicyClonePlan(credName, opts, rs.clonePlan, logger)
			return err
		}
	case stepTypeCreatePackagePolicy:
		if rs.createPkgPlan != nil {
			_, err := buddy.ApplyPolicyCreatePackagePlan(credName, opts, rs.createPkgPlan, logger)
			return err
		}
	case stepTypeCreateProfile:
		if rs.createProfile != nil {
			_, err := buddy.ProfileCreate(credName, opts, *rs.createProfile, logger)
			return err
		}
	case stepTypeNote:
		return nil
	}
	return executeStep(credName, rs.step, opts, logger)
}

// executeStep dispatches a validated Step to the appropriate buddy function.
func executeStep(credName string, s *Step, opts buddy.Opts, logger *slog.Logger) error {
	switch s.Type {
	case stepTypeNote:
		return nil

	case stepTypeUpgradeApp:
		return buddy.AppUpgrade(credName, opts, s.Name, s.Version, s.PackagePattern, logger)

	case stepTypeSetPackage:
		srcID, err := buddy.PolicyFind(credName, opts, s.FromTrigger, logger)
		if err != nil {
			return fmt.Errorf("find source policy %q: %w", s.FromTrigger, err)
		}
		return buddy.PolicyPackage(credName, opts, s.Name, s.Version, srcID, "", s.PackagePattern, s.DisableMode, logger)

	case stepTypeClonePolicy:
		srcID, err := buddy.PolicyFind(credName, opts, s.FromTrigger, logger)
		if err != nil {
			return fmt.Errorf("find source policy %q: %w", s.FromTrigger, err)
		}
		return buddy.PolicyClone(credName, opts, srcID, s.Name, s.OldVersion, s.Version, s.PackagePattern, s.Params, s.DisableMode, logger)

	case stepTypeSetSS:
		return buddy.PolicySS(credName, opts, s.PolicyID, s.Version, s.GroupIDs, logger)

	case stepTypeSetGroupVersion:
		return buddy.GroupVersion(credName, opts, s.GroupID, s.Version, logger)

	case stepTypeSetInstall:
		return buddy.PolicyInstall(credName, opts, s.PolicyID, s.Version, s.Params, logger)

	case stepTypeEnablePolicy:
		return buddy.PolicyEnable(credName, opts, s.PolicyID, true, logger)

	case stepTypeDisablePolicy:
		return buddy.PolicyEnable(credName, opts, s.PolicyID, false, logger)

	case stepTypeSetTrigger:
		return buddy.PolicyTrigger(credName, opts, s.PolicyID, s.Trigger, logger)

	case stepTypeSetScope:
		return buddy.PolicyScope(credName, opts, s.PolicyID, s.GroupIDs, logger)

	case stepTypeSetScopeAll:
		return buddy.PolicyScopeAll(credName, opts, s.PolicyID, logger)

	case stepTypeDeletePolicy:
		return buddy.PolicyDelete(credName, opts, s.PolicyIDs, logger)

	case stepTypeCreatePolicy:
		tmpl := buddy.PolicyTemplate{
			Name:              s.PolicyName,
			Trigger:           s.PolicyTrigger,
			ScriptID:          s.ScriptID,
			ScriptPriority:    s.ScriptPriority,
			ScriptParams:      s.ScriptParams,
			Frequency:         s.Frequency,
			Category:          s.Category,
			ScopeGroupIDs:     s.ScopeGroupIDs,
			ScopeAll:          s.ScopeAll,
			Recon:             s.Recon,
			DistributionPoint: s.DistributionPoint,
		}
		// {version} substitution was applied in stepFromStarlark; pass "" so
		// PolicyCreateFromTemplate performs no further substitution.
		return buddy.PolicyCreateFromTemplate(credName, opts, tmpl, "", logger)

	case stepTypeSetProfilePayload:
		return buddy.ProfileSetPayload(credName, opts, s.ProfileID, s.Domain, s.DictFile, s.Redeploy, logger)

	case stepTypeSetProfileXML:
		f, err := os.Open(s.XMLFile)
		if err != nil {
			return fmt.Errorf("open %s: %w", s.XMLFile, err)
		}
		defer f.Close()
		return buddy.ProfileSetXML(credName, opts, s.ProfileID, f, s.Redeploy, logger)

	case stepTypeCreatePackagePolicy:
		_, err := buddy.PolicyCreatePackage(credName, opts, s.Name, s.Version, s.Action, s.ScopeAll, s.ScopeGroupIDs, s.Category, s.PackagePattern, logger)
		return err

	case stepTypeCreateProfile:
		_, err := buddy.ProfileCreate(credName, opts, buddy.ProfileCreateSpec{
			Name:             s.PolicyName,
			PayloadsPlist:    s.PayloadsPlist,
			Category:         s.Category,
			ScopeAll:         s.ScopeAll,
			ScopeGroupIDs:    s.ScopeGroupIDs,
			RedeployOnUpdate: s.RedeployOnUpdate,
		}, logger)
		return err

	case stepTypeCreateSS:
		_, err := buddy.PolicyCreateSS(credName, opts, s.Trigger, s.DisplayName, s.Category, s.ScopeGroupID, s.ScriptID, logger)
		return err

	default:
		return fmt.Errorf("unknown step type %q", s.Type)
	}
}

// stepDict builds a Starlark dict with "type" plus any additional key/value pairs.
func stepDict(stepType string, pairs ...interface{}) *starlark.Dict {
	d := new(starlark.Dict)
	_ = d.SetKey(starlark.String("type"), starlark.String(stepType))
	for i := 0; i+1 < len(pairs); i += 2 {
		key := starlark.String(pairs[i].(string))
		var val starlark.Value
		switch v := pairs[i+1].(type) {
		case string:
			val = starlark.String(v)
		case int:
			val = starlark.MakeInt(v)
		case int64:
			val = starlark.MakeInt64(v)
		case bool:
			val = starlark.Bool(v)
		case starlark.Value:
			val = v
		default:
			val = starlark.String(fmt.Sprintf("%v", v))
		}
		_ = d.SetKey(key, val)
	}
	return d
}
