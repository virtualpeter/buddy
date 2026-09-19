package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// fakeAgent serves one canned response per request op, so the Go client can be
// exercised without the Swift Agent.
func fakeAgent(t *testing.T, handler func(request) response) string {
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
				reader := bufio.NewReader(conn)
				line, err := reader.ReadBytes('\n')
				if err != nil {
					return
				}
				var req request
				if err := json.Unmarshal(line, &req); err != nil {
					return
				}
				out, err := json.Marshal(handler(req))
				if err != nil {
					return
				}
				_, _ = conn.Write(append(out, '\n'))
			}()
		}
	}()
	t.Setenv(EnvSocket, path)
	t.Setenv(EnvDisable, "")
	return path
}

func TestStatusAndToken(t *testing.T) {
	expiry := time.Now().Add(20 * time.Minute).UTC().Truncate(time.Second)
	fakeAgent(t, func(req request) response {
		switch req.Op {
		case "status":
			return response{OK: true, Present: true, Unlocked: true, Version: "0.1.0"}
		case "token":
			if req.Auth != "sandbox" || req.BaseURL != "https://example.jamfcloud.com" || req.Kind != "oauth" {
				return response{Error: "bad_request", Message: "unexpected fields"}
			}
			return response{OK: true, AccessToken: "bearer-1", ExpiresAt: expiry.Format(time.RFC3339)}
		}
		return response{Error: "bad_request", Message: "unknown op"}
	})

	info, err := Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !info.Present || !info.Unlocked || info.Version != "0.1.0" {
		t.Fatalf("unexpected status: %+v", info)
	}

	tok, err := Token(context.Background(), TokenRequest{
		Auth:    "sandbox",
		BaseURL: "https://example.jamfcloud.com",
		Kind:    "oauth",
	})
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	if tok.AccessToken != "bearer-1" {
		t.Fatalf("token = %q", tok.AccessToken)
	}
	if !tok.ExpiresAt.Equal(expiry) {
		t.Fatalf("expiry = %v, want %v", tok.ExpiresAt, expiry)
	}
}

func TestConfigAndIntegrationsServedWhileLocked(t *testing.T) {
	doc := `{"context":{"default":"sandbox"},"contexts":{"sandbox":{"url":"https://example.jamfcloud.com"}}}`
	fakeAgent(t, func(req request) response {
		switch req.Op {
		case "status":
			return response{
				OK: true, Present: true, Unlocked: false, Version: "0.2.0",
				PeerCheck: "codesign", IntegrationCount: 1, TokenCount: 0,
			}
		case "config":
			return response{OK: true, Config: json.RawMessage(doc)}
		case "integrations":
			return response{OK: true, Integrations: []Integration{{
				Context: "sandbox", Auth: "sandbox-admin", Type: "platform",
				Scope: "tenant", Secret: "stored",
			}}}
		case "tokens":
			return response{OK: true, Tokens: []TokenInfo{}}
		case "token":
			return response{Error: "locked", Message: "unlock it from the menu bar"}
		}
		return response{Error: "bad_request", Message: "unknown op"}
	})

	info, err := Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if info.Unlocked || info.Integrations != 1 || info.PeerCheck != "codesign" {
		t.Fatalf("unexpected status: %+v", info)
	}

	raw, err := Config(context.Background())
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("config json: %v", err)
	}
	if _, ok := parsed["contexts"]; !ok {
		t.Fatalf("config document = %s", raw)
	}

	rows, err := Integrations(context.Background())
	if err != nil {
		t.Fatalf("integrations: %v", err)
	}
	if len(rows) != 1 || rows[0].Auth != "sandbox-admin" || rows[0].Secret != "stored" || rows[0].Scope != "tenant" {
		t.Fatalf("integrations = %+v", rows)
	}

	// Only minting needs the unlock.
	if _, err := Token(context.Background(), TokenRequest{
		Auth: "sandbox-admin", BaseURL: "https://example.jamfcloud.com", Kind: "oauth",
	}); err == nil || !strings.Contains(err.Error(), "locked") {
		t.Fatalf("token err = %v, want locked", err)
	}
}

