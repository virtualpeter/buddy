package buddy

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseMobileSearchDevices(t *testing.T) {
	items, err := parseMobileSearchDevices([]byte(`{
  "advanced_mobile_device_search": {
    "id": 2,
    "name": "Supervised",
    "mobile_devices": {"size": 1, "mobile_device": [{"id": 7, "name": "iPad", "udid": "abc"}]}
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || classicMapID(items[0]) != "7" || classicMapName(items[0]) != "iPad" {
		t.Fatalf("%+v", items)
	}
}

func startMobileSearchAPI(t *testing.T) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "tok", "expires": ""})
	})
	mux.HandleFunc("/api/v1/advanced-mobile-device-searches/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": id, "name": "Supervised", "criteria": []any{}, "displayFields": []string{"Display Name"},
			})
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), `"name"`) {
				http.Error(w, "missing name", 400)
				return
			}
			w.WriteHeader(http.StatusOK)
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "method", 405)
		}
	})
	mux.HandleFunc("/api/v1/advanced-mobile-device-searches", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"totalCount": 1,
				"results":    []map[string]any{{"id": "2", "name": "Supervised"}},
			})
		case http.MethodPost:
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "88", "href": "/api/v1/advanced-mobile-device-searches/88"})
		default:
			http.Error(w, "method", 405)
		}
	})
	mux.HandleFunc("/JSSResource/advancedmobiledevicesearches/id/{id}", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"advanced_mobile_device_search": map[string]any{
				"id":   2,
				"name": "Supervised",
				"mobile_devices": []map[string]any{
					{"id": 7, "name": "iPad", "Display_Name": "iPad", "udid": "abc"},
				},
			},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	SetActiveConfig(ConfigFromCredentialMap("ams-test", map[string]CredentialConfig{
		"ams-test": {
			URL:  srv.URL,
			Auth: AuthConfig{Type: "basic", Username: "u", Password: "p"},
		},
	}))
	clearCredAuthCache()
	t.Cleanup(func() {
		clearCredAuthCache()
		SetActiveConfig(nil)
	})
	return "ams-test"
}

func TestMobileSearchHTTP(t *testing.T) {
	cred := startMobileSearchAPI(t)
	rows, err := MobileSearchSearch(cred, "super", PageOpts{Page: PageAll}, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != "2" {
		t.Fatalf("list %+v", rows)
	}
	d, err := MobileSearchFetch(cred, "2")
	if err != nil {
		t.Fatal(err)
	}
	if classicMapName(d) != "Supervised" {
		t.Fatalf("get %+v", d)
	}
	devs, err := MobileSearchDevices(cred, "2", "ipad")
	if err != nil {
		t.Fatal(err)
	}
	if len(devs) != 1 || classicMapID(devs[0]) != "7" {
		t.Fatalf("devices %+v", devs)
	}
	sum, err := MobileSearchCreate(cred, Opts{SkipConfirm: true}, map[string]interface{}{"name": "New"}, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if sum == nil || sum.ID != "88" {
		t.Fatalf("create %+v", sum)
	}
	if err := MobileSearchPut(cred, Opts{SkipConfirm: true}, "2", map[string]interface{}{"name": "Supervised"}, slog.Default()); err != nil {
		t.Fatal(err)
	}
	if err := MobileSearchDelete(cred, Opts{SkipConfirm: true}, []string{"2"}, slog.Default()); err != nil {
		t.Fatal(err)
	}
}
