package buddy

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
)

const uriScripts = "/api/v1/scripts"

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

// ScriptSearch fetches scripts via paginated requests and returns summary records
// matching the optional filter (id, name, category only — never scriptContents).
// Jamf's list OpenAPI model may include full Script bodies; buddy strips them.
// When opts.Page is PageAll, all pages are fetched.
// Filter: bare term → name=="*term*" (server); RSQL expression → pass-through.
func ScriptSearch(credName, filter string, opts PageOpts, logger *slog.Logger) ([]ScriptRecord, error) {
	normalized, err := NormalizeListFilter(filter, FilterSpecScripts)
	if err != nil {
		return nil, err
	}
	extra := url.Values{}
	if normalized != "" {
		extra.Set("filter", normalized)
	}
	var all []ScriptRecord
	err = fetchJamfPaged(credName, uriScripts, extra, opts, func(resultsJSON []byte, _ int) (int, error) {
		// Unmarshal as full ScriptDetail (Jamf may send scriptContents), then project.
		var page []ScriptDetail
		if err := json.Unmarshal(resultsJSON, &page); err != nil {
			return 0, fmt.Errorf("parse response: %w", err)
		}
		for _, s := range page {
			all = append(all, ScriptRecord{
				ID:           s.ID,
				Name:         s.Name,
				CategoryID:   s.CategoryID,
				CategoryName: s.CategoryName,
			})
		}
		return len(page), nil
	})
	if err != nil {
		return nil, err
	}
	return all, nil
}

