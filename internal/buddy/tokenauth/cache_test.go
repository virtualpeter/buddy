package tokenauth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDiskTokenRoundTrip(t *testing.T) {
	dir := t.TempDir()
	restore := SetTokenCacheDirForTest(dir)
	t.Cleanup(restore)

	expiry := time.Now().Add(20 * time.Minute)
	if err := saveDiskToken("default", "https://jamf.example.com", "tok-abc", expiry); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "default.json")
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm()&0o077 != 0 {
		t.Fatalf("permissions too open: %v", fi.Mode())
	}

	tok, gotExp, ok := loadDiskToken("default", "https://jamf.example.com", clockSkewWindow)
	if !ok || tok != "tok-abc" {
		t.Fatalf("load ok=%v tok=%q", ok, tok)
	}
	if gotExp.Sub(expiry) > time.Second || expiry.Sub(gotExp) > time.Second {
		t.Fatalf("expiry mismatch got=%v want=%v", gotExp, expiry)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "tok-abc") {
		t.Fatal("plaintext token must not appear in cache file")
	}

	// Wrong URL → miss and file removed
	if _, _, ok := loadDiskToken("default", "https://other.example.com", clockSkewWindow); ok {
		t.Fatal("expected URL mismatch miss")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected URL-mismatch file removed, err=%v", err)
	}
}

func TestLegacyPlaintextTokenPurged(t *testing.T) {
	dir := t.TempDir()
	restore := SetTokenCacheDirForTest(dir)
	t.Cleanup(restore)

	path := filepath.Join(dir, "legacy.json")
	data, _ := json.Marshal(map[string]any{
		"token":  "plaintext-secret-token",
		"expiry": time.Now().Add(30 * time.Minute).UTC(),
		"url":    "https://jamf.example.com",
	})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := loadDiskToken("legacy", "https://jamf.example.com", clockSkewWindow); ok {
		t.Fatal("expected miss for legacy plaintext cache")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected legacy file purged, err=%v", err)
	}
}

func TestCorruptSealedTokenPurged(t *testing.T) {
	dir := t.TempDir()
	restore := SetTokenCacheDirForTest(dir)
	t.Cleanup(restore)

	path := filepath.Join(dir, "badseal.json")
	data, _ := json.Marshal(diskTokenFile{
		V:      diskTokenVersionV1,
		Token:  base64Garbage(),
		Expiry: time.Now().Add(30 * time.Minute).UTC(),
		URL:    "https://jamf.example.com",
	})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := loadDiskToken("badseal", "https://jamf.example.com", clockSkewWindow); ok {
		t.Fatal("expected miss for undecodable seal")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected undecodable file purged, err=%v", err)
	}
}

func base64Garbage() string {
	return "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
}

func TestLoadDiskTokenNearExpiryPurges(t *testing.T) {
	dir := t.TempDir()
	restore := SetTokenCacheDirForTest(dir)
	t.Cleanup(restore)

	// Write near-expiry sealed file directly (saveDiskToken would opportunistic-sweep it).
	path := filepath.Join(dir, "near.json")
	sealed, err := sealToken("tok")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(diskTokenFile{
		V:      diskTokenVersionV1,
		Token:  sealed,
		Expiry: time.Now().Add(5 * time.Second).UTC(),
		URL:    "https://jamf.example.com",
	})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := loadDiskToken("near", "https://jamf.example.com", clockSkewWindow); ok {
		t.Fatal("expected miss when within skew window")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected near-expiry file purged, err=%v", err)
	}
}

