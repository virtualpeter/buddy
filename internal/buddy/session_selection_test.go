package buddy

import "testing"

func TestSessionSelectionForMCP(t *testing.T) {
	cfg := &Config{
		Tenant: TenantSelection{
			Default: "sandbox",
			MCP:     "prod",
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
				Channels: []string{ChannelMCP, ChannelServe},
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
		t.Fatalf("tenant=%q want prod (mcp override)", got.Tenant)
	}
	if len(got.Auths) != 1 || got.Auths[0].Name != "mcp-admin" {
		t.Fatalf("auths=%+v want only mcp-admin", got.Auths)
	}
	info := got.InfoMap("1.2.3")
	if info["version"] != "1.2.3" || info["channel"] != ChannelMCP {
		t.Fatalf("info=%v", info)
	}
}
