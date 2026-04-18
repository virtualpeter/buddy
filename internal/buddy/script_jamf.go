package buddy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// ScriptRecord is a summary record from the scripts list endpoint.
type ScriptRecord struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	CategoryID   string `json:"categoryId"`
	CategoryName string `json:"categoryName"`
}

// ScriptDetail is the full script record from GET /api/v1/scripts/{id} and POST/PUT.
type ScriptDetail struct {
	ID             string `json:"id,omitempty"`
	Name           string `json:"name"`
	CategoryID     string `json:"categoryId,omitempty"`
	CategoryName   string `json:"categoryName,omitempty"`
	Info           string `json:"info,omitempty"`
	Notes          string `json:"notes,omitempty"`
	OSRequirements string `json:"osRequirements,omitempty"`
	Priority       string `json:"priority,omitempty"` // BEFORE | AFTER | AT_REBOOT
	Parameter4     string `json:"parameter4,omitempty"`
	Parameter5     string `json:"parameter5,omitempty"`
	Parameter6     string `json:"parameter6,omitempty"`
	Parameter7     string `json:"parameter7,omitempty"`
	Parameter8     string `json:"parameter8,omitempty"`
	Parameter9     string `json:"parameter9,omitempty"`
	Parameter10    string `json:"parameter10,omitempty"`
	Parameter11    string `json:"parameter11,omitempty"`
	ScriptContents string `json:"scriptContents,omitempty"`
}

// scriptSearchResults is the paginated response from GET /api/v1/scripts.
type scriptSearchResults struct {
	TotalCount int            `json:"totalCount"`
	Results    []ScriptDetail `json:"results"`
}

// ScriptHistoryEntry is one record from GET /api/v1/scripts/{id}/history.
type ScriptHistoryEntry struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Date     string `json:"date"`
	Note     string `json:"note,omitempty"`
	Details  string `json:"details,omitempty"`
}

type scriptHistoryResponse struct {
	TotalCount int                  `json:"totalCount"`
	Results    []ScriptHistoryEntry `json:"results"`
}

// isRSQLFilter reports whether s contains any RSQL comparison operator.
// Plain name fragments (e.g. "install") contain none of these.
func isRSQLFilter(s string) bool {
	for _, op := range []string{"==", "!=", "=lt=", "=gt=", "=le=", "=ge=", "=in=", "=out="} {
		if strings.Contains(s, op) {
			return true
		}
	}
	return false
}

const scriptPageSize = 200

