package buddy

import (
	"strings"
	"testing"
)

func TestResolveSession_SoftSwitchImplicitTenant(t *testing.T) {
	cfg := &Config{
		Tenant: TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{
			"sandbox": {URL: "https://sandbox.example.jamfcloud.com"},
			"prod":    {URL: "https://prod.example.jamfcloud.com"},
		},
		Auths: map[string]AuthEntry{
			"sandbox-admin": {
				Type:     "basic",
				Username: "admin",
				Password: "x",
				Tenants:  []string{"sandbox"},
				Access:   AccessFull,
				Surface:  SurfaceInstance,
			},
			"prod-ro": {
				Type:         "oauth",
				ClientID:     "cid",
				ClientSecret: "sec",
				Tenants:      []string{"prod"},
				Access:       AccessReadonly,
				Surface:      SurfaceInstance,
			},
		},
	}
	SetActiveConfig(cfg)
	SetActiveSelection("sandbox", "")
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveSelection("", "")
		setSurfacePreference("")
	})

	sess, err := cfg.ResolveSession("", "prod-ro", OpRead)
	if err != nil {
		t.Fatal(err)
	}
	if sess.TenantName != "prod" || sess.AuthName != "prod-ro" {
		t.Fatalf("got tenant=%q auth=%q", sess.TenantName, sess.AuthName)
	}

	_, err = cfg.ResolveSession("sandbox", "prod-ro", OpRead)
	if err == nil || !strings.Contains(err.Error(), "not linked") {
		t.Fatalf("expected not linked error, got %v", err)
	}
}

func TestResolveSession_SurfacePreference(t *testing.T) {
	t.Setenv(EnvClassicInstanceOnly, "")
	cfg := &Config{
		Tenant: TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{
			"sandbox": {
				URL:      "https://sandbox.example.jamfcloud.com",
				Region:   "us",
				TenantID: "tid-1",
			},
		},
		Auths: map[string]AuthEntry{
			"inst": {
				Type: "basic", Username: "a", Password: "b",
				Tenants: []string{"sandbox"}, Access: AccessFull, Surface: SurfaceInstance,
			},
			"plat": {
				Type: "oauth", ClientID: "c", ClientSecret: "s",
				Tenants: []string{"sandbox"}, Access: AccessFull, Surface: SurfacePlatform,
			},
		},
	}
	SetActiveConfig(cfg)
	SetActiveSelection("sandbox", "")
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveSelection("", "")
		setSurfacePreference("")
	})

	restore, err := BeginSessionOverrides(SessionOverrides{Surface: SurfacePlatform})
	if err != nil {
		t.Fatal(err)
	}
	defer restore()

	sess, err := cfg.ResolveSession("sandbox", "", OpRead)
	if err != nil {
		t.Fatal(err)
	}
	if sess.AuthName != "plat" {
		t.Fatalf("surface=platform preferred auth=%q want plat", sess.AuthName)
	}

	sess, err = cfg.ResolveSession("sandbox", "", OpClassicWrite)
	if err != nil {
		t.Fatal(err)
	}
	if sess.AuthName != "inst" || sess.Surface != SurfaceInstance {
		t.Fatalf("Classic write with surface=platform pref: got %s/%s want inst/instance", sess.AuthName, sess.Surface)
	}

	sess, err = cfg.ResolveSession("sandbox", "plat", OpClassicWrite)
	if err != nil {
		t.Fatal(err)
	}
	if sess.AuthName != "inst" || sess.Surface != SurfaceInstance {
		t.Fatalf("platform pin + surface=platform must not send Classic via gateway: got %s/%s", sess.AuthName, sess.Surface)
	}
}

func TestResolveSession_SoftSwitchSurfacePin(t *testing.T) {
	t.Setenv(EnvClassicInstanceOnly, "")
	cfg := &Config{
		Tenant: TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{
			"sandbox": {
				URL:      "https://sandbox.example.jamfcloud.com",
				Region:   "us",
				TenantID: "00000000-0000-4000-8000-000000000001",
			},
		},
		Auths: map[string]AuthEntry{
			"inst": {
				Type: "basic", Username: "a", Password: "b",
				Tenants: []string{"sandbox"}, Access: AccessFull, Surface: SurfaceInstance,
			},
			"plat": {
				Type: "oauth", ClientID: "c", ClientSecret: "s",
				Tenants: []string{"sandbox"}, Access: AccessFull, Surface: SurfacePlatform,
			},
		},
	}
	SetActiveConfig(cfg)
	SetActiveSelection("sandbox", "plat")
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveSelection("", "")
		setSurfacePreference("")
	})

	sess, err := cfg.ResolveSession("", "", OpClassicWrite)
	if err != nil {
		t.Fatal(err)
	}
	if sess.AuthName != "inst" || sess.Surface != SurfaceInstance {
		t.Fatalf("process platform pin + Classic write: got %s/%s want inst/instance", sess.AuthName, sess.Surface)
	}

	sess, err = cfg.ResolveSession("sandbox", "inst", OpPlatform)
	if err != nil {
		t.Fatal(err)
	}
	if sess.AuthName != "plat" || sess.Surface != SurfacePlatform {
		t.Fatalf("instance pin + platform op: got %s/%s want plat/platform", sess.AuthName, sess.Surface)
	}
}

