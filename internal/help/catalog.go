package help

import (
	"strings"
)

// LibraryFunc is one browsable function for the notebook libraries API.
type LibraryFunc struct {
	Name      string `json:"name"`
	Signature string `json:"signature"`
	Summary   string `json:"summary,omitempty"`
	Insert    string `json:"insert"`
	Section   string `json:"section,omitempty"`
}

// LibrarySection groups builtins by help section heading.
type LibrarySection struct {
	Title     string        `json:"title"`
	Functions []LibraryFunc `json:"functions"`
}

// BuiltinsLibrary returns the builtins catalog parsed from Builtins help text.
func BuiltinsLibrary() (sections []LibrarySection, flat []LibraryFunc) {
	return parseBuiltinsCatalog(Builtins)
}

func parseBuiltinsCatalog(src string) (sections []LibrarySection, flat []LibraryFunc) {
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	i := 0
	for i < len(lines) {
		line := lines[i]
		if strings.HasPrefix(line, "===") || strings.HasPrefix(line, "buddy built-in") {
			i++
			continue
		}
		break
	}
	for i < len(lines) {
		line := lines[i]
		if isBuiltinsSection(line) {
			break
		}
		i++
	}

	var cur *LibrarySection
	flush := func() {
		if cur != nil && len(cur.Functions) > 0 {
			sections = append(sections, *cur)
		}
	}

	for i < len(lines) {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			i++
			continue
		}
		if isBuiltinsSection(line) {
			flush()
			cur = &LibrarySection{Title: line}
			i++
			continue
		}
		if m := builtinsFuncRE.FindStringSubmatch(line); m != nil {
			if cur == nil {
				cur = &LibrarySection{Title: "BUILTINS"}
			}
			name := m[1]
			sigLines := []string{strings.TrimSpace(line)}
			i++
			for i < len(lines) {
				cont := lines[i]
				trim := strings.TrimSpace(cont)
				if trim == "" {
					break
				}
				if isBuiltinsSection(cont) || builtinsFuncRE.MatchString(cont) {
					break
				}
				if strings.HasPrefix(cont, "  ") && !strings.HasPrefix(strings.TrimLeft(cont, " "), "→") &&
					!strings.HasPrefix(strings.TrimLeft(cont, " "), "Params:") {
					left := strings.TrimLeft(cont, " ")
					if strings.HasPrefix(left, "→") || strings.HasPrefix(left, "Params:") {
						break
					}
					if strings.HasPrefix(cont, "                ") || (strings.HasPrefix(cont, "  ") && !strings.HasPrefix(cont, "      ")) {
						sigLines = append(sigLines, left)
						i++
						continue
					}
					if strings.HasPrefix(cont, "      ") {
						break
					}
					sigLines = append(sigLines, left)
					i++
					continue
				}
				break
			}
			var ret, params string
			for i < len(lines) {
				cont := lines[i]
				trim := strings.TrimSpace(cont)
				if trim == "" {
					i++
					break
				}
				if isBuiltinsSection(cont) || builtinsFuncRE.MatchString(cont) {
					break
				}
				left := strings.TrimLeft(cont, " ")
				if strings.HasPrefix(left, "→") {
					ret = strings.TrimSpace(strings.TrimPrefix(left, "→"))
					i++
					continue
				}
				if strings.HasPrefix(left, "Params:") {
					params = strings.TrimSpace(strings.TrimPrefix(left, "Params:"))
					i++
					continue
				}
				break
			}
			sig := strings.Join(sigLines, " ")
			summary := ret
			if summary == "" {
				summary = params
			}
			fn := LibraryFunc{
				Name:      name,
				Signature: sig,
				Summary:   summary,
				Insert:    name + "()",
				Section:   cur.Title,
			}
			cur.Functions = append(cur.Functions, fn)
			flat = append(flat, fn)
			continue
		}
		i++
	}
	flush()
	return sections, flat
}
