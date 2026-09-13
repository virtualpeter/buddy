package buddy

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"buddy/internal/buddy/secrets"
)

const (
	policyCacheManifestName  = "manifest.json"
	policyCacheSummariesName = "summaries.json"
	// PolicyCacheStaleAfter is how long a policy payload cache stays fresh.
	PolicyCacheStaleAfter = 72 * time.Hour
)

// PolicyCacheManifest describes a tenant's on-disk Classic policy cache.
type PolicyCacheManifest struct {
	FetchedAt time.Time `json:"fetched_at"`
	Tenant    string    `json:"tenant"`
	Auth      string    `json:"auth,omitempty"`
	Count     int       `json:"count"`
}

// PolicyCacheSummary is a compact row used for import related-policy scans.
type PolicyCacheSummary struct {
	ID                int      `json:"id"`
	Name              string   `json:"name"`
	Trigger           string   `json:"trigger,omitempty"`
	Enabled           bool     `json:"enabled"`
	UseForSelfService bool     `json:"use_for_self_service"`
	PackageNames      []string `json:"package_names,omitempty"`
	ScriptParams      []string `json:"script_params,omitempty"`
}

// PolicyCacheStatus is a human-facing cache status snapshot.
type PolicyCacheStatus struct {
	Dir       string
	Exists    bool
	Manifest  PolicyCacheManifest
	Stale     bool
	Age       time.Duration
	Summaries int
}

func policyCacheTenantKey(credName string) (string, error) {
	sess, err := ResolveSessionForCred(credName, OpClassic)
	if err != nil {
		return "", err
	}
	key := sess.AuthName + "@" + sess.TenantName
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("empty policy cache key")
	}
	replacer := strings.NewReplacer("/", "_", "\\", "_", "..", "_", ":", "_")
	return replacer.Replace(key), nil
}

// PolicyCacheTenantDir returns ~/.config/buddy/policy-cache/<auth@tenant>/.
func PolicyCacheTenantDir(credName string) (string, error) {
	root, err := secrets.PolicyCacheDir()
	if err != nil {
		return "", err
	}
	key, err := policyCacheTenantKey(credName)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, key), nil
}

// LoadPolicyCacheManifest loads manifest.json; missing → nil, nil.
func LoadPolicyCacheManifest(credName string) (*PolicyCacheManifest, error) {
	dir, err := PolicyCacheTenantDir(credName)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, policyCacheManifestName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var m PolicyCacheManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse policy cache manifest: %w", err)
	}
	return &m, nil
}

// PolicyCacheNeedsRefresh reports whether the cache is missing or older than PolicyCacheStaleAfter.
func PolicyCacheNeedsRefresh(m *PolicyCacheManifest, now time.Time) bool {
	if m == nil || m.FetchedAt.IsZero() {
		return true
	}
	if now.IsZero() {
		now = time.Now()
	}
	return now.Sub(m.FetchedAt) > PolicyCacheStaleAfter
}

// GetPolicyCacheStatus returns status for the active session's policy cache.
func GetPolicyCacheStatus(credName string) (PolicyCacheStatus, error) {
	dir, err := PolicyCacheTenantDir(credName)
	if err != nil {
		return PolicyCacheStatus{}, err
	}
	st := PolicyCacheStatus{Dir: dir}
	m, err := LoadPolicyCacheManifest(credName)
	if err != nil {
		return st, err
	}
	if m == nil {
		st.Stale = true
		return st, nil
	}
	st.Exists = true
	st.Manifest = *m
	st.Age = time.Since(m.FetchedAt)
	st.Stale = PolicyCacheNeedsRefresh(m, time.Now())
	sums, _ := LoadPolicyCacheSummaries(credName)
	st.Summaries = len(sums)
	return st, nil
}

// FormatPolicyCacheLine is a one-line cache age summary (fetched date + age).
func FormatPolicyCacheLine(st PolicyCacheStatus) string {
	if !st.Exists {
		return "Policy cache: missing"
	}
	age := st.Age.Round(time.Minute)
	state := "fresh"
	if st.Stale {
		state = "stale"
	}
	n := st.Manifest.Count
	if n == 0 {
		n = st.Summaries
	}
	return fmt.Sprintf("Policy cache: %s (%d policies, fetched %s, age %s)",
		state, n, st.Manifest.FetchedAt.UTC().Format(time.RFC3339), age)
}

