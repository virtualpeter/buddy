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

func TestParseCredAddPaste(t *testing.T) {
	got, err := parseCredAddArgs([]string{"--paste", "--context=sandbox", "buddy-cli"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !got.WantPaste || got.TenantName != "sandbox" || got.AuthName != "buddy-cli" {
		t.Fatalf("%+v", got)
	}
	pasteJSON := `{"client_name":"buddy-cli","client_id":"11111111-1111-1111-1111-111111111111","client_secret":"instance-secret"}`
	got, err = parseCredAddArgs([]string{"--paste=" + pasteJSON, "--context=sandbox"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !got.WantPaste || !strings.Contains(got.PasteBlob, "client_id") || !got.SkipConfirm {
		t.Fatalf("%+v", got)
	}
	got, err = parseCredAddArgs([]string{"--paste=-", "-y"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !got.WantPaste || got.PasteBlob != "" || !got.SkipConfirm {
		t.Fatalf("stdin paste: %+v", got)
	}
	got, err = parseCredAddArgs([]string{"--from=./quickstart.yaml", "--org=acme", "--environment-name=commercial"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if got.FromFile != "./quickstart.yaml" || got.OrgName != "acme" || got.EnvironmentName != "commercial" || !got.SkipConfirm {
		t.Fatalf("from/org flags: %+v", got)
	}
}

func TestRunCredListWorkplaces(t *testing.T) {
	cfg := &buddy.Config{
		Tenant: buddy.TenantSelection{Default: "prod"},
		Tenants: map[string]buddy.TenantConfig{
			"prod": {
				URL:           "https://prod.example.jamfcloud.com",
				Region:        "us",
				EnvironmentID: "11111111-1111-1111-1111-111111111111",
				TenantID:      "22222222-2222-2222-2222-222222222222",
				Org:           "acme",
			},
			"lab": {
				URL:           "https://lab.example.jamfcloud.com",
				Region:        "us",
				EnvironmentID: "33333333-3333-3333-3333-333333333333",
				TenantID:      "44444444-4444-4444-4444-444444444444",
				Org:           "acme",
			},
			"commercial": {Region: "us", EnvironmentID: "11111111-1111-1111-1111-111111111111", Org: "acme"},
			"staging":    {Region: "us", EnvironmentID: "33333333-3333-3333-3333-333333333333", Org: "acme"},
			"acme":       {},
			"jamf":       {URL: "https://example.jamfcloud.com"},
		},
		Auths: map[string]buddy.AuthEntry{
			"inst": {Type: "oauth", ClientID: "cid", Tenants: []string{"jamf"}},
			"plat": {Type: "platform", ClientID: "cid", Tenants: []string{"commercial"}, Scope: "environment"},
		},
	}
	out := captureStdout(t, func() {
		if err := runCredList(cfg); err != nil {
			t.Fatal(err)
		}
	})
	for _, want := range []string{"ORG", "ENVIRONMENT", "CONTEXT", "organization", "environment", "product-tenant", "instance", "SCOPE", "platform"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "11111111") || strings.Contains(out, "X-Environment-Id") {
		t.Fatalf("list should not print Account UUIDs:\n%s", out)
	}
	got := buddy.WorkplaceTreeOrder(cfg.Tenants)
	want := []string{"acme", "commercial", "prod", "staging", "lab", "jamf"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("tree order got %v want %v", got, want)
	}
}

func TestRunCredShowContextAndAuth(t *testing.T) {
	cfg := &buddy.Config{
		Tenants: map[string]buddy.TenantConfig{
			"commercial": {Region: "us", EnvironmentID: "11111111-1111-1111-1111-111111111111", Org: "acme"},
		},
		Auths: map[string]buddy.AuthEntry{
			"plat": {Type: "platform", ClientID: "cid", Tenants: []string{"commercial"}, Scope: "environment"},
		},
	}
	out := captureStdout(t, func() {
		if err := runCredShowTenant(cfg, "commercial"); err != nil {
			t.Fatal(err)
		}
	})
	for _, want := range []string{"context:", "kind:", "environment", "org:", "acme", "X-Environment-Id"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in get context:\n%s", want, out)
		}
	}
	out = captureStdout(t, func() {
		if err := runCredShowAuth(cfg, "plat"); err != nil {
			t.Fatal(err)
		}
	})
	for _, want := range []string{"scope:", "environment", "contexts:", "commercial", "X-Environment-Id"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in get auth:\n%s", want, out)
		}
	}
}
