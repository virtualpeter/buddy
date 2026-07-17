package serve

import (
	"bytes"
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

func extractTOC(markdown string) []tocEntry {
	var out []tocEntry
	for _, m := range headingRE.FindAllStringSubmatch(markdown, -1) {
		level := len(m[1])
		text := strings.TrimSpace(m[2])
		text = strings.Trim(text, "#")
		text = strings.TrimSpace(text)
		id := slugify(text)
		out = append(out, tocEntry{Level: level, ID: id, Text: text})
	}
	return out
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
	PageID string `json:"pageId"`
	Title  string `json:"title"`
	Snippet string `json:"snippet"`
}

func searchPages(pages []DocPage, q string, limit int) []searchHit {
	q = strings.TrimSpace(strings.ToLower(q))
	if q == "" || limit <= 0 {
		return nil
	}
	var hits []searchHit
	for _, p := range pages {
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
