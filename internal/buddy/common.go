package buddy

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"buddy/internal/buddy/tokenauth"
)

var logger *slog.Logger

// activeConfig holds the loaded configuration, set once at startup via SetActiveConfig.
var activeConfig *Config

// httpTrace when true logs each Jamf HTTP request and response (headers and bodies) to stderr.
var httpTrace atomic.Bool

// SetHTTPTrace enables or disables HTTP request/response tracing for PerformRequest.
func SetHTTPTrace(on bool) {
	httpTrace.Store(on)
}

const maxHTTPTraceBodyBytes = 65536

// SetActiveConfig stores the loaded configuration for use by auth and URL resolution.
// Must be called from main after LoadConfig.
func SetActiveConfig(cfg *Config) {
	activeConfig = cfg
}

// GetActiveConfig returns the config set by SetActiveConfig (may be nil).
func GetActiveConfig() *Config {
	return activeConfig
}

// InitLogger initializes the global logger with appropriate log level.
func InitLogger() {
	var level slog.Level
	if os.Getenv("DEBUG") != "" {
		level = slog.LevelDebug
	} else {
		level = slog.LevelInfo
	}
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}

// GetLogger returns the global logger instance.
func GetLogger() *slog.Logger {
	if logger == nil {
		InitLogger()
	}
	return logger
}

// SetVerbose upgrades the global logger to slog.LevelDebug when verbose is true.
// Call this after InitLogger once the CLI flags have been parsed.
func SetVerbose(verbose bool) {
	if verbose {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}
}

// resolveURL returns the base URL for the named credential.
// Panics if activeConfig has not been set via SetActiveConfig.
func resolveURL(credName string) (string, error) {
	if activeConfig == nil {
		return "", fmt.Errorf("internal error: config not loaded (SetActiveConfig not called)")
	}
	cred, err := activeConfig.ResolveCredential(credName)
	if err != nil {
		return "", err
	}
	if cred.URL == "" {
		return "", fmt.Errorf("credential %q has no URL configured", credName)
	}
	// Normalize: no trailing slash so path concatenation never produces "//".
	return strings.TrimRight(cred.URL, "/"), nil
}

// resolveAuthHeader returns the Authorization header value for the named credential.
// For command and oauth types this is "Bearer <token>".
// For basic type this is "Basic <base64(user:pass)>".
func resolveAuthHeader(credName string) (string, error) {
	if activeConfig == nil {
		return "", fmt.Errorf("internal error: config not loaded (SetActiveConfig not called)")
	}
	cred, err := activeConfig.ResolveCredential(credName)
	if err != nil {
		return "", err
	}
	switch cred.Auth.Type {
	case "command":
		return resolveCommandToken(credName, cred.Auth)
	case "basic", "oauth":
		return resolveCachedBearerToken(credName, cred)
	case "":
		return "", fmt.Errorf("credential %q has no auth type configured (use 'buddy cred get %s' to inspect)", credName, credName)
	default:
		return "", fmt.Errorf("credential %q has unknown auth type %q (supported: command, basic, oauth)", credName, cred.Auth.Type)
	}
}

// resolveCommandToken runs the configured command and returns the trimmed stdout
// as a Bearer token.
func resolveCommandToken(_ string, auth AuthConfig) (string, error) {
	if auth.RunCommand == "" {
		return "", fmt.Errorf("auth type is 'command' but runCommand is not set")
	}
	parts := strings.Fields(auth.RunCommand)
	if len(parts) == 0 {
		return "", fmt.Errorf("runCommand is empty")
	}
	cmd := exec.Command(parts[0], parts[1:]...) //nolint:gosec
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("auth command %q failed: %w", auth.RunCommand, err)
	}
	token := strings.TrimSpace(string(output))
	if token == "" {
		return "", fmt.Errorf("auth command %q returned empty token", auth.RunCommand)
	}
	return "Bearer " + token, nil
}

var credAuthCache struct {
	mu     sync.Mutex
	byCred map[string]*tokenauth.Authenticator
}

func init() {
	credAuthCache.byCred = make(map[string]*tokenauth.Authenticator)
}

func cachedAuthenticator(credName string, cred CredentialConfig) (*tokenauth.Authenticator, error) {
	credAuthCache.mu.Lock()
	defer credAuthCache.mu.Unlock()
	if a, ok := credAuthCache.byCred[credName]; ok {
		return a, nil
	}
	resolved, err := cred.ResolveAuthWithSecrets(credName)
	if err != nil {
		return nil, err
	}
	baseURL := strings.TrimRight(cred.URL, "/")
	cfg := tokenauth.Config{
		BaseURL: baseURL,
		Logger:  GetLogger(),
	}
	switch resolved.Type {
	case "basic":
		cfg.Username = resolved.Username
		cfg.Password = resolved.Password
	case "oauth":
		cfg.ClientId = resolved.ClientID
		cfg.ClientSecret = resolved.ClientSecret
	default:
		return nil, fmt.Errorf("internal: cachedAuthenticator for auth type %q", resolved.Type)
	}
	a := tokenauth.NewAuthenticator(cfg)
	credAuthCache.byCred[credName] = a
	return a, nil
}

