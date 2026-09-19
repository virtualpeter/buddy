package buddy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"howett.net/plist"

	"buddy/internal/buddy/secrets"
)

func writeTestPlist(t *testing.T, path string, dict map[string]any) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	enc := plist.NewEncoder(f)
	if err := enc.Encode(dict); err != nil {
		t.Fatal(err)
	}
}

func TestConfigFromManagedPlist_KeepsClientIDAndSecrets(t *testing.T) {
	dict := map[string]any{
		"tenants": map[string]any{
			"sandbox": map[string]any{"url": "https://example.jamfcloud.com"},
		},
		"auths": map[string]any{
			"sandbox-cli": map[string]any{
				"type":         "oauth",
				"clientId":     "cid",
				"clientSecret": "from-mdm",
				"tenants":      []any{"sandbox"},
			},
		},
	}
	var buf strings.Builder
	enc := plist.NewEncoder(&buf)
	if err := enc.Encode(dict); err != nil {
		t.Fatal(err)
	}
	cfg, err := configFromManagedPlist([]byte(buf.String()))
	if err != nil {
		t.Fatal(err)
	}
	a := cfg.Auths["sandbox-cli"]
	if a.ClientID != "cid" || a.ClientSecret != "from-mdm" {
		t.Fatalf("auth = %+v", a)
	}
}

func TestOverlayConfig_UserWinsPerName(t *testing.T) {
	mdm := &Config{
		Tenant: TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{
			"sandbox": {URL: "https://mdm.example.com"},
			"prod":    {URL: "https://prod.example.com"},
		},
		Auths: map[string]AuthEntry{
			"sandbox-cli": {Type: "oauth", ClientID: "mdm-id", Tenants: []string{"sandbox"}},
		},
		Channels: map[string]ChannelPolicy{
			"mcp": {Access: AccessReadonly, Tenant: "sandbox"},
		},
	}
	user := &Config{
		Tenants: map[string]TenantConfig{
			"sandbox": {URL: "https://local.example.com"},
		},
		Auths: map[string]AuthEntry{
			"mine": {Type: "basic", Username: "admin", Tenants: []string{"sandbox"}},
		},
	}
	got := overlayConfig(mdm, user)
	if got.Tenants["sandbox"].URL != "https://local.example.com" {
		t.Fatalf("sandbox url = %q", got.Tenants["sandbox"].URL)
	}
	if got.Tenants["prod"].URL != "https://prod.example.com" {
		t.Fatal("prod tenant from MDM missing")
	}
	if _, ok := got.Auths["sandbox-cli"]; !ok {
		t.Fatal("MDM auth missing")
	}
	if got.Auths["mine"].Username != "admin" {
		t.Fatal("user auth missing")
	}
	if got.Tenant.Default != "sandbox" {
		t.Fatalf("default = %q", got.Tenant.Default)
	}
	if got.Channels["mcp"].Access != AccessReadonly {
		t.Fatal("MDM channel missing")
	}
}

func TestLoadConfig_ManagedOverlayAndSaveOmitsUnchanged(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	plistPath := filepath.Join(dir, "managed.plist")
	writeTestPlist(t, plistPath, map[string]any{
		"tenant": map[string]any{"default": "sandbox"},
		"tenants": map[string]any{
			"sandbox": map[string]any{"url": "https://example.jamfcloud.com"},
			"prod":    map[string]any{"url": "https://prod.example.com"},
		},
		"auths": map[string]any{
			"sandbox-cli": map[string]any{
				"type":     "oauth",
				"clientId": "cid",
				"tenants":  []any{"sandbox"},
				"surface":  "instance",
			},
		},
		"channels": map[string]any{
			"mcp": map[string]any{"access": "readonly", "tenant": "sandbox"},
		},
	})
	t.Setenv(EnvManagedPlist, plistPath)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.HasManagedPrefs() {
		t.Fatal("expected managed prefs")
	}
	if cfg.Tenants["sandbox"].URL != "https://example.jamfcloud.com" || cfg.Tenants["prod"].URL == "" {
		t.Fatalf("tenants = %+v", cfg.Tenants)
	}
	if cfg.Auths["sandbox-cli"].ClientID != "cid" {
		t.Fatalf("auth = %+v", cfg.Auths["sandbox-cli"])
	}
	if sum := cfg.ManagedPrefsSummary(); !strings.Contains(sum, "sandbox") || !strings.Contains(sum, "managed:") {
		t.Fatalf("summary = %q", sum)
	}

	// User override sandbox URL; save must not snapshot prod.
	cfg.Tenants["sandbox"] = TenantConfig{URL: "https://local.example.com"}
	if err := SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "buddy", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "local.example.com") {
		t.Fatalf("override not saved:\n%s", s)
	}
	if strings.Contains(s, "prod.example.com") {
		t.Fatalf("MDM-only tenant snapshotted:\n%s", s)
	}
	if strings.Contains(s, "sandbox-cli") {
		t.Fatalf("unchanged MDM auth snapshotted:\n%s", s)
	}

	loaded, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Tenants["sandbox"].URL != "https://local.example.com" {
		t.Fatalf("reload sandbox = %+v", loaded.Tenants["sandbox"])
	}
	if loaded.Tenants["prod"].URL != "https://prod.example.com" {
		t.Fatalf("reload prod = %+v", loaded.Tenants["prod"])
	}
}

