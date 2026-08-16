package buddy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

var errComputerIDsRequired = errors.New("at least one computer ID required")

// ComputerRecord holds key fields from a computer inventory record.
type ComputerRecord struct {
	ID           int    `json:"id"`
	Name         string `json:"name,omitempty"`
	SerialNumber string `json:"serialNumber,omitempty"`
	UDID         string `json:"udid,omitempty"`
	ManagementID string `json:"managementId,omitempty"`
	MacAddress   string `json:"macAddress,omitempty"`
	Username     string `json:"username,omitempty"`
}

// computerInventoryListResponseV3 is the paginated response from GET /api/v3/computers-inventory.
type computerInventoryListResponseV3 struct {
	TotalCount int                  `json:"totalCount"`
	Results    []computerListItemV3 `json:"results"`
}

// computerListItemV3 is a single item in the v3 computers-inventory list.
// v3 returns id as string; we use interface{} to accept both for robustness.
type computerListItemV3 struct {
	ID           interface{} `json:"id"`
	UDID         string      `json:"udid,omitempty"`
	SerialNumber string      `json:"serialNumber,omitempty"`
	General      *struct {
		Name         string `json:"name,omitempty"`
		MacAddress   string `json:"macAddress,omitempty"`
		Username     string `json:"username,omitempty"`
		ManagementID string `json:"managementId,omitempty"`
	} `json:"general,omitempty"`
	Hardware *struct {
		SerialNumber string `json:"serialNumber,omitempty"`
	} `json:"hardware,omitempty"`
}

// computerInventoryDetailResponseV3 is the response from GET /api/v3/computers-inventory/{id}.
type computerInventoryDetailResponseV3 struct {
	ID           interface{} `json:"id"`
	UDID         string      `json:"udid,omitempty"`
	SerialNumber string      `json:"serialNumber,omitempty"`
	General      *struct {
		Name         string `json:"name,omitempty"`
		MacAddress   string `json:"macAddress,omitempty"`
		Username     string `json:"username,omitempty"`
		ManagementID string `json:"managementId,omitempty"`
	} `json:"general,omitempty"`
	Hardware *struct {
		SerialNumber string `json:"serialNumber,omitempty"`
	} `json:"hardware,omitempty"`
}

// parseComputerID extracts int from v3 id (string) or v1 id (number).
func parseComputerID(v interface{}) (int, error) {
	switch x := v.(type) {
	case float64:
		return int(x), nil
	case string:
		n, err := strconv.Atoi(x)
		if err != nil {
			return 0, fmt.Errorf("invalid id %q: %w", x, err)
		}
		return n, nil
	default:
		return 0, fmt.Errorf("invalid id type: %T", v)
	}
}

// mdmCommandRequest is the payload for POST /api/v2/mdm/commands.
type mdmCommandRequest struct {
	ClientData  []mdmClientData        `json:"clientData"`
	CommandData map[string]interface{} `json:"commandData"`
}

type mdmClientData struct {
	ManagementID string `json:"managementId"`
	ClientType   string `json:"clientType"`
}

// ComputerSearch fetches computers matching the filter and returns records.
// Bare term → general.name=="*term*"; RSQL expression → pass-through.
// When opts.Page is PageAll, all pages are fetched.
func ComputerSearch(credName, filter string, opts PageOpts, logger *slog.Logger) ([]ComputerRecord, error) {
	normalized, err := NormalizeListFilter(filter, FilterSpecComputers)
	if err != nil {
		return nil, err
	}
	extra := url.Values{}
	if normalized != "" {
		extra.Set("filter", normalized)
	}
	var all []ComputerRecord
	err = fetchJamfPaged(credName, "/api/v3/computers-inventory", extra, opts, func(resultsJSON []byte, _ int) (int, error) {
		var page []computerListItemV3
		if err := json.Unmarshal(resultsJSON, &page); err != nil {
			return 0, fmt.Errorf("parse response: %w", err)
		}
		for _, r := range page {
			id, err := parseComputerID(r.ID)
			if err != nil {
				return 0, err
			}
			rec := ComputerRecord{ID: id}
			if r.General != nil {
				rec.Name = r.General.Name
				rec.MacAddress = r.General.MacAddress
				rec.Username = r.General.Username
				rec.ManagementID = r.General.ManagementID
			}
			if r.SerialNumber != "" {
				rec.SerialNumber = r.SerialNumber
			} else if r.Hardware != nil && r.Hardware.SerialNumber != "" {
				rec.SerialNumber = r.Hardware.SerialNumber
			}
			rec.UDID = r.UDID
			all = append(all, rec)
		}
		return len(page), nil
	})
	if err != nil {
		return nil, err
	}
	return all, nil
}

