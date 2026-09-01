package buddy

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
)

// Jamf Pro v2 mobile-device group API. Field names differ from computer groups:
// summary rows use groupId/groupName/groupDescription, and static membership is
// mutated with PATCH assignments [{mobileDeviceId, selected}].
const (
	uriMobileSmartGroups           = "/api/v2/mobile-device-groups/smart-groups"
	uriMobileSmartGroupMembership  = "/api/v2/mobile-device-groups/smart-group-membership"
	uriMobileStaticGroups          = "/api/v2/mobile-device-groups/static-groups"
	uriMobileStaticGroupMembership = "/api/v2/mobile-device-groups/static-group-membership"
)

// MobileGroupSummary is one row from GET /api/v2/mobile-device-groups/{smart,static}-groups.
type MobileGroupSummary struct {
	GroupID          string `json:"groupId"`
	GroupName        string `json:"groupName"`
	GroupDescription string `json:"groupDescription,omitempty"`
	SiteID           string `json:"siteId,omitempty"`
	Count            int    `json:"count,omitempty"`
}

// MobileDeviceMember is one device in a mobile group membership list.
type MobileDeviceMember struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Serial string `json:"serial"`
}

// mobileGroupMembershipRow is the subset of InventoryListMobileDevice buddy needs.
type mobileGroupMembershipRow struct {
	MobileDeviceID string `json:"mobileDeviceId"`
	DisplayName    string `json:"displayName"`
	SerialNumber   string `json:"serialNumber"`
}

func mobileGroupSearchPaged(credName, apiPath, filter string, opts PageOpts) ([]MobileGroupSummary, error) {
	normalized, err := NormalizeListFilter(filter, FilterSpecMobileGroups)
	if err != nil {
		return nil, err
	}
	extra := url.Values{}
	if normalized != "" {
		extra.Set("filter", normalized)
	}
	var all []MobileGroupSummary
	err = fetchJamfPaged(credName, apiPath, extra, opts, func(resultsJSON []byte, _ int) (int, error) {
		var page []MobileGroupSummary
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

// MobileGroupSearchSmart fetches smart mobile-device groups.
// Bare filter term → groupName=="*term*" server-side; RSQL passes through.
func MobileGroupSearchSmart(credName, filter string, opts PageOpts, _ *slog.Logger) ([]MobileGroupSummary, error) {
	return mobileGroupSearchPaged(credName, uriMobileSmartGroups, filter, opts)
}

// MobileGroupSearchStatic fetches static mobile-device groups.
func MobileGroupSearchStatic(credName, filter string, opts PageOpts, _ *slog.Logger) ([]MobileGroupSummary, error) {
	return mobileGroupSearchPaged(credName, uriMobileStaticGroups, filter, opts)
}

// MobileGroupListPrint prints a table of mobile-device groups.
func MobileGroupListPrint(groups []MobileGroupSummary) {
	fmt.Printf("%-8s  %-48s  %s\n", "ID", "Name", "Count")
	fmt.Printf("%-8s  %-48s  %s\n", "--------", "------------------------------------------------", "-----")
	for _, g := range groups {
		fmt.Printf("%-8s  %-48s  %d\n", g.GroupID, truncate(g.GroupName, 48), g.Count)
	}
	fmt.Printf("\n%d group(s)\n", len(groups))
}

// MobileGroupFetchSmartRaw fetches a smart mobile-device group by ID (raw JSON).
func MobileGroupFetchSmartRaw(credName, groupID string) ([]byte, error) {
	return jamfGET(credName, fmt.Sprintf("%s/%s", uriMobileSmartGroups, groupID))
}

// MobileGroupFetchStaticRaw fetches a static mobile-device group by ID (raw JSON).
func MobileGroupFetchStaticRaw(credName, groupID string) ([]byte, error) {
	return jamfGET(credName, fmt.Sprintf("%s/%s", uriMobileStaticGroups, groupID))
}

// MobileGroupCreateSmart creates a smart mobile-device group from a JSON body
// (groupName required; criteria optional) and returns the created id/href.
func MobileGroupCreateSmart(credName string, opts Opts, payload []byte, logger *slog.Logger) (*GroupCreateResult, error) {
	if len(payload) == 0 {
		return nil, fmt.Errorf("payload is empty")
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt("\nPOST smart mobile-device group with JSON payload", logger) {
			return nil, nil
		}
	}
	out, err := jamfPOST(credName, uriMobileSmartGroups, payload, "application/json")
	if err != nil {
		return nil, fmt.Errorf("create smart mobile-device group: %w", err)
	}
	return parseGroupCreateResult(out)
}

// MobileGroupCreateStatic creates a static mobile-device group from a JSON body
// (groupName required; assignments optional) and returns the created id/href.
func MobileGroupCreateStatic(credName string, opts Opts, payload []byte, logger *slog.Logger) (*GroupCreateResult, error) {
	if len(payload) == 0 {
		return nil, fmt.Errorf("payload is empty")
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt("\nPOST static mobile-device group with JSON payload", logger) {
			return nil, nil
		}
	}
	out, err := jamfPOST(credName, uriMobileStaticGroups, payload, "application/json")
	if err != nil {
		return nil, fmt.Errorf("create static mobile-device group: %w", err)
	}
	return parseGroupCreateResult(out)
}

// MobileGroupPutSmartRaw replaces a smart mobile-device group by ID with the given JSON payload.
// The v2 PUT requires groupName; callers usually read-modify-write the GET body.
func MobileGroupPutSmartRaw(credName string, opts Opts, groupID string, payload []byte, logger *slog.Logger) error {
	if len(payload) == 0 {
		return fmt.Errorf("payload is empty")
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nPUT smart mobile-device group (ID %s) with JSON payload", groupID), logger) {
			return nil
		}
	}
	if _, err := jamfPUT(credName, fmt.Sprintf("%s/%s", uriMobileSmartGroups, groupID), payload, "application/json"); err != nil {
		return fmt.Errorf("update smart mobile-device group: %w", err)
	}
	return nil
}

// MobileGroupDeleteSmart deletes a smart mobile-device group by ID.
func MobileGroupDeleteSmart(credName string, opts Opts, groupID string, logger *slog.Logger) error {
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nDelete smart mobile-device group (ID %s)", groupID), logger) {
			return nil
		}
	}
	if err := jamfDELETE(credName, fmt.Sprintf("%s/%s", uriMobileSmartGroups, groupID)); err != nil {
		return fmt.Errorf("delete smart mobile-device group %s: %w", groupID, err)
	}
	return nil
}

