package serve

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"buddy/internal/buddy"
	"buddy/internal/script"

	"github.com/google/uuid"
)

type sessionStore struct {
	mu       sync.Mutex
	sessions map[string]*script.Session
	credName string
	opts     buddy.Opts
	logger   *slog.Logger
}

func newSessionStore(credName string, opts buddy.Opts, logger *slog.Logger) *sessionStore {
	return &sessionStore{
		sessions: make(map[string]*script.Session),
		credName: credName,
		opts:     opts,
		logger:   logger,
	}
}

type createSessionReq struct {
	Cred  string   `json:"cred"`
	Trace bool     `json:"trace"`
	Args  []string `json:"args"`
}

type createSessionResp struct {
	ID    string   `json:"id"`
	Cred  string   `json:"cred"`
	Trace bool     `json:"trace"`
	Args  []string `json:"args"`
}

type evalReq struct {
	Source string `json:"source"`
	Cell   string `json:"cell"`
}

type configSessionReq struct {
	Trace *bool    `json:"trace"`
	Args  []string `json:"args"`
	// ArgsSet is true when args was present in JSON (including empty list).
	ArgsSet bool `json:"-"`
}

func (c *configSessionReq) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if v, ok := raw["trace"]; ok {
		var b bool
		if err := json.Unmarshal(v, &b); err != nil {
			return err
		}
		c.Trace = &b
	}
	if v, ok := raw["args"]; ok {
		c.ArgsSet = true
		if err := json.Unmarshal(v, &c.Args); err != nil {
			return err
		}
	}
	return nil
}

type runReq struct {
	Source   string `json:"source"`
	Filename string `json:"filename"`
}

type runResp struct {
	Stdout  string            `json:"stdout"`
	Stderr  string            `json:"stderr"`
	Result  string            `json:"result,omitempty"`
	Error   string            `json:"error,omitempty"`
	Globals []script.GlobalVar `json:"globals"`
}

func (s *sessionStore) handleCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req createSessionReq
	_ = json.NewDecoder(r.Body).Decode(&req)
	cred := s.credName
	if req.Cred != "" {
		cred = req.Cred
	}
	sess, err := script.NewSessionArgs(cred, s.opts, s.logger, req.Trace, req.Args)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	id := uuid.NewString()
	s.mu.Lock()
	s.sessions[id] = sess
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, createSessionResp{
		ID: id, Cred: cred, Trace: req.Trace, Args: sess.Args(),
	})
}

func (s *sessionStore) get(id string) *script.Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[id]
}

func (s *sessionStore) handleEval(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sess := s.get(id)
	if sess == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	var req evalReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	cell := req.Cell
	if cell == "" {
		cell = "cell"
	}
	result := sess.Eval(cell, req.Source)
	writeJSON(w, http.StatusOK, result)
}

func (s *sessionStore) handleRun(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sess := s.get(id)
	if sess == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	var req runReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	filename := req.Filename
	if filename == "" {
		filename = "<script>"
	}
	out, globals := sess.RunScript(filename, req.Source)
	writeJSON(w, http.StatusOK, runResp{
		Stdout: out.Stdout, Stderr: out.Stderr, Result: out.Result, Error: out.Error, Globals: globals,
	})
}

func (s *sessionStore) handleConfig(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sess := s.get(id)
	if sess == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	var req configSessionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.ArgsSet {
		sess.SetArgs(req.Args)
	}
	if req.Trace != nil && *req.Trace != sess.Trace() {
		if err := sess.SetTrace(*req.Trace); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":    true,
		"trace": sess.Trace(),
		"args":  sess.Args(),
	})
}

func (s *sessionStore) handleGlobals(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sess := s.get(id)
	if sess == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"globals": sess.InspectGlobals()})
}

func (s *sessionStore) handleDelete(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	sess := s.sessions[id]
	if sess != nil {
		_ = sess.Reset()
		delete(s.sessions, id)
	}
	s.mu.Unlock()
	if sess == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func (s *sessionStore) handleReset(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sess := s.get(id)
	if sess == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	if err := sess.Reset(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"globals": sess.InspectGlobals(),
	})
}

func handleFSRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	path := r.URL.Query().Get("path")
	if r.Method == http.MethodPost {
		var body struct {
			Path string `json:"path"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Path != "" {
			path = body.Path
		}
	}
	abs, err := resolveIDEPath(path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	rel, _ := relToIDERoot(abs)
	root, _ := ideRoot()
	writeJSON(w, http.StatusOK, map[string]any{
		"path":    filepath.ToSlash(rel),
		"abs":     abs,
		"root":    root,
		"content": string(data),
	})
}

func handleFSWrite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	abs, err := resolveIDEPath(body.Path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := os.WriteFile(abs, []byte(body.Content), 0755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	rel, _ := relToIDERoot(abs)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "path": filepath.ToSlash(rel)})
}

func handleFSRoot(w http.ResponseWriter, r *http.Request) {
	root, err := ideRoot()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"root": root})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
