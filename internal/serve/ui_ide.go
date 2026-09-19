package serve

import (
	"net/http"
)

func serveIDEHTML(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(buildIDEHTML(RequestEmbed(r))))
}

var ideHTML = buildIDEHTML(false)

func buildIDEHTML(embed bool) string {
	session := `
  <label>context <select id="context"></select></label>
  <label>user <select id="auth"></select></label>`
	actions := `
  <button type="button" id="btn-open">Open</button>
  <button type="button" id="btn-save">Save</button>
  <button type="button" id="btn-save-as">Save As</button>
  <span id="path" title="">(untitled)</span>
  <button type="button" id="btn-reset">Reset</button>
  <button type="button" id="btn-libs">Libraries</button>
  <span id="status"></span>`
	run := ``

	return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>buddy ide</title>
` + chromeHeadTags(embed) + `<style>
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
  --out-h: 40%;
  flex: 1 1 auto;
  display: grid;
  grid-template-columns: 1fr minmax(14rem, 22rem);
  grid-template-rows: minmax(0, 1fr) 6px var(--out-h);
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
  grid-column: 2; grid-row: 1 / span 3;
  display: flex; flex-direction: column; min-height: 0;
  background: color-mix(in srgb, var(--panel) 92%, transparent);
}
.side-tabs {
  display: flex; gap: .25rem; padding: .35rem .5rem; border-bottom: 1px solid var(--line);
  flex-shrink: 0;
}
.side-tabs button { font-size: .7rem; padding: .2rem .45rem; }
.side-tabs button.on { border-color: var(--accent); color: var(--accent); }
.side-pane { flex: 1; min-height: 0; display: flex; flex-direction: column; }
.side-pane[hidden] { display: none !important; }
#vars {
  flex: 1; overflow: auto; margin: 0; padding: .5rem .75rem;
  font-size: .78rem; list-style: none;
}
#vars li { margin-bottom: .55rem; }
#vars .var-head { display: flex; align-items: baseline; gap: .35rem; }
#vars .n { color: var(--accent); cursor: pointer; }
#vars .n:hover { text-decoration: underline; }
#vars .t { color: var(--muted); font-size: .7rem; }
#vars .v { white-space: pre-wrap; word-break: break-word; color: var(--ok); margin-top: .15rem; }
#vars .empty { color: var(--hint); }
#vars .drop {
  margin-left: auto; font-size: .65rem; padding: .1rem .35rem;
  flex-shrink: 0;
}
.lib-search {
  margin: .45rem .5rem 0; width: calc(100% - 1rem);
}
.lib-body { flex: 1; overflow: auto; padding: .5rem .6rem 1.5rem; font-size: .78rem; }
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
.fn:hover { background: var(--surface-2); }
.fn .name { color: var(--ok); }
.fn .sig { display: block; color: var(--muted); font-size: .68rem; margin-top: .1rem; font-family: inherit; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.fn .sum { display: block; color: var(--muted); font-size: .7rem; margin-top: .15rem; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.fn-detail {
  margin: .15rem .4rem .5rem; padding: .45rem .55rem; border-radius: 4px;
  background: var(--detail-bg); border: 1px solid var(--line); font-size: .72rem; color: var(--muted);
  white-space: pre-wrap; font-family: inherit;
}
.fn-detail .actions { margin-top: .4rem; display: flex; gap: .4rem; flex-wrap: wrap; }
.fn-detail button {
  font: inherit; font-size: .72rem; padding: .2rem .5rem; border-radius: 4px;
  border: 1px solid var(--border); background: var(--surface-2); color: var(--ink); cursor: pointer;
}
.fn-detail button.primary { background: var(--accent); border-color: var(--accent); color: var(--accent-ink); }
.fn-detail a.docs-link {
  font: inherit; font-size: .72rem; padding: .2rem .5rem; border-radius: 4px;
  border: 1px solid var(--border); background: var(--surface-2); color: var(--ink); text-decoration: none;
  display: inline-block;
}
.fn-detail a.docs-link:hover { border-color: var(--accent); color: var(--accent); }
.drawer-empty { color: var(--muted); padding: .5rem; }
.split-h {
  grid-column: 1; grid-row: 2;
  cursor: ns-resize; background: var(--line);
  touch-action: none;
}
.split-h:hover, .split-h.dragging { background: var(--accent); }
.out-panel {
  grid-column: 1; grid-row: 3;
  background: var(--panel);
  display: flex; flex-direction: column; min-height: 0;
}
.out-tabs { display: flex; gap: .25rem; align-items: center; padding: .35rem .5rem; border-bottom: 1px solid var(--line); }
.out-tabs button { font-size: .7rem; padding: .2rem .45rem; }
.out-tabs button.on { border-color: var(--accent); color: var(--accent); }
.out-tabs .spacer { flex: 1; }
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
  .layout { grid-template-columns: 1fr; grid-template-rows: minmax(0, 1fr) 6px var(--out-h) auto; }
  .side { grid-column: 1; grid-row: 4; max-height: 40vh; border-top: 1px solid var(--line); }
  .split-h { grid-column: 1; grid-row: 2; }
  .out-panel { grid-column: 1; grid-row: 3; }
  .editor-wrap { border-right: 0; }
  #args { max-width: none; }
}
</style>
<style>` + confirmPollCSS + `</style>
</head>
<body>
` + chromeHeader("ide", session, actions, run, embed) + `
<div class="runbar">
  <button type="button" class="primary" id="btn-run">Run</button>
  <label for="args">args</label>
  <input type="text" id="args" placeholder='space-separated or ["a","b"]' autocomplete="off">
  <span class="hint">like CLI after --</span>
  <label class="chk"><input type="checkbox" id="trace"> trace</label>
  <button type="button" id="btn-clear-code">Clear code</button>
</div>
<div class="layout">
  <div class="editor-wrap">
    <textarea id="editor" spellcheck="false" autocomplete="off" autocorrect="off" autocapitalize="off"
      placeholder="#!/usr/bin/env buddy&#10;&#10;def main():&#10;    print(args)"></textarea>
  </div>
  <aside class="side">
    <div class="side-tabs">
      <button type="button" class="on" data-side="vars">Variables</button>
      <button type="button" data-side="libs">Libraries</button>
    </div>
    <div class="side-pane" id="vars-pane">
      <ul id="vars"><li class="empty">Run a script to inspect globals</li></ul>
    </div>
    <div class="side-pane" id="libs-pane" hidden>
      <input type="search" class="lib-search" id="libSearch" placeholder="Filter…" autocomplete="off">
      <div class="lib-body" id="libBody"><div class="drawer-empty">Loading…</div></div>
    </div>
  </aside>
  <div class="split-h" id="split-h" role="separator" aria-orientation="horizontal" title="Drag to resize output"></div>
  <div class="out-panel">
    <div class="out-tabs">
      <button type="button" class="on" data-tab="all">All</button>
      <button type="button" data-tab="stdout">stdout</button>
      <button type="button" data-tab="stderr">stderr</button>
      <button type="button" data-tab="result">result</button>
      <span class="spacer"></span>
      <button type="button" id="btn-copy" disabled>Copy</button>
      <button type="button" id="btn-save-out" disabled>Save</button>
      <button type="button" id="btn-stash" disabled>Stash as…</button>
      <button type="button" id="btn-clear-out" disabled>Clear</button>
    </div>
    <div id="out"><span class="empty">No output yet</span></div>
  </div>
</div>
<input type="file" id="file-input" accept=".buddy,text/plain">
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
` + confirmPollJS + `
const state = {
  sessionId: null,
  path: "",
  dirty: false,
  last: { stdout: "", stderr: "", result: "", error: "" },
  tab: "all",
  side: "vars",
  stashable: false,
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
  scheduleDraftSave();
}

const IDE_DRAFT_KEY = "buddy-serve-ide-draft";
let draftSaveTimer = null;
function collectDraft() {
  return {
    source: document.getElementById("editor").value,
    path: state.path || "",
    args: document.getElementById("args").value,
    dirty: !!state.dirty,
    trace: document.getElementById("trace").checked,
  };
}
function saveDraft() {
  buddyServeStore.set(IDE_DRAFT_KEY, collectDraft());
}
function scheduleDraftSave() {
  clearTimeout(draftSaveTimer);
  draftSaveTimer = setTimeout(saveDraft, 200);
}
function restoreDraft() {
  const d = buddyServeStore.get(IDE_DRAFT_KEY);
  if (!d || typeof d !== "object") return false;
  if (typeof d.source === "string") document.getElementById("editor").value = d.source;
  if (typeof d.args === "string") document.getElementById("args").value = d.args;
  if (typeof d.trace === "boolean") document.getElementById("trace").checked = d.trace;
  state.dirty = !!d.dirty;
  if (typeof d.path === "string" && d.path) {
    setPath(d.path);
  } else {
    document.getElementById("path").classList.toggle("dirty", state.dirty);
  }
  saveDraft();
  return true;
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
  const context = document.getElementById("context").value;
  const cred = document.getElementById("auth").value;
  const trace = document.getElementById("trace").checked;
  let args = [];
  try { args = parseArgs(document.getElementById("args").value); }
  catch (e) { throw new Error("args: " + e.message); }
  const res = await jfetch("/api/session", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ context, tenant: context, cred, auth: cred, channel: "ide", trace, args }),
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

function currentTabText() {
  const L = state.last;
  const tab = state.tab;
  const parts = [];
  if ((tab === "all" || tab === "stdout") && L.stdout) parts.push(L.stdout);
  if ((tab === "all" || tab === "stderr") && L.stderr) parts.push(L.stderr);
  if ((tab === "all" || tab === "result") && L.result) parts.push(L.result);
  if (L.error) parts.push(L.error);
  return parts.join("");
}

function updateOutActions() {
  const empty = !currentTabText();
  document.getElementById("btn-copy").disabled = empty;
  document.getElementById("btn-save-out").disabled = empty;
  document.getElementById("btn-clear-out").disabled = empty;
  document.getElementById("btn-stash").disabled = !state.stashable;
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
  updateOutActions();
}

function renderVars(globals) {
  const ul = document.getElementById("vars");
  if (!globals || !globals.length) {
    ul.innerHTML = '<li class="empty">No user globals</li>';
    return;
  }
  ul.innerHTML = globals.map(g =>
    '<li><div class="var-head"><span class="n" title="Insert at cursor">' + escapeHtml(g.name) + '</span> <span class="t">' +
    escapeHtml(g.type) + '</span><button type="button" class="drop" data-drop="' + escAttr(g.name) +
    '" title="Clear ' + escAttr(g.name) + '">Clear</button></div><div class="v">' +
    escapeHtml(g.value || "") + '</div></li>'
  ).join("");
}

function insertAtCursor(text) {
  const ta = document.getElementById("editor");
  if (!ta) return;
  const start = ta.selectionStart ?? ta.value.length;
  const end = ta.selectionEnd ?? start;
  const before = ta.value.slice(0, start);
  const after = ta.value.slice(end);
  ta.value = before + text + after;
  const pos = (before + text).length;
  ta.focus();
  ta.selectionStart = ta.selectionEnd = pos;
  markDirty(true);
}

function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, c => ({ "&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;" }[c]));
}

