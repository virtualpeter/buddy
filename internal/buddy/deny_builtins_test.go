package buddy

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestBuiltinNameDenied(t *testing.T) {
	patterns := []string{"jamf_*", "computer_erase", "uuid_new"}
	if !BuiltinNameDenied(patterns, "jamf_put") {
		t.Fatal("jamf_* should match jamf_put")
	}
	if !BuiltinNameDenied(patterns, "jamf_get") {
		t.Fatal("jamf_* should match jamf_get")
	}
	if !BuiltinNameDenied(patterns, "computer_erase") {
		t.Fatal("exact computer_erase")
	}
	if BuiltinNameDenied(patterns, "jamf") {
		t.Fatal("jamf alone should not match jamf_*")
	}
	if BuiltinNameDenied(patterns, "computer_reboot") {
		t.Fatal("computer_reboot should not match")
	}
	if BuiltinNameDenied(nil, "jamf_put") {
		t.Fatal("empty patterns deny nothing")
	}
	if !BuiltinNameDenied([]string{"*"}, "uuid_new") {
		t.Fatal("deny * should match every name")
	}
}

func TestNormalizeDenyBuiltins(t *testing.T) {
	got, err := NormalizeDenyBuiltins([]string{" jamf_* ", "computer_erase", "jamf_*", ""})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "jamf_*" || got[1] != "computer_erase" {
		t.Fatalf("got %#v", got)
	}
	star, err := NormalizeDenyBuiltins([]string{"*"})
	if err != nil {
		t.Fatal(err)
	}
	if len(star) != 1 || star[0] != "*" {
		t.Fatalf("deny * got %#v", star)
	}
	if _, err := NormalizeDenyBuiltins([]string{"jamf*put"}); err == nil {
		t.Fatal("expected error for mid-string *")
	}
}

func TestNormalizeAllowBuiltins(t *testing.T) {
	got, err := NormalizeAllowBuiltins([]string{" * ", "jamf_*", "*"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "*" || got[1] != "jamf_*" {
		t.Fatalf("got %#v", got)
	}
	if _, err := NormalizeAllowBuiltins([]string{"jamf*put"}); err == nil {
		t.Fatal("expected error for mid-string *")
	}
}

func TestBuiltinNameBlockedAllowFollowsDeny(t *testing.T) {
	deny := []string{"jamf_*", "computer_erase"}
	if !BuiltinNameBlocked(deny, nil, "jamf_put") {
		t.Fatal("jamf_put should be blocked without allow")
	}
	if BuiltinNameBlocked(deny, []string{"jamf_*"}, "jamf_put") {
		t.Fatal("allow jamf_* should unblock jamf_put")
	}
	if !BuiltinNameBlocked(deny, []string{"jamf_*"}, "computer_erase") {
		t.Fatal("allow jamf_* should not unblock computer_erase")
	}
	if BuiltinNameBlocked(deny, []string{"*"}, "computer_erase") {
		t.Fatal("allow * should unblock everything")
	}
	if BuiltinNameBlocked(deny, []string{"*"}, "jamf_put") {
		t.Fatal("allow * should unblock jamf_put")
	}
}

func TestDenyStarAllowlist(t *testing.T) {
	deny := []string{"*"}
	allow := []string{"policies", "policy", "uuid_new"}
	if !BuiltinNameBlocked(deny, allow, "jamf_put") {
		t.Fatal("deny * should block jamf_put unless allowed")
	}
	if BuiltinNameBlocked(deny, allow, "uuid_new") {
		t.Fatal("allowlist should unblock uuid_new")
	}
	if BuiltinNameBlocked(deny, allow, "policies") {
		t.Fatal("allowlist should unblock policies")
	}
}

func TestEffectiveDenyBuiltinsMCPDefault(t *testing.T) {
	got := EffectiveDenyBuiltins(nil, ChannelMCP)
	if len(got) != 1 || got[0] != DefaultMCPDenyBuiltin {
		t.Fatalf("nil cfg MCP deny=%v want [%s]", got, DefaultMCPDenyBuiltin)
	}
	if got := EffectiveDenyBuiltins(nil, ChannelCLI); len(got) != 0 {
		t.Fatalf("CLI should have no default deny, got %v", got)
	}

	cfg := &Config{
		Channels: map[string]ChannelPolicy{
			ChannelMCP: {DenyBuiltins: []string{"computer_erase"}},
			ChannelCLI: {DenyBuiltins: []string{"uuid_new"}},
		},
	}
	mcp := EffectiveDenyBuiltins(cfg, ChannelMCP)
	if len(mcp) != 2 || mcp[0] != "jamf_*" || mcp[1] != "computer_erase" {
		t.Fatalf("MCP union=%v", mcp)
	}
	cli := EffectiveDenyBuiltins(cfg, ChannelCLI)
	if len(cli) != 1 || cli[0] != "uuid_new" {
		t.Fatalf("CLI deny=%v", cli)
	}

	cfg.Channels[ChannelMCP] = ChannelPolicy{DenyBuiltins: []string{"jamf_*", "computer_erase"}}
	mcp = EffectiveDenyBuiltins(cfg, ChannelMCP)
	if len(mcp) != 2 || mcp[0] != "jamf_*" || mcp[1] != "computer_erase" {
		t.Fatalf("MCP dedupe=%v", mcp)
	}
}

func TestAllowBuiltinsYAMLScalar(t *testing.T) {
	var cfg Config
	if err := yaml.Unmarshal([]byte(`
channels:
  mcp:
    deny_builtins: computer_erase
    allow_builtins: "*"
`), &cfg); err != nil {
		t.Fatal(err)
	}
	p := cfg.Channels[ChannelMCP]
	if len(p.DenyBuiltins) != 1 || p.DenyBuiltins[0] != "computer_erase" {
		t.Fatalf("deny=%v", p.DenyBuiltins)
	}
	if len(p.AllowBuiltins) != 1 || p.AllowBuiltins[0] != "*" {
		t.Fatalf("allow=%v", p.AllowBuiltins)
	}

	var cfgList Config
	if err := yaml.Unmarshal([]byte(`
channels:
  mcp:
    deny_builtins: [jamf_*, computer_erase]
    allow_builtins: ["*"]
`), &cfgList); err != nil {
		t.Fatal(err)
	}
	p = cfgList.Channels[ChannelMCP]
	if len(p.DenyBuiltins) != 2 || p.DenyBuiltins[0] != "jamf_*" || p.DenyBuiltins[1] != "computer_erase" {
		t.Fatalf("list deny=%v", p.DenyBuiltins)
	}
	if len(p.AllowBuiltins) != 1 || p.AllowBuiltins[0] != "*" {
		t.Fatalf("list allow=%v", p.AllowBuiltins)
	}
}

func TestErrBuiltinDenied(t *testing.T) {
	err := ErrBuiltinDenied(ChannelMCP, "jamf_put")
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{"jamf_put", "mcp", "deny_builtins"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("msg %q missing %q", msg, want)
		}
	}
}

func TestLayerTenantDenyNotUnblockedByChannelAllow(t *testing.T) {
	cfg := &Config{
		Channels: map[string]ChannelPolicy{
			ChannelCLI: {AllowBuiltins: []string{"*"}},
		},
		Tenants: map[string]TenantConfig{
			"prod": {URL: "https://prod.example.jamfcloud.com", DenyBuiltins: []string{"uuid_new"}},
		},
	}
	SetActiveConfig(cfg)
	SetActiveChannel(ChannelCLI)
	SetActiveSelection("prod", "")
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveChannel("")
		SetActiveSelection("", "")
	})
	err := CheckBuiltinAllowed("uuid_new", "", "", "", "")
	if err == nil || !strings.Contains(err.Error(), "tenants.prod") {
		t.Fatalf("channel allow must not unblock tenant deny, got %v", err)
	}
	if err := CheckBuiltinAllowed("version_compare", "", "", "", ""); err != nil {
		t.Fatalf("unrelated builtin: %v", err)
	}
}

