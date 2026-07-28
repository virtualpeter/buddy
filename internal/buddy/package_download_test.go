package buddy

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackageIsCloudReady(t *testing.T) {
	t.Parallel()
	if packageIsCloudReady(nil) {
		t.Fatal("nil should not be ready")
	}
	if packageIsCloudReady(&PackageRecord{}) {
		t.Fatal("empty status should not be ready")
	}
	if packageIsCloudReady(&PackageRecord{CloudTransferStatus: "PENDING"}) {
		t.Fatal("PENDING should not be ready")
	}
	if packageIsCloudReady(&PackageRecord{CloudTransferStatus: "ready"}) {
		t.Fatal("lowercase ready should not match (case-sensitive)")
	}
	if !packageIsCloudReady(&PackageRecord{CloudTransferStatus: packageCloudTransferReady}) {
		t.Fatal("READY should be ready")
	}
}

func TestJcdsStreamVerifyToFile_OK(t *testing.T) {
	t.Parallel()
	payload := []byte("buddy-pkg-download-bytes")
	sum := sha256.Sum256(payload)
	want := hex.EncodeToString(sum[:])

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "24")
		_, _ = w.Write(payload)
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	out := filepath.Join(dir, "out.pkg")
	n, got, err := jcdsStreamVerifyToFile(srv.URL, out, want, nil)
	if err != nil {
		t.Fatalf("jcdsStreamVerifyToFile: %v", err)
	}
	if n != int64(len(payload)) {
		t.Fatalf("n = %d, want %d", n, len(payload))
	}
	if !strings.EqualFold(got, want) {
		t.Fatalf("got sha = %s, want %s", got, want)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != string(payload) {
		t.Fatalf("file contents mismatch")
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.Contains(e.Name(), ".buddy-pkg-") {
			t.Fatalf("temp file left behind: %s", e.Name())
		}
	}
}

func TestJcdsStreamVerifyToFile_MismatchDeletesTemp(t *testing.T) {
	t.Parallel()
	payload := []byte("wrong-bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	out := filepath.Join(dir, "out.pkg")
	_, _, err := jcdsStreamVerifyToFile(srv.URL, out, "deadbeef", nil)
	if err == nil || !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("expected sha256 mismatch, got %v", err)
	}
	if _, err := os.Stat(out); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("outfile should not exist after mismatch, err=%v", err)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".buddy-pkg-") || strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("temp file left behind: %s", e.Name())
		}
	}
}

func TestJcdsStreamVerifyToFile_HTTP403(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, "expired")
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	out := filepath.Join(dir, "out.pkg")
	_, _, err := jcdsStreamVerifyToFile(srv.URL, out, "abc", nil)
	var httpErr *jcdsHTTPStatusError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusForbidden {
		t.Fatalf("expected jcdsHTTPStatusError 403, got %v", err)
	}
}
