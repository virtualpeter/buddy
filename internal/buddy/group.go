package buddy

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"sort"
	"strings"
)

// smartGroupCriteria represents a single criterion in a smart computer group.
type smartGroupCriteria struct {
	Name         string `json:"name"`
	Priority     int    `json:"priority"`
	AndOr        string `json:"andOr"`
	SearchType   string `json:"searchType"`
	Value        string `json:"value"`
	OpeningParen *bool  `json:"openingParen,omitempty"`
	ClosingParen *bool  `json:"closingParen,omitempty"`
}

// smartGroupV2 represents a smart computer group v2 API (GET response and PUT payload).
type smartGroupV2 struct {
	Name        string               `json:"name"`
	Description string               `json:"description,omitempty"`
	Criteria    []smartGroupCriteria `json:"criteria"`
	SiteId      *string              `json:"siteId,omitempty"`
}

const uriSmartGroups = "/api/v3/computer-groups/smart-groups"
const uriSmartGroupMembership = "/api/v3/computer-groups/smart-group-membership"
const uriStaticGroups = "/api/v3/computer-groups/static-groups"
const uriClassicComputerGroup = "/JSSResource/computergroups/id"

// StaticComputerMember is one computer in a static or smart group membership list.
// Pro smart-group-membership and static assignments may return IDs only (Name/Serial empty).
type StaticComputerMember struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Serial string `json:"serial"`
}

// smartGroupMembershipJSON is GET /api/v3/computer-groups/smart-group-membership/{id}.
type smartGroupMembershipJSON struct {
	Members []int64 `json:"members"`
}

// computerGroupXMLRoot is the Classic API response for GET /JSSResource/computergroups/id/{id}.
type computerGroupXMLRoot struct {
	XMLName   xml.Name `xml:"computer_group"`
	IsSmart   bool     `xml:"is_smart"`
	Computers struct {
		Size      int                      `xml:"size"`
		Computers []staticGroupComputerXML `xml:"computer"`
	} `xml:"computers"`
}

type staticGroupComputerXML struct {
	ID           int    `xml:"id"`
	Name         string `xml:"name"`
	SerialNumber string `xml:"serial_number"`
}