async function run() {
  setStatus("running…");
  try {
    await applyConfig();
    const id = state.sessionId;
    const res = await withBuddyConfirmPoll(id, (async () => {
      return await jfetch("/api/session/" + id + "/run", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          source: document.getElementById("editor").value,
          filename: state.path || "<buffer>",
        }),
      });
    })());
    state.last = { stdout: res.stdout || "", stderr: res.stderr || "", result: res.result || "", error: res.error || "" };
    state.stashable = !!res.stashable && !res.error;
    renderOut();
    renderVars(res.globals || []);
    setStatus(res.error ? "error" : "ok");
  } catch (e) {
    state.last = { stdout: "", stderr: "", result: "", error: String(e.message || e) };
    state.stashable = false;
    renderOut();
    setStatus("error");
  }
}

async function resetSession() {
  let globals = [];
  if (state.sessionId) {
    const res = await jfetch("/api/session/" + state.sessionId + "/reset", { method: "POST" });
    globals = res.globals || [];
  }
  document.getElementById("editor").value = "";
  document.getElementById("args").value = "";
  state.fileHandle = null;
  state.localOnly = false;
  state.path = "";
  setPath("");
  markDirty(false);
  state.last = { stdout: "", stderr: "", result: "", error: "" };
  state.stashable = false;
  renderOut();
  renderVars(globals);
  saveDraft();
  setStatus("reset");
  focusEditor();
}

