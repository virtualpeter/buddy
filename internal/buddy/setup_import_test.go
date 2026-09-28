package buddy

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"buddy/internal/buddy/agent"
	"buddy/internal/buddy/secrets"

	"gopkg.in/yaml.v3"
)

func TestImportQuickstartNestedOrgs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "no-managed.plist"))
	secrets.SetStore(&mockSecretStore{values: map[string]string{}})
	t.Cleanup(func() { secrets.SetStore(nil) })

	src := filepath.Join(dir, "quickstart.yaml")
	body := []byte(`
context:
  default: sandbox
orgs:
  acme:
    environments:
      commercial:
        id: 11111111-1111-1111-1111-111111111111
        region: us
        tenants:
          sandbox:
            tenantId: 22222222-2222-2222-2222-222222222222
            url: https://example.jamfcloud.com
auths:
  should-ignore:
    type: oauth
    clientId: leak
    clientSecret: nope
`)
	if err := os.WriteFile(src, body, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{}
	if err := ImportQuickstart(cfg, src); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Auths) != 0 {
		t.Fatalf("auths must not import: %+v", cfg.Auths)
	}
	if !cfg.Tenants["acme"].IsOrganizationWorkplace() {
		t.Fatalf("org=%+v", cfg.Tenants["acme"])
	}
	env := cfg.Tenants["commercial"]
	if env.Org != "acme" || env.EnvironmentID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("env=%+v", env)
	}
	ten := cfg.Tenants["sandbox"]
	if ten.URL != "https://example.jamfcloud.com" || ten.TenantID != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("tenant=%+v", ten)
	}
	loaded, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !workplaceIdentityEqual(ten, loaded.Tenants["sandbox"]) {
		t.Fatalf("reload sandbox %+v vs %+v", ten, loaded.Tenants["sandbox"])
	}
	raw, err := os.ReadFile(filepath.Join(dir, "buddy", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "nope") || strings.Contains(string(raw), "should-ignore") {
		t.Fatalf("secrets leaked into config:\n%s", raw)
	}
}

func TestCredAddBasicInstanceNoOrgs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "no-managed.plist"))
	secrets.SetStore(&mockSecretStore{values: map[string]string{}})
	t.Cleanup(func() { secrets.SetStore(nil) })

	cfg := &Config{}
	res, err := CredAdd(cfg, CredAddOpts{
		TenantName:  "sandbox",
		URL:         "https://example.jamfcloud.com",
		Type:        AuthTypeBasic,
		Username:    "admin",
		Password:    "s3cret",
		Kind:        "instance",
		SkipConfirm: true,
		NoTest:      true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Tenants["sandbox"].URL != "https://example.jamfcloud.com" {
		t.Fatalf("%+v tenants=%+v", res, cfg.Tenants)
	}
	org, ok := cfg.Orgs["default"]
	if !ok {
		t.Fatalf("instance basic should write orgs.default: %+v", cfg.Orgs)
	}
	if org.Environments["sandbox"].EffectiveURL() != "https://example.jamfcloud.com" {
		t.Fatalf("nested instance=%+v", org.Environments["sandbox"])
	}
	okHas, err := HasCredentialSecret(res.AuthName, secrets.KindPassword)
	if err != nil || !okHas {
		t.Fatalf("password stored? %v %v", okHas, err)
	}
}

func TestCredAddFromFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "no-managed.plist"))
	secrets.SetStore(&mockSecretStore{values: map[string]string{}})
	t.Cleanup(func() { secrets.SetStore(nil) })

	src := filepath.Join(dir, "qs.yaml")
	if err := os.WriteFile(src, []byte(`
contexts:
  sandbox:
    jamfpro:
      url: https://example.jamfcloud.com
`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{}
	_, err := CredAdd(cfg, CredAddOpts{FromFile: src, SkipConfirm: true, NoTest: true}, nil)
	if err == nil {
		t.Fatal("expected missing-auth error when non-interactive import has no auths")
	}
	if cfg.Tenants["sandbox"].URL != "https://example.jamfcloud.com" {
		t.Fatalf("identity should still import: %+v", cfg.Tenants)
	}
}

func TestCredAddEnvWithOrgWritesNested(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "no-managed.plist"))
	secrets.SetStore(&mockSecretStore{values: map[string]string{}})
	t.Cleanup(func() { secrets.SetStore(nil) })

	cfg := &Config{}
	_, err := CredAdd(cfg, CredAddOpts{
		TenantName:    "commercial",
		Region:        "us",
		EnvironmentID: "11111111-1111-1111-1111-111111111111",
		Type:          AuthTypePlatform,
		ClientID:      "cid",
		ClientSecret:  "sec",
		OrgName:       "acme",
		SkipConfirm:   true,
		NoTest:        true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	env := cfg.Orgs["acme"].Environments["commercial"]
	if env.EffectiveID() != "11111111-1111-1111-1111-111111111111" || env.Region != "us" {
		t.Fatalf("nested env=%+v", env)
	}
	if !cfg.Tenants["acme"].IsOrganizationWorkplace() {
		t.Fatalf("org workplace=%+v", cfg.Tenants["acme"])
	}
}

func TestImportQuickstartPlist(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "no-managed.plist"))
	secrets.SetStore(&mockSecretStore{values: map[string]string{}})
	t.Cleanup(func() { secrets.SetStore(nil) })

	srcCfg := &Config{
		Tenant: TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{
			"sandbox": {URL: "https://example.jamfcloud.com"},
		},
		Auths: map[string]AuthEntry{
			"leak": {Type: AuthTypeOAuth, ClientID: "cid", ClientSecret: "nope", Tenants: []string{"sandbox"}},
		},
	}
	data, err := MarshalManagedPlist(srcCfg)
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "quickstart.plist")
	if err := os.WriteFile(src, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{}
	if err := ImportQuickstart(cfg, src); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Auths) != 0 {
		t.Fatalf("plist auths must not import: %+v", cfg.Auths)
	}
	if cfg.Tenants["sandbox"].URL != "https://example.jamfcloud.com" {
		t.Fatalf("sandbox=%+v", cfg.Tenants["sandbox"])
	}
}

