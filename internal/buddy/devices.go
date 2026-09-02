package buddy

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
)

const uriDevices = "/api/devices/v1/devices"

// DeviceOverview is a list-item summary from the Platform devices API.
type DeviceOverview struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	SerialNumber string `json:"serialNumber,omitempty"`
	Model        string `json:"model,omitempty"`
}

// DeviceListOpts controls Platform device list filtering and pagination.
type DeviceListOpts struct {
	Filter string
	Sort   string
	Page   PageOpts
}

// DeviceListValues lists Platform devices (all pages by default).
// Filter: bare term → name=="*term*"; RSQL → pass-through.
func DeviceListValues(credName string, listOpts DeviceListOpts, logger *slog.Logger) ([]DeviceOverview, error) {
	if err := requirePlatformCred(credName); err != nil {
		return nil, err
	}
	normalized, err := NormalizeListFilter(listOpts.Filter, FilterSpecDevices)
	if err != nil {
		return nil, err
	}
	extra := url.Values{}
	if normalized != "" {
		extra.Set("filter", normalized)
	}
	if listOpts.Sort != "" {
		extra.Set("sort", listOpts.Sort)
	}
	var all []DeviceOverview
	err = fetchJamfPaged(credName, uriDevices, extra, listOpts.Page, func(resultsJSON []byte, _ int) (int, error) {
		var page []DeviceOverview
		if err := json.Unmarshal(resultsJSON, &page); err != nil {
			return 0, fmt.Errorf("parse devices: %w", err)
		}
		all = append(all, page...)
		return len(page), nil
	})
	if err != nil {
		return nil, err
	}
	return all, nil
}

// DeviceList prints a table of Platform devices (or the rows as JSON).
func DeviceList(credName string, listOpts DeviceListOpts, asJSON bool, logger *slog.Logger) error {
	items, err := DeviceListValues(credName, listOpts, logger)
	if err != nil {
		return err
	}
	if asJSON {
		return printJSONValue(items)
	}
	if len(items) == 0 {
		fmt.Println("No devices found.")
		return nil
	}
	fmt.Printf("%-38s  %-28s  %s\n", "ID", "Name", "Serial")
	fmt.Printf("%-38s  %-28s  %s\n", strings.Repeat("-", 38), strings.Repeat("-", 28), strings.Repeat("-", 20))
	for _, d := range items {
		name := d.Name
		if len(name) > 28 {
			name = name[:25] + "..."
		}
		fmt.Printf("%-38s  %-28s  %s\n", d.ID, name, d.SerialNumber)
	}
	return nil
}

// DeviceGetValue fetches one Platform device.
func DeviceGetValue(credName, deviceID string, logger *slog.Logger) (map[string]interface{}, error) {
	if err := requirePlatformCred(credName); err != nil {
		return nil, err
	}
	if strings.TrimSpace(deviceID) == "" {
		return nil, fmt.Errorf("device-id is required")
	}
	body, err := jamfGET(credName, uriDevices+"/"+url.PathEscape(deviceID))
	if err != nil {
		return nil, err
	}
	out, err := decodeJSONObject(body)
	if err != nil {
		return nil, fmt.Errorf("parse device: %w", err)
	}
	return out, nil
}

// DeviceGet prints a Platform device as JSON.
func DeviceGet(credName, deviceID string, logger *slog.Logger) error {
	out, err := DeviceGetValue(credName, deviceID, logger)
	if err != nil {
		return err
	}
	return printJSON(out)
}

// DeviceSetValue PATCHes a Platform device.
func DeviceSetValue(credName, deviceID string, opts Opts, bodyJSON []byte, logger *slog.Logger) error {
	if err := requirePlatformMutate(credName); err != nil {
		return err
	}
	if strings.TrimSpace(deviceID) == "" {
		return fmt.Errorf("device-id is required")
	}
	if len(bodyJSON) == 0 {
		return fmt.Errorf("request body is required")
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("Update device %s?", deviceID), logger) {
			return nil
		}
	}
	_, err := jamfPATCH(credName, uriDevices+"/"+url.PathEscape(deviceID), bodyJSON, "application/json")
	return err
}

// DeviceSet PATCHes a Platform device.
func DeviceSet(credName, deviceID string, opts Opts, bodyJSON []byte, logger *slog.Logger) error {
	return DeviceSetValue(credName, deviceID, opts, bodyJSON, logger)
}

// DeviceDeleteValue deletes a Platform device.
func DeviceDeleteValue(credName, deviceID string, opts Opts, logger *slog.Logger) error {
	if err := requirePlatformMutate(credName); err != nil {
		return err
	}
	if strings.TrimSpace(deviceID) == "" {
		return fmt.Errorf("device-id is required")
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("Delete device %s?", deviceID), logger) {
			return nil
		}
	}
	return jamfDELETE(credName, uriDevices+"/"+url.PathEscape(deviceID))
}

