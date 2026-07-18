package buddy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalizeJamfPath(t *testing.T) {
	got, err := NormalizeJamfPath("/api/v1/packages")
	if err != nil || got != "/api/v1/packages" {
		t.Fatalf("got %q err %v", got, err)
	}
	for _, bad := range []string{"", "api/v1", "https://x/api", "/api/../v1", "//host/api"} {
		if _, err := NormalizeJamfPath(bad); err == nil {
			t.Errorf("NormalizeJamfPath(%q) expected error", bad)
		}
	}
}

func TestDefaultJamfMedia(t *testing.T) {
	ct, acc := DefaultJamfMedia("/JSSResource/policies/id/1")
	if ct != "application/xml" || acc != "application/xml" {
		t.Fatalf("classic: %q %q", ct, acc)
	}
	ct, acc = DefaultJamfMedia("/api/v1/packages")
	if ct != "application/json" || acc != "application/json" {
		t.Fatalf("pro: %q %q", ct, acc)
	}
}

func TestJamfRequest(t *testing.T) {
	InitLogger()
	var sawMethod, sawPath, sawAuth, sawAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawMethod = r.Method
		sawPath = r.URL.Path
		sawAuth = r.Header.Get("Authorization")
		sawAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()

	SetActiveConfig(&Config{
		Cred: "t",
		Credentials: map[string]CredentialConfig{
			"t": {
				URL: srv.URL,
				Auth: AuthConfig{
					Type:       "command",
					RunCommand: "printf Bearer-test-token",
				},
			},
		},
	})
	t.Cleanup(func() { SetActiveConfig(nil) })

	resp, err := JamfRequest("t", JamfRequestOpts{
		Method: http.MethodGet,
		Path:   "/api/v1/auth",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != 200 {
		t.Fatalf("status %d", resp.Status)
	}
	if string(resp.Body) != `{"ok":true}` {
		t.Fatalf("body %s", resp.Body)
	}
	if sawMethod != http.MethodGet || sawPath != "/api/v1/auth" {
		t.Fatalf("request %s %s", sawMethod, sawPath)
	}
	if sawAuth == "" {
		t.Fatal("missing Authorization")
	}
	if sawAccept != "application/json" {
		t.Fatalf("Accept %q", sawAccept)
	}
}

func TestJamfRequestNon2xx(t *testing.T) {
	InitLogger()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":"missing"}`)
	}))
	defer srv.Close()

	SetActiveConfig(&Config{
		Cred: "t",
		Credentials: map[string]CredentialConfig{
			"t": {
				URL: srv.URL,
				Auth: AuthConfig{
					Type:       "command",
					RunCommand: "printf tok",
				},
			},
		},
	})
	t.Cleanup(func() { SetActiveConfig(nil) })

	resp, err := JamfRequest("t", JamfRequestOpts{Method: "GET", Path: "/api/v1/nope"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != 404 {
		t.Fatalf("status %d", resp.Status)
	}
	if string(resp.Body) != `{"error":"missing"}` {
		t.Fatalf("body %s", resp.Body)
	}
}
