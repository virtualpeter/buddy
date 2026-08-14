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

// resultEnvelope inspects a buddy {"ok": bool, "value"/"error": ...} result dict.
// isEnvelope is false for ordinary dicts (e.g. schema payloads).
func resultEnvelope(v starlark.Value) (ok bool, value starlark.Value, errMsg string, isEnvelope bool) {
	d, okDict := v.(*starlark.Dict)
	if !okDict {
		return false, v, "", false
	}
	okVal, hasOK, _ := d.Get(starlark.String("ok"))
	if !hasOK {
		return false, v, "", false
	}
	b, isBool := okVal.(starlark.Bool)
	if !isBool {
		return false, v, "", false
	}
	_, hasValue, _ := d.Get(starlark.String("value"))
	errVal, hasErr, _ := d.Get(starlark.String("error"))
	if !hasValue && !hasErr {
		return false, v, "", false
	}
	msg := ""
	if hasErr {
		if s, ok := starlark.AsString(errVal); ok {
			msg = s
		} else if errVal != nil && errVal != starlark.None {
			msg = errVal.String()
		}
	}
	var val starlark.Value = starlark.None
	if hasValue {
		val, _, _ = d.Get(starlark.String("value"))
		if val == nil {
			val = starlark.None
		}
	}
	return bool(b), val, msg, true
}
