package buddy

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"buddy/internal/buddy/secrets"
)

// AppImportOpts configures interactive or flag-driven app import.
type AppImportOpts struct {
	Name               string
	AppsDir            string // default "apps"
	Output             string // script path; default apps/<slug>.buddy
	SSPolicyID         int
	InstallPolicyIDs   []int
	CachePolicyIDs     []int
	UninstallPolicyIDs []int
	PackagePattern     string
	InstallTrigger     string
	CacheTrigger       string
	UninstallTrigger   string
	Kind               string
	Version            string
}

// hasBindingFlags reports whether the caller supplied enough IDs to skip the interview.
func (o AppImportOpts) hasBindingFlags() bool {
	return o.SSPolicyID > 0
}

// AppImport writes apps/index.json + desired-state .buddy via interview or flags.
func AppImport(credName string, opts Opts, importOpts AppImportOpts, logger *slog.Logger) (string, error) {
	appsDir := strings.TrimSpace(importOpts.AppsDir)
	if appsDir == "" {
		appsDir = DefaultAppsDir()
	}
	if err := EnsurePolicyCache(credName, opts, logger); err != nil {
		return "", err
	}

	var entry AppIndexEntry
	var err error
	if importOpts.hasBindingFlags() {
		entry, err = appImportFromFlags(credName, importOpts, logger)
	} else {
		if !secrets.StdinIsTerminal() && !opts.SkipConfirm {
			return "", fmt.Errorf("app import requires a TTY for the interview, or pass --ss=<id> (and optional --install/--cache/--uninstall)")
		}
		entry, err = appImportInterview(credName, importOpts, logger)
	}
	if err != nil {
		return "", err
	}

	slug := normalizeAppSlugKey(entry.Name)
	if slug == "" {
		return "", fmt.Errorf("app name is required")
	}
	entry.Name = slug

	scriptPath := strings.TrimSpace(importOpts.Output)
	if scriptPath == "" {
		scriptPath = filepath.Join(appsDir, slug+".buddy")
	}
	entry.Script = filepath.Base(scriptPath)

	desired := appDesiredFromIndexEntry(entry)
	body, err := RenderAppImportScript(desired)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}
	if err := os.WriteFile(scriptPath, []byte(body), 0o755); err != nil {
		return "", err
	}
	if err := UpsertAppIndexEntry(appsDir, entry); err != nil {
		return "", err
	}

	fmt.Printf("Imported %q (%s) → %s\n", entry.Name, entry.Kind, scriptPath)
	fmt.Printf("Index: %s\n", AppIndexPath(appsDir))
	fmt.Printf("Edit VERSION if needed, then: buddy %s\n", scriptPath)
	return scriptPath, nil
}

func appDesiredFromIndexEntry(e AppIndexEntry) AppDesired {
	des := AppDesired{
		Name:             e.Name,
		Kind:             e.Kind,
		Version:          e.Version,
		InstallTrigger:   e.InstallTrigger,
		CacheTrigger:     e.CacheTrigger,
		UninstallTrigger: e.UninstallTrigger,
		PackagePattern:   e.PackagePattern,
		Risk:             "low",
	}
	if des.Version == "" {
		des.Version = "0"
	}
	if des.PackagePattern == "" {
		des.PackagePattern = DefaultAppPackagePattern(des.Name, des.Version)
	}
	des.Title = fmt.Sprintf("%s %s desired state", des.Name, des.Version)
	des.Summary = fmt.Sprintf("Reconcile policy app %q (%s) to version %s.", des.Name, des.Kind, des.Version)
	return des
}

func appImportFromFlags(credName string, o AppImportOpts, logger *slog.Logger) (AppIndexEntry, error) {
	name := normalizeAppSlugKey(o.Name)
	if name == "" {
		return AppIndexEntry{}, fmt.Errorf("app name is required")
	}
	entry := AppIndexEntry{
		Name:               name,
		SSPolicyID:         o.SSPolicyID,
		InstallPolicyIDs:   append([]int(nil), o.InstallPolicyIDs...),
		CachePolicyIDs:     append([]int(nil), o.CachePolicyIDs...),
		UninstallPolicyIDs: append([]int(nil), o.UninstallPolicyIDs...),
		InstallTrigger:     strings.TrimSpace(o.InstallTrigger),
		CacheTrigger:       strings.TrimSpace(o.CacheTrigger),
		UninstallTrigger:   strings.TrimSpace(o.UninstallTrigger),
		PackagePattern:     strings.TrimSpace(o.PackagePattern),
		Kind:               strings.TrimSpace(o.Kind),
		Version:            strings.TrimSpace(o.Version),
	}
	if err := fillIndexEntryFromPolicies(credName, &entry, logger); err != nil {
		return AppIndexEntry{}, err
	}
	entry.Kind = inferAppKind(entry)
	if o.Kind != "" {
		entry.Kind = strings.TrimSpace(o.Kind)
	}
	if entry.PackagePattern == "" {
		entry.PackagePattern = DefaultAppPackagePattern(entry.Name, entry.Version)
	}
	return entry, nil
}

