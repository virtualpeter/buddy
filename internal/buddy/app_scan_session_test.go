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

func TestResolveSession_EnvProUIUsesJamfProChild(t *testing.T) {
	t.Setenv(EnvClassicInstanceOnly, "false")
	cfg := envWithJamfProChildConfig()
	SetActiveConfig(cfg)
	SetActiveSelection("sandbox", "")
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveSelection("", "")
	})

	sess, err := cfg.ResolveSession("sandbox", "", OpClassic)
	if err != nil {
		t.Fatal(err)
	}
	if sess.TenantName != "sandbox" || sess.AuthName != "sandbox-platform" || sess.Surface != SurfacePlatform {
		t.Fatalf("Classic should stay on env platform, got %s/%s/%s", sess.TenantName, sess.AuthName, sess.Surface)
	}

	sess, err = cfg.ResolveSession("sandbox", "", OpRead)
	if err != nil {
		t.Fatal(err)
	}
	if sess.TenantName != "sandbox" || sess.AuthName != "sandbox-platform" || sess.Surface != SurfacePlatform {
		t.Fatalf("OpRead should stay on env platform, got %s/%s/%s", sess.TenantName, sess.AuthName, sess.Surface)
	}

	sess, err = cfg.ResolveSession("sandbox", "", OpProUI)
	if err != nil {
		t.Fatal(err)
	}
	if sess.TenantName != "sandbox-pro" || sess.AuthName != "sandbox-admin" || sess.Surface != SurfaceInstance {
		t.Fatalf("OpProUI should use child instance, got %s/%s/%s", sess.TenantName, sess.AuthName, sess.Surface)
	}

	sess, err = cfg.ResolveSession("sandbox", "", OpACS)
	if err != nil {
		t.Fatal(err)
	}
	if sess.TenantName != "sandbox-pro" || sess.AuthName != "sandbox-admin" {
		t.Fatalf("OpACS should use child, got %s/%s", sess.TenantName, sess.AuthName)
	}

	if got := jamfProIndexContext(""); got != "sandbox-pro" {
		t.Fatalf("index context = %q", got)
	}

	selT, selA := ActiveSelection()
	if selT != "sandbox" || selA != "" {
		t.Fatalf("selection leaked: %s/%s", selT, selA)
	}
}

func TestResolveSession_EnvWithoutChildSkipsProUI(t *testing.T) {
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

	if _, err := cfg.ResolveSession("sandbox", "", OpProUI); err == nil {
		t.Fatal("expected OpProUI to fail without a Jamf Pro child")
	}
	if err := policyListV1Session(""); err == nil {
		t.Fatal("environment-only should still skip v1")
	}
	if got := jamfProIndexContext(""); got != "" {
		t.Fatalf("index context = %q", got)
	}
}

func TestResolveSession_EnvChildWithoutInstanceAuthSkipped(t *testing.T) {
	t.Setenv(EnvClassicInstanceOnly, "false")
	cfg := envWithJamfProChildConfig()
	delete(cfg.Auths, "sandbox-admin")
	SetActiveConfig(cfg)
	SetActiveSelection("sandbox", "")
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveSelection("", "")
	})

	if _, err := cfg.ResolveSession("sandbox", "", OpProUI); err == nil {
		t.Fatal("expected no relocation when the child has no instance auth")
	}
}

func TestResolveSession_AlreadyOnProductTenant(t *testing.T) {
	t.Setenv(EnvClassicInstanceOnly, "false")
	cfg := envWithJamfProChildConfig()
	SetActiveConfig(cfg)
	SetActiveSelection("sandbox-pro", "sandbox-admin")
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveSelection("", "")
	})

	sess, err := cfg.ResolveSession("sandbox-pro", "sandbox-admin", OpProUI)
	if err != nil {
		t.Fatal(err)
	}
	if sess.TenantName != "sandbox-pro" || sess.AuthName != "sandbox-admin" {
		t.Fatalf("already on the product tenant; got %s/%s", sess.TenantName, sess.AuthName)
	}
	if got := jamfProIndexContext("sandbox-admin"); got != "" {
		t.Fatalf("index context should be empty on product tenant, got %q", got)
	}
}