func resolveCachedBearerToken(credName string, cred CredentialConfig) (string, error) {
	a, err := cachedAuthenticator(credName, cred)
	if err != nil {
		return "", err
	}
	token, err := a.GetToken()
	if err != nil {
		return "", err
	}
	return "Bearer " + token, nil
}

func resolveRawToken(credName string) (string, error) {
	header, err := resolveAuthHeader(credName)
	if err != nil {
		return "", err
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", fmt.Errorf("credential %q did not resolve to a bearer token", credName)
	}
	return strings.TrimPrefix(header, prefix), nil
}

// PerformRequest makes an authenticated HTTP request using the credential resolved from credName.
func PerformRequest(_ interface{}, req *http.Request, credName string) (*http.Response, error) {
	authHeader, err := resolveAuthHeader(credName)
	if err != nil {
		return nil, fmt.Errorf("get auth for %q: %w", credName, err)
	}
	req.Header.Set("Authorization", authHeader)
	logger.Debug("API request", "method", req.Method, "url", req.URL.String())

	if !httpTrace.Load() {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("HTTP request failed: %w", err)
		}
		logger.Debug("API response", "status", resp.StatusCode, "url", req.URL.String())
		return resp, nil
	}

	var reqBody []byte
	if req.Body != nil {
		reqBody, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("read request body for trace: %w", err)
		}
		req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(reqBody))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	respBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	resp.Body = io.NopCloser(bytes.NewReader(respBody))

	logger.Debug("API response", "status", resp.StatusCode, "url", req.URL.String())
	writeHTTPTrace(req, reqBody, resp, respBody)
	return resp, nil
}

func redactHeaderValue(name, value string) string {
	switch strings.ToLower(name) {
	case "authorization", "proxy-authorization", "cookie", "set-cookie":
		return "<redacted>"
	default:
		return value
	}
}

func sortedHeaderNames(h http.Header) []string {
	names := make([]string, 0, len(h))
	for k := range h {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

func formatTraceBody(b []byte) string {
	if len(b) == 0 {
		return "(empty)\n"
	}
	if len(b) <= maxHTTPTraceBodyBytes {
		return string(b) + "\n"
	}
	trunc := maxHTTPTraceBodyBytes
	return string(b[:trunc]) + fmt.Sprintf("\n... (truncated %d of %d bytes)\n", len(b)-trunc, len(b))
}

func writeHTTPTrace(req *http.Request, reqBody []byte, resp *http.Response, respBody []byte) {
	var b strings.Builder
	b.WriteString("\n=== HTTP trace ===\n")
	b.WriteString(fmt.Sprintf(">>> %s %s\n", req.Method, req.URL.String()))
	for _, name := range sortedHeaderNames(req.Header) {
		for _, v := range req.Header[name] {
			b.WriteString(fmt.Sprintf(">>> %s: %s\n", name, redactHeaderValue(name, v)))
		}
	}
	if len(reqBody) > 0 {
		b.WriteString(">>> body:\n")
		b.WriteString(formatTraceBody(reqBody))
	}
	b.WriteString(fmt.Sprintf("<<< %s\n", resp.Status))
	for _, name := range sortedHeaderNames(resp.Header) {
		for _, v := range resp.Header[name] {
			b.WriteString(fmt.Sprintf("<<< %s: %s\n", name, redactHeaderValue(name, v)))
		}
	}
	b.WriteString("<<< body:\n")
	b.WriteString(formatTraceBody(respBody))
	b.WriteString("=== end HTTP trace ===\n\n")
	fmt.Fprint(os.Stderr, b.String())
}

// jamfGET performs a GET request to the Jamf API and returns the response body.
func jamfGET(credName, path string) ([]byte, error) {
	return jamfGETWithAccept(credName, path, "application/json, application/xml")
}

// jamfGETWithAccept performs a GET request with a specific Accept header.
func jamfGETWithAccept(credName, path, accept string) ([]byte, error) {
	baseURL, err := resolveURL(credName)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", accept)
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s %s status %d: %s", http.MethodGet, path, resp.StatusCode, string(body))
	}
	return body, nil
}

// jamfPUT performs a PUT request with the given body and content type.
func jamfPUT(credName, path string, body []byte, contentType string) error {
	baseURL, err := resolveURL(credName)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", contentType)
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s status %d: %s", http.MethodPut, path, resp.StatusCode, string(respBody))
	}
	return nil
}

// jamfPOST performs a POST request with the given body and content type.
func jamfPOST(credName, path string, body []byte, contentType string) ([]byte, error) {
	baseURL, err := resolveURL(credName)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json, application/xml")
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s %s status %d: %s", http.MethodPost, path, resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

// jamfDELETE performs a DELETE request.
func jamfDELETE(credName, path string) error {
	baseURL, err := resolveURL(credName)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodDelete, baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s status %d: %s", http.MethodDelete, path, resp.StatusCode, string(body))
	}
	return nil
}

// GetBaseURL is retained as a package-level helper for callers that need the URL
// without making a full HTTP request. Prefer resolveURL internally.
func GetBaseURL(credName string) (string, error) {
	return resolveURL(credName)
}
