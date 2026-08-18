package buddy

import (
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
	if !strings.Contains(s, "tenants:") || !strings.Contains(s, "auths:") {
		t.Fatalf("expected tenants/auths:\n%s", s)
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
	order := yamlMapKeyOrder(data, "tenants")
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
				URL: "https://us.apigw.jamf.com",
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
	_, err = cfg.ResolveSession("t", "ro", OpMutate)
	if err == nil {
		t.Fatal("expected mutate to reject readonly pin")
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
