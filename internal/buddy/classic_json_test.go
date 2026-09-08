package buddy

import "testing"

func TestClassicItemMaps_ArrayAndSizeWrapper(t *testing.T) {
	raw := []byte(`{"advanced_computer_searches":[{"id":1,"name":"A"},{"id":2,"name":"B"}]}`)
	v, err := decodeJSONInterface(raw)
	if err != nil {
		t.Fatal(err)
	}
	items := classicItemMaps(classicPeek(v, "advanced_computer_searches"), "advanced_computer_search")
	if len(items) != 2 || classicMapID(items[0]) != "1" || classicMapName(items[1]) != "B" {
		t.Fatalf("array: %+v", items)
	}

	raw = []byte(`{"computers":{"size":1,"computer":[{"id":10,"name":"Mac","Computer_Name":"Mac"}]}}`)
	v, err = decodeJSONInterface(raw)
	if err != nil {
		t.Fatal(err)
	}
	items = classicItemMaps(classicPeek(v, "computers"), "computer")
	if len(items) != 1 || classicMapID(items[0]) != "10" || classicMapName(items[0]) != "Mac" {
		t.Fatalf("size wrapper: %+v", items)
	}
}

func TestPrepareClassicSearchBody_StripsComputersAndWraps(t *testing.T) {
	body := map[string]interface{}{
		"id":        float64(9),
		"name":      "Chrome inventory",
		"criteria":  []interface{}{},
		"computers": []interface{}{map[string]interface{}{"id": float64(1)}},
	}
	wrapped, err := prepareClassicSearchBody("advanced_computer_search", body, true)
	if err != nil {
		t.Fatal(err)
	}
	inner, ok := jsonAsMap(wrapped["advanced_computer_search"])
	if !ok {
		t.Fatalf("wrap: %+v", wrapped)
	}
	if _, has := inner["computers"]; has {
		t.Fatal("computers must be stripped")
	}
	if _, has := inner["id"]; has {
		t.Fatal("id must be stripped on create")
	}
	if classicMapName(inner) != "Chrome inventory" {
		t.Fatalf("name=%v", inner["name"])
	}

	already := map[string]interface{}{
		"advanced_computer_search": map[string]interface{}{"name": "X", "computers": []interface{}{}},
	}
	wrapped, err = prepareClassicSearchBody("advanced_computer_search", already, false)
	if err != nil {
		t.Fatal(err)
	}
	inner, _ = jsonAsMap(wrapped["advanced_computer_search"])
	if _, has := inner["computers"]; has {
		t.Fatal("nested computers must be stripped")
	}
}

func TestClassicCreatedID(t *testing.T) {
	if got := classicCreatedID([]byte(`{"id": 5}`), "advanced_computer_search"); got != "5" {
		t.Fatalf("bare id: %q", got)
	}
	if got := classicCreatedID([]byte(`{"advanced_computer_search":{"id":7,"name":"N"}}`), "advanced_computer_search"); got != "7" {
		t.Fatalf("wrapped: %q", got)
	}
}

func TestSearchResultMatches(t *testing.T) {
	row := map[string]interface{}{"id": float64(10), "Computer_Name": "Lab Mac", "serial": "C02ABC"}
	if !searchResultMatches(row, "") || !searchResultMatches(row, "lab") || !searchResultMatches(row, "c02") {
		t.Fatal("expected match")
	}
	if searchResultMatches(row, "chrome") {
		t.Fatal("should not match")
	}
}
