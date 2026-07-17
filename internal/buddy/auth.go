package buddy

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"

	"buddy/internal/buddy/tokenauth"
)

// DirectAuthOpts holds one-off authentication settings (no config credential).
type DirectAuthOpts struct {
	BaseURL         string
	Username        string
	Password        string
	ClientID        string
	ClientSecret    string
	OAuth2CredsFile string
}

func (o DirectAuthOpts) UsesDirectAuth() bool {
	return o.BaseURL != "" ||
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
	if direct.BaseURL == "" {
		return "", fmt.Errorf("Jamf Pro URL is required for direct auth (use --url)")
	}
	cfg := tokenauth.Config{
		BaseURL:      direct.BaseURL,
		Username:     direct.Username,
		Password:     direct.Password,
		ClientId:     direct.ClientID,
		ClientSecret: direct.ClientSecret,
		Logger:       log,
	}
	if err := validateDirectAuth(cfg); err != nil {
		return "", err
	}
	a := tokenauth.NewAuthenticator(cfg)
	return a.GetToken()
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

// AuthTestValue tests authentication against /api/v1/auth and returns the JSON object.
func AuthTestValue(credName string, log *slog.Logger) (map[string]interface{}, error) {
	baseURL, err := GetBaseURL(credName)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, baseURL+"/api/v1/auth", nil)
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
