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
	"buddy/internal/buddy/secrets"
)

func withTempBuddyHome(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
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
	if snap.NeedsSetup || len(snap.Tenants) != 1 || len(snap.Contexts) != 1 {
		t.Fatalf("after tenant: %+v", snap)
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
	if !snap.NeedsSetup || len(snap.Auths) != 0 {
		t.Fatalf("after delete: %+v", snap)
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
		`id="contexts-body"`,
		`id="default-context"`,
		`<h2>Contexts</h2>`,
	} {
		if !bytes.Contains([]byte(body), []byte(want)) {
			t.Fatalf("missing %q", want)
		}
	}
}
