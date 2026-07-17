package tokenauth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// diskTokenFile is the on-disk shape for a cached bearer token.
type diskTokenFile struct {
	Token  string    `json:"token"`
	Expiry time.Time `json:"expiry"`
	URL    string    `json:"url,omitempty"` // base URL for sanity checks
}

var (
	tokenCacheDirMu       sync.Mutex
	tokenCacheDirOverride string
)

// SetTokenCacheDirForTest overrides the token cache directory (tests only).
func SetTokenCacheDirForTest(dir string) {
	tokenCacheDirMu.Lock()
	defer tokenCacheDirMu.Unlock()
	tokenCacheDirOverride = dir
}

func tokenCacheDir() (string, error) {
	tokenCacheDirMu.Lock()
	override := tokenCacheDirOverride
	tokenCacheDirMu.Unlock()
	if override != "" {
		return override, nil
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "buddy", "tokens"), nil
}

func sanitizeCacheKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	replacer := strings.NewReplacer("/", "_", "\\", "_", "..", "_", ":", "_")
	return replacer.Replace(key)
}

func tokenCachePath(cacheKey string) (string, error) {
	key := sanitizeCacheKey(cacheKey)
	if key == "" {
		return "", fmt.Errorf("empty token cache key")
	}
	dir, err := tokenCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, key+".json"), nil
}

// loadDiskToken returns token+expiry if a valid cache file exists and is still usable.
// usable means expiry is more than minRemaining in the future.
func loadDiskToken(cacheKey, baseURL string, minRemaining time.Duration) (token string, expiry time.Time, ok bool) {
	path, err := tokenCachePath(cacheKey)
	if err != nil {
		return "", time.Time{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", time.Time{}, false
	}
	var f diskTokenFile
	if err := json.Unmarshal(data, &f); err != nil {
		_ = os.Remove(path)
		return "", time.Time{}, false
	}
	if f.Token == "" || f.Expiry.IsZero() {
		return "", time.Time{}, false
	}
	if baseURL != "" && f.URL != "" && strings.TrimRight(f.URL, "/") != strings.TrimRight(baseURL, "/") {
		return "", time.Time{}, false
	}
	if time.Until(f.Expiry) <= minRemaining {
		return "", time.Time{}, false
	}
	return f.Token, f.Expiry, true
}

func saveDiskToken(cacheKey, baseURL, token string, expiry time.Time) error {
	path, err := tokenCachePath(cacheKey)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f := diskTokenFile{
		Token:  token,
		Expiry: expiry.UTC(),
		URL:    strings.TrimRight(baseURL, "/"),
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func removeDiskToken(cacheKey string) {
	path, err := tokenCachePath(cacheKey)
	if err != nil {
		return
	}
	_ = os.Remove(path)
}
