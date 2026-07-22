package buddy

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"buddy/internal/buddy/tokenauth"
)

// DirectAuthOpts holds one-off authentication settings (no config credential).
type DirectAuthOpts struct {
	BaseURL         string
	Region          string // Platform API Gateway region (builds BaseURL when set and BaseURL empty)
	TenantID        string // Platform API Gateway tenant id
	Username        string
	Password        string
	ClientID        string
	ClientSecret    string
	OAuth2CredsFile string
}

func (o DirectAuthOpts) UsesDirectAuth() bool {
	return o.BaseURL != "" ||
		o.Region != "" ||
		o.TenantID != "" ||
		o.Username != "" ||
		o.Password != "" ||
		o.ClientID != "" ||
		o.ClientSecret != "" ||
		o.OAuth2CredsFile != ""
}

// OAuth2CredsFile represents client credentials stored in a JSON file.
type OAuth2CredsFile struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

// LoadOAuth2CredsFile reads client_id and client_secret from a JSON file.
func LoadOAuth2CredsFile(path string) (*OAuth2CredsFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read OAuth2 credentials file: %w", err)
	}
	var creds OAuth2CredsFile
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("parse OAuth2 credentials file: %w", err)
	}
	if creds.ClientID == "" {
		return nil, fmt.Errorf("OAuth2 credentials file missing client_id")
	}
	if creds.ClientSecret == "" {
		return nil, fmt.Errorf("OAuth2 credentials file missing client_secret")
	}
	return &creds, nil
}

func (o *DirectAuthOpts) resolveOAuth2File() error {
	if o.OAuth2CredsFile == "" {
		return nil
	}
	creds, err := LoadOAuth2CredsFile(o.OAuth2CredsFile)
	if err != nil {
		return err
	}
	o.ClientID = creds.ClientID
	o.ClientSecret = creds.ClientSecret
	return nil
}

// ObtainRawToken returns a bearer token string using either direct auth flags or a config credential.
func ObtainRawToken(credName string, direct DirectAuthOpts, log *slog.Logger) (string, error) {
	if direct.UsesDirectAuth() {
		return obtainRawTokenDirect(direct, log)
	}
	return resolveRawToken(credName)
}

func obtainRawTokenDirect(direct DirectAuthOpts, log *slog.Logger) (string, error) {
	if err := direct.resolveOAuth2File(); err != nil {
		return "", err
	}
	if direct.BaseURL == "" && strings.TrimSpace(direct.Region) != "" {
		region, err := NormalizePlatformRegion(direct.Region)
		if err != nil {
			return "", err
		}
		direct.BaseURL = PlatformGatewayURL(region)
	}
	if direct.BaseURL == "" {
		return "", fmt.Errorf("Jamf Pro URL is required for direct auth (use --url or --region)")
	}
	platform := strings.TrimSpace(direct.TenantID) != ""
	if platform && (direct.ClientID == "" || direct.ClientSecret == "") {
		return "", fmt.Errorf("Platform API Gateway direct auth requires --client-id and --client-secret (and --tenant-id)")
	}
	cfg := tokenauth.Config{
		BaseURL:      direct.BaseURL,
		Username:     direct.Username,
		Password:     direct.Password,
		ClientId:     direct.ClientID,
		ClientSecret: direct.ClientSecret,
		Platform:     platform,
		Logger:       log,
	}
	if err := validateDirectAuth(cfg); err != nil {
		return "", err
	}
	cfg.CacheKey = directAuthCacheKey(cfg, direct.TenantID)
	a := tokenauth.NewAuthenticator(cfg)
	return a.GetToken()
}

func directAuthCacheKey(cfg tokenauth.Config, tenantID string) string {
	base := strings.TrimRight(cfg.BaseURL, "/")
	if cfg.Platform {
		return "direct-platform-" + cfg.ClientId + "@" + base + "/" + strings.TrimSpace(tenantID)
	}
	if cfg.ClientId != "" {
		return "direct-oauth-" + cfg.ClientId + "@" + base
	}
	return "direct-basic-" + cfg.Username + "@" + base
}

