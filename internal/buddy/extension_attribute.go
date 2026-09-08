package buddy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"mime/multipart"
	"net/url"
	"os"
	"path"
	"strings"
)

const uriComputerExtensionAttributes = "/api/v1/computer-extension-attributes"

// ExtensionAttributeRecord is a summary from the computer EA list endpoint
// (never includes scriptContents).
type ExtensionAttributeRecord struct {
	ID                   string `json:"id"`
	Name                 string `json:"name"`
	Enabled              bool   `json:"enabled"`
	InputType            string `json:"inputType"`
	DataType             string `json:"dataType"`
	InventoryDisplayType string `json:"inventoryDisplayType"`
}

// ExtensionAttributeDetail is the full computer EA record from GET/POST/PUT.
type ExtensionAttributeDetail struct {
	ID                            string   `json:"id,omitempty"`
	Name                          string   `json:"name"`
	Description                   string   `json:"description,omitempty"`
	DataType                      string   `json:"dataType"`
	Enabled                       *bool    `json:"enabled,omitempty"`
	InventoryDisplayType          string   `json:"inventoryDisplayType"`
	InputType                     string   `json:"inputType"`
	ScriptContents                string   `json:"scriptContents,omitempty"`
	PopupMenuChoices              []string `json:"popupMenuChoices,omitempty"`
	LDAPAttributeMapping          string   `json:"ldapAttributeMapping,omitempty"`
	LDAPExtensionAttributeAllowed *bool    `json:"ldapExtensionAttributeAllowed,omitempty"`
	ManageExistingData            string   `json:"manageExistingData,omitempty"` // RETAIN | DELETE
}

// ExtensionAttributeHistoryEntry is one audit history row.
type ExtensionAttributeHistoryEntry struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Date     string `json:"date"`
	Note     string `json:"note,omitempty"`
	Details  string `json:"details,omitempty"`
}

type extensionAttributeHistoryResponse struct {
	TotalCount int                              `json:"totalCount"`
	Results    []ExtensionAttributeHistoryEntry `json:"results"`
}

// ExtensionAttributeDependency is one smart group / advanced search dependent.
type ExtensionAttributeDependency struct {
	ID               int    `json:"id"`
	ObjectID         int    `json:"objectId"`
	NameLocalization string `json:"nameLocalization"`
	IdentifiableName string `json:"identifiableName"`
	Hyperlink        string `json:"hyperlink,omitempty"`
}

// ExtensionAttributeTemplateSummary is a list row from /templates.
type ExtensionAttributeTemplateSummary struct {
	TemplateID           string `json:"templateId"`
	TemplateName         string `json:"templateName"`
	TemplateCategoryName string `json:"templateCategoryName"`
}

// ExtensionAttributeSearch fetches computer EAs via paginated requests and
// returns summary records (scriptContents stripped).
// Filter: bare term → name=="*term*"; RSQL → pass-through.
func ExtensionAttributeSearch(credName, filter string, opts PageOpts, logger *slog.Logger) ([]ExtensionAttributeRecord, error) {
	normalized, err := NormalizeListFilter(filter, FilterSpecExtensionAttributes)
	if err != nil {
		return nil, err
	}
	extra := url.Values{}
	if normalized != "" {
		extra.Set("filter", normalized)
	}
	var all []ExtensionAttributeRecord
	err = fetchJamfPaged(credName, uriComputerExtensionAttributes, extra, opts, func(resultsJSON []byte, _ int) (int, error) {
		var page []ExtensionAttributeDetail
		if err := json.Unmarshal(resultsJSON, &page); err != nil {
			return 0, fmt.Errorf("parse response: %w", err)
		}
		for _, ea := range page {
			all = append(all, extensionAttributeToRecord(ea))
		}
		return len(page), nil
	})
	if err != nil {
		return nil, err
	}
	return all, nil
}

func extensionAttributeToRecord(ea ExtensionAttributeDetail) ExtensionAttributeRecord {
	enabled := true
	if ea.Enabled != nil {
		enabled = *ea.Enabled
	}
	return ExtensionAttributeRecord{
		ID:                   ea.ID,
		Name:                 ea.Name,
		Enabled:              enabled,
		InputType:            ea.InputType,
		DataType:             ea.DataType,
		InventoryDisplayType: ea.InventoryDisplayType,
	}
}

