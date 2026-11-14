package buddy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"buddy/internal/buddy/secrets"
)

func init() {
	// cred restore refuses while a real Agent socket is up. Keep the suite
	// on the CLI vault path unless a test turns the probe back on.
	buddyAgentListening = func() (bool, error) { return false, nil }
}

func TestCredBackupRestoreRoundTrip(t *testing.T) {
	prev := credBackupIter
	credBackupIter = 10_000
	t.Cleanup(func() { credBackupIter = prev })
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "no-managed.plist"))
	secrets.SetStore(&mockSecretStore{values: map[string]string{
		"sandbox-cli/client-secret": "s3cret",
	}})
	t.Cleanup(func() { secrets.SetStore(nil) })

	cfg := &Config{
		Tenant: TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{
			"sandbox": {URL: "https://example.jamfcloud.com"},
		},
		Auths: map[string]AuthEntry{
			"sandbox-cli": {
				Type:     "oauth",
				ClientID: "cid",
				Tenants:  []string{"sandbox"},
				Surface:  SurfaceInstance,
			},
		},
		Channels: map[string]ChannelPolicy{
			"mcp": {Access: AccessReadonly, Tenant: "sandbox"},
		},
	}
	path := filepath.Join(dir, "buddy.backup")
	res, err := BackupCreds(cfg, CredBackupOpts{Path: path, Passphrase: "correct horse", SkipConfirm: true})
	if err != nil {
		t.Fatal(err)
	}
	if res == nil || res.Auths != 1 || res.Tenants != 1 {
		t.Fatalf("backup result %+v", res)
	}
	if len(res.MissingSecrets) != 0 {
		t.Fatalf("missing %v", res.MissingSecrets)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "s3cret") {
		t.Fatal("secret in plaintext backup file")
	}

	secrets.SetStore(&mockSecretStore{values: map[string]string{}})
	empty := &Config{Tenant: TenantSelection{Default: DefaultTenant}}
	got, err := RestoreCreds(empty, CredBackupOpts{Path: path, Passphrase: "correct horse", SkipConfirm: true})
	if err != nil {
		t.Fatal(err)
	}
	if got.Auths != 1 || got.Stored != 1 {
		t.Fatalf("restore %+v", got)
	}
	if empty.Tenants["sandbox"].URL != "https://example.jamfcloud.com" {
		t.Fatalf("tenant %+v", empty.Tenants["sandbox"])
	}
	a := empty.Auths["sandbox-cli"]
	if a.ClientID != "cid" || a.ClientSecret != "" {
		t.Fatalf("auth should keep clientId and not inline secret: %+v", a)
	}
	ok, err := HasCredentialSecret("sandbox-cli", secrets.KindClientSecret)
	if err != nil || !ok {
		t.Fatalf("secret stored? %v %v", ok, err)
	}
	if empty.Channels["mcp"].Access != AccessReadonly {
		t.Fatalf("channels %+v", empty.Channels)
	}
}

func TestCredBackupWrongPassphrase(t *testing.T) {
	prev := credBackupIter
	credBackupIter = 10_000
	t.Cleanup(func() { credBackupIter = prev })
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	secrets.SetStore(&mockSecretStore{values: map[string]string{}})
	t.Cleanup(func() { secrets.SetStore(nil) })
	cfg := &Config{
		Tenant:  TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{"sandbox": {URL: "https://example.jamfcloud.com"}},
		Auths:   map[string]AuthEntry{"a": {Type: "oauth", ClientID: "c", Tenants: []string{"sandbox"}}},
	}
	path := filepath.Join(dir, "b.json")
	if _, err := BackupCreds(cfg, CredBackupOpts{Path: path, Passphrase: "right", SkipConfirm: true}); err != nil {
		t.Fatal(err)
	}
	_, err := RestoreCreds(&Config{}, CredBackupOpts{Path: path, Passphrase: "wrong", SkipConfirm: true})
	if err == nil || !strings.Contains(err.Error(), "wrong passphrase") {
		t.Fatalf("err = %v", err)
	}
}

