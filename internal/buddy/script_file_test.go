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
	if !strings.Contains(out, "Local Filename") || !strings.Contains(out, "Restart-Jamf-Agent.sh") || strings.Contains(out, "Category") || strings.Contains(out, "Misc") {
		t.Fatalf("list=%s", out)
	}
	out = captureStdout(t, func() {
		if err := ScriptList(cred, "", true, PageOpts{Page: PageAll}, slog.Default()); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, `"localFilename": "Restart-Jamf-Agent.sh"`) || !strings.Contains(out, `"categoryName": "Misc"`) {
		t.Fatalf("json=%s", out)
	}
}

func TestScriptFilenameKeepsTrailingSH(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{name: "install-chrome.sh", want: "install-chrome.sh"},
		{name: "Install Chrome.SH", want: "Install-Chrome.SH"},
		{name: "Restart Jamf Agent!", want: "Restart-Jamf-Agent.sh"},
		{name: "TabNineKiller.sh", want: "TabNineKiller.sh"},
		{name: "notes.py", want: "notes.py"},
		{name: "Notes.PY", want: "Notes.PY"},
		{name: "Beta.sh NO JAMF", want: "Beta-sh-NO-JAMF.sh"},
	}
	for _, tc := range cases {
		if got := scriptFilename(tc.name); got != tc.want {
			t.Errorf("scriptFilename(%q)=%q, want %q", tc.name, got, tc.want)
		}
	}
	if !ScriptFileIsDownloadName("install-chrome.sh") || !ScriptFileIsDownloadName("notes.py") {
		t.Fatal("install-chrome.sh and notes.py should be download names")
	}
	if ScriptFileIsDownloadName("My Script.sh") {
		t.Fatal("a name with spaces is not a download name")
	}
}

type scriptStore struct {
	mu           sync.Mutex
	rows         []ScriptDetail
	posts        int
	puts         int
	dels         int
	filters      []string
	list         func(filter string, rows []ScriptDetail) []ScriptDetail
	rejectFilter bool
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
			reject := store.rejectFilter
			store.mu.Unlock()
			if reject && filter != "" {
				http.Error(w, "bad filter", http.StatusBadRequest)
				return
			}
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
	path := filepath.Join(dir, "Restart-Jamf-Agent.sh")
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
	id, err := ScriptUpdateByFile(cred, Opts{SkipConfirm: true}, path, false, slog.Default())
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

	_, err = ScriptUpdateByFile(cred, Opts{SkipConfirm: true}, filepath.Join(dir, "missing.sh"), false, slog.Default())
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
	_, err := ScriptUpdateByFile(cred, Opts{SkipConfirm: true}, file, false, slog.Default())
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
	blocked := filepath.Join(dir, "Restart-the-Jamf-Agent.sh")
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
	moved := filepath.Join(dir, "Restart-the-Jamf-Agent.sh")
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
	if err := ScriptRename(cred, opts, "4", "Foo-Bar", slog.Default()); err == nil || !strings.Contains(err.Error(), "Foo-Bar.sh") {
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
	stem, err := ScriptLocalStem("notes.py")
	if err != nil || stem != "notes" {
		t.Fatalf("notes.py stem=%q err=%v", stem, err)
	}
	if got := ScriptFilename(stem); got != "notes.sh" {
		t.Fatalf("stem of notes.py comes back as %q", got)
	}
	if _, err := ScriptLocalStem("My Script.sh"); err == nil {
		t.Fatal("a name with spaces should be rejected")
	}
}

func TestNotesPyDoesNotMatchNotesSh(t *testing.T) {
	// The store returns every row for any filter, so both scripts are candidates.
	cred, _ := startScriptStore(t, []ScriptDetail{
		{ID: "1", Name: "notes.sh", ScriptContents: "echo\n"},
		{ID: "2", Name: "notes.py", ScriptContents: "print(1)\n"},
	})
	dir := t.TempDir()
	py := writeScriptFile(t, dir, "notes.py", "print(1)\n")
	sh := writeScriptFile(t, dir, "notes.sh", "echo\n")

	hits, err := scriptHitsByFileName(cred, py)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].ID != "2" || hits[0].Name != "notes.py" {
		t.Fatalf("notes.py hits=%+v", hits)
	}
	hits, err = scriptHitsByFileName(cred, sh)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].ID != "1" || hits[0].Name != "notes.sh" {
		t.Fatalf("notes.sh hits=%+v", hits)
	}

	got, err := ScriptResolve(cred, "notes.py", slog.Default())
	if err != nil || got == nil || got.ID != "2" {
		t.Fatalf("resolve notes.py=%+v err=%v", got, err)
	}
	got, err = ScriptResolve(cred, "notes.sh", slog.Default())
	if err != nil || got == nil || got.ID != "1" {
		t.Fatalf("resolve notes.sh=%+v err=%v", got, err)
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

	store.mu.Lock()
	store.rows = []ScriptDetail{{ID: "7", Name: "Restart Jamf Agent!"}}
	store.filters = nil
	store.list = nil
	store.rejectFilter = true
	store.mu.Unlock()
	hits, err = scriptHitsByDownloadName(cred, "restart-jamf-agent.sh")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].ID != "7" {
		t.Fatalf("rejected filter fallback=%+v", hits)
	}
}

