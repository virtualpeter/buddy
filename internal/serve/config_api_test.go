package serve

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"buddy/internal/buddy"
	"buddy/internal/buddy/agent"
	"buddy/internal/buddy/secrets"
)

func withTempBuddyHome(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	// An empty config discovers a running Agent. The test binary is unsigned,
	// so a signed Agent refuses it and LoadConfig fails the GET with 503.
	t.Setenv(agent.EnvDisable, "1")
	cfg := &buddy.Config{}
	buddy.SetActiveConfig(cfg)
}

func TestConfigAPIRoundTrip(t *testing.T) {
	withTempBuddyHome(t)

	rec := httptest.NewRecorder()
	handleConfigAPI(rec, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status %d", rec.Code)
	}
	var snap buddy.ConfigSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatal(err)
	}
	if !snap.NeedsSetup {
		t.Fatal("expected needs_setup")
	}

	body := `{"url":"https://example.jamfcloud.com"}`
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/config/context/sandbox", bytes.NewBufferString(body))
	handleConfigAPI(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT tenant status %d body %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatal(err)
	}
	if snap.NeedsSetup || len(snap.Auths) != 0 {
		t.Fatalf("after tenant: %+v", snap)
	}
	found := false
	for _, tnt := range snap.Tenants {
		if tnt.Name == "sandbox" {
			found = true
		}
	}
	if !found {
		t.Fatalf("sandbox missing after PUT: %+v", snap)
	}

	authBody := `{"type":"oauth","tenants":["sandbox"],"client_id":"cid","access":"full"}`
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/config/auth/sandbox-oauth", bytes.NewBufferString(authBody))
	handleConfigAPI(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT auth status %d body %s", rec.Code, rec.Body.String())
	}

	path, err := secrets.ConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(path, "config.yaml")); err != nil {
		t.Fatalf("config.yaml missing: %v", err)
	}

	rec = httptest.NewRecorder()
	handleConfigAPI(rec, httptest.NewRequest(http.MethodDelete, "/api/config/tenant/sandbox", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("DELETE tenant status %d body %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatal(err)
	}
	if len(snap.Auths) != 0 {
		t.Fatalf("after delete: %+v", snap)
	}
	for _, tnt := range snap.Tenants {
		if tnt.Name == "sandbox" {
			t.Fatalf("sandbox still present: %+v", snap)
		}
	}
}

func TestSelectionPutBodyContextDualRead(t *testing.T) {
	var body selectionPutBody
	if err := json.Unmarshal([]byte(`{"default_context":"sandbox","channel":"mcp","channel_context":"prod"}`), &body); err != nil {
		t.Fatal(err)
	}
	if body.DefaultTenant != "sandbox" || body.Channel != "mcp" || body.ChannelTenant != "prod" {
		t.Fatalf("%+v", body)
	}
	body = selectionPutBody{}
	if err := json.Unmarshal([]byte(`{"default_tenant":"old","default_context":"new"}`), &body); err != nil {
		t.Fatal(err)
	}
	if body.DefaultTenant != "new" {
		t.Fatalf("context should win, got %q", body.DefaultTenant)
	}
}

func TestServeConfigHTML(t *testing.T) {
	rec := httptest.NewRecorder()
	serveConfigHTML(rec, httptest.NewRequest(http.MethodGet, "/config", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`class="active">config</a>`,
		`/api/config`,
		`buddy cred set secret`,
		`buddy setup`,
		`id="tree"`,
		`id="default-context"`,
		`id="a-scope"`,
		`id="t-org"`,
		`X-Environment-Id`,
		`<h2>Workplaces</h2>`,
		`data-ren=`,
		`data-ren-a=`,
		`id="auths-body"`,
		`class="table-wrap"`,
		`id="a-contexts-boxes"`,
		`jamfpro`,
		`id="f-orgid"`,
	} {
		if !bytes.Contains([]byte(body), []byte(want)) {
			t.Fatalf("missing %q", want)
		}
	}
}

func TestConfigAPIRenameAndTree(t *testing.T) {
	withTempBuddyHome(t)

	body := `{"url":"https://example.jamfcloud.com"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/config/context/sandbox", bytes.NewBufferString(body))
	handleConfigAPI(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT status %d body %s", rec.Code, rec.Body.String())
	}
	var snap buddy.ConfigSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatal(err)
	}
	if len(snap.Tree) == 0 {
		t.Fatalf("expected tree: %+v", snap)
	}
	foundSandbox := false
	walk := func(nodes []buddy.WorkplaceNodeSnapshot, fn func(buddy.WorkplaceNodeSnapshot)) {
		var recWalk func([]buddy.WorkplaceNodeSnapshot)
		recWalk = func(ns []buddy.WorkplaceNodeSnapshot) {
			for _, n := range ns {
				fn(n)
				recWalk(n.Children)
			}
		}
		recWalk(nodes)
	}
	walk(snap.Tree, func(n buddy.WorkplaceNodeSnapshot) {
		if n.Name == "sandbox" {
			foundSandbox = true
			if !n.CanDelete {
				t.Fatalf("instance leaf should be deletable: %+v", n)
			}
		}
		if n.Name == "default" && n.Kind == "organization" && n.CanDelete {
			t.Fatalf("org with children should not be deletable: %+v", n)
		}
	})
	if !foundSandbox {
		t.Fatalf("sandbox missing from tree: %+v", snap.Tree)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/config/rename", bytes.NewBufferString(`{"kind":"context","from":"default","to":"acme"}`))
	handleConfigAPI(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("rename status %d body %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatal(err)
	}
	foundAcme := false
	walk(snap.Tree, func(n buddy.WorkplaceNodeSnapshot) {
		if n.Name == "acme" && n.Kind == "organization" {
			foundAcme = true
		}
		if n.Name == "default" {
			t.Fatalf("default still present: %+v", snap.Tree)
		}
	})
	if !foundAcme {
		t.Fatalf("acme missing after rename: %+v", snap.Tree)
	}

	rec = httptest.NewRecorder()
	handleConfigAPI(rec, httptest.NewRequest(http.MethodDelete, "/api/config/context/acme", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("delete parent status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestConfigAPIRestoreLastGood(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(agent.EnvDisable, "1")
	t.Setenv(buddy.EnvManagedPlist, filepath.Join(dir, "no-managed.plist"))
	path := filepath.Join(dir, "buddy", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	good := []byte(`context:
  default: sandbox
orgs:
  default:
    environments:
      sandbox:
        url: https://example.jamfcloud.com
`)
	if err := os.WriteFile(path, good, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := buddy.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if err := buddy.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	buddy.SetActiveConfig(cfg)
	if err := os.WriteFile(path, []byte(":\tnot yaml\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = buddy.LoadConfig()
	if err == nil {
		t.Fatal("expected parse error")
	}
	buddy.SetConfigLoadError(err)
	buddy.SetActiveConfig(&buddy.Config{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/config/context/other", bytes.NewBufferString(`{"url":"https://x.example"}`))
	handleConfigAPI(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("broken mutations should 503, got %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handleConfigAPI(rec, httptest.NewRequest(http.MethodPost, "/api/config/restore", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("restore status %d body %s", rec.Code, rec.Body.String())
	}
	var snap buddy.ConfigSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatal(err)
	}
	if snap.ParseError != "" {
		t.Fatalf("parse_error after restore: %+v", snap)
	}
	found := false
	for _, tnt := range snap.Tenants {
		if tnt.Name == "sandbox" {
			found = true
		}
	}
	if !found {
		t.Fatalf("sandbox missing after restore: %+v", snap)
	}
}
