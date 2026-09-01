package buddy

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

// Opts holds common options for buddy operations.
type Opts struct {
	SkipConfirm bool
	Verbose     bool
	Trace       bool // -t/--trace: HTTP request/response tracing
	// Progress prints human lookup steps to stderr (API + what is being sought).
	// Distinct from Verbose, which also dumps XML payloads.
	Progress bool
}

const errFetchPolicyFmt = "fetch policy %d: %w"
const errNotPolicyXML = "input does not appear to be policy XML (missing <policy> or <ResourcePolicy> root)"

func (o Opts) logProgress() bool { return o.Progress || o.Verbose }

func progressf(opts Opts, format string, args ...interface{}) {
	if !opts.logProgress() {
		return
	}
	if !strings.HasSuffix(format, "\n") {
		format += "\n"
	}
	fmt.Fprintf(os.Stderr, format, args...)
}

// PolicySS updates policy self_service_description and optionally scope.
func PolicySS(credName string, opts Opts, policyID int, newVersion string, groupIDs []int, logger *slog.Logger) error {
	policy, err := policyGetByID(credName, policyID)
	if err != nil {
		return fmt.Errorf(errFetchPolicyFmt, policyID, err)
	}
	payload := buildPolicyUpdatePayload(policy, newVersion, groupIDs)
	if !opts.SkipConfirm {
		msg := fmt.Sprintf("\nUpdate policy %q (ID %d) self_service_description: %q -> %q", policy.General.Name, policyID, policy.SelfService.SelfServiceDescription, payload.SelfService.SelfServiceDescription)
		if len(groupIDs) > 0 {
			msg += fmt.Sprintf("\nSet scope to computer groups: %v", groupIDs)
		}
		if !ConfirmPrompt(msg, logger) {
			return nil
		}
	}
	if opts.Verbose {
		xmlPayload, _ := xml.MarshalIndent(payload, "", "  ")
		fmt.Fprintf(os.Stderr, "\n--- PUT payload (JSSResource/policies/id/%d) ---\n%s\n--- end payload ---\n\n", policyID, xmlPayload)
	}
	if err := putPolicy(credName, policyID, payload, logger); err != nil {
		return err
	}
	fmt.Printf("Updated policy %q (ID %d) self_service_description with version %s\n", policy.General.Name, policyID, newVersion)
	return nil
}

// PolicySSOpts holds optional overrides for self-service fields.
// Pointer fields: nil means "preserve existing", non-nil means "set to this value".
type PolicySSOpts struct {
	Version       string
	GroupIDs      []int
	DisplayName   *string
	Description   *string
	InstallText   *string
	ReinstallText *string
	ForceView     *bool
	Feature       *bool
	SelfService   *bool
}

// PolicySSUpdate updates self-service fields on a policy. Supports the legacy
// version-swap behaviour and/or individual field overrides via PolicySSOpts.
func PolicySSUpdate(credName string, opts Opts, policyID int, ssOpts PolicySSOpts, logger *slog.Logger) error {
	policy, err := policyGetByID(credName, policyID)
	if err != nil {
		return fmt.Errorf(errFetchPolicyFmt, policyID, err)
	}

	payload := buildPolicyUpdatePayload(policy, ssOpts.Version, ssOpts.GroupIDs)

	// Apply explicit overrides.
	if ssOpts.DisplayName != nil {
		payload.SelfService.SelfServiceDisplayName = *ssOpts.DisplayName
	}
	if ssOpts.Description != nil {
		payload.SelfService.SelfServiceDescription = *ssOpts.Description
	}
	if ssOpts.InstallText != nil {
		payload.SelfService.InstallButtonText = *ssOpts.InstallText
	}
	if ssOpts.ReinstallText != nil {
		payload.SelfService.ReinstallButtonText = *ssOpts.ReinstallText
	}
	if ssOpts.ForceView != nil {
		payload.SelfService.ForceUsersToViewDescription = *ssOpts.ForceView
	}
	if ssOpts.Feature != nil {
		payload.SelfService.FeatureOnMainPage = *ssOpts.Feature
	}
	if ssOpts.SelfService != nil {
		payload.SelfService.UseForSelfService = *ssOpts.SelfService
	}

	// Build confirmation message listing all changes.
	if !opts.SkipConfirm {
		var changes []string
		if ssOpts.Version != "" {
			changes = append(changes, fmt.Sprintf("description version: %q -> %q",
				policy.SelfService.SelfServiceDescription, payload.SelfService.SelfServiceDescription))
		}
		if ssOpts.DisplayName != nil {
			changes = append(changes, fmt.Sprintf("display_name: %q -> %q",
				policy.SelfService.SelfServiceDisplayName, *ssOpts.DisplayName))
		}
		if ssOpts.Description != nil {
			changes = append(changes, fmt.Sprintf("description: %q -> %q",
				policy.SelfService.SelfServiceDescription, *ssOpts.Description))
		}
		if ssOpts.InstallText != nil {
			changes = append(changes, fmt.Sprintf("install_button_text: %q -> %q",
				policy.SelfService.InstallButtonText, *ssOpts.InstallText))
		}
		if ssOpts.ReinstallText != nil {
			changes = append(changes, fmt.Sprintf("reinstall_button_text: %q -> %q",
				policy.SelfService.ReinstallButtonText, *ssOpts.ReinstallText))
		}
		if ssOpts.ForceView != nil {
			changes = append(changes, fmt.Sprintf("force_users_to_view_description: %v -> %v",
				policy.SelfService.ForceUsersToViewDescription, *ssOpts.ForceView))
		}
		if ssOpts.Feature != nil {
			changes = append(changes, fmt.Sprintf("feature_on_main_page: %v -> %v",
				policy.SelfService.FeatureOnMainPage, *ssOpts.Feature))
		}
		if ssOpts.SelfService != nil {
			changes = append(changes, fmt.Sprintf("use_for_self_service: %v -> %v",
				policy.SelfService.UseForSelfService, *ssOpts.SelfService))
		}
		if len(ssOpts.GroupIDs) > 0 {
			changes = append(changes, fmt.Sprintf("scope -> computer groups: %v", ssOpts.GroupIDs))
		}
		msg := fmt.Sprintf("\nUpdate policy %q (ID %d):", policy.General.Name, policyID)
		for _, c := range changes {
			msg += "\n  " + c
		}
		if !ConfirmPrompt(msg, logger) {
			return nil
		}
	}

	if opts.Verbose {
		xmlPayload, _ := xml.MarshalIndent(payload, "", "  ")
		fmt.Fprintf(os.Stderr, "\n--- PUT payload (JSSResource/policies/id/%d) ---\n%s\n--- end payload ---\n\n", policyID, xmlPayload)
	}
	if err := putPolicy(credName, policyID, payload, logger); err != nil {
		return err
	}
	fmt.Printf("Updated policy %q (ID %d) self-service settings\n", policy.General.Name, policyID)
	return nil
}

// PolicyScope sets policy scope to the given computer group IDs.
func PolicyScope(credName string, opts Opts, policyID int, groupIDs []int, logger *slog.Logger) error {
	policy, err := policyGetByID(credName, policyID)
	if err != nil {
		return fmt.Errorf(errFetchPolicyFmt, policyID, err)
	}
	if len(groupIDs) == 0 {
		return fmt.Errorf("at least one computer group ID required")
	}
	payload := buildPolicyUpdatePayload(policy, policy.SelfService.SelfServiceDescription, groupIDs)
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nSet policy %q (ID %d) scope to computer groups: %v", policy.General.Name, policyID, groupIDs), logger) {
			return nil
		}
	}
	if opts.Verbose {
		xmlPayload, _ := xml.MarshalIndent(payload, "", "  ")
		fmt.Fprintf(os.Stderr, "\n--- PUT payload (JSSResource/policies/id/%d) ---\n%s\n--- end payload ---\n\n", policyID, xmlPayload)
	}
	if err := putPolicy(credName, policyID, payload, logger); err != nil {
		return err
	}
	fmt.Printf("Set policy %q (ID %d) scope to computer groups: %v\n", policy.General.Name, policyID, groupIDs)
	return nil
}

// PolicyScopeAll sets policy scope to All Computers.
func PolicyScopeAll(credName string, opts Opts, policyID int, logger *slog.Logger) error {
	policy, err := policyGetByID(credName, policyID)
	if err != nil {
		return fmt.Errorf(errFetchPolicyFmt, policyID, err)
	}
	payload := buildPolicyUpdatePayload(policy, policy.SelfService.SelfServiceDescription, nil)
	payload.Scope.AllComputers = true
	payload.Scope.ComputerGroups = nil
	payload.Scope.Computers = nil
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nSet policy %q (ID %d) scope to All Computers", policy.General.Name, policyID), logger) {
			return nil
		}
	}
	if opts.Verbose {
		xmlPayload, _ := xml.MarshalIndent(payload, "", "  ")
		fmt.Fprintf(os.Stderr, "\n--- PUT payload (JSSResource/policies/id/%d) ---\n%s\n--- end payload ---\n\n", policyID, xmlPayload)
	}
	if err := putPolicy(credName, policyID, payload, logger); err != nil {
		return err
	}
	fmt.Printf("Set policy %q (ID %d) scope to All Computers\n", policy.General.Name, policyID)
	return nil
}

// PolicyEnable sets policy enabled state (true = enable, false = disable).
func PolicyEnable(credName string, opts Opts, policyID int, enabled bool, logger *slog.Logger) error {
	policy, err := policyGetByID(credName, policyID)
	if err != nil {
		return fmt.Errorf(errFetchPolicyFmt, policyID, err)
	}
	if policy.General.Enabled == enabled {
		state := "enabled"
		if !enabled {
			state = "disabled"
		}
		fmt.Printf("Policy %q (ID %d) is already %s.\n", policy.General.Name, policyID, state)
		return nil
	}
	if !opts.SkipConfirm {
		from, to := "disabled", "disabled"
		if policy.General.Enabled {
			from = "enabled"
		}
		if enabled {
			to = "enabled"
		}
		if !ConfirmPrompt(fmt.Sprintf("\nPolicy %q (ID %d): %s -> %s", policy.General.Name, policyID, from, to), logger) {
			return nil
		}
	}
	payload := &policyEnablePayload{}
	payload.General.Enabled = enabled
	if err := putPolicyGeneric(credName, policyID, payload, logger); err != nil {
		return err
	}
	verb := "Enabled"
	if !enabled {
		verb = "Disabled"
	}
	fmt.Printf("%s policy %q (ID %d)\n", verb, policy.General.Name, policyID)
	return nil
}

// PolicyGetEnable fetches policy and prints enabled/disabled state.
func PolicyGetEnable(credName string, opts Opts, policyID int, logger *slog.Logger) error {
	state, err := PolicyGetEnableValue(credName, policyID, logger)
	if err != nil {
		return err
	}
	fmt.Println(state)
	return nil
}

// PolicyGetEnableValue fetches policy and returns enabled/disabled state (for script use).
func PolicyGetEnableValue(credName string, policyID int, logger *slog.Logger) (string, error) {
	policy, err := policyGetByID(credName, policyID)
	if err != nil {
		return "", fmt.Errorf(errFetchPolicyFmt, policyID, err)
	}
	if policy.General.Enabled {
		return "enabled", nil
	}
	return "disabled", nil
}

// PolicyDetails holds extracted policy attributes for script use.
type PolicyDetails struct {
	ID           int
	Name         string
	Enabled      bool
	Trigger      string
	TriggerOther string
	Frequency    string
	Scope        struct {
		AllComputers     bool
		AllJSSUsers      bool
		ComputerGroupIDs []int
	}
	Packages []struct {
		ID     int
		Name   string
		Action string
	}
	Scripts []struct {
		ID       string
		Name     string
		Priority string
		Params   map[string]string
	}
}

// PolicyGetDetails fetches a policy and returns extracted details for script use.
func PolicyGetDetails(credName string, policyID int, logger *slog.Logger) (*PolicyDetails, error) {
	policy, err := policyGetByID(credName, policyID)
	if err != nil {
		return nil, fmt.Errorf(errFetchPolicyFmt, policyID, err)
	}
	d := &PolicyDetails{}
	d.ID = policy.General.ID
	d.Name = policy.General.Name
	d.Enabled = policy.General.Enabled
	d.Trigger = policy.General.Trigger
	d.TriggerOther = strings.TrimSpace(policy.General.TriggerOther)
	d.Frequency = policy.General.Frequency
	d.Scope.AllComputers = policy.Scope.AllComputers
	d.Scope.AllJSSUsers = policy.Scope.AllJSSUsers
	if policy.Scope.ComputerGroups != nil {
		for _, g := range *policy.Scope.ComputerGroups {
			d.Scope.ComputerGroupIDs = append(d.Scope.ComputerGroupIDs, g.ID)
		}
	}
	for _, pkg := range policy.PackageConfiguration.Packages {
		d.Packages = append(d.Packages, struct {
			ID     int
			Name   string
			Action string
		}{ID: pkg.ID, Name: pkg.Name, Action: pkg.Action})
	}
	for _, s := range policy.Scripts {
		params := make(map[string]string)
		for i := 4; i <= 11; i++ {
			v := getScriptParam(&s, i)
			if v != "" {
				params[fmt.Sprintf("%d", i)] = v
			}
		}
		d.Scripts = append(d.Scripts, struct {
			ID       string
			Name     string
			Priority string
			Params   map[string]string
		}{ID: s.ID, Name: s.Name, Priority: s.Priority, Params: params})
	}
	return d, nil
}

