package buddy

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
)

const uriDeviceGroups = "/api/device-groups/v1/device-groups"

// DeviceGroupOverview is a list-item summary from the Platform device-groups API.
type DeviceGroupOverview struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	GroupType   string `json:"groupType,omitempty"`
	DeviceType  string `json:"deviceType,omitempty"`
}

// DeviceGroupListOpts controls device-group list filtering and pagination.
type DeviceGroupListOpts struct {
	Filter string
	Page   PageOpts
}

// DeviceGroupListValues lists Platform device groups (all pages by default).
func DeviceGroupListValues(credName string, listOpts DeviceGroupListOpts, logger *slog.Logger) ([]DeviceGroupOverview, error) {
	if err := requirePlatformCred(credName); err != nil {
		return nil, err
	}
	extra := url.Values{}
	if listOpts.Filter != "" {
		extra.Set("filter", listOpts.Filter)
	}
	var all []DeviceGroupOverview
	err := fetchJamfPaged(credName, uriDeviceGroups, extra, listOpts.Page, func(resultsJSON []byte, _ int) (int, error) {
		var page []DeviceGroupOverview
		if err := json.Unmarshal(resultsJSON, &page); err != nil {
			return 0, fmt.Errorf("parse device-groups: %w", err)
		}
		all = append(all, page...)
		return len(page), nil
	})
	if err != nil {
		return nil, err
	}
	return all, nil
}

// DeviceGroupList prints a table of device groups.
func DeviceGroupList(credName string, listOpts DeviceGroupListOpts, logger *slog.Logger) error {
	items, err := DeviceGroupListValues(credName, listOpts, logger)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		fmt.Println("No device groups found.")
		return nil
	}
	fmt.Printf("%-38s  %-28s  %-10s  %s\n", "ID", "Name", "Type", "Device")
	fmt.Printf("%-38s  %-28s  %-10s  %s\n", strings.Repeat("-", 38), strings.Repeat("-", 28), strings.Repeat("-", 10), strings.Repeat("-", 10))
	for _, g := range items {
		name := g.Name
		if len(name) > 28 {
			name = name[:25] + "..."
		}
		fmt.Printf("%-38s  %-28s  %-10s  %s\n", g.ID, name, g.GroupType, g.DeviceType)
	}
	return nil
}

// DeviceGroupGetValue fetches one device group as a JSON object.
func DeviceGroupGetValue(credName, groupID string, logger *slog.Logger) (map[string]interface{}, error) {
	if err := requirePlatformCred(credName); err != nil {
		return nil, err
	}
	if strings.TrimSpace(groupID) == "" {
		return nil, fmt.Errorf("group-id is required")
	}
	body, err := jamfGET(credName, uriDeviceGroups+"/"+url.PathEscape(groupID))
	if err != nil {
		return nil, err
	}
	out, err := decodeJSONObject(body)
	if err != nil {
		return nil, fmt.Errorf("parse device-group: %w", err)
	}
	return out, nil
}

// DeviceGroupGet prints a device group as indented JSON.
func DeviceGroupGet(credName, groupID string, logger *slog.Logger) error {
	out, err := DeviceGroupGetValue(credName, groupID, logger)
	if err != nil {
		return err
	}
	return printJSON(out)
}

// DeviceGroupCreateValue creates a device group from JSON.
func DeviceGroupCreateValue(credName string, opts Opts, bodyJSON []byte, logger *slog.Logger) (map[string]interface{}, error) {
	if err := requirePlatformMutate(credName); err != nil {
		return nil, err
	}
	if len(bodyJSON) == 0 {
		return nil, fmt.Errorf("request body is required")
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt("Create device group?", logger) {
			return nil, nil
		}
	}
	respBody, err := jamfPOST(credName, uriDeviceGroups, bodyJSON, "application/json")
	if err != nil {
		return nil, err
	}
	out, err := decodeJSONObject(respBody)
	if err != nil {
		return map[string]interface{}{"raw": string(respBody)}, nil
	}
	return out, nil
}

// DeviceGroupCreate creates a device group and prints the response.
func DeviceGroupCreate(credName string, opts Opts, bodyJSON []byte, logger *slog.Logger) error {
	out, err := DeviceGroupCreateValue(credName, opts, bodyJSON, logger)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	return printJSON(out)
}

