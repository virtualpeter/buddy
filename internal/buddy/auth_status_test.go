package buddy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"buddy/internal/buddy/secrets"
	"buddy/internal/buddy/tokenauth"
)

func TestAuthTestForceRenewAndTokenOkOnProbe403(t *testing.T) {
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
		if r.URL.Path == "/api/v1/auth" {
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, `{"httpStatus":403}`)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	capsDir := t.TempDir()
	t.Cleanup(SetCapsCacheDirForTest(capsDir))
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

	// Seed a disk token so a naive GET /api/v1/auth would not need to mint.
	a, err := cachedAuthenticator("sandbox-oauth@sandbox", "sandbox-oauth", ComposeCredential(cfg.Tenants["sandbox"], cfg.Auths["sandbox-oauth"], SurfaceInstance))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.GetToken(); err != nil {
		t.Fatal(err)
	}
	if tokenHits.Load() != 1 {
		t.Fatalf("seed token hits=%d", tokenHits.Load())
	}

	res, err := AuthTestValue("sandbox-oauth", GetLogger())
	if err != nil {
		t.Fatal(err)
	}
	if live, _ := res["live"].(string); live != LiveTokenOK {
		t.Fatalf("live=%v body=%v", res["live"], res)
	}
	if ok, _ := res["ok"].(bool); !ok {
		t.Fatal("ok should be true for token-ok")
	}
	if tokenHits.Load() < 2 {
		t.Fatalf("ForceRenew should mint again, hits=%d", tokenHits.Load())
	}
	rec := LoadCaps("sandbox-oauth", "sandbox", SurfaceInstance, "")
	if rec == nil || rec.Live != LiveTokenOK || !rec.TokenOk {
		t.Fatalf("caps=%+v", rec)
	}
}

