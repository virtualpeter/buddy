package buddy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
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

// scriptListJSON is the buddy script list --json row. localFilename is derived
// and is not a field on ScriptRecord.
type scriptListJSON struct {
	ScriptRecord
	LocalFilename string `json:"localFilename"`
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

// ScriptList fetches and prints scripts (table, or JSON rows), with an optional name filter.
// opts.Page == PageAll fetches every page. A non-negative page fetches that page only.
func ScriptList(credName, filter string, asJSON bool, opts PageOpts, logger *slog.Logger) error {
	scripts, err := ScriptSearch(credName, filter, opts, logger)
	if err != nil {
		return err
	}
	if asJSON {
		rows := make([]scriptListJSON, len(scripts))
		for i, s := range scripts {
			rows[i] = scriptListJSON{ScriptRecord: s, LocalFilename: scriptFilename(s.Name)}
		}
		return printJSONValue(rows)
	}
	fmt.Printf("%-8s  %-40s  %-40s  %s\n", "ID", "Name", "Local Filename", "Category")
	fmt.Printf("%-8s  %-40s  %-40s  %s\n", "--------", "----------------------------------------", "----------------------------------------", "--------------------")
	for _, s := range scripts {
		cat := s.CategoryName
		if cat == "" {
			cat = s.CategoryID
		}
		fmt.Printf("%-8s  %-40s  %-40s  %s\n", s.ID, truncatePkg(s.Name, 40), scriptFilename(s.Name), cat)
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
func ScriptGet(credName, ref string, logger *slog.Logger) error {
	s, err := ScriptResolve(credName, ref, logger)
	if err != nil {
		return err
	}
	out, _ := json.MarshalIndent(s, "", "  ")
	fmt.Println(string(out))
	return nil
}

// ScriptGetContents fetches a script and writes only the script body to stdout.
// Useful for piping: buddy script get contents <script> > fixed.sh
func ScriptGetContents(credName, ref string, logger *slog.Logger) error {
	s, err := ScriptResolve(credName, ref, logger)
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

// ScriptResolve loads one script by id, exact name, or a local file's download name.
func ScriptResolve(credName, ref string, logger *slog.Logger) (*ScriptDetail, error) {
	resolved, err := resolveScriptRef(credName, ref)
	if err != nil {
		return nil, err
	}
	_ = logger
	return scriptGetFull(credName, resolved.ID)
}

// ScriptLookupID prints-style resolution for `get id`: an existing file matches
// by download name; anything else is an exact Jamf name, including a numeric string.
func ScriptLookupID(credName, ref string, logger *slog.Logger) (string, error) {
	ref = strings.TrimSpace(ref)
	info, err := os.Stat(ref)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if err == nil && !info.IsDir() {
		resolved, err := resolveScriptRef(credName, ref)
		if err != nil {
			return "", err
		}
		return resolved.ID, nil
	}
	return ScriptGetID(credName, ref, logger)
}

// ScriptPutJSONFile reads a JSON object and applies it to the script identified by ref.
func ScriptPutJSONFile(credName string, opts Opts, ref, path string, logger *slog.Logger) (*ScriptDetail, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read script json %q: %w", path, err)
	}
	var fields map[string]interface{}
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, fmt.Errorf("parse script json %q: %w", path, err)
	}
	resolved, err := resolveScriptRef(credName, ref)
	if err != nil {
		return nil, err
	}
	return ScriptPutFields(credName, opts, resolved.ID, fields, logger)
}

// ScriptGetID resolves a script name to its string ID (exact match, case-insensitive).
func ScriptGetID(credName, name string, logger *slog.Logger) (string, error) {
	s, err := ScriptFetchByName(credName, name, logger)
	if err != nil {
		return "", err
	}
	return s.ID, nil
}

// ScriptHistoryFetch returns recent audit history for a script. limit <= 0 means 20.
// total is the server's totalCount, which can be larger than the returned page.
func ScriptHistoryFetch(credName, id string, limit int) ([]ScriptHistoryEntry, int, error) {
	if limit <= 0 {
		limit = 20
	}
	body, err := jamfGET(credName, fmt.Sprintf("%s/%s/history?sort=date:desc&page-size=%d", uriScripts, id, limit))
	if err != nil {
		return nil, 0, fmt.Errorf("get script history %s: %w", id, err)
	}
	var result scriptHistoryResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, 0, fmt.Errorf("parse history response: %w", err)
	}
	return result.Results, result.TotalCount, nil
}

// ScriptHistory fetches and prints recent audit history for a script.
func ScriptHistory(credName, id string, limit int, logger *slog.Logger) error {
	entries, total, err := ScriptHistoryFetch(credName, id, limit)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Println("No history entries found.")
		return nil
	}
	fmt.Printf("%-30s  %-20s  %s\n", "Date", "User", "Note")
	fmt.Printf("%-30s  %-20s  %s\n", "------------------------------", "--------------------", "--------------------")
	for _, e := range entries {
		note := e.Note
		if note == "" {
			note = e.Details
		}
		fmt.Printf("%-30s  %-20s  %s\n", e.Date, e.Username, note)
	}
	fmt.Printf("\n%d of %d history entries\n", len(entries), total)
	return nil
}

