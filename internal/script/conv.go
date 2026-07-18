package script

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.starlark.net/starlark"
	"howett.net/plist"
)

// GoValueToStarlark converts a Go value (from json.Unmarshal or plist decode) to a Starlark value.
// Supports: map[string]interface{} -> dict, []interface{} -> list, string, float64, bool, nil.
// Integers in JSON are decoded as float64; they are converted to starlark.Int.
func GoValueToStarlark(v interface{}) (starlark.Value, error) {
	if v == nil {
		return starlark.None, nil
	}
	switch x := v.(type) {
	case map[string]interface{}:
		d := new(starlark.Dict)
		for k, val := range x {
			sv, err := GoValueToStarlark(val)
			if err != nil {
				return nil, err
			}
			if err := d.SetKey(starlark.String(k), sv); err != nil {
				return nil, err
			}
		}
		return d, nil
	case []interface{}:
		list := new(starlark.List)
		for _, item := range x {
			sv, err := GoValueToStarlark(item)
			if err != nil {
				return nil, err
			}
			if err := list.Append(sv); err != nil {
				return nil, err
			}
		}
		return list, nil
	case string:
		return starlark.String(x), nil
	case float64:
		// JSON numbers are float64; convert to int if whole
		if x == float64(int64(x)) {
			return starlark.MakeInt64(int64(x)), nil
		}
		return starlark.Float(x), nil
	case int:
		return starlark.MakeInt(x), nil
	case int32:
		return starlark.MakeInt64(int64(x)), nil
	case int64:
		return starlark.MakeInt64(x), nil
	case uint:
		return starlark.MakeUint64(uint64(x)), nil
	case uint32:
		return starlark.MakeUint64(uint64(x)), nil
	case uint64:
		return starlark.MakeUint64(x), nil
	case bool:
		return starlark.Bool(x), nil
	case []byte:
		return starlark.Bytes(x), nil
	case time.Time:
		return starlark.String(x.Format(time.RFC3339)), nil
	default:
		return nil, fmt.Errorf("unsupported type for Starlark: %T", v)
	}
}

// StarlarkValueToGo converts a Starlark value to a Go value suitable for json.Marshal or plist encode.
func StarlarkValueToGo(v starlark.Value) (interface{}, error) {
	switch x := v.(type) {
	case *starlark.Dict:
		m := make(map[string]interface{})
		for _, item := range x.Items() {
			k, kval := item[0], item[1]
			keyStr, ok := starlark.AsString(k)
			if !ok {
				return nil, fmt.Errorf("dict key must be string, got %s", k.Type())
			}
			gval, err := StarlarkValueToGo(kval)
			if err != nil {
				return nil, err
			}
			m[keyStr] = gval
		}
		return m, nil
	case *starlark.List:
		sl := make([]interface{}, 0, x.Len())
		iter := x.Iterate()
		defer iter.Done()
		var item starlark.Value
		for iter.Next(&item) {
			gval, err := StarlarkValueToGo(item)
			if err != nil {
				return nil, err
			}
			sl = append(sl, gval)
		}
		return sl, nil
	case starlark.String:
		return string(x), nil
	case starlark.Int:
		// Prefer int64 when it fits — matches JSON decoding and avoids surprising
		// map values for typical positive integers decoded as plist uint64.
		if i, ok := x.Int64(); ok {
			return i, nil
		}
		if u, ok := x.Uint64(); ok {
			return u, nil
		}
		return nil, fmt.Errorf("int out of range")
	case starlark.Float:
		return float64(x), nil
	case starlark.Bool:
		return bool(x), nil
	case starlark.Bytes:
		return []byte(x), nil
	case starlark.NoneType:
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported Starlark type for Go: %s", x.Type())
	}
}

// JSONDecodeToStarlark parses a JSON string and returns the equivalent Starlark value.
func JSONDecodeToStarlark(jsonStr string) (starlark.Value, error) {
	var v interface{}
	if err := json.Unmarshal([]byte(jsonStr), &v); err != nil {
		return nil, err
	}
	return GoValueToStarlark(v)
}

// StarlarkToJSON encodes a Starlark value to a JSON string.
func StarlarkToJSON(v starlark.Value) (string, error) {
	g, err := StarlarkValueToGo(v)
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(g)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// StarlarkToJSONPretty encodes a Starlark value to indented JSON (2-space).
func StarlarkToJSONPretty(v starlark.Value) (string, error) {
	g, err := StarlarkValueToGo(v)
	if err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// PlistDecodeToStarlark parses a plist XML string and returns the equivalent Starlark value.
func PlistDecodeToStarlark(plistStr string) (starlark.Value, error) {
	var v interface{}
	dec := plist.NewDecoder(strings.NewReader(plistStr))
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return GoValueToStarlark(v)
}

// StarlarkToPlist encodes a Starlark value to a plist XML string.
func StarlarkToPlist(v starlark.Value) (string, error) {
	g, err := StarlarkValueToGo(v)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := plist.NewEncoder(&buf).Encode(g); err != nil {
		return "", err
	}
	return buf.String(), nil
}
