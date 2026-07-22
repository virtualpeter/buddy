package buddy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const (
	uriBlueprints          = "/api/blueprints/v1/blueprints"
	uriBlueprintComponents = "/api/blueprints/v1/blueprint-components"
)

// requirePlatformCred returns an error unless the credential uses Platform API Gateway mode.
func requirePlatformCred(credName string) error {
	ok, err := IsPlatformCred(credName)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("blueprints require a Platform API Gateway credential (set tenantId on the credential); see 'buddy cred help'")
	}
	return nil
}

// BlueprintOverview is a list-item summary from the Blueprints API.
type BlueprintOverview struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Created     string `json:"created,omitempty"`
	Updated     string `json:"updated,omitempty"`
}

// BlueprintListOpts controls blueprint list filtering and pagination.
type BlueprintListOpts struct {
	Search string
	Sort   string
	Page   PageOpts
}

// BlueprintListValues lists blueprints (all pages by default).
func BlueprintListValues(credName string, listOpts BlueprintListOpts, logger *slog.Logger) ([]BlueprintOverview, error) {
	if err := requirePlatformCred(credName); err != nil {
		return nil, err
	}
	extra := url.Values{}
	if listOpts.Search != "" {
		extra.Set("search", listOpts.Search)
	}
	if listOpts.Sort != "" {
		extra.Set("sort", listOpts.Sort)
	} else {
		extra.Set("sort", "created:desc")
	}
	var all []BlueprintOverview
	err := fetchJamfPaged(credName, uriBlueprints, extra, listOpts.Page, func(resultsJSON []byte, _ int) (int, error) {
		var page []BlueprintOverview
		if err := json.Unmarshal(resultsJSON, &page); err != nil {
			return 0, fmt.Errorf("parse blueprints: %w", err)
		}
		all = append(all, page...)
		return len(page), nil
	})
	if err != nil {
		return nil, err
	}
	return all, nil
}

// BlueprintList prints a table of blueprints.
func BlueprintList(credName string, listOpts BlueprintListOpts, logger *slog.Logger) error {
	items, err := BlueprintListValues(credName, listOpts, logger)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		fmt.Println("No blueprints found.")
		return nil
	}
	fmt.Printf("%-38s  %-32s  %s\n", "ID", "Name", "Updated")
	fmt.Printf("%-38s  %-32s  %s\n", strings.Repeat("-", 38), strings.Repeat("-", 32), strings.Repeat("-", 20))
	for _, b := range items {
		name := b.Name
		if len(name) > 32 {
			name = name[:29] + "..."
		}
		fmt.Printf("%-38s  %-32s  %s\n", b.ID, name, b.Updated)
	}
	return nil
}

// BlueprintGetValue fetches a single blueprint as a generic JSON object.
func BlueprintGetValue(credName, blueprintID string, logger *slog.Logger) (map[string]interface{}, error) {
	if err := requirePlatformCred(credName); err != nil {
		return nil, err
	}
	if strings.TrimSpace(blueprintID) == "" {
		return nil, fmt.Errorf("blueprint-id is required")
	}
	body, err := jamfGET(credName, uriBlueprints+"/"+url.PathEscape(blueprintID))
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("parse blueprint: %w", err)
	}
	return out, nil
}

// BlueprintGet prints a blueprint as indented JSON.
func BlueprintGet(credName, blueprintID string, logger *slog.Logger) error {
	out, err := BlueprintGetValue(credName, blueprintID, logger)
	if err != nil {
		return err
	}
	enc, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(enc))
	return nil
}

// BlueprintCreateValue creates a blueprint from a JSON body and returns the response object.
func BlueprintCreateValue(credName string, opts Opts, bodyJSON []byte, logger *slog.Logger) (map[string]interface{}, error) {
	if err := requirePlatformCred(credName); err != nil {
		return nil, err
	}
	if len(bodyJSON) == 0 {
		return nil, fmt.Errorf("request body is required")
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt("Create blueprint?", logger) {
			return nil, nil
		}
	}
	respBody, err := jamfPOST(credName, uriBlueprints, bodyJSON, "application/json")
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &out); err != nil {
			return map[string]interface{}{"raw": string(respBody)}, nil
		}
	}
	if out == nil {
		out = map[string]interface{}{}
	}
	return out, nil
}

// BlueprintCreate creates a blueprint and prints the response.
func BlueprintCreate(credName string, opts Opts, bodyJSON []byte, logger *slog.Logger) error {
	out, err := BlueprintCreateValue(credName, opts, bodyJSON, logger)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	enc, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(enc))
	return nil
}

// BlueprintSetValue PATCHes a blueprint (merge-patch+json).
func BlueprintSetValue(credName, blueprintID string, opts Opts, bodyJSON []byte, logger *slog.Logger) error {
	if err := requirePlatformCred(credName); err != nil {
		return err
	}
	if strings.TrimSpace(blueprintID) == "" {
		return fmt.Errorf("blueprint-id is required")
	}
	if len(bodyJSON) == 0 {
		return fmt.Errorf("request body is required")
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("Update blueprint %s?", blueprintID), logger) {
			return nil
		}
	}
	endpoint, err := ResolveEndpoint(credName, uriBlueprints+"/"+url.PathEscape(blueprintID))
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPatch, endpoint, bytes.NewReader(bodyJSON))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/merge-patch+json")
	req.Header.Set("Accept", "application/json")
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("PATCH blueprint %s status %d: %s", blueprintID, resp.StatusCode, string(respBody))
	}
	return nil
}

