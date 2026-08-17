package buddy

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"buddy/internal/buddy/secrets"
)

const (
	credBackupVersion     = 1
	credBackupKDF         = "pbkdf2-sha256"
	credBackupSaltLen     = 16
	credBackupKeyLen      = 32
	credBackupAAD         = "buddy-cred-backup-v1"
	credBackupKind        = "cred-backup"
	credBackupDefaultIter = 600_000
)

// credBackupIter is the PBKDF2 iteration count for new backups (tests may lower it).
var credBackupIter = credBackupDefaultIter

// credBackupEnvelope is the on-disk JSON wrapper (no secrets in plaintext).
type credBackupEnvelope struct {
	Buddy string `json:"buddy"`
	V     int    `json:"v"`
	KDF   string `json:"kdf"`
	Iter  int    `json:"iter"`
	Salt  string `json:"salt"`
	Data  string `json:"data"` // base64(nonce || ciphertext)
}

// credBackupPayload is the decrypted JSON body.
type credBackupPayload struct {
	Tenant   TenantSelection          `json:"tenant"`
	Tenants  map[string]TenantConfig  `json:"tenants"`
	Auths    map[string]AuthEntry     `json:"auths"`
	Channels map[string]ChannelPolicy `json:"channels,omitempty"`
}

// CredBackupOpts controls BackupCreds / RestoreCreds.
type CredBackupOpts struct {
	Path        string
	Passphrase  string // empty → prompt (backup prompts twice)
	SkipConfirm bool
}

// CredBackupResult is returned by BackupCreds.
type CredBackupResult struct {
	Path           string
	Tenants        int
	Auths          int
	MissingSecrets []string
}

// CredRestoreResult is returned by RestoreCreds.
type CredRestoreResult struct {
	Path    string
	Tenants int
	Auths   int
	Stored  int
	Skipped int
}

// BackupCreds writes an encrypted snapshot of tenants, auths (with store secrets),
// channels, and tenant.default to opts.Path (mode 0600).
func BackupCreds(cfg *Config, opts CredBackupOpts) (*CredBackupResult, error) {
	if cfg == nil {
		return nil, fmt.Errorf("internal error: config is nil")
	}
	path := strings.TrimSpace(opts.Path)
	if path == "" {
		return nil, fmt.Errorf("backup path is required")
	}
	if len(cfg.Tenants) == 0 && len(cfg.Auths) == 0 {
		return nil, fmt.Errorf("nothing to backup (no tenants or auths)")
	}
	pass, err := resolveBackupPassphrase(opts.Passphrase, true)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); err == nil && !opts.SkipConfirm {
		if !AskYesNo(fmt.Sprintf("Overwrite %s?", path), false, nil) {
			return nil, nil
		}
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	payload, missing := collectCredBackup(cfg)
	if len(payload.Auths) == 0 {
		fmt.Fprintln(os.Stderr, "warning: backup contains no auths (tenants only)")
	}
	if orphans, err := OrphanCredentialSecrets(cfg); err == nil && len(orphans) > 0 {
		fmt.Fprintf(os.Stderr, "warning: %d secret(s) in the store have no matching auth in config and will not be in this backup\n", len(orphans))
	}
	plain, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode backup: %w", err)
	}
	env, err := encryptCredBackup(plain, pass)
	if err != nil {
		return nil, err
	}
	raw, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return nil, err
	}
	raw = append(raw, '\n')
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create backup directory: %w", err)
		}
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return nil, fmt.Errorf("write backup %s: %w", path, err)
	}
	return &CredBackupResult{
		Path:           path,
		Tenants:        len(payload.Tenants),
		Auths:          len(payload.Auths),
		MissingSecrets: missing,
	}, nil
}