// PolicyTrigger updates policy trigger_other.
func PolicyTrigger(credName string, opts Opts, policyID int, newTrigger string, logger *slog.Logger) error {
	policy, err := policyGetByID(credName, policyID)
	if err != nil {
		return fmt.Errorf(errFetchPolicyFmt, policyID, err)
	}
	currentTrigger := strings.TrimSpace(policy.General.TriggerOther)
	if currentTrigger == "" {
		return fmt.Errorf("policy %q (ID %d) does not have a custom trigger (trigger_other is empty)", policy.General.Name, policyID)
	}
	if strings.EqualFold(currentTrigger, newTrigger) {
		fmt.Printf("Policy %q (ID %d) already has trigger %q.\n", policy.General.Name, policyID, currentTrigger)
		return nil
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nPolicy %q (ID %d): trigger %q -> %q", policy.General.Name, policyID, currentTrigger, newTrigger), logger) {
			return nil
		}
	}
	payload := &policyTriggerPayload{General: struct {
		TriggerOther string `xml:"trigger_other"`
	}{TriggerOther: newTrigger}}
	if opts.Verbose {
		xmlPayload, _ := xml.MarshalIndent(payload, "", "  ")
		fmt.Fprintf(os.Stderr, "\n--- PUT payload (JSSResource/policies/id/%d) ---\n%s\n--- end payload ---\n\n", policyID, xmlPayload)
	}
	if err := putPolicyGeneric(credName, policyID, payload, logger); err != nil {
		return err
	}
	fmt.Printf("Updated policy %q (ID %d) trigger: %q -> %q\n", policy.General.Name, policyID, currentTrigger, newTrigger)
	return nil
}

// PolicySetName renames a policy.
func PolicySetName(credName string, opts Opts, policyID int, newName string, logger *slog.Logger) error {
	policy, err := policyGetByID(credName, policyID)
	if err != nil {
		return fmt.Errorf(errFetchPolicyFmt, policyID, err)
	}
	currentName := policy.General.Name
	if currentName == newName {
		fmt.Printf("Policy (ID %d) already has name %q.\n", policyID, currentName)
		return nil
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nRename policy (ID %d): %q -> %q", policyID, currentName, newName), logger) {
			return nil
		}
	}
	payload := &policyNamePayload{}
	payload.General.Name = newName
	if opts.Verbose {
		xmlPayload, _ := xml.MarshalIndent(payload, "", "  ")
		fmt.Fprintf(os.Stderr, "\n--- PUT payload (JSSResource/policies/id/%d) ---\n%s\n--- end payload ---\n\n", policyID, xmlPayload)
	}
	if err := putPolicyGeneric(credName, policyID, payload, logger); err != nil {
		return err
	}
	fmt.Printf("Renamed policy (ID %d): %q -> %q\n", policyID, currentName, newName)
	return nil
}

// PolicySetFrequency sets the policy execution frequency.
func PolicySetFrequency(credName string, opts Opts, policyID int, frequency string, logger *slog.Logger) error {
	policy, err := policyGetByID(credName, policyID)
	if err != nil {
		return fmt.Errorf(errFetchPolicyFmt, policyID, err)
	}
	currentFreq := strings.TrimSpace(policy.General.Frequency)
	newFreq := strings.TrimSpace(frequency)
	if strings.EqualFold(currentFreq, newFreq) {
		fmt.Printf("Policy %q (ID %d) already has frequency %q.\n", policy.General.Name, policyID, currentFreq)
		return nil
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nSet policy %q (ID %d) frequency: %q -> %q", policy.General.Name, policyID, currentFreq, newFreq), logger) {
			return nil
		}
	}
	payload := &policyFrequencyPayload{}
	payload.General.Frequency = newFreq
	if opts.Verbose {
		xmlPayload, _ := xml.MarshalIndent(payload, "", "  ")
		fmt.Fprintf(os.Stderr, "\n--- PUT payload (JSSResource/policies/id/%d) ---\n%s\n--- end payload ---\n\n", policyID, xmlPayload)
	}
	if err := putPolicyGeneric(credName, policyID, payload, logger); err != nil {
		return err
	}
	fmt.Printf("Updated policy %q (ID %d) frequency: %q -> %q\n", policy.General.Name, policyID, currentFreq, newFreq)
	return nil
}

// PolicySetIcon uploads an icon image and sets it on a policy's self-service configuration.
func PolicySetIcon(credName string, opts Opts, policyID int, filePath string, logger *slog.Logger) error {
	if _, err := os.Stat(filePath); err != nil {
		return fmt.Errorf("icon file %q: %w", filePath, err)
	}

	policy, err := policyGetByID(credName, policyID)
	if err != nil {
		return fmt.Errorf(errFetchPolicyFmt, policyID, err)
	}

	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nUpload icon %q and set on policy %q (ID %d)", filePath, policy.General.Name, policyID), logger) {
			return nil
		}
	}

	icon, err := IconUpload(credName, filePath)
	if err != nil {
		return fmt.Errorf("upload icon: %w", err)
	}
	logger.Debug("Uploaded icon", "id", icon.ID)
	return policyApplySelfServiceIcon(credName, opts, policy, policyID, icon.ID, logger)
}

// PolicySetIconID assigns an existing tenant icon id on a policy's Self Service config.
func PolicySetIconID(credName string, opts Opts, policyID, iconID int, logger *slog.Logger) error {
	if iconID <= 0 {
		return fmt.Errorf("icon id must be positive")
	}
	policy, err := policyGetByID(credName, policyID)
	if err != nil {
		return fmt.Errorf(errFetchPolicyFmt, policyID, err)
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nSet icon ID %d on policy %q (ID %d)", iconID, policy.General.Name, policyID), logger) {
			return nil
		}
	}
	return policyApplySelfServiceIcon(credName, opts, policy, policyID, iconID, logger)
}

func policyApplySelfServiceIcon(credName string, opts Opts, policy *resourcePolicy, policyID, iconID int, logger *slog.Logger) error {
	payload := &policyIconPayload{}
	payload.SelfService.UseForSelfService = policy.SelfService.UseForSelfService
	payload.SelfService.SelfServiceDisplayName = policy.SelfService.SelfServiceDisplayName
	payload.SelfService.InstallButtonText = policy.SelfService.InstallButtonText
	payload.SelfService.ReinstallButtonText = policy.SelfService.ReinstallButtonText
	payload.SelfService.SelfServiceDescription = policy.SelfService.SelfServiceDescription
	payload.SelfService.ForceUsersToViewDescription = policy.SelfService.ForceUsersToViewDescription
	payload.SelfService.FeatureOnMainPage = policy.SelfService.FeatureOnMainPage
	payload.SelfService.SelfServiceIcon.ID = iconID
	if opts.Verbose {
		xmlPayload, _ := xml.MarshalIndent(payload, "", "  ")
		fmt.Fprintf(os.Stderr, "\n--- PUT payload (JSSResource/policies/id/%d) ---\n%s\n--- end payload ---\n\n", policyID, xmlPayload)
	}
	if err := putPolicyGeneric(credName, policyID, payload, logger); err != nil {
		return err
	}
	fmt.Printf("Set icon on policy %q (ID %d): icon ID %d\n", policy.General.Name, policyID, iconID)
	return nil
}

// PolicyInstall updates install policy version in script params (edit in place only).
func PolicyInstall(credName string, opts Opts, policyID int, newVersion string, paramIndices []int, logger *slog.Logger) error {
	policy, err := policyGetByID(credName, policyID)
	if err != nil {
		return fmt.Errorf(errFetchPolicyFmt, policyID, err)
	}
	if len(policy.Scripts) == 0 {
		return fmt.Errorf("policy %d has no scripts", policyID)
	}
	if len(policy.PackageConfiguration.Packages) != 0 {
		return fmt.Errorf("policy %d has packages (expected script-only policy)", policyID)
	}

	trigger := policy.General.TriggerOther
	payload := buildPolicyInstallPayload(policy, policy.General.Name, trigger, newVersion, paramIndices)
	action := fmt.Sprintf("Update policy %q (ID %d): set version in script params", policy.General.Name, policyID)

	if !opts.SkipConfirm {
		fmt.Printf("\n%s\n\n", action)
		if !ConfirmPrompt("", logger) {
			return nil
		}
	}
	if opts.Verbose {
		xmlPayload, _ := xml.MarshalIndent(payload, "", "  ")
		fmt.Fprintf(os.Stderr, "\n--- PUT payload (JSSResource/policies/id/%d) ---\n%s\n--- end payload ---\n\n", policyID, xmlPayload)
	}
	if err := putPolicyInstall(credName, policyID, payload, logger); err != nil {
		return err
	}
	fmt.Printf("Updated policy %q (ID %d) with version %s in script params\n", policy.General.Name, policyID, newVersion)
	return nil
}

// DisableSourceMode controls whether a source policy is disabled after create/clone.
type DisableSourceMode int

const (
	// DisableSourceAuto uses the operation default (clone: yes; set package: only when triggers match).
	DisableSourceAuto DisableSourceMode = iota
	// DisableSourceForce always disables the source.
	DisableSourceForce
	// DisableSourceKeep never disables the source.
	DisableSourceKeep
)

// PolicyClonePlan is a resolved clone operation ready to confirm and apply.
type PolicyClonePlan struct {
	SourceID      int
	SourceName    string
	SourceTrigger string
	NewName       string
	NewTrigger    string
	PkgID         int
	PkgName       string
	OldPkgName    string
	IsScriptOnly  bool
	DisableSource bool
	DisableReason string
	Actions       []string

	sourcePolicy *resourcePolicy
	installPay   *policyInstallPayload
	packagePay   *policyPackagePayload
	paramIndices []int
	swapIdx      int
}

// PolicyPackagePlan is a resolved set-package operation ready to confirm and apply.
type PolicyPackagePlan struct {
	SourceID      int
	SourceName    string
	SourceTrigger string
	SourceVersion string
	IsModify      bool
	NewName       string
	NewTrigger    string
	PkgID         int
	PkgName       string
	DisableSource bool
	DisableReason string
	Actions       []string

	sourcePolicy  *resourcePolicy
	createPayload *policyPackagePayload
}

// ResolveCloneDisable returns whether to disable the source and a short reason.
// Clone default is to disable; Force/Keep override.
func ResolveCloneDisable(mode DisableSourceMode) (disable bool, reason string) {
	switch mode {
	case DisableSourceKeep:
		return false, "keep-source"
	case DisableSourceForce:
		return true, "disable-source"
	default:
		return true, "clone default"
	}
}

// ResolvePackageDisable returns whether to disable the source and a short reason.
// Auto: no for in-place modify; yes only when creating with the same trigger (replacement).
func ResolvePackageDisable(isModify bool, newTrigger, sourceTrigger string, mode DisableSourceMode) (disable bool, reason string) {
	switch mode {
	case DisableSourceKeep:
		return false, "keep-source"
	case DisableSourceForce:
		return true, "disable-source"
	}
	if isModify {
		return false, "same version — in-place update"
	}
	if strings.EqualFold(newTrigger, sourceTrigger) {
		return true, "same trigger — replacement"
	}
	return false, "trigger changes"
}

// FormatDisableAction returns a confirm-plan line for the disable decision.
func FormatDisableAction(disable bool, reason string) string {
	yn := "no"
	if disable {
		yn = "yes"
	}
	return fmt.Sprintf("Disable source: %s (%s)", yn, reason)
}

// PrintConfirmActions prints a numbered action list for ConfirmPrompt.
func PrintConfirmActions(actions []string) {
	fmt.Print("\nThe following changes will be made:\n\n")
	for i, a := range actions {
		fmt.Printf("  %d. %s\n", i+1, a)
	}
}

func sourceSummaryLine(name string, id int, trigger string) string {
	if trigger != "" {
		return fmt.Sprintf("Source policy %q (ID %d, trigger %s)", name, id, trigger)
	}
	return fmt.Sprintf("Source policy %q (ID %d)", name, id)
}

// PlanPolicyClone resolves a clone (script-only or package) without writing.
// name is the package name keyword; when empty it is derived from the source trigger.
func PlanPolicyClone(credName string, opts Opts, policyID int, name, oldVersion, newVersion, packageOverride string, paramIndices []int, disableMode DisableSourceMode, logger *slog.Logger) (*PolicyClonePlan, error) {
	policy, err := policyGetByID(credName, policyID)
	if err != nil {
		return nil, fmt.Errorf(errFetchPolicyFmt, policyID, err)
	}

	pkgs := policy.PackageConfiguration.Packages
	trigger := policy.General.TriggerOther
	disable, disableReason := ResolveCloneDisable(disableMode)

	plan := &PolicyClonePlan{
		SourceID:      policy.General.ID,
		SourceName:    policy.General.Name,
		SourceTrigger: trigger,
		NewTrigger:    trigger,
		DisableSource: disable,
		DisableReason: disableReason,
		sourcePolicy:  policy,
		paramIndices:  paramIndices,
		swapIdx:       -1,
	}

	if len(pkgs) == 0 {
		if len(policy.Scripts) == 0 {
			return nil, fmt.Errorf("policy %d has no scripts and no packages", policyID)
		}
		baseName := replaceVersionInString(policy.General.Name, newVersion)
		newName, err := uniquePolicyName(credName, baseName, logger)
		if err != nil {
			return nil, err
		}
		plan.IsScriptOnly = true
		plan.NewName = newName
		plan.installPay = buildPolicyInstallPayload(policy, newName, trigger, newVersion, paramIndices)
		plan.Actions = []string{
			sourceSummaryLine(plan.SourceName, plan.SourceID, plan.SourceTrigger),
			fmt.Sprintf("Create policy %q with trigger %s", newName, trigger),
			FormatDisableAction(disable, disableReason),
		}
		return plan, nil
	}

	if name == "" {
		name, _, _ = parseTrigger(trigger)
	}
	swapIdx, detectedOldVersion, err := findNamedPackageIdx(pkgs, name, oldVersion)
	if err != nil {
		return nil, err
	}
	if oldVersion == "" {
		oldVersion = detectedOldVersion
	}
	newPkgID, newPkgName, err := findPackage(credName, name, newVersion, packageOverride, logger)
	if err != nil {
		return nil, err
	}
	if newPkgID == 0 {
		return nil, fmt.Errorf("no package found for %s version %s", name, newVersion)
	}
	baseName := replaceVersionInString(policy.General.Name, newVersion)
	newName, err := uniquePolicyName(credName, baseName, logger)
	if err != nil {
		return nil, err
	}
	plan.NewName = newName
	plan.PkgID = newPkgID
	plan.PkgName = newPkgName
	plan.OldPkgName = pkgs[swapIdx].Name
	plan.swapIdx = swapIdx
	plan.packagePay = buildPolicyPackagePayloadClone(policy, newName, trigger, swapIdx, newPkgID, newPkgName)
	plan.Actions = []string{
		sourceSummaryLine(plan.SourceName, plan.SourceID, plan.SourceTrigger),
		fmt.Sprintf("Create policy %q with trigger %s, replacing package %s with %s (ID %d)", newName, trigger, pkgs[swapIdx].Name, newPkgName, newPkgID),
		FormatDisableAction(disable, disableReason),
	}
	return plan, nil
}

