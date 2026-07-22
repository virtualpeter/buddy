package buddy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Opts holds common options for buddy operations.
type Opts struct {
	SkipConfirm bool
	Verbose     bool
	Trace       bool // -t/--trace: HTTP request/response tracing
}

// PolicySS updates policy self_service_description and optionally scope.
func PolicySS(credName string, opts Opts, policyID int, newVersion string, groupIDs []int, logger *slog.Logger) error {
	policy, err := policyGetByID(credName, policyID)
	if err != nil {
		return fmt.Errorf("fetch policy %d: %w", policyID, err)
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
		return fmt.Errorf("fetch policy %d: %w", policyID, err)
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
		return fmt.Errorf("fetch policy %d: %w", policyID, err)
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
		return fmt.Errorf("fetch policy %d: %w", policyID, err)
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
		return fmt.Errorf("fetch policy %d: %w", policyID, err)
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
		return "", fmt.Errorf("fetch policy %d: %w", policyID, err)
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
		return nil, fmt.Errorf("fetch policy %d: %w", policyID, err)
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
		return fmt.Errorf("fetch policy %d: %w", policyID, err)
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
		return fmt.Errorf("fetch policy %d: %w", policyID, err)
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
		return fmt.Errorf("fetch policy %d: %w", policyID, err)
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
		return fmt.Errorf("fetch policy %d: %w", policyID, err)
	}

	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nUpload icon %q and set on policy %q (ID %d)", filePath, policy.General.Name, policyID), logger) {
			return nil
		}
	}

	iconID, err := iconUpload(credName, filePath)
	if err != nil {
		return fmt.Errorf("upload icon: %w", err)
	}
	logger.Debug("Uploaded icon", "id", iconID)

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
		return fmt.Errorf("fetch policy %d: %w", policyID, err)
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

// PolicyClone clones a policy via POST, then disables the source. If newVersion is non-empty, updates script params.
func PolicyClone(credName string, opts Opts, policyID int, appName, oldVersion, newVersion, packageOverride string, paramIndices []int, logger *slog.Logger) error {
	policy, err := policyGetByID(credName, policyID)
	if err != nil {
		return fmt.Errorf("fetch policy %d: %w", policyID, err)
	}

	pkgs := policy.PackageConfiguration.Packages
	trigger := policy.General.TriggerOther

	if len(pkgs) == 0 {
		// Script-only path (unchanged).
		if len(policy.Scripts) == 0 {
			return fmt.Errorf("policy %d has no scripts and no packages", policyID)
		}
		baseName := replaceVersionInString(policy.General.Name, newVersion)
		newName, err := uniquePolicyName(credName, baseName, logger)
		if err != nil {
			return err
		}
		payload := buildPolicyInstallPayload(policy, newName, trigger, newVersion, paramIndices)
		actions := []string{
			fmt.Sprintf("Create policy %q with trigger %s", newName, trigger),
			fmt.Sprintf("Disable source policy %q (ID %d)", policy.General.Name, policy.General.ID),
		}
		if !opts.SkipConfirm {
			fmt.Print("\nThe following changes will be made:\n\n")
			for i, a := range actions {
				fmt.Printf("  %d. %s\n", i+1, a)
			}
			if !ConfirmPrompt("", logger) {
				return nil
			}
		}
		if opts.Verbose {
			xmlPayload, _ := xml.MarshalIndent(payload, "", "  ")
			fmt.Fprintf(os.Stderr, "\n--- POST payload (JSSResource/policies/id/0) ---\n%s\n--- end payload ---\n\n", xmlPayload)
		}
		if err := postPolicyInstall(credName, payload, logger); err != nil {
			return err
		}
		fmt.Printf("Created policy %q with trigger %s\n", newName, trigger)
		disablePayload := buildPolicyInstallPayload(policy, policy.General.Name, trigger, extractInstallSourceVersion(policy, paramIndices), paramIndices)
		disablePayload.General.Enabled = false
		if err := putPolicyInstall(credName, policy.General.ID, disablePayload, logger); err != nil {
			return fmt.Errorf("disable source policy: %w", err)
		}
		fmt.Printf("Disabled source policy %q (ID %d)\n", policy.General.Name, policy.General.ID)
		return nil
	}

	// Package-clone path.
	if appName == "" {
		appName, _, _ = parseTrigger(trigger)
	}
	swapIdx, detectedOldVersion, err := findAppPackageIdx(pkgs, appName, oldVersion)
	if err != nil {
		return err
	}
	if oldVersion == "" {
		oldVersion = detectedOldVersion
	}
	newPkgID, newPkgName, err := findPackage(credName, appName, newVersion, packageOverride, logger)
	if err != nil {
		return err
	}
	if newPkgID == 0 {
		return fmt.Errorf("no package found for %s version %s", appName, newVersion)
	}
	baseName := replaceVersionInString(policy.General.Name, newVersion)
	newName, err := uniquePolicyName(credName, baseName, logger)
	if err != nil {
		return err
	}
	payload := buildPolicyPackagePayloadClone(policy, newName, trigger, swapIdx, newPkgID, newPkgName)
	actions := []string{
		fmt.Sprintf("Create policy %q with trigger %s, replacing package %s with %s (ID %d)", newName, trigger, pkgs[swapIdx].Name, newPkgName, newPkgID),
		fmt.Sprintf("Disable source policy %q (ID %d)", policy.General.Name, policy.General.ID),
	}
	if !opts.SkipConfirm {
		fmt.Print("\nThe following changes will be made:\n\n")
		for i, a := range actions {
			fmt.Printf("  %d. %s\n", i+1, a)
		}
		if !ConfirmPrompt("", logger) {
			return nil
		}
	}
	if opts.Verbose {
		xmlPayload, _ := xml.MarshalIndent(payload, "", "  ")
		fmt.Fprintf(os.Stderr, "\n--- POST payload (JSSResource/policies/id/0) ---\n%s\n--- end payload ---\n\n", xmlPayload)
	}
	if err := postPolicyPackage(credName, payload, logger); err != nil {
		return err
	}
	fmt.Printf("Created policy %q with trigger %s, package %s (ID %d)\n", newName, trigger, newPkgName, newPkgID)
	disablePayload := buildPolicyPackagePayloadClone(policy, policy.General.Name, trigger, -1, 0, "")
	disablePayload.General.Enabled = false
	if err := putPolicyPackage(credName, policy.General.ID, disablePayload, logger); err != nil {
		return fmt.Errorf("disable source policy: %w", err)
	}
	fmt.Printf("Disabled source policy %q (ID %d)\n", policy.General.Name, policy.General.ID)
	return nil
}

