package buddy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"buddy/internal/buddy/tokenauth"
)

func TestPerformRequestOp_RetriesOnceAfter401(t *testing.T) {
	InitLogger()
	var apiHits, tokenHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/oauth/token" {
			n := tokenHits.Add(1)
			_ = json.NewEncoder(w).Encode(tokenauth.OAuth2TokenResponse{
				AccessToken: "tok" + string(rune('0'+n)),
				ExpiresIn:   3600,
			})
			return
		}
		n := apiHits.Add(1)
		if n == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"error":"expired"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()

	credAuthCache.mu.Lock()
	credAuthCache.byCred = make(map[string]*tokenauth.Authenticator)
	credAuthCache.mu.Unlock()

	SetActiveConfig(ConfigFromCredentialMap("oauth", map[string]CredentialConfig{
		"oauth": {
			URL: srv.URL,
			Auth: AuthConfig{
				Type:         "oauth",
				ClientID:     "cid",
				ClientSecret: "sec",
			},
		},
	}))
	t.Cleanup(func() {
		SetActiveConfig(nil)
		credAuthCache.mu.Lock()
		credAuthCache.byCred = make(map[string]*tokenauth.Authenticator)
		credAuthCache.mu.Unlock()
	})

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/packages", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := PerformRequestOp(req, "oauth", OpRead)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if apiHits.Load() < 2 {
		t.Fatalf("apiHits=%d want >=2 (401 then retry)", apiHits.Load())
	}
	if tokenHits.Load() < 2 {
		t.Fatalf("tokenHits=%d want >=2 (initial + refresh)", tokenHits.Load())
	}
}