// GroupSummary is one entry from GET /api/v3/computer-groups/smart-groups or static-groups.
// Both endpoints share the same summary shape.
type GroupSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// groupSearchPaged fetches groups from a paginated v2 endpoint, applying an
// optional filter (bare term → name=="*term*" server-side; RSQL → pass-through).
// When opts.Page is PageAll, all pages are fetched; otherwise a single page.
func groupSearchPaged(credName, apiPath, filter string, opts PageOpts) ([]GroupSummary, error) {
	normalized, err := NormalizeListFilter(filter, FilterSpecGroups)
	if err != nil {
		return nil, err
	}
	extra := url.Values{}
	if normalized != "" {
		extra.Set("filter", normalized)
	}
	var all []GroupSummary
	err = fetchJamfPaged(credName, apiPath, extra, opts, func(resultsJSON []byte, _ int) (int, error) {
		var page []GroupSummary
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

// GroupSearchSmart fetches smart computer groups from the v2 API.
func GroupSearchSmart(credName, filter string, opts PageOpts, _ *slog.Logger) ([]GroupSummary, error) {
	return groupSearchPaged(credName, uriSmartGroups, filter, opts)
}

// GroupSearchStatic fetches static computer groups from the v2 API.
func GroupSearchStatic(credName, filter string, opts PageOpts, _ *slog.Logger) ([]GroupSummary, error) {
	return groupSearchPaged(credName, uriStaticGroups, filter, opts)
}

// GroupFetchSmartRaw fetches a smart group by ID and returns the raw JSON bytes (for script/component use).
func GroupFetchSmartRaw(credName, groupID string) ([]byte, error) {
	return jamfGET(credName, fmt.Sprintf("%s/%s", uriSmartGroups, groupID))
}

// GroupCreateResult is the Jamf HrefResponse returned when creating a computer group.
type GroupCreateResult struct {
	ID   string `json:"id"`
	Href string `json:"href,omitempty"`
}

func parseGroupCreateResult(body []byte) (*GroupCreateResult, error) {
	var out GroupCreateResult
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("parse create response: %w", err)
	}
	if strings.TrimSpace(out.ID) == "" {
		return nil, fmt.Errorf("create response missing id: %s", string(body))
	}
	return &out, nil
}

// GroupCreateSmartRaw creates a smart group using the v2 API and returns the raw JSON response.
// Callers typically include id: -1 in the JSON body (Jamf create convention).
func GroupCreateSmartRaw(credName string, opts Opts, payload []byte, logger *slog.Logger) ([]byte, error) {
	if len(payload) == 0 {
		return nil, fmt.Errorf("payload is empty")
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt("\nPOST smart group with JSON payload", logger) {
			return nil, nil
		}
	}
	out, err := jamfPOST(credName, uriSmartGroups, payload, "application/json")
	if err != nil {
		return nil, fmt.Errorf("create smart group: %w", err)
	}
	return out, nil
}

// GroupCreateSmart creates a smart computer group and returns the created id/href.
func GroupCreateSmart(credName string, opts Opts, payload []byte, logger *slog.Logger) (*GroupCreateResult, error) {
	out, err := GroupCreateSmartRaw(credName, opts, payload, logger)
	if err != nil {
		return nil, err
	}
	if out == nil {
		return nil, nil // cancelled
	}
	return parseGroupCreateResult(out)
}

// GroupCreateStaticRaw creates a static group using the v2 API and returns the raw JSON response.
func GroupCreateStaticRaw(credName string, opts Opts, payload []byte, logger *slog.Logger) ([]byte, error) {
	if len(payload) == 0 {
		return nil, fmt.Errorf("payload is empty")
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt("\nPOST static group with JSON payload", logger) {
			return nil, nil
		}
	}
	out, err := jamfPOST(credName, uriStaticGroups, payload, "application/json")
	if err != nil {
		return nil, fmt.Errorf("create static group: %w", err)
	}
	return out, nil
}

// GroupCreateStatic creates a static computer group and returns the created id/href.
func GroupCreateStatic(credName string, opts Opts, payload []byte, logger *slog.Logger) (*GroupCreateResult, error) {
	out, err := GroupCreateStaticRaw(credName, opts, payload, logger)
	if err != nil {
		return nil, err
	}
	if out == nil {
		return nil, nil // cancelled
	}
	return parseGroupCreateResult(out)
}

// GroupCreateSmartNamed creates a smart group from name, optional description, and criteria JSON array.
// criteriaJSON may be empty or "[]"; when non-empty it must be a JSON array of criterion objects.
func GroupCreateSmartNamed(credName string, opts Opts, name, description string, criteriaJSON []byte, logger *slog.Logger) (*GroupCreateResult, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	var criteria []smartGroupCriteria
	if len(criteriaJSON) > 0 && strings.TrimSpace(string(criteriaJSON)) != "" {
		if err := json.Unmarshal(criteriaJSON, &criteria); err != nil {
			return nil, fmt.Errorf("parse criteria JSON: %w", err)
		}
	}
	if criteria == nil {
		criteria = []smartGroupCriteria{}
	}
	body := map[string]interface{}{
		"id":       -1,
		"name":     name,
		"criteria": criteria,
	}
	if d := strings.TrimSpace(description); d != "" {
		body["description"] = d
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal smart group: %w", err)
	}
	if !opts.SkipConfirm {
		msg := fmt.Sprintf("\nCreate smart group %q (%d criterion/criteria)", name, len(criteria))
		if !ConfirmPrompt(msg, logger) {
			return nil, nil
		}
	}
	opts.SkipConfirm = true
	return GroupCreateSmart(credName, opts, payload, logger)
}

// GroupCreateStaticNamed creates a static group from name, optional description, and computer IDs.
func GroupCreateStaticNamed(credName string, opts Opts, name, description string, computerIDs []string, logger *slog.Logger) (*GroupCreateResult, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	assignments := normalizeAssignmentIDs(computerIDs)
	body := map[string]interface{}{
		"name":        name,
		"assignments": assignments,
	}
	if d := strings.TrimSpace(description); d != "" {
		body["description"] = d
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal static group: %w", err)
	}
	if !opts.SkipConfirm {
		msg := fmt.Sprintf("\nCreate static group %q (%d computer(s))", name, len(assignments))
		if !ConfirmPrompt(msg, logger) {
			return nil, nil
		}
	}
	opts.SkipConfirm = true
	return GroupCreateStatic(credName, opts, payload, logger)
}

// GroupPutSmartRaw updates a smart group by ID using the provided raw JSON payload (for script/component use).
func GroupPutSmartRaw(credName string, opts Opts, groupID string, payload []byte, logger *slog.Logger) error {
	if len(payload) == 0 {
		return fmt.Errorf("payload is empty")
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nPUT smart group (ID %s) with JSON payload", groupID), logger) {
			return nil
		}
	}
	if _, err := jamfPUT(credName, fmt.Sprintf("%s/%s", uriSmartGroups, groupID), payload, "application/json"); err != nil {
		return fmt.Errorf("update smart group: %w", err)
	}
	return nil
}

func printGroupTable(groups []GroupSummary) {
	fmt.Printf("%-8s  %s\n", "ID", "Name")
	fmt.Printf("%-8s  %s\n", "--------", "----------------------------------------")
	for _, g := range groups {
		fmt.Printf("%-8s  %s\n", g.ID, g.Name)
	}
	fmt.Printf("\n%d group(s)\n", len(groups))
}

// GroupListSmart fetches and prints smart computer groups (table, or JSON rows).
func GroupListSmart(credName, filter string, asJSON bool, logger *slog.Logger) error {
	groups, err := GroupSearchSmart(credName, filter, PageOpts{Page: PageAll}, logger)
	if err != nil {
		return err
	}
	if asJSON {
		return printJSONValue(groups)
	}
	printGroupTable(groups)
	return nil
}

// GroupListStatic fetches and prints static computer groups (table, or JSON rows).
func GroupListStatic(credName, filter string, asJSON bool, logger *slog.Logger) error {
	groups, err := GroupSearchStatic(credName, filter, PageOpts{Page: PageAll}, logger)
	if err != nil {
		return err
	}
	if asJSON {
		return printJSONValue(groups)
	}
	printGroupTable(groups)
	return nil
}

// GroupFindID finds a smart computer group by name.
// Prefers an exact case-insensitive match; falls back to substring match.
// Returns empty id and name if no match is found.
func GroupFindID(credName string, name string, logger *slog.Logger) (id string, groupName string, err error) {
	records, err := GroupSearchSmart(credName, name, PageOpts{Page: PageAll}, logger)
	if err != nil {
		return "", "", err
	}
	for _, g := range records {
		if strings.EqualFold(strings.TrimSpace(g.Name), name) {
			return g.ID, g.Name, nil
		}
	}
	if len(records) > 0 {
		return records[0].ID, records[0].Name, nil
	}
	return "", "", nil
}

// GroupGetVersion fetches smart group and prints the current version criterion value.
func GroupGetVersion(credName string, opts Opts, groupID string, logger *slog.Logger) error {
	version, err := GroupGetVersionValue(credName, groupID, logger)
	if err != nil {
		return err
	}
	fmt.Println(version)
	return nil
}

// GroupGetVersionValue fetches smart group and returns the version criterion value (for script use).
func GroupGetVersionValue(credName string, groupID string, logger *slog.Logger) (string, error) {
	body, err := jamfGET(credName, fmt.Sprintf("%s/%s", uriSmartGroups, groupID))
	if err != nil {
		return "", fmt.Errorf("fetch smart group %s: %w", groupID, err)
	}
	var group smartGroupV2
	if err := json.Unmarshal(body, &group); err != nil {
		return "", fmt.Errorf("parse smart group %s: %w", groupID, err)
	}
	idx := findVersionCriterion(group.Criteria)
	if idx < 0 {
		return "", fmt.Errorf("no criterion with name containing 'version' and a version-like value found in %q", group.Name)
	}
	return group.Criteria[idx].Value, nil
}

// GroupVersion updates the version criterion in a smart computer group.
func GroupVersion(credName string, opts Opts, groupID, newVersion string, logger *slog.Logger) error {
	body, err := jamfGET(credName, fmt.Sprintf("%s/%s", uriSmartGroups, groupID))
	if err != nil {
		return fmt.Errorf("fetch smart group %s: %w", groupID, err)
	}
	var group smartGroupV2
	if err := json.Unmarshal(body, &group); err != nil {
		return fmt.Errorf("parse smart group %s: %w", groupID, err)
	}
	idx := findVersionCriterion(group.Criteria)
	if idx < 0 {
		return fmt.Errorf("no criterion with name containing 'version' and a version-like value found in %q", group.Name)
	}
	oldVersion := group.Criteria[idx].Value
	if oldVersion == newVersion {
		fmt.Printf("Smart group %q (ID %s) already has version %s\n", group.Name, groupID, newVersion)
		return nil
	}
	group.Criteria[idx].Value = newVersion
	if !opts.SkipConfirm {
		msg := fmt.Sprintf("\nUpdate smart group %q (ID %s):\n  %s %s -> %s",
			group.Name, groupID, group.Criteria[idx].Name, oldVersion, newVersion)
		if !ConfirmPrompt(msg, logger) {
			return nil
		}
	}
	if opts.Verbose {
		jsonPayload, err := json.MarshalIndent(group, "", "  ")
		if err != nil {
			logger.Warn("Failed to marshal payload for verbose output", "error", err)
		} else {
			fmt.Fprintf(os.Stderr, "\n--- PUT payload (smart-groups/%s) ---\n%s\n--- end payload ---\n\n", groupID, jsonPayload)
		}
	}
	payloadBytes, err := json.Marshal(group)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	if _, err := jamfPUT(credName, fmt.Sprintf("%s/%s", uriSmartGroups, groupID), payloadBytes, "application/json"); err != nil {
		return fmt.Errorf("update smart group: %w", err)
	}
	fmt.Printf("Updated smart group %q (ID %s) %s: %s -> %s\n", group.Name, groupID, group.Criteria[idx].Name, oldVersion, newVersion)
	return nil
}

// GroupGet prints a computer group JSON by ID: smart group first, static fallback.
func GroupGet(credName, groupID string, logger *slog.Logger) error {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return fmt.Errorf("group id is required")
	}
	body, smartErr := jamfGET(credName, fmt.Sprintf("%s/%s", uriSmartGroups, url.PathEscape(groupID)))
	if smartErr != nil {
		var staticErr error
		body, staticErr = jamfGET(credName, fmt.Sprintf("%s/%s", uriStaticGroups, url.PathEscape(groupID)))
		if staticErr != nil {
			return fmt.Errorf("group %s not found as smart group (%v) or static group (%v)", groupID, smartErr, staticErr)
		}
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, body, "", "  "); err != nil {
		fmt.Println(string(body))
		return nil
	}
	fmt.Println(pretty.String())
	return nil
}