// The Buddy Agent reads these files directly (AgentBackup.swift) to import an
// Buddy Agent import reads this document: clear identity plus one encrypted
// secrets block. Renaming these keys breaks that import.
func TestCredBackupPayloadKeysForAgentImport(t *testing.T) {
	prev := credBackupIter
	credBackupIter = 10_000
	t.Cleanup(func() { credBackupIter = prev })
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	secrets.SetStore(&mockSecretStore{values: map[string]string{}})
	t.Cleanup(func() { secrets.SetStore(nil) })

	cfg := &Config{
		Tenant:  TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{"sandbox": {URL: "https://example.jamfcloud.com"}},
		Auths: map[string]AuthEntry{
			"sandbox-cli": {Type: "oauth", ClientID: "cid", ClientSecret: "inline", Tenants: []string{"sandbox"}},
		},
	}
	path := filepath.Join(dir, "b.json")
	if _, err := BackupCreds(cfg, CredBackupOpts{Path: path, Passphrase: "pw", SkipConfirm: true}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"context", "contexts", "auths", "features", "channels", "secrets"} {
		if _, ok := doc[key]; !ok {
			t.Fatalf("export is missing %q: %v", key, doc)
		}
	}
	var selection map[string]string
	if err := json.Unmarshal(doc["context"], &selection); err != nil {
		t.Fatal(err)
	}
	if selection["default"] != "sandbox" {
		t.Fatalf("context = %v", selection)
	}
	var contexts map[string]map[string]any
	if err := json.Unmarshal(doc["contexts"], &contexts); err != nil {
		t.Fatal(err)
	}
	if contexts["sandbox"]["url"] != "https://example.jamfcloud.com" {
		t.Fatalf("context = %v", contexts["sandbox"])
	}
	var auths map[string]map[string]any
	if err := json.Unmarshal(doc["auths"], &auths); err != nil {
		t.Fatal(err)
	}
	got := auths["sandbox-cli"]
	if got["type"] != "oauth" || got["clientId"] != "cid" || got["clientSecret"] != nil {
		t.Fatalf("auth = %v", got)
	}
	if list, _ := got["contexts"].([]any); len(list) != 1 || list[0] != "sandbox" {
		t.Fatalf("auth contexts = %v", got["contexts"])
	}
	plain, err := decryptCredBackup(doc["secrets"], "pw")
	if err != nil {
		t.Fatal(err)
	}
	var secretMap map[string]string
	if err := json.Unmarshal(plain, &secretMap); err != nil {
		t.Fatal(err)
	}
	if secretMap["sandbox-cli"] != "inline" {
		t.Fatalf("secrets = %v", secretMap)
	}

	plainPath := filepath.Join(dir, "nosecrets.json")
	if _, err := BackupCreds(cfg, CredBackupOpts{Path: plainPath, NoSecrets: true, SkipConfirm: true}); err != nil {
		t.Fatal(err)
	}
	plainRaw, err := os.ReadFile(plainPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plainRaw), "inline") || strings.Contains(string(plainRaw), `"secrets"`) {
		t.Fatalf("secrets leaked into identity export: %s", plainRaw)
	}
}

