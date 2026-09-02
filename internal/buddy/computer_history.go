package buddy

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	uriComputerHistory             = "/JSSResource/computerhistory"
	uriComputersClassicBasic       = "/JSSResource/computers/subset/basic"
	epochMillisCutoff        int64 = 1e12
)

// ComputerHistoryRef selects one computer for Classic computerhistory.
// Exactly one of ID, Name, Serial, or UDID must be set.
type ComputerHistoryRef struct {
	ID     string
	Name   string
	Serial string
	UDID   string
}

// ComputerPolicyLog is one flattened Classic policy_log row.
// Classic documents the run time as date_time*; Jamf UI calls it Date Completed.
// Both families are populated (date_completed* aliases date_time* when absent).
type ComputerPolicyLog struct {
	PolicyID           int    `json:"policy_id"`
	PolicyName         string `json:"policy_name"`
	Username           string `json:"username,omitempty"`
	DateTime           string `json:"date_time,omitempty"`
	DateTimeEpoch      int64  `json:"date_time_epoch,omitempty"`
	DateTimeUTC        string `json:"date_time_utc,omitempty"`
	DateCompleted      string `json:"date_completed,omitempty"`
	DateCompletedEpoch int64  `json:"date_completed_epoch,omitempty"`
	DateCompletedUTC   string `json:"date_completed_utc,omitempty"`
	Status             string `json:"status,omitempty"`
	ComputerID         int    `json:"computer_id,omitempty"`
	ComputerName       string `json:"computer_name,omitempty"`
	Serial             string `json:"serial,omitempty"`
}

// PolicyLogsQuery selects computers and client-side log filters for policy logs.
//
// Computer source is one of: ComputerID, ComputerIDs, last_contact/filter (Pro v4
// inventory), or Limit>0 (Classic GET /computers/subset/basic). Unconstrained
// fleet scan is rejected. last_contact/filter bound the computer list, not log
// dates — use Since/Until on date_completed_epoch.
type PolicyLogsQuery struct {
	PolicyID    int
	ComputerID  string
	ComputerIDs []string
	Filter      string // computers() RSQL or bare term (v4 inventory; not log date)
	LastContact string // ISO datetime → general.lastContact=ge= (inventory, not "logs since")
	Status      string
	Since       string // log date lower bound (ISO or epoch)
	Until       string // log date upper bound (ISO or epoch)
	Skip        int    // computers to skip after the candidate list is built
	Limit       int    // max computers to scan; required for Classic-basic source
}

// PolicyLogFilter is client-side filtering after Classic PolicyLogs flatten.
type PolicyLogFilter struct {
	PolicyID *int
	Status   string
	Since    string
	Until    string
}

type policyLogsSrc int

const (
	policyLogsSrcNone policyLogsSrc = iota
	policyLogsSrcComputerID
	policyLogsSrcComputerIDs
	policyLogsSrcInventory
	policyLogsSrcClassicBasic
)

// ComputerHistory is a parsed Classic computer_history document.
// PolicyLogs are flattened; Other holds remaining subsets after wrapper unwrap.
type ComputerHistory struct {
	General    map[string]interface{} `json:"general,omitempty"`
	PolicyLogs []ComputerPolicyLog    `json:"policy_logs,omitempty"`
	Other      map[string]interface{} `json:"-"`
}

// AsMap returns a JSON-friendly dict (general, flattened policy_logs, other subsets).
func (h *ComputerHistory) AsMap() map[string]interface{} {
	if h == nil {
		return map[string]interface{}{}
	}
	out := make(map[string]interface{}, 2+len(h.Other))
	if h.General != nil {
		out["general"] = h.General
	}
	if h.PolicyLogs != nil {
		out["policy_logs"] = h.PolicyLogs
	}
	for k, v := range h.Other {
		if k == "general" || k == "policy_logs" {
			continue
		}
		out[k] = v
	}
	return out
}

var computerHistorySubsetCanon = map[string]string{
	"general":                 "General",
	"computerusagelogs":       "ComputerUsageLogs",
	"audits":                  "Audits",
	"policylogs":              "PolicyLogs",
	"casperremotelogs":        "CasperRemoteLogs",
	"screensharinglogs":       "ScreenSharingLogs",
	"casperimaginglogs":       "CasperImagingLogs",
	"commands":                "Commands",
	"userlocation":            "UserLocation",
	"macappstoreapplications": "MacAppStoreApplications",
}

