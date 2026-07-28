package buddy

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"strings"
	"testing"
	"time"
)

func TestWaitUntilPackageReadable_SucceedsAfter404s(t *testing.T) {
	t.Parallel()
	attempts := 0
	gets, elapsed, err := waitUntilPackageReadable("42", func() (bool, error) {
		attempts++
		if attempts < 3 {
			return false, nil
		}
		return true, nil
	}, time.Minute, time.Millisecond, 2*time.Millisecond, func(time.Duration) {})
	if err != nil {
		t.Fatalf("waitUntilPackageReadable: %v", err)
	}
	if attempts != 3 || gets != 3 {
		t.Fatalf("attempts/gets = %d/%d, want 3/3", attempts, gets)
	}
	if elapsed < 0 {
		t.Fatalf("elapsed = %v", elapsed)
	}
}

func TestWaitUntilPackageReadable_Timeout(t *testing.T) {
	t.Parallel()
	gets, _, err := waitUntilPackageReadable("99", func() (bool, error) {
		return false, nil
	}, 5*time.Millisecond, time.Millisecond, time.Millisecond, func(d time.Duration) {
		time.Sleep(d)
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "not readable within") {
		t.Fatalf("unexpected error: %v", err)
	}
	if gets < 1 {
		t.Fatalf("gets = %d, want >= 1", gets)
	}
}

func TestWaitUntilPackageReadable_ProbeError(t *testing.T) {
	t.Parallel()
	want := errors.New("boom")
	gets, _, err := waitUntilPackageReadable("7", func() (bool, error) {
		return false, want
	}, time.Minute, time.Millisecond, time.Millisecond, func(time.Duration) {})
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	if gets != 1 {
		t.Fatalf("gets = %d, want 1", gets)
	}
}

func TestPackageAttrsApplyAndDiffers(t *testing.T) {
	t.Parallel()
	name := "Chrome"
	prio := 5
	reboot := true
	attrs := PackageAttrs{Name: &name, Priority: &prio, Reboot: &reboot}
	if err := attrs.Validate(); err != nil {
		t.Fatal(err)
	}
	pkg := &PackageRecord{PackageName: "Old", Priority: 10, RebootRequired: false}
	if !attrs.DiffersFrom(pkg) {
		t.Fatal("expected DiffersFrom")
	}
	attrs.Apply(pkg)
	if pkg.PackageName != "Chrome" || pkg.Priority != 5 || !pkg.RebootRequired {
		t.Fatalf("apply: %+v", pkg)
	}
	if attrs.DiffersFrom(pkg) {
		t.Fatal("expected no diff after apply")
	}
	bad := 99
	if err := (PackageAttrs{Priority: &bad}).Validate(); err == nil {
		t.Fatal("expected priority validation error")
	}
}

func TestIsPackageUploadNotFound(t *testing.T) {
	t.Parallel()
	if isPackageUploadNotFound(errors.New("other")) {
		t.Fatal("unexpected match")
	}
	if isPackageUploadNotFound(errors.New("x: " + errPackageUploadNotFound.Error())) {
		t.Fatal("plain string should not match errors.Is")
	}
	wrapped := fmt.Errorf("%w: status %d: %s", errPackageUploadNotFound, 404, "missing")
	if !isPackageUploadNotFound(wrapped) {
		t.Fatal("expected match for %w-wrapped errPackageUploadNotFound")
	}
}

func TestMultipartFormFileLength_MatchesStreamedBody(t *testing.T) {
	t.Parallel()
	filename := "Chrome-121.pkg"
	payload := []byte("hello-package-bytes")

	var buf strings.Builder
	w := multipart.NewWriter(&buf)
	contentType, contentLength, err := multipartFormFileLength(w.Boundary(), filename, int64(len(payload)))
	if err != nil {
		t.Fatalf("multipartFormFileLength: %v", err)
	}
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write(payload); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	body := buf.String()
	if int64(len(body)) != contentLength {
		t.Fatalf("contentLength = %d, actual body = %d", contentLength, len(body))
	}
	if !strings.HasPrefix(contentType, "multipart/form-data; boundary=") {
		t.Fatalf("contentType = %q", contentType)
	}
	if !strings.Contains(body, filename) {
		t.Fatal("body missing filename")
	}
	if !strings.Contains(body, string(payload)) {
		t.Fatal("body missing payload")
	}
}

func TestProgressReader_ReportsTotal(t *testing.T) {
	t.Parallel()
	var lastRead, lastTotal int64
	r := &progressReader{
		r:     strings.NewReader("abcdef"),
		total: 6,
		onProgress: func(read, total int64) {
			lastRead, lastTotal = read, total
		},
	}
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != "abcdef" {
		t.Fatalf("got %q", got)
	}
	if lastRead != 6 || lastTotal != 6 {
		t.Fatalf("progress = %d/%d, want 6/6", lastRead, lastTotal)
	}
}

func TestFormatProgressBar(t *testing.T) {
	t.Parallel()
	s := formatProgressBar(75, 100)
	if !strings.Contains(s, "75%") {
		t.Fatalf("bar = %q", s)
	}
	if formatProgressBar(1, 0) != "" {
		t.Fatal("expected empty for total<=0")
	}
}

func TestFormatUploadDurationAndRate(t *testing.T) {
	t.Parallel()
	if got := formatUploadDuration(250 * time.Millisecond); got != "250ms" {
		t.Fatalf("duration = %q, want 250ms", got)
	}
	if got := formatUploadDuration(12500 * time.Millisecond); got != "12.5s" {
		t.Fatalf("duration = %q, want 12.5s", got)
	}
	// 10 MiB in 2s → 5.0 MB/s (formatBytes uses 1024-based units labeled KB/MB)
	rate := formatUploadRate(10*1024*1024, 2*time.Second)
	if !strings.HasSuffix(rate, "/s") || !strings.Contains(rate, "5.0") {
		t.Fatalf("rate = %q", rate)
	}
}

func TestHashPackageReaderAndMatch(t *testing.T) {
	t.Parallel()
	payload := []byte("hello-package-bytes")
	local, err := hashPackageReader(strings.NewReader(string(payload)))
	if err != nil {
		t.Fatalf("hashPackageReader: %v", err)
	}
	if local.MD5 == "" || local.SHA256 == "" {
		t.Fatalf("empty hashes: %+v", local)
	}

	pkg := &PackageRecord{Sha256: local.SHA256, Md5: "deadbeef"}
	ok, field := packageFileMatches(pkg, local)
	if !ok || field != "sha256" {
		t.Fatalf("match = %v %q, want sha256", ok, field)
	}

	pkg = &PackageRecord{Md5: local.MD5}
	ok, field = packageFileMatches(pkg, local)
	if !ok || field != "md5" {
		t.Fatalf("match = %v %q, want md5", ok, field)
	}

	pkg = &PackageRecord{HashType: "MD5", HashValue: local.MD5}
	ok, field = packageFileMatches(pkg, local)
	if !ok || field != "hashValue(MD5)" {
		t.Fatalf("match = %v %q, want hashValue(MD5)", ok, field)
	}

	pkg = &PackageRecord{HashType: "SHA3_512", HashValue: "abc"}
	ok, field = packageFileMatches(pkg, local)
	if ok || field != "" {
		t.Fatalf("expected no match for SHA3-only server hash, got %v %q", ok, field)
	}

	pkg = &PackageRecord{}
	ok, _ = packageFileMatches(pkg, local)
	if ok {
		t.Fatal("empty server hashes should not match")
	}
}