func TestCredRestoreRefusesWhenAgentOwnsSecrets(t *testing.T) {
	prev := credBackupIter
	credBackupIter = 10_000
	t.Cleanup(func() { credBackupIter = prev })
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "no-managed.plist"))
	secrets.SetStore(&mockSecretStore{values: map[string]string{}})
	t.Cleanup(func() { secrets.SetStore(nil) })

	env, err := encryptCredBackup([]byte(`{"sandbox-cli":"s3cret"}`), "pw")
	if err != nil {
		t.Fatal(err)
	}
	envRaw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	doc := map[string]any{
		"context": map[string]string{"default": "sandbox"},
		"contexts": map[string]any{
			"sandbox": map[string]any{"url": "https://example.jamfcloud.com"},
		},
		"auths": map[string]any{
			"sandbox-cli": map[string]any{
				"type": "oauth", "clientId": "cid", "contexts": []string{"sandbox"},
			},
		},
		"secrets": json.RawMessage(envRaw),
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "export.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	on := true
	owned := &Config{Features: FeatureFlags{UseAgent: &on}}
	if _, err := RestoreCreds(owned, CredBackupOpts{Path: path, Passphrase: "pw", SkipConfirm: true}); err == nil {
		t.Fatal("expected restore to refuse while Buddy Agent owns configuration")
	}
	if ok, _ := HasCredentialSecret("sandbox-cli", secrets.KindClientSecret); ok {
		t.Fatal("secret was stored in the CLI vault")
	}

	prevListen := buddyAgentListening
	buddyAgentListening = func() (bool, error) { return true, nil }
	t.Cleanup(func() { buddyAgentListening = prevListen })
	local := &Config{}
	if _, err := RestoreCreds(local, CredBackupOpts{Path: path, Passphrase: "pw", SkipConfirm: true}); err == nil {
		t.Fatal("expected restore to refuse while Buddy Agent is running")
	}
	if ok, _ := HasCredentialSecret("sandbox-cli", secrets.KindClientSecret); ok {
		t.Fatal("secret was stored in the CLI vault")
	}
}

func TestEncryptCredBackupRoundTrip(t *testing.T) {
	prev := credBackupIter
	credBackupIter = 10_000
	t.Cleanup(func() { credBackupIter = prev })
	env, err := encryptCredBackup([]byte(`{"hello":"world"}`), "pw")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := decryptCredBackup(raw, "pw")
	if err != nil {
		t.Fatal(err)
	}
	if string(plain) != `{"hello":"world"}` {
		t.Fatalf("plain = %s", plain)
	}
}

func TestCredRestorePromptsEachAuth(t *testing.T) {
	prev := credBackupIter
	credBackupIter = 10_000
	t.Cleanup(func() { credBackupIter = prev })
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "no-managed.plist"))
	secrets.SetStore(&mockSecretStore{values: map[string]string{
		"keep-me/client-secret": "keep-secret",
		"skip-me/client-secret": "skip-secret",
	}})
	t.Cleanup(func() { secrets.SetStore(nil) })

	cfg := &Config{
		Tenant:  TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{"sandbox": {URL: "https://example.jamfcloud.com"}},
		Auths: map[string]AuthEntry{
			"keep-me": {Type: "oauth", ClientID: "keep-id", Tenants: []string{"sandbox"}},
			"skip-me": {Type: "oauth", ClientID: "skip-id", Tenants: []string{"sandbox"}},
		},
	}
	path := filepath.Join(dir, "buddy.backup")
	if _, err := BackupCreds(cfg, CredBackupOpts{Path: path, Passphrase: "pw", SkipConfirm: true}); err != nil {
		t.Fatal(err)
	}

	secrets.SetStore(&mockSecretStore{values: map[string]string{}})
	var questions []string
	defer PushConfirmAsk(func(opts ConfirmOpts) bool {
		questions = append(questions, opts.Question)
		return strings.Contains(opts.Question, "keep-me")
	})()

	empty := &Config{Tenant: TenantSelection{Default: DefaultTenant}}
	got, err := RestoreCreds(empty, CredBackupOpts{Path: path, Passphrase: "pw", SkipConfirm: false})
	if err != nil {
		t.Fatal(err)
	}
	if got.Auths != 1 || got.Skipped != 1 || got.Stored != 1 {
		t.Fatalf("restore %+v questions=%v", got, questions)
	}
	if len(questions) != 2 {
		t.Fatalf("want 2 auth prompts, got %v", questions)
	}
	if _, ok := empty.Auths["keep-me"]; !ok {
		t.Fatalf("keep-me missing: %+v", empty.Auths)
	}
	if _, ok := empty.Auths["skip-me"]; ok {
		t.Fatalf("skip-me should not be restored: %+v", empty.Auths)
	}
	ok, err := HasCredentialSecret("keep-me", secrets.KindClientSecret)
	if err != nil || !ok {
		t.Fatalf("keep-me secret stored? %v %v", ok, err)
	}
	ok, err = HasCredentialSecret("skip-me", secrets.KindClientSecret)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("skip-me secret should not be stored")
	}
}

