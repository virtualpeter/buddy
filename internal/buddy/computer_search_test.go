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

func TestParseComputerSearchList_JSONAndXML(t *testing.T) {
	rows, err := parseComputerSearchList([]byte(`{"advanced_computer_searches":[{"id":1,"name":"Chrome"},{"id":2,"name":"Lab"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].ID != "1" || rows[1].Name != "Lab" {
		t.Fatalf("%+v", rows)
	}
	rows, err = parseComputerSearchList([]byte(`<advanced_computer_searches>
  <size>1</size>
  <advanced_computer_search><id>9</id><name>XML</name></advanced_computer_search>
</advanced_computer_searches>`))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != "9" || rows[0].Name != "XML" {
		t.Fatalf("xml %+v", rows)
	}
}

func TestParseComputerSearchDefinition_StripsComputers(t *testing.T) {
	body := []byte(`{
  "advanced_computer_search": {
    "id": 3,
    "name": "Chrome",
    "criteria": [{"name": "Application Title", "value": "Google Chrome"}],
    "display_fields": [{"name": "Computer Name"}],
    "computers": {"size": 1, "computer": [{"id": 10, "name": "Mac"}]}
  }
}`)
	d, err := parseComputerSearchDefinition(body)
	if err != nil {
		t.Fatal(err)
	}
	if classicMapID(d) != "3" || classicMapName(d) != "Chrome" {
		t.Fatalf("%+v", d)
	}
	if _, ok := d["computers"]; ok {
		t.Fatal("computers must be stripped")
	}
	if _, ok := d["criteria"]; !ok {
		t.Fatal("criteria kept")
	}
}

func TestParseComputerSearchComputers_ReportsAndACS(t *testing.T) {
	items, err := parseComputerSearchComputers([]byte(`{"computer_reports":{"size":1,"computer":[{"id":10,"Computer_Name":"Lab Mac","serial":"C02"}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || classicMapID(items[0]) != "10" || classicMapName(items[0]) != "Lab Mac" {
		t.Fatalf("reports %+v", items)
	}
	items, err = parseComputerSearchComputers([]byte(`{"advanced_computer_search":{"id":3,"name":"S","computers":[{"id":11,"name":"A"}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || classicMapID(items[0]) != "11" {
		t.Fatalf("acs %+v", items)
	}
}

func startComputerSearchAPI(t *testing.T) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "tok", "expires": ""})
	})
	mux.HandleFunc("/JSSResource/advancedcomputersearches", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"advanced_computer_searches": []map[string]any{
				{"id": 3, "name": "Chrome inventory"},
				{"id": 4, "name": "Lab Macs"},
			},
		})
	})
	mux.HandleFunc("/JSSResource/advancedcomputersearches/id/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		switch r.Method {
		case http.MethodGet:
			writeACSDef(w, id)
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), `"name"`) {
				http.Error(w, "missing name", 400)
				return
			}
			if strings.Contains(string(body), `"computers"`) {
				http.Error(w, "computers must not be sent", 400)
				return
			}
			w.WriteHeader(http.StatusOK)
		case http.MethodDelete:
			w.WriteHeader(http.StatusOK)
		case http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 99})
		default:
			http.Error(w, "method", 405)
		}
	})
	mux.HandleFunc("/JSSResource/advancedcomputersearches/name/{name}", func(w http.ResponseWriter, r *http.Request) {
		writeACSDef(w, "3")
	})
	mux.HandleFunc("/JSSResource/computerreports/id/{id}", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"computer_reports": map[string]any{
				"size": 1,
				"computer": []map[string]any{
					{"id": 10, "Computer_Name": "Lab Mac", "serial": "C02ABC"},
				},
			},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	SetActiveConfig(ConfigFromCredentialMap("acs-test", map[string]CredentialConfig{
		"acs-test": {
			URL:  srv.URL,
			Auth: AuthConfig{Type: "basic", Username: "u", Password: "p"},
		},
	}))
	clearCredAuthCache()
	t.Cleanup(func() {
		clearCredAuthCache()
		SetActiveConfig(nil)
	})
	return "acs-test"
}

func writeACSDef(w http.ResponseWriter, id string) {
	_ = json.NewEncoder(w).Encode(map[string]any{
		"advanced_computer_search": map[string]any{
			"id":   id,
			"name": "Chrome inventory",
			"criteria": []map[string]any{
				{"name": "Application Title", "search_type": "is", "value": "Google Chrome", "and_or": "and", "priority": 0},
			},
			"computers": []map[string]any{{"id": 10, "name": "Lab Mac"}},
		},
	})
}

func TestComputerSearchHTTP(t *testing.T) {
	cred := startComputerSearchAPI(t)
	rows, err := ComputerSearchSearch(cred, "chrome", slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != "3" {
		t.Fatalf("list %+v", rows)
	}
	d, err := ComputerSearchFetch(cred, "3")
	if err != nil {
		t.Fatal(err)
	}
	if classicMapName(d) != "Chrome inventory" {
		t.Fatalf("get %+v", d)
	}
	if _, ok := d["computers"]; ok {
		t.Fatal("get must strip computers")
	}
	comps, err := ComputerSearchComputers(cred, "3", "lab")
	if err != nil {
		t.Fatal(err)
	}
	if len(comps) != 1 || classicMapSerial(comps[0]) != "C02ABC" {
		t.Fatalf("results %+v", comps)
	}
	sum, err := ComputerSearchCreate(cred, Opts{SkipConfirm: true}, map[string]interface{}{"name": "New Search"}, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if sum == nil || sum.ID != "99" {
		t.Fatalf("create %+v", sum)
	}
	if err := ComputerSearchPut(cred, Opts{SkipConfirm: true}, "3", map[string]interface{}{"name": "Chrome inventory", "criteria": []interface{}{}}, slog.Default()); err != nil {
		t.Fatal(err)
	}
	if err := ComputerSearchDelete(cred, Opts{SkipConfirm: true}, []string{"3"}, slog.Default()); err != nil {
		t.Fatal(err)
	}
}
