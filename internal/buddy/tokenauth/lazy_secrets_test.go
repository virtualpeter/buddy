package tokenauth

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetTokenSkipsLoadSecretsWhenDiskCacheHit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("token endpoint must not be called on cache hit")
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	SetTokenCacheDirForTest(dir)
	t.Cleanup(func() { SetTokenCacheDirForTest("") })

	cacheKey := "lazy-oauth"
	expiry := time.Now().Add(30 * time.Minute)
	if err := saveDiskToken(cacheKey, srv.URL, "cached-tok", expiry); err != nil {
		t.Fatal(err)
	}

	var loads atomic.Int32
	a := NewAuthenticator(Config{
		Name:         "sandbox",
		BaseURL:      srv.URL,
		ClientId:     "cid",
		ClientSecret: "", // vault-backed
		CacheKey:     cacheKey,
		LoadSecrets: func() (string, string, error) {
			loads.Add(1)
			return "", "should-not-load", nil
		},
	})
	tok, err := a.GetToken()
	if err != nil {
		t.Fatal(err)
	}
	if tok != "cached-tok" {
		t.Fatalf("token = %q", tok)
	}
	if loads.Load() != 0 {
		t.Fatalf("LoadSecrets called %d times; want 0 on disk cache hit", loads.Load())
	}
}

func TestGetTokenLoadsSecretsWhenMinting(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.Form.Get("client_secret") != "from-vault" {
			t.Fatalf("client_secret = %q", r.Form.Get("client_secret"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"fresh","expires_in":3600,"token_type":"Bearer"}`))
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	SetTokenCacheDirForTest(dir)
	t.Cleanup(func() { SetTokenCacheDirForTest("") })

	var loads atomic.Int32
	a := NewAuthenticator(Config{
		Name:     "sandbox",
		BaseURL:  srv.URL,
		ClientId: "cid",
		CacheKey: "lazy-mint",
		LoadSecrets: func() (string, string, error) {
			loads.Add(1)
			return "", "from-vault", nil
		},
	})
	tok, err := a.GetToken()
	if err != nil {
		t.Fatal(err)
	}
	if tok != "fresh" {
		t.Fatalf("token = %q", tok)
	}
	if loads.Load() != 1 {
		t.Fatalf("LoadSecrets called %d times; want 1", loads.Load())
	}
}
