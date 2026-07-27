package serve

import (
	"html/template"
	"net/http"
	"path"
	"strings"

	"buddy/internal/docpages"

	"github.com/yuin/goldmark"
)

func serveDocsHTML(w http.ResponseWriter, r *http.Request, pages []DocPage, md goldmark.Markdown) {
	id := strings.TrimPrefix(r.URL.Path, "/docs/")
	id = strings.Trim(id, "/")
	if id == "" {
		id = "docs/index"
		if pageByID(pages, id) == nil && len(pages) > 0 {
			id = pages[0].ID
		}
	}
	// Tolerate legacy / filename-style URLs: /docs/docs/foo.md → docs/foo
	if strings.HasSuffix(id, ".md") {
		trimmed := strings.TrimSuffix(id, ".md")
		if pageByID(pages, trimmed) != nil {
			http.Redirect(w, r, "/docs/"+trimmed, http.StatusFound)
			return
		}
		// docs/change-batches.md vs change-batches.md
		if base := path.Base(trimmed); pageByID(pages, "docs/"+base) != nil {
			http.Redirect(w, r, "/docs/docs/"+base, http.StatusFound)
			return
		}
	}
	p := pageByID(pages, id)
	if p == nil {
		http.NotFound(w, r)
		return
	}
	src := rewriteDocLinks(p.Markdown, pages)
	htmlBody, err := renderMarkdown(md, src)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	toc := extractTOC(p.Markdown)
	data := docsView{
		Pages:    pages,
		Sections: docpages.NavSections(pages),
		ActiveID: p.ID,
		Title:    p.Title,
		Body:     template.HTML(htmlBody),
		TOC:      toc,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := docsTmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type docsView struct {
	Pages    []DocPage
	Sections []string
	ActiveID string
	Title    string
	Body     template.HTML
	TOC      []tocEntry
}

var docsTmpl = template.Must(template.New("docs").Funcs(template.FuncMap{
	"indent": func(level int) int { return (level - 1) * 10 },
	"sectionOf": func(p DocPage) string {
		if p.Section == "" {
			return "Docs"
		}
		return p.Section
	},
}).Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}} · buddy</title>
<style>
:root {
  --bg: #f6f3ee;
  --ink: #1c1917;
  --muted: #78716c;
  --accent: #0f766e;
  --panel: #fffdf9;
  --line: #e7e5e4;
  --mono: "IBM Plex Mono", "SF Mono", ui-monospace, monospace;
  --sans: "Source Serif 4", "Iowan Old Style", Georgia, serif;
}
* { box-sizing: border-box; }
body {
  margin: 0;
  color: var(--ink);
  background:
    radial-gradient(900px 400px at 10% -10%, #d9f3ef 0%, transparent 55%),
    radial-gradient(700px 360px at 100% 0%, #fde7d4 0%, transparent 50%),
    var(--bg);
  font-family: var(--sans);
  line-height: 1.55;
}
a { color: var(--accent); text-decoration: none; }
a:hover { text-decoration: underline; }
.layout { display: grid; grid-template-columns: 240px minmax(0,1fr) 200px; min-height: 100vh; }
@media (max-width: 960px) {
  .layout { grid-template-columns: 1fr; }
  .toc, .nav { position: static !important; height: auto !important; }
}
.brand {
  font-family: var(--mono);
  font-weight: 700;
  font-size: 1.35rem;
  letter-spacing: -0.03em;
  color: var(--ink);
  display: block;
  margin-bottom: .35rem;
}
.nav, .toc {
  position: sticky; top: 0; height: 100vh; overflow: auto;
  padding: 1.25rem 1rem;
  border-right: 1px solid var(--line);
  background: color-mix(in srgb, var(--panel) 88%, transparent);
  backdrop-filter: blur(6px);
}
.toc { border-right: 0; border-left: 1px solid var(--line); font-size: .88rem; }
.nav a, .toc a { display: block; padding: .28rem 0; color: var(--ink); }
.nav a.active { color: var(--accent); font-weight: 600; }
.section { margin: 1.15rem 0 .35rem; font-family: var(--mono); font-size: .72rem; text-transform: uppercase; letter-spacing: .06em; color: var(--muted); }
.section:first-of-type { margin-top: .25rem; }
main { padding: 1.5rem 2rem 3rem; max-width: 52rem; }
main > h1:first-child { margin-top: 0; }
h1,h2,h3 { line-height: 1.25; }
pre, code { font-family: var(--mono); font-size: .88em; }
pre { background: #1c1917; color: #f5f5f4; padding: 1rem; overflow: auto; border-radius: 4px; }
.search { width: 100%; padding: .45rem .6rem; border: 1px solid var(--line); border-radius: 4px; font: inherit; margin: .75rem 0 1rem; background: #fff; }
.page-filter { width: 100%; max-width: 28rem; padding: .45rem .6rem; border: 1px solid var(--line); border-radius: 4px; font: inherit; margin: 0 0 1.25rem; background: #fff; }
.hits { font-size: .9rem; margin-bottom: 1rem; }
.hits div { padding: .35rem 0; border-bottom: 1px solid var(--line); }
.hits .snip { color: var(--muted); font-size: .82rem; }
.topbar { display: flex; gap: 1rem; align-items: baseline; margin-bottom: 1rem; }
.topbar a.nb { font-family: var(--mono); font-size: .85rem; }
.fn-hidden { display: none !important; }
</style>
</head>
<body>
<div class="layout">
  <aside class="nav">
    <a class="brand" href="/docs/">buddy</a>
    <div class="topbar"><a class="nb" href="/notebook">Notebook</a> · <a class="nb" href="/ide">IDE</a></div>
    <input class="search" id="q" type="search" placeholder="Search docs…" autocomplete="off">
    <div class="hits" id="hits"></div>
    {{range $sec := .Sections}}
    <div class="section">{{$sec}}</div>
    {{range $.Pages}}
      {{if eq (sectionOf .) $sec}}
      <a href="/docs/{{.ID}}" {{if eq .ID $.ActiveID}}class="active"{{end}}>{{.Title}}</a>
      {{end}}
    {{end}}
    {{end}}
  </aside>
  <main>
    {{if eq .ActiveID "builtins"}}
    <input class="page-filter" id="pageFilter" type="search" placeholder="Filter builtins on this page…" autocomplete="off">
    {{end}}
    {{.Body}}
  </main>
  <aside class="toc">
    <div class="section">On this page</div>
    {{range .TOC}}
      <a href="#{{.ID}}" data-toc-id="{{.ID}}" style="padding-left: {{indent .Level}}px">{{.Text}}</a>
    {{end}}
  </aside>
</div>
<script>
const hits = document.getElementById('hits');
const q = document.getElementById('q');
let t = null;
q.addEventListener('input', () => {
  clearTimeout(t);
  t = setTimeout(async () => {
    const v = q.value.trim();
    if (!v) { hits.innerHTML = ''; return; }
    const r = await fetch('/api/search?q=' + encodeURIComponent(v));
    const data = await r.json();
    hits.innerHTML = data.map(h => {
      const href = '/docs/' + h.pageId + (h.fragment ? ('#' + h.fragment) : '');
      return '<div><a href="' + href + '">' + esc(h.title) + '</a><div class="snip">' +
        esc(h.snippet||'') + '</div></div>';
    }).join('');
  }, 150);
});
function esc(s) {
  return String(s).replace(/[<>&]/g, c => ({'<':'&lt;','>':'&gt;','&':'&amp;'}[c]));
}

const pageFilter = document.getElementById('pageFilter');
if (pageFilter) {
  const main = document.querySelector('main');
  const tocLinks = [...document.querySelectorAll('.toc a[data-toc-id]')];
  function setHidden(el, hide) {
    if (!el) return;
    el.classList.toggle('fn-hidden', !!hide);
  }
  pageFilter.addEventListener('input', () => {
    const q = (pageFilter.value || '').trim().toLowerCase();
    const els = [...main.children].filter(el => el !== pageFilter);
    // Function blocks
    for (let i = 0; i < els.length; i++) {
      const el = els[i];
      if (el.tagName !== 'H3' || !el.id) continue;
      const show = !q || (el.id + ' ' + el.textContent).toLowerCase().includes(q);
      setHidden(el, !show);
      let j = i + 1;
      while (j < els.length && els[j].tagName !== 'H2' && els[j].tagName !== 'H3') {
        setHidden(els[j], !show);
        j++;
      }
      const tocFn = tocLinks.find(a => a.getAttribute('data-toc-id') === el.id);
      if (tocFn) setHidden(tocFn, !show);
    }
    // Sections
    for (let i = 0; i < els.length; i++) {
      if (els[i].tagName !== 'H2') continue;
      let j = i + 1;
      let hasFn = false, hasVisible = false;
      while (j < els.length && els[j].tagName !== 'H2') {
        if (els[j].tagName === 'H3' && els[j].id) {
          hasFn = true;
          if (!els[j].classList.contains('fn-hidden')) hasVisible = true;
        }
        j++;
      }
      const showSec = !q || !hasFn || hasVisible || els[i].textContent.toLowerCase().includes(q);
      setHidden(els[i], !showSec);
      // Section intro (before first h3)
      let k = i + 1;
      while (k < els.length && els[k].tagName !== 'H2' && els[k].tagName !== 'H3') {
        setHidden(els[k], !showSec);
        k++;
      }
      const tocSec = tocLinks.find(a => a.getAttribute('data-toc-id') === els[i].id);
      if (tocSec) setHidden(tocSec, !showSec);
    }
  });
}
</script>
</body>
</html>
`))