var classicLogChildKeys = []string{
	"policy_log", "usage_log", "audit", "casper_remote_log",
	"screen_sharing_log", "casper_imaging_log", "command", "location", "app",
}

// computerHistoryPath builds GET /JSSResource/computerhistory/{key}/{value}[/subset/…].
func computerHistoryPath(ref ComputerHistoryRef, subsets []string) (string, error) {
	kind, value, err := ref.key()
	if err != nil {
		return "", err
	}
	canon, err := normalizeHistorySubsets(subsets)
	if err != nil {
		return "", err
	}
	path := fmt.Sprintf("%s/%s/%s", uriComputerHistory, kind, url.PathEscape(value))
	if len(canon) > 0 {
		path += "/subset/" + strings.Join(canon, "&")
	}
	return path, nil
}

func (r ComputerHistoryRef) key() (kind, value string, err error) {
	type pair struct{ kind, value string }
	var hits []pair
	if s := strings.TrimSpace(r.ID); s != "" {
		hits = append(hits, pair{"id", s})
	}
	if s := strings.TrimSpace(r.Name); s != "" {
		hits = append(hits, pair{"name", s})
	}
	if s := strings.TrimSpace(r.Serial); s != "" {
		hits = append(hits, pair{"serialnumber", s})
	}
	if s := strings.TrimSpace(r.UDID); s != "" {
		hits = append(hits, pair{"udid", s})
	}
	if len(hits) != 1 {
		return "", "", fmt.Errorf("computer history: pass exactly one of id, name, serial, or udid")
	}
	return hits[0].kind, hits[0].value, nil
}

func normalizeHistorySubsets(subsets []string) ([]string, error) {
	var out []string
	seen := make(map[string]struct{})
	for _, s := range subsets {
		for _, part := range strings.FieldsFunc(s, func(r rune) bool {
			return r == ',' || r == '&'
		}) {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			canon, ok := canonicalHistorySubset(part)
			if !ok {
				return nil, fmt.Errorf("unknown computer history subset %q", part)
			}
			if _, dup := seen[canon]; dup {
				continue
			}
			seen[canon] = struct{}{}
			out = append(out, canon)
		}
	}
	return out, nil
}

func canonicalHistorySubset(s string) (string, bool) {
	key := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s), "_", ""))
	key = strings.ReplaceAll(key, "-", "")
	canon, ok := computerHistorySubsetCanon[key]
	return canon, ok
}

// ComputerHistoryFetch returns the raw Classic computerhistory body (JSON Accept).
func ComputerHistoryFetch(credName string, ref ComputerHistoryRef, subsets []string) ([]byte, error) {
	path, err := computerHistoryPath(ref, subsets)
	if err != nil {
		return nil, err
	}
	body, err := jamfGETWithAccept(credName, path, "application/json")
	if err != nil {
		return nil, fmt.Errorf("get computer history: %w", err)
	}
	return body, nil
}

// ComputerHistoryGet fetches and parses Classic computer history.
func ComputerHistoryGet(credName string, ref ComputerHistoryRef, subsets []string, _ *slog.Logger) (*ComputerHistory, error) {
	body, err := ComputerHistoryFetch(credName, ref, subsets)
	if err != nil {
		return nil, err
	}
	return parseComputerHistory(body)
}

// ComputerPolicyLogs fetches subset PolicyLogs and applies optional client-side filters.
func ComputerPolicyLogs(credName string, ref ComputerHistoryRef, f PolicyLogFilter, logger *slog.Logger) ([]ComputerPolicyLog, error) {
	h, err := ComputerHistoryGet(credName, ref, []string{"PolicyLogs"}, logger)
	if err != nil {
		return nil, err
	}
	logs := h.PolicyLogs
	if logs == nil {
		logs = []ComputerPolicyLog{}
	}
	logs, err = filterPolicyLogs(logs, f)
	if err != nil {
		return nil, err
	}
	attachHistoryComputer(logs, h, ref.ID)
	return logs, nil
}

