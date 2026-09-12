package buddy

import (
	"errors"
	"testing"
)

func TestConfigNeedsSetup(t *testing.T) {
	if !ConfigNeedsSetup(nil) {
		t.Fatal("nil config needs setup")
	}
	if !ConfigNeedsSetup(&Config{}) {
		t.Fatal("empty config needs setup")
	}
	if !ConfigNeedsSetup(&Config{Tenants: map[string]TenantConfig{}}) {
		t.Fatal("empty tenants map needs setup")
	}
	cfg := &Config{Tenants: map[string]TenantConfig{"sandbox": {URL: "https://example.jamfcloud.com"}}}
	if ConfigNeedsSetup(cfg) {
		t.Fatal("populated tenants should not need setup")
	}
}

func TestSetTenantAndAuthMetadata(t *testing.T) {
	cfg := &Config{}
	created, err := SetTenantMetadata(cfg, "sandbox", SetTenantOpts{
		URL:       "https://example.jamfcloud.com",
		Overwrite: true,
	})
	if err != nil || !created {
		t.Fatalf("create tenant: created=%v err=%v", created, err)
	}
	if cfg.Tenant.Default != "sandbox" {
		t.Fatalf("default tenant = %q", cfg.Tenant.Default)
	}
	_, err = SetTenantMetadata(cfg, "sandbox", SetTenantOpts{URL: "https://other.example.com"})
	if !errors.Is(err, ErrExists) {
		t.Fatalf("expected ErrExists, got %v", err)
	}
	created, err = SetAuthMetadata(cfg, "sandbox-oauth", SetAuthMetadataOpts{
		Type:      AuthTypeOAuth,
		ClientID:  "cid",
		Tenants:   []string{"sandbox"},
		Overwrite: true,
	})
	if err != nil || !created {
		t.Fatalf("create auth: created=%v err=%v", created, err)
	}
	snap := BuildConfigSnapshot(cfg)
	if snap.NeedsSetup || len(snap.Tenants) != 1 || len(snap.Auths) != 1 {
		t.Fatalf("snapshot %+v", snap)
	}
	if snap.Auths[0].Secret != SecretPresenceLocked {
		t.Fatalf("secret presence = %q", snap.Auths[0].Secret)
	}
}

func TestDeleteTenantCascadesAuth(t *testing.T) {
	cfg := &Config{
		Tenants: map[string]TenantConfig{"sandbox": {URL: "https://example.jamfcloud.com"}},
		Auths: map[string]AuthEntry{
			"a": {Type: AuthTypeOAuth, ClientID: "c", Tenants: []string{"sandbox"}},
		},
		Tenant: TenantSelection{Default: "sandbox"},
	}
	res, err := DeleteTenantMetadata(cfg, "sandbox")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.DeletedAuths) != 1 || res.DeletedAuths[0] != "a" {
		t.Fatalf("deleted auths = %v", res.DeletedAuths)
	}
	if !res.ClearedDefault {
		t.Fatal("expected cleared default")
	}
	if len(cfg.Tenants) != 0 || len(cfg.Auths) != 0 {
		t.Fatalf("tenants=%d auths=%d", len(cfg.Tenants), len(cfg.Auths))
	}
}

func TestSetSelection(t *testing.T) {
	cfg := &Config{
		Tenants: map[string]TenantConfig{
			"a": {URL: "https://a.example.com"},
			"b": {URL: "https://b.example.com"},
		},
	}
	if err := SetSelection(cfg, SetSelectionOpts{DefaultTenant: "a"}); err != nil {
		t.Fatal(err)
	}
	if cfg.Tenant.Default != "a" {
		t.Fatalf("default = %q", cfg.Tenant.Default)
	}
	if err := SetSelection(cfg, SetSelectionOpts{Channel: ChannelMCP, ChannelTenant: "b"}); err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultTenant(ChannelMCP) != "b" {
		t.Fatalf("mcp default = %q", cfg.DefaultTenant(ChannelMCP))
	}
}