// ScriptSearch fetches all scripts via paginated requests and returns records
// matching the optional filter. Two filter modes are supported:
//   - Plain string (e.g. "install"): fetches all pages, then filters client-side
//     by case-insensitive name substring.
//   - RSQL expression (e.g. `name=="*Jamf*"`): passes &filter= to the API on
//     each page request so the server filters and pagination walks only matching records.
func ScriptSearch(credName, filter string, logger *slog.Logger) ([]ScriptDetail, error) {
	rsql := filter != "" && isRSQLFilter(filter)
	var all []ScriptDetail
	baseURL, err := GetBaseURL(credName)
	if err != nil {
		return nil, err
	}
	for page := 0; ; page++ {
		endpoint := fmt.Sprintf("%s/api/v1/scripts?page=%d&page-size=%d",
			baseURL, page, scriptPageSize)
		if rsql {
			endpoint += "&filter=" + url.QueryEscape(filter)
		}
		req, err := http.NewRequest(http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		resp, err := PerformRequest(nil, req, credName)
		if err != nil {
			return nil, fmt.Errorf("list scripts (page %d): %w", page, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("list scripts status %d: %s", resp.StatusCode, string(body))
		}
		var result scriptSearchResults
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("parse response: %w", err)
		}
		all = append(all, result.Results...)
		if len(all) >= result.TotalCount || len(result.Results) == 0 {
			break
		}
	}
	if filter != "" && !rsql {
		filterLower := strings.ToLower(strings.TrimSpace(filter))
		var out []ScriptDetail
		for _, s := range all {
			if strings.Contains(strings.ToLower(s.Name), filterLower) {
				out = append(out, s)
			}
		}
		return out, nil
	}
	return all, nil
}

// ScriptList fetches and prints scripts in a tabular format, with an optional name filter.
func ScriptList(credName, filter string, logger *slog.Logger) error {
	scripts, err := ScriptSearch(credName, filter, logger)
	if err != nil {
		return err
	}
	fmt.Printf("%-8s  %-40s  %s\n", "ID", "Name", "Category")
	fmt.Printf("%-8s  %-40s  %s\n", "--------", "----------------------------------------", "--------------------")
	for _, s := range scripts {
		cat := s.CategoryName
		if cat == "" {
			cat = s.CategoryID
		}
		fmt.Printf("%-8s  %-40s  %s\n", s.ID, truncatePkg(s.Name, 40), cat)
	}
	fmt.Printf("\n%d script(s)\n", len(scripts))
	return nil
}

// scriptGetFull fetches a single script by ID and returns the full detail struct.
func scriptGetFull(credName, id string) (*ScriptDetail, error) {
	baseURL, err := GetBaseURL(credName)
	if err != nil {
		return nil, err
	}
	endpoint := baseURL + "/api/v1/scripts/" + id
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return nil, fmt.Errorf("get script %s: %w", id, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get script %s status %d: %s", id, resp.StatusCode, string(body))
	}
	var s ScriptDetail
	if err := json.Unmarshal(body, &s); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &s, nil
}

// ScriptFetchFull fetches a script by ID and returns the full detail struct (for script/component use).
func ScriptFetchFull(credName, id string) (*ScriptDetail, error) {
	return scriptGetFull(credName, id)
}

// ScriptGet fetches and pretty-prints a single script record by ID (including scriptContents).
func ScriptGet(credName, id string, logger *slog.Logger) error {
	s, err := scriptGetFull(credName, id)
	if err != nil {
		return err
	}
	out, _ := json.MarshalIndent(s, "", "  ")
	fmt.Println(string(out))
	return nil
}

// ScriptGetContents fetches a script and writes only the script body to stdout.
// Useful for piping: buddy script get contents <id> > fixed.sh
func ScriptGetContents(credName, id string, logger *slog.Logger) error {
	s, err := scriptGetFull(credName, id)
	if err != nil {
		return err
	}
	fmt.Print(s.ScriptContents)
	return nil
}

// ScriptGetContentsStr returns the script body as a string (for programmatic/Starlark use).
func ScriptGetContentsStr(credName, id string) (string, error) {
	s, err := scriptGetFull(credName, id)
	if err != nil {
		return "", err
	}
	return s.ScriptContents, nil
}

// ScriptGetID resolves a script name to its string ID (exact match, case-insensitive).
func ScriptGetID(credName, name string, logger *slog.Logger) (string, error) {
	scripts, err := ScriptSearch(credName, "", logger)
	if err != nil {
		return "", err
	}
	nameLower := strings.ToLower(strings.TrimSpace(name))
	for _, s := range scripts {
		if strings.ToLower(s.Name) == nameLower {
			return s.ID, nil
		}
	}
	return "", fmt.Errorf("no script found with name %q", name)
}

// ScriptHistory fetches and prints recent audit history for a script.
func ScriptHistory(credName, id string, limit int, logger *slog.Logger) error {
	if limit <= 0 {
		limit = 20
	}
	baseURL, err := GetBaseURL(credName)
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("%s/api/v1/scripts/%s/history?sort=date:desc&page-size=%d",
		baseURL, id, limit)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return fmt.Errorf("get script history %s: %w", id, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("get script history %s status %d: %s", id, resp.StatusCode, string(body))
	}
	var result scriptHistoryResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("parse history response: %w", err)
	}
	if len(result.Results) == 0 {
		fmt.Println("No history entries found.")
		return nil
	}
	fmt.Printf("%-30s  %-20s  %s\n", "Date", "User", "Note")
	fmt.Printf("%-30s  %-20s  %s\n", "------------------------------", "--------------------", "--------------------")
	for _, e := range result.Results {
		note := e.Note
		if note == "" {
			note = e.Details
		}
		fmt.Printf("%-30s  %-20s  %s\n", e.Date, e.Username, note)
	}
	fmt.Printf("\n%d of %d history entries\n", len(result.Results), result.TotalCount)
	return nil
}

// ScriptCreate creates a new Jamf script, reading the script body from a local file.
func ScriptCreate(credName string, opts Opts, name, file, category, priority, info, notes string, logger *slog.Logger) error {
	contents, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("read script file %q: %w", file, err)
	}
	s := ScriptDetail{
		Name:           name,
		ScriptContents: string(contents),
	}
	if category != "" {
		s.CategoryID = category
	}
	if priority != "" {
		s.Priority = strings.ToUpper(priority)
	}
	if info != "" {
		s.Info = info
	}
	if notes != "" {
		s.Notes = notes
	}
	if !opts.SkipConfirm {
		if !confirm(fmt.Sprintf("Create script %q from %q", name, file), logger) {
			return nil
		}
	}
	data, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	baseURL, err := GetBaseURL(credName)
	if err != nil {
		return err
	}
	endpoint := baseURL + "/api/v1/scripts"
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return fmt.Errorf("create script: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("create script status %d: %s", resp.StatusCode, string(body))
	}
	var created ScriptDetail
	if err := json.Unmarshal(body, &created); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	fmt.Printf("Created script (ID %s): %s\n", created.ID, created.Name)
	return nil
}