// RestoreCreds decrypts a backup and replaces tenants/channels in cfg.
// Each backup auth is confirmed unless opts.SkipConfirm (Enter restores).
// n skips that auth and leaves the existing config entry (or absence) alone.
// Auths not in the backup are kept. Secrets go into the platform store when available.
func RestoreCreds(cfg *Config, opts CredBackupOpts) (*CredRestoreResult, error) {
	if cfg == nil {
		return nil, fmt.Errorf("internal error: config is nil")
	}
	path := strings.TrimSpace(opts.Path)
	if path == "" {
		return nil, fmt.Errorf("backup path is required")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read backup %s: %w", path, err)
	}
	pass, err := resolveBackupPassphrase(opts.Passphrase, false)
	if err != nil {
		return nil, err
	}
	plain, err := decryptCredBackup(raw, pass)
	if err != nil {
		return nil, err
	}
	var payload credBackupPayload
	if err := json.Unmarshal(plain, &payload); err != nil {
		return nil, fmt.Errorf("parse backup: %w", err)
	}
	if len(payload.Tenants) == 0 && len(payload.Auths) == 0 {
		return nil, fmt.Errorf("backup %s has no tenants or auths", path)
	}
	if !opts.SkipConfirm && !StdinIsTerminal() && currentConfirmAsk() == nil {
		return nil, fmt.Errorf("restore confirmation required for each auth (interactive TTY, or -y)")
	}

	names := make([]string, 0, len(payload.Auths))
	for name := range payload.Auths {
		names = append(names, name)
	}
	sort.Strings(names)

	type pendingAuth struct {
		name string
		auth AuthEntry
	}
	var accept []pendingAuth
	skipped := 0
	if len(names) == 0 {
		fmt.Fprintf(os.Stderr, "warning: backup has no auths; existing auths will be left unchanged\n")
	} else if !opts.SkipConfirm {
		fmt.Fprintf(confirmOut, "Restoring tenants/channels. For each auth: y writes from backup, n leaves existing config (%d):\n", len(names))
		for _, name := range names {
			a := payload.Auths[name]
			if !Confirm(ConfirmOpts{Question: authRestorePrompt(name, a), Default: "y", SilentNo: true}, nil) {
				skipped++
				continue
			}
			accept = append(accept, pendingAuth{name: name, auth: a})
		}
	} else {
		for _, name := range names {
			accept = append(accept, pendingAuth{name: name, auth: payload.Auths[name]})
		}
	}

	cfg.Tenant = payload.Tenant
	cfg.Tenants = map[string]TenantConfig{}
	for n, t := range payload.Tenants {
		cfg.Tenants[n] = t
	}
	cfg.Channels = map[string]ChannelPolicy{}
	for n, p := range payload.Channels {
		cfg.Channels[n] = cloneChannelPolicy(p)
	}

	if cfg.Auths == nil {
		cfg.Auths = map[string]AuthEntry{}
	} else {
		cloned := make(map[string]AuthEntry, len(cfg.Auths))
		for n, a := range cfg.Auths {
			cloned[n] = cloneAuthEntry(a)
		}
		cfg.Auths = cloned
	}

	stored := 0
	for _, p := range accept {
		n, err := restoreAuthFromBackup(cfg, p.name, p.auth)
		if err != nil {
			return nil, err
		}
		stored += n
	}

	if err := cfg.ValidateTenantSelectionAndChannels(); err != nil {
		return nil, fmt.Errorf("restored config: %w", err)
	}
	if err := SaveConfig(cfg); err != nil {
		return nil, fmt.Errorf("save config: %w", err)
	}
	return &CredRestoreResult{
		Path:    path,
		Tenants: len(cfg.Tenants),
		Auths:   len(accept),
		Stored:  stored,
		Skipped: skipped,
	}, nil
}

func restoreAuthFromBackup(cfg *Config, name string, a AuthEntry) (stored int, err error) {
	pw, cs := a.Password, a.ClientSecret
	a.Password = ""
	a.ClientSecret = ""
	if err := a.Validate(); err != nil {
		return 0, fmt.Errorf("backup auth %q: %w", name, err)
	}
	cfg.Auths[name] = cloneAuthEntry(a)
	if pw != "" {
		if err := SetCredentialSecret(name, secrets.KindPassword, pw); err != nil {
			a.Password = pw
			cfg.Auths[name] = cloneAuthEntry(a)
			fmt.Fprintf(os.Stderr, "warning: could not store password for %q: %v (kept inline)\n", name, err)
		} else {
			stored++
		}
	}
	if cs != "" {
		a = cfg.Auths[name]
		if err := SetCredentialSecret(name, secrets.KindClientSecret, cs); err != nil {
			a.ClientSecret = cs
			cfg.Auths[name] = cloneAuthEntry(a)
			fmt.Fprintf(os.Stderr, "warning: could not store client secret for %q: %v (kept inline)\n", name, err)
		} else {
			stored++
		}
	}
	return stored, nil
}

func authRestorePrompt(name string, a AuthEntry) string {
	tenants := strings.Join(a.Tenants, ",")
	switch strings.ToLower(strings.TrimSpace(a.Type)) {
	case "oauth":
		return fmt.Sprintf("Restore auth %q (oauth clientId=%s tenants=%s)?", name, a.ClientID, tenants)
	case "basic":
		return fmt.Sprintf("Restore auth %q (basic user=%s tenants=%s)?", name, a.Username, tenants)
	case "command":
		return fmt.Sprintf("Restore auth %q (command tenants=%s)?", name, tenants)
	default:
		return fmt.Sprintf("Restore auth %q (%s tenants=%s)?", name, a.Type, tenants)
	}
}

