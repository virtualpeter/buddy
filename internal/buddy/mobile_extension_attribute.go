package buddy

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
)

const uriMobileExtensionAttributes = "/api/v1/mobile-device-extension-attributes"

// MobileExtensionAttributeRecord is a summary from the mobile EA list endpoint.
type MobileExtensionAttributeRecord struct {
	ID                   string `json:"id"`
	Name                 string `json:"name"`
	InputType            string `json:"inputType"`
	DataType             string `json:"dataType"`
	InventoryDisplayType string `json:"inventoryDisplayType"`
}

// MobileExtensionAttributeDetail is the full mobile EA record.
// No SCRIPT input type (TEXT | POPUP | DIRECTORY_SERVICE_ATTRIBUTE_MAPPING).
type MobileExtensionAttributeDetail struct {
	ID                            string   `json:"id,omitempty"`
	Name                          string   `json:"name"`
	Description                   string   `json:"description,omitempty"`
	DataType                      string   `json:"dataType"`
	InventoryDisplayType          string   `json:"inventoryDisplayType"`
	InputType                     string   `json:"inputType"`
	PopupMenuChoices              []string `json:"popupMenuChoices,omitempty"`
	LDAPAttributeMapping          string   `json:"ldapAttributeMapping,omitempty"`
	LDAPExtensionAttributeAllowed *bool    `json:"ldapExtensionAttributeAllowed,omitempty"`
}

// MobileExtensionAttributeSearch lists mobile EAs (paginated).
func MobileExtensionAttributeSearch(credName, filter string, opts PageOpts, logger *slog.Logger) ([]MobileExtensionAttributeRecord, error) {
	normalized, err := NormalizeListFilter(filter, FilterSpecExtensionAttributes)
	if err != nil {
		return nil, err
	}
	extra := url.Values{}
	if normalized != "" {
		extra.Set("filter", normalized)
	}
	var all []MobileExtensionAttributeRecord
	err = fetchJamfPaged(credName, uriMobileExtensionAttributes, extra, opts, func(resultsJSON []byte, _ int) (int, error) {
		var page []MobileExtensionAttributeDetail
		if err := json.Unmarshal(resultsJSON, &page); err != nil {
			return 0, fmt.Errorf("parse response: %w", err)
		}
		for _, ea := range page {
			all = append(all, MobileExtensionAttributeRecord{
				ID:                   ea.ID,
				Name:                 ea.Name,
				InputType:            ea.InputType,
				DataType:             ea.DataType,
				InventoryDisplayType: ea.InventoryDisplayType,
			})
		}
		return len(page), nil
	})
	if err != nil {
		return nil, err
	}
	return all, nil
}

