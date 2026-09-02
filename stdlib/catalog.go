package stdlib

import (
	"path"
	"regexp"
	"strings"
)

var (
	defRE     = regexp.MustCompile(`(?m)^def ([a-z][a-z0-9_]*)\(([\s\S]*?)\):`)
	commentRE = regexp.MustCompile(`(?m)^# ([a-z][a-z0-9_]*) — (.+)$`)
)

// FuncInfo describes an exported top-level function in a stdlib module.
type FuncInfo struct {
	Name      string
	Signature string // def name(...) compact
	Summary   string
	Insert    string
}

// ModuleInfo is a browsable stdlib module.
type ModuleInfo struct {
	ID        string // e.g. stdlib/demo
	Title     string // e.g. demo
	Path      string // e.g. stdlib/demo.buddy
	Functions []FuncInfo
}

// Catalog returns metadata for all embedded stdlib modules.
func Catalog() ([]ModuleInfo, error) {
	mods, err := List()
	if err != nil {
		return nil, err
	}
	out := make([]ModuleInfo, 0, len(mods))
	for _, modPath := range mods {
		data, err := Read(modPath)
		if err != nil {
			return nil, err
		}
		base := strings.TrimSuffix(path.Base(modPath), ".buddy")
		info := ModuleInfo{
			ID:        "stdlib/" + base,
			Title:     base,
			Path:      modPath,
			Functions: parseModuleFuncs(modPath, string(data)),
		}
		out = append(out, info)
	}
	return out, nil
}

func parseModuleFuncs(modPath, src string) []FuncInfo {
	summaries := map[string]string{}
	for _, m := range commentRE.FindAllStringSubmatch(src, -1) {
		summaries[m[1]] = strings.TrimSpace(m[2])
	}

	var out []FuncInfo
	for _, m := range defRE.FindAllStringSubmatch(src, -1) {
		name := m[1]
		if strings.HasPrefix(name, "_") {
			continue
		}
		params := compactParams(m[2])
		sig := "def " + name + "(" + params + ")"
		summary := summaries[name]
		if summary == "" {
			summary = extractDocstringAfterDef(src, m[0])
		}
		insert := "load(\"" + modPath + "\", \"" + name + "\")\n\n" +
			"r = " + name + "(" + stubArgs(params) + ")\n"
		out = append(out, FuncInfo{
			Name:      name,
			Signature: sig,
			Summary:   summary,
			Insert:    insert,
		})
	}
	return out
}

func compactParams(params string) string {
	parts := strings.Split(params, ",")
	var cleaned []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// Collapse whitespace inside default values.
		p = strings.Join(strings.Fields(p), " ")
		cleaned = append(cleaned, p)
	}
	return strings.Join(cleaned, ", ")
}

func stubArgs(params string) string {
	if params == "" {
		return ""
	}
	parts := strings.Split(params, ",")
	var args []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		name := p
		if i := strings.Index(p, "="); i >= 0 {
			name = strings.TrimSpace(p[:i])
		}
		args = append(args, name+" = …")
	}
	return strings.Join(args, ", ")
}

func extractDocstringAfterDef(src, defMatch string) string {
	idx := strings.Index(src, defMatch)
	if idx < 0 {
		return ""
	}
	rest := src[idx+len(defMatch):]
	rest = strings.TrimLeft(rest, " \t\n")
	for _, quote := range []string{`"""`, `'''`} {
		if strings.HasPrefix(rest, quote) {
			body := rest[len(quote):]
			end := strings.Index(body, quote)
			if end < 0 {
				return ""
			}
			doc := strings.TrimSpace(body[:end])
			if line, _, ok := strings.Cut(doc, "\n"); ok {
				return strings.TrimSpace(line)
			}
			return doc
		}
	}
	return ""
}
