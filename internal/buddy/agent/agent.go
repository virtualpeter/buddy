// Package agent talks to the optional macOS Buddy Agent: a menu-bar app that
// owns vault unlock and mints Jamf bearer tokens so the DEK never enters the
// CLI process. The wire is newline-delimited JSON over a unix-domain socket.
//
// Callers treat the Agent as best-effort discovery but strict once found:
//
//	ErrAbsent      no socket — use the in-process vault as before
//	ErrUnsupported the Agent cannot serve this auth — use the vault
//	any other err  the Agent is there and said no — fail, do not open the vault
//
// Set BUDDY_NO_AGENT=1 to ignore the Agent entirely (bit-identical to the
// pre-Agent behaviour). See macos/BuddyAgent/README.md.
package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// EnvDisable is the kill switch: never dial the Agent.
	EnvDisable = "BUDDY_NO_AGENT"
	// EnvSocket overrides the socket path (same variable on the Swift side).
	EnvSocket = "BUDDY_AGENT_SOCK"
	// EnvNoPrompt stops buddy asking a locked Agent to raise its presence
	// prompt; a locked Agent then fails the way it did before v0.3.
	EnvNoPrompt = "BUDDY_AGENT_NO_PROMPT"

	// dialTimeout keeps a wedged Agent from stalling every buddy command.
	dialTimeout = 2 * time.Second
	// callTimeout covers the Agent's own round trip to the Jamf token endpoint.
	callTimeout = 35 * time.Second
	// unlockCallTimeout also covers a person walking back to the Mac. It sits
	// above the Agent's own 60s prompt timeout so the Agent's answer wins the
	// race and buddy can report why the unlock failed.
	unlockCallTimeout = 90 * time.Second
)

var (
	// ErrAbsent means no Agent is listening; the caller falls back to the vault.
	ErrAbsent = errors.New("buddy agent is not running")
	// ErrUnsupported means the Agent declined this auth (for example basic auth
	// in the spike); the caller falls back to the vault.
	ErrUnsupported = errors.New("buddy agent does not serve this auth")
)

// Info is the reply to the status op.
type Info struct {
	Present  bool
	Unlocked bool
	Version  string
	Socket   string
	// PeerCheck is "codesign" or "unsigned-allowed" (dev Agents).
	PeerCheck string
	// Integrations is how many workplace+auth pairs the Agent owns.
	Integrations int
	// Tokens is how many bearers the Agent has cached this session.
	Tokens int
	// Presence is how the Agent proves the person is there: "acl" (Keychain
	// userPresence), "simulated" (LocalAuthentication on an unsigned build),
	// or "unavailable". Empty from a pre-0.3 Agent.
	Presence string
	// IdleLock is the idle period after which the Agent relocks itself; 0 means
	// never.
	IdleLock time.Duration
	// UnlockTimeout is how long the Agent leaves its prompt on screen; 0 means
	// it will not prompt at all.
	UnlockTimeout time.Duration
	// ConfigIntegrity is the Agent's agent-config.json seal: "ok", "sealed",
	// "missing", "tampered", "session_stale" (Keychain rejected the unlock
	// context; the Agent relocks and prompts again, then relaunches once if
	// that fresh unlock is still rejected), or "unavailable". Empty from a
	// pre-integrity Agent.
	ConfigIntegrity string
	// ManagedPrefs is a short summary of MDM keys the Agent can see (optional).
	ManagedPrefs string
}

// Integration is one workplace+auth pair the Agent owns. Secret is "stored" or
// "missing": the value itself never crosses the socket.
type Integration struct {
	Context  string `json:"context"`
	Auth     string `json:"auth"`
	Type     string `json:"type"`
	URL      string `json:"url,omitempty"`
	ClientID string `json:"client_id,omitempty"`
	Username string `json:"username,omitempty"`
	Scope    string `json:"scope,omitempty"`
	Secret   string `json:"secret"`
	Problem  string `json:"problem,omitempty"`
}

// TokenInfo describes one cached bearer without disclosing it.
type TokenInfo struct {
	Auth      string `json:"auth"`
	BaseURL   string `json:"base_url"`
	ExpiresAt string `json:"expires_at"`
}

// TokenRequest asks the Agent to mint (or reuse) a bearer for one auth.
type TokenRequest struct {
	Auth    string
	BaseURL string
	// Kind is "oauth", "platform", or "basic" (buddy's auth types).
	Kind string
	// Force skips the Agent's own token cache.
	Force bool
	// AllowUnlock lets a locked Agent raise its own presence prompt and keeps
	// this call waiting for the answer. Off by default so a caller has to mean
	// it; a pre-0.3 Agent ignores the field and still answers "locked".
	AllowUnlock bool
}

