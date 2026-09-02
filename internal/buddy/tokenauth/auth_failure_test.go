package tokenauth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOAuthTokenRejectionIsActionable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/oauth/token" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_client"}`))
	}))
	defer srv.Close()

	a := NewAuthenticator(Config{
		Name:         "sandbox",
		BaseURL:      srv.URL,
		ClientId:     "cid",
		ClientSecret: "wrong",
	})
	_, err := a.GetToken()
	if err == nil {
		t.Fatal("expected error for 401 from token endpoint")
	}
	msg := err.Error()
	for _, want := range []string{"client secret", `credential "sandbox"`, "buddy cred set secret sandbox"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error %q missing %q", msg, want)
		}
	}
}

func TestBasicTokenRejectionIsActionable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/token" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	a := NewAuthenticator(Config{
		Name:     "onprem",
		BaseURL:  srv.URL,
		Username: "admin",
		Password: "wrong",
	})
	_, err := a.GetToken()
	if err == nil {
		t.Fatal("expected error for 401 from token endpoint")
	}
	msg := err.Error()
	if !strings.Contains(msg, "password") || !strings.Contains(msg, "buddy cred set secret onprem") {
		t.Fatalf("error %q not actionable", msg)
	}
}
