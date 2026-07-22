package buddy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

type ApiRole struct {
	ID          string   `json:"id"`
	DisplayName string   `json:"displayName"`
	Privileges  []string `json:"privileges"`
}

type ApiRoleRequest struct {
	DisplayName string   `json:"displayName"`
	Privileges  []string `json:"privileges"`
}

type ApiRoleResult struct {
	TotalCount int       `json:"totalCount"`
	Results    []ApiRole `json:"results"`
}

type ApiRolePrivileges struct {
	Privileges []string `json:"privileges"`
}

// RoleList fetches and prints all API roles.
func RoleList(credName string, logger *slog.Logger) error {
	roles, err := RoleListValues(credName, PageOpts{Page: PageAll}, logger)
	if err != nil {
		return err
	}
	result := ApiRoleResult{TotalCount: len(roles), Results: roles}
	out, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(out))
	return nil
}

// RoleListValues fetches API roles and returns them (for script use).
// When opts.Page is PageAll, all pages are fetched.
func RoleListValues(credName string, opts PageOpts, logger *slog.Logger) ([]ApiRole, error) {
	var all []ApiRole
	err := fetchJamfPaged(credName, "/api/v1/api-roles", nil, opts, func(resultsJSON []byte, _ int) (int, error) {
		var page []ApiRole
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

// RoleGet fetches and prints a single API role by ID.
func RoleGet(credName, id string, logger *slog.Logger) error {
	url, err := ResolveEndpoint(credName, "/api/v1/api-roles/"+id)
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
		return fmt.Errorf("get role %s: %w", id, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("get role %s status %d: %s", id, resp.StatusCode, string(body))
	}
	var role ApiRole
	if err := json.Unmarshal(body, &role); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	out, _ := json.MarshalIndent(role, "", "  ")
	fmt.Println(string(out))
	return nil
}

// RoleFetch fetches a single API role by ID and returns it (for script/component use).
func RoleFetch(credName, id string, logger *slog.Logger) (*ApiRole, error) {
	url, err := ResolveEndpoint(credName, "/api/v1/api-roles/"+id)
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
		return nil, fmt.Errorf("get role %s: %w", id, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get role %s status %d: %s", id, resp.StatusCode, string(body))
	}
	var role ApiRole
	if err := json.Unmarshal(body, &role); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &role, nil
}

// RoleCreate creates a new API role with the given display name and privileges.
func RoleCreate(credName string, opts Opts, displayName string, privileges []string, logger *slog.Logger) error {
	role, err := RoleCreateWithID(credName, opts, displayName, privileges, logger)
	if err != nil {
		return err
	}
	if role == nil {
		return nil
	}
	out, _ := json.MarshalIndent(role, "", "  ")
	fmt.Printf("Created API role (ID %s):\n%s\n", role.ID, string(out))
	return nil
}

// RoleCreateWithID creates a new API role and returns the created role (including ID).
func RoleCreateWithID(credName string, opts Opts, displayName string, privileges []string, logger *slog.Logger) (*ApiRole, error) {
	payload := ApiRoleRequest{
		DisplayName: displayName,
		Privileges:  privileges,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("Create API role %q with %d privilege(s)", displayName, len(privileges)), logger) {
			return nil, nil
		}
	}
	url, err := ResolveEndpoint(credName, "/api/v1/api-roles")
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
		return nil, fmt.Errorf("create role: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("create role status %d: %s", resp.StatusCode, string(body))
	}
	var role ApiRole
	if err := json.Unmarshal(body, &role); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &role, nil
}

// RoleUpdate updates an existing API role by ID.
func RoleUpdate(credName, id string, opts Opts, displayName string, privileges []string, logger *slog.Logger) error {
	role, err := RoleUpdateWithResult(credName, id, opts, displayName, privileges, logger)
	if err != nil {
		return err
	}
	if role == nil {
		return nil
	}
	out, _ := json.MarshalIndent(role, "", "  ")
	fmt.Printf("Updated API role (ID %s):\n%s\n", role.ID, string(out))
	return nil
}

// RoleUpdateWithResult updates an existing API role by ID and returns the updated role (for script/component use).
func RoleUpdateWithResult(credName, id string, opts Opts, displayName string, privileges []string, logger *slog.Logger) (*ApiRole, error) {
	payload := ApiRoleRequest{
		DisplayName: displayName,
		Privileges:  privileges,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("Update API role %s to name %q with %d privilege(s)", id, displayName, len(privileges)), logger) {
			return nil, nil
		}
	}
	url, err := ResolveEndpoint(credName, "/api/v1/api-roles/" + id)
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
		return nil, fmt.Errorf("update role %s: %w", id, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update role %s status %d: %s", id, resp.StatusCode, string(body))
	}
	var role ApiRole
	if err := json.Unmarshal(body, &role); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &role, nil
}

// RoleDelete deletes an API role by ID.
func RoleDelete(credName, id string, opts Opts, logger *slog.Logger) error {
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("Delete API role %s", id), logger) {
			return nil
		}
	}
	url, err := ResolveEndpoint(credName, "/api/v1/api-roles/" + id)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return fmt.Errorf("delete role %s: %w", id, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete role %s status %d: %s", id, resp.StatusCode, string(body))
	}
	fmt.Printf("Deleted API role (ID %s)\n", id)
	return nil
}

// RolePrivileges fetches and prints all available API role privileges.
func RolePrivileges(credName string, logger *slog.Logger) error {
	privs, err := RolePrivilegesList(credName, logger)
	if err != nil {
		return err
	}
	for _, p := range privs {
		fmt.Println(p)
	}
	return nil
}

// RolePrivilegesList fetches and returns all available API role privilege strings.
func RolePrivilegesList(credName string, logger *slog.Logger) ([]string, error) {
	url, err := ResolveEndpoint(credName, "/api/v1/api-role-privileges")
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
		return nil, fmt.Errorf("list privileges: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list privileges status %d: %s", resp.StatusCode, string(body))
	}
	var result ApiRolePrivileges
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return result.Privileges, nil
}
