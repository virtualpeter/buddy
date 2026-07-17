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
}
.brand { font-weight: 700; font-size: 1.2rem; letter-spacing: -0.03em; }
.brand a { color: var(--ink); text-decoration: none; }
.spacer { flex: 1; }
select, button {
  font: inherit; font-size: .85rem;
  background: var(--panel); color: var(--ink);
  border: 1px solid #334155; border-radius: 4px; padding: .35rem .6rem;
}
button { cursor: pointer; }
button.primary { background: var(--accent); color: #042f2e; border-color: transparent; font-weight: 600; }
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
</style>
</head>
<body>
<header>
  <div class="brand"><a href="/docs/">buddy</a> · notebook</div>
  <label>cred <select id="cred"></select></label>
  <button id="runAll" class="primary">Run all</button>
  <button id="addCell">Add cell</button>
  <button id="reset">Reset kernel</button>
  <div class="spacer"></div>
  <a href="/docs/" style="color:var(--muted);font-size:.85rem">Docs</a>
</header>
<div id="cells"></div>
<script>
let sessionId = null;
let cells = [];

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
  wrap.querySelector('textarea').value = src;
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

(async () => {
  await loadCreds();
  addCell('print("buddy notebook ready")\nerror("stderr demo")\n1 + 1');
})();
</script>
</body>
</html>
`
