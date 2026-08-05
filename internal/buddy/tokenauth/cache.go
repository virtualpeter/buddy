package tokenauth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"buddy/internal/buddy/secrets"
)

// diskTokenFile is the on-disk shape for a cached bearer token.
// Version 1 stores Token as base64(AES-GCM(nonce||ciphertext)) using a
// compiled-in obfuscation key — enough to stop casual `cat` of the cache,
// not a substitute for 0600 permissions or a real secret store.
type diskTokenFile struct {
	V      int       `json:"v"`
	Token  string    `json:"token"`
	Expiry time.Time `json:"expiry"`
	URL    string    `json:"url,omitempty"`
}

const diskTokenVersion = 1

// clockSkewWindow is how close to expiry a token may be and still be used.
// Avoids sending a bearer that expires mid-request; replaces the old 3-minute
// proactive refresh window.
const clockSkewWindow = 15 * time.Second

// Compiled obfuscation material (not a user secret). Changing this invalidates
// existing cache files — they are deleted on the next load/sweep.
const tokenCacheObfuscationMaterial = "buddy\x00token-cache\x00v1\x00" +
	"7f3c9e1a2b4d6e8f0a1c3d5e7b9f2a4c6d8e0f1a3b5c7d9e"

func tokenCacheAESKey() []byte {
	sum := sha256.Sum256([]byte(tokenCacheObfuscationMaterial))
	return sum[:]
}

func sealToken(plain string) (string, error) {
	block, err := aes.NewCipher(tokenCacheAESKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(out), nil
}

func openToken(sealed string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(sealed))
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(tokenCacheAESKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	ns := gcm.NonceSize()
	if len(raw) < ns {
		return "", fmt.Errorf("ciphertext too short")
	}
	plain, err := gcm.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// parseDiskTokenFile unmarshals and opens the bearer. On any format/decrypt
// failure returns ok=false (caller should delete the file).
func parseDiskTokenFile(data []byte) (token string, expiry time.Time, url string, ok bool) {
	var f diskTokenFile
	if err := json.Unmarshal(data, &f); err != nil {
		return "", time.Time{}, "", false
	}
	if f.V != diskTokenVersion || f.Token == "" || f.Expiry.IsZero() {
		return "", time.Time{}, "", false
	}
	tok, err := openToken(f.Token)
	if err != nil || tok == "" {
		return "", time.Time{}, "", false
	}
	return tok, f.Expiry, f.URL, true
}

var (
	tokenCacheDirMu       sync.Mutex
	tokenCacheDirOverride string
)

// SetTokenCacheDirForTest overrides the token cache directory (tests only).
// Returns a restore function that puts back the previous override.
func SetTokenCacheDirForTest(dir string) (restore func()) {
	tokenCacheDirMu.Lock()
	prev := tokenCacheDirOverride
	tokenCacheDirOverride = dir
	tokenCacheDirMu.Unlock()
	return func() {
		tokenCacheDirMu.Lock()
		tokenCacheDirOverride = prev
		tokenCacheDirMu.Unlock()
	}
}

func tokenCacheDir() (string, error) {
	tokenCacheDirMu.Lock()
	override := tokenCacheDirOverride
	tokenCacheDirMu.Unlock()
	if override != "" {
		return override, nil
	}
	return secrets.TokensDir()
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

func tokenUsable(expiry time.Time, minRemaining time.Duration) bool {
	if expiry.IsZero() {
		return false
	}
	return time.Until(expiry) > minRemaining
}

// loadDiskToken returns token+expiry if a valid cache file exists and is still usable.
// usable means expiry is more than minRemaining in the future.
// Expired, near-expiry, URL-mismatched, undecodable, or corrupt files are removed.
func loadDiskToken(cacheKey, baseURL string, minRemaining time.Duration) (token string, expiry time.Time, ok bool) {
	path, err := tokenCachePath(cacheKey)
	if err != nil {
		return "", time.Time{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", time.Time{}, false
	}
	tok, exp, fileURL, ok := parseDiskTokenFile(data)
	if !ok {
		_ = os.Remove(path)
		return "", time.Time{}, false
	}
	if baseURL != "" && fileURL != "" && strings.TrimRight(fileURL, "/") != strings.TrimRight(baseURL, "/") {
		_ = os.Remove(path)
		return "", time.Time{}, false
	}
	if !tokenUsable(exp, minRemaining) {
		_ = os.Remove(path)
		return "", time.Time{}, false
	}
	return tok, exp, true
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
	sealed, err := sealToken(token)
	if err != nil {
		return fmt.Errorf("seal token: %w", err)
	}
	f := diskTokenFile{
		V:      diskTokenVersion,
		Token:  sealed,
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
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	// Opportunistic sweep of other stale entries (best-effort).
	_, _ = CleanExpiredTokenCache()
	return nil
}

func removeDiskToken(cacheKey string) {
	path, err := tokenCachePath(cacheKey)
	if err != nil {
		return
	}
	_ = os.Remove(path)
}

// TokenCacheCleanResult summarizes a tokens/ directory sweep.
type TokenCacheCleanResult struct {
	Deleted int
	Kept    int
	Dir     string
}

// CleanExpiredTokenCache removes expired, empty, corrupt, or undecodable token cache files.
// Still-valid entries are kept. Returns how many were deleted.
func CleanExpiredTokenCache() (TokenCacheCleanResult, error) {
	dir, err := tokenCacheDir()
	if err != nil {
		return TokenCacheCleanResult{}, err
	}
	out := TokenCacheCleanResult{Dir: dir}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return out, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		if shouldDeleteTokenCacheFile(path) {
			if err := os.Remove(path); err == nil {
				out.Deleted++
			}
			continue
		}
		out.Kept++
	}
	return out, nil
}

func shouldDeleteTokenCacheFile(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return true
	}
	_, exp, _, ok := parseDiskTokenFile(data)
	if !ok {
		return true
	}
	// Sweep uses skew so near-expiry leftovers don't linger.
	return !tokenUsable(exp, clockSkewWindow)
}