// PolicyCacheInfo is the JSON/Starlark view of PolicyCacheStatus.
type PolicyCacheInfo struct {
	Exists     bool   `json:"exists"`
	Stale      bool   `json:"stale"`
	FetchedAt  string `json:"fetched_at,omitempty"`
	AgeSeconds int    `json:"age_seconds"`
	Count      int    `json:"count"`
	Tenant     string `json:"tenant,omitempty"`
	Dir        string `json:"dir,omitempty"`
}

// Info returns a JSON-friendly snapshot of the cache (fetched date, age, count).
func (st PolicyCacheStatus) Info() PolicyCacheInfo {
	info := PolicyCacheInfo{
		Exists:     st.Exists,
		Stale:      st.Stale,
		AgeSeconds: int(st.Age.Seconds()),
		Count:      st.Manifest.Count,
		Tenant:     st.Manifest.Tenant,
		Dir:        st.Dir,
	}
	if info.Count == 0 {
		info.Count = st.Summaries
	}
	if !st.Manifest.FetchedAt.IsZero() {
		info.FetchedAt = st.Manifest.FetchedAt.UTC().Format(time.RFC3339)
	}
	return info
}

// PrintPolicyCacheLine writes FormatPolicyCacheLine for the active session.
// Errors resolving the cache dir are ignored so listing apps still works.
func PrintPolicyCacheLine(credName string) {
	st, err := GetPolicyCacheStatus(credName)
	if err != nil {
		return
	}
	fmt.Println(FormatPolicyCacheLine(st))
}

// PrintPolicyCacheStatus writes a short status line to stdout.
func PrintPolicyCacheStatus(credName string) error {
	st, err := GetPolicyCacheStatus(credName)
	if err != nil {
		return err
	}
	fmt.Println(FormatPolicyCacheLine(st))
	if !st.Exists {
		fmt.Printf("  dir: %s\n", st.Dir)
		return nil
	}
	fmt.Printf("  tenant: %s\n  dir: %s\n", st.Manifest.Tenant, st.Dir)
	return nil
}

// LoadPolicyCacheSummaries loads summaries.json; missing → empty slice.
func LoadPolicyCacheSummaries(credName string) ([]PolicyCacheSummary, error) {
	dir, err := PolicyCacheTenantDir(credName)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, policyCacheSummariesName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var sums []PolicyCacheSummary
	if err := json.Unmarshal(data, &sums); err != nil {
		return nil, fmt.Errorf("parse policy cache summaries: %w", err)
	}
	return sums, nil
}

// PolicyCacheGet loads a policy from disk cache, falling back to a live Classic GET.
func PolicyCacheGet(credName string, policyID int, logger *slog.Logger) (*resourcePolicy, error) {
	if policyID <= 0 {
		return nil, fmt.Errorf("policy id is required")
	}
	dir, err := PolicyCacheTenantDir(credName)
	if err == nil {
		path := filepath.Join(dir, strconv.Itoa(policyID)+".xml")
		if data, rerr := os.ReadFile(path); rerr == nil {
			var p resourcePolicy
			if uerr := xml.Unmarshal(data, &p); uerr == nil {
				return &p, nil
			}
		}
	}
	if logger != nil {
		logger.Debug("policy cache miss; live GET", "id", policyID)
	}
	return policyGetByID(credName, policyID)
}

