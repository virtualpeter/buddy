package buddy

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestResolveCredential_NilAuths(t *testing.T) {
	cfg := &Config{Auths: nil, Tenants: nil}
	_, err := cfg.ResolveCredential("default")
	if err == nil {
		t.Fatal("expected error when Auths is nil")
	}
}

func TestResolveCredential_Missing(t *testing.T) {
	cfg := ConfigFromCredentialMap("default", map[string]CredentialConfig{
		"default": {URL: "https://jamf.example.com", Auth: AuthConfig{Type: "basic", Username: "u"}},
	})
	_, err := cfg.ResolveCredential("missing")
	if err == nil {
		t.Fatal("expected error when auth name not found")
	}
	if !strings.Contains(err.Error(), "auth \"missing\" not found") {
		t.Errorf("error = %v", err)
	}
}

func TestResolveCredential_Found(t *testing.T) {
	cfg := ConfigFromCredentialMap("default", map[string]CredentialConfig{
		"default": {
			URL: "https://jamf.example.com",
			Auth: AuthConfig{
				Type:     "basic",
				Username: "user",
				Password: "secret",
			},
		},
	})
	got, err := cfg.ResolveCredential("default")
	if err != nil {
		t.Fatalf("ResolveCredential: %v", err)
	}
	if got.URL != "https://jamf.example.com" || got.Auth.Username != "user" || got.Auth.Password != "secret" {
		t.Errorf("got %+v", got)
	}
}

func TestMarshalConfig_OmitsEmptyAuthFields(t *testing.T) {
	cfg := &Config{
		Tenant: TenantSelection{Default: "oauth1"},
		Tenants: map[string]TenantConfig{
			"oauth1": {URL: "https://example.jamfcloud.com"},
			"basic1": {URL: "https://example.jamfcloud.com"},
		},
		Auths: map[string]AuthEntry{
			"oauth1": {
				Type:     "oauth",
				ClientID: "cid",
				Tenants:  []string{"oauth1"},
			},
			"basic1": {
				Type:     "basic",
				Username: "admin",
				Tenants:  []string{"basic1"},
			},
		},
	}
	data, err := marshalConfig(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, bad := range []string{"runCommand", "password:", "clientSecret:", `username: ""`} {
		if strings.Contains(s, bad) {
			t.Fatalf("marshal should omit %q, got:\n%s", bad, s)
		}
	}
	if !strings.Contains(s, "clientId: cid") {
		t.Fatalf("missing clientId:\n%s", s)
	}
	if !strings.Contains(s, "username: admin") {
		t.Fatalf("missing username:\n%s", s)
	}
	if strings.Contains(s, "credentials:") {
		t.Fatalf("must not write credentials:\n%s", s)
	}
	if !strings.Contains(s, "contexts:") || !strings.Contains(s, "auths:") {
		t.Fatalf("expected contexts/auths:\n%s", s)
	}
	if !strings.Contains(s, "jamfpro:") {
		t.Fatalf("instance auths should write jamfpro:\n%s", s)
	}
	if strings.Contains(s, "\ntenants:") {
		t.Fatalf("must write contexts: not tenants:\n%s", s)
	}

	var got Config
	if err := yaml.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Auths["oauth1"].ClientID != "cid" || got.Auths["oauth1"].Type != "oauth" {
		t.Fatalf("round-trip oauth1 = %+v", got.Auths["oauth1"])
	}
	if got.Auths["basic1"].Username != "admin" {
		t.Fatalf("round-trip basic1 = %+v", got.Auths["basic1"])
	}
	if strings.Contains(s, "surface:") {
		t.Fatalf("default instance surface should be omitted:\n%s", s)
	}
}

func TestMarshalConfig_PreservesOrder(t *testing.T) {
	existing := []byte(`tenant: zebra
tenants:
    zebra:
        url: https://z.example.com
    alpha:
        url: https://a.example.com
auths:
    zebra:
        type: oauth
        clientId: z
        tenants: [zebra]
    alpha:
        type: oauth
        clientId: a
        tenants: [alpha]
`)
	cfg := &Config{
		Tenant: TenantSelection{Default: "zebra"},
		Tenants: map[string]TenantConfig{
			"alpha": {URL: "https://a.example.com"},
			"zebra": {URL: "https://z.example.com"},
			"beta":  {URL: "https://b.example.com"},
		},
		Auths: map[string]AuthEntry{
			"alpha": {Type: "oauth", ClientID: "a", Tenants: []string{"alpha"}},
			"zebra": {Type: "oauth", ClientID: "z", Tenants: []string{"zebra"}},
			"beta":  {Type: "oauth", ClientID: "b", Tenants: []string{"beta"}},
		},
	}
	data, err := marshalConfig(cfg, existing)
	if err != nil {
		t.Fatal(err)
	}
	order := yamlMapKeyOrder(data, "contexts")
	want := []string{"zebra", "alpha", "beta"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v\n%s", order, want, data)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v\n%s", order, want, data)
		}
	}
}

