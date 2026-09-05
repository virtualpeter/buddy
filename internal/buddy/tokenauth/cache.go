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
// v=1: AES-GCM with a compiled-in obfuscation key (Linux / unsigned fallback).
// v=2: AES-GCM with a Darwin Keychain wrap key (account token-wrap, no userPresence).
type diskTokenFile struct {
	V      int       `json:"v"`
	Token  string    `json:"token"`
	Expiry time.Time `json:"expiry"`
	URL    string    `json:"url,omitempty"`
}

const diskTokenVersion = diskTokenVersionV2

// clockSkewWindow is how close to expiry a token may be and still be used.
// Avoids sending a bearer that expires mid-request; replaces the old 3-minute
// proactive refresh window.
const clockSkewWindow = 15 * time.Second

// Compiled obfuscation material (not a user secret). Used for v=1 files and
// when no wrap key is available. Changing this invalidates v=1 cache files.
const tokenCacheObfuscationMaterial = "buddy\x00token-cache\x00v1\x00" +
	"7f3c9e1a2b4d6e8f0a1c3d5e7b9f2a4c6d8e0f1a3b5c7d9e"

func tokenCacheAESKey() []byte {
	sum := sha256.Sum256([]byte(tokenCacheObfuscationMaterial))
	return sum[:]
}

func sealTokenWithKey(plain string, key []byte) (string, error) {
	if len(key) != tokenWrapKeySize {
		return "", fmt.Errorf("invalid wrap key size")
	}
	block, err := aes.NewCipher(key)
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

func openTokenWithKey(sealed string, key []byte) (string, error) {
	if len(key) != tokenWrapKeySize {
		return "", fmt.Errorf("invalid wrap key size")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(sealed))
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
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

// sealToken seals with wrap key (v=2) when available, else compiled-in (v=1).
func sealToken(plain string) (string, error) {
	if k, ok := wrapKeyOrCreate(); ok {
		return sealTokenWithKey(plain, k)
	}
	return sealTokenWithKey(plain, tokenCacheAESKey())
}

func sealVersion() int {
	if _, ok := wrapKeyOrCreate(); ok {
		return diskTokenVersionV2
	}
	return diskTokenVersionV1
}

func openToken(sealed string, version int) (string, error) {
	switch version {
	case diskTokenVersionV2:
		k, ok := wrapKey()
		if !ok {
			return "", fmt.Errorf("token wrap key missing")
		}
		return openTokenWithKey(sealed, k)
	case diskTokenVersionV1:
		return openTokenWithKey(sealed, tokenCacheAESKey())
	default:
		return "", fmt.Errorf("unknown token cache version %d", version)
	}
}

// parseDiskTokenFile unmarshals and opens the bearer. On any format/decrypt
// failure returns ok=false (caller should delete the file).
func parseDiskTokenFile(data []byte) (token string, expiry time.Time, url string, ok bool) {
	var f diskTokenFile
	if err := json.Unmarshal(data, &f); err != nil {
		return "", time.Time{}, "", false
	}
	if f.Token == "" || f.Expiry.IsZero() {
		return "", time.Time{}, "", false
	}
	if f.V != diskTokenVersionV1 && f.V != diskTokenVersionV2 {
		return "", time.Time{}, "", false
	}
	tok, err := openToken(f.Token, f.V)
	if err != nil || tok == "" {
		return "", time.Time{}, "", false
	}
	return tok, f.Expiry, f.URL, true
}

func parseDiskTokenVersion(data []byte) int {
	var f diskTokenFile
	if err := json.Unmarshal(data, &f); err != nil {
		return 0
	}
	return f.V
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
	if parseDiskTokenVersion(data) == diskTokenVersionV1 {
		if _, hasWrap := wrapKey(); hasWrap {
			_ = saveDiskToken(cacheKey, fileURL, tok, exp)
		}
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
		V:      sealVersion(),
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

// DiskTokenInfo is metadata for a tokens/ cache file (never includes the bearer).
type DiskTokenInfo struct {
	Key     string
	URL     string
	Expiry  time.Time
	Usable  bool
	Present bool
}

// PeekDiskToken reports cache metadata without deleting expired or corrupt files.
func PeekDiskToken(cacheKey string) DiskTokenInfo {
	info := DiskTokenInfo{Key: cacheKey}
	path, err := tokenCachePath(cacheKey)
	if err != nil {
		return info
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return info
	}
	_, exp, fileURL, ok := parseDiskTokenFile(data)
	if !ok {
		return info
	}
	info.Present = true
	info.URL = fileURL
	info.Expiry = exp
	info.Usable = tokenUsable(exp, clockSkewWindow)
	return info
}

// ListDiskTokens returns metadata for every *.json file under the token cache dir.
func ListDiskTokens() ([]DiskTokenInfo, error) {
	dir, err := tokenCacheDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []DiskTokenInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		key := strings.TrimSuffix(e.Name(), ".json")
		out = append(out, PeekDiskToken(key))
	}
	return out, nil
}

// DropDiskToken removes the cache file for cacheKey. Reports whether a file was present.
func DropDiskToken(cacheKey string) bool {
	path, err := tokenCachePath(cacheKey)
	if err != nil {
		return false
	}
	if _, err := os.Stat(path); err != nil {
		return false
	}
	_ = os.Remove(path)
	return true
}

// TokenCacheDropResult summarizes deleting every tokens/ cache file.
type TokenCacheDropResult struct {
	Dropped int
	Dir     string
}

// DropAllDiskTokens removes every token cache file (including still-valid ones).
func DropAllDiskTokens() (TokenCacheDropResult, error) {
	dir, err := tokenCacheDir()
	if err != nil {
		return TokenCacheDropResult{}, err
	}
	out := TokenCacheDropResult{Dir: dir}
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
		if err := os.Remove(filepath.Join(dir, e.Name())); err == nil {
			out.Dropped++
		}
	}
	return out, nil
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
