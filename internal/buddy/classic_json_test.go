package buddy

import (
	"encoding/csv"
	"strings"
	"testing"
)

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
	row := map[string]interface{}{"id": float64(10), "Computer_Name": "Lab Mac", "Full_Name": "Ada Lovelace", "serial": "C02ABC"}
	if !searchResultMatches(row, "") || !searchResultMatches(row, "lab") || !searchResultMatches(row, "c02") || !searchResultMatches(row, "lovelace") {
		t.Fatal("expected match")
	}
	if searchResultMatches(row, "chrome") {
		t.Fatal("should not match")
	}
}

func TestSearchResultColumnKeysAndTable(t *testing.T) {
	rows := []map[string]interface{}{
		{
			"id":                       float64(12198),
			"Computer_Name":            "C02ABC",
			"Full_Name":                "Ada Lovelace",
			"Operating_System_Version": "26.6.2",
			"Wi_Fi_History":            `{"v":1,"updated":"2026-09-09T14:11:24+10:00","current":{"id":"10.31.48.167"}}`,
		},
		{
			"id":            float64(12200),
			"Computer_Name": "C02DEF",
			"Username":      "ada@example.com",
		},
	}
	keys := searchResultColumnKeys(rows)
	wantHead := []string{"id", "Computer_Name"}
	if len(keys) < 2 || keys[0] != wantHead[0] || keys[1] != wantHead[1] {
		t.Fatalf("prefer order: %v", keys)
	}
	joined := strings.Join(keys, ",")
	if !strings.Contains(joined, "Full_Name") || !strings.Contains(joined, "Operating_System_Version") || !strings.Contains(joined, "Username") || !strings.Contains(joined, "Wi_Fi_History") {
		t.Fatalf("missing display fields: %v", keys)
	}
	var buf strings.Builder
	printSearchResultTable(&buf, rows, nil, "computer(s)")
	out := buf.String()
	for _, want := range []string{"id", "Computer Name", "Full Name", "Operating System Version", "Username", "Wi Fi History", "Ada Lovelace", "26.6.2", "2 computer(s)"} {
		if !strings.Contains(out, want) {
			t.Fatalf("table missing %q:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "…") {
		t.Fatalf("expected truncated Wi_Fi_History:\n%s", out)
	}
}

func TestParseAndResolveSearchFields(t *testing.T) {
	if got := ParseSearchFields(" Computer_Name, Username ,Computer Name"); len(got) != 2 || got[0] != "Computer_Name" || got[1] != "Username" {
		t.Fatalf("dedupe: %v", got)
	}
	rows := []map[string]interface{}{
		{"id": float64(1), "Computer_Name": "Mac", "Username": "ada", "Full_Name": "Ada"},
	}
	keys, err := ResolveSearchResultFields(rows, ParseSearchFields("Computer Name,username"))
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 || keys[0] != "Computer_Name" || keys[1] != "Username" {
		t.Fatalf("resolve: %v", keys)
	}
	proj := ProjectSearchResultRows(rows, keys)
	if len(proj) != 1 || proj[0]["Computer_Name"] != "Mac" {
		t.Fatalf("project: %+v", proj)
	}
	if _, ok := proj[0]["Full_Name"]; ok {
		t.Fatal("Full_Name should be dropped")
	}
	if _, err := ResolveSearchResultFields(rows, []string{"Not_A_Field"}); err == nil || !strings.Contains(err.Error(), "Not_A_Field") {
		t.Fatalf("unknown field: %v", err)
	}
	var buf strings.Builder
	printSearchResultTable(&buf, rows, keys, "computer(s)")
	out := buf.String()
	if !strings.Contains(out, "Computer Name") || !strings.Contains(out, "Username") || strings.Contains(out, "Full Name") {
		t.Fatalf("selected table:\n%s", out)
	}
}

func TestPrintSearchResultCSV(t *testing.T) {
	rows := []map[string]interface{}{
		{"Computer_Name": "Mac, 1", "Username": "ada\nlab"},
		{"Computer_Name": "Mac-2", "Username": `say "hi"`},
	}
	keys := []string{"Computer_Name", "Username"}
	var buf strings.Builder
	if err := printSearchResultCSV(&buf, rows, keys); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, "computer(s)") {
		t.Fatalf("csv must not include count footer:\n%s", out)
	}
	r := csv.NewReader(strings.NewReader(out))
	recs, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 3 || recs[0][0] != "Computer_Name" || recs[0][1] != "Username" {
		t.Fatalf("header: %v", recs)
	}
	if recs[1][0] != "Mac, 1" || recs[1][1] != "ada\nlab" {
		t.Fatalf("quoted cells: %v", recs[1])
	}
	if recs[2][1] != `say "hi"` {
		t.Fatalf("quotes: %v", recs[2])
	}
}

func TestWriteSearchResults_FormatFlags(t *testing.T) {
	rows := []map[string]interface{}{
		{"Computer_Name": "Mac-A", "Username": "ada"},
		{"Computer_Name": "Mac-B", "Username": "bob"},
	}
	var buf strings.Builder
	if err := writeSearchResults(&buf, rows, SearchResultPrintOpts{JSON: true, CSV: true}, "computer(s)"); err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("json+csv: %v", err)
	}
	if err := writeSearchResults(&buf, rows, SearchResultPrintOpts{JSON: true, Raw: true, JQ: "."}, "computer(s)"); err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("json+raw: %v", err)
	}
	if err := writeSearchResults(&buf, rows, SearchResultPrintOpts{Raw: true}, "computer(s)"); err == nil || !strings.Contains(err.Error(), "--raw requires --jq") {
		t.Fatalf("raw without jq: %v", err)
	}
	if err := writeSearchResults(&buf, rows, SearchResultPrintOpts{JQ: ".[].Computer_Name", CSV: true}, "computer(s)"); err == nil || !strings.Contains(err.Error(), "not a table") {
		t.Fatalf("scalar stream csv: %v", err)
	}

	buf.Reset()
	if err := writeSearchResults(&buf, rows, SearchResultPrintOpts{JQ: ".[].Computer_Name", Raw: true}, "computer(s)"); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if got != "Mac-A\nMac-B\n" {
		t.Fatalf("raw:\n%q", got)
	}

	buf.Reset()
	if err := writeSearchResults(&buf, rows, SearchResultPrintOpts{Fields: "Computer_Name,Username", CSV: true}, "computer(s)"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(buf.String(), "Computer_Name,Username\n") || strings.Contains(buf.String(), "computer(s)") {
		t.Fatalf("csv:\n%s", buf.String())
	}

	buf.Reset()
	if err := writeSearchResults(&buf, rows, SearchResultPrintOpts{JQ: "map({name: .Computer_Name})", CSV: true}, "computer(s)"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "name") || !strings.Contains(buf.String(), "Mac-A") {
		t.Fatalf("jq csv:\n%s", buf.String())
	}
}