func TestMigrateCredentialsToTenantsAuths(t *testing.T) {
	raw := &legacyConfigFile{
		Cred: "gw",
		Credentials: map[string]CredentialConfig{
			"gw": {
				URL: "https://us.api.jamfcloud.com",
				Auth: AuthConfig{
					Type:     "oauth",
					ClientID: "cid",
					TenantID: "e77c1408-10c8-4007-b177-abc9157fbcaa",
				},
			},
			"inst": {
				URL:  "https://example.jamfcloud.com",
				Auth: AuthConfig{Type: "basic", Username: "admin"},
			},
		},
	}
	cfg, err := migrateCredentialsToTenantsAuths(raw)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Tenant.Default != "gw" {
		t.Fatalf("tenant = %q", cfg.Tenant.Default)
	}
	gw := cfg.Tenants["gw"]
	if gw.TenantID != "e77c1408-10c8-4007-b177-abc9157fbcaa" {
		t.Fatalf("tenantId = %q", gw.TenantID)
	}
	if gw.Region != "us" {
		t.Fatalf("region = %q", gw.Region)
	}
	a := cfg.Auths["gw"]
	if a.ClientID != "cid" || a.Type != AuthTypePlatform || len(a.Tenants) != 1 || a.Tenants[0] != "gw" {
		t.Fatalf("auth gw = %+v", a)
	}
	if cfg.Auths["inst"].EffectiveSurface() != SurfaceInstance {
		t.Fatalf("inst surface = %q", cfg.Auths["inst"].EffectiveSurface())
	}
}

func TestMultiTenantAuth(t *testing.T) {
	cfg := &Config{
		Tenant: TenantSelection{Default: "t1"},
		Tenants: map[string]TenantConfig{
			"t1": {Region: "us", TenantID: "11111111-1111-1111-1111-111111111111"},
			"t2": {Region: "us", TenantID: "22222222-2222-2222-2222-222222222222"},
		},
		Auths: map[string]AuthEntry{
			"shared": {
				Type:     AuthTypePlatform,
				ClientID: "cid",
				Tenants:  []string{"t1", "t2"},
			},
		},
	}
	SetActiveConfig(cfg)
	t.Cleanup(func() { SetActiveConfig(nil); SetActiveSelection("", "") })
	SetActiveSelection("t2", "")
	sess, err := cfg.ResolveSession("", "shared", OpPlatform)
	if err != nil {
		t.Fatal(err)
	}
	if sess.TenantName != "t2" || sess.Credential.EffectiveTenantID() != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("%+v", sess)
	}
}

