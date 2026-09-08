package buddy

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

// ComputerExtensionAttributeValue is one inventory EA value on a computer.
type ComputerExtensionAttributeValue struct {
	DefinitionID string   `json:"id"` // definitionId from Jamf
	Name         string   `json:"name"`
	InputType    string   `json:"inputType"`
	DataType     string   `json:"dataType"`
	Values       []string `json:"values"`
	Enabled      bool     `json:"enabled,omitempty"`
	MultiValue   bool     `json:"multiValue,omitempty"`
}

type computerInventoryEAJSON struct {
	DefinitionID string   `json:"definitionId"`
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	Enabled      bool     `json:"enabled"`
	MultiValue   bool     `json:"multiValue"`
	Values       []string `json:"values"`
	DataType     string   `json:"dataType"`
	InputType    string   `json:"inputType"`
	Options      []string `json:"options,omitempty"`
}

// ComputerFetchExtensionAttributes returns typed EA values from
// GET /api/v4/computers-inventory/{id}?section=EXTENSION_ATTRIBUTES.
// Optional filter matches name or definition id (case-insensitive substring).
func ComputerFetchExtensionAttributes(credName, computerID, filter string) ([]ComputerExtensionAttributeValue, error) {
	body, err := ComputerFetchInventory(credName, computerID, []string{"EXTENSION_ATTRIBUTES"})
	if err != nil {
		return nil, err
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("parse computer %s: %w", computerID, err)
	}
	raw, ok := doc["extensionAttributes"]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return []ComputerExtensionAttributeValue{}, nil
	}
	var rows []computerInventoryEAJSON
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("parse extensionAttributes: %w", err)
	}
	filter = strings.ToLower(strings.TrimSpace(filter))
	out := make([]ComputerExtensionAttributeValue, 0, len(rows))
	for _, r := range rows {
		if filter != "" {
			name := strings.ToLower(r.Name)
			id := strings.ToLower(r.DefinitionID)
			if !strings.Contains(name, filter) && !strings.Contains(id, filter) {
				continue
			}
		}
		out = append(out, ComputerExtensionAttributeValue{
			DefinitionID: r.DefinitionID,
			Name:         r.Name,
			InputType:    r.InputType,
			DataType:     r.DataType,
			Values:       r.Values,
			Enabled:      r.Enabled,
			MultiValue:   r.MultiValue,
		})
	}
	return out, nil
}

// ComputerExtensionAttributesList prints EA values for a computer.
func ComputerExtensionAttributesList(credName, computerID, filter string, asJSON bool, logger *slog.Logger) error {
	rows, err := ComputerFetchExtensionAttributes(credName, computerID, filter)
	if err != nil {
		return err
	}
	if asJSON {
		return printJSONValue(rows)
	}
	fmt.Printf("%-8s  %-36s  %-10s  %s\n", "EA ID", "Name", "Input", "Values")
	fmt.Printf("%-8s  %-36s  %-10s  %s\n", "--------", "------------------------------------", "----------", "--------------------")
	for _, r := range rows {
		fmt.Printf("%-8s  %-36s  %-10s  %s\n",
			r.DefinitionID, truncatePkg(r.Name, 36), r.InputType, strings.Join(r.Values, ", "))
	}
	fmt.Printf("\n%d extension attribute value(s)\n", len(rows))
	return nil
}

// ComputerSetExtensionAttributeSpec is input for setting a TEXT/POPUP EA value.
type ComputerSetExtensionAttributeSpec struct {
	ComputerID string
	EAID       string // definitionId
	EAName     string // exact name resolve when EAID empty
	Values     []string
}

// ComputerSetExtensionAttribute PATCHes TEXT/POPUP inventory EA values.
// Rejects SCRIPT and LDAP input types.
func ComputerSetExtensionAttribute(credName string, opts Opts, spec ComputerSetExtensionAttributeSpec, logger *slog.Logger) error {
	computerID := strings.TrimSpace(spec.ComputerID)
	if computerID == "" {
		return fmt.Errorf("computer id is required")
	}
	eaID := strings.TrimSpace(spec.EAID)
	eaName := strings.TrimSpace(spec.EAName)
	if eaID == "" && eaName == "" {
		return fmt.Errorf("pass ea id or name")
	}
	if len(spec.Values) == 0 {
		return fmt.Errorf("at least one value is required")
	}

	rows, err := ComputerFetchExtensionAttributes(credName, computerID, "")
	if err != nil {
		return err
	}
	var match *ComputerExtensionAttributeValue
	for i := range rows {
		r := &rows[i]
		if eaID != "" {
			if r.DefinitionID == eaID {
				match = r
				break
			}
			continue
		}
		if strings.EqualFold(r.Name, eaName) {
			if match != nil {
				return fmt.Errorf("multiple extension attributes named %q on computer %s", eaName, computerID)
			}
			match = r
		}
	}
	if match == nil {
		if eaID != "" {
			return fmt.Errorf("extension attribute id %q not found on computer %s", eaID, computerID)
		}
		return fmt.Errorf("extension attribute name %q not found on computer %s", eaName, computerID)
	}
	input := strings.ToUpper(strings.TrimSpace(match.InputType))
	switch input {
	case "TEXT", "POPUP":
	case "SCRIPT", "LDAP":
		return fmt.Errorf("cannot set values for inputType %s (only TEXT/POPUP are writable)", match.InputType)
	default:
		// Jamf sometimes returns lowercase; normalize again
		switch strings.ToUpper(input) {
		case "TEXT", "POPUP":
		default:
			return fmt.Errorf("cannot set values for inputType %q (only TEXT/POPUP are writable)", match.InputType)
		}
	}

	label := match.Name
	if label == "" {
		label = match.DefinitionID
	}
	if !opts.SkipConfirm {
		msg := fmt.Sprintf("Set computer %s EA %s (%s) values → %v", computerID, match.DefinitionID, label, spec.Values)
		if !ConfirmPrompt(msg, logger) {
			return nil
		}
	}

	payload := map[string]any{
		"extensionAttributes": []map[string]any{
			{
				"definitionId": match.DefinitionID,
				"values":       spec.Values,
			},
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := jamfPATCH(credName, uriComputersInventoryDetail+"/"+computerID, data, "application/json"); err != nil {
		return fmt.Errorf("set computer %s extension attribute %s: %w", computerID, match.DefinitionID, err)
	}
	fmt.Printf("Updated computer %s: EA %s (%s) = %s\n", computerID, match.DefinitionID, label, strings.Join(spec.Values, ", "))
	return nil
}
