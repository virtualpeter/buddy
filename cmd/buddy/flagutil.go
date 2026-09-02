package main

import (
	"fmt"
	"strings"
)

// takeFlagValue reads --name=value or --name value at args[i].
// matched is false when args[i] is not this flag.
func takeFlagValue(args []string, i int, name string) (val string, next int, matched bool, err error) {
	if i < 0 || i >= len(args) {
		return "", i, false, nil
	}
	a := args[i]
	flag := "--" + name
	switch {
	case a == flag:
		if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
			return "", i, true, fmt.Errorf("%s requires a value", flag)
		}
		return args[i+1], i + 2, true, nil
	case strings.HasPrefix(a, flag+"="):
		return strings.TrimPrefix(a, flag+"="), i + 1, true, nil
	default:
		return "", i, false, nil
	}
}

// extractStringFlag removes all --name=value / --name value occurrences from args.
// The last value wins. Remaining args are returned as rest.
func extractStringFlag(args []string, name string) (value string, rest []string, err error) {
	rest = make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		val, next, matched, err := takeFlagValue(args, i, name)
		if err != nil {
			return "", nil, err
		}
		if matched {
			value = val
			i = next - 1
			continue
		}
		rest = append(rest, args[i])
	}
	return value, rest, nil
}

// extractBareFlag removes all --name occurrences from args (boolean flags with no value).
func extractBareFlag(args []string, name string) (present bool, rest []string) {
	want := "--" + name
	rest = make([]string, 0, len(args))
	for _, a := range args {
		if a == want {
			present = true
			continue
		}
		rest = append(rest, a)
	}
	return present, rest
}
