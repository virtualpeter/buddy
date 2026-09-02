package buddy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAppIndexRoundTrip(t *testing.T) {
	dir := t.TempDir()
	appsDir := filepath.Join(dir, "apps")
	entry := AppIndexEntry{
		Name:               "santa",
		Kind:               "cache",
		Version:            "2026.3",
		Script:             "santa.buddy",
		SSPolicyID:         21,
		InstallPolicyIDs:   []int{23},
		CachePolicyIDs:     []int{22},
		UninstallPolicyIDs: []int{99},
		InstallTrigger:     "install-santa-next",
		CacheTrigger:       "cache-santa-2026.3",
		UninstallTrigger:   "uninstall-santa",
		PackagePattern:     "*santa*2026.3*",
	}
	if err := UpsertAppIndexEntry(appsDir, entry); err != nil {
		t.Fatal(err)
	}
	got, ok, err := LookupAppIndexEntry(appsDir, "santa")
	if err != nil || !ok {
		t.Fatalf("lookup: ok=%v err=%v", ok, err)
	}
	if got.SSPolicyID != 21 || got.UninstallTrigger != "uninstall-santa" {
		t.Fatalf("entry=%+v", got)
	}
	list, err := AppListImported(appsDir, "santa")
	if err != nil || len(list) != 1 || list[0].Name != "santa" {
		t.Fatalf("list=%v err=%v", list, err)
	}
	if _, err := os.Stat(AppIndexPath(appsDir)); err != nil {
		t.Fatal(err)
	}
}

func TestLookupAppIndexCompatible(t *testing.T) {
	dir := t.TempDir()
	appsDir := filepath.Join(dir, "apps")
	if err := UpsertAppIndexEntry(appsDir, AppIndexEntry{
		Name: "cursor-ide", Kind: "install", SSPolicyID: 1, Script: "cursor-ide.buddy",
	}); err != nil {
		t.Fatal(err)
	}
	got, ok, err := LookupAppIndexEntry(appsDir, "cursor")
	if err != nil || !ok || got.Name != "cursor-ide" {
		t.Fatalf("got=%+v ok=%v err=%v", got, ok, err)
	}
}

func TestPolicyCacheStatusInfo(t *testing.T) {
	fetched := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	info := (PolicyCacheStatus{
		Exists:    true,
		Stale:     false,
		Age:       90 * time.Minute,
		Dir:       "/tmp/cache",
		Manifest:  PolicyCacheManifest{FetchedAt: fetched, Count: 42, Tenant: "sandbox"},
		Summaries: 42,
	}).Info()
	if !info.Exists || info.Stale || info.Count != 42 || info.FetchedAt != "2026-08-10T12:00:00Z" || info.Tenant != "sandbox" {
		t.Fatalf("%+v", info)
	}
	if (PolicyCacheStatus{}).Info().Exists {
		t.Fatal("missing cache should not exist")
	}
}

func TestFormatPolicyCacheLine(t *testing.T) {
	if got := FormatPolicyCacheLine(PolicyCacheStatus{}); got != "Policy cache: missing" {
		t.Fatalf("missing: %q", got)
	}
	fetched := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	got := FormatPolicyCacheLine(PolicyCacheStatus{
		Exists:   true,
		Stale:    false,
		Age:      90 * time.Minute,
		Manifest: PolicyCacheManifest{FetchedAt: fetched, Count: 42},
	})
	want := "Policy cache: fresh (42 policies, fetched 2026-08-10T12:00:00Z, age 1h30m0s)"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	got = FormatPolicyCacheLine(PolicyCacheStatus{
		Exists:   true,
		Stale:    true,
		Age:      4 * 24 * time.Hour,
		Manifest: PolicyCacheManifest{FetchedAt: fetched, Count: 10},
	})
	if !strings.Contains(got, "stale") || !strings.Contains(got, "fetched 2026-08-10T12:00:00Z") {
		t.Fatalf("stale: %q", got)
	}
}

