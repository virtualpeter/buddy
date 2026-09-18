package buddy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"buddy/internal/buddy/secrets"
)

const (
	credBackupAAD  = secrets.AADCredBackup
	credBackupKind = secrets.KindCredBackup
)

// credBackupIter is the PBKDF2 iteration count for new backups (tests may lower it).
var credBackupIter = secrets.DefaultEnvelopeIter

type credBackupEnvelope = secrets.Envelope

// credBackupPayload is the decrypted JSON body.
type credBackupPayload struct {
	Tenant   TenantSelection          `json:"tenant"`
	Tenants  map[string]TenantConfig  `json:"tenants"`
	Orgs     map[string]OrgConfig     `json:"orgs,omitempty"`
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
	if len(cfg.Tenants) == 0 && len(cfg.Orgs) == 0 && len(cfg.Auths) == 0 {
		return nil, fmt.Errorf("nothing to backup (no tenants or auths)")
	}
	prevIter := secrets.EnvelopeIter
	secrets.EnvelopeIter = credBackupIter
	defer func() { secrets.EnvelopeIter = prevIter }()
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
	prevIter := secrets.EnvelopeIter
	secrets.EnvelopeIter = credBackupIter
	defer func() { secrets.EnvelopeIter = prevIter }()
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
	if len(payload.Tenants) == 0 && len(payload.Orgs) == 0 && len(payload.Auths) == 0 {
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
		cfg.Tenants[n] = cloneTenantConfig(t)
	}
	cfg.Orgs = nil
	if len(payload.Orgs) > 0 {
		cfg.Orgs = make(map[string]OrgConfig, len(payload.Orgs))
		for n, o := range payload.Orgs {
			cfg.Orgs[n] = cloneOrgConfig(o)
		}
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

	if !secrets.UsingOverrideStore() {
		if ok, err := secrets.HasMasterPassphrase(); err == nil && !ok {
			if err := secrets.SetMasterPassphrase(pass); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not wrap vault with backup passphrase: %v (run 'buddy cred set passphrase')\n", err)
			}
		}
	}

	stored := 0
	for _, p := range accept {
		n, err := restoreAuthFromBackup(cfg, p.name, p.auth)
		if err != nil {
			return nil, err
		}
		stored += n
	}

	if err := expandOrgs(cfg); err != nil {
		return nil, fmt.Errorf("restored config: %w", err)
	}
	if err := cfg.ValidateTenantSelectionAndChannels(); err != nil {
		return nil, fmt.Errorf("restored config: %w", err)
	}
	nTenants := len(cfg.Tenants)
	if err := SaveConfig(cfg); err != nil {
		return nil, fmt.Errorf("save config: %w", err)
	}
	return &CredRestoreResult{
		Path:    path,
		Tenants: nTenants,
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
	case "oauth", "platform":
		return fmt.Sprintf("Restore auth %q (%s clientId=%s tenants=%s)?", name, a.Type, a.ClientID, tenants)
	case "basic":
		return fmt.Sprintf("Restore auth %q (basic user=%s tenants=%s)?", name, a.Username, tenants)
	case "command":
		return fmt.Sprintf("Restore auth %q (command tenants=%s)?", name, tenants)
	default:
		return fmt.Sprintf("Restore auth %q (%s tenants=%s)?", name, a.Type, tenants)
	}
}

func resolveBackupPassphrase(provided string, confirm bool) (string, error) {
	pass := strings.TrimSpace(provided)
	// Never default to a machine-stored Keychain/file master — backup encryption
	// must be a knowledge factor (typed / --passphrase=), not something already
	// on the machine that could decrypt buddy.creds after theft.
	if pass == "" {
		if p := strings.TrimSpace(os.Getenv(secrets.EnvVaultPassphrase)); p != "" {
			if _, isDEK := secrets.ParseDEK(p); !isDEK {
				pass = p
			}
		}
	}
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
		out.Tenants[n] = cloneTenantConfig(t)
	}
	if len(cfg.Orgs) > 0 {
		out.Orgs = make(map[string]OrgConfig, len(cfg.Orgs))
		for n, o := range cfg.Orgs {
			out.Orgs[n] = cloneOrgConfig(o)
		}
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
		case "oauth", "platform":
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
	prev := secrets.EnvelopeIter
	secrets.EnvelopeIter = credBackupIter
	defer func() { secrets.EnvelopeIter = prev }()
	return secrets.EncryptEnvelope(plain, passphrase, credBackupKind, credBackupAAD)
}

func decryptCredBackup(raw []byte, passphrase string) ([]byte, error) {
	return secrets.DecryptEnvelope(raw, passphrase, credBackupKind, credBackupAAD, "cred backup")
}