// PolicyLogs returns flattened logs for one policy on one computer or a bounded
// computer scan. Jamf has no GET /policies/{id}/logs — each computer is still
// Classic computerhistory (full PolicyLogs payload, then client-side policy_id).
func PolicyLogs(credName string, q PolicyLogsQuery, logger *slog.Logger) ([]ComputerPolicyLog, error) {
	if q.PolicyID <= 0 {
		return nil, fmt.Errorf("policy logs: policy id is required")
	}
	src, err := policyLogsComputerSource(q)
	if err != nil {
		return nil, err
	}
	pid := q.PolicyID
	f := PolicyLogFilter{PolicyID: &pid, Status: q.Status, Since: q.Since, Until: q.Until}
	if src == policyLogsSrcComputerID {
		return ComputerPolicyLogs(credName, ComputerHistoryRef{ID: strings.TrimSpace(q.ComputerID)}, f, logger)
	}
	recs, err := policyLogsScanCandidates(credName, q, src, logger)
	if err != nil {
		return nil, err
	}
	var all []ComputerPolicyLog
	for i, rec := range recs {
		policyLogsProgress(i, len(recs), rec)
		logs, err := ComputerPolicyLogs(credName, ComputerHistoryRef{ID: strconv.Itoa(rec.ID)}, f, logger)
		if err != nil {
			return nil, fmt.Errorf("policy logs computer %d: %w", rec.ID, err)
		}
		attachComputer(logs, rec)
		all = append(all, logs...)
	}
	if all == nil {
		all = []ComputerPolicyLog{}
	}
	return all, nil
}

func policyLogsComputerSource(q PolicyLogsQuery) (policyLogsSrc, error) {
	computerID := strings.TrimSpace(q.ComputerID)
	ids := trimComputerIDList(q.ComputerIDs)
	inventory := strings.TrimSpace(q.Filter) != "" || strings.TrimSpace(q.LastContact) != ""
	n := 0
	if computerID != "" {
		n++
	}
	if len(ids) > 0 {
		n++
	}
	if inventory {
		n++
	}
	if n > 1 {
		return policyLogsSrcNone, fmt.Errorf("policy logs: pass one of computer_id, computer_ids, or last_contact=/filter=, not both")
	}
	if computerID != "" {
		if q.Skip != 0 || q.Limit != 0 {
			return policyLogsSrcNone, fmt.Errorf("policy logs: skip=/limit= apply to scans, not computer_id=")
		}
		return policyLogsSrcComputerID, nil
	}
	if q.Skip < 0 {
		return policyLogsSrcNone, fmt.Errorf("policy logs: skip must be >= 0")
	}
	if q.Limit < 0 {
		return policyLogsSrcNone, fmt.Errorf("policy logs: limit must be >= 0")
	}
	if len(ids) > 0 {
		return policyLogsSrcComputerIDs, nil
	}
	if inventory {
		return policyLogsSrcInventory, nil
	}
	if q.Limit > 0 {
		return policyLogsSrcClassicBasic, nil
	}
	return policyLogsSrcNone, fmt.Errorf("policy logs: require computer_id=, computer_ids=, last_contact=/filter=, or limit= (no unconstrained fleet scan)")
}

func policyLogsScanCandidates(credName string, q PolicyLogsQuery, src policyLogsSrc, logger *slog.Logger) ([]ComputerRecord, error) {
	var recs []ComputerRecord
	switch src {
	case policyLogsSrcComputerIDs:
		recs = recordsFromComputerIDs(q.ComputerIDs)
	case policyLogsSrcInventory:
		scanFilter, err := policyLogsScanFilter(strings.TrimSpace(q.Filter), strings.TrimSpace(q.LastContact))
		if err != nil {
			return nil, err
		}
		recs, err = ComputerSearch(credName, ComputerListQuery{Filter: scanFilter}, PageOpts{Page: PageAll}, logger)
		if err != nil {
			return nil, err
		}
	case policyLogsSrcClassicBasic:
		var err error
		recs, err = computersFetchClassicBasic(credName)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("policy logs: internal: unknown computer source")
	}
	return applySkipLimit(recs, q.Skip, q.Limit)
}

func computersFetchClassicBasic(credName string) ([]ComputerRecord, error) {
	body, err := jamfGETWithAccept(credName, uriComputersClassicBasic, "application/json")
	if err != nil {
		return nil, fmt.Errorf("list computers (classic basic): %w", err)
	}
	return parseComputersClassicBasic(body)
}