func validateDirectAuth(cfg tokenauth.Config) error {
	hasBasic := cfg.Username != "" && cfg.Password != ""
	hasOAuth := cfg.ClientId != "" && cfg.ClientSecret != ""
	methods := 0
	if hasBasic {
		methods++
	}
	if hasOAuth {
		methods++
	}
	if methods == 0 {
		return fmt.Errorf("direct auth requires username/password or client-id/client-secret (or --oauth2-creds-file)")
	}
	if methods > 1 {
		return fmt.Errorf("cannot specify multiple direct authentication methods")
	}
	return nil
}

// RefreshRawToken extends a bearer token using the keep-alive endpoint.
func RefreshRawToken(baseURL, tokenValue string, log *slog.Logger) (string, error) {
	if baseURL == "" {
		return "", fmt.Errorf("Jamf Pro URL is required (use --url or --user with a configured credential)")
	}
	if tokenValue == "" {
		return "", fmt.Errorf("token is required")
	}
	a := tokenauth.NewAuthenticator(tokenauth.Config{
		BaseURL: baseURL,
		Logger:  log,
	})
	return a.RefreshTokenValue(tokenValue)
}

// AuthHeader resolves and returns the Authorization header value for the named credential.
func AuthHeader(credName string) (string, error) {
	return resolveAuthHeader(credName)
}

// GetToken resolves and prints the Authorization header value for the named credential.
// Output is the full header value, e.g. "Bearer eyJ..." or "Basic dXNl...".
func GetToken(credName string) error {
	header, err := AuthHeader(credName)
	if err != nil {
		return err
	}
	fmt.Println(header)
	return nil
}

// PrintRawToken prints the bearer token for a credential or direct auth flags.
func PrintRawToken(credName string, direct DirectAuthOpts, log *slog.Logger) error {
	token, err := ObtainRawToken(credName, direct, log)
	if err != nil {
		return err
	}
	fmt.Println(token)
	return nil
}

// AuthInfo returns compact info about the current credential's Jamf session.
// With renew=true, discards any cached token and obtains a fresh one first.
// Does not include the raw bearer token.
func AuthInfo(credName string, renew bool, log *slog.Logger) (map[string]interface{}, error) {
	if activeConfig == nil {
		return nil, fmt.Errorf("internal error: config not loaded (SetActiveConfig not called)")
	}
	cred, err := activeConfig.ResolveCredential(credName)
	if err != nil {
		return nil, err
	}
	if renew {
		switch cred.Auth.Type {
		case "basic", "oauth":
			a, err := cachedAuthenticator(credName, cred)
			if err != nil {
				return nil, err
			}
			if err := a.ForceRenew(); err != nil {
				return nil, fmt.Errorf("renew token: %w", err)
			}
		case "command":
			// command auth is not disk-cached; AuthTestValue re-runs the command
		case "":
			return nil, fmt.Errorf("credential %q has no auth type configured", credName)
		default:
			return nil, fmt.Errorf("credential %q has unknown auth type %q", credName, cred.Auth.Type)
		}
	}

	raw, err := AuthTestValue(credName, log)
	if err != nil {
		return nil, err
	}
	out := CompactAuthInfo(raw)
	out["cred"] = credName
	out["url"] = strings.TrimRight(cred.URL, "/")
	out["authType"] = cred.Auth.Type
	out["renewed"] = renew
	if cred.Auth.Type == "basic" || cred.Auth.Type == "oauth" {
		if a, err := cachedAuthenticator(credName, cred); err == nil {
			if exp := a.TokenExpiry(); !exp.IsZero() {
				out["expiresAt"] = exp.UTC().Format(time.RFC3339)
			}
		}
	}
	return out, nil
}

// CompactAuthInfo reduces a GET /api/v1/auth payload to a small summary dict.
func CompactAuthInfo(info map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	if at, ok := info["authenticationType"]; ok {
		out["authenticationType"] = at
	}
	acct, _ := info["account"].(map[string]interface{})
	if acct != nil {
		out["username"] = acct["username"]
		out["accessLevel"] = acct["accessLevel"]
		out["privilegeSet"] = acct["privilegeSet"]
		if bySite, ok := acct["privilegesBySite"].(map[string]interface{}); ok {
			n := 0
			for _, v := range bySite {
				if list, ok := v.([]interface{}); ok {
					n += len(list)
				}
			}
			out["privilegeCount"] = n
		}
	}
	if sites, ok := info["sites"]; ok {
		out["sites"] = sites
	}
	return out
}

