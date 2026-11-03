package buddy

import (
	"context"
	"crypto/sha3"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestRetryPackageUploadSucceedsAfter500(t *testing.T) {
	t.Parallel()
	var pauses []time.Duration
	n := 0
	got, elapsed, err := retryPackageUpload(packageUploadMaxAttempts, packageUploadRetryPauses, func(d time.Duration) {
		pauses = append(pauses, d)
	}, func() (int64, time.Duration, error) {
		n++
		if n < 3 {
			return 0, 0, &packageUploadStatusError{Code: 500, Body: "busy"}
		}
		return 42, time.Second, nil
	})
	if err != nil {
		t.Fatalf("retryPackageUpload: %v", err)
	}
	if got != 42 || elapsed != time.Second {
		t.Fatalf("result = %d %v", got, elapsed)
	}
	if n != 3 {
		t.Fatalf("attempts = %d, want 3", n)
	}
	if len(pauses) != 2 || pauses[0] != time.Second || pauses[1] != 2*time.Second {
		t.Fatalf("pauses = %v, want 1s, 2s", pauses)
	}
}

func TestRetryPackageUploadNoRetryOn400(t *testing.T) {
	t.Parallel()
	n := 0
	_, _, err := retryPackageUpload(packageUploadMaxAttempts, packageUploadRetryPauses, func(time.Duration) {
		t.Fatal("slept on a non-retryable status")
	}, func() (int64, time.Duration, error) {
		n++
		return 0, 0, &packageUploadStatusError{Code: 400, Body: "bad"}
	})
	if n != 1 {
		t.Fatalf("attempts = %d, want 1", n)
	}
	var st *packageUploadStatusError
	if !errors.As(err, &st) || st.Code != 400 {
		t.Fatalf("err = %v", err)
	}
}

func TestRetryPackageUploadStopsAfterSix(t *testing.T) {
	t.Parallel()
	var pauses []time.Duration
	n := 0
	_, _, err := retryPackageUpload(packageUploadMaxAttempts, packageUploadRetryPauses, func(d time.Duration) {
		pauses = append(pauses, d)
	}, func() (int64, time.Duration, error) {
		n++
		return 0, 0, &packageUploadStatusError{Code: 503, Body: "down"}
	})
	if n != packageUploadMaxAttempts {
		t.Fatalf("attempts = %d, want %d", n, packageUploadMaxAttempts)
	}
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second}
	if len(pauses) != len(want) {
		t.Fatalf("pauses = %v, want %v", pauses, want)
	}
	for i := range want {
		if pauses[i] != want[i] {
			t.Fatalf("pauses = %v, want %v", pauses, want)
		}
	}
	var st *packageUploadStatusError
	if !errors.As(err, &st) || st.Code != 503 {
		t.Fatalf("err = %v", err)
	}
}

func TestRetryPackageUploadTransportAndLocalRead(t *testing.T) {
	t.Parallel()
	if !isPackageUploadRetryable(fmt.Errorf("upload package file: %w", io.ErrUnexpectedEOF)) {
		t.Fatal("unexpected EOF should retry")
	}
	if !isPackageUploadRetryable(fmt.Errorf("upload package file: connection reset by peer")) {
		t.Fatal("connection reset should retry")
	}
	if isPackageUploadRetryable(&packageUploadStatusError{Code: 401, Body: "nope"}) {
		t.Fatal("401 should not retry")
	}
	if !isPackageUploadRetryable(fmt.Errorf("package 12: %w", errPackageUploadNotStored)) {
		t.Fatal("accepted upload with no stored file should retry")
	}
	if uploadRetryReason(errPackageUploadNotStored) != "no file stored" {
		t.Fatalf("reason = %q", uploadRetryReason(errPackageUploadNotStored))
	}
	readErr := fmt.Errorf("read package file: %w", &os.PathError{Op: "read", Path: "Chrome.pkg", Err: os.ErrNotExist})
	if isPackageUploadRetryable(readErr) {
		t.Fatal("local read should not retry")
	}
	n := 0
	_, _, err := retryPackageUpload(packageUploadMaxAttempts, packageUploadRetryPauses, func(time.Duration) {
		t.Fatal("slept on a local read error")
	}, func() (int64, time.Duration, error) {
		n++
		return 0, 0, readErr
	})
	if n != 1 || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("attempts=%d err=%v", n, err)
	}
}