func resolveBackupPassphrase(provided string, confirm bool) (string, error) {
	pass := provided
	if pass == "" {
		if !StdinIsTerminal() {
			return "", fmt.Errorf("passphrase required (interactive prompt, or --passphrase=)")
		}
		var err error
		pass, err = PromptSecret("Encryption passphrase")
		if err != nil {
			return "", err
		}
		if confirm {
			again, err := PromptSecret("Confirm passphrase")
			if err != nil {
				return "", err
			}
			if pass != again {
				return "", fmt.Errorf("passphrases do not match")
			}
		}
	}
	if strings.TrimSpace(pass) == "" {
		return "", fmt.Errorf("passphrase must not be empty")
	}
	return pass, nil
}

func collectCredBackup(cfg *Config) (credBackupPayload, []string) {
	out := credBackupPayload{
		Tenant:  cfg.Tenant,
		Tenants: map[string]TenantConfig{},
		Auths:   map[string]AuthEntry{},
	}
	for n, t := range cfg.Tenants {
		out.Tenants[n] = t
	}
	if len(cfg.Channels) > 0 {
		out.Channels = map[string]ChannelPolicy{}
		for n, p := range cfg.Channels {
			out.Channels[n] = cloneChannelPolicy(p)
		}
	}
	var missing []string
	names := make([]string, 0, len(cfg.Auths))
	for n := range cfg.Auths {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		a := cloneAuthEntry(cfg.Auths[name])
		switch strings.ToLower(strings.TrimSpace(a.Type)) {
		case "command":
			// Command auth is the runCommand only — do not query the secret store
			// or execute the command.
			a.Password = ""
			a.ClientSecret = ""
		case "basic":
			if a.Password == "" {
				if pw := peekStoreSecret(name, secrets.KindPassword); pw != "" {
					a.Password = pw
				} else {
					missing = append(missing, name+" password")
				}
			}
		case "oauth":
			if a.ClientSecret == "" {
				if cs := peekStoreSecret(name, secrets.KindClientSecret); cs != "" {
					a.ClientSecret = cs
				} else {
					missing = append(missing, name+" client-secret")
				}
			}
		}
		out.Auths[name] = a
	}
	return out, missing
}

func peekStoreSecret(authName string, kind secrets.Kind) string {
	store, err := secrets.DefaultStore()
	if err != nil {
		return ""
	}
	ok, err := store.Has(authName, kind)
	if err != nil || !ok {
		return ""
	}
	v, err := store.Get(authName, kind)
	if err != nil {
		return ""
	}
	return v
}

func encryptCredBackup(plain []byte, passphrase string) (credBackupEnvelope, error) {
	salt := make([]byte, credBackupSaltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return credBackupEnvelope{}, fmt.Errorf("salt: %w", err)
	}
	key, err := pbkdf2.Key(sha256.New, passphrase, salt, credBackupIter, credBackupKeyLen)
	if err != nil {
		return credBackupEnvelope{}, fmt.Errorf("kdf: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return credBackupEnvelope{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return credBackupEnvelope{}, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return credBackupEnvelope{}, fmt.Errorf("nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, plain, []byte(credBackupAAD))
	return credBackupEnvelope{
		Buddy: credBackupKind,
		V:     credBackupVersion,
		KDF:   credBackupKDF,
		Iter:  credBackupIter,
		Salt:  base64.StdEncoding.EncodeToString(salt),
		Data:  base64.StdEncoding.EncodeToString(sealed),
	}, nil
}

func decryptCredBackup(raw []byte, passphrase string) ([]byte, error) {
	var env credBackupEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("not a buddy cred backup (parse): %w", err)
	}
	if env.Buddy != credBackupKind || env.V != credBackupVersion {
		return nil, fmt.Errorf("not a buddy cred backup (buddy=%q v=%d)", env.Buddy, env.V)
	}
	if env.KDF != credBackupKDF {
		return nil, fmt.Errorf("unsupported kdf %q", env.KDF)
	}
	if env.Iter < 1000 || env.Iter > 10_000_000 {
		return nil, fmt.Errorf("invalid kdf iteration count")
	}
	salt, err := base64.StdEncoding.DecodeString(env.Salt)
	if err != nil || len(salt) < 8 {
		return nil, fmt.Errorf("invalid salt")
	}
	sealed, err := base64.StdEncoding.DecodeString(env.Data)
	if err != nil {
		return nil, fmt.Errorf("invalid ciphertext")
	}
	key, err := pbkdf2.Key(sha256.New, passphrase, salt, env.Iter, credBackupKeyLen)
	if err != nil {
		return nil, fmt.Errorf("kdf: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	if len(sealed) < ns {
		return nil, fmt.Errorf("ciphertext too short")
	}
	plain, err := gcm.Open(nil, sealed[:ns], sealed[ns:], []byte(credBackupAAD))
	if err != nil {
		return nil, fmt.Errorf("wrong passphrase or corrupt backup")
	}
	return plain, nil
}
