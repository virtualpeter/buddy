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

func TestComputerFetchExtensionAttributes(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "tok", "expires": ""})
	})
	mux.HandleFunc("/api/v4/computers-inventory/101", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "101",
			"extensionAttributes": []map[string]any{
				{
					"definitionId": "7",
					"name":         "Asset Tag",
					"inputType":    "TEXT",
					"dataType":     "STRING",
					"values":       []string{"LAB-1"},
					"enabled":      true,
				},
				{
					"definitionId": "8",
					"name":         "OS Check",
					"inputType":    "SCRIPT",
					"dataType":     "STRING",
					"values":       []string{"14.0"},
					"enabled":      true,
				},
			},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	SetActiveConfig(ConfigFromCredentialMap("cea-val", map[string]CredentialConfig{
		"cea-val": {URL: srv.URL, Auth: AuthConfig{Type: "basic", Username: "u", Password: "p"}},
	}))
	clearCredAuthCache()
	t.Cleanup(func() {
		clearCredAuthCache()
		SetActiveConfig(nil)
	})

	rows, err := ComputerFetchExtensionAttributes("cea-val", "101", "asset")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].DefinitionID != "7" || rows[0].Values[0] != "LAB-1" {
		t.Fatalf("rows=%+v", rows)
	}
}

func TestComputerSetExtensionAttribute_RejectsScript(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "tok", "expires": ""})
	})
	mux.HandleFunc("/api/v4/computers-inventory/101", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "101",
			"extensionAttributes": []map[string]any{
				{"definitionId": "8", "name": "OS Check", "inputType": "SCRIPT", "dataType": "STRING", "values": []string{"14.0"}},
			},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	SetActiveConfig(ConfigFromCredentialMap("cea-val2", map[string]CredentialConfig{
		"cea-val2": {URL: srv.URL, Auth: AuthConfig{Type: "basic", Username: "u", Password: "p"}},
	}))
	clearCredAuthCache()
	t.Cleanup(func() {
		clearCredAuthCache()
		SetActiveConfig(nil)
	})

	err := ComputerSetExtensionAttribute("cea-val2", Opts{SkipConfirm: true}, ComputerSetExtensionAttributeSpec{
		ComputerID: "101",
		EAID:       "8",
		Values:     []string{"x"},
	}, slog.Default())
	if err == nil || !strings.Contains(err.Error(), "SCRIPT") {
		t.Fatalf("expected SCRIPT reject, got %v", err)
	}
}

func TestComputerSetExtensionAttribute_PATCH(t *testing.T) {
	var sawBody string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "tok", "expires": ""})
	})
	mux.HandleFunc("/api/v4/computers-inventory/101", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "101",
			"extensionAttributes": []map[string]any{
				{"definitionId": "7", "name": "Asset Tag", "inputType": "TEXT", "dataType": "STRING", "values": []string{"OLD"}},
			},
		})
	})
	mux.HandleFunc("/api/v4/computers-inventory-detail/101", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		b, _ := io.ReadAll(r.Body)
		sawBody = string(b)
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	SetActiveConfig(ConfigFromCredentialMap("cea-val3", map[string]CredentialConfig{
		"cea-val3": {URL: srv.URL, Auth: AuthConfig{Type: "basic", Username: "u", Password: "p"}},
	}))
	clearCredAuthCache()
	t.Cleanup(func() {
		clearCredAuthCache()
		SetActiveConfig(nil)
	})

	err := ComputerSetExtensionAttribute("cea-val3", Opts{SkipConfirm: true}, ComputerSetExtensionAttributeSpec{
		ComputerID: "101",
		EAName:     "Asset Tag",
		Values:     []string{"LAB-42"},
	}, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sawBody, `"definitionId":"7"`) || !strings.Contains(sawBody, `"LAB-42"`) {
		t.Fatalf("patch body=%s", sawBody)
	}
}