// findAppPackageIdx finds the index of the package to swap in a multi-package policy.
// It searches for a package whose name contains appName (case-insensitive).
// If oldVersion is non-empty, the package must also contain oldVersion.
// If oldVersion is empty, it is auto-detected via extractVersionFromPackageName.
// Returns the index, the detected (or provided) oldVersion, and any error.
func findAppPackageIdx(pkgs []policySubsetPackageConfigurationPackage, appName, oldVersion string) (int, string, error) {
	appLower := strings.ToLower(appName)
	for i, pkg := range pkgs {
		n := strings.ToLower(pkg.Name)
		if !strings.Contains(n, appLower) {
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
		return -1, "", fmt.Errorf("no package found for app %q with version %q", appName, oldVersion)
	}
	return -1, "", fmt.Errorf("no versioned package found for app %q", appName)
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
func uniquePolicyName(credName string, baseName string, logger *slog.Logger) (string, error) {
	policies, err := fetchPolicies(credName, logger)
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

// PolicyPackage updates package policy version.
func PolicyPackage(credName string, opts Opts, appName, newVersion string, policyIDOverride int, triggerOverride, packageOverride string, logger *slog.Logger) error {
	if triggerOverride != "" {
		if opts.Verbose {
			fmt.Fprintf(os.Stderr, "Resolving policy from trigger %q...\n", triggerOverride)
		}
		id, err := PolicyFind(credName, opts, triggerOverride, false, logger)
		if err != nil {
			return err
		}
		policyIDOverride = id
		if opts.Verbose {
			fmt.Fprintf(os.Stderr, "  -> policy ID %d\n", policyIDOverride)
		}
	}
	sourcePolicy, err := getPackageSourcePolicy(credName, appName, policyIDOverride, opts, logger)
	if err != nil {
		return err
	}
	if sourcePolicy == nil {
		return fmt.Errorf("no install or cache policy found for app %q", appName)
	}
	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "Looking up package for %s %s...\n", appName, newVersion)
	}
	pkgID, pkgName, err := findPackage(credName, appName, newVersion, packageOverride, logger)
	if err != nil {
		return err
	}
	if pkgID == 0 {
		return fmt.Errorf("no package found for %s version %s", appName, newVersion)
	}
	action := getPackageAction(sourcePolicy)
	if action == "" {
		return fmt.Errorf("source policy has no package or invalid action")
	}
	isInstall := strings.EqualFold(action, "Install")
	sourceVersion := extractVersionFromTrigger(strings.ToLower(sourcePolicy.General.TriggerOther), appName)
	if sourceVersion == "" {
		sourceVersion = extractVersionFromPackageName(sourcePolicy.PackageConfiguration.Packages[0].Name)
	}
	isModify := sourceVersion != "" && compareVersions(sourceVersion, newVersion) == 0

	var createPayload *policyPackagePayload
	var newTrigger, newPolicyName string
	var actions []string
	var toDisable *resourcePolicy

	if isModify {
		newPolicyName = sourcePolicy.General.Name
		newTrigger = sourcePolicy.General.TriggerOther
		createPayload = buildPolicyPackagePayload(sourcePolicy, newPolicyName, newTrigger, pkgID, pkgName, action)
		actions = []string{fmt.Sprintf("Update policy %q (ID %d): set package to %s (ID %d)", newPolicyName, sourcePolicy.General.ID, pkgName, pkgID)}
	} else {
		_, newTrigger, newPolicyName = buildNewPackagePolicy(sourcePolicy, appName, newVersion, pkgID, pkgName, isInstall)
		createPayload = buildPolicyPackagePayload(sourcePolicy, newPolicyName, newTrigger, pkgID, pkgName, action)
		// Only disable the source when the new policy uses the same custom trigger (replacement).
		// When the trigger changes (e.g. versioned cache-app-1.0 → cache-app-2.0), leave the source
		// enabled so both policies remain available.
		if strings.EqualFold(newTrigger, sourcePolicy.General.TriggerOther) {
			toDisable = sourcePolicy
		}
		actions = buildPackageActions(toDisable, newPolicyName, newTrigger, pkgName, pkgID)
	}

	if !opts.SkipConfirm {
		fmt.Print("\nThe following changes will be made:\n\n")
		for i, a := range actions {
			fmt.Printf("  %d. %s\n", i+1, a)
		}
		if !ConfirmPrompt("", logger) {
			return nil
		}
	}
	if toDisable != nil {
		disablePayload := &policyEnablePayload{}
		disablePayload.General.Enabled = false
		if err := putPolicyGeneric(credName, toDisable.General.ID, disablePayload, logger); err != nil {
			return fmt.Errorf("disable policy: %w", err)
		}
	}
	if opts.Verbose {
		xmlPayload, _ := xml.MarshalIndent(createPayload, "", "  ")
		if isModify {
			fmt.Fprintf(os.Stderr, "\n--- PUT payload (JSSResource/policies/id/%d) ---\n%s\n--- end payload ---\n\n", sourcePolicy.General.ID, xmlPayload)
		} else {
			fmt.Fprintf(os.Stderr, "\n--- POST payload (JSSResource/policies/id/0) ---\n%s\n--- end payload ---\n\n", xmlPayload)
		}
	}
	if isModify {
		if err := putPolicyPackage(credName, sourcePolicy.General.ID, createPayload, logger); err != nil {
			return err
		}
		fmt.Printf("Updated policy %q (ID %d) with package %s (ID %d)\n", newPolicyName, sourcePolicy.General.ID, pkgName, pkgID)
	} else {
		if err := postPolicyPackage(credName, createPayload, logger); err != nil {
			return err
		}
		fmt.Printf("Created policy %q with trigger %s, package %s (ID %d)\n", newPolicyName, newTrigger, pkgName, pkgID)
	}
	return nil
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
	// Basic validation: must look like policy XML (get outputs <ResourcePolicy>, API accepts <policy>)
	if !bytes.Contains(data, []byte("<policy")) && !bytes.Contains(data, []byte("<ResourcePolicy")) {
		return fmt.Errorf("input does not appear to be policy XML (missing <policy> or <ResourcePolicy> root)")
	}
	// Strip date_time_limitations to avoid propagating activation/expiration datetimes.
	data = dateTimeLimitationsBlockRegex.ReplaceAll(data, []byte(dateTimeLimitationsEmptyXML))
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nPUT policy (ID %d) with XML from stdin", policyID), logger) {
			return nil
		}
	}
	url, err := ResolveEndpoint(credName, fmt.Sprintf("/JSSResource/policies/id/%d", policyID))
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/xml")
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return fmt.Errorf("update policy: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("update policy status %d: %s", resp.StatusCode, string(body))
	}
	fmt.Printf("Updated policy (ID %d)\n", policyID)
	return nil
}

