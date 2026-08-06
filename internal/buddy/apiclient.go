package buddy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
)

type ApiIntegrationResponse struct {
	ID                      int      `json:"id"`
	DisplayName             string   `json:"displayName"`
	Enabled                 bool     `json:"enabled"`
	AccessTokenLifetimeSecs int      `json:"accessTokenLifetimeSeconds"`
	AppType                 string   `json:"appType"`
	ClientID                string   `json:"clientId"`
	AuthorizationScopes     []string `json:"authorizationScopes"`
}

type ApiIntegrationRequest struct {
	DisplayName             string   `json:"displayName"`
	Enabled                 bool     `json:"enabled"`
	AccessTokenLifetimeSecs int      `json:"accessTokenLifetimeSeconds,omitempty"`
	AuthorizationScopes     []string `json:"authorizationScopes"`
}

type ApiIntegrationSearchResult struct {
	TotalCount int                      `json:"totalCount"`
	Results    []ApiIntegrationResponse `json:"results"`
}

type OAuthClientCredentials struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

// ClientList fetches and prints API integrations matching the optional filter.
// Bare term → displayName=="*term*"; RSQL expression → pass-through.
func ClientList(credName, filter string, logger *slog.Logger) error {
	clients, err := ClientListValues(credName, filter, PageOpts{Page: PageAll}, logger)
	if err != nil {
		return err
	}
	result := ApiIntegrationSearchResult{TotalCount: len(clients), Results: clients}
	out, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(out))
	return nil
}

// ClientListValues fetches API integrations and returns them (for script/setup use).
// When opts.Page is PageAll, all pages are fetched.
// Filter: bare term → displayName=="*term*"; RSQL → pass-through.
func ClientListValues(credName, filter string, opts PageOpts, logger *slog.Logger) ([]ApiIntegrationResponse, error) {
	normalized, err := NormalizeListFilter(filter, FilterSpecClients)
	if err != nil {
		return nil, err
	}
	extra := url.Values{}
	if normalized != "" {
		extra.Set("filter", normalized)
	}
	var all []ApiIntegrationResponse
	err = fetchJamfPaged(credName, "/api/v1/api-integrations", extra, opts, func(resultsJSON []byte, _ int) (int, error) {
		var page []ApiIntegrationResponse
		if err := json.Unmarshal(resultsJSON, &page); err != nil {
			return 0, fmt.Errorf("parse response: %w", err)
		}
		all = append(all, page...)
		return len(page), nil
	})
	if err != nil {
		return nil, err
	}
	return all, nil
}

// ClientFindByDisplayName returns the first API integration whose display name matches.
func ClientFindByDisplayName(credName, displayName string, logger *slog.Logger) (*ApiIntegrationResponse, error) {
	clients, err := ClientListValues(credName, "", PageOpts{Page: PageAll}, logger)
	if err != nil {
		return nil, err
	}
	for i := range clients {
		if clients[i].DisplayName == displayName {
			return &clients[i], nil
		}
	}
	return nil, fmt.Errorf("no API integration with display name %q found", displayName)
}

// ClientGet fetches and prints a single API integration by ID.
func ClientGet(credName, id string, logger *slog.Logger) error {
	url, err := ResolveEndpoint(credName, "/api/v1/api-integrations/" + id)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return fmt.Errorf("get client %s: %w", id, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("get client %s status %d: %s", id, resp.StatusCode, string(body))
	}
	var client ApiIntegrationResponse
	if err := json.Unmarshal(body, &client); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	out, _ := json.MarshalIndent(client, "", "  ")
	fmt.Println(string(out))
	return nil
}

// ClientFetch fetches a single API integration by ID and returns it (for script/component use).
func ClientFetch(credName, id string, logger *slog.Logger) (*ApiIntegrationResponse, error) {
	return clientGetRaw(credName, id)
}

// ClientCreate creates a new API integration with the given display name and authorization scopes.
func ClientCreate(credName string, opts Opts, displayName string, scopes []string, logger *slog.Logger) error {
	client, err := ClientCreateWithResult(credName, opts, displayName, scopes, logger)
	if err != nil {
		return err
	}
	if client == nil {
		return nil
	}
	out, _ := json.MarshalIndent(client, "", "  ")
	fmt.Printf("Created API client (ID %d):\n%s\n", client.ID, string(out))
	return nil
}

// ClientCreateWithResult creates a new API integration and returns the created client (including ID).
func ClientCreateWithResult(credName string, opts Opts, displayName string, scopes []string, logger *slog.Logger) (*ApiIntegrationResponse, error) {
	payload := ApiIntegrationRequest{
		DisplayName:         displayName,
		Enabled:             true,
		AuthorizationScopes: scopes,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("Create API client %q with scopes: %v", displayName, scopes), logger) {
			return nil, nil
		}
	}
	url, err := ResolveEndpoint(credName, "/api/v1/api-integrations")
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("create client status %d: %s", resp.StatusCode, string(body))
	}
	var client ApiIntegrationResponse
	if err := json.Unmarshal(body, &client); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &client, nil
}

// ClientUpdate updates an existing API integration by ID.
func ClientUpdate(credName, id string, opts Opts, displayName string, scopes []string, logger *slog.Logger) error {
	client, err := ClientUpdateWithResult(credName, id, opts, displayName, scopes, logger)
	if err != nil {
		return err
	}
	if client == nil {
		return nil
	}
	out, _ := json.MarshalIndent(client, "", "  ")
	fmt.Printf("Updated API client (ID %d):\n%s\n", client.ID, string(out))
	return nil
}

