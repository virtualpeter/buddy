package buddy

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
)

func jsonAsMap(v interface{}) (map[string]interface{}, bool) {
	m, ok := v.(map[string]interface{})
	return m, ok
}

func copyStringAnyMap(m map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func classicString(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case json.Number:
		return t.String()
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func classicMapID(m map[string]interface{}) string {
	for _, k := range []string{"id", "ID"} {
		if s := strings.TrimSpace(classicString(m[k])); s != "" && s != "<nil>" {
			return s
		}
	}
	return ""
}

func classicMapName(m map[string]interface{}) string {
	for _, k := range []string{"name", "Name", "Computer_Name", "Display_Name", "Username"} {
		if s := strings.TrimSpace(classicString(m[k])); s != "" && s != "<nil>" {
			return s
		}
	}
	return ""
}

func classicMapSerial(m map[string]interface{}) string {
	for _, k := range []string{"serial", "Serial_Number", "serial_number", "udid", "UDID"} {
		if s := strings.TrimSpace(classicString(m[k])); s != "" && s != "<nil>" {
			return s
		}
	}
	return ""
}

// classicPeek returns m[key] for the first matching key, otherwise v unchanged.
func classicPeek(v interface{}, keys ...string) interface{} {
	m, ok := jsonAsMap(v)
	if !ok {
		return v
	}
	for _, k := range keys {
		if inner, ok := m[k]; ok {
			return inner
		}
	}
	return v
}

// classicItemMaps flattens Classic JSON collections:
// arrays, {size, <item>: [...]}, or a single object.
func classicItemMaps(v interface{}, itemKeys ...string) []map[string]interface{} {
	v = classicPeek(v, itemKeys...)
	switch t := v.(type) {
	case []interface{}:
		out := make([]map[string]interface{}, 0, len(t))
		for _, el := range t {
			el = classicPeek(el, itemKeys...)
			if m, ok := jsonAsMap(el); ok {
				out = append(out, m)
			}
		}
		return out
	case map[string]interface{}:
		for _, k := range itemKeys {
			if inner, ok := t[k]; ok {
				return classicItemMaps(inner, itemKeys...)
			}
		}
		if _, hasSize := t["size"]; hasSize {
			for k, inner := range t {
				if k == "size" {
					continue
				}
				return classicItemMaps(inner, itemKeys...)
			}
			return nil
		}
		return []map[string]interface{}{t}
	default:
		return nil
	}
}

func decodeJSONInterface(body []byte) (interface{}, error) {
	var v interface{}
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, err
	}
	return v, nil
}

func prepareClassicSearchBody(wrapperKey string, body map[string]interface{}, forCreate bool) (map[string]interface{}, error) {
	if body == nil {
		return nil, fmt.Errorf("body is required")
	}
	inner := body
	if wrapped, ok := jsonAsMap(body[wrapperKey]); ok {
		inner = wrapped
	}
	out := copyStringAnyMap(inner)
	delete(out, "computers")
	delete(out, "mobile_devices")
	if forCreate {
		delete(out, "id")
		delete(out, "ID")
	}
	if strings.TrimSpace(classicString(out["name"])) == "" && strings.TrimSpace(classicString(out["Name"])) == "" {
		return nil, fmt.Errorf("name is required")
	}
	return map[string]interface{}{wrapperKey: out}, nil
}

func classicCreatedID(body []byte, wrapperKeys ...string) string {
	v, err := decodeJSONInterface(body)
	if err != nil {
		return ""
	}
	v = classicPeek(v, wrapperKeys...)
	if m, ok := jsonAsMap(v); ok {
		return classicMapID(m)
	}
	items := classicItemMaps(v, wrapperKeys...)
	if len(items) > 0 {
		return classicMapID(items[0])
	}
	return ""
}

func searchResultMatches(row map[string]interface{}, filterLower string) bool {
	if filterLower == "" {
		return true
	}
	for _, v := range row {
		if strings.Contains(strings.ToLower(searchResultCell(v)), filterLower) {
			return true
		}
	}
	return false
}

const searchResultCellMax = 48

var searchResultKeyPrefer = []string{
	"id", "ID",
	"Computer_Name", "name", "Name", "Display_Name",
	"serial", "Serial_Number", "serial_number",
	"udid", "UDID",
}

func searchResultColumnKeys(rows []map[string]interface{}) []string {
	seen := map[string]struct{}{}
	for _, r := range rows {
		for k := range r {
			if strings.TrimSpace(k) == "" {
				continue
			}
			seen[k] = struct{}{}
		}
	}
	used := map[string]bool{}
	out := make([]string, 0, len(seen))
	for _, k := range searchResultKeyPrefer {
		if _, ok := seen[k]; ok && !used[k] {
			out = append(out, k)
			used[k] = true
		}
	}
	rest := make([]string, 0, len(seen))
	for k := range seen {
		if !used[k] {
			rest = append(rest, k)
		}
	}
	sort.Slice(rest, func(i, j int) bool {
		return strings.ToLower(rest[i]) < strings.ToLower(rest[j])
	})
	return append(out, rest...)
}

func searchResultCell(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case map[string]interface{}, []interface{}:
		b, err := json.Marshal(t)
		if err != nil {
			return strings.TrimSpace(fmt.Sprint(t))
		}
		return string(b)
	default:
		return classicString(v)
	}
}