func TestCleanExpiredTokenCache(t *testing.T) {
	dir := t.TempDir()
	restore := SetTokenCacheDirForTest(dir)
	t.Cleanup(restore)

	goodSeal, err := sealToken("ok")
	if err != nil {
		t.Fatal(err)
	}
	badSeal, err := sealToken("old")
	if err != nil {
		t.Fatal(err)
	}
	good := diskTokenFile{V: diskTokenVersionV1, Token: goodSeal, Expiry: time.Now().Add(30 * time.Minute).UTC(), URL: "https://a.example"}
	bad := diskTokenFile{V: diskTokenVersionV1, Token: badSeal, Expiry: time.Now().Add(-time.Minute).UTC(), URL: "https://a.example"}
	write := func(name string, f diskTokenFile) {
		b, _ := json.Marshal(f)
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("good.json", good)
	write("bad.json", bad)
	write("corrupt.json", diskTokenFile{})
	_ = os.WriteFile(filepath.Join(dir, "notjson.json"), []byte("{"), 0o600)

	res, err := CleanExpiredTokenCache()
	if err != nil {
		t.Fatal(err)
	}
	if res.Deleted < 3 {
		t.Fatalf("deleted=%d want >= 3", res.Deleted)
	}
	if res.Kept != 1 {
		t.Fatalf("kept=%d want 1", res.Kept)
	}
	if _, err := os.Stat(filepath.Join(dir, "good.json")); err != nil {
		t.Fatal("good entry should remain")
	}
}

func TestPeekDiskTokenDoesNotDeleteExpired(t *testing.T) {
	dir := t.TempDir()
	restore := SetTokenCacheDirForTest(dir)
	t.Cleanup(restore)

	path := filepath.Join(dir, "stale.json")
	sealed, err := sealToken("old")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(diskTokenFile{
		V:      diskTokenVersionV1,
		Token:  sealed,
		Expiry: time.Now().Add(-time.Minute).UTC(),
		URL:    "https://jamf.example.com",
	})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	info := PeekDiskToken("stale")
	if !info.Present {
		t.Fatal("expected present expired cache")
	}
	if info.Usable {
		t.Fatal("expected unusable")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("peek must not delete expired file: %v", err)
	}
}

func TestDropDiskTokenAndDropAll(t *testing.T) {
	dir := t.TempDir()
	restore := SetTokenCacheDirForTest(dir)
	t.Cleanup(restore)

	if err := saveDiskToken("keep-me", "https://a.example", "tok", time.Now().Add(30*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if !DropDiskToken("keep-me") {
		t.Fatal("expected drop of existing file")
	}
	if DropDiskToken("keep-me") {
		t.Fatal("second drop should report missing")
	}
	if err := saveDiskToken("a", "https://a.example", "t1", time.Now().Add(30*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := saveDiskToken("b", "https://a.example", "t2", time.Now().Add(30*time.Minute)); err != nil {
		t.Fatal(err)
	}
	res, err := DropAllDiskTokens()
	if err != nil {
		t.Fatal(err)
	}
	if res.Dropped != 2 {
		t.Fatalf("dropped=%d want 2", res.Dropped)
	}
	list, err := ListDiskTokens()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("list=%d after drop all", len(list))
	}
}

func TestAuthenticatorDiskCache(t *testing.T) {
	dir := t.TempDir()
	restore := SetTokenCacheDirForTest(dir)
	t.Cleanup(restore)

	a := NewAuthenticator(Config{
		BaseURL:  "https://jamf.example.com",
		CacheKey: "sandbox",
	})
	a.setToken("cached-tok", time.Now().Add(15*time.Minute))

	a2 := NewAuthenticator(Config{
		BaseURL:  "https://jamf.example.com",
		CacheKey: "sandbox",
	})
	if !a2.loadFromDiskIfValid() {
		t.Fatal("expected load from disk")
	}
	if a2.token != "cached-tok" {
		t.Fatalf("token=%q", a2.token)
	}

	a2.InvalidateDiskCache()
	a3 := NewAuthenticator(Config{
		BaseURL:  "https://jamf.example.com",
		CacheKey: "sandbox",
	})
	if a3.loadFromDiskIfValid() {
		t.Fatal("expected miss after invalidate")
	}
}

func TestOAuth2_ReusesUntilExpiry(t *testing.T) {
	var tokenCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "keep-alive") {
			t.Errorf("unexpected keep-alive call to %s", r.URL.Path)
		}
		if r.URL.Path == "/api/oauth/token" {
			tokenCalls++
			_ = json.NewEncoder(w).Encode(OAuth2TokenResponse{
				AccessToken: "tok",
				ExpiresIn:   60, // previously triggered 3m proactive re-auth
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
	})
	if _, err := a.GetToken(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.GetToken(); err != nil {
		t.Fatal(err)
	}
	if tokenCalls != 1 {
		t.Fatalf("tokenCalls = %d, want 1 (reuse until expiry)", tokenCalls)
	}
}

func TestBasicAuth_NoProactiveKeepAlive(t *testing.T) {
	var tokenCalls, keepAliveCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "keep-alive"):
			keepAliveCalls++
			_ = json.NewEncoder(w).Encode(TokenResponse{Token: "kept", Expires: ""})
		case strings.Contains(r.URL.Path, "/api/v1/auth/token"):
			tokenCalls++
			_ = json.NewEncoder(w).Encode(TokenResponse{Token: "t1", Expires: ""})
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	a := NewAuthenticator(Config{
		BaseURL:  srv.URL,
		Username: "u",
		Password: "p",
	})
	if _, err := a.GetToken(); err != nil {
		t.Fatal(err)
	}
	// Simulate near-expiry memory token (old proactive window would keep-alive).
	a.tokenExpiry = time.Now().Add(2 * time.Minute)
	if _, err := a.GetToken(); err != nil {
		t.Fatal(err)
	}
	if keepAliveCalls != 0 {
		t.Fatalf("keep-alive calls = %d, want 0", keepAliveCalls)
	}
	if tokenCalls != 1 {
		t.Fatalf("tokenCalls = %d, want 1", tokenCalls)
	}
}