func wifiHistorySample() string {
	return `{"v":1,"updated":"2026-09-09T15:27:14+10:00","current":{"place":"hobo","wifi":true,"ip":["10.31.34.150"]}}`
}

func TestTryParseJSONValue(t *testing.T) {
	obj, ok := jsonAsMap(tryParseJSONValue(wifiHistorySample()))
	if !ok {
		t.Fatal("expected object")
	}
	if classicString(obj["v"]) != "1" {
		t.Fatalf("v=%v", obj["v"])
	}
	if tryParseJSONValue("not json") != "not json" {
		t.Fatal("non-json string should stay")
	}
	if tryParseJSONValue("{nope") != "{nope" {
		t.Fatal("bad json should stay")
	}
	arr, ok := tryParseJSONValue(`[1,2]`).([]interface{})
	if !ok || len(arr) != 2 {
		t.Fatalf("array: %#v", tryParseJSONValue(`[1,2]`))
	}
}

func TestJSONFlatten(t *testing.T) {
	got, err := JSONFlatten(map[string]interface{}{"current": map[string]interface{}{"place": "hobo"}}, "h", "")
	if err != nil {
		t.Fatal(err)
	}
	if classicString(got["h.current.place"]) != "hobo" {
		t.Fatalf("flatten: %#v", got)
	}
	if _, err := JSONFlatten("nope", "", "."); err == nil {
		t.Fatal("expected dict error")
	}
}

func TestFlattenSearchResultFields_CSV(t *testing.T) {
	rows := []map[string]interface{}{
		{"id": float64(12212), "Computer_Name": "GQK9MH9N4L", "Wi_Fi_History": wifiHistorySample()},
	}
	var buf strings.Builder
	if err := writeSearchResults(&buf, rows, SearchResultPrintOpts{
		Fields:   "id,Computer_Name,Wi_Fi_History",
		FromJSON: "Wi_Fi_History",
		Flatten:  "Wi_Fi_History",
		CSV:      true,
	}, "computer(s)"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	header := strings.Split(out, "\n")[0]
	if !strings.Contains(header, "Wi_Fi_History.current.place") || !strings.Contains(header, "id") || !strings.Contains(header, "Computer_Name") {
		t.Fatalf("header: %s", header)
	}
	if strings.Contains(header, ",Wi_Fi_History,") || strings.HasSuffix(header, ",Wi_Fi_History") || strings.HasPrefix(header, "Wi_Fi_History,") {
		t.Fatalf("parent still present: %s", header)
	}
	if !strings.Contains(out, "hobo") {
		t.Fatalf("missing place:\n%s", out)
	}
	if err := writeSearchResults(&buf, rows, SearchResultPrintOpts{FromJSON: "Not_A_Field", JSON: true}, "computer(s)"); err == nil || !strings.Contains(err.Error(), "Not_A_Field") {
		t.Fatalf("unknown fromjson: %v", err)
	}

	mixed := []map[string]interface{}{
		{"id": float64(1), "Wi_Fi_History": wifiHistorySample()},
		{"id": float64(2), "Wi_Fi_History": "not-json"},
	}
	buf.Reset()
	if err := writeSearchResults(&buf, mixed, SearchResultPrintOpts{Flatten: "Wi_Fi_History", CSV: true}, "computer(s)"); err != nil {
		t.Fatal(err)
	}
	mixedOut := buf.String()
	if !strings.Contains(mixedOut, "not-json") || !strings.Contains(mixedOut, "hobo") {
		t.Fatalf("skip bad json:\n%s", mixedOut)
	}
}