func TestCredRestoreSkipLeavesExistingAuth(t *testing.T) {
	prev := credBackupIter
	credBackupIter = 10_000
	t.Cleanup(func() { credBackupIter = prev })
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "no-managed.plist"))
	secrets.SetStore(&mockSecretStore{values: map[string]string{
		"keep-me/client-secret": "keep-secret",
		"skip-me/client-secret": "skip-secret",
	}})
	t.Cleanup(func() { secrets.SetStore(nil) })

	src := &Config{
		Tenant:  TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{"sandbox": {URL: "https://example.jamfcloud.com"}},
		Auths: map[string]AuthEntry{
			"keep-me": {Type: "oauth", ClientID: "backup-keep", Tenants: []string{"sandbox"}},
			"skip-me": {Type: "oauth", ClientID: "backup-skip", Tenants: []string{"sandbox"}},
		},
	}
	path := filepath.Join(dir, "buddy.backup")
	if _, err := BackupCreds(src, CredBackupOpts{Path: path, Passphrase: "pw", SkipConfirm: true}); err != nil {
		t.Fatal(err)
	}

	secrets.SetStore(&mockSecretStore{values: map[string]string{}})
	defer PushConfirmAsk(func(opts ConfirmOpts) bool {
		return strings.Contains(opts.Question, "keep-me")
	})()

	dst := &Config{
		Tenant:  TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{"sandbox": {URL: "https://old.example.jamfcloud.com"}},
		Auths: map[string]AuthEntry{
			"skip-me": {Type: "oauth", ClientID: "local-skip", Tenants: []string{"sandbox"}},
			"local":   {Type: "oauth", ClientID: "local-only", Tenants: []string{"sandbox"}},
		},
	}
	got, err := RestoreCreds(dst, CredBackupOpts{Path: path, Passphrase: "pw", SkipConfirm: false})
	if err != nil {
		t.Fatal(err)
	}
	if got.Auths != 1 || got.Skipped != 1 {
		t.Fatalf("restore %+v", got)
	}
	if dst.Auths["keep-me"].ClientID != "backup-keep" {
		t.Fatalf("keep-me not restored: %+v", dst.Auths["keep-me"])
	}
	if dst.Auths["skip-me"].ClientID != "local-skip" {
		t.Fatalf("skip-me should stay local: %+v", dst.Auths["skip-me"])
	}
	if dst.Auths["local"].ClientID != "local-only" {
		t.Fatalf("local auth dropped: %+v", dst.Auths)
	}
}

