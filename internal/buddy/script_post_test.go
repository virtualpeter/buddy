package buddy

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestMergeScriptWriteResponse_HrefOnlyKeepsPosted(t *testing.T) {
	out, err := mergeScriptWriteResponse(ScriptDetail{
		Name:           "Install Thing",
		ScriptContents: "echo hi\n",
		Priority:       "AFTER",
		CategoryID:     "3",
	}, []byte(`{"id":"99","href":"https://example.jamfcloud.com/api/v1/scripts/99"}`), "")
	if err != nil {
		t.Fatal(err)
	}
	if out.ID != "99" || out.Name != "Install Thing" || out.ScriptContents != "echo hi\n" || out.Priority != "AFTER" || out.CategoryID != "3" {
		t.Fatalf("merged=%+v", out)
	}
}

func TestMergeScriptWriteResponse_ResponseFieldsWin(t *testing.T) {
	out, err := mergeScriptWriteResponse(ScriptDetail{
		Name:       "Local",
		CategoryID: "1",
	}, []byte(`{"id":"9","name":"Server","categoryName":"Misc"}`), "")
	if err != nil {
		t.Fatal(err)
	}
	if out.ID != "9" || out.Name != "Server" || out.CategoryName != "Misc" || out.CategoryID != "1" {
		t.Fatalf("merged=%+v", out)
	}
}

func TestMergeScriptWriteResponse_PutFallbackID(t *testing.T) {
	out, err := mergeScriptWriteResponse(ScriptDetail{
		Name:           "Keep",
		ScriptContents: "echo\n",
	}, []byte(`{"name":"Keep","scriptContents":"echo\n","priority":"BEFORE"}`), "7")
	if err != nil {
		t.Fatal(err)
	}
	if out.ID != "7" || out.Priority != "BEFORE" || out.Name != "Keep" {
		t.Fatalf("merged=%+v", out)
	}
}

func TestMergeScriptWriteResponse_MissingID(t *testing.T) {
	_, err := mergeScriptWriteResponse(ScriptDetail{Name: "X"}, []byte(`{"href":"/x"}`), "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestScriptPostDetail_NoFollowUpGET(t *testing.T) {
	var gets atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "tok", "expires": ""})
	})
	mux.HandleFunc("/api/v1/scripts/{id}", func(w http.ResponseWriter, r *http.Request) {
		gets.Add(1)
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api/v1/scripts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":   "99",
			"href": "https://example.jamfcloud.com/api/v1/scripts/99",
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	SetActiveConfig(ConfigFromCredentialMap("scripts-post-test", map[string]CredentialConfig{
		"scripts-post-test": {
			URL:  srv.URL,
			Auth: AuthConfig{Type: "basic", Username: "u", Password: "p"},
		},
	}))
	clearCredAuthCache()
	t.Cleanup(func() {
		clearCredAuthCache()
		SetActiveConfig(nil)
	})

	created, err := ScriptPostDetail("scripts-post-test", Opts{SkipConfirm: true}, ScriptDetail{
		Name:           "Install Thing",
		ScriptContents: "echo hi\n",
		Priority:       "AFTER",
	}, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != "99" || created.Name != "Install Thing" || created.ScriptContents != "echo hi\n" {
		t.Fatalf("created=%+v", created)
	}
	if gets.Load() != 0 {
		t.Fatalf("GET count=%d, want 0", gets.Load())
	}
}

func TestScriptDelete_GETThenDELETE(t *testing.T) {
	var gets, deletes atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "tok", "expires": ""})
	})
	mux.HandleFunc("/api/v1/scripts/{id}", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			gets.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":   "7",
				"name": "Install Thing",
			})
		case http.MethodDelete:
			deletes.Add(1)
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "method", http.StatusMethodNotAllowed)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	SetActiveConfig(ConfigFromCredentialMap("scripts-delete-test", map[string]CredentialConfig{
		"scripts-delete-test": {
			URL:  srv.URL,
			Auth: AuthConfig{Type: "basic", Username: "u", Password: "p"},
		},
	}))
	clearCredAuthCache()
	t.Cleanup(func() {
		clearCredAuthCache()
		SetActiveConfig(nil)
	})

	if err := ScriptDelete("scripts-delete-test", Opts{SkipConfirm: true}, []string{"7"}, slog.Default()); err != nil {
		t.Fatal(err)
	}
	if gets.Load() != 1 {
		t.Fatalf("GET count=%d, want 1", gets.Load())
	}
	if deletes.Load() != 1 {
		t.Fatalf("DELETE count=%d, want 1", deletes.Load())
	}
}