// BlueprintSet PATCHes a blueprint.
func BlueprintSet(credName, blueprintID string, opts Opts, bodyJSON []byte, logger *slog.Logger) error {
	return BlueprintSetValue(credName, blueprintID, opts, bodyJSON, logger)
}

// BlueprintDeleteValue deletes a blueprint.
func BlueprintDeleteValue(credName, blueprintID string, opts Opts, logger *slog.Logger) error {
	if err := requirePlatformCred(credName); err != nil {
		return err
	}
	if strings.TrimSpace(blueprintID) == "" {
		return fmt.Errorf("blueprint-id is required")
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("Delete blueprint %s?", blueprintID), logger) {
			return nil
		}
	}
	return jamfDELETE(credName, uriBlueprints+"/"+url.PathEscape(blueprintID))
}

// BlueprintDelete deletes a blueprint.
func BlueprintDelete(credName, blueprintID string, opts Opts, logger *slog.Logger) error {
	return BlueprintDeleteValue(credName, blueprintID, opts, logger)
}

func blueprintPostAction(credName, blueprintID, action string, opts Opts, logger *slog.Logger) error {
	if err := requirePlatformCred(credName); err != nil {
		return err
	}
	if strings.TrimSpace(blueprintID) == "" {
		return fmt.Errorf("blueprint-id is required")
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("%s blueprint %s?", action, blueprintID), logger) {
			return nil
		}
	}
	path := uriBlueprints + "/" + url.PathEscape(blueprintID) + "/" + action
	_, err := jamfPOST(credName, path, nil, "application/json")
	return err
}

// BlueprintDeploy deploys a blueprint.
func BlueprintDeploy(credName, blueprintID string, opts Opts, logger *slog.Logger) error {
	return blueprintPostAction(credName, blueprintID, "deploy", opts, logger)
}

// BlueprintUndeploy undeploys a blueprint.
func BlueprintUndeploy(credName, blueprintID string, opts Opts, logger *slog.Logger) error {
	return blueprintPostAction(credName, blueprintID, "undeploy", opts, logger)
}

// BlueprintStatusValue fetches the deployment status report for a blueprint.
func BlueprintStatusValue(credName, blueprintID string, logger *slog.Logger) (map[string]interface{}, error) {
	if err := requirePlatformCred(credName); err != nil {
		return nil, err
	}
	if strings.TrimSpace(blueprintID) == "" {
		return nil, fmt.Errorf("blueprint-id is required")
	}
	body, err := jamfGET(credName, uriBlueprints+"/"+url.PathEscape(blueprintID)+"/report")
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("parse blueprint status: %w", err)
	}
	return out, nil
}

// BlueprintStatus prints the blueprint status report.
func BlueprintStatus(credName, blueprintID string, logger *slog.Logger) error {
	out, err := BlueprintStatusValue(credName, blueprintID, logger)
	if err != nil {
		return err
	}
	enc, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(enc))
	return nil
}

// BlueprintComponentsValues lists available blueprint components.
func BlueprintComponentsValues(credName string, pageOpts PageOpts, logger *slog.Logger) ([]map[string]interface{}, error) {
	if err := requirePlatformCred(credName); err != nil {
		return nil, err
	}
	var all []map[string]interface{}
	err := fetchJamfPaged(credName, uriBlueprintComponents, nil, pageOpts, func(resultsJSON []byte, _ int) (int, error) {
		var page []map[string]interface{}
		if err := json.Unmarshal(resultsJSON, &page); err != nil {
			return 0, fmt.Errorf("parse blueprint components: %w", err)
		}
		all = append(all, page...)
		return len(page), nil
	})
	if err != nil {
		return nil, err
	}
	return all, nil
}

// BlueprintComponentValue fetches one component by identifier.
func BlueprintComponentValue(credName, identifier string, logger *slog.Logger) (map[string]interface{}, error) {
	if err := requirePlatformCred(credName); err != nil {
		return nil, err
	}
	if strings.TrimSpace(identifier) == "" {
		return nil, fmt.Errorf("component identifier is required")
	}
	body, err := jamfGET(credName, uriBlueprintComponents+"/"+url.PathEscape(identifier))
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("parse blueprint component: %w", err)
	}
	return out, nil
}

// BlueprintComponents prints components list or a single component.
func BlueprintComponents(credName string, identifier string, logger *slog.Logger) error {
	if identifier != "" {
		out, err := BlueprintComponentValue(credName, identifier, logger)
		if err != nil {
			return err
		}
		enc, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(enc))
		return nil
	}
	items, err := BlueprintComponentsValues(credName, PageOpts{Page: PageAll}, logger)
	if err != nil {
		return err
	}
	enc, _ := json.MarshalIndent(items, "", "  ")
	fmt.Println(string(enc))
	return nil
}

// ReadJSONBodyFile reads JSON from a file path or "-" for stdin.
func ReadJSONBodyFile(path string) ([]byte, error) {
	if path == "" || path == "-" {
		return io.ReadAll(os.Stdin)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("%s is not valid JSON", path)
	}
	return data, nil
}
