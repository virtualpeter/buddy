package serve

import (
	"net/http"
)

func serveConfigHTML(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write([]byte(configHTML(RequestEmbed(r))))
}

func configHTML(embed bool) string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>buddy config</title>
` + chromeHeadTags(embed) + `
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
main { padding: 1.5rem 2rem 3rem; max-width: 76rem; }
main h1 { margin: 0 0 1rem; font-size: 1.45rem; }
main h2 {
  margin: 1.75rem 0 .75rem;
  padding-bottom: .35rem;
  border-bottom: 1px solid var(--line);
  font-size: 1.05rem;
}
.banner {
  margin: 0 0 1.25rem;
  padding: .85rem 1rem;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: color-mix(in srgb, var(--accent) 12%, var(--panel));
}
.banner code { font-family: var(--mono); font-size: .88em; }
.hint { color: var(--muted); font-size: .9rem; margin: 0 0 1rem; }
.hint a { color: var(--accent); }
.section-note {
  color: var(--muted);
  font-size: .86rem;
  margin: -.35rem 0 1rem;
  max-width: 48rem;
}
.section-note code { font-family: var(--mono); font-size: .88em; }
ul.section-note { padding-left: 1.2rem; margin-top: .15rem; }
.section-note li { margin: .22rem 0; }
.form-grid label .why {
  font-size: .75rem;
  font-weight: 400;
  color: var(--muted);
  line-height: 1.35;
}
.err {
  display: none;
  margin: 0 0 1rem;
  padding: .65rem .85rem;
  border-radius: 6px;
  background: var(--error-bg);
  color: var(--err);
  font-family: var(--mono);
  font-size: .85rem;
}
.err.show { display: block; }
table {
  width: 100%;
  border-collapse: collapse;
  font-size: .9rem;
  margin: 0 0 1rem;
}
th, td {
  text-align: left;
  vertical-align: top;
  padding: .5rem .55rem;
  border-bottom: 1px solid var(--line);
}
th {
  font-family: var(--mono);
  font-size: .7rem;
  text-transform: uppercase;
  letter-spacing: .04em;
  color: var(--muted);
}
td code { font-family: var(--mono); font-size: .84em; }
.muted { color: var(--muted); }
.badge {
  display: inline-block;
  font-family: var(--mono);
  font-size: .72rem;
  padding: .1rem .4rem;
  border-radius: 4px;
  background: var(--surface-2);
}
.badge.gap { color: var(--err); }
.badge.ok { color: var(--ok); }
.form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(11rem, 1fr));
  gap: .65rem .75rem;
  margin: 0 0 1rem;
  padding: .9rem 1rem;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: color-mix(in srgb, var(--panel) 88%, var(--bg));
}
.form-grid label {
  display: flex;
  flex-direction: column;
  gap: .25rem;
  font-size: .82rem;
  color: var(--muted);
}
.form-grid input, .form-grid select {
  font: inherit;
  font-family: var(--mono);
  font-size: .85rem;
  padding: .35rem .45rem;
  border: 1px solid var(--border);
  border-radius: 4px;
  background: var(--panel);
  color: var(--ink);
}
.form-actions {
  grid-column: 1 / -1;
  display: flex;
  gap: .5rem;
  align-items: center;
  flex-wrap: wrap;
}
button {
  font: inherit;
  font-family: var(--mono);
  font-size: .8rem;
  padding: .35rem .7rem;
  border-radius: 4px;
  border: 1px solid var(--border);
  background: var(--panel);
  color: var(--ink);
  cursor: pointer;
}
button.primary {
  background: var(--accent);
  color: var(--accent-ink);
  border-color: transparent;
  font-weight: 600;
}
button.danger { color: var(--err); }
button:hover { filter: brightness(1.06); }
button:disabled { opacity: .45; cursor: not-allowed; filter: none; }
body.config-readonly main button,
body.config-readonly main input,
body.config-readonly main select,
body.config-readonly main textarea { pointer-events: none; opacity: .55; }
body.config-readonly main a { pointer-events: auto; opacity: 1; }
button.icon {
  padding: .15rem .4rem;
  font-size: 1rem;
  line-height: 1;
  border: none;
  background: transparent;
}
.work {
  display: grid;
  grid-template-columns: minmax(0, 1.4fr) minmax(18rem, 20rem);
  gap: 1.75rem;
  align-items: start;
}
.work > * { min-width: 0; }
.table-wrap { overflow-x: auto; }
.work table { margin: 0; }
#auth-form { grid-template-columns: 1fr; }
ul.tree { list-style: none; margin: 0; padding: 0; }
ul.tree ul.tree { margin: .15rem 0 0 .9rem; padding-left: .65rem; border-left: 1px solid var(--line); }
.tree-row {
  display: flex;
  align-items: center;
  gap: .35rem;
  padding: .28rem .4rem;
  border-radius: 6px;
  cursor: pointer;
}
.tree-row:hover { background: color-mix(in srgb, var(--accent) 8%, transparent); }
.tree-row.on { background: color-mix(in srgb, var(--accent) 16%, var(--panel)); }
.tree-row .name { font-family: var(--mono); font-size: .85rem; }
.tree-row .url { color: var(--muted); font-size: .78rem; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 14rem; }
.tree-row .grow { flex: 1; }
.row-actions { white-space: nowrap; text-align: right; }
tr.linked { background: color-mix(in srgb, var(--accent) 14%, var(--panel)); }
tr.other { opacity: .55; }
.checks {
  display: flex;
  flex-wrap: wrap;
  gap: .35rem .75rem;
  padding: .2rem 0;
}
.checks .check {
  display: flex;
  flex-direction: row;
  align-items: center;
  gap: .35rem;
  font-family: var(--mono);
  font-size: .82rem;
  color: var(--ink);
}
</style>
</head>
<body>
` + chromeHeader("config", "", "", "", embed) + `
<main>
  <h1>Config</h1>
  <p class="hint">On macOS, add workplaces in <strong>Buddy Agent</strong> (Buddy.app → Agent Settings).
    Without an Agent, first-run identity lives in the terminal: <code>buddy setup</code> or <code>buddy cred add</code>.
    This page edits workplace and auth <strong>metadata</strong> only. When the Agent owns config, the page is read-only.
    Vault secrets stay in the terminal: <code>buddy cred set secret &lt;auth&gt;</code> / <code>buddy setup secrets</code>.</p>
  <div id="banner" class="banner" style="display:none"></div>
  <div id="err" class="err"></div>

  <h2>Defaults</h2>
  <div class="form-grid" id="defaults-form">
    <label>Default context
      <select id="default-context"></select>
    </label>
    <label>CLI channel context
      <select id="ch-cli"></select>
    </label>
    <label>MCP channel context
      <select id="ch-mcp"></select>
    </label>
    <label>IDE channel context
      <select id="ch-ide"></select>
    </label>
    <div class="form-actions">
      <button type="button" class="primary" id="btn-save-defaults">Save defaults</button>
    </div>
  </div>


  <h2>Workplaces</h2>
  <p class="section-note">A context is a <strong>workplace name</strong> (<code>--context</code>), not a Jamf UUID.
    Jamf Account <strong>scope</strong> lives on the <em>auth</em>.
    See <a href="/docs/docs/auth#contexts-and-auths">Auth: contexts and auths</a>.
    Pencil renames; trash deletes (parents with children are blocked).</p>
  <div class="work">
    <div>
      <ul class="tree" id="tree"></ul>
      <div class="form-actions" style="margin-top:.75rem">
        <button type="button" id="btn-add-org">Add organization</button>
        <button type="button" id="btn-add-child">Add child</button>
      </div>
    </div>
    <div>
      <div class="form-grid" id="detail">
        <label>Name <input id="t-name" autocomplete="off" placeholder="sandbox"></label>
        <label>Kind <input id="t-kind" readonly></label>
        <label id="f-url">Instance URL
          <span class="why">Jamf Pro host. Blank for org or environment-only workplaces.</span>
          <input id="t-url" placeholder="https://example.jamfcloud.com">
        </label>
        <label id="f-region">Region
          <span class="why">Required for Account environments (us, eu, apac).</span>
          <input id="t-region" placeholder="us">
        </label>
        <label id="f-eid">Environment id
          <span class="why">Account environment UUID. Becomes <code>X-Environment-Id</code>.</span>
          <input id="t-eid" placeholder="11111111-1111-1111-1111-111111111111">
        </label>
        <label id="f-tid">Jamf Pro tenant id
          <span class="why">Jamf Pro product-tenant UUID for <code>scope: tenant</code>.</span>
          <input id="t-tid">
        </label>
        <label id="f-org">Parent org
          <span class="why">Organization workplace name.</span>
          <input id="t-org" placeholder="acme">
        </label>
        <label id="f-orgid">Org id
          <span class="why">Optional Account organization UUID (never sent as a header).</span>
          <input id="t-orgid">
        </label>
        <div class="form-actions">
          <button type="button" class="primary" id="btn-save-context">Save workplace</button>
        </div>
      </div>
    </div>
  </div>

  <h2>Auths</h2>
  <p class="section-note"><code>oauth</code> / <code>basic</code> talk to Jamf Pro (instance).
    <code>platform</code> talks to the Account gateway; <strong>scope</strong> chooses the header:
    <code>environment</code> → <code>X-Environment-Id</code>;
    <code>organization</code> → none;
    <code>tenant</code> → <code>X-Tenant-Id</code>.
    Selecting a workplace highlights the auths linked to it.</p>
  <div class="work">
    <div class="table-wrap">
      <table>
        <thead><tr><th>Name</th><th>Type</th><th>Scope</th><th>Contexts</th><th>Access</th><th>Secret</th><th></th></tr></thead>
        <tbody id="auths-body"></tbody>
      </table>
    </div>
    <div>
      <div class="form-grid" id="auth-form">
        <label>Name <input id="a-name" autocomplete="off"></label>
        <label>Type
          <select id="a-type">
            <option value="oauth">oauth</option>
            <option value="basic">basic</option>
            <option value="platform">platform</option>
          </select>
        </label>
        <label id="f-scope">Platform scope
          <span class="why">Only for type platform. environment is the GA default.</span>
          <select id="a-scope">
            <option value="">(instance — omit)</option>
            <option value="organization">organization — no env/tenant header</option>
            <option value="environment">environment — X-Environment-Id</option>
            <option value="tenant">tenant — X-Tenant-Id</option>
          </select>
        </label>
        <label>Access
          <select id="a-access">
            <option value="full">full</option>
            <option value="readonly">readonly</option>
          </select>
        </label>
        <label id="f-user">Username (basic) <input id="a-user"></label>
        <label id="f-cid">Client ID (oauth/platform) <input id="a-cid"></label>
        <label style="grid-column:1/-1">Linked contexts
          <span class="why">Workplaces this auth may use.</span>
          <div class="checks" id="a-contexts-boxes"></div>
        </label>
        <label style="grid-column:1/-1">Channels
          <span class="why">Blank / all three = every client.</span>
          <div class="checks">
            <label class="check"><input type="checkbox" id="a-ch-cli" checked> cli</label>
            <label class="check"><input type="checkbox" id="a-ch-mcp" checked> mcp</label>
            <label class="check"><input type="checkbox" id="a-ch-ide" checked> ide</label>
          </div>
        </label>
        <div class="form-actions">
          <button type="button" class="primary" id="btn-save-auth">Add / update auth</button>
        </div>
      </div>
    </div>
  </div>