// PolicyPutXML writes the given policy XML to Jamf for the given policy ID (for script/component use).
// Uses opts.SkipConfirm to skip confirmation when called from scripts. Does not print success message.
func PolicyPutXML(credName string, opts Opts, policyID int, xml []byte, logger *slog.Logger) error {
	data := xml
	if !bytes.Contains(data, []byte("<policy")) && !bytes.Contains(data, []byte("<ResourcePolicy")) {
		return fmt.Errorf("input does not appear to be policy XML (missing <policy> or <ResourcePolicy> root)")
	}
	data = dateTimeLimitationsBlockRegex.ReplaceAll(data, []byte(dateTimeLimitationsEmptyXML))
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nPUT policy (ID %d) with XML", policyID), logger) {
			return nil
		}
	}
	url, err := ResolveEndpoint(credName, fmt.Sprintf("/JSSResource/policies/id/%d", policyID))
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/xml")
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return fmt.Errorf("update policy: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("update policy status %d: %s", resp.StatusCode, string(body))
	}
	return nil
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
			return fmt.Errorf("fetch policy %d: %w", id, err)
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
		fmt.Printf("Deleted policy %q (ID %d)\n", p.name, p.id)
	}
	return nil
}

// PolicyFind finds policy ID by trigger. Returns the ID or error.
func PolicyFind(credName string, opts Opts, trigger string, forceRefresh bool, logger *slog.Logger) (int, error) {
	var policies []policyListItem

	if !forceRefresh {
		if ent, ok := loadPolicyCache(credName, logger); ok {
			policies = ent.Policies
		}
	}
	if policies == nil {
		var err error
		policies, err = policyFetchList(credName)
		if err != nil {
			return 0, err
		}
		_ = savePolicyCache(credName, policies, logger)
	}

	// Phase 1: exact trigger_other match by fetching detail for name-based candidates only.
	triggerNorm := normalizeTriggerForMatch(trigger)
	for _, c := range findCandidates(policies, trigger) {
		detail, err := policyGetByID(credName, c.ID)
		if err == nil && normalizeTriggerForMatch(detail.General.TriggerOther) == triggerNorm {
			return c.ID, nil
		}
	}

	// Phase 2: fall back to name-based heuristics.
	id, ok := findByAppAndVersion(policies, trigger)
	if !ok {
		return 0, fmt.Errorf("no policy found for trigger %q", trigger)
	}
	return id, nil
}