// scriptPut replaces a script record via PUT /api/v1/scripts/{id}.
func scriptPut(credName, id string, s *ScriptDetail) (*ScriptDetail, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	baseURL, err := GetBaseURL(credName)
	if err != nil {
		return nil, err
	}
	endpoint := baseURL + "/api/v1/scripts/" + id
	req, err := http.NewRequest(http.MethodPut, endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return nil, fmt.Errorf("update script %s: %w", id, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update script %s status %d: %s", id, resp.StatusCode, string(body))
	}
	var updated ScriptDetail
	if err := json.Unmarshal(body, &updated); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &updated, nil
}

// ScriptPutFields fetches the existing script, applies updates from fields, then PUTs the full payload.
// Intended for scripts/component use where callers want to update a subset of fields.
//
// Supported fields (keys are the API JSON field names):
// - name (string)
// - categoryId (string)
// - info (string)
// - notes (string)
// - osRequirements (string)
// - priority (string)  // BEFORE | AFTER | AT_REBOOT
// - parameter4..parameter11 (string)
// - scriptContents (string)
func ScriptPutFields(credName string, opts Opts, id string, fields map[string]interface{}, logger *slog.Logger) (*ScriptDetail, error) {
	s, err := scriptGetFull(credName, id)
	if err != nil {
		return nil, err
	}
	if v, ok := fields["name"].(string); ok {
		s.Name = v
	}
	if v, ok := fields["categoryId"].(string); ok {
		s.CategoryID = v
		s.CategoryName = ""
	}
	if v, ok := fields["info"].(string); ok {
		s.Info = v
	}
	if v, ok := fields["notes"].(string); ok {
		s.Notes = v
	}
	if v, ok := fields["osRequirements"].(string); ok {
		s.OSRequirements = v
	}
	if v, ok := fields["priority"].(string); ok {
		s.Priority = v
	}
	for i := 4; i <= 11; i++ {
		key := fmt.Sprintf("parameter%d", i)
		if v, ok := fields[key].(string); ok {
			switch i {
			case 4:
				s.Parameter4 = v
			case 5:
				s.Parameter5 = v
			case 6:
				s.Parameter6 = v
			case 7:
				s.Parameter7 = v
			case 8:
				s.Parameter8 = v
			case 9:
				s.Parameter9 = v
			case 10:
				s.Parameter10 = v
			case 11:
				s.Parameter11 = v
			}
		}
	}
	if v, ok := fields["scriptContents"].(string); ok {
		s.ScriptContents = v
	}

	if !opts.SkipConfirm {
		if !confirm(fmt.Sprintf("Update script %s (%s) via PUT", id, s.Name), logger) {
			return nil, nil
		}
	}
	return scriptPut(credName, id, s)
}

// ScriptUpdate replaces the script contents from a local file (fetch-then-PUT).
func ScriptUpdate(credName string, opts Opts, id, file string, logger *slog.Logger) error {
	contents, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("read script file %q: %w", file, err)
	}
	s, err := scriptGetFull(credName, id)
	if err != nil {
		return err
	}
	if !opts.SkipConfirm {
		if !confirm(fmt.Sprintf("Update script %s (%s) contents from %q", id, s.Name, file), logger) {
			return nil
		}
	}
	s.ScriptContents = string(contents)
	updated, err := scriptPut(credName, id, s)
	if err != nil {
		return err
	}
	fmt.Printf("Updated script %s (%s): contents replaced from %q\n", updated.ID, updated.Name, file)
	return nil
}

// ScriptDelete deletes one or more scripts by ID, fetching the name first for confirmation.
func ScriptDelete(credName string, opts Opts, ids []string, logger *slog.Logger) error {
	if len(ids) == 0 {
		return fmt.Errorf("at least one script ID required")
	}
	for _, id := range ids {
		s, err := scriptGetFull(credName, id)
		if err != nil {
			return fmt.Errorf("fetch script %s: %w", id, err)
		}
		if !opts.SkipConfirm {
			if !confirm(fmt.Sprintf("Delete script %s (%s)", id, s.Name), logger) {
				continue
			}
		}
		if err := scriptDeleteOne(credName, id, s.Name); err != nil {
			return err
		}
	}
	return nil
}

