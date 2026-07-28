package buddy

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func sampleChannelConfig() *Config {
	return &Config{
		Tenant: TenantSelection{Default: "sandbox"},
		Channels: map[string]ChannelPolicy{
			ChannelCLI: {Tenant: "prod"},
			ChannelMCP: {Tenant: "sandbox", Access: AccessReadonly, Tenants: []string{"sandbox"}},
			ChannelIDE: {Tenant: "prod"},
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
				Channels: []string{ChannelCLI, ChannelIDE},
			},
			"sandbox-ro": {
				Type: "oauth", ClientID: "c2",
				Tenants:  []string{"sandbox", "prod"},
				Access:   AccessReadonly,
				Surface:  SurfaceInstance,
				Channels: []string{ChannelMCP, ChannelNotebook},
			},
		},
	}
}

func TestDefaultTenantFromChannels(t *testing.T) {
	cfg := sampleChannelConfig()
	if got := cfg.DefaultTenant(ChannelCLI); got != "prod" {
		t.Fatalf("cli=%q", got)
	}
	if got := cfg.DefaultTenant(ChannelMCP); got != "sandbox" {
		t.Fatalf("mcp=%q", got)
	}
	if got := cfg.DefaultTenant(ChannelIDE); got != "prod" {
		t.Fatalf("ide=%q", got)
	}
	if got := cfg.DefaultTenant(ChannelNotebook); got != "sandbox" {
		t.Fatalf("notebook fallback default=%q", got)
	}
	cfg.Channels = nil
	if got := cfg.DefaultTenant(ChannelCLI); got != "sandbox" {
		t.Fatalf("global=%q", got)
	}
}

func TestAuthAllowedAndFilter(t *testing.T) {
	cfg := sampleChannelConfig()
	if !cfg.AuthAllowed(ChannelMCP, "sandbox-ro") {
		t.Fatal("sandbox-ro should be allowed for mcp")
	}
	if cfg.AuthAllowed(ChannelMCP, "sandbox-admin") {
		t.Fatal("sandbox-admin should not be allowed for mcp")
	}
	if cfg.AuthAllowed(ChannelIDE, "sandbox-ro") {
		t.Fatal("ide should not share notebook/mcp-only auth")
	}
	if !cfg.AuthAllowed(ChannelIDE, "sandbox-admin") {
		t.Fatal("admin should be allowed for ide")
	}
	got := cfg.FilterAuths(ChannelNotebook, []string{"sandbox-admin", "sandbox-ro"})
	if len(got) != 1 || got[0] != "sandbox-ro" {
		t.Fatalf("filter notebook=%v", got)
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
	cfg := sampleChannelConfig()
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

func TestChannelReadonlyBlocksMutate(t *testing.T) {
	cfg := sampleChannelConfig()
	// Full auth allowed on mcp, but channel is readonly.
	cfg.Auths["sandbox-admin"] = AuthEntry{
		Type: "oauth", ClientID: "c1",
		Tenants:  []string{"sandbox"},
		Surface:  SurfaceInstance,
		Channels: []string{ChannelMCP, ChannelCLI},
		Access:   AccessFull,
	}
	SetActiveConfig(cfg)
	SetActiveChannel(ChannelMCP)
	SetActiveSelection("sandbox", "sandbox-admin")
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveSelection("", "")
		SetActiveChannel("")
	})
	_, err := cfg.ResolveSession("", "sandbox-admin", OpMutate)
	if err == nil || !strings.Contains(err.Error(), "channel mcp is readonly") {
		t.Fatalf("want channel readonly error, got %v", err)
	}
	SetActiveChannel(ChannelCLI)
	if _, err := cfg.ResolveSession("sandbox", "sandbox-admin", OpMutate); err != nil {
		t.Fatalf("cli mutate should work: %v", err)
	}
}