func TestCredRestoreSkipAllKeepsExistingAuths(t *testing.T) {
	prev := credBackupIter
	credBackupIter = 10_000
	t.Cleanup(func() { credBackupIter = prev })
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "no-managed.plist"))
	secrets.SetStore(&mockSecretStore{values: map[string]string{
		"keep-me/client-secret": "keep-secret",
	}})
	t.Cleanup(func() { secrets.SetStore(nil) })

	src := &Config{
		Tenant:  TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{"sandbox": {URL: "https://example.jamfcloud.com"}},
		Auths: map[string]AuthEntry{
			"keep-me": {Type: "oauth", ClientID: "keep-id", Tenants: []string{"sandbox"}},
		},
	}
	path := filepath.Join(dir, "buddy.backup")
	if _, err := BackupCreds(src, CredBackupOpts{Path: path, Passphrase: "pw", SkipConfirm: true}); err != nil {
		t.Fatal(err)
	}

	dst := &Config{
		Tenant:  TenantSelection{Default: "other"},
		Tenants: map[string]TenantConfig{"other": {URL: "https://other.example.jamfcloud.com"}},
		Auths: map[string]AuthEntry{
			"local": {Type: "oauth", ClientID: "local-id", Tenants: []string{"other"}},
		},
	}
	defer PushConfirmAsk(func(ConfirmOpts) bool { return false })()
	got, err := RestoreCreds(dst, CredBackupOpts{Path: path, Passphrase: "pw", SkipConfirm: false})
	if err != nil {
		t.Fatal(err)
	}
	if got.Auths != 0 || got.Skipped != 1 || got.Tenants != 1 {
		t.Fatalf("restore %+v", got)
	}
	if dst.Tenants["sandbox"].URL != "https://example.jamfcloud.com" {
		t.Fatalf("tenants not restored: %+v", dst.Tenants)
	}
	if dst.Auths["local"].ClientID != "local-id" {
		t.Fatalf("existing auth dropped: %+v", dst.Auths)
	}
	if _, ok := dst.Auths["keep-me"]; ok {
		t.Fatalf("skipped backup auth should not be added: %+v", dst.Auths)
	}
}

func TestCredRestoreEmptyAuthBackupKeepsExistingAuths(t *testing.T) {
	prev := credBackupIter
	credBackupIter = 10_000
	t.Cleanup(func() { credBackupIter = prev })
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "no-managed.plist"))
	secrets.SetStore(&mockSecretStore{values: map[string]string{}})
	t.Cleanup(func() { secrets.SetStore(nil) })

	tenantsOnly := &Config{
		Tenant:  TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{"sandbox": {URL: "https://example.jamfcloud.com"}},
	}
	path := filepath.Join(dir, "tenants.backup")
	if _, err := BackupCreds(tenantsOnly, CredBackupOpts{Path: path, Passphrase: "pw", SkipConfirm: true}); err != nil {
		t.Fatal(err)
	}

	dst := &Config{
		Tenant:  TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{"sandbox": {URL: "https://old.example.jamfcloud.com"}},
		Auths: map[string]AuthEntry{
			"keep": {Type: "oauth", ClientID: "keep-id", Tenants: []string{"sandbox"}},
		},
	}
	got, err := RestoreCreds(dst, CredBackupOpts{Path: path, Passphrase: "pw", SkipConfirm: true})
	if err != nil {
		t.Fatal(err)
	}
	if got.Auths != 0 || got.Tenants != 1 {
		t.Fatalf("restore %+v", got)
	}
	if dst.Tenants["sandbox"].URL != "https://example.jamfcloud.com" {
		t.Fatalf("tenant not restored: %+v", dst.Tenants["sandbox"])
	}
	if dst.Auths["keep"].ClientID != "keep-id" {
		t.Fatalf("existing auth dropped: %+v", dst.Auths)
	}
}

type recordingSecretStore struct {
	mockSecretStore
	lookups []string
}

func (r *recordingSecretStore) Has(credName string, kind secrets.Kind) (bool, error) {
	r.lookups = append(r.lookups, credName+"/"+kind.String())
	return r.mockSecretStore.Has(credName, kind)
}

func (r *recordingSecretStore) Get(credName string, kind secrets.Kind) (string, error) {
	r.lookups = append(r.lookups, credName+"/"+kind.String())
	return r.mockSecretStore.Get(credName, kind)
}

