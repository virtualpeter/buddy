package buddy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestIconSHAFromURL(t *testing.T) {
	t.Parallel()
	sum := strings.Repeat("ab", 32)
	u := "https://ics.services.jamfcloud.com/icon/hash_" + sum
	if got := iconSHAFromURL(u); got != sum {
		t.Fatalf("got %q", got)
	}
	if iconSHAFromURL("https://example.jamfcloud.com/api/v1/icon/download/1") != "" {
		t.Fatal("instance url should not parse as hash")
	}
	if iconSHAFromURL("https://ics.example/icon/hash_abc") != "" {
		t.Fatal("short hash should not parse")
	}
}

func TestIconListWalksIdsAndStopsAfterTail(t *testing.T) {
	prevTail := iconScanTail
	iconScanTail = 3
	t.Cleanup(func() { iconScanTail = prevTail })

	icsSHA := strings.Repeat("ab", 32)
	instanceBytes := []byte("instance-png")
	var mu sync.Mutex
	var metaGets []int
	var instanceGets int

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "tok", "expires": ""})
	})
	var instanceURL string
	mux.HandleFunc("/api/v1/icon/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.Atoi(r.PathValue("id"))
		mu.Lock()
		metaGets = append(metaGets, id)
		mu.Unlock()
		switch id {
		case 1:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": 1, "name": "systemsetting.png", "url": instanceURL,
			})
		case 2:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": 2, "name": "santa.png",
				"url": "https://ics.services.jamfcloud.com/icon/hash_" + icsSHA,
			})
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("/dl/1", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		instanceGets++
		mu.Unlock()
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(instanceBytes)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	instanceURL = srv.URL + "/dl/1"

	useTempCatalogCache(t)
	SetActiveConfig(ConfigFromCredentialMap("icon-walk", map[string]CredentialConfig{
		"icon-walk": {
			URL:  srv.URL,
			Auth: AuthConfig{Type: "basic", Username: "u", Password: "p"},
		},
	}))
	clearCredAuthCache()
	t.Cleanup(func() {
		clearCredAuthCache()
		SetActiveConfig(nil)
	})

	res, err := IconList("icon-walk", IconListOpts{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.FromCache {
		t.Fatal("first list should walk")
	}
	if res.ScannedFrom != 1 || res.ScannedTo != 5 {
		t.Fatalf("scanned %d–%d want 1–5", res.ScannedFrom, res.ScannedTo)
	}
	if res.Total != 2 || len(res.Icons) != 2 {
		t.Fatalf("icons %+v", res.Icons)
	}
	mu.Lock()
	gots := append([]int(nil), metaGets...)
	gotInstance := instanceGets
	mu.Unlock()
	if len(gots) != 5 || gots[0] != 1 || gots[4] != 5 {
		t.Fatalf("meta GETs %v", gots)
	}
	if gotInstance != 1 {
		t.Fatalf("instance hash fetches %d", gotInstance)
	}
	if res.Icons[0].ID != 1 || res.Icons[0].SHA256 != IconSHA256Hex(instanceBytes) {
		t.Fatalf("icon 1 %+v", res.Icons[0])
	}
	if res.Icons[1].ID != 2 || res.Icons[1].SHA256 != icsSHA {
		t.Fatalf("icon 2 %+v", res.Icons[1])
	}

	cat, err := loadIconCatalog("icon-walk")
	if err != nil || cat == nil {
		t.Fatalf("cache: %v %+v", err, cat)
	}
	if cat.MaxID != 2 || cat.FetchedAt.IsZero() {
		t.Fatalf("cache envelope %+v", cat)
	}
}

func TestIconListEmptyTenantWritesCache(t *testing.T) {
	prevTail := iconScanTail
	iconScanTail = 3
	t.Cleanup(func() { iconScanTail = prevTail })

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "tok", "expires": ""})
	})
	mux.HandleFunc("/api/v1/icon/{id}", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	useTempCatalogCache(t)
	SetActiveConfig(ConfigFromCredentialMap("icon-empty", map[string]CredentialConfig{
		"icon-empty": {
			URL:  srv.URL,
			Auth: AuthConfig{Type: "basic", Username: "u", Password: "p"},
		},
	}))
	clearCredAuthCache()
	t.Cleanup(func() {
		clearCredAuthCache()
		SetActiveConfig(nil)
	})

	res, err := IconList("icon-empty", IconListOpts{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.FromCache || res.Total != 0 || res.ScannedTo != 3 {
		t.Fatalf("%+v", res)
	}
	cat, err := loadIconCatalog("icon-empty")
	if err != nil || cat == nil || cat.MaxID != 0 || cat.FetchedAt.IsZero() {
		t.Fatalf("cache: %v %+v", err, cat)
	}
}

func TestIconListFreshCacheDoesNotWalk(t *testing.T) {
	prevTail := iconScanTail
	iconScanTail = 3
	t.Cleanup(func() { iconScanTail = prevTail })

	var mu sync.Mutex
	var metaGets []int
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "tok", "expires": ""})
	})
	mux.HandleFunc("/api/v1/icon/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.Atoi(r.PathValue("id"))
		mu.Lock()
		metaGets = append(metaGets, id)
		mu.Unlock()
		http.NotFound(w, r)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	useTempCatalogCache(t)
	SetActiveConfig(ConfigFromCredentialMap("icon-fresh", map[string]CredentialConfig{
		"icon-fresh": {
			URL:  srv.URL,
			Auth: AuthConfig{Type: "basic", Username: "u", Password: "p"},
		},
	}))
	clearCredAuthCache()
	t.Cleanup(func() {
		clearCredAuthCache()
		SetActiveConfig(nil)
	})

	if err := saveIconCatalog("icon-fresh", IconCatalogCache{
		FetchedAt: time.Now().UTC(),
		Tenant:    "icon-fresh",
		MaxID:     2,
		Items: []Icon{
			{ID: 1, Name: "systemsetting.png", URL: "https://example/dl/1"},
			{ID: 2, Name: "santa.png", URL: "https://ics.example/icon/hash_x"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	res, err := IconList("icon-fresh", IconListOpts{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !res.FromCache || res.Total != 2 {
		t.Fatalf("%+v", res)
	}
	if len(metaGets) != 0 {
		t.Fatalf("fresh list walked %v", metaGets)
	}

	filtered, err := IconList("icon-fresh", IconListOpts{Filter: "santa"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered.Icons) != 1 || filtered.Icons[0].Name != "santa.png" || filtered.Total != 2 {
		t.Fatalf("filter %+v", filtered)
	}

	res2, err := IconList("icon-fresh", IconListOpts{Refresh: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res2.FromCache {
		t.Fatal("refresh should walk")
	}
	if iconSliceContains(metaGets, 1) || iconSliceContains(metaGets, 2) {
		t.Fatalf("refresh re-got cached ids: %v", metaGets)
	}
	if !equalIntSlice(metaGets, []int{3, 4, 5}) {
		t.Fatalf("refresh probes: %v", metaGets)
	}
	if res2.Total != 2 || res2.ScannedFrom != 3 || res2.ScannedTo != 5 {
		t.Fatalf("%+v", res2)
	}
}

func TestIconListRefreshWalksFromMaxID(t *testing.T) {
	prevTail := iconScanTail
	iconScanTail = 3
	t.Cleanup(func() { iconScanTail = prevTail })

	var mu sync.Mutex
	var metaGets []int
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "tok", "expires": ""})
	})
	mux.HandleFunc("/api/v1/icon/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.Atoi(r.PathValue("id"))
		mu.Lock()
		metaGets = append(metaGets, id)
		mu.Unlock()
		switch id {
		case 2:
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 2, "name": "gap.png", "url": "https://ics.example/icon/hash_" + strings.Repeat("cd", 32)})
		case 4:
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 4, "name": "new.png", "url": "https://ics.example/icon/hash_" + strings.Repeat("ef", 32)})
		default:
			http.NotFound(w, r)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	useTempCatalogCache(t)
	SetActiveConfig(ConfigFromCredentialMap("icon-incr", map[string]CredentialConfig{
		"icon-incr": {
			URL:  srv.URL,
			Auth: AuthConfig{Type: "basic", Username: "u", Password: "p"},
		},
	}))
	clearCredAuthCache()
	t.Cleanup(func() {
		clearCredAuthCache()
		SetActiveConfig(nil)
	})

	if err := saveIconCatalog("icon-incr", IconCatalogCache{
		FetchedAt: time.Now().UTC(),
		Tenant:    "icon-incr",
		MaxID:     3,
		Items:     []Icon{{ID: 1, Name: "cached.png", URL: "https://example/dl/1"}},
	}); err != nil {
		t.Fatal(err)
	}

	res, err := IconList("icon-incr", IconListOpts{Refresh: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.FromCache {
		t.Fatal("refresh should walk")
	}
	if iconSliceContains(metaGets, 1) || iconSliceContains(metaGets, 2) || iconSliceContains(metaGets, 3) {
		t.Fatalf("refresh re-got ids at or below max: %v", metaGets)
	}
	// from max_id+1; hit 4 grows stop to 7
	if !equalIntSlice(metaGets, []int{4, 5, 6, 7}) {
		t.Fatalf("probes: %v", metaGets)
	}
	if res.Total != 2 || res.ScannedFrom != 4 || res.ScannedTo != 7 {
		t.Fatalf("%+v", res)
	}
	gotIDs := make([]int, 0, len(res.Icons))
	for _, it := range res.Icons {
		gotIDs = append(gotIDs, it.ID)
	}
	if !equalIntSlice(gotIDs, []int{1, 4}) {
		t.Fatalf("icons %v", gotIDs)
	}
	if res.Icons[0].Name != "cached.png" {
		t.Fatalf("kept cached row %+v", res.Icons[0])
	}

	cat, err := loadIconCatalog("icon-incr")
	if err != nil || cat == nil {
		t.Fatalf("cache: %v %+v", err, cat)
	}
	if cat.MaxID != 4 {
		t.Fatalf("max_id %d", cat.MaxID)
	}
}

func TestIconListDropCacheWalksFromOne(t *testing.T) {
	prevTail := iconScanTail
	iconScanTail = 3
	t.Cleanup(func() { iconScanTail = prevTail })

	var mu sync.Mutex
	var metaGets []int
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "tok", "expires": ""})
	})
	mux.HandleFunc("/api/v1/icon/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.Atoi(r.PathValue("id"))
		mu.Lock()
		metaGets = append(metaGets, id)
		mu.Unlock()
		if id == 2 {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 2, "name": "live.png", "url": "https://ics.example/icon/hash_" + strings.Repeat("ab", 32)})
			return
		}
		http.NotFound(w, r)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	useTempCatalogCache(t)
	SetActiveConfig(ConfigFromCredentialMap("icon-drop", map[string]CredentialConfig{
		"icon-drop": {
			URL:  srv.URL,
			Auth: AuthConfig{Type: "basic", Username: "u", Password: "p"},
		},
	}))
	clearCredAuthCache()
	t.Cleanup(func() {
		clearCredAuthCache()
		SetActiveConfig(nil)
	})

	if err := saveIconCatalog("icon-drop", IconCatalogCache{
		FetchedAt: time.Now().UTC(),
		MaxID:     9,
		Items:     []Icon{{ID: 1, Name: "stale.png"}},
	}); err != nil {
		t.Fatal(err)
	}

	res, err := IconList("icon-drop", IconListOpts{DropCache: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.FromCache {
		t.Fatal("dropcache should walk")
	}
	if !equalIntSlice(metaGets, []int{1, 2, 3, 4, 5}) {
		t.Fatalf("dropcache probes: %v", metaGets)
	}
	if res.Total != 1 || res.Icons[0].ID != 2 || res.ScannedFrom != 1 || res.ScannedTo != 5 {
		t.Fatalf("%+v", res)
	}
}

func iconSliceContains(ids []int, want int) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

func equalIntSlice(got, want []int) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestIconListStaleCacheWalks(t *testing.T) {
	prevTail := iconScanTail
	iconScanTail = 2
	t.Cleanup(func() { iconScanTail = prevTail })

	var metaGets int
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "tok", "expires": ""})
	})
	mux.HandleFunc("/api/v1/icon/{id}", func(w http.ResponseWriter, r *http.Request) {
		metaGets++
		http.NotFound(w, r)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	useTempCatalogCache(t)
	SetActiveConfig(ConfigFromCredentialMap("icon-stale", map[string]CredentialConfig{
		"icon-stale": {
			URL:  srv.URL,
			Auth: AuthConfig{Type: "basic", Username: "u", Password: "p"},
		},
	}))
	clearCredAuthCache()
	t.Cleanup(func() {
		clearCredAuthCache()
		SetActiveConfig(nil)
	})

	if err := saveIconCatalog("icon-stale", IconCatalogCache{
		FetchedAt: time.Now().UTC().Add(-8 * 24 * time.Hour),
		MaxID:     0,
		Items:     []Icon{},
	}); err != nil {
		t.Fatal(err)
	}

	res, err := IconList("icon-stale", IconListOpts{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.FromCache || metaGets == 0 {
		t.Fatalf("stale should walk: %+v gets=%d", res, metaGets)
	}
}

func TestIconListAbortsOnNon404(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "tok", "expires": ""})
	})
	mux.HandleFunc("/api/v1/icon/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"httpStatus":500}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	useTempCatalogCache(t)
	SetActiveConfig(ConfigFromCredentialMap("icon-500", map[string]CredentialConfig{
		"icon-500": {
			URL:  srv.URL,
			Auth: AuthConfig{Type: "basic", Username: "u", Password: "p"},
		},
	}))
	clearCredAuthCache()
	t.Cleanup(func() {
		clearCredAuthCache()
		SetActiveConfig(nil)
	})

	_, err := IconList("icon-500", IconListOpts{}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	cat, _ := loadIconCatalog("icon-500")
	if cat != nil {
		t.Fatalf("should not write cache on abort: %+v", cat)
	}
}

func TestIconGetCacheHitAndMissUpsert(t *testing.T) {
	var liveGets int
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "tok", "expires": ""})
	})
	mux.HandleFunc("/api/v1/icon/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.Atoi(r.PathValue("id"))
		if id != 7 {
			http.NotFound(w, r)
			return
		}
		liveGets++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": 7, "name": "live.png", "url": "https://ics.example/icon/hash_" + strings.Repeat("cd", 32),
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	useTempCatalogCache(t)
	SetActiveConfig(ConfigFromCredentialMap("icon-get", map[string]CredentialConfig{
		"icon-get": {
			URL:  srv.URL,
			Auth: AuthConfig{Type: "basic", Username: "u", Password: "p"},
		},
	}))
	clearCredAuthCache()
	t.Cleanup(func() {
		clearCredAuthCache()
		SetActiveConfig(nil)
	})

	fetched := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	if err := saveIconCatalog("icon-get", IconCatalogCache{
		FetchedAt: fetched,
		MaxID:     1,
		Items:     []Icon{{ID: 1, Name: "cached.png", URL: "https://example/1"}},
	}); err != nil {
		t.Fatal(err)
	}

	hit, err := IconGet("icon-get", 1)
	if err != nil {
		t.Fatal(err)
	}
	if hit.Name != "cached.png" || liveGets != 0 {
		t.Fatalf("cache hit %+v liveGets=%d", hit, liveGets)
	}

	miss, err := IconGet("icon-get", 7)
	if err != nil {
		t.Fatal(err)
	}
	if miss.ID != 7 || miss.Name != "live.png" || liveGets != 1 {
		t.Fatalf("miss %+v liveGets=%d", miss, liveGets)
	}
	cat, err := loadIconCatalog("icon-get")
	if err != nil || cat == nil {
		t.Fatal(err)
	}
	if !cat.FetchedAt.Equal(fetched) {
		t.Fatalf("upsert must not reset fetched_at: %s", cat.FetchedAt)
	}
	if cat.MaxID != 7 {
		t.Fatalf("max_id %d", cat.MaxID)
	}
	if _, ok := iconCatalogGetByID("icon-get", 7); !ok {
		t.Fatal("upsert missing id 7")
	}

	if _, err := IconGet("icon-get", 7); err != nil {
		t.Fatal(err)
	}
	if liveGets != 1 {
		t.Fatalf("second get should be cache: liveGets=%d", liveGets)
	}
}