// ScriptHistoryRef resolves ref (id, exact name, or a local file) and prints history.
func ScriptHistoryRef(credName, ref string, limit int, logger *slog.Logger) error {
	resolved, err := resolveScriptRef(credName, ref)
	if err != nil {
		return err
	}
	return ScriptHistory(credName, resolved.ID, limit, logger)
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

// ScriptCreateExtra is the optional create payload beyond name, file, category, priority, info, and notes.
type ScriptCreateExtra struct {
	OSRequirements string
	Parameter4     string
	Parameter5     string
	Parameter6     string
	Parameter7     string
	Parameter8     string
	Parameter9     string
	Parameter10    string
	Parameter11    string
}

// ScriptCreate creates a new Jamf script, reading the script body from a local file.
// It refuses when name already exists, or when another script already downloads to scriptFilename(name).
func ScriptCreate(credName string, opts Opts, name, file, category, priority, info, notes string, extra ScriptCreateExtra, logger *slog.Logger) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("script name is required")
	}
	contents, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("read script file %q: %w", file, err)
	}
	if err := scriptExactNameConflict(credName, name, ""); err != nil {
		return err
	}
	if err := scriptRefuseDownloadName(credName, name, ""); err != nil {
		return err
	}
	s := ScriptDetail{
		Name:           name,
		ScriptContents: string(contents),
		OSRequirements: extra.OSRequirements,
		Parameter4:     extra.Parameter4,
		Parameter5:     extra.Parameter5,
		Parameter6:     extra.Parameter6,
		Parameter7:     extra.Parameter7,
		Parameter8:     extra.Parameter8,
		Parameter9:     extra.Parameter9,
		Parameter10:    extra.Parameter10,
		Parameter11:    extra.Parameter11,
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
	_, err := scriptSetName(credName, opts, id, value, logger)
	return err
}

