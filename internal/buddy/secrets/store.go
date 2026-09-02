package secrets

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

// Kind identifies which secret field is stored.
type Kind int

const (
	KindPassword Kind = iota
	KindClientSecret
)

func (k Kind) String() string {
	switch k {
	case KindPassword:
		return "password"
	case KindClientSecret:
		return "client-secret"
	default:
		return "unknown"
	}
}

// Store persists credential secrets outside config.yaml.
type Store interface {
	Get(credName string, kind Kind) (string, error)
	Set(credName string, kind Kind, value string) error
	Delete(credName string, kind Kind) error
	Has(credName string, kind Kind) (bool, error)
	Description() string
}

var overrideStore Store

// SetStore replaces the default store (for tests). Pass nil to reset.
func SetStore(s Store) {
	overrideStore = s
}

// DefaultStore returns the platform secret store (or BUDDY_SECRET_STORE override).
func DefaultStore() (Store, error) {
	if overrideStore != nil {
		return overrideStore, nil
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("BUDDY_SECRET_STORE"))) {
	case "file":
		return NewFileStore()
	case "pass":
		return NewPassStore()
	case "keychain":
		if runtime.GOOS != "darwin" {
			return nil, fmt.Errorf("BUDDY_SECRET_STORE=keychain is only supported on macOS")
		}
		return NewKeychainStore(), nil
	case "":
		if runtime.GOOS == "darwin" {
			return NewKeychainStore(), nil
		}
		return NewFileStore()
	default:
		return nil, fmt.Errorf("unknown BUDDY_SECRET_STORE %q (supported: keychain, file, pass)", os.Getenv("BUDDY_SECRET_STORE"))
	}
}

// StoreInfo describes the active secret backend for CLI output.
func StoreInfo() (kind string, detail string, err error) {
	store, err := DefaultStore()
	if err != nil {
		return "", "", err
	}
	return storeKindName(), store.Description(), nil
}

func storeKindName() string {
	if overrideStore != nil {
		return "override"
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("BUDDY_SECRET_STORE"))) {
	case "file":
		return "file"
	case "pass":
		return "pass"
	case "keychain":
		return "keychain"
	default:
		if runtime.GOOS == "darwin" {
			return "keychain"
		}
		return "file"
	}
}
