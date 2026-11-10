package buddy

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestScriptListPrintsDownloadFilename(t *testing.T) {
	cred, _ := startScriptStore(t, []ScriptDetail{{
		ID: "7", Name: "Restart Jamf Agent!", CategoryName: "Misc",
	}})
	out := captureStdout(t, func() {
		if err := ScriptList(cred, "", false, PageOpts{Page: PageAll}, slog.Default()); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "Local Filename") || !strings.Contains(out, "restart-jamf-agent.sh") {
		t.Fatalf("list=%s", out)
	}
	out = captureStdout(t, func() {
		if err := ScriptList(cred, "", true, PageOpts{Page: PageAll}, slog.Default()); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, `"localFilename": "restart-jamf-agent.sh"`) {
		t.Fatalf("json=%s", out)
	}
}

func TestScriptFilenameKeepsTrailingSH(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{name: "install-chrome.sh", want: "install-chrome.sh"},
		{name: "Install Chrome.SH", want: "install-chrome.sh"},
		{name: "Restart Jamf Agent!", want: "restart-jamf-agent.sh"},
		{name: "notes.py", want: "notes-py.sh"},
	}
	for _, tc := range cases {
		if got := scriptFilename(tc.name); got != tc.want {
			t.Errorf("scriptFilename(%q)=%q, want %q", tc.name, got, tc.want)
		}
	}
	if !ScriptFileIsDownloadName("install-chrome.sh") {
		t.Fatal("install-chrome.sh should be a download name")
	}
	if ScriptFileIsDownloadName("My Script.sh") || ScriptFileIsDownloadName("notes.py") {
		t.Fatal("mixed case or a non-.sh extension is not a download name")
	}
}

type scriptStore struct {
	mu      sync.Mutex
	rows    []ScriptDetail
	posts   int
	puts    int
	dels    int
	filters []string
	list    func(filter string, rows []ScriptDetail) []ScriptDetail
}

func (s *scriptStore) snapshot() []ScriptDetail {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ScriptDetail, len(s.rows))
	copy(out, s.rows)
	return out
}

func startScriptStore(t *testing.T, rows []ScriptDetail) (string, *scriptStore) {
	t.Helper()
	store := &scriptStore{rows: append([]ScriptDetail(nil), rows...)}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "tok", "expires": ""})
	})
	mux.HandleFunc("/api/v1/scripts/{id}/history", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"totalCount": 1,
			"results": []ScriptHistoryEntry{{
				ID: 9, Username: "ada", Date: "2026-01-02T03:04:05Z", Note: "edited", Details: "body",
			}},
		})
	})
	mux.HandleFunc("/api/v1/scripts/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		store.mu.Lock()
		defer store.mu.Unlock()
		idx := -1
		for i, row := range store.rows {
			if row.ID == id {
				idx = i
				break
			}
		}
		switch r.Method {
		case http.MethodGet:
			if idx < 0 {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(store.rows[idx])
		case http.MethodPut:
			if idx < 0 {
				http.NotFound(w, r)
				return
			}
			var body ScriptDetail
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if body.Name != "" {
				store.rows[idx].Name = body.Name
			}
			store.rows[idx].ScriptContents = body.ScriptContents
			store.puts++
			body.ID = id
			_ = json.NewEncoder(w).Encode(store.rows[idx])
		case http.MethodDelete:
			if idx < 0 {
				http.NotFound(w, r)
				return
			}
			store.rows = append(store.rows[:idx], store.rows[idx+1:]...)
			store.dels++
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "method", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/v1/scripts", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			filter := r.URL.Query().Get("filter")
			store.mu.Lock()
			store.filters = append(store.filters, filter)
			rows := append([]ScriptDetail{}, store.rows...)
			list := store.list
			store.mu.Unlock()
			if list != nil {
				rows = list(filter, rows)
			}
			if rows == nil {
				rows = []ScriptDetail{}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"totalCount": len(rows),
				"results":    rows,
			})
		case http.MethodPost:
			var body ScriptDetail
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			store.mu.Lock()
			body.ID = "100"
			store.rows = append(store.rows, body)
			store.posts++
			store.mu.Unlock()
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":   body.ID,
				"href": "/api/v1/scripts/" + body.ID,
			})
		default:
			http.Error(w, "method", http.StatusMethodNotAllowed)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	SetActiveConfig(ConfigFromCredentialMap("scripts-file-test", map[string]CredentialConfig{
		"scripts-file-test": {
			URL:  srv.URL,
			Auth: AuthConfig{Type: "basic", Username: "u", Password: "p"},
		},
	}))
	clearCredAuthCache()
	t.Cleanup(func() {
		clearCredAuthCache()
		SetActiveConfig(nil)
	})
	return "scripts-file-test", store
}

func writeScriptFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestScriptCreateRefusesExistingNameAndDownloadName(t *testing.T) {
	cred, store := startScriptStore(t, []ScriptDetail{{
		ID: "7", Name: "Install Chrome", ScriptContents: "old\n",
	}})
	dir := t.TempDir()
	file := writeScriptFile(t, dir, "install-chrome.sh", "new\n")
	opts := Opts{SkipConfirm: true}

	err := ScriptCreate(cred, opts, "Install Chrome", file, "", "", "", "", ScriptCreateExtra{}, slog.Default())
	if err == nil || !strings.Contains(err.Error(), "already exists") || !strings.Contains(err.Error(), "use buddy script update") {
		t.Fatalf("exact name: %v", err)
	}
	err = ScriptCreate(cred, opts, "install-chrome.sh", file, "", "", "", "", ScriptCreateExtra{}, slog.Default())
	if err == nil || !strings.Contains(err.Error(), "install-chrome.sh") || !strings.Contains(err.Error(), "7 (Install Chrome)") {
		t.Fatalf("download name: %v", err)
	}
	if store.posts != 0 {
		t.Fatalf("posts=%d", store.posts)
	}
}

func TestScriptCreateFromDownloadName(t *testing.T) {
	cred, store := startScriptStore(t, nil)
	dir := t.TempDir()
	file := writeScriptFile(t, dir, "install-chrome.sh", "echo hi\n")
	if err := ScriptCreate(cred, Opts{SkipConfirm: true}, "install-chrome.sh", file, "", "", "", "", ScriptCreateExtra{}, slog.Default()); err != nil {
		t.Fatal(err)
	}
	rows := store.snapshot()
	if len(rows) != 1 || rows[0].Name != "install-chrome.sh" || rows[0].ScriptContents != "echo hi\n" {
		t.Fatalf("rows=%+v", rows)
	}
}