// MobileGroupDeleteStatic deletes a static mobile-device group by ID.
func MobileGroupDeleteStatic(credName string, opts Opts, groupID string, logger *slog.Logger) error {
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nDelete static mobile-device group (ID %s)", groupID), logger) {
			return nil
		}
	}
	if err := jamfDELETE(credName, fmt.Sprintf("%s/%s", uriMobileStaticGroups, groupID)); err != nil {
		return fmt.Errorf("delete static mobile-device group %s: %w", groupID, err)
	}
	return nil
}

func mobileGroupFetchMembers(credName, membershipPath, groupID string) ([]MobileDeviceMember, error) {
	var all []MobileDeviceMember
	apiPath := fmt.Sprintf("%s/%s", membershipPath, groupID)
	err := fetchJamfPaged(credName, apiPath, nil, PageOpts{Page: PageAll}, func(resultsJSON []byte, _ int) (int, error) {
		var page []mobileGroupMembershipRow
		if err := json.Unmarshal(resultsJSON, &page); err != nil {
			return 0, fmt.Errorf("parse response: %w", err)
		}
		for _, r := range page {
			all = append(all, MobileDeviceMember{ID: r.MobileDeviceID, Name: r.DisplayName, Serial: r.SerialNumber})
		}
		return len(page), nil
	})
	if err != nil {
		return nil, err
	}
	return all, nil
}

// MobileSmartGroupFetchMembers lists effective smart mobile-device group membership.
func MobileSmartGroupFetchMembers(credName, groupID string) ([]MobileDeviceMember, error) {
	return mobileGroupFetchMembers(credName, uriMobileSmartGroupMembership, groupID)
}

// MobileStaticGroupFetchMembers lists static mobile-device group membership.
func MobileStaticGroupFetchMembers(credName, groupID string) ([]MobileDeviceMember, error) {
	return mobileGroupFetchMembers(credName, uriMobileStaticGroupMembership, groupID)
}

// FilterMobileGroupMembers applies a client-side substring filter on id/name/serial.
func FilterMobileGroupMembers(members []MobileDeviceMember, filter string) []MobileDeviceMember {
	f := strings.ToLower(strings.TrimSpace(filter))
	if f == "" {
		return members
	}
	var out []MobileDeviceMember
	for _, m := range members {
		if strings.Contains(strings.ToLower(m.ID), f) ||
			strings.Contains(strings.ToLower(m.Name), f) ||
			strings.Contains(strings.ToLower(m.Serial), f) {
			out = append(out, m)
		}
	}
	return out
}

// mobileStaticGroupAssignment is the PATCH body for static membership changes.
type mobileStaticGroupAssignment struct {
	MobileDeviceID string `json:"mobileDeviceId"`
	Selected       bool   `json:"selected"`
}

