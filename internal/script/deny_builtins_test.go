package script

import (
	"log/slog"
	"strings"
	"testing"

	"buddy/internal/buddy"
)

func TestDenyBuiltinsRaisesOnCall(t *testing.T) {
	cfg := &buddy.Config{
		Tenant: buddy.TenantSelection{Default: "sandbox"},
		Channels: map[string]buddy.ChannelPolicy{
			buddy.ChannelMCP: {DenyBuiltins: []string{"jamf_*", "uuid_new"}},
		},
		Tenants: map[string]buddy.TenantConfig{
			"sandbox": {URL: "https://example.jamfcloud.com"},
		},
	}
	buddy.SetActiveConfig(cfg)
	prev := buddy.ActiveChannel()
	buddy.SetActiveChannel(buddy.ChannelMCP)
	t.Cleanup(func() {
		buddy.SetActiveConfig(nil)
		buddy.SetActiveChannel(prev)
	})

	s, err := NewSession("", buddy.Opts{SkipConfirm: true}, slog.Default(), false)
	if err != nil {
		t.Fatal(err)
	}
	res := s.Eval("deny", `jamf_put("/api/v1/x", {})`)
	if res.Error == "" || !strings.Contains(res.Error, "deny_builtins") || !strings.Contains(res.Error, "jamf_put") {
		t.Fatalf("expected deny error, got %#v", res)
	}

	// Offline builtin not denied should still work.
	res = s.Eval("ok", `version_compare("1.0", "1.1")`)
	if res.Error != "" {
		t.Fatalf("version_compare should work: %#v", res)
	}

	// Exact deny.
	res = s.Eval("uuid", `uuid_new()`)
	if res.Error == "" || !strings.Contains(res.Error, "uuid_new") {
		t.Fatalf("expected uuid_new denied: %#v", res)
	}
}

func TestScriptPutRejectsScriptIDKwarg(t *testing.T) {
	s, err := NewSession("", buddy.Opts{SkipConfirm: true}, slog.Default(), false)
	if err != nil {
		t.Fatal(err)
	}
	res := s.Eval("oldkw", `script_put(script_id="1", dict={})`)
	if res.Error == "" || !strings.Contains(res.Error, "unexpected keyword argument") || !strings.Contains(res.Error, "script_id") {
		t.Fatalf("expected unknown script_id kwarg, got %#v", res)
	}
}
