package serve

import (
	"testing"

	"buddy/internal/buddy"
)

func TestSessionSelectionJSON(t *testing.T) {
	cfg := &buddy.Config{
		Tenant: buddy.TenantSelection{Default: "sandbox"},
		Tenants: map[string]buddy.TenantConfig{
			"sandbox": {URL: "https://example.jamfcloud.com"},
			"prod":    {URL: "https://prod.example.jamfcloud.com"},
		},
		Auths: map[string]buddy.AuthEntry{
			"sandbox-admin": {
				Type: "oauth", ClientID: "c1",
				Tenants: []string{"sandbox"}, Access: "full", Surface: "instance",
			},
			"platform-ro": {
				Type: "oauth", ClientID: "c2",
				Tenants: []string{"sandbox", "prod"}, Access: "readonly", Surface: "platform",
			},
		},
	}
	buddy.SetActiveConfig(cfg)
	buddy.SetActiveSelection("sandbox", "")
	t.Cleanup(func() {
		buddy.SetActiveConfig(nil)
		buddy.SetActiveSelection("", "")
	})

	got := sessionSelectionJSON(buddy.ChannelIDE, "", "")
	tenants, _ := got["tenants"].([]string)
	if len(tenants) != 2 || tenants[0] != "prod" || tenants[1] != "sandbox" {
		t.Fatalf("tenants=%v", tenants)
	}
	if got["tenant"] != "sandbox" {
		t.Fatalf("tenant=%v", got["tenant"])
	}
	if got["auth"] != "" {
		t.Fatalf("auth=%v want empty auto", got["auth"])
	}
	auths, ok := got["auths"].([]authInfo)
	if !ok || len(auths) != 2 {
		t.Fatalf("auths=%T %+v", got["auths"], got["auths"])
	}
	if auths[0].Name != "platform-ro" || auths[1].Name != "sandbox-admin" {
		t.Fatalf("auth names=%v %v", auths[0].Name, auths[1].Name)
	}

	got2 := sessionSelectionJSON(buddy.ChannelIDE, "prod", "platform-ro")
	if got2["tenant"] != "prod" || got2["auth"] != "platform-ro" {
		t.Fatalf("override got tenant=%v auth=%v", got2["tenant"], got2["auth"])
	}
}

func TestSessionSelectionJSONChannelPolicy(t *testing.T) {
	cfg := &buddy.Config{
		Tenant: buddy.TenantSelection{Default: "sandbox"},
		Channels: map[string]buddy.ChannelPolicy{
			buddy.ChannelMCP: {Tenant: "sandbox", Access: buddy.AccessReadonly},
			buddy.ChannelIDE: {Tenant: "prod"},
		},
		Tenants: map[string]buddy.TenantConfig{
			"sandbox": {URL: "https://example.jamfcloud.com"},
			"prod":    {URL: "https://prod.example.jamfcloud.com"},
		},
		Auths: map[string]buddy.AuthEntry{
			"sandbox-admin": {
				Type: "oauth", ClientID: "c1",
				Tenants:  []string{"sandbox"},
				Channels: []string{buddy.ChannelCLI},
			},
			"platform-ro": {
				Type: "oauth", ClientID: "c2",
				Tenants:  []string{"sandbox", "prod"},
				Access:   "readonly",
				Surface:  "platform",
				Channels: []string{buddy.ChannelMCP},
			},
		},
	}
	buddy.SetActiveConfig(cfg)
	buddy.SetActiveSelection("sandbox", "sandbox-admin")
	t.Cleanup(func() {
		buddy.SetActiveConfig(nil)
		buddy.SetActiveSelection("", "")
		buddy.SetActiveChannel("")
	})

	got := sessionSelectionJSON(buddy.ChannelMCP, "", "")
	if got["tenant"] != "sandbox" {
		t.Fatalf("tenant=%v", got["tenant"])
	}
	if got["channel_access"] != buddy.AccessReadonly {
		t.Fatalf("channel_access=%v", got["channel_access"])
	}
	auths, ok := got["auths"].([]authInfo)
	if !ok || len(auths) != 1 || auths[0].Name != "platform-ro" {
		t.Fatalf("auths=%+v", got["auths"])
	}
	if got["auth"] != "" {
		t.Fatalf("disallowed auth pin should be cleared, got %v", got["auth"])
	}

	buddy.SetActiveSelection("", "")
	ide := sessionSelectionJSON(buddy.ChannelIDE, "", "")
	if ide["tenant"] != "prod" {
		t.Fatalf("ide tenant=%v", ide["tenant"])
	}
}