// scriptSetName renames id. applied is false when the confirm prompt is declined.
func scriptSetName(credName string, opts Opts, id, value string, logger *slog.Logger) (bool, error) {
	s, err := scriptGetFull(credName, id)
	if err != nil {
		return false, err
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("Set script %s name: %q → %q", id, s.Name, value), logger) {
			return false, nil
		}
	}
	s.Name = value
	updated, err := scriptPut(credName, id, s)
	if err != nil {
		return false, err
	}
	fmt.Printf("Updated script %s: name = %q\n", id, updated.Name)
	return true, nil
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
// A trailing ".sh" (any case) is kept as the extension: the stem is lowercased,
// runs of non-alphanumeric characters collapse to one hyphen, and ".sh" is
// reattached. A name with no ".sh" is slugged the same way and then ".sh" is
// appended. "Restart Jamf Agent!" → "restart-jamf-agent.sh".
// "install-chrome.sh" and "Install Chrome.SH" → "install-chrome.sh".
// "notes.py" → "notes-py.sh".
func scriptFilename(name string) string {
	name = strings.TrimSpace(name)
	if strings.HasSuffix(strings.ToLower(name), ".sh") {
		name = strings.TrimSpace(name[:len(name)-len(".sh")])
	}
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

// ScriptFilename is the local download name for a Jamf script name.
func ScriptFilename(name string) string {
	return scriptFilename(name)
}

// ScriptLocalStem is the inverse of ScriptFilename for a download-form filename.
// install-chrome.sh → install-chrome. A name with spaces is not recovered.
func ScriptLocalStem(filename string) (string, error) {
	base := filepath.Base(strings.TrimSpace(filename))
	if !ScriptFileIsDownloadName(base) {
		return "", fmt.Errorf("script file %q is not a download name", filename)
	}
	return strings.TrimSuffix(base, ".sh"), nil
}

// ScriptFileIsDownloadName reports whether file's base name is already a download name.
// create <file> accepts only that form; the Jamf name is the base name itself.
func ScriptFileIsDownloadName(file string) bool {
	base := filepath.Base(file)
	return base != "." && base != ".." && scriptFilename(base) == base
}

// ScriptTokenIsID reports whether s is a numeric script id.
func ScriptTokenIsID(s string) bool {
	return scriptTokenIsID(strings.TrimSpace(s))
}

func scriptTokenIsID(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

type scriptNameHit struct {
	ID   string
	Name string
}

func formatScriptHits(hits []scriptNameHit) string {
	parts := make([]string, len(hits))
	for i, h := range hits {
		parts[i] = fmt.Sprintf("%s (%s)", h.ID, h.Name)
	}
	return strings.Join(parts, ", ")
}

func scriptHitsByExactName(credName, name string) ([]scriptNameHit, error) {
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
	var hits []scriptNameHit
	for _, raw := range rows {
		var s ScriptDetail
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, fmt.Errorf("parse response: %w", err)
		}
		if strings.EqualFold(strings.TrimSpace(s.Name), name) {
			hits = append(hits, scriptNameHit{ID: s.ID, Name: s.Name})
		}
	}
	return hits, nil
}

func scriptDownloadNameFilter(filename string) (string, bool) {
	stem := filename
	if strings.HasSuffix(strings.ToLower(filename), ".sh") {
		stem = filename[:len(filename)-len(".sh")]
	}
	var tokens []string
	for _, part := range strings.Split(strings.ToLower(stem), "-") {
		if part == "" {
			continue
		}
		alnum := true
		for _, r := range part {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
				alnum = false
				break
			}
		}
		if alnum {
			tokens = append(tokens, part)
		}
	}
	if len(tokens) == 0 {
		return "", false
	}
	filter, err := RSQLEq("name", "*"+strings.Join(tokens, "*")+"*")
	if err != nil {
		return "", false
	}
	return filter, true
}

func scriptHitsMatchingDownloadName(recs []ScriptRecord, filename string) []scriptNameHit {
	var hits []scriptNameHit
	for _, s := range recs {
		if scriptFilename(s.Name) == filename {
			hits = append(hits, scriptNameHit{ID: s.ID, Name: s.Name})
		}
	}
	return hits
}

func scriptHitsByDownloadName(credName, filename string) ([]scriptNameHit, error) {
	if filter, ok := scriptDownloadNameFilter(filename); ok {
		recs, err := ScriptSearch(credName, filter, PageOpts{Page: PageAll}, nil)
		if err != nil {
			return nil, err
		}
		if hits := scriptHitsMatchingDownloadName(recs, filename); len(hits) > 0 {
			return hits, nil
		}
	}
	recs, err := ScriptSearch(credName, "", PageOpts{Page: PageAll}, nil)
	if err != nil {
		return nil, err
	}
	return scriptHitsMatchingDownloadName(recs, filename), nil
}

func scriptOneHit(hits []scriptNameHit, none, many string) (scriptNameHit, error) {
	switch len(hits) {
	case 0:
		return scriptNameHit{}, fmt.Errorf("%s", none)
	case 1:
		return hits[0], nil
	default:
		return scriptNameHit{}, fmt.Errorf("%s", many)
	}
}

// RequireNewScriptName reports an error when a script with this exact name already
// exists, or when another script already downloads to scriptFilename(name).
func RequireNewScriptName(credName, name string) error {
	if err := scriptExactNameConflict(credName, name, ""); err != nil {
		return err
	}
	return scriptRefuseDownloadName(credName, name, "")
}

