package buddy

import (
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"
)

// PolicyAppKind classifies a policy-delivered app.
type PolicyAppKind string

const (
	PolicyAppSingle  PolicyAppKind = "single"
	PolicyAppInstall PolicyAppKind = "install"
	PolicyAppCache   PolicyAppKind = "cache"
)

// PolicyAppSummary is one discovered policy app (list row).
type PolicyAppSummary struct {
	Name           string     `json:"name"`
	Kind           PolicyAppKind `json:"kind"`
	Version        string     `json:"version,omitempty"`
	SSPolicyID     int        `json:"ss_policy_id"`
	SSPolicyName   string     `json:"ss_policy_name,omitempty"`
	InstallTrigger string     `json:"install_trigger,omitempty"`
	InstallPolicyID int       `json:"install_policy_id,omitempty"`
	CachePolicyID  int        `json:"cache_policy_id,omitempty"`
	PackageID      int        `json:"package_id,omitempty"`
	PackageName    string     `json:"package_name,omitempty"`
}

// PolicyAppDetail is AppGet output: summary plus linked policy IDs/names.
type PolicyAppDetail struct {
	PolicyAppSummary
	SSDisplayName    string             `json:"ss_display_name,omitempty"`
	SSDescription    string             `json:"ss_description,omitempty"`
	InstallPolicyName string            `json:"install_policy_name,omitempty"`
	CachePolicyName  string             `json:"cache_policy_name,omitempty"`
	CachePolicies    []PolicyAppRef  `json:"cache_policies,omitempty"`
	InstallPolicies  []PolicyAppRef  `json:"install_policies,omitempty"`
}

// PolicyAppRef is a short linked-policy row.
type PolicyAppRef struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Trigger string `json:"trigger,omitempty"`
	Enabled bool   `json:"enabled"`
	Version string `json:"version,omitempty"`
}

var (
	ssNameMarkerRe     = regexp.MustCompile(`(?i)\[ss\]`)
	installLikeParamRe = regexp.MustCompile(`(?i)^(install-.+|.+-install(?:-next)?)$`)
	cacheTriggerRe     = regexp.MustCompile(`(?i)^(?:cache-.+|.+-cache)(?:-\d+\.\d+(?:\.\d+)*)?$`)
)

