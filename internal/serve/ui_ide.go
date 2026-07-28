package serve

import (
	"net/http"
)

func serveIDEHTML(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(ideHTML))
}

var ideHTML = buildIDEHTML()

func buildIDEHTML() string {
	session := `
  <label>tenant <select id="tenant"></select></label>
  <label>user <select id="auth"></select></label>`
	actions := `
  <button type="button" id="btn-open">Open</button>
  <button type="button" id="btn-save">Save</button>
  <button type="button" id="btn-save-as">Save As</button>
  <span id="path" title="">(untitled)</span>
  <button type="button" id="btn-reset">Reset</button>
  <span id="status"></span>`
	run := `<button type="button" class="primary" id="btn-run">Run</button>`

	return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>buddy ide</title>
` + chromeHeadTags() + `<style>
html, body { height: 100%; }
body {
  margin: 0;
  background: var(--bg-grad);
  color: var(--ink);
  font-family: var(--mono);
  display: flex; flex-direction: column;
  overflow: hidden;
}
.runbar {
  display: flex; flex-wrap: wrap; gap: .5rem; align-items: center;
  padding: .45rem 1rem; border-bottom: 1px solid var(--line);
  background: var(--runbar-bg);
  font-family: var(--mono);
}
.runbar .hint { color: var(--hint); font-size: .7rem; margin-left: .15rem; }
.runbar .spacer { flex: 1; }
select, button, input[type=text], input[type=search], label {
  font: inherit; font-size: .8rem;
}
select, button, input[type=text], input[type=search] {
  background: var(--panel); color: var(--ink);
  border: 1px solid var(--border); border-radius: 4px; padding: .3rem .55rem;
}
button { cursor: pointer; }
button.primary { background: var(--accent); color: var(--accent-ink); border-color: transparent; font-weight: 600; }
button:disabled { opacity: .5; cursor: default; }
button.linkish {
  background: transparent; border: 0; color: var(--muted); padding: .2rem .35rem;
  text-decoration: underline; font-size: .72rem;
}
button.linkish:hover { color: var(--accent); }
label.chk { display: flex; align-items: center; gap: .35rem; color: var(--muted); }
#args {
  flex: 1; min-width: 10rem; max-width: 36rem;
  font-family: var(--mono);
}
.layout {
  flex: 1 1 auto;
  display: grid;
  grid-template-columns: 1fr minmax(14rem, 22rem);
  grid-template-rows: minmax(0, 1fr) minmax(8rem, 28vh);
  min-height: 0;
  height: 0; /* force flex child to use remaining viewport height */
}
.editor-wrap {
  grid-column: 1; grid-row: 1;
  position: relative;
  min-height: 0;
  border-right: 1px solid var(--line);
  background: var(--code-bg);
}
#editor {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  margin: 0;
  border: 0;
  padding: .85rem 1rem;
  resize: none;
  background: var(--code-bg);
  color: var(--code-fg);
  font: inherit;
  line-height: 1.45;
  tab-size: 4;
  overflow: auto;
  white-space: pre;
  overflow-wrap: normal;
}
#editor:focus { outline: 1px solid color-mix(in srgb, var(--accent) 50%, transparent); outline-offset: -1px; }
.side {
  grid-column: 2; grid-row: 1 / span 2;
  display: flex; flex-direction: column; min-height: 0;
  background: color-mix(in srgb, var(--panel) 92%, transparent);
}
.side h2 {
  margin: 0; padding: .5rem .75rem; font-size: .7rem; letter-spacing: .06em;
  text-transform: uppercase; color: var(--muted);
  border-bottom: 1px solid var(--line);
}
#vars {
  flex: 1; overflow: auto; margin: 0; padding: .5rem .75rem;
  font-size: .78rem; list-style: none;
}
#vars li { margin-bottom: .55rem; }
#vars .n { color: var(--accent); }
#vars .t { color: var(--muted); font-size: .7rem; }
#vars .v { white-space: pre-wrap; word-break: break-word; color: var(--ok); margin-top: .15rem; }
#vars .empty { color: var(--hint); }
.out-panel {
  grid-column: 1; grid-row: 2;
  border-top: 1px solid var(--line); background: var(--panel);
  display: flex; flex-direction: column; min-height: 0;
}
.out-tabs { display: flex; gap: .25rem; padding: .35rem .5rem; border-bottom: 1px solid var(--line); }
.out-tabs button { font-size: .7rem; padding: .2rem .45rem; }
.out-tabs button.on { border-color: var(--accent); color: var(--accent); }
#out {
  flex: 1; overflow: auto; padding: .6rem .75rem;
  font-size: .82rem; white-space: pre-wrap; word-break: break-word;
}
#out .stderr { color: #fde68a; }
html[data-theme="light"] #out .stderr { color: #92400e; }
#out .error { color: var(--err); }
#out .result { color: var(--ok); }
#out .empty { color: var(--hint); }
dialog {
  border: 1px solid var(--border); border-radius: 6px; background: var(--panel); color: var(--ink);
  padding: 1rem; min-width: min(28rem, 92vw);
}
dialog::backdrop { background: rgba(0,0,0,.55); }
dialog label { display: block; margin-bottom: .35rem; color: var(--muted); font-size: .75rem; }
dialog input { width: 100%; margin-bottom: .75rem; }
dialog .row { display: flex; gap: .5rem; justify-content: flex-end; align-items: center; }
dialog .row .spacer { flex: 1; }
#file-input { display: none; }
@media (max-width: 900px) {
  .layout { grid-template-columns: 1fr; grid-template-rows: 1fr 12rem auto; }
  .side { grid-column: 1; grid-row: 3; max-height: 40vh; border-top: 1px solid var(--line); }
  .editor-wrap { border-right: 0; }
  #args { max-width: none; }
}
</style>
</head>
<body>
` + chromeHeader("ide", session, actions, run) + `
<div class="runbar">
  <label for="args">args</label>
  <input type="text" id="args" placeholder='space-separated or ["a","b"]' autocomplete="off">
  <span class="hint">like CLI after --</span>
  <label class="chk"><input type="checkbox" id="trace"> trace</label>
