package help

import (
	"regexp"
	"strings"
)

var (
	builtinsSectionRE = regexp.MustCompile(`^[A-Z][A-Z0-9 /&._\-]*(?: — .+)?$`)
	builtinsFuncRE    = regexp.MustCompile(`^  ([a-z_][a-z0-9_]*)\(`)
)

type builtinFields struct {
	Does    string
	Returns string
	Params  string
	Notes   string
}

func isBuiltinFieldLabel(left string) bool {
	return strings.HasPrefix(left, "→") ||
		strings.HasPrefix(left, "Does:") ||
		strings.HasPrefix(left, "Params:") ||
		strings.HasPrefix(left, "Notes:") ||
		strings.HasPrefix(left, "Returns:")
}

func appendWrappedField(cur *string, extra string) {
	extra = strings.TrimSpace(extra)
	if extra == "" {
		return
	}
	if *cur == "" {
		*cur = extra
		return
	}
	*cur = *cur + " " + extra
}

// consumeBuiltinSignature reads a function signature starting at lines[i].
func consumeBuiltinSignature(lines []string, i int) (name string, sigLines []string, next int, ok bool) {
	if i >= len(lines) {
		return "", nil, i, false
	}
	m := builtinsFuncRE.FindStringSubmatch(lines[i])
	if m == nil {
		return "", nil, i, false
	}
	name = m[1]
	sigLines = []string{strings.TrimSpace(lines[i])}
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
		left := strings.TrimLeft(cont, " ")
		if isBuiltinFieldLabel(left) {
			break
		}
		if strings.HasPrefix(cont, "  ") {
			sigLines = append(sigLines, left)
			i++
			continue
		}
		break
	}
	return name, sigLines, i, true
}

// consumeBuiltinFields reads Does:/→/Params:/Notes: (and wrapped continuations).
func consumeBuiltinFields(lines []string, i int) (builtinFields, int) {
	var f builtinFields
	var cur *string
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
		switch {
		case strings.HasPrefix(left, "→"):
			f.Returns = strings.TrimSpace(strings.TrimPrefix(left, "→"))
			cur = &f.Returns
			i++
		case strings.HasPrefix(left, "Returns:"):
			f.Returns = strings.TrimSpace(strings.TrimPrefix(left, "Returns:"))
			cur = &f.Returns
			i++
		case strings.HasPrefix(left, "Does:"):
			f.Does = strings.TrimSpace(strings.TrimPrefix(left, "Does:"))
			cur = &f.Does
			i++
		case strings.HasPrefix(left, "Params:"):
			f.Params = strings.TrimSpace(strings.TrimPrefix(left, "Params:"))
			cur = &f.Params
			i++
		case strings.HasPrefix(left, "Notes:"):
			f.Notes = strings.TrimSpace(strings.TrimPrefix(left, "Notes:"))
			cur = &f.Notes
			i++
		default:
			if cur != nil && strings.HasPrefix(cont, " ") {
				appendWrappedField(cur, left)
				i++
				continue
			}
			return f, i
		}
	}
	return f, i
}

// BuiltinsMarkdown converts the CLI builtins help text into structured Markdown
// suitable for HTML docs (section headings + per-function anchors).
func BuiltinsMarkdown() string {
	return formatBuiltinsMarkdown(Builtins)
}

