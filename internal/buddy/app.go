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
	Name            string        `json:"name"`
	Kind            PolicyAppKind `json:"kind"`
	Version         string        `json:"version,omitempty"`
	SSPolicyID      int           `json:"ss_policy_id"`
	SSPolicyName    string        `json:"ss_policy_name,omitempty"`
	InstallTrigger  string        `json:"install_trigger,omitempty"`
	InstallPolicyID int           `json:"install_policy_id,omitempty"`
	CachePolicyID   int           `json:"cache_policy_id,omitempty"`
	PackageID       int           `json:"package_id,omitempty"`
	PackageName     string        `json:"package_name,omitempty"`
}

// PolicyAppDetail is AppGet output: summary plus linked policy IDs/names.
type PolicyAppDetail struct {
	PolicyAppSummary
	SSDisplayName       string         `json:"ss_display_name,omitempty"`
	SSDescription       string         `json:"ss_description,omitempty"`
	InstallPolicyName   string         `json:"install_policy_name,omitempty"`
	CachePolicyName     string         `json:"cache_policy_name,omitempty"`
	UninstallTrigger    string         `json:"uninstall_trigger,omitempty"`
	UninstallPolicyID   int            `json:"uninstall_policy_id,omitempty"`
	UninstallPolicyName string         `json:"uninstall_policy_name,omitempty"`
	CachePolicies       []PolicyAppRef `json:"cache_policies,omitempty"`
	InstallPolicies     []PolicyAppRef `json:"install_policies,omitempty"`
	UninstallPolicies   []PolicyAppRef `json:"uninstall_policies,omitempty"`
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
// Uses the on-disk policy cache when present (no Jamf call); otherwise one hybrid
// policy list call and name-only heuristics (no per-policy Classic GETs).
// Kind/version/trigger may be approximate; AppGet does full XML classify.
// filter is an optional client-side substring on name / kind / version / trigger / package.
func AppSearch(credName, filter string, logger *slog.Logger) ([]PolicyAppSummary, error) {
	res, err := AppScan(credName, AppScanOpts{Filter: filter}, logger)
	if err != nil {
		return nil, err
	}
	return res.Apps, nil
}

// AppScanOpts configures buddy app scan.
type AppScanOpts struct {
	Filter  string
	Refresh bool // rebuild the Classic policy cache, then scan it
}

// AppScanResult is a heuristic app discovery pass plus which source it used.
type AppScanResult struct {
	Apps   []PolicyAppSummary
	Source string // "cache" or "live"
	Cache  PolicyCacheStatus
	// IndexContext is a Jamf Pro product-tenant workplace when scan relocated
	// from an environment so undocumented GET /api/v1/policies can run.
	IndexContext string `json:"index_context,omitempty"`
}

// AppScan discovers policy apps for onboarding. Prefers the policy payload cache
// (summaries.json) when present so a 1300-policy tenant is not listed live.
// --refresh rebuilds that cache first. Missing cache falls back to a live list.
func AppScan(credName string, opts AppScanOpts, logger *slog.Logger) (AppScanResult, error) {
	restore, pin, indexCtx := beginAppPolicyIndexSession(credName, logger)
	defer restore()
	credName = pin

	if opts.Refresh {
		if err := PolicyCacheRefresh(credName, logger); err != nil {
			return AppScanResult{}, err
		}
	}
	st, err := GetPolicyCacheStatus(credName)
	if err != nil {
		return AppScanResult{}, err
	}
	sums, err := LoadPolicyCacheSummaries(credName)
	if err != nil {
		return AppScanResult{}, err
	}
	if len(sums) > 0 {
		list := policyListFromCacheSummaries(sums)
		ss := ssCandidatesFromCacheSummaries(sums, list)
		apps := discoverPolicyApps(ss, list, opts.Filter)
		enrichScanPackagesFromCache(apps, sums)
		return AppScanResult{Apps: apps, Source: "cache", Cache: st, IndexContext: indexCtx}, nil
	}
	list, err := getPolicyList(credName, logger)
	if err != nil {
		return AppScanResult{}, err
	}
	var ss []policyListItem
	for _, c := range list {
		if ssNameMarkerRe.MatchString(c.Name) {
			ss = append(ss, c)
		}
	}
	apps := discoverPolicyApps(ss, list, opts.Filter)
	return AppScanResult{Apps: apps, Source: "live", Cache: st, IndexContext: indexCtx}, nil
}

func policyListFromCacheSummaries(sums []PolicyCacheSummary) []policyListItem {
	out := make([]policyListItem, 0, len(sums))
	for _, s := range sums {
		item := policyListItem{ID: s.ID, Name: s.Name}
		en := s.Enabled
		item.Enabled = &en
		if t := strings.TrimSpace(s.Trigger); t != "" {
			trig := t
			item.TriggerManual = &trig
		}
		out = append(out, item)
	}
	return out
}

func ssCandidatesFromCacheSummaries(sums []PolicyCacheSummary, list []policyListItem) []policyListItem {
	byID := make(map[int]policyListItem, len(list))
	for _, p := range list {
		byID[p.ID] = p
	}
	var out []policyListItem
	seen := map[int]bool{}
	for _, s := range sums {
		if !s.UseForSelfService && !ssNameMarkerRe.MatchString(s.Name) {
			continue
		}
		p, ok := byID[s.ID]
		if !ok || seen[s.ID] {
			continue
		}
		seen[s.ID] = true
		out = append(out, p)
	}
	return out
}

func enrichScanPackagesFromCache(apps []PolicyAppSummary, sums []PolicyCacheSummary) {
	byID := make(map[int]PolicyCacheSummary, len(sums))
	for _, s := range sums {
		byID[s.ID] = s
	}
	for i := range apps {
		if apps[i].PackageName != "" {
			continue
		}
		for _, id := range []int{apps[i].InstallPolicyID, apps[i].CachePolicyID, apps[i].SSPolicyID} {
			if id == 0 {
				continue
			}
			if s, ok := byID[id]; ok && len(s.PackageNames) > 0 {
				apps[i].PackageName = s.PackageNames[0]
				break
			}
		}
	}
}

func discoverPolicyApps(ssCandidates, list []policyListItem, filter string) []PolicyAppSummary {
	out := make([]PolicyAppSummary, 0)
	seen := map[string]bool{}
	for _, c := range ssCandidates {
		sum, ok := summarizePolicyAppFromList(c, list)
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
	return out
}

// AppGet returns one policy app by app name (case-insensitive).
// Prefers apps/index.json bindings when present; otherwise Classic XML classify.
// Query matching uses normalized slugs (hyphen/space equivalent) and accepts
// compatible forms (e.g. "cursor" matches SS slug "cursor-ide").
func AppGet(credName, name string, logger *slog.Logger) (*PolicyAppDetail, error) {
	return AppGetWithAppsDir(credName, DefaultAppsDir(), name, logger)
}

// AppGetWithAppsDir is AppGet with an explicit apps registry directory.
func AppGetWithAppsDir(credName, appsDir, name string, logger *slog.Logger) (*PolicyAppDetail, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("app name is required")
	}
	if entry, ok, err := LookupAppIndexEntry(appsDir, name); err != nil {
		return nil, err
	} else if ok {
		return AppGetFromIndex(credName, entry, logger)
	}
	return appGetHeuristic(credName, name, logger)
}

// AppGetFromIndex builds detail from a curated index entry (cache/live policy GETs).
func AppGetFromIndex(credName string, e AppIndexEntry, logger *slog.Logger) (*PolicyAppDetail, error) {
	sum := policyAppSummaryFromIndex(e)
	d := &PolicyAppDetail{
		PolicyAppSummary: sum,
		UninstallTrigger: e.UninstallTrigger,
	}
	if e.SSPolicyID > 0 {
		ss, err := PolicyCacheGet(credName, e.SSPolicyID, logger)
		if err != nil {
			return nil, fmt.Errorf("ss policy %d: %w", e.SSPolicyID, err)
		}
		d.SSPolicyName = ss.General.Name
		d.SSDisplayName = ss.SelfService.SelfServiceDisplayName
		d.SSDescription = ss.SelfService.SelfServiceDescription
		if d.Version == "" {
			d.Version = ExtractVersion(ss.SelfService.SelfServiceDescription)
		}
	}
	for i, id := range e.InstallPolicyIDs {
		p, err := PolicyCacheGet(credName, id, logger)
		if err != nil {
			return nil, fmt.Errorf("install policy %d: %w", id, err)
		}
		ref := refFromPolicy(p)
		d.InstallPolicies = append(d.InstallPolicies, ref)
		if i == 0 || (p.General.Enabled && d.InstallPolicyID == e.InstallPolicyIDs[0]) {
			d.InstallPolicyID = p.General.ID
			d.InstallPolicyName = p.General.Name
			if len(p.PackageConfiguration.Packages) > 0 {
				d.PackageID = p.PackageConfiguration.Packages[0].ID
				d.PackageName = p.PackageConfiguration.Packages[0].Name
			}
			if ver := policyAppVersion(p, nil); ver != "" {
				d.Version = ver
			}
		}
	}
	for _, id := range e.CachePolicyIDs {
		p, err := PolicyCacheGet(credName, id, logger)
		if err != nil {
			return nil, fmt.Errorf("cache policy %d: %w", id, err)
		}
		d.CachePolicies = append(d.CachePolicies, refFromPolicy(p))
	}
	if cur := selectCurrentCacheRef(d.CachePolicies, d.Version); cur.ID > 0 {
		d.CachePolicyID = cur.ID
		d.CachePolicyName = cur.Name
		if d.Version == "" && cur.Version != "" {
			d.Version = cur.Version
		}
		if d.PackageID == 0 {
			for _, id := range e.CachePolicyIDs {
				if id != cur.ID {
					continue
				}
				if p, err := PolicyCacheGet(credName, id, logger); err == nil && len(p.PackageConfiguration.Packages) > 0 {
					d.PackageID = p.PackageConfiguration.Packages[0].ID
					d.PackageName = p.PackageConfiguration.Packages[0].Name
				}
				break
			}
		}
	}
	for i, id := range e.UninstallPolicyIDs {
		p, err := PolicyCacheGet(credName, id, logger)
		if err != nil {
			return nil, fmt.Errorf("uninstall policy %d: %w", id, err)
		}
		ref := refFromPolicy(p)
		d.UninstallPolicies = append(d.UninstallPolicies, ref)
		if i == 0 {
			d.UninstallPolicyID = p.General.ID
			d.UninstallPolicyName = p.General.Name
			if d.UninstallTrigger == "" {
				d.UninstallTrigger = strings.TrimSpace(p.General.TriggerOther)
			}
		}
	}
	if e.Version != "" && d.Version == "" {
		d.Version = e.Version
	}
	return d, nil
}

func refFromPolicy(p *resourcePolicy) PolicyAppRef {
	if p == nil {
		return PolicyAppRef{}
	}
	return PolicyAppRef{
		ID:      p.General.ID,
		Name:    p.General.Name,
		Trigger: strings.TrimSpace(p.General.TriggerOther),
		Enabled: p.General.Enabled,
		Version: policyAppVersion(p, nil),
	}
}

// appGetHeuristic discovers an app via [SS] name heuristics + Classic classify.
func appGetHeuristic(credName, name string, logger *slog.Logger) (*PolicyAppDetail, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("app name is required")
	}
	query := normalizeAppSlugKey(name)
	if query == "" {
		return nil, fmt.Errorf("app name is required")
	}
	list, err := getPolicyList(credName, logger)
	if err != nil {
		return nil, err
	}
	var exactID, fuzzyID int
	var exactName, fuzzyName string
	for _, p := range list {
		if !ssNameMarkerRe.MatchString(p.Name) {
			continue
		}
		slug := policyAppNameFromListName(p.Name)
		if slug == "" {
			continue
		}
		if slug == query {
			exactID = p.ID
			exactName = p.Name
			break
		}
		if fuzzyID == 0 && appSlugsCompatible(slug, query) {
			fuzzyID = p.ID
			fuzzyName = p.Name
		}
	}
	ssID := exactID
	ssListName := exactName
	if ssID == 0 {
		ssID = fuzzyID
		ssListName = fuzzyName
	}
	if ssID == 0 {
		return nil, fmt.Errorf("app %q not found (not in apps/index.json and no matching [SS] policy); try: buddy app scan --filter=%s", name, query)
	}
	cache := newPolicyFetchCache(credName)
	sum, ok, err := classifyPolicyAppFromSS(credName, ssID, list, cache, logger)
	if err != nil {
		return nil, err
	}
	if !ok {
		label := ssListName
		if label == "" {
			label = fmt.Sprintf("id %d", ssID)
		}
		return nil, fmt.Errorf("app %q matches Self Service policy %q but could not classify it (need an Install package on the SS policy, or an install-like script trigger / sibling install policy)", name, label)
	}
	return enrichPolicyAppDetail(credName, sum, list, cache, logger)
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
	cacheTrig := cacheTriggerFromDetail(d)
	// Keep prior versioned cache for rollback. Prefer pinned cache policy ID over trigger search.
	if err := PolicyPackage(credName, opts, d.Name, version, d.CachePolicyID, cacheTrig, packagePattern, DisableSourceKeep, logger); err != nil {
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
	if strings.Contains(hay, filter) {
		return true
	}
	// Treat spaces and hyphens as equivalent (claude-cli ↔ "claude cli").
	return strings.Contains(normalizeAppSlugKey(hay), normalizeAppSlugKey(filter))
}

// policyFetchCache memoizes Classic policyGetByID within one classify/enrich pass.
type policyFetchCache struct {
	cred string
	m    map[int]*resourcePolicy
}

func newPolicyFetchCache(credName string) *policyFetchCache {
	return &policyFetchCache{cred: credName, m: map[int]*resourcePolicy{}}
}

func (c *policyFetchCache) get(id int) (*resourcePolicy, error) {
	if p, ok := c.m[id]; ok {
		return p, nil
	}
	p, err := PolicyCacheGet(c.cred, id, nil)
	if err != nil {
		return nil, err
	}
	c.m[id] = p
	return p, nil
}

// summarizePolicyAppFromList builds a list-row summary from policy names only (no GETs).
// Returns false for non-app [SS] stubs (e.g. inventory) that lack Install naming and
// install/cache sibling policies.
func summarizePolicyAppFromList(ss policyListItem, list []policyListItem) (PolicyAppSummary, bool) {
	name := policyAppNameFromListName(ss.Name)
	if name == "" {
		return PolicyAppSummary{}, false
	}
	slug := strings.ToLower(name)
	kind := PolicyAppSingle
	var version string
	var installID, cacheID int
	var bestCacheVer string
	var installTrigFromSibling string

	for _, p := range list {
		if p.ID == ss.ID {
			continue
		}
		if !policyNameContainsAppSlug(p.Name, slug) {
			continue
		}
		n := strings.ToLower(p.Name)
		if strings.Contains(n, "cache") {
			kind = PolicyAppCache
			ver := versionFromName(p.Name)
			if cacheID == 0 || (ver != "" && (bestCacheVer == "" || CompareVersions(ver, bestCacheVer) > 0)) {
				cacheID = p.ID
				if ver != "" {
					bestCacheVer = ver
					version = ver
				}
			}
			continue
		}
		if strings.Contains(n, "install") && !ssNameMarkerRe.MatchString(p.Name) {
			if kind != PolicyAppCache {
				kind = PolicyAppInstall
			}
			// Prefer siblings with a custom trigger (v1); SS/single rows usually have none.
			cur := listItemByID(list, installID)
			take := installID == 0 ||
				(policyListHasCustomTrigger(p) && !policyListHasCustomTrigger(cur))
			if take {
				installID = p.ID
				if t := policyListCustomTrigger(p); t != "" {
					installTrigFromSibling = t
				}
			}
			if version == "" {
				version = versionFromName(p.Name)
			}
		}
	}

	// Soft signal from v1: if the SS row itself has an install-like custom trigger, use it.
	// Empty triggerManual is typical for Self Service / single (not used alone for inclusion —
	// inventory SS often has none too).
	if ss.TriggerManual != nil && strings.TrimSpace(*ss.TriggerManual) != "" && looksLikeInstallTrigger(*ss.TriggerManual) {
		if kind == PolicyAppSingle {
			kind = PolicyAppInstall
		}
		installTrigFromSibling = normalizeTriggerForMatch(*ss.TriggerManual)
	}

	// Drop inventory/config [SS] stubs: require Install-named SS or install/cache siblings.
	if kind == PolicyAppSingle && !ssPolicyNameLooksLikeInstallApp(ss.Name) {
		return PolicyAppSummary{}, false
	}

	sum := PolicyAppSummary{
		Name:            name,
		Kind:            kind,
		Version:         version,
		SSPolicyID:      ss.ID,
		SSPolicyName:    ss.Name,
		InstallPolicyID: installID,
		CachePolicyID:   cacheID,
	}
	if kind == PolicyAppInstall || kind == PolicyAppCache {
		if installTrigFromSibling != "" {
			sum.InstallTrigger = installTrigFromSibling
		} else {
			// Best-effort trigger guess from naming convention.
			sum.InstallTrigger = "install-" + name + "-next"
		}
	}
	if sum.Version == "" {
		sum.Version = versionFromName(ss.Name)
	}
	return sum, true
}

func listItemByID(list []policyListItem, id int) policyListItem {
	for _, p := range list {
		if p.ID == id {
			return p
		}
	}
	return policyListItem{}
}

// policyListHasCustomTrigger is true when v1 reported a non-empty triggerManual.
func policyListHasCustomTrigger(p policyListItem) bool {
	return p.TriggerManual != nil && strings.TrimSpace(*p.TriggerManual) != ""
}

func policyListCustomTrigger(p policyListItem) string {
	if p.TriggerManual == nil {
		return ""
	}
	return normalizeTriggerForMatch(*p.TriggerManual)
}

// ssPolicyNameLooksLikeInstallApp reports whether an [SS] list name looks like a
// single-policy app (e.g. "Install Chrome [SS]") rather than inventory/config.
func ssPolicyNameLooksLikeInstallApp(policyName string) bool {
	s := ssNameMarkerRe.ReplaceAllString(policyName, " ")
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.HasPrefix(s, "install ") || strings.HasPrefix(s, "install-")
}

// normalizeAppSlugKey lowercases and collapses spaces/underscores to hyphens.
func normalizeAppSlugKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	b.Grow(len(s))
	prevHyphen := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevHyphen = false
		case r == ' ' || r == '-' || r == '_':
			if !prevHyphen && b.Len() > 0 {
				b.WriteByte('-')
				prevHyphen = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// appSlugTokens splits a name/slug into alphanumeric tokens (hyphen/space/_ agnostic).
func appSlugTokens(s string) []string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return nil
	}
	var out []string
	start := -1
	flush := func(i int) {
		if start >= 0 && i > start {
			out = append(out, s[start:i])
		}
		start = -1
	}
	for i := 0; i < len(s); i++ {
		if isAppSlugAlnum(s[i]) {
			if start < 0 {
				start = i
			}
			continue
		}
		flush(i)
	}
	flush(len(s))
	return out
}