// PolicyMutationOutcome describes what a package/clone apply actually did.
// Created policy IDs are filled after POST (not known at plan time).
type PolicyMutationOutcome struct {
	Kind               string // "update" or "create"
	PolicyID           int
	PolicyName         string
	Trigger            string
	PackageID          int
	PackageName        string
	DisabledSourceID   int
	DisabledSourceName string
}

// ApplyPolicyClonePlan performs the writes for a resolved clone plan.
func ApplyPolicyClonePlan(credName string, opts Opts, plan *PolicyClonePlan, logger *slog.Logger) (*PolicyMutationOutcome, error) {
	if plan == nil {
		return nil, fmt.Errorf("nil clone plan")
	}
	policy := plan.sourcePolicy
	trigger := plan.SourceTrigger
	out := &PolicyMutationOutcome{
		Kind:        "create",
		PolicyName:  plan.NewName,
		Trigger:     trigger,
		PackageID:   plan.PkgID,
		PackageName: plan.PkgName,
	}

	if plan.IsScriptOnly {
		if opts.Verbose {
			xmlPayload, _ := xml.MarshalIndent(plan.installPay, "", "  ")
			fmt.Fprintf(os.Stderr, "\n--- POST payload (JSSResource/policies/id/0) ---\n%s\n--- end payload ---\n\n", xmlPayload)
		}
		body, err := postPolicyInstall(credName, plan.installPay, logger)
		if err != nil {
			return nil, err
		}
		id, err := resolveCreatedPolicyID(credName, opts, body, plan.NewName, logger)
		if err != nil {
			return nil, err
		}
		out.PolicyID = id
		fmt.Printf("Created policy %q (ID %d) with trigger %s\n", plan.NewName, id, trigger)
		if plan.DisableSource {
			disablePayload := buildPolicyInstallPayload(policy, policy.General.Name, trigger, extractInstallSourceVersion(policy, plan.paramIndices), plan.paramIndices)
			disablePayload.General.Enabled = false
			if err := putPolicyInstall(credName, policy.General.ID, disablePayload, logger); err != nil {
				return out, fmt.Errorf("disable source policy: %w", err)
			}
			out.DisabledSourceID = policy.General.ID
			out.DisabledSourceName = policy.General.Name
			fmt.Printf("Disabled source policy %q (ID %d)\n", policy.General.Name, policy.General.ID)
		}
		return out, nil
	}

	if opts.Verbose {
		xmlPayload, _ := xml.MarshalIndent(plan.packagePay, "", "  ")
		fmt.Fprintf(os.Stderr, "\n--- POST payload (JSSResource/policies/id/0) ---\n%s\n--- end payload ---\n\n", xmlPayload)
	}
	body, err := postPolicyPackage(credName, plan.packagePay, logger)
	if err != nil {
		return nil, err
	}
	id, err := resolveCreatedPolicyID(credName, opts, body, plan.NewName, logger)
	if err != nil {
		return nil, err
	}
	out.PolicyID = id
	fmt.Printf("Created policy %q (ID %d) with trigger %s, package %s (ID %d)\n", plan.NewName, id, trigger, plan.PkgName, plan.PkgID)
	if plan.DisableSource {
		disablePayload := buildPolicyPackagePayloadClone(policy, policy.General.Name, trigger, -1, 0, "")
		disablePayload.General.Enabled = false
		if err := putPolicyPackage(credName, policy.General.ID, disablePayload, logger); err != nil {
			return out, fmt.Errorf("disable source policy: %w", err)
		}
		out.DisabledSourceID = policy.General.ID
		out.DisabledSourceName = policy.General.Name
		fmt.Printf("Disabled source policy %q (ID %d)\n", policy.General.Name, policy.General.ID)
	}
	return out, nil
}

// PolicyClone clones a policy via POST, then optionally disables the source.
// If newVersion is non-empty, updates script params (script-only) or swaps the named package.
func PolicyClone(credName string, opts Opts, policyID int, name, oldVersion, newVersion, packageOverride string, paramIndices []int, disableMode DisableSourceMode, logger *slog.Logger) error {
	plan, err := PlanPolicyClone(credName, opts, policyID, name, oldVersion, newVersion, packageOverride, paramIndices, disableMode, logger)
	if err != nil {
		return err
	}
	if !opts.SkipConfirm {
		PrintConfirmActions(plan.Actions)
		if !ConfirmPrompt("", logger) {
			return nil
		}
	}
	_, err = ApplyPolicyClonePlan(credName, opts, plan, logger)
	return err
}

// findNamedPackageIdx finds the index of the package to swap in a multi-package policy.
// It searches for a package whose filename contains name (case-insensitive).
// If oldVersion is non-empty, the package must also contain oldVersion.
// If oldVersion is empty, it is auto-detected via extractVersionFromPackageName.
// Returns the index, the detected (or provided) oldVersion, and any error.
func findNamedPackageIdx(pkgs []policySubsetPackageConfigurationPackage, name, oldVersion string) (int, string, error) {
	nameLower := strings.ToLower(name)
	for i, pkg := range pkgs {
		n := strings.ToLower(pkg.Name)
		if !strings.Contains(n, nameLower) {
			continue
		}
		if oldVersion != "" {
			if strings.Contains(n, strings.ToLower(oldVersion)) {
				return i, oldVersion, nil
			}
			continue
		}
		if detected := extractVersionFromPackageName(pkg.Name); detected != "" {
			return i, detected, nil
		}
	}
	if oldVersion != "" {
		return -1, "", fmt.Errorf("no package found matching %q with version %q", name, oldVersion)
	}
	return -1, "", fmt.Errorf("no versioned package found matching %q", name)
}

// buildPolicyPackagePayloadClone builds a policyPackagePayload from source, copying all
// packages but replacing the one at swapIdx with newPkgID/newPkgName.
// If swapIdx < 0, all packages are copied verbatim (useful for building a disable payload).
func buildPolicyPackagePayloadClone(source *resourcePolicy, name, trigger string, swapIdx, newPkgID int, newPkgName string) *policyPackagePayload {
	p := &policyPackagePayload{}
	p.General.Name = name
	p.General.Enabled = true
	p.General.Trigger = "EVENT"
	p.General.TriggerCheckin = source.General.TriggerCheckin
	p.General.TriggerEnrollmentComplete = source.General.TriggerEnrollmentComplete
	p.General.TriggerLogin = source.General.TriggerLogin
	p.General.TriggerLogout = source.General.TriggerLogout
	p.General.TriggerNetworkStateChanged = source.General.TriggerNetworkStateChanged
	p.General.TriggerStartup = source.General.TriggerStartup
	p.General.TriggerOther = trigger
	if source.General.Frequency != "" {
		p.General.Frequency = source.General.Frequency
	} else {
		p.General.Frequency = "Once per computer"
	}
	p.General.Site.ID = -1
	p.General.Site.Name = "None"
	if source.General.Category != nil {
		p.General.Category = &struct {
			ID   int    `xml:"id,omitempty"`
			Name string `xml:"name,omitempty"`
		}{ID: source.General.Category.ID, Name: source.General.Category.Name}
	}
	p.General.DateTimeLimitations = dateTimeLimitationsEmpty
	p.Scope = source.Scope
	pkgEntry := func(id int, name, action string) struct {
		ID     int    `xml:"id"`
		Name   string `xml:"name"`
		Action string `xml:"action"`
	} {
		return struct {
			ID     int    `xml:"id"`
			Name   string `xml:"name"`
			Action string `xml:"action"`
		}{ID: id, Name: name, Action: action}
	}
	packages := make([]struct {
		ID     int    `xml:"id"`
		Name   string `xml:"name"`
		Action string `xml:"action"`
	}, len(source.PackageConfiguration.Packages))
	for i, pkg := range source.PackageConfiguration.Packages {
		if i == swapIdx {
			packages[i] = pkgEntry(newPkgID, newPkgName, pkg.Action)
		} else {
			packages[i] = pkgEntry(pkg.ID, pkg.Name, pkg.Action)
		}
	}
	p.PackageConfiguration.Packages.Package = packages
	p.Scripts.Script = copyScriptsFromSource(source.Scripts)
	p.FilesProcesses = source.FilesProcesses
	p.Maintenance.Recon = source.Maintenance.Recon
	return p
}

// uniquePolicyName returns a policy name that does not exist. Base "Foo" -> "Foo 2", "Foo 3", etc.
// Always fetches live (never cached) so the uniqueness check cannot race a stale list.
func uniquePolicyName(credName string, baseName string, logger *slog.Logger) (string, error) {
	policies, err := policyFetchList(credName)
	if err != nil {
		return "", err
	}
	pat := regexp.MustCompile(`^` + regexp.QuoteMeta(baseName) + `(?: (\d+))?$`)
	maxN := 0
	hasExact := false
	for _, p := range policies {
		m := pat.FindStringSubmatch(p.Name)
		if m == nil {
			continue
		}
		if m[1] == "" {
			hasExact = true
		} else {
			var n int
			fmt.Sscanf(m[1], "%d", &n)
			if n > maxN {
				maxN = n
			}
		}
	}
	if !hasExact && maxN == 0 {
		return baseName, nil
	}
	return fmt.Sprintf("%s %d", baseName, maxN+1), nil
}

// PlanPolicyPackage resolves a set-package operation without writing.
// The source policy is selected by policyIDOverride or triggerOverride (one is
// required). name is the package name keyword used for the version search; when
// empty it is derived from the source policy trigger (install-<name>-… etc.).
func PlanPolicyPackage(credName string, opts Opts, name, newVersion string, policyIDOverride int, triggerOverride, packageOverride string, disableMode DisableSourceMode, logger *slog.Logger) (*PolicyPackagePlan, error) {
	triggerOverride = strings.TrimSpace(triggerOverride)
	if policyIDOverride <= 0 && triggerOverride != "" {
		progressf(opts, "  finding source policy by trigger %q", triggerOverride)
		progressf(opts, "    API: GET /api/v1/policies (name index), then Classic GET /JSSResource/policies/id/{id} for trigger_other")
		id, err := PolicyFind(credName, opts, triggerOverride, logger)
		if err != nil {
			return nil, err
		}
		policyIDOverride = id
		progressf(opts, "    -> policy ID %d", policyIDOverride)
	} else if policyIDOverride > 0 {
		progressf(opts, "  using pinned source policy ID %d (skip trigger search)", policyIDOverride)
		if triggerOverride != "" {
			progressf(opts, "    expected trigger %q — will verify the source is the matching cache/install role", triggerOverride)
		}
	}
	if policyIDOverride <= 0 {
		return nil, fmt.Errorf("set package requires a policy id or trigger")
	}
	progressf(opts, "  GET Classic /JSSResource/policies/id/%d (packages + trigger_other)", policyIDOverride)
	sourcePolicy, err := getPackageSourcePolicy(credName, name, policyIDOverride, opts, logger)
	if err != nil {
		return nil, err
	}
	if err := packageSourceRoleError(triggerOverride, sourcePolicy); err != nil {
		return nil, err
	}
	if name == "" {
		name, _, _ = parseTrigger(strings.ToLower(sourcePolicy.General.TriggerOther))
	}
	if name == "" && packageOverride == "" {
		return nil, fmt.Errorf("cannot derive a package name from trigger %q; pass a name or package pattern", sourcePolicy.General.TriggerOther)
	}
	curPkg := ""
	if len(sourcePolicy.PackageConfiguration.Packages) > 0 {
		curPkg = sourcePolicy.PackageConfiguration.Packages[0].Name
	}
	progressf(opts, "    source %q trigger=%q action=%s current package=%q",
		sourcePolicy.General.Name, sourcePolicy.General.TriggerOther, getPackageAction(sourcePolicy), curPkg)
	pattern := strings.TrimSpace(packageOverride)
	if pattern == "" {
		progressf(opts, "  looking up package via GET /api/v1/packages (packageName contains %q and version %q)", name, newVersion)
	} else {
		progressf(opts, "  looking up package via GET /api/v1/packages (pattern %q)", pattern)
	}
	pkgID, pkgName, err := findPackage(credName, name, newVersion, packageOverride, logger)
	if err != nil {
		return nil, err
	}
	if pkgID == 0 {
		return nil, fmt.Errorf("no package found for %s version %s", name, newVersion)
	}
	progressf(opts, "    found package %q (ID %d)", pkgName, pkgID)
	action := getPackageAction(sourcePolicy)
	if action == "" {
		return nil, fmt.Errorf("source policy has no package or invalid action")
	}
	isInstall := strings.EqualFold(action, "Install")
	sourceVersion := extractVersionFromTrigger(strings.ToLower(sourcePolicy.General.TriggerOther), name)
	if sourceVersion == "" {
		sourceVersion = extractVersionFromPackageName(sourcePolicy.PackageConfiguration.Packages[0].Name)
	}
	isModify := sourceVersion != "" && CompareVersions(sourceVersion, newVersion) == 0

	plan := &PolicyPackagePlan{
		SourceID:      sourcePolicy.General.ID,
		SourceName:    sourcePolicy.General.Name,
		SourceTrigger: sourcePolicy.General.TriggerOther,
		SourceVersion: sourceVersion,
		IsModify:      isModify,
		PkgID:         pkgID,
		PkgName:       pkgName,
		sourcePolicy:  sourcePolicy,
	}

	if isModify {
		plan.NewName = sourcePolicy.General.Name
		plan.NewTrigger = sourcePolicy.General.TriggerOther
		plan.createPayload = buildPolicyPackagePayload(sourcePolicy, plan.NewName, plan.NewTrigger, pkgID, pkgName, action)
	} else {
		_, newTrigger, newPolicyName := buildNewPackagePolicy(sourcePolicy, name, newVersion, pkgID, pkgName, isInstall)
		plan.NewName = newPolicyName
		plan.NewTrigger = newTrigger
		plan.createPayload = buildPolicyPackagePayload(sourcePolicy, newPolicyName, newTrigger, pkgID, pkgName, action)
	}

	plan.DisableSource, plan.DisableReason = ResolvePackageDisable(isModify, plan.NewTrigger, plan.SourceTrigger, disableMode)
	plan.Actions = buildPackagePlanActions(plan)
	if plan.IsModify {
		progressf(opts, "    plan: in-place update of %q → package %s", plan.NewName, plan.PkgName)
	} else {
		progressf(opts, "    plan: create %q with trigger %s, package %s (action %s)", plan.NewName, plan.NewTrigger, plan.PkgName, action)
	}
	progressf(opts, "    %s", FormatDisableAction(plan.DisableSource, plan.DisableReason))
	return plan, nil
}