</div>
<div class="layout">
  <div class="editor-wrap">
    <textarea id="editor" spellcheck="false" autocomplete="off" autocorrect="off" autocapitalize="off"
      placeholder="#!/usr/bin/env buddy&#10;&#10;def main():&#10;    print(args)"></textarea>
  </div>
  <aside class="side">
    <h2>Variables</h2>
    <ul id="vars"><li class="empty">Run a script to inspect globals</li></ul>
  </aside>
  <div class="out-panel">
    <div class="out-tabs">
      <button type="button" class="on" data-tab="all">All</button>
      <button type="button" data-tab="stdout">stdout</button>
      <button type="button" data-tab="stderr">stderr</button>
      <button type="button" data-tab="result">result</button>
    </div>
    <div id="out"><span class="empty">No output yet</span></div>
  </div>
</div>
<input type="file" id="file-input" accept=".buddy,.star,text/plain">
<dialog id="dlg">
  <form method="dialog" id="dlg-form">
    <label for="dlg-path">Path relative to IDE root</label>
    <input type="text" id="dlg-path" required placeholder="script.buddy" autocomplete="off">
    <p id="dlg-root" style="font-size:.7rem;color:var(--muted);margin:0 0 .75rem"></p>
    <div class="row">
      <button type="button" class="linkish" id="dlg-browse">Choose file…</button>
      <div class="spacer"></div>
      <button type="submit" value="cancel">Cancel</button>
      <button type="submit" class="primary" id="dlg-ok" value="ok">OK</button>
    </div>
  </form>
</dialog>
<script>
const state = {
  sessionId: null,
  path: "",
  dirty: false,
  last: { stdout: "", stderr: "", result: "", error: "" },
  tab: "all",
  dlgMode: "open",
  fileHandle: null, // File System Access API handle when available
  localOnly: false, // opened via <input type=file>; Save needs Save As / picker
};

const canPickFile = typeof window.showOpenFilePicker === "function";
const canSaveFile = typeof window.showSaveFilePicker === "function";

async function jfetch(url, opts) {
  const r = await fetch(url, opts);
  const data = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(data.error || r.statusText);
  return data;
}

function setStatus(msg) { document.getElementById("status").textContent = msg || ""; }

function focusEditor() {
  const el = document.getElementById("editor");
  // Ensure nothing left the page inert (modal dialog) and the buffer can take keys.
  const dlg = document.getElementById("dlg");
  if (dlg.open) dlg.close();
  el.readOnly = false;
  el.disabled = false;
  el.focus();
}

