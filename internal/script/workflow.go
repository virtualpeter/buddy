package script

import (
	"bufio"
	"fmt"
	"io"
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
	stepTypeSetProfilePayload = "set_profile_payload"
	stepTypeSetProfileXML     = "set_profile_xml"
)

// Step is a validated, typed workflow step descriptor.
type Step struct {
	Type string
	// set_package
	App         string
	FromTrigger string
	// set_package, clone_policy, set_ss, set_group_version, set_install
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
	// set_package, clone_policy
	PackagePattern string
	// set_trigger
	Trigger string
	// delete_policy
	PolicyIDs []int
	// create_policy (template-based)
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
	// set_profile_payload / set_profile_xml
	ProfileID int
	Domain    string
	DictFile  string
	XMLFile   string
	Redeploy  bool
}

// planLine returns a human-readable description of the step for the plan output.
func (s *Step) planLine() string {
	switch s.Type {
	case stepTypeSetPackage:
		return fmt.Sprintf("set_package        %s %s  (from %s)", s.App, s.Version, s.FromTrigger)
	case stepTypeClonePolicy:
		if s.App != "" {
			oldVer := s.OldVersion
			if oldVer == "" {
				oldVer = "?"
			}
			return fmt.Sprintf("clone_policy       %s  %s %s -> %s", s.FromTrigger, s.App, oldVer, s.Version)
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

	switch typVal {
	case stepTypeSetPackage:
		if s.App, _, err = getStr("app"); err != nil {
			return nil, err
		}
		if s.Version, _, err = getStr("version"); err != nil {
			return nil, err
		}
		if s.FromTrigger, _, err = getStr("from_trigger"); err != nil {
			return nil, err
		}
		if s.App == "" || s.Version == "" || s.FromTrigger == "" {
			return nil, fmt.Errorf("step_set_package requires app, version, from_trigger")
		}
		s.PackagePattern, _, _ = getStr("package_pattern")

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
		if s.App, _, err = getStr("app"); err != nil {
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

	default:
		return nil, fmt.Errorf("unknown workflow step type %q", typVal)
	}

	return s, nil
}

// runWorkflow validates all steps, prints a plan, optionally confirms, then executes.
func runWorkflow(credName string, stepVals []starlark.Value, logger *slog.Logger) error {
	// Validate all steps upfront before executing any.
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

	// Print plan.
	fmt.Println("Workflow plan:")
	for i, s := range steps {
		fmt.Printf("  %d. %s\n", i+1, s.planLine())
	}
	fmt.Println()

	// Confirm unless the user passed -y to buddy run.
	if !runnerOpts.SkipConfirm {
		fmt.Print("Proceed? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return fmt.Errorf("read confirmation: %w", err)
		}
		line = strings.TrimSpace(strings.ToLower(line))
		if line != "y" && line != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	opts := buddy.Opts{SkipConfirm: true, Verbose: scriptOpts.Verbose}

	// Execute steps in order.
	for i, s := range steps {
		if err := executeStep(credName, s, opts, logger); err != nil {
			return fmt.Errorf("step %d (%s): %w", i+1, s.Type, err)
		}
	}
	return nil
}

// executeStep dispatches a validated Step to the appropriate buddy function.
func executeStep(credName string, s *Step, opts buddy.Opts, logger *slog.Logger) error {
	switch s.Type {
	case stepTypeSetPackage:
		// Find the source policy ID by trigger, then set the package.
		srcID, err := buddy.PolicyFind(credName, opts, s.FromTrigger, false, logger)
		if err != nil {
			return fmt.Errorf("find source policy %q: %w", s.FromTrigger, err)
		}
		return buddy.PolicyPackage(credName, opts, s.App, s.Version, srcID, "", s.PackagePattern, logger)

	case stepTypeClonePolicy:
		srcID, err := buddy.PolicyFind(credName, opts, s.FromTrigger, false, logger)
		if err != nil {
			return fmt.Errorf("find source policy %q: %w", s.FromTrigger, err)
		}
		return buddy.PolicyClone(credName, opts, srcID, s.App, s.OldVersion, s.Version, s.PackagePattern, s.Params, logger)

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
