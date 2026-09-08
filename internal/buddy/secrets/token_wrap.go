package secrets

import (
	"runtime"
	"sync"
)

var (
	overrideTokenWrap   MasterStore
	overrideTokenWrapMu sync.Mutex
)

// SetTokenWrapStore replaces the token-cache wrap-key backend (tests only). Pass nil to reset.
func SetTokenWrapStore(m MasterStore) {
	overrideTokenWrapMu.Lock()
	overrideTokenWrap = m
	overrideTokenWrapMu.Unlock()
}

func currentOverrideTokenWrap() MasterStore {
	overrideTokenWrapMu.Lock()
	defer overrideTokenWrapMu.Unlock()
	return overrideTokenWrap
}

// TokenWrapSupported is true when a persistent wrap key can be stored (macOS Keychain).
func TokenWrapSupported() bool {
	if currentOverrideTokenWrap() != nil {
		return true
	}
	return runtime.GOOS == "darwin"
}

// DefaultTokenWrapStore returns the token-cache wrap-key backend.
// Off Darwin this is nil unless tests injected a store.
func DefaultTokenWrapStore() MasterStore {
	if m := currentOverrideTokenWrap(); m != nil {
		return m
	}
	if runtime.GOOS == "darwin" {
		return KeychainTokenWrap{}
	}
	return nil
}

// DeleteTokenWrap removes the wrap key from the persistent backend (best-effort).
func DeleteTokenWrap() error {
	st := DefaultTokenWrapStore()
	if st == nil {
		return nil
	}
	return st.Delete()
}
