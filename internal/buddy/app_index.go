package buddy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	appIndexFileName = "index.json"
	appIndexVersion  = 1
	defaultAppsDir   = "apps"
)

// AppIndex is the curated local registry of imported policy apps.
type AppIndex struct {
	Version int                      `json:"version"`
	Apps    map[string]AppIndexEntry `json:"apps"`
}

// AppIndexEntry is one imported app binding (policy IDs + triggers + script).
type AppIndexEntry struct {
	Name               string    `json:"name"`
	Kind               string    `json:"kind"`
	Version            string    `json:"version,omitempty"`
	Script             string    `json:"script"`
	SSPolicyID         int       `json:"ss_policy_id"`
	InstallPolicyIDs   []int     `json:"install_policy_ids,omitempty"`
	CachePolicyIDs     []int     `json:"cache_policy_ids,omitempty"`
	UninstallPolicyIDs []int     `json:"uninstall_policy_ids,omitempty"`
	InstallTrigger     string    `json:"install_trigger,omitempty"`
	CacheTrigger       string    `json:"cache_trigger,omitempty"`
	UninstallTrigger   string    `json:"uninstall_trigger,omitempty"`
	PackagePattern     string    `json:"package_pattern,omitempty"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// DefaultAppsDir returns the default apps registry directory (cwd-relative).
func DefaultAppsDir() string { return defaultAppsDir }

// AppIndexPath returns appsDir/index.json.
func AppIndexPath(appsDir string) string {
	if strings.TrimSpace(appsDir) == "" {
		appsDir = defaultAppsDir
	}
	return filepath.Join(appsDir, appIndexFileName)
}

// LoadAppIndex reads apps/index.json. Missing file returns an empty index (no error).
func LoadAppIndex(appsDir string) (*AppIndex, error) {
	path := AppIndexPath(appsDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &AppIndex{Version: appIndexVersion, Apps: map[string]AppIndexEntry{}}, nil
		}
		return nil, err
	}
	var idx AppIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if idx.Apps == nil {
		idx.Apps = map[string]AppIndexEntry{}
	}
	if idx.Version == 0 {
		idx.Version = appIndexVersion
	}
	return &idx, nil
}

// SaveAppIndex writes apps/index.json (creates appsDir).
func SaveAppIndex(appsDir string, idx *AppIndex) error {
	if idx == nil {
		return fmt.Errorf("nil app index")
	}
	if idx.Apps == nil {
		idx.Apps = map[string]AppIndexEntry{}
	}
	if idx.Version == 0 {
		idx.Version = appIndexVersion
	}
	if strings.TrimSpace(appsDir) == "" {
		appsDir = defaultAppsDir
	}
	if err := os.MkdirAll(appsDir, 0o755); err != nil {
		return fmt.Errorf("create apps dir: %w", err)
	}
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	path := AppIndexPath(appsDir)
	return os.WriteFile(path, data, 0o644)
}

// UpsertAppIndexEntry sets or replaces one app entry keyed by normalized slug.
func UpsertAppIndexEntry(appsDir string, entry AppIndexEntry) error {
	idx, err := LoadAppIndex(appsDir)
	if err != nil {
		return err
	}
	slug := normalizeAppSlugKey(entry.Name)
	if slug == "" {
		return fmt.Errorf("app name is required")
	}
	entry.Name = slug
	if entry.Script == "" {
		entry.Script = slug + ".buddy"
	}
	entry.UpdatedAt = time.Now().UTC()
	idx.Apps[slug] = entry
	return SaveAppIndex(appsDir, idx)
}

// LookupAppIndexEntry finds an imported app by name (exact slug or compatible).
func LookupAppIndexEntry(appsDir, name string) (AppIndexEntry, bool, error) {
	idx, err := LoadAppIndex(appsDir)
	if err != nil {
		return AppIndexEntry{}, false, err
	}
	query := normalizeAppSlugKey(name)
	if query == "" {
		return AppIndexEntry{}, false, nil
	}
	if e, ok := idx.Apps[query]; ok {
		return e, true, nil
	}
	var fuzzy AppIndexEntry
	fuzzyOK := false
	for _, e := range idx.Apps {
		if appSlugsCompatible(e.Name, query) {
			if fuzzyOK {
				return AppIndexEntry{}, false, fmt.Errorf("ambiguous app name %q matches %q and %q; use the exact slug", name, fuzzy.Name, e.Name)
			}
			fuzzy = e
			fuzzyOK = true
		}
	}
	return fuzzy, fuzzyOK, nil
}

// AppListImported returns curated apps from apps/index.json (no Jamf calls).
func AppListImported(appsDir, filter string) ([]PolicyAppSummary, error) {
	idx, err := LoadAppIndex(appsDir)
	if err != nil {
		return nil, err
	}
	out := make([]PolicyAppSummary, 0, len(idx.Apps))
	for _, e := range idx.Apps {
		sum := policyAppSummaryFromIndex(e)
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

func policyAppSummaryFromIndex(e AppIndexEntry) PolicyAppSummary {
	sum := PolicyAppSummary{
		Name:            e.Name,
		Kind:            PolicyAppKind(e.Kind),
		Version:         e.Version,
		SSPolicyID:      e.SSPolicyID,
		InstallTrigger:  e.InstallTrigger,
		PackageName:     e.PackagePattern,
	}
	if len(e.InstallPolicyIDs) > 0 {
		sum.InstallPolicyID = e.InstallPolicyIDs[0]
	}
	if len(e.CachePolicyIDs) > 0 {
		sum.CachePolicyID = e.CachePolicyIDs[0]
	}
	return sum
}
