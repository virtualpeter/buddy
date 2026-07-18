package serve

import (
	"net/http"
	"strings"

	"buddy/internal/help"
	"buddy/stdlib"
)

type libraryListItem struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Kind  string `json:"kind"`
	Path  string `json:"path,omitempty"`
}

type libraryDetail struct {
	ID        string                `json:"id"`
	Title     string                `json:"title"`
	Kind      string                `json:"kind"`
	Path      string                `json:"path,omitempty"`
	Sections  []help.LibrarySection `json:"sections,omitempty"`
	Functions []help.LibraryFunc    `json:"functions,omitempty"`
}

func handleLibrariesList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	out := []libraryListItem{
		{ID: "builtins", Title: "Builtins", Kind: "builtins"},
	}
	mods, err := stdlib.Catalog()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	for _, m := range mods {
		out = append(out, libraryListItem{
			ID:    m.ID,
			Title: m.Title,
			Kind:  "stdlib",
			Path:  m.Path,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func handleLibraryDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/libraries/")
	id = strings.Trim(id, "/")
	if id == "" || strings.Contains(id, "..") {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}

	if id == "builtins" {
		sections, flat := help.BuiltinsLibrary()
		writeJSON(w, http.StatusOK, libraryDetail{
			ID:        "builtins",
			Title:     "Builtins",
			Kind:      "builtins",
			Sections:  sections,
			Functions: flat,
		})
		return
	}

	mods, err := stdlib.Catalog()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	for _, m := range mods {
		if m.ID != id && m.Path != id {
			continue
		}
		fns := make([]help.LibraryFunc, 0, len(m.Functions))
		for _, f := range m.Functions {
			fns = append(fns, help.LibraryFunc{
				Name:      f.Name,
				Signature: f.Signature,
				Summary:   f.Summary,
				Insert:    f.Insert,
			})
		}
		writeJSON(w, http.StatusOK, libraryDetail{
			ID:        m.ID,
			Title:     m.Title,
			Kind:      "stdlib",
			Path:      m.Path,
			Functions: fns,
		})
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
}
