package main

import (
	"io"
	"os"
	"strings"
	"testing"

	"buddy/internal/buddy"
	"buddy/internal/clispec"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()
	fn()
	_ = w.Close()
	b, err := io.ReadAll(r)
	_ = r.Close()
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRunCredBarePrintsShortHelp(t *testing.T) {
	out := captureStdout(t, func() {
		if err := runCred(nil, buddy.Opts{}, nil, nil); err != nil {
			t.Fatalf("runCred: %v", err)
		}
	})
	if out != helpCredShort {
		t.Fatalf("bare cred should print CredShort, got %d bytes", len(out))
	}
	if strings.Contains(out, "Flags for 'status'") {
		t.Fatal("short help should not include flag encyclopedia")
	}
}

func TestRunCredHelpPrintsFull(t *testing.T) {
	out := captureStdout(t, func() {
		if err := runCred(nil, buddy.Opts{}, []string{clispec.SubHelp}, nil); err != nil {
			t.Fatalf("runCred: %v", err)
		}
	})
	if out != helpCred {
		t.Fatalf("cred help should print full Cred, got %d bytes", len(out))
	}
	if !strings.Contains(out, "Flags for 'status'") {
		t.Fatal("full help should include flags")
	}
}

func TestRunCredUnknownSubcommandListsSetupFirst(t *testing.T) {
	err := runCred(nil, buddy.Opts{}, []string{"nope"}, nil)
	if err == nil {
		t.Fatal("expected unknown subcommand")
	}
	msg := err.Error()
	add := strings.Index(msg, clispec.SubAdd)
	set := strings.Index(msg, clispec.SubSet)
	list := strings.Index(msg, clispec.SubList)
	if add < 0 || set < 0 || list < 0 || !(add < set && set < list) {
		t.Fatalf("want add, set before list in candidates: %s", msg)
	}
}

func TestRunCredSetChannelTenant(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	cfg := &buddy.Config{
		Tenants: map[string]buddy.TenantConfig{"sandbox": {URL: "https://example.jamfcloud.com"}},
		Tenant:  buddy.TenantSelection{Default: "sandbox"},
	}
	out := captureStdout(t, func() {
		if err := runCredSetChannel(cfg, []string{"mcp", "--tenant=sandbox"}); err != nil {
			t.Fatalf("runCredSetChannel: %v", err)
		}
	})
	if cfg.Channels["mcp"].Tenant != "sandbox" {
		t.Fatalf("channels.mcp.tenant = %q, want sandbox; out=%s", cfg.Channels["mcp"].Tenant, out)
	}
	out = captureStdout(t, func() {
		if err := runCredSetChannel(cfg, []string{"cli", "--tenant", "sandbox"}); err != nil {
			t.Fatalf("runCredSetChannel space: %v", err)
		}
	})
	if cfg.Channels["cli"].Tenant != "sandbox" {
		t.Fatalf("channels.cli.tenant = %q; out=%s", cfg.Channels["cli"].Tenant, out)
	}
}
