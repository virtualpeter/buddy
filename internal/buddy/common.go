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
	"buddy/internal/redact"
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
	logger = slog.New(redact.WrapHandler(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
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
		logger = slog.New(redact.WrapHandler(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
	}
}

// resolveURL returns the base URL for the named auth pin (or active session).
func resolveURL(credName string) (string, error) {
	return resolveURLOp(credName, OpAny)
}

func resolveURLOp(credName string, op OpClass) (string, error) {
	sess, err := ResolveSessionForCred(credName, op)
	if err != nil {
		return "", err
	}
	base := sess.Credential.EffectiveBaseURL()
	if base == "" {
		return "", fmt.Errorf("tenant %q / auth %q has no URL or region configured", sess.TenantName, sess.AuthName)
	}
	return base, nil
}

// resolveAuthHeader returns the Authorization header for the named auth pin (or active session).
func resolveAuthHeader(credName string) (string, error) {
	return resolveAuthHeaderOp(credName, OpAny)
}

func resolveAuthHeaderOp(credName string, op OpClass) (string, error) {
	sess, err := ResolveSessionForCred(credName, op)
	if err != nil {
		return "", err
	}
	cred := sess.Credential
	cacheName := sessionCacheName(sess)
	switch cred.Auth.Type {
	case "command":
		return resolveCommandToken(sess.AuthName, cred.Auth)
	case "basic", "oauth":
		return resolveCachedBearerToken(cacheName, sess.AuthName, cred)
	case "":
		return "", fmt.Errorf("auth %q has no type configured (use 'buddy cred get auth %s' to inspect)", sess.AuthName, sess.AuthName)
	default:
		return "", fmt.Errorf("auth %q has unknown type %q (supported: command, basic, oauth)", sess.AuthName, cred.Auth.Type)
	}
}

func sessionCacheName(sess *Session) string {
	if sess == nil {
		return ""
	}
	name := sess.AuthName + "@" + sess.TenantName
	if sess.Surface == SurfacePlatform {
		tid := sess.Credential.EffectiveTenantID()
		if tid != "" {
			return name + "@platform:" + tid
		}
		return name + "@platform"
	}
	return name
}

func sessionDiskCacheKey(sess *Session) string {
	if sess == nil {
		return ""
	}
	return sessionDiskCacheKeyFromParts(sessionCacheName(sess), sess.Credential)
}

func sessionDiskCacheKeyFromParts(cacheName string, cred CredentialConfig) string {
	if cred.IsPlatform() && !strings.Contains(cacheName, "@platform:") {
		return cacheName + "@platform:" + cred.EffectiveTenantID()
	}
	return cacheName
}

func evictAuthenticator(cacheName string) {
	credAuthCache.mu.Lock()
	defer credAuthCache.mu.Unlock()
	if a, ok := credAuthCache.byCred[cacheName]; ok {
		a.ClearMemoryToken()
		delete(credAuthCache.byCred, cacheName)
	}
}

func evictAllAuthenticators() {
	credAuthCache.mu.Lock()
	defer credAuthCache.mu.Unlock()
	for _, a := range credAuthCache.byCred {
		a.ClearMemoryToken()
	}
	credAuthCache.byCred = make(map[string]*tokenauth.Authenticator)
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

// cachedAuthenticator builds or returns a cached Authenticator.
// cacheName is the process-cache key (auth@tenant, or auth@tenant@platform:{id} on platform);
// secretName is the secret-store key (auth name).
// Vault secrets are loaded lazily only when a token must be minted or refreshed.
func cachedAuthenticator(cacheName, secretName string, cred CredentialConfig) (*tokenauth.Authenticator, error) {
	credAuthCache.mu.Lock()
	defer credAuthCache.mu.Unlock()
	if a, ok := credAuthCache.byCred[cacheName]; ok {
		return a, nil
	}
	if err := cred.Validate(); err != nil {
		return nil, fmt.Errorf("auth %q: %w", secretName, err)
	}
	baseURL := cred.EffectiveBaseURL()
	if baseURL == "" {
		return nil, fmt.Errorf("auth %q has no URL or region configured", secretName)
	}
	diskKey := sessionDiskCacheKeyFromParts(cacheName, cred)
	auth := cred.Auth
	cfg := tokenauth.Config{
		Name:     secretName,
		BaseURL:  baseURL,
		CacheKey: diskKey,
		Platform: cred.UsesPlatformAuth(),
		Logger:   GetLogger(),
	}
	switch auth.Type {
	case "basic":
		cfg.Username = auth.Username
		cfg.Password = auth.Password // inline only; vault via LoadSecrets
		if strings.TrimSpace(cfg.Password) == "" {
			name := secretName
			cc := cred
			cfg.LoadSecrets = func() (string, string, error) {
				resolved, err := cc.ResolveAuthWithSecrets(name)
				if err != nil {
					return "", "", err
				}
				return resolved.Password, "", nil
			}
		}
	case "oauth":
		cfg.ClientId = auth.ClientID
		cfg.ClientSecret = auth.ClientSecret // inline only; vault via LoadSecrets
		if strings.TrimSpace(cfg.ClientSecret) == "" {
			name := secretName
			cc := cred
			cfg.LoadSecrets = func() (string, string, error) {
				resolved, err := cc.ResolveAuthWithSecrets(name)
				if err != nil {
					return "", "", err
				}
				return "", resolved.ClientSecret, nil
			}
		}
	default:
		return nil, fmt.Errorf("internal: cachedAuthenticator for auth type %q", auth.Type)
	}
	a := tokenauth.NewAuthenticator(cfg)
	credAuthCache.byCred[cacheName] = a
	return a, nil
}

func resolveCachedBearerToken(cacheName, secretName string, cred CredentialConfig) (string, error) {
	a, err := cachedAuthenticator(cacheName, secretName, cred)
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
// Auth/surface are chosen from the request path (same rules as JamfRequest).
func PerformRequest(_ interface{}, req *http.Request, credName string) (*http.Response, error) {
	path := ""
	method := http.MethodGet
	if req != nil {
		if req.URL != nil {
			path = req.URL.Path
		}
		if req.Method != "" {
			method = req.Method
		}
	}
	return PerformRequestOp(req, credName, opClassForPath(method, path))
}

// PerformRequestOp is like PerformRequest with an operation class for auth picking.
// On HTTP 401, refreshes the token once (OAuth: client credentials; basic: keep-alive or re-auth) and retries.
func PerformRequestOp(req *http.Request, credName string, op OpClass) (*http.Response, error) {
	bodyBytes, err := bufferRequestBody(req)
	if err != nil {
		return nil, err
	}
	if req != nil && req.URL != nil {
		if ResourceClassPreference() == "" {
			restoreClass := setResourceClass(resourceClassForPath(req.URL.Path))
			defer restoreClass()
		}
	}

	authHeader, err := resolveAuthHeaderOp(credName, op)
	if err != nil {
		return nil, fmt.Errorf("get auth for %q: %w", credName, err)
	}
	req.Header.Set("Authorization", authHeader)
	resp, err := performHTTP(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}

	sess, sessErr := ResolveSessionForCred(credName, op)
	if sessErr != nil {
		return resp, nil
	}
	a, authErr := cachedAuthenticator(sessionCacheName(sess), sess.AuthName, sess.Credential)
	if authErr != nil {
		return resp, nil
	}
	// Basic: if keep-alive probe still succeeds, this is privilege-denied — do not retry.
	if a.ProbeTokenStillValid() {
		return resp, nil
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if err := a.RefreshToken(); err != nil {
		invalidateCapsForSession(sess)
		return nil, fmt.Errorf("refresh token after 401: %w", err)
	}
	authHeader, err = resolveAuthHeaderOp(credName, op)
	if err != nil {
		return nil, fmt.Errorf("get auth for %q after refresh: %w", credName, err)
	}
	req.Header.Set("Authorization", authHeader)
	restoreRequestBody(req, bodyBytes)
	return performHTTP(req)
}

func bufferRequestBody(req *http.Request) ([]byte, error) {
	if req == nil || req.Body == nil {
		return nil, nil
	}
	bodyBytes, err := io.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("read request body: %w", err)
	}
	restoreRequestBody(req, bodyBytes)
	return bodyBytes, nil
}

func restoreRequestBody(req *http.Request, bodyBytes []byte) {
	if req == nil {
		return
	}
	if bodyBytes == nil {
		req.Body = nil
		return
	}
	req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	req.ContentLength = int64(len(bodyBytes))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(bodyBytes)), nil
	}
}

// PerformRequestNoAuth makes an HTTP request without an Authorization header.
// Use it for endpoints served publicly by the Jamf host (e.g. the Pro/Classic
// schema documents) so callers only need the base URL — schema discovery then
// works even when a credential has no stored secret.
func PerformRequestNoAuth(req *http.Request) (*http.Response, error) {
	return performHTTP(req)
}

// performHTTP executes req (honoring the HTTP trace toggle) without touching auth.
func performHTTP(req *http.Request) (*http.Response, error) {
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
		var err error
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
		return redact.Placeholder
	default:
		return redact.String(value)
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
		return redact.String(string(b)) + "\n"
	}
	trunc := maxHTTPTraceBodyBytes
	return redact.String(string(b[:trunc])) + fmt.Sprintf("\n... (truncated %d of %d bytes)\n", len(b)-trunc, len(b))
}

func writeHTTPTrace(req *http.Request, reqBody []byte, resp *http.Response, respBody []byte) {
	var b strings.Builder
	b.WriteString("\n=== HTTP trace ===\n")
	b.WriteString(fmt.Sprintf(">>> %s %s\n", req.Method, redact.String(req.URL.String())))
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
	fmt.Fprint(os.Stderr, redact.String(b.String()))
}

func readHTTPBody(resp *http.Response) ([]byte, error) {
	if resp == nil || resp.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	return body, nil
}

func httpErrorBody(resp *http.Response) string {
	body, err := readHTTPBody(resp)
	if err != nil {
		return fmt.Sprintf("<%v>", err)
	}
	return string(body)
}

// jamfGET performs a GET request to the Jamf API and returns the response body.
func jamfGET(credName, path string) ([]byte, error) {
	return jamfGETWithAccept(credName, path, "application/json, application/xml")
}

// jamfGETWithAccept performs a GET request with a specific Accept header.
func jamfGETWithAccept(credName, path, accept string) ([]byte, error) {
	endpoint, err := ResolveEndpoint(credName, path)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
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
func jamfPUT(credName, path string, body []byte, contentType string) ([]byte, error) {
	endpoint, err := ResolveEndpoint(credName, path)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPut, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", contentType)
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
		return nil, fmt.Errorf("%s %s status %d: %s", http.MethodPut, path, resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

// jamfErrorHasStatus reports whether err is a jamfGET/PUT/POST/PATCH/DELETE status error.
func jamfErrorHasStatus(err error, status int) bool {
	if err == nil || status < 100 {
		return false
	}
	return strings.Contains(err.Error(), fmt.Sprintf(" status %d:", status))
}

// jamfPOST performs a POST request with the given body and content type.
func jamfPOST(credName, path string, body []byte, contentType string) ([]byte, error) {
	endpoint, err := ResolveEndpoint(credName, path)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
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

// jamfPATCH performs a PATCH request with the given body and content type.
func jamfPATCH(credName, path string, body []byte, contentType string) ([]byte, error) {
	endpoint, err := ResolveEndpoint(credName, path)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPatch, endpoint, bytes.NewReader(body))
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
		return nil, fmt.Errorf("%s %s status %d: %s", http.MethodPatch, path, resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

// jamfDELETE performs a DELETE request.
func jamfDELETE(credName, path string) error {
	endpoint, err := ResolveEndpoint(credName, path)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodDelete, endpoint, nil)
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
