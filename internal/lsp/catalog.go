package lsp

import (
	"strings"
	"sync"

	"buddy/internal/help"
	"buddy/internal/script"
	"buddy/stdlib"

	"github.com/qri-io/starlib/re"
	"go.starlark.net/starlark"
)

// Symbol is a completable / hoverable name in the buddy dialect.
type Symbol struct {
	Name        string
	Signature   string
	Detail      string
	Insert      string
	Kind        string // "builtin", "predeclared", "stdlib", "universe"
	Deprecated  bool
	Replacement string // when Deprecated
}

type catalog struct {
	byName      map[string]Symbol
	completion  []Symbol // non-deprecated builtins + predeclared + stdlib helpers
	predeclared map[string]bool
	loadModules []string                     // "re", "stdlib/….buddy"
	stdlibFuncs map[string][]stdlib.FuncInfo // module path → funcs
}

var (
	catalogOnce sync.Once
	catalogInst *catalog
	catalogErr  error
)

func getCatalog() (*catalog, error) {
	catalogOnce.Do(func() {
		catalogInst, catalogErr = buildCatalog()
	})
	return catalogInst, catalogErr
}

func buildCatalog() (*catalog, error) {
	c := &catalog{
		byName:      make(map[string]Symbol),
		predeclared: make(map[string]bool),
		stdlibFuncs: make(map[string][]stdlib.FuncInfo),
	}

	deprecated := script.DeprecatedAliases()

	_, flat := help.BuiltinsLibrary()
	seen := make(map[string]bool)
	for _, fn := range flat {
		if seen[fn.Name] {
			continue
		}
		seen[fn.Name] = true
		sym := Symbol{
			Name:      fn.Name,
			Signature: fn.Signature,
			Detail:    fn.Summary,
			Insert:    fn.Insert,
			Kind:      "builtin",
		}
		if repl, ok := deprecated[fn.Name]; ok {
			sym.Deprecated = true
			sym.Replacement = repl
		}
		c.byName[fn.Name] = sym
		c.predeclared[fn.Name] = true
		if !sym.Deprecated {
			c.completion = append(c.completion, sym)
		}
	}

	// Deprecated aliases may not appear in help catalog; still resolve them.
	for old, repl := range deprecated {
		if _, ok := c.byName[old]; ok {
			continue
		}
		c.byName[old] = Symbol{
			Name:        old,
			Signature:   old + "(...)",
			Detail:      "deprecated; use " + repl,
			Insert:      old + "()",
			Kind:        "builtin",
			Deprecated:  true,
			Replacement: repl,
		}
		c.predeclared[old] = true
	}

	for _, name := range []string{"cred", "args"} {
		sym := Symbol{
			Name:      name,
			Signature: name,
			Detail:    "predeclared in buddy scripts",
			Insert:    name,
			Kind:      "predeclared",
		}
		c.byName[name] = sym
		c.predeclared[name] = true
		c.completion = append(c.completion, sym)
	}

	reMod, err := re.LoadModule()
	if err != nil {
		return nil, err
	}
	for k := range reMod {
		c.predeclared[k] = true
		if _, ok := c.byName[k]; !ok {
			sym := Symbol{
				Name:      k,
				Signature: k,
				Detail:    "starlib re module (also load(\"re\", \"re\"))",
				Insert:    k,
				Kind:      "predeclared",
			}
			c.byName[k] = sym
			c.completion = append(c.completion, sym)
		}
	}

	for name := range starlark.Universe {
		c.predeclared[name] = true
		if _, ok := c.byName[name]; !ok {
			c.byName[name] = Symbol{
				Name:      name,
				Signature: name,
				Detail:    "Starlark universe",
				Insert:    name,
				Kind:      "universe",
			}
		}
	}

	c.loadModules = []string{"re"}
	mods, err := stdlib.Catalog()
	if err != nil {
		return nil, err
	}
	for _, m := range mods {
		c.loadModules = append(c.loadModules, m.Path)
		c.stdlibFuncs[m.Path] = m.Functions
		for _, fn := range m.Functions {
			sym := Symbol{
				Name:      fn.Name,
				Signature: fn.Signature,
				Detail:    fn.Summary + " (via load(\"" + m.Path + "\", \"" + fn.Name + "\"))",
				Insert:    fn.Insert,
				Kind:      "stdlib",
			}
			// Stdlib names are not predeclared until loaded; keep for hover after load completion.
			key := m.Path + "#" + fn.Name
			c.byName[key] = sym
			c.completion = append(c.completion, Symbol{
				Name:      fn.Name,
				Signature: fn.Signature,
				Detail:    sym.Detail,
				Insert:    fn.Insert,
				Kind:      "stdlib",
			})
		}
	}

	return c, nil
}

func (c *catalog) isPredeclared(name string) bool {
	return c.predeclared[name]
}

func (c *catalog) lookup(name string) (Symbol, bool) {
	sym, ok := c.byName[name]
	return sym, ok
}

func (c *catalog) filterCompletion(prefix string) []Symbol {
	prefix = strings.ToLower(prefix)
	var out []Symbol
	for _, sym := range c.completion {
		if prefix == "" || strings.HasPrefix(strings.ToLower(sym.Name), prefix) {
			out = append(out, sym)
		}
	}
	return out
}

func validLoadModule(module string) bool {
	if module == "re" {
		return true
	}
	return stdlib.ValidModule(module)
}