func mobileExtensionAttributeSearchRaw(credName, rsqlFilter string, opts PageOpts) ([]json.RawMessage, error) {
	extra := url.Values{}
	if strings.TrimSpace(rsqlFilter) != "" {
		extra.Set("filter", rsqlFilter)
	}
	var all []json.RawMessage
	err := fetchJamfPaged(credName, uriMobileExtensionAttributes, extra, opts, func(resultsJSON []byte, _ int) (int, error) {
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

// MobileExtensionAttributeList prints mobile EAs as table or JSON.
func MobileExtensionAttributeList(credName, filter string, asJSON bool, logger *slog.Logger) error {
	rows, err := MobileExtensionAttributeSearch(credName, filter, PageOpts{Page: PageAll}, logger)
	if err != nil {
		return err
	}
	if asJSON {
		return printJSONValue(rows)
	}
	fmt.Printf("%-8s  %-40s  %-12s  %-8s  %s\n", "ID", "Name", "Input", "Data", "Display")
	fmt.Printf("%-8s  %-40s  %-12s  %-8s  %s\n",
		"--------", "----------------------------------------", "------------", "--------", "--------------------")
	for _, r := range rows {
		fmt.Printf("%-8s  %-40s  %-12s  %-8s  %s\n",
			r.ID, truncatePkg(r.Name, 40), r.InputType, r.DataType, r.InventoryDisplayType)
	}
	fmt.Printf("\n%d mobile extension attribute(s)\n", len(rows))
	return nil
}

func mobileExtensionAttributeGetFull(credName, id string) (*MobileExtensionAttributeDetail, error) {
	body, err := jamfGET(credName, uriMobileExtensionAttributes+"/"+id)
	if err != nil {
		return nil, fmt.Errorf("get mobile extension attribute %s: %w", id, err)
	}
	var ea MobileExtensionAttributeDetail
	if err := json.Unmarshal(body, &ea); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &ea, nil
}

// MobileExtensionAttributeFetchFull fetches a mobile EA by ID.
func MobileExtensionAttributeFetchFull(credName, id string) (*MobileExtensionAttributeDetail, error) {
	return mobileExtensionAttributeGetFull(credName, id)
}

// MobileExtensionAttributeFetchByName loads one mobile EA by exact name.
func MobileExtensionAttributeFetchByName(credName, name string, logger *slog.Logger) (*MobileExtensionAttributeDetail, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("mobile extension attribute name is required")
	}
	filter, err := RSQLEq("name", name)
	if err != nil {
		return nil, err
	}
	rows, err := mobileExtensionAttributeSearchRaw(credName, filter, PageOpts{Page: PageAll})
	if err != nil {
		return nil, err
	}
	var matches []MobileExtensionAttributeDetail
	for _, raw := range rows {
		var ea MobileExtensionAttributeDetail
		if err := json.Unmarshal(raw, &ea); err != nil {
			return nil, fmt.Errorf("parse response: %w", err)
		}
		if strings.EqualFold(ea.Name, name) {
			matches = append(matches, ea)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no mobile extension attribute found with name %q", name)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("multiple mobile extension attributes found with name %q", name)
	}
	return &matches[0], nil
}

// MobileExtensionAttributeGet pretty-prints one mobile EA.
func MobileExtensionAttributeGet(credName, id string, logger *slog.Logger) error {
	ea, err := mobileExtensionAttributeGetFull(credName, id)
	if err != nil {
		return err
	}
	out, _ := json.MarshalIndent(ea, "", "  ")
	fmt.Println(string(out))
	return nil
}

// MobileExtensionAttributeGetID resolves name → id.
func MobileExtensionAttributeGetID(credName, name string, logger *slog.Logger) (string, error) {
	ea, err := MobileExtensionAttributeFetchByName(credName, name, logger)
	if err != nil {
		return "", err
	}
	return ea.ID, nil
}

func normalizeMobileExtensionAttributeCreate(ea *MobileExtensionAttributeDetail) error {
	ea.Name = strings.TrimSpace(ea.Name)
	if ea.Name == "" {
		return fmt.Errorf("mobile extension attribute name is required")
	}
	ea.ID = ""
	ea.InputType = strings.ToUpper(strings.TrimSpace(ea.InputType))
	if ea.InputType == "" {
		ea.InputType = "TEXT"
	}
	switch ea.InputType {
	case "TEXT", "POPUP", "DIRECTORY_SERVICE_ATTRIBUTE_MAPPING":
	case "SCRIPT":
		return fmt.Errorf("mobile extension attributes do not support SCRIPT inputType")
	default:
		return fmt.Errorf("inputType must be TEXT, POPUP, or DIRECTORY_SERVICE_ATTRIBUTE_MAPPING, got %q", ea.InputType)
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
	case "GENERAL", "HARDWARE", "USER_AND_LOCATION", "PURCHASING", "EXTENSION_ATTRIBUTES":
	default:
		return fmt.Errorf("inventoryDisplayType must be GENERAL, HARDWARE, USER_AND_LOCATION, PURCHASING, or EXTENSION_ATTRIBUTES, got %q", ea.InventoryDisplayType)
	}
	if ea.InputType == "POPUP" && len(ea.PopupMenuChoices) == 0 {
		return fmt.Errorf("popupMenuChoices required when inputType is POPUP")
	}
	if ea.InputType == "DIRECTORY_SERVICE_ATTRIBUTE_MAPPING" && strings.TrimSpace(ea.LDAPAttributeMapping) == "" {
		return fmt.Errorf("ldapAttributeMapping required when inputType is DIRECTORY_SERVICE_ATTRIBUTE_MAPPING")
	}
	return nil
}

func mergeMobileExtensionAttributeWriteResponse(posted MobileExtensionAttributeDetail, body []byte, fallbackID string) (*MobileExtensionAttributeDetail, error) {
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
	var out MobileExtensionAttributeDetail
	if err := json.Unmarshal(outJSON, &out); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &out, nil
}

// MobileExtensionAttributePostDetail POSTs a mobile EA.
func MobileExtensionAttributePostDetail(credName string, opts Opts, ea MobileExtensionAttributeDetail, logger *slog.Logger) (*MobileExtensionAttributeDetail, error) {
	if err := normalizeMobileExtensionAttributeCreate(&ea); err != nil {
		return nil, err
	}
	data, err := json.Marshal(ea)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("Create mobile extension attribute %q", ea.Name), logger) {
			return nil, nil
		}
	}
	body, err := jamfPOST(credName, uriMobileExtensionAttributes, data, "application/json")
	if err != nil {
		return nil, fmt.Errorf("create mobile extension attribute: %w", err)
	}
	created, err := mergeMobileExtensionAttributeWriteResponse(ea, body, "")
	if err != nil {
		return nil, err
	}
	fmt.Printf("Created mobile extension attribute (ID %s): %s\n", created.ID, created.Name)
	return created, nil
}

// MobileExtensionAttributeCreateSpec holds CLI create flags.
type MobileExtensionAttributeCreateSpec struct {
	Name                 string
	InputType            string
	DataType             string
	InventoryDisplayType string
	Description          string
	PopupChoices         []string
	LDAPAttributeMapping string
}

// MobileExtensionAttributeCreate creates from CLI fields.
func MobileExtensionAttributeCreate(credName string, opts Opts, spec MobileExtensionAttributeCreateSpec, logger *slog.Logger) error {
	ea := MobileExtensionAttributeDetail{
		Name:                 spec.Name,
		InputType:            spec.InputType,
		DataType:             spec.DataType,
		InventoryDisplayType: spec.InventoryDisplayType,
		Description:          spec.Description,
		PopupMenuChoices:     spec.PopupChoices,
		LDAPAttributeMapping: spec.LDAPAttributeMapping,
	}
	_, err := MobileExtensionAttributePostDetail(credName, opts, ea, logger)
	return err
}

func mobileExtensionAttributePut(credName, id string, ea *MobileExtensionAttributeDetail) (*MobileExtensionAttributeDetail, error) {
	data, err := json.Marshal(ea)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	body, err := jamfPUT(credName, uriMobileExtensionAttributes+"/"+id, data, "application/json")
	if err != nil {
		return nil, fmt.Errorf("update mobile extension attribute %s: %w", id, err)
	}
	return mergeMobileExtensionAttributeWriteResponse(*ea, body, id)
}

// MobileExtensionAttributePutFields fetch-merges then PUTs.
func MobileExtensionAttributePutFields(credName string, opts Opts, id string, fields map[string]interface{}, logger *slog.Logger) (*MobileExtensionAttributeDetail, error) {
	ea, err := mobileExtensionAttributeGetFull(credName, id)
	if err != nil {
		return nil, err
	}
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
	if v, ok := fields["ldapAttributeMapping"].(string); ok {
		ea.LDAPAttributeMapping = v
	}
	if v, ok := fields["ldapExtensionAttributeAllowed"].(bool); ok {
		ea.LDAPExtensionAttributeAllowed = &v
	}
	if v, ok := fields["popupMenuChoices"]; ok {
		ea.PopupMenuChoices = stringSliceFromAny(v)
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("Update mobile extension attribute %s (%s) via PUT", id, ea.Name), logger) {
			return nil, nil
		}
	}
	return mobileExtensionAttributePut(credName, id, ea)
}

