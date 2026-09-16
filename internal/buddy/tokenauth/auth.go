package tokenauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"log/slog"

	"buddy/internal/buddy/agent"
	"buddy/internal/redact"
)

// Authenticator handles authentication for Jamf Pro API
type Authenticator struct {
	name         string // credential name, used only for diagnostics
	baseURL      string
	username     string
	password     string
	tokenCommand []string
	clientId     string
	clientSecret string
	platform     bool   // Platform API Gateway OAuth
	cacheKey     string // when set, persist token+expiry under ~/.config/buddy/tokens/
	loadSecrets  func() (password, clientSecret string, err error)
	token        string
	tokenExpiry  time.Time
	httpClient   *http.Client
	logger       *slog.Logger
	// forceAgentMint makes the next Buddy Agent call skip the Agent's own
	// token cache (set by ForceRenew, which must prove the secret).
	forceAgentMint bool
	// agentOwned means the Agent holds this credential; there is no vault copy.
	agentOwned bool
}

// Config holds authentication configuration
type Config struct {
	// Name is the credential name, used only to make auth errors actionable.
	Name         string
	BaseURL      string
	Username     string
	Password     string
	TokenCommand []string
	ClientId     string
	ClientSecret string
	// Platform enables Platform API Gateway OAuth (token at /auth/token, no keep-alive).
	Platform bool
	// CacheKey enables on-disk token cache (typically the credential name). Empty disables disk cache.
	CacheKey string
	// LoadSecrets fills Password / ClientSecret from the vault when a token must
	// be minted or refreshed. Not called when a usable disk/memory token exists.
	LoadSecrets func() (password, clientSecret string, err error)
	// AgentOwned marks an auth the macOS Buddy Agent owns (features.use_agent).
	// buddy holds no secret for it, so an Agent that cannot serve it is fatal
	// rather than a fallback to the vault.
	AgentOwned bool
	Logger     *slog.Logger
}

// TokenResponse represents the standard token response from Jamf Pro
type TokenResponse struct {
	Token   string `json:"token"`
	Expires string `json:"expires"`
}

// OAuth2TokenResponse represents the OAuth2 token response from Jamf Pro
type OAuth2TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"` // Expires in seconds
	Scope       string `json:"scope,omitempty"`
}

// NewAuthenticator creates a new Authenticator with the given configuration
func NewAuthenticator(cfg Config) *Authenticator {
	httpClient := &http.Client{Timeout: 30 * time.Second}
	if cfg.Logger == nil {
		// Create a minimal logger if none provided
		cfg.Logger = slog.Default()
	}
	return &Authenticator{
		name:         cfg.Name,
		baseURL:      cfg.BaseURL,
		username:     cfg.Username,
		password:     cfg.Password,
		tokenCommand: cfg.TokenCommand,
		clientId:     cfg.ClientId,
		clientSecret: cfg.ClientSecret,
		platform:     cfg.Platform,
		cacheKey:     cfg.CacheKey,
		loadSecrets:  cfg.LoadSecrets,
		agentOwned:   cfg.AgentOwned,
		httpClient:   httpClient,
		logger:       redact.Logger(cfg.Logger),
	}
}

func (a *Authenticator) debugf(format string, args ...any) {
	if a.logger != nil {
		a.logger.Debug(redact.String(fmt.Sprintf(format, args...)))
	}
}

func (a *Authenticator) verbosef(format string, args ...any) {
	if a.logger != nil {
		a.logger.Debug(redact.String(fmt.Sprintf(format, args...)))
	}
}

// GetToken returns a valid bearer token, refreshing if necessary
func (a *Authenticator) GetToken() (string, error) {
	if err := a.ensureValidToken(); err != nil {
		return "", err
	}
	return a.token, nil
}

// ForceRenew discards any cached token (memory and disk) and obtains a new one.
func (a *Authenticator) ForceRenew() error {
	a.clearMemoryToken()
	a.InvalidateDiskCache()
	a.forceAgentMint = true
	return a.ensureValidToken()
}

// TokenExpiry returns the current in-memory token expiry, or zero if none is held.
func (a *Authenticator) TokenExpiry() time.Time {
	return a.tokenExpiry
}

// ClearMemoryToken drops the in-memory bearer without touching the disk cache.
func (a *Authenticator) ClearMemoryToken() {
	a.clearMemoryToken()
}

const (
	TokenActionObtain  = "obtain"
	TokenActionRefresh = "refresh"
	TokenActionReplace = "replace"
)