func TestMarshalManagedPlist_OmitsAuths(t *testing.T) {
	cfg := &Config{
		Tenant: TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{
			"sandbox": {URL: "https://example.jamfcloud.com"},
		},
		Channels: map[string]ChannelPolicy{
			"mcp": {Access: AccessReadonly, Tenant: "sandbox"},
		},
		Auths: map[string]AuthEntry{
			"sandbox-cli": {
				Type:         "oauth",
				ClientID:     "cid",
				ClientSecret: "secret",
				Tenants:      []string{"sandbox"},
			},
			"sandbox-peter": {
				Type:     "basic",
				Username: "jdoe",
				Password: "hunter2",
				Tenants:  []string{"sandbox"},
			},
		},
	}
	data, err := MarshalManagedPlist(cfg)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, bad := range []string{"auths", "sandbox-cli", "sandbox-peter", "secret", "cid", "jdoe", "hunter2"} {
		if strings.Contains(s, bad) {
			t.Fatalf("%q in plist:\n%s", bad, s)
		}
	}
	if !strings.Contains(s, "sandbox") || !strings.Contains(s, "example.jamfcloud.com") {
		t.Fatalf("missing tenant keys:\n%s", s)
	}
	if !strings.Contains(s, "mcp") {
		t.Fatalf("missing channel:\n%s", s)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Fatal("expected trailing newline")
	}
}

func TestSetupSecrets_CreateAuthAndStoreSecret(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "no-managed.plist"))
	secrets.SetStore(&mockSecretStore{values: map[string]string{}})
	t.Cleanup(func() { secrets.SetStore(nil) })

	cfg := &Config{
		Tenant: TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{
			"sandbox": {URL: "https://example.jamfcloud.com"},
		},
	}
	res, err := SetupSecrets(cfg, SetupSecretsOpts{
		Type:         "oauth",
		AuthName:     "sandbox-oauth",
		ClientID:     "cid",
		ClientSecret: "s3cret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.CreatedAuths) != 1 || res.CreatedAuths[0] != "sandbox-oauth" {
		t.Fatalf("created = %v", res.CreatedAuths)
	}
	a := cfg.Auths["sandbox-oauth"]
	if a.Type != "oauth" || a.ClientID != "cid" || a.ClientSecret != "" {
		t.Fatalf("auth = %+v", a)
	}
	ok, err := HasCredentialSecret("sandbox-oauth", secrets.KindClientSecret)
	if err != nil || !ok {
		t.Fatalf("stored? %v %v", ok, err)
	}
	loaded, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Auths["sandbox-oauth"].ClientID != "cid" {
		t.Fatal("auth not persisted")
	}
}

func TestSetupSecrets_FillsMissingSecret(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "no-managed.plist"))
	secrets.SetStore(&mockSecretStore{values: map[string]string{}})
	t.Cleanup(func() { secrets.SetStore(nil) })

	cfg := &Config{
		Tenant: TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{
			"sandbox": {URL: "https://example.jamfcloud.com"},
		},
		Auths: map[string]AuthEntry{
			"sandbox-cli": {Type: "oauth", ClientID: "cid", Tenants: []string{"sandbox"}, Surface: SurfaceInstance},
		},
	}
	res, err := SetupSecrets(cfg, SetupSecretsOpts{ClientSecret: "from-setup"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.CreatedAuths) != 0 {
		t.Fatalf("created = %v", res.CreatedAuths)
	}
	if len(res.StoredSecrets) != 1 {
		t.Fatalf("stored = %v", res.StoredSecrets)
	}
	ok, err := HasCredentialSecret("sandbox-cli", secrets.KindClientSecret)
	if err != nil || !ok {
		t.Fatalf("stored? %v %v", ok, err)
	}
}

func TestSetupSecrets_NonInteractiveReportsGaps(t *testing.T) {
	cfg := &Config{
		Tenant: TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{
			"sandbox": {URL: "https://example.jamfcloud.com"},
		},
	}
	_, err := SetupSecrets(cfg, SetupSecretsOpts{})
	if err == nil {
		t.Fatal("expected gap error")
	}
	if !strings.Contains(err.Error(), "has no auth") {
		t.Fatalf("err = %v", err)
	}
}

func TestSetupSecrets_SkipsWhenSecretPresent(t *testing.T) {
	secrets.SetStore(&mockSecretStore{values: map[string]string{
		"sandbox-cli/client-secret": "already",
	}})
	t.Cleanup(func() { secrets.SetStore(nil) })
	cfg := &Config{
		Tenant: TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{
			"sandbox": {URL: "https://example.jamfcloud.com"},
		},
		Auths: map[string]AuthEntry{
			"sandbox-cli": {Type: "oauth", ClientID: "cid", Tenants: []string{"sandbox"}},
		},
	}
	res, err := SetupSecrets(cfg, SetupSecretsOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.CreatedAuths) != 0 || len(res.StoredSecrets) != 0 || len(res.Missing) != 0 {
		t.Fatalf("got %+v", res)
	}
}

func TestManagedForceAgent(t *testing.T) {
	cfg := &Config{}
	if cfg.ManagedForceAgent() {
		t.Fatal("empty config")
	}
	cfg.mdm = &Config{Features: FeatureFlags{ForceAgent: boolPtr(true)}}
	if !cfg.ManagedForceAgent() {
		t.Fatal("mdm true")
	}
	cfg.mdm.Features.ForceAgent = boolPtr(false)
	if cfg.ManagedForceAgent() {
		t.Fatal("mdm false")
	}
}