func TestPickAuthReadonlyPrefer(t *testing.T) {
	cfg := &Config{
		Tenant: TenantSelection{Default: "t"},
		Tenants: map[string]TenantConfig{
			"t": {URL: "https://example.jamfcloud.com"},
		},
		Auths: map[string]AuthEntry{
			"ro": {Type: "basic", Username: "ro", Tenants: []string{"t"}, Access: AccessReadonly, Surface: SurfaceInstance},
			"rw": {Type: "basic", Username: "rw", Tenants: []string{"t"}, Access: AccessFull, Surface: SurfaceInstance},
		},
	}
	sess, err := cfg.ResolveSession("t", "", OpRead)
	if err != nil {
		t.Fatal(err)
	}
	if sess.AuthName != "ro" {
		t.Fatalf("want ro, got %s", sess.AuthName)
	}
	sess, err = cfg.ResolveSession("t", "ro", OpMutate)
	if err != nil {
		t.Fatal(err)
	}
	if sess.AuthName != "rw" {
		t.Fatalf("readonly pin should fall back to full auth, got %s", sess.AuthName)
	}
	sess, err = cfg.ResolveSession("t", "", OpMutate)
	if err != nil {
		t.Fatal(err)
	}
	if sess.AuthName != "rw" {
		t.Fatalf("want rw, got %s", sess.AuthName)
	}
}

func TestPickAuthClassicWritePrefersInstance(t *testing.T) {
	t.Setenv(EnvClassicInstanceOnly, "")
	cfg := &Config{
		Tenant: TenantSelection{Default: "t"},
		Tenants: map[string]TenantConfig{
			"t": {
				URL:      "https://example.jamfcloud.com",
				Region:   "us",
				TenantID: "00000000-0000-4000-8000-000000000001",
			},
		},
		Auths: map[string]AuthEntry{
			"plat": {
				Type: AuthTypePlatform, ClientID: "c", ClientSecret: "s",
				Tenants: []string{"t"}, Access: AccessFull,
			},
			"inst": {
				Type: "basic", Username: "a", Password: "b",
				Tenants: []string{"t"}, Access: AccessFull, Surface: SurfaceInstance,
			},
		},
	}
	sess, err := cfg.ResolveSession("t", "", OpClassicWrite)
	if err != nil {
		t.Fatal(err)
	}
	if sess.AuthName != "inst" || sess.Surface != SurfaceInstance {
		t.Fatalf("want inst/instance, got %s/%s", sess.AuthName, sess.Surface)
	}
	sess, err = cfg.ResolveSession("t", "plat", OpClassicWrite)
	if err != nil {
		t.Fatal(err)
	}
	if sess.AuthName != "plat" || sess.Surface != SurfacePlatform {
		t.Fatalf("default: platform pin should use platform for Classic, got %s/%s", sess.AuthName, sess.Surface)
	}
}

func TestClassicInstanceOnly_DefaultAndMarshal(t *testing.T) {
	t.Setenv(EnvClassicInstanceOnly, "")
	var nilCfg *Config
	if nilCfg.ClassicInstanceOnly() {
		t.Fatal("nil config should default false")
	}
	cfg := &Config{}
	if cfg.ClassicInstanceOnly() {
		t.Fatal("omitted YAML should default false")
	}
	data, err := marshalConfig(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "classic_instance_only") {
		t.Fatalf("default should omit features:\n%s", data)
	}
	cfg.Features.ClassicInstanceOnly = boolPtr(false)
	data, err = marshalConfig(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "classic_instance_only") {
		t.Fatalf("explicit false is the default and should omit features:\n%s", data)
	}
	cfg.Features.ClassicInstanceOnly = boolPtr(true)
	if !cfg.ClassicInstanceOnly() {
		t.Fatal("YAML true")
	}
	data, err = marshalConfig(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "classic_instance_only: true") {
		t.Fatalf("want persisted true, got:\n%s", data)
	}
	t.Setenv(EnvClassicInstanceOnly, "false")
	if cfg.ClassicInstanceOnly() {
		t.Fatal("env false should override YAML true")
	}
}

