package script

import (
	"fmt"
	"os"

	"go.starlark.net/starlark"
)

// unwrap(result, pretty?=True) → value
//
// Optional CLI/jq helper: JSON-print a value to stdout and return it.
// Not part of the error model — builtins raise on failure.
//
// Legacy: if result is still an old {"ok","value"/"error"} envelope, peel it;
// ok=false raises (no soft None).
func unwrapBuiltin(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var result starlark.Value
	pretty := starlark.Bool(true)
	if err := starlark.UnpackArgs("unwrap", args, kwargs,
		"result", &result,
		"pretty?", &pretty,
	); err != nil {
		return nil, err
	}

	out := result
	ok, inner, errMsg, isEnv := resultEnvelope(result)
	if isEnv {
		if !ok {
			if errMsg == "" {
				errMsg = "result ok=false"
			}
			return nil, fmt.Errorf("%s", errMsg)
		}
		out = inner
	}

	var s string
	var err error
	if bool(pretty) {
		s, err = StarlarkToJSONPretty(out)
	} else {
		s, err = StarlarkToJSON(out)
	}
	if err != nil {
		return nil, err
	}
	fmt.Fprintln(writerFromLocal(thread, localStdout, os.Stdout), s)
	return out, nil
}

// attempt(fn) → {"ok": bool, "value": …, "error": str}
//
// Soft wrapper for rare cases that must observe failure without aborting
// (e.g. expected-reject integration checks). Prefer bare calls; builtins raise.
func attemptBuiltin(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var fn starlark.Callable
	if err := starlark.UnpackArgs("attempt", args, kwargs, "fn", &fn); err != nil {
		return nil, err
	}
	val, err := starlark.Call(thread, fn, nil, nil)
	d := new(starlark.Dict)
	if err != nil {
		_ = d.SetKey(starlark.String("ok"), starlark.False)
		_ = d.SetKey(starlark.String("value"), starlark.None)
		_ = d.SetKey(starlark.String("error"), starlark.String(err.Error()))
		return d, nil
	}
	if val == nil {
		val = starlark.None
	}
	_ = d.SetKey(starlark.String("ok"), starlark.True)
	_ = d.SetKey(starlark.String("value"), val)
	_ = d.SetKey(starlark.String("error"), starlark.String(""))
	return d, nil
}
