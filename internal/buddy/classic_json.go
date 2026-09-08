package buddy

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
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
	for _, k := range []string{"id", "name", "Computer_Name", "Display_Name", "serial", "Serial_Number", "udid", "UDID", "Username"} {
		if strings.Contains(strings.ToLower(classicString(row[k])), filterLower) {
			return true
		}
	}
	return false
}
