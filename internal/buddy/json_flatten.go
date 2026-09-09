package buddy

import (
	"fmt"
	"sort"
	"strings"
)

func tryParseJSONValue(v interface{}) interface{} {
	s, ok := v.(string)
	if !ok {
		return v
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return v
	}
	if s[0] != '{' && s[0] != '[' {
		return v
	}
	parsed, err := decodeJSONInterface([]byte(s))
	if err != nil {
		return v
	}
	return parsed
}

func parseJSONStringFields(rows []map[string]interface{}, keys []string) {
	for _, row := range rows {
		for _, k := range keys {
			if v, ok := row[k]; ok {
				row[k] = tryParseJSONValue(v)
			}
		}
	}
}

func copySearchRows(rows []map[string]interface{}) []map[string]interface{} {
	out := make([]map[string]interface{}, len(rows))
	for i, r := range rows {
		out[i] = copyStringAnyMap(r)
	}
	return out
}

func jsonFlattenJoin(prefix, sep, key string) string {
	if prefix == "" {
		return key
	}
	if key == "" {
		return prefix
	}
	return prefix + sep + key
}

func flattenJSONInto(out map[string]interface{}, prefix, sep string, v interface{}) {
	m, ok := jsonAsMap(v)
	if !ok {
		if prefix != "" {
			out[prefix] = v
		}
		return
	}
	if len(m) == 0 {
		if prefix != "" {
			out[prefix] = map[string]interface{}{}
		}
		return
	}
	for k, child := range m {
		path := jsonFlattenJoin(prefix, sep, k)
		if _, isObj := jsonAsMap(child); isObj {
			flattenJSONInto(out, path, sep, child)
			continue
		}
		out[path] = child
	}
}

// JSONFlatten hoists nested object keys into a flat dict with dotted names.
// Arrays and scalars stay one value (arrays remain lists). sep defaults to ".".
func JSONFlatten(value interface{}, prefix, sep string) (map[string]interface{}, error) {
	if _, ok := jsonAsMap(value); !ok {
		return nil, fmt.Errorf("json_flatten: value must be a dict")
	}
	if sep == "" {
		sep = "."
	}
	out := map[string]interface{}{}
	flattenJSONInto(out, prefix, sep, value)
	return out, nil
}

func flattenSearchResultFields(rows []map[string]interface{}, parents, keepOrder []string) ([]map[string]interface{}, []string) {
	flatSeen := map[string]bool{}
	var flatKeys []string
	out := make([]map[string]interface{}, len(rows))
	for i, row := range rows {
		m := copyStringAnyMap(row)
		for _, k := range parents {
			v, ok := m[k]
			if !ok {
				continue
			}
			parsed := tryParseJSONValue(v)
			obj, isObj := jsonAsMap(parsed)
			if !isObj {
				m[k] = parsed
				continue
			}
			delete(m, k)
			flat := map[string]interface{}{}
			flattenJSONInto(flat, k, ".", obj)
			for fk, fv := range flat {
				m[fk] = fv
				if !flatSeen[fk] {
					flatSeen[fk] = true
					flatKeys = append(flatKeys, fk)
				}
			}
		}
		out[i] = m
	}
	sort.Slice(flatKeys, func(i, j int) bool {
		return strings.ToLower(flatKeys[i]) < strings.ToLower(flatKeys[j])
	})
	present := map[string]bool{}
	for _, m := range out {
		for k := range m {
			present[k] = true
		}
	}
	kept := make([]string, 0, len(keepOrder))
	keptSet := map[string]bool{}
	for _, k := range keepOrder {
		if !present[k] || keptSet[k] {
			continue
		}
		kept = append(kept, k)
		keptSet[k] = true
	}
	for _, fk := range flatKeys {
		if keptSet[fk] {
			continue
		}
		kept = append(kept, fk)
		keptSet[fk] = true
	}
	return out, kept
}