func buildPackagePlanActions(plan *PolicyPackagePlan) []string {
	actions := []string{sourceSummaryLine(plan.SourceName, plan.SourceID, plan.SourceTrigger)}
	if plan.IsModify {
		actions = append(actions, fmt.Sprintf("Update policy %q (ID %d): set package to %s (ID %d)", plan.NewName, plan.SourceID, plan.PkgName, plan.PkgID))
	} else {
		actions = append(actions, fmt.Sprintf("Create policy %q with trigger %s, package %s (ID %d)", plan.NewName, plan.NewTrigger, plan.PkgName, plan.PkgID))
	}
	actions = append(actions, FormatDisableAction(plan.DisableSource, plan.DisableReason))
	return actions
}

// ApplyPolicyPackagePlan performs the writes for a resolved set-package plan.
func ApplyPolicyPackagePlan(credName string, opts Opts, plan *PolicyPackagePlan, logger *slog.Logger) (*PolicyMutationOutcome, error) {
	if plan == nil {
		return nil, fmt.Errorf("nil package plan")
	}
	out := &PolicyMutationOutcome{
		PolicyName:  plan.NewName,
		Trigger:     plan.NewTrigger,
		PackageID:   plan.PkgID,
		PackageName: plan.PkgName,
	}
	if plan.DisableSource {
		disablePayload := &policyEnablePayload{}
		disablePayload.General.Enabled = false
		if err := putPolicyGeneric(credName, plan.SourceID, disablePayload, logger); err != nil {
			return nil, fmt.Errorf("disable policy: %w", err)
		}
		out.DisabledSourceID = plan.SourceID
		out.DisabledSourceName = plan.SourceName
		fmt.Printf("Disabled source policy %q (ID %d)\n", plan.SourceName, plan.SourceID)
	}
	if opts.Verbose {
		xmlPayload, _ := xml.MarshalIndent(plan.createPayload, "", "  ")
		if plan.IsModify {
			fmt.Fprintf(os.Stderr, "\n--- PUT payload (JSSResource/policies/id/%d) ---\n%s\n--- end payload ---\n\n", plan.SourceID, xmlPayload)
		} else {
			fmt.Fprintf(os.Stderr, "\n--- POST payload (JSSResource/policies/id/0) ---\n%s\n--- end payload ---\n\n", xmlPayload)
		}
	}
	if plan.IsModify {
		if err := putPolicyPackage(credName, plan.SourceID, plan.createPayload, logger); err != nil {
			return nil, err
		}
		out.Kind = "update"
		out.PolicyID = plan.SourceID
		fmt.Printf("Updated policy %q (ID %d) with package %s (ID %d)\n", plan.NewName, plan.SourceID, plan.PkgName, plan.PkgID)
		return out, nil
	}
	body, err := postPolicyPackage(credName, plan.createPayload, logger)
	if err != nil {
		return nil, err
	}
	id, err := resolveCreatedPolicyID(credName, opts, body, plan.NewName, logger)
	if err != nil {
		return nil, err
	}
	out.Kind = "create"
	out.PolicyID = id
	fmt.Printf("Created policy %q (ID %d) with trigger %s, package %s (ID %d)\n", plan.NewName, id, plan.NewTrigger, plan.PkgName, plan.PkgID)
	return out, nil
}

// PolicyPackage updates package policy version (in-place or create + optional disable source).
func PolicyPackage(credName string, opts Opts, name, newVersion string, policyIDOverride int, triggerOverride, packageOverride string, disableMode DisableSourceMode, logger *slog.Logger) error {
	plan, err := PlanPolicyPackage(credName, opts, name, newVersion, policyIDOverride, triggerOverride, packageOverride, disableMode, logger)
	if err != nil {
		return err
	}
	if !opts.SkipConfirm {
		PrintConfirmActions(plan.Actions)
		if !ConfirmPrompt("", logger) {
			return nil
		}
	}
	_, err = ApplyPolicyPackagePlan(credName, opts, plan, logger)
	return err
}

// parsePolicyIDFromPostBody extracts a policy id from a Classic POST response body.
func parsePolicyIDFromPostBody(body []byte) int {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return 0
	}
	// XML: <policy><id>123</id>… or bare <id>123</id>
	var wrap struct {
		ID int `xml:"id"`
	}
	if err := xml.Unmarshal(body, &wrap); err == nil && wrap.ID > 0 {
		return wrap.ID
	}
	re := regexp.MustCompile(`(?i)<id>\s*(\d+)\s*</id>`)
	if m := re.FindSubmatch(body); len(m) == 2 {
		id, _ := strconv.Atoi(string(m[1]))
		return id
	}
	// JSON: {"id":123} or {"policy":{"id":123}}
	var j map[string]interface{}
	if err := json.Unmarshal(body, &j); err == nil {
		if id := jsonIntID(j["id"]); id > 0 {
			return id
		}
		if pol, ok := j["policy"].(map[string]interface{}); ok {
			if id := jsonIntID(pol["id"]); id > 0 {
				return id
			}
		}
	}
	return 0
}

func jsonIntID(v interface{}) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case string:
		id, _ := strconv.Atoi(n)
		return id
	default:
		return 0
	}
}

func resolveCreatedPolicyID(credName string, opts Opts, postBody []byte, policyName string, logger *slog.Logger) (int, error) {
	if id := parsePolicyIDFromPostBody(postBody); id > 0 {
		return id, nil
	}
	ids, err := PolicyFindByKeywords(credName, opts, []string{policyName}, nil, logger)
	if err != nil {
		return 0, fmt.Errorf("created policy %q but could not resolve ID: %w", policyName, err)
	}
	for _, id := range ids {
		pol, err := policyGetByID(credName, id)
		if err != nil {
			continue
		}
		if pol.General.Name == policyName {
			return id, nil
		}
	}
	if len(ids) == 1 {
		return ids[0], nil
	}
	return 0, fmt.Errorf("created policy %q but could not resolve its ID", policyName)
}

// PolicyGetXML fetches a policy and writes the XML PUT payload to w (typically os.Stdout).
func PolicyGetXML(credName string, policyID int, w io.Writer, logger *slog.Logger) error {
	body, err := PolicyFetchXML(credName, policyID)
	if err != nil {
		return err
	}
	_, err = w.Write(append(body, '\n'))
	return err
}

// PolicyFetchXML fetches a policy and returns the raw XML bytes (for script/component use).
func PolicyFetchXML(credName string, policyID int) ([]byte, error) {
	return jamfGETWithAccept(credName, fmt.Sprintf("%s/id/%d", uriPolicies, policyID), "application/xml")
}

// PolicySetXML reads XML from r (typically os.Stdin) and PUTs it to the policy.
// Sends the raw XML bytes directly to avoid round-trip struct conversion that can corrupt the payload.
func PolicySetXML(credName string, opts Opts, policyID int, r io.Reader, logger *slog.Logger) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read XML: %w", err)
	}
	wrote, err := policyPutXML(credName, opts, policyID, data, "\nPUT policy (ID %d) with XML from stdin", logger)
	if err != nil || !wrote {
		return err
	}
	fmt.Printf("Updated policy (ID %d)\n", policyID)
	return nil
}

// PolicyPutXML writes the given policy XML to Jamf for the given policy ID (for script/component use).
// Uses opts.SkipConfirm to skip confirmation when called from scripts. Does not print success message.
func PolicyPutXML(credName string, opts Opts, policyID int, xml []byte, logger *slog.Logger) error {
	_, err := policyPutXML(credName, opts, policyID, xml, "\nPUT policy (ID %d) with XML", logger)
	return err
}

func policyPutXML(credName string, opts Opts, policyID int, data []byte, confirmFmt string, logger *slog.Logger) (wrote bool, err error) {
	if !bytes.Contains(data, []byte("<policy")) && !bytes.Contains(data, []byte("<ResourcePolicy")) {
		return false, fmt.Errorf(errNotPolicyXML)
	}
	data = dateTimeLimitationsBlockRegex.ReplaceAll(data, []byte(dateTimeLimitationsEmptyXML))
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf(confirmFmt, policyID), logger) {
			return false, nil
		}
	}
	if _, err := jamfPUT(credName, fmt.Sprintf("%s/id/%d", uriPolicies, policyID), data, "application/xml"); err != nil {
		return false, fmt.Errorf("update policy: %w", err)
	}
	return true, nil
}

// PolicyDelete deletes one or more policies by ID.
func PolicyDelete(credName string, opts Opts, policyIDs []int, logger *slog.Logger) error {
	type policyInfo struct {
		id   int
		name string
	}
	infos := make([]policyInfo, 0, len(policyIDs))
	for _, id := range policyIDs {
		policy, err := policyGetByID(credName, id)
		if err != nil {
			return fmt.Errorf(errFetchPolicyFmt, id, err)
		}
		infos = append(infos, policyInfo{id: id, name: policy.General.Name})
	}
	if !opts.SkipConfirm {
		msg := "\nDelete policies:"
		for _, p := range infos {
			msg += fmt.Sprintf("\n  %q (ID %d)", p.name, p.id)
		}
		if !ConfirmPrompt(msg, logger) {
			return nil
		}
	}
	for _, p := range infos {
		if err := jamfDELETE(credName, fmt.Sprintf("%s/id/%d", uriPolicies, p.id)); err != nil {
			return fmt.Errorf("delete policy %d: %w", p.id, err)
		}
		invalidatePolicyListCache(credName)
		fmt.Printf("Deleted policy %q (ID %d)\n", p.name, p.id)
	}
	return nil
}

// PolicyFind finds policy ID by trigger. Returns the ID or error.
func PolicyFind(credName string, opts Opts, trigger string, logger *slog.Logger) (int, error) {
	progressf(opts, "    GET /api/v1/policies (name index for trigger %q)", trigger)
	policies, err := getPolicyList(credName, logger)
	if err != nil {
		return 0, err
	}

	nameHint, wantVer, _ := parseTrigger(trigger)
	candidates := findCandidates(policies, trigger)
	progressf(opts, "    %d policies listed; %d name candidates containing %q — Classic GET /JSSResource/policies/id/{id} for trigger_other",
		len(policies), len(candidates), nameHint)

	// Phase 1: exact trigger_other match by fetching detail for name-based candidates only.
	triggerNorm := normalizeTriggerForMatch(trigger)
	for _, c := range candidates {
		detail, err := policyGetByID(credName, c.ID)
		if err == nil && normalizeTriggerForMatch(detail.General.TriggerOther) == triggerNorm {
			progressf(opts, "    exact trigger_other match: ID %d %q", c.ID, detail.General.Name)
			return c.ID, nil
		}
	}

	// Phase 2: fall back to name-based heuristics.
	progressf(opts, "    no exact trigger_other match; name/version heuristic (prefer %s in policy name, version %q)",
		preferNameTokenForTrigger(trigger), wantVer)
	id, ok := findByNameAndVersion(policies, trigger)
	if !ok {
		hint := ""
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(trigger)), "cache-") {
			hint = `; for a cache bump, look for a policy named like "Cache …" with trigger_other matching the cache trigger — not the Install policy`
		}
		return 0, fmt.Errorf("no policy found for trigger %q%s", trigger, hint)
	}
	var pickedName string
	for _, p := range policies {
		if p.ID == id {
			pickedName = p.Name
			break
		}
	}
	progressf(opts, "    heuristic source: ID %d %q", id, pickedName)
	return id, nil
}

// PolicySummary is a short policy list entry (id + name).
type PolicySummary struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// PolicyFindSummariesByKeywords returns policies matching all keywords (case-insensitive).
// A policy matches when every keyword appears in its name, or in a hyphen-normalized form of
// the name (spaces → hyphens). When there is a single non-empty keyword, an exact custom-trigger
// match via PolicyFind is also included (so `find install-app-next` works).
// ExcludeIDs are not included in the result.
func PolicyFindSummariesByKeywords(credName string, opts Opts, keywords []string, excludeIDs []int, logger *slog.Logger) ([]PolicySummary, error) {
	policies, err := getPolicyList(credName, logger)
	if err != nil {
		return nil, err
	}

	excludeSet := make(map[int]bool)
	for _, id := range excludeIDs {
		excludeSet[id] = true
	}

	seen := make(map[int]bool)
	var result []PolicySummary
	add := func(id int, name string) {
		if excludeSet[id] || seen[id] {
			return
		}
		seen[id] = true
		result = append(result, PolicySummary{ID: id, Name: name})
	}

	var nonEmpty []string
	for _, kw := range keywords {
		k := strings.TrimSpace(kw)
		if k != "" {
			nonEmpty = append(nonEmpty, k)
		}
	}
	if len(nonEmpty) == 0 {
		for _, p := range policies {
			add(p.ID, p.Name)
		}
		return result, nil
	}

	for _, p := range policies {
		if excludeSet[p.ID] {
			continue
		}
		nameLower := strings.ToLower(p.Name)
		nameAsTrigger := normalizeTriggerForMatch(p.Name)
		matches := true
		for _, k := range nonEmpty {
			kl := strings.ToLower(k)
			kt := normalizeTriggerForMatch(k)
			if !strings.Contains(nameLower, kl) && !strings.Contains(nameAsTrigger, kt) {
				matches = false
				break
			}
		}
		if matches {
			add(p.ID, p.Name)
		}
	}

	// Single trigger-like keyword: also resolve via custom trigger (trigger_other).
	if len(nonEmpty) == 1 && looksLikeTriggerKeyword(nonEmpty[0]) {
		if id, err := PolicyFind(credName, opts, nonEmpty[0], logger); err == nil {
			name := ""
			for _, p := range policies {
				if p.ID == id {
					name = p.Name
					break
				}
			}
			if name == "" {
				if pol, err := policyGetByID(credName, id); err == nil {
					name = pol.General.Name
				}
			}
			add(id, name)
		}
	}

	return result, nil
}