// DeviceDelete deletes a Platform device.
func DeviceDelete(credName, deviceID string, opts Opts, logger *slog.Logger) error {
	return DeviceDeleteValue(credName, deviceID, opts, logger)
}

// DeviceApplicationsValue lists installed applications for a Platform device.
// Filter: bare term → name=="*term*"; RSQL → pass-through.
func DeviceApplicationsValue(credName, deviceID string, pageOpts PageOpts, filter string, logger *slog.Logger) ([]map[string]interface{}, error) {
	if err := requirePlatformCred(credName); err != nil {
		return nil, err
	}
	if strings.TrimSpace(deviceID) == "" {
		return nil, fmt.Errorf("device-id is required")
	}
	normalized, err := NormalizeListFilter(filter, FilterSpecDevices)
	if err != nil {
		return nil, err
	}
	extra := url.Values{}
	if normalized != "" {
		extra.Set("filter", normalized)
	}
	path := uriDevices + "/" + url.PathEscape(deviceID) + "/applications"
	var all []map[string]interface{}
	err = fetchJamfPaged(credName, path, extra, pageOpts, func(resultsJSON []byte, _ int) (int, error) {
		page, err := decodeJSONObjectList(resultsJSON)
		if err != nil {
			return 0, fmt.Errorf("parse applications: %w", err)
		}
		all = append(all, page...)
		return len(page), nil
	})
	if err != nil {
		return nil, err
	}
	return all, nil
}

// DeviceApplications prints applications JSON.
func DeviceApplications(credName, deviceID string, filter string, logger *slog.Logger) error {
	items, err := DeviceApplicationsValue(credName, deviceID, PageOpts{Page: PageAll}, filter, logger)
	if err != nil {
		return err
	}
	return printJSON(items)
}

// DevicesForUserValue lists Platform devices for a user id.
// Filter: bare term → name=="*term*"; RSQL → pass-through.
func DevicesForUserValue(credName, userID string, listOpts DeviceListOpts, logger *slog.Logger) ([]DeviceOverview, error) {
	if err := requirePlatformCred(credName); err != nil {
		return nil, err
	}
	if strings.TrimSpace(userID) == "" {
		return nil, fmt.Errorf("user-id is required")
	}
	normalized, err := NormalizeListFilter(listOpts.Filter, FilterSpecDevices)
	if err != nil {
		return nil, err
	}
	extra := url.Values{}
	if normalized != "" {
		extra.Set("filter", normalized)
	}
	if listOpts.Sort != "" {
		extra.Set("sort", listOpts.Sort)
	}
	path := "/api/devices/v1/users/" + url.PathEscape(userID) + "/devices"
	var all []DeviceOverview
	err = fetchJamfPaged(credName, path, extra, listOpts.Page, func(resultsJSON []byte, _ int) (int, error) {
		var page []DeviceOverview
		if err := json.Unmarshal(resultsJSON, &page); err != nil {
			return 0, fmt.Errorf("parse user devices: %w", err)
		}
		all = append(all, page...)
		return len(page), nil
	})
	if err != nil {
		return nil, err
	}
	return all, nil
}

const uriDeviceActions = "/api/device-actions/v1/devices"

func deviceAction(credName, deviceID, action string, opts Opts, logger *slog.Logger) error {
	if err := requirePlatformMutate(credName); err != nil {
		return err
	}
	if strings.TrimSpace(deviceID) == "" {
		return fmt.Errorf("device-id is required")
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("%s device %s?", action, deviceID), logger) {
			return nil
		}
	}
	path := uriDeviceActions + "/" + url.PathEscape(deviceID) + "/" + action
	_, err := jamfPOST(credName, path, nil, "application/json")
	return err
}

// DeviceCheckIn requests a Platform device check-in.
func DeviceCheckIn(credName, deviceID string, opts Opts, logger *slog.Logger) error {
	return deviceAction(credName, deviceID, "check-in", opts, logger)
}

// DeviceRestart restarts a Platform device.
func DeviceRestart(credName, deviceID string, opts Opts, logger *slog.Logger) error {
	return deviceAction(credName, deviceID, "restart", opts, logger)
}

// DeviceShutdown shuts down a Platform device.
func DeviceShutdown(credName, deviceID string, opts Opts, logger *slog.Logger) error {
	return deviceAction(credName, deviceID, "shutdown", opts, logger)
}

// DeviceErase erases a Platform device.
func DeviceErase(credName, deviceID string, opts Opts, logger *slog.Logger) error {
	return deviceAction(credName, deviceID, "erase", opts, logger)
}

// DeviceUnmanage unmanages a Platform device.
func DeviceUnmanage(credName, deviceID string, opts Opts, logger *slog.Logger) error {
	return deviceAction(credName, deviceID, "unmanage", opts, logger)
}