func TestPackageUploadStatus404UnwrapsNotFound(t *testing.T) {
	t.Parallel()
	err := &packageUploadStatusError{Code: 404, Body: "missing"}
	if !isPackageUploadNotFound(err) || !isPackageUploadRetryable(err) {
		t.Fatalf("404 = notFound %v retry %v", isPackageUploadNotFound(err), isPackageUploadRetryable(err))
	}
}

func TestPerformHTTPStreamDoesNotBufferBody(t *testing.T) {
	t.Parallel()
	gotFirst := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1)
		if _, err := io.ReadFull(r.Body, buf); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		close(gotFirst)
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	pr, pw := io.Pipe()
	writeErr := make(chan error, 1)
	go func() {
		if _, err := pw.Write([]byte("A")); err != nil {
			writeErr <- err
			return
		}
		select {
		case <-gotFirst:
		case <-time.After(2 * time.Second):
			writeErr <- fmt.Errorf("server did not read the first byte before the rest of the body")
			_ = pw.Close()
			return
		}
		if _, err := pw.Write([]byte("BC")); err != nil {
			writeErr <- err
			return
		}
		writeErr <- pw.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL, pr)
	if err != nil {
		t.Fatal(err)
	}
	req.ContentLength = 3
	resp, err := performHTTPStream(req)
	if werr := <-writeErr; werr != nil && err == nil {
		err = werr
	}
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestPackageWaitInterruptIsSuccessOnlyForSentinel(t *testing.T) {
	t.Parallel()
	ch := make(chan struct{})
	close(ch)
	err := sleepOrInterrupt(time.Hour, ch)
	if !packageWaitInterruptOK(err) {
		t.Fatalf("closed interrupt = %v", err)
	}
	if packageWaitInterruptOK(errors.New("package 1 not cloudTransferStatus READY")) {
		t.Fatal("timeout must stay an error")
	}
	if err := sleepOrInterrupt(time.Millisecond, nil); err != nil {
		t.Fatalf("nil interrupt: %v", err)
	}
}

func TestClassifyPackageCheck(t *testing.T) {
	t.Parallel()
	local := packageFileHashes{MD5: "abc", SHA256: "def"}
	class, ok, matched, _ := classifyPackageCheck(nil, local)
	if class != PackageCheckMissing || ok || matched {
		t.Fatalf("missing: %s ok=%v matched=%v", class, ok, matched)
	}
	class, ok, matched, field := classifyPackageCheck(&PackageRecord{
		CloudTransferStatus: packageCloudTransferReady,
		Sha256:              local.SHA256,
	}, local)
	if class != PackageCheckOK || !ok || !matched || field != "sha256" {
		t.Fatalf("ok: %s ok=%v matched=%v field=%s", class, ok, matched, field)
	}
	class, ok, matched, _ = classifyPackageCheck(&PackageRecord{
		CloudTransferStatus: packageCloudTransferReady,
		Sha256:              "other",
	}, local)
	if class != PackageCheckMismatch || ok || matched {
		t.Fatalf("mismatch: %s ok=%v matched=%v", class, ok, matched)
	}
	class, ok, _, _ = classifyPackageCheck(&PackageRecord{
		CloudTransferStatus: "PENDING",
		HashValue:           "x",
	}, local)
	if class != PackageIssuePending || ok {
		t.Fatalf("pending: %s ok=%v", class, ok)
	}
	class, ok, _, _ = classifyPackageCheck(&PackageRecord{}, local)
	if class != PackageIssueFailedUpload || ok {
		t.Fatalf("failed: %s ok=%v", class, ok)
	}
	class, ok, _, _ = classifyPackageCheck(&PackageRecord{
		HashType:  "SHA3_512",
		HashValue: packageEmptyFileSHA3,
	}, local)
	if class != PackageIssueFailedUpload || ok {
		t.Fatalf("empty-file hash: %s ok=%v", class, ok)
	}
	class, ok, _, _ = classifyPackageCheck(&PackageRecord{
		CloudTransferStatus: packageCloudTransferReady,
	}, local)
	if class != PackageIssueUnusableReady || ok {
		t.Fatalf("unusable: %s ok=%v", class, ok)
	}
}