func TestScriptUpdateByFileRoundTrip(t *testing.T) {
	cred, store := startScriptStore(t, []ScriptDetail{{
		ID: "9", Name: "Restart Jamf Agent!", ScriptContents: "old\n",
	}})
	dir := t.TempDir()
	t.Chdir(dir)
	if _, err := ScriptDownload(cred, "Restart Jamf Agent!", "", slog.Default()); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "restart-jamf-agent.sh")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old\n" {
		t.Fatalf("downloaded %q", got)
	}
	if err := os.WriteFile(path, []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	id, err := ScriptUpdateByFile(cred, Opts{SkipConfirm: true}, path, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if id != "9" {
		t.Fatalf("id=%s", id)
	}
	rows := store.snapshot()
	if len(rows) != 1 || rows[0].ScriptContents != "new\n" {
		t.Fatalf("rows=%+v", rows)
	}

	_, err = ScriptUpdateByFile(cred, Opts{SkipConfirm: true}, filepath.Join(dir, "missing.sh"), slog.Default())
	if err == nil || !strings.Contains(err.Error(), "use buddy script create") {
		t.Fatalf("missing: %v", err)
	}
}

func TestScriptUpdateByFileAmbiguous(t *testing.T) {
	cred, _ := startScriptStore(t, []ScriptDetail{
		{ID: "1", Name: "Foo Bar"},
		{ID: "2", Name: "foo-bar"},
	})
	dir := t.TempDir()
	file := writeScriptFile(t, dir, "foo-bar.sh", "x\n")
	_, err := ScriptUpdateByFile(cred, Opts{SkipConfirm: true}, file, slog.Default())
	if err == nil || !strings.Contains(err.Error(), "1 (Foo Bar)") || !strings.Contains(err.Error(), "2 (foo-bar)") {
		t.Fatalf("ambiguous: %v", err)
	}
}

func TestScriptCheckMatchMismatchAndMissing(t *testing.T) {
	body := "echo hi\n"
	cred, _ := startScriptStore(t, []ScriptDetail{{
		ID: "4", Name: "install-chrome.sh", ScriptContents: body,
	}})
	dir := t.TempDir()
	file := writeScriptFile(t, dir, "install-chrome.sh", body)
	log := slog.Default()

	res, err := ScriptCheck(cred, file, "", log)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || res.Class != ScriptCheckMatch || res.ID != "4" || res.SHA256 != res.LocalSHA256 {
		t.Fatalf("match=%+v", res)
	}
	out := captureStdout(t, func() {
		if err := ScriptCheckPrint(cred, file, "", log); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "id:           4") || !strings.Contains(out, "match:        yes") {
		t.Fatalf("print=%s", out)
	}

	if err := os.WriteFile(file, []byte("different\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err = ScriptCheck(cred, file, "", log)
	if err != nil {
		t.Fatal(err)
	}
	if res.OK || res.Class != ScriptCheckMismatch || res.ID != "4" {
		t.Fatalf("mismatch=%+v", res)
	}
	out = captureStdout(t, func() {
		err = ScriptCheckPrint(cred, file, "", log)
	})
	if err == nil || !strings.Contains(err.Error(), "script check: mismatch") || !strings.Contains(out, "id:           4") {
		t.Fatalf("print err=%v out=%s", err, out)
	}

	missing := writeScriptFile(t, dir, "absent.sh", "x\n")
	res, err = ScriptCheck(cred, missing, "", log)
	if err != nil {
		t.Fatal(err)
	}
	if res.Class != ScriptCheckMissing || res.ID != "" {
		t.Fatalf("missing=%+v", res)
	}
	out = captureStdout(t, func() {
		err = ScriptCheckPrint(cred, missing, "", log)
	})
	if err == nil || !strings.Contains(err.Error(), "script check: missing") || strings.Contains(out, "id:") {
		t.Fatalf("print err=%v out=%s", err, out)
	}

	res, err = ScriptCheck(cred, file, "99", log)
	if err != nil {
		t.Fatal(err)
	}
	if res.Class != ScriptCheckMissing {
		t.Fatalf("id 404=%+v", res)
	}
}

func TestScriptRenameMovesLocalFileAndDeleteLeavesIt(t *testing.T) {
	cred, store := startScriptStore(t, []ScriptDetail{{
		ID: "4", Name: "Restart Jamf Agent!", ScriptContents: "echo\n",
	}})
	dir := t.TempDir()
	file := writeScriptFile(t, dir, "restart-jamf-agent.sh", "echo\n")
	opts := Opts{SkipConfirm: true}
	blocked := filepath.Join(dir, "restart-the-jamf-agent.sh")
	if err := os.WriteFile(blocked, []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ScriptRename(cred, opts, file, "Restart the Jamf Agent", slog.Default()); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("existing local file: %v", err)
	}
	if rows := store.snapshot(); len(rows) != 1 || rows[0].Name != "Restart Jamf Agent!" {
		t.Fatalf("name changed despite local conflict: %+v", rows)
	}
	if err := os.Remove(blocked); err != nil {
		t.Fatal(err)
	}
	if err := ScriptRename(cred, opts, file, "Restart the Jamf Agent", slog.Default()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("old file stat=%v", err)
	}
	moved := filepath.Join(dir, "restart-the-jamf-agent.sh")
	if _, err := os.Stat(moved); err != nil {
		t.Fatal(err)
	}
	rows := store.snapshot()
	if len(rows) != 1 || rows[0].Name != "Restart the Jamf Agent" {
		t.Fatalf("rows=%+v", rows)
	}

	// A second script already owns this download name.
	store.mu.Lock()
	store.rows = append(store.rows, ScriptDetail{ID: "8", Name: "Foo Bar"})
	store.mu.Unlock()
	if err := ScriptRename(cred, opts, "4", "Foo-Bar", slog.Default()); err == nil || !strings.Contains(err.Error(), "foo-bar.sh") {
		t.Fatalf("taken download name: %v", err)
	}

	if err := ScriptDeleteRefs(cred, opts, []string{moved}, slog.Default()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(moved); err != nil {
		t.Fatalf("local file removed: %v", err)
	}
	rows = store.snapshot()
	if len(rows) != 1 || rows[0].ID != "8" {
		t.Fatalf("after delete=%+v", rows)
	}
	if store.dels != 1 {
		t.Fatalf("deletes=%d", store.dels)
	}
}

func TestScriptLocalStemRoundTrip(t *testing.T) {
	for _, filename := range []string{"install-chrome.sh", "restart-jamf-agent.sh", "script.sh"} {
		stem, err := ScriptLocalStem(filename)
		if err != nil {
			t.Fatal(err)
		}
		if got := ScriptFilename(stem); got != filename {
			t.Fatalf("ScriptFilename(ScriptLocalStem(%q))=%q", filename, got)
		}
	}
	if _, err := ScriptLocalStem("notes.py"); err == nil {
		t.Fatal("notes.py should be rejected")
	}
	if _, err := ScriptLocalStem("My Script.sh"); err == nil {
		t.Fatal("a name with spaces should be rejected")
	}
}

func TestScriptDownloadNameFilterFocusAndFallback(t *testing.T) {
	cred, store := startScriptStore(t, []ScriptDetail{{
		ID: "7", Name: "Restart Jamf Agent!", ScriptContents: "echo\n",
	}})
	hits, err := scriptHitsByDownloadName(cred, "restart-jamf-agent.sh")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].ID != "7" {
		t.Fatalf("hits=%+v", hits)
	}
	store.mu.Lock()
	filters := append([]string(nil), store.filters...)
	store.mu.Unlock()
	if len(filters) != 1 || filters[0] != `name=="*restart*jamf*agent*"` {
		t.Fatalf("filters=%q", filters)
	}

	store.mu.Lock()
	store.filters = nil
	store.list = func(filter string, rows []ScriptDetail) []ScriptDetail {
		if filter != "" {
			return []ScriptDetail{}
		}
		return rows
	}
	store.mu.Unlock()
	hits, err = scriptHitsByDownloadName(cred, "restart-jamf-agent.sh")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Name != "Restart Jamf Agent!" {
		t.Fatalf("fallback hits=%+v", hits)
	}
	store.mu.Lock()
	filters = append([]string(nil), store.filters...)
	store.mu.Unlock()
	if len(filters) != 2 || filters[0] == "" || filters[1] != "" {
		t.Fatalf("fallback filters=%q", filters)
	}

	store.mu.Lock()
	store.rows = []ScriptDetail{{ID: "1", Name: "ab"}}
	store.filters = nil
	store.list = nil
	store.mu.Unlock()
	hits, err = scriptHitsByDownloadName(cred, "a-b.sh")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 0 {
		t.Fatalf("ab should not match a-b.sh: %+v", hits)
	}
}

func TestScriptResolveFileCheckDownloadHistory(t *testing.T) {
	cred, _ := startScriptStore(t, []ScriptDetail{{
		ID: "7", Name: "Restart Jamf Agent!", ScriptContents: "echo hi\n",
	}})
	dir := t.TempDir()
	t.Chdir(dir)
	file := writeScriptFile(t, dir, "restart-jamf-agent.sh", "echo hi\n")
	got, err := ScriptResolve(cred, file, slog.Default())
	if err != nil || got.ID != "7" || got.Name != "Restart Jamf Agent!" {
		t.Fatalf("resolve=%+v err=%v", got, err)
	}
	res, err := ScriptCheck(cred, file, "", slog.Default())
	if err != nil || !res.OK || res.Class != ScriptCheckMatch || res.ID != "7" {
		t.Fatalf("check=%+v err=%v", res, err)
	}
	written, err := ScriptDownload(cred, "Restart Jamf Agent!", "", slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(written) != "restart-jamf-agent.sh" {
		t.Fatalf("written=%q", written)
	}
	entries, _, err := ScriptHistoryFetch(cred, "7", 5)
	if err != nil || len(entries) != 1 || entries[0].Username != "ada" || entries[0].Note != "edited" {
		t.Fatalf("history=%+v err=%v", entries, err)
	}
	if err := RequireNewScriptName(cred, "restart jamf agent"); err == nil || !strings.Contains(err.Error(), "restart-jamf-agent.sh") {
		t.Fatalf("create slug: %v", err)
	}
}

func TestScriptCreateCarriesParameters(t *testing.T) {
	cred, store := startScriptStore(t, nil)
	dir := t.TempDir()
	file := writeScriptFile(t, dir, "install-chrome.sh", "echo hi\n")
	err := ScriptCreate(cred, Opts{SkipConfirm: true}, "install-chrome.sh", file, "", "", "", "", ScriptCreateExtra{
		OSRequirements: "10.15",
		Parameter4:     "p4",
	}, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	rows := store.snapshot()
	if len(rows) != 1 || rows[0].OSRequirements != "10.15" || rows[0].Parameter4 != "p4" {
		t.Fatalf("rows=%+v", rows)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old }()
	fn()
	_ = w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}