// GroupDelete deletes a smart computer group by ID.
func GroupDelete(credName string, opts Opts, groupID string, logger *slog.Logger) error {
	body, err := jamfGET(credName, fmt.Sprintf("%s/%s", uriSmartGroups, groupID))
	if err != nil {
		return fmt.Errorf("fetch smart group %s: %w", groupID, err)
	}
	var group smartGroupV2
	if err := json.Unmarshal(body, &group); err != nil {
		return fmt.Errorf("parse smart group %s: %w", groupID, err)
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nDelete smart group %q (ID %s)", group.Name, groupID), logger) {
			return nil
		}
	}
	if err := jamfDELETE(credName, fmt.Sprintf("%s/%s", uriSmartGroups, groupID)); err != nil {
		return fmt.Errorf("delete smart group: %w", err)
	}
	fmt.Printf("Deleted smart group %q (ID %s)\n", group.Name, groupID)
	return nil
}

// parseSmartGroupMembership parses SmartGroupMembership JSON ({"members":[1,2,…]}).
func parseSmartGroupMembership(body []byte) ([]StaticComputerMember, error) {
	var raw smartGroupMembershipJSON
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse smart group membership: %w", err)
	}
	out := make([]StaticComputerMember, 0, len(raw.Members))
	for _, id := range raw.Members {
		if id <= 0 {
			continue
		}
		out = append(out, StaticComputerMember{ID: fmt.Sprintf("%d", id)})
	}
	return out, nil
}

