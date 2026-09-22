package buddy

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCheckUpdatesFlag(t *testing.T) {
	var nilCfg *Config
	if nilCfg.CheckUpdatesCLI() || !nilCfg.CheckUpdatesGUI() {
		t.Fatal("nil config: CLI off, GUI on")
	}
	cfg := &Config{}
	if cfg.CheckUpdatesCLI() || !cfg.CheckUpdatesGUI() {
		t.Fatal("omitted: CLI off, GUI on")
	}
	cfg.Features.CheckUpdates = boolPtr(true)
	if !cfg.CheckUpdatesCLI() || !cfg.CheckUpdatesGUI() {
		t.Fatal("true: CLI and GUI on")
	}
	cfg.Features.CheckUpdates = boolPtr(false)
	if cfg.CheckUpdatesCLI() || cfg.CheckUpdatesGUI() {
		t.Fatal("false: CLI and GUI off")
	}

	data, err := marshalConfig(&Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "check_updates") {
		t.Fatalf("omitted should not be written:\n%s", data)
	}
	off := &Config{}
	off.Features.CheckUpdates = boolPtr(false)
	data, err = marshalConfig(off, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "check_updates: false") {
		t.Fatalf("explicit false must be stored:\n%s", data)
	}
	on := &Config{}
	on.Features.CheckUpdates = boolPtr(true)
	data, err = marshalConfig(on, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "check_updates: true") {
		t.Fatalf("explicit true must be stored:\n%s", data)
	}

	parsed, _, err := parseAndMigrateUserYAML([]byte("features:\n  check_updates: false\n"))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.CheckUpdatesGUI() || parsed.CheckUpdatesCLI() {
		t.Fatal("YAML false should disable both")
	}
}

func TestReleaseNewer(t *testing.T) {
	if !releaseNewer("v12.1.5", "v12.1.6") {
		t.Fatal("newer tag")
	}
	if releaseNewer("v12.1.6", "v12.1.5") || releaseNewer("v12.1.5", "v12.1.5") {
		t.Fatal("not newer")
	}
	if releaseNewer("dev", "v12.1.6") || releaseNewer("v12.1.5-dirty", "v12.1.6") || releaseNewer("abc1234", "v12.1.6") {
		t.Fatal("non-release current must stay quiet")
	}
	if validReleaseURL("https://github.com/virtualpeter/buddy/releases/tag/v12.1.6") == false {
		t.Fatal("release page")
	}
	if validReleaseURL("https://example.com/virtualpeter/buddy/releases/tag/v1") ||
		validReleaseURL("https://github.com/virtualpeter/buddy/archive/v12.1.6.tar.gz") {
		t.Fatal("reject non-release URLs")
	}
}

func TestUpdateCheckCacheAndFetch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update-check.json")
	t.Setenv("BUDDY_UPDATE_CHECK_PATH", path)

	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing user agent")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v12.2.0","html_url":"https://github.com/virtualpeter/buddy/releases/tag/v12.2.0"}`))
	}))
	t.Cleanup(srv.Close)
	prevURL := updateReleaseURL
	updateReleaseURL = srv.URL
	t.Cleanup(func() { updateReleaseURL = prevURL })

	n := RefreshUpdateCheck(t.Context(), "dev")
	if n.Update || !n.Checked || hits != 0 {
		t.Fatalf("dev should stay quiet without a fetch: %+v hits=%d", n, hits)
	}

	n = RefreshUpdateCheck(t.Context(), "v12.1.5")
	if !n.Update || n.Version != "v12.2.0" || n.URL != "https://github.com/virtualpeter/buddy/releases/tag/v12.2.0" {
		t.Fatalf("want update notice, got %+v", n)
	}
	if hits != 1 {
		t.Fatalf("hits=%d", hits)
	}
	again := RefreshUpdateCheck(t.Context(), "v12.1.5")
	if !again.Update || hits != 1 {
		t.Fatalf("fresh cache should skip the network: %+v hits=%d", again, hits)
	}

	stale := updateCacheFile{
		CheckedAt: time.Now().Add(-25 * time.Hour).UTC().Format(time.RFC3339),
		Current:   "v12.1.5",
		Latest:    "v12.2.0",
		URL:       "https://github.com/virtualpeter/buddy/releases/tag/v12.2.0",
		OK:        true,
	}
	if err := writeUpdateCache(stale); err != nil {
		t.Fatal(err)
	}
	if cached := CachedUpdateNotice("v12.1.5"); cached.Checked {
		t.Fatal("stale cache should not count as checked")
	}
	n = RefreshUpdateCheck(t.Context(), "v12.1.5")
	if !n.Update || hits != 2 {
		t.Fatalf("stale cache should fetch again: %+v hits=%d", n, hits)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateCheckKeepsPriorNoticeOnFailure(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BUDDY_UPDATE_CHECK_PATH", filepath.Join(dir, "update-check.json"))
	prev := updateCacheFile{
		CheckedAt: time.Now().Add(-25 * time.Hour).UTC().Format(time.RFC3339),
		Current:   "v12.1.5",
		Latest:    "v12.2.0",
		URL:       "https://github.com/virtualpeter/buddy/releases/tag/v12.2.0",
		OK:        true,
	}
	if err := writeUpdateCache(prev); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)
	prevURL := updateReleaseURL
	updateReleaseURL = srv.URL
	t.Cleanup(func() { updateReleaseURL = prevURL })

	n := RefreshUpdateCheck(t.Context(), "v12.1.5")
	if !n.Update || n.Version != "v12.2.0" {
		t.Fatalf("failed fetch should keep the previous notice: %+v", n)
	}
	if cached := CachedUpdateNotice("v12.1.5"); !cached.Checked || !cached.Update {
		t.Fatalf("failure should still count for the day: %+v", cached)
	}
}
