package secrets

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
)

// EnvVaultPassphrase overrides the recovery passphrase (CI/tests; never written).
// If the value is a tagged DEK (dek:v1:…), it overrides the vault data key instead.
const EnvVaultPassphrase = "BUDDY_VAULT_PASSPHRASE"

// MasterStore holds a small secret (recovery passphrase or encoded DEK).
type MasterStore interface {
	Get() (string, error)
	Set(passphrase string) error
	Delete() error
	Has() (bool, error)
	Description() string
}

var (
	overrideMaster   MasterStore
	overrideMasterMu sync.Mutex
	overrideDEK      MasterStore
	overrideDEKMu    sync.Mutex
)

// SetMasterStore replaces the recovery-passphrase backend (for tests). Pass nil to reset.
func SetMasterStore(m MasterStore) {
	overrideMasterMu.Lock()
	overrideMaster = m
	overrideMasterMu.Unlock()
}

// SetDEKStore replaces the vault data-key backend (for tests). Pass nil to reset.
func SetDEKStore(m MasterStore) {
	overrideDEKMu.Lock()
	overrideDEK = m
	overrideDEKMu.Unlock()
}

func currentOverrideMaster() MasterStore {
	overrideMasterMu.Lock()
	defer overrideMasterMu.Unlock()
	return overrideMaster
}

func currentOverrideDEK() MasterStore {
	overrideDEKMu.Lock()
	defer overrideDEKMu.Unlock()
	return overrideDEK
}

// DefaultMasterStore returns where the recovery passphrase is stored.
func DefaultMasterStore() (MasterStore, error) {
	if m := currentOverrideMaster(); m != nil {
		return wrapEnvMaster(m), nil
	}
	inner, err := newPlatformKeyStore(false)
	if err != nil {
		return nil, err
	}
	return wrapEnvMaster(inner), nil
}

// DefaultDEKStore returns where the vault data key is stored.
func DefaultDEKStore() (MasterStore, error) {
	if m := currentOverrideDEK(); m != nil {
		return wrapEnvDEK(m), nil
	}
	inner, err := newPlatformKeyStore(true)
	if err != nil {
		return nil, err
	}
	return wrapEnvDEK(inner), nil
}

func newPlatformKeyStore(dek bool) (MasterStore, error) {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("BUDDY_SECRET_STORE"))) {
	case "file":
		if dek {
			return NewFileDEK()
		}
		return NewFileMaster()
	case "pass":
		if dek {
			return NewPassDEK()
		}
		return NewPassMaster()
	case "keychain":
		if runtime.GOOS != "darwin" {
			return nil, fmt.Errorf("BUDDY_SECRET_STORE=keychain is only supported on macOS")
		}
		if dek {
			return KeychainDEK{}, nil
		}
		return KeychainMaster{}, nil
	case "":
		if runtime.GOOS == "darwin" {
			if dek {
				return KeychainDEK{}, nil
			}
			return KeychainMaster{}, nil
		}
		if dek {
			return NewFileDEK()
		}
		return NewFileMaster()
	default:
		return nil, fmt.Errorf("unknown BUDDY_SECRET_STORE %q (supported: keychain, file, pass)", os.Getenv("BUDDY_SECRET_STORE"))
	}
}

func envVaultPassphrase() string {
	return strings.TrimSpace(os.Getenv(EnvVaultPassphrase))
}

type envMaster struct {
	inner MasterStore
}

func wrapEnvMaster(inner MasterStore) MasterStore {
	if inner == nil {
		return nil
	}
	return &envMaster{inner: inner}
}

func (m *envMaster) Get() (string, error) {
	if p := envVaultPassphrase(); p != "" {
		if _, ok := ParseDEK(p); !ok {
			return p, nil
		}
	}
	return m.inner.Get()
}

func (m *envMaster) Set(passphrase string) error {
	return m.inner.Set(passphrase)
}

func (m *envMaster) Delete() error {
	return m.inner.Delete()
}

func (m *envMaster) Has() (bool, error) {
	if p := envVaultPassphrase(); p != "" {
		if _, ok := ParseDEK(p); !ok {
			return true, nil
		}
	}
	return m.inner.Has()
}

func (m *envMaster) Description() string {
	if p := envVaultPassphrase(); p != "" {
		if _, ok := ParseDEK(p); !ok {
			return EnvVaultPassphrase + " + " + m.inner.Description()
		}
	}
	return m.inner.Description()
}

type envDEK struct {
	inner MasterStore
}