func parseComputersClassicBasic(body []byte) ([]ComputerRecord, error) {
	trimmed := bytes.TrimLeftFunc(body, unicode.IsSpace)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("parse computers basic: empty body")
	}
	if trimmed[0] == '<' {
		return parseComputersClassicBasicXML(body)
	}
	var raw interface{}
	if err := json.Unmarshal(trimmed, &raw); err != nil {
		return nil, fmt.Errorf("parse computers basic: %w", err)
	}
	items := classicCollectionMaps(raw, "computers", "computer")
	out := make([]ComputerRecord, 0, len(items))
	for _, m := range items {
		id, ok := anyToInt(m["id"])
		if !ok || id <= 0 {
			continue
		}
		rec := ComputerRecord{ID: id, Name: anyToString(m["name"]), UDID: anyToString(m["udid"])}
		rec.SerialNumber = anyToString(m["serial_number"])
		if rec.SerialNumber == "" {
			rec.SerialNumber = anyToString(m["serialNumber"])
		}
		out = append(out, rec)
	}
	return out, nil
}

func parseComputersClassicBasicXML(body []byte) ([]ComputerRecord, error) {
	var doc struct {
		Computers []struct {
			ID     int    `xml:"id"`
			Name   string `xml:"name"`
			Serial string `xml:"serial_number"`
			UDID   string `xml:"udid"`
		} `xml:"computer"`
	}
	if err := xml.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("parse computers basic xml: %w", err)
	}
	out := make([]ComputerRecord, 0, len(doc.Computers))
	for _, c := range doc.Computers {
		if c.ID <= 0 {
			continue
		}
		out = append(out, ComputerRecord{ID: c.ID, Name: c.Name, SerialNumber: c.Serial, UDID: c.UDID})
	}
	return out, nil
}

func classicCollectionMaps(v interface{}, collectionKey, itemKey string) []map[string]interface{} {
	switch t := v.(type) {
	case []interface{}:
		return mapsFromInterfaceList(t, itemKey)
	case map[string]interface{}:
		if inner, ok := t[collectionKey]; ok {
			return classicCollectionMaps(inner, collectionKey, itemKey)
		}
		if inner, ok := t[itemKey]; ok {
			return classicCollectionMaps(inner, collectionKey, itemKey)
		}
		if _, ok := t["id"]; ok {
			return []map[string]interface{}{t}
		}
	}
	return nil
}

func mapsFromInterfaceList(items []interface{}, itemKey string) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if inner, ok := m[itemKey].(map[string]interface{}); ok {
			m = inner
		}
		out = append(out, m)
	}
	return out
}

func recordsFromComputerIDs(ids []string) []ComputerRecord {
	out := make([]ComputerRecord, 0, len(ids))
	for _, s := range trimComputerIDList(ids) {
		rec := ComputerRecord{}
		if n, err := strconv.Atoi(s); err == nil {
			rec.ID = n
		}
		out = append(out, rec)
	}
	return out
}

