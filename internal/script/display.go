package script

import (
	"buddy/internal/help"

	"go.starlark.net/starlark"
)

// FormatDisplayValue renders a Starlark value for IDE/REPL result panels.
// Dicts and lists (and nested structures) are shown as indented JSON with real
// newlines — not Starlark's Python-like String()/repr, which looks JSON-ish but
// is not JSON and escapes newlines inside strings.
// Bare builtins print catalog help (signature / returns / params).
func FormatDisplayValue(v starlark.Value) string {
	if v == nil || v == starlark.None {
		return ""
	}
	if b, ok := v.(*starlark.Builtin); ok {
		if fn, ok := help.LookupBuiltin(b.Name()); ok {
			return help.FormatBuiltinHelp(fn)
		}
		return b.String()
	}
	if s, ok := starlark.AsString(v); ok {
		return s
	}
	if _, ok := v.(starlark.Bytes); ok {
		return v.String()
	}
	pretty, err := StarlarkToJSONPretty(v)
	if err != nil {
		return v.String()
	}
	return pretty
}