func TestIconGetByName(t *testing.T) {
	useTempCatalogCache(t)
	SetActiveConfig(ConfigFromCredentialMap("icon-name", map[string]CredentialConfig{
		"icon-name": {
			URL:  "https://example.invalid",
			Auth: AuthConfig{Type: "basic", Username: "u", Password: "p"},
		},
	}))
	clearCredAuthCache()
	t.Cleanup(func() {
		clearCredAuthCache()
		SetActiveConfig(nil)
	})

	if _, err := IconGetByName("icon-name", "santa.png"); err == nil {
		t.Fatal("missing cache should error")
	}

	if err := saveIconCatalog("icon-name", IconCatalogCache{
		FetchedAt: time.Now().UTC(),
		MaxID:     3,
		Items: []Icon{
			{ID: 2, Name: "santa.png"},
			{ID: 3, Name: "dup.png"},
			{ID: 4, Name: "dup.png"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	got, err := IconGetByName("icon-name", "santa.png")
	if err != nil || got.ID != 2 {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := IconGetByName("icon-name", "missing.png"); err == nil {
		t.Fatal("unknown name")
	}
	if _, err := IconGetByName("icon-name", "dup.png"); err == nil || !strings.Contains(err.Error(), "multiple icons") {
		t.Fatalf("dup: %v", err)
	}
}

func TestIconCheckFromCache(t *testing.T) {
	payload := []byte("cached-png")
	sum := IconSHA256Hex(payload)

	okSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer okSrv.Close()
	deny := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer deny.Close()

	useTempCatalogCache(t)
	SetActiveConfig(ConfigFromCredentialMap("icon-ck", map[string]CredentialConfig{
		"icon-ck": {
			URL:  "https://example.invalid",
			Auth: AuthConfig{Type: "basic", Username: "u", Password: "p"},
		},
	}))
	clearCredAuthCache()
	t.Cleanup(func() {
		clearCredAuthCache()
		SetActiveConfig(nil)
	})

	instanceURL := okSrv.URL + "/dl/1"
	if err := saveIconCatalog("icon-ck", IconCatalogCache{
		FetchedAt: time.Now().UTC(),
		MaxID:     1,
		Items: []Icon{{
			ID: 1, Name: "systemsetting.png", URL: instanceURL, SHA256: sum,
		}},
	}); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "systemsetting.png")
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := IconCheckFile("icon-ck", path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Present || res.ICS || res.IconID != 1 || res.URL != instanceURL {
		t.Fatalf("cache match %+v", res)
	}

	icsURL := deny.URL + "/icon/hash_" + sum
	if err := saveIconCatalog("icon-ck", IconCatalogCache{
		FetchedAt: time.Now().UTC(),
		MaxID:     2,
		Items: []Icon{{
			ID: 2, Name: "santa.png", URL: icsURL, SHA256: sum,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	res2, err := IconCheckFile("icon-ck", path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if res2.Present || res2.ICS || res2.IconID != 2 || res2.URL != icsURL {
		t.Fatalf("dead ics url %+v", res2)
	}
}

func TestIconCheckFromCacheDuplicateSHA(t *testing.T) {
	payload := []byte("dup-png")
	sum := IconSHA256Hex(payload)

	ics := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ics.Close()

	useTempCatalogCache(t)
	SetActiveConfig(ConfigFromCredentialMap("icon-dupsha", map[string]CredentialConfig{
		"icon-dupsha": {
			URL:  "https://example.invalid",
			Auth: AuthConfig{Type: "basic", Username: "u", Password: "p"},
		},
	}))
	clearCredAuthCache()
	t.Cleanup(func() {
		clearCredAuthCache()
		SetActiveConfig(nil)
	})

	u := ics.URL + "/icon/hash_" + sum
	if err := saveIconCatalog("icon-dupsha", IconCatalogCache{
		FetchedAt: time.Now().UTC(),
		MaxID:     12,
		Items: []Icon{
			{ID: 7, Name: "alloy_icon.png", URL: u, SHA256: sum},
			{ID: 12, Name: "alloy_icon.png", URL: u, SHA256: sum},
		},
	}); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "alloy_icon.png")
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := IconCheckFile("icon-dupsha", path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Present || !res.ICS || res.IconID != 7 {
		t.Fatalf("dup sha cache match %+v", res)
	}
}

func TestIconCheckProbesHostFromCacheURL(t *testing.T) {
	payload := []byte("regional-png")
	sum := IconSHA256Hex(payload)

	hit := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer hit.Close()

	useTempCatalogCache(t)
	SetActiveConfig(ConfigFromCredentialMap("icon-reg", map[string]CredentialConfig{
		"icon-reg": {
			URL:  "https://example.invalid",
			Auth: AuthConfig{Type: "basic", Username: "u", Password: "p"},
		},
	}))
	clearCredAuthCache()
	t.Cleanup(func() {
		clearCredAuthCache()
		SetActiveConfig(nil)
	})

	hitURL := hit.URL + "/icon/hash_" + sum
	if err := saveIconCatalog("icon-reg", IconCatalogCache{
		FetchedAt: time.Now().UTC(),
		MaxID:     430,
		Items:     []Icon{{ID: 430, Name: "alloy_icon.png", URL: hitURL, SHA256: sum}},
	}); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "alloy_icon.png")
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := IconCheckFile("icon-reg", path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Present || !res.ICS || res.IconID != 430 || res.URL != hitURL {
		t.Fatalf("regional cache host %+v want url %s", res, hitURL)
	}
}

func TestIconUploadUpsertsCache(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "tok", "expires": ""})
	})
	mux.HandleFunc("/api/v1/icon", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": 9, "name": "logo.png",
			"url": "https://ics.example/icon/hash_" + strings.Repeat("ee", 32),
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	useTempCatalogCache(t)
	SetActiveConfig(ConfigFromCredentialMap("icon-up", map[string]CredentialConfig{
		"icon-up": {
			URL:  srv.URL,
			Auth: AuthConfig{Type: "basic", Username: "u", Password: "p"},
		},
	}))
	clearCredAuthCache()
	t.Cleanup(func() {
		clearCredAuthCache()
		SetActiveConfig(nil)
	})

	fetched := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	if err := saveIconCatalog("icon-up", IconCatalogCache{
		FetchedAt: fetched,
		MaxID:     2,
		Items:     []Icon{{ID: 1, Name: "old.png"}},
	}); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "logo.png")
	if err := os.WriteFile(path, []byte("file-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	up, err := IconUpload("icon-up", path)
	if err != nil {
		t.Fatal(err)
	}
	if up.ID != 9 {
		t.Fatalf("%+v", up)
	}
	cat, err := loadIconCatalog("icon-up")
	if err != nil || cat == nil {
		t.Fatal(err)
	}
	if !cat.FetchedAt.Equal(fetched) {
		t.Fatalf("fetched_at %s", cat.FetchedAt)
	}
	if cat.MaxID != 9 {
		t.Fatalf("max_id %d", cat.MaxID)
	}
	if rec, ok := iconCatalogGetByID("icon-up", 9); !ok || rec.Name != "logo.png" || rec.SHA256 != strings.Repeat("ee", 32) {
		t.Fatalf("upsert %+v ok=%v", rec, ok)
	}
}

func TestIconUploadReusesLowestLiveCacheID(t *testing.T) {
	payload := []byte("same-png")
	sum := IconSHA256Hex(payload)
	var posts int
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "tok", "expires": ""})
	})
	mux.HandleFunc("/api/v1/icon", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posts++
			http.Error(w, "should reuse cache", http.StatusInternalServerError)
			return
		}
		http.Error(w, "method", http.StatusMethodNotAllowed)
	})
	mux.HandleFunc("/api/v1/icon/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.Atoi(r.PathValue("id"))
		switch id {
		case 7, 43:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": id, "name": "app-receiver.png",
				"url": "https://ics.example/icon/hash_" + sum,
			})
		default:
			http.NotFound(w, r)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	useTempCatalogCache(t)
	SetActiveConfig(ConfigFromCredentialMap("icon-reuse", map[string]CredentialConfig{
		"icon-reuse": {
			URL:  srv.URL,
			Auth: AuthConfig{Type: "basic", Username: "u", Password: "p"},
		},
	}))
	clearCredAuthCache()
	t.Cleanup(func() {
		clearCredAuthCache()
		SetActiveConfig(nil)
	})

	if err := saveIconCatalog("icon-reuse", IconCatalogCache{
		FetchedAt: time.Now().UTC(),
		MaxID:     43,
		Items: []Icon{
			{ID: 43, Name: "app-receiver.png", SHA256: sum},
			{ID: 7, Name: "app-receiver.png", SHA256: sum},
		},
	}); err != nil {
		t.Fatal(err)
	}

	got, err := IconUploadBytes("icon-reuse", "app-receiver.png", payload)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 7 {
		t.Fatalf("want lowest id 7, got %+v", got)
	}
	if posts != 0 {
		t.Fatalf("posted %d times", posts)
	}
}

func TestIconUploadSkips404ThenReuses(t *testing.T) {
	payload := []byte("same-png")
	sum := IconSHA256Hex(payload)
	var posts int
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "tok", "expires": ""})
	})
	mux.HandleFunc("/api/v1/icon", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posts++
			http.Error(w, "should reuse", http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("/api/v1/icon/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.Atoi(r.PathValue("id"))
		if id == 7 {
			http.NotFound(w, r)
			return
		}
		if id == 43 {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": 43, "name": "app-receiver.png",
				"url": "https://ics.example/icon/hash_" + sum,
			})
			return
		}
		http.NotFound(w, r)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	useTempCatalogCache(t)
	SetActiveConfig(ConfigFromCredentialMap("icon-dead", map[string]CredentialConfig{
		"icon-dead": {
			URL:  srv.URL,
			Auth: AuthConfig{Type: "basic", Username: "u", Password: "p"},
		},
	}))
	clearCredAuthCache()
	t.Cleanup(func() {
		clearCredAuthCache()
		SetActiveConfig(nil)
	})

	if err := saveIconCatalog("icon-dead", IconCatalogCache{
		FetchedAt: time.Now().UTC(),
		MaxID:     43,
		Items: []Icon{
			{ID: 7, SHA256: sum},
			{ID: 43, SHA256: sum},
		},
	}); err != nil {
		t.Fatal(err)
	}

	got, err := IconUploadBytes("icon-dead", "x.png", payload)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 43 || posts != 0 {
		t.Fatalf("got %+v posts=%d", got, posts)
	}
}

