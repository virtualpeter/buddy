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
	groups := docsNavGroups(pages)
	applyNavOpen(groups, p.ID, pageSection(*p))
	data := docsView{
		Groups:        groups,
		ActiveID:      p.ID,
		ActiveSection: pageSection(*p),
		Title:         p.Title,
		Body:          template.HTML(htmlBody),
		TOC:           toc,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := docsTmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type docsView struct {
	Groups        []docsNavGroup
	ActiveID      string
	ActiveSection string
	Title         string
	Body          template.HTML
	TOC           []tocEntry
}

type docsNavGroup struct {
	Name     string
	Pages    []DocPage
	Children []docsNavGroup
	Open     bool
}

func (g docsNavGroup) Count() int {
	n := len(g.Pages)
	for _, c := range g.Children {
		n += c.Count()
	}
	return n
}

func pageSection(p DocPage) string {
	if p.Section == "" {
		return "Docs"
	}
	return p.Section
}

func docsNavGroups(pages []DocPage) []docsNavGroup {
	secs := docpages.NavSections(pages)
	byName := make(map[string]*docsNavGroup, len(secs))
	for _, sec := range secs {
		byName[sec] = &docsNavGroup{Name: sec}
	}
	for _, p := range pages {
		sec := pageSection(p)
		if g := byName[sec]; g != nil {
			g.Pages = append(g.Pages, p)
		}
	}
	nested := make(map[string]bool)
	for _, sec := range secs {
		parent := docpages.NavParent(sec)
		if parent == "" {
			continue
		}
		child, parentG := byName[sec], byName[parent]
		if child == nil || parentG == nil {
			continue
		}
		parentG.Children = append(parentG.Children, *child)
		nested[sec] = true
	}
	out := make([]docsNavGroup, 0, len(secs))
	for _, sec := range secs {
		if nested[sec] {
			continue
		}
		out = append(out, *byName[sec])
	}
	return out
}

func applyNavOpen(groups []docsNavGroup, activeID, activeSec string) {
	var walk func(*docsNavGroup)
	walk = func(g *docsNavGroup) {
		if g.Name == "Start here" || g.Name == activeSec {
			g.Open = true
		}
		for _, p := range g.Pages {
			if p.ID == activeID {
				g.Open = true
				break
			}
		}
		for i := range g.Children {
			walk(&g.Children[i])
			if g.Children[i].Open {
				g.Open = true
			}
		}
	}
	for i := range groups {
		walk(&groups[i])
	}
}

var docsTmpl = template.Must(template.New("docs").Funcs(template.FuncMap{
	"indent":       func(level int) int { return (level - 1) * 10 },
	"chromeHead":   func() template.HTML { return template.HTML(chromeHeadTags()) },
	"chromeHeader": func() template.HTML { return template.HTML(chromeHeader("docs", "", "", "")) },
	"chromeToggle": func() template.HTML { return template.HTML(chromeToggleScript()) },
}).Parse(docsTmplSrc))

const docsTmplSrc = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>buddy docs</title>
{{chromeHead}}
<style>
body {
  margin: 0;
  color: var(--ink);
  background: var(--bg-grad);
  font-family: var(--sans);
  line-height: 1.55;
  display: flex;
  flex-direction: column;
  min-height: 100vh;
}
a { color: var(--accent); text-decoration: none; }
a:hover { text-decoration: underline; }
.layout {
  display: grid;
  grid-template-columns: 240px minmax(0,1fr) 200px;
  flex: 1;
  min-height: 0;
}
@media (max-width: 960px) {
  .layout { grid-template-columns: 1fr; }
  .toc, .side-nav { position: static !important; height: auto !important; }
}
.side-nav, .toc {
  position: sticky; top: var(--app-header-h); height: calc(100vh - var(--app-header-h)); overflow: auto;
  padding: 1.25rem 1rem;
  border-right: 1px solid var(--line);
  background: color-mix(in srgb, var(--panel) 88%, transparent);
  backdrop-filter: blur(6px);
  font-family: var(--mono);
}
.toc { border-right: 0; border-left: 1px solid var(--line); font-size: .88rem; }
.side-nav a, .toc a { display: block; padding: .28rem 0; color: var(--ink); }
.side-nav a.active { color: var(--accent); font-weight: 600; }
.section { margin: 1.15rem 0 .35rem; font-family: var(--mono); font-size: .72rem; text-transform: uppercase; letter-spacing: .06em; color: var(--muted); }
.section:first-of-type { margin-top: .25rem; }
.nav-group { margin: 0 0 .15rem; }
.nav-group > summary {
  cursor: pointer;
  list-style: none;
  display: flex;
  align-items: center;
  gap: .4rem;
  margin: .85rem 0 .2rem;
  padding: .15rem 0;
  font-family: var(--mono);
  font-size: .72rem;
  text-transform: uppercase;
  letter-spacing: .06em;
  color: var(--muted);
  user-select: none;
}
.nav-group:first-of-type > summary { margin-top: .15rem; }
.nav-group > summary::-webkit-details-marker { display: none; }
.nav-group > summary::before {
  content: "";
  width: 0; height: 0;
  border-left: .32em solid currentColor;
  border-top: .22em solid transparent;
  border-bottom: .22em solid transparent;
  opacity: .7;
  flex-shrink: 0;
  transition: transform .12s ease;
}
.nav-group[open] > summary::before { transform: rotate(90deg); }
.nav-group[open] > summary { color: var(--accent); }
.nav-group > a { padding-left: .85rem; }
.nav-group .nav-group {
  margin: .1rem 0 .2rem;
}
.nav-group .nav-group > summary {
  margin-top: .4rem;
  font-size: .68rem;
}
.nav-group .nav-group > a { padding-left: 1.15rem; }
.nav-count {
  margin-left: auto;
  font-size: .65rem;
  letter-spacing: 0;
  text-transform: none;
  font-weight: 500;
  opacity: .75;
}
main { padding: 1.5rem 2rem 3rem; max-width: 52rem; }
main > h1:first-child { margin-top: 0; }
h1,h2,h3 { line-height: 1.25; }
main h2 { margin: 1.75rem 0 .75rem; font-size: 1.15rem; }
main h3 { margin: 1.35rem 0 .55rem; font-size: 1.02rem; }
main p { margin: 0 0 .9rem; max-width: 42rem; }
main ul, main ol {
  margin: 0 0 1.15rem;
  padding-left: 1.15rem;
  max-width: 44rem;
}
main li {
  margin: 0 0 .65rem;
  line-height: 1.65;
  padding-left: .15rem;
}
main li:last-child { margin-bottom: 0; }
main li > p { margin: 0; }
pre, code { font-family: var(--mono); font-size: .88em; }
main :not(pre) > code {
  font-size: .84em;
  background: color-mix(in srgb, var(--code-bg) 82%, var(--panel));
  color: var(--code-fg);
  padding: .12em .38em;
  border-radius: 3px;
  white-space: break-spaces;
}
main table {
  width: 100%;
  max-width: 52rem;
  border-collapse: collapse;
  font-size: .92rem;
  margin: 0 0 1.25rem;
}
main th, main td {
  text-align: left;
  vertical-align: top;
  padding: .55rem .65rem;
  border-bottom: 1px solid var(--line);
  line-height: 1.5;
}
main th {
  font-family: var(--mono);
  font-size: .72rem;
  text-transform: uppercase;
  letter-spacing: .04em;
  color: var(--muted);
  font-weight: 600;
}
main td:first-child { white-space: nowrap; font-weight: 600; width: 7.5rem; }
main td:nth-child(2), main td:nth-child(3) { width: 28%; }
main td code { white-space: break-spaces; }
pre { background: var(--code-bg); color: var(--code-fg); padding: 1rem; overflow: auto; border-radius: 4px; }
.code-try { position: relative; margin: 0 0 1.15rem; }
.code-try > pre { margin: 0; padding-top: 2.1rem; }
.code-try .try-it {
  position: absolute; top: .45rem; right: .45rem; z-index: 1;
  font: inherit; font-family: var(--mono); font-size: .72rem;
  padding: .25rem .55rem; border-radius: 4px; cursor: pointer;
  background: var(--accent); color: var(--accent-ink); border: 0; font-weight: 600;
}
.code-try .try-it:hover { filter: brightness(1.08); }
.search, .page-filter {
  width: 100%; padding: .45rem .6rem; border: 1px solid var(--line); border-radius: 4px;
  font: inherit; font-family: var(--mono); margin: 0 0 1rem;
  background: var(--panel); color: var(--ink);
}
.page-filter { max-width: 28rem; margin: 0 0 1.25rem; }
.hits { font-size: .9rem; margin-bottom: 1rem; }
.hits div { padding: .35rem 0; border-bottom: 1px solid var(--line); }
.hits .snip { color: var(--muted); font-size: .82rem; }
.fn-hidden { display: none !important; }
</style>
</head>
<body>
{{chromeHeader}}
<div class="layout">
  <aside class="side-nav">
    <input class="search" id="q" type="search" placeholder="Search docs…" autocomplete="off">
    <div class="hits" id="hits"></div>
    {{range .Groups}}
    <details class="nav-group" data-sec="{{.Name}}"{{if .Open}} open{{end}}>
      <summary>{{.Name}} <span class="nav-count">{{.Count}}</span></summary>
      {{range .Pages}}
      <a href="/docs/{{.ID}}" {{if eq .ID $.ActiveID}}class="active"{{end}}>{{.Title}}</a>
      {{end}}
      {{range .Children}}
      <details class="nav-group" data-sec="{{.Name}}"{{if .Open}} open{{end}}>
        <summary>{{.Name}} <span class="nav-count">{{.Count}}</span></summary>
        {{range .Pages}}
        <a href="/docs/{{.ID}}" {{if eq .ID $.ActiveID}}class="active"{{end}}>{{.Title}}</a>
        {{end}}
      </details>
      {{end}}
    </details>
    {{end}}
  </aside>
  <script>
  (function(){
    var KEY = 'buddy-serve-docs-nav';
    function load(){ try { return JSON.parse(localStorage.getItem(KEY) || '{}'); } catch (e) { return {}; } }
    function save(s){ try { localStorage.setItem(KEY, JSON.stringify(s)); } catch (e) {} }
    var state = load();
    document.querySelectorAll('.nav-group').forEach(function(d){
      var sec = d.getAttribute('data-sec') || '';
      var hasActive = !!d.querySelector('a.active');
      var want;
      if (hasActive) want = true;
      else if (Object.prototype.hasOwnProperty.call(state, sec)) want = !!state[sec];
      else want = sec === 'Start here';
      if (d.open !== want) d.open = want;
      d.addEventListener('toggle', function(){
        var s = load();
        s[sec] = d.open;
        save(s);
      });
    });
  })();
  </script>
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
try { sessionStorage.setItem('buddy-serve-docs-last', '{{.ActiveID}}'); } catch (e) {}
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
{{chromeToggle}}
<script>
(function(){
  const TRY_KEY = 'buddy-serve-ide-try';
  function isTryableScript(text) {
    const t = (text || '').trim();
    if (t.length < 8) return false;
    const first = (t.split('\n').find(l => l.trim()) || '').trim();
    if (/^(buddy\s|\$\s|curl\s|make\s|npm\s|go\s)/.test(first) && !/\bdef\s/.test(first)) return false;
    if (first.startsWith('{') || first.startsWith('[')) return false;
    return /\bdef\s+\w+\s*\(/.test(t)
      || /\bload\s*\(/.test(t)
      || /#!\/usr\/bin\/env\s+buddy/.test(t)
      || /\b(plan_change|run_change|change_batch|prefs_payload|pppc_fda)\s*\(/.test(t);
  }
  function tryInIde(source) {
    try { localStorage.setItem(TRY_KEY, JSON.stringify({ source: source, t: Date.now() })); } catch (e) {}
    const msg = { type: 'buddy-ide-try', source: source };
    if (typeof buddyServeOpenOrFocus === 'function') {
      buddyServeOpenOrFocus('buddy-ide', '/ide', msg);
    } else {
      window.open('/ide', 'buddy-ide');
    }
  }
  document.querySelectorAll('main pre').forEach(pre => {
    if (pre.closest('.code-try')) return;
    const src = pre.textContent || '';
    if (!isTryableScript(src)) return;
    const wrap = document.createElement('div');
    wrap.className = 'code-try';
    pre.parentNode.insertBefore(wrap, pre);
    wrap.appendChild(pre);
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'try-it';
    btn.textContent = 'Try it';
    btn.title = 'Open this example in the IDE';
    btn.addEventListener('click', () => tryInIde(src.replace(/\n$/, '')));
    wrap.appendChild(btn);
  });
})();
</script>
</body>
</html>
`
