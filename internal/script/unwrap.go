package script

import (
	"fmt"
	"os"

	"go.starlark.net/starlark"
)

// unwrap(result, pretty?=True) → value or None
//
// Soft helper for result envelopes:
//   - ok=True: print JSON of value to stdout, return value
//   - ok=False: print error to stderr, return None (no process exit)
//   - plain value: print JSON of it, return it
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
			fmt.Fprintln(writerFromLocal(thread, localStderr, os.Stderr), errMsg)
			return starlark.None, nil
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