func TestMarshalQuickstartYAML_OmitsAuthsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "no-managed.plist"))
	secrets.SetStore(&mockSecretStore{values: map[string]string{}})
	t.Cleanup(func() { secrets.SetStore(nil) })

	var src Config
	if err := yaml.Unmarshal([]byte(nestedOrgsYAML), &src); err != nil {
		t.Fatal(err)
	}
	data, err := MarshalQuickstartYAML(&src)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, bad := range []string{"auths", "acme-org", "env-plat", "sandgod", "oid", "eid", "cid"} {
		if strings.Contains(s, bad) {
			t.Fatalf("%q in quickstart yaml:\n%s", bad, s)
		}
	}
	if !strings.Contains(s, "orgs:") || !strings.Contains(s, "example.jamfcloud.com") {
		t.Fatalf("missing identity:\n%s", s)
	}

	path := filepath.Join(dir, "quickstart.yaml")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{}
	if err := ImportQuickstart(cfg, path); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Auths) != 0 {
		t.Fatalf("auths leaked: %+v", cfg.Auths)
	}
	if !workplaceIdentityEqual(src.Tenants["acme"], cfg.Tenants["acme"]) ||
		!workplaceIdentityEqual(src.Tenants["commercial"], cfg.Tenants["commercial"]) ||
		!workplaceIdentityEqual(src.Tenants["sandbox"], cfg.Tenants["sandbox"]) {
		t.Fatalf("round-trip workplaces src=%+v got=%+v", src.Tenants, cfg.Tenants)
	}
}

