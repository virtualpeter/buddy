package buddy

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func sampleDefaultsConfig() *Config {
	return &Config{
		Tenant: TenantSelection{
			Default: "sandbox",
			CLI:     "prod",
			MCP:     "sandbox",
			Serve:   "sandbox",
			IDE:     "prod",
		},
		Tenants: map[string]TenantConfig{
			"sandbox": {URL: "https://example.jamfcloud.com"},
			"prod":    {URL: "https://prod.example.jamfcloud.com"},
		},
		Auths: map[string]AuthEntry{
			"sandbox-admin": {
				Type: "oauth", ClientID: "c1",
				Tenants:  []string{"sandbox", "prod"},
				Surface:  SurfaceInstance,
				Channels: []string{ChannelCLI, ChannelServe},
			},
			"sandbox-ro": {
				Type: "oauth", ClientID: "c2",
				Tenants:  []string{"sandbox", "prod"},
				Access:   AccessReadonly,
				Surface:  SurfaceInstance,
				Channels: []string{ChannelMCP, ChannelServe},
			},
		},
	}
}

func TestDefaultTenantFallback(t *testing.T) {
	cfg := sampleDefaultsConfig()
	if got := cfg.DefaultTenant(ChannelCLI); got != "prod" {
		t.Fatalf("cli=%q", got)
	}
	if got := cfg.DefaultTenant(ChannelMCP); got != "sandbox" {
		t.Fatalf("mcp=%q", got)
	}
	if got := cfg.DefaultTenant(ChannelIDE); got != "prod" {
		t.Fatalf("ide=%q", got)
	}
	cfg.Tenant.IDE = ""
	cfg.Tenant.Serve = "sandbox"
	if got := cfg.DefaultTenant(ChannelIDE); got != "sandbox" {
		t.Fatalf("ide fallback to serve=%q", got)
	}
	cfg.Tenant = TenantSelection{Default: "sandbox"}
	if got := cfg.DefaultTenant(ChannelCLI); got != "sandbox" {
		t.Fatalf("global=%q", got)
	}
}

func TestAuthAllowedAndFilter(t *testing.T) {
	cfg := sampleDefaultsConfig()
	if !cfg.AuthAllowed(ChannelMCP, "sandbox-ro") {
		t.Fatal("sandbox-ro should be allowed for mcp")
	}
	if cfg.AuthAllowed(ChannelMCP, "sandbox-admin") {
		t.Fatal("sandbox-admin should not be allowed for mcp")
	}
	if !cfg.AuthAllowed(ChannelIDE, "sandbox-ro") {
		t.Fatal("ide shares serve channel")
	}
	got := cfg.FilterAuths(ChannelServe, []string{"sandbox-admin", "sandbox-ro"})
	if len(got) != 2 {
		t.Fatalf("filter serve=%v", got)
	}
	got = cfg.FilterAuths(ChannelMCP, []string{"sandbox-admin", "sandbox-ro"})
	if len(got) != 1 || got[0] != "sandbox-ro" {
		t.Fatalf("filter mcp=%v", got)
	}
	cfg.Auths["sandbox-admin"] = AuthEntry{
		Type: "oauth", ClientID: "c1",
		Tenants: []string{"sandbox"}, Surface: SurfaceInstance,
	}
	if !cfg.AuthAllowed(ChannelMCP, "sandbox-admin") {
		t.Fatal("empty channels = all")
	}
}

func TestResolveSessionAllowlist(t *testing.T) {
	cfg := sampleDefaultsConfig()
	SetActiveConfig(cfg)
	SetActiveChannel(ChannelMCP)
	SetActiveSelection("sandbox", "")
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveSelection("", "")
		SetActiveChannel("")
	})

	sess, err := cfg.ResolveSession("", "", OpRead)
	if err != nil {
		t.Fatalf("auto-pick: %v", err)
	}
	if sess.AuthName != "sandbox-ro" {
		t.Fatalf("auth=%q want sandbox-ro", sess.AuthName)
	}

	_, err = cfg.ResolveSession("", "sandbox-admin", OpAny)
	if err == nil || !strings.Contains(err.Error(), "not allowed for mcp") {
		t.Fatalf("pin reject: %v", err)
	}
}

func TestValidateTenantSelectionAndChannels(t *testing.T) {
	cfg := sampleDefaultsConfig()
	if err := cfg.ValidateTenantSelectionAndChannels(); err != nil {
		t.Fatal(err)
	}
	cfg.Tenant.CLI = "missing"
	if err := cfg.ValidateTenantSelectionAndChannels(); err == nil || !strings.Contains(err.Error(), "unknown tenant") {
		t.Fatalf("err=%v", err)
	}
	cfg = sampleDefaultsConfig()
	a := cfg.Auths["sandbox-ro"]
	a.Channels = []string{"nope"}
	cfg.Auths["sandbox-ro"] = a
	if err := cfg.ValidateTenantSelectionAndChannels(); err == nil || !strings.Contains(err.Error(), "unknown channel") {
		t.Fatalf("err=%v", err)
	}
}

func TestMarshalConfig_TenantObjectAndChannels(t *testing.T) {
	cfg := sampleDefaultsConfig()
	data, err := marshalConfig(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{"tenant:", "default: sandbox", "cli: prod", "ide: prod", "channels:", "mcp"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in:\n%s", want, s)
		}
	}
	if strings.Contains(s, "defaults:") || strings.Contains(s, "allow_auths:") {
		t.Fatalf("legacy keys should not be written:\n%s", s)
	}
	var round Config
	if err := yaml.Unmarshal(data, &round); err != nil {
		t.Fatal(err)
	}
	if round.Tenant.Default != "sandbox" || round.Tenant.CLI != "prod" || round.Tenant.IDE != "prod" {
		t.Fatalf("tenant=%+v", round.Tenant)
	}
	if len(round.Auths["sandbox-ro"].Channels) == 0 {
		t.Fatalf("channels not preserved: %+v", round.Auths["sandbox-ro"])
	}
}

func TestMigrateLegacyDefaultsAllowAuths(t *testing.T) {
	cfg := &Config{
		Tenant: TenantSelection{Default: "sandbox"},
		Defaults: map[string]string{
			"cli": "prod",
			"ide": "prod",
		},
		AllowAuths: map[string][]string{
			"mcp": {"sandbox-ro"},
		},
		Tenants: map[string]TenantConfig{
			"sandbox": {URL: "https://example.jamfcloud.com"},
			"prod":    {URL: "https://prod.example.jamfcloud.com"},
		},
		Auths: map[string]AuthEntry{
			"sandbox-admin": {Type: "oauth", ClientID: "c1", Tenants: []string{"sandbox"}},
			"sandbox-ro":    {Type: "oauth", ClientID: "c2", Tenants: []string{"sandbox"}},
		},
	}
	if !migrateLegacyTenantDefaultsAllow(cfg, true) {
		t.Fatal("expected migration")
	}
	if cfg.Tenant.CLI != "prod" || cfg.Tenant.IDE != "prod" {
		t.Fatalf("tenant=%+v", cfg.Tenant)
	}
	if cfg.AuthAllowed(ChannelMCP, "sandbox-admin") {
		t.Fatal("admin should lose mcp after invert")
	}
	if !cfg.AuthAllowed(ChannelMCP, "sandbox-ro") {
		t.Fatal("ro should keep mcp")
	}
	if !cfg.AuthAllowed(ChannelCLI, "sandbox-admin") {
		t.Fatal("cli unrestricted → admin kept")
	}
}
