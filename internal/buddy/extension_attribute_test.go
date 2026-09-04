package buddy

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtensionAttributeSearch_StripsScriptContents(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "tok", "expires": ""})
	})
	mux.HandleFunc("/api/v1/computer-extension-attributes", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"totalCount": 1,
			"results": []map[string]any{
				{
					"id":                   "42",
					"name":                 "OS Version Check",
					"enabled":              true,
					"inputType":            "SCRIPT",
					"dataType":             "STRING",
					"inventoryDisplayType": "EXTENSION_ATTRIBUTES",
					"scriptContents":       "#!/bin/bash\necho secret\n",
					"description":          "should not appear in search",
				},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	SetActiveConfig(ConfigFromCredentialMap("ea-test", map[string]CredentialConfig{
		"ea-test": {
			URL:  srv.URL,
			Auth: AuthConfig{Type: "basic", Username: "u", Password: "p"},
		},
	}))
	clearCredAuthCache()
	t.Cleanup(func() {
		clearCredAuthCache()
		SetActiveConfig(nil)
	})

	recs, err := ExtensionAttributeSearch("ea-test", "", PageOpts{Page: PageAll}, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 {
		t.Fatalf("len=%d", len(recs))
	}
	r := recs[0]
	if r.ID != "42" || r.Name != "OS Version Check" || !r.Enabled || r.InputType != "SCRIPT" {
		t.Fatalf("record=%+v", r)
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var asMap map[string]any
	if err := json.Unmarshal(b, &asMap); err != nil {
		t.Fatal(err)
	}
	if _, ok := asMap["scriptContents"]; ok {
		t.Fatal("scriptContents must not appear on ExtensionAttributeSearch results")
	}
}

func startEAAPI(t *testing.T, listRow map[string]any, getByID map[string]any) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "tok", "expires": ""})
	})
	mux.HandleFunc("/api/v1/computer-extension-attributes/{id}", func(w http.ResponseWriter, r *http.Request) {
		if getByID == nil {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(getByID)
	})
	mux.HandleFunc("/api/v1/computer-extension-attributes", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"totalCount": 1,
			"results":    []map[string]any{listRow},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	SetActiveConfig(ConfigFromCredentialMap("ea-test", map[string]CredentialConfig{
		"ea-test": {
			URL:  srv.URL,
			Auth: AuthConfig{Type: "basic", Username: "u", Password: "p"},
		},
	}))
	clearCredAuthCache()
	t.Cleanup(func() {
		clearCredAuthCache()
		SetActiveConfig(nil)
	})
	return "ea-test"
}

func TestExtensionAttributeFetchByName_UsesListWhenContentsPresent(t *testing.T) {
	cred := startEAAPI(t, map[string]any{
		"id":                   "42",
		"name":                 "OS Version Check",
		"inputType":            "SCRIPT",
		"dataType":             "STRING",
		"inventoryDisplayType": "EXTENSION_ATTRIBUTES",
		"scriptContents":       "#!/bin/bash\necho from list\n",
	}, map[string]any{
		"id":             "42",
		"name":           "OS Version Check",
		"inputType":      "SCRIPT",
		"scriptContents": "#!/bin/bash\necho from get\n",
	})
	ea, err := ExtensionAttributeFetchByName(cred, "OS Version Check", slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if ea.ScriptContents != "#!/bin/bash\necho from list\n" {
		t.Fatalf("should use list row, got %q", ea.ScriptContents)
	}
}

func TestExtensionAttributeFetchByName_GetsByIDWhenContentsOmitted(t *testing.T) {
	cred := startEAAPI(t, map[string]any{
		"id":                   "42",
		"name":                 "OS Version Check",
		"inputType":            "SCRIPT",
		"dataType":             "STRING",
		"inventoryDisplayType": "EXTENSION_ATTRIBUTES",
	}, map[string]any{
		"id":             "42",
		"name":           "OS Version Check",
		"inputType":      "SCRIPT",
		"scriptContents": "#!/bin/bash\necho from get\n",
	})
	ea, err := ExtensionAttributeFetchByName(cred, "os version check", slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if ea.ScriptContents != "#!/bin/bash\necho from get\n" {
		t.Fatalf("should GET by id, got %q", ea.ScriptContents)
	}
}

func TestPrepareExtensionAttributeForCreate_stripsIDSetsName(t *testing.T) {
	en := true
	src := &ExtensionAttributeDetail{
		ID:                   "42",
		Name:                 "Old",
		InputType:            "SCRIPT",
		DataType:             "STRING",
		InventoryDisplayType: "EXTENSION_ATTRIBUTES",
		ScriptContents:       "echo hi",
		Enabled:              &en,
	}
	out, err := PrepareExtensionAttributeForCreate(src, ExtensionAttributePrepareSpec{Name: "New"})
	if err != nil {
		t.Fatal(err)
	}
	if out.ID != "" {
		t.Fatalf("id should be cleared, got %q", out.ID)
	}
	if out.Name != "New" {
		t.Fatalf("name: %q", out.Name)
	}
	if out.ScriptContents != "echo hi" || out.InputType != "SCRIPT" {
		t.Fatalf("fields not preserved: %+v", out)
	}
}

func TestPrepareExtensionAttributeForCreate_requiresName(t *testing.T) {
	_, err := PrepareExtensionAttributeForCreate(&ExtensionAttributeDetail{Name: "x"}, ExtensionAttributePrepareSpec{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMergeExtensionAttributeWriteResponse_HrefOnly(t *testing.T) {
	en := true
	out, err := mergeExtensionAttributeWriteResponse(ExtensionAttributeDetail{
		Name:                   "Check",
		InputType:              "SCRIPT",
		DataType:               "STRING",
		InventoryDisplayType:   "EXTENSION_ATTRIBUTES",
		ScriptContents:         "echo hi\n",
		Enabled:                &en,
	}, []byte(`{"id":"99","href":"https://example.jamfcloud.com/api/v1/computer-extension-attributes/99"}`), "")
	if err != nil {
		t.Fatal(err)
	}
	if out.ID != "99" || out.Name != "Check" || out.ScriptContents != "echo hi\n" {
		t.Fatalf("merged=%+v", out)
	}
}

func TestNormalizeExtensionAttributeCreate_defaults(t *testing.T) {
	ea := ExtensionAttributeDetail{Name: "X", InputType: "text"}
	if err := normalizeExtensionAttributeCreate(&ea); err != nil {
		t.Fatal(err)
	}
	if ea.InputType != "TEXT" || ea.DataType != "STRING" || ea.InventoryDisplayType != "EXTENSION_ATTRIBUTES" {
		t.Fatalf("defaults: %+v", ea)
	}
}

func TestFilterSpecExtensionAttributes(t *testing.T) {
	got, err := NormalizeListFilter("Chrome", FilterSpecExtensionAttributes)
	if err != nil {
		t.Fatal(err)
	}
	if got != `name=="*Chrome*"` {
		t.Fatalf("got %q", got)
	}
}