func TestScriptLocalFilenameResolvesWithoutTheFile(t *testing.T) {
	cred, _ := startScriptStore(t, []ScriptDetail{
		{ID: "7", Name: "Restart Jamf Agent!", ScriptContents: "echo hi\n"},
		{ID: "8", Name: "notes.py", ScriptContents: "print(1)\n"},
		{ID: "9", Name: "notes-py", ScriptContents: "other\n"},
	})
	dir := t.TempDir()
	t.Chdir(dir)
	got, err := ScriptResolve(cred, "restart-jamf-agent.sh", slog.Default())
	if err != nil || got == nil || got.ID != "7" || got.Name != "Restart Jamf Agent!" {
		t.Fatalf("download name=%+v err=%v", got, err)
	}
	got, err = ScriptResolveName(cred, "notes.py", slog.Default())
	if err != nil || got == nil || got.ID != "8" {
		t.Fatalf("exact name wins=%+v err=%v", got, err)
	}
	id, err := ScriptLookupID(cred, "restart-jamf-agent.sh", slog.Default())
	if err != nil || id != "7" {
		t.Fatalf("lookup=%s err=%v", id, err)
	}
	written, err := ScriptDownload(cred, "restart-jamf-agent.sh", "", slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(written) != "Restart-Jamf-Agent.sh" {
		t.Fatalf("written=%q", written)
	}

	file := writeScriptFile(t, dir, "Restart Jamf Agent!.sh", "echo hi\n")
	res, err := ScriptCheck(cred, file, "", slog.Default())
	if err != nil || !res.OK || res.ID != "7" {
		t.Fatalf("slugged file=%+v err=%v", res, err)
	}
	notes := writeScriptFile(t, dir, "notes.py", "print(1)\n")
	got, err = ScriptResolve(cred, notes, slog.Default())
	if err != nil || got == nil || got.ID != "8" {
		t.Fatalf("notes.py file=%+v err=%v", got, err)
	}
}

func TestScriptDiffMatchDifferAndMissing(t *testing.T) {
	body := "alpha\nbeta\ngamma\n"
	cred, _ := startScriptStore(t, []ScriptDetail{{
		ID: "7", Name: "Restart Jamf Agent!", ScriptContents: body,
	}})
	dir := t.TempDir()
	t.Chdir(dir)
	file := writeScriptFile(t, dir, "restart-jamf-agent.sh", body)
	log := slog.Default()

	res, err := ScriptDiff(cred, file, "", log)
	if err != nil || !res.OK || res.Class != ScriptDiffMatch || res.Diff != "" || res.ID != "7" {
		t.Fatalf("match=%+v err=%v", res, err)
	}
	out := captureStdout(t, func() {
		if err := ScriptDiffPrint(cred, file, "", log); err != nil {
			t.Fatal(err)
		}
	})
	if out != "" {
		t.Fatalf("match printed %q", out)
	}

	if err := os.WriteFile(file, []byte("alpha\nBETA\ngamma\ndelta\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err = ScriptDiff(cred, file, "restart-jamf-agent.sh", log)
	if err != nil || res.OK || res.Class != ScriptDiffDiffer || res.ID != "7" {
		t.Fatalf("differ=%+v err=%v", res, err)
	}
	if !strings.Contains(res.Diff, "2c2\n") || !strings.Contains(res.Diff, "< beta\n") || !strings.Contains(res.Diff, "> BETA\n") || !strings.Contains(res.Diff, "> delta\n") {
		t.Fatalf("diff=%s", res.Diff)
	}
	out = captureStdout(t, func() {
		err = ScriptDiffPrint(cred, file, "", log)
	})
	if err == nil || !strings.Contains(err.Error(), "script diff: differ") || !strings.Contains(out, "< beta\n") {
		t.Fatalf("print err=%v out=%s", err, out)
	}

	missing := writeScriptFile(t, dir, "absent.sh", "x\n")
	res, err = ScriptDiff(cred, missing, "", log)
	if err != nil || res.Class != ScriptDiffMissing || res.Diff != "" {
		t.Fatalf("missing=%+v err=%v", res, err)
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
	if filepath.Base(written) != "Restart-Jamf-Agent.sh" {
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

func TestScriptsCheckDirAndCreateFromFile(t *testing.T) {
	cred, store := startScriptStore(t, []ScriptDetail{
		{ID: "1", Name: "Keep.sh", ScriptContents: "same\n"},
		{ID: "2", Name: "Change Me", ScriptContents: "old\n"},
		{ID: "3", Name: "Only Remote", ScriptContents: "remote\n"},
	})
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeScriptFile(t, dir, "Keep.sh", "same\n")
	writeScriptFile(t, dir, "Change-Me.sh", "new\n")
	writeScriptFile(t, dir, "fresh.py", "print(1)\n")
	writeScriptFile(t, filepath.Join(dir, "nested"), "ignored.sh", "nope\n")

	rows, err := ScriptsCheck(cred, dir, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]ScriptDirCheck{}
	for _, row := range rows {
		got[row.File] = row
	}
	if got["Keep.sh"].Class != ScriptCheckMatch || got["Keep.sh"].ID != "1" {
		t.Fatalf("keep=%+v", got["Keep.sh"])
	}
	if got["Change-Me.sh"].Class != ScriptCheckMismatch || got["Change-Me.sh"].Name != "Change Me" {
		t.Fatalf("change=%+v", got["Change-Me.sh"])
	}
	if got["fresh.py"].Class != ScriptCheckMissing || got["fresh.py"].ID != "" {
		t.Fatalf("fresh=%+v", got["fresh.py"])
	}
	if got["Only-Remote.sh"].Class != ScriptCheckRemoteOnly || got["Only-Remote.sh"].Path != "" || got["Only-Remote.sh"].ID != "3" {
		t.Fatalf("remote=%+v", got["Only-Remote.sh"])
	}
	if _, ok := got["ignored.sh"]; ok {
		t.Fatal("subdirectory file was included")
	}

	store.mu.Lock()
	store.rows = append(store.rows, ScriptDetail{ID: "4", Name: "keep.sh", ScriptContents: "other\n"})
	store.mu.Unlock()
	both := t.TempDir()
	writeScriptFile(t, both, "Keep.sh", "same\n")
	if _, err := ScriptsCheck(cred, both, slog.Default()); err == nil || !strings.Contains(err.Error(), "multiple scripts") {
		t.Fatalf("shared download name: %v", err)
	}
	two := t.TempDir()
	writeScriptFile(t, two, "Foo Bar.sh", "a\n")
	writeScriptFile(t, two, "Foo-Bar.sh", "b\n")
	if _, err := ScriptsCheck(cred, two, slog.Default()); err == nil || !strings.Contains(err.Error(), "multiple local files") {
		t.Fatalf("two local files: %v", err)
	}

	if _, err := ScriptUpdateByFile(cred, Opts{SkipConfirm: true}, filepath.Join(dir, "fresh.py"), false, slog.Default()); err == nil {
		t.Fatal("create=false should fail when the script is absent")
	}
	id, err := ScriptUpdateByFile(cred, Opts{SkipConfirm: true}, filepath.Join(dir, "fresh.py"), true, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if id != "100" {
		t.Fatalf("created id=%s", id)
	}
	var created ScriptDetail
	for _, row := range store.snapshot() {
		if row.ID == "100" {
			created = row
		}
	}
	if created.Name != "fresh.py" || created.ScriptContents != "print(1)\n" {
		t.Fatalf("created=%+v", created)
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
