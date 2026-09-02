package help

import (
	"regexp"
	"strings"
)

type parsedCLI struct {
	Does     string
	Lead     string
	Sections []cliSection
}

type cliSection struct {
	Label string
	Body  string
}

var cliSectionHeaderRE = regexp.MustCompile(`(?i)^(Usage|Subcommands|Arguments|Commands|Global flags|Notes|Examples|Well-known config paths|Flags(?: for '[^']+')?)\s*:\s*$`)

func parseCLIHelp(body string) parsedCLI {
	lines := strings.Split(strings.ReplaceAll(strings.TrimSpace(body), "\r\n", "\n"), "\n")
	var out parsedCLI
	if len(lines) == 0 {
		return out
	}
	out.Does = cliHeadlineDoes(lines[0])
	i := 1
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}

	var lead []string
	for i < len(lines) {
		if _, ok := cliSectionHeader(lines[i]); ok {
			break
		}
		lead = append(lead, lines[i])
		i++
	}
	out.Lead = strings.TrimSpace(strings.Join(lead, "\n"))

	var cur *cliSection
	flush := func() {
		if cur == nil {
			return
		}
		cur.Body = strings.TrimRight(cur.Body, "\n")
		if strings.TrimSpace(cur.Body) != "" || cur.Label != "" {
			out.Sections = append(out.Sections, *cur)
		}
		cur = nil
	}
	start := func(label string) {
		flush()
		cur = &cliSection{Label: label}
	}
	for i < len(lines) {
		line := lines[i]
		if label, ok := cliSectionHeader(line); ok {
			start(cliSectionLabel(label))
			i++
			continue
		}
		if cur == nil {
			i++
			continue
		}
		unindented := strings.TrimSpace(line) != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t")
		if unindented && !strings.EqualFold(cur.Label, "Notes") && !strings.EqualFold(cur.Label, "Examples") {
			start("Notes")
		}
		if cur.Body != "" {
			cur.Body += "\n"
		}
		cur.Body += line
		i++
	}
	flush()
	return out
}

func cliSectionHeader(line string) (label string, ok bool) {
	if m := cliSectionHeaderRE.FindStringSubmatch(line); m != nil {
		return m[1], true
	}
	if strings.TrimRight(line, " \t") == "  Notes:" {
		return "Notes", true
	}
	return "", false
}

func cliHeadlineDoes(headline string) string {
	headline = strings.TrimSpace(headline)
	for _, sep := range []string{" - ", " — "} {
		if i := strings.Index(headline, sep); i >= 0 {
			return strings.TrimSpace(headline[i+len(sep):])
		}
	}
	return ""
}

func cliSectionLabel(raw string) string {
	s := strings.TrimSpace(raw)
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "flags for '") && strings.HasSuffix(lower, "'") {
		inner := s[len("Flags for '") : len(s)-1]
		if inner != "" {
			return "Flags (" + inner + ")"
		}
	}
	switch lower {
	case "well-known config paths":
		return "Paths"
	case "global flags":
		return "Globals"
	case "arguments":
		return "Args"
	case "usage":
		return "Usage"
	case "subcommands":
		return "Subcommands"
	case "commands":
		return "Commands"
	case "notes":
		return "Notes"
	case "examples":
		return "Examples"
	case "flags":
		return "Flags"
	}
	return s
}

func renderCLIEntryHTML(c CommandHelp) string {
	parsed := parseCLIHelp(c.Body)
	var b strings.Builder
	b.WriteString(`<article class="fn-entry">`)
	b.WriteString("\n<h3 id=\"")
	b.WriteString(htmlEscapeText(c.ID))
	b.WriteString(`"><code class="fn-sig">`)
	b.WriteString(`<b class="fn-name">`)
	b.WriteString(htmlEscapeText(c.Title))
	b.WriteString("</b>")
	if parsed.Does != "" {
		b.WriteString(" — ")
		b.WriteString(htmlEscapeText(parsed.Does))
	}
	b.WriteString("</code></h3>\n")
	if parsed.Lead != "" {
		for _, para := range strings.Split(parsed.Lead, "\n\n") {
			para = strings.TrimSpace(para)
			if para == "" {
				continue
			}
			b.WriteString("<p>")
			b.WriteString(htmlEscapeText(strings.Join(strings.Fields(para), " ")))
			b.WriteString("</p>\n")
		}
	}
	if len(parsed.Sections) > 0 {
		b.WriteString(`<dl class="fn-meta">`)
		b.WriteString("\n")
		for _, sec := range parsed.Sections {
			writeCLISection(&b, sec)
		}
		b.WriteString("</dl>\n")
	}
	b.WriteString("</article>\n\n")
	return b.String()
}