// PolicySummary is a short policy list entry (id + name).
type PolicySummary struct {
	ID   int
	Name string
}

// PolicyFindSummariesByKeywords returns policies whose name contains all keywords (case-insensitive).
// ExcludeIDs are not included in the result.
// If forceRefresh is true, the policy list cache is bypassed to avoid stale IDs (e.g. deleted policies).
func PolicyFindSummariesByKeywords(credName string, opts Opts, keywords []string, excludeIDs []int, forceRefresh bool, logger *slog.Logger) ([]PolicySummary, error) {
	var policies []policyListItem
	if !forceRefresh {
		if ent, ok := loadPolicyCache(credName, logger); ok {
			policies = ent.Policies
		}
	}
	if policies == nil {
		var err error
		policies, err = fetchPolicies(credName, logger)
		if err != nil {
			return nil, err
		}
		_ = savePolicyCache(credName, policies, logger)
	}

	excludeSet := make(map[int]bool)
	for _, id := range excludeIDs {
		excludeSet[id] = true
	}

	var result []PolicySummary
	for _, p := range policies {
		if excludeSet[p.ID] {
			continue
		}
		nameLower := strings.ToLower(p.Name)
		matches := true
		sawNonEmpty := false
		for _, kw := range keywords {
			k := strings.TrimSpace(kw)
			if k == "" {
				continue
			}
			sawNonEmpty = true
			if !strings.Contains(nameLower, strings.ToLower(k)) {
				matches = false
				break
			}
		}
		if matches && sawNonEmpty {
			result = append(result, PolicySummary{ID: p.ID, Name: p.Name})
		}
	}
	return result, nil
}

// PolicyFindByKeywords returns policy IDs whose name contains all keywords (case-insensitive).
func PolicyFindByKeywords(credName string, opts Opts, keywords []string, excludeIDs []int, forceRefresh bool, logger *slog.Logger) ([]int, error) {
	summaries, err := PolicyFindSummariesByKeywords(credName, opts, keywords, excludeIDs, forceRefresh, logger)
	if err != nil {
		return nil, err
	}
	ids := make([]int, len(summaries))
	for i, s := range summaries {
		ids[i] = s.ID
	}
	return ids, nil
}

// ConfirmPrompt prints msg (if non-empty), asks "Proceed? [y/N]" on stdin, and returns true for y/yes.
func ConfirmPrompt(msg string, logger *slog.Logger) bool {
	if msg != "" {
		fmt.Printf("%s\n\n", msg)
	}
	fmt.Print("Proceed? [y/N]: ")
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		logger.Error("Failed to read input", "error", err)
		return false
	}
	line = strings.TrimSpace(strings.ToLower(line))
	if line != "y" && line != "yes" {
		logger.Debug("User cancelled")
		fmt.Println("Aborted.")
		return false
	}
	return true
}

