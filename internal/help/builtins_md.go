package help

import (
	"regexp"
	"strings"
)

var (
	builtinsSectionRE = regexp.MustCompile(`^[A-Z][A-Z0-9 /&._\-]*(?: — .+)?$`)
	builtinsFuncRE    = regexp.MustCompile(`^  ([a-z_][a-z0-9_]*)\(`)
)

// BuiltinsMarkdown converts the CLI builtins help text into structured Markdown
// suitable for HTML docs (section headings + per-function anchors).
func BuiltinsMarkdown() string {
	return formatBuiltinsMarkdown(Builtins)
}

func formatBuiltinsMarkdown(src string) string {
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	var b strings.Builder
	b.WriteString("# Starlark builtins\n\n")
	b.WriteString("Canonical reference for buddy’s Starlark built-in functions ")
	b.WriteString("(same text as `buddy help builtins`, rendered for browsing).\n\n")

	i := 0
	// Skip title / underline
	for i < len(lines) {
		line := lines[i]
		if strings.HasPrefix(line, "===") || strings.HasPrefix(line, "buddy built-in") {
			i++
			continue
		}
		break
	}

	// Intro until first section header
	var intro []string
	for i < len(lines) {
		line := lines[i]
		if isBuiltinsSection(line) {
			break
		}
		intro = append(intro, line)
		i++
	}
	introText := strings.TrimSpace(strings.Join(intro, "\n"))
	if introText != "" {
		b.WriteString(introText)
		b.WriteString("\n\n")
	}

	for i < len(lines) {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			i++
			continue
		}
		if isBuiltinsSection(line) {
			b.WriteString("<h2 id=\"")
			b.WriteString(sectionAnchorID(line))
			b.WriteString("\">")
			b.WriteString(htmlEscapeText(line))
			b.WriteString("</h2>\n\n")
			i++
			continue
		}
		if m := builtinsFuncRE.FindStringSubmatch(line); m != nil {
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
				// Signature continuation (indented, not → / Params)
				if strings.HasPrefix(cont, "  ") && !strings.HasPrefix(strings.TrimLeft(cont, " "), "→") &&
					!strings.HasPrefix(strings.TrimLeft(cont, " "), "Params:") {
					// Could be signature wrap or → / Params with more indent
					left := strings.TrimLeft(cont, " ")
					if strings.HasPrefix(left, "→") || strings.HasPrefix(left, "Params:") {
						break
					}
					// If heavily indented continuation of signature (no colon start for params)
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
				// Unexpected — stop
				break
			}
			b.WriteString("<h3 id=\"")
			b.WriteString(name)
			b.WriteString("\"><code>")
			b.WriteString(name)
			b.WriteString("</code></h3>\n\n")
			b.WriteString("```\n")
			b.WriteString(strings.Join(sigLines, "\n"))
			b.WriteString("\n```\n\n")
			if ret != "" {
				b.WriteString("**Returns:** ")
				b.WriteString(ret)
				b.WriteString("\n\n")
			}
			if params != "" {
				b.WriteString(params)
				b.WriteString("\n\n")
			}
			continue
		}
		// Free-form lines (examples, notes under a section)
		if !strings.HasPrefix(line, " ") {
			b.WriteString(line)
			b.WriteString("\n\n")
			i++
			continue
		}
		// Indented prose / examples — keep as code or paragraph
		b.WriteString("```\n")
		for i < len(lines) {
			cont := lines[i]
			if strings.TrimSpace(cont) == "" {
				i++
				break
			}
			if isBuiltinsSection(cont) || builtinsFuncRE.MatchString(cont) {
				break
			}
			if !strings.HasPrefix(cont, " ") && cont != "" {
				break
			}
			b.WriteString(cont)
			b.WriteString("\n")
			i++
		}
		b.WriteString("```\n\n")
	}
	return b.String()
}

func isBuiltinsSection(line string) bool {
	if line == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
		return false
	}
	// Title already skipped; section headers are short ALL-CAPS-ish lines.
	if strings.HasPrefix(line, "buddy ") || strings.HasPrefix(line, "All ") ||
		strings.HasPrefix(line, "Check ") || strings.HasPrefix(line, "Conventions:") ||
		strings.HasPrefix(line, "Use ") || strings.HasPrefix(line, "Paginated ") ||
		strings.HasPrefix(line, "Omit ") || strings.HasPrefix(line, "Each ") {
		return false
	}
	if strings.HasPrefix(line, "EXAMPLE") {
		return true
	}
	return builtinsSectionRE.MatchString(line)
}

// sectionAnchorID builds a stable id for a builtins section heading (avoids
// colliding with function names like policy vs POLICY).
func sectionAnchorID(section string) string {
	s := strings.ToLower(strings.TrimSpace(section))
	var b strings.Builder
	b.WriteString("section-")
	prevDash := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevDash = false
			continue
		}
		if !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func htmlEscapeText(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
