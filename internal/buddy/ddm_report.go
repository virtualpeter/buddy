package buddy

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
)

const uriDDMReport = "/api/ddm/report/v1"

// DDMDeclarationDevicesValue lists devices reporting a declaration identifier.
func DDMDeclarationDevicesValue(credName, declarationID string, pageOpts PageOpts, filter, sort string, logger *slog.Logger) (interface{}, error) {
	if err := requirePlatformCred(credName); err != nil {
		return nil, err
	}
	if strings.TrimSpace(declarationID) == "" {
		return nil, fmt.Errorf("declaration-id is required")
	}
	extra := url.Values{}
	if filter != "" {
		extra.Set("filter", filter)
	}
	if sort != "" {
		extra.Set("sort", sort)
	}
	path := uriDDMReport + "/declarations/" + url.PathEscape(declarationID) + "/devices"
	// Prefer the filtered devices endpoint; fall back to collecting pages into a list.
	var all []map[string]interface{}
	err := fetchJamfPaged(credName, path, extra, pageOpts, func(resultsJSON []byte, _ int) (int, error) {
		page, err := decodeJSONObjectList(resultsJSON)
		if err != nil {
			return 0, fmt.Errorf("parse declaration devices: %w", err)
		}
		all = append(all, page...)
		return len(page), nil
	})
	if err != nil {
		// Some responses may not be a {totalCount,results} envelope; try a plain GET.
		q := extra.Encode()
		getPath := path
		if q != "" {
			getPath = path + "?" + q
		}
		body, getErr := jamfGET(credName, getPath)
		if getErr != nil {
			return nil, err
		}
		var out interface{}
		if jsonErr := json.Unmarshal(body, &out); jsonErr != nil {
			return nil, fmt.Errorf("parse declaration devices: %w", jsonErr)
		}
		return out, nil
	}
	return all, nil
}

// DDMDeclarationDevices prints declaration device report JSON.
func DDMDeclarationDevices(credName, declarationID string, filter, sort string, logger *slog.Logger) error {
	out, err := DDMDeclarationDevicesValue(credName, declarationID, PageOpts{Page: PageAll}, filter, sort, logger)
	if err != nil {
		return err
	}
	return printJSON(out)
}

// DDMDeviceDeclarationsValue lists declarations reported for a device.
func DDMDeviceDeclarationsValue(credName, deviceID string, pageOpts PageOpts, filter, sort string, logger *slog.Logger) (interface{}, error) {
	if err := requirePlatformCred(credName); err != nil {
		return nil, err
	}
	if strings.TrimSpace(deviceID) == "" {
		return nil, fmt.Errorf("device-id is required")
	}
	extra := url.Values{}
	if filter != "" {
		extra.Set("filter", filter)
	}
	if sort != "" {
		extra.Set("sort", sort)
	}
	path := uriDDMReport + "/devices/" + url.PathEscape(deviceID) + "/declarations"
	var all []map[string]interface{}
	err := fetchJamfPaged(credName, path, extra, pageOpts, func(resultsJSON []byte, _ int) (int, error) {
		page, err := decodeJSONObjectList(resultsJSON)
		if err != nil {
			return 0, fmt.Errorf("parse device declarations: %w", err)
		}
		all = append(all, page...)
		return len(page), nil
	})
	if err != nil {
		q := extra.Encode()
		getPath := path
		if q != "" {
			getPath = path + "?" + q
		}
		body, getErr := jamfGET(credName, getPath)
		if getErr != nil {
			return nil, err
		}
		var out interface{}
		if jsonErr := json.Unmarshal(body, &out); jsonErr != nil {
			return nil, fmt.Errorf("parse device declarations: %w", jsonErr)
		}
		return out, nil
	}
	return all, nil
}

// DDMDeviceDeclarations prints device declaration report JSON.
func DDMDeviceDeclarations(credName, deviceID string, filter, sort string, logger *slog.Logger) error {
	out, err := DDMDeviceDeclarationsValue(credName, deviceID, PageOpts{Page: PageAll}, filter, sort, logger)
	if err != nil {
		return err
	}
	return printJSON(out)
}

// DDMDeviceChannelsValue lists channels available for a device.
func DDMDeviceChannelsValue(credName, deviceID string, logger *slog.Logger) (interface{}, error) {
	if err := requirePlatformCred(credName); err != nil {
		return nil, err
	}
	if strings.TrimSpace(deviceID) == "" {
		return nil, fmt.Errorf("device-id is required")
	}
	path := uriDDMReport + "/devices/" + url.PathEscape(deviceID) + "/channels"
	body, err := jamfGET(credName, path)
	if err != nil {
		return nil, err
	}
	var out interface{}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("parse device channels: %w", err)
	}
	return out, nil
}

// DDMDeviceChannels prints device channels JSON.
func DDMDeviceChannels(credName, deviceID string, logger *slog.Logger) error {
	out, err := DDMDeviceChannelsValue(credName, deviceID, logger)
	if err != nil {
		return err
	}
	return printJSON(out)
}
