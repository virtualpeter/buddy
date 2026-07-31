package script

import (
	"fmt"
	"strconv"

	"go.starlark.net/starlark"
)

// starlarkIDString accepts int or str Starlark value as an ID string.
func starlarkIDString(v starlark.Value, label string) (string, error) {
	switch x := v.(type) {
	case starlark.String:
		s := string(x)
		if s == "" {
			return "", fmt.Errorf("%s must be non-empty", label)
		}
		return s, nil
	case starlark.Int:
		n, ok := x.Int64()
		if !ok {
			return "", fmt.Errorf("%s out of range", label)
		}
		return strconv.FormatInt(n, 10), nil
	default:
		return "", fmt.Errorf("%s must be int or str, got %s", label, v.Type())
	}
}

func starlarkIDInt(v starlark.Value, label string) (int, error) {
	switch x := v.(type) {
	case starlark.Int:
		n, err := starlark.AsInt32(x)
		if err != nil {
			return 0, fmt.Errorf("%s: %w", label, err)
		}
		return n, nil
	case starlark.String:
		n, err := strconv.Atoi(string(x))
		if err != nil {
			return 0, fmt.Errorf("%s: %w", label, err)
		}
		return n, nil
	default:
		return 0, fmt.Errorf("%s must be int or str, got %s", label, v.Type())
	}
}

func boolFromStarlark(v starlark.Value) bool {
	b, ok := v.(starlark.Bool)
	return ok && bool(b)
}

func stringListFromStarlark(v starlark.Value, label string) ([]string, error) {
	switch x := v.(type) {
	case starlark.String:
		return []string{string(x)}, nil
	case *starlark.List:
		out := make([]string, 0, x.Len())
		for i := 0; i < x.Len(); i++ {
			s, ok := x.Index(i).(starlark.String)
			if !ok {
				return nil, fmt.Errorf("%s must be a list of strings", label)
			}
			out = append(out, string(s))
		}
		return out, nil
	default:
		return nil, fmt.Errorf("%s must be str or list of str, got %s", label, v.Type())
	}
}

func intListFromStarlark(v *starlark.List, label string) ([]int, error) {
	if v == nil {
		return nil, nil
	}
	out := make([]int, 0, v.Len())
	for i := 0; i < v.Len(); i++ {
		n, ok := v.Index(i).(starlark.Int)
		if !ok {
			return nil, fmt.Errorf("%s must be a list of ints", label)
		}
		n64, _ := n.Int64()
		out = append(out, int(n64))
	}
	return out, nil
}

// xorIDOrKey ensures exactly one of idSet / keySet is true.
func xorIDOrKey(fn string, idSet, keySet bool, keyName string) error {
	switch {
	case idSet && keySet:
		return fmt.Errorf("%s: pass either id= or %s=, not both", fn, keyName)
	case !idSet && !keySet:
		return fmt.Errorf("%s: require id= or %s=", fn, keyName)
	}
	return nil
}