func TestResolveSession_SurfacePinNoFallback(t *testing.T) {
	t.Setenv(EnvClassicInstanceOnly, "")
	cfg := &Config{
		Tenant: TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{
			"sandbox": {
				URL:      "https://sandbox.example.jamfcloud.com",
				Region:   "us",
				TenantID: "00000000-0000-4000-8000-000000000001",
			},
		},
		Auths: map[string]AuthEntry{
			"plat": {
				Type: "oauth", ClientID: "c", ClientSecret: "s",
				Tenants: []string{"sandbox"}, Access: AccessFull, Surface: SurfacePlatform,
			},
		},
	}
	_, err := cfg.ResolveSession("sandbox", "plat", OpClassicWrite)
	if err == nil {
		t.Fatal("expected error when no instance auth exists")
	}
	if !strings.Contains(err.Error(), "no suitable auth") {
		t.Fatalf("got %v", err)
	}
}

func dualSurfaceTestConfig() *Config {
	return &Config{
		Tenant: TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{
			"sandbox": {
				URL:      "https://sandbox.example.jamfcloud.com",
				Region:   "us",
				TenantID: "00000000-0000-4000-8000-000000000001",
			},
		},
		Auths: map[string]AuthEntry{
			"inst": {
				Type: "basic", Username: "a", Password: "b",
				Tenants: []string{"sandbox"}, Access: AccessFull, Surface: SurfaceInstance,
			},
			"plat": {
				Type: "oauth", ClientID: "c", ClientSecret: "s",
				Tenants: []string{"sandbox"}, Access: AccessFull, Surface: SurfacePlatform,
			},
		},
	}
}

func TestClassicInstanceOnly_AllowsPlatformPin(t *testing.T) {
	t.Setenv(EnvClassicInstanceOnly, "")
	cfg := dualSurfaceTestConfig()
	cfg.Features.ClassicInstanceOnly = boolPtr(false)

	sess, err := cfg.ResolveSession("sandbox", "plat", OpClassicWrite)
	if err != nil {
		t.Fatal(err)
	}
	if sess.AuthName != "plat" || sess.Surface != SurfacePlatform {
		t.Fatalf("flag off should honor platform pin for Classic, got %s/%s", sess.AuthName, sess.Surface)
	}

	sess, err = cfg.ResolveSession("sandbox", "", OpClassicWrite)
	if err != nil {
		t.Fatal(err)
	}
	if sess.AuthName != "inst" {
		t.Fatalf("auto-pick still prefers instance, got %s", sess.AuthName)
	}
}

func TestClassicInstanceOnly_EnvFalse(t *testing.T) {
	t.Setenv(EnvClassicInstanceOnly, "false")
	cfg := dualSurfaceTestConfig()
	cfg.Features.ClassicInstanceOnly = boolPtr(true)

	sess, err := cfg.ResolveSession("sandbox", "plat", OpClassicWrite)
	if err != nil {
		t.Fatal(err)
	}
	if sess.AuthName != "plat" || sess.Surface != SurfacePlatform {
		t.Fatalf("env false should override YAML true, got %s/%s", sess.AuthName, sess.Surface)
	}
}

func TestClassicInstanceOnly_PlatformOnlyWhenOff(t *testing.T) {
	t.Setenv(EnvClassicInstanceOnly, "false")
	cfg := &Config{
		Tenant: TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{
			"sandbox": {
				URL:      "https://sandbox.example.jamfcloud.com",
				Region:   "us",
				TenantID: "00000000-0000-4000-8000-000000000001",
			},
		},
		Auths: map[string]AuthEntry{
			"plat": {
				Type: "oauth", ClientID: "c", ClientSecret: "s",
				Tenants: []string{"sandbox"}, Access: AccessFull, Surface: SurfacePlatform,
			},
		},
	}
	sess, err := cfg.ResolveSession("sandbox", "", OpClassicWrite)
	if err != nil {
		t.Fatal(err)
	}
	if sess.AuthName != "plat" || sess.Surface != SurfacePlatform {
		t.Fatalf("got %s/%s", sess.AuthName, sess.Surface)
	}
}

func TestACSWriteAlwaysInstance(t *testing.T) {
	t.Setenv(EnvClassicInstanceOnly, "false")
	cfg := dualSurfaceTestConfig()
	cfg.Features.ClassicInstanceOnly = boolPtr(false)

	sess, err := cfg.ResolveSession("sandbox", "plat", OpACSWrite)
	if err != nil {
		t.Fatal(err)
	}
	if sess.AuthName != "inst" || sess.Surface != SurfaceInstance {
		t.Fatalf("ACS must stay on instance even when Classic-on-platform is allowed: got %s/%s", sess.AuthName, sess.Surface)
	}
}