// confirmDangerous prompts for a dangerous action. For ERASE, requireType is "erase" and the user must type "erase".
// For LOCK or other actions, requireType is empty and it behaves like ConfirmPrompt.
func confirmDangerous(msg, requireType string, logger *slog.Logger) bool {
	if msg != "" {
		fmt.Printf("%s\n\n", msg)
	}
	if requireType != "" {
		fmt.Printf("Type %q to confirm: ", requireType)
	} else {
		fmt.Print("Proceed? [y/N]: ")
	}
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		logger.Error("Failed to read input", "error", err)
		return false
	}
	line = strings.TrimSpace(line)
	if requireType != "" {
		if line != requireType {
			logger.Debug("User did not type required confirmation")
			fmt.Println("Aborted.")
			return false
		}
		return true
	}
	lineLower := strings.ToLower(line)
	if lineLower != "y" && lineLower != "yes" {
		logger.Debug("User cancelled")
		fmt.Println("Aborted.")
		return false
	}
	return true
}

func replaceVersionInString(s, newVersion string) string {
	verRe := regexp.MustCompile(`\d+\.\d+(?:\.\d+)*`)
	return verRe.ReplaceAllString(s, newVersion)
}

func compareVersions(a, b string) int {
	pa := strings.Split(a, ".")
	pb := strings.Split(b, ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var na, nb int
		if i < len(pa) {
			fmt.Sscanf(pa[i], "%d", &na)
		}
		if i < len(pb) {
			fmt.Sscanf(pb[i], "%d", &nb)
		}
		if na < nb {
			return -1
		}
		if na > nb {
			return 1
		}
	}
	return 0
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
	return jamfPUT(credName, path, body, "application/xml")
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
	return jamfPUT(credName, path, body, "application/xml")
}

func postPolicyInstall(credName string, payload *policyInstallPayload, logger *slog.Logger) error {
	path := uriPolicies + "/id/0"
	body, err := xml.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal policy: %w", err)
	}
	_, err = jamfPOST(credName, path, body, "application/xml")
	return err
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

func getPackageSourcePolicy(credName, appName string, policyID int, opts Opts, logger *slog.Logger) (*resourcePolicy, error) {
	if policyID > 0 {
		detail, err := policyGetByID(credName, policyID)
		if err != nil {
			return nil, fmt.Errorf("fetch policy %d: %w", policyID, err)
		}
		if len(detail.PackageConfiguration.Packages) != 1 {
			return nil, fmt.Errorf("policy %d has %d packages (expected 1)", policyID, len(detail.PackageConfiguration.Packages))
		}
		action := detail.PackageConfiguration.Packages[0].Action
		if !strings.EqualFold(action, "Install") && !strings.EqualFold(action, "Cache") {
			return nil, fmt.Errorf("policy %d package action is %q (expected Install or Cache)", policyID, action)
		}
		trigger := strings.ToLower(detail.General.TriggerOther)
		name := strings.ToLower(detail.General.Name)
		pkgName := strings.ToLower(detail.PackageConfiguration.Packages[0].Name)
		if !strings.Contains(trigger, appName) && !strings.Contains(name, appName) && !strings.Contains(pkgName, appName) {
			return nil, fmt.Errorf("policy %d does not appear to be for app %q", policyID, appName)
		}
		return detail, nil
	}
	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "Finding source policy for app %q (scanning policies)...\n", appName)
	}
	return findPackageSourcePolicy(credName, appName, opts, logger)
}

