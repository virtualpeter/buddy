package tokenauth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"log/slog"
)

// Authenticator handles authentication for Jamf Pro API
type Authenticator struct {
	baseURL      string
	username     string
	password     string
	tokenCommand []string
	clientId     string
	clientSecret string
	cacheKey     string // when set, persist token+expiry under ~/.config/buddy/tokens/
	token        string
	tokenExpiry  time.Time
	httpClient   *http.Client
	logger       *slog.Logger
}

// Config holds authentication configuration
type Config struct {
	BaseURL      string
	Username     string
	Password     string
	TokenCommand []string
	ClientId     string
	ClientSecret string
	// CacheKey enables on-disk token cache (typically the credential name). Empty disables disk cache.
	CacheKey string
	Logger   *slog.Logger
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
}

// NewAuthenticator creates a new Authenticator with the given configuration
func NewAuthenticator(cfg Config) *Authenticator {
	httpClient := &http.Client{Timeout: 30 * time.Second}
	if cfg.Logger == nil {
		// Create a minimal logger if none provided
		cfg.Logger = slog.Default()
	}
	return &Authenticator{
		baseURL:      cfg.BaseURL,
		username:     cfg.Username,
		password:     cfg.Password,
		tokenCommand: cfg.TokenCommand,
		clientId:     cfg.ClientId,
		clientSecret: cfg.ClientSecret,
		cacheKey:     cfg.CacheKey,
		httpClient:   httpClient,
		logger:       cfg.Logger,
	}
}

func (a *Authenticator) debugf(format string, args ...any) {
	if a.logger != nil {
		a.logger.Debug(fmt.Sprintf(format, args...))
	}
}

func (a *Authenticator) verbosef(format string, args ...any) {
	if a.logger != nil {
		a.logger.Debug(fmt.Sprintf(format, args...))
	}
}

func (a *Authenticator) debugJSON(label string, data any) {
	if a.logger == nil {
		return
	}
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		a.debugf("%s: %+v", label, data)
		return
	}
	a.debugf("%s:\n%s", label, string(jsonData))
}

// GetToken returns a valid bearer token, refreshing if necessary
func (a *Authenticator) GetToken() (string, error) {
	if err := a.ensureValidToken(); err != nil {
		return "", err
	}
	return a.token, nil
}

const proactiveRefreshWindow = 3 * time.Minute