// MobileExtensionAttributeDelete deletes one or more mobile EAs.
func MobileExtensionAttributeDelete(credName string, opts Opts, ids []string, logger *slog.Logger) error {
	if len(ids) == 0 {
		return fmt.Errorf("at least one mobile extension attribute ID required")
	}
	for _, id := range ids {
		ea, err := mobileExtensionAttributeGetFull(credName, id)
		if err != nil {
			return fmt.Errorf("fetch mobile extension attribute %s: %w", id, err)
		}
		if !opts.SkipConfirm {
			if !ConfirmPrompt(fmt.Sprintf("Delete mobile extension attribute %s (%s)", id, ea.Name), logger) {
				continue
			}
		}
		if err := jamfDELETE(credName, uriMobileExtensionAttributes+"/"+id); err != nil {
			return fmt.Errorf("delete mobile extension attribute %s: %w", id, err)
		}
		fmt.Printf("Deleted mobile extension attribute %s (%s)\n", id, ea.Name)
	}
	return nil
}

// PrepareMobileExtensionAttributeForCreate strips id and sets name.
func PrepareMobileExtensionAttributeForCreate(src *MobileExtensionAttributeDetail, name string) (MobileExtensionAttributeDetail, error) {
	if src == nil {
		return MobileExtensionAttributeDetail{}, fmt.Errorf("source mobile extension attribute is required")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return MobileExtensionAttributeDetail{}, fmt.Errorf("mobile extension attribute name is required")
	}
	out := *src
	out.ID = ""
	out.Name = name
	return out, nil
}