func fillIndexEntryFromPolicies(credName string, entry *AppIndexEntry, logger *slog.Logger) error {
	if entry.SSPolicyID > 0 {
		ss, err := PolicyCacheGet(credName, entry.SSPolicyID, logger)
		if err != nil {
			return fmt.Errorf("ss policy %d: %w", entry.SSPolicyID, err)
		}
		if entry.InstallTrigger == "" {
			entry.InstallTrigger = installTriggerFromScripts(ss.Scripts)
		}
		if entry.Version == "" {
			entry.Version = ExtractVersion(ss.SelfService.SelfServiceDescription)
		}
		if len(entry.InstallPolicyIDs) == 0 && len(ss.PackageConfiguration.Packages) > 0 && entry.InstallTrigger == "" {
			// single-kind: package on SS
		}
	}
	for _, id := range entry.InstallPolicyIDs {
		p, err := PolicyCacheGet(credName, id, logger)
		if err != nil {
			return fmt.Errorf("install policy %d: %w", id, err)
		}
		if entry.InstallTrigger == "" {
			entry.InstallTrigger = strings.TrimSpace(p.General.TriggerOther)
		}
		if entry.Version == "" {
			entry.Version = policyAppVersion(p, nil)
		}
	}
	for _, id := range entry.CachePolicyIDs {
		p, err := PolicyCacheGet(credName, id, logger)
		if err != nil {
			return fmt.Errorf("cache policy %d: %w", id, err)
		}
		if entry.CacheTrigger == "" {
			entry.CacheTrigger = strings.TrimSpace(p.General.TriggerOther)
		}
		if entry.Version == "" {
			entry.Version = policyAppVersion(p, nil)
		}
	}
	for _, id := range entry.UninstallPolicyIDs {
		p, err := PolicyCacheGet(credName, id, logger)
		if err != nil {
			return fmt.Errorf("uninstall policy %d: %w", id, err)
		}
		if entry.UninstallTrigger == "" {
			entry.UninstallTrigger = strings.TrimSpace(p.General.TriggerOther)
		}
	}
	return nil
}

func inferAppKind(e AppIndexEntry) string {
	if len(e.CachePolicyIDs) > 0 {
		return string(PolicyAppCache)
	}
	if len(e.InstallPolicyIDs) > 0 || e.InstallTrigger != "" {
		return string(PolicyAppInstall)
	}
	return string(PolicyAppSingle)
}

func appImportInterview(credName string, o AppImportOpts, logger *slog.Logger) (AppIndexEntry, error) {
	name := strings.TrimSpace(o.Name)
	if name == "" {
		fmt.Fprint(confirmOut, "App name/slug: ")
		line, err := readInterviewLine()
		if err != nil {
			return AppIndexEntry{}, err
		}
		name = line
	} else {
		fmt.Fprintf(confirmOut, "App name/slug [%s]: ", name)
		line, err := readInterviewLine()
		if err != nil {
			return AppIndexEntry{}, err
		}
		if strings.TrimSpace(line) != "" {
			name = strings.TrimSpace(line)
		}
	}
	name = normalizeAppSlugKey(name)
	if name == "" {
		return AppIndexEntry{}, fmt.Errorf("app name is required")
	}

	sums, err := LoadPolicyCacheSummaries(credName)
	if err != nil {
		return AppIndexEntry{}, err
	}
	if len(sums) == 0 {
		return AppIndexEntry{}, fmt.Errorf("policy cache has no summaries; run: buddy app cache refresh")
	}
	groups := RankRelatedPolicies(name, sums)

	fmt.Printf("\nRelated policies for %q (from policy cache):\n", name)
	printRelatedGroup("Self Service", groups.SS)
	printRelatedGroup("Install", groups.Install)
	printRelatedGroup("Cache", groups.Cache)
	printRelatedGroup("Uninstall", groups.Uninstall)
	if len(groups.Other) > 0 && len(groups.Other) <= 15 {
		printRelatedGroup("Other matches", groups.Other)
	}

	ssID, err := promptPolicyIDs("Self Service policy ID", suggestedIDs(groups.SS, 1), true)
	if err != nil {
		return AppIndexEntry{}, err
	}
	if len(ssID) == 0 {
		return AppIndexEntry{}, fmt.Errorf("Self Service policy ID is required")
	}
	installIDs, err := promptPolicyIDs("Install policy ID(s)", suggestedIDs(groups.Install, 5), false)
	if err != nil {
		return AppIndexEntry{}, err
	}
	cacheIDs, err := promptPolicyIDs("Cache policy ID(s)", suggestedIDs(groups.Cache, 8), false)
	if err != nil {
		return AppIndexEntry{}, err
	}
	uninstallIDs, err := promptPolicyIDs("Uninstall policy ID(s)", suggestedIDs(groups.Uninstall, 3), false)
	if err != nil {
		return AppIndexEntry{}, err
	}

	entry := AppIndexEntry{
		Name:               name,
		SSPolicyID:         ssID[0],
		InstallPolicyIDs:   installIDs,
		CachePolicyIDs:     cacheIDs,
		UninstallPolicyIDs: uninstallIDs,
	}
	if err := fillIndexEntryFromPolicies(credName, &entry, logger); err != nil {
		return AppIndexEntry{}, err
	}
	entry.Kind = inferAppKind(entry)

	entry.InstallTrigger, err = promptStringDefault("Install trigger", entry.InstallTrigger)
	if err != nil {
		return AppIndexEntry{}, err
	}
	entry.CacheTrigger, err = promptStringDefault("Cache trigger", entry.CacheTrigger)
	if err != nil {
		return AppIndexEntry{}, err
	}
	entry.UninstallTrigger, err = promptStringDefault("Uninstall trigger", entry.UninstallTrigger)
	if err != nil {
		return AppIndexEntry{}, err
	}
	entry.Version, err = promptStringDefault("Current version", entry.Version)
	if err != nil {
		return AppIndexEntry{}, err
	}
	if entry.PackagePattern == "" {
		entry.PackagePattern = DefaultAppPackagePattern(entry.Name, entry.Version)
	}
	entry.PackagePattern, err = promptStringDefault("Package pattern", entry.PackagePattern)
	if err != nil {
		return AppIndexEntry{}, err
	}
	kind, err := promptStringDefault("Kind (single|install|cache)", entry.Kind)
	if err != nil {
		return AppIndexEntry{}, err
	}
	entry.Kind = strings.TrimSpace(kind)
	return entry, nil
}

