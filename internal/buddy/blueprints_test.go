package buddy

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"buddy/internal/buddy/tokenauth"
)

func clearCredAuthCache() {
	credAuthCache.mu.Lock()
	credAuthCache.byCred = make(map[string]*tokenauth.Authenticator)
	credAuthCache.mu.Unlock()
}

func TestBlueprintList_RewritesPath(t *testing.T) {
	var sawPath string
	var sawTenant string
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "tok",
			"expires_in":   900,
		})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		sawTenant = r.Header.Get(HeaderTenantID)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"totalCount": 1,
			"results": []map[string]string{
				{"id": "bp-1", "name": "Test BP"},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	tid := "tenant-uuid-1"
	SetActiveConfig(ConfigFromCredentialMap("gw", map[string]CredentialConfig{
		"gw": {
			URL:      srv.URL,
			TenantID: tid,
			Auth: AuthConfig{
				Type:         "oauth",
				ClientID:     "cid",
				ClientSecret: "sec",
			},
		},
	}))
	clearCredAuthCache()
	t.Cleanup(func() {
		clearCredAuthCache()
		SetActiveConfig(nil)
	})

	items, err := BlueprintListValues("gw", BlueprintListOpts{Page: PageOpts{Page: PageAll}}, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != "bp-1" {
		t.Fatalf("items = %+v", items)
	}
	want := "/blueprints/v1/blueprints"
	if sawPath != want {
		t.Fatalf("path = %q want %q", sawPath, want)
	}
	if sawTenant != tid {
		t.Fatalf("X-Tenant-Id = %q want %q", sawTenant, tid)
	}
}

func TestBlueprintRequirePlatform_OrgScope(t *testing.T) {
	SetActiveConfig(ConfigFromCredentialMap("org", map[string]CredentialConfig{
		"org": {
			Region: "us",
			Auth:   AuthConfig{Type: "oauth", ClientID: "c", ClientSecret: "s"},
		},
	}))
	t.Cleanup(func() { SetActiveConfig(nil) })
	_, err := BlueprintListValues("org", BlueprintListOpts{}, slog.Default())
	if err == nil || !strings.Contains(err.Error(), "environment") {
		t.Fatalf("expected environment/tenant id error, got %v", err)
	}
}

func TestBlueprintRequirePlatform(t *testing.T) {
	SetActiveConfig(ConfigFromCredentialMap("inst", map[string]CredentialConfig{
		"inst": {
			URL:  "https://example.jamfcloud.com",
			Auth: AuthConfig{Type: "oauth", ClientID: "c", ClientSecret: "s"},
		},
	}))
	t.Cleanup(func() { SetActiveConfig(nil) })
	_, err := BlueprintListValues("inst", BlueprintListOpts{}, slog.Default())
	if err == nil || !strings.Contains(err.Error(), "Platform API Gateway") {
		t.Fatalf("expected platform error, got %v", err)
	}
}

func TestJamfRequest_PlatformRewrite(t *testing.T) {
	var sawPath string
	var sawTenant string
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "tok",
			"expires_in":   900,
		})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		sawTenant = r.Header.Get(HeaderTenantID)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	tid := "t-1"
	SetActiveConfig(ConfigFromCredentialMap("gw", map[string]CredentialConfig{
		"gw": {
			URL:      srv.URL,
			TenantID: tid,
			Auth:     AuthConfig{Type: "oauth", ClientID: "c", ClientSecret: "s"},
		},
	}))
	clearCredAuthCache()
	t.Cleanup(func() {
		clearCredAuthCache()
		SetActiveConfig(nil)
	})

	resp, err := JamfRequest("gw", JamfRequestOpts{Method: http.MethodGet, Path: "/api/v1/buildings"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != 200 {
		t.Fatalf("status %d body %s", resp.Status, resp.Body)
	}
	want := "/pro/v1/buildings"
	if sawPath != want {
		t.Fatalf("path = %q want %q", sawPath, want)
	}
	if sawTenant != tid {
		t.Fatalf("X-Tenant-Id = %q want %q", sawTenant, tid)
	}
}

func TestReadJSONBodyFile_Stdin(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = old })
	go func() {
		_, _ = w.Write([]byte(`{"name":"x"}`))
		_ = w.Close()
	}()
	data, err := ReadJSONBodyFile("-")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"name":"x"}` {
		t.Fatalf("got %s", data)
	}
}
