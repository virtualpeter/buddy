package tokenauth

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
	"sync"

	"buddy/internal/buddy/secrets"
)

const (
	diskTokenVersionV1 = 1
	diskTokenVersionV2 = 2
	tokenWrapPrefix    = "wrap:v1:"
	tokenWrapKeySize   = 32
)

var (
	wrapKeyMu    sync.Mutex
	wrapKeyCache []byte
	wrapKeyTest  *[]byte // tests: non-nil disables Keychain; 32 bytes = use that key
)

// SetTokenWrapKeyForTest injects a wrap key (32 bytes) or forces compiled-in
// obfuscation (any other length). Pass nil to restore production lookup.
func SetTokenWrapKeyForTest(key []byte) (restore func()) {
	wrapKeyMu.Lock()
	prev := wrapKeyTest
	prevCache := wrapKeyCache
	if key == nil {
		wrapKeyTest = nil
	} else {
		k := append([]byte(nil), key...)
		wrapKeyTest = &k
	}
	wrapKeyCache = nil
	wrapKeyMu.Unlock()
	return func() {
		wrapKeyMu.Lock()
		wrapKeyTest = prev
		wrapKeyCache = prevCache
		wrapKeyMu.Unlock()
	}
}

// ClearWrapKeyCache drops the process-cached wrap key (after cred erase).
func ClearWrapKeyCache() {
	wrapKeyMu.Lock()
	wrapKeyCache = nil
	wrapKeyMu.Unlock()
}

func encodeWrapKey(raw []byte) string {
	return tokenWrapPrefix + base64.StdEncoding.EncodeToString(raw)
}

func parseWrapKey(s string) ([]byte, bool) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, tokenWrapPrefix) {
		return nil, false
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(s, tokenWrapPrefix))
	if err != nil || len(raw) != tokenWrapKeySize {
		return nil, false
	}
	return raw, true
}

// wrapKey returns the process wrap key if one is available without creating.
func wrapKey() ([]byte, bool) {
	wrapKeyMu.Lock()
	defer wrapKeyMu.Unlock()
	if wrapKeyTest != nil {
		if len(*wrapKeyTest) == tokenWrapKeySize {
			return append([]byte(nil), *wrapKeyTest...), true
		}
		return nil, false
	}
	if len(wrapKeyCache) == tokenWrapKeySize {
		return append([]byte(nil), wrapKeyCache...), true
	}
	tokenCacheDirMu.Lock()
	testDir := tokenCacheDirOverride != ""
	tokenCacheDirMu.Unlock()
	if testDir {
		return nil, false
	}
	st := secrets.DefaultTokenWrapStore()
	if st == nil {
		return nil, false
	}
	s, err := st.Get()
	if err != nil {
		return nil, false
	}
	k, ok := parseWrapKey(s)
	if !ok {
		return nil, false
	}
	wrapKeyCache = append([]byte(nil), k...)
	return k, true
}

// wrapKeyOrCreate returns a wrap key for new v=2 files, generating one on Darwin.
func wrapKeyOrCreate() ([]byte, bool) {
	if k, ok := wrapKey(); ok {
		return k, true
	}
	wrapKeyMu.Lock()
	defer wrapKeyMu.Unlock()
	if wrapKeyTest != nil {
		return nil, false
	}
	if len(wrapKeyCache) == tokenWrapKeySize {
		return append([]byte(nil), wrapKeyCache...), true
	}
	tokenCacheDirMu.Lock()
	testDir := tokenCacheDirOverride != ""
	tokenCacheDirMu.Unlock()
	if testDir {
		return nil, false
	}
	if !secrets.TokenWrapSupported() {
		return nil, false
	}
	st := secrets.DefaultTokenWrapStore()
	if st == nil {
		return nil, false
	}
	raw := make([]byte, tokenWrapKeySize)
	if _, err := rand.Read(raw); err != nil {
		return nil, false
	}
	if err := st.Set(encodeWrapKey(raw)); err != nil {
		return nil, false
	}
	wrapKeyCache = append([]byte(nil), raw...)
	return raw, true
}
