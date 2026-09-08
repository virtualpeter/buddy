package tokenauth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRefreshSessionBasicKeepAlive(t *testing.T) {
	var tokenCalls, keepAliveCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "keep-alive"):
			keepAliveCalls++
			_ = json.NewEncoder(w).Encode(TokenResponse{Token: "kept", Expires: ""})
		case strings.Contains(r.URL.Path, "/api/v1/auth/token"):
			tokenCalls++
			_ = json.NewEncoder(w).Encode(TokenResponse{Token: "fresh", Expires: ""})
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	t.Cleanup(SetTokenCacheDirForTest(dir))

	if err := saveDiskToken("basic-ka", srv.URL, "cached", time.Now().Add(15*time.Minute)); err != nil {
		t.Fatal(err)
	}
	a := NewAuthenticator(Config{
		BaseURL:  srv.URL,
		Username: "u",
		Password: "p",
		CacheKey: "basic-ka",
	})
	res, err := a.RefreshSession()
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != TokenActionRefresh {
		t.Fatalf("action=%q want %q", res.Action, TokenActionRefresh)
	}
	if res.Token != "kept" {
		t.Fatalf("token=%q", res.Token)
	}
	if keepAliveCalls != 1 {
		t.Fatalf("keep-alive calls=%d", keepAliveCalls)
	}
	if tokenCalls != 0 {
		t.Fatalf("token calls=%d want 0", tokenCalls)
	}
}

func TestRefreshSessionBasicObtain(t *testing.T) {
	var tokenCalls, keepAliveCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "keep-alive"):
			keepAliveCalls++
			w.WriteHeader(401)
		case strings.Contains(r.URL.Path, "/api/v1/auth/token"):
			tokenCalls++
			_ = json.NewEncoder(w).Encode(TokenResponse{Token: "fresh", Expires: ""})
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	t.Cleanup(SetTokenCacheDirForTest(dir))

	a := NewAuthenticator(Config{
		BaseURL:  srv.URL,
		Username: "u",
		Password: "p",
		CacheKey: "basic-obtain",
	})
	res, err := a.RefreshSession()
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != TokenActionObtain {
		t.Fatalf("action=%q want %q", res.Action, TokenActionObtain)
	}
	if res.Token != "fresh" {
		t.Fatalf("token=%q", res.Token)
	}
	if tokenCalls != 1 {
		t.Fatalf("token calls=%d", tokenCalls)
	}
	if keepAliveCalls != 0 {
		t.Fatalf("keep-alive calls=%d want 0", keepAliveCalls)
	}
}

func TestRefreshSessionOAuthReplace(t *testing.T) {
	var tokenCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "keep-alive") {
			t.Errorf("unexpected keep-alive")
		}
		if r.URL.Path == "/api/oauth/token" {
			tokenCalls++
			_ = json.NewEncoder(w).Encode(OAuth2TokenResponse{
				AccessToken: "new-oauth",
				ExpiresIn:   1800,
				TokenType:   "Bearer",
			})
			return
		}
		w.WriteHeader(404)
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	t.Cleanup(SetTokenCacheDirForTest(dir))

	if err := saveDiskToken("oauth-rep", srv.URL, "cached-oauth", time.Now().Add(30*time.Minute)); err != nil {
		t.Fatal(err)
	}
	a := NewAuthenticator(Config{
		BaseURL:      srv.URL,
		ClientId:     "cid",
		ClientSecret: "sec",
		CacheKey:     "oauth-rep",
	})
	res, err := a.RefreshSession()
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != TokenActionReplace {
		t.Fatalf("action=%q want %q", res.Action, TokenActionReplace)
	}
	if res.Token != "new-oauth" {
		t.Fatalf("token=%q", res.Token)
	}
	if tokenCalls != 1 {
		t.Fatalf("token calls=%d", tokenCalls)
	}
	if PeekDiskToken("oauth-rep").Present && PeekDiskToken("oauth-rep").Usable {
		tok, _, ok := loadDiskToken("oauth-rep", srv.URL, clockSkewWindow)
		if !ok || tok == "cached-oauth" {
			t.Fatalf("expected replaced disk token, ok=%v tok=%q", ok, tok)
		}
	}
}