// TokenRefreshResult is the outcome of RefreshSession.
type TokenRefreshResult struct {
	Action string
	Token  string
	Expiry time.Time
}

// RefreshSession hydrates the disk cache, then obtain / keep-alive / replace
// depending on auth type. Used by buddy auth token refresh.
func (a *Authenticator) RefreshSession() (TokenRefreshResult, error) {
	if a.token == "" {
		_ = a.loadFromDiskIfValid()
	}
	hadToken := a.token != ""

	if len(a.tokenCommand) > 0 {
		a.clearMemoryToken()
		if err := a.getTokenFromCommand(); err != nil {
			return TokenRefreshResult{}, err
		}
		return TokenRefreshResult{Action: TokenActionReplace, Token: a.token, Expiry: a.tokenExpiry}, nil
	}

	// A Buddy Agent credential is replaced by the Agent, past its own cache.
	a.forceAgentMint = true
	if minted, err := a.agentToken(); err != nil {
		return TokenRefreshResult{}, err
	} else if minted {
		return TokenRefreshResult{Action: TokenActionReplace, Token: a.token, Expiry: a.tokenExpiry}, nil
	}

	if a.clientId != "" {
		if err := a.loadSecretsIfNeeded(); err != nil {
			return TokenRefreshResult{}, err
		}
		a.clearMemoryToken()
		a.InvalidateDiskCache()
		if err := a.getTokenFromOAuth2(); err != nil {
			return TokenRefreshResult{}, err
		}
		return TokenRefreshResult{Action: TokenActionReplace, Token: a.token, Expiry: a.tokenExpiry}, nil
	}

	if hadToken {
		if err := a.refreshTokenWithKeepAlive(); err == nil {
			return TokenRefreshResult{Action: TokenActionRefresh, Token: a.token, Expiry: a.tokenExpiry}, nil
		} else {
			a.verbosef("Keep-alive failed, obtaining new token: %v", err)
		}
	}

	if err := a.loadSecretsIfNeeded(); err != nil {
		return TokenRefreshResult{}, err
	}
	a.clearMemoryToken()
	a.InvalidateDiskCache()
	if err := a.getTokenFromCredentials(); err != nil {
		return TokenRefreshResult{}, err
	}
	return TokenRefreshResult{Action: TokenActionObtain, Token: a.token, Expiry: a.tokenExpiry}, nil
}

// setToken stores token+expiry in memory and persists to disk when CacheKey is set.
func (a *Authenticator) setToken(token string, expiry time.Time) {
	a.token = token
	a.tokenExpiry = expiry
	if a.cacheKey == "" || token == "" || len(a.tokenCommand) > 0 {
		return
	}
	if err := saveDiskToken(a.cacheKey, a.baseURL, token, expiry); err != nil {
		a.debugf("token disk cache save failed: %v", err)
	} else {
		a.debugf("saved token to disk cache key=%q (expires %v)", a.cacheKey, expiry)
	}
}

// InvalidateDiskCache removes the on-disk token for this authenticator's CacheKey.
func (a *Authenticator) InvalidateDiskCache() {
	if a.cacheKey == "" {
		return
	}
	removeDiskToken(a.cacheKey)
}

func (a *Authenticator) loadFromDiskIfValid() bool {
	if a.cacheKey == "" || len(a.tokenCommand) > 0 {
		return false
	}
	if a.agentOwned {
		// The Agent owns this credential, so buddy must ask it every time: a
		// cached file would keep working after the Agent is locked. Sweep any
		// left by an earlier release.
		a.InvalidateDiskCache()
		return false
	}
	tok, exp, ok := loadDiskToken(a.cacheKey, a.baseURL, clockSkewWindow)
	if !ok {
		return false
	}
	a.token = tok
	a.tokenExpiry = exp
	a.debugf("loaded token from disk cache key=%q (expires %v)", a.cacheKey, exp)
	return true
}

func (a *Authenticator) clearMemoryToken() {
	a.token = ""
	a.tokenExpiry = time.Time{}
}

func (a *Authenticator) memoryTokenUsable() bool {
	return a.token != "" && tokenUsable(a.tokenExpiry, clockSkewWindow)
}