func findPackageSourcePolicy(credName, appName string, opts Opts, logger *slog.Logger) (*resourcePolicy, error) {
	var policies []policyListItem
	if ent, ok := loadPolicyCache(credName, logger); ok {
		policies = ent.Policies
	}
	if policies == nil {
		var err error
		policies, err = fetchPolicies(credName, logger)
		if err != nil {
			return nil, fmt.Errorf("get policies: %w", err)
		}
		_ = savePolicyCache(credName, policies, logger)
	}

	appLower := strings.ToLower(appName)
	var nameCandidates []policyListItem
	for _, p := range policies {
		if strings.Contains(strings.ToLower(p.Name), appLower) {
			nameCandidates = append(nameCandidates, p)
		}
	}

	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "  Scanning %d policies for app %q (%d name matches)...\n", len(policies), appName, len(nameCandidates))
	}

	var candidates []*resourcePolicy
	for i, p := range nameCandidates {
		if opts.Verbose && (i%20 == 0 || i == len(nameCandidates)-1) {
			fmt.Fprintf(os.Stderr, "  fetching policy %d/%d (ID %d)...\n", i+1, len(nameCandidates), p.ID)
		}
		detail, err := policyGetByID(credName, p.ID)
		if err != nil {
			continue
		}
		if len(detail.PackageConfiguration.Packages) != 1 {
			continue
		}
		action := detail.PackageConfiguration.Packages[0].Action
		if !strings.EqualFold(action, "Install") && !strings.EqualFold(action, "Cache") {
			continue
		}
		trigger := strings.ToLower(detail.General.TriggerOther)
		name := strings.ToLower(detail.General.Name)
		pkgName := strings.ToLower(detail.PackageConfiguration.Packages[0].Name)
		if !strings.Contains(trigger, appName) && !strings.Contains(name, appName) && !strings.Contains(pkgName, appName) {
			continue
		}
		version := extractVersionFromTrigger(trigger, appName)
		if version == "" {
			version = extractVersionFromPackageName(detail.PackageConfiguration.Packages[0].Name)
		}
		if version == "" {
			continue
		}
		candidates = append(candidates, detail)
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		vi := extractVersionFromTrigger(strings.ToLower(candidates[i].General.TriggerOther), appName)
		if vi == "" {
			vi = extractVersionFromPackageName(candidates[i].PackageConfiguration.Packages[0].Name)
		}
		vj := extractVersionFromTrigger(strings.ToLower(candidates[j].General.TriggerOther), appName)
		if vj == "" {
			vj = extractVersionFromPackageName(candidates[j].PackageConfiguration.Packages[0].Name)
		}
		return compareVersions(vi, vj) > 0
	})
	return candidates[0], nil
}

