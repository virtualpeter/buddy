package tokenauth

import (
	"bufio"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"buddy/internal/buddy/agent"
)

// fakeAgentSocket serves one canned JSON reply per connection and points the
// agent client at it for the duration of the test.
func fakeAgentSocket(t *testing.T, reply func(op string) map[string]any) {
	t.Helper()
	fakeAgentSocketRaw(t, func(req map[string]any) map[string]any {
		op, _ := req["op"].(string)
		return reply(op)
	})
}

// fakeAgentSocketRaw hands the whole decoded request to the reply function,
// for tests that care about fields such as force.
func fakeAgentSocketRaw(t *testing.T, reply func(req map[string]any) map[string]any) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("unix socket fake not run on windows")
	}
	// Not t.TempDir(): sun_path is 104 bytes and Go's test names are long.
	dir, err := os.MkdirTemp("", "bd")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, "agent.sock")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				line, err := bufio.NewReader(conn).ReadBytes('\n')
				if err != nil {
					return
				}
				var req map[string]any
				_ = json.Unmarshal(line, &req)
				out, err := json.Marshal(reply(req))
				if err != nil {
					return
				}
				_, _ = conn.Write(append(out, '\n'))
			}()
		}
	}()
	t.Setenv(agent.EnvDisable, "")
	t.Setenv(agent.EnvSocket, path)
}

func TestAgentMintsTokenWithoutVault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("token endpoint must not be called when the agent mints")
	}))
	t.Cleanup(srv.Close)
	expiry := time.Now().Add(20 * time.Minute).UTC().Truncate(time.Second)
	fakeAgentSocket(t, func(string) map[string]any {
		return map[string]any{
			"ok":           true,
			"access_token": "agent-bearer",
			"expires_at":   expiry.Format(time.RFC3339),
		}
	})

	var loads atomic.Int32
	a := NewAuthenticator(Config{
		Name:     "sandbox",
		BaseURL:  srv.URL,
		ClientId: "cid",
		CacheKey: "agent-mint",
		LoadSecrets: func() (string, string, error) {
			loads.Add(1)
			return "", "from-vault", nil
		},
	})
	tok, err := a.GetToken()
	if err != nil {
		t.Fatal(err)
	}
	if tok != "agent-bearer" {
		t.Fatalf("token = %q", tok)
	}
	if loads.Load() != 0 {
		t.Fatalf("LoadSecrets called %d times; the agent must keep the vault shut", loads.Load())
	}
}

func TestAgentLockedDoesNotOpenVault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("token endpoint must not be called when the agent is locked")
	}))
	t.Cleanup(srv.Close)
	fakeAgentSocket(t, func(string) map[string]any {
		return map[string]any{"ok": false, "error": "locked", "message": "unlock it from the menu bar"}
	})

	var loads atomic.Int32
	a := NewAuthenticator(Config{
		Name:     "sandbox",
		BaseURL:  srv.URL,
		ClientId: "cid",
		CacheKey: "agent-locked",
		LoadSecrets: func() (string, string, error) {
			loads.Add(1)
			return "", "from-vault", nil
		},
	})
	_, err := a.GetToken()
	if err == nil || !strings.Contains(err.Error(), "locked") {
		t.Fatalf("err = %v, want a locked error", err)
	}
	if loads.Load() != 0 {
		t.Fatalf("LoadSecrets called %d times; a locked agent must not fall through", loads.Load())
	}
}

func TestAgentOwnedAuthHasNoVaultFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("token endpoint must not be called for an agent-owned auth")
	}))
	t.Cleanup(srv.Close)
	fakeAgentSocket(t, func(string) map[string]any {
		return map[string]any{"ok": false, "error": "unsupported", "message": "no integration named sandbox"}
	})

	var loads atomic.Int32
	a := NewAuthenticator(Config{
		Name:       "sandbox",
		BaseURL:    srv.URL,
		ClientId:   "cid",
		CacheKey:   "agent-owned",
		AgentOwned: true,
		LoadSecrets: func() (string, string, error) {
			loads.Add(1)
			return "", "from-vault", nil
		},
	})
	_, err := a.GetToken()
	if err == nil || !strings.Contains(err.Error(), "owned by the Buddy Agent") {
		t.Fatalf("err = %v, want the agent-owned error", err)
	}
	if loads.Load() != 0 {
		t.Fatalf("LoadSecrets called %d times; an agent-owned auth has no vault copy", loads.Load())
	}
}

func TestAgentOwnedAuthWithAgentDisabled(t *testing.T) {
	t.Setenv(agent.EnvDisable, "1")
	a := NewAuthenticator(Config{
		Name:       "sandbox",
		BaseURL:    "https://example.jamfcloud.com",
		ClientId:   "cid",
		CacheKey:   "agent-owned-disabled",
		AgentOwned: true,
	})
	_, err := a.GetToken()
	if err == nil || !strings.Contains(err.Error(), agent.EnvDisable) {
		t.Fatalf("err = %v, want the disabled-agent error", err)
	}
}