// ComputerSearchPrint prints a table of computers matching the filter.
func ComputerSearchPrint(credName, filter string, logger *slog.Logger) error {
	records, err := ComputerSearch(credName, filter, PageOpts{Page: PageAll}, logger)
	if err != nil {
		return err
	}
	fmt.Printf("%-8s  %-40s  %-20s  %s\n", "ID", "Name", "Serial", "UDID")
	fmt.Printf("%-8s  %-40s  %-20s  %s\n", "--------", "----------------------------------------", "--------------------", "--------------------")
	for _, r := range records {
		fmt.Printf("%-8d  %-40s  %-20s  %s\n", r.ID, truncateComputer(r.Name, 40), truncateComputer(r.SerialNumber, 20), truncateComputer(r.UDID, 20))
	}
	fmt.Printf("\n%d computer(s)\n", len(records))
	return nil
}

func truncateComputer(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}

// ComputerGet fetches a single computer by ID and prints full inventory JSON
// (passthrough of GET /api/v3/computers-inventory/{id}; default section is GENERAL).
func ComputerGet(credName, idStr string, sections []string, logger *slog.Logger) error {
	body, err := ComputerFetchInventory(credName, idStr, sections)
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

// ComputerDetail prints full inventory from GET /api/v3/computers-inventory-detail/{id} (all sections).
func ComputerDetail(credName, idStr string, logger *slog.Logger) error {
	body, err := ComputerFetchDetail(credName, idStr)
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

// NormalizeComputerSections trims, uppercases, and dedupes section names for ?section=.
func NormalizeComputerSections(sections []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, s := range sections {
		for _, part := range strings.Split(s, ",") {
			part = strings.ToUpper(strings.TrimSpace(part))
			if part == "" {
				continue
			}
			if _, ok := seen[part]; ok {
				continue
			}
			seen[part] = struct{}{}
			out = append(out, part)
		}
	}
	return out
}

func computerInventoryPath(idStr string, sections []string) (string, error) {
	id, err := strconv.Atoi(strings.TrimSpace(idStr))
	if err != nil || id <= 0 {
		return "", fmt.Errorf("invalid computer-id %q: must be positive integer", idStr)
	}
	path := "/api/v3/computers-inventory/" + strconv.Itoa(id)
	secs := NormalizeComputerSections(sections)
	if len(secs) == 0 {
		return path, nil
	}
	q := url.Values{}
	for _, s := range secs {
		q.Add("section", s)
	}
	return path + "?" + q.Encode(), nil
}

// ComputerFetchInventory returns raw JSON from GET /api/v3/computers-inventory/{id}.
// When sections is empty, Jamf defaults to the GENERAL section.
func ComputerFetchInventory(credName, idStr string, sections []string) ([]byte, error) {
	path, err := computerInventoryPath(idStr, sections)
	if err != nil {
		return nil, err
	}
	body, err := jamfGET(credName, path)
	if err != nil {
		return nil, fmt.Errorf("get computer %s: %w", strings.TrimSpace(idStr), err)
	}
	return body, nil
}

// ComputerFetchDetail returns raw JSON from GET /api/v3/computers-inventory-detail/{id} (all sections).
func ComputerFetchDetail(credName, idStr string) ([]byte, error) {
	id, err := strconv.Atoi(strings.TrimSpace(idStr))
	if err != nil || id <= 0 {
		return nil, fmt.Errorf("invalid computer-id %q: must be positive integer", idStr)
	}
	path := "/api/v3/computers-inventory-detail/" + strconv.Itoa(id)
	body, err := jamfGET(credName, path)
	if err != nil {
		return nil, fmt.Errorf("get computer detail %s: %w", strings.TrimSpace(idStr), err)
	}
	return body, nil
}

// ComputerGetValue fetches inventory JSON and returns it as a map (full passthrough; no field projection).
// sections maps to ?section= (empty = Jamf GENERAL default).
func ComputerGetValue(credName, idStr string, sections []string, logger *slog.Logger) (map[string]interface{}, error) {
	_ = logger
	body, err := ComputerFetchInventory(credName, idStr, sections)
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("parse computer %s: %w", strings.TrimSpace(idStr), err)
	}
	return out, nil
}

// ComputerDetailValue fetches all-section inventory JSON as a map.
func ComputerDetailValue(credName, idStr string, logger *slog.Logger) (map[string]interface{}, error) {
	_ = logger
	body, err := ComputerFetchDetail(credName, idStr)
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("parse computer detail %s: %w", strings.TrimSpace(idStr), err)
	}
	return out, nil
}