// SmartGroupFetchMembers returns effective membership for a smart group via
// GET /api/v3/computer-groups/smart-group-membership/{id} (computer IDs only).
func SmartGroupFetchMembers(credName, groupID string) ([]StaticComputerMember, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, fmt.Errorf("group id is required")
	}
	body, err := jamfGET(credName, fmt.Sprintf("%s/%s", uriSmartGroupMembership, url.PathEscape(groupID)))
	if err != nil {
		return nil, fmt.Errorf("fetch smart group membership %s: %w", groupID, err)
	}
	return parseSmartGroupMembership(body)
}

// StaticGroupFetchMembers returns computers in a static group: v2 GET first if JSON includes
// "assignments"; otherwise Classic GET /JSSResource/computergroups/id/{id}.
func StaticGroupFetchMembers(credName, groupID string) ([]StaticComputerMember, error) {
	body, err := jamfGET(credName, fmt.Sprintf("%s/%s", uriStaticGroups, url.PathEscape(groupID)))
	if err != nil {
		return nil, fmt.Errorf("fetch static group %s: %w", groupID, err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse static group %s: %w", groupID, err)
	}
	if assignRaw, ok := raw["assignments"]; ok {
		var ids []string
		if err := json.Unmarshal(assignRaw, &ids); err != nil {
			return nil, fmt.Errorf("parse static group %s assignments: %w", groupID, err)
		}
		out := make([]StaticComputerMember, 0, len(ids))
		for _, id := range ids {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			out = append(out, StaticComputerMember{ID: id})
		}
		return out, nil
	}
	return staticGroupFetchMembersClassic(credName, groupID)
}

func staticGroupFetchMembersClassic(credName, groupID string) ([]StaticComputerMember, error) {
	path := fmt.Sprintf("%s/%s", uriClassicComputerGroup, url.PathEscape(groupID))
	body, err := jamfGETWithAccept(credName, path, "application/xml")
	if err != nil {
		return nil, fmt.Errorf("fetch classic computer group %s: %w", groupID, err)
	}
	var root computerGroupXMLRoot
	if err := xml.Unmarshal(body, &root); err != nil {
		return nil, fmt.Errorf("parse computer group XML %s: %w", groupID, err)
	}
	out := make([]StaticComputerMember, 0, len(root.Computers.Computers))
	for _, c := range root.Computers.Computers {
		if c.ID <= 0 {
			continue
		}
		idStr := fmt.Sprintf("%d", c.ID)
		out = append(out, StaticComputerMember{
			ID:     idStr,
			Name:   strings.TrimSpace(c.Name),
			Serial: strings.TrimSpace(c.SerialNumber),
		})
	}
	return out, nil
}

// FilterStaticGroupMembers applies a client-side substring filter on id, name, and serial (case-insensitive).
func FilterStaticGroupMembers(members []StaticComputerMember, filter string) []StaticComputerMember {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return members
	}
	fl := strings.ToLower(filter)
	var out []StaticComputerMember
	for _, m := range members {
		if strings.Contains(strings.ToLower(m.ID), fl) ||
			strings.Contains(strings.ToLower(m.Name), fl) ||
			strings.Contains(strings.ToLower(m.Serial), fl) {
			out = append(out, m)
		}
	}
	return out
}

