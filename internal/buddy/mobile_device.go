package buddy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

const uriMobileDevices = "/api/v2/mobile-devices"

var errMobileDeviceIDsRequired = errors.New("at least one mobile-device ID required")

// MobileDeviceRecord holds the fields of one row from GET /api/v2/mobile-devices.
type MobileDeviceRecord struct {
	ID                     string `json:"id"`
	Name                   string `json:"name,omitempty"`
	SerialNumber           string `json:"serialNumber,omitempty"`
	WifiMacAddress         string `json:"wifiMacAddress,omitempty"`
	UDID                   string `json:"udid,omitempty"`
	PhoneNumber            string `json:"phoneNumber,omitempty"`
	Model                  string `json:"model,omitempty"`
	ModelIdentifier        string `json:"modelIdentifier,omitempty"`
	Username               string `json:"username,omitempty"`
	Type                   string `json:"type,omitempty"`
	ManagementID           string `json:"managementId,omitempty"`
	SoftwareUpdateDeviceID string `json:"softwareUpdateDeviceId,omitempty"`
}

// mobileDeviceMatchesFilter reports whether the record matches a client-side
// substring filter (case-insensitive) on id, name, serial, udid, username,
// model, or type. GET /api/v2/mobile-devices has no server-side filter.
func mobileDeviceMatchesFilter(r MobileDeviceRecord, filterLower string) bool {
	if filterLower == "" {
		return true
	}
	for _, s := range []string{r.ID, r.Name, r.SerialNumber, r.UDID, r.Username, r.Model, r.Type} {
		if strings.Contains(strings.ToLower(s), filterLower) {
			return true
		}
	}
	return false
}

// MobileDeviceSearch fetches mobile devices from GET /api/v2/mobile-devices.
// filter is a client-side substring matched against id/name/serial/udid/username/model/type
// (the endpoint has no server-side filter). When opts.Page is PageAll, all pages are fetched.
func MobileDeviceSearch(credName, filter string, opts PageOpts, _ *slog.Logger) ([]MobileDeviceRecord, error) {
	filterLower := strings.ToLower(strings.TrimSpace(filter))
	var all []MobileDeviceRecord
	err := fetchJamfPaged(credName, uriMobileDevices, nil, opts, func(resultsJSON []byte, _ int) (int, error) {
		var page []MobileDeviceRecord
		if err := json.Unmarshal(resultsJSON, &page); err != nil {
			return 0, fmt.Errorf("parse response: %w", err)
		}
		for _, r := range page {
			if mobileDeviceMatchesFilter(r, filterLower) {
				all = append(all, r)
			}
		}
		return len(page), nil
	})
	if err != nil {
		return nil, err
	}
	return all, nil
}

// MobileDeviceSearchPrint prints a table of mobile devices, or the matching rows as JSON.
func MobileDeviceSearchPrint(credName, filter string, asJSON bool, logger *slog.Logger) error {
	records, err := MobileDeviceSearch(credName, filter, PageOpts{Page: PageAll}, logger)
	if err != nil {
		return err
	}
	if asJSON {
		return printJSONValue(records)
	}
	fmt.Printf("%-8s  %-32s  %-14s  %-10s  %-28s  %s\n", "ID", "Name", "Serial", "Type", "Model", "Username")
	fmt.Printf("%-8s  %-32s  %-14s  %-10s  %-28s  %s\n", "--------", "--------------------------------", "--------------", "----------", "----------------------------", "----------------")
	for _, r := range records {
		fmt.Printf("%-8s  %-32s  %-14s  %-10s  %-28s  %s\n",
			r.ID, truncate(r.Name, 32), truncate(r.SerialNumber, 14), truncate(r.Type, 10), truncate(r.Model, 28), truncate(r.Username, 24))
	}
	fmt.Printf("\n%d mobile device(s)\n", len(records))
	return nil
}

func printJSONValue(v interface{}) error {
	return printJSONValueTo(os.Stdout, v)
}

func printJSONValueTo(w io.Writer, v interface{}) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, body, "", "  "); err != nil {
		fmt.Fprintln(w, string(body))
		return nil
	}
	fmt.Fprintln(w, pretty.String())
	return nil
}

func mobileDevicePathID(idStr string) (int, error) {
	id, err := strconv.Atoi(strings.TrimSpace(idStr))
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid mobile-device-id %q: must be positive integer", idStr)
	}
	return id, nil
}

// MobileDeviceFetchInventory returns raw JSON from GET /api/v2/mobile-devices/{id}.
func MobileDeviceFetchInventory(credName, idStr string) ([]byte, error) {
	id, err := mobileDevicePathID(idStr)
	if err != nil {
		return nil, err
	}
	body, err := jamfGET(credName, uriMobileDevices+"/"+strconv.Itoa(id))
	if err != nil {
		return nil, fmt.Errorf("get mobile device %d: %w", id, err)
	}
	return body, nil
}