func TestEffectiveSurface_FromType(t *testing.T) {
	if got := (AuthEntry{}).EffectiveSurface(); got != SurfaceInstance {
		t.Fatalf("omitted=%q", got)
	}
	if got := (AuthEntry{Type: AuthTypeOAuth}).EffectiveSurface(); got != SurfaceInstance {
		t.Fatalf("oauth=%q", got)
	}
	if got := (AuthEntry{Type: AuthTypePlatform}).EffectiveSurface(); got != SurfacePlatform {
		t.Fatalf("platform type=%q", got)
	}
	if got := (AuthEntry{Type: AuthTypeOAuth, Surface: SurfacePlatform}).EffectiveSurface(); got != SurfacePlatform {
		t.Fatalf("legacy oauth+surface=%q", got)
	}
	if err := (AuthEntry{Type: AuthTypePlatform, ClientID: "c", Tenants: []string{"t"}}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (AuthEntry{Type: AuthTypeOAuth, ClientID: "c", Tenants: []string{"t"}}).Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateAuthSurfaceIntoType(t *testing.T) {
	cfg := &Config{
		Auths: map[string]AuthEntry{
			"dual": {Type: "oauth", ClientID: "c", Tenants: []string{"t"}, Surface: "any"},
			"ok":   {Type: "oauth", ClientID: "c", Tenants: []string{"t"}, Surface: SurfacePlatform},
		},
	}
	if !migrateAuthSurfaceIntoType(cfg) {
		t.Fatal("expected rewrite")
	}
	if cfg.Auths["dual"].Surface != "" || cfg.Auths["dual"].Type != AuthTypeOAuth {
		t.Fatalf("any should drop surface and stay oauth, got %+v", cfg.Auths["dual"])
	}
	if cfg.Auths["ok"].Type != AuthTypePlatform || cfg.Auths["ok"].Surface != "" {
		t.Fatalf("oauth+platform surface → type platform, got %+v", cfg.Auths["ok"])
	}
	if migrateAuthSurfaceIntoType(cfg) {
		t.Fatal("second pass should be a no-op")
	}
}

func TestUnmarshalCanonicalContexts(t *testing.T) {
	data := []byte(`
context:
  default: sandbox
contexts:
  sandbox:
    id: 11111111-1111-1111-1111-111111111111
    region: us
    jamfpro:
      url: https://example.jamfcloud.com
      tenantId: 22222222-2222-2222-2222-222222222222
auths:
  sandgod:
    type: oauth
    jamfpro: sandbox
    clientId: cid
  sandbox-platform:
    type: platform
    scope: environment
    contexts: [sandbox]
    clientId: pid
`)
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Tenant.Default != "sandbox" {
		t.Fatalf("default=%q", cfg.Tenant.Default)
	}
	tc := cfg.Tenants["sandbox"]
	if tc.URL != "https://example.jamfcloud.com" || tc.EnvironmentID != "11111111-1111-1111-1111-111111111111" || tc.TenantID != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("context=%+v", tc)
	}
	if err := tc.Validate(); err != nil {
		t.Fatal(err)
	}
	inst := cfg.Auths["sandgod"]
	if !inst.LinkedTo("sandbox") || inst.JamfPro != "sandbox" {
		t.Fatalf("sandgod=%+v", inst)
	}
	plat := cfg.Auths["sandbox-platform"]
	if plat.EffectiveScope() != PlatformScopeEnvironment || !plat.LinkedTo("sandbox") {
		t.Fatalf("platform=%+v", plat)
	}
	cred := ComposeCredential(tc, plat, SurfacePlatform)
	if cred.EnvironmentID != tc.EnvironmentID || cred.TenantID != "" {
		t.Fatalf("env-scoped compose=%+v", cred)
	}
	tenantScoped := plat
	tenantScoped.Scope = PlatformScopeTenant
	cred = ComposeCredential(tc, tenantScoped, SurfacePlatform)
	if cred.TenantID != tc.TenantID || cred.EnvironmentID != "" {
		t.Fatalf("tenant-scoped compose=%+v", cred)
	}
}

func TestUnmarshalOrganizationWorkplace(t *testing.T) {
	data := []byte(`
context:
  default: acme
contexts:
  acme: {}
  sandbox:
    org: acme
    id: 11111111-1111-1111-1111-111111111111
    region: us
    jamfpro:
      url: https://example.jamfcloud.com
auths:
  acme-org:
    type: platform
    scope: organization
    contexts: [acme]
    clientId: oid
  sandbox-platform:
    type: platform
    scope: environment
    contexts: [sandbox]
    clientId: pid
`)
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	org := cfg.Tenants["acme"]
	if !org.IsOrganizationWorkplace() || org.Region != "" || org.URL != "" {
		t.Fatalf("org workplace=%+v", org)
	}
	if err := org.Validate(); err != nil {
		t.Fatal(err)
	}
	child := cfg.Tenants["sandbox"]
	if child.Org != "acme" {
		t.Fatalf("child org parent=%q", child.Org)
	}
	if err := cfg.ValidateTenantSelectionAndChannels(); err != nil {
		t.Fatal(err)
	}
	names := cfg.AuthsForTenant("sandbox")
	got := map[string]bool{}
	for _, n := range names {
		got[n] = true
	}
	if !got["sandbox-platform"] {
		t.Fatalf("sandbox should keep env auth: %v", names)
	}
	if got["acme-org"] {
		t.Fatalf("org-scoped auth must not apply to child workplaces: %v", names)
	}
	if orgNames := cfg.AuthsForTenant("acme"); len(orgNames) != 1 || orgNames[0] != "acme-org" {
		t.Fatalf("acme auths=%v", orgNames)
	}
	cred := ComposeCredential(org, cfg.Auths["acme-org"], SurfacePlatform)
	if cred.URL != OrgPlatformGatewayURL || cred.Region != OrgPlatformRegion {
		t.Fatalf("org compose=%+v", cred)
	}
}

func TestValidateContextOrgMustBeWorkplace(t *testing.T) {
	cfg := &Config{
		Tenant: TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{
			"sandbox": {URL: "https://example.jamfcloud.com"},
			"prod":    {URL: "https://prod.example.jamfcloud.com", Org: "sandbox"},
		},
	}
	err := cfg.ValidateTenantSelectionAndChannels()
	if err == nil || !strings.Contains(err.Error(), "organization workplace") {
		t.Fatalf("expected org workplace error, got %v", err)
	}
}

func TestUnmarshalLegacyTenants(t *testing.T) {
	data := []byte(`
tenant:
  default: sandbox
tenants:
  sandbox:
    url: https://example.jamfcloud.com
auths:
  sandgod:
    type: oauth
    tenants: [sandbox]
    clientId: cid
`)
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Tenant.Default != "sandbox" || cfg.Tenants["sandbox"].URL != "https://example.jamfcloud.com" {
		t.Fatalf("%+v %+v", cfg.Tenant, cfg.Tenants["sandbox"])
	}
	if !cfg.Auths["sandgod"].LinkedTo("sandbox") {
		t.Fatal("legacy tenants: link")
	}
	out, err := marshalConfig(&cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "context:") || !strings.Contains(s, "contexts:") || !strings.Contains(s, "jamfpro:") {
		t.Fatalf("rewrite should write new keys:\n%s", s)
	}
	if strings.Contains(s, "\ntenants:") || strings.Contains(s, "\ntenant:") {
		t.Fatalf("must not keep old top-level keys:\n%s", s)
	}
}

func TestLoadConfigRewritesLegacyWorkplaceKeys(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	path := filepath.Join(dir, "buddy", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := []byte(`tenant:
  default: sandbox
tenants:
  sandbox:
    url: https://example.jamfcloud.com
auths:
  sandgod:
    type: oauth
    tenants: [sandbox]
    clientId: cid
`)
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	_, notes, err := parseAndMigrateUserYAML(legacy)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, n := range notes {
		if strings.Contains(n, "context/contexts") {
			found = true
		}
	}
	if !found {
		t.Fatalf("notes=%v", notes)
	}

	loaded, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Tenants["sandbox"].URL != "https://example.jamfcloud.com" {
		t.Fatalf("loaded=%+v", loaded.Tenants["sandbox"])
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "context:") || !strings.Contains(s, "orgs:") {
		t.Fatalf("rewrite missing new keys:\n%s", s)
	}
	if yamlHasTopLevelKey(s, "contexts") {
		t.Fatalf("folded workplaces should not remain under contexts:\n%s", s)
	}
	if strings.Contains(s, "\ntenants:") || strings.Contains(s, "\ntenant:") {
		t.Fatalf("old keys still on disk:\n%s", s)
	}
	_, notes2, err := parseAndMigrateUserYAML(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range notes2 {
		if strings.Contains(n, "context/contexts") {
			t.Fatalf("second pass should not migrate workplace keys: %v", notes2)
		}
	}
}

func TestTenantConfig_BothIDsValid(t *testing.T) {
	tcfg := TenantConfig{
		URL:           "https://example.jamfcloud.com",
		Region:        "us",
		EnvironmentID: "env-1",
		TenantID:      "tid-1",
	}
	if err := tcfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestFormatWorkplaceLabels(t *testing.T) {
	org := TenantConfig{}
	if FormatWorkplaceKind(org) != "organization" {
		t.Fatalf("org kind=%q", FormatWorkplaceKind(org))
	}
	if !strings.Contains(FormatWorkplaceAccount(org), "no X-Environment-Id/X-Tenant-Id") {
		t.Fatalf("org account=%q", FormatWorkplaceAccount(org))
	}
	env := TenantConfig{Region: "us", EnvironmentID: "11111111-1111-1111-1111-111111111111", Org: "acme"}
	if FormatWorkplaceKind(env) != "environment" {
		t.Fatalf("env kind=%q", FormatWorkplaceKind(env))
	}
	acc := FormatWorkplaceAccount(env)
	if !strings.Contains(acc, "X-Environment-Id=11111111-1111-1111-1111-111111111111") || !strings.Contains(acc, "org=acme") {
		t.Fatalf("env account=%q", acc)
	}
	ten := TenantConfig{
		URL:           "https://example.jamfcloud.com",
		Region:        "us",
		EnvironmentID: "11111111-1111-1111-1111-111111111111",
		TenantID:      "22222222-2222-2222-2222-222222222222",
	}
	if FormatWorkplaceKind(ten) != "product-tenant" {
		t.Fatalf("tenant kind=%q", FormatWorkplaceKind(ten))
	}
	tacc := FormatWorkplaceAccount(ten)
	if !strings.Contains(tacc, "X-Tenant-Id=") || !strings.Contains(tacc, "https://example.jamfcloud.com") {
		t.Fatalf("tenant account=%q", tacc)
	}
	inst := TenantConfig{URL: "https://example.jamfcloud.com"}
	if FormatWorkplaceKind(inst) != "instance" {
		t.Fatalf("instance kind=%q", FormatWorkplaceKind(inst))
	}
	if FormatWorkplaceAccount(inst) != "https://example.jamfcloud.com" {
		t.Fatalf("instance account=%q", FormatWorkplaceAccount(inst))
	}
	if FormatAuthScope(AuthEntry{Type: AuthTypePlatform}) != PlatformScopeEnvironment {
		t.Fatalf("platform default scope=%q", FormatAuthScope(AuthEntry{Type: AuthTypePlatform}))
	}
	if FormatAuthScope(AuthEntry{Type: AuthTypeOAuth}) != "-" {
		t.Fatalf("oauth scope=%q", FormatAuthScope(AuthEntry{Type: AuthTypeOAuth}))
	}
	if !strings.Contains(FormatAuthScopeDetail(AuthEntry{Type: AuthTypePlatform, Scope: PlatformScopeEnvironment}), "X-Environment-Id") {
		t.Fatal("environment scope detail")
	}
}

func TestWorkplaceTreeOrder(t *testing.T) {
	tenants := map[string]TenantConfig{
		"jamf":       {URL: "https://example.jamfcloud.com"},
		"prod":       {URL: "https://prod.example.jamfcloud.com", Region: "us", EnvironmentID: "e1", TenantID: "t1", Org: "acme"},
		"lab":        {URL: "https://lab.example.jamfcloud.com", Region: "us", EnvironmentID: "e2", TenantID: "t2", Org: "acme"},
		"staging":    {Region: "us", EnvironmentID: "e2", Org: "acme"},
		"commercial": {Region: "us", EnvironmentID: "e1", Org: "acme"},
		"acme":       {},
		"beta":       {},
		"beta-env":   {Region: "eu", EnvironmentID: "e3", Org: "beta"},
	}
	got := WorkplaceTreeOrder(tenants)
	want := []string{"acme", "commercial", "prod", "staging", "lab", "beta", "beta-env", "jamf"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v want %v", got, want)
	}
	rows := WorkplaceTree(tenants)
	byName := map[string]WorkplaceTreeRow{}
	for _, r := range rows {
		byName[r.Name] = r
	}
	if byName["prod"].Org != "acme" || byName["prod"].Environment != "commercial" || byName["prod"].Kind != "product-tenant" {
		t.Fatalf("prod row=%+v", byName["prod"])
	}
	if byName["lab"].Environment != "staging" {
		t.Fatalf("lab row=%+v", byName["lab"])
	}
	if byName["acme"].Org != "acme" || byName["acme"].Environment != "" {
		t.Fatalf("acme row=%+v", byName["acme"])
	}
}

func TestRewriteLeadingYAMLTabs(t *testing.T) {
	in := []byte("context:\n\tdefault: sandbox\n")
	out := rewriteLeadingYAMLTabs(in)
	if bytes.Contains(out, []byte{'\t'}) {
		t.Fatalf("tabs remain: %q", out)
	}
	if !bytes.Contains(out, []byte("  default: sandbox")) {
		t.Fatalf("expected two-space indent: %q", out)
	}
}

func TestParseAndMigrateUserYAML_FoldsFlatAndTabs(t *testing.T) {
	raw := []byte("context:\n\tdefault: sandbox\ncontexts:\n\tsandbox:\n\t\tjamfpro:\n\t\t\turl: https://example.jamfcloud.com\n")
	cfg, notes, err := parseAndMigrateUserYAML(raw)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(notes, "; ")
	if !strings.Contains(joined, "leading tabs") || !strings.Contains(joined, "flat contexts") {
		t.Fatalf("notes=%v", notes)
	}
	if cfg.Orgs["default"].Environments["sandbox"].EffectiveURL() != "https://example.jamfcloud.com" {
		t.Fatalf("orgs=%+v", cfg.Orgs)
	}
}

func TestLoadConfig_LeadingTabsRewritePersists(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "no-managed.plist"))
	path := filepath.Join(dir, "buddy", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	raw := []byte("context:\n\tdefault: sandbox\ncontexts:\n\tsandbox:\n\t\tjamfpro:\n\t\t\turl: https://example.jamfcloud.com\n")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Tenants["sandbox"].URL != "https://example.jamfcloud.com" {
		t.Fatalf("sandbox=%+v", loaded.Tenants["sandbox"])
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if strings.Contains(s, "\t") {
		t.Fatalf("tabs still on disk:\n%s", s)
	}
	if !strings.Contains(s, "orgs:") || yamlHasTopLevelKey(s, "contexts") {
		t.Fatalf("expected orgs-only rewrite:\n%s", s)
	}
}

func TestLastGoodConfigRestore(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "no-managed.plist"))
	path := filepath.Join(dir, "buddy", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	good := []byte(`context:
  default: sandbox
orgs:
  default:
    environments:
      sandbox:
        url: https://example.jamfcloud.com
`)
	if err := os.WriteFile(path, good, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	okPath := lastGoodConfigPath(path)
	if _, err := os.Stat(okPath); err != nil {
		t.Fatalf("expected last-good snapshot: %v", err)
	}
	if err := os.WriteFile(path, []byte(":\tnot yaml\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = LoadConfig()
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !IsConfigParseError(err) {
		t.Fatalf("IsConfigParseError=false: %v", err)
	}
	if !strings.Contains(err.Error(), "last good") || !strings.Contains(err.Error(), "restore-config") {
		t.Fatalf("error should mention last good: %v", err)
	}
	if err := RestoreLastGoodConfig(); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Tenants["sandbox"].URL != "https://example.jamfcloud.com" {
		t.Fatalf("restored=%+v", loaded.Tenants["sandbox"])
	}
}