func TestChannelTenantAllowlist(t *testing.T) {
	cfg := sampleChannelConfig()
	SetActiveConfig(cfg)
	SetActiveChannel(ChannelMCP)
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveSelection("", "")
		SetActiveChannel("")
	})
	_, err := cfg.ResolveSession("prod", "sandbox-ro", OpRead)
	if err == nil || !strings.Contains(err.Error(), "not allowed for mcp") {
		t.Fatalf("prod should be blocked on mcp: %v", err)
	}
}

func TestValidateTenantSelectionAndChannels(t *testing.T) {
	cfg := sampleChannelConfig()
	if err := cfg.ValidateTenantSelectionAndChannels(); err != nil {
		t.Fatal(err)
	}
	cfg.Channels[ChannelCLI] = ChannelPolicy{Tenant: "missing"}
	if err := cfg.ValidateTenantSelectionAndChannels(); err == nil || !strings.Contains(err.Error(), "unknown tenant") {
		t.Fatalf("err=%v", err)
	}
	cfg = sampleChannelConfig()
	a := cfg.Auths["sandbox-ro"]
	a.Channels = []string{"nope"}
	cfg.Auths["sandbox-ro"] = a
	if err := cfg.ValidateTenantSelectionAndChannels(); err == nil || !strings.Contains(err.Error(), "unknown channel") {
		t.Fatalf("err=%v", err)
	}
	cfg = sampleChannelConfig()
	a = cfg.Auths["sandbox-ro"]
	a.Channels = []string{"serve"}
	cfg.Auths["sandbox-ro"] = a
	if err := cfg.ValidateTenantSelectionAndChannels(); err == nil || !strings.Contains(err.Error(), "removed") {
		t.Fatalf("serve reject err=%v", err)
	}
}

func TestMarshalConfig_Channels(t *testing.T) {
	cfg := sampleChannelConfig()
	data, err := marshalConfig(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{"tenant:", "default: sandbox", "channels:", "mcp:", "access: readonly", "cli:"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in:\n%s", want, s)
		}
	}
	if strings.Contains(s, "defaults:") || strings.Contains(s, "allow_auths:") {
		t.Fatalf("legacy keys should not be written:\n%s", s)
	}
	if strings.Contains(s, "\n  cli: prod\n") && strings.Contains(s, "tenant:\n") {
		// ensure per-channel tenant is under channels, not tenant.cli
	}
	var round Config
	if err := yaml.Unmarshal(data, &round); err != nil {
		t.Fatal(err)
	}
	if round.Tenant.Default != "sandbox" {
		t.Fatalf("tenant=%+v", round.Tenant)
	}
	if round.Channels[ChannelCLI].Tenant != "prod" || round.Channels[ChannelMCP].EffectiveAccess() != AccessReadonly {
		t.Fatalf("channels=%+v", round.Channels)
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
	if cfg.Channels[ChannelCLI].Tenant != "prod" || cfg.Channels[ChannelIDE].Tenant != "prod" {
		t.Fatalf("channels=%+v", cfg.Channels)
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

func TestTenantLegacyKeysRejected(t *testing.T) {
	var tsel TenantSelection
	err := yaml.Unmarshal([]byte("mcp: sandbox\ndefault: sandbox\n"), &tsel)
	if err == nil || !strings.Contains(err.Error(), "no longer supported") {
		t.Fatalf("err=%v", err)
	}
}

func TestParseAuthChannels(t *testing.T) {
	got, err := ParseAuthChannels("cli,mcp,ide,notebook")
	if err != nil || got != nil {
		t.Fatalf("all four should clear: %v %v", got, err)
	}
	_, err = ParseAuthChannels("serve")
	if err == nil || !strings.Contains(err.Error(), "removed") {
		t.Fatalf("err=%v", err)
	}
	got, err = ParseAuthChannels("mcp,notebook")
	if err != nil || len(got) != 2 {
		t.Fatalf("got=%v err=%v", got, err)
	}
}