func extensionAttributeSearchRaw(credName, rsqlFilter string, opts PageOpts) ([]json.RawMessage, error) {
	extra := url.Values{}
	if strings.TrimSpace(rsqlFilter) != "" {
		extra.Set("filter", rsqlFilter)
	}
	var all []json.RawMessage
	err := fetchJamfPaged(credName, uriComputerExtensionAttributes, extra, opts, func(resultsJSON []byte, _ int) (int, error) {
		var page []json.RawMessage
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

func extensionAttributeJSONHasContents(raw json.RawMessage) bool {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return false
	}
	_, ok := m["scriptContents"]
	return ok
}

// ExtensionAttributeFetchByName loads one EA by exact name (case-insensitive).
func ExtensionAttributeFetchByName(credName, name string, logger *slog.Logger) (*ExtensionAttributeDetail, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("extension attribute name is required")
	}
	filter, err := RSQLEq("name", name)
	if err != nil {
		return nil, err
	}
	rows, err := extensionAttributeSearchRaw(credName, filter, PageOpts{Page: PageAll})
	if err != nil {
		return nil, err
	}
	type hit struct {
		raw json.RawMessage
		ea  ExtensionAttributeDetail
	}
	var matches []hit
	for _, raw := range rows {
		var ea ExtensionAttributeDetail
		if err := json.Unmarshal(raw, &ea); err != nil {
			return nil, fmt.Errorf("parse response: %w", err)
		}
		if strings.EqualFold(ea.Name, name) {
			matches = append(matches, hit{raw: raw, ea: ea})
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no extension attribute found with name %q", name)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("multiple extension attributes found with name %q", name)
	}
	m := matches[0]
	if strings.EqualFold(m.ea.InputType, "SCRIPT") && !extensionAttributeJSONHasContents(m.raw) {
		if strings.TrimSpace(m.ea.ID) == "" {
			return nil, fmt.Errorf("extension attribute %q has no id", name)
		}
		return extensionAttributeGetFull(credName, m.ea.ID)
	}
	return &m.ea, nil
}

// ExtensionAttributeList prints EAs as a table or JSON summaries.
func ExtensionAttributeList(credName, filter string, asJSON bool, logger *slog.Logger) error {
	rows, err := ExtensionAttributeSearch(credName, filter, PageOpts{Page: PageAll}, logger)
	if err != nil {
		return err
	}
	if asJSON {
		return printJSONValue(rows)
	}
	fmt.Printf("%-8s  %-36s  %-8s  %-12s  %-8s  %s\n", "ID", "Name", "Enabled", "Input", "Data", "Display")
	fmt.Printf("%-8s  %-36s  %-8s  %-12s  %-8s  %s\n",
		"--------", "------------------------------------", "--------", "------------", "--------", "--------------------")
	for _, r := range rows {
		en := "true"
		if !r.Enabled {
			en = "false"
		}
		fmt.Printf("%-8s  %-36s  %-8s  %-12s  %-8s  %s\n",
			r.ID, truncatePkg(r.Name, 36), en, r.InputType, r.DataType, r.InventoryDisplayType)
	}
	fmt.Printf("\n%d extension attribute(s)\n", len(rows))
	return nil
}

func extensionAttributeGetFull(credName, id string) (*ExtensionAttributeDetail, error) {
	body, err := jamfGET(credName, uriComputerExtensionAttributes+"/"+id)
	if err != nil {
		return nil, fmt.Errorf("get extension attribute %s: %w", id, err)
	}
	var ea ExtensionAttributeDetail
	if err := json.Unmarshal(body, &ea); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &ea, nil
}

// ExtensionAttributeFetchFull fetches a computer EA by ID.
func ExtensionAttributeFetchFull(credName, id string) (*ExtensionAttributeDetail, error) {
	return extensionAttributeGetFull(credName, id)
}

// ExtensionAttributeGet pretty-prints one EA by ID.
func ExtensionAttributeGet(credName, id string, logger *slog.Logger) error {
	ea, err := extensionAttributeGetFull(credName, id)
	if err != nil {
		return err
	}
	out, _ := json.MarshalIndent(ea, "", "  ")
	fmt.Println(string(out))
	return nil
}

// ExtensionAttributeGetContents writes scriptContents to stdout.
func ExtensionAttributeGetContents(credName, id string, logger *slog.Logger) error {
	contents, err := ExtensionAttributeGetContentsStr(credName, id)
	if err != nil {
		return err
	}
	fmt.Print(contents)
	return nil
}

// ExtensionAttributeGetContentsStr returns scriptContents; fails if not SCRIPT.
func ExtensionAttributeGetContentsStr(credName, id string) (string, error) {
	ea, err := extensionAttributeGetFull(credName, id)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(ea.InputType, "SCRIPT") {
		return "", fmt.Errorf("extension attribute %s inputType is %q (need SCRIPT)", id, ea.InputType)
	}
	return ea.ScriptContents, nil
}

// ExtensionAttributeGetID resolves a name to its ID (exact, case-insensitive).
func ExtensionAttributeGetID(credName, name string, logger *slog.Logger) (string, error) {
	ea, err := ExtensionAttributeFetchByName(credName, name, logger)
	if err != nil {
		return "", err
	}
	return ea.ID, nil
}

// ExtensionAttributeHistory prints recent audit history.
func ExtensionAttributeHistory(credName, id string, limit int, logger *slog.Logger) error {
	entries, total, err := ExtensionAttributeFetchHistory(credName, id, limit)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Println("No history entries found.")
		return nil
	}
	fmt.Printf("%-30s  %-20s  %s\n", "Date", "User", "Note")
	fmt.Printf("%-30s  %-20s  %s\n", "------------------------------", "--------------------", "--------------------")
	for _, e := range entries {
		note := e.Note
		if note == "" {
			note = e.Details
		}
		fmt.Printf("%-30s  %-20s  %s\n", e.Date, e.Username, note)
	}
	fmt.Printf("\n%d of %d history entries\n", len(entries), total)
	return nil
}

// ExtensionAttributeFetchHistory returns history entries (sorted date:desc).
func ExtensionAttributeFetchHistory(credName, id string, limit int) ([]ExtensionAttributeHistoryEntry, int, error) {
	if limit <= 0 {
		limit = 20
	}
	body, err := jamfGET(credName, fmt.Sprintf("%s/%s/history?sort=date:desc&page-size=%d", uriComputerExtensionAttributes, id, limit))
	if err != nil {
		return nil, 0, fmt.Errorf("get extension attribute history %s: %w", id, err)
	}
	var result extensionAttributeHistoryResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, 0, fmt.Errorf("parse history response: %w", err)
	}
	return result.Results, result.TotalCount, nil
}

