# Buddy MCP server

**buddy** can run as a [Model Context Protocol](https://modelcontextprotocol.io/) server so Cursor (and other MCP hosts) call Jamf helpers as tools instead of shelling out.

```sh
buddy mcp
```

Uses stdio JSON-RPC. The active credential comes from `~/.config/buddy/config.yaml` (override with global `--user=<name>` on the command in mcp.json). MCP inherits that credential’s Jamf privileges—treat it like the CLI.

A Platform API Gateway credential (`region`/`url` + `tenantId`) works the same way: Pro tools and the first-class `blueprint*` tools route through the gateway.

---

## Cursor setup

### 1. Install a current buddy

```sh
cd /path/to/buddy
make buddy
make install   # copies to ~/go/bin/buddy
```

Confirm:

```sh
buddy --version
buddy help mcp
```

### 2. Add an MCP server entry

**Project** (this repo): [`.cursor/mcp.json`](../.cursor/mcp.json)  
**User-wide**: `~/.cursor/mcp.json`

Prefer an **absolute path** to the binary. MCP subprocesses often do not inherit your interactive shell `PATH`.

```json
{
  "mcpServers": {
    "buddy": {
      "command": "/Users/YOU/go/bin/buddy",
      "args": ["mcp"]
    }
  }
}
```

| Field | Why |
|-------|-----|
| `command` | Absolute path avoids “command not found” when Cursor spawns the server |
| `args: ["mcp"]` | Starts the stdio MCP server |

Docs, `AGENTS.md`, examples, and Cursor rules are **embedded in the buddy binary** and exposed as `buddy://docs/...` resources. `BUDDY_DOCS_ROOT` is optional (live override while editing sources). After changing docs/rules, run `make sync-docfs` (or `make buddy`) and reload MCP.

A minimal example also lives at [`examples/mcp.json`](../examples/mcp.json).

### 3. Reload MCP in Cursor

Open **Cursor Settings → MCP**, enable **buddy**, and refresh (or restart Cursor). The server should show as ready with tools such as `auth_test`, `smart_groups`, `starlark_eval`.

### 4. Use it in chat

Ask in natural language; the agent calls tools for you:

- “Test my Jamf auth with buddy”
- “List Jamf smart groups”
- “Find computer IDs where name contains lab”
- “Resolve policy ID for trigger install-foo-1.0”
- “List Platform blueprints” (calls `blueprints`; needs a gateway credential)
- “Show buddy builtin conventions / AGENTS.md via docs”

Optional tool argument `cred` selects a named credential from config.

---

## Tools

Tool names follow the Starlark v3 plural/singular grammar (see [builtins-v3.md](builtins-v3.md)).

| Tool | Returns `value` | Notes |
|------|-----------------|-------|
| `auth_test` | Compact auth summary | Set `verbose: true` for full privileges payload (large) |
| `smart_groups` | List of `{id,name,description}` | `filter`, `page`, `page_size`; omit `page` = all pages |
| `static_groups` | Same shape | Same paging |
| `computers` | Summary dicts (or IDs if `ids=true`) | RSQL `filter` |
| `scripts` | Script summaries | Name substring or RSQL |
| `packages` | Package summaries | RSQL `filter` |
| `policy` | `{id, trigger}` | Resolve by trigger name |
| `blueprints` | Blueprint summaries | Platform gateway credential; `search`, `sort`, paging |
| `blueprint` | Full blueprint JSON | By `blueprint_id` |
| `blueprint_status` | Deployment report | By `blueprint_id` |
| `blueprint_components` | Components list or one | Omit `identifier` = all |
| `blueprint_create` / `blueprint_set` / `blueprint_delete` / `blueprint_deploy` / `blueprint_undeploy` | `{id, …}` | **Mutating** (gateway credential) |
| `jamf_schema` | Operations catalog (+ optional docs) | Live Pro OpenAPI + Classic Swagger + embedded Platform specs; see below |
| `starlark_eval` | `{stdout, stderr, result, error}` | Fresh session; see below |
| `docs` | Catalog entries or page markdown | Embedded buddy docs/rules; see below |

### Resources

MCP also exposes the **full agent knowledge** catalog as `buddy://docs/...` resources (broader than the curated `buddy serve` HTML UI). Hosts that support resource pickers can attach them; **agents that call tools should use the `docs` tool** (same catalog) so conventions and rules are loaded during chat.

| URI | Content |
|-----|---------|
| `buddy://docs/builtins` | Starlark builtins (Markdown) |
| `buddy://docs/docs/*` | User docs + `builtins-v3` migration map |
| `buddy://docs/agents` | `AGENTS.md` (auth, secrets, contributor context) |
| `buddy://docs/readme` | `README.md` |
| `buddy://docs/rules/*` | Cursor rules (CLI, builtins, auth-secrets) |
| `buddy://docs/examples/*` | Example scripts and mcp.json |
| `buddy://docs/help/*` | CLI help (root, shell, starlark) |
| `buddy://docs/*.md` | Legacy aliases for former disk filenames |

The HTML docs UI shows only user-facing guides (no AGENTS/rules/history dumps).

---

## Response shape (important)

Every tool returns a **JSON object** (never a bare array) as both text content and structured content:

```json
{
  "ok": true,
  "value": [ ... ],
  "count": 4
}
```

On failure:

```json
{
  "ok": false,
  "error": "..."
}
```

**Why:** Some MCP hosts (including Cursor) validate `structuredContent` as an object. Returning a raw list caused errors such as `expected record, received array`. List payloads always live under `value`, with optional `count`.

This mirrors Starlark builtins (`{"ok", "value"}` / `{"ok", "error"}`).

### `auth_test` summary vs verbose

Default `value` is small and agent-friendly:

```json
{
  "ok": true,
  "value": {
    "username": "buddy-cli",
    "authenticationType": "CLIENT_CREDENTIALS",
    "accessLevel": "FullAccess",
    "privilegeSet": "CUSTOM",
    "privilegeCount": 420,
    "sites": [ ... ]
  }
}
```

Use `verbose: true` only when you need the full privilege strings.

### `jamf_schema` tips

Learn endpoints from the Jamf host schemas plus the embedded Platform specs:

| `api` | Source |
|-------|--------|
| `all` (default) | Jamf Pro OpenAPI + Classic Swagger + Platform specs |
| `both` | Jamf Pro OpenAPI **and** Classic Swagger |
| `pro` | `GET /api/schema` (OpenAPI 3, live) |
| `classic` | `GET /classicapi/doc/swagger.yaml` (Swagger 2, live) |
| `platform` | Embedded Platform API Gateway specs (no credential needed) |

- The Platform API Gateway has no live schema endpoint, so its specs (blueprints, devices, device-groups, device-actions, ddm/report, compliance-benchmarks) are vendored into the binary. Platform paths appear with their gateway base, e.g. `/api/blueprints/v1/tenant/{tenantId}/blueprints`.
- No schema requires authentication: the Pro/Classic schema documents are served publicly by the Jamf host (only data endpoints need a bearer token), and Platform specs are embedded. buddy sends no `Authorization` header for these fetches — the credential is used only to resolve the host URL for `pro`/`classic`, so discovery works even with a secret-less credential.
- Prefer `filter` (e.g. `"blueprints deploy"`, `"policy delete"`) — space-separated terms are AND-matched against path, method, tags, summary, operationId, description.
- Default return is a compact `operations` list (`path`, `method`, `summary`, `api`, …). Set `full: true` only when you need parameter/response schemas.
- Use this before inventing paths; then call typed tools or `starlark_eval` / `jamf_get` for live data.

### `docs` tips

Read embedded buddy documentation and Cursor rules (same data as `buddy://docs/...` resources):

| `action` | Args | Returns |
|----------|------|---------|
| `list` (default) | optional `query` | `[{id, title, section, uri}, …]` |
| `get` | `id` required | `{id, title, uri, markdown}` |

- Prefer `docs` before inventing buddy CLI/Starlark conventions.
- Typical flow: `docs` with `query: "builtin"` or `query: "agents"`, then `action: "get"` with the page `id`.
- `id` may be a short id (`agents`, `builtins`, `rules/buddy-builtins`) or a full `buddy://docs/…` URI (including legacy `….md` aliases).

### Blueprint tips

- All `blueprint*` tools require a **Platform API Gateway credential** (`region`/`url` + `tenantId`); they return `{ok:false, error}` otherwise.
- Read-only: `blueprints`, `blueprint`, `blueprint_status`, `blueprint_components`. Mutating: `blueprint_create`, `blueprint_set`, `blueprint_delete`, `blueprint_deploy`, `blueprint_undeploy`.
- Mutating tools **do not** prompt for confirmation (there is no interactive stdin in MCP) — the host’s tool-call approval is the guard. Treat them like `-y`.
- `blueprint_create` / `blueprint_set` take a JSON `body` object; `blueprint_set` is a merge-patch.

### `starlark_eval` tips

- Prefer a **single expression**, or define `main()` **without** calling it (MCP sets `CallMain` and invokes `main` for you).
- Top-level `if` / `for` outside a function is invalid Starlark—put control flow inside `main()`.
- Mutating Jamf calls still respect confirmations unless the MCP process was started with global `-y`.
- You can `load("stdlib/….buddy", …)` the same as in CLI scripts / the notebook (e.g. `load("stdlib/release.buddy", "bump_release")`).

Example:

```python
def main():
    r = smart_groups()
    if not r["ok"]:
        return r
    return {"count": len(r["value"]), "names": [g["name"] for g in r["value"]]}
```
---

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| Server missing / failed to start | Use absolute `command` path; run `buddy mcp` in a terminal to see errors |
| Docs resources empty / stale | Rebuild buddy (`make install`) after `make sync-docfs`; reload MCP |
| Auth / API errors | Fix config with `buddy auth test`; check `--user` / default cred |
| Agent can’t use list tools | Rebuild buddy ≥ response-envelope change; reload MCP |
| `starlark_eval` “if not within a function” | Wrap logic in `def main(): ...` |

---

## Security

- MCP runs as your user and uses your buddy secrets (Keychain / file store).
- Do not point a remote or shared MCP host at production credentials without review.
- Prefer read-only exploration tools first; treat `starlark_eval` like an interactive shell.
