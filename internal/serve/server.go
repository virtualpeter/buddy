package serve

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"buddy/internal/buddy"
)

// Options configures the local buddy web server.
type Options struct {
	Addr       string // e.g. "127.0.0.1:8742"
	DocsRoot   string // optional override
	Open       bool
	Version    string // buddy binary version (shown in chrome)
	TenantName string // default tenant (empty → config / ActiveSelection)
	CredName   string // optional auth pin (empty → auto-pick); Starlark auth=
	Opts       buddy.Opts
	Logger     *slog.Logger
}

// Run starts the HTTP server and blocks until the context is cancelled or Listen fails.
func Run(ctx context.Context, o Options) error {
	if o.Addr == "" {
		o.Addr = "127.0.0.1:8742"
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	setChromeVersion(o.Version)
	// Rebuild surfaces that bake chrome into a static HTML string at init.
	ideHTML = buildIDEHTML()
	host, _, err := net.SplitHostPort(o.Addr)
	if err != nil {
		return fmt.Errorf("addr: %w", err)
	}
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return fmt.Errorf("buddy serve binds localhost only (got %q)", host)
	}

	pages, docsSource, err := loadDocPages(o.DocsRoot)
	if err != nil {
		return err
	}
	md := newMarkdown()
	store := newSessionStore(o.TenantName, o.CredName, o.Opts, o.Logger)

	mux := http.NewServeMux()
	mux.HandleFunc("/buddy.png", serveLogo)
	mux.HandleFunc("/favicon.ico", serveLogo)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/docs/", http.StatusFound)
	})
	mux.HandleFunc("/docs", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/docs/", http.StatusFound)
	})
	mux.HandleFunc("/docs/", func(w http.ResponseWriter, r *http.Request) {
		if strings.Trim(strings.TrimPrefix(r.URL.Path, "/docs/"), "/") == "help/builtins" {
			http.Redirect(w, r, "/docs/builtins", http.StatusFound)
			return
		}
		// Live docs root: re-read markdown on each request so docs-serve picks up edits.
		live := pages
		if docsSource != "" && docsSource != "embedded" {
			if p, _, err := loadDocPages(o.DocsRoot); err == nil {
				live = p
			}
		}
		serveDocsHTML(w, r, live, md)
	})
	mux.HandleFunc("/ide", func(w http.ResponseWriter, r *http.Request) {
		serveIDEHTML(w, r)
	})
	mux.HandleFunc("/config", serveConfigHTML)
	mux.HandleFunc("/api/config", handleConfigAPI)
	mux.HandleFunc("/api/config/", handleConfigAPI)
	mux.HandleFunc("/api/pages", func(w http.ResponseWriter, r *http.Request) {
		type item struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		}
		out := make([]item, 0, len(pages))
		for _, p := range pages {
			out = append(out, item{ID: p.ID, Title: p.Title})
		}
		writeJSON(w, http.StatusOK, out)
	})
	mux.HandleFunc("/api/search", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		writeJSON(w, http.StatusOK, searchPages(pages, q, 40))
	})
	mux.HandleFunc("/api/page/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/page/")
		p := pageByID(pages, id)
		if p == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		htmlBody, err := renderMarkdown(md, prepareDocMarkdown(p.Markdown, pages, p.ID))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"id":    p.ID,
			"title": p.Title,
			"html":  htmlBody,
			"toc":   extractTOC(p.Markdown),
		})
	})
	mux.HandleFunc("/api/creds", func(w http.ResponseWriter, r *http.Request) {
		ch := r.URL.Query().Get("channel")
		if ch == "" {
			ch = buddy.ChannelIDE
		}
		writeJSON(w, http.StatusOK, sessionSelectionJSON(ch, o.TenantName, o.CredName))
	})
	mux.HandleFunc("/api/libraries", handleLibrariesList)
	mux.HandleFunc("/api/libraries/", handleLibraryDetail)
	mux.HandleFunc("/api/fs/root", handleFSRoot)
	mux.HandleFunc("/api/fs/read", handleFSRead)
	mux.HandleFunc("/api/fs/write", handleFSWrite)
	mux.HandleFunc("/api/session", store.handleCreate)
	mux.HandleFunc("/api/session/", func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/api/session/")
		parts := strings.Split(rest, "/")
		if len(parts) == 0 || parts[0] == "" {
			http.NotFound(w, r)
			return
		}
		id := parts[0]
		if len(parts) == 1 {
			store.handleDelete(w, r, id)
			return
		}
		switch parts[1] {
		case "run":
			store.handleRun(w, r, id)
		case "confirm":
			store.handleConfirm(w, r, id)
		case "config":
			store.handleConfig(w, r, id)
		case "globals":
			store.handleGlobals(w, r, id)
		case "stash":
			store.handleStash(w, r, id)
		case "drop":
			store.handleDrop(w, r, id)
		case "reset":
			store.handleReset(w, r, id)
		default:
			http.NotFound(w, r)
		}
	})

	srv := &http.Server{
		Addr:              o.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ln, err := net.Listen("tcp", o.Addr)
	if err != nil {
		return err
	}
	url := "http://" + o.Addr + "/docs/"
	o.Logger.Info("buddy serve listening", "url", url, "docs", docsSource)
	fmt.Printf("buddy serve: %s\n", url)
	fmt.Printf("config:      http://%s/config\n", o.Addr)
	fmt.Printf("ide:         http://%s/ide\n", o.Addr)

	if o.Open {
		go func() {
			time.Sleep(200 * time.Millisecond)
			_ = openBrowser(url)
		}()
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ln)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return fmt.Errorf("unsupported OS for --open")
	}
	return cmd.Start()
}

type authInfo = buddy.AuthSummary

// sessionSelectionJSON builds the /api/creds payload (tenants + auths) for channel.
func sessionSelectionJSON(channel, defaultTenant, defaultAuth string) map[string]any {
	return buddy.SessionSelectionFor(channel, defaultTenant, defaultAuth).ToMap()
}