func TestAgentUnsupportedFallsBackToVault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.Form.Get("client_secret") != "from-vault" {
			t.Fatalf("client_secret = %q", r.Form.Get("client_secret"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"vault-bearer","expires_in":3600,"token_type":"Bearer"}`))
	}))
	t.Cleanup(srv.Close)
	fakeAgentSocket(t, func(string) map[string]any {
		return map[string]any{"ok": false, "error": "unsupported", "message": "no spike credential"}
	})

	a := NewAuthenticator(Config{
		Name:     "sandbox",
		BaseURL:  srv.URL,
		ClientId: "cid",
		CacheKey: "agent-unsupported",
		LoadSecrets: func() (string, string, error) {
			return "", "from-vault", nil
		},
	})
	tok, err := a.GetToken()
	if err != nil {
		t.Fatal(err)
	}
	if tok != "vault-bearer" {
		t.Fatalf("token = %q", tok)
	}
}

func TestAgentSkippedOnDiskCacheHit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("token endpoint must not be called on cache hit")
	}))
	t.Cleanup(srv.Close)
	var dialed atomic.Int32
	fakeAgentSocket(t, func(string) map[string]any {
		dialed.Add(1)
		return map[string]any{"ok": true, "access_token": "agent-bearer"}
	})

	cacheKey := "agent-cache-hit"
	if err := saveDiskToken(cacheKey, srv.URL, "cached-tok", time.Now().Add(30*time.Minute)); err != nil {
		t.Fatal(err)
	}
	a := NewAuthenticator(Config{
		Name:     "sandbox",
		BaseURL:  srv.URL,
		ClientId: "cid",
		CacheKey: cacheKey,
	})
	tok, err := a.GetToken()
	if err != nil {
		t.Fatal(err)
	}
	if tok != "cached-tok" {
		t.Fatalf("token = %q", tok)
	}
	if dialed.Load() != 0 {
		t.Fatalf("agent dialled %d times; a valid disk token must skip it", dialed.Load())
	}
}

// A bearer the Agent minted must not outlive the Agent's lock, so it stays in
// this process and never reaches ~/.config/buddy/tokens/.
func TestAgentTokenIsNotCachedOnDisk(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("token endpoint must not be called when the agent mints")
	}))
	t.Cleanup(srv.Close)
	expiry := time.Now().Add(20 * time.Minute).UTC().Truncate(time.Second)
	fakeAgentSocket(t, func(string) map[string]any {
		return map[string]any{
			"ok":           true,
			"access_token": "agent-bearer",
			"expires_at":   expiry.Format(time.RFC3339),
		}
	})

	cacheKey := "agent-no-disk"
	a := NewAuthenticator(Config{
		Name:     "sandbox",
		BaseURL:  srv.URL,
		ClientId: "cid",
		CacheKey: cacheKey,
	})
	if _, err := a.GetToken(); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := loadDiskToken(cacheKey, srv.URL, clockSkewWindow); ok {
		t.Fatal("an agent-minted bearer was written to the disk cache")
	}
}

// An Agent-owned auth ignores the disk cache outright: a file from an older
// buddy would keep working after the Agent was locked.
func TestAgentOwnedIgnoresDiskCache(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("token endpoint must not be called for an agent-owned auth")
	}))
	t.Cleanup(srv.Close)
	var dialed atomic.Int32
	fakeAgentSocket(t, func(string) map[string]any {
		dialed.Add(1)
		return map[string]any{
			"ok":           true,
			"access_token": "agent-bearer",
			"expires_at":   time.Now().Add(20 * time.Minute).UTC().Format(time.RFC3339),
		}
	})

	cacheKey := "agent-owned-stale-file"
	if err := saveDiskToken(cacheKey, srv.URL, "stale-tok", time.Now().Add(30*time.Minute)); err != nil {
		t.Fatal(err)
	}
	a := NewAuthenticator(Config{
		Name:       "sandbox",
		BaseURL:    srv.URL,
		ClientId:   "cid",
		CacheKey:   cacheKey,
		AgentOwned: true,
	})
	tok, err := a.GetToken()
	if err != nil {
		t.Fatal(err)
	}
	if tok != "agent-bearer" {
		t.Fatalf("token = %q; the stale file must not win", tok)
	}
	if dialed.Load() == 0 {
		t.Fatal("the agent should have been asked")
	}
	if _, _, ok := loadDiskToken(cacheKey, srv.URL, clockSkewWindow); ok {
		t.Fatal("the stale file should have been swept")
	}
}

// On 401 the Agent mints the replacement rather than buddy opening the vault.
func TestAgentRefreshGoesBackToTheAgent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("token endpoint must not be called when the agent mints")
	}))
	t.Cleanup(srv.Close)
	var forced atomic.Int32
	fakeAgentSocketRaw(t, func(req map[string]any) map[string]any {
		if force, _ := req["force"].(bool); force {
			forced.Add(1)
		}
		return map[string]any{
			"ok":           true,
			"access_token": "agent-bearer-2",
			"expires_at":   time.Now().Add(20 * time.Minute).UTC().Format(time.RFC3339),
		}
	})

	var loads atomic.Int32
	a := NewAuthenticator(Config{
		Name:       "sandbox",
		BaseURL:    srv.URL,
		ClientId:   "cid",
		CacheKey:   "agent-refresh",
		AgentOwned: true,
		LoadSecrets: func() (string, string, error) {
			loads.Add(1)
			return "", "from-vault", nil
		},
	})
	if err := a.RefreshToken(); err != nil {
		t.Fatal(err)
	}
	tok, err := a.GetToken()
	if err != nil {
		t.Fatal(err)
	}
	if tok != "agent-bearer-2" {
		t.Fatalf("token = %q", tok)
	}
	if forced.Load() == 0 {
		t.Fatal("refresh must ask the agent to bypass its own cache")
	}
	if loads.Load() != 0 {
		t.Fatalf("LoadSecrets called %d times during refresh", loads.Load())
	}
}

// A token request asks a locked Agent to raise its own prompt; the three ways
// that can fail all have to reach the user intact and must never be papered
// over by opening the local vault instead.
func TestAgentUnlockFailuresAreFatal(t *testing.T) {
	cases := []struct {
		name  string
		code  string
		msg   string
		wants string
	}{
		{"declined", "unlock_declined", "unlock declined", "declined at the presence prompt"},
		{"timeout", "unlock_timeout", "unlock prompt went unanswered for 60s", "unlock timed out"},
		{"unavailable", "unlock_unavailable", "no presence on this Mac", "cannot prompt for unlock"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Fatal("token endpoint must not be called when the unlock fails")
			}))
			t.Cleanup(srv.Close)
			var allowed atomic.Int32
			fakeAgentSocketRaw(t, func(req map[string]any) map[string]any {
				if allow, _ := req["allow_unlock"].(bool); allow {
					allowed.Add(1)
				}
				return map[string]any{"ok": false, "error": tc.code, "message": tc.msg}
			})

			var loads atomic.Int32
			a := NewAuthenticator(Config{
				Name:     "sandbox",
				BaseURL:  srv.URL,
				ClientId: "cid",
				CacheKey: "agent-" + tc.code,
				LoadSecrets: func() (string, string, error) {
					loads.Add(1)
					return "", "from-vault", nil
				},
			})
			_, err := a.GetToken()
			if err == nil || !strings.Contains(err.Error(), tc.wants) {
				t.Fatalf("err = %v, want one mentioning %q", err, tc.wants)
			}
			if allowed.Load() == 0 {
				t.Fatal("the token request should have allowed an unlock prompt")
			}
			if loads.Load() != 0 {
				t.Fatalf("LoadSecrets called %d times; a failed unlock must not open the vault", loads.Load())
			}
		})
	}
}

// BUDDY_AGENT_NO_PROMPT=1 keeps the pre-0.3 behaviour: the Agent is never asked
// to prompt, so a locked one fails straight away.
func TestAgentNoPromptOptOut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("token endpoint must not be called when the agent is locked")
	}))
	t.Cleanup(srv.Close)
	var asked, allowed atomic.Int32
	fakeAgentSocketRaw(t, func(req map[string]any) map[string]any {
		asked.Add(1)
		if allow, _ := req["allow_unlock"].(bool); allow {
			allowed.Add(1)
		}
		return map[string]any{"ok": false, "error": "locked", "message": "unlock it from the menu bar"}
	})
	t.Setenv(agent.EnvNoPrompt, "1")

	a := NewAuthenticator(Config{
		Name:     "sandbox",
		BaseURL:  srv.URL,
		ClientId: "cid",
		CacheKey: "agent-no-prompt",
	})
	_, err := a.GetToken()
	if err == nil || !strings.Contains(err.Error(), "locked") {
		t.Fatalf("err = %v, want the plain locked error", err)
	}
	if asked.Load() == 0 {
		t.Fatal("the agent should still have been asked for a token")
	}
	if allowed.Load() != 0 {
		t.Fatalf("allow_unlock was set %d times despite %s=1", allowed.Load(), agent.EnvNoPrompt)
	}
}

func TestAgentIgnoredWhenDisabled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"vault-bearer","expires_in":3600,"token_type":"Bearer"}`))
	}))
	t.Cleanup(srv.Close)
	var dialed atomic.Int32
	fakeAgentSocket(t, func(string) map[string]any {
		dialed.Add(1)
		return map[string]any{"ok": true, "access_token": "agent-bearer"}
	})
	t.Setenv(agent.EnvDisable, "1")

	a := NewAuthenticator(Config{
		Name:     "sandbox",
		BaseURL:  srv.URL,
		ClientId: "cid",
		CacheKey: "agent-disabled",
		LoadSecrets: func() (string, string, error) {
			return "", "from-vault", nil
		},
	})
	tok, err := a.GetToken()
	if err != nil {
		t.Fatal(err)
	}
	if tok != "vault-bearer" || dialed.Load() != 0 {
		t.Fatalf("token = %q, agent dialled %d times with BUDDY_NO_AGENT=1", tok, dialed.Load())
	}
}
