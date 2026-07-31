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
}