func wrapEnvDEK(inner MasterStore) MasterStore {
	if inner == nil {
		return nil
	}
	return &envDEK{inner: inner}
}

func (m *envDEK) Get() (string, error) {
	if p := envVaultPassphrase(); p != "" {
		if _, ok := ParseDEK(p); ok {
			return p, nil
		}
	}
	return m.inner.Get()
}

func (m *envDEK) Set(value string) error {
	return m.inner.Set(value)
}

func (m *envDEK) Delete() error {
	return m.inner.Delete()
}

func (m *envDEK) Has() (bool, error) {
	if p := envVaultPassphrase(); p != "" {
		if _, ok := ParseDEK(p); ok {
			return true, nil
		}
	}
	return m.inner.Has()
}

func (m *envDEK) Description() string {
	if p := envVaultPassphrase(); p != "" {
		if _, ok := ParseDEK(p); ok {
			return EnvVaultPassphrase + " + " + m.inner.Description()
		}
	}
	return m.inner.Description()
}

func (m *envDEK) GetPresence(reason string) (string, error) {
	if p := envVaultPassphrase(); p != "" {
		if _, ok := ParseDEK(p); ok {
			return p, nil
		}
	}
	if p, ok := m.inner.(presenceDEKStore); ok {
		return p.GetPresence(reason)
	}
	return m.inner.Get()
}

func (m *envDEK) SetPresence(value, reason string) error {
	if p, ok := m.inner.(presenceDEKStore); ok {
		return p.SetPresence(value, reason)
	}
	return m.inner.Set(value)
}

// MemoryMaster is an in-memory key/passphrase (tests).
type MemoryMaster struct {
	Phrase string
}

func (m *MemoryMaster) Get() (string, error) {
	if m == nil || strings.TrimSpace(m.Phrase) == "" {
		return "", fmt.Errorf("vault key not set")
	}
	return m.Phrase, nil
}

func (m *MemoryMaster) Set(passphrase string) error {
	if m == nil {
		return fmt.Errorf("master store is nil")
	}
	if strings.TrimSpace(passphrase) == "" {
		return fmt.Errorf("value must not be empty")
	}
	m.Phrase = passphrase
	return nil
}

func (m *MemoryMaster) Delete() error {
	if m != nil {
		m.Phrase = ""
	}
	return nil
}

func (m *MemoryMaster) Has() (bool, error) {
	return m != nil && strings.TrimSpace(m.Phrase) != "", nil
}

func (m *MemoryMaster) Description() string { return "memory" }

// RecoveryPassphrase returns the backup/recovery passphrase (env, then store).
func RecoveryPassphrase() (string, error) {
	return MasterPassphrase()
}

// MasterPassphrase returns the recovery passphrase (env, then store).
func MasterPassphrase() (string, error) {
	m, err := DefaultMasterStore()
	if err != nil {
		return "", err
	}
	return m.Get()
}

// HasMasterPassphrase reports whether a recovery passphrase is available.
func HasMasterPassphrase() (bool, error) {
	m, err := DefaultMasterStore()
	if err != nil {
		return false, err
	}
	return m.Has()
}

// SetMasterPassphrase stores the recovery passphrase and rewraps the vault DEK when needed.
func SetMasterPassphrase(newPass string) error {
	newPass = strings.TrimSpace(newPass)
	if newPass == "" {
		return fmt.Errorf("passphrase must not be empty")
	}
	m, err := DefaultMasterStore()
	if err != nil {
		return err
	}
	store, err := DefaultStore()
	if err != nil {
		return err
	}
	vs, ok := store.(*VaultStore)
	if !ok {
		return m.Set(newPass)
	}
	return vs.SetPassphrase(newPass)
}

// DeleteMasterPassphrase removes the stored recovery passphrase.
func DeleteMasterPassphrase() error {
	m, err := DefaultMasterStore()
	if err != nil {
		return err
	}
	return m.Delete()
}

// DeleteDEK removes the stored vault data key.
func DeleteDEK() error {
	m, err := DefaultDEKStore()
	if err != nil {
		return err
	}
	return m.Delete()
}

// DeleteVaultKeys removes the DEK and recovery passphrase.
func DeleteVaultKeys() error {
	if err := DeleteDEK(); err != nil {
		return err
	}
	return DeleteMasterPassphrase()
}

func masterKindName() string {
	if currentOverrideMaster() != nil {
		return "override"
	}
	return platformKindName()
}

func dekKindName() string {
	if currentOverrideDEK() != nil {
		return "override"
	}
	return platformKindName()
}

func platformKindName() string {
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