// DeviceGroupSetValue PATCHes a device group.
func DeviceGroupSetValue(credName, groupID string, opts Opts, bodyJSON []byte, logger *slog.Logger) error {
	if err := requirePlatformMutate(credName); err != nil {
		return err
	}
	if strings.TrimSpace(groupID) == "" {
		return fmt.Errorf("group-id is required")
	}
	if len(bodyJSON) == 0 {
		return fmt.Errorf("request body is required")
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("Update device group %s?", groupID), logger) {
			return nil
		}
	}
	_, err := jamfPATCH(credName, uriDeviceGroups+"/"+url.PathEscape(groupID), bodyJSON, "application/json")
	return err
}

// DeviceGroupSet PATCHes a device group.
func DeviceGroupSet(credName, groupID string, opts Opts, bodyJSON []byte, logger *slog.Logger) error {
	return DeviceGroupSetValue(credName, groupID, opts, bodyJSON, logger)
}

// DeviceGroupDeleteValue deletes a device group.
func DeviceGroupDeleteValue(credName, groupID string, opts Opts, logger *slog.Logger) error {
	if err := requirePlatformMutate(credName); err != nil {
		return err
	}
	if strings.TrimSpace(groupID) == "" {
		return fmt.Errorf("group-id is required")
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("Delete device group %s?", groupID), logger) {
			return nil
		}
	}
	return jamfDELETE(credName, uriDeviceGroups+"/"+url.PathEscape(groupID))
}

// DeviceGroupDelete deletes a device group.
func DeviceGroupDelete(credName, groupID string, opts Opts, logger *slog.Logger) error {
	return DeviceGroupDeleteValue(credName, groupID, opts, logger)
}

// DeviceGroupMembersValue fetches members of a device group.
func DeviceGroupMembersValue(credName, groupID string, logger *slog.Logger) (interface{}, error) {
	if err := requirePlatformCred(credName); err != nil {
		return nil, err
	}
	if strings.TrimSpace(groupID) == "" {
		return nil, fmt.Errorf("group-id is required")
	}
	body, err := jamfGET(credName, uriDeviceGroups+"/"+url.PathEscape(groupID)+"/members")
	if err != nil {
		return nil, err
	}
	var out interface{}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("parse device-group members: %w", err)
	}
	return out, nil
}

// DeviceGroupMembers prints members JSON.
func DeviceGroupMembers(credName, groupID string, logger *slog.Logger) error {
	out, err := DeviceGroupMembersValue(credName, groupID, logger)
	if err != nil {
		return err
	}
	return printJSON(out)
}

// DeviceGroupSetMembersValue PATCHes device group membership.
func DeviceGroupSetMembersValue(credName, groupID string, opts Opts, bodyJSON []byte, logger *slog.Logger) error {
	if err := requirePlatformMutate(credName); err != nil {
		return err
	}
	if strings.TrimSpace(groupID) == "" {
		return fmt.Errorf("group-id is required")
	}
	if len(bodyJSON) == 0 {
		return fmt.Errorf("request body is required")
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("Update members of device group %s?", groupID), logger) {
			return nil
		}
	}
	_, err := jamfPATCH(credName, uriDeviceGroups+"/"+url.PathEscape(groupID)+"/members", bodyJSON, "application/json")
	return err
}

// DeviceGroupSetMembers PATCHes membership.
func DeviceGroupSetMembers(credName, groupID string, opts Opts, bodyJSON []byte, logger *slog.Logger) error {
	return DeviceGroupSetMembersValue(credName, groupID, opts, bodyJSON, logger)
}

// DeviceGroupsForDeviceValue lists device groups for a Platform device id.
func DeviceGroupsForDeviceValue(credName, deviceID string, logger *slog.Logger) (interface{}, error) {
	if err := requirePlatformCred(credName); err != nil {
		return nil, err
	}
	if strings.TrimSpace(deviceID) == "" {
		return nil, fmt.Errorf("device-id is required")
	}
	path := "/api/device-groups/v1/devices/" + url.PathEscape(deviceID) + "/device-groups"
	body, err := jamfGET(credName, path)
	if err != nil {
		return nil, err
	}
	var out interface{}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("parse device groups for device: %w", err)
	}
	return out, nil
}