function setPath(p) {
  state.path = p || "";
  const el = document.getElementById("path");
  el.textContent = state.path || "(untitled)";
  el.classList.toggle("dirty", state.dirty);
  el.title = state.path;
}

function markDirty(d) {
  state.dirty = !!d;
  document.getElementById("path").classList.toggle("dirty", state.dirty);
}

function parseArgs(raw) {
  raw = (raw || "").trim();
  if (!raw) return [];
  if (raw.startsWith("[")) {
    const v = JSON.parse(raw);
    if (!Array.isArray(v)) throw new Error("args JSON must be an array");
    return v.map(String);
  }
  return raw.match(/(?:[^\s"]+|"[^"]*")+/g).map(s => s.replace(/^"|"$/g, ""));
}

async function ensureSession(force) {
  if (state.sessionId && !force) return state.sessionId;
  if (state.sessionId) {
    try { await fetch("/api/session/" + state.sessionId, { method: "DELETE" }); } catch (_) {}
  }
  const tenant = document.getElementById("tenant").value;
  const cred = document.getElementById("auth").value;
  const trace = document.getElementById("trace").checked;
  let args = [];
  try { args = parseArgs(document.getElementById("args").value); }
  catch (e) { throw new Error("args: " + e.message); }
  const res = await jfetch("/api/session", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ tenant, cred, auth: cred, channel: "ide", trace, args }),
  });
  state.sessionId = res.id;
  return state.sessionId;
}

async function applyConfig() {
  const id = await ensureSession(false);
  const trace = document.getElementById("trace").checked;
  let args = [];
  try { args = parseArgs(document.getElementById("args").value); }
  catch (e) { throw new Error("args: " + e.message); }
  await jfetch("/api/session/" + id + "/config", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ trace, args }),
  });
}

function renderOut() {
  const L = state.last;
  const el = document.getElementById("out");
  const parts = [];
  const tab = state.tab;
  if ((tab === "all" || tab === "stdout") && L.stdout) {
    parts.push('<div class="stdout">' + escapeHtml(L.stdout) + '</div>');
  }
  if ((tab === "all" || tab === "stderr") && L.stderr) {
    parts.push('<div class="stderr">' + escapeHtml(L.stderr) + '</div>');
  }
  if ((tab === "all" || tab === "result") && L.result) {
    parts.push('<div class="result">' + escapeHtml(L.result) + '</div>');
  }
  if (L.error) {
    parts.push('<div class="error">' + escapeHtml(L.error) + '</div>');
  }
  el.innerHTML = parts.length ? parts.join("\n") : '<span class="empty">No output</span>';
}

function renderVars(globals) {
  const ul = document.getElementById("vars");
  if (!globals || !globals.length) {
    ul.innerHTML = '<li class="empty">No user globals</li>';
    return;
  }
  ul.innerHTML = globals.map(g =>
    '<li><div><span class="n">' + escapeHtml(g.name) + '</span> <span class="t">' +
    escapeHtml(g.type) + '</span></div><div class="v">' + escapeHtml(g.value || "") + '</div></li>'
  ).join("");
}

function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, c => ({ "&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;" }[c]));
}

async function run() {
  setStatus("running…");
  try {
    await applyConfig();
    const id = state.sessionId;
    const res = await jfetch("/api/session/" + id + "/run", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        source: document.getElementById("editor").value,
        filename: state.path || "<buffer>",
      }),
    });
    state.last = { stdout: res.stdout || "", stderr: res.stderr || "", result: res.result || "", error: res.error || "" };
    renderOut();
    renderVars(res.globals || []);
    setStatus(res.error ? "error" : "ok");
  } catch (e) {
    state.last = { stdout: "", stderr: "", result: "", error: String(e.message || e) };
    renderOut();
    setStatus("error");
  }
}

async function resetSession() {
  if (!state.sessionId) return;
  await jfetch("/api/session/" + state.sessionId + "/reset", { method: "POST" });
  renderVars([]);
  state.last = { stdout: "", stderr: "", result: "", error: "" };
  renderOut();
  setStatus("reset");
}

const pickerTypes = [{
  description: "Buddy scripts",
  accept: { "text/plain": [".buddy", ".star"] },
}];

