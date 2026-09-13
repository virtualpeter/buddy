package buddy

import (
	"strings"
	"testing"
)

func envWithJamfProChildConfig() *Config {
	return &Config{
		Tenant: TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{
			"sandbox": {
				Region:        "us",
				EnvironmentID: "11111111-1111-1111-1111-111111111111",
			},
			"sandbox-pro": {
				URL:           "https://sandbox.example.jamfcloud.com",
				Region:        "us",
				EnvironmentID: "11111111-1111-1111-1111-111111111111",
				TenantID:      "22222222-2222-2222-2222-222222222222",
			},
		},
		Auths: map[string]AuthEntry{
			"sandbox-platform": {
				Type: AuthTypePlatform, ClientID: "c", ClientSecret: "s",
				Tenants: []string{"sandbox"}, Access: AccessFull,
			},
			"sandbox-admin": {
				Type: "basic", Username: "a", Password: "b",
				Tenants: []string{"sandbox-pro"}, Access: AccessFull, Surface: SurfaceInstance,
			},
		},
	}
}

func TestProductTenantsUnderEnvironment(t *testing.T) {
	tenants := map[string]TenantConfig{
		"prod":       {URL: "https://prod.example.jamfcloud.com", Region: "us", EnvironmentID: "e1", TenantID: "t1", Org: "acme"},
		"lab":        {URL: "https://lab.example.jamfcloud.com", Region: "us", EnvironmentID: "e2", TenantID: "t2", Org: "acme"},
		"staging":    {Region: "us", EnvironmentID: "e2", Org: "acme"},
		"commercial": {Region: "us", EnvironmentID: "e1", Org: "acme"},
		"acme":       {},
	}
	if got := strings.Join(ProductTenantsUnderEnvironment(tenants, "commercial"), ","); got != "prod" {
		t.Fatalf("commercial children = %q", got)
	}
	if got := strings.Join(ProductTenantsUnderEnvironment(tenants, "staging"), ","); got != "lab" {
		t.Fatalf("staging children = %q", got)
	}
	if got := ProductTenantsUnderEnvironment(tenants, "acme"); len(got) != 0 {
		t.Fatalf("org should have no product-tenant children, got %v", got)
	}
}

func TestPickAppScanProUISession_UsesJamfProChild(t *testing.T) {
	t.Setenv(EnvClassicInstanceOnly, "false")
	cfg := envWithJamfProChildConfig()
	SetActiveConfig(cfg)
	SetActiveSelection("sandbox", "")
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveSelection("", "")
	})

	tenant, auth, ok := pickAppPolicyIndexSession("")
	if !ok || tenant != "sandbox-pro" || auth != "sandbox-admin" {
		t.Fatalf("got tenant=%q auth=%q ok=%v", tenant, auth, ok)
	}

	restore, pin, indexCtx := beginAppPolicyIndexSession("", nil)
	defer restore()
	if pin != "sandbox-admin" || indexCtx != "sandbox-pro" {
		t.Fatalf("pin=%q context=%q", pin, indexCtx)
	}
	if err := policyListV1Session(pin); err != nil {
		t.Fatalf("relocated scan should resolve OpProUI: %v", err)
	}
	selT, selA := ActiveSelection()
	if selT != "sandbox-pro" || selA != "sandbox-admin" {
		t.Fatalf("active selection %s/%s", selT, selA)
	}
}

func TestPickAppScanProUISession_NoChildStaysOnEnvironment(t *testing.T) {
	t.Setenv(EnvClassicInstanceOnly, "false")
	cfg := envWithJamfProChildConfig()
	delete(cfg.Tenants, "sandbox-pro")
	delete(cfg.Auths, "sandbox-admin")
	SetActiveConfig(cfg)
	SetActiveSelection("sandbox", "")
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveSelection("", "")
	})

	if _, _, ok := pickAppPolicyIndexSession(""); ok {
		t.Fatal("expected no relocation without a Jamf Pro child")
	}
	if err := policyListV1Session(""); err == nil {
		t.Fatal("environment-only should still skip v1")
	}
}

func TestPickAppScanProUISession_ChildWithoutInstanceAuthSkipped(t *testing.T) {
	t.Setenv(EnvClassicInstanceOnly, "false")
	cfg := envWithJamfProChildConfig()
	delete(cfg.Auths, "sandbox-admin")
	SetActiveConfig(cfg)
	SetActiveSelection("sandbox", "")
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveSelection("", "")
	})

	if _, _, ok := pickAppPolicyIndexSession(""); ok {
		t.Fatal("expected no relocation when the child has no instance auth")
	}
}

func TestPickAppScanProUISession_AlreadyOnProductTenant(t *testing.T) {
	t.Setenv(EnvClassicInstanceOnly, "false")
	cfg := envWithJamfProChildConfig()
	SetActiveConfig(cfg)
	SetActiveSelection("sandbox-pro", "sandbox-admin")
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveSelection("", "")
	})

	if _, _, ok := pickAppPolicyIndexSession("sandbox-admin"); ok {
		t.Fatal("already on the product tenant; do not relocate")
	}
}

func TestPickAppScanProUISession_SkipsChildWithoutInstanceURL(t *testing.T) {
	t.Setenv(EnvClassicInstanceOnly, "false")
	cfg := envWithJamfProChildConfig()
	cfg.Tenants["protect"] = TenantConfig{
		Region:        "us",
		EnvironmentID: "11111111-1111-1111-1111-111111111111",
		TenantID:      "33333333-3333-3333-3333-333333333333",
	}
	SetActiveConfig(cfg)
	SetActiveSelection("sandbox", "")
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveSelection("", "")
	})

	tenant, auth, ok := pickAppPolicyIndexSession("")
	if !ok || tenant != "sandbox-pro" || auth != "sandbox-admin" {
		t.Fatalf("should skip URL-less child, got tenant=%q auth=%q ok=%v", tenant, auth, ok)
	}
}

func TestGetPolicyCacheStatus_UsesJamfProChildDir(t *testing.T) {
	t.Setenv(EnvClassicInstanceOnly, "false")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := envWithJamfProChildConfig()
	SetActiveConfig(cfg)
	SetActiveSelection("sandbox", "")
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveSelection("", "")
	})

	st, err := GetPolicyCacheStatus("")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(st.Dir, "sandbox-admin@sandbox-pro") {
		t.Fatalf("cache dir should follow the Jamf Pro child, got %q", st.Dir)
	}
}