func TestLockReportsState(t *testing.T) {
	fakeAgent(t, func(req request) response {
		if req.Op != "lock" {
			return response{Error: "bad_request", Message: "unexpected op " + req.Op}
		}
		return response{OK: true, Present: true, Unlocked: false, Version: "0.2.0"}
	})
	info, err := Lock(context.Background())
	if err != nil {
		t.Fatalf("lock: %v", err)
	}
	if info.Unlocked {
		t.Fatal("lock should report a locked agent")
	}
}

func TestAbsentSocket(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix socket fake not run on windows")
	}
	t.Setenv(EnvDisable, "")
	t.Setenv(EnvSocket, filepath.Join(t.TempDir(), "missing.sock"))
	if _, err := Status(context.Background()); !errors.Is(err, ErrAbsent) {
		t.Fatalf("status err = %v, want ErrAbsent", err)
	}
	_, err := Token(context.Background(), TokenRequest{Auth: "a", BaseURL: "https://x", Kind: "oauth"})
	if !errors.Is(err, ErrAbsent) {
		t.Fatalf("token err = %v, want ErrAbsent", err)
	}
}

func TestLockedIsFatal(t *testing.T) {
	fakeAgent(t, func(request) response {
		return response{Error: "locked", Message: "unlock it from the menu bar"}
	})
	_, err := Token(context.Background(), TokenRequest{Auth: "a", BaseURL: "https://x", Kind: "oauth"})
	if err == nil || errors.Is(err, ErrAbsent) || errors.Is(err, ErrUnsupported) {
		t.Fatalf("locked err = %v, want a fatal error", err)
	}
	if !strings.Contains(err.Error(), "locked") {
		t.Fatalf("locked err = %v", err)
	}
}

func TestUnsupportedFallsBack(t *testing.T) {
	fakeAgent(t, func(request) response {
		return response{Error: "unsupported", Message: "basic auth is not served by the Agent spike"}
	})
	_, err := Token(context.Background(), TokenRequest{Auth: "a", BaseURL: "https://x", Kind: "basic"})
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v, want ErrUnsupported", err)
	}
}

func TestDeniedIsFatal(t *testing.T) {
	fakeAgent(t, func(request) response {
		return response{Error: "denied", Message: "peer failed codesign requirement"}
	})
	_, err := Token(context.Background(), TokenRequest{Auth: "a", BaseURL: "https://x", Kind: "oauth"})
	if err == nil || errors.Is(err, ErrAbsent) || errors.Is(err, ErrUnsupported) {
		t.Fatalf("denied err = %v, want a fatal error", err)
	}
}

// Status carries the v0.3 presence fields so 'buddy agent status' can say
// whether a locked Agent is able to prompt at all.
func TestStatusCarriesPresenceFields(t *testing.T) {
	fakeAgent(t, func(request) response {
		return response{
			OK: true, Present: true, Unlocked: false, Version: "0.3.0",
			Presence: "simulated", IdleLockSeconds: 900, UnlockTimeoutSeconds: 60,
		}
	})
	info, err := Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if info.Presence != "simulated" {
		t.Fatalf("presence = %q", info.Presence)
	}
	if info.IdleLock != 15*time.Minute || info.UnlockTimeout != time.Minute {
		t.Fatalf("idle lock = %v, unlock timeout = %v", info.IdleLock, info.UnlockTimeout)
	}
}

func TestStatusCarriesIntegrityAndManagedPrefs(t *testing.T) {
	fakeAgent(t, func(request) response {
		return response{
			OK: true, Present: true, Unlocked: false, Version: "0.4.0",
			ConfigIntegrity: "sealed", ManagedPrefs: "channels cli, features",
		}
	})
	info, err := Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if info.ConfigIntegrity != "sealed" {
		t.Fatalf("integrity = %q", info.ConfigIntegrity)
	}
	if info.ManagedPrefs != "channels cli, features" {
		t.Fatalf("managed prefs = %q", info.ManagedPrefs)
	}
}