func looksLikeTriggerKeyword(s string) bool {
	// Require a hyphen so bare words like "install" / "chrome" do not invoke PolicyFind heuristics.
	return strings.Contains(strings.TrimSpace(s), "-")
}

// PolicyFindByKeywords returns policy IDs matching keywords (see PolicyFindSummariesByKeywords).
func PolicyFindByKeywords(credName string, opts Opts, keywords []string, excludeIDs []int, logger *slog.Logger) ([]int, error) {
	summaries, err := PolicyFindSummariesByKeywords(credName, opts, keywords, excludeIDs, logger)
	if err != nil {
		return nil, err
	}
	ids := make([]int, len(summaries))
	for i, s := range summaries {
		ids[i] = s.ID
	}
	return ids, nil
}

func replaceVersionInString(s, newVersion string) string {
	verRe := regexp.MustCompile(`\d+\.\d+(?:\.\d+)*`)
	return verRe.ReplaceAllString(s, newVersion)
}

// --- setss payloads ---

// dateTimeLimitationsPayload is the Jamf schema for date_time_limitations.
// Used when cloning/creating policies to avoid propagating source activation/expiration datetimes.
type dateTimeLimitationsPayload struct {
	ActivationDate      string `xml:"activation_date"`
	ActivationDateEpoch int64  `xml:"activation_date_epoch"`
	ActivationDateUTC   string `xml:"activation_date_utc"`
	ExpirationDate      string `xml:"expiration_date"`
	ExpirationDateEpoch int64  `xml:"expiration_date_epoch"`
	ExpirationDateUTC   string `xml:"expiration_date_utc"`
	NoExecuteOn         string `xml:"no_execute_on"`
	NoExecuteStart      string `xml:"no_execute_start"`
	NoExecuteEnd        string `xml:"no_execute_end"`
}

// dateTimeLimitationsEmpty is the zero-value for date_time_limitations (no activation/expiration).
var dateTimeLimitationsEmpty = dateTimeLimitationsPayload{}

// dateTimeLimitationsEmptyXML is the Jamf XML for empty date_time_limitations (no activation/expiration).
// Used when stripping raw XML in PolicySetXML to avoid propagating activation/expiration.
const dateTimeLimitationsEmptyXML = `<date_time_limitations>
  <activation_date></activation_date>
  <activation_date_epoch>0</activation_date_epoch>
  <activation_date_utc></activation_date_utc>
  <expiration_date></expiration_date>
  <expiration_date_epoch>0</expiration_date_epoch>
  <expiration_date_utc></expiration_date_utc>
  <no_execute_on></no_execute_on>
  <no_execute_start></no_execute_start>
  <no_execute_end></no_execute_end>
</date_time_limitations>`

var dateTimeLimitationsBlockRegex = regexp.MustCompile(`<date_time_limitations>[\s\S]*?</date_time_limitations>`)

type policyUpdatePayload struct {
	XMLName xml.Name `xml:"policy"`
	General struct {
		Name                       string `xml:"name"`
		Enabled                    bool   `xml:"enabled"`
		Trigger                    string `xml:"trigger"`
		TriggerCheckin             bool   `xml:"trigger_checkin"`
		TriggerEnrollmentComplete  bool   `xml:"trigger_enrollment_complete"`
		TriggerLogin               bool   `xml:"trigger_login"`
		TriggerLogout              bool   `xml:"trigger_logout"`
		TriggerNetworkStateChanged bool   `xml:"trigger_network_state_changed"`
		TriggerStartup             bool   `xml:"trigger_startup"`
		TriggerOther               string `xml:"trigger_other"`
		Frequency                  string `xml:"frequency"`
		Site                       struct {
			ID   int    `xml:"id"`
			Name string `xml:"name"`
		} `xml:"site"`
	} `xml:"general"`
	Scope       policySubsetScope `xml:"scope"`
	SelfService struct {
		UseForSelfService           bool   `xml:"use_for_self_service"`
		SelfServiceDisplayName      string `xml:"self_service_display_name"`
		InstallButtonText           string `xml:"install_button_text"`
		ReinstallButtonText         string `xml:"reinstall_button_text"`
		SelfServiceDescription      string `xml:"self_service_description"`
		ForceUsersToViewDescription bool   `xml:"force_users_to_view_description"`
		FeatureOnMainPage           bool   `xml:"feature_on_main_page"`
		SelfServiceIcon             *struct {
			ID int `xml:"id"`
		} `xml:"self_service_icon,omitempty"`
	} `xml:"self_service"`
	PackageConfiguration struct {
		Packages struct {
			Package []struct {
				ID     int    `xml:"id"`
				Name   string `xml:"name"`
				Action string `xml:"action"`
			} `xml:"package"`
		} `xml:"packages"`
		DistributionPoint string `xml:"distribution_point"`
	} `xml:"package_configuration"`
	Scripts struct {
		Script []struct {
			ID          string `xml:"id"`
			Name        string `xml:"name,omitempty"`
			Priority    string `xml:"priority"`
			Parameter4  string `xml:"parameter4,omitempty"`
			Parameter5  string `xml:"parameter5,omitempty"`
			Parameter6  string `xml:"parameter6,omitempty"`
			Parameter7  string `xml:"parameter7,omitempty"`
			Parameter8  string `xml:"parameter8,omitempty"`
			Parameter9  string `xml:"parameter9,omitempty"`
			Parameter10 string `xml:"parameter10,omitempty"`
			Parameter11 string `xml:"parameter11,omitempty"`
		} `xml:"script"`
	} `xml:"scripts"`
	Maintenance struct {
		Recon bool `xml:"recon"`
	} `xml:"maintenance"`
}

func buildPolicyUpdatePayload(source *resourcePolicy, newVersion string, groupIDs []int) *policyUpdatePayload {
	p := &policyUpdatePayload{}
	p.General.Name = source.General.Name
	p.General.Enabled = source.General.Enabled
	p.General.Trigger = source.General.Trigger
	if p.General.Trigger == "" {
		p.General.Trigger = "EVENT"
	}
	p.General.TriggerCheckin = source.General.TriggerCheckin
	p.General.TriggerEnrollmentComplete = source.General.TriggerEnrollmentComplete
	p.General.TriggerLogin = source.General.TriggerLogin
	p.General.TriggerLogout = source.General.TriggerLogout
	p.General.TriggerNetworkStateChanged = source.General.TriggerNetworkStateChanged
	p.General.TriggerStartup = source.General.TriggerStartup
	p.General.TriggerOther = source.General.TriggerOther
	p.General.Frequency = source.General.Frequency
	if p.General.Frequency == "" {
		p.General.Frequency = "Once per computer"
	}
	if source.General.Site != nil {
		p.General.Site.ID = source.General.Site.ID
		p.General.Site.Name = source.General.Site.Name
	} else {
		p.General.Site.ID = -1
		p.General.Site.Name = "None"
	}
	if len(groupIDs) > 0 {
		p.Scope.AllComputers = false
		p.Scope.AllJSSUsers = false
		p.Scope.Computers = nil
		p.Scope.ComputerGroups = &[]policySubsetComputerGroup{}
		for _, id := range groupIDs {
			*p.Scope.ComputerGroups = append(*p.Scope.ComputerGroups, policySubsetComputerGroup{ID: id})
		}
		p.Scope.JSSUsers = nil
		p.Scope.JSSUserGroups = nil
		p.Scope.Buildings = nil
		p.Scope.Departments = nil
		p.Scope.Limitations = nil
		p.Scope.Exclusions = nil
	} else {
		p.Scope = source.Scope
	}
	p.SelfService.UseForSelfService = source.SelfService.UseForSelfService
	p.SelfService.SelfServiceDisplayName = source.SelfService.SelfServiceDisplayName
	p.SelfService.InstallButtonText = source.SelfService.InstallButtonText
	p.SelfService.ReinstallButtonText = source.SelfService.ReinstallButtonText
	p.SelfService.SelfServiceDescription = replaceVersionInString(source.SelfService.SelfServiceDescription, newVersion)
	p.SelfService.ForceUsersToViewDescription = source.SelfService.ForceUsersToViewDescription
	p.SelfService.FeatureOnMainPage = source.SelfService.FeatureOnMainPage
	// Omit icon to avoid "Icon specified does not exist" (409) - policy may reference a deleted icon
	for _, pkg := range source.PackageConfiguration.Packages {
		p.PackageConfiguration.Packages.Package = append(p.PackageConfiguration.Packages.Package, struct {
			ID     int    `xml:"id"`
			Name   string `xml:"name"`
			Action string `xml:"action"`
		}{ID: pkg.ID, Name: pkg.Name, Action: pkg.Action})
	}
	if source.PackageConfiguration.DistributionPoint != "" {
		p.PackageConfiguration.DistributionPoint = source.PackageConfiguration.DistributionPoint
	} else {
		p.PackageConfiguration.DistributionPoint = "default"
	}
	for _, s := range source.Scripts {
		p.Scripts.Script = append(p.Scripts.Script, struct {
			ID          string `xml:"id"`
			Name        string `xml:"name,omitempty"`
			Priority    string `xml:"priority"`
			Parameter4  string `xml:"parameter4,omitempty"`
			Parameter5  string `xml:"parameter5,omitempty"`
			Parameter6  string `xml:"parameter6,omitempty"`
			Parameter7  string `xml:"parameter7,omitempty"`
			Parameter8  string `xml:"parameter8,omitempty"`
			Parameter9  string `xml:"parameter9,omitempty"`
			Parameter10 string `xml:"parameter10,omitempty"`
			Parameter11 string `xml:"parameter11,omitempty"`
		}{
			ID: s.ID, Name: s.Name, Priority: s.Priority,
			Parameter4: s.Parameter4, Parameter5: s.Parameter5, Parameter6: s.Parameter6,
			Parameter7: s.Parameter7, Parameter8: s.Parameter8, Parameter9: s.Parameter9,
			Parameter10: s.Parameter10, Parameter11: s.Parameter11,
		})
	}
	p.Maintenance.Recon = source.Maintenance.Recon
	return p
}

func putPolicy(credName string, policyID int, payload *policyUpdatePayload, logger *slog.Logger) error {
	return putPolicyGeneric(credName, policyID, payload, logger)
}

type policyEnablePayload struct {
	XMLName xml.Name `xml:"policy"`
	General struct {
		Enabled bool `xml:"enabled"`
	} `xml:"general"`
}

type policyTriggerPayload struct {
	XMLName xml.Name `xml:"policy"`
	General struct {
		TriggerOther string `xml:"trigger_other"`
	} `xml:"general"`
}

type policyNamePayload struct {
	XMLName xml.Name `xml:"policy"`
	General struct {
		Name string `xml:"name"`
	} `xml:"general"`
}

type policyFrequencyPayload struct {
	XMLName xml.Name `xml:"policy"`
	General struct {
		Frequency string `xml:"frequency"`
	} `xml:"general"`
}

type policyIconPayload struct {
	XMLName     xml.Name `xml:"policy"`
	SelfService struct {
		UseForSelfService           bool   `xml:"use_for_self_service"`
		SelfServiceDisplayName      string `xml:"self_service_display_name"`
		InstallButtonText           string `xml:"install_button_text"`
		ReinstallButtonText         string `xml:"reinstall_button_text"`
		SelfServiceDescription      string `xml:"self_service_description"`
		ForceUsersToViewDescription bool   `xml:"force_users_to_view_description"`
		FeatureOnMainPage           bool   `xml:"feature_on_main_page"`
		SelfServiceIcon             struct {
			ID int `xml:"id"`
		} `xml:"self_service_icon"`
	} `xml:"self_service"`
}

func putPolicyGeneric(credName string, policyID int, payload interface{}, logger *slog.Logger) error {
	path := fmt.Sprintf("%s/id/%d", uriPolicies, policyID)
	body, err := xml.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal policy: %w", err)
	}
	_, err = jamfPUT(credName, path, body, "application/xml")
	return err
}

// --- setinstall payloads ---

type policyInstallPayload struct {
	XMLName xml.Name `xml:"policy"`
	General struct {
		Name                       string `xml:"name"`
		Enabled                    bool   `xml:"enabled"`
		Trigger                    string `xml:"trigger"`
		TriggerCheckin             bool   `xml:"trigger_checkin"`
		TriggerEnrollmentComplete  bool   `xml:"trigger_enrollment_complete"`
		TriggerLogin               bool   `xml:"trigger_login"`
		TriggerLogout              bool   `xml:"trigger_logout"`
		TriggerNetworkStateChanged bool   `xml:"trigger_network_state_changed"`
		TriggerStartup             bool   `xml:"trigger_startup"`
		TriggerOther               string `xml:"trigger_other"`
		Frequency                  string `xml:"frequency"`
		Category                   *struct {
			ID   int    `xml:"id,omitempty"`
			Name string `xml:"name,omitempty"`
		} `xml:"category,omitempty"`
		Site struct {
			ID   int    `xml:"id"`
			Name string `xml:"name"`
		} `xml:"site"`
		DateTimeLimitations dateTimeLimitationsPayload `xml:"date_time_limitations"`
	} `xml:"general"`
	Scope   policySubsetScope `xml:"scope"`
	Scripts struct {
		Script []scriptPayload `xml:"script"`
	} `xml:"scripts"`
	PackageConfiguration struct {
		Packages struct {
			Package []struct{} `xml:"package"`
		} `xml:"packages"`
		DistributionPoint string `xml:"distribution_point"`
	} `xml:"package_configuration"`
	Maintenance struct {
		Recon bool `xml:"recon"`
	} `xml:"maintenance"`
}

