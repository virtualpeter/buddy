package serve

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

var pngMagic = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

func TestLogoPNGEmbedded(t *testing.T) {
	if len(logoPNG) < len(pngMagic) {
		t.Fatalf("embedded logo too small: %d bytes", len(logoPNG))
	}
	for i, b := range pngMagic {
		if logoPNG[i] != b {
			t.Fatalf("embedded logo is not a PNG (byte %d)", i)
		}
	}
}

func TestServeLogo(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/buddy.png", nil)
	serveLogo(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("content-type %q", ct)
	}
	if rec.Body.Len() != len(logoPNG) {
		t.Fatalf("body %d want %d", rec.Body.Len(), len(logoPNG))
	}

	head := httptest.NewRecorder()
	serveLogo(head, httptest.NewRequest(http.MethodHead, "/favicon.ico", nil))
	if head.Code != http.StatusOK {
		t.Fatalf("HEAD status %d", head.Code)
	}
	if head.Body.Len() != 0 {
		t.Fatalf("HEAD should not write a body (%d bytes)", head.Body.Len())
	}

	bad := httptest.NewRecorder()
	serveLogo(bad, httptest.NewRequest(http.MethodPost, "/buddy.png", nil))
	if bad.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status %d want 405", bad.Code)
	}
}