function clearCode() {
  document.getElementById("editor").value = "";
  if (state.path) markDirty(true);
  else markDirty(false);
  saveDraft();
  setStatus("cleared code");
  focusEditor();
}

function clearOutput() {
  state.last = { stdout: "", stderr: "", result: "", error: "" };
  renderOut();
  setStatus("cleared output");
}

async function dropVar(name) {
  const id = await ensureSession(false);
  const res = await jfetch("/api/session/" + id + "/drop", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name }),
  });
  renderVars(res.globals || []);
  if (name === "_") {
    state.stashable = false;
    updateOutActions();
  }
  setStatus("cleared " + name);
}

const pickerTypes = [{
  description: "Buddy scripts",
  accept: { "text/plain": [".buddy"] },
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
  const context = document.getElementById("context").value;
  const sel = document.getElementById("auth");
  const prev = selected !== undefined ? selected : sel.value;
  sel.innerHTML = "";
  const auto = document.createElement("option");
  auto.value = "";
  auto.textContent = "(auto)";
  sel.appendChild(auto);
  (authCatalog || []).forEach(a => {
    const linkedNames = a.contexts || a.tenants || [];
    const linked = !context || !linkedNames.length || linkedNames.includes(context);
    if (!linked) return;
    const o = document.createElement("option");
    o.value = a.name;
    let label = a.name;
    if (a.access && a.access !== "full") label += " · " + a.access;
    if (a.type && a.type !== "oauth") label += " · " + a.type;
    o.textContent = label;
    sel.appendChild(o);
  });
  if ([...sel.options].some(o => o.value === prev)) sel.value = prev;
  else sel.value = "";
}

async function loadCreds() {
  const res = await jfetch("/api/creds?channel=ide");
  authCatalog = res.auths || [];
  const tsel = document.getElementById("context");
  tsel.innerHTML = "";
  const names = res.contexts || res.tenants || [];
  const current = res.context || res.tenant || "";
  names.forEach(t => {
    const o = document.createElement("option");
    o.value = t; o.textContent = t;
    if (t === current) o.selected = true;
    tsel.appendChild(o);
  });
  if (!tsel.options.length) {
    const o = document.createElement("option");
    o.value = current;
    o.textContent = o.value || "(no contexts)";
    tsel.appendChild(o);
  }
  const preselect = current;
  if (preselect && [...tsel.options].some(o => o.value === preselect)) {
    tsel.value = preselect;
  }
  fillAuthSelect(res.auth || "");
  tsel.addEventListener("change", () => {
    fillAuthSelect("");
    ensureSession(true).catch(e => setStatus(e.message));
  });
}

function loadTrySource(source) {
  if (typeof source !== "string") return false;
  document.getElementById("editor").value = source;
  state.fileHandle = null;
  state.localOnly = false;
  state.path = "";
  setPath("");
  markDirty(true);
  setStatus("loaded from docs");
  focusEditor();
  return true;
}

const TRY_KEY = "buddy-serve-ide-try";
function consumeTryPayload() {
  let raw = null;
  try { raw = localStorage.getItem(TRY_KEY); } catch (_) {}
  if (!raw) return false;
  try { localStorage.removeItem(TRY_KEY); } catch (_) {}
  try {
    const d = JSON.parse(raw);
    if (d && typeof d.source === "string") return loadTrySource(d.source);
  } catch (_) {}
  return false;
}

window.addEventListener("message", (e) => {
  if (e.origin !== location.origin) return;
  if (!e.data || e.data.type !== "buddy-ide-try") return;
  loadTrySource(e.data.source);
});
window.addEventListener("storage", (e) => {
  if (e.key !== TRY_KEY || !e.newValue) return;
  consumeTryPayload();
});

async function copyOutput() {
  const text = currentTabText();
  if (!text) return;
  await navigator.clipboard.writeText(text);
  setStatus("copied");
}

async function saveOutput() {
  const text = currentTabText();
  if (!text) return;
  const suggested = (state.tab === "all" ? "output" : state.tab) + ".txt";
  if (canSaveFile) {
    try {
      const handle = await window.showSaveFilePicker({
        suggestedName: suggested,
        types: [{ description: "Text", accept: { "text/plain": [".txt"] } }],
      });
      const writable = await handle.createWritable();
      await writable.write(text);
      await writable.close();
      setStatus("saved output");
      return;
    } catch (e) {
      if (e && e.name === "AbortError") return;
    }
  }
  const path = window.prompt("Save output as (relative to IDE root)", suggested);
  if (!path) return;
  await jfetch("/api/fs/write", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ path, content: text }),
  });
  setStatus("saved output");
}

