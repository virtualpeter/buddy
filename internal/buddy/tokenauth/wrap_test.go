package tokenauth

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testWrapKey(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, tokenWrapKeySize)
	if _, err := rand.Read(k); err != nil {
		t.Fatal(err)
	}
	return k
}

func TestV1StillLoadsWithoutWrapKey(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(SetTokenCacheDirForTest(dir))
	t.Cleanup(SetTokenWrapKeyForTest([]byte{})) // compiled-in

	if err := saveDiskToken("v1", "https://jamf.example.com", "tok-v1", time.Now().Add(20*time.Minute)); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	if parseDiskTokenVersion(raw) != diskTokenVersionV1 {
		t.Fatalf("version=%d want v1", parseDiskTokenVersion(raw))
	}
	tok, _, ok := loadDiskToken("v1", "https://jamf.example.com", clockSkewWindow)
	if !ok || tok != "tok-v1" {
		t.Fatalf("ok=%v tok=%q", ok, tok)
	}
}

func TestV1MigratesToV2WhenWrapPresent(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(SetTokenCacheDirForTest(dir))
	t.Cleanup(SetTokenWrapKeyForTest([]byte{}))

	if err := saveDiskToken("mig", "https://jamf.example.com", "tok-mig", time.Now().Add(20*time.Minute)); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "mig.json"))
	if parseDiskTokenVersion(raw) != diskTokenVersionV1 {
		t.Fatal("expected v1 seed")
	}

	t.Cleanup(SetTokenWrapKeyForTest(testWrapKey(t)))
	tok, _, ok := loadDiskToken("mig", "https://jamf.example.com", clockSkewWindow)
	if !ok || tok != "tok-mig" {
		t.Fatalf("ok=%v tok=%q", ok, tok)
	}
	raw, _ = os.ReadFile(filepath.Join(dir, "mig.json"))
	if parseDiskTokenVersion(raw) != diskTokenVersionV2 {
		t.Fatalf("version=%d want v2 after migrate", parseDiskTokenVersion(raw))
	}
	tok2, _, ok := loadDiskToken("mig", "https://jamf.example.com", clockSkewWindow)
	if !ok || tok2 != "tok-mig" {
		t.Fatalf("reload ok=%v tok=%q", ok, tok2)
	}
}

func TestV2FailsClosedWithoutWrapKey(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(SetTokenCacheDirForTest(dir))
	wrap := testWrapKey(t)
	t.Cleanup(SetTokenWrapKeyForTest(wrap))

	if err := saveDiskToken("v2", "https://jamf.example.com", "secret-tok", time.Now().Add(20*time.Minute)); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "v2.json"))
	if parseDiskTokenVersion(raw) != diskTokenVersionV2 {
		t.Fatalf("version=%d want v2", parseDiskTokenVersion(raw))
	}

	t.Cleanup(SetTokenWrapKeyForTest([]byte{}))
	if _, _, ok := loadDiskToken("v2", "https://jamf.example.com", clockSkewWindow); ok {
		t.Fatal("v2 must not unwrap without wrap key")
	}
	if _, err := os.Stat(filepath.Join(dir, "v2.json")); !os.IsNotExist(err) {
		t.Fatal("undecodable v2 file should be purged on load")
	}
}

func TestParseWrapKey(t *testing.T) {
	k := testWrapKey(t)
	enc := encodeWrapKey(k)
	got, ok := parseWrapKey(enc)
	if !ok || !bytes.Equal(got, k) {
		t.Fatalf("ok=%v", ok)
	}
	if _, ok := parseWrapKey("not-a-wrap"); ok {
		t.Fatal("expected reject")
	}
}

func TestOAuthDebugLogOmitsAccessToken(t *testing.T) {
	const bearer = "super-secret-access-token-value"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(OAuth2TokenResponse{
			AccessToken: bearer,
			ExpiresIn:   1800,
			TokenType:   "Bearer",
		})
	}))
	t.Cleanup(srv.Close)

	t.Cleanup(SetTokenCacheDirForTest(t.TempDir()))
	t.Cleanup(SetTokenWrapKeyForTest([]byte{}))

	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	a := NewAuthenticator(Config{
		BaseURL:      srv.URL,
		ClientId:     "cid",
		ClientSecret: "sec",
		Logger:       log,
	})
	if _, err := a.GetToken(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, bearer) {
		t.Fatalf("debug log leaked access_token: %s", out)
	}
}