// ensureValidToken ensures we have a valid token, refreshing if necessary
func (a *Authenticator) ensureValidToken() error {
	// If using external token command, get token from that (no disk cache)
	if len(a.tokenCommand) > 0 {
		return a.ensureTokenCommand()
	}

	// Hydrate from disk before deciding to hit Jamf (or the vault)
	if a.token == "" {
		_ = a.loadFromDiskIfValid()
	}

	// OAuth is keyed by client id even when the secret is loaded lazily.
	if a.clientId != "" {
		return a.ensureOAuth2Token()
	}

	// Username/password auth
	return a.ensureBasicAuthToken()
}

// ensureTokenCommand ensures a valid token for token command authentication
func (a *Authenticator) ensureTokenCommand() error {
	if a.memoryTokenUsable() {
		return nil
	}
	return a.getTokenFromCommand()
}

// ensureOAuth2Token ensures a valid OAuth2 token.
// Reuses the token until expiry (minus clock skew); refresh is reactive on 401
// or when the token is already unusable. No proactive client-credentials re-auth.
func (a *Authenticator) ensureOAuth2Token() error {
	if a.memoryTokenUsable() {
		return nil
	}
	if minted, err := a.agentToken(); err != nil {
		return err
	} else if minted {
		return nil
	}
	if err := a.loadSecretsIfNeeded(); err != nil {
		return err
	}
	if a.token != "" {
		a.debugf("OAuth2 token expired or within skew window; requesting new token")
		a.InvalidateDiskCache()
		a.clearMemoryToken()
	}
	return a.getTokenFromOAuth2()
}

// ensureBasicAuthToken ensures a valid token for username/password authentication.
// Reuses until expiry (minus skew); keep-alive is only used from RefreshToken (401 / auth token refresh).
func (a *Authenticator) ensureBasicAuthToken() error {
	if a.memoryTokenUsable() {
		return nil
	}
	if minted, err := a.agentToken(); err != nil {
		return err
	} else if minted {
		return nil
	}
	if err := a.loadSecretsIfNeeded(); err != nil {
		return err
	}
	if a.token != "" {
		a.InvalidateDiskCache()
		a.clearMemoryToken()
	}
	return a.getTokenFromCredentials()
}

// agentToken asks the optional macOS Buddy Agent to mint the bearer, so the
// vault DEK never enters this process. Reports whether a token was set.
//
// Absent Agent (or one that cannot serve this auth) falls through to the vault.
// Any other Agent error is fatal: a running Agent owning the credential must
// not be bypassed by quietly unlocking the local vault instead. For an
// Agent-owned auth there is no vault copy at all, so absent and unsupported are
// fatal too.
func (a *Authenticator) agentToken() (bool, error) {
	force := a.forceAgentMint
	a.forceAgentMint = false
	if len(a.tokenCommand) > 0 || strings.TrimSpace(a.name) == "" || !agent.Enabled() {
		if a.agentOwned {
			return false, fmt.Errorf("auth %q is owned by the Buddy Agent, but the Agent is disabled (%s)", a.name, agent.EnvDisable)
		}
		return false, nil
	}
	kind := "basic"
	switch {
	case a.clientId != "" && a.platform:
		kind = "platform"
	case a.clientId != "":
		kind = "oauth"
	}
	// An inline secret is already in memory, so the Agent buys nothing.
	if (kind == "basic" && a.password != "") || (kind != "basic" && a.clientSecret != "") {
		return false, nil
	}
	tok, err := agent.Token(context.Background(), agent.TokenRequest{
		Auth:    a.name,
		BaseURL: a.baseURL,
		Kind:    kind,
		Force:   force,
	})
	switch {
	case errors.Is(err, agent.ErrAbsent), errors.Is(err, agent.ErrUnsupported):
		if a.agentOwned {
			return false, fmt.Errorf("auth %q is owned by the Buddy Agent and buddy holds no secret for it: %w", a.name, err)
		}
		a.debugf("buddy agent not used (%v); falling back to the local vault", err)
		return false, nil
	case err != nil:
		return false, err
	}
	// Memory only, and drop any file left by an earlier vault mint: the Agent
	// is the one holding this bearer, so locking it must stop the next process
	// rather than leave buddy a usable copy on disk.
	a.token = tok.AccessToken
	a.tokenExpiry = tok.ExpiresAt
	a.InvalidateDiskCache()
	a.debugf("bearer minted by buddy agent (expires %v; not cached on disk)", tok.ExpiresAt)
	return true, nil
}