func TestIconCatalogStatusAndProgressLines(t *testing.T) {
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	if got := iconCatalogStatusLine(nil, now); got != "icon cache missing" {
		t.Fatalf("nil: %q", got)
	}
	if got := iconCatalogStatusLine(&IconCatalogCache{}, now); got != "icon cache missing" {
		t.Fatalf("zero fetched: %q", got)
	}
	cat := &IconCatalogCache{
		FetchedAt: now.Add(-50 * time.Hour),
		MaxID:     400,
		Items:     []Icon{{ID: 1}, {ID: 2}},
	}
	if got := iconCatalogStatusLine(cat, now); got != "icon cache 2d old (2 icons, max id 400)" {
		t.Fatalf("status: %q", got)
	}
	if got := iconWalkBanner(nil, now, 0); got != fmt.Sprintf("icon cache missing; scanning 1–%d", iconScanTail) {
		t.Fatalf("banner missing: %q", got)
	}
	if got := iconWalkBanner(cat, now, 400); got != fmt.Sprintf("icon cache 2d old (2 icons, max id 400); scanning %d–%d", 401, 400+iconScanTail) {
		t.Fatalf("banner: %q", got)
	}
	dense := &IconCatalogCache{FetchedAt: now.Add(-time.Hour), MaxID: 2, Items: []Icon{{ID: 1}, {ID: 2}}}
	if got := iconWalkBanner(dense, now, 2); got != fmt.Sprintf("icon cache 1h old (2 icons, max id 2); scanning %d–%d", 3, 2+iconScanTail) {
		t.Fatalf("banner from max: %q", got)
	}
	if got := iconScanProgressLine(150, 420, 12, 148, 75*time.Second); got != "icon scan: 150/420 (12 hits, last 148) 1m15s" {
		t.Fatalf("progress: %q", got)
	}
	if got := iconScanElapsed(500 * time.Millisecond); got != "0s" {
		t.Fatalf("elapsed: %q", got)
	}
}
