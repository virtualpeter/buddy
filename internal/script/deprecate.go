package script

import (
	"fmt"
	"os"
	"sync"

	"go.starlark.net/starlark"
)

var deprecOnce sync.Map // oldName -> struct{}

// deprecate wraps a builtin so the first call per process warns on stderr.
func deprecate(oldName, newName string, impl func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error)) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		if _, loaded := deprecOnce.LoadOrStore(oldName, struct{}{}); !loaded {
			fmt.Fprintf(os.Stderr, "buddy: deprecated builtin %s → use %s (alias will be removed in a future release)\n", oldName, newName)
		}
		return impl(thread, b, args, kwargs)
	}
}
