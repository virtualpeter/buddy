package buddy

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
)

const (
	uriClassicComputerSearches = "/JSSResource/advancedcomputersearches"
	uriClassicComputerReports  = "/JSSResource/computerreports"
	classicACSWrapper          = "advanced_computer_search"
)

// ComputerSearchSummary is one row from Classic advancedcomputersearches list.
type ComputerSearchSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type computerSearchListXML struct {
	XMLName xml.Name `xml:"advanced_computer_searches"`
	Items   []struct {
		ID   int    `xml:"id"`
		Name string `xml:"name"`
	} `xml:"advanced_computer_search"`
}

// ComputerSearchSearch lists Classic advanced computer searches.
// filter is a client-side substring matched against id and name.
func ComputerSearchSearch(credName, filter string, _ *slog.Logger) ([]ComputerSearchSummary, error) {
	body, err := jamfGETWithAccept(credName, uriClassicComputerSearches, "application/json")
	if err != nil {
		body, err = jamfGETWithAccept(credName, uriClassicComputerSearches, "application/xml")
		if err != nil {
			return nil, err
		}
	}
	rows, err := parseComputerSearchList(body)
	if err != nil {
		return nil, err
	}
	filterLower := strings.ToLower(strings.TrimSpace(filter))
	if filterLower == "" {
		return rows, nil
	}
	out := make([]ComputerSearchSummary, 0, len(rows))
	for _, r := range rows {
		if strings.Contains(strings.ToLower(r.ID), filterLower) || strings.Contains(strings.ToLower(r.Name), filterLower) {
			out = append(out, r)
		}
	}
	return out, nil
}

func parseComputerSearchList(body []byte) ([]ComputerSearchSummary, error) {
	if v, err := decodeJSONInterface(body); err == nil {
		items := classicItemMaps(classicPeek(v, "advanced_computer_searches", "computer_reports"), "advanced_computer_search", "computer_report")
		if items != nil {
			out := make([]ComputerSearchSummary, 0, len(items))
			for _, m := range items {
				out = append(out, ComputerSearchSummary{ID: classicMapID(m), Name: classicMapName(m)})
			}
			return out, nil
		}
	}
	var list computerSearchListXML
	if err := xml.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("parse advanced computer searches list: %w", err)
	}
	out := make([]ComputerSearchSummary, 0, len(list.Items))
	for _, it := range list.Items {
		out = append(out, ComputerSearchSummary{ID: classicString(it.ID), Name: it.Name})
	}
	return out, nil
}

func computerSearchPath(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("search-id is required")
	}
	return uriClassicComputerSearches + "/id/" + url.PathEscape(id), nil
}

func computerSearchNamePath(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("search name is required")
	}
	return uriClassicComputerSearches + "/name/" + url.PathEscape(name), nil
}

func parseComputerSearchDefinition(body []byte) (map[string]interface{}, error) {
	v, err := decodeJSONInterface(body)
	if err != nil {
		return nil, fmt.Errorf("parse advanced computer search JSON: %w", err)
	}
	v = classicPeek(v, classicACSWrapper)
	m, ok := jsonAsMap(v)
	if !ok {
		return nil, fmt.Errorf("parse advanced computer search JSON: expected object")
	}
	out := copyStringAnyMap(m)
	delete(out, "computers")
	return out, nil
}

func parseComputerSearchComputers(body []byte) ([]map[string]interface{}, error) {
	v, err := decodeJSONInterface(body)
	if err != nil {
		return nil, fmt.Errorf("parse computer search results JSON: %w", err)
	}
	// computerreports: {computer_reports|computer_report → computers/computer}
	// ACS: {advanced_computer_search.computers → computer[]}
	if m, ok := jsonAsMap(classicPeek(v, classicACSWrapper)); ok {
		if _, has := m["computers"]; has {
			items := classicItemMaps(m["computers"], "computer")
			if items == nil {
				items = []map[string]interface{}{}
			}
			return items, nil
		}
	}
	items := classicItemMaps(classicPeek(v, "computer_reports", "computer_report", "computers"), "computer", "Computer")
	if items == nil {
		items = []map[string]interface{}{}
	}
	return items, nil
}

func computerSearchGET(credName, path string) ([]byte, error) {
	return jamfGETWithAccept(credName, path, "application/json")
}

// ComputerSearchFetch loads one advanced computer search by id (definition only; computers stripped).
func ComputerSearchFetch(credName, id string) (map[string]interface{}, error) {
	path, err := computerSearchPath(id)
	if err != nil {
		return nil, err
	}
	body, err := computerSearchGET(credName, path)
	if err != nil {
		return nil, err
	}
	return parseComputerSearchDefinition(body)
}

// ComputerSearchFetchByName loads one advanced computer search by exact Classic name.
func ComputerSearchFetchByName(credName, name string) (map[string]interface{}, error) {
	path, err := computerSearchNamePath(name)
	if err != nil {
		return nil, err
	}
	body, err := computerSearchGET(credName, path)
	if err != nil {
		return nil, err
	}
	return parseComputerSearchDefinition(body)
}

