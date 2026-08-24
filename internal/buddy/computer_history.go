package buddy

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"unicode"
)

const uriComputerHistory = "/JSSResource/computerhistory"

// ComputerHistoryRef selects one computer for Classic computerhistory.
// Exactly one of ID, Name, Serial, or UDID must be set.
type ComputerHistoryRef struct {
	ID     string
	Name   string
	Serial string
	UDID   string
}

// ComputerPolicyLog is one flattened Classic policy_log row.
type ComputerPolicyLog struct {
	PolicyID      int    `json:"policy_id"`
	PolicyName    string `json:"policy_name"`
	Username      string `json:"username,omitempty"`
	DateTime      string `json:"date_time,omitempty"`
	DateTimeEpoch int64  `json:"date_time_epoch,omitempty"`
	DateTimeUTC   string `json:"date_time_utc,omitempty"`
	Status        string `json:"status,omitempty"`
}

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
func ComputerPolicyLogs(credName string, ref ComputerHistoryRef, policyID *int, status string, logger *slog.Logger) ([]ComputerPolicyLog, error) {
	h, err := ComputerHistoryGet(credName, ref, []string{"PolicyLogs"}, logger)
	if err != nil {
		return nil, err
	}
	logs := h.PolicyLogs
	if logs == nil {
		logs = []ComputerPolicyLog{}
	}
	return filterPolicyLogs(logs, policyID, status), nil
}

func filterPolicyLogs(logs []ComputerPolicyLog, policyID *int, status string) []ComputerPolicyLog {
	status = strings.TrimSpace(status)
	if policyID == nil && status == "" {
		return logs
	}
	out := make([]ComputerPolicyLog, 0, len(logs))
	for _, l := range logs {
		if policyID != nil && l.PolicyID != *policyID {
			continue
		}
		if status != "" && !strings.EqualFold(l.Status, status) {
			continue
		}
		out = append(out, l)
	}
	return out
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
func ComputerPolicyLogsPrint(credName string, ref ComputerHistoryRef, policyID *int, status string, asJSON bool, logger *slog.Logger) error {
	logs, err := ComputerPolicyLogs(credName, ref, policyID, status, logger)
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
	fmt.Printf("%-8s  %-32s  %-12s  %-20s  %s\n", "POLICY", "NAME", "STATUS", "DATE", "USER")
	fmt.Printf("%-8s  %-32s  %-12s  %-20s  %s\n", "--------", "--------------------------------", "------------", "--------------------", "--------------------")
	for _, l := range logs {
		fmt.Printf("%-8d  %-32s  %-12s  %-20s  %s\n",
			l.PolicyID,
			truncateComputer(l.PolicyName, 32),
			truncateComputer(l.Status, 12),
			truncateComputer(l.DateTime, 20),
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
	epoch, _ := anyToInt64(m["date_time_epoch"])
	return ComputerPolicyLog{
		PolicyID:      id,
		PolicyName:    anyToString(m["policy_name"]),
		Username:      anyToString(m["username"]),
		DateTime:      anyToString(m["date_time"]),
		DateTimeEpoch: epoch,
		DateTimeUTC:   anyToString(m["date_time_utc"]),
		Status:        anyToString(m["status"]),
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
	PolicyID      int    `xml:"policy_id"`
	PolicyName    string `xml:"policy_name"`
	Username      string `xml:"username"`
	DateTime      string `xml:"date_time"`
	DateTimeEpoch string `xml:"date_time_epoch"`
	DateTimeUTC   string `xml:"date_time_utc"`
	Status        string `xml:"status"`
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
			epoch, _ := anyToInt64(l.DateTimeEpoch)
			h.PolicyLogs = append(h.PolicyLogs, ComputerPolicyLog{
				PolicyID:      l.PolicyID,
				PolicyName:    l.PolicyName,
				Username:      l.Username,
				DateTime:      l.DateTime,
				DateTimeEpoch: epoch,
				DateTimeUTC:   l.DateTimeUTC,
				Status:        l.Status,
			})
		}
	}
	return h, nil
}
