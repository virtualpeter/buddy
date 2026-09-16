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
	"time"
)

const (
	// EnvDisable is the kill switch: never dial the Agent.
	EnvDisable = "BUDDY_NO_AGENT"
	// EnvSocket overrides the socket path (same variable on the Swift side).
	EnvSocket = "BUDDY_AGENT_SOCK"

	// dialTimeout keeps a wedged Agent from stalling every buddy command.
	dialTimeout = 2 * time.Second
	// callTimeout covers the Agent's own round trip to the Jamf token endpoint.
	callTimeout = 35 * time.Second
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
		Present:      true,
		Unlocked:     resp.Unlocked,
		Version:      resp.Version,
		Socket:       path,
		PeerCheck:    resp.PeerCheck,
		Integrations: resp.IntegrationCount,
		Tokens:       resp.TokenCount,
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
	resp, err := call(ctx, path, request{
		Op:      "token",
		Auth:    req.Auth,
		BaseURL: req.BaseURL,
		Kind:    req.Kind,
		Force:   req.Force,
	})
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

type request struct {
	Op      string `json:"op"`
	Auth    string `json:"auth,omitempty"`
	BaseURL string `json:"base_url,omitempty"`
	Kind    string `json:"kind,omitempty"`
	Force   bool   `json:"force,omitempty"`
}

type response struct {
	OK               bool            `json:"ok"`
	Present          bool            `json:"present"`
	Unlocked         bool            `json:"unlocked"`
	Version          string          `json:"version"`
	PeerCheck        string          `json:"peer_check"`
	IntegrationCount int             `json:"integration_count"`
	TokenCount       int             `json:"token_count"`
	Config           json.RawMessage `json:"config"`
	Integrations     []Integration   `json:"integrations"`
	Tokens           []TokenInfo     `json:"tokens"`
	AccessToken      string          `json:"access_token"`
	ExpiresAt        string          `json:"expires_at"`
	Error            string          `json:"error"`
	Message          string          `json:"message"`
}

func call(ctx context.Context, path string, req request) (response, error) {
	if ctx == nil {
		ctx = context.Background()
	}
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
		return ErrUnsupported
	case "locked":
		if msg == "" {
			msg = "unlock it from the menu bar"
		}
		return fmt.Errorf("buddy agent is locked: %s", msg)
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