// ExtensionAttributeFetchDependencies returns smart group / advanced search dependents.
func ExtensionAttributeFetchDependencies(credName, id string) ([]ExtensionAttributeDependency, error) {
	body, err := jamfGET(credName, uriComputerExtensionAttributes+"/"+id+"/data-dependency")
	if err != nil {
		return nil, fmt.Errorf("get extension attribute dependencies %s: %w", id, err)
	}
	var env struct {
		TotalCount int                            `json:"totalCount"`
		Results    []ExtensionAttributeDependency `json:"results"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("parse dependencies response: %w", err)
	}
	return env.Results, nil
}

// ExtensionAttributeDependencies prints dependents for an EA.
func ExtensionAttributeDependencies(credName, id string, logger *slog.Logger) error {
	deps, err := ExtensionAttributeFetchDependencies(credName, id)
	if err != nil {
		return err
	}
	if len(deps) == 0 {
		fmt.Println("No dependent objects.")
		return nil
	}
	fmt.Printf("%-8s  %-32s  %s\n", "ID", "Type", "Name")
	fmt.Printf("%-8s  %-32s  %s\n", "--------", "--------------------------------", "--------------------")
	for _, d := range deps {
		fmt.Printf("%-8d  %-32s  %s\n", d.ID, d.NameLocalization, d.IdentifiableName)
	}
	fmt.Printf("\n%d dependent object(s)\n", len(deps))
	return nil
}

// ExtensionAttributeTemplateSearch lists EA templates (paginated).
func ExtensionAttributeTemplateSearch(credName, filter string, opts PageOpts, logger *slog.Logger) ([]ExtensionAttributeTemplateSummary, error) {
	normalized, err := NormalizeListFilter(filter, ListFilterSpec{DefaultField: "templateName", Match: ListFilterContains})
	if err != nil {
		return nil, err
	}
	extra := url.Values{}
	if normalized != "" {
		extra.Set("filter", normalized)
	}
	var all []ExtensionAttributeTemplateSummary
	err = fetchJamfPaged(credName, uriComputerExtensionAttributes+"/templates", extra, opts, func(resultsJSON []byte, _ int) (int, error) {
		var page []ExtensionAttributeTemplateSummary
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

// ExtensionAttributeTemplatesList prints templates as a table or JSON.
func ExtensionAttributeTemplatesList(credName, filter string, asJSON bool, logger *slog.Logger) error {
	rows, err := ExtensionAttributeTemplateSearch(credName, filter, PageOpts{Page: PageAll}, logger)
	if err != nil {
		return err
	}
	if asJSON {
		return printJSONValue(rows)
	}
	fmt.Printf("%-8s  %-40s  %s\n", "ID", "Name", "Category")
	fmt.Printf("%-8s  %-40s  %s\n", "--------", "----------------------------------------", "--------------------")
	for _, r := range rows {
		fmt.Printf("%-8s  %-40s  %s\n", r.TemplateID, truncatePkg(r.TemplateName, 40), r.TemplateCategoryName)
	}
	fmt.Printf("\n%d template(s)\n", len(rows))
	return nil
}

// ExtensionAttributeFetchTemplate loads a template by ID (full EA-shaped body).
func ExtensionAttributeFetchTemplate(credName, id string) (*ExtensionAttributeDetail, error) {
	body, err := jamfGET(credName, uriComputerExtensionAttributes+"/templates/"+id)
	if err != nil {
		return nil, fmt.Errorf("get extension attribute template %s: %w", id, err)
	}
	var ea ExtensionAttributeDetail
	if err := json.Unmarshal(body, &ea); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &ea, nil
}

func mergeExtensionAttributeWriteResponse(posted ExtensionAttributeDetail, body []byte, fallbackID string) (*ExtensionAttributeDetail, error) {
	postedJSON, err := json.Marshal(posted)
	if err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	var merged map[string]any
	if err := json.Unmarshal(postedJSON, &merged); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	id := strings.TrimSpace(fallbackID)
	if v, ok := resp["id"]; ok {
		if s := jsonStringID(v); s != "" {
			id = s
		}
	}
	if id == "" {
		return nil, fmt.Errorf("write response missing id: %s", string(body))
	}
	for k, v := range resp {
		if k == "href" {
			continue
		}
		merged[k] = v
	}
	merged["id"] = id
	outJSON, err := json.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	var out ExtensionAttributeDetail
	if err := json.Unmarshal(outJSON, &out); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &out, nil
}

func normalizeExtensionAttributeCreate(ea *ExtensionAttributeDetail) error {
	ea.Name = strings.TrimSpace(ea.Name)
	if ea.Name == "" {
		return fmt.Errorf("extension attribute name is required")
	}
	ea.ID = ""
	ea.InputType = strings.ToUpper(strings.TrimSpace(ea.InputType))
	if ea.InputType == "" {
		ea.InputType = "TEXT"
	}
	switch ea.InputType {
	case "SCRIPT", "TEXT", "POPUP", "DIRECTORY_SERVICE_ATTRIBUTE_MAPPING":
	default:
		return fmt.Errorf("inputType must be SCRIPT, TEXT, POPUP, or DIRECTORY_SERVICE_ATTRIBUTE_MAPPING, got %q", ea.InputType)
	}
	ea.DataType = strings.ToUpper(strings.TrimSpace(ea.DataType))
	if ea.DataType == "" {
		ea.DataType = "STRING"
	}
	switch ea.DataType {
	case "STRING", "INTEGER", "DATE":
	default:
		return fmt.Errorf("dataType must be STRING, INTEGER, or DATE, got %q", ea.DataType)
	}
	ea.InventoryDisplayType = strings.ToUpper(strings.TrimSpace(ea.InventoryDisplayType))
	if ea.InventoryDisplayType == "" {
		ea.InventoryDisplayType = "EXTENSION_ATTRIBUTES"
	}
	switch ea.InventoryDisplayType {
	case "GENERAL", "HARDWARE", "OPERATING_SYSTEM", "USER_AND_LOCATION", "PURCHASING", "EXTENSION_ATTRIBUTES":
	default:
		return fmt.Errorf("inventoryDisplayType must be GENERAL, HARDWARE, OPERATING_SYSTEM, USER_AND_LOCATION, PURCHASING, or EXTENSION_ATTRIBUTES, got %q", ea.InventoryDisplayType)
	}
	if ea.InputType == "SCRIPT" && ea.Enabled == nil {
		t := true
		ea.Enabled = &t
	}
	if ea.InputType == "POPUP" && len(ea.PopupMenuChoices) == 0 {
		return fmt.Errorf("popupMenuChoices required when inputType is POPUP")
	}
	if ea.InputType == "DIRECTORY_SERVICE_ATTRIBUTE_MAPPING" && strings.TrimSpace(ea.LDAPAttributeMapping) == "" {
		return fmt.Errorf("ldapAttributeMapping required when inputType is DIRECTORY_SERVICE_ATTRIBUTE_MAPPING")
	}
	if ea.ManageExistingData != "" {
		ea.ManageExistingData = strings.ToUpper(strings.TrimSpace(ea.ManageExistingData))
		switch ea.ManageExistingData {
		case "RETAIN", "DELETE":
		default:
			return fmt.Errorf("manageExistingData must be RETAIN or DELETE, got %q", ea.ManageExistingData)
		}
	}
	return nil
}

// ExtensionAttributePostDetail POSTs a full EA record and returns the created detail.
func ExtensionAttributePostDetail(credName string, opts Opts, ea ExtensionAttributeDetail, logger *slog.Logger) (*ExtensionAttributeDetail, error) {
	if err := normalizeExtensionAttributeCreate(&ea); err != nil {
		return nil, err
	}
	data, err := json.Marshal(ea)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("Create extension attribute %q", ea.Name), logger) {
			return nil, nil
		}
	}
	body, err := jamfPOST(credName, uriComputerExtensionAttributes, data, "application/json")
	if err != nil {
		return nil, fmt.Errorf("create extension attribute: %w", err)
	}
	created, err := mergeExtensionAttributeWriteResponse(ea, body, "")
	if err != nil {
		return nil, err
	}
	fmt.Printf("Created extension attribute (ID %s): %s\n", created.ID, created.Name)
	return created, nil
}

// ExtensionAttributeCreateSpec holds CLI create flags.
type ExtensionAttributeCreateSpec struct {
	Name                 string
	InputType            string
	DataType             string
	InventoryDisplayType string
	Description          string
	File                 string // SCRIPT body file
	PopupChoices         []string
	LDAPAttributeMapping string
	Enabled              *bool
}

// ExtensionAttributeCreate creates from CLI-oriented fields.
func ExtensionAttributeCreate(credName string, opts Opts, spec ExtensionAttributeCreateSpec, logger *slog.Logger) error {
	ea := ExtensionAttributeDetail{
		Name:                 spec.Name,
		InputType:            spec.InputType,
		DataType:             spec.DataType,
		InventoryDisplayType: spec.InventoryDisplayType,
		Description:          spec.Description,
		PopupMenuChoices:     spec.PopupChoices,
		LDAPAttributeMapping: spec.LDAPAttributeMapping,
		Enabled:              spec.Enabled,
	}
	if strings.TrimSpace(spec.File) != "" {
		contents, err := os.ReadFile(spec.File)
		if err != nil {
			return fmt.Errorf("read script file %q: %w", spec.File, err)
		}
		ea.ScriptContents = string(contents)
		if ea.InputType == "" {
			ea.InputType = "SCRIPT"
		}
	}
	_, err := ExtensionAttributePostDetail(credName, opts, ea, logger)
	return err
}

// ExtensionAttributeCreateFromTemplate POSTs a template body (optionally renamed).
func ExtensionAttributeCreateFromTemplate(credName string, opts Opts, templateID, name string, logger *slog.Logger) (*ExtensionAttributeDetail, error) {
	tmpl, err := ExtensionAttributeFetchTemplate(credName, templateID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(name) != "" {
		tmpl.Name = name
	}
	tmpl.ID = ""
	return ExtensionAttributePostDetail(credName, opts, *tmpl, logger)
}

func extensionAttributePut(credName, id string, ea *ExtensionAttributeDetail) (*ExtensionAttributeDetail, error) {
	data, err := json.Marshal(ea)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	body, err := jamfPUT(credName, uriComputerExtensionAttributes+"/"+id, data, "application/json")
	if err != nil {
		return nil, fmt.Errorf("update extension attribute %s: %w", id, err)
	}
	updated, err := mergeExtensionAttributeWriteResponse(*ea, body, id)
	if err != nil {
		return nil, fmt.Errorf("update extension attribute %s: %w", id, err)
	}
	return updated, nil
}

// ExtensionAttributePutFields fetch-merges field map then PUTs.
func ExtensionAttributePutFields(credName string, opts Opts, id string, fields map[string]interface{}, logger *slog.Logger) (*ExtensionAttributeDetail, error) {
	ea, err := extensionAttributeGetFull(credName, id)
	if err != nil {
		return nil, err
	}
	applyExtensionAttributeFields(ea, fields)
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("Update extension attribute %s (%s) via PUT", id, ea.Name), logger) {
			return nil, nil
		}
	}
	return extensionAttributePut(credName, id, ea)
}

func applyExtensionAttributeFields(ea *ExtensionAttributeDetail, fields map[string]interface{}) {
	if v, ok := fields["name"].(string); ok {
		ea.Name = v
	}
	if v, ok := fields["description"].(string); ok {
		ea.Description = v
	}
	if v, ok := fields["dataType"].(string); ok {
		ea.DataType = strings.ToUpper(v)
	}
	if v, ok := fields["inventoryDisplayType"].(string); ok {
		ea.InventoryDisplayType = strings.ToUpper(v)
	}
	if v, ok := fields["inputType"].(string); ok {
		ea.InputType = strings.ToUpper(v)
	}
	if v, ok := fields["scriptContents"].(string); ok {
		ea.ScriptContents = v
	}
	if v, ok := fields["ldapAttributeMapping"].(string); ok {
		ea.LDAPAttributeMapping = v
	}
	if v, ok := fields["manageExistingData"].(string); ok {
		ea.ManageExistingData = strings.ToUpper(v)
	}
	if v, ok := fields["enabled"].(bool); ok {
		ea.Enabled = &v
	}
	if v, ok := fields["ldapExtensionAttributeAllowed"].(bool); ok {
		ea.LDAPExtensionAttributeAllowed = &v
	}
	if v, ok := fields["popupMenuChoices"]; ok {
		ea.PopupMenuChoices = stringSliceFromAny(v)
	}
}

func stringSliceFromAny(v interface{}) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, item := range t {
			out = append(out, fmt.Sprint(item))
		}
		return out
	default:
		return nil
	}
}

// ExtensionAttributeUpdate replaces SCRIPT body from a local file.
func ExtensionAttributeUpdate(credName string, opts Opts, id, file string, logger *slog.Logger) error {
	contents, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("read script file %q: %w", file, err)
	}
	ea, err := extensionAttributeGetFull(credName, id)
	if err != nil {
		return err
	}
	if !strings.EqualFold(ea.InputType, "SCRIPT") {
		return fmt.Errorf("extension attribute %s inputType is %q (need SCRIPT)", id, ea.InputType)
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("Update extension attribute %s (%s) scriptContents from %q", id, ea.Name, file), logger) {
			return nil
		}
	}
	ea.ScriptContents = string(contents)
	updated, err := extensionAttributePut(credName, id, ea)
	if err != nil {
		return err
	}
	fmt.Printf("Updated extension attribute %s (%s): scriptContents replaced from %q\n", updated.ID, updated.Name, file)
	return nil
}

// ExtensionAttributeDelete deletes one or more EAs by ID.
// Multiple IDs use POST .../delete-multiple when confirmed as a batch.
func ExtensionAttributeDelete(credName string, opts Opts, ids []string, logger *slog.Logger) error {
	if len(ids) == 0 {
		return fmt.Errorf("at least one extension attribute ID required")
	}
	if len(ids) == 1 {
		ea, err := extensionAttributeGetFull(credName, ids[0])
		if err != nil {
			return fmt.Errorf("fetch extension attribute %s: %w", ids[0], err)
		}
		if !opts.SkipConfirm {
			if !ConfirmPrompt(fmt.Sprintf("Delete extension attribute %s (%s)", ids[0], ea.Name), logger) {
				return nil
			}
		}
		return extensionAttributeDeleteOne(credName, ids[0], ea.Name)
	}
	// Fetch names for confirmation.
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		ea, err := extensionAttributeGetFull(credName, id)
		if err != nil {
			return fmt.Errorf("fetch extension attribute %s: %w", id, err)
		}
		names = append(names, fmt.Sprintf("%s (%s)", id, ea.Name))
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("Delete %d extension attributes: %s", len(ids), strings.Join(names, ", ")), logger) {
			return nil
		}
	}
	payload, err := json.Marshal(map[string]any{"ids": ids})
	if err != nil {
		return err
	}
	if _, err := jamfPOST(credName, uriComputerExtensionAttributes+"/delete-multiple", payload, "application/json"); err != nil {
		return fmt.Errorf("delete-multiple extension attributes: %w", err)
	}
	for i, id := range ids {
		fmt.Printf("Deleted extension attribute %s\n", names[i])
		_ = id
	}
	return nil
}

func extensionAttributeDeleteOne(credName, id, name string) error {
	if err := jamfDELETE(credName, uriComputerExtensionAttributes+"/"+id); err != nil {
		return fmt.Errorf("delete extension attribute %s: %w", id, err)
	}
	fmt.Printf("Deleted extension attribute %s (%s)\n", id, name)
	return nil
}

// ExtensionAttributeSetName renames an EA.
func ExtensionAttributeSetName(credName, id string, opts Opts, value string, logger *slog.Logger) error {
	return extensionAttributeSetField(credName, id, opts, "name", value, func(ea *ExtensionAttributeDetail) {
		ea.Name = value
	}, logger)
}

// ExtensionAttributeSetDescription updates description.
func ExtensionAttributeSetDescription(credName, id string, opts Opts, value string, logger *slog.Logger) error {
	return extensionAttributeSetField(credName, id, opts, "description", value, func(ea *ExtensionAttributeDetail) {
		ea.Description = value
	}, logger)
}

// ExtensionAttributeSetDisplay updates inventoryDisplayType.
func ExtensionAttributeSetDisplay(credName, id string, opts Opts, value string, logger *slog.Logger) error {
	value = strings.ToUpper(strings.TrimSpace(value))
	switch value {
	case "GENERAL", "HARDWARE", "OPERATING_SYSTEM", "USER_AND_LOCATION", "PURCHASING", "EXTENSION_ATTRIBUTES":
	default:
		return fmt.Errorf("display must be GENERAL, HARDWARE, OPERATING_SYSTEM, USER_AND_LOCATION, PURCHASING, or EXTENSION_ATTRIBUTES, got %q", value)
	}
	return extensionAttributeSetField(credName, id, opts, "inventoryDisplayType", value, func(ea *ExtensionAttributeDetail) {
		ea.InventoryDisplayType = value
	}, logger)
}

// ExtensionAttributeSetEnabled sets enabled (SCRIPT EAs).
func ExtensionAttributeSetEnabled(credName, id string, opts Opts, value string, logger *slog.Logger) error {
	b, err := parseBoolArg(value)
	if err != nil {
		return err
	}
	return extensionAttributeSetField(credName, id, opts, "enabled", value, func(ea *ExtensionAttributeDetail) {
		ea.Enabled = &b
	}, logger)
}

// ExtensionAttributeSetContents replaces scriptContents from a string value.
func ExtensionAttributeSetContents(credName, id string, opts Opts, value string, logger *slog.Logger) error {
	ea, err := extensionAttributeGetFull(credName, id)
	if err != nil {
		return err
	}
	if !strings.EqualFold(ea.InputType, "SCRIPT") {
		return fmt.Errorf("extension attribute %s inputType is %q (need SCRIPT)", id, ea.InputType)
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("Set extension attribute %s (%s) scriptContents", id, ea.Name), logger) {
			return nil
		}
	}
	ea.ScriptContents = value
	updated, err := extensionAttributePut(credName, id, ea)
	if err != nil {
		return err
	}
	fmt.Printf("Updated extension attribute %s: scriptContents set\n", updated.ID)
	return nil
}

func extensionAttributeSetField(credName, id string, opts Opts, label, value string, apply func(*ExtensionAttributeDetail), logger *slog.Logger) error {
	ea, err := extensionAttributeGetFull(credName, id)
	if err != nil {
		return err
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("Set extension attribute %s (%s) %s → %q", id, ea.Name, label, value), logger) {
			return nil
		}
	}
	apply(ea)
	updated, err := extensionAttributePut(credName, id, ea)
	if err != nil {
		return err
	}
	fmt.Printf("Updated extension attribute %s: %s set\n", updated.ID, label)
	return nil
}

func parseBoolArg(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "1", "yes", "y", "on":
		return true, nil
	case "false", "0", "no", "n", "off":
		return false, nil
	default:
		return false, fmt.Errorf("boolean value required (true/false), got %q", s)
	}
}

// ExtensionAttributeUploadXML POSTs a Jamf Nation XML file via multipart upload.
func ExtensionAttributeUploadXML(credName string, opts Opts, filePath string, logger *slog.Logger) (*ExtensionAttributeDetail, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", filePath, err)
	}
	filename := path.Base(filePath)
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("Upload extension attribute XML from %q", filePath), logger) {
			return nil, nil
		}
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return nil, fmt.Errorf("write file to form: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("close multipart: %w", err)
	}
	body, err := jamfPOST(credName, uriComputerExtensionAttributes+"/upload", buf.Bytes(), w.FormDataContentType())
	if err != nil {
		return nil, fmt.Errorf("upload extension attribute: %w", err)
	}
	var ea ExtensionAttributeDetail
	if err := json.Unmarshal(body, &ea); err != nil {
		return nil, fmt.Errorf("parse upload response: %w", err)
	}
	fmt.Printf("Uploaded extension attribute (ID %s): %s\n", ea.ID, ea.Name)
	return &ea, nil
}

// ExtensionAttributeDownloadXML downloads the Classic-style XML export.
func ExtensionAttributeDownloadXML(credName, id, outfile string, logger *slog.Logger) error {
	ea, err := extensionAttributeGetFull(credName, id)
	if err != nil {
		return err
	}
	body, err := jamfGETWithAccept(credName, uriComputerExtensionAttributes+"/"+id+"/download", "application/xml")
	if err != nil {
		return fmt.Errorf("download extension attribute %s: %w", id, err)
	}
	if outfile == "" {
		outfile = extensionAttributeFilename(ea.Name) + ".xml"
	}
	if err := os.WriteFile(outfile, body, 0644); err != nil {
		return fmt.Errorf("write %q: %w", outfile, err)
	}
	fmt.Printf("Downloaded extension attribute %s (%s) → %s\n", ea.ID, ea.Name, outfile)
	return nil
}

func extensionAttributeFilename(name string) string {
	var b strings.Builder
	prevHyphen := false
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevHyphen = false
		} else if !prevHyphen {
			b.WriteRune('-')
			prevHyphen = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		slug = "extension-attribute"
	}
	return slug
}

// BoolPtr returns a *bool for EA enabled/ldap fields.
func BoolPtr(v bool) *bool { return &v }

// ParsePopupChoices splits comma-separated popup choices.
func ParsePopupChoices(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
