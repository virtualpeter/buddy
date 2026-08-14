# Buddy MCP server

**buddy** can run as a [Model Context Protocol](https://modelcontextprotocol.io/) server so AI tools (Cursor, Claude Desktop, and other MCP hosts) call Jamf helpers as tools instead of shelling out.

```sh
buddy mcp
```

Uses **stdio** JSON-RPC (the first-class product path). The active tenant/auth comes from `~/.config/buddy/config.yaml` (override with `--tenant=<name>` and/or `--user=<auth>` on the command in the host’s MCP config). MCP inherits that session’s Jamf privileges—treat it like the CLI.

A Platform-capable tenant (`region`/`url` + `tenantId`) with a linked oauth auth works the same way: Platform product builtins (`blueprint*`, `device*`, `device_group*`, `ddm_*`, `benchmark*`) route through the gateway via `buddyscript`.

Streamable HTTP MCP under `buddy serve` is **not** offered today. Hosts (Cursor, Claude Desktop) spawn `buddy mcp` over stdio; keep that as the supported setup. A localhost streamable endpoint may be revisited later for HTTP-only clients or long-lived debug workflows — not as a replacement for stdio.

---

## Auth layers (stdio vs Inspector vs remote MCP)

Three different “tokens” get confused. Only the middle one is Jamf:

| Layer | What it is | Buddy stdio? |
|-------|------------|--------------|
| **Host → buddy** | Cursor/Claude spawn a local process; no MCP Bearer | Yes — this is how buddy MCP works |
| **Buddy → Jamf** | Config + Keychain/secrets → Jamf bearer inside buddy | Yes — tools call Jamf; the host never sees that bearer |
| **Remote MCP OAuth** | HTTPS MCP (e.g. `developer.jamf.com/mcp`) may require OAuth / `Authorization: Bearer` to talk to *that* server | No — unrelated to `buddy mcp` |

**MCP Inspector 2.x** adds a fourth layer that is *neither* Jamf nor buddy:

- Inspector runs a **local proxy** that can spawn stdio servers (including `buddy mcp`) or connect to HTTP MCP targets.
- On start it prints a random **proxy session token** (or use `MCP_PROXY_AUTH_TOKEN`) and requires `x-mcp-proxy-auth: Bearer …` so a random browser tab cannot drive the proxy.
- Open the **full** URL Inspector prints (including `?MCP_PROXY_AUTH_TOKEN=…`). Opening bare `http://localhost:6274` without the token often looks like “stdio doesn’t work.”
- If the default browser (e.g. corporate Edge) fails to open or strips the query string, paste that full URL manually. Cursor can still use buddy via stdio even when Inspector’s browser path is broken.
- `DANGEROUSLY_OMIT_AUTH=true` disables proxy auth for local debug only — do not leave it on.

Buddy does not implement Inspector’s proxy token and does not require a Bearer for stdio.

---

## Setup

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

### 2. Register buddy with your AI host

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
| `command` | Absolute path avoids “command not found” when the host spawns the server |
| `args: ["mcp"]` | Starts the stdio MCP server |

**Discover / write well-known configs:**

```sh
buddy mcp setup                     # list paths + whether buddy is configured
buddy mcp setup --write             # merge into existing files (confirms each)
buddy -y mcp setup --write --create # also create missing well-known files
buddy mcp setup --host=cursor
buddy mcp setup --host=claude-desktop
buddy mcp setup --path=/custom/mcp.json --write
```

| Host | Well-known path |
|------|-----------------|
| Cursor (user) | `~/.cursor/mcp.json` |
| Cursor (project) | `./.cursor/mcp.json` (cwd) |
| Claude Desktop (macOS) | `~/Library/Application Support/Claude/claude_desktop_config.json` |
| Claude Desktop (Linux) | `~/.config/Claude/claude_desktop_config.json` |
| Claude Desktop (Windows) | `%APPDATA%\Claude\claude_desktop_config.json` |

A minimal example also lives at [`examples/mcp.json`](../examples/mcp.json).

Docs and examples are **embedded in the buddy binary** and exposed as `buddy://docs/...` resources (agent-tone generated pages where dual docs exist). `AGENTS.md` and `.cursor/rules` stay in the repo for humans/Cursor — they are **not** embedded. `BUDDY_DOCS_ROOT` is optional (live override while editing sources). After changing docs, run `make sync-docfs` (or `make buddy`) and reload MCP in the host. For **policy apps**, edit `internal/buddy/app_doc.go` then `make gendocs`.

### 3. Reload the AI host

Restart or refresh MCP in your client so it picks up the new server. The server should show as ready with tools such as `get_instructions`, `buddy_info`, `buddyscript`, `docs`.

### 4. Use it in chat

Ask in natural language; the agent calls tools for you:

- “How should I use buddy MCP?” (`get_instructions`)
- “What buddy version / tenant is MCP using?” (`buddy_info`)
- “List Jamf smart groups via buddyscript” / “Bump the version criterion on smart group X”
- “List computers in static group Y” / “Add computer 42 to static group Y”
- “Patch the Chrome payload in profile Z”
- “Find computer IDs where name contains lab”
- “Load policy details for trigger install-foo-1.0” / “Copy policy 12 to tenant sandbox as New Name”
- “Migrate policy 12 to prod (upsert by name)”
- “List Platform blueprints” (via `buddyscript` + `blueprints`; needs a gateway credential)
- “Show MCP tool priority / groups vs profiles via docs” (`docs` → `mcp-tools`)

Optional tool arguments `tenant`, `surface` (`instance`|`platform`), and `auth` select the session for a call (tenant-first; `auth` pins like CLI `--user`). `cred` was removed.

MCP blocks Jamf token/keep-alive paths via `jamf_*` / `buddyscript` and refuses `file_read` of buddy config/secrets/token cache files.

**Tool priority for agents:** call `get_instructions` once first, then `docs` → `buddyscript` with named builtins → `jamf_get`/`jamf_put` only if no helper exists. See [mcp-tools.md](mcp-tools.md) for the lean scorecard, smart vs static vs Platform groups, the profile payload cookbook, and how buddy relates to Jamf’s hosted MCP (`developer.jamf.com/mcp`). The MCP server also advertises that split in Initialize `instructions` and via the `get_instructions` tool.

---

## Tools

Lean surface (5 tools). Call **`get_instructions` once** at session start before the others. Resource automation uses **named buddyscript builtins** via `buddyscript` (see [reference manual](reference-manual.md#built-in-functions) or `buddy help builtins`). Full scorecard: [mcp-tools.md](mcp-tools.md).

| Tool | Returns `value` | Notes |
|------|-----------------|-------|
| `get_instructions` | markdown playbook | Firm agent guidance (no Jamf call). Call once before other tools |
| `buddy_info` | `{version, channel, tenant, auth, tenants, auths, channel_access, …}` | Binary version + mcp session selection (no Jamf call). Live auth: first `buddyscript` Jamf call or CLI `buddy auth test` |
| `docs` | Catalog entries or page markdown | Embedded buddy docs (+ agent-tone generated pages); prefer `mcp-tools` for agent guidance |
| `jamf_schema` | Operations catalog (+ optional docs) | Discovery; then prefer named builtins via `buddyscript` |
| `buddyscript` | `{stdout, stderr, result, error}` | **Primary** automation surface; see below |

Former typed MCP tools (Pro groups/profiles/policies, package READY/download, Platform CRUD, list stubs) were removed — call the same names as builtins inside `buddyscript`.

### Resources

MCP also exposes the **agent docs** catalog as `buddy://docs/...` resources (broader than the curated `buddy serve` HTML UI; agent pages use imperative tone + parameter schemas where generated). Hosts that support resource pickers can attach them; **agents that call tools should use the `docs` tool** (same catalog).

| URI | Content |
|-----|---------|
| `buddy://docs/builtins` | Function index (Markdown) |
| `buddy://docs/docs/*` | Cookbooks (agent tone when dual-generated, e.g. `docs/apps`) |
| `buddy://docs/readme` | `README.md` |
| `buddy://docs/examples/*` | Example scripts and mcp.json |
| `buddy://docs/help/*` | CLI help (root, shell, starlark) |
| `buddy://docs/*.md` | Legacy aliases for former disk filenames |

The HTML docs UI shows neutral user-facing guides (no AGENTS/rules dumps; human-tone generated pages).

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

**Why:** Some MCP hosts validate `structuredContent` as an object. Returning a raw list caused errors such as `expected record, received array`. List payloads always live under `value`, with optional `count`.

buddyscript builtins return bare values and raise on failure; the `{ok, value}` envelope is **MCP-only**.

### `buddy_info`

Reports the buddy binary version (same as the MCP handshake `Implementation.Version`) plus the **mcp**-channel session selection and policy without calling Jamf:

```json
{
  "ok": true,
  "value": {
    "version": "dev",
    "channel": "mcp",
    "tenant": "sandbox",
    "auth": "",
    "tenants": ["prod", "sandbox"],
    "auths": [
      {"name": "sandbox-admin", "tenants": ["sandbox"], "access": "full", "surface": "instance", "type": "oauth"}
    ],
    "channel_access": "readonly",
    "channel_tenant": "sandbox",
    "channel_tenants": ["sandbox"],
    "deny_builtins": ["jamf_*", "computer_erase"]
  }
}
```

`tenant` / `auth` reflect the process default (`--tenant` / `--user`, or `channels.mcp.tenant` / `tenant.default`). Empty `auth` means auto-pick. `auths` is filtered by each auth’s `channels:` list (omit = all). `channel_access` / `channel_tenants` / `deny_builtins` come from top-level `channels.mcp` policy. `deny_builtins` (exact names or trailing-`*` prefix globs like `jamf_*`) makes matching buddyscript builtins raise on call so agents cannot fall back to generic HTTP — treat that raise as a coverage gap. Live Jamf auth is proven by the first `buddyscript` Jamf call, or CLI `buddy auth test`.

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
- After discovering a path, prefer a **named builtin** via `buddyscript`. Use `jamf_get` / `jamf_put` via `buddyscript` only when no helper exists. Do not invent Classic profile or Pro group PUT bodies from schema alone.

### Groups, profiles, and object cookbooks

See **[mcp-tools.md](mcp-tools.md)** pathfinding map and the **Resources / Platform** pages in [index.md](index.md). Payload deep-dive: [profile-payloads.md](/docs/docs/profile-payloads). Query `docs` with a resource noun (`policy`, `packages`, `smart-groups`, `blueprints`, …) — list matches id/title/section/**keywords**.

### `docs` tips

Read embedded buddy documentation (same data as `buddy://docs/...` resources):

| `action` | Args | Returns |
|----------|------|---------|
| `list` (default) | optional `query` | `[{id, title, section, uri}, …]` |
| `get` | `id` required | `{id, title, uri, markdown}` |

- Prefer `docs` before inventing buddy CLI/buddyscript conventions.
- Typical flow: `docs` with a **resource noun** (`query: "policy"`) or page id (`"builtins"`, `"docs/apps"`), then `action: "get"` with the page `id` (e.g. `docs/policies`).
- `query` is a case-insensitive substring on **id / title / section / keywords** (not full-text markdown body).
- `id` may be a short id (`builtins`, `docs/policies`) or a full `buddy://docs/…` URI (including legacy `….md` aliases).
- `AGENTS.md` and Cursor rules are **not** in the MCP catalog (repo/editor only).

### Platform builtins (via `buddyscript`)

- Platform product builtins (`blueprint*`, `device*`, `device_group*`, `ddm_*`, `benchmark*`) require a **Platform API Gateway credential** (`region`/`url` + `tenantId`); they return `{ok:false, error}` otherwise.
- Mutating calls **do not** prompt for confirmation in MCP (no interactive stdin) — the host’s tool-call approval is the guard. Treat them like `-y`. Readonly auths are rejected for mutates.
- `blueprint_create` / `blueprint_set` take a JSON `body` object; `blueprint_set` is a merge-patch.

### `buddyscript` tips

- **Primary Jamf automation surface** — call named builtins here (groups, profiles, policies, Platform, …).
- In buddyscript, builtins return **bare values** and **raise** on failure. Do not use `unwrap` / `r["ok"]` for correctness; `unwrap` is optional JSON-print for CLI/`jq`.
- MCP returns `{ok, value: {stdout, stderr, result, error}}`. **Primary output is `value.result`** (last expression or `main()` return). `stdout` is from `print` / `unwrap`; when `result` is set and `stdout` is empty, `result` is mirrored into `stdout`.
- Prefer a **single expression**, or define `main()` **without** calling it and **without** a trailing top-level expression (MCP sets `CallMain`; a trailing expression skips `main`).
- Top-level `if` / `for` outside a function is invalid Starlark—put control flow inside `main()`.
- Starlark has **no `while`** (also no `try`/`class`). Use `for … in` / `range`, or wait builtins like `package_wait_ready` for polling — full vs-Python list: [buddyscript-syntax](buddyscript-syntax.md).
- Mutating Jamf calls **always skip** interactive confirmation in MCP (same as `-y`); the host’s tool-call approval is the guard. You do **not** need `args: ["-y", "mcp"]`.
- You can `load("stdlib/….buddy", …)` the same as in CLI scripts / the IDE (e.g. `load("stdlib/release.buddy", "bump_release")`).
- Prefer named builtins over `jamf_get`/`jamf_put` when a builtin exists. Before resource mutates, `docs` query the object noun (see [mcp-tools.md](mcp-tools.md) pathfinding) — e.g. [profile-payloads.md](/docs/docs/profile-payloads), [packages.md](/docs/docs/packages).
- **Packages / JCDS:** after upload use `package_wait_ready` before policy attach; use `package_ready` / `package_download` — do not invent from schema (see [packages.md](/docs/docs/packages)).

Example:

```python
def main():
    groups = smart_groups()
    return {"count": len(groups), "names": [g["name"] for g in groups]}
```
---

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| Server missing / failed to start | Use absolute `command` path; run `buddy mcp` in a terminal to see errors |
| Docs resources empty / stale | Rebuild buddy (`make install`) after `make sync-docfs`; reload MCP |
| Auth / API errors | Fix config with `buddy auth test`; check `--user` / default cred |
| Empty `stdout` but data present | Read `value.result` (or rely on mirror when stdout was empty) |
| `buddyscript` “if not within a function” | Wrap logic in `def main(): ...` |
| Inspector UI 401 / won’t connect | Use the printed URL with `MCP_PROXY_AUTH_TOKEN` (Inspector proxy auth — not Jamf). See [Auth layers](#auth-layers-stdio-vs-inspector-vs-remote-mcp) |
| Inspector tab doesn’t open (Edge/corp browser) | Paste the full Inspector URL into the browser; confirm Cursor stdio still works separately |

---

## Security

- MCP runs as your user and uses your buddy secrets (Keychain / file store).
- Do not point a remote or shared MCP host at production credentials without review.
- Prefer read-only exploration first; treat `buddyscript` like an interactive shell.
