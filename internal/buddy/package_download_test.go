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

func TestPackageIsUsable(t *testing.T) {
	t.Parallel()
	if packageIsUsable(&PackageRecord{CloudTransferStatus: packageCloudTransferReady}) {
		t.Fatal("READY without sha256 should not be usable")
	}
	if packageIsUsable(&PackageRecord{CloudTransferStatus: "PENDING", Sha256: "abc"}) {
		t.Fatal("PENDING with sha256 should not be usable")
	}
	if !packageIsUsable(&PackageRecord{CloudTransferStatus: packageCloudTransferReady, Sha256: "abc"}) {
		t.Fatal("READY with sha256 should be usable")
	}
}

func TestPackageHasComparableChecksum(t *testing.T) {
	t.Parallel()
	if packageHasComparableChecksum(&PackageRecord{}) {
		t.Fatal("empty")
	}
	if !packageHasComparableChecksum(&PackageRecord{Sha256: "aa"}) {
		t.Fatal("sha256")
	}
	if !packageHasComparableChecksum(&PackageRecord{Md5: "bb"}) {
		t.Fatal("md5")
	}
	if !packageHasComparableChecksum(&PackageRecord{HashType: "SHA-256", HashValue: "cc"}) {
		t.Fatal("hashValue SHA-256")
	}
	if packageHasComparableChecksum(&PackageRecord{HashType: "SHA3_512", HashValue: "cc"}) {
		t.Fatal("SHA3 should not count")
	}
}

func TestPackageUploadSkipsWhenPendingNoChecksum(t *testing.T) {
	t.Parallel()
	// Mid-transfer: empty comparable checksums + sha3512 + not READY → wait, not replace.
	pkg := &PackageRecord{
		PackageName:         "Chrome",
		CloudTransferStatus: "PENDING",
		Sha3512:             "abc123sha3512",
	}
	local := packageFileHashes{MD5: "dead", SHA256: "beef"}
	match, _ := packageFileMatches(pkg, local)
	if match {
		t.Fatal("should not match without server checksums")
	}
	if !packageUploadShouldWaitNotReady(pkg) {
		t.Fatal("expected pending wait path (sha3512 set, no sha256)")
	}
	// Failed upload: no checksums at all → re-upload allowed.
	failed := &PackageRecord{
		PackageName:         "Chrome",
		CloudTransferStatus: "",
	}
	if packageUploadShouldWaitNotReady(failed) {
		t.Fatal("empty checksums without sha3512 should allow re-upload")
	}
	// READY with no checksums still means we cannot prove identity — upload path allowed.
	readyEmpty := &PackageRecord{CloudTransferStatus: packageCloudTransferReady}
	if packageUploadShouldWaitNotReady(readyEmpty) {
		t.Fatal("READY empty checksums should not take pending wait path")
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
