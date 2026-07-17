package tokenauth

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDiskTokenRoundTrip(t *testing.T) {
	dir := t.TempDir()
	SetTokenCacheDirForTest(dir)
	t.Cleanup(func() { SetTokenCacheDirForTest("") })

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

	tok, gotExp, ok := loadDiskToken("default", "https://jamf.example.com", 3*time.Minute)
	if !ok || tok != "tok-abc" {
		t.Fatalf("load ok=%v tok=%q", ok, tok)
	}
	if gotExp.Sub(expiry) > time.Second || expiry.Sub(gotExp) > time.Second {
		t.Fatalf("expiry mismatch got=%v want=%v", gotExp, expiry)
	}

	// Wrong URL → miss
	if _, _, ok := loadDiskToken("default", "https://other.example.com", 3*time.Minute); ok {
		t.Fatal("expected URL mismatch miss")
	}

	removeDiskToken("default")
	if _, _, ok := loadDiskToken("default", "https://jamf.example.com", 3*time.Minute); ok {
		t.Fatal("expected miss after remove")
	}
}

func TestLoadDiskTokenNearExpiry(t *testing.T) {
	dir := t.TempDir()
	SetTokenCacheDirForTest(dir)
	t.Cleanup(func() { SetTokenCacheDirForTest("") })

	expiry := time.Now().Add(2 * time.Minute)
	if err := saveDiskToken("c", "https://jamf.example.com", "tok", expiry); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := loadDiskToken("c", "https://jamf.example.com", 3*time.Minute); ok {
		t.Fatal("expected miss when within minRemaining")
	}
}

func TestAuthenticatorDiskCache(t *testing.T) {
	dir := t.TempDir()
	SetTokenCacheDirForTest(dir)
	t.Cleanup(func() { SetTokenCacheDirForTest("") })

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