// MobileDeviceFetchDetail returns raw JSON from GET /api/v2/mobile-devices/{id}/detail (full inventory).
func MobileDeviceFetchDetail(credName, idStr string) ([]byte, error) {
	id, err := mobileDevicePathID(idStr)
	if err != nil {
		return nil, err
	}
	body, err := jamfGET(credName, uriMobileDevices+"/"+strconv.Itoa(id)+"/detail")
	if err != nil {
		return nil, fmt.Errorf("get mobile device detail %d: %w", id, err)
	}
	return body, nil
}

// MobileDeviceGetValue fetches GET /api/v2/mobile-devices/{id} JSON as a map.
func MobileDeviceGetValue(credName, idStr string) (map[string]interface{}, error) {
	body, err := MobileDeviceFetchInventory(credName, idStr)
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("parse mobile device %s: %w", strings.TrimSpace(idStr), err)
	}
	return out, nil
}

// MobileDeviceDetailValue fetches GET /api/v2/mobile-devices/{id}/detail JSON as a map.
func MobileDeviceDetailValue(credName, idStr string) (map[string]interface{}, error) {
	body, err := MobileDeviceFetchDetail(credName, idStr)
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("parse mobile device detail %s: %w", strings.TrimSpace(idStr), err)
	}
	return out, nil
}

// MobileDeviceGet prints mobile device JSON (basic record, or /detail when detail is true).
func MobileDeviceGet(credName, idStr string, detail bool, _ *slog.Logger) error {
	var body []byte
	var err error
	if detail {
		body, err = MobileDeviceFetchDetail(credName, idStr)
	} else {
		body, err = MobileDeviceFetchInventory(credName, idStr)
	}
	if err != nil {
		return err
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, body, "", "  "); err != nil {
		fmt.Println(string(body))
		return nil
	}
	fmt.Println(pretty.String())
	return nil
}

// MobileDeviceResolve finds one mobile device by exactly one of id, name, serial, or udid.
// name/serial/udid walk the list (no server-side filter); name prefers an exact
// case-insensitive match and falls back to a unique substring match.
func MobileDeviceResolve(credName, id, name, serial, udid string, logger *slog.Logger) (MobileDeviceRecord, error) {
	keys := 0
	for _, s := range []string{id, name, serial, udid} {
		if strings.TrimSpace(s) != "" {
			keys++
		}
	}
	if keys != 1 {
		return MobileDeviceRecord{}, fmt.Errorf("pass exactly one of id, name, serial, or udid")
	}
	if strings.TrimSpace(id) != "" {
		body, err := MobileDeviceFetchInventory(credName, id)
		if err != nil {
			return MobileDeviceRecord{}, err
		}
		var rec MobileDeviceRecord
		if err := json.Unmarshal(body, &rec); err != nil {
			return MobileDeviceRecord{}, fmt.Errorf("parse mobile device %s: %w", id, err)
		}
		return rec, nil
	}
	records, err := MobileDeviceSearch(credName, "", PageOpts{Page: PageAll}, logger)
	if err != nil {
		return MobileDeviceRecord{}, err
	}
	if s := strings.TrimSpace(serial); s != "" {
		for _, r := range records {
			if strings.EqualFold(r.SerialNumber, s) {
				return r, nil
			}
		}
		return MobileDeviceRecord{}, fmt.Errorf("no mobile device with serial %q", s)
	}
	if u := strings.TrimSpace(udid); u != "" {
		for _, r := range records {
			if strings.EqualFold(r.UDID, u) {
				return r, nil
			}
		}
		return MobileDeviceRecord{}, fmt.Errorf("no mobile device with udid %q", u)
	}
	n := strings.TrimSpace(name)
	for _, r := range records {
		if strings.EqualFold(r.Name, n) {
			return r, nil
		}
	}
	var matches []MobileDeviceRecord
	nLower := strings.ToLower(n)
	for _, r := range records {
		if strings.Contains(strings.ToLower(r.Name), nLower) {
			matches = append(matches, r)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return MobileDeviceRecord{}, fmt.Errorf("no mobile device with name %q", n)
	default:
		return MobileDeviceRecord{}, fmt.Errorf("%d mobile devices match name %q; use id= or serial=", len(matches), n)
	}
}

// mobileDeviceGetManagementID fetches a mobile device's managementId (for MDM commands).
func mobileDeviceGetManagementID(credName string, deviceID int) (string, error) {
	body, err := jamfGET(credName, uriMobileDevices+"/"+strconv.Itoa(deviceID))
	if err != nil {
		return "", fmt.Errorf("get mobile device %d: %w", deviceID, err)
	}
	var rec MobileDeviceRecord
	if err := json.Unmarshal(body, &rec); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if rec.ManagementID == "" {
		return "", fmt.Errorf("mobile device %d: no managementId found (device may not be MDM-enrolled)", deviceID)
	}
	return rec.ManagementID, nil
}

// MobileDeviceSendMDMCommand sends an MDM command to one or more mobile devices via
// POST /api/v2/mdm/commands (clientType MOBILE_DEVICE). Each device's managementId is
// fetched via GET /api/v2/mobile-devices/{id}.
func MobileDeviceSendMDMCommand(credName string, deviceIDs []int, commandType string, commandData map[string]interface{}, _ *slog.Logger) error {
	if len(deviceIDs) == 0 {
		return errMobileDeviceIDsRequired
	}
	if commandData == nil {
		commandData = make(map[string]interface{})
	}
	commandData["commandType"] = commandType

	for _, id := range deviceIDs {
		mgmtID, err := mobileDeviceGetManagementID(credName, id)
		if err != nil {
			return err
		}
		if err := sendMDMCommandToClient(credName, mgmtID, "MOBILE_DEVICE", commandData); err != nil {
			return fmt.Errorf("send MDM command to mobile device %d: %w", id, err)
		}
	}
	return nil
}

// MobileDeviceUpdateInventory sends DEVICE_INFORMATION to refresh inventory on one or more devices.
func MobileDeviceUpdateInventory(credName string, opts Opts, deviceIDs []int, logger *slog.Logger) error {
	if len(deviceIDs) == 0 {
		return errMobileDeviceIDsRequired
	}
	if err := MobileDeviceSendMDMCommand(credName, deviceIDs, "DEVICE_INFORMATION", nil, logger); err != nil {
		return err
	}
	fmt.Printf("Inventory update queued for %d mobile device(s)\n", len(deviceIDs))
	return nil
}

// MobileDeviceRestart sends RESTART_DEVICE to one or more mobile devices.
func MobileDeviceRestart(credName string, opts Opts, deviceIDs []int, logger *slog.Logger) error {
	if len(deviceIDs) == 0 {
		return errMobileDeviceIDsRequired
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nRestart %d mobile device(s): %v", len(deviceIDs), deviceIDs), logger) {
			return nil
		}
	}
	if err := MobileDeviceSendMDMCommand(credName, deviceIDs, "RESTART_DEVICE", nil, logger); err != nil {
		return err
	}
	fmt.Printf("Restart command sent to %d mobile device(s)\n", len(deviceIDs))
	return nil
}

