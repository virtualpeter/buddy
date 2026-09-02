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
main { padding: 1.5rem 2rem 3rem; max-width: 56rem; }
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
  <p class="hint">Edit tenants and auth <strong>metadata</strong> only. Passwords and client secrets stay in the terminal vault:
    <code>buddy cred set secret &lt;auth&gt;</code> or <code>buddy cred add</code>.</p>
  <div id="banner" class="banner" style="display:none"></div>
  <div id="err" class="err"></div>

  <h2>Defaults</h2>
  <div class="form-grid" id="defaults-form">
    <label>Default tenant
      <select id="default-tenant"></select>
    </label>
    <label>CLI channel tenant
      <select id="ch-cli"></select>
    </label>
    <label>MCP channel tenant
      <select id="ch-mcp"></select>
    </label>
    <label>IDE channel tenant
      <select id="ch-ide"></select>
    </label>
    <div class="form-actions">
      <button type="button" class="primary" id="btn-save-defaults">Save defaults</button>
    </div>
  </div>

  <h2>Tenants</h2>
  <table>
    <thead><tr><th>Name</th><th>URL</th><th>Region</th><th>Tenant ID</th><th></th></tr></thead>
    <tbody id="tenants-body"></tbody>
  </table>
  <div class="form-grid">
    <label>Name <input id="t-name" autocomplete="off"></label>
    <label>Instance URL <input id="t-url" placeholder="https://… (blank ok)"></label>
    <label>Platform region <input id="t-region" placeholder="us|eu|apac"></label>
    <label>Platform tenant id <input id="t-tid"></label>
    <div class="form-actions">
      <button type="button" class="primary" id="btn-save-tenant">Add / update tenant</button>
    </div>
  </div>

  <h2>Auths</h2>
  <table>
    <thead><tr><th>Name</th><th>Type</th><th>Tenants</th><th>Access</th><th>Secret</th><th></th></tr></thead>
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
    <label>Linked tenants (csv) <input id="a-tenants" placeholder="sandbox"></label>
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
  <table>
    <thead><tr><th>Tenant</th><th>Surface</th><th>Status</th><th>Auths</th></tr></thead>
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
  function tenantOptions(selected, includeBlank){
    var html = includeBlank ? '<option value="">(from default)</option>' : '';
    (state.tenants || []).forEach(function(t){
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
      banner.innerHTML = '<strong>Not configured yet.</strong> Add a tenant below, then an auth. Store secrets in a terminal: <code>buddy cred set secret &lt;auth&gt;</code>. See <a href="/docs/docs/getting-started" target="buddy-docs">Getting started</a>.';
    } else {
      banner.style.display = 'none';
    }
    var def = state.default_tenant || '';
    var ch = state.channel_tenants || {};
    document.getElementById('default-tenant').innerHTML = tenantOptions(def, false);
    document.getElementById('ch-cli').innerHTML = tenantOptions(ch.cli || '', true);
    document.getElementById('ch-mcp').innerHTML = tenantOptions(ch.mcp || '', true);
    document.getElementById('ch-ide').innerHTML = tenantOptions(ch.ide || '', true);
    // Mark channel selects: if channel equals default, show blank option selected when policy uses default
    // (effective value is still shown in options list)

    var tb = document.getElementById('tenants-body');
    tb.innerHTML = '';
    (state.tenants || []).forEach(function(t){
      var tr = document.createElement('tr');
      tr.innerHTML = '<td><code>'+esc(t.name)+'</code>'+(t.managed?' <span class="badge">mdm</span>':'')+'</td>'+
        '<td>'+esc(t.url||'')+'</td><td>'+esc(t.region||'')+'</td><td>'+esc(t.tenant_id||'')+'</td>'+
        '<td class="row-actions"><button type="button" data-edit-t="'+esc(t.name)+'">Edit</button>'+
        '<button type="button" class="danger" data-del-t="'+esc(t.name)+'">Delete</button></td>';
      tb.appendChild(tr);
    });
    if (!(state.tenants || []).length) {
      tb.innerHTML = '<tr><td colspan="5" class="muted">No tenants</td></tr>';
    }

    var ab = document.getElementById('auths-body');
    ab.innerHTML = '';
    (state.auths || []).forEach(function(a){
      var secretCls = a.secret === 'missing' ? 'gap' : 'ok';
      var tr = document.createElement('tr');
      tr.innerHTML = '<td><code>'+esc(a.name)+'</code>'+(a.managed?' <span class="badge">mdm</span>':'')+'</td>'+
        '<td>'+esc(a.type)+'</td><td>'+esc((a.tenants||[]).join(', '))+'</td>'+
        '<td>'+esc(a.access)+'</td><td><span class="badge '+secretCls+'">'+esc(a.secret)+'</span></td>'+
        '<td class="row-actions"><button type="button" data-edit-a="'+esc(a.name)+'">Edit</button>'+
        '<button type="button" class="danger" data-del-a="'+esc(a.name)+'">Delete</button></td>';
      ab.appendChild(tr);
    });
    if (!(state.auths || []).length) {
      ab.innerHTML = '<tr><td colspan="6" class="muted">No auths</td></tr>';
    }

    var cb = document.getElementById('cov-body');
    cb.innerHTML = '';
    var cells = (state.coverage && state.coverage.cells) || [];
    cells.forEach(function(c){
      var auths = (c.auths||[]).map(function(x){ return x.name+' ('+x.secret+')'; }).join(', ') || '—';
      var st = c.gap ? '<span class="badge gap">gap</span> '+esc(c.gap_reason||'') : '<span class="badge ok">ok</span>';
      var tr = document.createElement('tr');
      tr.innerHTML = '<td>'+esc(c.tenant)+'</td><td>'+esc(c.surface)+'</td><td>'+st+'</td><td>'+esc(auths)+'</td>';
      cb.appendChild(tr);
    });
    if (!cells.length) {
      cb.innerHTML = '<tr><td colspan="4" class="muted">No coverage rows</td></tr>';
    }
  }
  async function load(){
    state = await api('GET', '/api/config');
    render();
  }
  document.getElementById('btn-save-defaults').onclick = async function(){
    try {
      var def = document.getElementById('default-tenant').value;
      state = await api('PUT', '/api/config/selection', { default_tenant: def });
      // channel tenants: set each when different from blank-as-default
      async function setCh(ch, selId){
        var v = document.getElementById(selId).value;
        state = await api('PUT', '/api/config/selection', { channel: ch, channel_tenant: v });
      }
      await setCh('cli', 'ch-cli');
      await setCh('mcp', 'ch-mcp');
      await setCh('ide', 'ch-ide');
      if (def) state = await api('PUT', '/api/config/selection', { default_tenant: def });
      render();
    } catch (e) { showErr(e.message); }
  };
  document.getElementById('btn-save-tenant').onclick = async function(){
    try {
      var name = document.getElementById('t-name').value.trim();
      if (!name) throw new Error('tenant name required');
      state = await api('PUT', '/api/config/tenant/' + encodeURIComponent(name), {
        url: document.getElementById('t-url').value.trim(),
        region: document.getElementById('t-region').value.trim(),
        tenant_id: document.getElementById('t-tid').value.trim()
      });
      render();
    } catch (e) { showErr(e.message); }
  };
  document.getElementById('btn-save-auth').onclick = async function(){
    try {
      var name = document.getElementById('a-name').value.trim();
      if (!name) throw new Error('auth name required');
      var tenants = document.getElementById('a-tenants').value.split(',').map(function(s){return s.trim();}).filter(Boolean);
      var channels = document.getElementById('a-channels').value.split(',').map(function(s){return s.trim();}).filter(Boolean);
      state = await api('PUT', '/api/config/auth/' + encodeURIComponent(name), {
        type: document.getElementById('a-type').value,
        tenants: tenants,
        access: document.getElementById('a-access').value,
        channels: channels,
        username: document.getElementById('a-user').value.trim(),
        client_id: document.getElementById('a-cid').value.trim()
      });
      render();
    } catch (e) { showErr(e.message); }
  };
  document.getElementById('tenants-body').onclick = async function(ev){
    var t = ev.target;
    if (t.dataset.editT) {
      var row = (state.tenants||[]).find(function(x){return x.name===t.dataset.editT;});
      if (!row) return;
      document.getElementById('t-name').value = row.name;
      document.getElementById('t-url').value = row.url || '';
      document.getElementById('t-region').value = row.region || '';
      document.getElementById('t-tid').value = row.tenant_id || '';
    }
    if (t.dataset.delT) {
      if (!confirm('Delete tenant '+t.dataset.delT+'? Linked sole auths and their secrets will be removed.')) return;
      try {
        state = await api('DELETE', '/api/config/tenant/' + encodeURIComponent(t.dataset.delT));
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
      document.getElementById('a-tenants').value = (row.tenants||[]).join(', ');
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