func trimComputerIDList(ids []string) []string {
	var out []string
	for _, s := range ids {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func applySkipLimit(recs []ComputerRecord, skip, limit int) ([]ComputerRecord, error) {
	if skip < 0 {
		return nil, fmt.Errorf("policy logs: skip must be >= 0")
	}
	if limit < 0 {
		return nil, fmt.Errorf("policy logs: limit must be >= 0")
	}
	if skip > 0 {
		if skip >= len(recs) {
			return []ComputerRecord{}, nil
		}
		recs = recs[skip:]
	}
	if limit > 0 && len(recs) > limit {
		recs = recs[:limit]
	}
	return recs, nil
}

func policyLogsProgress(i, n int, rec ComputerRecord) {
	if n <= 1 {
		return
	}
	name := rec.Name
	if name == "" {
		name = "-"
	}
	fmt.Fprintf(os.Stderr, "policy logs: %d/%d computer %d %s\n", i+1, n, rec.ID, name)
}

func policyLogsScanFilter(filter, lastContact string) (string, error) {
	normalized, err := NormalizeListFilter(filter, FilterSpecComputers)
	if err != nil {
		return "", err
	}
	ge, err := lastContactGeClause(lastContact)
	if err != nil {
		return "", err
	}
	joined := joinRSQL(normalized, ge)
	if joined == "" {
		return "", fmt.Errorf("policy logs: empty scan filter")
	}
	return joined, nil
}

func lastContactGeClause(iso string) (string, error) {
	iso = strings.TrimSpace(iso)
	if iso == "" {
		return "", nil
	}
	if isRSQLFilter(iso) || strings.Contains(iso, ";") {
		return "", fmt.Errorf("last_contact must be an ISO datetime (use filter= for RSQL)")
	}
	return "general.lastContact=ge=" + iso, nil
}

func joinRSQL(clauses ...string) string {
	var parts []string
	for _, c := range clauses {
		c = strings.TrimSpace(c)
		if c != "" {
			parts = append(parts, c)
		}
	}
	return strings.Join(parts, ";")
}

func attachHistoryComputer(logs []ComputerPolicyLog, h *ComputerHistory, fallbackID string) {
	rec := ComputerRecord{}
	if n, err := strconv.Atoi(strings.TrimSpace(fallbackID)); err == nil {
		rec.ID = n
	}
	if h != nil && h.General != nil {
		if id, ok := anyToInt(h.General["id"]); ok && id > 0 {
			rec.ID = id
		}
		if name := anyToString(h.General["name"]); name != "" {
			rec.Name = name
		}
		if serial := anyToString(h.General["serial_number"]); serial != "" {
			rec.SerialNumber = serial
		}
	}
	attachComputer(logs, rec)
}

func attachComputer(logs []ComputerPolicyLog, rec ComputerRecord) {
	for i := range logs {
		if rec.ID > 0 {
			logs[i].ComputerID = rec.ID
		}
		if rec.Name != "" {
			logs[i].ComputerName = rec.Name
		}
		if rec.SerialNumber != "" {
			logs[i].Serial = rec.SerialNumber
		}
	}
}

func filterPolicyLogs(logs []ComputerPolicyLog, f PolicyLogFilter) ([]ComputerPolicyLog, error) {
	status := strings.TrimSpace(f.Status)
	since, err := parseLogTimeBound(f.Since, "since")
	if err != nil {
		return nil, err
	}
	until, err := parseLogTimeBound(f.Until, "until")
	if err != nil {
		return nil, err
	}
	if f.PolicyID == nil && status == "" && since == 0 && until == 0 {
		return logs, nil
	}
	out := make([]ComputerPolicyLog, 0, len(logs))
	for _, l := range logs {
		if f.PolicyID != nil && l.PolicyID != *f.PolicyID {
			continue
		}
		if status != "" && !strings.EqualFold(l.Status, status) {
			continue
		}
		if since != 0 || until != 0 {
			ms := policyLogMillis(l)
			if ms == 0 {
				continue
			}
			if since != 0 && ms < since {
				continue
			}
			if until != 0 && ms > until {
				continue
			}
		}
		out = append(out, l)
	}
	return out, nil
}

func parseLogTimeBound(s, label string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return epochToMillis(n), nil
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC().UnixMilli(), nil
		}
	}
	return 0, fmt.Errorf("policy logs: %s must be ISO datetime or epoch, got %q", label, s)
}

func epochToMillis(n int64) int64 {
	if n == 0 {
		return 0
	}
	if n < epochMillisCutoff {
		return n * 1000
	}
	return n
}

func policyLogMillis(l ComputerPolicyLog) int64 {
	n := l.DateCompletedEpoch
	if n == 0 {
		n = l.DateTimeEpoch
	}
	if n != 0 {
		return epochToMillis(n)
	}
	for _, s := range []string{l.DateCompletedUTC, l.DateTimeUTC} {
		if s == "" {
			continue
		}
		ms, err := parseLogTimeBound(s, "date")
		if err == nil && ms > 0 {
			return ms
		}
	}
	return 0
}

// ComputerHistoryPrint prints the parsed history as a policy-log table, or --json dict.
func ComputerHistoryPrint(credName string, ref ComputerHistoryRef, subsets []string, asJSON bool, logger *slog.Logger) error {
	h, err := ComputerHistoryGet(credName, ref, subsets, logger)
	if err != nil {
		return err
	}
	if asJSON {
		return printComputerHistoryJSON(h.AsMap())
	}
	return printPolicyLogsTable(h.PolicyLogs)
}

// ComputerPolicyLogsPrint prints flattened policy logs as a table, or --json list.
func ComputerPolicyLogsPrint(credName string, ref ComputerHistoryRef, f PolicyLogFilter, asJSON bool, logger *slog.Logger) error {
	logs, err := ComputerPolicyLogs(credName, ref, f, logger)
	if err != nil {
		return err
	}
	if asJSON {
		return printComputerHistoryJSON(logs)
	}
	return printPolicyLogsTable(logs)
}