// loadSecretsIfNeeded calls LoadSecrets when password/client secret are still empty.
func (a *Authenticator) loadSecretsIfNeeded() error {
	if a == nil || a.loadSecrets == nil {
		return nil
	}
	needPassword := a.clientId == "" && a.username != "" && a.password == ""
	needClientSecret := a.clientId != "" && a.clientSecret == ""
	if !needPassword && !needClientSecret {
		return nil
	}
	pw, cs, err := a.loadSecrets()
	if err != nil {
		return err
	}
	if needPassword && strings.TrimSpace(pw) != "" {
		a.password = pw
	}
	if needClientSecret && strings.TrimSpace(cs) != "" {
		a.clientSecret = cs
	}
	return nil
}

// getTokenFromCommand obtains a token by executing an external command
func (a *Authenticator) getTokenFromCommand() error {
	if len(a.tokenCommand) == 0 {
		return fmt.Errorf("token_command is empty")
	}

	a.verbosef("Executing token command: %v", a.tokenCommand)
	cmd := exec.Command(a.tokenCommand[0], a.tokenCommand[1:]...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("token command failed: %s", string(exitErr.Stderr))
		}
		return fmt.Errorf("failed to execute token command: %w", err)
	}

	token := strings.TrimSpace(string(output))
	if token == "" {
		return fmt.Errorf("token command returned empty token")
	}

	a.token = token
	// Set expiry to 18 minutes (conservative estimate)
	a.tokenExpiry = time.Now().Add(18 * time.Minute)
	a.verbosef("Token obtained successfully, expires at %v", a.tokenExpiry)

	return nil
}

// getTokenFromCredentials obtains a token using username/password basic auth
// authEndpointError builds an actionable error for a 400/401/403 returned by a
// token endpoint, which almost always means the stored secret is wrong, revoked,
// or expired rather than a transient failure. The raw body is not logged.
func (a *Authenticator) authEndpointError(status int, secretKind string, body []byte) error {
	a.debugf("token endpoint returned %d", status)
	who := ""
	action := "verify the stored credentials and the integration/user in Jamf"
	if a.name != "" {
		who = fmt.Sprintf(" for credential %q", a.name)
		action = fmt.Sprintf("re-store it with 'buddy cred set secret %s' and verify the integration/user in Jamf", a.name)
	}
	return fmt.Errorf("authentication failed at the token endpoint (status %d)%s: the %s is likely wrong, revoked, or expired — %s", status, who, secretKind, action)
}

func isAuthRejection(status int) bool {
	return status == http.StatusUnauthorized || status == http.StatusForbidden || status == http.StatusBadRequest
}

func (a *Authenticator) getTokenFromCredentials() error {
	if strings.TrimSpace(a.password) == "" {
		who := "credential"
		if a.name != "" {
			who = fmt.Sprintf("credential %q", a.name)
		}
		return fmt.Errorf("password missing for %s — run 'buddy cred set secret'", who)
	}
	url := fmt.Sprintf("%s/api/v1/auth/token", a.baseURL)
	a.verbosef("Requesting authentication token from %s", url)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create token request: %w", err)
	}

	req.SetBasicAuth(a.username, a.password)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to request token: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		if isAuthRejection(resp.StatusCode) {
			return a.authEndpointError(resp.StatusCode, "password", body)
		}
		a.debugf("Token request failed with status %d", resp.StatusCode)
		return fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, redact.String(string(body)))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return fmt.Errorf("failed to decode token response: %w", err)
	}

	a.setToken(tokenResp.Token, time.Now().Add(18*time.Minute))
	a.verbosef("Token obtained successfully, expires at %v", a.tokenExpiry)

	return nil
}

// getTokenFromOAuth2 obtains a bearer token using OAuth2 client credentials flow
func (a *Authenticator) getTokenFromOAuth2() error {
	if strings.TrimSpace(a.clientSecret) == "" {
		who := "credential"
		if a.name != "" {
			who = fmt.Sprintf("credential %q", a.name)
		}
		return fmt.Errorf("client secret missing for %s — run 'buddy cred set secret'", who)
	}
	if a.platform {
		return a.getTokenFromPlatformOAuth2()
	}
	tokenURL := fmt.Sprintf("%s/api/oauth/token", a.baseURL)
	a.verbosef("Requesting OAuth2 token from %s", tokenURL)

	// Prepare form data with proper URL encoding
	formData := url.Values{}
	formData.Set("grant_type", "client_credentials")
	formData.Set("client_id", a.clientId)
	formData.Set("client_secret", a.clientSecret)

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create OAuth2 token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to request OAuth2 token: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		if isAuthRejection(resp.StatusCode) {
			return a.authEndpointError(resp.StatusCode, "client secret", body)
		}
		a.debugf("OAuth2 token request failed with status %d", resp.StatusCode)
		return fmt.Errorf("OAuth2 token request failed with status %d: %s", resp.StatusCode, redact.String(string(body)))
	}

	return a.applyOAuth2TokenResponse(body)
}