func TestPackageEmptyFileSHA3AndStoredFile(t *testing.T) {
	t.Parallel()
	sum := sha3.Sum512(nil)
	if hex.EncodeToString(sum[:]) != packageEmptyFileSHA3 {
		t.Fatalf("constant = %s, sha3-512(\"\") = %s", packageEmptyFileSHA3, hex.EncodeToString(sum[:]))
	}
	if packageRecordHasFile(nil) || packageRecordHasFile(&PackageRecord{HashValue: packageEmptyFileSHA3}) {
		t.Fatal("empty-file hash is not a stored file")
	}
	if packageRecordHasFile(&PackageRecord{HashValue: packageEmptyFileSHA3, Size: "0"}) {
		t.Fatal("size 0 is not a stored file")
	}
	if !packageRecordHasFile(&PackageRecord{HashValue: packageEmptyFileSHA3, Size: "12"}) {
		t.Fatal("non-zero size counts as stored")
	}
	if !packageRecordHasFile(&PackageRecord{HashType: "SHA3_512", HashValue: "abc"}) {
		t.Fatal("real hash counts as stored")
	}
	if !packageRecordHasFile(&PackageRecord{Sha256: "abc"}) {
		t.Fatal("sha256 counts as stored")
	}
}

func TestWaitUntilPackageFileStored(t *testing.T) {
	t.Parallel()
	n := 0
	err := waitUntilPackageFileStored("12", func() (bool, error) {
		n++
		return n >= 3, nil
	}, time.Second, time.Millisecond, 2*time.Millisecond, func(time.Duration) {})
	if err != nil || n != 3 {
		t.Fatalf("stored: n=%d err=%v", n, err)
	}

	n = 0
	err = waitUntilPackageFileStored("12", func() (bool, error) {
		n++
		return false, nil
	}, 5*time.Millisecond, time.Millisecond, 2*time.Millisecond, func(time.Duration) {})
	if !errors.Is(err, errPackageUploadNotStored) {
		t.Fatalf("err = %v", err)
	}
	if n < 2 {
		t.Fatalf("probes = %d, want more than one", n)
	}

	want := errors.New("get failed")
	err = waitUntilPackageFileStored("12", func() (bool, error) {
		return false, want
	}, time.Second, time.Millisecond, time.Millisecond, func(time.Duration) {
		t.Fatal("slept after probe error")
	})
	if !errors.Is(err, want) {
		t.Fatalf("err = %v", err)
	}
}