type scriptPayload struct {
	ID          string `xml:"id"`
	Name        string `xml:"name,omitempty"`
	Priority    string `xml:"priority"`
	Parameter4  string `xml:"parameter4,omitempty"`
	Parameter5  string `xml:"parameter5,omitempty"`
	Parameter6  string `xml:"parameter6,omitempty"`
	Parameter7  string `xml:"parameter7,omitempty"`
	Parameter8  string `xml:"parameter8,omitempty"`
	Parameter9  string `xml:"parameter9,omitempty"`
	Parameter10 string `xml:"parameter10,omitempty"`
	Parameter11 string `xml:"parameter11,omitempty"`
}

func getScriptParam(s *policySubsetScript, idx int) string {
	switch idx {
	case 4:
		return s.Parameter4
	case 5:
		return s.Parameter5
	case 6:
		return s.Parameter6
	case 7:
		return s.Parameter7
	case 8:
		return s.Parameter8
	case 9:
		return s.Parameter9
	case 10:
		return s.Parameter10
	case 11:
		return s.Parameter11
	default:
		return ""
	}
}

// copyScriptsFromSource copies scripts from a policy to scriptPayload slice (for package policy payloads).
func copyScriptsFromSource(scripts []policySubsetScript) []scriptPayload {
	out := make([]scriptPayload, 0, len(scripts))
	for _, s := range scripts {
		out = append(out, scriptPayload{
			ID:         s.ID,
			Name:       s.Name,
			Priority:   s.Priority,
			Parameter4: s.Parameter4, Parameter5: s.Parameter5, Parameter6: s.Parameter6,
			Parameter7: s.Parameter7, Parameter8: s.Parameter8, Parameter9: s.Parameter9,
			Parameter10: s.Parameter10, Parameter11: s.Parameter11,
		})
	}
	return out
}

func setScriptParam(s *scriptPayload, idx int, val string) {
	switch idx {
	case 4:
		s.Parameter4 = val
	case 5:
		s.Parameter5 = val
	case 6:
		s.Parameter6 = val
	case 7:
		s.Parameter7 = val
	case 8:
		s.Parameter8 = val
	case 9:
		s.Parameter9 = val
	case 10:
		s.Parameter10 = val
	case 11:
		s.Parameter11 = val
	}
}

func extractInstallSourceVersion(p *resourcePolicy, paramIndices []int) string {
	for _, script := range p.Scripts {
		for _, idx := range paramIndices {
			val := getScriptParam(&script, idx)
			if val != "" {
				verRe := regexp.MustCompile(`\d+\.\d+(?:\.\d+)*`)
				if m := verRe.FindString(val); m != "" {
					return m
				}
			}
		}
	}
	return ""
}

func replaceVersionInParams(scripts []policySubsetScript, newVersion string, paramIndices []int) []scriptPayload {
	out := make([]scriptPayload, len(scripts))
	for i, s := range scripts {
		out[i] = scriptPayload{ID: s.ID, Name: s.Name, Priority: s.Priority}
		if s.Priority == "" {
			out[i].Priority = "After"
		}
		for _, idx := range paramIndices {
			val := getScriptParam(&s, idx)
			if val != "" {
				if newVersion != "" {
					setScriptParam(&out[i], idx, replaceVersionInString(val, newVersion))
				} else {
					setScriptParam(&out[i], idx, val)
				}
			}
		}
		for j := 4; j <= 11; j++ {
			needsReplace := false
			for _, idx := range paramIndices {
				if j == idx {
					needsReplace = true
					break
				}
			}
			if !needsReplace {
				val := getScriptParam(&s, j)
				if val != "" {
					setScriptParam(&out[i], j, val)
				}
			}
		}
	}
	return out
}

func buildPolicyInstallPayload(source *resourcePolicy, name, trigger, newVersion string, paramIndices []int) *policyInstallPayload {
	p := &policyInstallPayload{}
	p.General.Name = name
	p.General.Enabled = true
	p.General.Trigger = "EVENT"
	p.General.TriggerCheckin = source.General.TriggerCheckin
	p.General.TriggerEnrollmentComplete = source.General.TriggerEnrollmentComplete
	p.General.TriggerLogin = source.General.TriggerLogin
	p.General.TriggerLogout = source.General.TriggerLogout
	p.General.TriggerNetworkStateChanged = source.General.TriggerNetworkStateChanged
	p.General.TriggerStartup = source.General.TriggerStartup
	p.General.TriggerOther = trigger
	if source.General.Frequency != "" {
		p.General.Frequency = source.General.Frequency
	} else {
		p.General.Frequency = "Once per computer"
	}
	p.General.Site.ID = -1
	p.General.Site.Name = "None"
	if source.General.Category != nil {
		p.General.Category = &struct {
			ID   int    `xml:"id,omitempty"`
			Name string `xml:"name,omitempty"`
		}{ID: source.General.Category.ID, Name: source.General.Category.Name}
	}
	p.General.DateTimeLimitations = dateTimeLimitationsEmpty
	p.Scope = source.Scope
	p.Scripts.Script = replaceVersionInParams(source.Scripts, newVersion, paramIndices)
	if source.PackageConfiguration.DistributionPoint != "" {
		p.PackageConfiguration.DistributionPoint = source.PackageConfiguration.DistributionPoint
	} else {
		p.PackageConfiguration.DistributionPoint = "default"
	}
	p.Maintenance.Recon = source.Maintenance.Recon
	return p
}

func putPolicyInstall(credName string, policyID int, payload *policyInstallPayload, logger *slog.Logger) error {
	path := fmt.Sprintf("%s/id/%d", uriPolicies, policyID)
	body, err := xml.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal policy: %w", err)
	}
	_, err = jamfPUT(credName, path, body, "application/xml")
	return err
}

func postPolicyInstall(credName string, payload *policyInstallPayload, logger *slog.Logger) ([]byte, error) {
	path := uriPolicies + "/id/0"
	body, err := xml.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal policy: %w", err)
	}
	resp, err := jamfPOST(credName, path, body, "application/xml")
	if err == nil {
		invalidatePolicyListCache(credName)
	}
	return resp, err
}

// --- setpackage ---

type policyPackagePayload struct {
	XMLName xml.Name `xml:"policy"`
	General struct {
		Name                       string `xml:"name"`
		Enabled                    bool   `xml:"enabled"`
		Trigger                    string `xml:"trigger"`
		TriggerCheckin             bool   `xml:"trigger_checkin"`
		TriggerEnrollmentComplete  bool   `xml:"trigger_enrollment_complete"`
		TriggerLogin               bool   `xml:"trigger_login"`
		TriggerLogout              bool   `xml:"trigger_logout"`
		TriggerNetworkStateChanged bool   `xml:"trigger_network_state_changed"`
		TriggerStartup             bool   `xml:"trigger_startup"`
		TriggerOther               string `xml:"trigger_other"`
		Frequency                  string `xml:"frequency"`
		Category                   *struct {
			ID   int    `xml:"id,omitempty"`
			Name string `xml:"name,omitempty"`
		} `xml:"category,omitempty"`
		Site struct {
			ID   int    `xml:"id"`
			Name string `xml:"name"`
		} `xml:"site"`
		DateTimeLimitations dateTimeLimitationsPayload `xml:"date_time_limitations"`
	} `xml:"general"`
	Scope                policySubsetScope `xml:"scope"`
	PackageConfiguration struct {
		Packages struct {
			Package []struct {
				ID     int    `xml:"id"`
				Name   string `xml:"name"`
				Action string `xml:"action"`
			} `xml:"package"`
		} `xml:"packages"`
	} `xml:"package_configuration"`
	Scripts struct {
		Script []scriptPayload `xml:"script"`
	} `xml:"scripts,omitempty"`
	FilesProcesses policySubsetFilesProcesses `xml:"files_processes,omitempty"`
	Maintenance    struct {
		Recon bool `xml:"recon"`
	} `xml:"maintenance"`
}

// getPackageSourcePolicy fetches and validates the source Install/Cache policy by ID.
// When name is non-empty, the policy must reference it (trigger, name, or package name)
// as a typo guard.
func getPackageSourcePolicy(credName, name string, policyID int, opts Opts, logger *slog.Logger) (*resourcePolicy, error) {
	detail, err := policyGetByID(credName, policyID)
	if err != nil {
		return nil, fmt.Errorf(errFetchPolicyFmt, policyID, err)
	}
	if len(detail.PackageConfiguration.Packages) != 1 {
		return nil, fmt.Errorf("policy %d has %d packages (expected 1)", policyID, len(detail.PackageConfiguration.Packages))
	}
	action := detail.PackageConfiguration.Packages[0].Action
	if !strings.EqualFold(action, "Install") && !strings.EqualFold(action, "Cache") {
		return nil, fmt.Errorf("policy %d package action is %q (expected Install or Cache)", policyID, action)
	}
	if name != "" {
		trigger := strings.ToLower(detail.General.TriggerOther)
		polName := strings.ToLower(detail.General.Name)
		pkgName := strings.ToLower(detail.PackageConfiguration.Packages[0].Name)
		if !strings.Contains(trigger, name) && !strings.Contains(polName, name) && !strings.Contains(pkgName, name) {
			return nil, fmt.Errorf("policy %d does not appear to match %q", policyID, name)
		}
	}
	return detail, nil
}

func extractVersionFromTrigger(trigger, name string) string {
	_, ver, _ := parseTrigger(strings.ToLower(strings.TrimSpace(trigger)))
	if ver != "" {
		return ver
	}
	if name == "" {
		return ""
	}
	re := regexp.MustCompile(`(?:cache|install)-` + regexp.QuoteMeta(strings.ToLower(name)) + `-(\d+\.\d+(?:\.\d+)?)`)
	m := re.FindStringSubmatch(strings.ToLower(trigger))
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

// packageSourceRoleError fails when from_trigger is a cache/install selector but
// the resolved source policy is clearly the other role (e.g. cache trigger → Install policy).
func packageSourceRoleError(wantTrigger string, source *resourcePolicy) error {
	wantTrigger = strings.TrimSpace(wantTrigger)
	if wantTrigger == "" || source == nil {
		return nil
	}
	wantCache := looksLikeCacheTrigger(wantTrigger)
	wantInstall := looksLikeInstallTrigger(wantTrigger) && !wantCache
	if !wantCache && !wantInstall {
		return nil
	}
	gotCache := sourceLooksLikeCache(source)
	action := getPackageAction(source)
	if wantCache && !gotCache {
		return fmt.Errorf("from_trigger %q is a cache selector, but source policy %q (ID %d) looks like an install policy (trigger %q, action %s); pin the cache policy id or set cache_trigger to the versioned cache policy",
			wantTrigger, source.General.Name, source.General.ID, source.General.TriggerOther, action)
	}
	if wantInstall && gotCache {
		return fmt.Errorf("from_trigger %q is an install selector, but source policy %q (ID %d) looks like a cache policy (trigger %q, action %s)",
			wantTrigger, source.General.Name, source.General.ID, source.General.TriggerOther, action)
	}
	return nil
}

func sourceLooksLikeCache(p *resourcePolicy) bool {
	if p == nil {
		return false
	}
	if looksLikeCacheTrigger(p.General.TriggerOther) {
		return true
	}
	n := strings.ToLower(p.General.Name)
	if strings.Contains(n, "cache") && !strings.Contains(n, "install") {
		return true
	}
	return strings.EqualFold(getPackageAction(p), "Cache")
}

// extractVersionFromPackageName extracts a dotted version from a package filename.
// Delegates to ExtractVersion.
func extractVersionFromPackageName(pkgName string) string {
	return ExtractVersion(pkgName)
}

// findPackage looks up a package by name keyword and version via PackageFindID (v1 API).
// When patternOverride is non-empty, it overrides the default name+version search.
// Returns (0, "", nil) when no matching package is found.
func findPackage(credName, name, version, patternOverride string, logger *slog.Logger) (int, string, error) {
	idStr, pkgName, err := PackageFindID(credName, name, version, patternOverride, logger)
	if err != nil {
		return 0, "", err
	}
	if idStr == "" {
		return 0, "", nil
	}
	var id int
	fmt.Sscanf(idStr, "%d", &id)
	return id, pkgName, nil
}

func getPackageAction(p *resourcePolicy) string {
	if len(p.PackageConfiguration.Packages) == 0 {
		return ""
	}
	return p.PackageConfiguration.Packages[0].Action
}

func findExistingInstallNext(credName, name string, opts Opts, logger *slog.Logger) (*resourcePolicy, error) {
	policies, err := getPolicyList(credName, logger)
	if err != nil {
		return nil, err
	}

	targetTrigger := "install-" + name + "-next"
	// Filter by policy name containing the keyword or "next" to reduce fetches
	var nameCandidates []policyListItem
	nameLower := strings.ToLower(name)
	for _, p := range policies {
		n := strings.ToLower(p.Name)
		if strings.Contains(n, nameLower) || strings.Contains(n, "next") {
			nameCandidates = append(nameCandidates, p)
		}
	}

	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "  Looking for install-%s-next (%d candidates)...\n", name, len(nameCandidates))
	}

	for i, p := range nameCandidates {
		if opts.Verbose && (i%20 == 0 || i == len(nameCandidates)-1) {
			fmt.Fprintf(os.Stderr, "  checking policy %d/%d (ID %d)...\n", i+1, len(nameCandidates), p.ID)
		}
		detail, err := policyGetByID(credName, p.ID)
		if err != nil {
			continue
		}
		if strings.EqualFold(detail.General.TriggerOther, targetTrigger) && detail.General.Enabled {
			return detail, nil
		}
	}
	return nil, nil
}

func replaceVersionInName(name, newVersion string) string {
	verRe := regexp.MustCompile(`\d+\.\d+(?:\.\d+)*`)
	return verRe.ReplaceAllString(name, newVersion)
}