// computerGetManagementID fetches a computer's managementId (for MDM commands).
func computerGetManagementID(credName string, computerID int) (string, error) {
	endpoint, err := ResolveEndpoint(credName, "/api/v3/computers-inventory/"+strconv.Itoa(computerID)+"?section=GENERAL")
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return "", fmt.Errorf("get computer %d: %w", computerID, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("get computer %d status %d: %s", computerID, resp.StatusCode, string(body))
	}
	var detail computerInventoryDetailResponseV3
	if err := json.Unmarshal(body, &detail); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	// managementId is in General (v3 schema)
	if detail.General != nil && detail.General.ManagementID != "" {
		return detail.General.ManagementID, nil
	}
	// Fallback: check general.managementId or top-level in raw JSON
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if v, ok := raw["managementId"]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s, nil
		}
	}
	if g, ok := raw["general"].(map[string]interface{}); ok {
		if v, ok := g["managementId"]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s, nil
			}
		}
	}
	return "", fmt.Errorf("computer %d: no managementId found (device may not be MDM-enrolled)", computerID)
}

// computerSendMDMCommand sends an MDM command to one or more computers via POST /api/v2/mdm/commands.
// Each computer's managementId is fetched via GET /api/v3/computers-inventory/{id}?section=GENERAL.
func computerSendMDMCommand(credName string, computerIDs []int, commandType string, commandData map[string]interface{}, logger *slog.Logger) error {
	if len(computerIDs) == 0 {
		return errComputerIDsRequired
	}
	// Ensure commandType is in commandData
	if commandData == nil {
		commandData = make(map[string]interface{})
	}
	commandData["commandType"] = commandType

	for _, id := range computerIDs {
		mgmtID, err := computerGetManagementID(credName, id)
		if err != nil {
			return err
		}
		payload := mdmCommandRequest{
			ClientData: []mdmClientData{
				{ManagementID: mgmtID, ClientType: "COMPUTER"},
			},
			CommandData: commandData,
		}
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal MDM command payload: %w", err)
		}
		endpoint, err := ResolveEndpoint(credName, "/api/v2/mdm/commands")
		if err != nil {
			return err
		}
		req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		resp, err := PerformRequest(nil, req, credName)
		if err != nil {
			return fmt.Errorf("send MDM command to computer %d: %w", id, err)
		}
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("MDM command computer %d status %d: %s", id, resp.StatusCode, string(respBody))
		}
	}
	return nil
}

// ComputerSendMDMCommand sends an MDM command to one or more computers (for script/component use).
// commandType examples: "Reboot", "Shutdown", "EraseDevice", "DeviceLock", "LogOutUser", etc.
// commandData is optional and commandType will be injected into it as "commandType".
func ComputerSendMDMCommand(credName string, computerIDs []int, commandType string, commandData map[string]interface{}, logger *slog.Logger) error {
	return computerSendMDMCommand(credName, computerIDs, commandType, commandData, logger)
}