// ComputerSearchComputers returns result rows for a saved search.
// Prefers Classic computerreports (results-only); falls back to ACS computers.
// filter is a client-side substring on id/name/serial/udid.
func ComputerSearchComputers(credName, id, filter string) ([]map[string]interface{}, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("search-id is required")
	}
	var items []map[string]interface{}
	reportBody, err := computerSearchGET(credName, uriClassicComputerReports+"/id/"+url.PathEscape(id))
	if err == nil {
		items, err = parseComputerSearchComputers(reportBody)
	}
	if err != nil {
		path, pErr := computerSearchPath(id)
		if pErr != nil {
			return nil, pErr
		}
		body, gErr := computerSearchGET(credName, path)
		if gErr != nil {
			if err != nil {
				return nil, err
			}
			return nil, gErr
		}
		items, err = parseComputerSearchComputers(body)
		if err != nil {
			return nil, err
		}
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

func computerSearchSummaryFromDef(m map[string]interface{}) ComputerSearchSummary {
	return ComputerSearchSummary{ID: classicMapID(m), Name: classicMapName(m)}
}

// ComputerSearchCreate POSTs a Classic advanced computer search JSON body.
func ComputerSearchCreate(credName string, opts Opts, body map[string]interface{}, logger *slog.Logger) (*ComputerSearchSummary, error) {
	wrapped, err := prepareClassicSearchBody(classicACSWrapper, body, true)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(wrapped)
	if err != nil {
		return nil, fmt.Errorf("marshal advanced computer search: %w", err)
	}
	inner, _ := jsonAsMap(wrapped[classicACSWrapper])
	name := classicMapName(inner)
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nCreate computer search %q?", name), logger) {
			return nil, nil
		}
	}
	resp, err := jamfPOST(credName, uriClassicComputerSearches+"/id/0", payload, "application/json")
	if err != nil {
		return nil, fmt.Errorf("create advanced computer search: %w", err)
	}
	sum := &ComputerSearchSummary{Name: name, ID: classicCreatedID(resp, classicACSWrapper)}
	if sum.ID == "" && name != "" {
		if d, fErr := ComputerSearchFetchByName(credName, name); fErr == nil {
			*sum = computerSearchSummaryFromDef(d)
		}
	}
	if sum.ID != "" {
		fmt.Printf("Created computer search %q (ID %s)\n", sum.Name, sum.ID)
	} else {
		fmt.Printf("Created computer search %q\n", sum.Name)
	}
	return sum, nil
}

// ComputerSearchPut PUTs a full Classic advanced computer search (computers stripped).
func ComputerSearchPut(credName string, opts Opts, id string, body map[string]interface{}, logger *slog.Logger) error {
	path, err := computerSearchPath(id)
	if err != nil {
		return err
	}
	wrapped, err := prepareClassicSearchBody(classicACSWrapper, body, false)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(wrapped)
	if err != nil {
		return fmt.Errorf("marshal advanced computer search: %w", err)
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nPUT computer search (ID %s)?", id), logger) {
			return nil
		}
	}
	if _, err := jamfPUT(credName, path, payload, "application/json"); err != nil {
		return fmt.Errorf("update advanced computer search: %w", err)
	}
	return nil
}

// ComputerSearchDelete deletes one or more Classic advanced computer searches by id.
func ComputerSearchDelete(credName string, opts Opts, ids []string, logger *slog.Logger) error {
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
		if d, err := ComputerSearchFetch(credName, id); err == nil && classicMapName(d) != "" {
			name = classicMapName(d)
		}
		infos = append(infos, info{id: id, name: name})
	}
	if !opts.SkipConfirm {
		msg := "\nDelete computer searches:"
		for _, p := range infos {
			msg += fmt.Sprintf("\n  %q (ID %s)", p.name, p.id)
		}
		if !ConfirmPrompt(msg, logger) {
			return nil
		}
	}
	for _, p := range infos {
		path, err := computerSearchPath(p.id)
		if err != nil {
			return err
		}
		if err := jamfDELETE(credName, path); err != nil {
			return fmt.Errorf("delete advanced computer search %s: %w", p.id, err)
		}
		fmt.Printf("Deleted computer search %q (ID %s)\n", p.name, p.id)
	}
	return nil
}

// ComputerSearchList prints a table of computer searches, or JSON.
func ComputerSearchList(credName, filter string, asJSON bool, logger *slog.Logger) error {
	rows, err := ComputerSearchSearch(credName, filter, logger)
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
	fmt.Printf("\n%d computer search(es)\n", len(rows))
	return nil
}

// ComputerSearchGet prints the definition JSON (no computers).
func ComputerSearchGet(credName, id string, _ *slog.Logger) error {
	d, err := ComputerSearchFetch(credName, id)
	if err != nil {
		return err
	}
	return printJSONValue(d)
}

// ComputerSearchGetID prints the numeric id for an exact name.
func ComputerSearchGetID(credName, name string, _ *slog.Logger) (string, error) {
	d, err := ComputerSearchFetchByName(credName, name)
	if err != nil {
		return "", err
	}
	id := classicMapID(d)
	if id == "" {
		return "", fmt.Errorf("computer search %q has no id", name)
	}
	return id, nil
}

// ComputerSearchResultsPrint prints result rows as a table, JSON, CSV, or jq --raw.
func ComputerSearchResultsPrint(credName, id, filter string, opts SearchResultPrintOpts, _ *slog.Logger) error {
	rows, err := ComputerSearchComputers(credName, id, filter)
	if err != nil {
		return err
	}
	return formatSearchResults(rows, opts, "computer(s)")
}