async function stashAs() {
  if (!state.stashable || !state.sessionId) return;
  const name = window.prompt("Stash last result as", "");
  if (!name) return;
  const res = await jfetch("/api/session/" + state.sessionId + "/stash", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name }),
  });
  renderVars(res.globals || []);
  setStatus("stashed as " + name);
}

const OUT_H_KEY = "buddy-serve-ide-out-h";
function applyOutH(h) {
  document.querySelector(".layout").style.setProperty("--out-h", h);
}
function restoreOutH() {
  try {
    const h = sessionStorage.getItem(OUT_H_KEY);
    if (h) applyOutH(h);
  } catch (_) {}
}
function initSplitter() {
  const handle = document.getElementById("split-h");
  const layout = document.querySelector(".layout");
  handle.addEventListener("pointerdown", (e) => {
    e.preventDefault();
    handle.classList.add("dragging");
    handle.setPointerCapture(e.pointerId);
    const onMove = (ev) => {
      const rect = layout.getBoundingClientRect();
      const min = 8 * 16;
      const side = document.querySelector(".side");
      const stacked = window.matchMedia("(max-width: 900px)").matches;
      const sideH = stacked && side ? side.getBoundingClientRect().height : 0;
      const max = Math.max(min, rect.height - sideH - min);
      let h = rect.bottom - sideH - ev.clientY;
      h = Math.max(min, Math.min(max, h));
      const val = h + "px";
      applyOutH(val);
      try { sessionStorage.setItem(OUT_H_KEY, val); } catch (_) {}
    };
    const onUp = () => {
      handle.classList.remove("dragging");
      handle.releasePointerCapture(e.pointerId);
      handle.removeEventListener("pointermove", onMove);
      handle.removeEventListener("pointerup", onUp);
    };
    handle.addEventListener("pointermove", onMove);
    handle.addEventListener("pointerup", onUp);
  });
}