func scriptExactNameConflict(credName, name, exceptID string) error {
	hits, err := scriptHitsByExactName(credName, name)
	if err != nil {
		return err
	}
	var others []scriptNameHit
	for _, h := range hits {
		if h.ID != exceptID {
			others = append(others, h)
		}
	}
	if len(others) == 0 {
		return nil
	}
	if len(others) == 1 {
		msg := fmt.Sprintf("script %q already exists (ID %s)", name, others[0].ID)
		if exceptID == "" {
			msg += "; use buddy script update"
		}
		return fmt.Errorf("%s", msg)
	}
	return fmt.Errorf("multiple scripts found with name %q: %s", name, formatScriptHits(others))
}

func scriptRefuseDownloadName(credName, name, exceptID string) error {
	filename := scriptFilename(name)
	hits, err := scriptHitsByDownloadName(credName, filename)
	if err != nil {
		return err
	}
	var others []scriptNameHit
	for _, h := range hits {
		if h.ID != exceptID {
			others = append(others, h)
		}
	}
	if len(others) == 0 {
		return nil
	}
	msg := fmt.Sprintf("script download name %q is already used by %s", filename, formatScriptHits(others))
	if exceptID == "" {
		msg += "; use buddy script update"
	}
	return fmt.Errorf("%s", msg)
}

type scriptRef struct {
	ID        string
	LocalPath string
}

// resolveScriptRef resolves one CLI identity: an existing file (by download
// name), otherwise a numeric id, otherwise an exact Jamf name.
func resolveScriptRef(credName, ref string) (scriptRef, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return scriptRef{}, fmt.Errorf("script is required")
	}
	info, err := os.Stat(ref)
	if err != nil && !os.IsNotExist(err) {
		return scriptRef{}, err
	}
	if err == nil && !info.IsDir() {
		base := filepath.Base(ref)
		hits, err := scriptHitsByDownloadName(credName, base)
		if err != nil {
			return scriptRef{}, err
		}
		hit, err := scriptOneHit(hits,
			fmt.Sprintf("no script downloads as %q", base),
			fmt.Sprintf("multiple scripts download as %q: %s", base, formatScriptHits(hits)),
		)
		if err != nil {
			return scriptRef{}, err
		}
		return scriptRef{ID: hit.ID, LocalPath: ref}, nil
	}
	if scriptTokenIsID(ref) {
		return scriptRef{ID: ref}, nil
	}
	hits, err := scriptHitsByExactName(credName, ref)
	if err != nil {
		return scriptRef{}, err
	}
	hit, err := scriptOneHit(hits,
		fmt.Sprintf("no script found with name %q", ref),
		fmt.Sprintf("multiple scripts found with name %q: %s", ref, formatScriptHits(hits)),
	)
	if err != nil {
		return scriptRef{}, err
	}
	return scriptRef{ID: hit.ID}, nil
}

func scriptIDFromNameOrID(credName, idOrName string) (string, error) {
	idOrName = strings.TrimSpace(idOrName)
	if idOrName == "" {
		return "", fmt.Errorf("script-id or name is required")
	}
	if scriptTokenIsID(idOrName) {
		return idOrName, nil
	}
	hits, err := scriptHitsByExactName(credName, idOrName)
	if err != nil {
		return "", err
	}
	hit, err := scriptOneHit(hits,
		fmt.Sprintf("no script found with name %q", idOrName),
		fmt.Sprintf("multiple scripts found with name %q: %s", idOrName, formatScriptHits(hits)),
	)
	if err != nil {
		return "", err
	}
	return hit.ID, nil
}

// ScriptUpdateByFile replaces the unique script that downloads as file's base name.
// It returns that script id. No owner tells the caller to create; several owners is an error.
func ScriptUpdateByFile(credName string, opts Opts, file string, logger *slog.Logger) (string, error) {
	base := filepath.Base(file)
	hits, err := scriptHitsByDownloadName(credName, base)
	if err != nil {
		return "", err
	}
	hit, err := scriptOneHit(hits,
		fmt.Sprintf("no script downloads as %q; use buddy script create", base),
		fmt.Sprintf("multiple scripts download as %q: %s", base, formatScriptHits(hits)),
	)
	if err != nil {
		return "", err
	}
	if err := ScriptUpdate(credName, opts, hit.ID, file, logger); err != nil {
		return "", err
	}
	return hit.ID, nil
}