// ComputerReboot sends RESTART_DEVICE MDM command to one or more computers.
func ComputerReboot(credName string, opts Opts, computerIDs []int, logger *slog.Logger) error {
	if len(computerIDs) == 0 {
		return errComputerIDsRequired
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nReboot %d computer(s): %v", len(computerIDs), computerIDs), logger) {
			return nil
		}
	}
	cmdData := map[string]interface{}{"notifyUser": "false"}
	if err := computerSendMDMCommand(credName, computerIDs, "RESTART_DEVICE", cmdData, logger); err != nil {
		return err
	}
	fmt.Printf("Reboot command sent to %d computer(s)\n", len(computerIDs))
	return nil
}

// ComputerShutdown sends SHUT_DOWN_DEVICE MDM command to one or more computers.
func ComputerShutdown(credName string, opts Opts, computerIDs []int, logger *slog.Logger) error {
	if len(computerIDs) == 0 {
		return errComputerIDsRequired
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nShutdown %d computer(s): %v", len(computerIDs), computerIDs), logger) {
			return nil
		}
	}
	if err := computerSendMDMCommand(credName, computerIDs, "SHUT_DOWN_DEVICE", nil, logger); err != nil {
		return err
	}
	fmt.Printf("Shutdown command sent to %d computer(s)\n", len(computerIDs))
	return nil
}