// PolicyLogsPrint prints PolicyLogs as a table, or --json list.
func PolicyLogsPrint(credName string, q PolicyLogsQuery, asJSON bool, logger *slog.Logger) error {
	logs, err := PolicyLogs(credName, q, logger)
	if err != nil {
		return err
	}
	if asJSON {
		return printComputerHistoryJSON(logs)
	}
	return printPolicyLogsTable(logs)
}

func printComputerHistoryJSON(v interface{}) error {
	body, err := json.Marshal(v)
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

func printPolicyLogsTable(logs []ComputerPolicyLog) error {
	if logs == nil {
		logs = []ComputerPolicyLog{}
	}
	showComputer := false
	for _, l := range logs {
		if l.ComputerID != 0 || l.ComputerName != "" {
			showComputer = true
			break
		}
	}
	if showComputer {
		fmt.Printf("%-8s  %-24s  %-8s  %-28s  %-12s  %-20s  %s\n", "COMPUTER", "HOST", "POLICY", "NAME", "STATUS", "COMPLETED", "USER")
		fmt.Printf("%-8s  %-24s  %-8s  %-28s  %-12s  %-20s  %s\n", "--------", "------------------------", "--------", "----------------------------", "------------", "--------------------", "--------------------")
	} else {
		fmt.Printf("%-8s  %-32s  %-12s  %-20s  %s\n", "POLICY", "NAME", "STATUS", "COMPLETED", "USER")
		fmt.Printf("%-8s  %-32s  %-12s  %-20s  %s\n", "--------", "--------------------------------", "------------", "--------------------", "--------------------")
	}
	for _, l := range logs {
		completed := l.DateCompleted
		if completed == "" {
			completed = l.DateTime
		}
		if showComputer {
			host := l.ComputerName
			if host == "" {
				host = strconv.Itoa(l.ComputerID)
			}
			fmt.Printf("%-8d  %-24s  %-8d  %-28s  %-12s  %-20s  %s\n",
				l.ComputerID,
				truncateComputer(host, 24),
				l.PolicyID,
				truncateComputer(l.PolicyName, 28),
				truncateComputer(l.Status, 12),
				truncateComputer(completed, 20),
				truncateComputer(l.Username, 20),
			)
			continue
		}
		fmt.Printf("%-8d  %-32s  %-12s  %-20s  %s\n",
			l.PolicyID,
			truncateComputer(l.PolicyName, 32),
			truncateComputer(l.Status, 12),
			truncateComputer(completed, 20),
			truncateComputer(l.Username, 20),
		)
	}
	fmt.Printf("\n%d policy log(s)\n", len(logs))
	return nil
}

func parseComputerHistory(body []byte) (*ComputerHistory, error) {
	trimmed := bytes.TrimLeftFunc(body, unicode.IsSpace)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("parse computer history: empty body")
	}
	if trimmed[0] == '<' {
		return parseComputerHistoryXML(body)
	}
	h, err := parseComputerHistoryJSON(body)
	if err != nil && bytes.Contains(trimmed, []byte("<")) {
		return parseComputerHistoryXML(body)
	}
	return h, err
}

func parseComputerHistoryJSON(body []byte) (*ComputerHistory, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse computer history JSON: %w", err)
	}
	root := unwrapComputerHistoryJSON(raw)
	flat, ok := flattenHistoryValue(root).(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("parse computer history JSON: unexpected root type")
	}
	return computerHistoryFromMap(flat)
}

func unwrapComputerHistoryJSON(raw map[string]interface{}) map[string]interface{} {
	if inner, ok := raw["computer_history"].(map[string]interface{}); ok {
		return inner
	}
	return raw
}

func flattenHistoryValue(v interface{}) interface{} {
	switch t := v.(type) {
	case []interface{}:
		out := make([]interface{}, 0, len(t))
		for _, item := range t {
			flat := unwrapLogItem(item)
			if flat != nil {
				out = append(out, flattenHistoryValue(flat))
			} else {
				out = append(out, flattenHistoryValue(item))
			}
		}
		return out
	case map[string]interface{}:
		if arr := coerceLogArray(t, ""); len(arr) > 0 && looksLikeLogBag(t) {
			return flattenHistoryValue(arr)
		}
		out := make(map[string]interface{}, len(t))
		for k, val := range t {
			if k == "size" {
				continue
			}
			if coerced := coerceLogArray(map[string]interface{}{k: val}, k); coerced != nil && isLogChildKey(k) {
				out[k] = flattenHistoryValue(coerced)
				continue
			}
			out[k] = flattenHistoryValue(val)
		}
		return out
	default:
		return v
	}
}

