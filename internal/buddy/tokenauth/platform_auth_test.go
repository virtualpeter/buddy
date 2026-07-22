package tokenauth

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPlatformOAuth2Token_PrimaryEndpoint(t *testing.T) {
	var sawPath string
	var sawBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		sawBody = string(b)
		_ = json.NewEncoder(w).Encode(OAuth2TokenResponse{
			AccessToken: "plat-token",
			TokenType:   "Bearer",
			ExpiresIn:   900,
		})
	}))
	defer srv.Close()

	a := NewAuthenticator(Config{
		BaseURL:      srv.URL,
		ClientId:     "cid",
		ClientSecret: "sec",
		Platform:     true,
	})
	tok, err := a.GetToken()
	if err != nil {
		t.Fatal(err)
	}
	if tok != "plat-token" {
		t.Fatalf("token = %q", tok)
	}
	if sawPath != "/auth/token" {
		t.Fatalf("path = %q", sawPath)
	}
	if !strings.Contains(sawBody, "grant_type=client_credentials") ||
		!strings.Contains(sawBody, "client_id=cid") {
		t.Fatalf("body = %q", sawBody)
	}
	if a.TokenExpiry().Before(time.Now().Add(10 * time.Minute)) {
		t.Fatalf("expected ~900s expiry, got %v", a.TokenExpiry())
	}
}

func TestPlatformOAuth2Token_FallbackOAuth2Token(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.URL.Path == "/auth/token" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(OAuth2TokenResponse{
			AccessToken: "fallback-token",
			ExpiresIn:   900,
		})
	}))
	defer srv.Close()

	a := NewAuthenticator(Config{
		BaseURL:      srv.URL,
		ClientId:     "cid",
		ClientSecret: "sec",
		Platform:     true,
	})
	tok, err := a.GetToken()
	if err != nil {
		t.Fatal(err)
	}
	if tok != "fallback-token" {
		t.Fatalf("token = %q", tok)
	}
	if len(paths) < 2 || paths[0] != "/auth/token" || paths[1] != "/oauth2/token" {
		t.Fatalf("paths = %v", paths)
	}
}

func TestPlatformOAuth2_NoKeepAliveOnNearExpiry(t *testing.T) {
	var tokenCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "keep-alive") {
			t.Errorf("unexpected keep-alive call to %s", r.URL.Path)
		}
		if r.URL.Path == "/auth/token" {
			tokenCalls++
			_ = json.NewEncoder(w).Encode(OAuth2TokenResponse{
				AccessToken: "t" + string(rune('0'+tokenCalls)),
				ExpiresIn:   60, // within proactive window
			})
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	a := NewAuthenticator(Config{
		BaseURL:      srv.URL,
		ClientId:     "cid",
		ClientSecret: "sec",
		Platform:     true,
	})
	if _, err := a.GetToken(); err != nil {
		t.Fatal(err)
	}
	// Second call should re-request token (near expiry) rather than keep-alive.
	if _, err := a.GetToken(); err != nil {
		t.Fatal(err)
	}
	if tokenCalls < 2 {
		t.Fatalf("tokenCalls = %d, want >= 2", tokenCalls)
	}
}