const SIDE_TAB_KEY = "buddy-serve-ide-side-tab";
function setSideTab(tab, persist) {
  state.side = tab === "libs" ? "libs" : "vars";
  document.querySelectorAll(".side-tabs button").forEach(b => {
    b.classList.toggle("on", b.dataset.side === state.side);
  });
  document.getElementById("vars-pane").hidden = state.side !== "vars";
  document.getElementById("libs-pane").hidden = state.side !== "libs";
  document.getElementById("btn-libs").classList.toggle("active", state.side === "libs");
  if (state.side === "libs") renderLibraries();
  if (persist !== false) {
    try { sessionStorage.setItem(SIDE_TAB_KEY, state.side); } catch (_) {}
  }
}
function toggleLibraries() {
  setSideTab(state.side === "libs" ? "vars" : "libs");
}

const libCache = {};
let libDetailEl = null;
async function fetchLibrary(id) {
  if (libCache[id]) return libCache[id];
  const r = await fetch("/api/libraries/" + encodeURIComponent(id));
  const data = await r.json();
  if (!r.ok) throw new Error(data.error || "library failed");
  libCache[id] = data;
  return data;
}
function fnMatches(fn, q) {
  if (!q) return true;
  const hay = (fn.name + " " + (fn.signature || "") + " " + (fn.summary || "") + " " +
    (fn.params || "") + " " + (fn.returns || "") + " " + (fn.notes || "") + " " +
    (fn.section || "") + " " + (fn.group || "")).toLowerCase();
  return hay.includes(q);
}
async function renderLibraries() {
  const body = document.getElementById("libBody");
  const q = (document.getElementById("libSearch").value || "").trim().toLowerCase();
  libDetailEl = null;
  try {
    const listR = await fetch("/api/libraries");
    const list = await listR.json();
    if (!listR.ok) throw new Error(list.error || "list failed");
    const parts = [];
    for (const item of list) {
      const detail = await fetchLibrary(item.id);
      if (detail.kind === "builtins" && detail.sections) {
        for (const sec of detail.sections) {
          const fns = (sec.functions || []).filter(f => fnMatches(f, q));
          if (!fns.length) continue;
          const open = sec.title === "Getting started" || !!q;
          parts.push(renderGroup(sec.title, fns, open));
        }
      } else {
        const fns = (detail.functions || []).filter(f => fnMatches(f, q));
        if (!fns.length) continue;
        const title = item.kind === "stdlib" ? "stdlib / " + item.title : item.title;
        parts.push(renderGroup(title, fns, true));
      }
    }
    body.innerHTML = parts.join("") || '<div class="drawer-empty">No matches</div>';
    body.querySelectorAll(".fn").forEach(btn => {
      btn.onclick = () => showFnDetail(btn);
    });
  } catch (e) {
    body.innerHTML = '<div class="drawer-empty">' + escapeHtml(String(e)) + '</div>';
  }
}
function showFnDetail(btn) {
  if (libDetailEl) libDetailEl.remove();
  const insert = btn.dataset.insert || (btn.dataset.name + "()");
  const sig = btn.dataset.sig || "";
  const ret = btn.dataset.returns || "";
  const params = btn.dataset.params || "";
  const sum = btn.dataset.summary || "";
  const notes = btn.dataset.notes || "";
  let body = "";
  if (sig) body += escapeHtml(sig) + "\n";
  if (sum) body += "Does: " + escapeHtml(sum) + "\n";
  if (ret) body += "→ " + escapeHtml(ret) + "\n";
  if (params) body += "Params: " + escapeHtml(params) + "\n";
  if (notes) body += "Notes: " + escapeHtml(notes) + "\n";
  if (!body.trim()) body = "(no documentation)";
  const div = document.createElement("div");
  div.className = "fn-detail";
  const name = btn.dataset.name || "";
  const docsHref = name ? ("/docs/builtins#" + encodeURIComponent(name)) : "/docs/builtins";
  div.innerHTML = "<div>" + body.trimEnd().replace(/\n/g, "<br>") + "</div>" +
    '<div class="actions">' +
    '<button type="button" class="primary" data-act="insert">Insert</button>' +
    (name ? '<a class="docs-link" href="' + docsHref + '">Docs</a>' : "") +
    '<button type="button" data-act="close">Close</button>' +
    "</div>";
  btn.after(div);
  libDetailEl = div;
  div.querySelector("[data-act=insert]").onclick = () => insertAtCursor(insert);
  div.querySelector("[data-act=close]").onclick = () => { div.remove(); libDetailEl = null; };
}
function renderGroup(title, fns, open) {
  const items = fns.map(f => {
    const insert = f.insert || (f.name + "()");
    const sumText = f.summary || f.returns || "";
    const sum = sumText ? '<span class="sum">' + escapeHtml(sumText) + "</span>" : "";
    const sig = f.signature ? '<span class="sig">' + escapeHtml(f.signature) + "</span>" : "";
    return '<button type="button" class="fn" data-insert="' + escAttr(insert) + '" data-name="' + escAttr(f.name) +
      '" data-sig="' + escAttr(f.signature || "") + '" data-returns="' + escAttr(f.returns || "") +
      '" data-params="' + escAttr(f.params || "") + '" data-summary="' + escAttr(f.summary || "") +
      '" data-notes="' + escAttr(f.notes || "") + '">' +
      '<span class="name">' + escapeHtml(f.name) + "</span>" + sig + sum + "</button>";
  }).join("");
  const openAttr = open ? " open" : "";
  return '<details class="lib-group"' + openAttr + "><summary>" + escapeHtml(title) + "</summary>" + items + "</details>";
}
function escAttr(s) {
  return String(s).replace(/&/g, "&amp;").replace(/"/g, "&quot;").replace(/</g, "&lt;");
}

document.getElementById("btn-run").onclick = () => run();
document.getElementById("btn-reset").onclick = () => resetSession().catch(e => setStatus(e.message));
document.getElementById("btn-clear-code").onclick = () => clearCode();
document.getElementById("btn-clear-out").onclick = () => clearOutput();
document.getElementById("btn-libs").onclick = () => toggleLibraries();
document.getElementById("btn-copy").onclick = () => copyOutput().catch(e => setStatus(e.message));
document.getElementById("btn-save-out").onclick = () => saveOutput().catch(e => setStatus(e.message));
document.getElementById("btn-stash").onclick = () => stashAs().catch(e => setStatus(e.message));
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
  if (buddyServeSoftTab(e.target, e)) markDirty(true);
});
document.getElementById("args").addEventListener("input", () => scheduleDraftSave());
document.getElementById("dlg-form").addEventListener("submit", (ev) => {
  onDlgSubmit(ev).catch(e => setStatus(e.message));
});
document.getElementById("auth").addEventListener("change", () => {
  ensureSession(true).catch(e => setStatus(e.message));
});
document.getElementById("trace").addEventListener("change", () => {
  scheduleDraftSave();
  applyConfig().catch(e => setStatus(e.message));
});
document.querySelectorAll(".out-tabs button[data-tab]").forEach(btn => {
  btn.onclick = () => {
    document.querySelectorAll(".out-tabs button[data-tab]").forEach(b => b.classList.remove("on"));
    btn.classList.add("on");
    state.tab = btn.dataset.tab;
    renderOut();
  };
});
document.querySelectorAll(".side-tabs button").forEach(btn => {
  btn.onclick = () => setSideTab(btn.dataset.side);
});
document.getElementById("vars").addEventListener("click", (e) => {
  const drop = e.target.closest("[data-drop]");
  if (drop) {
    e.preventDefault();
    dropVar(drop.dataset.drop).catch(err => setStatus(err.message));
    return;
  }
  const n = e.target.closest(".n");
  if (!n) return;
  insertAtCursor(n.textContent);
});
document.getElementById("libSearch").addEventListener("input", () => {
  if (state.side === "libs") renderLibraries();
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
window.addEventListener("beforeunload", () => saveDraft());

restoreOutH();
restoreDraft();
try {
  const side = sessionStorage.getItem(SIDE_TAB_KEY);
  if (side === "libs" || side === "vars") setSideTab(side, false);
} catch (_) {}
consumeTryPayload(); // docs "Try it" wins over restored draft when present
initSplitter();
loadCreds().then(() => ensureSession(true)).then(() => focusEditor()).catch(e => setStatus(e.message));
</script>
` + chromeToggleScript(embed) + `
</body>
</html>
`
}