func TestCredBackupSkipsCommandAuthSecrets(t *testing.T) {
	store := &recordingSecretStore{mockSecretStore: mockSecretStore{values: map[string]string{
		"cmd-auth/password":        "should-not-be-read",
		"cmd-auth/client-secret":   "should-not-be-read",
		"oauth-auth/client-secret": "oauth-secret",
	}}}
	secrets.SetStore(store)
	t.Cleanup(func() { secrets.SetStore(nil) })

	cfg := &Config{
		Tenant: TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{
			"sandbox": {URL: "https://example.jamfcloud.com"},
		},
		Auths: map[string]AuthEntry{
			"cmd-auth": {
				Type:       "command",
				RunCommand: "printf token",
				Tenants:    []string{"sandbox"},
			},
			"oauth-auth": {
				Type:     "oauth",
				ClientID: "cid",
				Tenants:  []string{"sandbox"},
			},
		},
	}
	payload, missing := collectCredBackup(cfg)
	if len(missing) != 0 {
		t.Fatalf("missing %v", missing)
	}
	cmd := payload.Auths["cmd-auth"]
	if cmd.RunCommand != "printf token" || cmd.Password != "" || cmd.ClientSecret != "" {
		t.Fatalf("command auth leaked secrets or dropped runCommand: %+v", cmd)
	}
	if payload.Auths["oauth-auth"].ClientSecret != "oauth-secret" {
		t.Fatalf("oauth secret not collected: %+v", payload.Auths["oauth-auth"])
	}
	for _, lookup := range store.lookups {
		if strings.HasPrefix(lookup, "cmd-auth/") {
			t.Fatalf("command auth triggered store lookup %q (lookups=%v)", lookup, store.lookups)
		}
	}
	if len(store.lookups) == 0 {
		t.Fatal("expected oauth auth to peek the store")
	}
}

func TestResolveBackupPassphraseIgnoresStoredMaster(t *testing.T) {
	t.Setenv(secrets.EnvVaultPassphrase, "")
	secrets.SetStore(nil)
	secrets.SetMasterStore(&secrets.MemoryMaster{Phrase: "machine-master"})
	t.Cleanup(func() { secrets.SetMasterStore(nil) })
	_, err := resolveBackupPassphrase("", false)
	if err == nil {
		t.Fatal("backup must not default to machine-stored master")
	}
}

func TestResolveBackupPassphraseEnv(t *testing.T) {
	t.Setenv(secrets.EnvVaultPassphrase, "from-env")
	t.Cleanup(func() { t.Setenv(secrets.EnvVaultPassphrase, "") })
	secrets.SetMasterStore(&secrets.MemoryMaster{Phrase: "ignored-master"})
	t.Cleanup(func() { secrets.SetMasterStore(nil) })
	pass, err := resolveBackupPassphrase("", false)
	if err != nil {
		t.Fatal(err)
	}
	if pass != "from-env" {
		t.Fatalf("pass = %q", pass)
	}
}

func TestResolveBackupPassphraseIgnoresDEK(t *testing.T) {
	t.Setenv(secrets.EnvVaultPassphrase, "")
	secrets.SetStore(nil)
	enc, err := secrets.EncodeDEK(make([]byte, secrets.EnvelopeKeyLen))
	if err != nil {
		t.Fatal(err)
	}
	secrets.SetDEKStore(&secrets.MemoryMaster{Phrase: enc})
	secrets.SetMasterStore(&secrets.MemoryMaster{Phrase: "recovery-only"})
	t.Cleanup(func() {
		secrets.SetDEKStore(nil)
		secrets.SetMasterStore(nil)
	})
	_, err = resolveBackupPassphrase("", false)
	if err == nil {
		t.Fatal("must not use DEK or stored master as backup passphrase")
	}
	pass, err := resolveBackupPassphrase("explicit", false)
	if err != nil {
		t.Fatal(err)
	}
	if pass != "explicit" {
		t.Fatalf("pass = %q", pass)
	}
}