func formatBuiltinsMarkdown(src string) string {
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	var b strings.Builder
	b.WriteString("# Function index\n\n")
	b.WriteString("Canonical reference for buddy’s buddyscript built-in functions ")
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
		if name, sigLines, next, ok := consumeBuiltinSignature(lines, i); ok {
			i = next
			fields, next := consumeBuiltinFields(lines, i)
			i = next
			sig := strings.Join(sigLines, " ")
			sig, inlineRet := splitInlineReturn(sig)
			if fields.Returns == "" {
				fields.Returns = inlineRet
			}
			b.WriteString(renderBuiltinEntryHTML(name, sig, fields))
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

func renderBuiltinEntryHTML(name, sig string, fields builtinFields) string {
	var b strings.Builder
	b.WriteString(`<article class="fn-entry">`)
	b.WriteString("\n<h3 id=\"")
	b.WriteString(name)
	b.WriteString(`"><code class="fn-sig">`)
	b.WriteString(formatSignatureHTML(name, sig))
	b.WriteString("</code></h3>\n")
	if fields.Does != "" {
		b.WriteString("<p>")
		b.WriteString(htmlEscapeText(fields.Does))
		b.WriteString("</p>\n")
	}
	ret := displayReturns(fields.Returns)
	if ret != "" || fields.Params != "" || fields.Notes != "" {
		b.WriteString(`<dl class="fn-meta">`)
		b.WriteString("\n")
		writeFnMeta(&b, "Returns", ret)
		writeFnParams(&b, fields.Params)
		writeFnMeta(&b, "Notes", fields.Notes)
		b.WriteString("</dl>\n")
	}
	b.WriteString("</article>\n\n")
	return b.String()
}

func formatSignatureHTML(name, sig string) string {
	nameHTML := `<b class="fn-name">` + htmlEscapeText(name) + `</b>`
	sig = strings.TrimSpace(sig)
	if sig == "" {
		return nameHTML
	}
	token := name + "("
	if !strings.Contains(sig, token) {
		return nameHTML + " " + htmlEscapeText(sig)
	}
	parts := strings.Split(sig, token)
	var b strings.Builder
	for i, p := range parts {
		if i > 0 {
			b.WriteString(nameHTML)
			b.WriteString("(")
		}
		b.WriteString(htmlEscapeText(p))
	}
	return b.String()
}

func writeFnMeta(b *strings.Builder, label, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	b.WriteString("<dt>")
	b.WriteString(label)
	b.WriteString("</dt>\n<dd>")
	b.WriteString(htmlEscapeText(value))
	b.WriteString("</dd>\n")
}

func writeFnParams(b *strings.Builder, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	items := splitParamItems(value)
	b.WriteString("<dt>Params</dt>\n<dd>")
	if len(items) == 1 {
		b.WriteString(formatParamItemHTML(items[0]))
		b.WriteString("</dd>\n")
		return
	}
	b.WriteString(`<ul class="fn-params">`)
	for _, it := range items {
		b.WriteString("<li>")
		b.WriteString(formatParamItemHTML(it))
		b.WriteString("</li>")
	}
	b.WriteString("</ul></dd>\n")
}

func splitParamItems(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var items []string
	start := 0
	for i := 0; i < len(s)-1; i++ {
		if (s[i] != '.' && s[i] != ';') || s[i+1] != ' ' {
			continue
		}
		rest := strings.TrimSpace(s[i+2:])
		if !looksLikeParamKeyPrefix(rest) {
			continue
		}
		item := strings.TrimSpace(s[start:i])
		if item != "" {
			items = append(items, item)
		}
		start = i + 2
	}
	if tail := strings.TrimSpace(s[start:]); tail != "" {
		items = append(items, tail)
	}
	if len(items) == 0 {
		return []string{s}
	}
	return items
}

func looksLikeParamKeyPrefix(s string) bool {
	if s == "" {
		return false
	}
	c := s[0]
	if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && c != '_' {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9',
			c == '_', c == '?', c == '/':
			continue
		case c == ':':
			return true
		default:
			return false
		}
	}
	return false
}

func formatParamItemHTML(s string) string {
	s = strings.TrimSpace(s)
	key, rest, ok := strings.Cut(s, ":")
	if !ok || !looksLikeParamKeyPrefix(key+":") {
		return htmlEscapeText(s)
	}
	key = strings.TrimSpace(key)
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return "<code>" + htmlEscapeText(key) + "</code>"
	}
	return "<code>" + htmlEscapeText(key) + "</code> — " + htmlEscapeText(rest)
}

func formatParamItemMarkdown(s string) string {
	s = strings.TrimSpace(s)
	key, rest, ok := strings.Cut(s, ":")
	if !ok || !looksLikeParamKeyPrefix(key+":") {
		return s
	}
	key = strings.TrimSpace(key)
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return "`" + key + "`"
	}
	return "`" + key + "` — " + rest
}

func displayReturns(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "value:")
	s = strings.TrimSpace(s)
	return s
}

func isBuiltinsSection(line string) bool {
	if line == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
		return false
	}
	// Title already skipped; section headers are short ALL-CAPS-ish lines.
	if strings.HasPrefix(line, "buddy ") || strings.HasPrefix(line, "All ") ||
		strings.HasPrefix(line, "Check ") || strings.HasPrefix(line, "Conventions:") ||
		strings.HasPrefix(line, "Use ") || strings.HasPrefix(line, "Paginated ") ||
		strings.HasPrefix(line, "Omit ") || strings.HasPrefix(line, "Each ") ||
		strings.HasPrefix(line, "print ") || strings.HasPrefix(line, "(or ") ||
		strings.HasPrefix(line, "cover ") {
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