async function openViaPicker() {
  if (canPickFile) {
    const [handle] = await window.showOpenFilePicker({ types: pickerTypes, multiple: false });
    const file = await handle.getFile();
    document.getElementById("editor").value = await file.text();
    state.fileHandle = handle;
    state.localOnly = false;
    setPath(handle.name || file.name);
    markDirty(false);
    setStatus("opened");
    focusEditor();
    return;
  }
  // Fallback: classic file input (Finder / file dialog; no write-back handle)
  document.getElementById("file-input").value = "";
  document.getElementById("file-input").click();
}

function onFileInputChange(ev) {
  const file = ev.target.files && ev.target.files[0];
  if (!file) return;
  const reader = new FileReader();
  reader.onload = () => {
    document.getElementById("editor").value = String(reader.result || "");
    state.fileHandle = null;
    state.localOnly = true;
    setPath(file.name);
    markDirty(false);
    setStatus("opened");
    focusEditor();
  };
  reader.onerror = () => setStatus("failed to read file");
  reader.readAsText(file);
}

async function openPathDlg() {
  state.dlgMode = "open";
  const root = await jfetch("/api/fs/root");
  document.getElementById("dlg-root").textContent =
    "IDE root: " + root.root + " — type a path under this root, or Choose file…";
  document.getElementById("dlg-path").value = state.path || "";
  document.getElementById("dlg-browse").hidden = false;
  document.getElementById("dlg").showModal();
  document.getElementById("dlg-path").focus();
}

async function openFile() {
  try {
    await openViaPicker();
  } catch (e) {
    if (e && e.name === "AbortError") return;
    // Picker unavailable / denied — fall back to path dialog
    await openPathDlg();
  }
}

async function saveViaHandle(handle) {
  const writable = await handle.createWritable();
  await writable.write(document.getElementById("editor").value);
  await writable.close();
  state.fileHandle = handle;
  state.localOnly = false;
  setPath(handle.name || state.path);
  markDirty(false);
  setStatus("saved");
}

async function saveAsViaPicker() {
  const handle = await window.showSaveFilePicker({
    types: pickerTypes,
    suggestedName: state.path || "script.buddy",
  });
  await saveViaHandle(handle);
}

async function savePathDlg() {
  state.dlgMode = "save";
  const root = await jfetch("/api/fs/root");
  document.getElementById("dlg-root").textContent =
    "IDE root: " + root.root + " — path relative to this root";
  document.getElementById("dlg-path").value = state.path || "";
  document.getElementById("dlg-browse").hidden = true;
  document.getElementById("dlg").showModal();
  document.getElementById("dlg-path").focus();
}

async function onDlgSubmit(ev) {
  const val = ev.submitter && ev.submitter.value;
  if (val !== "ok") return;
  ev.preventDefault();
  const path = document.getElementById("dlg-path").value.trim();
  document.getElementById("dlg").close();
  if (state.dlgMode === "open") {
    const res = await jfetch("/api/fs/read?path=" + encodeURIComponent(path));
    document.getElementById("editor").value = res.content;
    state.fileHandle = null;
    state.localOnly = false;
    setPath(res.path);
    markDirty(false);
    setStatus("opened");
    focusEditor();
  } else {
    await jfetch("/api/fs/write", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ path, content: document.getElementById("editor").value }),
    });
    state.fileHandle = null;
    state.localOnly = false;
    setPath(path);
    markDirty(false);
    setStatus("saved");
  }
}

async function save() {
  if (state.fileHandle) {
    await saveViaHandle(state.fileHandle);
    return;
  }
  if (state.localOnly || !state.path) {
    await saveAs();
    return;
  }
  if (state.path) {
    await jfetch("/api/fs/write", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ path: state.path, content: document.getElementById("editor").value }),
    });
    markDirty(false);
    setStatus("saved");
    return;
  }
  await saveAs();
}

async function saveAs() {
  if (canSaveFile) {
    try {
      await saveAsViaPicker();
      return;
    } catch (e) {
      if (e && e.name === "AbortError") return;
    }
  }
  await savePathDlg();
}

let authCatalog = [];