// policyNameContainsAppSlug reports whether policyName contains appSlug as a
// contiguous token sequence (hyphens/spaces equivalent), avoiding short-slug
// false positives inside longer words.
func policyNameContainsAppSlug(policyName, appSlug string) bool {
	want := appSlugTokens(appSlug)
	if len(want) == 0 {
		return false
	}
	have := appSlugTokens(policyName)
	if len(have) < len(want) {
		return false
	}
	for i := 0; i+len(want) <= len(have); i++ {
		match := true
		for j := range want {
			if have[i+j] != want[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func isAppSlugAlnum(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
}

// policyAppNameFromListName derives an app slug from an SS policy list name.
func policyAppNameFromListName(policyName string) string {
	display := ssNameMarkerRe.ReplaceAllString(policyName, "")
	display = strings.TrimSpace(display)
	if strings.HasPrefix(strings.ToLower(display), "install ") {
		display = strings.TrimSpace(display[len("Install "):])
	}
	display = strings.TrimSpace(display)
	if i := strings.Index(display, " - "); i > 0 {
		display = strings.TrimSpace(display[:i])
	}
	// Drop trailing version tokens so "Santa 2026.2 [SS]" → santa (rare).
	if v := versionFromName(display); v != "" {
		display = strings.TrimSpace(strings.Replace(display, v, "", 1))
	}
	return normalizeAppSlugKey(display)
}

func classifyPolicyAppFromSS(credName string, ssID int, list []policyListItem, cache *policyFetchCache, logger *slog.Logger) (PolicyAppSummary, bool, error) {
	if cache == nil {
		cache = newPolicyFetchCache(credName)
	}
	ss, err := cache.get(ssID)
	if err != nil {
		return PolicyAppSummary{}, false, err
	}
	if !ss.SelfService.UseForSelfService {
		return PolicyAppSummary{}, false, nil
	}

	pkgs := ss.PackageConfiguration.Packages
	installTrig := installTriggerFromScripts(ss.Scripts)
	ssSlug := policyAppNameFromSS(ss, "")

	// single: packages on SS and no install-like script param
	if len(pkgs) > 0 && installTrig == "" {
		pkg := pkgs[0]
		ver := ExtractVersion(pkg.Name)
		if ver == "" {
			ver = versionFromName(ss.General.Name)
		}
		if ver == "" {
			ver = ExtractVersion(ss.SelfService.SelfServiceDescription)
		}
		return PolicyAppSummary{
			Name:         ssSlug,
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
		installTrig = inferInstallTrigger(credName, ssSlug, list, cache, logger)
		if installTrig == "" {
			return PolicyAppSummary{}, false, nil
		}
	}

	trigName, _, _ := parseTrigger(installTrig)
	appName := chooseAppCanonicalName(ssSlug, trigName)

	installs := findPoliciesByTrigger(credName, list, installTrig, cache, logger)
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

	caches := findCachePoliciesForApp(credName, list, appName, cache, logger)
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

func enrichPolicyAppDetail(credName string, sum PolicyAppSummary, list []policyListItem, cache *policyFetchCache, logger *slog.Logger) (*PolicyAppDetail, error) {
	if cache == nil {
		cache = newPolicyFetchCache(credName)
	}
	if list == nil {
		var err error
		list, err = getPolicyList(credName, logger)
		if err != nil {
			return nil, err
		}
	}
	d := &PolicyAppDetail{PolicyAppSummary: sum}
	if sum.SSPolicyID > 0 {
		if ss, err := cache.get(sum.SSPolicyID); err == nil {
			d.SSDisplayName = ss.SelfService.SelfServiceDisplayName
			d.SSDescription = ss.SelfService.SelfServiceDescription
		}
	}
	if sum.InstallTrigger != "" {
		for _, p := range findPoliciesByTrigger(credName, list, sum.InstallTrigger, cache, logger) {
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
	caches := findCachePoliciesForApp(credName, list, sum.Name, cache, logger)
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
	ssSlug := strings.Trim(slug, "-")
	if installTrig == "" {
		return ssSlug
	}
	trigName, _, _ := parseTrigger(installTrig)
	return chooseAppCanonicalName(ssSlug, trigName)
}

// chooseAppCanonicalName picks the discovery slug for an app.
// Prefer the Self Service-derived slug when it is equal to, or a more specific
// form of, the install-trigger slug (e.g. SS "cursor-ide" vs trigger "cursor").
func chooseAppCanonicalName(ssSlug, triggerSlug string) string {
	ssSlug = normalizeAppSlugKey(ssSlug)
	triggerSlug = normalizeAppSlugKey(triggerSlug)
	switch {
	case ssSlug == "" && triggerSlug == "":
		return ""
	case ssSlug == "":
		return triggerSlug
	case triggerSlug == "":
		return ssSlug
	case ssSlug == triggerSlug:
		return ssSlug
	case strings.HasPrefix(ssSlug, triggerSlug+"-"):
		return ssSlug
	case strings.HasPrefix(triggerSlug, ssSlug+"-"):
		return triggerSlug
	case policyNameContainsAppSlug(ssSlug, triggerSlug):
		return ssSlug
	case policyNameContainsAppSlug(triggerSlug, ssSlug):
		return triggerSlug
	default:
		// Conflicting names — prefer SS (the AppGet discovery key).
		return ssSlug
	}
}

// appSlugsCompatible reports whether two app slugs refer to the same app
// (exact, hyphen-boundary prefix, or token containment).
func appSlugsCompatible(a, b string) bool {
	a = normalizeAppSlugKey(a)
	b = normalizeAppSlugKey(b)
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	if strings.HasPrefix(a, b+"-") || strings.HasPrefix(b, a+"-") {
		return true
	}
	return policyNameContainsAppSlug(a, b) || policyNameContainsAppSlug(b, a)
}

func inferInstallTrigger(credName, appName string, list []policyListItem, cache *policyFetchCache, logger *slog.Logger) string {
	if appName == "" {
		return ""
	}
	for _, trig := range installTriggerCandidates(appName) {
		if len(findPoliciesByTrigger(credName, list, trig, cache, logger)) > 0 {
			return trig
		}
	}
	return ""
}

// installTriggerCandidates returns install trigger forms to try for an app slug,
// including shorter hyphen prefixes (cursor-ide → also try install-cursor).
func installTriggerCandidates(appName string) []string {
	appName = normalizeAppSlugKey(appName)
	if appName == "" {
		return nil
	}
	var names []string
	seen := map[string]bool{}
	addName := func(n string) {
		n = normalizeAppSlugKey(n)
		if n == "" || seen[n] {
			return
		}
		seen[n] = true
		names = append(names, n)
	}
	addName(appName)
	parts := strings.Split(appName, "-")
	for i := len(parts) - 1; i >= 1; i-- {
		addName(strings.Join(parts[:i], "-"))
	}
	var out []string
	for _, n := range names {
		out = append(out,
			"install-"+n+"-next",
			"install-"+n,
			n+"-install-next",
			n+"-install",
		)
	}
	return out
}

func findPoliciesByTrigger(credName string, list []policyListItem, trigger string, cache *policyFetchCache, logger *slog.Logger) []*resourcePolicy {
	want := normalizeTriggerForMatch(trigger)
	if want == "" {
		return nil
	}
	if cache == nil {
		cache = newPolicyFetchCache(credName)
	}
	nameHint, _, _ := parseTrigger(want)
	nameHint = strings.ToLower(nameHint)
	var out []*resourcePolicy
	for _, item := range list {
		if !policyListNameHasAppHint(item.Name, nameHint) {
			continue
		}
		p, err := cache.get(item.ID)
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

func findCachePoliciesForApp(credName string, list []policyListItem, appName string, cache *policyFetchCache, logger *slog.Logger) []*resourcePolicy {
	appName = strings.ToLower(strings.TrimSpace(appName))
	if appName == "" {
		return nil
	}
	if cache == nil {
		cache = newPolicyFetchCache(credName)
	}
	var out []*resourcePolicy
	for _, item := range list {
		// Require app name in the policy name (do not GET every "cache" policy on the tenant).
		if !policyListNameHasAppHint(item.Name, appName) {
			continue
		}
		p, err := cache.get(item.ID)
		if err != nil {
			continue
		}
		trig := normalizeTriggerForMatch(p.General.TriggerOther)
		if !looksLikeCacheTrigger(trig) {
			continue
		}
		name, _, _ := parseTrigger(trig)
		if !appSlugsCompatible(name, appName) {
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

// policyListNameHasAppHint is the Classic GET prefilter for trigger/cache scans:
// require the app name as a token sequence in the list name (empty hint matches all).
// Hyphens and spaces are equivalent (claude-cli ↔ "Claude CLI").
// Multi-token slugs also accept the primary (first) token so "Cache Cursor"
// is considered when classifying "cursor-ide".
func policyListNameHasAppHint(policyName, appHint string) bool {
	appHint = strings.TrimSpace(appHint)
	if appHint == "" {
		return true
	}
	if policyNameContainsAppSlug(policyName, appHint) {
		return true
	}
	tokens := appSlugTokens(appHint)
	if len(tokens) > 1 {
		return policyNameContainsAppSlug(policyName, tokens[0])
	}
	return false
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

// selectCurrentCacheRef picks the live cache policy to clone from: matching
// preferVersion, else newest enabled version, else newest version, else first enabled.
func selectCurrentCacheRef(refs []PolicyAppRef, preferVersion string) PolicyAppRef {
	if len(refs) == 0 {
		return PolicyAppRef{}
	}
	if preferVersion != "" {
		for _, r := range refs {
			if r.Version != "" && CompareVersions(r.Version, preferVersion) == 0 {
				return r
			}
		}
	}
	bestEnabled := PolicyAppRef{}
	bestAny := PolicyAppRef{}
	for _, r := range refs {
		if r.Version != "" && (bestAny.Version == "" || CompareVersions(r.Version, bestAny.Version) > 0) {
			bestAny = r
		}
		if r.Enabled && r.Version != "" && (bestEnabled.Version == "" || CompareVersions(r.Version, bestEnabled.Version) > 0) {
			bestEnabled = r
		}
	}
	if bestEnabled.ID != 0 {
		return bestEnabled
	}
	if bestAny.ID != 0 {
		return bestAny
	}
	for _, r := range refs {
		if r.Enabled {
			return r
		}
	}
	return refs[0]
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
