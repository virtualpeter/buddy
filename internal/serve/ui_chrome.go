package serve

import (
	"strings"
)

// Shared top chrome + light/dark theme for docs, notebook, and IDE.

const chromeCSS = `
:root, html[data-theme="dark"] {
  --bg: #0f1419;
  --panel: #1a222c;
  --ink: #e7ecf1;
  --muted: #8b9aab;
  --accent: #3d9c8f;
  --accent-ink: #042f2e;
  --err: #f07178;
  --ok: #c3e88d;
  --line: #2a3542;
  --border: #334155;
  --hint: #64748b;
  --code-bg: #11181f;
  --code-fg: #f5f5f4;
  --surface-2: #243040;
  --runbar-bg: #121920;
  --stream-bg: #0d1218;
  --stderr-bg: #1a1510;
  --result-bg: #0f1a12;
  --error-bg: #1a1012;
  --detail-bg: #1a2330;
  --bg-grad: linear-gradient(160deg, #152028 0%, var(--bg) 45%, #121820 100%);
  --mono: "IBM Plex Mono", "SF Mono", ui-monospace, monospace;
  --sans: "Source Serif 4", "Iowan Old Style", Georgia, serif;
  --app-header-h: 3.25rem;
}
html[data-theme="light"] {
  --bg: #f6f3ee;
  --panel: #fffdf9;
  --ink: #1c1917;
  --muted: #78716c;
  --accent: #0f766e;
  --accent-ink: #042f2e;
  --err: #b91c1c;
  --ok: #15803d;
  --line: #e7e5e4;
  --border: #d6d3d1;
  --hint: #a8a29e;
  --code-bg: #1c1917;
  --code-fg: #f5f5f4;
  --surface-2: #f5f0e8;
  --runbar-bg: #f0ebe3;
  --stream-bg: #f5f5f4;
  --stderr-bg: #fff7ed;
  --result-bg: #f0fdf4;
  --error-bg: #fef2f2;
  --detail-bg: #f5f0e8;
  --bg-grad:
    radial-gradient(900px 400px at 10% -10%, #d9f3ef 0%, transparent 55%),
    radial-gradient(700px 360px at 100% 0%, #fde7d4 0%, transparent 50%),
    var(--bg);
}
* { box-sizing: border-box; }
.app-header {
  display: flex; flex-wrap: wrap; gap: .65rem; align-items: center;
  padding: .65rem 1rem; min-height: var(--app-header-h);
  border-bottom: 1px solid var(--line);
  background: color-mix(in srgb, var(--panel) 80%, transparent);
  backdrop-filter: blur(6px);
  position: sticky; top: 0; z-index: 20;
  font-family: var(--mono);
}
.app-header .brand {
  font-weight: 700; font-size: 1.15rem; letter-spacing: -0.03em;
  margin: 0;
}
.app-header .brand a { color: var(--ink); text-decoration: none; }
.app-nav {
  display: flex; gap: .65rem; align-items: center;
  font-size: .85rem;
}
.app-nav a {
  color: var(--muted); text-decoration: none;
}
.app-nav a:hover { color: var(--accent); text-decoration: none; }
.app-nav a.active { color: var(--accent); font-weight: 600; }
.app-header .spacer { flex: 1; min-width: .5rem; }
.app-header label:not(.theme-toggle):not(.chk) {
  display: inline-flex; align-items: center; gap: .35rem;
  color: var(--muted); font-size: .8rem;
}
.app-header select, .app-header button, .app-header input[type=text], .app-header input[type=search] {
  font: inherit; font-size: .8rem;
  background: var(--panel); color: var(--ink);
  border: 1px solid var(--border); border-radius: 4px; padding: .3rem .55rem;
}
.app-header button { cursor: pointer; }
.app-header button.primary {
  background: var(--accent); color: var(--accent-ink);
  border-color: transparent; font-weight: 600;
}
.app-header button.active { border-color: var(--accent); color: var(--accent); }
.app-header button:disabled { opacity: .5; cursor: default; }
.app-header #path {
  color: var(--muted); font-size: .75rem; max-width: 28rem;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.app-header #path.dirty::after { content: " •"; color: var(--accent); }
.app-header #status { font-size: .75rem; color: var(--muted); }
.theme-toggle {
  display: inline-flex; align-items: center; gap: .35rem;
  color: var(--muted); font-size: .8rem; cursor: pointer; user-select: none;
  margin: 0;
}
.theme-toggle input { accent-color: var(--accent); cursor: pointer; }
`

// Theme resolution: explicit localStorage dark|light wins; otherwise follow
// prefers-color-scheme. Checkbox sets an explicit override.
const chromeThemeBootJS = `(function(){
  function sys(){try{return window.matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light';}catch(e){return 'dark';}}
  function resolve(){
    try{var t=localStorage.getItem('buddy-serve-theme');if(t==='light'||t==='dark')return t;}catch(e){}
    return sys();
  }
  document.documentElement.setAttribute('data-theme',resolve());
})();`

const chromeThemeToggleJS = `(function(){
  var cb=document.getElementById('theme-dark');
  if(!cb) return;
  function sys(){try{return window.matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light';}catch(e){return 'dark';}}
  function stored(){try{var t=localStorage.getItem('buddy-serve-theme');if(t==='light'||t==='dark')return t;}catch(e){} return null;}
  function apply(t){document.documentElement.setAttribute('data-theme',t);cb.checked=t!=='light';}
  apply(stored()||sys());
  cb.addEventListener('change',function(){
    var t=cb.checked?'dark':'light';
    apply(t);
    try{localStorage.setItem('buddy-serve-theme',t);}catch(e){}
  });
  try{
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change',function(){
      if(stored()) return;
      apply(sys());
    });
  }catch(e){}
})();`

// chromeHeader builds the shared sticky top bar.
// active is "docs", "notebook", or "ide".
// sessionHTML typically holds tenant/user selects; actionsHTML page controls;
// runHTML the primary run button (placed before the dark toggle).
func chromeHeader(active, sessionHTML, actionsHTML, runHTML string) string {
	nav := func(id, href, label string) string {
		cls := ""
		if id == active {
			cls = ` class="active"`
		}
		return `<a href="` + href + `"` + cls + `>` + label + `</a>`
	}
	var b strings.Builder
	b.WriteString(`<header class="app-header">`)
	b.WriteString(`<div class="brand"><a href="/docs/">buddy</a></div>`)
	b.WriteString(`<nav class="app-nav">`)
	b.WriteString(nav("docs", "/docs/", "docs"))
	b.WriteString(nav("notebook", "/notebook", "notebook"))
	b.WriteString(nav("ide", "/ide", "ide"))
	b.WriteString(`</nav>`)
	if sessionHTML != "" {
		b.WriteString(sessionHTML)
	}
	if actionsHTML != "" {
		b.WriteString(actionsHTML)
	}
	b.WriteString(`<div class="spacer"></div>`)
	if runHTML != "" {
		b.WriteString(runHTML)
	}
	b.WriteString(`<label class="theme-toggle"><input type="checkbox" id="theme-dark"> dark</label>`)
	b.WriteString(`</header>`)
	return b.String()
}

func chromeHeadTags() string {
	return `<script>` + chromeThemeBootJS + `</script>` + "\n<style>\n" + chromeCSS + "\n</style>\n"
}

func chromeToggleScript() string {
	return `<script>` + chromeThemeToggleJS + `</script>`
}