// PromptSuppressed reports whether the user has asked buddy never to trigger
// the Agent's presence prompt (BUDDY_AGENT_NO_PROMPT=1).
func PromptSuppressed() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(EnvNoPrompt))) {
	case "1", "true", "yes":
		return true
	}
	return false
}

// Bearer is a minted access token. No secret material ever crosses the socket.
type Bearer struct {
	AccessToken string
	ExpiresAt   time.Time
}

// Enabled reports whether the Agent should be consulted at all. It is macOS
// only, except when EnvSocket points somewhere explicitly (tests, dev).
func Enabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(EnvDisable))) {
	case "1", "true", "yes":
		return false
	}
	if strings.TrimSpace(os.Getenv(EnvSocket)) != "" {
		return true
	}
	return runtime.GOOS == "darwin"
}

// KeychainService is the Agent generic-password service. Secrets stay there;
// buddy receives tokens, not the secret values.
const KeychainService = "virtualpete.buddy.agent"

// SupportDir is ~/Library/Application Support/Buddy.
func SupportDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "", fmt.Errorf("resolve buddy agent directory: home directory unavailable")
	}
	return filepath.Join(home, "Library", "Application Support", "Buddy"), nil
}

// ConfigPath is the Agent identity file, agent-config.json under SupportDir.
func ConfigPath() (string, error) {
	dir, err := SupportDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "agent-config.json"), nil
}

// SocketPath is ~/Library/Application Support/Buddy/agent.sock, or EnvSocket.
func SocketPath() (string, error) {
	if p := strings.TrimSpace(os.Getenv(EnvSocket)); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "", fmt.Errorf("resolve buddy agent socket: home directory unavailable")
	}
	return filepath.Join(home, "Library", "Application Support", "Buddy", "agent.sock"), nil
}

// Status reports whether the Agent is running and unlocked. Absent socket is
// ErrAbsent, not a failure.
func Status(ctx context.Context) (Info, error) {
	path, err := SocketPath()
	if err != nil {
		return Info{}, err
	}
	return statusOp(ctx, path, "status")
}

// Lock tells the Agent to forget its cached bearers and relock. It reports the
// state afterwards.
func Lock(ctx context.Context) (Info, error) {
	path, err := SocketPath()
	if err != nil {
		return Info{}, err
	}
	return statusOp(ctx, path, "lock")
}

func statusOp(ctx context.Context, path, op string) (Info, error) {
	resp, err := call(ctx, path, request{Op: op})
	if err != nil {
		return Info{Socket: path}, err
	}
	return Info{
		Present:         true,
		Unlocked:        resp.Unlocked,
		Version:         resp.Version,
		Socket:          path,
		PeerCheck:       resp.PeerCheck,
		Integrations:    resp.IntegrationCount,
		Tokens:          resp.TokenCount,
		Presence:        strings.TrimSpace(resp.Presence),
		IdleLock:        time.Duration(resp.IdleLockSeconds) * time.Second,
		UnlockTimeout:   time.Duration(resp.UnlockTimeoutSeconds) * time.Second,
		ConfigIntegrity: strings.TrimSpace(resp.ConfigIntegrity),
		ManagedPrefs:    strings.TrimSpace(resp.ManagedPrefs),
	}, nil
}

// Config returns the Agent's config document: a config.yaml-shaped JSON object
// with no secrets in it. Served while the Agent is locked, because identity is
// not secret material — only minting needs the unlock.
func Config(ctx context.Context) (json.RawMessage, error) {
	path, err := SocketPath()
	if err != nil {
		return nil, err
	}
	resp, err := call(ctx, path, request{Op: "config"})
	if err != nil {
		return nil, err
	}
	if len(resp.Config) == 0 {
		return nil, fmt.Errorf("buddy agent returned no config document")
	}
	return resp.Config, nil
}

// Integrations lists what the Agent owns, including whether each secret is
// stored. Served while locked.
func Integrations(ctx context.Context) ([]Integration, error) {
	path, err := SocketPath()
	if err != nil {
		return nil, err
	}
	resp, err := call(ctx, path, request{Op: "integrations"})
	if err != nil {
		return nil, err
	}
	return resp.Integrations, nil
}

// Tokens lists the Agent's cached bearers as metadata only.
func Tokens(ctx context.Context) ([]TokenInfo, error) {
	path, err := SocketPath()
	if err != nil {
		return nil, err
	}
	resp, err := call(ctx, path, request{Op: "tokens"})
	if err != nil {
		return nil, err
	}
	return resp.Tokens, nil
}