func writeCLISection(b *strings.Builder, sec cliSection) {
	body := strings.Trim(sec.Body, "\n")
	if body == "" {
		return
	}
	b.WriteString("<dt>")
	b.WriteString(htmlEscapeText(sec.Label))
	b.WriteString("</dt>\n<dd>")
	switch {
	case strings.EqualFold(sec.Label, "Notes"):
		b.WriteString(cliNotesHTML(body))
	case strings.HasPrefix(strings.ToLower(sec.Label), "flags") ||
		strings.EqualFold(sec.Label, "Subcommands") ||
		strings.EqualFold(sec.Label, "Commands") ||
		strings.EqualFold(sec.Label, "Globals") ||
		strings.EqualFold(sec.Label, "Args"):
		if items := splitCLIDefItems(body); len(items) > 0 {
			b.WriteString(cliItemsHTML(items))
		} else {
			b.WriteString(cliPreHTML(body))
		}
	default:
		b.WriteString(cliPreHTML(body))
	}
	b.WriteString("</dd>\n")
}

type cliDefItem struct {
	Key  string
	Desc string
}

func splitCLIDefItems(s string) []cliDefItem {
	lines := strings.Split(s, "\n")
	var items []cliDefItem
	var cur *cliDefItem
	flush := func() {
		if cur == nil {
			return
		}
		cur.Key = strings.TrimSpace(cur.Key)
		cur.Desc = strings.TrimSpace(cur.Desc)
		if cur.Key != "" {
			items = append(items, *cur)
		}
		cur = nil
	}
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		key, desc, ok := cliDefLine(line)
		if ok {
			flush()
			cur = &cliDefItem{Key: key, Desc: desc}
			continue
		}
		if cur != nil && (strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t")) {
			extra := strings.TrimSpace(line)
			if extra == "" {
				continue
			}
			if cur.Desc != "" {
				cur.Desc += " "
			}
			cur.Desc += extra
			continue
		}
		flush()
		return nil
	}
	flush()
	if len(items) == 0 {
		return nil
	}
	return items
}

func cliDefLine(line string) (key, desc string, ok bool) {
	if !strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "   ") {
		return "", "", false
	}
	rest := strings.TrimRight(line[2:], " \t")
	if rest == "" {
		return "", "", false
	}
	idx := -1
	for i := 0; i < len(rest)-1; i++ {
		if rest[i] == '\t' || (rest[i] == ' ' && rest[i+1] == ' ') {
			idx = i
			break
		}
	}
	if idx >= 0 {
		key = strings.TrimSpace(rest[:idx])
		desc = strings.TrimSpace(rest[idx:])
		if key == "" {
			return "", "", false
		}
		return key, desc, true
	}
	// Column overflow: "device-group Manage …" (one space before a capital).
	if i := strings.IndexByte(rest, ' '); i > 0 {
		tok, rem := rest[:i], strings.TrimSpace(rest[i+1:])
		if rem != "" && rem[0] >= 'A' && rem[0] <= 'Z' && !strings.ContainsAny(tok, "<,") {
			return tok, rem, true
		}
	}
	return rest, "", true
}

func cliItemsHTML(items []cliDefItem) string {
	var b strings.Builder
	b.WriteString(`<ul class="fn-params">`)
	for _, it := range items {
		b.WriteString("<li>")
		b.WriteString("<code>")
		b.WriteString(htmlEscapeText(it.Key))
		b.WriteString("</code>")
		if it.Desc != "" {
			b.WriteString(" — ")
			b.WriteString(htmlEscapeText(it.Desc))
		}
		b.WriteString("</li>")
	}
	b.WriteString("</ul>")
	return b.String()
}

func cliNotesHTML(s string) string {
	s = strings.TrimSpace(s)
	paras := strings.Split(s, "\n\n")
	var parts []string
	for _, p := range paras {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		joined := strings.Join(strings.Fields(p), " ")
		parts = append(parts, htmlEscapeText(joined))
	}
	return strings.Join(parts, "<br><br>")
}

func cliPreHTML(s string) string {
	s = strings.TrimRight(s, "\n")
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		if strings.HasPrefix(ln, "  ") {
			lines[i] = ln[2:]
		}
	}
	return "<pre class=\"fn-pre\">" + htmlEscapeText(strings.Join(lines, "\n")) + "</pre>"
}
