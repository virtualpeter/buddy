package serve

import (
	"net/http"
)

func serveNotebookHTML(w http.ResponseWriter, _ *http.Request, defaultCred string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(notebookHTML))
	_ = defaultCred
}

const notebookHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>buddy notebook</title>
<style>
:root {
  --bg: #0f1419;
  --panel: #1a222c;
  --ink: #e7ecf1;
  --muted: #8b9aab;
  --accent: #3d9c8f;
  --err: #f07178;
  --ok: #c3e88d;
  --mono: "IBM Plex Mono", "SF Mono", ui-monospace, monospace;
  --drawer: 20rem;
}
* { box-sizing: border-box; }
body {
  margin: 0;
  background: linear-gradient(160deg, #152028 0%, var(--bg) 45%, #121820 100%);
  color: var(--ink);
  font-family: var(--mono);
  min-height: 100vh;
}
header {
  display: flex; flex-wrap: wrap; gap: .75rem; align-items: center;
  padding: 1rem 1.25rem; border-bottom: 1px solid #2a3542;
  background: color-mix(in srgb, var(--panel) 80%, transparent);
  position: sticky; top: 0; z-index: 5;
}
.brand { font-weight: 700; font-size: 1.2rem; letter-spacing: -0.03em; }
.brand a { color: var(--ink); text-decoration: none; }
.spacer { flex: 1; }
select, button, input[type=search] {
  font: inherit; font-size: .85rem;
  background: var(--panel); color: var(--ink);
  border: 1px solid #334155; border-radius: 4px; padding: .35rem .6rem;
}
button { cursor: pointer; }
button.primary { background: var(--accent); color: #042f2e; border-color: transparent; font-weight: 600; }
button.active { border-color: var(--accent); color: var(--accent); }
.layout { display: flex; align-items: stretch; min-height: calc(100vh - 4rem); }
.main { flex: 1; min-width: 0; }
#cells { padding: 1rem 1.25rem 3rem; max-width: 56rem; margin: 0 auto; }
.cell {
  border: 1px solid #2a3542; border-radius: 6px; margin-bottom: .9rem;
  background: var(--panel); overflow: hidden;
}
.cell-bar {
  display: flex; gap: .5rem; align-items: center;
  padding: .35rem .6rem; border-bottom: 1px solid #2a3542; font-size: .75rem; color: var(--muted);
}
textarea {
  width: 100%; min-height: 5.5rem; resize: vertical;
  border: 0; padding: .75rem; background: #11181f; color: var(--ink);
  font: inherit; line-height: 1.45;
}
.out {
  padding: .5rem .75rem; border-top: 1px solid #2a3542;
  font-size: .85rem; min-height: 1.5rem;
  display: flex; flex-direction: column; gap: .45rem;
}
.out .stream, .out .result, .out .error {
  white-space: pre-wrap; word-break: break-word;
  padding: .4rem .5rem; border-radius: 4px;
  border-left: 3px solid transparent;
}
.out .stream-label, .out .result-label, .out .error-label {
  display: block; font-size: .7rem; letter-spacing: .04em;
  text-transform: uppercase; margin-bottom: .2rem; opacity: .85;
}
.out .stdout { color: var(--ink); background: #0d1218; border-left-color: #64748b; }
.out .stderr { color: #fde68a; background: #1a1510; border-left-color: #fbbf24; }
.out .result { color: var(--ok); background: #0f1a12; border-left-color: var(--ok); }
.out .error { color: var(--err); background: #1a1012; border-left-color: var(--err); }
.out .empty { color: #64748b; }
.drawer {
  width: 0; overflow: hidden; border-left: 0 solid #2a3542;
  background: color-mix(in srgb, var(--panel) 92%, #000);
  transition: width .15s ease;
  display: flex; flex-direction: column;
}
.layout.libs-open .drawer {
  width: var(--drawer); border-left-width: 1px;
}
.drawer-inner {
  width: var(--drawer); height: 100%;
  display: flex; flex-direction: column; min-height: 0;
}
.drawer-head {
  padding: .75rem; border-bottom: 1px solid #2a3542;
  display: flex; flex-direction: column; gap: .5rem;
}
.drawer-head h2 { margin: 0; font-size: .95rem; font-weight: 600; }
.drawer-body { flex: 1; overflow: auto; padding: .5rem .6rem 1.5rem; font-size: .78rem; }
.lib-group { margin-bottom: .75rem; }
.lib-group > summary {
  cursor: pointer; color: var(--muted); font-size: .72rem;
  text-transform: uppercase; letter-spacing: .04em; list-style: none;
  padding: .25rem 0;
}
.lib-group > summary::-webkit-details-marker { display: none; }
.lib-group[open] > summary { color: var(--accent); }
.fn {
  display: block; width: 100%; text-align: left;
  background: transparent; border: 0; border-radius: 4px;
  padding: .35rem .4rem; color: var(--ink); cursor: pointer;
  font: inherit; font-size: .78rem;
}
.fn:hover { background: #243040; }
.fn .name { color: var(--ok); }
.fn .sum { display: block; color: var(--muted); font-size: .7rem; margin-top: .15rem; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.drawer-empty { color: var(--muted); padding: .5rem; }
@media (max-width: 900px) {
  .layout.libs-open .drawer { position: fixed; right: 0; top: 0; bottom: 0; z-index: 10; width: min(var(--drawer), 90vw); border-left-width: 1px; }
}
</style>
</head>
<body>
<header>
  <div class="brand"><a href="/docs/">buddy</a> · notebook</div>
  <label>cred <select id="cred"></select></label>
  <button id="runAll" class="primary">Run all</button>
  <button id="addCell">Add cell</button>
  <button id="reset">Reset kernel</button>
  <button id="toggleLibs">Libraries</button>
  <div class="spacer"></div>
  <a href="/docs/" style="color:var(--muted);font-size:.85rem">Docs</a>
</header>
<div class="layout" id="layout">
  <div class="main"><div id="cells"></div></div>
  <aside class="drawer" id="drawer" aria-label="Libraries">
    <div class="drawer-inner">
      <div class="drawer-head">
        <h2>Libraries</h2>
        <input type="search" id="libSearch" placeholder="Filter…" autocomplete="off">
      </div>
      <div class="drawer-body" id="libBody"><div class="drawer-empty">Loading…</div></div>
    </div>
  </aside>
</div>
<script>
let sessionId = null;
let cells = [];
let focusedTa = null;
let libCache = {};

async function ensureSession() {
  if (sessionId) return sessionId;
  const cred = document.getElementById('cred').value;
  const r = await fetch('/api/session', {
    method: 'POST',
    headers: {'Content-Type':'application/json'},
    body: JSON.stringify({cred})
  });
  const data = await r.json();
  if (!r.ok) throw new Error(data.error || 'session failed');
  sessionId = data.id;
  return sessionId;
}

async function loadCreds() {
  const r = await fetch('/api/creds');
  const data = await r.json();
  const sel = document.getElementById('cred');
  sel.innerHTML = '';
  (data.creds || []).forEach(c => {
    const o = document.createElement('option');
    o.value = c; o.textContent = c;
    if (c === data.active) o.selected = true;
    sel.appendChild(o);
  });
  if (!sel.options.length) {
    const o = document.createElement('option');
    o.value = data.active || 'default'; o.textContent = o.value;
    sel.appendChild(o);
  }
  sel.addEventListener('change', async () => {
    if (sessionId) {
      await fetch('/api/session/' + sessionId, {method:'DELETE'});
      sessionId = null;
    }
  });
}

function renderOut(el, res) {
  const parts = [];
  const block = (cls, label, body) =>
    '<div class="' + cls + '"><span class="' + cls.split(' ')[0] + '-label">' + label + '</span>' + esc(body) + '</div>';
  if (res.stdout) parts.push(block('stream stdout', 'stdout', res.stdout.replace(/\n$/, '')));
  if (res.stderr) parts.push(block('stream stderr', 'stderr', res.stderr.replace(/\n$/, '')));
  if (res.result) parts.push(block('result', 'Out', res.result));
  if (res.error) parts.push(block('error', 'error', res.error));
  el.innerHTML = parts.join('') || '<span class="empty">(no output)</span>';
}
function esc(s) {
  return String(s).replace(/[&<>]/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;'}[c]));
}

function addCell(src='') {
  const id = 'c' + (cells.length + 1) + '-' + Math.random().toString(36).slice(2,7);
  const wrap = document.createElement('div');
  wrap.className = 'cell';
  wrap.innerHTML =
    '<div class="cell-bar"><span>In</span>' +
    '<button class="run">Run</button>' +
    '<button class="rm">Remove</button></div>' +
    '<textarea spellcheck="false"></textarea><div class="out"></div>';
  const ta = wrap.querySelector('textarea');
  ta.value = src;
  ta.addEventListener('focus', () => { focusedTa = ta; });
  wrap.querySelector('.run').onclick = () => runCell(wrap, id);
  wrap.querySelector('.rm').onclick = () => { wrap.remove(); cells = cells.filter(c => c.el !== wrap); };
  document.getElementById('cells').appendChild(wrap);
  cells.push({id, el: wrap});
  return wrap;
}

async function runCell(wrap, id) {
  const src = wrap.querySelector('textarea').value;
  const out = wrap.querySelector('.out');
  out.textContent = '…';
  try {
    await ensureSession();
    const r = await fetch('/api/session/' + sessionId + '/eval', {
      method: 'POST',
      headers: {'Content-Type':'application/json'},
      body: JSON.stringify({source: src, cell: id})
    });
    const res = await r.json();
    renderOut(out, res);
  } catch (e) {
    renderOut(out, {error: String(e)});
  }
}

function insertAtCursor(text) {
  let ta = focusedTa;
  if (!ta || !document.body.contains(ta)) {
    const last = cells[cells.length - 1];
    ta = last ? last.el.querySelector('textarea') : null;
  }
  if (!ta) {
    addCell(text);
    return;
  }
  const start = ta.selectionStart ?? ta.value.length;
  const end = ta.selectionEnd ?? start;
  const before = ta.value.slice(0, start);
  const after = ta.value.slice(end);
  const pad = before && !before.endsWith('\n') ? '\n' : '';
  ta.value = before + pad + text + (after.startsWith('\n') || !after ? '' : '\n') + after;
  ta.focus();
  const pos = (before + pad + text).length;
  ta.selectionStart = ta.selectionEnd = pos;
  focusedTa = ta;
}

async function fetchLibrary(id) {
  if (libCache[id]) return libCache[id];
  const r = await fetch('/api/libraries/' + encodeURIComponent(id));
  const data = await r.json();
  if (!r.ok) throw new Error(data.error || 'library failed');
  libCache[id] = data;
  return data;
}

function fnMatches(fn, q) {
  if (!q) return true;
  const hay = (fn.name + ' ' + (fn.signature || '') + ' ' + (fn.summary || '') + ' ' + (fn.section || '')).toLowerCase();
  return hay.includes(q);
}

async function renderLibraries() {
  const body = document.getElementById('libBody');
  const q = (document.getElementById('libSearch').value || '').trim().toLowerCase();
  try {
    const listR = await fetch('/api/libraries');
    const list = await listR.json();
    if (!listR.ok) throw new Error(list.error || 'list failed');
    const parts = [];
    for (const item of list) {
      const detail = await fetchLibrary(item.id);
      if (detail.kind === 'builtins' && detail.sections) {
        for (const sec of detail.sections) {
          const fns = (sec.functions || []).filter(f => fnMatches(f, q));
          if (!fns.length) continue;
          parts.push(renderGroup('Builtins · ' + sec.title, fns));
        }
      } else {
        const fns = (detail.functions || []).filter(f => fnMatches(f, q));
        if (!fns.length) continue;
        const title = item.kind === 'stdlib' ? 'stdlib / ' + item.title : item.title;
        parts.push(renderGroup(title, fns));
      }
    }
    body.innerHTML = parts.join('') || '<div class="drawer-empty">No matches</div>';
    body.querySelectorAll('.fn').forEach(btn => {
      btn.onclick = () => insertAtCursor(btn.dataset.insert || btn.dataset.name + '()');
    });
  } catch (e) {
    body.innerHTML = '<div class="drawer-empty">' + esc(String(e)) + '</div>';
  }
}

function renderGroup(title, fns) {
  const items = fns.map(f => {
    const insert = f.insert || (f.name + '()');
    const sum = f.summary ? '<span class="sum">' + esc(f.summary) + '</span>' : '';
    return '<button type="button" class="fn" data-insert="' + escAttr(insert) + '" data-name="' + escAttr(f.name) + '">' +
      '<span class="name">' + esc(f.name) + '</span>' + sum + '</button>';
  }).join('');
  return '<details class="lib-group" open><summary>' + esc(title) + '</summary>' + items + '</details>';
}

function escAttr(s) {
  return String(s).replace(/&/g,'&amp;').replace(/"/g,'&quot;').replace(/</g,'&lt;');
}

document.getElementById('addCell').onclick = () => addCell('');
document.getElementById('runAll').onclick = async () => {
  for (const c of cells) await runCell(c.el, c.id);
};
document.getElementById('reset').onclick = async () => {
  if (sessionId) {
    await fetch('/api/session/' + sessionId + '/reset', {method:'POST'});
  }
  sessionId = null;
  document.querySelectorAll('.out').forEach(o => o.textContent = '');
};
document.getElementById('toggleLibs').onclick = (e) => {
  const layout = document.getElementById('layout');
  const open = layout.classList.toggle('libs-open');
  e.target.classList.toggle('active', open);
  if (open) renderLibraries();
};
document.getElementById('libSearch').addEventListener('input', () => {
  if (document.getElementById('layout').classList.contains('libs-open')) renderLibraries();
});

(async () => {
  await loadCreds();
  addCell('print("buddy notebook ready")\nerror("stderr demo")\n1 + 1');
})();
</script>
</body>
</html>
`
