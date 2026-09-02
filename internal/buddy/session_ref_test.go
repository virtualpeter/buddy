package buddy

import (
	"testing"
)

func TestSessionRefEqual(t *testing.T) {
	a := SessionRef{Tenant: "sandbox", Auth: "admin"}
	b := SessionRef{Tenant: " sandbox ", Auth: "admin"}
	if !a.Equal(b) {
		t.Fatal("expected equal after trim")
	}
	if a.Equal(SessionRef{Tenant: "prod", Auth: "admin"}) {
		t.Fatal("different tenants")
	}
}

func TestResolveSessionRefTenantOnly(t *testing.T) {
	cfg := &Config{
		Tenant: TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{
			"sandbox": {URL: "https://example.jamfcloud.com"},
			"prod":    {URL: "https://prod.example.jamfcloud.com"},
		},
		Auths: map[string]AuthEntry{
			"sandbox-admin": {
				Type:       "command",
				RunCommand: "printf tok",
				Tenants:    []string{"sandbox"},
			},
			"prod-admin": {
				Type:       "command",
				RunCommand: "printf tok2",
				Tenants:    []string{"prod"},
			},
		},
	}
	SetActiveConfig(cfg)
	t.Cleanup(func() { SetActiveConfig(nil) })
	SetActiveSelection("sandbox", "")
	t.Cleanup(func() { SetActiveSelection("", "") })

	src, err := ResolveSessionRef(SessionRef{Tenant: "sandbox"}, OpRead)
	if err != nil {
		t.Fatal(err)
	}
	if src.Tenant != "sandbox" || src.Auth != "sandbox-admin" {
		t.Fatalf("src=%+v", src)
	}
	dst, err := ResolveSessionRef(SessionRef{Tenant: "prod"}, OpMutate)
	if err != nil {
		t.Fatal(err)
	}
	if dst.Tenant != "prod" || dst.Auth != "prod-admin" {
		t.Fatalf("dst=%+v", dst)
	}
	if src.Equal(dst) {
		t.Fatal("src and dst should differ")
	}

	// Ambient selection must be restored after resolve.
	tName, pin := ActiveSelection()
	if tName != "sandbox" || pin != "" {
		t.Fatalf("ActiveSelection after resolve: %q %q", tName, pin)
	}
}
