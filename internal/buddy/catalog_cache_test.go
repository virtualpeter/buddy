package buddy

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCatalogCacheStale(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	if !CatalogCacheStale(time.Time{}, now, CatalogCacheTTL) {
		t.Fatal("zero fetched_at should be stale")
	}
	fresh := now.Add(-24 * time.Hour)
	if CatalogCacheStale(fresh, now, CatalogCacheTTL) {
		t.Fatal("1d old should be fresh with 7d TTL")
	}
	old := now.Add(-8 * 24 * time.Hour)
	if !CatalogCacheStale(old, now, CatalogCacheTTL) {
		t.Fatal("8d old should be stale")
	}
}

func TestFormatCatalogCacheAge(t *testing.T) {
	t.Parallel()
	if got := FormatCatalogCacheAge(30 * time.Minute); got != "<1h" {
		t.Fatalf("got %q", got)
	}
	if got := FormatCatalogCacheAge(5 * time.Hour); got != "5h" {
		t.Fatalf("got %q", got)
	}
	if got := FormatCatalogCacheAge(50 * time.Hour); got != "2d" {
		t.Fatalf("got %q", got)
	}
}

func TestSaveLoadCatalogCacheJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "icons.json")
	in := IconCatalogCache{
		FetchedAt: time.Date(2026, 8, 27, 5, 18, 0, 0, time.UTC),
		Tenant:    "sandbox",
		Auth:      "sandbox-oauth",
		MaxID:     2,
		Items:     []Icon{{ID: 1, Name: "systemsetting.png"}},
	}
	if err := saveCatalogCacheJSON(path, in); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("tmp should be gone: %v", err)
	}
	var out IconCatalogCache
	if err := loadCatalogCacheJSON(path, &out); err != nil {
		t.Fatal(err)
	}
	if out.MaxID != 2 || out.Tenant != "sandbox" || len(out.Items) != 1 || out.Items[0].Name != "systemsetting.png" {
		t.Fatalf("%+v", out)
	}
	if !out.FetchedAt.Equal(in.FetchedAt) {
		t.Fatalf("fetched_at %s want %s", out.FetchedAt, in.FetchedAt)
	}
}

func TestCatalogCacheKeyedByTenantNotAuth(t *testing.T) {
	useTempCatalogCache(t)
	SetActiveConfig(ConfigFromCredentialMap("icon-a", map[string]CredentialConfig{
		"icon-a": {
			URL:  "https://example.jamfcloud.com",
			Auth: AuthConfig{Type: "basic", Username: "u", Password: "p"},
		},
		"icon-b": {
			URL:  "https://example.jamfcloud.com",
			Auth: AuthConfig{Type: "basic", Username: "v", Password: "q"},
		},
	}))
	t.Cleanup(func() { SetActiveConfig(nil) })

	dirA, err := CatalogCacheTenantDir("icon-a")
	if err != nil {
		t.Fatal(err)
	}
	dirB, err := CatalogCacheTenantDir("icon-b")
	if err != nil {
		t.Fatal(err)
	}
	if dirA != dirB {
		t.Fatalf("dirs differ: %s vs %s", dirA, dirB)
	}
	if filepath.Base(dirA) != "icon-a" {
		t.Fatalf("tenant dir %s", dirA)
	}

	if err := saveIconCatalog("icon-a", IconCatalogCache{
		FetchedAt: time.Now().UTC(),
		Tenant:    "icon-a",
		MaxID:     7,
		Items:     []Icon{{ID: 7, Name: "shared.png"}},
	}); err != nil {
		t.Fatal(err)
	}
	got, err := loadIconCatalog("icon-b")
	if err != nil || got == nil || got.MaxID != 7 || len(got.Items) != 1 {
		t.Fatalf("shared load: %v %+v", err, got)
	}
}

func TestCatalogCacheLoadsLegacyAuthTenantDir(t *testing.T) {
	useTempCatalogCache(t)
	SetActiveConfig(ConfigFromCredentialMap("icon-legacy", map[string]CredentialConfig{
		"icon-legacy": {
			URL:  "https://example.jamfcloud.com",
			Auth: AuthConfig{Type: "basic", Username: "u", Password: "p"},
		},
	}))
	t.Cleanup(func() { SetActiveConfig(nil) })

	tenant, err := catalogCacheTenantKey("icon-legacy")
	if err != nil {
		t.Fatal(err)
	}
	root, err := catalogCacheRoot()
	if err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(root, "oldauth@"+tenant, iconCatalogFile)
	if err := saveCatalogCacheJSON(legacy, IconCatalogCache{
		FetchedAt: time.Now().UTC(),
		MaxID:     9,
		Items:     []Icon{{ID: 9, Name: "legacy.png"}},
	}); err != nil {
		t.Fatal(err)
	}

	got, err := loadIconCatalog("icon-legacy")
	if err != nil || got == nil || got.MaxID != 9 {
		t.Fatalf("legacy load: %v %+v", err, got)
	}

	got.Tenant = "icon-legacy"
	if err := saveIconCatalog("icon-legacy", *got); err != nil {
		t.Fatal(err)
	}
	canonical, err := catalogCacheFilePath("icon-legacy", iconCatalogFile)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(canonical); err != nil {
		t.Fatalf("canonical missing: %v", err)
	}
	removeCatalogCacheLegacyFiles("icon-legacy", iconCatalogFile)
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy still present: %v", err)
	}
}
