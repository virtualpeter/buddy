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
	_, _ = w.Write([]byte(configHTML()))
}

func configHTML() string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>buddy config</title>
` + chromeHeadTags() + `
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
main { padding: 1.5rem 2rem 3rem; max-width: 68rem; }
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
.row-actions { white-space: nowrap; }
.row-actions button { margin-right: .25rem; }
</style>
</head>
<body>
` + chromeHeader("config", "", "", "") + `
<main>
  <h1>Config</h1>
  <p class="hint">First-run identity lives in the terminal: <code>buddy setup</code> or <code>buddy cred add</code>
    (instance JSON/basic, Platform curl, quickstart import, or MDM + secrets). This page edits workplace and auth
    <strong>metadata</strong> only. Passwords and client secrets stay in the vault:
    <code>buddy cred set secret &lt;auth&gt;</code> / <code>buddy setup secrets</code>.</p>
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

  <h2>Contexts</h2>
  <p class="section-note">A context is a <strong>workplace name</strong> (<code>--context</code>), not a Jamf UUID.
    Jamf Account <strong>scope</strong> lives on the <em>auth</em>.
    See <a href="/docs/docs/auth#contexts-and-auths">Auth: contexts and auths</a>.</p>
  <ul class="section-note">
    <li><strong>Instance</strong> — Jamf Pro URL only. No region or Account ids.</li>
    <li><strong>Organization</strong> — name only. US gateway, <em>no</em> <code>X-Environment-Id</code> / <code>X-Tenant-Id</code>.</li>
    <li><strong>Environment</strong> — <code>region</code> + Account <em>environment</em> UUID. Pair with a <code>platform</code> auth whose scope is <code>environment</code> (sends <code>X-Environment-Id</code>). This is the usual GA path for blueprints/devices.</li>
    <li><strong>Product tenant</strong> — same region + environment id, plus Jamf Pro <em>tenant</em> UUID (optional instance URL). Auth scope <code>tenant</code> sends <code>X-Tenant-Id</code>.</li>
  </ul>
  <table>
    <thead><tr><th>Org</th><th>Environment</th><th>Context</th><th>Kind</th><th>URL</th><th></th></tr></thead>
    <tbody id="contexts-body"></tbody>
  </table>
  <div class="form-grid">
    <label>Name <input id="t-name" autocomplete="off" placeholder="sandbox"></label>
    <label>Instance URL
      <span class="why">Jamf Pro host. Blank for org or environment-only workplaces.</span>
      <input id="t-url" placeholder="https://example.jamfcloud.com">
    </label>
    <label>Region
      <span class="why">Required for environment / product-tenant (us, eu, apac). Omit on org workplaces.</span>
      <input id="t-region" placeholder="us">
    </label>
    <label>Environment id
      <span class="why">Account environment UUID from Jamf Account → Integrations. Becomes <code>X-Environment-Id</code>.</span>
      <input id="t-eid" placeholder="11111111-1111-1111-1111-111111111111">
    </label>
    <label>Product tenant id
      <span class="why">Jamf Pro product-tenant UUID. Only for a tenant workplace / <code>scope: tenant</code>.</span>
      <input id="t-tid" placeholder="leave blank for environment">
    </label>
    <label>Parent org
      <span class="why">Optional workplace name of the organization (nested <code>orgs:</code>).</span>
      <input id="t-org" placeholder="acme">
    </label>
    <div class="form-actions">
      <button type="button" class="primary" id="btn-save-context">Add / update context</button>
    </div>
  </div>

  <h2>Auths</h2>
  <p class="section-note"><code>oauth</code> / <code>basic</code> talk to Jamf Pro (instance).
    <code>platform</code> talks to the Account gateway; <strong>scope</strong> chooses the header:
    <code>environment</code> → <code>X-Environment-Id</code> from the linked environment workplace;
    <code>organization</code> → none (link the org workplace only; not inherited by env/tenant);
    <code>tenant</code> → <code>X-Tenant-Id</code>.
    Product APIs (blueprints, devices) want an environment-scoped platform auth.</p>
  <table>
    <thead><tr><th>Name</th><th>Type</th><th>Scope</th><th>Contexts</th><th>Access</th><th>Secret</th><th></th></tr></thead>
    <tbody id="auths-body"></tbody>
  </table>
  <div class="form-grid">
    <label>Name <input id="a-name" autocomplete="off"></label>
    <label>Type
      <select id="a-type">
        <option value="oauth">oauth</option>
        <option value="basic">basic</option>
        <option value="platform">platform</option>
      </select>
    </label>
    <label>Linked contexts (csv) <input id="a-contexts" placeholder="sandbox"></label>
    <label>Platform scope
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
    <label>Channels (csv, blank=all) <input id="a-channels" placeholder="cli,mcp,ide"></label>
    <label>Username (basic) <input id="a-user"></label>
    <label>Client ID (oauth/platform) <input id="a-cid"></label>
    <div class="form-actions">
      <button type="button" class="primary" id="btn-save-auth">Add / update auth</button>
    </div>
  </div>

  <h2>Coverage</h2>
  <p class="section-note">Each row is a workplace × <em>surface</em> (instance URL vs Platform gateway) — not Jamf Account scope.
    An environment workplace needs a <code>platform</code> auth with scope <code>environment</code>.</p>
  <table>
    <thead><tr><th>Org</th><th>Environment</th><th>Context</th><th>Kind</th><th>Surface</th><th>Status</th><th>Auths</th></tr></thead>
    <tbody id="cov-body"></tbody>
  </table>