func extractVersionFromTrigger(trigger, appName string) string {
	re := regexp.MustCompile(`(?:cache|install)-` + regexp.QuoteMeta(appName) + `-(\d+\.\d+(?:\.\d+)?)`)
	m := re.FindStringSubmatch(trigger)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

func extractVersionFromPackageName(pkgName string) string {
	name := strings.ToLower(pkgName)
	// Primary: -1.2.3.pkg, -1.2.3-arm64.pkg
	if m := regexp.MustCompile(`-(\d+\.\d+(?:\.\d+)?)(?:-arm64|-amd64)?\.pkg$`).FindStringSubmatch(name); len(m) >= 2 {
		return m[1]
	}
	// Space or underscore before version: " 1.2.3.pkg", "_1.2.3.pkg"
	if m := regexp.MustCompile(`[\s_](\d+\.\d+(?:\.\d+)?)(?:-arm64|-amd64)?\.pkg$`).FindStringSubmatch(name); len(m) >= 2 {
		return m[1]
	}
	// .dmg: -1.2.3.dmg, 1.2.3.dmg
	if m := regexp.MustCompile(`[-_\s]?(\d+\.\d+(?:\.\d+)?)\.dmg$`).FindStringSubmatch(name); len(m) >= 2 {
		return m[1]
	}
	// Fallback: last version-like pattern in filename (e.g. "AppName 1.2.3.pkg", "App_v1.2.3.pkg")
	verRe := regexp.MustCompile(`(\d+\.\d+(?:\.\d+)?)`)
	matches := verRe.FindAllString(name, -1)
	if len(matches) > 0 {
		return matches[len(matches)-1]
	}
	return ""
}

// findPackage looks up a package by app name and version via PackageFindID (v1 API).
// When patternOverride is non-empty, it overrides the default app+version search.
// Returns (0, "", nil) when no matching package is found.
func findPackage(credName, appName, version, patternOverride string, logger *slog.Logger) (int, string, error) {
	idStr, name, err := PackageFindID(credName, appName, version, patternOverride, logger)
	if err != nil {
		return 0, "", err
	}
	if idStr == "" {
		return 0, "", nil
	}
	var id int
	fmt.Sscanf(idStr, "%d", &id)
	return id, name, nil
}

func getPackageAction(p *resourcePolicy) string {
	if len(p.PackageConfiguration.Packages) == 0 {
		return ""
	}
	return p.PackageConfiguration.Packages[0].Action
}

func findExistingInstallNext(credName, appName string, opts Opts, logger *slog.Logger) (*resourcePolicy, error) {
	var policies []policyListItem
	if ent, ok := loadPolicyCache(credName, logger); ok {
		policies = ent.Policies
	}
	if policies == nil {
		var err error
		policies, err = fetchPolicies(credName, logger)
		if err != nil {
			return nil, err
		}
		_ = savePolicyCache(credName, policies, logger)
	}

	targetTrigger := "install-" + appName + "-next"
	// Filter by name containing app or "next" to reduce fetches
	var nameCandidates []policyListItem
	appLower := strings.ToLower(appName)
	for _, p := range policies {
		n := strings.ToLower(p.Name)
		if strings.Contains(n, appLower) || strings.Contains(n, "next") {
			nameCandidates = append(nameCandidates, p)
		}
	}

	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "  Looking for install-%s-next (%d candidates)...\n", appName, len(nameCandidates))
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

func buildNewPackagePolicy(source *resourcePolicy, appName, newVersion string, pkgID int, pkgName string, isInstall bool) (*resourcePolicy, string, string) {
	var newTrigger string
	sourceTriggerVersion := extractVersionFromTrigger(strings.ToLower(source.General.TriggerOther), appName)
	if sourceTriggerVersion != "" {
		// Source has version in trigger (e.g. install-crowdstrike-7.29) - use versioned trigger for new policy
		if isInstall {
			newTrigger = fmt.Sprintf("install-%s-%s", appName, newVersion)
		} else {
			newTrigger = fmt.Sprintf("cache-%s-%s", appName, newVersion)
		}
	} else {
		// Source trigger has no version (e.g. crowdstrike-install) - keep same trigger for the replacement
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

func buildPackageActions(toDisable *resourcePolicy, newPolicyName, newTrigger, pkgName string, pkgID int) []string {
	var actions []string
	if toDisable != nil {
		actions = append(actions, fmt.Sprintf("Disable policy %q (ID %d)", toDisable.General.Name, toDisable.General.ID))
	}
	actions = append(actions, fmt.Sprintf("Create policy %q with trigger %s, package %s (ID %d)", newPolicyName, newTrigger, pkgName, pkgID))
	return actions
}

func putPolicyPackage(credName string, policyID int, payload *policyPackagePayload, logger *slog.Logger) error {
	path := fmt.Sprintf("%s/id/%d", uriPolicies, policyID)
	body, err := xml.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal policy: %w", err)
	}
	return jamfPUT(credName, path, body, "application/xml")
}

func postPolicyPackage(credName string, payload *policyPackagePayload, logger *slog.Logger) error {
	path := uriPolicies + "/id/0"
	body, err := xml.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal policy: %w", err)
	}
	_, err = jamfPOST(credName, path, body, "application/xml")
	return err
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

// PolicyCreatePackage creates a new package policy from scratch (no source policy required).
func PolicyCreatePackage(credName string, opts Opts, appName, version string, action string, scopeAll bool, scopeGroupIDs []int, category, packageOverride string, logger *slog.Logger) error {
	pkgIDStr, pkgName, err := PackageFindID(credName, appName, version, packageOverride, logger)
	if err != nil {
		return err
	}
	if pkgIDStr == "" {
		return fmt.Errorf("no package found for %s version %s", appName, version)
	}
	var pkgID int
	if _, err := fmt.Sscanf(pkgIDStr, "%d", &pkgID); err != nil {
		return fmt.Errorf("invalid package ID: %w", err)
	}
	actionNorm := strings.ToLower(strings.TrimSpace(action))
	if actionNorm != "install" && actionNorm != "cache" {
		return fmt.Errorf("action must be install or cache, got %q", action)
	}
	actionAPI := "Install"
	if actionNorm == "cache" {
		actionAPI = "Cache"
	}
	appTitle := appName
	if r := []rune(appName); len(r) > 0 {
		r[0] = unicode.ToUpper(r[0])
		appTitle = string(r)
	}
	var trigger, baseName string
	if actionNorm == "install" {
		trigger = "install-" + strings.ToLower(appName) + "-next"
		baseName = "Install " + appTitle + " " + version
	} else {
		trigger = fmt.Sprintf("cache-%s-%s", strings.ToLower(appName), version)
		baseName = "Cache " + appTitle + " " + version
	}
	policyName, err := uniquePolicyName(credName, baseName, logger)
	if err != nil {
		return err
	}
	var catID int
	var catName string
	if category != "" {
		idStr, err := categoryGetByName(credName, category)
		if err != nil {
			return fmt.Errorf("look up category %q: %w", category, err)
		}
		catID, _ = strconv.Atoi(idStr)
		catName = category
	}
	payload := buildPolicyPackagePayloadFromScratch(policyName, trigger, pkgID, pkgName, actionAPI, scopeAll, scopeGroupIDs, catID, catName)
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nCreate policy %q with trigger %q, package %s (ID %d)", policyName, trigger, pkgName, pkgID), logger) {
			return nil
		}
	}
	if opts.Verbose {
		xmlPayload, _ := xml.MarshalIndent(payload, "", "  ")
		fmt.Fprintf(os.Stderr, "\n--- POST payload (JSSResource/policies/id/0) ---\n%s\n--- end payload ---\n\n", xmlPayload)
	}
	if err := postPolicyPackage(credName, payload, logger); err != nil {
		return err
	}
	fmt.Printf("Created policy %q with trigger %s, package %s (ID %d)\n", policyName, trigger, pkgName, pkgID)
	return nil
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
	ids, err := PolicyFindByKeywords(credName, opts, []string{policyName}, nil, true, logger)
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
	return err
}