// ComputerLock sends DEVICE_LOCK MDM command to one or more computers. Requires extra confirmation unless opts.SkipConfirm.
func ComputerLock(credName string, opts Opts, computerIDs []int, message, phoneNumber, pin string, logger *slog.Logger) error {
	if len(computerIDs) == 0 {
		return errComputerIDsRequired
	}
	if !opts.SkipConfirm {
		if !confirmDangerous("DEVICE_LOCK will lock the user out.", "", logger) {
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
	if err := computerSendMDMCommand(credName, computerIDs, "DEVICE_LOCK", cmdData, logger); err != nil {
		return err
	}
	fmt.Printf("Lock command sent to %d computer(s)\n", len(computerIDs))
	return nil
}

// ComputerLogOut sends LOG_OUT_USER MDM command to one or more computers.
func ComputerLogOut(credName string, opts Opts, computerIDs []int, logger *slog.Logger) error {
	if len(computerIDs) == 0 {
		return errComputerIDsRequired
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nLog out user on %d computer(s): %v", len(computerIDs), computerIDs), logger) {
			return nil
		}
	}
	if err := computerSendMDMCommand(credName, computerIDs, "LOG_OUT_USER", nil, logger); err != nil {
		return err
	}
	fmt.Printf("Logout command sent to %d computer(s)\n", len(computerIDs))
	return nil
}

// ComputerErase sends ERASE_DEVICE MDM command to one or more computers. Requires typing "erase" to confirm unless opts.SkipConfirm.
func ComputerErase(credName string, opts Opts, computerIDs []int, pin, obliterationBehavior string, preserveDataPlan, disallowProximitySetup bool, logger *slog.Logger) error {
	if len(computerIDs) == 0 {
		return errComputerIDsRequired
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
	if err := computerSendMDMCommand(credName, computerIDs, "ERASE_DEVICE", cmdData, logger); err != nil {
		return err
	}
	fmt.Printf("Erase command sent to %d computer(s)\n", len(computerIDs))
	return nil
}

// ComputerDeviceInfo sends DEVICE_INFORMATION MDM command. Results are reported asynchronously; command is queued.
func ComputerDeviceInfo(credName string, opts Opts, computerIDs []int, queries []string, logger *slog.Logger) error {
	if len(computerIDs) == 0 {
		return errComputerIDsRequired
	}
	var cmdData map[string]interface{}
	if len(queries) > 0 {
		cmdData = map[string]interface{}{"queries": queries}
	}
	if err := computerSendMDMCommand(credName, computerIDs, "DEVICE_INFORMATION", cmdData, logger); err != nil {
		return err
	}
	fmt.Printf("Command queued for %d computer(s)\n", len(computerIDs))
	return nil
}

// ComputerDeviceLocation sends DEVICE_LOCATION MDM command. Results are reported asynchronously; command is queued.
func ComputerDeviceLocation(credName string, opts Opts, computerIDs []int, logger *slog.Logger) error {
	if len(computerIDs) == 0 {
		return errComputerIDsRequired
	}
	if err := computerSendMDMCommand(credName, computerIDs, "DEVICE_LOCATION", nil, logger); err != nil {
		return err
	}
	fmt.Printf("Command queued for %d computer(s)\n", len(computerIDs))
	return nil
}

// ComputerSecurityInfo sends SECURITY_INFO MDM command. Results are reported asynchronously; command is queued.
func ComputerSecurityInfo(credName string, opts Opts, computerIDs []int, logger *slog.Logger) error {
	if len(computerIDs) == 0 {
		return errComputerIDsRequired
	}
	if err := computerSendMDMCommand(credName, computerIDs, "SECURITY_INFO", nil, logger); err != nil {
		return err
	}
	fmt.Printf("Command queued for %d computer(s)\n", len(computerIDs))
	return nil
}

// ComputerInstalledApps sends INSTALLED_APPLICATION_LIST MDM command. Results are reported asynchronously; command is queued.
func ComputerInstalledApps(credName string, opts Opts, computerIDs []int, identifiers []string, logger *slog.Logger) error {
	if len(computerIDs) == 0 {
		return errComputerIDsRequired
	}
	var cmdData map[string]interface{}
	if len(identifiers) > 0 {
		cmdData = map[string]interface{}{"identifiers": identifiers}
	}
	if err := computerSendMDMCommand(credName, computerIDs, "INSTALLED_APPLICATION_LIST", cmdData, logger); err != nil {
		return err
	}
	fmt.Printf("Command queued for %d computer(s)\n", len(computerIDs))
	return nil
}

// ComputerManagedApps sends MANAGED_APPLICATION_LIST MDM command. Results are reported asynchronously; command is queued.
func ComputerManagedApps(credName string, opts Opts, computerIDs []int, identifiers []string, logger *slog.Logger) error {
	if len(computerIDs) == 0 {
		return errComputerIDsRequired
	}
	var cmdData map[string]interface{}
	if len(identifiers) > 0 {
		cmdData = map[string]interface{}{"identifiers": identifiers}
	}
	if err := computerSendMDMCommand(credName, computerIDs, "MANAGED_APPLICATION_LIST", cmdData, logger); err != nil {
		return err
	}
	fmt.Printf("Command queued for %d computer(s)\n", len(computerIDs))
	return nil
}

// ComputerCertificates sends CERTIFICATE_LIST MDM command. Results are reported asynchronously; command is queued.
func ComputerCertificates(credName string, opts Opts, computerIDs []int, logger *slog.Logger) error {
	if len(computerIDs) == 0 {
		return errComputerIDsRequired
	}
	if err := computerSendMDMCommand(credName, computerIDs, "CERTIFICATE_LIST", nil, logger); err != nil {
		return err
	}
	fmt.Printf("Command queued for %d computer(s)\n", len(computerIDs))
	return nil
}

// ComputerRedeploy triggers Jamf Management Framework redeploy on one or more computers.
func ComputerRedeploy(credName string, opts Opts, computerIDs []int, logger *slog.Logger) error {
	if len(computerIDs) == 0 {
		return errComputerIDsRequired
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nRedeploy Jamf Management Framework on %d computer(s): %v", len(computerIDs), computerIDs), logger) {
			return nil
		}
	}
	for _, id := range computerIDs {
		endpoint, err := ResolveEndpoint(credName, "/api/v1/jamf-management-framework/redeploy/"+strconv.Itoa(id))
		if err != nil {
			return err
		}
		req, err := http.NewRequest(http.MethodPost, endpoint, nil)
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		resp, err := PerformRequest(nil, req, credName)
		if err != nil {
			return fmt.Errorf("redeploy computer %d: %w", id, err)
		}
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
			return fmt.Errorf("redeploy computer %d status %d: %s", id, resp.StatusCode, string(respBody))
		}
	}
	fmt.Printf("Redeploy command sent to %d computer(s)\n", len(computerIDs))
	return nil
}