func looksLikeLogBag(m map[string]interface{}) bool {
	if _, ok := m["size"]; ok && len(m) <= 3 {
		for k := range m {
			if k != "size" && isLogChildKey(k) {
				return true
			}
		}
	}
	if len(m) == 1 {
		for k := range m {
			if isLogChildKey(k) {
				return true
			}
		}
	}
	return false
}

func isLogChildKey(k string) bool {
	for _, c := range classicLogChildKeys {
		if k == c {
			return true
		}
	}
	return false
}

func coerceLogArray(v interface{}, childKey string) []interface{} {
	switch t := v.(type) {
	case []interface{}:
		return t
	case map[string]interface{}:
		keys := classicLogChildKeys
		if childKey != "" {
			keys = []string{childKey}
		}
		for _, k := range keys {
			inner, ok := t[k]
			if !ok {
				continue
			}
			switch u := inner.(type) {
			case []interface{}:
				return wrapLogItems(u, k)
			case map[string]interface{}:
				return []interface{}{map[string]interface{}{k: u}}
			}
		}
	}
	return nil
}

func wrapLogItems(items []interface{}, childKey string) []interface{} {
	out := make([]interface{}, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]interface{}); ok {
			if _, has := m[childKey]; has {
				out = append(out, m)
			} else {
				out = append(out, map[string]interface{}{childKey: m})
			}
			continue
		}
		out = append(out, item)
	}
	return out
}

func unwrapLogItem(item interface{}) map[string]interface{} {
	m, ok := item.(map[string]interface{})
	if !ok {
		return nil
	}
	for _, k := range classicLogChildKeys {
		if inner, ok := m[k]; ok {
			if im, ok := inner.(map[string]interface{}); ok {
				return im
			}
		}
	}
	return m
}

func computerHistoryFromMap(root map[string]interface{}) (*ComputerHistory, error) {
	if root == nil {
		return nil, fmt.Errorf("parse computer history: empty document")
	}
	h := &ComputerHistory{Other: map[string]interface{}{}}
	if g, ok := root["general"].(map[string]interface{}); ok {
		h.General = g
	}
	if raw, ok := root["policy_logs"]; ok {
		h.PolicyLogs = policyLogsFromValue(raw)
	}
	for k, v := range root {
		if k == "general" || k == "policy_logs" {
			continue
		}
		h.Other[k] = v
	}
	return h, nil
}

func policyLogsFromValue(v interface{}) []ComputerPolicyLog {
	var items []interface{}
	switch t := v.(type) {
	case []interface{}:
		items = t
	case []map[string]interface{}:
		for _, m := range t {
			items = append(items, m)
		}
	default:
		if coerced := coerceLogArray(v, "policy_log"); coerced != nil {
			for _, item := range coerced {
				if flat := unwrapLogItem(item); flat != nil {
					items = append(items, flat)
				}
			}
		}
	}
	out := make([]ComputerPolicyLog, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			if flat := unwrapLogItem(item); flat != nil {
				m = flat
			} else {
				continue
			}
		}
		if inner := unwrapLogItem(m); inner != nil {
			m = inner
		}
		out = append(out, policyLogFromMap(m))
	}
	return out
}

func policyLogFromMap(m map[string]interface{}) ComputerPolicyLog {
	id, _ := anyToInt(m["policy_id"])
	l := ComputerPolicyLog{
		PolicyID:         id,
		PolicyName:       anyToString(m["policy_name"]),
		Username:         anyToString(m["username"]),
		DateTime:         anyToString(m["date_time"]),
		DateTimeUTC:      anyToString(m["date_time_utc"]),
		DateCompleted:    anyToString(m["date_completed"]),
		DateCompletedUTC: anyToString(m["date_completed_utc"]),
		Status:           anyToString(m["status"]),
	}
	if n, ok := anyToInt64(m["date_time_epoch"]); ok {
		l.DateTimeEpoch = n
	}
	if n, ok := anyToInt64(m["date_completed_epoch"]); ok {
		l.DateCompletedEpoch = n
	}
	normalizePolicyLogDates(&l)
	return l
}