// MobileStaticGroupAssign PATCHes static group membership: addIDs are selected,
// removeIDs are deselected. The v2 PATCH merges assignments server-side, so no
// read-modify-write is needed. No-op when both lists are empty.
func MobileStaticGroupAssign(credName string, opts Opts, groupID string, addIDs, removeIDs []string, logger *slog.Logger) error {
	var assignments []mobileStaticGroupAssignment
	for _, id := range normalizeAssignmentIDs(addIDs) {
		assignments = append(assignments, mobileStaticGroupAssignment{MobileDeviceID: id, Selected: true})
	}
	for _, id := range normalizeAssignmentIDs(removeIDs) {
		assignments = append(assignments, mobileStaticGroupAssignment{MobileDeviceID: id, Selected: false})
	}
	if len(assignments) == 0 {
		return nil
	}
	// PATCH requires groupName; read it from the current group.
	body, err := MobileGroupFetchStaticRaw(credName, groupID)
	if err != nil {
		return fmt.Errorf("fetch static mobile-device group %s: %w", groupID, err)
	}
	var current struct {
		GroupName string `json:"groupName"`
	}
	if err := json.Unmarshal(body, &current); err != nil {
		return fmt.Errorf("parse static mobile-device group %s: %w", groupID, err)
	}
	payload, err := json.Marshal(map[string]interface{}{
		"groupName":   current.GroupName,
		"assignments": assignments,
	})
	if err != nil {
		return fmt.Errorf("marshal assignments: %w", err)
	}
	if !opts.SkipConfirm {
		msg := fmt.Sprintf("\nUpdate static mobile-device group (ID %s): +%d / -%d device(s)", groupID, len(addIDs), len(removeIDs))
		if !ConfirmPrompt(msg, logger) {
			return nil
		}
	}
	if _, err := jamfPATCH(credName, fmt.Sprintf("%s/%s", uriMobileStaticGroups, groupID), payload, "application/json"); err != nil {
		return fmt.Errorf("update static mobile-device group %s: %w", groupID, err)
	}
	return nil
}

// MobileStaticGroupSetDevices replaces the entire membership of a static mobile-device
// group: current members not in deviceIDs are removed, missing ones are added.
func MobileStaticGroupSetDevices(credName string, opts Opts, groupID string, deviceIDs []string, logger *slog.Logger) error {
	want := make(map[string]struct{})
	for _, id := range normalizeAssignmentIDs(deviceIDs) {
		want[id] = struct{}{}
	}
	current, err := MobileStaticGroupFetchMembers(credName, groupID)
	if err != nil {
		return err
	}
	have := make(map[string]struct{}, len(current))
	var removeIDs []string
	for _, m := range current {
		have[m.ID] = struct{}{}
		if _, ok := want[m.ID]; !ok {
			removeIDs = append(removeIDs, m.ID)
		}
	}
	var addIDs []string
	for id := range want {
		if _, ok := have[id]; !ok {
			addIDs = append(addIDs, id)
		}
	}
	if len(addIDs) == 0 && len(removeIDs) == 0 {
		return nil
	}
	return MobileStaticGroupAssign(credName, opts, groupID, addIDs, removeIDs, logger)
}

// MobileSmartGroupCriteriaValue returns the criteria array of a smart mobile-device group.
func MobileSmartGroupCriteriaValue(credName, groupID string) ([]any, error) {
	body, err := MobileGroupFetchSmartRaw(credName, groupID)
	if err != nil {
		return nil, fmt.Errorf("fetch smart mobile-device group %s: %w", groupID, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("parse smart mobile-device group %s: %w", groupID, err)
	}
	criteria, ok := doc["criteria"].([]any)
	if !ok {
		return []any{}, nil
	}
	return criteria, nil
}

// MobileSmartGroupSetCriteria replaces the criteria array of a smart mobile-device group,
// preserving groupName/description/siteId (read-modify-write PUT).
func MobileSmartGroupSetCriteria(credName string, opts Opts, groupID string, criteriaJSON []byte, logger *slog.Logger) error {
	var criteria []any
	if err := json.Unmarshal(criteriaJSON, &criteria); err != nil {
		return fmt.Errorf("parse criteria JSON: %w", err)
	}
	body, err := MobileGroupFetchSmartRaw(credName, groupID)
	if err != nil {
		return fmt.Errorf("fetch smart mobile-device group %s: %w", groupID, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		return fmt.Errorf("parse smart mobile-device group %s: %w", groupID, err)
	}
	doc["criteria"] = criteria
	payload, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal smart mobile-device group: %w", err)
	}
	putOpts := opts
	putOpts.SkipConfirm = true
	if !opts.SkipConfirm {
		msg := fmt.Sprintf("\nReplace criteria on smart mobile-device group (ID %s) with %d criterion/criteria", groupID, len(criteria))
		if !ConfirmPrompt(msg, logger) {
			return nil
		}
	}
	return MobileGroupPutSmartRaw(credName, putOpts, groupID, payload, logger)
}
