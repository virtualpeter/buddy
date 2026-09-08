package buddy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"buddy/internal/buddy/secrets"
	"buddy/internal/buddy/tokenauth"
)

func TestAuthRefreshOAuthReplaceAndTokenDrop(t *testing.T) {
	InitLogger()
	var tokenHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/oauth/token" {
			n := tokenHits.Add(1)
			_ = json.NewEncoder(w).Encode(tokenauth.OAuth2TokenResponse{
				AccessToken: "tok" + string(rune('0'+n)),
				ExpiresIn:   3600,
			})
			return
		}
		if strings.Contains(r.URL.Path, "keep-alive") {
			t.Errorf("unexpected keep-alive")
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	tokDir := t.TempDir()
	t.Cleanup(tokenauth.SetTokenCacheDirForTest(tokDir))

	secrets.SetStore(&mockSecretStore{values: map[string]string{
		"sandbox-oauth/client-secret": "s3cret",
	}})
	t.Cleanup(func() { secrets.SetStore(nil) })

	cfg := &Config{
		Tenant: TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{
			"sandbox": {URL: srv.URL},
		},
		Auths: map[string]AuthEntry{
			"sandbox-oauth": {Type: "oauth", ClientID: "cid", Tenants: []string{"sandbox"}, Surface: SurfaceInstance},
		},
	}
	SetActiveConfig(cfg)
	SetActiveSelection("sandbox", "sandbox-oauth")
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveSelection("", "")
		credAuthCache.mu.Lock()
		credAuthCache.byCred = make(map[string]*tokenauth.Authenticator)
		credAuthCache.mu.Unlock()
	})

	cred := ComposeCredential(cfg.Tenants["sandbox"], cfg.Auths["sandbox-oauth"], SurfaceInstance)
	a, err := cachedAuthenticator("sandbox-oauth@sandbox", "sandbox-oauth", cred)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.GetToken(); err != nil {
		t.Fatal(err)
	}
	if tokenHits.Load() != 1 {
		t.Fatalf("seed hits=%d", tokenHits.Load())
	}

	st, err := SessionTokenStatus("sandbox-oauth")
	if err != nil {
		t.Fatal(err)
	}
	if st.Cache != "present" {
		t.Fatalf("cache=%q want present", st.Cache)
	}

	res, err := AuthRefresh("sandbox-oauth", GetLogger())
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != tokenauth.TokenActionReplace {
		t.Fatalf("action=%q", res.Action)
	}
	if tokenHits.Load() != 2 {
		t.Fatalf("replace should mint again, hits=%d", tokenHits.Load())
	}

	if err := DropTokenCache("sandbox-oauth", false); err != nil {
		t.Fatal(err)
	}
	st, err = SessionTokenStatus("sandbox-oauth")
	if err != nil {
		t.Fatal(err)
	}
	if st.Cache != "missing" {
		t.Fatalf("after drop cache=%q", st.Cache)
	}
}

func TestAuthRefreshBasicObtain(t *testing.T) {
	InitLogger()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/api/v1/auth/token") {
			_ = json.NewEncoder(w).Encode(tokenauth.TokenResponse{Token: "basic-fresh"})
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	tokDir := t.TempDir()
	t.Cleanup(tokenauth.SetTokenCacheDirForTest(tokDir))

	secrets.SetStore(&mockSecretStore{values: map[string]string{
		"sandbox-basic/password": "pw",
	}})
	t.Cleanup(func() { secrets.SetStore(nil) })

	cfg := &Config{
		Tenant: TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{
			"sandbox": {URL: srv.URL},
		},
		Auths: map[string]AuthEntry{
			"sandbox-basic": {Type: "basic", Username: "admin", Tenants: []string{"sandbox"}, Surface: SurfaceInstance},
		},
	}
	SetActiveConfig(cfg)
	SetActiveSelection("sandbox", "sandbox-basic")
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveSelection("", "")
		credAuthCache.mu.Lock()
		credAuthCache.byCred = make(map[string]*tokenauth.Authenticator)
		credAuthCache.mu.Unlock()
	})

	res, err := AuthRefresh("sandbox-basic", GetLogger())
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != tokenauth.TokenActionObtain {
		t.Fatalf("action=%q", res.Action)
	}
	if res.Token != "basic-fresh" {
		t.Fatalf("token=%q", res.Token)
	}
	if time.Until(res.Expiry) < time.Minute {
		t.Fatalf("expiry=%v", res.Expiry)
	}
}