func searchResultHeader(key string) string {
	return strings.ReplaceAll(key, "_", " ")
}

func printSearchResultTable(w io.Writer, rows []map[string]interface{}, keys []string, noun string) {
	if len(keys) == 0 {
		keys = searchResultColumnKeys(rows)
	}
	if len(keys) > 0 {
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		headers := make([]string, len(keys))
		for i, k := range keys {
			headers[i] = searchResultHeader(k)
		}
		fmt.Fprintln(tw, strings.Join(headers, "\t"))
		for _, row := range rows {
			cells := make([]string, len(keys))
			for i, k := range keys {
				cells[i] = truncateRunes(searchResultCell(row[k]), searchResultCellMax)
			}
			fmt.Fprintln(tw, strings.Join(cells, "\t"))
		}
		_ = tw.Flush()
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "%d %s\n", len(rows), noun)
}

func normalizeSearchFieldName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return s
}

// ParseSearchFields splits a comma-separated --fields= list. Empty means all fields.
func ParseSearchFields(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	raw := strings.Split(s, ",")
	out := make([]string, 0, len(raw))
	seen := map[string]bool{}
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n := normalizeSearchFieldName(p)
		if seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, p)
	}
	return out
}

// ResolveSearchResultFields maps requested field names onto JSON keys present in rows.
// Names match case-insensitively and treat spaces/hyphens as underscores
// ("Computer Name" == "Computer_Name"). Empty want returns every key in default order.
func ResolveSearchResultFields(rows []map[string]interface{}, want []string) ([]string, error) {
	available := searchResultColumnKeys(rows)
	if len(want) == 0 {
		return available, nil
	}
	if len(available) == 0 {
		return want, nil
	}
	byNorm := map[string]string{}
	for _, k := range available {
		n := normalizeSearchFieldName(k)
		if _, ok := byNorm[n]; !ok {
			byNorm[n] = k
		}
	}
	out := make([]string, 0, len(want))
	seen := map[string]bool{}
	var missing []string
	for _, w := range want {
		k, ok := byNorm[normalizeSearchFieldName(w)]
		if !ok {
			missing = append(missing, w)
			continue
		}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("unknown field %s (have: %s)", strings.Join(missing, ", "), strings.Join(available, ", "))
	}
	return out, nil
}

