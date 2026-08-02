package tokenauth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInstanceOAuth2_NoKeepAlive(t *testing.T) {
	var tokenCalls, keepAliveCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "keep-alive"):
			keepAliveCalls++
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{}`))
		case r.URL.Path == "/api/oauth/token":
			tokenCalls++
			_ = json.NewEncoder(w).Encode(OAuth2TokenResponse{
				AccessToken: "t" + string(rune('0'+tokenCalls)),
				ExpiresIn:   60,
			})
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	a := NewAuthenticator(Config{
		BaseURL:      srv.URL,
		ClientId:     "cid",
		ClientSecret: "sec",
	})
	if _, err := a.GetToken(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.GetToken(); err != nil {
		t.Fatal(err)
	}
	if keepAliveCalls != 0 {
		t.Fatalf("keep-alive calls = %d, want 0", keepAliveCalls)
	}
	if tokenCalls != 1 {
		t.Fatalf("tokenCalls = %d, want 1 (reuse 60s token)", tokenCalls)
	}
}
