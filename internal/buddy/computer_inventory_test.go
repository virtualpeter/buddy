package buddy

import (
	"encoding/json"
	"testing"
)

func TestNormalizeComputerSections(t *testing.T) {
	got := NormalizeComputerSections([]string{"general", "HARDWARE,storage", " GENERAL "})
	want := []string{"GENERAL", "HARDWARE", "STORAGE"}
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v want %#v", got, want)
		}
	}
}

func TestComputerInventoryPath(t *testing.T) {
	path, err := computerInventoryPath("42", nil)
	if err != nil {
		t.Fatal(err)
	}
	if path != uriComputersInventory+"/42" {
		t.Fatalf("path = %q", path)
	}
	path, err = computerInventoryPath("42", []string{"hardware", "GENERAL"})
	if err != nil {
		t.Fatal(err)
	}
	if path != uriComputersInventory+"/42?section=HARDWARE&section=GENERAL" {
		t.Fatalf("path = %q", path)
	}
	if _, err := computerInventoryPath("0", nil); err == nil {
		t.Fatal("expected invalid id error")
	}
}

func TestComputerListQueryValues(t *testing.T) {
	v, err := computerListQueryValues(ComputerListQuery{
		Filter:   `general.reportDate=ge=2026-01-01T00:00:00Z`,
		Sort:     "general.reportDate:desc",
		Sections: []string{"hardware", "OPERATING_SYSTEM"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := v.Get("filter"); got != `general.reportDate=ge=2026-01-01T00:00:00Z` {
		t.Fatalf("filter = %q", got)
	}
	if got := v.Get("sort"); got != "general.reportDate:desc" {
		t.Fatalf("sort = %q", got)
	}
	gotSec := v["section"]
	wantSec := []string{"HARDWARE", "OPERATING_SYSTEM"}
	if len(gotSec) != len(wantSec) {
		t.Fatalf("section = %#v want %#v", gotSec, wantSec)
	}
	for i := range wantSec {
		if gotSec[i] != wantSec[i] {
			t.Fatalf("section = %#v want %#v", gotSec, wantSec)
		}
	}

	empty, err := computerListQueryValues(ComputerListQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Fatalf("empty query should omit params, got %#v", empty)
	}

	wrapped, err := computerListQueryValues(ComputerListQuery{Filter: "Example"})
	if err != nil {
		t.Fatal(err)
	}
	if got := wrapped.Get("filter"); got != `general.name=="*Example*"` {
		t.Fatalf("bare filter wrap = %q", got)
	}
}

func TestComputerRecordFromListJSON(t *testing.T) {
	const raw = `[
  {
    "id": "42",
    "udid": "00000000-0000-4000-8000-000000000001",
    "general": {
      "name": "Example Mac",
      "platform": "Mac",
      "macAddress": "AA:BB:CC:DD:EE:FF",
      "username": "ada",
      "managementId": "00000000-0000-4000-8000-000000000003",
      "lastContact": "2026-01-15T10:30:00.000Z",
      "lastCheckIn": "2026-01-15T09:00:00.000Z",
      "reportDate": "2026-01-14T08:00:00.000Z",
      "lastEnrolledDate": "2025-06-01T12:00:00.000Z"
    },
    "hardware": {
      "serialNumber": "C02EXAMPLE0001"
    }
  }
]`
	var page []computerListItemV4
	if err := json.Unmarshal([]byte(raw), &page); err != nil {
		t.Fatal(err)
	}
	if len(page) != 1 {
		t.Fatalf("len=%d", len(page))
	}
	rec, err := computerRecordFromListItem(page[0])
	if err != nil {
		t.Fatal(err)
	}
	if rec.ID != 42 {
		t.Fatalf("id = %d", rec.ID)
	}
	if rec.Name != "Example Mac" {
		t.Fatalf("name = %q", rec.Name)
	}
	if rec.SerialNumber != "C02EXAMPLE0001" {
		t.Fatalf("serial = %q", rec.SerialNumber)
	}
	if rec.UDID != "00000000-0000-4000-8000-000000000001" {
		t.Fatalf("udid = %q", rec.UDID)
	}
	if rec.ManagementID != "00000000-0000-4000-8000-000000000003" {
		t.Fatalf("managementId = %q", rec.ManagementID)
	}
	if rec.MacAddress != "AA:BB:CC:DD:EE:FF" {
		t.Fatalf("mac = %q", rec.MacAddress)
	}
	if rec.Username != "ada" {
		t.Fatalf("username = %q", rec.Username)
	}
	if rec.LastContact != "2026-01-15T10:30:00.000Z" {
		t.Fatalf("lastContact = %q", rec.LastContact)
	}
	if rec.LastCheckIn != "2026-01-15T09:00:00.000Z" {
		t.Fatalf("lastCheckIn = %q", rec.LastCheckIn)
	}
	if rec.ReportDate != "2026-01-14T08:00:00.000Z" {
		t.Fatalf("reportDate = %q", rec.ReportDate)
	}
	if rec.LastEnrolledDate != "2025-06-01T12:00:00.000Z" {
		t.Fatalf("lastEnrolledDate = %q", rec.LastEnrolledDate)
	}
	if rec.Platform != "Mac" {
		t.Fatalf("platform = %q", rec.Platform)
	}
}

func TestComputerRecordFromListJSONMissingGeneral(t *testing.T) {
	const raw = `[{"id": 7, "udid": "u", "serialNumber": "S1"}]`
	var page []computerListItemV4
	if err := json.Unmarshal([]byte(raw), &page); err != nil {
		t.Fatal(err)
	}
	rec, err := computerRecordFromListItem(page[0])
	if err != nil {
		t.Fatal(err)
	}
	if rec.ID != 7 || rec.SerialNumber != "S1" || rec.UDID != "u" {
		t.Fatalf("got %+v", rec)
	}
	if rec.LastContact != "" || rec.LastCheckIn != "" || rec.ReportDate != "" || rec.Platform != "" {
		t.Fatalf("expected empty GENERAL fields, got %+v", rec)
	}
}