// --- findpolicy ---

const policyCacheMaxAge = 24 * time.Hour

type policyListItem struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type policyCacheEntry struct {
	Updated  time.Time        `json:"updated"`
	Cred     string           `json:"cred"`
	Policies []policyListItem `json:"policies"`
}

func policyCachePath(credName string) (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	base := filepath.Join(dir, "buddy")
	if err := os.MkdirAll(base, 0755); err != nil {
		return "", err
	}
	return filepath.Join(base, fmt.Sprintf("policies-%s.json", credName)), nil
}

func loadPolicyCache(credName string, logger *slog.Logger) (*policyCacheEntry, bool) {
	path, err := policyCachePath(credName)
	if err != nil {
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var ent policyCacheEntry
	if err := json.Unmarshal(data, &ent); err != nil {
		return nil, false
	}
	if ent.Cred != credName {
		return nil, false
	}
	if time.Since(ent.Updated) > policyCacheMaxAge {
		return nil, false
	}
	return &ent, true
}

func savePolicyCache(credName string, policies []policyListItem, logger *slog.Logger) error {
	path, err := policyCachePath(credName)
	if err != nil {
		return err
	}
	ent := policyCacheEntry{
		Updated:  time.Now(),
		Cred:     credName,
		Policies: policies,
	}
	data, err := json.MarshalIndent(ent, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

func fetchPolicies(credName string, logger *slog.Logger) ([]policyListItem, error) {
	return policyFetchList(credName)
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

func parseTrigger(trigger string) (app string, version string, isNext bool) {
	t := strings.ToLower(strings.TrimSpace(trigger))

	// Pre-compile regexes so they are available in both the -next and plain branches.
	reVersion := regexp.MustCompile(`^(?:cache|install)-(.+)-(\d+\.\d+(?:\.\d+)*)$`)
	reInstallCache := regexp.MustCompile(`^(?:install|cache)-(.+)$`)
	// Reversed format: <app>-<verb>  e.g. "filebeat-install", "filebeat-cache"
	reAppVerb := regexp.MustCompile(`^(.+)-(?:install|cache)$`)

	if strings.HasSuffix(t, "-next") {
		remainder := strings.TrimSuffix(t, "-next")
		// install-{app}-{version}-next
		if m := reVersion.FindStringSubmatch(remainder); len(m) >= 3 {
			return m[1], m[2], true
		}
		// install-{app}-next
		if m := reInstallCache.FindStringSubmatch(remainder); len(m) >= 2 {
			return m[1], "", true
		}
		// {app}-install-next
		if m := reAppVerb.FindStringSubmatch(remainder); len(m) >= 2 {
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
	if m := reAppVerb.FindStringSubmatch(t); len(m) >= 2 {
		return m[1], "", false
	}
	return t, "", false
}

var versionInNameRe = regexp.MustCompile(`\d+\.\d+(?:\.\d+)*`)

func versionFromName(name string) string {
	return versionInNameRe.FindString(name)
}

// findCandidates returns all policies whose name contains the app name derived from trigger.
func findCandidates(policies []policyListItem, trigger string) []policyListItem {
	app, _, _ := parseTrigger(trigger)
	appLower := strings.ToLower(app)
	var candidates []policyListItem
	for _, p := range policies {
		if strings.Contains(strings.ToLower(p.Name), appLower) {
			candidates = append(candidates, p)
		}
	}
	return candidates
}

func findByAppAndVersion(policies []policyListItem, trigger string) (int, bool) {
	triggerLower := strings.ToLower(strings.TrimSpace(trigger))
	_, wantVersion, isNext := parseTrigger(trigger)
	preferInName := ""
	if strings.HasPrefix(triggerLower, "install-") && !strings.HasSuffix(triggerLower, "-next") {
		preferInName = "install"
	} else if strings.HasPrefix(triggerLower, "cache-") {
		preferInName = "cache"
	}
	candidates := findCandidates(policies, trigger)
	if len(candidates) == 0 {
		return 0, false
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
	if err := postPolicyInstall(credName, payload, logger); err != nil {
		return err
	}
	fmt.Printf("Created policy %q with trigger %s\n", name, trigger)
	return nil
}