// ClientUpdateWithResult updates an existing API integration by ID and returns the updated record (for script/component use).
func ClientUpdateWithResult(credName, id string, opts Opts, displayName string, scopes []string, logger *slog.Logger) (*ApiIntegrationResponse, error) {
	// Fetch the existing integration to preserve fields not being updated.
	existing, err := clientGetRaw(credName, id)
	if err != nil {
		return nil, fmt.Errorf("fetch existing client: %w", err)
	}
	payload := ApiIntegrationRequest{
		DisplayName:             displayName,
		Enabled:                 existing.Enabled,
		AccessTokenLifetimeSecs: existing.AccessTokenLifetimeSecs,
		AuthorizationScopes:     scopes,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("Update API client %s to name %q with scopes: %v", id, displayName, scopes), logger) {
			return nil, nil
		}
	}
	url, err := ResolveEndpoint(credName, "/api/v1/api-integrations/" + id)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return nil, fmt.Errorf("update client %s: %w", id, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update client %s status %d: %s", id, resp.StatusCode, string(body))
	}
	var client ApiIntegrationResponse
	if err := json.Unmarshal(body, &client); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &client, nil
}

// ClientPutFields fetches the existing client, applies updates from fields, then PUTs the payload.
// Supported fields: displayName (string), authorizationScopes ([]string).
func ClientPutFields(credName string, opts Opts, id string, fields map[string]interface{}, logger *slog.Logger) (*ApiIntegrationResponse, error) {
	existing, err := clientGetRaw(credName, id)
	if err != nil {
		return nil, err
	}
	displayName := existing.DisplayName
	if v, ok := fields["displayName"].(string); ok && v != "" {
		displayName = v
	}
	scopes := existing.AuthorizationScopes
	if v, ok := fields["authorizationScopes"]; ok {
		switch arr := v.(type) {
		case []interface{}:
			var s []string
			for _, item := range arr {
				if str, ok := item.(string); ok {
					s = append(s, str)
				}
			}
			if s != nil {
				scopes = s
			}
		case []string:
			scopes = arr
		}
	}
	return ClientUpdateWithResult(credName, id, opts, displayName, scopes, logger)
}

// ClientDelete deletes an API integration by ID.
func ClientDelete(credName, id string, opts Opts, logger *slog.Logger) error {
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("Delete API client %s", id), logger) {
			return nil
		}
	}
	url, err := ResolveEndpoint(credName, "/api/v1/api-integrations/" + id)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return fmt.Errorf("delete client %s: %w", id, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete client %s status %d: %s", id, resp.StatusCode, string(body))
	}
	fmt.Printf("Deleted API client (ID %s)\n", id)
	return nil
}

// ClientCredentials generates and prints OAuth client credentials for an API integration.
// The returned client secret is shown only once — store it securely.
func ClientCredentials(credName, id string, opts Opts, logger *slog.Logger) error {
	clientID, clientSecret, err := ClientCredentialsWithResult(credName, id, opts, logger)
	if err != nil {
		return err
	}
	if clientID == "" && clientSecret == "" {
		return nil
	}
	fmt.Printf("Client ID:     %s\nClient Secret: %s\n", clientID, clientSecret)
	return nil
}

// ClientCredentialsWithResult generates OAuth client credentials and returns them (caller must persist).
func ClientCredentialsWithResult(credName, id string, opts Opts, logger *slog.Logger) (clientID, clientSecret string, err error) {
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("Generate new client credentials for API client %s (existing secret will be invalidated)", id), logger) {
			return "", "", nil
		}
	}
	url, err := ResolveEndpoint(credName, "/api/v1/api-integrations/" + id + "/client-credentials")
	if err != nil {
		return "", "", err
	}
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return "", "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return "", "", fmt.Errorf("generate credentials for client %s: %w", id, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", "", fmt.Errorf("generate credentials status %d: %s", resp.StatusCode, string(body))
	}
	var creds OAuthClientCredentials
	if err := json.Unmarshal(body, &creds); err != nil {
		return "", "", fmt.Errorf("parse response: %w", err)
	}
	return creds.ClientID, creds.ClientSecret, nil
}

// ClientSetTokenLifetime updates only the accessTokenLifetimeSeconds of an API integration.
func ClientSetTokenLifetime(credName, id string, opts Opts, seconds int, logger *slog.Logger) error {
	existing, err := clientGetRaw(credName, id)
	if err != nil {
		return fmt.Errorf("fetch existing client: %w", err)
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("Set token lifetime for API client %s (%s): %ds → %ds",
			id, existing.DisplayName, existing.AccessTokenLifetimeSecs, seconds), logger) {
			return nil
		}
	}
	payload := ApiIntegrationRequest{
		DisplayName:             existing.DisplayName,
		Enabled:                 existing.Enabled,
		AccessTokenLifetimeSecs: seconds,
		AuthorizationScopes:     existing.AuthorizationScopes,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	endpoint, err := ResolveEndpoint(credName, "/api/v1/api-integrations/" + id)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return fmt.Errorf("update client %s: %w", id, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("update client %s status %d: %s", id, resp.StatusCode, string(body))
	}
	var updated ApiIntegrationResponse
	if err := json.Unmarshal(body, &updated); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	fmt.Printf("Updated API client %s: accessTokenLifetimeSeconds = %d\n", id, updated.AccessTokenLifetimeSecs)
	return nil
}

// clientGetRaw fetches an API integration without printing, used internally.
func clientGetRaw(credName, id string) (*ApiIntegrationResponse, error) {
	url, err := ResolveEndpoint(credName, "/api/v1/api-integrations/" + id)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return nil, fmt.Errorf("get client %s: %w", id, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get client %s status %d: %s", id, resp.StatusCode, string(body))
	}
	var client ApiIntegrationResponse
	if err := json.Unmarshal(body, &client); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &client, nil
}
