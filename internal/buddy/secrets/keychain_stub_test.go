//go:build !darwin

package secrets

import "testing"

func TestKeychainStoreStubUnsupported(t *testing.T) {
	store := NewKeychainStore()
	if _, err := store.Get("sandbox", KindPassword); err == nil {
		t.Fatal("expected error on non-darwin keychain Get")
	}
	if err := store.Set("sandbox", KindPassword, "x"); err == nil {
		t.Fatal("expected error on non-darwin keychain Set")
	}
}