// getTokenFromPlatformOAuth2 requests a token from the Platform API Gateway.
// Primary endpoint is /auth/token (jamf-cli / jamfplatform-go-sdk); falls back to /oauth2/token on 404.
func (a *Authenticator) getTokenFromPlatformOAuth2() error {
	formData := url.Values{}
	formData.Set("grant_type", "client_credentials")
	formData.Set("client_id", a.clientId)
	formData.Set("client_secret", a.clientSecret)
	encoded := formData.Encode()

	tryURL := func(tokenURL string) (body []byte, status int, err error) {
		a.verbosef("Requesting Platform OAuth2 token from %s", tokenURL)
		req, err := http.NewRequest("POST", tokenURL, strings.NewReader(encoded))
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create Platform OAuth2 token request: %w", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := a.httpClient.Do(req)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to request Platform OAuth2 token: %w", err)
		}
		defer resp.Body.Close()
		body, _ = io.ReadAll(resp.Body)
		return body, resp.StatusCode, nil
	}

	base := strings.TrimRight(a.baseURL, "/")
	body, status, err := tryURL(base + "/auth/token")
	if err != nil {
		return err
	}
	if status == http.StatusNotFound {
		body, status, err = tryURL(base + "/oauth2/token")
		if err != nil {
			return err
		}
	}
	if status != http.StatusOK {
		if isAuthRejection(status) {
			return a.authEndpointError(status, "client secret", body)
		}
		a.debugf("Platform OAuth2 token request failed with status %d", status)
		return fmt.Errorf("Platform OAuth2 token request failed with status %d: %s", status, redact.String(string(body)))
	}
	return a.applyOAuth2TokenResponse(body)
}

func (a *Authenticator) applyOAuth2TokenResponse(body []byte) error {
	var oauthResp OAuth2TokenResponse
	if err := json.Unmarshal(body, &oauthResp); err != nil {
		return fmt.Errorf("failed to decode OAuth2 token response: %w", err)
	}

	if oauthResp.AccessToken == "" {
		return fmt.Errorf("OAuth2 response missing access_token")
	}

	a.debugf("OAuth2 token obtained (length %d)", len(oauthResp.AccessToken))

	// Calculate expiry from expires_in (typically 1800 seconds = 30 minutes; Platform ~900)
	expiresIn := time.Duration(oauthResp.ExpiresIn) * time.Second
	if expiresIn == 0 {
		// Default to 30 minutes if not specified; Platform tokens are often 900s
		if a.platform {
			expiresIn = 15 * time.Minute
		} else {
			expiresIn = 30 * time.Minute
		}
	}
	a.setToken(oauthResp.AccessToken, time.Now().Add(expiresIn))
	actualLifetimeSeconds := int(expiresIn.Seconds())
	a.verbosef("OAuth2 token obtained successfully, expires in %d seconds", actualLifetimeSeconds)
	a.debugf("OAuth2 token expiry set to: %v (current time: %v)", a.tokenExpiry, time.Now())

	return nil
}

// ProbeTokenStillValid checks whether the current token is still accepted by the server (e.g. keep-alive returns 200).
// Used to distinguish 401 "permission denied" from "token expired". For basic auth, probes /api/v1/auth/keep-alive.
// For token_command or OAuth2, returns false so the client will refresh and retry once.
// This is a read-only probe and does not change stored token state.
func (a *Authenticator) ProbeTokenStillValid() bool {
	if len(a.tokenCommand) > 0 || (a.clientId != "" && a.clientSecret != "") {
		return false
	}
	if a.token == "" {
		return false
	}
	url := fmt.Sprintf("%s/api/v1/auth/keep-alive", a.baseURL)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		a.debugf("ProbeTokenStillValid: failed to create request: %v", err)
		return false
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.token))
	resp, err := a.httpClient.Do(req)
	if err != nil {
		a.debugf("ProbeTokenStillValid: request failed: %v", err)
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode == http.StatusOK
}

