package serve

import (
	"encoding/json"
	"log/slog"
	"net/http"
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
	Cred string `json:"cred"`
}

type createSessionResp struct {
	ID   string `json:"id"`
	Cred string `json:"cred"`
}

type evalReq struct {
	Source string `json:"source"`
	Cell   string `json:"cell"`
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
	sess, err := script.NewSession(cred, s.opts, s.logger, false)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	id := uuid.NewString()
	s.mu.Lock()
	s.sessions[id] = sess
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, createSessionResp{ID: id, Cred: cred})
}

func (s *sessionStore) handleEval(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	sess := s.sessions[id]
	s.mu.Unlock()
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
	s.mu.Lock()
	sess := s.sessions[id]
	s.mu.Unlock()
	if sess == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	if err := sess.Reset(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
