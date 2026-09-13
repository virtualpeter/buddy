package buddy

import (
	"strings"
	"testing"
)

func TestPolicyListV1Session_EnvironmentOnlySkips(t *testing.T) {
	t.Setenv(EnvClassicInstanceOnly, "false")
	cfg := &Config{
		Tenant: TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{
			"sandbox": {
				Region:        "us",
				EnvironmentID: "11111111-1111-1111-1111-111111111111",
			},
		},
		Auths: map[string]AuthEntry{
			"sandbox-platform": {
				Type: AuthTypePlatform, ClientID: "c", ClientSecret: "s",
				Tenants: []string{"sandbox"}, Access: AccessFull,
			},
		},
	}
	SetActiveConfig(cfg)
	SetActiveSelection("sandbox", "")
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveSelection("", "")
	})

	err := policyListV1Session("")
	if err == nil {
		t.Fatal("expected OpProUI to fail on an environment-only workplace")
	}
	if !strings.Contains(err.Error(), "no instance url") {
		t.Fatalf("got %v", err)
	}

	sess, err := cfg.ResolveSession("sandbox", "", OpClassic)
	if err != nil {
		t.Fatal(err)
	}
	if sess.AuthName != "sandbox-platform" || sess.Surface != SurfacePlatform {
		t.Fatalf("Classic should use platform on the gateway, got %s/%s", sess.AuthName, sess.Surface)
	}
}

func TestPolicyListV1Session_InstanceURLTriesV1(t *testing.T) {
	t.Setenv(EnvClassicInstanceOnly, "false")
	cfg := dualSurfaceTestConfig()
	cfg.Features.ClassicInstanceOnly = boolPtr(false)
	SetActiveConfig(cfg)
	SetActiveSelection("sandbox", "")
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveSelection("", "")
	})

	if err := policyListV1Session(""); err != nil {
		t.Fatalf("hybrid with instance URL should try v1: %v", err)
	}
	if err := policyListV1Session("plat"); err != nil {
		t.Fatalf("platform pin should still fall back to instance for OpProUI: %v", err)
	}
}

func TestPolicyListV1Session_PlatformOnlyOnHybridSkips(t *testing.T) {
	t.Setenv(EnvClassicInstanceOnly, "false")
	cfg := dualSurfaceTestConfig()
	delete(cfg.Auths, "inst")
	SetActiveConfig(cfg)
	SetActiveSelection("sandbox", "")
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveSelection("", "")
	})

	err := policyListV1Session("")
	if err == nil {
		t.Fatal("expected OpProUI to fail when only platform auth is linked")
	}
}