// Token asks the Agent for a bearer.
func Token(ctx context.Context, req TokenRequest) (Bearer, error) {
	path, err := SocketPath()
	if err != nil {
		return Bearer{}, err
	}
	allowUnlock := req.AllowUnlock && !PromptSuppressed()
	if allowUnlock {
		if _, ok := ctx.Deadline(); !ok {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(orBackground(ctx), unlockCallTimeout)
			defer cancel()
		}
	}
	resp, err := call(ctx, path, tokenRequest(req, allowUnlock))
	if errors.Is(err, errKeychainRestart) {
		// The Agent is replacing itself. The listen socket is already closed;
		// wait until the new process binds, then mint once more. A dial that
		// fails here must not surface as ErrAbsent — that would open the vault
		// behind an Agent that is coming back.
		if werr := waitForAgentSocket(ctx, path); werr != nil {
			return Bearer{}, err
		}
		resp, err = call(ctx, path, tokenRequest(req, allowUnlock))
		if errors.Is(err, ErrAbsent) {
			return Bearer{}, fmt.Errorf("%w: the new process did not start listening", errKeychainRestart)
		}
	}
	if err != nil {
		return Bearer{}, err
	}
	if strings.TrimSpace(resp.AccessToken) == "" {
		return Bearer{}, fmt.Errorf("buddy agent returned an empty token")
	}
	expiry := time.Now().Add(15 * time.Minute)
	if strings.TrimSpace(resp.ExpiresAt) != "" {
		parsed, perr := time.Parse(time.RFC3339, resp.ExpiresAt)
		if perr != nil {
			return Bearer{}, fmt.Errorf("buddy agent returned a bad expires_at: %w", perr)
		}
		expiry = parsed
	}
	return Bearer{AccessToken: resp.AccessToken, ExpiresAt: expiry}, nil
}

func tokenRequest(req TokenRequest, allowUnlock bool) request {
	return request{
		Op:          "token",
		Auth:        req.Auth,
		BaseURL:     req.BaseURL,
		Kind:        req.Kind,
		Force:       req.Force,
		AllowUnlock: allowUnlock,
		Client:      tokenLogClient(),
	}
}

// errKeychainRestart is the Agent replacing a process whose data-protection
// keychain session cannot be recovered in place (macOS kept it across a restart,
// or launched it before the keybag was ready).
var errKeychainRestart = errors.New("buddy agent is restarting to recover keychain access")