// ScriptUpdateByName replaces the unique script with this exact name.
func ScriptUpdateByName(credName string, opts Opts, name, file string, logger *slog.Logger) (string, error) {
	hits, err := scriptHitsByExactName(credName, name)
	if err != nil {
		return "", err
	}
	hit, err := scriptOneHit(hits,
		fmt.Sprintf("no script found with name %q; use buddy script create", name),
		fmt.Sprintf("multiple scripts found with name %q: %s", name, formatScriptHits(hits)),
	)
	if err != nil {
		return "", err
	}
	if err := ScriptUpdate(credName, opts, hit.ID, file, logger); err != nil {
		return "", err
	}
	return hit.ID, nil
}

// ScriptRenameID refuses a new name that exists or whose download filename is
// already owned by a different script, then renames id.
func ScriptRenameID(credName string, opts Opts, id, newName string, logger *slog.Logger) error {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return fmt.Errorf("script name is required")
	}
	if err := scriptExactNameConflict(credName, newName, id); err != nil {
		return err
	}
	if err := scriptRefuseDownloadName(credName, newName, id); err != nil {
		return err
	}
	_, err := scriptSetName(credName, opts, id, newName, logger)
	return err
}

// ScriptRename resolves ref (id, exact name, or a local file) and renames the
// Jamf script. When ref is a local file, that file is renamed to the new download name
// after the Jamf rename is applied.
func ScriptRename(credName string, opts Opts, ref, newName string, logger *slog.Logger) error {
	resolved, err := resolveScriptRef(credName, ref)
	if err != nil {
		return err
	}
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return fmt.Errorf("script name is required")
	}
	if err := scriptExactNameConflict(credName, newName, resolved.ID); err != nil {
		return err
	}
	if err := scriptRefuseDownloadName(credName, newName, resolved.ID); err != nil {
		return err
	}
	dest, err := localScriptDest(resolved.LocalPath, newName)
	if err != nil {
		return err
	}
	applied, err := scriptSetName(credName, opts, resolved.ID, newName, logger)
	if err != nil || !applied {
		return err
	}
	return renameLocalScriptFile(resolved.LocalPath, dest)
}

