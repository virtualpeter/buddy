package buddy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"buddy/internal/buddy/agent"
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
// Backup includes workplaces, auths, policy, and secrets unless a No* flag is set.
type CredBackupOpts struct {
	Path         string
	Passphrase   string // empty → prompt when secrets are included (backup prompts twice)
	SkipConfirm  bool
	NoWorkplaces bool
	NoAuths      bool
	NoPolicy     bool
	NoSecrets    bool
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

// BackupCreds writes the same document as a Buddy Agent export: identity JSON
// with an optional encrypted secrets block. Secrets are included unless
// NoSecrets or NoAuths is set. Older whole-file backups remain readable by RestoreCreds.
func BackupCreds(cfg *Config, opts CredBackupOpts) (*CredBackupResult, error) {
	if cfg == nil {
		return nil, fmt.Errorf("internal error: config is nil")
	}
	path := strings.TrimSpace(opts.Path)
	if path == "" {
		return nil, fmt.Errorf("backup path is required")
	}
	includeWorkplaces := !opts.NoWorkplaces
	includeAuths := !opts.NoAuths
	includePolicy := !opts.NoPolicy
	includeSecrets := !opts.NoSecrets && includeAuths
	if !includeWorkplaces && !includeAuths && !includePolicy {
		return nil, fmt.Errorf("nothing to backup")
	}
	if includeWorkplaces && len(cfg.Tenants) == 0 && len(cfg.Orgs) == 0 && includeAuths && len(cfg.Auths) == 0 && !includePolicy {
		return nil, fmt.Errorf("nothing to backup (no tenants or auths)")
	}
	prevIter := secrets.EnvelopeIter
	secrets.EnvelopeIter = credBackupIter
	defer func() { secrets.EnvelopeIter = prevIter }()
	var pass string
	if includeSecrets {
		var err error
		pass, err = resolveBackupPassphrase(opts.Passphrase, true)
		if err != nil {
			return nil, err
		}
	}
	if _, err := os.Stat(path); err == nil && !opts.SkipConfirm {
		if !AskYesNo(fmt.Sprintf("Overwrite %s?", path), false, nil) {
			return nil, nil
		}
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	doc := map[string]any{}
	nTenants := 0
	if includeWorkplaces {
		if def := strings.TrimSpace(cfg.Tenant.Default); def != "" {
			doc["context"] = map[string]string{"default": def}
		}
		if len(cfg.Orgs) > 0 {
			doc["orgs"] = cfg.Orgs
		}
		if contexts := exportContexts(cfg); len(contexts) > 0 {
			doc["contexts"] = contexts
		}
		if len(cfg.Tenants) > 0 {
			nTenants = len(cfg.Tenants)
		} else if implied, err := workplacesFromOrgs(cfg.Orgs); err == nil {
			nTenants = len(implied)
		}
	}
	nAuths := 0
	if includeAuths {
		auths := exportAuths(cfg)
		doc["auths"] = auths
		nAuths = len(auths)
		if nAuths == 0 {
			fmt.Fprintln(os.Stderr, "warning: backup contains no auths (tenants only)")
		}
	}
	if includePolicy {
		doc["features"] = exportFeatures(cfg.Features)
		doc["channels"] = exportChannels(cfg.Channels)
	}
	var missing []string
	if includeSecrets {
		if orphans, err := OrphanCredentialSecrets(cfg); err == nil && len(orphans) > 0 {
			fmt.Fprintf(os.Stderr, "warning: %d secret(s) in the store have no matching auth in config and will not be in this backup\n", len(orphans))
		}
		secretMap, missingSecrets := exportSecretMap(cfg)
		missing = missingSecrets
		plain, err := json.Marshal(secretMap)
		if err != nil {
			return nil, fmt.Errorf("encode secrets: %w", err)
		}
		env, err := encryptCredBackup(plain, pass)
		if err != nil {
			return nil, err
		}
		doc["secrets"] = env
	}
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode backup: %w", err)
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
		Tenants:        nTenants,
		Auths:          nAuths,
		MissingSecrets: missing,
	}, nil
}

func exportContexts(cfg *Config) map[string]exportWorkplace {
	implied, _ := workplacesFromOrgs(cfg.Orgs)
	out := map[string]exportWorkplace{}
	names := make([]string, 0, len(cfg.Tenants))
	for name := range cfg.Tenants {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		stored := cfg.Tenants[name]
		if src, ok := implied[name]; ok && workplaceIdentityEqual(stored, src) &&
			len(stored.DenyBuiltins) == 0 && len(stored.AllowBuiltins) == 0 {
			continue
		}
		out[name] = exportWorkplaceFrom(stored)
	}
	return out
}

type exportWorkplace struct {
	Org           string   `json:"org,omitempty"`
	Region        string   `json:"region,omitempty"`
	URL           string   `json:"url,omitempty"`
	EnvironmentID string   `json:"environmentId,omitempty"`
	TenantID      string   `json:"tenantId,omitempty"`
	DenyBuiltins  []string `json:"deny_builtins,omitempty"`
	AllowBuiltins []string `json:"allow_builtins,omitempty"`
}

func exportWorkplaceFrom(t TenantConfig) exportWorkplace {
	return exportWorkplace{
		Org:           strings.TrimSpace(t.Org),
		Region:        strings.TrimSpace(t.Region),
		URL:           strings.TrimSpace(t.URL),
		EnvironmentID: strings.TrimSpace(t.EnvironmentID),
		TenantID:      strings.TrimSpace(t.TenantID),
		DenyBuiltins:  append([]string(nil), t.DenyBuiltins...),
		AllowBuiltins: append([]string(nil), t.AllowBuiltins...),
	}
}

type exportAuth struct {
	Type          string   `json:"type"`
	ClientID      string   `json:"clientId,omitempty"`
	Username      string   `json:"username,omitempty"`
	RunCommand    string   `json:"runCommand,omitempty"`
	Scope         string   `json:"scope,omitempty"`
	Access        string   `json:"access,omitempty"`
	Contexts      []string `json:"contexts,omitempty"`
	Channels      []string `json:"channels,omitempty"`
	DenyBuiltins  []string `json:"deny_builtins,omitempty"`
	AllowBuiltins []string `json:"allow_builtins,omitempty"`
}

func exportAuths(cfg *Config) map[string]exportAuth {
	out := map[string]exportAuth{}
	names := make([]string, 0, len(cfg.Auths))
	for name := range cfg.Auths {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		a := cfg.Auths[name]
		links := mergeAuthLinks(a.Tenants, a.Contexts, a.JamfPro)
		out[name] = exportAuth{
			Type:          a.Type,
			ClientID:      strings.TrimSpace(a.ClientID),
			Username:      strings.TrimSpace(a.Username),
			RunCommand:    strings.TrimSpace(a.RunCommand),
			Scope:         strings.TrimSpace(a.Scope),
			Access:        strings.TrimSpace(a.Access),
			Contexts:      links,
			Channels:      append([]string(nil), a.Channels...),
			DenyBuiltins:  append([]string(nil), a.DenyBuiltins...),
			AllowBuiltins: append([]string(nil), a.AllowBuiltins...),
		}
	}
	return out
}

func exportFeatures(f FeatureFlags) map[string]any {
	classic := false
	if f.ClassicInstanceOnly != nil {
		classic = *f.ClassicInstanceOnly
	}
	out := map[string]any{"classic_instance_only": classic}
	if f.CheckUpdates != nil {
		out["check_updates"] = *f.CheckUpdates
	}
	return out
}

func exportChannels(channels map[string]ChannelPolicy) map[string]any {
	out := map[string]any{}
	names := make([]string, 0, len(channels))
	for name := range channels {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		p := channels[name]
		entry := map[string]any{}
		if access := strings.TrimSpace(p.Access); access != "" {
			entry["access"] = access
		}
		if ctx := strings.TrimSpace(p.Tenant); ctx != "" {
			entry["context"] = ctx
		}
		if len(p.Tenants) > 0 {
			entry["contexts"] = append([]string(nil), p.Tenants...)
		}
		if len(p.DenyBuiltins) > 0 {
			entry["deny_builtins"] = append([]string(nil), p.DenyBuiltins...)
		}
		if len(p.AllowBuiltins) > 0 {
			entry["allow_builtins"] = append([]string(nil), p.AllowBuiltins...)
		}
		if len(entry) > 0 {
			out[name] = entry
		}
	}
	return out
}

func exportSecretMap(cfg *Config) (map[string]string, []string) {
	out := map[string]string{}
	var missing []string
	heldByAgent := cfg != nil && (cfg.UseAgent() || cfg.ForceAgent() || cfg.HasAgentConfig())
	why := "not in config or store"
	if heldByAgent {
		why = "held by Buddy Agent; export from the Agent"
	}
	names := make([]string, 0, len(cfg.Auths))
	for name := range cfg.Auths {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		a := cfg.Auths[name]
		switch NormalizeAuthType(a.Type) {
		case AuthTypeCommand:
		case AuthTypeBasic:
			pw := strings.TrimSpace(a.Password)
			if pw == "" {
				pw = peekStoreSecret(name, secrets.KindPassword)
			}
			if pw == "" {
				missing = append(missing, name+" password ("+why+")")
			} else {
				out[name] = pw
			}
		case AuthTypeOAuth, AuthTypePlatform:
			cs := strings.TrimSpace(a.ClientSecret)
			if cs == "" {
				cs = peekStoreSecret(name, secrets.KindClientSecret)
			}
			if cs == "" {
				missing = append(missing, name+" client-secret ("+why+")")
			} else {
				out[name] = cs
			}
		}
	}
	return out, missing
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
	if err := refuseCredRestoreSecrets(cfg, raw); err != nil {
		return nil, err
	}
	prevIter := secrets.EnvelopeIter
	secrets.EnvelopeIter = credBackupIter
	defer func() { secrets.EnvelopeIter = prevIter }()
	payload, features, pass, err := readRestoreDocument(raw, func() (string, error) {
		return resolveBackupPassphrase(opts.Passphrase, false)
	})
	if err != nil {
		return nil, err
	}
	if len(payload.Tenants) == 0 && len(payload.Orgs) == 0 && len(payload.Auths) == 0 && features == nil && len(payload.Channels) == 0 {
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
	if features != nil {
		cfg.Features = *features
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

	if pass != "" && !secrets.UsingOverrideStore() {
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

// refuseCredRestoreSecrets stops a CLI restore from writing a second secret
// store while Buddy Agent is the one that holds secrets.
func refuseCredRestoreSecrets(cfg *Config, raw []byte) error {
	secretsInFile, err := backupCarriesSecrets(raw)
	if err != nil {
		return err
	}
	if cfg.UseAgent() {
		return fmt.Errorf("Buddy Agent owns this configuration; import the file in the Agent window so secrets are not also stored in the CLI vault")
	}
	if !secretsInFile {
		return nil
	}
	present, err := buddyAgentListening()
	if err != nil {
		return err
	}
	if present {
		return fmt.Errorf("Buddy Agent is running; import the file in the Agent window so secrets stay in one place")
	}
	return nil
}

func backupCarriesSecrets(raw []byte) (bool, error) {
	var probe struct {
		Buddy   string          `json:"buddy"`
		Secrets json.RawMessage `json:"secrets"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return false, fmt.Errorf("parse backup: %w", err)
	}
	if probe.Buddy == credBackupKind {
		return true, nil
	}
	if len(probe.Secrets) == 0 || string(probe.Secrets) == "null" {
		return false, nil
	}
	return true, nil
}

// buddyAgentListening reports whether a Buddy Agent socket answered.
// Tests replace it.
var buddyAgentListening = func() (bool, error) {
	if !agent.Enabled() {
		return false, nil
	}
	info, err := agent.Status(context.Background())
	if errors.Is(err, agent.ErrAbsent) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check Buddy Agent: %w", err)
	}
	return info.Present, nil
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

// readRestoreDocument accepts either an older whole-file cred backup or a
// Buddy Agent export (identity JSON plus an optional encrypted secrets block).
// The passphrase function runs only when the file actually contains secrets.
func readRestoreDocument(raw []byte, passphrase func() (string, error)) (credBackupPayload, *FeatureFlags, string, error) {
	var probe struct {
		Buddy string `json:"buddy"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return credBackupPayload{}, nil, "", fmt.Errorf("parse backup: %w", err)
	}
	if probe.Buddy == credBackupKind {
		pass, err := passphrase()
		if err != nil {
			return credBackupPayload{}, nil, "", err
		}
		plain, err := decryptCredBackup(raw, pass)
		if err != nil {
			return credBackupPayload{}, nil, "", err
		}
		var payload credBackupPayload
		if err := json.Unmarshal(plain, &payload); err != nil {
			return credBackupPayload{}, nil, "", fmt.Errorf("parse backup: %w", err)
		}
		return payload, nil, pass, nil
	}

	var imported Config
	if err := yaml.Unmarshal(raw, &imported); err != nil {
		return credBackupPayload{}, nil, "", fmt.Errorf("parse backup: %w", err)
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return credBackupPayload{}, nil, "", fmt.Errorf("parse backup: %w", err)
	}
	var pass string
	if block, ok := root["secrets"]; ok && len(block) > 0 && string(block) != "null" {
		var err error
		pass, err = passphrase()
		if err != nil {
			return credBackupPayload{}, nil, "", err
		}
		plain, err := decryptCredBackup(block, pass)
		if err != nil {
			return credBackupPayload{}, nil, "", err
		}
		var secretMap map[string]string
		if err := json.Unmarshal(plain, &secretMap); err != nil {
			return credBackupPayload{}, nil, "", fmt.Errorf("parse secrets: %w", err)
		}
		if err := applyExportSecrets(imported.Auths, secretMap); err != nil {
			return credBackupPayload{}, nil, "", err
		}
	}
	payload := credBackupPayload{
		Tenant:   imported.Tenant,
		Tenants:  imported.Tenants,
		Orgs:     imported.Orgs,
		Auths:    imported.Auths,
		Channels: imported.Channels,
	}
	var features *FeatureFlags
	if _, ok := root["features"]; ok {
		f := imported.Features
		features = &f
	}
	return payload, features, pass, nil
}

func applyExportSecrets(auths map[string]AuthEntry, secretMap map[string]string) error {
	for name, secret := range secretMap {
		secret = strings.TrimSpace(secret)
		if secret == "" {
			continue
		}
		a, ok := auths[name]
		if !ok {
			return fmt.Errorf("secret %q does not match an auth in the file", name)
		}
		switch NormalizeAuthType(a.Type) {
		case AuthTypeBasic:
			a.Password = secret
		case AuthTypeOAuth, AuthTypePlatform:
			a.ClientSecret = secret
		default:
			return fmt.Errorf("secret %q is for auth type %q, which does not take a stored secret", name, a.Type)
		}
		auths[name] = a
	}
	return nil
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