func TestRetryPackageUploadNotStored(t *testing.T) {
	t.Parallel()
	n := 0
	var pauses []time.Duration
	got, _, err := retryPackageUpload(packageUploadMaxAttempts, packageUploadRetryPauses, func(d time.Duration) {
		pauses = append(pauses, d)
	}, func() (int64, time.Duration, error) {
		n++
		if n == 1 {
			return 0, 0, fmt.Errorf("package 12: %w", errPackageUploadNotStored)
		}
		return 9, time.Second, nil
	})
	if err != nil || got != 9 || n != 2 || len(pauses) != 1 || pauses[0] != time.Second {
		t.Fatalf("got=%d n=%d pauses=%v err=%v", got, n, pauses, err)
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
	s := formatProgressBar("", 75, 100)
	if !strings.Contains(s, "75%") {
		t.Fatalf("bar = %q", s)
	}
	if formatProgressBar("", 1, 0) != "" {
		t.Fatal("expected empty for total<=0")
	}
	labeled := formatProgressBar("Uploading", 50, 100)
	if !strings.HasPrefix(labeled, "Uploading: ") || !strings.Contains(labeled, "50%") {
		t.Fatalf("labeled=%q", labeled)
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
	local, err := hashPackageReader(strings.NewReader(string(payload)), int64(len(payload)))
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

func TestPackageUploadShouldWaitNotReady(t *testing.T) {
	t.Parallel()
	if packageUploadShouldWaitNotReady(nil) {
		t.Fatal("nil")
	}
	if !packageUploadShouldWaitNotReady(&PackageRecord{
		CloudTransferStatus: "PENDING",
		HashType:            "SHA3_512",
		HashValue:           "x",
	}) {
		t.Fatal("pending with SHA3 hashValue")
	}
	if !packageUploadShouldWaitNotReady(&PackageRecord{
		CloudTransferStatus: "PENDING",
		HashValue:           "x", // hashType often omitted mid-transfer
	}) {
		t.Fatal("pending with hashValue and empty hashType")
	}
	if packageUploadShouldWaitNotReady(&PackageRecord{
		CloudTransferStatus: "PENDING",
		Sha3512:             "x", // sha3512 alone is not the signal
	}) {
		t.Fatal("sha3512 alone should not force wait")
	}
	if packageUploadShouldWaitNotReady(&PackageRecord{
		CloudTransferStatus: "PENDING",
	}) {
		t.Fatal("pending without hashValue is failed upload")
	}
	if packageUploadShouldWaitNotReady(&PackageRecord{
		CloudTransferStatus: "PENDING",
		HashType:            "SHA3_512",
		HashValue:           "x",
		Sha256:              "y",
	}) {
		t.Fatal("has comparable checksum — not wait-only")
	}
	if packageUploadShouldWaitNotReady(&PackageRecord{
		CloudTransferStatus: "PENDING",
		HashType:            "SHA_256",
		HashValue:           "y",
	}) {
		t.Fatal("SHA-256 hashValue is comparable, not pending-only evidence")
	}
	if packageUploadShouldWaitNotReady(&PackageRecord{
		CloudTransferStatus: packageCloudTransferReady,
		HashType:            "SHA3_512",
		HashValue:           "x",
	}) {
		t.Fatal("READY should not wait-not-ready")
	}
	if packageUploadShouldWaitNotReady(&PackageRecord{
		HashType:  "SHA3_512",
		HashValue: packageEmptyFileSHA3,
	}) {
		t.Fatal("SHA3-512 of an empty file is a failed upload, not mid-transfer")
	}
}

func TestPackageUploadWaitOnly(t *testing.T) {
	t.Parallel()
	pending := &PackageRecord{
		CloudTransferStatus: "PENDING",
		HashType:            "SHA3_512",
		HashValue:           "x",
	}
	if !packageUploadWaitOnly(PackageUploadOptions{WaitReady: true}, pending) {
		t.Fatal("wait-ready on pending should skip file upload and wait")
	}
	if packageUploadWaitOnly(PackageUploadOptions{}, pending) {
		t.Fatal("pending without wait-ready is not wait-only")
	}
	if packageUploadWaitOnly(PackageUploadOptions{WaitReady: true, Force: true}, pending) {
		t.Fatal("force should re-upload, not wait-only")
	}
	if packageUploadWaitOnly(PackageUploadOptions{WaitReady: true}, &PackageRecord{}) {
		t.Fatal("failed/empty upload should re-upload, not wait-only")
	}
}

func TestPackageIssueClass(t *testing.T) {
	t.Parallel()
	if packageIssueClass(nil) != "" {
		t.Fatal("nil")
	}
	if got := packageIssueClass(&PackageRecord{}); got != PackageIssueFailedUpload {
		t.Fatalf("empty record: %q", got)
	}
	if got := packageIssueClass(&PackageRecord{
		CloudTransferStatus: "PENDING",
		HashType:            "SHA3_512",
		HashValue:           "abc",
	}); got != PackageIssuePending {
		t.Fatalf("pending: %q", got)
	}
	if got := packageIssueClass(&PackageRecord{
		HashType:  "SHA3_512",
		HashValue: packageEmptyFileSHA3,
	}); got != PackageIssueFailedUpload {
		t.Fatalf("empty-file SHA3: %q", got)
	}
	if got := packageIssueClass(&PackageRecord{
		CloudTransferStatus: packageCloudTransferReady,
	}); got != PackageIssueUnusableReady {
		t.Fatalf("READY no sha256: %q", got)
	}
	if got := packageIssueClass(&PackageRecord{
		CloudTransferStatus: packageCloudTransferReady,
		Sha256:              "deadbeef",
	}); got != "" {
		t.Fatalf("healthy: %q", got)
	}
}

func TestParsePackageScanClasses(t *testing.T) {
	t.Parallel()
	all, err := ParsePackageScanClasses("")
	if err != nil || len(all) != 3 {
		t.Fatalf("default all: %v %v", all, err)
	}
	got, err := ParsePackageScanClasses("failed-upload,pending")
	if err != nil || len(got) != 2 || got[0] != PackageIssueFailedUpload {
		t.Fatalf("got %#v err=%v", got, err)
	}
	if _, err := ParsePackageScanClasses("nope"); err == nil {
		t.Fatal("expected error")
	}
}