func TestAuthTestOrganizationWorkplaceProbesLicenses(t *testing.T) {
	InitLogger()
	var tokenHits, buildingHits, licenseHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/token", "/oauth2/token", "/api/oauth/token":
			tokenHits.Add(1)
			_ = json.NewEncoder(w).Encode(tokenauth.OAuth2TokenResponse{
				AccessToken: "org-tok",
				ExpiresIn:   3600,
			})
			return
		case "/api/v1/buildings", "/pro/v1/buildings":
			buildingHits.Add(1)
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"errors":[{"code":"REQUEST_CONTEXT_NOT_PROVIDED"}]}`)
			return
		case "/licensing/v1/licenses", "/api/licensing/v1/licenses":
			licenseHits.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"totalCount": 0,
				"results":    []any{},
			})
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	capsDir := t.TempDir()
	t.Cleanup(SetCapsCacheDirForTest(capsDir))
	tokDir := t.TempDir()
	t.Cleanup(tokenauth.SetTokenCacheDirForTest(tokDir))
	t.Cleanup(SetOrgPlatformGatewayURLForTest(srv.URL))
	clearCredAuthCache()
	t.Cleanup(clearCredAuthCache)

	cfg := orgLicensesTestConfig()
	SetActiveConfig(cfg)
	SetActiveSelection("sandbox", "sandbox")
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveSelection("", "")
	})

	res, err := AuthTestValue("sandbox", GetLogger())
	if err != nil {
		t.Fatal(err)
	}
	if live, _ := res["live"].(string); live != LiveOK {
		t.Fatalf("live=%v body=%v", res["live"], res)
	}
	if probe, _ := res["probe"].(string); probe != "licenses" {
		t.Fatalf("probe=%v want licenses", res["probe"])
	}
	if tokenHits.Load() < 1 {
		t.Fatal("expected a token mint")
	}
	if licenseHits.Load() < 1 {
		t.Fatal("expected a licenses probe")
	}
	if buildingHits.Load() != 0 {
		t.Fatalf("organization workplace should not probe buildings, hits=%d", buildingHits.Load())
	}
}

func TestAuthTestOrganizationWorkplaceLicenses403(t *testing.T) {
	InitLogger()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/token", "/oauth2/token", "/api/oauth/token":
			_ = json.NewEncoder(w).Encode(tokenauth.OAuth2TokenResponse{
				AccessToken: "org-tok",
				ExpiresIn:   3600,
			})
			return
		case "/licensing/v1/licenses", "/api/licensing/v1/licenses":
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, `{"httpStatus":403}`)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	capsDir := t.TempDir()
	t.Cleanup(SetCapsCacheDirForTest(capsDir))
	tokDir := t.TempDir()
	t.Cleanup(tokenauth.SetTokenCacheDirForTest(tokDir))
	t.Cleanup(SetOrgPlatformGatewayURLForTest(srv.URL))
	clearCredAuthCache()
	t.Cleanup(clearCredAuthCache)

	SetActiveConfig(orgLicensesTestConfig())
	SetActiveSelection("sandbox", "sandbox")
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveSelection("", "")
	})

	res, err := AuthTestValue("sandbox", GetLogger())
	if err != nil {
		t.Fatal(err)
	}
	if live, _ := res["live"].(string); live != LiveTokenOK {
		t.Fatalf("live=%v body=%v", res["live"], res)
	}
	if probe, _ := res["probe"].(string); probe != "licenses" {
		t.Fatalf("probe=%v want licenses", res["probe"])
	}
	rec := LoadCaps("sandbox", "sandbox", SurfacePlatform, "org")
	if rec == nil || rec.Live != LiveTokenOK || !rec.TokenOk {
		t.Fatalf("caps=%+v", rec)
	}
}

func TestAuthTestSecretMissing(t *testing.T) {
	secrets.SetStore(&mockSecretStore{values: map[string]string{}})
	t.Cleanup(func() { secrets.SetStore(nil) })
	cfg := &Config{
		Tenant: TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{
			"sandbox": {URL: "https://example.jamfcloud.com"},
		},
		Auths: map[string]AuthEntry{
			"sandbox-oauth": {Type: "oauth", ClientID: "cid", Tenants: []string{"sandbox"}},
		},
	}
	SetActiveConfig(cfg)
	SetActiveSelection("sandbox", "sandbox-oauth")
	t.Cleanup(func() {
		SetActiveConfig(nil)
		SetActiveSelection("", "")
	})
	dir := t.TempDir()
	t.Cleanup(SetCapsCacheDirForTest(dir))

	_, err := AuthTestValue("sandbox-oauth", nil)
	if err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("err=%v", err)
	}
	rec := LoadCaps("sandbox-oauth", "sandbox", SurfaceInstance, "")
	if rec == nil || rec.Live != LiveSecretMissing {
		t.Fatalf("caps=%+v", rec)
	}
}

func TestScoreAuthCapsBonusAndPenalty(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(SetCapsCacheDirForTest(dir))
	cfg := &Config{}
	auth := AuthEntry{Type: "oauth", ClientID: "c", Access: AccessFull, Surface: SurfaceInstance}
	tenant := TenantConfig{URL: "https://example.jamfcloud.com"}

	base := scoreAuth(cfg, "good", auth, "sandbox", tenant, SurfaceInstance, OpRead, ChannelCLI)

	if err := SaveCaps(CapsRecord{
		Auth: "good", Tenant: "sandbox", Surface: SurfaceInstance,
		Live: LiveOK, TokenOk: true, ProbeOk: true,
		ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	bonus := scoreAuth(cfg, "good", auth, "sandbox", tenant, SurfaceInstance, OpRead, ChannelCLI)
	if bonus <= base {
		t.Fatalf("token-ok should bonus: base=%d bonus=%d", base, bonus)
	}

	if err := SaveCaps(CapsRecord{
		Auth: "bad", Tenant: "sandbox", Surface: SurfaceInstance,
		Live: LiveTokenFail, TokenOk: false,
		ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	penalty := scoreAuth(cfg, "bad", auth, "sandbox", tenant, SurfaceInstance, OpRead, ChannelCLI)
	if penalty >= base {
		t.Fatalf("token-fail should penalize: base=%d penalty=%d", base, penalty)
	}
}

func TestSurfaceForOpHonorsAuthSurface(t *testing.T) {
	auth := AuthEntry{Type: AuthTypeOAuth, ClientID: "c"}
	got := surfaceForOp(nil, OpRead, auth)
	if got != SurfaceInstance {
		t.Fatalf("instance auth=%q", got)
	}
	auth.Type = AuthTypePlatform
	got = surfaceForOp(nil, OpRead, auth)
	if got != SurfacePlatform {
		t.Fatalf("platform auth=%q", got)
	}
}

func TestSetupSecretsFillsPlatformGap(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, dir+"/no-managed.plist")
	secrets.SetStore(&mockSecretStore{values: map[string]string{
		"sandbox-oauth/client-secret": "already",
	}})
	t.Cleanup(func() { secrets.SetStore(nil) })

	cfg := &Config{
		Tenant: TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{
			"sandbox": {
				URL:      "https://example.jamfcloud.com",
				Region:   "us",
				TenantID: "11111111-1111-1111-1111-111111111111",
			},
		},
		Auths: map[string]AuthEntry{
			"sandbox-oauth": {Type: "oauth", ClientID: "cid", Tenants: []string{"sandbox"}, Surface: SurfaceInstance},
		},
	}
	_, err := SetupSecrets(cfg, SetupSecretsOpts{
		Type:         "oauth",
		AuthName:     "sandbox-platform",
		ClientID:     "plat-cid",
		ClientSecret: "plat-sec",
	})
	if err != nil {
		t.Fatal(err)
	}
	a, ok := cfg.Auths["sandbox-platform"]
	if !ok || a.Type != AuthTypePlatform || a.ClientID != "plat-cid" {
		t.Fatalf("auth=%+v", a)
	}
	okHas, err := HasCredentialSecret("sandbox-platform", secrets.KindClientSecret)
	if err != nil || !okHas {
		t.Fatalf("stored? %v %v", okHas, err)
	}
}

func TestCredAddNonInteractive(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, dir+"/no-managed.plist")
	secrets.SetStore(&mockSecretStore{values: map[string]string{}})
	t.Cleanup(func() { secrets.SetStore(nil) })

	cfg := &Config{Tenant: TenantSelection{Default: ""}}
	res, err := CredAdd(cfg, CredAddOpts{
		TenantName:   "sandbox",
		URL:          "https://example.jamfcloud.com",
		AuthName:     "sandbox-oauth",
		Type:         "oauth",
		ClientID:     "cid",
		ClientSecret: "s3cret",
		SkipConfirm:  true,
		NoTest:       true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.AuthName != "sandbox-oauth" || !res.CreatedTenant {
		t.Fatalf("%+v", res)
	}
	if _, ok := cfg.Tenants["sandbox"]; !ok {
		t.Fatal("tenant missing")
	}
	okHas, err := HasCredentialSecret("sandbox-oauth", secrets.KindClientSecret)
	if err != nil || !okHas {
		t.Fatalf("secret stored? %v %v", okHas, err)
	}
}

func TestCredAddRequiresFieldsNonInteractive(t *testing.T) {
	cfg := &Config{}
	_, err := CredAdd(cfg, CredAddOpts{SkipConfirm: true, NoTest: true}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCredAddFromPastedInstanceJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, dir+"/no-managed.plist")
	secrets.SetStore(&mockSecretStore{values: map[string]string{}})
	t.Cleanup(func() { secrets.SetStore(nil) })

	cfg := &Config{}
	res, err := CredAdd(cfg, CredAddOpts{
		TenantName:  "sandbox",
		URL:         "https://example.jamfcloud.com",
		PasteBlob:   instanceClientJSON,
		SkipConfirm: true,
		NoTest:      true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.AuthName != "buddy-cli" {
		t.Fatalf("auth name from client_name: %+v", res)
	}
	a := cfg.Auths["buddy-cli"]
	if a.Type != AuthTypeOAuth || a.ClientID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("auth=%+v", a)
	}
	okHas, err := HasCredentialSecret("buddy-cli", secrets.KindClientSecret)
	if err != nil || !okHas {
		t.Fatalf("secret stored? %v %v", okHas, err)
	}
	if len(cfg.Orgs) != 0 {
		t.Fatalf("instance paste must write flat contexts only, got orgs=%+v", cfg.Orgs)
	}
	if cfg.Tenants["sandbox"].URL != "https://example.jamfcloud.com" {
		t.Fatalf("instance url=%+v", cfg.Tenants["sandbox"])
	}
}

func TestCredAddFromPastedPlatformCurl(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, dir+"/no-managed.plist")
	secrets.SetStore(&mockSecretStore{values: map[string]string{}})
	t.Cleanup(func() { secrets.SetStore(nil) })

	cfg := &Config{}
	res, err := CredAdd(cfg, CredAddOpts{
		TenantName:  "sandbox",
		PasteBlob:   platformOrgCurl,
		SkipConfirm: true,
		NoTest:      true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.AuthName != "sandbox-platform" {
		t.Fatalf("%+v", res)
	}
	tc := cfg.Tenants["sandbox"]
	if tc.Region != "" || tc.URL != "" || !tc.IsOrganizationWorkplace() {
		t.Fatalf("context=%+v", tc)
	}
	if _, ok := cfg.Orgs["sandbox"]; !ok {
		t.Fatalf("org paste should write orgs.sandbox: %+v", cfg.Orgs)
	}
	a := cfg.Auths["sandbox-platform"]
	if a.Type != AuthTypePlatform || a.Scope != PlatformScopeOrganization || a.ClientID != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("auth=%+v", a)
	}
}

func TestCredAddFromPastedUSEnvCurl(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, dir+"/no-managed.plist")
	secrets.SetStore(&mockSecretStore{values: map[string]string{}})
	t.Cleanup(func() { secrets.SetStore(nil) })

	cfg := &Config{}
	_, err := CredAdd(cfg, CredAddOpts{
		TenantName:    "sandbox",
		PasteBlob:     platformEnvCurl,
		EnvironmentID: "11111111-1111-1111-1111-111111111111",
		SkipConfirm:   true,
		NoTest:        true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	tc := cfg.Tenants["sandbox"]
	if tc.Region != "us" || tc.EnvironmentID != "11111111-1111-1111-1111-111111111111" || tc.IsOrganizationWorkplace() {
		t.Fatalf("env context=%+v", tc)
	}
	a := cfg.Auths["sandbox-platform"]
	if a.Type != AuthTypePlatform || a.Scope == PlatformScopeOrganization {
		t.Fatalf("env auth should not be org: %+v", a)
	}
}

func TestCredAddOrgAuthOntoInstanceContext(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, dir+"/no-managed.plist")
	secrets.SetStore(&mockSecretStore{values: map[string]string{}})
	t.Cleanup(func() { secrets.SetStore(nil) })

	cfg := &Config{Tenants: map[string]TenantConfig{
		"sandbox": {URL: "https://example.jamfcloud.com"},
	}}
	_, err := CredAdd(cfg, CredAddOpts{
		TenantName:  "sandbox",
		AuthName:    "org",
		PasteBlob:   platformOrgCurl,
		SkipConfirm: true,
		NoTest:      true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	tc := cfg.Tenants["sandbox"]
	if tc.URL != "https://example.jamfcloud.com" || tc.Region != "" {
		t.Fatalf("instance context must stay instance: %+v", tc)
	}
	a := cfg.Auths["org"]
	if a.Type != AuthTypePlatform || a.Scope != PlatformScopeOrganization || !a.LinkedTo("sandbox") {
		t.Fatalf("auth=%+v", a)
	}
	cred := ComposeCredential(tc, a, SurfacePlatform)
	if cred.URL != OrgPlatformGatewayURL || cred.Region != OrgPlatformRegion {
		t.Fatalf("org session: %+v", cred)
	}
}