// staticGroupPutAllowedKeys are the only JSON keys sent on PUT (per Jamf StaticComputerGroupAssignment).
// Extra keys from GET (e.g. HATEOAS) are dropped to avoid server errors.
var staticGroupPutAllowedKeys = map[string]struct{}{
	"id": {}, "name": {}, "description": {}, "siteId": {}, "assignments": {},
}

// StaticGroupPutAssignments replaces the full membership list for a static group (v2 PUT).
func StaticGroupPutAssignments(credName, groupID string, assignments []string) error {
	path := fmt.Sprintf("%s/%s", uriStaticGroups, url.PathEscape(groupID))
	body, err := jamfGET(credName, path)
	if err != nil {
		return fmt.Errorf("fetch static group %s: %w", groupID, err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return fmt.Errorf("parse static group %s: %w", groupID, err)
	}
	var name string
	if err := json.Unmarshal(raw["name"], &name); err != nil || strings.TrimSpace(name) == "" {
		return fmt.Errorf("static group %s: missing name in API response", groupID)
	}
	normalized := normalizeAssignmentIDs(assignments)
	assignmentsJSON, err := json.Marshal(normalized)
	if err != nil {
		return fmt.Errorf("marshal assignments: %w", err)
	}
	put := make(map[string]json.RawMessage)
	for k := range staticGroupPutAllowedKeys {
		if v, ok := raw[k]; ok {
			put[k] = v
		}
	}
	put["assignments"] = assignmentsJSON
	if _, ok := put["id"]; !ok {
		idJSON, err := json.Marshal(groupID)
		if err != nil {
			return fmt.Errorf("marshal group id: %w", err)
		}
		put["id"] = idJSON
	}
	put["name"], err = json.Marshal(name)
	if err != nil {
		return fmt.Errorf("marshal name: %w", err)
	}
	payloadBytes, err := json.Marshal(put)
	if err != nil {
		return fmt.Errorf("marshal static group PUT: %w", err)
	}
	if _, err := jamfPUT(credName, path, payloadBytes, "application/json"); err != nil {
		return fmt.Errorf("update static group %s membership: %w", groupID, err)
	}
	return nil
}

func normalizeAssignmentIDs(ids []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Strings(out)
	if out == nil {
		return []string{}
	}
	return out
}

func staticGroupMergeIDs(existing []StaticComputerMember, add []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, m := range existing {
		id := strings.TrimSpace(m.ID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	for _, a := range add {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		if _, ok := seen[a]; !ok {
			seen[a] = struct{}{}
			out = append(out, a)
		}
	}
	sort.Strings(out)
	if out == nil {
		return []string{}
	}
	return out
}

func staticGroupSubtractIDs(existing []StaticComputerMember, remove []string) []string {
	removeSet := make(map[string]struct{})
	for _, r := range remove {
		r = strings.TrimSpace(r)
		if r != "" {
			removeSet[r] = struct{}{}
		}
	}
	var out []string
	for _, m := range existing {
		id := strings.TrimSpace(m.ID)
		if id == "" {
			continue
		}
		if _, drop := removeSet[id]; drop {
			continue
		}
		out = append(out, id)
	}
	sort.Strings(out)
	if out == nil {
		return []string{}
	}
	return out
}

// StaticGroupAddComputers adds computer IDs to a static group (read-modify-write via v2 PUT).
// The bool is true if the group's membership was updated on the server.
func StaticGroupAddComputers(credName string, opts Opts, groupID string, computerIDs []string, logger *slog.Logger) (bool, error) {
	members, err := StaticGroupFetchMembers(credName, groupID)
	if err != nil {
		return false, err
	}
	next := staticGroupMergeIDs(members, computerIDs)
	if len(next) == len(members) {
		return false, nil
	}
	if !opts.SkipConfirm {
		msg := fmt.Sprintf("\nAdd computers to static group ID %s (IDs: %v)", groupID, computerIDs)
		if !ConfirmPrompt(msg, logger) {
			return false, nil
		}
	}
	if err := StaticGroupPutAssignments(credName, groupID, next); err != nil {
		return false, err
	}
	return true, nil
}

// StaticGroupRemoveComputers removes computer IDs from a static group.
// The bool is true if the group's membership was updated on the server.
func StaticGroupRemoveComputers(credName string, opts Opts, groupID string, computerIDs []string, logger *slog.Logger) (bool, error) {
	members, err := StaticGroupFetchMembers(credName, groupID)
	if err != nil {
		return false, err
	}
	next := staticGroupSubtractIDs(members, computerIDs)
	if len(next) == len(members) {
		return false, nil
	}
	if !opts.SkipConfirm {
		msg := fmt.Sprintf("\nRemove computers from static group ID %s (IDs: %v)", groupID, computerIDs)
		if !ConfirmPrompt(msg, logger) {
			return false, nil
		}
	}
	if err := StaticGroupPutAssignments(credName, groupID, next); err != nil {
		return false, err
	}
	return true, nil
}

// StaticGroupSetComputers replaces the static group membership with the given computer IDs.
// The bool is true if a PUT was performed (false if the user declined confirmation).
func StaticGroupSetComputers(credName string, opts Opts, groupID string, computerIDs []string, logger *slog.Logger) (bool, error) {
	if !opts.SkipConfirm {
		msg := fmt.Sprintf("\nReplace static group ID %s membership with %d computer(s)", groupID, len(normalizeAssignmentIDs(computerIDs)))
		if !ConfirmPrompt(msg, logger) {
			return false, nil
		}
	}
	if err := StaticGroupPutAssignments(credName, groupID, computerIDs); err != nil {
		return false, err
	}
	return true, nil
}

// GroupStaticListMembers prints static group members as a table.
func GroupStaticListMembers(credName, groupID, filter string, _ *slog.Logger) error {
	members, err := StaticGroupFetchMembers(credName, groupID)
	if err != nil {
		return err
	}
	printGroupMembersTable(FilterStaticGroupMembers(members, filter))
	return nil
}

// GroupSmartListMembers prints smart group effective members as a table.
func GroupSmartListMembers(credName, groupID, filter string, _ *slog.Logger) error {
	members, err := SmartGroupFetchMembers(credName, groupID)
	if err != nil {
		return err
	}
	printGroupMembersTable(FilterStaticGroupMembers(members, filter))
	return nil
}

// GroupSmartPrintCriteria pretty-prints the smart group's criteria JSON array.
func GroupSmartPrintCriteria(credName, groupID string, _ *slog.Logger) error {
	crit, err := SmartGroupCriteriaValue(credName, groupID)
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(crit, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal criteria: %w", err)
	}
	fmt.Println(string(raw))
	return nil
}

func printGroupMembersTable(members []StaticComputerMember) {
	fmt.Printf("%-12s  %-36s  %s\n", "ID", "Name", "Serial")
	fmt.Printf("%-12s  %-36s  %s\n", "------------", "------------------------------------", "------------------------------")
	for _, m := range members {
		name := m.Name
		if name == "" {
			name = "—"
		}
		serial := m.Serial
		if serial == "" {
			serial = "—"
		}
		fmt.Printf("%-12s  %-36s  %s\n", m.ID, truncateRunes(name, 36), truncateRunes(serial, 28))
	}
	fmt.Printf("\n%d computer(s)\n", len(members))
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

// SmartGroupResolveID returns a smart group ID from either id or name (exactly one required).
func SmartGroupResolveID(credName, id, name string, logger *slog.Logger) (string, error) {
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	if (id == "") == (name == "") {
		return "", fmt.Errorf("pass exactly one of id or name")
	}
	if id != "" {
		return id, nil
	}
	found, _, err := GroupFindID(credName, name, logger)
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("no smart group found for name %q", name)
	}
	return found, nil
}

// SmartGroupFetchValue fetches a smart group by ID and returns the full JSON as a map.
func SmartGroupFetchValue(credName, groupID string) (map[string]any, error) {
	body, err := GroupFetchSmartRaw(credName, groupID)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("parse smart group %s: %w", groupID, err)
	}
	return out, nil
}

// SmartGroupCriteriaValue returns the criteria array from a smart group (empty list if missing).
func SmartGroupCriteriaValue(credName, groupID string) ([]any, error) {
	root, err := SmartGroupFetchValue(credName, groupID)
	if err != nil {
		return nil, err
	}
	raw, ok := root["criteria"]
	if !ok || raw == nil {
		return []any{}, nil
	}
	lst, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("smart group %s: criteria is not an array", groupID)
	}
	return lst, nil
}

// SmartGroupSetCriteria replaces the criteria array on a smart group (fetch → replace → PUT).
// criteriaJSON must be a JSON array (same shape as smart_group_criteria / buddy group smart criteria).
func SmartGroupSetCriteria(credName string, opts Opts, groupID string, criteriaJSON []byte, logger *slog.Logger) error {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return fmt.Errorf("group id is required")
	}
	criteriaJSON = bytes.TrimSpace(criteriaJSON)
	if len(criteriaJSON) == 0 {
		return fmt.Errorf("criteria JSON is empty")
	}
	var criteria []any
	if err := json.Unmarshal(criteriaJSON, &criteria); err != nil {
		return fmt.Errorf("criteria must be a JSON array: %w", err)
	}
	root, err := SmartGroupFetchValue(credName, groupID)
	if err != nil {
		return err
	}
	name, _ := root["name"].(string)
	root["criteria"] = criteria
	payload, err := json.Marshal(root)
	if err != nil {
		return fmt.Errorf("marshal smart group: %w", err)
	}
	if !opts.SkipConfirm {
		label := name
		if label == "" {
			label = groupID
		} else {
			label = fmt.Sprintf("%q (ID %s)", label, groupID)
		}
		msg := fmt.Sprintf("\nReplace criteria on smart group %s (%d criterion(s))", label, len(criteria))
		if !ConfirmPrompt(msg, logger) {
			return nil
		}
	}
	opts.SkipConfirm = true
	if err := GroupPutSmartRaw(credName, opts, groupID, payload, logger); err != nil {
		return err
	}
	fmt.Printf("Updated smart group %s criteria (%d criterion(s))\n", groupID, len(criteria))
	return nil
}