// AppSearch discovers policy apps from Self Service policies whose name contains "[SS]".
// filter is an optional client-side substring on name / kind / version / trigger / package.
func AppSearch(credName, filter string, logger *slog.Logger) ([]PolicyAppSummary, error) {
	list, err := getPolicyList(credName, logger)
	if err != nil {
		return nil, err
	}
	var candidates []policyListItem
	for _, p := range list {
		if ssNameMarkerRe.MatchString(p.Name) {
			candidates = append(candidates, p)
		}
	}
	out := make([]PolicyAppSummary, 0, len(candidates))
	seen := map[string]bool{}
	for _, c := range candidates {
		sum, ok, err := classifyPolicyAppFromSS(credName, c.ID, list, logger)
		if err != nil {
			if logger != nil {
				logger.Debug("app classify skip", "policy_id", c.ID, "error", err.Error())
			}
			continue
		}
		if !ok {
			continue
		}
		key := strings.ToLower(sum.Name)
		if seen[key] {
			continue
		}
		seen[key] = true
		if !policyAppMatchesFilter(sum, filter) {
			continue
		}
		out = append(out, sum)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

// AppGet returns one policy app by app name (case-insensitive).
func AppGet(credName, name string, logger *slog.Logger) (*PolicyAppDetail, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("app name is required")
	}
	apps, err := AppSearch(credName, "", logger)
	if err != nil {
		return nil, err
	}
	var match *PolicyAppSummary
	for i := range apps {
		if strings.EqualFold(apps[i].Name, name) {
			match = &apps[i]
			break
		}
	}
	if match == nil {
		return nil, fmt.Errorf("app %q not found", name)
	}
	return enrichPolicyAppDetail(credName, *match, logger)
}

// AppUpgrade bumps a policy app to version using package_pattern (optional glob).
// Dispatches to PolicyPackage / PolicySSUpdate by kind.
func AppUpgrade(credName string, opts Opts, name, version, packagePattern string, logger *slog.Logger) error {
	name = strings.TrimSpace(name)
	version = strings.TrimSpace(version)
	if name == "" {
		return fmt.Errorf("app name is required")
	}
	if version == "" {
		return fmt.Errorf("version is required")
	}
	detail, err := AppGet(credName, name, logger)
	if err != nil {
		return err
	}
	switch detail.Kind {
	case PolicyAppSingle:
		return upgradePolicyAppSingle(credName, opts, detail, version, packagePattern, logger)
	case PolicyAppInstall:
		return upgradePolicyAppInstall(credName, opts, detail, version, packagePattern, logger)
	case PolicyAppCache:
		return upgradePolicyAppCache(credName, opts, detail, version, packagePattern, logger)
	default:
		return fmt.Errorf("app %q has unknown kind %q", name, detail.Kind)
	}
}

func upgradePolicyAppSingle(credName string, opts Opts, d *PolicyAppDetail, version, packagePattern string, logger *slog.Logger) error {
	if d.SSPolicyID <= 0 {
		return fmt.Errorf("app %q: missing Self Service policy", d.Name)
	}
	source, err := getPackageSourcePolicy(credName, d.Name, d.SSPolicyID, opts, logger)
	if err != nil {
		return err
	}
	pkgID, pkgName, err := findPackage(credName, d.Name, version, packagePattern, logger)
	if err != nil {
		return err
	}
	if pkgID == 0 {
		return fmt.Errorf("no package found for %s version %s", d.Name, version)
	}
	action := getPackageAction(source)
	payload := buildPolicyUpdatePayload(source, version, nil)
	payload.PackageConfiguration.Packages.Package = []struct {
		ID     int    `xml:"id"`
		Name   string `xml:"name"`
		Action string `xml:"action"`
	}{{ID: pkgID, Name: pkgName, Action: action}}
	if !opts.SkipConfirm {
		msg := fmt.Sprintf("\nUpdate Self Service policy %q (ID %d): package %s -> %s (version %s)",
			source.General.Name, d.SSPolicyID, source.PackageConfiguration.Packages[0].Name, pkgName, version)
		PrintConfirmActions([]string{msg})
		if !ConfirmPrompt("", logger) {
			return nil
		}
	}
	if err := putPolicyGeneric(credName, d.SSPolicyID, payload, logger); err != nil {
		return err
	}
	fmt.Printf("Updated policy app %q (single) policy %q (ID %d) package -> %s\n", d.Name, source.General.Name, d.SSPolicyID, pkgName)
	return nil
}

func upgradePolicyAppInstall(credName string, opts Opts, d *PolicyAppDetail, version, packagePattern string, logger *slog.Logger) error {
	if d.InstallPolicyID <= 0 && d.InstallTrigger == "" {
		return fmt.Errorf("app %q: missing install policy", d.Name)
	}
	return PolicyPackage(credName, opts, d.Name, version, d.InstallPolicyID, d.InstallTrigger, packagePattern, DisableSourceAuto, logger)
}

func upgradePolicyAppCache(credName string, opts Opts, d *PolicyAppDetail, version, packagePattern string, logger *slog.Logger) error {
	if d.CachePolicyID <= 0 {
		return fmt.Errorf("app %q: no current cache policy to clone from", d.Name)
	}
	// Keep prior versioned cache for rollback.
	if err := PolicyPackage(credName, opts, d.Name, version, d.CachePolicyID, "", packagePattern, DisableSourceKeep, logger); err != nil {
		return err
	}
	trigger := d.InstallTrigger
	if trigger == "" {
		trigger = "install-" + strings.ToLower(d.Name) + "-next"
	}
	if err := PolicyPackage(credName, opts, d.Name, version, d.InstallPolicyID, trigger, packagePattern, DisableSourceAuto, logger); err != nil {
		return err
	}
	if d.SSPolicyID > 0 && ssDescriptionHasVersion(credName, d.SSPolicyID) {
		return PolicySSUpdate(credName, opts, d.SSPolicyID, PolicySSOpts{Version: version}, logger)
	}
	return nil
}

func ssDescriptionHasVersion(credName string, policyID int) bool {
	p, err := policyGetByID(credName, policyID)
	if err != nil {
		return false
	}
	return ExtractVersion(p.SelfService.SelfServiceDescription) != ""
}

func policyAppMatchesFilter(sum PolicyAppSummary, filter string) bool {
	filter = strings.ToLower(strings.TrimSpace(filter))
	if filter == "" {
		return true
	}
	hay := strings.ToLower(strings.Join([]string{
		sum.Name, string(sum.Kind), sum.Version, sum.InstallTrigger,
		sum.SSPolicyName, sum.PackageName,
	}, " "))
	return strings.Contains(hay, filter)
}

func classifyPolicyAppFromSS(credName string, ssID int, list []policyListItem, logger *slog.Logger) (PolicyAppSummary, bool, error) {
	ss, err := policyGetByID(credName, ssID)
	if err != nil {
		return PolicyAppSummary{}, false, err
	}
	if !ss.SelfService.UseForSelfService {
		return PolicyAppSummary{}, false, nil
	}

	pkgs := ss.PackageConfiguration.Packages
	installTrig := installTriggerFromScripts(ss.Scripts)

	// single: packages on SS and no install-like script param
	if len(pkgs) > 0 && installTrig == "" {
		pkg := pkgs[0]
		name := policyAppNameFromSS(ss, "")
		ver := ExtractVersion(pkg.Name)
		if ver == "" {
			ver = versionFromName(ss.General.Name)
		}
		if ver == "" {
			ver = ExtractVersion(ss.SelfService.SelfServiceDescription)
		}
		return PolicyAppSummary{
			Name:         name,
			Kind:         PolicyAppSingle,
			Version:      ver,
			SSPolicyID:   ss.General.ID,
			SSPolicyName: ss.General.Name,
			PackageID:    pkg.ID,
			PackageName:  pkg.Name,
		}, true, nil
	}

	if installTrig == "" {
		// Stub SS: infer install trigger from display/name slug.
		slug := policyAppNameFromSS(ss, "")
		installTrig = inferInstallTrigger(credName, slug, list, logger)
		if installTrig == "" {
			return PolicyAppSummary{}, false, nil
		}
	}

	appName, _, _ := parseTrigger(installTrig)
	if appName == "" {
		appName = policyAppNameFromSS(ss, installTrig)
	}

	installs := findPoliciesByTrigger(credName, list, installTrig, logger)
	var current *resourcePolicy
	for _, p := range installs {
		if p.General.Enabled {
			current = p
			break
		}
	}
	if current == nil && len(installs) > 0 {
		current = installs[0]
	}

	caches := findCachePoliciesForApp(credName, list, appName, logger)
	kind := PolicyAppInstall
	if len(caches) > 0 {
		kind = PolicyAppCache
	}

	sum := PolicyAppSummary{
		Name:           appName,
		Kind:           kind,
		SSPolicyID:     ss.General.ID,
		SSPolicyName:   ss.General.Name,
		InstallTrigger: installTrig,
	}
	if current != nil {
		sum.InstallPolicyID = current.General.ID
		if len(current.PackageConfiguration.Packages) > 0 {
			sum.PackageID = current.PackageConfiguration.Packages[0].ID
			sum.PackageName = current.PackageConfiguration.Packages[0].Name
		}
		sum.Version = policyAppVersion(current, ss)
	}
	if kind == PolicyAppCache {
		curCache := pickCurrentCache(caches, sum.Version)
		if curCache != nil {
			sum.CachePolicyID = curCache.General.ID
			if sum.Version == "" {
				sum.Version = policyAppVersion(curCache, ss)
			}
			if sum.PackageID == 0 && len(curCache.PackageConfiguration.Packages) > 0 {
				sum.PackageID = curCache.PackageConfiguration.Packages[0].ID
				sum.PackageName = curCache.PackageConfiguration.Packages[0].Name
			}
		}
	}
	if sum.Version == "" {
		sum.Version = ExtractVersion(ss.SelfService.SelfServiceDescription)
	}
	return sum, true, nil
}

func enrichPolicyAppDetail(credName string, sum PolicyAppSummary, logger *slog.Logger) (*PolicyAppDetail, error) {
	list, err := getPolicyList(credName, logger)
	if err != nil {
		return nil, err
	}
	d := &PolicyAppDetail{PolicyAppSummary: sum}
	if sum.SSPolicyID > 0 {
		if ss, err := policyGetByID(credName, sum.SSPolicyID); err == nil {
			d.SSDisplayName = ss.SelfService.SelfServiceDisplayName
			d.SSDescription = ss.SelfService.SelfServiceDescription
		}
	}
	if sum.InstallTrigger != "" {
		for _, p := range findPoliciesByTrigger(credName, list, sum.InstallTrigger, logger) {
			ref := PolicyAppRef{
				ID:      p.General.ID,
				Name:    p.General.Name,
				Trigger: strings.TrimSpace(p.General.TriggerOther),
				Enabled: p.General.Enabled,
				Version: policyAppVersion(p, nil),
			}
			d.InstallPolicies = append(d.InstallPolicies, ref)
			if p.General.ID == sum.InstallPolicyID {
				d.InstallPolicyName = p.General.Name
			}
		}
	}
	caches := findCachePoliciesForApp(credName, list, sum.Name, logger)
	for _, p := range caches {
		ref := PolicyAppRef{
			ID:      p.General.ID,
			Name:    p.General.Name,
			Trigger: strings.TrimSpace(p.General.TriggerOther),
			Enabled: p.General.Enabled,
			Version: policyAppVersion(p, nil),
		}
		d.CachePolicies = append(d.CachePolicies, ref)
		if p.General.ID == sum.CachePolicyID {
			d.CachePolicyName = p.General.Name
		}
	}
	return d, nil
}

func installTriggerFromScripts(scripts []policySubsetScript) string {
	// Prefer parameter4, then scan 4–11.
	for _, s := range scripts {
		for i := 4; i <= 11; i++ {
			v := strings.TrimSpace(getScriptParam(&s, i))
			if v == "" {
				continue
			}
			if looksLikeInstallTrigger(v) {
				return normalizeTriggerForMatch(v)
			}
		}
	}
	return ""
}

func looksLikeInstallTrigger(s string) bool {
	t := normalizeTriggerForMatch(s)
	if installLikeParamRe.MatchString(t) {
		return true
	}
	// install-foo-next already covered; also bare install-foo
	if strings.HasPrefix(t, "install-") {
		return true
	}
	return false
}

func looksLikeCacheTrigger(s string) bool {
	t := normalizeTriggerForMatch(s)
	return cacheTriggerRe.MatchString(t) || strings.HasPrefix(t, "cache-") || strings.HasSuffix(t, "-cache")
}

func policyAppNameFromSS(ss *resourcePolicy, installTrig string) string {
	if installTrig != "" {
		if n, _, _ := parseTrigger(installTrig); n != "" {
			return n
		}
	}
	display := strings.TrimSpace(ss.SelfService.SelfServiceDisplayName)
	if display == "" {
		display = ss.General.Name
	}
	display = ssNameMarkerRe.ReplaceAllString(display, "")
	display = strings.TrimSpace(display)
	// Strip common "Install " prefix.
	if strings.HasPrefix(strings.ToLower(display), "install ") {
		display = strings.TrimSpace(display[len("Install "):])
	}
	display = strings.TrimSpace(display)
	// Drop trailing TROUBLESHOOTING-style suffixes after " - ".
	if i := strings.Index(display, " - "); i > 0 {
		display = strings.TrimSpace(display[:i])
	}
	slug := strings.ToLower(display)
	slug = strings.ReplaceAll(slug, " ", "-")
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	return strings.Trim(slug, "-")
}

func inferInstallTrigger(credName, appName string, list []policyListItem, logger *slog.Logger) string {
	if appName == "" {
		return ""
	}
	candidates := []string{
		"install-" + appName + "-next",
		"install-" + appName,
		appName + "-install-next",
		appName + "-install",
	}
	for _, trig := range candidates {
		if len(findPoliciesByTrigger(credName, list, trig, logger)) > 0 {
			return trig
		}
	}
	return ""
}

func findPoliciesByTrigger(credName string, list []policyListItem, trigger string, logger *slog.Logger) []*resourcePolicy {
	want := normalizeTriggerForMatch(trigger)
	if want == "" {
		return nil
	}
	nameHint, _, _ := parseTrigger(want)
	var out []*resourcePolicy
	for _, item := range list {
		n := strings.ToLower(item.Name)
		if nameHint != "" && !strings.Contains(n, strings.ToLower(nameHint)) &&
			!strings.Contains(n, "install") && !strings.Contains(n, "cache") {
			continue
		}
		p, err := policyGetByID(credName, item.ID)
		if err != nil {
			continue
		}
		got := normalizeTriggerForMatch(p.General.TriggerOther)
		if got == want {
			out = append(out, p)
		}
	}
	return out
}

func findCachePoliciesForApp(credName string, list []policyListItem, appName string, logger *slog.Logger) []*resourcePolicy {
	appName = strings.ToLower(strings.TrimSpace(appName))
	if appName == "" {
		return nil
	}
	var out []*resourcePolicy
	for _, item := range list {
		n := strings.ToLower(item.Name)
		if !strings.Contains(n, "cache") && !strings.Contains(n, appName) {
			continue
		}
		p, err := policyGetByID(credName, item.ID)
		if err != nil {
			continue
		}
		trig := normalizeTriggerForMatch(p.General.TriggerOther)
		if !looksLikeCacheTrigger(trig) {
			continue
		}
		name, _, _ := parseTrigger(trig)
		if !strings.EqualFold(name, appName) {
			continue
		}
		out = append(out, p)
	}
	// Newest version first when comparable.
	sort.Slice(out, func(i, j int) bool {
		vi := policyAppVersion(out[i], nil)
		vj := policyAppVersion(out[j], nil)
		if vi != "" && vj != "" {
			return CompareVersions(vi, vj) > 0
		}
		return out[i].General.ID > out[j].General.ID
	})
	return out
}

func pickCurrentCache(caches []*resourcePolicy, preferVersion string) *resourcePolicy {
	if len(caches) == 0 {
		return nil
	}
	if preferVersion != "" {
		for _, p := range caches {
			if CompareVersions(policyAppVersion(p, nil), preferVersion) == 0 {
				return p
			}
		}
	}
	for _, p := range caches {
		if p.General.Enabled {
			return p
		}
	}
	return caches[0]
}

func policyAppVersion(p *resourcePolicy, ss *resourcePolicy) string {
	if p != nil {
		if len(p.PackageConfiguration.Packages) > 0 {
			if v := ExtractVersion(p.PackageConfiguration.Packages[0].Name); v != "" {
				return v
			}
		}
		trig := strings.ToLower(strings.TrimSpace(p.General.TriggerOther))
		name, ver, _ := parseTrigger(trig)
		_ = name
		if ver != "" {
			return ver
		}
		if v := versionFromName(p.General.Name); v != "" {
			return v
		}
	}
	if ss != nil {
		if v := ExtractVersion(ss.SelfService.SelfServiceDescription); v != "" {
			return v
		}
		if v := versionFromName(ss.General.Name); v != "" {
			return v
		}
	}
	return ""
}