func TestKwargTenantSwitchesLayer(t *testing.T) {
	cfg := &Config{
		Tenant: TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{
			"sandbox": {URL: "https://sandbox.example.jamfcloud.com"},
			"prod":    {URL: "https://prod.example.jamfcloud.com", DenyBuiltins: []string{"*"}, AllowBuiltins: []string{"policies"}},
		},
	}
	SetActiveConfig(cfg)
	SetActiveChannel(ChannelCLI)
	SetActiveSelection("sandbox", "")
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveChannel("")
		SetActiveSelection("", "")
	})
	if err := CheckBuiltinAllowed("jamf_put", "", "", "", ""); err != nil {
		t.Fatalf("sandbox should allow jamf_put: %v", err)
	}
	err := CheckBuiltinAllowed("jamf_put", "prod", "", "", "")
	if err == nil || !strings.Contains(err.Error(), "tenants.prod") {
		t.Fatalf("prod deny * should block jamf_put, got %v", err)
	}
	if err := CheckBuiltinAllowed("policies", "prod", "", "", ""); err != nil {
		t.Fatalf("prod allowlist policies: %v", err)
	}
	err = CheckBuiltinAllowed("policy_copy", "", "", "prod", "")
	if err == nil || !strings.Contains(err.Error(), "tenants.prod") {
		t.Fatalf("to_tenant=prod should block policy_copy, got %v", err)
	}
}

func TestMarshalAllowStarQuoted(t *testing.T) {
	cfg := &Config{
		Channels: map[string]ChannelPolicy{
			ChannelMCP: {AllowBuiltins: []string{"*"}},
		},
	}
	data, err := marshalConfig(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "allow_builtins:") || !strings.Contains(s, `"*"`) {
		t.Fatalf("expected allow_builtins list with quoted star, got:\n%s", s)
	}
	if strings.Contains(s, `allow_builtins: "*"`) {
		t.Fatalf("allow_builtins should marshal as a list, got:\n%s", s)
	}
}

func TestTenantAuthBuiltinListsYAML(t *testing.T) {
	var cfg Config
	if err := yaml.Unmarshal([]byte(`
tenants:
  prod:
    url: https://prod.example.jamfcloud.com
    deny_builtins: ["*"]
    allow_builtins: [policies, policy]
auths:
  mcp-ro:
    type: oauth
    clientId: c1
    tenants: [prod]
    deny_builtins: computer_erase
`), &cfg); err != nil {
		t.Fatal(err)
	}
	tnt := cfg.Tenants["prod"]
	if len(tnt.DenyBuiltins) != 1 || tnt.DenyBuiltins[0] != "*" {
		t.Fatalf("tenant deny=%v", tnt.DenyBuiltins)
	}
	if len(tnt.AllowBuiltins) != 2 || tnt.AllowBuiltins[0] != "policies" {
		t.Fatalf("tenant allow=%v", tnt.AllowBuiltins)
	}
	a := cfg.Auths["mcp-ro"]
	if len(a.DenyBuiltins) != 1 || a.DenyBuiltins[0] != "computer_erase" {
		t.Fatalf("auth deny=%v", a.DenyBuiltins)
	}

	cfg.Tenant.Default = "prod"
	data, err := marshalConfig(&cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "deny_builtins:") || !strings.Contains(s, `"*"`) {
		t.Fatalf("expected quoted deny * in tenant marshal, got:\n%s", s)
	}
}