func scriptDeleteOne(credName, id, name string) error {
	baseURL, err := GetBaseURL(credName)
	if err != nil {
		return err
	}
	endpoint := baseURL + "/api/v1/scripts/" + id
	req, err := http.NewRequest(http.MethodDelete, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	resp, err := PerformRequest(nil, req, credName)
	if err != nil {
		return fmt.Errorf("delete script %s: %w", id, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete script %s status %d: %s", id, resp.StatusCode, string(body))
	}
	fmt.Printf("Deleted script %s (%s)\n", id, name)
	return nil
}

// ScriptSetName renames a script.
func ScriptSetName(credName, id string, opts Opts, value string, logger *slog.Logger) error {
	s, err := scriptGetFull(credName, id)
	if err != nil {
		return err
	}
	if !opts.SkipConfirm {
		if !confirm(fmt.Sprintf("Set script %s name: %q → %q", id, s.Name, value), logger) {
			return nil
		}
	}
	s.Name = value
	updated, err := scriptPut(credName, id, s)
	if err != nil {
		return err
	}
	fmt.Printf("Updated script %s: name = %q\n", id, updated.Name)
	return nil
}

// ScriptSetCategory updates the category of a script by category ID.
// Clear s.CategoryName so the API resolves only by categoryId.
func ScriptSetCategory(credName, id string, opts Opts, categoryID string, logger *slog.Logger) error {
	s, err := scriptGetFull(credName, id)
	if err != nil {
		return err
	}
	if !opts.SkipConfirm {
		if !confirm(fmt.Sprintf("Set script %s (%s) category: %q → %q", id, s.Name, s.CategoryID, categoryID), logger) {
			return nil
		}
	}
	s.CategoryID = categoryID
	s.CategoryName = ""
	updated, err := scriptPut(credName, id, s)
	if err != nil {
		return err
	}
	fmt.Printf("Updated script %s: categoryId = %s\n", id, updated.CategoryID)
	return nil
}

// ScriptSetPriority updates the execution priority of a script.
// Valid values: BEFORE, AFTER, AT_REBOOT (case-insensitive on input).
func ScriptSetPriority(credName, id string, opts Opts, priority string, logger *slog.Logger) error {
	priority = strings.ToUpper(strings.TrimSpace(priority))
	switch priority {
	case "BEFORE", "AFTER", "AT_REBOOT":
	default:
		return fmt.Errorf("priority must be BEFORE, AFTER, or AT_REBOOT, got %q", priority)
	}
	s, err := scriptGetFull(credName, id)
	if err != nil {
		return err
	}
	if !opts.SkipConfirm {
		if !confirm(fmt.Sprintf("Set script %s (%s) priority: %s → %s", id, s.Name, s.Priority, priority), logger) {
			return nil
		}
	}
	s.Priority = priority
	updated, err := scriptPut(credName, id, s)
	if err != nil {
		return err
	}
	fmt.Printf("Updated script %s: priority = %s\n", id, updated.Priority)
	return nil
}

// ScriptSetInfo updates the info field of a script.
func ScriptSetInfo(credName, id string, opts Opts, value string, logger *slog.Logger) error {
	s, err := scriptGetFull(credName, id)
	if err != nil {
		return err
	}
	if !opts.SkipConfirm {
		if !confirm(fmt.Sprintf("Set script %s (%s) info", id, s.Name), logger) {
			return nil
		}
	}
	s.Info = value
	if _, err := scriptPut(credName, id, s); err != nil {
		return err
	}
	fmt.Printf("Updated script %s: info set\n", id)
	return nil
}

// scriptFilename derives a safe local filename from a Jamf script name.
// Converts to lowercase, collapses runs of non-alphanumeric characters to a
// single hyphen, trims leading/trailing hyphens, and appends ".sh".
// e.g. "Restart Jamf Agent!" → "restart-jamf-agent.sh"
func scriptFilename(name string) string {
	var b strings.Builder
	prevHyphen := false
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevHyphen = false
		} else if !prevHyphen {
			b.WriteRune('-')
			prevHyphen = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		slug = "script"
	}
	return slug + ".sh"
}

// ScriptDownload fetches a script and writes its contents to a local file.
// If outfile is empty, the filename is derived from the script's name via scriptFilename.
func ScriptDownload(credName, id, outfile string, logger *slog.Logger) error {
	s, err := scriptGetFull(credName, id)
	if err != nil {
		return err
	}
	if outfile == "" {
		outfile = scriptFilename(s.Name)
	}
	if err := os.WriteFile(outfile, []byte(s.ScriptContents), 0644); err != nil {
		return fmt.Errorf("write %q: %w", outfile, err)
	}
	fmt.Printf("Downloaded script %s (%s) → %s\n", s.ID, s.Name, outfile)
	return nil
}

// ScriptSetNotes updates the notes field of a script.
func ScriptSetNotes(credName, id string, opts Opts, value string, logger *slog.Logger) error {
	s, err := scriptGetFull(credName, id)
	if err != nil {
		return err
	}
	if !opts.SkipConfirm {
		if !confirm(fmt.Sprintf("Set script %s (%s) notes", id, s.Name), logger) {
			return nil
		}
	}
	s.Notes = value
	if _, err := scriptPut(credName, id, s); err != nil {
		return err
	}
	fmt.Printf("Updated script %s: notes set\n", id)
	return nil
}
