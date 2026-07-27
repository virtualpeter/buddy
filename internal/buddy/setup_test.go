package buddy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"buddy/internal/buddy/secrets"
)

func TestWriteSetupScript(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "setup_buddy_role.buddy")
	if err := WriteSetupScript(path); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	script := string(b)

	mustContain := []string{
		"#!/usr/bin/env buddy",
		`role_name = "Buddy"`,
		`client_name = "buddy-cli"`,
		"roles()",
		"role_privileges()",
		"role_create(role_name, pl[\"value\"])",
		"client_create(client_name, [role_name])",
		"clients()",
		"buddy setup store-oauth",
		"buddy --user <auth-name> auth test",
		"buddy client credentials",
	}
	for _, s := range mustContain {
		if !strings.Contains(script, s) {
			t.Errorf("generated script missing %q", s)
		}
	}

	forbidden := []string{
		"role_create(cred,",
		"client_create(cred,",
		"roles(cred=cred)",
		"role_privileges(cred=cred)",
		"configure-oauth",
	}
	for _, s := range forbidden {
		if strings.Contains(script, s) {
			t.Errorf("generated script still has obsolete pattern %q", s)
		}
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0111 == 0 {
		t.Errorf("expected executable bits, got mode %v", info.Mode())
	}
}

func TestFindOAuthAuthForClient(t *testing.T) {
	cfg := &Config{
		Auths: map[string]AuthEntry{
			"other":   {Type: "oauth", ClientID: "cid-other", Tenants: []string{"sandbox"}},
			"sandbox": {Type: "oauth", ClientID: "cid-match", Tenants: []string{"sandbox", "prod"}},
			"basic":   {Type: "basic", Tenants: []string{"sandbox"}},
			"wrong-t": {Type: "oauth", ClientID: "cid-match", Tenants: []string{"prod"}},
		},
	}
	name, ok := findOAuthAuthForClient(cfg, "sandbox", "cid-match")
	if !ok || name != "sandbox" {
		t.Fatalf("got %q ok=%v, want sandbox", name, ok)
	}
	if _, ok := findOAuthAuthForClient(cfg, "sandbox", "missing"); ok {
		t.Fatal("expected no match")
	}
}

func TestDefaultOAuthAuthName(t *testing.T) {
	if got := defaultOAuthAuthName("sandbox"); got != "sandbox-oauth" {
		t.Fatalf("got %q", got)
	}
	if got := defaultOAuthAuthName(DefaultTenant); got != "buddy-oauth" {
		t.Fatalf("got %q", got)
	}
	if got := defaultOAuthAuthName(""); got != "buddy-oauth" {
		t.Fatalf("got %q", got)
	}
}

func TestSetupBasic(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	secrets.SetStore(&mockSecretStore{values: map[string]string{}})
	t.Cleanup(func() { secrets.SetStore(nil) })

	cfg := &Config{Tenant: TenantSelection{Default: DefaultTenant}}
	res, err := SetupBasic(cfg, BasicSetupOpts{
		TenantName:  "sandbox",
		URL:         "https://example.jamfcloud.com",
		AuthName:    "sandbox-admin",
		Username:    "admin",
		Password:    "s3cret",
		SkipConfirm: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res == nil || res.TenantName != "sandbox" || res.AuthName != "sandbox-admin" {
		t.Fatalf("got %+v", res)
	}
	if cfg.Tenant.Default != "sandbox" {
		t.Fatalf("default tenant %q", cfg.Tenant.Default)
	}
	tn, ok := cfg.Tenants["sandbox"]
	if !ok || tn.URL != "https://example.jamfcloud.com" {
		t.Fatalf("tenant %+v", tn)
	}
	au, ok := cfg.Auths["sandbox-admin"]
	if !ok || au.Type != "basic" || au.Username != "admin" || au.Surface != SurfaceInstance {
		t.Fatalf("auth %+v", au)
	}
	okHas, err := HasCredentialSecret("sandbox-admin", secrets.KindPassword)
	if err != nil || !okHas {
		t.Fatalf("password stored? %v %v", okHas, err)
	}
	// Reloaded from disk under XDG_CONFIG_HOME
	loaded, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := loaded.Tenants["sandbox"]; !ok {
		t.Fatal("tenant not persisted")
	}
}

func TestSetupBasicRequiresFlagsNonInteractive(t *testing.T) {
	// Without a TTY, missing fields must error (don't hang).
	cfg := &Config{}
	_, err := SetupBasic(cfg, BasicSetupOpts{})
	if err == nil {
		t.Fatal("expected error without tenant/url when non-interactive")
	}
}