// MobileDeviceLock sends DEVICE_LOCK to one or more mobile devices. Requires extra
// confirmation unless opts.SkipConfirm.
func MobileDeviceLock(credName string, opts Opts, deviceIDs []int, message, phoneNumber, pin string, logger *slog.Logger) error {
	if len(deviceIDs) == 0 {
		return errMobileDeviceIDsRequired
	}
	if !opts.SkipConfirm {
		if !confirmDangerous("DEVICE_LOCK will lock the device screen.", "", logger) {
			return nil
		}
	}
	cmdData := make(map[string]interface{})
	if message != "" {
		cmdData["message"] = message
	}
	if phoneNumber != "" {
		cmdData["phoneNumber"] = phoneNumber
	}
	if pin != "" {
		cmdData["pin"] = pin
	}
	if err := MobileDeviceSendMDMCommand(credName, deviceIDs, "DEVICE_LOCK", cmdData, logger); err != nil {
		return err
	}
	fmt.Printf("Lock command sent to %d mobile device(s)\n", len(deviceIDs))
	return nil
}

// MobileDeviceErase sends ERASE_DEVICE to one or more mobile devices. Requires typing
// "erase" to confirm unless opts.SkipConfirm.
func MobileDeviceErase(credName string, opts Opts, deviceIDs []int, pin, obliterationBehavior string, preserveDataPlan, disallowProximitySetup bool, logger *slog.Logger) error {
	if len(deviceIDs) == 0 {
		return errMobileDeviceIDsRequired
	}
	if !opts.SkipConfirm {
		if !confirmDangerous("ERASE_DEVICE will permanently wipe the device. This cannot be undone.", "erase", logger) {
			return nil
		}
	}
	cmdData := make(map[string]interface{})
	if pin != "" {
		cmdData["pin"] = pin
	}
	if obliterationBehavior != "" {
		cmdData["obliterationBehavior"] = obliterationBehavior
	}
	if preserveDataPlan {
		cmdData["preserveDataPlan"] = true
	}
	if disallowProximitySetup {
		cmdData["disallowProximitySetup"] = true
	}
	if err := MobileDeviceSendMDMCommand(credName, deviceIDs, "ERASE_DEVICE", cmdData, logger); err != nil {
		return err
	}
	fmt.Printf("Erase command sent to %d mobile device(s)\n", len(deviceIDs))
	return nil
}