// MobileExtensionAttributeCopySpec is input for copy.
type MobileExtensionAttributeCopySpec struct {
	SourceID string
	Name     string
}

// MobileExtensionAttributeMigrateSpec is input for migrate.
type MobileExtensionAttributeMigrateSpec struct {
	SourceID string
	Name     string
}

// MobileExtensionAttributeCreateOutcome is returned after create/copy.
type MobileExtensionAttributeCreateOutcome struct {
	EA MobileExtensionAttributeDetail
}

// MobileExtensionAttributeMigrateOutcome is returned after migrate.
type MobileExtensionAttributeMigrateOutcome struct {
	EA      MobileExtensionAttributeDetail
	Created bool
}

// MobileExtensionAttributeCopy copies a mobile EA across sessions.
func MobileExtensionAttributeCopy(src, dst SessionRef, opts Opts, spec MobileExtensionAttributeCopySpec, logger *slog.Logger) (*MobileExtensionAttributeCreateOutcome, error) {
	src, err := ResolveSessionRef(src, OpRead)
	if err != nil {
		return nil, err
	}
	dst, err = ResolveSessionRef(dst, OpMutate)
	if err != nil {
		return nil, err
	}
	var detail *MobileExtensionAttributeDetail
	err = WithSessionRef(src, func(pin string) error {
		var fetchErr error
		detail, fetchErr = MobileExtensionAttributeFetchFull(pin, spec.SourceID)
		return fetchErr
	})
	if err != nil {
		return nil, err
	}
	var outcome *MobileExtensionAttributeCreateOutcome
	err = WithSessionRef(dst, func(pin string) error {
		prepared, err := PrepareMobileExtensionAttributeForCreate(detail, spec.Name)
		if err != nil {
			return err
		}
		if !opts.SkipConfirm {
			msg := fmt.Sprintf("\nCopy mobile extension attribute (ID %s) as %q to destination?", spec.SourceID, prepared.Name)
			if !ConfirmPrompt(msg, logger) {
				return nil
			}
		}
		optsCopy := opts
		optsCopy.SkipConfirm = true
		created, err := MobileExtensionAttributePostDetail(pin, optsCopy, prepared, logger)
		if err != nil {
			return err
		}
		if created == nil {
			return nil
		}
		outcome = &MobileExtensionAttributeCreateOutcome{EA: *created}
		return nil
	})
	return outcome, err
}

// MobileExtensionAttributeMigrate upserts by exact name.
func MobileExtensionAttributeMigrate(src, dst SessionRef, opts Opts, spec MobileExtensionAttributeMigrateSpec, logger *slog.Logger) (*MobileExtensionAttributeMigrateOutcome, error) {
	src, err := ResolveSessionRef(src, OpRead)
	if err != nil {
		return nil, err
	}
	dst, err = ResolveSessionRef(dst, OpMutate)
	if err != nil {
		return nil, err
	}
	var detail *MobileExtensionAttributeDetail
	err = WithSessionRef(src, func(pin string) error {
		var fetchErr error
		detail, fetchErr = MobileExtensionAttributeFetchFull(pin, spec.SourceID)
		return fetchErr
	})
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		name = detail.Name
	}
	var outcome *MobileExtensionAttributeMigrateOutcome
	err = WithSessionRef(dst, func(pin string) error {
		prepared, err := PrepareMobileExtensionAttributeForCreate(detail, name)
		if err != nil {
			return err
		}
		if !opts.SkipConfirm {
			msg := fmt.Sprintf("\nMigrate mobile extension attribute (ID %s) as %q to destination (upsert by name)?", spec.SourceID, prepared.Name)
			if !ConfirmPrompt(msg, logger) {
				return nil
			}
		}
		optsCopy := opts
		optsCopy.SkipConfirm = true
		destID, findErr := MobileExtensionAttributeGetID(pin, prepared.Name, logger)
		if findErr == nil && destID != "" {
			prepared.ID = destID
			updated, err := mobileExtensionAttributePut(pin, destID, &prepared)
			if err != nil {
				return err
			}
			fmt.Printf("Updated mobile extension attribute (ID %s): %s\n", updated.ID, updated.Name)
			outcome = &MobileExtensionAttributeMigrateOutcome{EA: *updated, Created: false}
			return nil
		}
		created, err := MobileExtensionAttributePostDetail(pin, optsCopy, prepared, logger)
		if err != nil {
			return err
		}
		if created == nil {
			return nil
		}
		outcome = &MobileExtensionAttributeMigrateOutcome{EA: *created, Created: true}
		return nil
	})
	return outcome, err
}