function fillAuthSelect(selected) {
  const tenant = document.getElementById("tenant").value;
  const sel = document.getElementById("auth");
  const prev = selected !== undefined ? selected : sel.value;
  sel.innerHTML = "";
  const auto = document.createElement("option");
  auto.value = "";
  auto.textContent = "(auto)";
  sel.appendChild(auto);
  (authCatalog || []).forEach(a => {
    const linked = !tenant || !(a.tenants || []).length || (a.tenants || []).includes(tenant);
    if (!linked) return;
    const o = document.createElement("option");
    o.value = a.name;
    let label = a.name;
    if (a.access && a.access !== "full") label += " · " + a.access;
    if (a.surface && a.surface !== "any") label += " · " + a.surface;
    o.textContent = label;
    sel.appendChild(o);
  });
  if ([...sel.options].some(o => o.value === prev)) sel.value = prev;
  else sel.value = "";
}

async function loadCreds() {
  const res = await jfetch("/api/creds?channel=ide");
  authCatalog = res.auths || [];
  const tsel = document.getElementById("tenant");
  tsel.innerHTML = "";
  (res.tenants || []).forEach(t => {
    const o = document.createElement("option");
    o.value = t; o.textContent = t;
    if (t === res.tenant) o.selected = true;
    tsel.appendChild(o);
  });
  if (!tsel.options.length) {
    const o = document.createElement("option");
    o.value = res.tenant || "";
    o.textContent = o.value || "(no tenants)";
    tsel.appendChild(o);
  }
  const preselect = res.tenant || "";
  if (preselect && [...tsel.options].some(o => o.value === preselect)) {
    tsel.value = preselect;
  }
  fillAuthSelect(res.auth || "");
  tsel.addEventListener("change", () => {
    fillAuthSelect("");
    ensureSession(true).catch(e => setStatus(e.message));
  });
}

document.getElementById("btn-run").onclick = () => run();
document.getElementById("btn-reset").onclick = () => resetSession().catch(e => setStatus(e.message));
document.getElementById("btn-open").onclick = () => openFile().catch(e => setStatus(e.message));
document.getElementById("btn-save").onclick = () => save().catch(e => setStatus(e.message));
document.getElementById("btn-save-as").onclick = () => saveAs().catch(e => setStatus(e.message));
document.getElementById("file-input").addEventListener("change", onFileInputChange);
document.getElementById("dlg-browse").onclick = () => {
  document.getElementById("dlg").close();
  openViaPicker().catch(e => {
    if (e && e.name === "AbortError") return;
    setStatus(e.message || String(e));
  });
};
document.querySelector(".editor-wrap").addEventListener("mousedown", (e) => {
  if (e.target !== document.getElementById("editor")) focusEditor();
});
document.getElementById("editor").addEventListener("input", () => markDirty(true));
document.getElementById("editor").addEventListener("keydown", (e) => {
  // Keep Tab in the buffer instead of moving focus.
  if (e.key === "Tab" && !e.metaKey && !e.ctrlKey && !e.altKey) {
    e.preventDefault();
    const ta = e.target;
    const start = ta.selectionStart, end = ta.selectionEnd;
    ta.value = ta.value.slice(0, start) + "\t" + ta.value.slice(end);
    ta.selectionStart = ta.selectionEnd = start + 1;
    markDirty(true);
  }
});
document.getElementById("dlg-form").addEventListener("submit", (ev) => {
  onDlgSubmit(ev).catch(e => setStatus(e.message));
});
document.getElementById("auth").addEventListener("change", () => {
  ensureSession(true).catch(e => setStatus(e.message));
});
document.getElementById("trace").addEventListener("change", () => {
  applyConfig().catch(e => setStatus(e.message));
});
document.querySelectorAll(".out-tabs button").forEach(btn => {
  btn.onclick = () => {
    document.querySelectorAll(".out-tabs button").forEach(b => b.classList.remove("on"));
    btn.classList.add("on");
    state.tab = btn.dataset.tab;
    renderOut();
  };
});
document.addEventListener("keydown", (e) => {
  if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
    e.preventDefault();
    run();
  }
  if ((e.metaKey || e.ctrlKey) && e.key === "s") {
    e.preventDefault();
    save().catch(err => setStatus(err.message));
  }
  if ((e.metaKey || e.ctrlKey) && e.key === "o") {
    e.preventDefault();
    openFile().catch(err => setStatus(err.message));
  }
});

loadCreds().then(() => ensureSession(true)).then(() => focusEditor()).catch(e => setStatus(e.message));
</script>
` + chromeToggleScript() + `
</body>
</html>
`
}