// setToken stores token+expiry in memory and persists to disk when CacheKey is set.
func (a *Authenticator) setToken(token string, expiry time.Time) {
	a.token = token
	a.tokenExpiry = expiry
	if a.cacheKey == "" || token == "" || len(a.tokenCommand) > 0 {
		return
	}
	if err := saveDiskToken(a.cacheKey, a.baseURL, token, expiry); err != nil {
		a.debugf("token disk cache save failed: %v", err)
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
	tok, exp, ok := loadDiskToken(a.cacheKey, a.baseURL, proactiveRefreshWindow)
	if !ok {
		return false
	}
	a.token = tok
	a.tokenExpiry = exp
	a.debugf("loaded token from disk cache (expires %v)", exp)
	return true
}

func (a *Authenticator) clearMemoryToken() {
	a.token = ""
	a.tokenExpiry = time.Time{}
}

// ensureValidToken ensures we have a valid token, refreshing if necessary
func (a *Authenticator) ensureValidToken() error {
	// If using external token command, get token from that (no disk cache)
	if len(a.tokenCommand) > 0 {
		return a.ensureTokenCommand()
	}

	// Hydrate from disk before deciding to hit Jamf
	if a.token == "" {
		_ = a.loadFromDiskIfValid()
	}

	// If using OAuth2 client credentials, handle that flow
	if a.clientId != "" && a.clientSecret != "" {
		return a.ensureOAuth2Token()
	}

	// Username/password auth
	return a.ensureBasicAuthToken()
}

// ensureTokenCommand ensures a valid token for token command authentication
func (a *Authenticator) ensureTokenCommand() error {
	if a.token != "" && time.Now().Before(a.tokenExpiry) {
		return nil
	}
	return a.getTokenFromCommand()
}

// ensureOAuth2Token ensures a valid OAuth2 token
func (a *Authenticator) ensureOAuth2Token() error {
	now := time.Now()
	a.debugf("OAuth2 token check: token exists=%v, expiry=%v, now=%v", a.token != "", a.tokenExpiry, now)

	// If we have no token, get a new one
	if a.token == "" {
		a.debugf("OAuth2 token is empty, requesting new token")
		return a.getTokenFromOAuth2()
	}

	// If token is still valid, check if we should proactively refresh it
	if now.Before(a.tokenExpiry) {
		return a.handleOAuth2TokenRefresh(now)
	}

	// Token has expired, get a new one
	timeSinceExpiry := now.Sub(a.tokenExpiry)
	a.debugf("OAuth2 token has expired (%d seconds ago), requesting new token", int(timeSinceExpiry.Seconds()))
	return a.getTokenFromOAuth2()
}

// handleOAuth2TokenRefresh handles OAuth2 token refresh logic when token is still valid
func (a *Authenticator) handleOAuth2TokenRefresh(now time.Time) error {
	timeUntilExpiry := a.tokenExpiry.Sub(now)
	secondsRemaining := int(timeUntilExpiry.Seconds())
	a.debugf("OAuth2 token is valid, expires in %d seconds", secondsRemaining)

	if timeUntilExpiry > 3*time.Minute {
		// Token is still good for more than 3 minutes, no need to refresh
		a.debugf("OAuth2 token is valid for more than 3 minutes (%d seconds remaining), reusing existing token", secondsRemaining)
		return nil
	}

	// Token expires soon, try keep-alive first
	a.debugf("OAuth2 token expires soon (%d seconds), attempting keep-alive", secondsRemaining)
	keepAliveErr := a.refreshTokenWithKeepAlive()
	if keepAliveErr == nil {
		a.debugf("OAuth2 token keep-alive successful")
		return nil
	}

	// Keep-alive failed, but token is still valid - continue using it
	// We'll try keep-alive again on the next call, or get a new token when it actually expires
	a.verbosef("Keep-alive failed for OAuth2 token, but token is still valid (expires in %d seconds). Will retry keep-alive on next call: %v", secondsRemaining, keepAliveErr)
	return nil
}

// ensureBasicAuthToken ensures a valid token for username/password authentication
func (a *Authenticator) ensureBasicAuthToken() error {
	// If we have no token, get a new one
	if a.token == "" {
		return a.getTokenFromCredentials()
	}

	// If token is still valid, check if we should proactively refresh it
	now := time.Now()
	if now.Before(a.tokenExpiry) {
		// Check if token expires within 3 minutes - if so, proactively refresh it
		timeUntilExpiry := a.tokenExpiry.Sub(now)
		if timeUntilExpiry > 3*time.Minute {
			// Token is still good for more than 3 minutes, no need to refresh
			return nil
		}
		// Token expires soon, proactively refresh it using keep-alive
		return a.refreshTokenWithKeepAlive()
	}

	// Token has expired, get a new one
	return a.getTokenFromCredentials()
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
func (a *Authenticator) getTokenFromCredentials() error {
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
		a.debugf("Token request failed with status %d: %s", resp.StatusCode, string(body))
		return fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
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
		a.debugf("OAuth2 token request failed with status %d: %s", resp.StatusCode, string(body))
		return fmt.Errorf("OAuth2 token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var oauthResp OAuth2TokenResponse
	if err := json.Unmarshal(body, &oauthResp); err != nil {
		return fmt.Errorf("failed to decode OAuth2 token response: %w", err)
	}

	if oauthResp.AccessToken == "" {
		return fmt.Errorf("OAuth2 response missing access_token")
	}

	a.debugJSON("OAuth2 token response", oauthResp)

	// Calculate expiry from expires_in (typically 1800 seconds = 30 minutes)
	expiresIn := time.Duration(oauthResp.ExpiresIn) * time.Second
	if expiresIn == 0 {
		// Default to 30 minutes if not specified
		expiresIn = 30 * time.Minute
	}
	a.setToken(oauthResp.AccessToken, time.Now().Add(expiresIn))
	// Log the actual token lifetime
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

	a.verbosef("Proactively refreshing authentication token using keep-alive")
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
		a.debugf("Token refresh failed with status %d: %s", resp.StatusCode, string(body))
		return fmt.Errorf("token refresh failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return fmt.Errorf("failed to decode token response: %w", err)
	}

	a.debugJSON("Token refresh response", tokenResp)
	a.setToken(tokenResp.Token, time.Now().Add(18*time.Minute))
	a.verbosef("Token refreshed successfully, expires at %v", a.tokenExpiry)

	return nil
}

// RefreshToken handles token refresh reactively (on 401) or proactively (before expiry)
func (a *Authenticator) RefreshToken() error {
	a.verbosef("Refreshing authentication token")
	// If using external token command, just get a new token from the command
	if len(a.tokenCommand) > 0 {
		// Clear current token to force refresh
		a.clearMemoryToken()
		return a.getTokenFromCommand()
	}

	// If using OAuth2, get a new token (OAuth2 client credentials doesn't support keep-alive)
	if a.clientId != "" && a.clientSecret != "" {
		a.clearMemoryToken()
		a.InvalidateDiskCache()
		return a.getTokenFromOAuth2()
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