func normalizePolicyLogDates(l *ComputerPolicyLog) {
	if l.DateTime == "" {
		l.DateTime = l.DateCompleted
	}
	if l.DateCompleted == "" {
		l.DateCompleted = l.DateTime
	}
	if l.DateTimeUTC == "" {
		l.DateTimeUTC = l.DateCompletedUTC
	}
	if l.DateCompletedUTC == "" {
		l.DateCompletedUTC = l.DateTimeUTC
	}
	if l.DateTimeEpoch == 0 {
		l.DateTimeEpoch = l.DateCompletedEpoch
	}
	if l.DateCompletedEpoch == 0 {
		l.DateCompletedEpoch = l.DateTimeEpoch
	}
}

func anyToString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case json.Number:
		return t.String()
	case float64:
		return strconv.FormatInt(int64(t), 10)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	default:
		if t == nil {
			return ""
		}
		return fmt.Sprint(t)
	}
}

func anyToInt(v interface{}) (int, bool) {
	n, ok := anyToInt64(v)
	return int(n), ok
}

func anyToInt64(v interface{}) (int64, bool) {
	switch t := v.(type) {
	case float64:
		return int64(t), true
	case json.Number:
		n, err := t.Int64()
		return n, err == nil
	case int:
		return int64(t), true
	case int64:
		return t, true
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return 0, false
		}
		n, err := strconv.ParseInt(s, 10, 64)
		return n, err == nil
	default:
		return 0, false
	}
}

type computerHistoryXML struct {
	XMLName    xml.Name                  `xml:"computer_history"`
	General    computerHistoryGeneralXML `xml:"general"`
	PolicyLogs []computerPolicyLogXML    `xml:"policy_logs>policy_log"`
}

type computerHistoryGeneralXML struct {
	ID           int    `xml:"id"`
	Name         string `xml:"name"`
	UDID         string `xml:"udid"`
	SerialNumber string `xml:"serial_number"`
	MacAddress   string `xml:"mac_address"`
}

type computerPolicyLogXML struct {
	PolicyID           int    `xml:"policy_id"`
	PolicyName         string `xml:"policy_name"`
	Username           string `xml:"username"`
	DateTime           string `xml:"date_time"`
	DateTimeEpoch      string `xml:"date_time_epoch"`
	DateTimeUTC        string `xml:"date_time_utc"`
	DateCompleted      string `xml:"date_completed"`
	DateCompletedEpoch string `xml:"date_completed_epoch"`
	DateCompletedUTC   string `xml:"date_completed_utc"`
	Status             string `xml:"status"`
}

func parseComputerHistoryXML(body []byte) (*ComputerHistory, error) {
	var doc computerHistoryXML
	if err := xml.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("parse computer history XML: %w", err)
	}
	h := &ComputerHistory{Other: map[string]interface{}{}}
	if doc.General.ID != 0 || doc.General.Name != "" {
		g := map[string]interface{}{}
		if doc.General.ID != 0 {
			g["id"] = doc.General.ID
		}
		if doc.General.Name != "" {
			g["name"] = doc.General.Name
		}
		if doc.General.UDID != "" {
			g["udid"] = doc.General.UDID
		}
		if doc.General.SerialNumber != "" {
			g["serial_number"] = doc.General.SerialNumber
		}
		if doc.General.MacAddress != "" {
			g["mac_address"] = doc.General.MacAddress
		}
		h.General = g
	}
	if doc.PolicyLogs != nil {
		h.PolicyLogs = make([]ComputerPolicyLog, 0, len(doc.PolicyLogs))
		for _, l := range doc.PolicyLogs {
			row := ComputerPolicyLog{
				PolicyID:         l.PolicyID,
				PolicyName:       l.PolicyName,
				Username:         l.Username,
				DateTime:         l.DateTime,
				DateTimeUTC:      l.DateTimeUTC,
				DateCompleted:    l.DateCompleted,
				DateCompletedUTC: l.DateCompletedUTC,
				Status:           l.Status,
			}
			if n, ok := anyToInt64(l.DateTimeEpoch); ok {
				row.DateTimeEpoch = n
			}
			if n, ok := anyToInt64(l.DateCompletedEpoch); ok {
				row.DateCompletedEpoch = n
			}
			normalizePolicyLogDates(&row)
			h.PolicyLogs = append(h.PolicyLogs, row)
		}
	}
	return h, nil
}