// localScriptDest is the path a local script file should take after a Jamf rename.
// An empty dest means the file already has that download name. A dest that already
// exists is refused before the Jamf name changes.
func localScriptDest(localPath, newName string) (string, error) {
	if localPath == "" {
		return "", nil
	}
	dest := filepath.Join(filepath.Dir(localPath), scriptFilename(newName))
	if filepath.Clean(dest) == filepath.Clean(localPath) {
		return "", nil
	}
	if _, err := os.Stat(dest); err == nil {
		return "", fmt.Errorf("local file %q already exists", dest)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	return dest, nil
}

func renameLocalScriptFile(localPath, dest string) error {
	if localPath == "" || dest == "" {
		return nil
	}
	if err := os.Rename(localPath, dest); err != nil {
		return fmt.Errorf("rename %q to %q: %w", localPath, dest, err)
	}
	fmt.Printf("Renamed %s → %s\n", localPath, dest)
	return nil
}

// ScriptDeleteRefs deletes each ref. A ref is an id, an exact name, or a local
// file matched by download name. The local file is left in place.
func ScriptDeleteRefs(credName string, opts Opts, refs []string, logger *slog.Logger) error {
	if len(refs) == 0 {
		return fmt.Errorf("at least one script is required")
	}
	ids := make([]string, 0, len(refs))
	for _, ref := range refs {
		resolved, err := resolveScriptRef(credName, ref)
		if err != nil {
			return err
		}
		ids = append(ids, resolved.ID)
	}
	return ScriptDelete(credName, opts, ids, logger)
}

// ScriptCheckClass is the read-only comparison of a local file to a script body.
const (
	ScriptCheckMatch    = "match"
	ScriptCheckMismatch = "mismatch"
	ScriptCheckMissing  = "missing"
)

// ScriptCheckResult is the report from ScriptCheck.
type ScriptCheckResult struct {
	OK          bool
	Class       string
	ID          string
	Name        string
	File        string
	SHA256      string
	LocalSHA256 string
}

// ScriptCheck hashes file and the Jamf script body. id empty resolves file by
// download name. A missing script is Class missing, not an API error. More than
// one download-name owner is an error. Bytes are compared as stored.
func ScriptCheck(credName, file, id string, logger *slog.Logger) (ScriptCheckResult, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return ScriptCheckResult{}, fmt.Errorf("read script file %q: %w", file, err)
	}
	local := sha256Hex(data)
	base := filepath.Base(file)
	id = strings.TrimSpace(id)
	var s *ScriptDetail
	if id != "" {
		got, err := scriptGetFull(credName, id)
		if err != nil {
			if scriptIsNotFound(err) {
				return ScriptCheckResult{Class: ScriptCheckMissing, File: base, LocalSHA256: local}, nil
			}
			return ScriptCheckResult{}, err
		}
		s = got
	} else {
		hits, err := scriptHitsByDownloadName(credName, base)
		if err != nil {
			return ScriptCheckResult{}, err
		}
		if len(hits) > 1 {
			return ScriptCheckResult{}, fmt.Errorf("multiple scripts download as %q: %s", base, formatScriptHits(hits))
		}
		if len(hits) == 0 {
			return ScriptCheckResult{Class: ScriptCheckMissing, File: base, LocalSHA256: local}, nil
		}
		got, err := scriptGetFull(credName, hits[0].ID)
		if err != nil {
			return ScriptCheckResult{}, err
		}
		s = got
	}
	remote := sha256Hex([]byte(s.ScriptContents))
	res := ScriptCheckResult{
		Class: ScriptCheckMismatch, ID: s.ID, Name: s.Name, File: base,
		SHA256: remote, LocalSHA256: local,
	}
	if remote == local {
		res.Class = ScriptCheckMatch
		res.OK = true
	}
	return res, nil
}

// ScriptCheckPrint hashes and compares, prints the report, and returns an error
// unless the SHA-256 values match. API and I/O errors are returned before a report.
func ScriptCheckPrint(credName, file, id string, logger *slog.Logger) error {
	res, err := ScriptCheck(credName, file, id, logger)
	if err != nil {
		return err
	}
	printScriptCheck(res)
	if !res.OK {
		return fmt.Errorf("script check: %s", res.Class)
	}
	return nil
}

func printScriptCheck(res ScriptCheckResult) {
	fmt.Printf("Script check: %s\n", res.Class)
	if res.ID != "" {
		fmt.Printf("  id:           %s\n", res.ID)
	}
	if res.Name != "" {
		fmt.Printf("  name:         %s\n", res.Name)
	}
	fmt.Printf("  file:         %s\n", res.File)
	if res.Class != ScriptCheckMissing {
		fmt.Printf("  sha256:       %s\n", res.SHA256)
	}
	fmt.Printf("  local sha256: %s\n", res.LocalSHA256)
	if res.OK {
		fmt.Printf("  match:        yes\n")
	} else {
		fmt.Printf("  match:        no\n")
	}
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func scriptIsNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "status 404")
}

// ScriptDownload fetches a script and writes its contents to a local file.
// idOrName is a numeric script id or an exact Jamf name. If outfile is empty,
// the filename is derived from the script's name via scriptFilename.
func ScriptDownload(credName, idOrName, outfile string, logger *slog.Logger) (string, error) {
	id, err := scriptIDFromNameOrID(credName, idOrName)
	if err != nil {
		return "", err
	}
	s, err := scriptGetFull(credName, id)
	if err != nil {
		return "", err
	}
	if outfile == "" {
		outfile = scriptFilename(s.Name)
	}
	if err := os.WriteFile(outfile, []byte(s.ScriptContents), 0644); err != nil {
		return "", fmt.Errorf("write %q: %w", outfile, err)
	}
	fmt.Printf("Downloaded script %s (%s) → %s\n", s.ID, s.Name, outfile)
	return outfile, nil
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
