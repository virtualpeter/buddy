package buddy

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
)

const (
	uriProMobileSearches       = "/api/v1/advanced-mobile-device-searches"
	uriClassicMobileSearches   = "/JSSResource/advancedmobiledevicesearches"
	classicMobileSearchWrapper = "advanced_mobile_device_search"
)

// MobileSearchSummary is one Pro advanced mobile device search list row.
type MobileSearchSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type mobileSearchListRow struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Criteria      []interface{} `json:"criteria,omitempty"`
	DisplayFields []string      `json:"displayFields,omitempty"`
	SiteID        *string       `json:"siteId,omitempty"`
}

func mobileSearchSummaryMatches(s MobileSearchSummary, filterLower string) bool {
	if filterLower == "" {
		return true
	}
	return strings.Contains(strings.ToLower(s.ID), filterLower) ||
		strings.Contains(strings.ToLower(s.Name), filterLower)
}

// MobileSearchSearch lists Pro advanced mobile device searches.
// filter is a client-side substring on id/name. page/page_size follow Jamf list builtins
// (omit page = all pages).
func MobileSearchSearch(credName, filter string, opts PageOpts, _ *slog.Logger) ([]MobileSearchSummary, error) {
	filterLower := strings.ToLower(strings.TrimSpace(filter))
	var all []MobileSearchSummary
	err := fetchJamfPaged(credName, uriProMobileSearches, nil, opts, func(resultsJSON []byte, _ int) (int, error) {
		var page []mobileSearchListRow
		if err := json.Unmarshal(resultsJSON, &page); err != nil {
			return 0, fmt.Errorf("parse advanced mobile device searches: %w", err)
		}
		for _, r := range page {
			sum := MobileSearchSummary{ID: r.ID, Name: r.Name}
			if mobileSearchSummaryMatches(sum, filterLower) {
				all = append(all, sum)
			}
		}
		return len(page), nil
	})
	if err != nil {
		return nil, err
	}
	return all, nil
}

func mobileSearchProPath(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("search-id is required")
	}
	return uriProMobileSearches + "/" + url.PathEscape(id), nil
}

func parseMobileSearchDefinition(body []byte) (map[string]interface{}, error) {
	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("parse advanced mobile device search JSON: %w", err)
	}
	return m, nil
}

// MobileSearchFetch loads one Pro advanced mobile device search (definition; no results).
func MobileSearchFetch(credName, id string) (map[string]interface{}, error) {
	path, err := mobileSearchProPath(id)
	if err != nil {
		return nil, err
	}
	body, err := jamfGET(credName, path)
	if err != nil {
		return nil, err
	}
	return parseMobileSearchDefinition(body)
}

// MobileSearchFetchByName loads a Pro search by exact name (case-insensitive list match).
func MobileSearchFetchByName(credName, name string) (map[string]interface{}, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("search name is required")
	}
	rows, err := MobileSearchSearch(credName, "", PageOpts{Page: PageAll}, nil)
	if err != nil {
		return nil, err
	}
	want := strings.ToLower(name)
	var id string
	for _, r := range rows {
		if strings.ToLower(r.Name) == want {
			if id != "" && id != r.ID {
				return nil, fmt.Errorf("multiple mobile searches named %q", name)
			}
			id = r.ID
		}
	}
	if id == "" {
		return nil, fmt.Errorf("mobile search %q not found", name)
	}
	return MobileSearchFetch(credName, id)
}

func parseMobileSearchDevices(body []byte) ([]map[string]interface{}, error) {
	v, err := decodeJSONInterface(body)
	if err != nil {
		return nil, fmt.Errorf("parse mobile search results JSON: %w", err)
	}
	if m, ok := jsonAsMap(classicPeek(v, classicMobileSearchWrapper)); ok {
		if _, has := m["mobile_devices"]; has {
			items := classicItemMaps(m["mobile_devices"], "mobile_device")
			if items == nil {
				items = []map[string]interface{}{}
			}
			return items, nil
		}
	}
	items := classicItemMaps(classicPeek(v, "mobile_devices"), "mobile_device")
	if items == nil {
		items = []map[string]interface{}{}
	}
	return items, nil
}

// MobileSearchDevices returns Classic result rows for a saved mobile search.
// Pro GET has no membership; this uses GET /JSSResource/advancedmobiledevicesearches/id/{id}.
func MobileSearchDevices(credName, id, filter string) ([]map[string]interface{}, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("search-id is required")
	}
	path := uriClassicMobileSearches + "/id/" + url.PathEscape(id)
	body, err := jamfGETWithAccept(credName, path, "application/json")
	if err != nil {
		return nil, err
	}
	items, err := parseMobileSearchDevices(body)
	if err != nil {
		return nil, err
	}
	filterLower := strings.ToLower(strings.TrimSpace(filter))
	if filterLower == "" {
		return items, nil
	}
	out := make([]map[string]interface{}, 0, len(items))
	for _, row := range items {
		if searchResultMatches(row, filterLower) {
			out = append(out, row)
		}
	}
	return out, nil
}

func mobileSearchPrepareBody(body map[string]interface{}, forCreate bool) (map[string]interface{}, error) {
	if body == nil {
		return nil, fmt.Errorf("body is required")
	}
	out := copyStringAnyMap(body)
	delete(out, "mobile_devices")
	delete(out, "computers")
	if forCreate {
		delete(out, "id")
		delete(out, "ID")
	}
	if strings.TrimSpace(classicString(out["name"])) == "" {
		return nil, fmt.Errorf("name is required")
	}
	return out, nil
}

