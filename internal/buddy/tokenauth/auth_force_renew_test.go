package tokenauth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestForceRenewObtainsFreshOAuthToken(t *testing.T) {
	var tokenCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/oauth/token" {
			http.NotFound(w, r)
			return
		}
		n := tokenCalls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": fmt.Sprintf("tok-%d", n),
			"expires_in":   1800,
			"token_type":   "Bearer",
		})
	}))
	defer srv.Close()

	dir := t.TempDir()
	restore := SetTokenCacheDirForTest(dir)
	t.Cleanup(restore)

	a := NewAuthenticator(Config{
		BaseURL:      srv.URL,
		ClientId:     "cid",
		ClientSecret: "sec",
		CacheKey:     "test-cred",
	})
	tok1, err := a.GetToken()
	if err != nil {
		t.Fatal(err)
	}
	if tokenCalls.Load() != 1 {
		t.Fatalf("calls=%d", tokenCalls.Load())
	}
	exp1 := a.TokenExpiry()
	if exp1.IsZero() || time.Until(exp1) < time.Minute {
		t.Fatalf("expiry=%v", exp1)
	}

	if err := a.ForceRenew(); err != nil {
		t.Fatal(err)
	}
	tok2, err := a.GetToken()
	if err != nil {
		t.Fatal(err)
	}
	if tokenCalls.Load() != 2 {
		t.Fatalf("after renew calls=%d", tokenCalls.Load())
	}
	if tok1 == "" || tok2 == "" {
		t.Fatalf("empty tokens %q %q", tok1, tok2)
	}
	if tok1 == tok2 {
		t.Fatalf("expected distinct tokens after ForceRenew, both %q", tok1)
	}
}
