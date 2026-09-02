package buddy

import (
	"strings"
	"testing"
	"time"
)

func TestComputerHistoryPath(t *testing.T) {
	path, err := computerHistoryPath(ComputerHistoryRef{ID: "101"}, []string{"PolicyLogs"})
	if err != nil {
		t.Fatal(err)
	}
	if path != "/JSSResource/computerhistory/id/101/subset/PolicyLogs" {
		t.Fatalf("path=%s", path)
	}

	path, err = computerHistoryPath(ComputerHistoryRef{Serial: "C02Q7KHTGFWF"}, []string{"General", "PolicyLogs"})
	if err != nil {
		t.Fatal(err)
	}
	if path != "/JSSResource/computerhistory/serialnumber/C02Q7KHTGFWF/subset/General&PolicyLogs" {
		t.Fatalf("combined path=%s", path)
	}

	path, err = computerHistoryPath(ComputerHistoryRef{Name: "Admins Mac"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if path != "/JSSResource/computerhistory/name/Admins%20Mac" {
		t.Fatalf("name path=%s", path)
	}

	if _, err := computerHistoryPath(ComputerHistoryRef{}, nil); err == nil {
		t.Fatal("expected XOR error")
	}
	if _, err := computerHistoryPath(ComputerHistoryRef{ID: "1", Name: "x"}, nil); err == nil {
		t.Fatal("expected XOR error for two keys")
	}
	if _, err := computerHistoryPath(ComputerHistoryRef{ID: "1"}, []string{"Nope"}); err == nil {
		t.Fatal("expected unknown subset")
	}
}

func TestParseComputerHistory_JSON_Wrapped(t *testing.T) {
	body := []byte(`{
  "computer_history": {
    "general": {"id": 101, "name": "Admins MacBook Pro", "serial_number": "C02Q7KHTGFWF"},
    "policy_logs": [
      {"policy_log": {
        "policy_id": 1,
        "policy_name": "Update Inventory",
        "username": "admin",
        "date_time": "2017-07-07 18:37:04",
        "date_time_epoch": "1499470624555",
        "date_time_utc": "2017-07-07T18:37:04.555-0500",
        "status": "Completed"
      }}
    ]
  }
}`)
	h, err := parseComputerHistory(body)
	if err != nil {
		t.Fatal(err)
	}
	if h.General["name"] != "Admins MacBook Pro" {
		t.Fatalf("general=%v", h.General)
	}
	if len(h.PolicyLogs) != 1 {
		t.Fatalf("logs=%d", len(h.PolicyLogs))
	}
	l := h.PolicyLogs[0]
	if l.PolicyID != 1 || l.PolicyName != "Update Inventory" || l.Status != "Completed" {
		t.Fatalf("log=%+v", l)
	}
	if l.DateTimeEpoch != 1499470624555 {
		t.Fatalf("epoch=%d", l.DateTimeEpoch)
	}
	if l.DateCompleted != l.DateTime || l.DateCompletedEpoch != l.DateTimeEpoch || l.DateCompletedUTC != l.DateTimeUTC {
		t.Fatalf("date_completed aliases: %+v", l)
	}
	m := h.AsMap()
	logs, ok := m["policy_logs"].([]ComputerPolicyLog)
	if !ok || len(logs) != 1 {
		t.Fatalf("AsMap policy_logs=%T %v", m["policy_logs"], m["policy_logs"])
	}
}

func TestParseComputerHistory_JSON_FlatItems(t *testing.T) {
	body := []byte(`{
  "computer_history": {
    "policy_logs": [
      {"policy_id": 42, "policy_name": "Install Chrome", "status": "Failed", "username": "u"}
    ]
  }
}`)
	h, err := parseComputerHistory(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.PolicyLogs) != 1 || h.PolicyLogs[0].PolicyID != 42 || h.PolicyLogs[0].Status != "Failed" {
		t.Fatalf("got %+v", h.PolicyLogs)
	}
}

func TestParseComputerHistory_JSON_UnwrappedRoot(t *testing.T) {
	body := []byte(`{
  "general": {"id": 7, "name": "Foo"},
  "policy_logs": [{"policy_id": 9, "policy_name": "Bar", "status": "Completed"}]
}`)
	h, err := parseComputerHistory(body)
	if err != nil {
		t.Fatal(err)
	}
	if h.General["name"] != "Foo" || len(h.PolicyLogs) != 1 || h.PolicyLogs[0].PolicyID != 9 {
		t.Fatalf("got %+v %+v", h.General, h.PolicyLogs)
	}
}

func TestParseComputerHistory_JSON_XMLStyleBag(t *testing.T) {
	body := []byte(`{
  "computer_history": {
    "policy_logs": {
      "size": 1,
      "policy_log": {"policy_id": 3, "policy_name": "Once", "status": "Completed"}
    }
  }
}`)
	h, err := parseComputerHistory(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.PolicyLogs) != 1 || h.PolicyLogs[0].PolicyID != 3 {
		t.Fatalf("got %+v", h.PolicyLogs)
	}
}

func TestParseComputerHistory_XML(t *testing.T) {
	body := []byte(`<computer_history>
  <general>
    <id>101</id>
    <name>Admins MacBook Pro</name>
    <serial_number>C02Q7KHTGFWF</serial_number>
  </general>
  <policy_logs>
    <policy_log>
      <policy_id>1</policy_id>
      <policy_name>Update Inventory</policy_name>
      <username>admin</username>
      <date_time>2017-07-07 18:37:04</date_time>
      <date_time_epoch>1499470624555</date_time_epoch>
      <status>Completed</status>
    </policy_log>
  </policy_logs>
</computer_history>`)
	h, err := parseComputerHistory(body)
	if err != nil {
		t.Fatal(err)
	}
	if h.General["name"] != "Admins MacBook Pro" {
		t.Fatalf("general=%v", h.General)
	}
	if len(h.PolicyLogs) != 1 || h.PolicyLogs[0].PolicyID != 1 || h.PolicyLogs[0].DateTimeEpoch != 1499470624555 {
		t.Fatalf("logs=%+v", h.PolicyLogs)
	}
	if h.PolicyLogs[0].DateCompleted != h.PolicyLogs[0].DateTime {
		t.Fatalf("XML date_completed alias=%+v", h.PolicyLogs[0])
	}
}

func TestParseComputerHistory_JSON_DateCompletedOnly(t *testing.T) {
	body := []byte(`{
  "policy_logs": [{"date_completed": "2026-08-01 10:00:00", "date_completed_epoch": 1754042400000, "date_completed_utc": "2026-08-01T00:00:00Z", "policy_id": 5, "policy_name": "X", "status": "Completed"}]
}`)
	h, err := parseComputerHistory(body)
	if err != nil {
		t.Fatal(err)
	}
	l := h.PolicyLogs[0]
	if l.DateCompleted != "2026-08-01 10:00:00" || l.DateTime != l.DateCompleted {
		t.Fatalf("got %+v", l)
	}
	if l.DateCompletedEpoch != 1754042400000 || l.DateTimeEpoch != l.DateCompletedEpoch {
		t.Fatalf("epoch %+v", l)
	}
}

func TestPolicyLogsScanFilter(t *testing.T) {
	got, err := policyLogsScanFilter("", "2026-08-23T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if got != "general.lastContact=ge=2026-08-23T00:00:00Z" {
		t.Fatalf("last_contact: %s", got)
	}
	got, err = policyLogsScanFilter("lab", "2026-08-23T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if got != `general.name=="*lab*";general.lastContact=ge=2026-08-23T00:00:00Z` {
		t.Fatalf("and: %s", got)
	}
	if _, err := lastContactGeClause(`general.lastContact=ge=2026-01-01T00:00:00Z`); err == nil {
		t.Fatal("expected last_contact RSQL reject")
	}
}

func TestFilterPolicyLogs(t *testing.T) {
	logs := []ComputerPolicyLog{
		{PolicyID: 1, PolicyName: "A", Status: "Completed", DateCompletedEpoch: 1_700_000_000_000},
		{PolicyID: 2, PolicyName: "B", Status: "Failed", DateCompletedEpoch: 1_800_000_000_000},
		{PolicyID: 1, PolicyName: "A", Status: "Failed", DateCompletedEpoch: 1_750_000_000_000},
	}
	id := 1
	got, err := filterPolicyLogs(logs, PolicyLogFilter{PolicyID: &id})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("policy_id filter: %d", len(got))
	}
	got, err = filterPolicyLogs(logs, PolicyLogFilter{Status: "failed"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("status filter: %d", len(got))
	}
	got, err = filterPolicyLogs(logs, PolicyLogFilter{PolicyID: &id, Status: "Completed"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Status != "Completed" {
		t.Fatalf("combined: %+v", got)
	}
	got, err = filterPolicyLogs(logs, PolicyLogFilter{Since: "2023-01-01T00:00:00Z", Until: "2026-01-01T00:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("since/until: %d %+v", len(got), got)
	}
}

func TestParseComputersClassicBasic(t *testing.T) {
	recs, err := parseComputersClassicBasic([]byte(`{"computers":[{"id":1,"name":"Ada","serial_number":"C02A"},{"id":2,"name":"Bob"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 || recs[0].ID != 1 || recs[0].Name != "Ada" || recs[0].SerialNumber != "C02A" || recs[1].ID != 2 {
		t.Fatalf("json list: %+v", recs)
	}
	recs, err = parseComputersClassicBasic([]byte(`{"computers":{"size":1,"computer":{"id":9,"name":"Nested","serial_number":"X"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].ID != 9 || recs[0].Name != "Nested" {
		t.Fatalf("json bag: %+v", recs)
	}
	recs, err = parseComputersClassicBasic([]byte(`
<computers>
  <size>1</size>
  <computer>
    <id>7</id>
    <name>XML Mac</name>
    <serial_number>S7</serial_number>
  </computer>
</computers>`))
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].ID != 7 || recs[0].Name != "XML Mac" || recs[0].SerialNumber != "S7" {
		t.Fatalf("xml: %+v", recs)
	}
}

func TestApplySkipLimit(t *testing.T) {
	recs := []ComputerRecord{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}}
	got, err := applySkipLimit(recs, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != 2 || got[1].ID != 3 {
		t.Fatalf("got %+v", got)
	}
	got, err = applySkipLimit(recs, 10, 2)
	if err != nil || len(got) != 0 {
		t.Fatalf("past end: %+v %v", got, err)
	}
}

func TestPolicyLogsComputerSource(t *testing.T) {
	if _, err := policyLogsComputerSource(PolicyLogsQuery{PolicyID: 1}); err == nil || !strings.Contains(err.Error(), "unconstrained") {
		t.Fatalf("want unconstrained, got %v", err)
	}
	src, err := policyLogsComputerSource(PolicyLogsQuery{PolicyID: 1, Limit: 20})
	if err != nil || src != policyLogsSrcClassicBasic {
		t.Fatalf("classic basic: src=%v err=%v", src, err)
	}
	src, err = policyLogsComputerSource(PolicyLogsQuery{PolicyID: 1, ComputerIDs: []string{"1", "2"}})
	if err != nil || src != policyLogsSrcComputerIDs {
		t.Fatalf("ids: src=%v err=%v", src, err)
	}
	if _, err := policyLogsComputerSource(PolicyLogsQuery{PolicyID: 1, ComputerID: "9", LastContact: "2026-01-01T00:00:00Z"}); err == nil || !strings.Contains(err.Error(), "not both") {
		t.Fatalf("want xor error, got %v", err)
	}
	if _, err := policyLogsComputerSource(PolicyLogsQuery{PolicyID: 1, ComputerID: "9", Limit: 5}); err == nil {
		t.Fatal("computer_id+limit should fail")
	}
}

func TestParseLogTimeBound(t *testing.T) {
	ms, err := parseLogTimeBound("2026-08-23T00:00:00Z", "since")
	if err != nil {
		t.Fatal(err)
	}
	if ms != time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC).UnixMilli() {
		t.Fatalf("iso ms=%d", ms)
	}
	ms, err = parseLogTimeBound("1700000000", "since")
	if err != nil || ms != 1_700_000_000_000 {
		t.Fatalf("seconds: %d %v", ms, err)
	}
	ms, err = parseLogTimeBound("1700000000000", "since")
	if err != nil || ms != 1_700_000_000_000 {
		t.Fatalf("millis: %d %v", ms, err)
	}
}

func TestNormalizeHistorySubsets(t *testing.T) {
	got, err := normalizeHistorySubsets([]string{"policy_logs", "General&Commands"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "PolicyLogs,General,Commands" {
		t.Fatalf("got %v", got)
	}
}
