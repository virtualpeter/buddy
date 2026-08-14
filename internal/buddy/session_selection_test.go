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
	if got.Tenant != "prod" {
		t.Fatalf("tenant=%q want prod", got.Tenant)
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
	info := got.InfoMap("1.2.3")
	if info["version"] != "1.2.3" || info["channel"] != ChannelMCP || info["channel_access"] != AccessReadonly {
		t.Fatalf("info=%v", info)
	}
	db, ok := info["deny_builtins"].([]string)
	if !ok || len(db) != 2 || db[0] != "jamf_*" {
		t.Fatalf("info deny_builtins=%v (%T)", info["deny_builtins"], info["deny_builtins"])
	}
}
