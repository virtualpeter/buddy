package buddy

import "testing"

func TestSessionSelectionForMCP(t *testing.T) {
	cfg := &Config{
		Tenant: TenantSelection{Default: "sandbox"},
		Channels: map[string]ChannelPolicy{
			ChannelMCP: {
				Tenant:       "prod",
				Access:       AccessReadonly,
				Tenants:      []string{"prod", "sandbox"},
				DenyBuiltins: []string{"jamf_*", "computer_erase"},
			},
		},
		Tenants: map[string]TenantConfig{
			"sandbox": {URL: "https://example.jamfcloud.com"},
			"prod":    {URL: "https://prod.example.jamfcloud.com"},
		},
		Auths: map[string]AuthEntry{
			"cli-only": {
				Type: "oauth", ClientID: "c0",
				Tenants:  []string{"sandbox"},
				Channels: []string{ChannelCLI},
			},
			"mcp-admin": {
				Type: "oauth", ClientID: "c1",
				Tenants:  []string{"sandbox", "prod"},
				Access:   "full",
				Surface:  "instance",
				Channels: []string{ChannelMCP, ChannelIDE},
			},
		},
	}
	SetActiveConfig(cfg)
	SetActiveSelection("", "")
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveSelection("", "")
	})

	got := SessionSelectionFor(ChannelMCP, "", "")
	if got.Channel != ChannelMCP {
		t.Fatalf("channel=%q", got.Channel)
	}
	if got.Tenant != "prod" || got.Context != "prod" {
		t.Fatalf("tenant=%q context=%q want prod", got.Tenant, got.Context)
	}
	if got.ChannelAccess != AccessReadonly {
		t.Fatalf("channel_access=%q", got.ChannelAccess)
	}
	if len(got.Auths) != 1 || got.Auths[0].Name != "mcp-admin" {
		t.Fatalf("auths=%+v want only mcp-admin", got.Auths)
	}
	if len(got.DenyBuiltins) != 2 || got.DenyBuiltins[0] != "jamf_*" || got.DenyBuiltins[1] != "computer_erase" {
		t.Fatalf("deny_builtins=%v", got.DenyBuiltins)
	}
	if len(got.AllowBuiltins) != 0 {
		t.Fatalf("allow_builtins=%v", got.AllowBuiltins)
	}
	info := got.InfoMap("1.2.3")
	if info["version"] != "1.2.3" || info["channel"] != ChannelMCP || info["channel_access"] != AccessReadonly {
		t.Fatalf("info=%v", info)
	}
	db, ok := info["deny_builtins"].([]string)
	if !ok || len(db) != 2 || db[0] != "jamf_*" {
		t.Fatalf("info deny_builtins=%v (%T)", info["deny_builtins"], info["deny_builtins"])
	}
}

func TestSessionSelectionMCPDefaultDeny(t *testing.T) {
	cfg := &Config{
		Tenant: TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{
			"sandbox": {URL: "https://example.jamfcloud.com"},
		},
	}
	SetActiveConfig(cfg)
	SetActiveSelection("", "")
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveSelection("", "")
	})
	got := SessionSelectionFor(ChannelMCP, "", "")
	if len(got.DenyBuiltins) != 1 || got.DenyBuiltins[0] != DefaultMCPDenyBuiltin {
		t.Fatalf("MCP default deny=%v", got.DenyBuiltins)
	}
	cli := SessionSelectionFor(ChannelCLI, "", "")
	if len(cli.DenyBuiltins) != 0 {
		t.Fatalf("CLI default deny=%v", cli.DenyBuiltins)
	}
}

func TestSessionSelectionTenantAuthBuiltinLists(t *testing.T) {
	cfg := &Config{
		Tenant: TenantSelection{Default: "prod"},
		Tenants: map[string]TenantConfig{
			"prod": {URL: "https://prod.example.jamfcloud.com", DenyBuiltins: []string{"*"}, AllowBuiltins: []string{"policies"}},
		},
		Auths: map[string]AuthEntry{
			"ro": {
				Type: "oauth", ClientID: "c1",
				Tenants:      []string{"prod"},
				DenyBuiltins: []string{"computer_erase"},
			},
		},
	}
	SetActiveConfig(cfg)
	SetActiveSelection("prod", "ro")
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveSelection("", "")
	})
	got := SessionSelectionFor(ChannelCLI, "", "")
	if len(got.TenantDenyBuiltins) != 1 || got.TenantDenyBuiltins[0] != "*" {
		t.Fatalf("tenant deny=%v", got.TenantDenyBuiltins)
	}
	if len(got.TenantAllowBuiltins) != 1 || got.TenantAllowBuiltins[0] != "policies" {
		t.Fatalf("tenant allow=%v", got.TenantAllowBuiltins)
	}
	if len(got.AuthDenyBuiltins) != 1 || got.AuthDenyBuiltins[0] != "computer_erase" {
		t.Fatalf("auth deny=%v", got.AuthDenyBuiltins)
	}
	info := got.InfoMap("1")
	td, ok := info["tenant_deny_builtins"].([]string)
	if !ok || len(td) != 1 || td[0] != "*" {
		t.Fatalf("info tenant_deny=%v", info["tenant_deny_builtins"])
	}
}