func mobileSearchCreatedID(body []byte) string {
	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err != nil {
		return ""
	}
	if id := classicMapID(m); id != "" {
		return id
	}
	return ""
}

// MobileSearchCreate POSTs a Pro advanced mobile device search.
func MobileSearchCreate(credName string, opts Opts, body map[string]interface{}, logger *slog.Logger) (*MobileSearchSummary, error) {
	out, err := mobileSearchPrepareBody(body, true)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("marshal advanced mobile device search: %w", err)
	}
	name := classicString(out["name"])
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nCreate mobile search %q?", name), logger) {
			return nil, nil
		}
	}
	resp, err := jamfPOST(credName, uriProMobileSearches, payload, "application/json")
	if err != nil {
		return nil, fmt.Errorf("create advanced mobile device search: %w", err)
	}
	sum := &MobileSearchSummary{Name: name, ID: mobileSearchCreatedID(resp)}
	if sum.ID == "" && name != "" {
		if d, fErr := MobileSearchFetchByName(credName, name); fErr == nil {
			sum.ID = classicMapID(d)
			if n := classicMapName(d); n != "" {
				sum.Name = n
			}
		}
	}
	if sum.ID != "" {
		fmt.Printf("Created mobile search %q (ID %s)\n", sum.Name, sum.ID)
	} else {
		fmt.Printf("Created mobile search %q\n", sum.Name)
	}
	return sum, nil
}

// MobileSearchPut PUTs a Pro advanced mobile device search definition.
func MobileSearchPut(credName string, opts Opts, id string, body map[string]interface{}, logger *slog.Logger) error {
	path, err := mobileSearchProPath(id)
	if err != nil {
		return err
	}
	out, err := mobileSearchPrepareBody(body, false)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(out)
	if err != nil {
		return fmt.Errorf("marshal advanced mobile device search: %w", err)
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nPUT mobile search (ID %s)?", id), logger) {
			return nil
		}
	}
	if _, err := jamfPUT(credName, path, payload, "application/json"); err != nil {
		return fmt.Errorf("update advanced mobile device search: %w", err)
	}
	return nil
}

// MobileSearchDelete deletes one or more Pro advanced mobile device searches.
func MobileSearchDelete(credName string, opts Opts, ids []string, logger *slog.Logger) error {
	if len(ids) == 0 {
		return fmt.Errorf("at least one search-id is required")
	}
	type info struct{ id, name string }
	infos := make([]info, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			return fmt.Errorf("search-id is required")
		}
		name := id
		if d, err := MobileSearchFetch(credName, id); err == nil && classicMapName(d) != "" {
			name = classicMapName(d)
		}
		infos = append(infos, info{id: id, name: name})
	}
	if !opts.SkipConfirm {
		msg := "\nDelete mobile searches:"
		for _, p := range infos {
			msg += fmt.Sprintf("\n  %q (ID %s)", p.name, p.id)
		}
		if !ConfirmPrompt(msg, logger) {
			return nil
		}
	}
	for _, p := range infos {
		path, err := mobileSearchProPath(p.id)
		if err != nil {
			return err
		}
		if err := jamfDELETE(credName, path); err != nil {
			return fmt.Errorf("delete advanced mobile device search %s: %w", p.id, err)
		}
		fmt.Printf("Deleted mobile search %q (ID %s)\n", p.name, p.id)
	}
	return nil
}

// MobileSearchList prints a table of mobile searches, or JSON.
func MobileSearchList(credName, filter string, asJSON bool, logger *slog.Logger) error {
	rows, err := MobileSearchSearch(credName, filter, PageOpts{Page: PageAll}, logger)
	if err != nil {
		return err
	}
	if asJSON {
		return printJSONValue(rows)
	}
	fmt.Printf("%-8s  %s\n", "ID", "Name")
	fmt.Printf("%-8s  %s\n", "--------", "----------------------------------------")
	for _, r := range rows {
		fmt.Printf("%-8s  %s\n", r.ID, r.Name)
	}
	fmt.Printf("\n%d mobile search(es)\n", len(rows))
	return nil
}

// MobileSearchGet prints the Pro definition JSON.
func MobileSearchGet(credName, id string, _ *slog.Logger) error {
	d, err := MobileSearchFetch(credName, id)
	if err != nil {
		return err
	}
	return printJSONValue(d)
}

// MobileSearchGetID resolves an exact name to id.
func MobileSearchGetID(credName, name string, _ *slog.Logger) (string, error) {
	d, err := MobileSearchFetchByName(credName, name)
	if err != nil {
		return "", err
	}
	id := classicMapID(d)
	if id == "" {
		return "", fmt.Errorf("mobile search %q has no id", name)
	}
	return id, nil
}

// MobileSearchResultsPrint prints Classic result rows as a table, JSON, CSV, or jq --raw.
func MobileSearchResultsPrint(credName, id, filter string, opts SearchResultPrintOpts, _ *slog.Logger) error {
	rows, err := MobileSearchDevices(credName, id, filter)
	if err != nil {
		return err
	}
	return formatSearchResults(rows, opts, "mobile device(s)")
}