func buildNewPackagePolicy(source *resourcePolicy, name, newVersion string, pkgID int, pkgName string, isInstall bool) (*resourcePolicy, string, string) {
	var newTrigger string
	srcTrig := strings.ToLower(source.General.TriggerOther)
	sourceTriggerVersion := extractVersionFromTrigger(srcTrig, name)
	if sourceTriggerVersion != "" {
		// Source has version in trigger (e.g. cache-example-7.29) — versioned trigger for the new policy.
		// Prefer the source trigger/action role over isInstall so a cache policy is never rewritten as install-*.
		if looksLikeCacheTrigger(srcTrig) || sourceLooksLikeCache(source) || !isInstall {
			newTrigger = fmt.Sprintf("cache-%s-%s", name, newVersion)
		} else {
			newTrigger = fmt.Sprintf("install-%s-%s", name, newVersion)
		}
	} else {
		// Source trigger has no version (e.g. example-install) - keep same trigger for the replacement
		newTrigger = source.General.TriggerOther
	}
	newPolicyName := replaceVersionInName(source.General.Name, newVersion)
	return nil, newTrigger, newPolicyName
}

func buildPolicyPackagePayload(source *resourcePolicy, name, trigger string, pkgID int, pkgName, action string) *policyPackagePayload {
	p := &policyPackagePayload{}
	p.General.Name = name
	p.General.Enabled = true
	p.General.Trigger = "EVENT"
	p.General.TriggerCheckin = source.General.TriggerCheckin
	p.General.TriggerEnrollmentComplete = source.General.TriggerEnrollmentComplete
	p.General.TriggerLogin = source.General.TriggerLogin
	p.General.TriggerLogout = source.General.TriggerLogout
	p.General.TriggerNetworkStateChanged = source.General.TriggerNetworkStateChanged
	p.General.TriggerStartup = source.General.TriggerStartup
	p.General.TriggerOther = trigger
	if source.General.Frequency != "" {
		p.General.Frequency = source.General.Frequency
	} else {
		p.General.Frequency = "Once per computer"
	}
	p.General.Site.ID = -1
	p.General.Site.Name = "None"
	if source.General.Category != nil {
		p.General.Category = &struct {
			ID   int    `xml:"id,omitempty"`
			Name string `xml:"name,omitempty"`
		}{ID: source.General.Category.ID, Name: source.General.Category.Name}
	}
	p.General.DateTimeLimitations = dateTimeLimitationsEmpty
	p.Scope = source.Scope
	p.PackageConfiguration.Packages.Package = []struct {
		ID     int    `xml:"id"`
		Name   string `xml:"name"`
		Action string `xml:"action"`
	}{{ID: pkgID, Name: pkgName, Action: action}}
	p.Scripts.Script = copyScriptsFromSource(source.Scripts)
	p.FilesProcesses = source.FilesProcesses
	p.Maintenance.Recon = source.Maintenance.Recon
	return p
}

func putPolicyPackage(credName string, policyID int, payload *policyPackagePayload, logger *slog.Logger) error {
	path := fmt.Sprintf("%s/id/%d", uriPolicies, policyID)
	body, err := xml.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal policy: %w", err)
	}
	_, err = jamfPUT(credName, path, body, "application/xml")
	return err
}

func postPolicyPackage(credName string, payload *policyPackagePayload, logger *slog.Logger) ([]byte, error) {
	path := uriPolicies + "/id/0"
	body, err := xml.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal policy: %w", err)
	}
	resp, err := jamfPOST(credName, path, body, "application/xml")
	if err == nil {
		invalidatePolicyListCache(credName)
	}
	return resp, err
}

// buildPolicyPackagePayloadFromScratch builds a minimal policyPackagePayload without a source policy.
func buildPolicyPackagePayloadFromScratch(name, trigger string, pkgID int, pkgName, action string, scopeAll bool, scopeGroupIDs []int, categoryID int, categoryName string) *policyPackagePayload {
	p := &policyPackagePayload{}
	p.General.Name = name
	p.General.Enabled = true
	p.General.Trigger = "EVENT"
	p.General.TriggerOther = trigger
	p.General.Frequency = "Once per computer"
	p.General.Site.ID = -1
	p.General.Site.Name = "None"
	if categoryID != 0 && categoryName != "" {
		p.General.Category = &struct {
			ID   int    `xml:"id,omitempty"`
			Name string `xml:"name,omitempty"`
		}{ID: categoryID, Name: categoryName}
	}
	p.General.DateTimeLimitations = dateTimeLimitationsEmpty
	if scopeAll {
		p.Scope.AllComputers = true
		p.Scope.ComputerGroups = nil
	} else if len(scopeGroupIDs) > 0 {
		p.Scope.AllComputers = false
		p.Scope.ComputerGroups = &[]policySubsetComputerGroup{}
		for _, id := range scopeGroupIDs {
			*p.Scope.ComputerGroups = append(*p.Scope.ComputerGroups, policySubsetComputerGroup{ID: id})
		}
	} else {
		p.Scope.AllComputers = true
	}
	p.PackageConfiguration.Packages.Package = []struct {
		ID     int    `xml:"id"`
		Name   string `xml:"name"`
		Action string `xml:"action"`
	}{{ID: pkgID, Name: pkgName, Action: action}}
	p.Maintenance.Recon = false
	return p
}

// PolicyCreatePackagePlan is a resolved greenfield package policy create (no source policy).
type PolicyCreatePackagePlan struct {
	Name          string
	Version       string
	Action        string // "install" or "cache"
	NewName       string
	NewTrigger    string
	PkgID         int
	PkgName       string
	ScopeAll      bool
	ScopeGroupIDs []int
	Category      string
	Actions       []string
	createPayload *policyPackagePayload
}

// PlanPolicyCreatePackage resolves package + unique name/trigger for a from-scratch create.
func PlanPolicyCreatePackage(credName string, opts Opts, name, version, action string, scopeAll bool, scopeGroupIDs []int, category, packageOverride string, logger *slog.Logger) (*PolicyCreatePackagePlan, error) {
	pkgIDStr, pkgName, err := PackageFindID(credName, name, version, packageOverride, logger)
	if err != nil {
		return nil, err
	}
	if pkgIDStr == "" {
		return nil, fmt.Errorf("no package found for %s version %s", name, version)
	}
	var pkgID int
	if _, err := fmt.Sscanf(pkgIDStr, "%d", &pkgID); err != nil {
		return nil, fmt.Errorf("invalid package ID: %w", err)
	}
	actionNorm := strings.ToLower(strings.TrimSpace(action))
	if actionNorm == "" {
		actionNorm = "install"
	}
	if actionNorm != "install" && actionNorm != "cache" {
		return nil, fmt.Errorf("action must be install or cache, got %q", action)
	}
	actionAPI := "Install"
	if actionNorm == "cache" {
		actionAPI = "Cache"
	}
	title := name
	if r := []rune(name); len(r) > 0 {
		r[0] = unicode.ToUpper(r[0])
		title = string(r)
	}
	var trigger, baseName string
	if actionNorm == "install" {
		trigger = "install-" + strings.ToLower(name) + "-next"
		baseName = "Install " + title + " " + version
	} else {
		trigger = fmt.Sprintf("cache-%s-%s", strings.ToLower(name), version)
		baseName = "Cache " + title + " " + version
	}
	policyName, err := uniquePolicyName(credName, baseName, logger)
	if err != nil {
		return nil, err
	}
	var catID int
	var catName string
	if category != "" {
		idStr, err := categoryGetByName(credName, category)
		if err != nil {
			return nil, fmt.Errorf("look up category %q: %w", category, err)
		}
		catID, _ = strconv.Atoi(idStr)
		catName = category
	}
	if !scopeAll && len(scopeGroupIDs) == 0 {
		scopeAll = true
	}
	payload := buildPolicyPackagePayloadFromScratch(policyName, trigger, pkgID, pkgName, actionAPI, scopeAll, scopeGroupIDs, catID, catName)
	plan := &PolicyCreatePackagePlan{
		Name:          name,
		Version:       version,
		Action:        actionNorm,
		NewName:       policyName,
		NewTrigger:    trigger,
		PkgID:         pkgID,
		PkgName:       pkgName,
		ScopeAll:      scopeAll,
		ScopeGroupIDs: scopeGroupIDs,
		Category:      category,
		createPayload: payload,
	}
	scopeDesc := "all computers"
	if !scopeAll && len(scopeGroupIDs) > 0 {
		scopeDesc = fmt.Sprintf("groups %v", scopeGroupIDs)
	}
	plan.Actions = []string{
		fmt.Sprintf("Create %s policy %q with trigger %s", actionNorm, policyName, trigger),
		fmt.Sprintf("Package: %s (ID %d)", pkgName, pkgID),
		fmt.Sprintf("Scope: %s", scopeDesc),
	}
	return plan, nil
}

// ApplyPolicyCreatePackagePlan POSTs a greenfield package policy and returns the created ID.
func ApplyPolicyCreatePackagePlan(credName string, opts Opts, plan *PolicyCreatePackagePlan, logger *slog.Logger) (*PolicyMutationOutcome, error) {
	if plan == nil || plan.createPayload == nil {
		return nil, fmt.Errorf("nil create-package plan")
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nCreate policy %q with trigger %q, package %s (ID %d)", plan.NewName, plan.NewTrigger, plan.PkgName, plan.PkgID), logger) {
			return nil, nil
		}
	}
	if opts.Verbose {
		xmlPayload, _ := xml.MarshalIndent(plan.createPayload, "", "  ")
		fmt.Fprintf(os.Stderr, "\n--- POST payload (JSSResource/policies/id/0) ---\n%s\n--- end payload ---\n\n", xmlPayload)
	}
	body, err := postPolicyPackage(credName, plan.createPayload, logger)
	if err != nil {
		return nil, err
	}
	id, err := resolveCreatedPolicyID(credName, opts, body, plan.NewName, logger)
	if err != nil {
		return nil, err
	}
	out := &PolicyMutationOutcome{
		Kind:        "create",
		PolicyID:    id,
		PolicyName:  plan.NewName,
		Trigger:     plan.NewTrigger,
		PackageID:   plan.PkgID,
		PackageName: plan.PkgName,
	}
	fmt.Printf("Created policy %q (ID %d) with trigger %s, package %s (ID %d)\n", plan.NewName, id, plan.NewTrigger, plan.PkgName, plan.PkgID)
	return out, nil
}

// PolicyCreatePackage creates a new package policy from scratch (no source policy required).
func PolicyCreatePackage(credName string, opts Opts, name, version string, action string, scopeAll bool, scopeGroupIDs []int, category, packageOverride string, logger *slog.Logger) (*PolicyMutationOutcome, error) {
	plan, err := PlanPolicyCreatePackage(credName, opts, name, version, action, scopeAll, scopeGroupIDs, category, packageOverride, logger)
	if err != nil {
		return nil, err
	}
	return ApplyPolicyCreatePackagePlan(credName, opts, plan, logger)
}

// --- create ss ---

type policySelfServicePayload struct {
	XMLName xml.Name `xml:"policy"`
	General struct {
		Name         string `xml:"name"`
		Enabled      bool   `xml:"enabled"`
		Trigger      string `xml:"trigger"`
		TriggerOther string `xml:"trigger_other"`
		Frequency    string `xml:"frequency"`
		Category     *struct {
			ID   int    `xml:"id,omitempty"`
			Name string `xml:"name,omitempty"`
		} `xml:"category,omitempty"`
		Site struct {
			ID   int    `xml:"id"`
			Name string `xml:"name"`
		} `xml:"site"`
	} `xml:"general"`
	Scope       policySubsetScope `xml:"scope"`
	SelfService struct {
		UseForSelfService           bool   `xml:"use_for_self_service"`
		SelfServiceDisplayName      string `xml:"self_service_display_name"`
		InstallButtonText           string `xml:"install_button_text"`
		ReinstallButtonText         string `xml:"reinstall_button_text"`
		SelfServiceDescription      string `xml:"self_service_description"`
		ForceUsersToViewDescription bool   `xml:"force_users_to_view_description"`
		FeatureOnMainPage           bool   `xml:"feature_on_main_page"`
	} `xml:"self_service"`
	Scripts struct {
		Script []scriptPayload `xml:"script"`
	} `xml:"scripts"`
	PackageConfiguration struct {
		DistributionPoint string `xml:"distribution_point"`
	} `xml:"package_configuration"`
	Maintenance struct {
		Recon bool `xml:"recon"`
	} `xml:"maintenance"`
}

