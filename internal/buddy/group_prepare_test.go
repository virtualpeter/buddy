package buddy

import (
	"testing"
)

func TestPrepareSmartGroupForCreate(t *testing.T) {
	src := map[string]any{
		"id":          "12",
		"name":        "Chrome",
		"description": "desc",
		"siteId":      "1",
		"criteria": []any{
			map[string]any{"name": "Application Title", "value": "Chrome"},
		},
		"membershipCount": 9,
	}
	out, err := PrepareSmartGroupForCreate(src, SmartGroupPrepareSpec{Name: "Chrome Copy"})
	if err != nil {
		t.Fatal(err)
	}
	if out["id"] != -1 {
		t.Fatalf("id: %v", out["id"])
	}
	if out["name"] != "Chrome Copy" {
		t.Fatalf("name: %v", out["name"])
	}
	if _, ok := out["siteId"]; ok {
		t.Fatal("siteId should be cleared by default")
	}
	if _, ok := out["membershipCount"]; ok {
		t.Fatal("membershipCount should be dropped")
	}
	if out["description"] != "desc" {
		t.Fatal("description should be kept")
	}
	site := "5"
	out2, err := PrepareSmartGroupForCreate(src, SmartGroupPrepareSpec{Name: "X", SiteID: &site})
	if err != nil {
		t.Fatal(err)
	}
	if out2["siteId"] != "5" {
		t.Fatalf("siteId override: %v", out2["siteId"])
	}
}
