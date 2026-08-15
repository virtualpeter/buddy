package serve

import (
	"bytes"
	"path"
	"regexp"
	"strings"
	"sync"

	"buddy/internal/help"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

var headingRE = regexp.MustCompile(`(?m)^(#{1,3})\s+(.+)$`)
var htmlHeadingRE = regexp.MustCompile(`(?is)<h([23])\s+[^>]*id="([^"]+)"[^>]*>(.*?)</h[23]>`)
var mdLinkRE = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
var builtinsFnHeadingRE = regexp.MustCompile(`(?i)<h3\s+[^>]*id="([a-z_][a-z0-9_]*)"[^>]*>`)
var builtinIdentRE = regexp.MustCompile(`^([a-z_][a-z0-9_]*)`)

var (
	builtinIndexOnce sync.Once
	builtinNameSet   map[string]bool
	builtinNameOrder []string
)

func builtinNameIndex() (map[string]bool, []string) {
	builtinIndexOnce.Do(func() {
		_, flat := help.BuiltinsLibrary()
		builtinNameSet = make(map[string]bool, len(flat))
		builtinNameOrder = make([]string, 0, len(flat))
		for _, f := range flat {
			if f.Name == "" || builtinNameSet[f.Name] {
				continue
			}
			builtinNameSet[f.Name] = true
			builtinNameOrder = append(builtinNameOrder, f.Name)
		}
	})
	return builtinNameSet, builtinNameOrder
}

// prepareDocMarkdown rewrites in-repo links and turns BuddyScript names into
// function-index anchors (skip the index page itself).
func prepareDocMarkdown(markdown string, pages []DocPage, pageID string) string {
	src := rewriteDocLinks(markdown, pages)
	if pageID == "builtins" || pageID == "cli" {
		return src
	}
	return linkCLICodeSpans(linkBuiltinCodeSpans(src))
}

// linkBuiltinCodeSpans wraps inline `name` / `name(...)` / `name*` spans that
// match a known builtin with a markdown link to /docs/builtins#name.
// Fenced code blocks and already-linked spans are left alone.
func linkBuiltinCodeSpans(markdown string) string {
	names, order := builtinNameIndex()
	var b strings.Builder
	b.Grow(len(markdown) + 64)
	inFence := false
	i := 0
	for i < len(markdown) {
		if strings.HasPrefix(markdown[i:], "```") {
			inFence = !inFence
			end := strings.IndexByte(markdown[i:], '\n')
			if end < 0 {
				b.WriteString(markdown[i:])
				return b.String()
			}
			b.WriteString(markdown[i : i+end+1])
			i += end + 1
			continue
		}
		if inFence {
			end := strings.IndexByte(markdown[i:], '\n')
			if end < 0 {
				b.WriteString(markdown[i:])
				return b.String()
			}
			b.WriteString(markdown[i : i+end+1])
			i += end + 1
			continue
		}
		if markdown[i] != '`' {
			b.WriteByte(markdown[i])
			i++
			continue
		}
		j := i + 1
		for j < len(markdown) && markdown[j] != '`' && markdown[j] != '\n' {
			j++
		}
		if j >= len(markdown) || markdown[j] != '`' {
			b.WriteByte(markdown[i])
			i++
			continue
		}
		// Already [`code`](url) — the opening [ was written on a prior step.
		if i > 0 && markdown[i-1] == '[' && j+2 < len(markdown) && markdown[j+1] == ']' && markdown[j+2] == '(' {
			k := j + 3
			for k < len(markdown) && markdown[k] != ')' {
				k++
			}
			if k < len(markdown) {
				b.WriteString(markdown[i : k+1])
				i = k + 1
				continue
			}
		}
		code := markdown[i+1 : j]
		if anchor := resolveBuiltinAnchor(code, names, order); anchor != "" {
			b.WriteString("[`")
			b.WriteString(code)
			b.WriteString("`](/docs/builtins#")
			b.WriteString(anchor)
			b.WriteString(")")
			i = j + 1
			continue
		}
		b.WriteString(markdown[i : j+1])
		i = j + 1
	}
	return b.String()
}

func resolveBuiltinAnchor(code string, names map[string]bool, order []string) string {
	s := strings.TrimSpace(code)
	m := builtinIdentRE.FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	name := m[1]
	rest := s[len(name):]
	if names[name] {
		if rest == "" || strings.HasPrefix(rest, "(") || strings.HasPrefix(rest, "*") {
			return name
		}
		return ""
	}
	if !strings.Contains(s, "*") {
		return ""
	}
	prefix := strings.TrimRight(name, "_")
	if prefix == "" {
		return ""
	}
	for _, n := range order {
		if n == prefix || strings.HasPrefix(n, prefix+"_") || strings.HasPrefix(n, prefix) {
			return n
		}
	}
	return ""
}

var (
	cliIndexOnce sync.Once
	cliAnchorSet map[string]bool
)

func cliAnchors() map[string]bool {
	cliIndexOnce.Do(func() {
		cliAnchorSet = help.CommandAnchorIDs()
	})
	return cliAnchorSet
}

// linkCLICodeSpans wraps `buddy <command> …` spans with Command index anchors.
func linkCLICodeSpans(markdown string) string {
	ids := cliAnchors()
	var b strings.Builder
	b.Grow(len(markdown) + 64)
	inFence := false
	i := 0
	for i < len(markdown) {
		if strings.HasPrefix(markdown[i:], "```") {
			inFence = !inFence
			end := strings.IndexByte(markdown[i:], '\n')
			if end < 0 {
				b.WriteString(markdown[i:])
				return b.String()
			}
			b.WriteString(markdown[i : i+end+1])
			i += end + 1
			continue
		}
		if inFence {
			end := strings.IndexByte(markdown[i:], '\n')
			if end < 0 {
				b.WriteString(markdown[i:])
				return b.String()
			}
			b.WriteString(markdown[i : i+end+1])
			i += end + 1
			continue
		}
		if markdown[i] != '`' {
			b.WriteByte(markdown[i])
			i++
			continue
		}
		j := i + 1
		for j < len(markdown) && markdown[j] != '`' && markdown[j] != '\n' {
			j++
		}
		if j >= len(markdown) || markdown[j] != '`' {
			b.WriteByte(markdown[i])
			i++
			continue
		}
		if i > 0 && markdown[i-1] == '[' && j+2 < len(markdown) && markdown[j+1] == ']' && markdown[j+2] == '(' {
			k := j + 3
			for k < len(markdown) && markdown[k] != ')' {
				k++
			}
			if k < len(markdown) {
				b.WriteString(markdown[i : k+1])
				i = k + 1
				continue
			}
		}
		code := markdown[i+1 : j]
		if anchor := resolveCLIAnchor(code, ids); anchor != "" {
			b.WriteString("[`")
			b.WriteString(code)
			b.WriteString("`](/docs/cli#")
			b.WriteString(anchor)
			b.WriteString(")")
			i = j + 1
			continue
		}
		b.WriteString(markdown[i : j+1])
		i = j + 1
	}
	return b.String()
}

func resolveCLIAnchor(code string, ids map[string]bool) string {
	s := strings.TrimSpace(code)
	if !strings.HasPrefix(s, "buddy ") && s != "buddy" {
		return ""
	}
	rest := strings.TrimSpace(strings.TrimPrefix(s, "buddy"))
	var parts []string
	for _, tok := range strings.Fields(rest) {
		if strings.HasPrefix(tok, "-") || strings.HasPrefix(tok, "<") || tok == "…" || tok == "..." {
			break
		}
		if strings.ContainsAny(tok, "|/") {
			break
		}
		parts = append(parts, tok)
		if len(parts) == 2 {
			break
		}
	}
	if len(parts) == 2 {
		id := parts[0] + "-" + parts[1]
		if ids[id] {
			return id
		}
	}
	if len(parts) >= 1 && ids[parts[0]] {
		return parts[0]
	}
	return ""
}

type tocEntry struct {
	Level int
	ID    string
	Text  string
}

func newMarkdown() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Linkify,
			highlighting.NewHighlighting(
				highlighting.WithStyle("github"),
				highlighting.WithFormatOptions(chromahtml.WithClasses(true)),
			),
		),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)
}

