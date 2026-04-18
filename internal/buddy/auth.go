package buddy

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

// GetToken resolves and prints the Authorization header value for the named credential.
// Output is the full header value, e.g. "Bearer eyJ..." or "Basic dXNl...".
func GetToken(credName string) error {
	header, err := resolveAuthHeader(credName)
	if err != nil {
		return err
	}
	fmt.Println(header)
	return nil
}

// AuthTest tests authentication against /api/v1/auth and pretty-prints the response.
func AuthTest(credName string, log *slog.Logger) error {
	baseURL, err := GetBaseURL(credName)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodGet, baseURL+"/api/v1/auth", nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return fmt.Errorf("auth test: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == 401 {
			return fmt.Errorf("auth test returned 401 Unauthorized: %s — check username/password; for Jamf Pro API, OAuth (client credentials) may be required instead of Basic auth", string(body))
		}
		return fmt.Errorf("auth test returned %d: %s", resp.StatusCode, string(body))
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Printf("OK (status %d)\n%s\n", resp.StatusCode, string(body))
		return nil
	}
	out, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(out))
	return nil
}