func suggestedIDs(hits []RelatedPolicyHit, limit int) []int {
	if limit <= 0 {
		limit = 5
	}
	var ids []int
	for i, h := range hits {
		if i >= limit {
			break
		}
		ids = append(ids, h.Summary.ID)
	}
	return ids
}

func printRelatedGroup(title string, hits []RelatedPolicyHit) {
	if len(hits) == 0 {
		return
	}
	fmt.Printf("\n  %s:\n", title)
	limit := 12
	for i, h := range hits {
		if i >= limit {
			fmt.Printf("    … %d more\n", len(hits)-limit)
			break
		}
		s := h.Summary
		ss := ""
		if s.UseForSelfService {
			ss = " ss=yes"
		}
		en := "off"
		if s.Enabled {
			en = "on"
		}
		fmt.Printf("    %6d  %-40s  trig=%-24s  %s%s\n",
			s.ID, truncateRunes(s.Name, 40), truncateRunes(s.Trigger, 24), en, ss)
	}
}

func promptPolicyIDs(label string, suggested []int, required bool) ([]int, error) {
	def := joinIntCSV(suggested)
	hint := def
	if hint == "" {
		hint = "none"
	}
	fmt.Fprintf(confirmOut, "%s [%s] (comma-separated IDs, empty=accept): ", label, hint)
	line, err := readInterviewLine()
	if err != nil {
		return nil, err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		if required && len(suggested) == 0 {
			return nil, fmt.Errorf("%s is required", label)
		}
		return append([]int(nil), suggested...), nil
	}
	if strings.EqualFold(line, "none") || line == "-" {
		if required {
			return nil, fmt.Errorf("%s is required", label)
		}
		return nil, nil
	}
	ids, err := parseIntCSV(line)
	if err != nil {
		return nil, err
	}
	if required && len(ids) == 0 {
		return nil, fmt.Errorf("%s is required", label)
	}
	return ids, nil
}

func promptStringDefault(label, def string) (string, error) {
	fmt.Fprintf(confirmOut, "%s [%s]: ", label, def)
	line, err := readInterviewLine()
	if err != nil {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return def, nil
	}
	return line, nil
}

func readInterviewLine() (string, error) {
	reader := bufio.NewReader(confirmIn)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func parseIntCSV(s string) ([]int, error) {
	parts := strings.Split(s, ",")
	var out []int
	seen := map[int]bool{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid policy id %q", p)
		}
		if seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	return out, nil
}

func joinIntCSV(ids []int) string {
	if len(ids) == 0 {
		return ""
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.Itoa(id)
	}
	return strings.Join(parts, ",")
}

// ParseIntCSVList is exported for CLI flag parsing tests.
func ParseIntCSVList(s string) ([]int, error) {
	return parseIntCSV(s)
}
