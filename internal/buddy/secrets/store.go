package secrets

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
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

// ParseKind parses a kind string ("password" or "client-secret").
func ParseKind(s string) (Kind, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "password":
		return KindPassword, nil
	case "client-secret", "clientsecret", "client_secret":
		return KindClientSecret, nil
	default:
		return 0, fmt.Errorf("unknown secret kind %q (use password or client-secret)", s)
	}
}

// Entry is one stored secret identity (auth name + kind); values are never listed.
type Entry struct {
	AuthName string
	Kind     Kind
}

// Store persists credential secrets outside config.yaml.
type Store interface {
	Get(credName string, kind Kind) (string, error)
	Set(credName string, kind Kind, value string) error
	Delete(credName string, kind Kind) error
	Has(credName string, kind Kind) (bool, error)
	List() ([]Entry, error)
	Description() string
}

var (
	overrideStore   Store
	overrideStoreMu sync.Mutex

	defaultVaultMu   sync.Mutex
	defaultVault     *VaultStore
	defaultVaultPath string
)

// SetStore replaces the default store (for tests). Pass nil to reset.
func SetStore(s Store) {
	overrideStoreMu.Lock()
	overrideStore = s
	overrideStoreMu.Unlock()
	if s == nil {
		resetCachedDefaultStore()
		resetDeviceOwnerConfirm()
	}
}

func currentOverrideStore() Store {
	overrideStoreMu.Lock()
	defer overrideStoreMu.Unlock()
	return overrideStore
}

// UsingOverrideStore reports whether tests replaced DefaultStore.
func UsingOverrideStore() bool {
	return currentOverrideStore() != nil
}

// DefaultStore returns the process-wide encrypted vault (or a test override).
// The same VaultStore is reused so Keychain presence / Touch ID runs once per process.
func DefaultStore() (Store, error) {
	if s := currentOverrideStore(); s != nil {
		return s, nil
	}
	path, err := VaultPath()
	if err != nil {
		return nil, err
	}
	defaultVaultMu.Lock()
	defer defaultVaultMu.Unlock()
	if defaultVault != nil && defaultVaultPath == path {
		return defaultVault, nil
	}
	dek, err := DefaultDEKStore()
	if err != nil {
		return nil, err
	}
	recovery, err := DefaultMasterStore()
	if err != nil {
		return nil, err
	}
	defaultVault = NewVaultStore(path, dek).WithRecovery(recovery)
	defaultVaultPath = path
	return defaultVault, nil
}

func resetCachedDefaultStore() {
	defaultVaultMu.Lock()
	defaultVault = nil
	defaultVaultPath = ""
	defaultVaultMu.Unlock()
}

// LegacyStore returns the pre-vault per-auth backend (Keychain / file / pass)
// used only to import existing secrets into the vault.
func LegacyStore() (Store, error) {
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
	if currentOverrideStore() != nil {
		return "override"
	}
	return "vault"
}

// MasterKindName is the recovery-passphrase backend (keychain, file, pass).
func MasterKindName() string {
	return masterKindName()
}

// DEKKindName is the vault data-key backend (keychain, file, pass).
func DEKKindName() string {
	return dekKindName()
}