// ProjectSearchResultRows keeps only keys, in that order, on each row.
func ProjectSearchResultRows(rows []map[string]interface{}, keys []string) []map[string]interface{} {
	out := make([]map[string]interface{}, len(rows))
	for i, r := range rows {
		m := make(map[string]interface{}, len(keys))
		for _, k := range keys {
			if v, ok := r[k]; ok {
				m[k] = v
			}
		}
		out[i] = m
	}
	return out
}

// SearchResultPrintOpts is CLI presentation for computer/mobile search results.
type SearchResultPrintOpts struct {
	Fields   string
	JQ       string
	FromJSON string
	Flatten  string
	JSON     bool
	CSV      bool
	Raw      bool
}

func printSearchResultCSV(w io.Writer, rows []map[string]interface{}, keys []string) error {
	if len(keys) == 0 {
		keys = searchResultColumnKeys(rows)
	}
	cw := csv.NewWriter(w)
	if err := cw.Write(keys); err != nil {
		return err
	}
	for _, row := range rows {
		rec := make([]string, len(keys))
		for i, k := range keys {
			rec[i] = searchResultCell(row[k])
		}
		if err := cw.Write(rec); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func searchResultFormatError(opts SearchResultPrintOpts) error {
	n := 0
	if opts.JSON {
		n++
	}
	if opts.CSV {
		n++
	}
	if opts.Raw {
		n++
	}
	if n > 1 {
		return fmt.Errorf("--json, --csv, and --raw are mutually exclusive")
	}
	if opts.Raw && strings.TrimSpace(opts.JQ) == "" {
		return fmt.Errorf("--raw requires --jq")
	}
	return nil
}

func formatSearchResults(rows []map[string]interface{}, opts SearchResultPrintOpts, noun string) error {
	return writeSearchResults(os.Stdout, rows, opts, noun)
}

func writeSearchResults(w io.Writer, rows []map[string]interface{}, opts SearchResultPrintOpts, noun string) error {
	if err := searchResultFormatError(opts); err != nil {
		return err
	}
	jqExpr := strings.TrimSpace(opts.JQ)
	fromJSONWant := ParseSearchFields(opts.FromJSON)
	flattenWant := ParseSearchFields(opts.Flatten)

	want := ParseSearchFields(opts.Fields)
	keys, err := ResolveSearchResultFields(rows, want)
	if err != nil {
		return err
	}
	if len(want) > 0 {
		rows = ProjectSearchResultRows(rows, keys)
	}
	if len(fromJSONWant) > 0 || len(flattenWant) > 0 {
		rows = copySearchRows(rows)
	}
	if len(fromJSONWant) > 0 {
		fjKeys, err := ResolveSearchResultFields(rows, fromJSONWant)
		if err != nil {
			return err
		}
		parseJSONStringFields(rows, fjKeys)
	}

	if jqExpr != "" {
		stream, err := ApplyJQStream(searchRowsToAny(rows), jqExpr)
		if err != nil {
			return err
		}
		if opts.Raw {
			return printJQRaw(w, stream)
		}
		collapsed := collapseJQStream(stream)
		shaped, ok := jqValueAsRows(collapsed)
		if !ok {
			if opts.JSON && len(flattenWant) == 0 {
				return printJSONValueTo(w, collapsed)
			}
			return fmt.Errorf("jq result is not a table; use --json or --raw")
		}
		if opts.JSON && len(flattenWant) == 0 {
			return printJSONValueTo(w, collapsed)
		}
		rows = shaped
		keys = searchResultColumnKeys(rows)
	}

	if len(flattenWant) > 0 {
		flKeys, err := ResolveSearchResultFields(rows, flattenWant)
		if err != nil {
			return err
		}
		rows, keys = flattenSearchResultFields(rows, flKeys, keys)
	}

	if opts.JSON {
		return printJSONValueTo(w, rows)
	}
	if opts.CSV {
		return printSearchResultCSV(w, rows, keys)
	}
	printSearchResultTable(w, rows, keys, noun)
	return nil
}