// Each unlock failure keeps its own wording: "declined" and "timed out" are
// different problems for the user, and neither is a reason to open the vault.
func TestUnlockOutcomesAreDistinctAndFatal(t *testing.T) {
	cases := map[string]string{
		"unlock_declined":    "declined at the presence prompt",
		"unlock_timeout":     "unlock timed out",
		"unlock_unavailable": "cannot prompt for unlock",
	}
	for code, want := range cases {
		t.Run(code, func(t *testing.T) {
			fakeAgent(t, func(req request) response {
				if !req.AllowUnlock {
					return response{Error: "locked", Message: "allow_unlock was not set"}
				}
				return response{Error: code, Message: "from the agent"}
			})
			_, err := Token(context.Background(), TokenRequest{
				Auth: "a", BaseURL: "https://x", Kind: "oauth", AllowUnlock: true,
			})
			if err == nil || errors.Is(err, ErrAbsent) || errors.Is(err, ErrUnsupported) {
				t.Fatalf("err = %v, want a fatal error", err)
			}
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("err = %v, want one mentioning %q", err, want)
			}
		})
	}
}

// A signed Agent rejects an unsigned buddy on the Team ID, which is the most
// likely way anyone meets "denied". The error has to name the fix.
func TestDeniedExplainsTheSigningMismatch(t *testing.T) {
	fakeAgent(t, func(req request) response {
		return response{Error: "denied", Message: "peer is not signed by this team"}
	})
	_, err := Token(context.Background(), TokenRequest{
		Auth: "a", BaseURL: "https://x", Kind: "oauth",
	})
	if err == nil || errors.Is(err, ErrAbsent) || errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v, want a fatal error that does not fall back to the vault", err)
	}
	for _, want := range []string{"peer is not signed by this team", "Helpers/buddy", "BUDDY_AGENT_ALLOW_UNSIGNED=1"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("err = %v, want one mentioning %q", err, want)
		}
	}
}

func TestNoPromptClearsAllowUnlock(t *testing.T) {
	fakeAgent(t, func(req request) response {
		if req.AllowUnlock {
			return response{Error: "bad_request", Message: "allow_unlock leaked past the opt-out"}
		}
		return response{OK: true, AccessToken: "bearer"}
	})
	t.Setenv(EnvNoPrompt, "1")
	if !PromptSuppressed() {
		t.Fatal("BUDDY_AGENT_NO_PROMPT=1 should suppress prompting")
	}
	if _, err := Token(context.Background(), TokenRequest{
		Auth: "a", BaseURL: "https://x", Kind: "oauth", AllowUnlock: true,
	}); err != nil {
		t.Fatalf("token: %v", err)
	}
}

func TestEnabledKillSwitch(t *testing.T) {
	t.Setenv(EnvSocket, filepath.Join(t.TempDir(), "agent.sock"))
	t.Setenv(EnvDisable, "")
	if !Enabled() {
		t.Fatal("explicit socket should enable the agent")
	}
	t.Setenv(EnvDisable, "1")
	if Enabled() {
		t.Fatal("BUDDY_NO_AGENT=1 must disable the agent")
	}
}

func TestSocketPathDefault(t *testing.T) {
	t.Setenv(EnvSocket, "")
	path, err := SocketPath()
	if err != nil {
		t.Fatalf("socket path: %v", err)
	}
	if !strings.HasSuffix(path, filepath.Join("Buddy", "agent.sock")) {
		t.Fatalf("socket path = %q", path)
	}
}

func TestBadExpiry(t *testing.T) {
	fakeAgent(t, func(request) response {
		return response{OK: true, AccessToken: "bearer", ExpiresAt: "not-a-date"}
	})
	if _, err := Token(context.Background(), TokenRequest{Auth: "a", BaseURL: "https://x", Kind: "oauth"}); err == nil {
		t.Fatal("expected a parse error for a bad expires_at")
	}
}