func scriptSearchRaw(credName, rsqlFilter string, opts PageOpts) ([]json.RawMessage, error) {
	extra := url.Values{}
	if strings.TrimSpace(rsqlFilter) != "" {
		extra.Set("filter", rsqlFilter)
	}
	var all []json.RawMessage
	err := fetchJamfPaged(credName, uriScripts, extra, opts, func(resultsJSON []byte, _ int) (int, error) {
		var page []json.RawMessage
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

func scriptJSONHasContents(raw json.RawMessage) bool {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return false
	}
	_, ok := m["scriptContents"]
	return ok
}

// ScriptFetchByName loads one script by exact name (case-insensitive).
// It filters with RSQL name=="…" and returns that list row when Jamf included
// scriptContents; otherwise GET /api/v1/scripts/{id} for the full document.
func ScriptFetchByName(credName, name string, logger *slog.Logger) (*ScriptDetail, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("script name is required")
	}
	filter, err := RSQLEq("name", name)
	if err != nil {
		return nil, err
	}
	rows, err := scriptSearchRaw(credName, filter, PageOpts{Page: PageAll})
	if err != nil {
		return nil, err
	}
	type hit struct {
		raw json.RawMessage
		s   ScriptDetail
	}
	var matches []hit
	for _, raw := range rows {
		var s ScriptDetail
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, fmt.Errorf("parse response: %w", err)
		}
		if strings.EqualFold(s.Name, name) {
			matches = append(matches, hit{raw: raw, s: s})
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no script found with name %q", name)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("multiple scripts found with name %q", name)
	}
	m := matches[0]
	if scriptJSONHasContents(m.raw) {
		return &m.s, nil
	}
	if strings.TrimSpace(m.s.ID) == "" {
		return nil, fmt.Errorf("script %q has no id", name)
	}
	return scriptGetFull(credName, m.s.ID)
}

// ScriptList fetches and prints scripts in a tabular format, with an optional name filter.
func ScriptList(credName, filter string, logger *slog.Logger) error {
	scripts, err := ScriptSearch(credName, filter, PageOpts{Page: PageAll}, logger)
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
	body, err := jamfGET(credName, uriScripts+"/"+id)
	if err != nil {
		return nil, fmt.Errorf("get script %s: %w", id, err)
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
	s, err := ScriptFetchByName(credName, name, logger)
	if err != nil {
		return "", err
	}
	return s.ID, nil
}

// ScriptHistory fetches and prints recent audit history for a script.
func ScriptHistory(credName, id string, limit int, logger *slog.Logger) error {
	if limit <= 0 {
		limit = 20
	}
	body, err := jamfGET(credName, fmt.Sprintf("%s/%s/history?sort=date:desc&page-size=%d", uriScripts, id, limit))
	if err != nil {
		return fmt.Errorf("get script history %s: %w", id, err)
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

// ScriptPostDetail POSTs a full script record and returns the created detail.
func ScriptPostDetail(credName string, opts Opts, s ScriptDetail, logger *slog.Logger) (*ScriptDetail, error) {
	s.Name = strings.TrimSpace(s.Name)
	if s.Name == "" {
		return nil, fmt.Errorf("script name is required")
	}
	s.ID = ""
	data, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("Create script %q", s.Name), logger) {
			return nil, nil
		}
	}
	body, err := jamfPOST(credName, uriScripts, data, "application/json")
	if err != nil {
		return nil, fmt.Errorf("create script: %w", err)
	}
	created, err := mergeScriptWriteResponse(s, body, "")
	if err != nil {
		return nil, err
	}
	fmt.Printf("Created script (ID %s): %s\n", created.ID, created.Name)
	return created, nil
}

// mergeScriptWriteResponse overlays a POST/PUT JSON body onto the payload we sent.
// Jamf POST is {id, href}; PUT is a Script. Keys present in the response win; href is dropped.
// fallbackID is used when the body has no id (PUT path id).
func mergeScriptWriteResponse(posted ScriptDetail, body []byte, fallbackID string) (*ScriptDetail, error) {
	postedJSON, err := json.Marshal(posted)
	if err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	var merged map[string]any
	if err := json.Unmarshal(postedJSON, &merged); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	id := strings.TrimSpace(fallbackID)
	if v, ok := resp["id"]; ok {
		if s := jsonStringID(v); s != "" {
			id = s
		}
	}
	if id == "" {
		return nil, fmt.Errorf("write response missing id: %s", string(body))
	}
	for k, v := range resp {
		if k == "href" {
			continue
		}
		merged[k] = v
	}
	merged["id"] = id
	outJSON, err := json.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	var out ScriptDetail
	if err := json.Unmarshal(outJSON, &out); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &out, nil
}

func jsonStringID(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case json.Number:
		return strings.TrimSpace(t.String())
	case float64:
		if t == float64(int64(t)) {
			return fmt.Sprintf("%.0f", t)
		}
		return strings.TrimSpace(fmt.Sprintf("%v", t))
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
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
	_, err = ScriptPostDetail(credName, opts, s, logger)
	return err
}

// scriptPut replaces a script record via PUT /api/v1/scripts/{id}.
func scriptPut(credName, id string, s *ScriptDetail) (*ScriptDetail, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	body, err := jamfPUT(credName, uriScripts+"/"+id, data, "application/json")
	if err != nil {
		return nil, fmt.Errorf("update script %s: %w", id, err)
	}
	updated, err := mergeScriptWriteResponse(*s, body, id)
	if err != nil {
		return nil, fmt.Errorf("update script %s: %w", id, err)
	}
	return updated, nil
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
		if !ConfirmPrompt(fmt.Sprintf("Update script %s (%s) via PUT", id, s.Name), logger) {
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
		if !ConfirmPrompt(fmt.Sprintf("Update script %s (%s) contents from %q", id, s.Name, file), logger) {
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
			if !ConfirmPrompt(fmt.Sprintf("Delete script %s (%s)", id, s.Name), logger) {
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
	if err := jamfDELETE(credName, uriScripts+"/"+id); err != nil {
		return fmt.Errorf("delete script %s: %w", id, err)
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
		if !ConfirmPrompt(fmt.Sprintf("Set script %s name: %q → %q", id, s.Name, value), logger) {
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
		if !ConfirmPrompt(fmt.Sprintf("Set script %s (%s) category: %q → %q", id, s.Name, s.CategoryID, categoryID), logger) {
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
		if !ConfirmPrompt(fmt.Sprintf("Set script %s (%s) priority: %s → %s", id, s.Name, s.Priority, priority), logger) {
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
		if !ConfirmPrompt(fmt.Sprintf("Set script %s (%s) info", id, s.Name), logger) {
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
		if !ConfirmPrompt(fmt.Sprintf("Set script %s (%s) notes", id, s.Name), logger) {
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
