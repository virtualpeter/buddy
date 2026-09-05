package secrets

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type vaultPayload struct {
	Secrets map[string]secretFile `json:"secrets"`
}

type vaultMem struct {
	modTime time.Time
	size    int64
	key     string
	payload vaultPayload
}

var vaultCaches sync.Map // path -> *vaultMem

// VaultStore persists secrets in an AES-GCM sidecar (secrets.enc).
type VaultStore struct {
	path       string
	dek        MasterStore
	recovery   MasterStore
	sessionDEK string // encoded DEK held for this process after UnlockSession
	mu         sync.Mutex
}

// NewVaultStore creates a vault. Empty path uses VaultPath() on each operation.
// dek holds the encoded data key; recovery is the backup passphrase (optional in tests).
func NewVaultStore(path string, dek MasterStore) *VaultStore {
	return &VaultStore{path: path, dek: dek}
}

// WithRecovery sets the recovery-passphrase backend used by SetPassphrase and wrap.
func (v *VaultStore) WithRecovery(recovery MasterStore) *VaultStore {
	if v != nil {
		v.recovery = recovery
	}
	return v
}

func (v *VaultStore) vaultPath() (string, error) {
	if v != nil && v.path != "" {
		return v.path, nil
	}
	return VaultPath()
}

func (v *VaultStore) Description() string {
	path, err := v.vaultPath()
	if err != nil {
		return "vault (path unknown)"
	}
	dek := "dek unknown"
	if v != nil && v.dek != nil {
		dek = v.dek.Description()
	}
	return fmt.Sprintf("%s (dek: %s)", path, dek)
}

func (v *VaultStore) Get(credName string, kind Kind) (string, error) {
	payload, err := v.load()
	if err != nil {
		return "", err
	}
	sf := payload.Secrets[credName]
	val, ok, err := valueFromSecretFile(&sf, kind)
	if err != nil {
		return "", err
	}
	if !ok || val == "" {
		return "", fmt.Errorf("%w: secret %q not found in vault for credential %q", ErrNotFound, kind, credName)
	}
	return val, nil
}

func (v *VaultStore) Has(credName string, kind Kind) (bool, error) {
	payload, err := v.load()
	if err != nil {
		return false, err
	}
	sf := payload.Secrets[credName]
	val, ok, err := valueFromSecretFile(&sf, kind)
	if err != nil {
		return false, err
	}
	return ok && val != "", nil
}

func (v *VaultStore) List() ([]Entry, error) {
	payload, err := v.load()
	if err != nil {
		return nil, err
	}
	var names []string
	for name := range payload.Secrets {
		names = append(names, name)
	}
	sort.Strings(names)
	var out []Entry
	for _, name := range names {
		sf := payload.Secrets[name]
		if sf.Password != "" {
			out = append(out, Entry{AuthName: name, Kind: KindPassword})
		}
		if sf.ClientSecret != "" {
			out = append(out, Entry{AuthName: name, Kind: KindClientSecret})
		}
	}
	return out, nil
}

func (v *VaultStore) Set(credName string, kind Kind, value string) error {
	if strings.TrimSpace(credName) == "" {
		return fmt.Errorf("credential name is required")
	}
	payload, err := v.loadOrEmpty()
	if err != nil {
		return err
	}
	if payload.Secrets == nil {
		payload.Secrets = map[string]secretFile{}
	}
	sf := payload.Secrets[credName]
	if err := setSecretFileField(&sf, kind, value); err != nil {
		return err
	}
	payload.Secrets[credName] = sf
	return v.save(payload)
}

func (v *VaultStore) Delete(credName string, kind Kind) error {
	payload, err := v.loadOrEmpty()
	if err != nil {
		return err
	}
	sf, ok := payload.Secrets[credName]
	if !ok {
		return nil
	}
	switch kind {
	case KindPassword:
		sf.Password = ""
	case KindClientSecret:
		sf.ClientSecret = ""
	default:
		return fmt.Errorf("unknown secret kind %v", kind)
	}
	if sf.Password == "" && sf.ClientSecret == "" {
		delete(payload.Secrets, credName)
	} else {
		payload.Secrets[credName] = sf
	}
	return v.save(payload)
}

