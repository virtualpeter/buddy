package buddy

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestScriptSearch_StripsScriptContents(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":   "tok",
			"expires": "",
		})
	})
	mux.HandleFunc("/api/v1/scripts", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"totalCount": 1,
			"results": []map[string]any{
				{
					"id":             "42",
					"name":           "Install Thing",
					"categoryId":     "1",
					"categoryName":   "Misc",
					"scriptContents": "#!/bin/bash\necho secret body\n",
					"info":           "should not appear in search",
					"parameter4":     "x",
				},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	SetActiveConfig(ConfigFromCredentialMap("scripts-test", map[string]CredentialConfig{
		"scripts-test": {
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

	recs, err := ScriptSearch("scripts-test", "", PageOpts{Page: PageAll}, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 {
		t.Fatalf("len=%d", len(recs))
	}
	r := recs[0]
	if r.ID != "42" || r.Name != "Install Thing" || r.CategoryID != "1" || r.CategoryName != "Misc" {
		t.Fatalf("record=%+v", r)
	}
	// ScriptRecord has no ScriptContents field; re-encode to prove bodies are gone.
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"id":"42","name":"Install Thing","categoryId":"1","categoryName":"Misc"}` {
		t.Fatalf("json=%s", b)
	}
	var asMap map[string]any
	if err := json.Unmarshal(b, &asMap); err != nil {
		t.Fatal(err)
	}
	if _, ok := asMap["scriptContents"]; ok {
		t.Fatal("scriptContents must not appear on ScriptSearch results")
	}
}
