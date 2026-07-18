// Package stdlib holds embedded Starlark library modules loaded via
// load("stdlib/<name>.star", ...).
package stdlib

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"
)

//go:embed *.star
var content embed.FS

var moduleNameRE = regexp.MustCompile(`^[a-z][a-z0-9_]*\.star$`)

// ModulePath returns the load path for a bare file name (e.g. "release.star" → "stdlib/release.star").
func ModulePath(file string) string {
	return path.Join("stdlib", file)
}

// ValidModule reports whether module is an allowed stdlib load path.
func ValidModule(module string) bool {
	if !strings.HasPrefix(module, "stdlib/") {
		return false
	}
	base := strings.TrimPrefix(module, "stdlib/")
	if strings.Contains(base, "/") || strings.Contains(base, "..") {
		return false
	}
	return moduleNameRE.MatchString(base)
}

// Read returns the source of a stdlib module. module must be a ValidModule path.
func Read(module string) ([]byte, error) {
	if !ValidModule(module) {
		return nil, fmt.Errorf("invalid stdlib module path: %s", module)
	}
	base := strings.TrimPrefix(module, "stdlib/")
	data, err := content.ReadFile(base)
	if err != nil {
		return nil, fmt.Errorf("stdlib module %s: %w", module, err)
	}
	return data, nil
}

// List returns sorted load paths for all embedded stdlib modules.
func List() ([]string, error) {
	entries, err := fs.ReadDir(content, ".")
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !moduleNameRE.MatchString(name) {
			continue
		}
		out = append(out, ModulePath(name))
	}
	sort.Strings(out)
	return out, nil
}
