package buddy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"buddy/internal/buddy/secrets"
)

const (
	// CatalogCacheTTL is how long a catalog-cache JSON file stays fresh.
	CatalogCacheTTL = 7 * 24 * time.Hour
)

// TODO: reuse this directory for other small Jamf catalogs (buildings.json,
// network segments). Categories already have a real list API — only cache them
// if a weekly snapshot is still useful. Do not use SQLite unless we later
// cache large, frequently queried sets (computers/packages).

var (
	catalogCacheDirMu       sync.Mutex
	catalogCacheDirOverride string
)

// SetCatalogCacheDirForTest overrides the catalog-cache root (tests only).
func SetCatalogCacheDirForTest(dir string) (restore func()) {
	catalogCacheDirMu.Lock()
	prev := catalogCacheDirOverride
	catalogCacheDirOverride = dir
	catalogCacheDirMu.Unlock()
	return func() {
		catalogCacheDirMu.Lock()
		catalogCacheDirOverride = prev
		catalogCacheDirMu.Unlock()
	}
}

func catalogCacheRoot() (string, error) {
	catalogCacheDirMu.Lock()
	override := catalogCacheDirOverride
	catalogCacheDirMu.Unlock()
	if override != "" {
		return override, nil
	}
	return secrets.CatalogCacheDir()
}

func sanitizeCacheDirName(s string) string {
	s = strings.TrimSpace(s)
	replacer := strings.NewReplacer("/", "_", "\\", "_", "..", "_", ":", "_")
	return replacer.Replace(s)
}

func catalogCacheTenantKey(credName string) (string, error) {
	sess, err := ResolveSessionForCred(credName, OpRead)
	if err != nil {
		return "", err
	}
	key := sanitizeCacheDirName(sess.TenantName)
	if key == "" {
		return "", fmt.Errorf("empty catalog cache key")
	}
	return key, nil
}

// CatalogCacheTenantDir returns ~/.config/buddy/catalog-cache/<tenant>/.
// Icon (and later building/network) catalogs are tenant-wide: two auths on the
// same tenant share one cache. Tokens and caps stay auth@tenant.
func CatalogCacheTenantDir(credName string) (string, error) {
	root, err := catalogCacheRoot()
	if err != nil {
		return "", err
	}
	key, err := catalogCacheTenantKey(credName)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, key), nil
}

// catalogCacheLegacyFiles returns leftover auth@tenant paths from before
// catalogs were keyed by tenant name only.
func catalogCacheLegacyFiles(credName, filename string) ([]string, error) {
	root, err := catalogCacheRoot()
	if err != nil {
		return nil, err
	}
	tenant, err := catalogCacheTenantKey(credName)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	suffix := "@" + tenant
	var out []string
	for _, e := range entries {
		if !e.IsDir() || !strings.HasSuffix(e.Name(), suffix) {
			continue
		}
		p := filepath.Join(root, e.Name(), filename)
		if _, err := os.Stat(p); err == nil {
			out = append(out, p)
		}
	}
	return out, nil
}

func catalogCacheNewestLegacyFile(credName, filename string) (string, error) {
	paths, err := catalogCacheLegacyFiles(credName, filename)
	if err != nil || len(paths) == 0 {
		return "", err
	}
	newest := ""
	var newestMod time.Time
	for _, p := range paths {
		st, err := os.Stat(p)
		if err != nil {
			continue
		}
		if newest == "" || st.ModTime().After(newestMod) {
			newest = p
			newestMod = st.ModTime()
		}
	}
	return newest, nil
}

func removeCatalogCacheLegacyFiles(credName, filename string) {
	paths, err := catalogCacheLegacyFiles(credName, filename)
	if err != nil {
		return
	}
	for _, p := range paths {
		_ = os.Remove(p)
	}
}

// CatalogCacheStale reports whether fetchedAt is missing or older than ttl.
func CatalogCacheStale(fetchedAt, now time.Time, ttl time.Duration) bool {
	if fetchedAt.IsZero() {
		return true
	}
	if now.IsZero() {
		now = time.Now()
	}
	if ttl <= 0 {
		ttl = CatalogCacheTTL
	}
	return now.Sub(fetchedAt) > ttl
}

// FormatCatalogCacheAge is a short age for stderr (e.g. "2d", "5h", "<1h").
func FormatCatalogCacheAge(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	if d < time.Hour {
		return "<1h"
	}
	hours := int(d / time.Hour)
	if hours < 24 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dd", hours/24)
}

func catalogCacheFilePath(credName, filename string) (string, error) {
	dir, err := CatalogCacheTenantDir(credName)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, filename), nil
}

func loadCatalogCacheJSON(path string, dest any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("parse catalog cache %s: %w", filepath.Base(path), err)
	}
	return nil
}

func saveCatalogCacheJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create catalog cache dir: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write catalog cache: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace catalog cache: %w", err)
	}
	return nil
}