// AuthTestValue tests authentication and returns a JSON-friendly object.
// Instance credentials use GET /api/v1/auth. Platform credentials obtain a token
// and probe GET /api/v1/buildings (rewritten to the gateway); if buildings is
// forbidden but the token succeeded, a token-acquired summary is returned.
func AuthTestValue(credName string, log *slog.Logger) (map[string]interface{}, error) {
	platform, err := IsPlatformCred(credName)
	if err != nil {
		return nil, err
	}
	if platform {
		return authTestPlatform(credName, log)
	}
	endpoint, err := ResolveEndpoint(credName, "/api/v1/auth")
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return nil, fmt.Errorf("auth test: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == 401 {
			return nil, fmt.Errorf("auth test returned 401 Unauthorized: %s — check username/password; for Jamf Pro API, OAuth (client credentials) may be required instead of Basic auth", string(body))
		}
		return nil, fmt.Errorf("auth test returned %d: %s", resp.StatusCode, string(body))
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return map[string]interface{}{"raw": string(body), "status": resp.StatusCode}, nil
	}
	return result, nil
}

func authTestPlatform(credName string, log *slog.Logger) (map[string]interface{}, error) {
	// Force a fresh platform token so disk-cached tokens from a prior endpoint
	// (e.g. /oauth2/token) cannot mask gateway REQUEST_CONTEXT failures.
	if activeConfig == nil {
		return nil, fmt.Errorf("internal error: config not loaded (SetActiveConfig not called)")
	}
	cred, err := activeConfig.ResolveCredential(credName)
	if err != nil {
		return nil, err
	}
	a, err := cachedAuthenticator(credName, cred)
	if err != nil {
		return nil, fmt.Errorf("platform auth token: %w", err)
	}
	if err := a.ForceRenew(); err != nil {
		return nil, fmt.Errorf("platform auth token: %w", err)
	}
	endpoint, err := ResolveEndpoint(credName, "/api/v1/buildings")
	if err != nil {
		return nil, err
	}
	endpoint += "?page=0&page-size=1"
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return nil, fmt.Errorf("platform auth test: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	tenantID, _ := tenantIDForCred(credName)
	baseURL, _ := resolveURL(credName)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return map[string]interface{}{
			"ok":       true,
			"mode":     "platform",
			"url":      baseURL,
			"tenantId": tenantID,
			"probe":    "buildings",
			"status":   resp.StatusCode,
		}, nil
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		// Token was obtained; privilege for buildings may be missing.
		if log != nil {
			log.Debug("platform auth probe buildings not permitted; token acquired", "status", resp.StatusCode)
		}
		return map[string]interface{}{
			"ok":            true,
			"mode":          "platform",
			"url":           baseURL,
			"tenantId":      tenantID,
			"tokenAcquired": true,
			"probe":         "buildings",
			"probeStatus":   resp.StatusCode,
			"note":          "Bearer token acquired; buildings probe not permitted (need read:pro:buildings for a full probe)",
		}, nil
	}
	looksUUID := len(tenantID) == 36 && strings.Count(tenantID, "-") == 4
	if strings.Contains(string(body), "REQUEST_CONTEXT_NOT_PROVIDED") && !looksUUID {
		return nil, fmt.Errorf("platform auth test returned %d: %s\nHint: tenantId %q does not look like a Jamf Account tenant UUID. In Jamf Account → Integrations, open the integration and click the tenant pill to copy the tenant id (not the jamfcloud.com subdomain)", resp.StatusCode, string(body), tenantID)
	}
	return nil, fmt.Errorf("platform auth test returned %d: %s", resp.StatusCode, string(body))
}

// AuthTest tests authentication against /api/v1/auth and pretty-prints the response.
func AuthTest(credName string, log *slog.Logger) error {
	result, err := AuthTestValue(credName, log)
	if err != nil {
		return err
	}
	out, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(out))
	return nil
}