// PolicyCreateSS creates a Self Service policy using the given trigger as script parameter4
// when scriptID > 0 (Jamf script numeric ID). scriptID 0 means no script payloads on the policy.
//
// If scopeGroupID > 0, the policy is scoped to that computer group ID.
// Otherwise, it is scoped to all computers.
func PolicyCreateSS(credName string, opts Opts, trigger, displayName, category string, scopeGroupID int, scriptID int, logger *slog.Logger) (int, error) {
	policyName := displayName + " [SS]"

	p := &policySelfServicePayload{}
	p.General.Name = policyName
	p.General.Enabled = true
	p.General.Trigger = "USER_INITIATED"
	p.General.TriggerOther = ""
	p.General.Frequency = "Ongoing"
	p.General.Site.ID = -1
	p.General.Site.Name = "None"

	if category != "" {
		idStr, err := categoryGetByName(credName, category)
		if err != nil {
			return 0, fmt.Errorf("look up category %q: %w", category, err)
		}
		catID, _ := strconv.Atoi(idStr)
		p.General.Category = &struct {
			ID   int    `xml:"id,omitempty"`
			Name string `xml:"name,omitempty"`
		}{ID: catID, Name: category}
	}

	if scopeGroupID > 0 {
		// Scope to a specific group.
		p.Scope.AllComputers = false
		p.Scope.ComputerGroups = &[]policySubsetComputerGroup{{ID: scopeGroupID}}
	} else {
		// Scope to all computers (portable default for scripts/tests).
		p.Scope.AllComputers = true
		p.Scope.ComputerGroups = nil
	}

	// Self Service configuration.
	p.SelfService.UseForSelfService = true
	p.SelfService.SelfServiceDisplayName = displayName
	p.SelfService.InstallButtonText = "Install"
	p.SelfService.ReinstallButtonText = "Reinstall"
	p.SelfService.SelfServiceDescription = fmt.Sprintf(
		"**%s** is available for installation via Self Service.\r\n\r\nVersion: (set via policy trigger)",
		displayName,
	)
	p.SelfService.ForceUsersToViewDescription = true
	p.SelfService.FeatureOnMainPage = false

	// Optional script with trigger as parameter4.
	if scriptID > 0 {
		p.Scripts.Script = []scriptPayload{{
			ID:         strconv.Itoa(scriptID),
			Priority:   "After",
			Parameter4: trigger,
		}}
	}

	p.PackageConfiguration.DistributionPoint = "default"
	p.Maintenance.Recon = true

	if !opts.SkipConfirm {
		scopeMsg := "all computers"
		if scopeGroupID > 0 {
			scopeMsg = fmt.Sprintf("computer group %d", scopeGroupID)
		}
		if !ConfirmPrompt(fmt.Sprintf("\nCreate Self Service policy %q (trigger param: %s, scope: %s)", policyName, trigger, scopeMsg), logger) {
			return 0, nil
		}
	}
	if opts.Verbose {
		xmlPayload, _ := xml.MarshalIndent(p, "", "  ")
		fmt.Fprintf(os.Stderr, "\n--- POST payload (JSSResource/policies/id/0) ---\n%s\n--- end payload ---\n\n", xmlPayload)
	}
	if err := postPolicyGeneric(credName, p, logger); err != nil {
		return 0, err
	}
	ids, err := PolicyFindByKeywords(credName, opts, []string{policyName}, nil, logger)
	if err != nil {
		return 0, fmt.Errorf("created policy but could not resolve ID: %w", err)
	}
	if len(ids) == 0 {
		return 0, fmt.Errorf("created policy %q but could not find it in policy list", policyName)
	}
	fmt.Printf("Created Self Service policy %q (trigger param: %s)\n", policyName, trigger)
	return ids[0], nil
}

func postPolicyGeneric(credName string, payload interface{}, logger *slog.Logger) error {
	path := uriPolicies + "/id/0"
	body, err := xml.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal policy: %w", err)
	}
	_, err = jamfPOST(credName, path, body, "application/xml")
	if err == nil {
		invalidatePolicyListCache(credName)
	}
	return err
}

// --- findpolicy ---

// The policy {id,name} list is cached in process memory per resolved session
// (tenant+auth), with a short TTL so long-lived processes (MCP, serve) pick up
// external changes. Creates/deletes in this process invalidate the entry.
const policyListCacheTTL = 60 * time.Second

type policyListItem struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	// Optional fields from undocumented Pro GET /api/v1/policies (nil/empty on Classic list).
	Enabled            *bool  `json:"enabled,omitempty"`
	CategoryID         string `json:"categoryId,omitempty"`
	ExecutionFrequency string `json:"executionFrequency,omitempty"`
	TriggerCheckin     *bool  `json:"triggerCheckin,omitempty"`
	// TriggerManual is set (non-nil) only when the v1 list included triggerManual
	// (often "" for Self Service / single policies). Classic list leaves it nil.
	TriggerManual   *string `json:"triggerManual,omitempty"`
	UpdateInventory *bool   `json:"updateInventory,omitempty"`
}

type policyListCacheEntry struct {
	fetched  time.Time
	policies []policyListItem
}

var policyListCache = struct {
	mu      sync.Mutex
	entries map[string]policyListCacheEntry
}{entries: map[string]policyListCacheEntry{}}

// policyListCacheKey resolves the session the same way the HTTP layer will,
// so the cache is isolated per tenant+auth. ok=false disables caching.
func policyListCacheKey(credName string) (string, bool) {
	sess, err := ResolveSessionForCred(credName, OpClassic)
	if err != nil {
		return "", false
	}
	return sess.AuthName + "@" + sess.TenantName, true
}

// getPolicyList returns the policy {id,name} list for the resolved session,
// served from the process cache when fresh (see policyListCacheTTL).
func getPolicyList(credName string, logger *slog.Logger) ([]policyListItem, error) {
	key, ok := policyListCacheKey(credName)
	if ok {
		policyListCache.mu.Lock()
		ent, hit := policyListCache.entries[key]
		policyListCache.mu.Unlock()
		if hit && time.Since(ent.fetched) < policyListCacheTTL {
			return ent.policies, nil
		}
	}
	policies, err := policyFetchListHybrid(credName, logger)
	if err != nil {
		return nil, err
	}
	if ok {
		policyListCache.mu.Lock()
		policyListCache.entries[key] = policyListCacheEntry{fetched: time.Now(), policies: policies}
		policyListCache.mu.Unlock()
	}
	return policies, nil
}

// invalidatePolicyListCache drops the cached list for the resolved session.
// Called after policy create/delete so trigger and keyword lookups see the change.
func invalidatePolicyListCache(credName string) {
	key, ok := policyListCacheKey(credName)
	if !ok {
		return
	}
	policyListCache.mu.Lock()
	delete(policyListCache.entries, key)
	policyListCache.mu.Unlock()
}

// normalizeTriggerForMatch normalizes trigger strings for comparison.
// Jamf may store trigger_other with spaces (e.g. "Install ExampleApp Next") while
// users pass hyphens (e.g. "install-cursor-next"). Both are normalized to
// lowercase with spaces and repeated hyphens collapsed to single hyphens.
func normalizeTriggerForMatch(t string) string {
	s := strings.ToLower(strings.TrimSpace(t))
	// Replace spaces with hyphens
	s = strings.ReplaceAll(s, " ", "-")
	// Collapse multiple hyphens
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}

func parseTrigger(trigger string) (name string, version string, isNext bool) {
	t := strings.ToLower(strings.TrimSpace(trigger))

	// Pre-compile regexes so they are available in both the -next and plain branches.
	reVersion := regexp.MustCompile(`^(?:cache|install)-(.+)-(\d+\.\d+(?:\.\d+)*)$`)
	reInstallCache := regexp.MustCompile(`^(?:install|cache)-(.+)$`)
	// Reversed format: <name>-<verb>  e.g. "filebeat-install", "filebeat-cache"
	reNameVerb := regexp.MustCompile(`^(.+)-(?:install|cache)$`)

	if strings.HasSuffix(t, "-next") {
		remainder := strings.TrimSuffix(t, "-next")
		// install-{name}-{version}-next
		if m := reVersion.FindStringSubmatch(remainder); len(m) >= 3 {
			return m[1], m[2], true
		}
		// install-{name}-next
		if m := reInstallCache.FindStringSubmatch(remainder); len(m) >= 2 {
			return m[1], "", true
		}
		// {name}-install-next
		if m := reNameVerb.FindStringSubmatch(remainder); len(m) >= 2 {
			return m[1], "", true
		}
		parts := strings.SplitN(remainder, "-", 2)
		if len(parts) >= 2 {
			return parts[1], "", true
		}
		return remainder, "", true
	}
	if m := reVersion.FindStringSubmatch(t); len(m) >= 3 {
		return m[1], m[2], false
	}
	if m := reInstallCache.FindStringSubmatch(t); len(m) >= 2 {
		return m[1], "", false
	}
	if m := reNameVerb.FindStringSubmatch(t); len(m) >= 2 {
		return m[1], "", false
	}
	return t, "", false
}

var versionInNameRe = regexp.MustCompile(`\d+\.\d+(?:\.\d+)*`)

func versionFromName(name string) string {
	return versionInNameRe.FindString(name)
}

// findCandidates returns all policies whose name contains the keyword derived from trigger.
func findCandidates(policies []policyListItem, trigger string) []policyListItem {
	name, _, _ := parseTrigger(trigger)
	nameLower := strings.ToLower(name)
	var candidates []policyListItem
	for _, p := range policies {
		if strings.Contains(strings.ToLower(p.Name), nameLower) {
			candidates = append(candidates, p)
		}
	}
	return candidates
}

func preferNameTokenForTrigger(trigger string) string {
	triggerLower := strings.ToLower(strings.TrimSpace(trigger))
	if strings.HasPrefix(triggerLower, "cache-") || strings.HasSuffix(triggerLower, "-cache") {
		return "cache"
	}
	if strings.HasPrefix(triggerLower, "install-") || strings.HasSuffix(strings.TrimSuffix(triggerLower, "-next"), "-install") {
		return "install"
	}
	return ""
}

func findByNameAndVersion(policies []policyListItem, trigger string) (int, bool) {
	triggerLower := strings.ToLower(strings.TrimSpace(trigger))
	_, wantVersion, isNext := parseTrigger(trigger)
	preferInName := preferNameTokenForTrigger(trigger)
	candidates := findCandidates(policies, trigger)
	if len(candidates) == 0 {
		return 0, false
	}
	if preferInName != "" {
		var roleHits []policyListItem
		for _, p := range candidates {
			if strings.Contains(strings.ToLower(p.Name), preferInName) {
				roleHits = append(roleHits, p)
			}
		}
		if len(roleHits) > 0 {
			candidates = roleHits
		} else if preferInName == "cache" {
			// Never fall back to an Install policy when the selector is a cache trigger.
			return 0, false
		}
	}
	if len(candidates) == 1 && !isNext && wantVersion == "" {
		return candidates[0].ID, true
	}
	if isNext {
		if strings.HasPrefix(triggerLower, "install-") {
			for _, p := range candidates {
				n := strings.ToLower(p.Name)
				if strings.Contains(n, "install") && strings.Contains(n, "next") {
					return p.ID, true
				}
			}
		}
		for _, p := range candidates {
			n := strings.ToLower(p.Name)
			if strings.Contains(n, "next") {
				return p.ID, true
			}
		}
		for _, p := range candidates {
			n := strings.ToLower(p.Name)
			if strings.Contains(n, "install") {
				return p.ID, true
			}
		}
		return candidates[0].ID, true
	}
	if wantVersion != "" {
		for _, p := range candidates {
			if versionFromName(p.Name) == wantVersion {
				return p.ID, true
			}
		}
	}
	if preferInName != "" {
		for _, p := range candidates {
			if strings.Contains(strings.ToLower(p.Name), preferInName) {
				return p.ID, true
			}
		}
	}
	return candidates[0].ID, true
}

// ---------------------------------------------------------------------------
// PolicyCreateFromTemplate
// ---------------------------------------------------------------------------

// PolicyTemplate describes a Jamf policy to be created from scratch.
// All string fields support {version} substitution at execution time.
type PolicyTemplate struct {
	Name              string
	Trigger           string
	ScriptID          int
	ScriptPriority    string
	ScriptParams      map[int]string // parameter index (4-11) -> value (may contain {version})
	Frequency         string
	Category          string // category name; looked up to get ID
	ScopeGroupIDs     []int
	ScopeAll          bool
	Recon             bool
	DistributionPoint string
}

// PolicyCreateFromTemplate creates a new Jamf policy from a PolicyTemplate.
// {version} placeholders in string fields are replaced with version at call time.
func PolicyCreateFromTemplate(credName string, opts Opts, tmpl PolicyTemplate, version string, logger *slog.Logger) error {
	sub := func(s string) string {
		return strings.ReplaceAll(s, "{version}", version)
	}

	name := sub(tmpl.Name)
	trigger := sub(tmpl.Trigger)

	payload := &policyInstallPayload{}
	payload.General.Name = name
	payload.General.Enabled = true
	payload.General.Trigger = "EVENT"
	payload.General.TriggerOther = trigger
	payload.General.Frequency = tmpl.Frequency
	if payload.General.Frequency == "" {
		payload.General.Frequency = "Once per computer"
	}
	payload.General.Site.ID = -1
	payload.General.Site.Name = "None"

	// Category lookup by name.
	if tmpl.Category != "" {
		idStr, err := categoryGetByName(credName, tmpl.Category)
		if err != nil {
			return fmt.Errorf("look up category %q: %w", tmpl.Category, err)
		}
		catID, _ := strconv.Atoi(idStr)
		payload.General.Category = &struct {
			ID   int    `xml:"id,omitempty"`
			Name string `xml:"name,omitempty"`
		}{ID: catID, Name: tmpl.Category}
	}

	// Scope.
	if tmpl.ScopeAll {
		payload.Scope.AllComputers = true
	} else if len(tmpl.ScopeGroupIDs) > 0 {
		payload.Scope.AllComputers = false
		payload.Scope.ComputerGroups = &[]policySubsetComputerGroup{}
		for _, id := range tmpl.ScopeGroupIDs {
			*payload.Scope.ComputerGroups = append(*payload.Scope.ComputerGroups, policySubsetComputerGroup{ID: id})
		}
	}

	// Script.
	priority := tmpl.ScriptPriority
	if priority == "" {
		priority = "After"
	}
	script := scriptPayload{
		ID:       fmt.Sprintf("%d", tmpl.ScriptID),
		Priority: priority,
	}
	for idx, val := range tmpl.ScriptParams {
		setScriptParam(&script, idx, sub(val))
	}
	payload.Scripts.Script = []scriptPayload{script}

	// Package config.
	payload.PackageConfiguration.DistributionPoint = tmpl.DistributionPoint
	if payload.PackageConfiguration.DistributionPoint == "" {
		payload.PackageConfiguration.DistributionPoint = "default"
	}

	payload.Maintenance.Recon = tmpl.Recon

	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nCreate policy %q with trigger %q", name, trigger), logger) {
			return nil
		}
	}
	if opts.Verbose {
		xmlPayload, _ := xml.MarshalIndent(payload, "", "  ")
		fmt.Fprintf(os.Stderr, "\n--- POST payload (JSSResource/policies/id/0) ---\n%s\n--- end payload ---\n\n", xmlPayload)
	}
	if _, err := postPolicyInstall(credName, payload, logger); err != nil {
		return err
	}
	fmt.Printf("Created policy %q with trigger %s\n", name, trigger)
	return nil
}
