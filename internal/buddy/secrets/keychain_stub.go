//go:build !darwin

package secrets

import "fmt"

// KeychainStore is a stub; buddy's generic-password Keychain backend is macOS-only.
type KeychainStore struct{}

// NewKeychainStore creates a Keychain-backed secret store (macOS only).
func NewKeychainStore() *KeychainStore {
	return &KeychainStore{}
}

func (k *KeychainStore) Description() string {
	return "macOS Keychain (unsupported on this platform)"
}

func keychainUnsupported() error {
	return fmt.Errorf("macOS Keychain secret store is only supported on darwin")
}

func keychainMasterDescription() string {
	return "macOS Keychain (unsupported on this platform)"
}

func keychainDEKDescription() string {
	return "macOS Keychain (unsupported on this platform)"
}

func keychainMasterGet() (string, error) {
	return "", keychainUnsupported()
}

func keychainMasterSet(string) error {
	return keychainUnsupported()
}

func keychainMasterDelete() error {
	return keychainUnsupported()
}

func keychainDEKGet() (string, error) {
	return "", keychainUnsupported()
}

func keychainDEKSet(string) error {
	return keychainUnsupported()
}

func keychainDEKDelete() error {
	return keychainUnsupported()
}

func (k *KeychainStore) Get(credName string, kind Kind) (string, error) {
	return "", keychainUnsupported()
}

func (k *KeychainStore) Set(credName string, kind Kind, value string) error {
	return keychainUnsupported()
}

func (k *KeychainStore) Delete(credName string, kind Kind) error {
	return keychainUnsupported()
}

func (k *KeychainStore) Has(credName string, kind Kind) (bool, error) {
	return false, keychainUnsupported()
}

func (k *KeychainStore) List() ([]Entry, error) {
	return nil, keychainUnsupported()
}
