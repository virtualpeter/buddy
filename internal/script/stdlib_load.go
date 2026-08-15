package script

import (
	"fmt"
	"strings"
	"sync"

	"buddy/stdlib"

	"go.starlark.net/starlark"
)

// stdlibLoader returns a Load callback that serves "re" and embedded stdlib modules.
// cache is per-thread/session so builtins closed over credName stay correct.
func stdlibLoader(reModule, predeclared starlark.StringDict) func(*starlark.Thread, string) (starlark.StringDict, error) {
	var mu sync.Mutex
	cache := make(map[string]starlark.StringDict)

	return func(thread *starlark.Thread, module string) (starlark.StringDict, error) {
		if module == "re" {
			return reModule, nil
		}
		if !stdlib.HasModule(module) {
			if strings.HasPrefix(module, "stdlib/") {
				if !stdlib.ValidModule(module) {
					return nil, fmt.Errorf("invalid stdlib module %q (want stdlib/<name>.buddy)", module)
				}
				return nil, fmt.Errorf("unknown module: %s (no embedded stdlib modules)", module)
			}
			return nil, fmt.Errorf("unknown module: %s (try load(\"re\", ...))", module)
		}

		mu.Lock()
		if cached, ok := cache[module]; ok {
			mu.Unlock()
			return cached, nil
		}
		mu.Unlock()

		data, err := stdlib.Read(module)
		if err != nil {
			return nil, err
		}

		// Fresh predeclared copy so module assignments do not mutate the caller's env.
		modPre := make(starlark.StringDict, len(predeclared))
		for k, v := range predeclared {
			modPre[k] = v
		}

		globals, err := starlark.ExecFile(thread, module, data, modPre)
		if err != nil {
			return nil, fmt.Errorf("exec %s: %w", module, err)
		}

		exports := filterStdlibExports(globals, predeclared)
		mu.Lock()
		cache[module] = exports
		mu.Unlock()
		return exports, nil
	}
}

func filterStdlibExports(globals, predeclared starlark.StringDict) starlark.StringDict {
	out := make(starlark.StringDict)
	for k, v := range globals {
		if strings.HasPrefix(k, "_") {
			continue
		}
		if _, isPre := predeclared[k]; isPre {
			continue
		}
		out[k] = v
	}
	return out
}