// GroupSmartSetCriteria is the CLI entry for smart set-criteria.
func GroupSmartSetCriteria(credName string, opts Opts, groupID string, criteriaJSON []byte, logger *slog.Logger) error {
	return SmartGroupSetCriteria(credName, opts, groupID, criteriaJSON, logger)
}

// SmartGroupUpdateCriterionValue sets value on every criterion whose name contains nameSubstring
// (case-insensitive). Preserves the full group JSON document. Fails if no criteria match.
func SmartGroupUpdateCriterionValue(credName string, opts Opts, groupID, nameSubstring, newValue string, logger *slog.Logger) error {
	body, err := GroupFetchSmartRaw(credName, groupID)
	if err != nil {
		return err
	}
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return fmt.Errorf("parse smart group %s: %w", groupID, err)
	}
	raw, ok := root["criteria"]
	if !ok || raw == nil {
		return fmt.Errorf("smart group has no criteria array")
	}
	lst, ok := raw.([]any)
	if !ok {
		return fmt.Errorf("smart group criteria is not an array")
	}
	nameLower := strings.ToLower(strings.TrimSpace(nameSubstring))
	changed := 0
	for _, item := range lst {
		cd, ok := item.(map[string]any)
		if !ok {
			continue
		}
		nameStr, _ := cd["name"].(string)
		if strings.Contains(strings.ToLower(nameStr), nameLower) {
			cd["value"] = newValue
			changed++
		}
	}
	if changed == 0 {
		return fmt.Errorf("no criteria found with name containing %q", nameSubstring)
	}
	root["criteria"] = lst
	payload, err := json.Marshal(root)
	if err != nil {
		return fmt.Errorf("marshal smart group: %w", err)
	}
	return GroupPutSmartRaw(credName, opts, groupID, payload, logger)
}

func findVersionCriterion(criteria []smartGroupCriteria) int {
	lower := strings.ToLower
	for i, c := range criteria {
		name := lower(c.Name)
		if strings.Contains(name, "version") && looksLikeVersion(c.Value) {
			return i
		}
	}
	return -1
}

func looksLikeVersion(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r != '.' && (r < '0' || r > '9') {
			return false
		}
	}
	return strings.Contains(s, ".")
}