// Erase removes the vault file and cached plaintext.
func (v *VaultStore) Erase() (int, error) {
	payload, err := v.loadOrEmpty()
	if err != nil {
		return 0, err
	}
	n := 0
	for _, sf := range payload.Secrets {
		if sf.Password != "" {
			n++
		}
		if sf.ClientSecret != "" {
			n++
		}
	}
	path, err := v.vaultPath()
	if err != nil {
		return 0, err
	}
	vaultCaches.Delete(path)
	v.LockSession()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return 0, fmt.Errorf("remove vault: %w", err)
	}
	return n, nil
}

// UnlockSession loads the vault DEK into process memory (one Keychain presence
// prompt on signed macOS). MCP and serve call this at start; CLI unlocks lazily
// via dekOptional on first vault use.
func (v *VaultStore) UnlockSession(reason string) error {
	if v == nil {
		return fmt.Errorf("vault store is nil")
	}
	return v.ensureSessionDEKOptional(reason)
}

// LockSession clears the in-memory DEK and vault plaintext cache (tests / future).
func (v *VaultStore) LockSession() {
	if v == nil {
		return
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	v.sessionDEK = ""
	if path, err := v.vaultPath(); err == nil {
		vaultCaches.Delete(path)
	}
}

// SessionUnlocked reports whether UnlockSession (or lazy unlock) holds a DEK.
func (v *VaultStore) SessionUnlocked() bool {
	if v == nil {
		return false
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	_, ok := ParseDEK(v.sessionDEK)
	return ok
}

// UnlockDefaultStore unlocks the process vault session via DefaultStore.
func UnlockDefaultStore(reason string) error {
	store, err := DefaultStore()
	if err != nil {
		return err
	}
	vs, ok := store.(*VaultStore)
	if !ok {
		return nil
	}
	return vs.UnlockSession(reason)
}

// SetPassphrase stores newPass as the recovery passphrase (backups) and
// rewraps the DEK in the vault file. It does not change the DEK used to
// encrypt secrets.
func (v *VaultStore) SetPassphrase(newPass string) error {
	newPass = strings.TrimSpace(newPass)
	if newPass == "" {
		return fmt.Errorf("passphrase must not be empty")
	}
	rec := v.recoveryStore()
	if rec == nil {
		return fmt.Errorf("recovery store is nil")
	}

	path, err := v.vaultPath()
	if err != nil {
		return err
	}
	info, statErr := os.Stat(path)
	exists := statErr == nil && info.Size() > 0
	if os.IsNotExist(statErr) {
		exists = false
	} else if statErr != nil {
		return statErr
	}

	if !exists {
		if err := rec.Set(newPass); err != nil {
			return err
		}
		_, err := v.ensureDEK(true)
		return err
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read vault: %w", err)
	}
	env, err := ParseEnvelope(raw)
	if err != nil {
		return fmt.Errorf("parse vault: %w", err)
	}

	payload, dek, err := v.unlockExisting(env, raw, newPass)
	if err != nil {
		return err
	}
	if err := rec.Set(newPass); err != nil {
		return err
	}
	return v.saveWithDEK(payload, dek, newPass)
}

func (v *VaultStore) load() (vaultPayload, error) {
	return v.loadOrEmpty()
}

func (v *VaultStore) loadOrEmpty() (vaultPayload, error) {
	path, err := v.vaultPath()
	if err != nil {
		return vaultPayload{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return vaultPayload{Secrets: map[string]secretFile{}}, nil
		}
		return vaultPayload{}, fmt.Errorf("stat vault: %w", err)
	}
	if info.Size() == 0 {
		return vaultPayload{Secrets: map[string]secretFile{}}, nil
	}
	// Best-effort Keychain presence prompt outside v.mu (OK if no DEK yet;
	// unwrap/recovery paths resolve inside unlockExisting).
	_ = v.ensureSessionDEKOptional(DefaultUnlockReason)

	v.mu.Lock()
	info, err = os.Stat(path)
	if err != nil {
		v.mu.Unlock()
		if os.IsNotExist(err) {
			return vaultPayload{Secrets: map[string]secretFile{}}, nil
		}
		return vaultPayload{}, fmt.Errorf("stat vault: %w", err)
	}
	if info.Size() == 0 {
		v.mu.Unlock()
		return vaultPayload{Secrets: map[string]secretFile{}}, nil
	}
	if cached, ok := vaultCaches.Load(path); ok {
		mem := cached.(*vaultMem)
		if mem.modTime.Equal(info.ModTime()) && mem.size == info.Size() &&
			mem.key != "" && mem.key == v.sessionDEK {
			out := cloneVaultPayload(mem.payload)
			v.mu.Unlock()
			return out, nil
		}
	}
	v.mu.Unlock()

	// Decrypt without holding v.mu (ensureDEK / Keychain must not re-enter the lock).
	raw, err := os.ReadFile(path)
	if err != nil {
		return vaultPayload{}, fmt.Errorf("read vault: %w", err)
	}
	env, err := ParseEnvelope(raw)
	if err != nil {
		return vaultPayload{}, fmt.Errorf("parse vault: %w", err)
	}
	payload, dek, err := v.unlockExisting(env, raw, "")
	if err != nil {
		return vaultPayload{}, err
	}
	enc, err := EncodeDEK(dek)
	if err != nil {
		return vaultPayload{}, err
	}
	info, err = os.Stat(path)
	if err != nil {
		return vaultPayload{}, err
	}
	v.mu.Lock()
	v.sessionDEK = enc
	vaultCaches.Store(path, &vaultMem{
		modTime: info.ModTime(),
		size:    info.Size(),
		key:     enc,
		payload: cloneVaultPayload(payload),
	})
	out := cloneVaultPayload(payload)
	v.mu.Unlock()
	return out, nil
}

func (v *VaultStore) save(payload vaultPayload) error {
	dek, err := v.ensureDEK(true)
	if err != nil {
		return err
	}
	return v.saveWithDEK(payload, dek, v.recoveryOptional())
}

func (v *VaultStore) saveWithDEK(payload vaultPayload, dek []byte, recoveryPass string) error {
	path, err := v.vaultPath()
	if err != nil {
		return err
	}
	if payload.Secrets == nil {
		payload.Secrets = map[string]secretFile{}
	}
	plain, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode vault: %w", err)
	}
	env, err := EncryptRaw(plain, dek, KindSecretVault, AADSecretVault)
	if err != nil {
		return err
	}
	if strings.TrimSpace(recoveryPass) != "" {
		wrap, werr := EncryptEnvelope(dek, recoveryPass, KindSecretVaultWrap, AADSecretVaultWrap)
		if werr != nil {
			return werr
		}
		env.Wrap = &wrap
	}
	raw, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := writeFileAtomic0600(path, raw); err != nil {
		return fmt.Errorf("write vault: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	enc, err := EncodeDEK(dek)
	if err != nil {
		return err
	}
	vaultCaches.Store(path, &vaultMem{
		modTime: info.ModTime(),
		size:    info.Size(),
		key:     enc,
		payload: cloneVaultPayload(payload),
	})
	return nil
}

func (v *VaultStore) unlockExisting(env Envelope, raw []byte, typedPass string) (vaultPayload, []byte, error) {
	switch {
	case env.Buddy != KindSecretVault:
		return vaultPayload{}, nil, fmt.Errorf("not a buddy vault (buddy=%q v=%d)", env.Buddy, env.V)
	case env.V == EnvelopeVersionRaw && env.KDF == EnvelopeKDFRaw:
		dek, err := v.resolveDEK(env, typedPass)
		if err != nil {
			return vaultPayload{}, nil, err
		}
		plain, err := decryptRawEnv(env, dek, KindSecretVault, AADSecretVault, "vault")
		if err != nil {
			return vaultPayload{}, nil, err
		}
		payload, err := parseVaultPayload(plain)
		if err != nil {
			return vaultPayload{}, nil, err
		}
		return payload, dek, nil
	case env.V == EnvelopeVersion && env.KDF == EnvelopeKDF:
		pass, err := v.resolveV1Passphrase(typedPass)
		if err != nil {
			return vaultPayload{}, nil, err
		}
		plain, err := DecryptEnvelope(raw, pass, KindSecretVault, AADSecretVault, "vault")
		if err != nil && typedPass != "" && typedPass != pass {
			plain, err = DecryptEnvelope(raw, typedPass, KindSecretVault, AADSecretVault, "vault")
			if err == nil {
				pass = typedPass
			}
		}
		if err != nil {
			return vaultPayload{}, nil, err
		}
		payload, err := parseVaultPayload(plain)
		if err != nil {
			return vaultPayload{}, nil, err
		}
		dek, err := v.ensureDEK(true)
		if err != nil {
			return vaultPayload{}, nil, err
		}
		if rec := v.recoveryStore(); rec != nil {
			if ok, _ := rec.Has(); !ok && strings.TrimSpace(pass) != "" {
				_ = rec.Set(pass)
			}
		}
		if err := v.saveWithDEK(payload, dek, v.recoveryOptional()); err != nil {
			return vaultPayload{}, nil, err
		}
		return payload, dek, nil
	default:
		return vaultPayload{}, nil, fmt.Errorf("not a buddy vault (buddy=%q v=%d)", env.Buddy, env.V)
	}
}

func parseVaultPayload(plain []byte) (vaultPayload, error) {
	var payload vaultPayload
	if err := json.Unmarshal(plain, &payload); err != nil {
		return vaultPayload{}, fmt.Errorf("parse vault: %w", err)
	}
	if payload.Secrets == nil {
		payload.Secrets = map[string]secretFile{}
	}
	return payload, nil
}

func (v *VaultStore) resolveDEK(env Envelope, typedPass string) ([]byte, error) {
	v.mu.Lock()
	if key, ok := ParseDEK(v.sessionDEK); ok {
		v.mu.Unlock()
		return key, nil
	}
	v.mu.Unlock()
	if enc, err := v.fetchDEK(DefaultUnlockReason); err == nil {
		if key, ok := ParseDEK(enc); ok {
			v.mu.Lock()
			v.sessionDEK = enc
			v.mu.Unlock()
			return key, nil
		}
	}
	pass := strings.TrimSpace(typedPass)
	if pass == "" {
		pass = v.recoveryOptional()
	}
	if env.Wrap != nil && pass != "" {
		wrapRaw, err := json.Marshal(env.Wrap)
		if err != nil {
			return nil, err
		}
		key, err := DecryptEnvelope(wrapRaw, pass, KindSecretVaultWrap, AADSecretVaultWrap, "vault key wrap")
		if err != nil {
			return nil, fmt.Errorf("could not unwrap vault key: %w", err)
		}
		if len(key) != EnvelopeKeyLen {
			return nil, fmt.Errorf("unwrapped vault key has invalid length")
		}
		enc, err := EncodeDEK(key)
		if err != nil {
			return nil, err
		}
		if v.dek == nil {
			return nil, fmt.Errorf("dek store is nil")
		}
		if err := v.storeDEK(enc, "Buddy wants to store the vault key"); err != nil {
			return nil, err
		}
		v.mu.Lock()
		v.sessionDEK = enc
		v.mu.Unlock()
		return key, nil
	}
	return nil, fmt.Errorf("vault key not available — restore from backup, or run 'buddy cred set passphrase' with the recovery phrase")
}

func (v *VaultStore) resolveV1Passphrase(typedPass string) (string, error) {
	if p := strings.TrimSpace(typedPass); p != "" {
		return p, nil
	}
	if p := v.recoveryOptional(); p != "" {
		return p, nil
	}
	if enc, err := v.dekOptional(); err == nil && strings.TrimSpace(enc) != "" {
		if _, ok := ParseDEK(enc); !ok {
			return enc, nil
		}
	}
	return "", fmt.Errorf("vault passphrase not set — run 'buddy cred set passphrase'")
}

func (v *VaultStore) ensureDEK(create bool) ([]byte, error) {
	v.mu.Lock()
	if key, ok := ParseDEK(v.sessionDEK); ok {
		v.mu.Unlock()
		return key, nil
	}
	v.mu.Unlock()
	if enc, err := v.fetchDEK(DefaultUnlockReason); err == nil {
		if key, ok := ParseDEK(enc); ok {
			v.mu.Lock()
			v.sessionDEK = enc
			v.mu.Unlock()
			return key, nil
		}
	}
	if !create {
		return nil, fmt.Errorf("vault key not set")
	}
	if v.dek == nil {
		return nil, fmt.Errorf("dek store is nil")
	}
	key, err := GenerateDEK()
	if err != nil {
		return nil, err
	}
	enc, err := EncodeDEK(key)
	if err != nil {
		return nil, err
	}
	if err := v.storeDEK(enc, "Buddy wants to create a vault key"); err != nil {
		return nil, err
	}
	v.mu.Lock()
	v.sessionDEK = enc
	v.mu.Unlock()
	return key, nil
}

func (v *VaultStore) storeDEK(enc, reason string) error {
	if v.dek == nil {
		return fmt.Errorf("dek store is nil")
	}
	if p, ok := v.dek.(presenceDEKStore); ok {
		return p.SetPresence(enc, reason)
	}
	return v.dek.Set(enc)
}

func (v *VaultStore) dekOptional() (string, error) {
	if err := v.ensureSessionDEK(DefaultUnlockReason); err != nil {
		return "", err
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.sessionDEK, nil
}

func (v *VaultStore) ensureSessionDEK(reason string) error {
	return v.unlockSession(reason, false)
}

// ensureSessionDEKOptional is for process start (mcp/serve): OK if no DEK yet.
func (v *VaultStore) ensureSessionDEKOptional(reason string) error {
	return v.unlockSession(reason, true)
}

// unlockSession loads the DEK into session memory. Keychain I/O runs outside v.mu.
func (v *VaultStore) unlockSession(reason string, allowMissing bool) error {
	if v == nil {
		return fmt.Errorf("vault store is nil")
	}
	v.mu.Lock()
	if _, ok := ParseDEK(v.sessionDEK); ok {
		v.mu.Unlock()
		return nil
	}
	v.mu.Unlock()

	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = DefaultUnlockReason
	}
	enc, err := v.fetchDEK(reason)
	if err != nil {
		if allowMissing && isVaultKeyMissing(err) {
			return nil
		}
		return err
	}
	if _, ok := ParseDEK(enc); !ok {
		if allowMissing {
			return nil
		}
		return fmt.Errorf("vault key not set")
	}
	v.mu.Lock()
	v.sessionDEK = enc
	v.mu.Unlock()
	return nil
}

func isVaultKeyMissing(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "not found") || strings.Contains(s, "vault key not set")
}

func (v *VaultStore) fetchDEK(reason string) (string, error) {
	if v.dek == nil {
		return "", fmt.Errorf("dek store is nil")
	}
	if p, ok := v.dek.(presenceDEKStore); ok {
		return p.GetPresence(reason)
	}
	return v.dek.Get()
}

func (v *VaultStore) recoveryStore() MasterStore {
	if v != nil && v.recovery != nil {
		return v.recovery
	}
	return nil
}

func (v *VaultStore) recoveryOptional() string {
	rec := v.recoveryStore()
	if rec == nil {
		return ""
	}
	p, err := rec.Get()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(p)
}

func cloneVaultPayload(p vaultPayload) vaultPayload {
	out := vaultPayload{Secrets: map[string]secretFile{}}
	for k, v := range p.Secrets {
		out.Secrets[k] = v
	}
	return out
}
