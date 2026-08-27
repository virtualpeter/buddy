package buddy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIconSHA256HexAndHashURL(t *testing.T) {
	t.Parallel()
	// SHA-256("hello")
	got := IconSHA256Hex([]byte("hello"))
	want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if got != want {
		t.Fatalf("sha256 = %s want %s", got, want)
	}
	u := IconHashURL("https://ics.services.jamfcloud.com", got)
	if u != "https://ics.services.jamfcloud.com/icon/hash_"+want {
		t.Fatalf("url = %s", u)
	}
}

func TestParseIconJSON(t *testing.T) {
	t.Parallel()
	icon, err := parseIconJSON([]byte(`{"id":2,"name":"santa.png","url":"https://ics.example/icon/hash_abc"}`))
	if err != nil {
		t.Fatal(err)
	}
	if icon.ID != 2 || icon.Name != "santa.png" || icon.URL == "" {
		t.Fatalf("%+v", icon)
	}
}

func TestIconGetUploadDownload(t *testing.T) {
	var sawPostCT, sawDownloadAccept string
	var postBody []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "tok", "expires": ""})
	})
	mux.HandleFunc("/api/v1/icon/download/7", func(w http.ResponseWriter, r *http.Request) {
		sawDownloadAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("PNGDATA"))
	})
	mux.HandleFunc("/api/v1/icon/7", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": 7, "name": "santa.png", "url": "",
		})
	})
	mux.HandleFunc("/api/v1/icon", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		sawPostCT = r.Header.Get("Content-Type")
		postBody, _ = io.ReadAll(r.Body)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": 9, "name": "logo.png", "url": "https://ics.example/icon/hash_y",
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	SetActiveConfig(ConfigFromCredentialMap("icon-test", map[string]CredentialConfig{
		"icon-test": {
			URL: srv.URL,
			Auth: AuthConfig{
				Type:     "basic",
				Username: "u",
				Password: "p",
			},
		},
	}))
	clearCredAuthCache()
	t.Cleanup(func() {
		clearCredAuthCache()
		SetActiveConfig(nil)
	})

	got, err := IconGet("icon-test", 7)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 7 || got.Name != "santa.png" {
		t.Fatalf("get %+v", got)
	}

	data, err := IconDownload("icon-test", 7)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "PNGDATA" {
		t.Fatalf("download %q", data)
	}
	if sawDownloadAccept != "*/*" {
		t.Fatalf("download Accept %q", sawDownloadAccept)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "logo.png")
	if err := os.WriteFile(path, []byte("file-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	up, err := IconUpload("icon-test", path)
	if err != nil {
		t.Fatal(err)
	}
	if up.ID != 9 || up.Name != "logo.png" || up.URL == "" {
		t.Fatalf("upload %+v", up)
	}
	if !strings.HasPrefix(sawPostCT, "multipart/form-data") {
		t.Fatalf("content-type %q", sawPostCT)
	}
	if !strings.Contains(string(postBody), "logo.png") {
		t.Fatalf("multipart missing filename: %s", postBody)
	}

	up2, err := IconUploadBytes("icon-test", "santa.png", []byte("bytes"))
	if err != nil {
		t.Fatal(err)
	}
	if up2.ID != 9 {
		t.Fatalf("upload bytes %+v", up2)
	}
	if !strings.Contains(string(postBody), "santa.png") {
		t.Fatalf("upload bytes filename: %s", postBody)
	}
}

func TestNormalizeIconURL_DoubleSlash(t *testing.T) {
	t.Parallel()
	got := normalizeIconURL("https://example.jamfcloud.com//api/v1/icon/download/1")
	want := "https://example.jamfcloud.com/api/v1/icon/download/1"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	ics := "https://use1.ics.services.jamfcloud.com/icon/hash_abc"
	if normalizeIconURL(ics) != ics {
		t.Fatalf("ics url changed: %s", normalizeIconURL(ics))
	}
}

func TestParseIconJSON_NormalizesDoubleSlash(t *testing.T) {
	t.Parallel()
	icon, err := parseIconJSON([]byte(`{"id":1,"name":"systemsetting.png","url":"https://example.jamfcloud.com//api/v1/icon/download/1"}`))
	if err != nil {
		t.Fatal(err)
	}
	if icon.URL != "https://example.jamfcloud.com/api/v1/icon/download/1" {
		t.Fatalf("url %q", icon.URL)
	}
}

func TestIconDownloadPrefersMetadataURLWhenDownloadAPI500(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "tok", "expires": ""})
	})
	var metaURL string
	mux.HandleFunc("/api/v1/icon/2", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": 2, "name": "santa.png", "url": metaURL,
		})
	})
	mux.HandleFunc("/api/v1/icon/download/2", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"httpStatus":500,"errors":[]}`))
	})
	mux.HandleFunc("/cdn/santa.png", func(w http.ResponseWriter, r *http.Request) {
		if a := r.Header.Get("Authorization"); a != "" {
			t.Errorf("metadata URL must not send Authorization, got %q", a)
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("ICSPNG"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	metaURL = srv.URL + "/cdn/santa.png"

	SetActiveConfig(ConfigFromCredentialMap("icon-ics", map[string]CredentialConfig{
		"icon-ics": {
			URL: srv.URL,
			Auth: AuthConfig{
				Type:     "basic",
				Username: "u",
				Password: "p",
			},
		},
	}))
	clearCredAuthCache()
	t.Cleanup(func() {
		clearCredAuthCache()
		SetActiveConfig(nil)
	})

	data, err := IconDownload("icon-ics", 2)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "ICSPNG" {
		t.Fatalf("got %q", data)
	}
}

func TestIconDownloadFallsBackWhenURLMissing(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "tok", "expires": ""})
	})
	var missingURL string
	mux.HandleFunc("/api/v1/icon/3", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": 3, "name": "x.png", "url": missingURL,
		})
	})
	mux.HandleFunc("/api/v1/icon/download/3", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("FALLBACK"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	missingURL = srv.URL + "/missing"

	SetActiveConfig(ConfigFromCredentialMap("icon-fb", map[string]CredentialConfig{
		"icon-fb": {
			URL: srv.URL,
			Auth: AuthConfig{
				Type:     "basic",
				Username: "u",
				Password: "p",
			},
		},
	}))
	clearCredAuthCache()
	t.Cleanup(func() {
		clearCredAuthCache()
		SetActiveConfig(nil)
	})

	data, err := IconDownload("icon-fb", 3)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "FALLBACK" {
		t.Fatalf("got %q", data)
	}
}

func TestIconCheckHEADPresent(t *testing.T) {
	sum := IconSHA256Hex([]byte("hello"))
	var methods []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		if r.URL.Path != "/icon/hash_"+sum {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	prev := iconICSHosts
	iconICSHosts = []string{srv.URL}
	t.Cleanup(func() { iconICSHosts = prev })

	dir := t.TempDir()
	path := filepath.Join(dir, "a.png")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := IconCheck(path)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Present || !res.ICS || res.SHA256 != sum {
		t.Fatalf("%+v", res)
	}
	if res.URL != srv.URL+"/icon/hash_"+sum {
		t.Fatalf("url %s", res.URL)
	}
	if len(methods) != 1 || methods[0] != http.MethodHead {
		t.Fatalf("methods %v", methods)
	}
}

func TestIconCheckHEAD405FallsBackToGET(t *testing.T) {
	var methods []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method+" "+r.Header.Get("Range"))
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.Method == http.MethodGet && r.Header.Get("Range") == "bytes=0-0" {
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write([]byte("x"))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	prev := iconICSHosts
	iconICSHosts = []string{srv.URL}
	t.Cleanup(func() { iconICSHosts = prev })

	res, err := iconCheckBytes([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if !res.Present {
		t.Fatalf("want present: %+v methods %v", res, methods)
	}
	if !res.ICS {
		t.Fatalf("want ics: %+v", res)
	}
	if len(methods) < 2 || methods[0] != "HEAD " {
		t.Fatalf("methods %v", methods)
	}
}

func TestIconCheckMissingUsesCanonicalURL(t *testing.T) {
	sum := IconSHA256Hex([]byte("hello"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	prev := iconICSHosts
	iconICSHosts = []string{srv.URL, srv.URL}
	t.Cleanup(func() { iconICSHosts = prev })

	res, err := iconCheckBytes([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Present || res.ICS {
		t.Fatalf("want absent: %+v", res)
	}
	if res.URL != IconHashURL(iconICSCanonicalHost, sum) {
		t.Fatalf("canonical url %s", res.URL)
	}
}

func TestIconCheckSecondHostWins(t *testing.T) {
	sum := IconSHA256Hex([]byte("hello"))
	miss := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer miss.Close()
	hit := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer hit.Close()

	prev := iconICSHosts
	iconICSHosts = []string{miss.URL, hit.URL}
	t.Cleanup(func() { iconICSHosts = prev })

	res, err := iconCheckBytes([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if !res.Present || !res.ICS || res.URL != hit.URL+"/icon/hash_"+sum {
		t.Fatalf("%+v", res)
	}
}

func TestIconCheckICS403IsAbsent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><Error><Code>AccessDenied</Code></Error>`))
	}))
	defer srv.Close()

	prev := iconICSHosts
	iconICSHosts = []string{srv.URL}
	t.Cleanup(func() { iconICSHosts = prev })

	res, err := iconCheckBytes([]byte("systemsetting"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Present || res.ICS {
		t.Fatalf("403 must be absent: %+v", res)
	}
}

func TestIconCheckFileMatchesInstanceIcon(t *testing.T) {
	payload := []byte("instance-png")
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "tok", "expires": ""})
	})
	var metaURL string
	mux.HandleFunc("/api/v1/icon/1", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": 1, "name": "systemsetting.png", "url": metaURL,
		})
	})
	mux.HandleFunc("/api/v1/icon/download/1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(payload)
	})
	mux.HandleFunc("/dl/1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(payload)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	metaURL = srv.URL + "/dl/1"

	ics := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ics.Close()
	prev := iconICSHosts
	iconICSHosts = []string{ics.URL}
	t.Cleanup(func() { iconICSHosts = prev })

	SetActiveConfig(ConfigFromCredentialMap("icon-chk", map[string]CredentialConfig{
		"icon-chk": {
			URL:  srv.URL,
			Auth: AuthConfig{Type: "basic", Username: "u", Password: "p"},
		},
	}))
	clearCredAuthCache()
	t.Cleanup(func() {
		clearCredAuthCache()
		SetActiveConfig(nil)
	})

	dir := t.TempDir()
	path := filepath.Join(dir, "systemsetting.png")
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatal(err)
	}

	icsOnly, err := IconCheck(path)
	if err != nil {
		t.Fatal(err)
	}
	if icsOnly.Present || icsOnly.ICS {
		t.Fatalf("instance-only bytes must miss ICS: %+v", icsOnly)
	}

	res, err := IconCheckFile("icon-chk", path, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Present || res.ICS || res.IconID != 1 {
		t.Fatalf("%+v", res)
	}
	if res.URL != metaURL {
		t.Fatalf("url %s want %s", res.URL, metaURL)
	}

	other := filepath.Join(dir, "other.png")
	if err := os.WriteFile(other, []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	miss, err := IconCheckFile("icon-chk", other, 1)
	if err != nil {
		t.Fatal(err)
	}
	if miss.Present || miss.ICS || miss.IconID != 1 {
		t.Fatalf("mismatch: %+v", miss)
	}
}