func waitForAgentSocket(ctx context.Context, path string) error {
	deadline := time.Now().Add(8 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	var last error
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		conn, err := net.DialTimeout("unix", path, 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		last = err
		if !time.Now().Before(deadline) {
			return last
		}
		timer := time.NewTimer(200 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

type request struct {
	Op          string `json:"op"`
	Auth        string `json:"auth,omitempty"`
	BaseURL     string `json:"base_url,omitempty"`
	Kind        string `json:"kind,omitempty"`
	Force       bool   `json:"force,omitempty"`
	AllowUnlock bool   `json:"allow_unlock,omitempty"`
	// Client names the caller in the Agent activity log: cli, script, mcp, or ide.
	// It is not an auth check. Channels policy for scripts stays cli.
	Client string `json:"client,omitempty"`
}

// Who is asking for a token, for the Agent activity log only.
var (
	channelSourceMu sync.Mutex
	channelSource   func() string
	scriptRunning   atomic.Bool
)

// SetChannelSource supplies the process channel (cli, mcp, or ide). The CLI
// registers buddy.ActiveChannel so an IDE session is visible on the next token.
func SetChannelSource(fn func() string) {
	channelSourceMu.Lock()
	channelSource = fn
	channelSourceMu.Unlock()
}

// SetScriptRunning marks the three CLI script entry points (a .buddy file,
// -e, or the REPL). MCP and IDE stay their own names while executing buddyscript.
func SetScriptRunning(on bool) {
	scriptRunning.Store(on)
}

func tokenLogClient() string {
	channelSourceMu.Lock()
	fn := channelSource
	channelSourceMu.Unlock()
	ch := ""
	if fn != nil {
		ch = strings.ToLower(strings.TrimSpace(fn()))
	}
	switch ch {
	case "mcp", "ide":
		return ch
	}
	if scriptRunning.Load() {
		return "script"
	}
	return "cli"
}

type response struct {
	OK                   bool            `json:"ok"`
	Present              bool            `json:"present"`
	Unlocked             bool            `json:"unlocked"`
	Version              string          `json:"version"`
	PeerCheck            string          `json:"peer_check"`
	IntegrationCount     int             `json:"integration_count"`
	TokenCount           int             `json:"token_count"`
	Presence             string          `json:"presence"`
	IdleLockSeconds      int             `json:"idle_lock_seconds"`
	UnlockTimeoutSeconds int             `json:"unlock_timeout_seconds"`
	ConfigIntegrity      string          `json:"config_integrity"`
	ManagedPrefs         string          `json:"managed_prefs"`
	Config               json.RawMessage `json:"config"`
	Integrations         []Integration   `json:"integrations"`
	Tokens               []TokenInfo     `json:"tokens"`
	AccessToken          string          `json:"access_token"`
	ExpiresAt            string          `json:"expires_at"`
	Error                string          `json:"error"`
	Message              string          `json:"message"`
}

func orBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func call(ctx context.Context, path string, req request) (response, error) {
	ctx = orBackground(ctx)
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, callTimeout)
		defer cancel()
	}
	dialer := net.Dialer{Timeout: dialTimeout}
	conn, err := dialer.DialContext(ctx, "unix", path)
	if err != nil {
		if absentDial(err) {
			return response{}, ErrAbsent
		}
		return response{}, fmt.Errorf("dial buddy agent: %w", err)
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	line, err := json.Marshal(req)
	if err != nil {
		return response{}, err
	}
	if _, err := conn.Write(append(line, '\n')); err != nil {
		return response{}, fmt.Errorf("write to buddy agent: %w", err)
	}

	reader := bufio.NewReader(conn)
	raw, err := reader.ReadBytes('\n')
	if err != nil && len(raw) == 0 {
		return response{}, fmt.Errorf("read from buddy agent: %w", err)
	}
	var resp response
	if err := json.Unmarshal(raw, &resp); err != nil {
		return response{}, fmt.Errorf("buddy agent returned malformed JSON: %w", err)
	}
	if !resp.OK {
		return resp, agentError(resp)
	}
	return resp, nil
}

func agentError(resp response) error {
	code := strings.TrimSpace(resp.Error)
	msg := strings.TrimSpace(resp.Message)
	switch code {
	case "unsupported":
		if msg != "" {
			return fmt.Errorf("%w: %s", ErrUnsupported, msg)
		}
		return ErrUnsupported
	case "locked":
		if msg == "" {
			msg = "unlock it from the menu bar"
		}
		return fmt.Errorf("buddy agent is locked: %s", msg)
	case "unlock_declined":
		return fmt.Errorf(
			"buddy agent: unlock declined at the presence prompt (set %s=1 to stop buddy asking)",
			EnvNoPrompt)
	case "unlock_timeout":
		if msg == "" {
			msg = "the presence prompt went unanswered"
		}
		return fmt.Errorf("buddy agent: unlock timed out — %s", msg)
	case "unlock_unavailable":
		if msg == "" {
			msg = "unlock it from the menu bar"
		}
		return fmt.Errorf("buddy agent: cannot prompt for unlock — %s", msg)
	case "keychain_restart":
		if msg == "" {
			msg = "run this command again"
		}
		return fmt.Errorf("%w: %s", errKeychainRestart, msg)
	case "denied":
		// Nearly always the signed-Agent / unsigned-buddy mismatch: the Agent
		// checks the caller's Team ID, and a `go build` or `make buddy` binary
		// has no team at all.
		if msg == "" {
			msg = "the agent refused this process"
		}
		return fmt.Errorf(
			"buddy agent refused this buddy: %s\n"+
				"run the signed CLI (/usr/local/bin/buddy, or "+
				"Buddy.app/Contents/Helpers/buddy) — an unsigned build has no Team ID "+
				"for the agent to match. For a dev build, start the agent with "+
				"BUDDY_AGENT_ALLOW_UNSIGNED=1",
			msg)
	case "":
		if msg == "" {
			msg = "request refused"
		}
		return fmt.Errorf("buddy agent: %s", msg)
	}
	if msg == "" {
		return fmt.Errorf("buddy agent: %s", code)
	}
	return fmt.Errorf("buddy agent: %s: %s", code, msg)
}

// absentDial distinguishes "no Agent installed/running" (fall back to the
// vault) from a real transport failure.
func absentDial(err error) bool {
	// A dial timeout is deliberately not "absent": something is bound to the
	// socket, so buddy must not quietly open the vault behind it.
	if errors.Is(err, os.ErrNotExist) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		msg := opErr.Err.Error()
		if strings.Contains(msg, "no such file or directory") ||
			strings.Contains(msg, "connection refused") ||
			strings.Contains(msg, "socket operation on non-socket") {
			return true
		}
	}
	return false
}