// refreshTokenWithKeepAlive extends the current token using the keep-alive endpoint
func (a *Authenticator) refreshTokenWithKeepAlive() error {
	if a.token == "" {
		return fmt.Errorf("cannot refresh token: no token available")
	}

	a.verbosef("Refreshing authentication token using keep-alive")
	url := fmt.Sprintf("%s/api/v1/auth/keep-alive", a.baseURL)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create keep-alive request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.token))

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to refresh token: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		a.debugf("Token refresh failed with status %d", resp.StatusCode)
		return fmt.Errorf("token refresh failed with status %d: %s", resp.StatusCode, redact.String(string(body)))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return fmt.Errorf("failed to decode token response: %w", err)
	}

	a.debugf("keep-alive token obtained (length %d)", len(tokenResp.Token))
	a.setToken(tokenResp.Token, time.Now().Add(18*time.Minute))
	a.verbosef("Token refreshed successfully, expires at %v", a.tokenExpiry)

	return nil
}

// RefreshToken handles token refresh reactively on HTTP 401 (keep-alive or re-auth).
func (a *Authenticator) RefreshToken() error {
	a.verbosef("Refreshing authentication token")
	// If using external token command, just get a new token from the command
	if len(a.tokenCommand) > 0 {
		// Clear current token to force refresh
		a.clearMemoryToken()
		return a.getTokenFromCommand()
	}

	// The Agent minted the stale bearer, so it mints the replacement: going to
	// the vault here would ask for a secret buddy may not even hold. Force it
	// past the Agent's own cache, which is what returned the stale one.
	a.forceAgentMint = true
	if minted, err := a.agentToken(); err != nil {
		return err
	} else if minted {
		return nil
	}

	// If using OAuth2, get a new token (OAuth2 client credentials doesn't support keep-alive)
	if a.clientId != "" {
		if err := a.loadSecretsIfNeeded(); err != nil {
			return err
		}
		a.clearMemoryToken()
		a.InvalidateDiskCache()
		return a.getTokenFromOAuth2()
	}

	if err := a.loadSecretsIfNeeded(); err != nil {
		return err
	}

	// If we have a token, try to extend it with keep-alive (for username/password auth)
	if a.token != "" {
		// Try keep-alive first
		if err := a.refreshTokenWithKeepAlive(); err == nil {
			return nil
		} else {
			// If keep-alive failed (e.g., token already expired), fall through to get new token
			a.verbosef("Keep-alive failed, getting new token: %v", err)
		}
	}

	// No token or keep-alive failed, get a new token
	a.clearMemoryToken()
	a.InvalidateDiskCache()
	return a.getTokenFromCredentials()
}

// RefreshTokenValue takes a token string and attempts to refresh it using keep-alive
// Returns the refreshed token, or the original token if refresh fails or is not supported
func (a *Authenticator) RefreshTokenValue(tokenValue string) (string, error) {
	if tokenValue == "" {
		return "", fmt.Errorf("token value is empty")
	}

	a.debugf("RefreshTokenValue called with token length: %d characters", len(tokenValue))

	// OAuth2 tokens don't support keep-alive, return the original token
	if a.clientId != "" && a.clientSecret != "" {
		a.debugf("OAuth2 client credentials detected - keep-alive not supported for OAuth2 tokens")
		a.verbosef("OAuth2 tokens do not support keep-alive, returning original token")
		return tokenValue, nil
	}

	// Token command tokens also don't need keep-alive (they're refreshed via command)
	if len(a.tokenCommand) > 0 {
		a.debugf("Token command mode detected - keep-alive not applicable")
		a.verbosef("Token command mode does not support keep-alive, returning original token")
		return tokenValue, nil
	}

	a.debugf("Using basic auth keep-alive method for token refresh")

	// Set the token temporarily to use keep-alive
	originalToken := a.token
	a.token = tokenValue
	a.debugf("Temporarily set authenticator token for keep-alive request")

	// Attempt to refresh using keep-alive
	a.debugf("Calling refreshTokenWithKeepAlive()")
	err := a.refreshTokenWithKeepAlive()
	refreshedToken := a.token

	// Restore original token state
	a.token = originalToken
	a.debugf("Restored authenticator token state")

	if err != nil {
		// If keep-alive fails, return the original token (it might still be valid)
		a.debugf("Keep-alive request failed: %v", err)
		a.verbosef("Keep-alive failed for provided token: %v", err)
		return tokenValue, nil
	}

	if refreshedToken != tokenValue {
		a.debugf("Token successfully refreshed (token changed from original)")
	} else {
		a.debugf("Token unchanged after keep-alive (token may not have changed)")
	}

	return refreshedToken, nil
}