// PolicyCacheRefresh rebuilds the full Classic policy payload cache for the session.
func PolicyCacheRefresh(credName string, logger *slog.Logger) error {
	indexCtx := jamfProIndexContext(credName)

	dir, err := PolicyCacheTenantDir(credName)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create policy cache dir: %w", err)
	}
	list, err := getPolicyList(credName, logger)
	if err != nil {
		return err
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })

	sess, _ := ResolveSessionForCred(credName, OpClassic)
	tenantName, authName := "", ""
	if sess != nil {
		tenantName = sess.TenantName
		authName = sess.AuthName
	}

	summaries := make([]PolicyCacheSummary, 0, len(list))
	okCount := 0
	for i, item := range list {
		path := fmt.Sprintf("%s/id/%d", uriPolicies, item.ID)
		body, gerr := jamfGETWithAccept(credName, path, "application/xml")
		if gerr != nil {
			if logger != nil {
				logger.Warn("policy cache: skip GET", "id", item.ID, "error", gerr.Error())
			}
			continue
		}
		xmlPath := filepath.Join(dir, strconv.Itoa(item.ID)+".xml")
		if werr := os.WriteFile(xmlPath, body, 0o600); werr != nil {
			return fmt.Errorf("write policy cache %d: %w", item.ID, werr)
		}
		var p resourcePolicy
		if uerr := xml.Unmarshal(body, &p); uerr != nil {
			if logger != nil {
				logger.Warn("policy cache: skip parse", "id", item.ID, "error", uerr.Error())
			}
			continue
		}
		summaries = append(summaries, summaryFromResourcePolicy(&p))
		okCount++
		if (i+1)%25 == 0 || i+1 == len(list) {
			fmt.Printf("  cached %d/%d policies…\n", i+1, len(list))
		}
	}

	sumData, err := json.MarshalIndent(summaries, "", "  ")
	if err != nil {
		return err
	}
	sumData = append(sumData, '\n')
	if err := os.WriteFile(filepath.Join(dir, policyCacheSummariesName), sumData, 0o600); err != nil {
		return err
	}

	manifest := PolicyCacheManifest{
		FetchedAt: time.Now().UTC(),
		Tenant:    tenantName,
		Auth:      authName,
		Count:     okCount,
	}
	mData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	mData = append(mData, '\n')
	if err := os.WriteFile(filepath.Join(dir, policyCacheManifestName), mData, 0o600); err != nil {
		return err
	}
	fmt.Printf("Policy cache refreshed: %d policies → %s (fetched %s)\n", okCount, dir, manifest.FetchedAt.UTC().Format(time.RFC3339))
	if indexCtx != "" {
		fmt.Printf("  Policy index via product tenant %s\n", indexCtx)
	}
	return nil
}

// EnsurePolicyCache prompts to refresh when missing or older than 3 days.
// Missing + decline → error. Stale + decline → warn and continue.
func EnsurePolicyCache(credName string, opts Opts, logger *slog.Logger) error {
	m, err := LoadPolicyCacheManifest(credName)
	if err != nil {
		return err
	}
	PrintPolicyCacheLine(credName)
	if !PolicyCacheNeedsRefresh(m, time.Now()) {
		return nil
	}
	missing := m == nil || m.FetchedAt.IsZero()
	question := "Policy cache is older than 3 days. Refresh now?"
	if missing {
		question = "Policy cache is missing. Build it now? (required for import)"
	}
	doRefresh := opts.SkipConfirm
	if !doRefresh {
		doRefresh = AskYesNo(question, true, logger)
	}
	if doRefresh {
		return PolicyCacheRefresh(credName, logger)
	}
	if missing {
		return fmt.Errorf("policy cache is required; run: buddy app cache refresh")
	}
	fmt.Println("Continuing with stale policy cache.")
	return nil
}

func summaryFromResourcePolicy(p *resourcePolicy) PolicyCacheSummary {
	if p == nil {
		return PolicyCacheSummary{}
	}
	sum := PolicyCacheSummary{
		ID:                p.General.ID,
		Name:              p.General.Name,
		Trigger:           strings.TrimSpace(p.General.TriggerOther),
		Enabled:           p.General.Enabled,
		UseForSelfService: p.SelfService.UseForSelfService,
	}
	for _, pkg := range p.PackageConfiguration.Packages {
		if pkg.Name != "" {
			sum.PackageNames = append(sum.PackageNames, pkg.Name)
		}
	}
	for _, s := range p.Scripts {
		for _, param := range []string{s.Parameter4, s.Parameter5, s.Parameter6, s.Parameter7, s.Parameter8, s.Parameter9, s.Parameter10, s.Parameter11} {
			param = strings.TrimSpace(param)
			if param != "" {
				sum.ScriptParams = append(sum.ScriptParams, param)
			}
		}
	}
	return sum
}
