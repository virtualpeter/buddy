package serve

import (
	"crypto/sha256"
	_ "embed"
	"fmt"
	"net/http"
	"strconv"
)

// Logo PNG (512×512 robot mascot). Served at /buddy.png; README points here too.
//
//go:embed buddy.png
var logoPNG []byte

// logoURL is /buddy.png?v=<content-hash> so browsers pick up logo replacements
// despite prior Cache-Control: max-age responses.
var logoURL = func() string {
	sum := sha256.Sum256(logoPNG)
	return fmt.Sprintf("/buddy.png?v=%x", sum[:8])
}()

func serveLogo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	etag := `"` + logoURL[len("/buddy.png?v="):] + `"`
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=3600, must-revalidate")
	w.Header().Set("ETag", etag)
	w.Header().Set("Content-Length", strconv.Itoa(len(logoPNG)))
	if match := r.Header.Get("If-None-Match"); match == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(logoPNG)
}