func TestPolicyCacheNeedsRefresh(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	if !PolicyCacheNeedsRefresh(nil, now) {
		t.Fatal("nil should need refresh")
	}
	fresh := &PolicyCacheManifest{FetchedAt: now.Add(-24 * time.Hour)}
	if PolicyCacheNeedsRefresh(fresh, now) {
		t.Fatal("24h should be fresh")
	}
	stale := &PolicyCacheManifest{FetchedAt: now.Add(-4 * 24 * time.Hour)}
	if !PolicyCacheNeedsRefresh(stale, now) {
		t.Fatal("4d should be stale")
	}
}

func TestRankRelatedPolicies(t *testing.T) {
	sums := []PolicyCacheSummary{
		{ID: 1, Name: "Install Santa [SS]", UseForSelfService: true, ScriptParams: []string{"install-santa-next"}},
		{ID: 2, Name: "Install Santa 2026.3", Trigger: "install-santa-next", Enabled: true},
		{ID: 3, Name: "Cache Santa 2026.3", Trigger: "cache-santa-2026.3", Enabled: true},
		{ID: 4, Name: "Uninstall Santa", Trigger: "uninstall-santa", Enabled: true},
		{ID: 5, Name: "Install Chrome [SS]", UseForSelfService: true},
	}
	g := RankRelatedPolicies("santa", sums)
	if len(g.SS) == 0 || g.SS[0].Summary.ID != 1 {
		t.Fatalf("SS=%+v", g.SS)
	}
	if len(g.Install) == 0 || g.Install[0].Summary.ID != 2 {
		t.Fatalf("Install=%+v", g.Install)
	}
	if len(g.Cache) == 0 || g.Cache[0].Summary.ID != 3 {
		t.Fatalf("Cache=%+v", g.Cache)
	}
	if len(g.Uninstall) == 0 || g.Uninstall[0].Summary.ID != 4 {
		t.Fatalf("Uninstall=%+v", g.Uninstall)
	}
	for _, h := range append(append(g.SS, g.Install...), append(g.Cache, g.Uninstall...)...) {
		if h.Summary.ID == 5 {
			t.Fatal("chrome should not match santa")
		}
	}
}

func TestParseIntCSVList(t *testing.T) {
	ids, err := ParseIntCSVList("1, 2,2, 3")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 3 || ids[0] != 1 || ids[2] != 3 {
		t.Fatalf("%v", ids)
	}
	if _, err := ParseIntCSVList("x"); err == nil {
		t.Fatal("expected error")
	}
}

func TestInferAppKind(t *testing.T) {
	if inferAppKind(AppIndexEntry{CachePolicyIDs: []int{1}}) != "cache" {
		t.Fatal("cache")
	}
	if inferAppKind(AppIndexEntry{InstallPolicyIDs: []int{1}}) != "install" {
		t.Fatal("install")
	}
	if inferAppKind(AppIndexEntry{SSPolicyID: 1}) != "single" {
		t.Fatal("single")
	}
}

func TestAppDesiredFromIndexEntryUninstall(t *testing.T) {
	des := appDesiredFromIndexEntry(AppIndexEntry{
		Name: "santa", Kind: "cache", Version: "2026.3",
		UninstallTrigger: "uninstall-santa",
		InstallTrigger:   "install-santa-next",
		CacheTrigger:     "cache-santa-2026.3",
	})
	body, err := RenderAppImportScript(des)
	if err != nil {
		t.Fatal(err)
	}
	if !containsAll(body, `UNINSTALL_TRIGGER = "uninstall-santa"`, `"uninstall_trigger": UNINSTALL_TRIGGER`) {
		t.Fatalf("script missing uninstall:\n%s", body)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !stringContains(s, p) {
			return false
		}
	}
	return true
}

func stringContains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i+len(sub) <= len(s); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
