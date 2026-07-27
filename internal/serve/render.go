package serve

import (
	"bytes"
	"path"
	"regexp"
	"strings"

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
		if p.ID == "builtins" {
			fnHits := 0
			for _, m := range builtinsFnHeadingRE.FindAllStringSubmatch(p.Markdown, -1) {
				name := m[1]
				if !strings.Contains(strings.ToLower(name), q) {
					continue
				}
				hits = append(hits, searchHit{
					PageID:   p.ID,
					Title:    name,
					Snippet:  "builtin · " + name,
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
