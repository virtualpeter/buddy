package buddy

import (
	"strings"
	"testing"
	"time"
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
	if sess.AuthName != "plat" || sess.Surface != SurfacePlatform {
		t.Fatalf("Classic write with surface=platform pref: got %s/%s want plat/platform", sess.AuthName, sess.Surface)
	}

	sess, err = cfg.ResolveSession("sandbox", "plat", OpClassicWrite)
	if err != nil {
		t.Fatal(err)
	}
	if sess.AuthName != "plat" || sess.Surface != SurfacePlatform {
		t.Fatalf("platform pin + surface=platform should send Classic via gateway: got %s/%s", sess.AuthName, sess.Surface)
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
	if sess.AuthName != "plat" || sess.Surface != SurfacePlatform {
		t.Fatalf("process platform pin + Classic write: got %s/%s want plat/platform", sess.AuthName, sess.Surface)
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
	cfg.Features.ClassicInstanceOnly = boolPtr(true)
	_, err := cfg.ResolveSession("sandbox", "plat", OpClassicWrite)
	if err == nil {
		t.Fatal("expected error when Classic is locked to instance and no instance auth exists")
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

	sess, err := cfg.ResolveSession("sandbox", "plat", OpClassicWrite)
	if err != nil {
		t.Fatal(err)
	}
	if sess.AuthName != "plat" || sess.Surface != SurfacePlatform {
		t.Fatalf("default off should honor platform pin for Classic, got %s/%s", sess.AuthName, sess.Surface)
	}

	sess, err = cfg.ResolveSession("sandbox", "", OpClassicWrite)
	if err != nil {
		t.Fatal(err)
	}
	if sess.AuthName != "inst" {
		t.Fatalf("auto-pick still prefers instance, got %s", sess.AuthName)
	}
}

func TestClassicInstanceOnly_TrueForcesInstance(t *testing.T) {
	t.Setenv(EnvClassicInstanceOnly, "")
	cfg := dualSurfaceTestConfig()
	cfg.Features.ClassicInstanceOnly = boolPtr(true)

	sess, err := cfg.ResolveSession("sandbox", "plat", OpClassicWrite)
	if err != nil {
		t.Fatal(err)
	}
	if sess.AuthName != "inst" || sess.Surface != SurfaceInstance {
		t.Fatalf("flag on should soft-switch Classic to instance, got %s/%s", sess.AuthName, sess.Surface)
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

func TestProUIPoliciesListAlwaysInstance(t *testing.T) {
	t.Setenv(EnvClassicInstanceOnly, "false")
	cfg := dualSurfaceTestConfig()
	cfg.Features.ClassicInstanceOnly = boolPtr(false)

	if got := opClassForPath("GET", "/api/v1/policies"); got != OpProUI {
		t.Fatalf("GET /api/v1/policies → %q want %q", got, OpProUI)
	}

	sess, err := cfg.ResolveSession("sandbox", "plat", OpProUI)
	if err != nil {
		t.Fatal(err)
	}
	if sess.AuthName != "inst" || sess.Surface != SurfaceInstance {
		t.Fatalf("undocumented Pro policy list must stay on instance even when Classic-on-platform is allowed: got %s/%s", sess.AuthName, sess.Surface)
	}

	restore, err := BeginSessionOverrides(SessionOverrides{Surface: SurfacePlatform})
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	sess, err = cfg.ResolveSession("sandbox", "", OpProUI)
	if err != nil {
		t.Fatal(err)
	}
	if sess.AuthName != "inst" || sess.Surface != SurfaceInstance {
		t.Fatalf("surface=platform must not send GET /api/v1/policies via gateway: got %s/%s", sess.AuthName, sess.Surface)
	}
}

func TestPickAuthHybridReadResourceClassFlip(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(SetCapsCacheDirForTest(dir))
	cfg := &Config{
		Tenant: TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{
			"sandbox": {
				URL:      "https://example.jamfcloud.com",
				Region:   "us",
				TenantID: "00000000-0000-4000-8000-000000000001",
			},
		},
		Auths: map[string]AuthEntry{
			"aaa-plat": {
				Type: "oauth", ClientID: "c", ClientSecret: "s",
				Tenants: []string{"sandbox"}, Access: AccessFull, Surface: SurfacePlatform,
			},
			"zzz-inst": {
				Type: "oauth", ClientID: "c2", ClientSecret: "s2",
				Tenants: []string{"sandbox"}, Access: AccessFull, Surface: SurfaceInstance,
			},
		},
	}
	exp := time.Now().Add(time.Hour)
	tid := "00000000-0000-4000-8000-000000000001"
	if err := SaveCaps(CapsRecord{
		Auth: "aaa-plat", Tenant: "sandbox", Surface: SurfacePlatform, TenantID: tid,
		Live: LiveOK, TokenOk: true, ProbeOk: true, ExpiresAt: exp,
	}); err != nil {
		t.Fatal(err)
	}
	if err := SaveCaps(CapsRecord{
		Auth: "zzz-inst", Tenant: "sandbox", Surface: SurfaceInstance,
		Live: LiveOK, TokenOk: true, ProbeOk: true,
		Privileges: []string{"Read Scripts"}, ExpiresAt: exp,
	}); err != nil {
		t.Fatal(err)
	}
	tenant := cfg.Tenants["sandbox"]
	cands := []string{"aaa-plat", "zzz-inst"}

	name, _, surface, err := pickAuth(cfg, "sandbox", tenant, cands, OpRead, ChannelCLI)
	if err != nil {
		t.Fatal(err)
	}
	if name != "aaa-plat" || surface != SurfacePlatform {
		t.Fatalf("no resource class: got %s/%s want aaa-plat/platform", name, surface)
	}

	restore := setResourceClass("scripts")
	defer restore()
	name, _, surface, err = pickAuth(cfg, "sandbox", tenant, cands, OpRead, ChannelCLI)
	if err != nil {
		t.Fatal(err)
	}
	if name != "zzz-inst" || surface != SurfaceInstance {
		t.Fatalf("scripts class: got %s/%s want zzz-inst/instance", name, surface)
	}
}