func TestResolveSession_SkipsChildWithoutInstanceURL(t *testing.T) {
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

	sess, err := cfg.ResolveSession("sandbox", "", OpProUI)
	if err != nil {
		t.Fatal(err)
	}
	if sess.TenantName != "sandbox-pro" || sess.AuthName != "sandbox-admin" {
		t.Fatalf("should skip URL-less child, got %s/%s", sess.TenantName, sess.AuthName)
	}
}

func TestResolveSession_EnvPlatformPinNotReusedOnChild(t *testing.T) {
	t.Setenv(EnvClassicInstanceOnly, "false")
	cfg := envWithJamfProChildConfig()
	SetActiveConfig(cfg)
	SetActiveSelection("sandbox", "sandbox-platform")
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveSelection("", "")
	})

	sess, err := cfg.ResolveSession("sandbox", "sandbox-platform", OpProUI)
	if err != nil {
		t.Fatal(err)
	}
	if sess.TenantName != "sandbox-pro" || sess.AuthName != "sandbox-admin" {
		t.Fatalf("env pin should not be reused: %s/%s", sess.TenantName, sess.AuthName)
	}
}

func TestResolveSession_EnvWithoutPlatformAuthUsesChildForClassic(t *testing.T) {
	t.Setenv(EnvClassicInstanceOnly, "false")
	cfg := envWithJamfProChildConfig()
	delete(cfg.Auths, "sandbox-platform")
	SetActiveConfig(cfg)
	SetActiveSelection("sandbox", "")
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveSelection("", "")
	})

	sess, err := cfg.ResolveSession("sandbox", "", OpClassic)
	if err != nil {
		t.Fatal(err)
	}
	if sess.TenantName != "sandbox-pro" || sess.AuthName != "sandbox-admin" {
		t.Fatalf("Classic without env platform auth should use child, got %s/%s", sess.TenantName, sess.AuthName)
	}

	sess, err = cfg.ResolveSession("sandbox", "", OpRead)
	if err != nil {
		t.Fatal(err)
	}
	if sess.TenantName != "sandbox-pro" || sess.AuthName != "sandbox-admin" {
		t.Fatalf("OpRead without env platform auth should use child, got %s/%s", sess.TenantName, sess.AuthName)
	}
}

func TestResolveSession_ClassicInstanceOnlyUsesProChild(t *testing.T) {
	t.Setenv(EnvClassicInstanceOnly, "true")
	cfg := envWithJamfProChildConfig()
	cfg.Features.ClassicInstanceOnly = boolPtr(true)
	SetActiveConfig(cfg)
	SetActiveSelection("sandbox", "")
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveSelection("", "")
	})

	sess, err := cfg.ResolveSession("sandbox", "", OpClassic)
	if err != nil {
		t.Fatal(err)
	}
	if sess.TenantName != "sandbox-pro" || sess.AuthName != "sandbox-admin" || sess.Surface != SurfaceInstance {
		t.Fatalf("classic_instance_only should use child, got %s/%s/%s", sess.TenantName, sess.AuthName, sess.Surface)
	}
}

func TestResolveSession_OpAnyAndPlatformStayOnEnvironment(t *testing.T) {
	t.Setenv(EnvClassicInstanceOnly, "false")
	cfg := envWithJamfProChildConfig()
	SetActiveConfig(cfg)
	SetActiveSelection("sandbox", "")
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveSelection("", "")
	})

	sess, err := cfg.ResolveSession("sandbox", "", OpAny)
	if err != nil {
		t.Fatal(err)
	}
	if sess.TenantName != "sandbox" || sess.AuthName != "sandbox-platform" {
		t.Fatalf("OpAny must not walk to Pro child, got %s/%s", sess.TenantName, sess.AuthName)
	}

	sess, err = cfg.ResolveSession("sandbox", "", OpPlatform)
	if err != nil {
		t.Fatal(err)
	}
	if sess.TenantName != "sandbox" || sess.AuthName != "sandbox-platform" {
		t.Fatalf("OpPlatform must not walk to Pro child, got %s/%s", sess.TenantName, sess.AuthName)
	}
}

func TestGetPolicyCacheStatus_UsesEnvironmentDirWhenClassicStaysOnGateway(t *testing.T) {
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
	if !strings.Contains(st.Dir, "sandbox-platform@sandbox") {
		t.Fatalf("cache dir should follow Classic-on-gateway, got %q", st.Dir)
	}
}