func renderMarkdown(md goldmark.Markdown, src string) (string, error) {
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// rewriteDocLinks rewrites relative markdown / example links to /docs/<page-id>
// so in-page navigation works in the docs UI (bare foo.md would otherwise 404).
func rewriteDocLinks(markdown string, pages []DocPage) string {
	byID := map[string]bool{}
	byBase := map[string][]string{} // basename → page ids (may collide, e.g. README.md)
	for _, p := range pages {
		byID[p.ID] = true
		base := path.Base(p.ID)
		byBase[base] = append(byBase[base], p.ID)
		if strings.HasSuffix(base, ".md") {
			stem := strings.TrimSuffix(base, ".md")
			byBase[stem] = append(byBase[stem], p.ID)
		}
	}
	pick := func(candidates ...string) string {
		seen := map[string]bool{}
		var matches []string
		for _, c := range candidates {
			if c == "" || seen[c] {
				continue
			}
			seen[c] = true
			if byID[c] {
				matches = append(matches, c)
				continue
			}
			if ids := byBase[c]; len(ids) == 1 {
				matches = append(matches, ids[0])
			} else if len(ids) > 1 {
				// Prefer docs/<name> over examples/README.md style collisions.
				for _, id := range ids {
					if strings.HasPrefix(id, "docs/") {
						matches = append(matches, id)
						break
					}
				}
			}
		}
		if len(matches) == 0 {
			return ""
		}
		return matches[0]
	}

	return mdLinkRE.ReplaceAllStringFunc(markdown, func(m string) string {
		sub := mdLinkRE.FindStringSubmatch(m)
		if len(sub) != 3 {
			return m
		}
		text, dest := sub[1], sub[2]
		dest = strings.TrimSpace(dest)
		if dest == "" || strings.HasPrefix(dest, "#") ||
			strings.HasPrefix(dest, "http://") || strings.HasPrefix(dest, "https://") ||
			strings.HasPrefix(dest, "mailto:") || strings.HasPrefix(dest, "/docs/") {
			return m
		}
		frag := ""
		if i := strings.IndexByte(dest, '#'); i >= 0 {
			frag = dest[i:]
			dest = dest[:i]
		}
		dest = strings.TrimSpace(dest)
		if dest == "" {
			return m
		}
		// Normalize paths like ../examples/foo.buddy or ./change-batches.md
		clean := path.Clean(strings.ReplaceAll(dest, "\\", "/"))
		origClean := clean
		clean = strings.TrimPrefix(clean, "./")
		upCount := 0
		for strings.HasPrefix(clean, "../") {
			clean = strings.TrimPrefix(clean, "../")
			upCount++
		}
		// Repo-root escapes (../README.md, ../scripts/…) are not docs pages.
		// Do not leave bare ../ links: from /docs/examples/… the browser would
		// resolve them under /docs/ and 404. Flatten to a code-styled path instead.
		if upCount > 0 && !strings.Contains(origClean, "/docs/") && !strings.Contains(origClean, "/examples/") &&
			!strings.HasPrefix(clean, "docs/") && !strings.HasPrefix(clean, "examples/") {
			return "`" + strings.TrimPrefix(origClean, "../") + frag + "`"
		}
		base := path.Base(clean)
		candidates := []string{clean, "docs/" + strings.TrimSuffix(base, ".md"), "examples/" + base, base}
		if strings.HasSuffix(base, ".md") {
			candidates = append(candidates, strings.TrimSuffix(base, ".md"))
		}
		if id := pick(candidates...); id != "" {
			return "[" + text + "](/docs/" + id + frag + ")"
		}
		return m
	})
}

func extractTOC(markdown string) []tocEntry {
	var out []tocEntry
	for _, m := range headingRE.FindAllStringSubmatch(markdown, -1) {
		level := len(m[1])
		text := strings.TrimSpace(m[2])
		text = strings.Trim(text, "#")
		text = strings.TrimSpace(text)
		id := slugify(text)
		out = append(out, tocEntry{Level: level, ID: id, Text: stripMDCode(text)})
	}
	for _, m := range htmlHeadingRE.FindAllStringSubmatch(markdown, -1) {
		level := 0
		switch m[1] {
		case "2":
			level = 2
		case "3":
			level = 3
		}
		id := m[2]
		text := strings.TrimSpace(stripHTMLTags(m[3]))
		out = append(out, tocEntry{Level: level, ID: id, Text: text})
	}
	return out
}

func stripMDCode(s string) string {
	return strings.Trim(strings.TrimSpace(s), "`")
}

func stripHTMLTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func slugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
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

// searchHit is one match for client or server search.
type searchHit struct {
	PageID   string `json:"pageId"`
	Title    string `json:"title"`
	Snippet  string `json:"snippet"`
	Fragment string `json:"fragment,omitempty"`
}

func searchPages(pages []DocPage, q string, limit int) []searchHit {
	q = strings.TrimSpace(strings.ToLower(q))
	if q == "" || limit <= 0 {
		return nil
	}
	var hits []searchHit
	for _, p := range pages {
		if p.ID == "builtins" || p.ID == "cli" {
			kind := "builtin"
			if p.ID == "cli" {
				kind = "command"
			}
			fnHits := 0
			for _, m := range htmlHeadingRE.FindAllStringSubmatch(p.Markdown, -1) {
				name := m[2]
				text := strings.TrimSpace(stripHTMLTags(m[3]))
				if !strings.Contains(strings.ToLower(name+" "+text), q) {
					continue
				}
				title := text
				if title == "" {
					title = name
				}
				hits = append(hits, searchHit{
					PageID:   p.ID,
					Title:    title,
					Snippet:  kind + " · " + title,
					Fragment: name,
				})
				fnHits++
				if len(hits) >= limit {
					return hits
				}
			}
			if fnHits == 0 && (strings.Contains(strings.ToLower(p.Title), q) || strings.Contains(strings.ToLower(p.Markdown), q)) {
				hits = append(hits, searchHit{PageID: p.ID, Title: p.Title, Snippet: p.Title})
				if len(hits) >= limit {
					return hits
				}
			}
			continue
		}
		body := strings.ToLower(p.Markdown)
		idx := strings.Index(body, q)
		if idx < 0 {
			if strings.Contains(strings.ToLower(p.Title), q) {
				hits = append(hits, searchHit{PageID: p.ID, Title: p.Title, Snippet: p.Title})
			}
			continue
		}
		start := idx - 40
		if start < 0 {
			start = 0
		}
		end := idx + len(q) + 60
		if end > len(p.Markdown) {
			end = len(p.Markdown)
		}
		snip := strings.ReplaceAll(p.Markdown[start:end], "\n", " ")
		hits = append(hits, searchHit{PageID: p.ID, Title: p.Title, Snippet: snip})
		if len(hits) >= limit {
			break
		}
	}
	return hits
}