func TestCredRestoreAgentExport(t *testing.T) {
	prev := credBackupIter
	credBackupIter = 10_000
	t.Cleanup(func() { credBackupIter = prev })
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "no-managed.plist"))
	secrets.SetStore(&mockSecretStore{values: map[string]string{}})
	t.Cleanup(func() { secrets.SetStore(nil) })

	env, err := encryptCredBackup([]byte(`{"sandbox-cli":"s3cret"}`), "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	envRaw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	doc := map[string]any{
		"context": map[string]string{"default": "sandbox"},
		"orgs": map[string]any{
			"example": map[string]any{
				"environments": map[string]any{
					"sandbox": map[string]any{
						"id":     "env-1",
						"region": "us",
						"tenants": map[string]any{
							"pro": map[string]any{
								"tenantId": "ten-1",
								"url":      "https://example.jamfcloud.com",
							},
						},
					},
				},
			},
		},
		"auths": map[string]any{
			"sandbox-cli": map[string]any{
				"type":     "oauth",
				"clientId": "cid",
				"contexts": []string{"pro"},
			},
		},
		"features": map[string]any{
			"classic_instance_only": false,
			"check_updates":         true,
		},
		"channels": map[string]any{},
		"secrets":  json.RawMessage(envRaw),
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "s3cret") {
		t.Fatal("secret in plaintext export")
	}
	path := filepath.Join(dir, "buddy-export.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	classic := true
	cfg := &Config{Features: FeatureFlags{ClassicInstanceOnly: &classic}}
	got, err := RestoreCreds(cfg, CredBackupOpts{Path: path, Passphrase: "correct horse", SkipConfirm: true})
	if err != nil {
		t.Fatal(err)
	}
	if got.Auths != 1 || got.Stored != 1 {
		t.Fatalf("restore %+v", got)
	}
	if cfg.Tenant.Default != "sandbox" {
		t.Fatalf("default context %q", cfg.Tenant.Default)
	}
	pro := cfg.Tenants["pro"]
	if pro.URL != "https://example.jamfcloud.com" || pro.TenantID != "ten-1" || pro.Org != "example" {
		t.Fatalf("product tenant %+v", pro)
	}
	envWP := cfg.Tenants["sandbox"]
	if envWP.Region != "us" || envWP.EnvironmentID != "env-1" {
		t.Fatalf("environment %+v", envWP)
	}
	auth := cfg.Auths["sandbox-cli"]
	if auth.ClientID != "cid" || auth.ClientSecret != "" || len(auth.Tenants) != 1 || auth.Tenants[0] != "pro" {
		t.Fatalf("auth %+v", auth)
	}
	if cfg.Features.ClassicInstanceOnly == nil || *cfg.Features.ClassicInstanceOnly {
		t.Fatalf("classic_instance_only %+v", cfg.Features.ClassicInstanceOnly)
	}
	if cfg.Features.CheckUpdates == nil || !*cfg.Features.CheckUpdates {
		t.Fatalf("check_updates %+v", cfg.Features.CheckUpdates)
	}
	ok, err := HasCredentialSecret("sandbox-cli", secrets.KindClientSecret)
	if err != nil || !ok {
		t.Fatalf("secret stored? %v %v", ok, err)
	}

	plainDoc := map[string]any{
		"context": map[string]string{"default": "sandbox"},
		"orgs":    doc["orgs"],
		"auths":   doc["auths"],
	}
	plainRaw, err := json.Marshal(plainDoc)
	if err != nil {
		t.Fatal(err)
	}
	plainPath := filepath.Join(dir, "identity.json")
	if err := os.WriteFile(plainPath, plainRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	again := &Config{}
	if _, err := RestoreCreds(again, CredBackupOpts{Path: plainPath, SkipConfirm: true}); err != nil {
		t.Fatal(err)
	}
	if again.Auths["sandbox-cli"].ClientID != "cid" || again.Tenants["pro"].URL == "" {
		t.Fatalf("identity-only restore auths=%+v pro=%+v", again.Auths["sandbox-cli"], again.Tenants["pro"])
	}
}