func TestImportQuickstart_MDMOverlayThenSecrets(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	plistPath := filepath.Join(dir, "managed.plist")
	writeTestPlist(t, plistPath, map[string]any{
		"tenant": map[string]any{"default": "sandbox"},
		"tenants": map[string]any{
			"sandbox": map[string]any{"url": "https://example.jamfcloud.com"},
			"prod":    map[string]any{"url": "https://prod.example.com"},
		},
	})
	t.Setenv(EnvManagedPlist, plistPath)
	secrets.SetStore(&mockSecretStore{values: map[string]string{}})
	t.Cleanup(func() { secrets.SetStore(nil) })

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.HasManagedPrefs() {
		t.Fatal("expected managed prefs")
	}
	src := filepath.Join(dir, "extra.yaml")
	if err := os.WriteFile(src, []byte(`
contexts:
  lab:
    jamfpro:
      url: https://lab.example.com
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ImportQuickstart(cfg, src); err != nil {
		t.Fatal(err)
	}
	if cfg.Tenants["lab"].URL != "https://lab.example.com" {
		t.Fatalf("imported lab=%+v", cfg.Tenants["lab"])
	}
	if cfg.Tenants["sandbox"].URL != "https://example.jamfcloud.com" || cfg.Tenants["prod"].URL != "https://prod.example.com" {
		t.Fatalf("MDM tenants dropped: %+v", cfg.Tenants)
	}

	res, err := SetupSecrets(cfg, SetupSecretsOpts{
		Type:         AuthTypeOAuth,
		ClientID:     "cid",
		ClientSecret: "s3cret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.CreatedAuths) == 0 {
		t.Fatalf("expected auths created: %+v", res)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "buddy", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "lab.example.com") {
		t.Fatalf("imported lab not saved:\n%s", s)
	}
	if strings.Contains(s, "prod.example.com") {
		t.Fatalf("unmodified MDM tenant snapshotted:\n%s", s)
	}
	loaded, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Tenants["prod"].URL != "https://prod.example.com" {
		t.Fatalf("reload prod=%+v", loaded.Tenants["prod"])
	}
	if loaded.Tenants["lab"].URL != "https://lab.example.com" {
		t.Fatalf("reload lab=%+v", loaded.Tenants["lab"])
	}
}

func TestRunFirstTimeSetupNonInteractiveRequiresContext(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "no-managed.plist"))
	secrets.SetStore(&mockSecretStore{values: map[string]string{}})
	t.Cleanup(func() { secrets.SetStore(nil) })

	_, err := RunFirstTimeSetup(&Config{}, Opts{SkipConfirm: true}, nil)
	if err == nil {
		t.Fatal("expected -y empty setup to require cred add flags")
	}
}

func TestRunFirstTimeSetupManagedPrefsFillsSecrets(t *testing.T) {
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
		mdm: &Config{
			Tenants: map[string]TenantConfig{
				"sandbox": {URL: "https://example.jamfcloud.com"},
			},
		},
	}
	_, err := RunFirstTimeSetup(cfg, Opts{SkipConfirm: true}, nil)
	if err == nil {
		t.Fatal("expected non-interactive secrets gap")
	}
}

func TestFirstTimeSetupAgentHint(t *testing.T) {
	t.Setenv(agent.EnvDisable, "")
	t.Setenv(agent.EnvSocket, "")
	if runtime.GOOS == "darwin" {
		if firstTimeSetupAgentHint() == "" {
			t.Fatal("want a Buddy Agent hint on macOS")
		}
		t.Setenv(agent.EnvDisable, "1")
		if firstTimeSetupAgentHint() != "" {
			t.Fatal("BUDDY_NO_AGENT should hide the hint")
		}
		return
	}
	if firstTimeSetupAgentHint() != "" {
		t.Fatal("other platforms have no Agent hint")
	}
	t.Setenv(agent.EnvSocket, filepath.Join(t.TempDir(), "agent.sock"))
	if firstTimeSetupAgentHint() != "" {
		t.Fatal("EnvSocket on a non-Mac must not print the macOS setup note")
	}
}

func TestFirstTimeSetupOptions(t *testing.T) {
	opts := firstTimeSetupOptions()
	if len(opts) < 4 || opts[3].key != "4" || opts[3].label != "Restore a cred backup" {
		t.Fatalf("options = %+v", opts)
	}
	hasMDM := false
	for _, opt := range opts {
		if strings.Contains(opt.label, "MDM") {
			hasMDM = true
		}
	}
	if hasMDM != (runtime.GOOS == "darwin") {
		t.Fatalf("MDM choice present=%v on %s", hasMDM, runtime.GOOS)
	}
}

func TestCredAddBasicReplacesPlaceholderDefaultContext(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "no-managed.plist"))
	secrets.SetStore(&mockSecretStore{values: map[string]string{}})
	t.Cleanup(func() { secrets.SetStore(nil) })

	cfg := &Config{Tenant: TenantSelection{Default: DefaultTenant}}
	_, err := CredAdd(cfg, CredAddOpts{
		TenantName:  "sandbox",
		URL:         "https://example.jamfcloud.com",
		Type:        AuthTypeBasic,
		Username:    "admin",
		Password:    "s3cret",
		Kind:        "instance",
		SkipConfirm: true,
		NoTest:      true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Tenant.Default != "sandbox" {
		t.Fatalf("default context = %q, want sandbox", cfg.Tenant.Default)
	}
}

func TestCredAddBasicKeepsExistingDefaultContext(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "no-managed.plist"))
	secrets.SetStore(&mockSecretStore{values: map[string]string{}})
	t.Cleanup(func() { secrets.SetStore(nil) })

	cfg := &Config{
		Tenant:  TenantSelection{Default: "prod"},
		Tenants: map[string]TenantConfig{"prod": {URL: "https://prod.example.com"}},
	}
	_, err := CredAdd(cfg, CredAddOpts{
		TenantName:  "sandbox",
		URL:         "https://example.jamfcloud.com",
		Type:        AuthTypeBasic,
		Username:    "admin",
		Password:    "s3cret",
		Kind:        "instance",
		SkipConfirm: true,
		NoTest:      true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Tenant.Default != "prod" {
		t.Fatalf("default context = %q, want prod", cfg.Tenant.Default)
	}
}