</main>
` + chromeToggleScript(embed) + `
<script>
(function(){
  var state = null;
  var selected = null;
  var errEl = document.getElementById('err');
  function showErr(msg){
    errEl.textContent = msg || '';
    errEl.classList.toggle('show', !!msg);
  }
  function esc(s){
    return String(s == null ? '' : s)
      .replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
  }
  async function api(method, path, body){
    var opts = { method: method, headers: {} };
    if (body !== undefined) {
      opts.headers['Content-Type'] = 'application/json';
      opts.body = JSON.stringify(body);
    }
    var res = await fetch(path, opts);
    var data = await res.json().catch(function(){ return {}; });
    if (!res.ok) throw new Error(data.error || (res.status + ' ' + res.statusText));
    return data;
  }
  function workplaceRows(st){
    return st.contexts || st.tenants || [];
  }
  function walkTree(nodes, fn){
    (nodes || []).forEach(function(n){
      fn(n);
      walkTree(n.children, fn);
    });
  }
  function findNode(name){
    var found = null;
    walkTree(state && state.tree, function(n){ if (n.name === name) found = n; });
    if (found) return found;
    return workplaceRows(state).find(function(x){ return x.name === name; }) || null;
  }
  function defaultContext(st){
    return st.default_context || st.default_tenant || '';
  }
  function channelContexts(st){
    return st.channel_contexts || st.channel_tenants || {};
  }
  function authContexts(a){
    var c = a.contexts || [];
    if (c.length) return c;
    return a.tenants || [];
  }
  function contextOptions(selectedName, includeBlank){
    var html = includeBlank ? '<option value="">(from default)</option>' : '';
    workplaceRows(state).forEach(function(t){
      html += '<option value="'+esc(t.name)+'"'+(t.name===selectedName?' selected':'')+'>'+esc(t.name)+'</option>';
    });
    return html;
  }
  function kindLabel(k){
    if (k === 'tenant' || k === 'product-tenant') return 'jamfpro';
    return k || '';
  }
  function collectedContexts(){
    var out = [];
    document.querySelectorAll('#a-contexts-boxes input:checked').forEach(function(i){ out.push(i.value); });
    return out;
  }
  function collectedChannels(){
    var out = [];
    if (document.getElementById('a-ch-cli').checked) out.push('cli');
    if (document.getElementById('a-ch-mcp').checked) out.push('mcp');
    if (document.getElementById('a-ch-ide').checked) out.push('ide');
    if (out.length === 0 || out.length === 3) return [];
    return out;
  }
  function setChannelChecks(list){
    var all = !list || !list.length;
    document.getElementById('a-ch-cli').checked = all || list.indexOf('cli') >= 0;
    document.getElementById('a-ch-mcp').checked = all || list.indexOf('mcp') >= 0;
    document.getElementById('a-ch-ide').checked = all || list.indexOf('ide') >= 0;
  }
  function syncContextChecks(selectedNames){
    var box = document.getElementById('a-contexts-boxes');
    var names = workplaceRows(state).map(function(t){ return t.name; });
    var have = {};
    if (selectedNames) selectedNames.forEach(function(n){ have[n] = true; });
    else box.querySelectorAll('input:checked').forEach(function(i){ have[i.value] = true; });
    var html = '';
    names.forEach(function(n){
      html += '<label class="check"><input type="checkbox" value="'+esc(n)+'"'+(have[n]?' checked':'')+'> '+esc(n)+'</label>';
    });
    box.innerHTML = html || '<span class="muted">No workplaces</span>';
  }
  function syncAuthFields(){
    var typ = document.getElementById('a-type').value;
    setVis('f-scope', typ === 'platform');
    setVis('f-user', typ === 'basic');
    setVis('f-cid', typ === 'oauth' || typ === 'platform');
  }
  function renderNode(n){
    var canDel = !!n.can_delete && !n.managed && !n.agent && !state.config_readonly;
    var trashTitle = canDel ? 'Delete' : (n.managed ? 'Managed by MDM' : (n.agent ? 'Managed by Buddy Agent' : 'Delete children first'));
    var html = '<li><div class="tree-row'+(selected===n.name?' on':'')+'" data-sel="'+esc(n.name)+'">'+
      '<span class="badge">'+esc(kindLabel(n.kind))+'</span>'+
      '<span class="name">'+esc(n.name)+'</span>'+
      (n.managed?' <span class="badge">mdm</span>':'')+
      (n.agent?' <span class="badge">agent</span>':'')+
      (n.gap?' <span class="badge gap">gap</span>':'')+
      (n.url?' <span class="url">'+esc(n.url)+'</span>':'')+
      '<span class="grow"></span>'+
      (n.managed || n.agent || state.config_readonly?'':
        '<button type="button" class="icon" data-ren="'+esc(n.name)+'" title="Rename">✎</button>'+
        '<button type="button" class="icon danger" data-del="'+esc(n.name)+'" title="'+esc(trashTitle)+'" '+(canDel?'':'disabled')+'>🗑</button>')+
      '</div>';
    if (n.children && n.children.length) {
      html += '<ul class="tree">'+n.children.map(renderNode).join('')+'</ul>';
    }
    return html + '</li>';
  }
  function setVis(id, on){
    var el = document.getElementById(id);
    if (el) el.style.display = on ? '' : 'none';
  }
  function fillDetail(n){
    document.getElementById('t-name').value = n && n.name || '';
    document.getElementById('t-kind').value = n && kindLabel(n.kind) || '';
    document.getElementById('t-url').value = n && n.url || '';
    document.getElementById('t-region').value = n && n.region || '';
    document.getElementById('t-eid').value = n && n.environment_id || '';
    document.getElementById('t-tid').value = n && n.tenant_id || '';
    document.getElementById('t-org').value = n && n.org && n.kind !== 'organization' ? n.org : '';
    document.getElementById('t-orgid').value = n && n.org_id || '';
    var k = n && n.kind || '';
    setVis('f-url', !n || k === 'instance' || k === 'product-tenant' || k === 'tenant');
    setVis('f-region', !n || k === 'environment');
    setVis('f-eid', !n || k === 'environment');
    setVis('f-tid', !n || k === 'product-tenant' || k === 'tenant');
    setVis('f-org', !n || k !== 'organization');
    setVis('f-orgid', !n || k === 'organization');
  }
  function confirmDeleteWorkplace(n){
    var msg = 'Delete '+kindLabel(n.kind)+' "'+n.name+'"?';
    var removed = [], unlinked = [];
    (state.auths || []).forEach(function(a){
      var links = authContexts(a);
      if (links.indexOf(n.name) < 0) return;
      if (links.length === 1) removed.push(a.name); else unlinked.push(a.name);
    });
    if (removed.length) msg += '\nAuths that will be removed: '+removed.join(', ');
    if (unlinked.length) msg += '\nAuths that will be unlinked: '+unlinked.join(', ');
    if (!removed.length && !unlinked.length) msg += '\nNo auths linked.';
    return confirm(msg);
  }
  function render(){
    if (!state) return;
    showErr('');
    var banner = document.getElementById('banner');
    if (state.parse_error) {
      banner.style.display = '';
      banner.innerHTML = '<strong>config.yaml could not be parsed.</strong> '+esc(state.parse_error)+
        (state.last_good ? ' <button type="button" class="primary" id="btn-restore">Restore last good</button>' : '');
    } else if (state.config_readonly) {
      banner.style.display = '';
      banner.innerHTML = '<strong>Buddy Agent manages workplaces and auths.</strong> Edit them in Buddy Agent → Settings… (or turn off <code>features.use_agent</code> in config.yaml).';
    } else if (state.needs_setup) {
      banner.style.display = '';
      banner.innerHTML = '<strong>Not configured yet.</strong> On macOS, add a workplace in Buddy Agent (Buddy.app → Agent Settings). Or run <code>buddy setup</code> in a terminal. This page does not collect secrets. See <a href="/docs/docs/getting-started">Getting started</a>.';
    } else {
      banner.style.display = 'none';
    }
    document.body.classList.toggle('config-readonly', !!state.config_readonly);
    var rest = document.getElementById('btn-restore');
    if (rest) rest.onclick = async function(){
      try { state = await api('POST', '/api/config/restore'); selected = null; render(); }
      catch (e) { showErr(e.message); }
    };
    var def = defaultContext(state);
    var ch = channelContexts(state);
    document.getElementById('default-context').innerHTML = contextOptions(def, false);
    document.getElementById('ch-cli').innerHTML = contextOptions(ch.cli || '', true);
    document.getElementById('ch-mcp').innerHTML = contextOptions(ch.mcp || '', true);
    document.getElementById('ch-ide').innerHTML = contextOptions(ch.ide || '', true);
    var tree = state.tree && state.tree.length ? state.tree : workplaceRows(state).map(function(t){
      return { name: t.name, kind: t.kind, url: t.url, can_delete: !t.managed, managed: t.managed, org: t.org, region: t.region, environment_id: t.environment_id, tenant_id: t.tenant_id };
    });
    document.getElementById('tree').innerHTML = tree.length ? tree.map(renderNode).join('') : '<li class="muted">No workplaces</li>';
    fillDetail(findNode(selected));
    syncContextChecks();
    syncAuthFields();
    var ab = document.getElementById('auths-body');
    ab.innerHTML = '';
    var auths = state.auths || [];
    auths.forEach(function(a){
      var secretCls = a.secret === 'missing' ? 'gap' : 'ok';
      var links = authContexts(a);
      var cls = '';
      if (selected) cls = links.indexOf(selected) >= 0 ? 'linked' : 'other';
      var tr = document.createElement('tr');
      if (cls) tr.className = cls;
      tr.innerHTML = '<td><code data-edit-a="'+esc(a.name)+'" title="Edit metadata">'+esc(a.name)+'</code>'+(a.managed?' <span class="badge">mdm</span>':'')+(a.agent?' <span class="badge">agent</span>':'')+'</td>'+
        '<td>'+esc(a.type)+'</td><td>'+esc(a.scope||'')+'</td><td>'+esc(links.join(', '))+'</td>'+
        '<td>'+esc(a.access||'')+'</td><td><span class="badge '+secretCls+'">'+esc(a.secret)+'</span></td>'+
        '<td class="row-actions">'+(a.managed || a.agent || state.config_readonly?'':
          '<button type="button" class="icon" data-ren-a="'+esc(a.name)+'" title="Rename">✎</button>'+
          '<button type="button" class="icon danger" data-del-a="'+esc(a.name)+'" title="Delete">🗑</button>')+'</td>';
      ab.appendChild(tr);
    });
    if (!auths.length) ab.innerHTML = '<tr><td colspan="7" class="muted">No auths</td></tr>';
  }
  async function load(){
    state = await api('GET', '/api/config');
    render();
  }
  document.getElementById('btn-save-defaults').onclick = async function(){
    try {
      var def = document.getElementById('default-context').value;
      state = await api('PUT', '/api/config/selection', { default_context: def, default_tenant: def });
      async function setCh(ch, selId){
        var v = document.getElementById(selId).value;
        state = await api('PUT', '/api/config/selection', { channel: ch, channel_context: v, channel_tenant: v });
      }
      await setCh('cli', 'ch-cli');
      await setCh('mcp', 'ch-mcp');
      await setCh('ide', 'ch-ide');
      if (def) state = await api('PUT', '/api/config/selection', { default_context: def, default_tenant: def });
      render();
    } catch (e) { showErr(e.message); }
  };
  document.getElementById('btn-save-context').onclick = async function(){
    try {
      var name = document.getElementById('t-name').value.trim();
      if (!name) throw new Error('workplace name required');
      var prev = selected;
      state = await api('PUT', '/api/config/context/' + encodeURIComponent(name), {
        url: document.getElementById('t-url').value.trim(),
        region: document.getElementById('t-region').value.trim(),
        environment_id: document.getElementById('t-eid').value.trim(),
        tenant_id: document.getElementById('t-tid').value.trim(),
        org: document.getElementById('t-org').value.trim(),
        org_id: document.getElementById('t-orgid').value.trim()
      });
      if (prev && prev !== name) {
        try { await api('POST', '/api/config/rename', { kind: 'context', from: prev, to: name }); state = await api('GET', '/api/config'); } catch (e2) {}
      }
      selected = name;
      render();
    } catch (e) { showErr(e.message); }
  };
  document.getElementById('btn-add-org').onclick = function(){
    selected = null;
    fillDetail({ kind: 'organization' });
    document.getElementById('t-name').focus();
  };
  document.getElementById('btn-add-child').onclick = function(){
    var n = findNode(selected);
    var parent = n && (n.kind === 'organization' ? n.name : n.org) || '';
    fillDetail({ kind: n && n.kind === 'environment' ? 'product-tenant' : 'instance', org: parent || selected });
    document.getElementById('t-name').value = '';
    document.getElementById('t-name').focus();
  };
  document.getElementById('tree').onclick = async function(ev){
    var t = ev.target;
    if (t.dataset.ren) {
      var from = t.dataset.ren;
      var to = prompt('Rename '+from+' to:', from);
      if (!to || to === from) return;
      try {
        state = await api('POST', '/api/config/rename', { kind: 'context', from: from, to: to });
        selected = to;
        render();
      } catch (e) { showErr(e.message); }
      return;
    }
    if (t.dataset.del) {
      var n = findNode(t.dataset.del) || { name: t.dataset.del, kind: 'workplace' };
      if (!confirmDeleteWorkplace(n)) return;
      try {
        state = await api('DELETE', '/api/config/context/' + encodeURIComponent(t.dataset.del));
        if (selected === t.dataset.del) selected = null;
        render();
      } catch (e) { showErr(e.message); }
      return;
    }
    var row = t.closest && t.closest('[data-sel]');
    if (row && row.dataset.sel) {
      selected = row.dataset.sel;
      render();
    }
  };
  document.getElementById('a-type').onchange = syncAuthFields;
  document.getElementById('btn-save-auth').onclick = async function(){
    try {
      var name = document.getElementById('a-name').value.trim();
      if (!name) throw new Error('auth name required');
      var contexts = collectedContexts();
      var channels = collectedChannels();
      var typ = document.getElementById('a-type').value;
      state = await api('PUT', '/api/config/auth/' + encodeURIComponent(name), {
        type: typ,
        scope: typ === 'platform' ? document.getElementById('a-scope').value : '',
        contexts: contexts,
        tenants: contexts,
        access: document.getElementById('a-access').value,
        channels: channels,
        username: document.getElementById('a-user').value.trim(),
        client_id: document.getElementById('a-cid').value.trim()
      });
      render();
    } catch (e) { showErr(e.message); }
  };
  document.getElementById('auths-body').onclick = async function(ev){
    var t = ev.target;
    if (t.dataset.editA) {
      var row = (state.auths||[]).find(function(x){return x.name===t.dataset.editA;});
      if (!row) return;
      document.getElementById('a-name').value = row.name;
      document.getElementById('a-type').value = row.type || 'oauth';
      document.getElementById('a-scope').value = row.scope || '';
      document.getElementById('a-access').value = row.access || 'full';
      document.getElementById('a-user').value = row.username || '';
      document.getElementById('a-cid').value = row.client_id || '';
      setChannelChecks(row.channels || []);
      syncContextChecks(authContexts(row));
      syncAuthFields();
    }
    if (t.dataset.delA) {
      if (!confirm('Delete auth '+t.dataset.delA+' and vault secrets?')) return;
      try {
        state = await api('DELETE', '/api/config/auth/' + encodeURIComponent(t.dataset.delA));
        render();
      } catch (e) { showErr(e.message); }
    }
    if (t.dataset.renA) {
      var from = t.dataset.renA;
      var to = prompt('Rename auth '+from+' to:', from);
      if (!to || to === from) return;
      try {
        state = await api('POST', '/api/config/rename', { kind: 'auth', from: from, to: to });
        render();
      } catch (e) { showErr(e.message); }
    }
  };
  load().catch(function(e){ showErr(e.message); });
})();
</script>
</body>
</html>`
}