</main>
` + chromeToggleScript() + `
<script>
(function(){
  var state = null;
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
  function workplaceKind(t){
    if (t.kind) return t.kind;
    if (t.organization) return 'organization';
    if (t.tenant_id) return 'tenant';
    if (t.environment_id || (t.region && !t.url)) return 'environment';
    return 'instance';
  }
  function workplaceKindLabel(k){
    if (k === 'tenant' || k === 'product-tenant') return 'product tenant';
    return k || '';
  }
  function workplaceRows(st){
    return st.contexts || st.tenants || [];
  }
  function defaultContext(st){
    return st.default_context || st.default_tenant || '';
  }
  function channelContexts(st){
    return st.channel_contexts || st.channel_tenants || {};
  }
  function authContexts(a){
    return a.contexts || a.tenants || [];
  }
  function contextOptions(selected, includeBlank){
    var html = includeBlank ? '<option value="">(from default)</option>' : '';
    workplaceRows(state).forEach(function(t){
      html += '<option value="'+esc(t.name)+'"'+(t.name===selected?' selected':'')+'>'+esc(t.name)+'</option>';
    });
    return html;
  }
  function render(){
    if (!state) return;
    showErr('');
    var banner = document.getElementById('banner');
    if (state.needs_setup) {
      banner.style.display = '';
      banner.innerHTML = '<strong>Not configured yet.</strong> Run <code>buddy setup</code> or <code>buddy cred add</code> in a terminal (instance JSON/basic, Platform curl, <code>setup import</code>, or MDM + <code>setup secrets</code>). This page does not collect secrets. See <a href="/docs/docs/getting-started">Getting started</a>.';
    } else {
      banner.style.display = 'none';
    }
    var def = defaultContext(state);
    var ch = channelContexts(state);
    document.getElementById('default-context').innerHTML = contextOptions(def, false);
    document.getElementById('ch-cli').innerHTML = contextOptions(ch.cli || '', true);
    document.getElementById('ch-mcp').innerHTML = contextOptions(ch.mcp || '', true);
    document.getElementById('ch-ide').innerHTML = contextOptions(ch.ide || '', true);

    var tb = document.getElementById('contexts-body');
    tb.innerHTML = '';
    workplaceRows(state).forEach(function(t){
      var kind = workplaceKind(t);
      var org = t.org || (kind === 'organization' ? t.name : '');
      var env = t.env || (kind === 'environment' ? t.name : '');
      var url = (kind === 'instance' || kind === 'tenant' || kind === 'product-tenant') ? (t.url || '') : '';
      var tr = document.createElement('tr');
      tr.innerHTML = '<td>'+esc(org)+'</td><td>'+esc(env)+'</td>'+
        '<td><code>'+esc(t.name)+'</code>'+(t.managed?' <span class="badge">mdm</span>':'')+'</td>'+
        '<td><span class="badge">'+esc(workplaceKindLabel(kind))+'</span></td>'+
        '<td>'+esc(url)+'</td>'+
        '<td class="row-actions"><button type="button" data-edit-t="'+esc(t.name)+'">Edit</button>'+
        '<button type="button" class="danger" data-del-t="'+esc(t.name)+'">Delete</button></td>';
      tb.appendChild(tr);
    });
    if (!workplaceRows(state).length) {
      tb.innerHTML = '<tr><td colspan="6" class="muted">No contexts</td></tr>';
    }

    var ab = document.getElementById('auths-body');
    ab.innerHTML = '';
    (state.auths || []).forEach(function(a){
      var secretCls = a.secret === 'missing' ? 'gap' : 'ok';
      var tr = document.createElement('tr');
      tr.innerHTML = '<td><code>'+esc(a.name)+'</code>'+(a.managed?' <span class="badge">mdm</span>':'')+'</td>'+
        '<td>'+esc(a.type)+'</td><td>'+esc(a.scope||'')+'</td><td>'+esc(authContexts(a).join(', '))+'</td>'+
        '<td>'+esc(a.access)+'</td><td><span class="badge '+secretCls+'">'+esc(a.secret)+'</span></td>'+
        '<td class="row-actions"><button type="button" data-edit-a="'+esc(a.name)+'">Edit</button>'+
        '<button type="button" class="danger" data-del-a="'+esc(a.name)+'">Delete</button></td>';
      ab.appendChild(tr);
    });
    if (!(state.auths || []).length) {
      ab.innerHTML = '<tr><td colspan="7" class="muted">No auths</td></tr>';
    }

    var cb = document.getElementById('cov-body');
    cb.innerHTML = '';
    var cells = (state.coverage && state.coverage.cells) || [];
    cells.forEach(function(c){
      var auths = (c.auths||[]).map(function(x){ return x.name+' ('+x.secret+')'; }).join(', ') || '—';
      var st = c.gap ? '<span class="badge gap">gap</span> '+esc(c.gap_reason||'') : '<span class="badge ok">ok</span>';
      var tr = document.createElement('tr');
      tr.innerHTML = '<td>'+esc(c.org||'')+'</td><td>'+esc(c.environment||'')+'</td>'+
        '<td>'+esc(c.context||c.tenant)+'</td><td>'+esc(c.kind||'')+'</td><td>'+esc(c.surface)+'</td><td>'+st+'</td><td>'+esc(auths)+'</td>';
      cb.appendChild(tr);
    });
    if (!cells.length) {
      cb.innerHTML = '<tr><td colspan="7" class="muted">No coverage rows</td></tr>';
    }
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
      if (!name) throw new Error('context name required');
      state = await api('PUT', '/api/config/context/' + encodeURIComponent(name), {
        url: document.getElementById('t-url').value.trim(),
        region: document.getElementById('t-region').value.trim(),
        environment_id: document.getElementById('t-eid').value.trim(),
        tenant_id: document.getElementById('t-tid').value.trim(),
        org: document.getElementById('t-org').value.trim()
      });
      render();
    } catch (e) { showErr(e.message); }
  };
  document.getElementById('btn-save-auth').onclick = async function(){
    try {
      var name = document.getElementById('a-name').value.trim();
      if (!name) throw new Error('auth name required');
      var contexts = document.getElementById('a-contexts').value.split(',').map(function(s){return s.trim();}).filter(Boolean);
      var channels = document.getElementById('a-channels').value.split(',').map(function(s){return s.trim();}).filter(Boolean);
      state = await api('PUT', '/api/config/auth/' + encodeURIComponent(name), {
        type: document.getElementById('a-type').value,
        scope: document.getElementById('a-scope').value,
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
  document.getElementById('contexts-body').onclick = async function(ev){
    var t = ev.target;
    if (t.dataset.editT) {
      var row = workplaceRows(state).find(function(x){return x.name===t.dataset.editT;});
      if (!row) return;
      document.getElementById('t-name').value = row.name;
      document.getElementById('t-url').value = row.url || '';
      document.getElementById('t-region').value = row.region || '';
      document.getElementById('t-eid').value = row.environment_id || '';
      document.getElementById('t-tid').value = row.tenant_id || '';
      document.getElementById('t-org').value = row.org || '';
    }
    if (t.dataset.delT) {
      if (!confirm('Delete context '+t.dataset.delT+'? Linked sole auths and their secrets will be removed.')) return;
      try {
        state = await api('DELETE', '/api/config/context/' + encodeURIComponent(t.dataset.delT));
        render();
      } catch (e) { showErr(e.message); }
    }
  };
  document.getElementById('auths-body').onclick = async function(ev){
    var t = ev.target;
    if (t.dataset.editA) {
      var row = (state.auths||[]).find(function(x){return x.name===t.dataset.editA;});
      if (!row) return;
      document.getElementById('a-name').value = row.name;
      document.getElementById('a-type').value = row.type || 'oauth';
      document.getElementById('a-scope').value = row.scope || '';
      document.getElementById('a-contexts').value = authContexts(row).join(', ');
      document.getElementById('a-access').value = row.access || 'full';
      document.getElementById('a-channels').value = (row.channels||[]).join(', ');
      document.getElementById('a-user').value = row.username || '';
      document.getElementById('a-cid').value = row.client_id || '';
    }
    if (t.dataset.delA) {
      if (!confirm('Delete auth '+t.dataset.delA+' and vault secrets?')) return;
      try {
        state = await api('DELETE', '/api/config/auth/' + encodeURIComponent(t.dataset.delA));
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
