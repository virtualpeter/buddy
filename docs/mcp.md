# Buddy MCP server

**buddy** can run as a [Model Context Protocol](https://modelcontextprotocol.io/) server so Cursor (and other MCP hosts) call Jamf helpers as tools instead of shelling out.

```sh
buddy mcp
```

Uses stdio JSON-RPC. The active credential comes from `~/.config/buddy/config.yaml` (override with global `--user=<name>` on the command in mcp.json). MCP inherits that credential’s Jamf privileges—treat it like the CLI.

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
      "args": ["mcp"],
      "env": {
        "BUDDY_DOCS_ROOT": "/Users/YOU/work/buddy/docs"
      }
    }
  }
}
```

| Field | Why |
|-------|-----|
| `command` | Absolute path avoids “command not found” when Cursor spawns the server |
| `args: ["mcp"]` | Starts the stdio MCP server |
| `BUDDY_DOCS_ROOT` | So `buddy://docs/...` resources resolve even if cwd is not the repo |

A minimal example also lives at [`examples/mcp.json`](../examples/mcp.json) (uses `buddy` on `PATH`).

### 3. Reload MCP in Cursor

Open **Cursor Settings → MCP**, enable **buddy**, and refresh (or restart Cursor). The server should show as ready with tools such as `auth_test`, `group_list_smart`, `starlark_eval`.

### 4. Use it in chat

Ask in natural language; the agent calls tools for you:

- “Test my Jamf auth with buddy”
- “List Jamf smart groups”
- “Find computer IDs where name contains lab”
- “Resolve policy ID for trigger install-foo-1.0”

Optional tool argument `cred` selects a named credential from config.

---

## Tools

| Tool | Returns `value` | Notes |
|------|-----------------|-------|
| `auth_test` | Compact auth summary | Set `verbose: true` for full privileges payload (large) |
| `group_list_smart` | List of `{id,name,description}` | `filter`, `page`, `page_size`; omit `page` = all pages |
| `group_list_static` | Same shape | Same paging |
| `computer_find_ids` | List of int IDs | RSQL `filter` |
| `script_list` | Script summaries | Name substring or RSQL |
| `package_list` | Package summaries | RSQL `filter` |
| `policy_get_id` | `{id, trigger}` | |
| `starlark_eval` | `{stdout, stderr, result, error}` | Fresh session; see below |

### Resources

| URI | Content |
|-----|---------|
| `buddy://docs/builtins` | Starlark builtins (Markdown, same as `buddy serve` /docs/builtins) |
| `buddy://docs/help/*` | CLI help text (root, shell, starlark, builtins) |
| `buddy://docs/*.md` | Repo docs when `BUDDY_DOCS_ROOT` / cwd finds `docs/` |

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

### `starlark_eval` tips

- Prefer a **single expression**, or define `main()` **without** calling it (MCP sets `CallMain` and invokes `main` for you).
- Top-level `if` / `for` outside a function is invalid Starlark—put control flow inside `main()`.
- Mutating Jamf calls still respect confirmations unless the MCP process was started with global `-y`.

Example:

```python
def main():
    r = group_list_smart()
    if not r["ok"]:
        return r
    return {"count": len(r["value"]), "names": [g["name"] for g in r["value"]]}
```
---

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| Server missing / failed to start | Use absolute `command` path; run `buddy mcp` in a terminal to see errors |
| Docs resources empty | Set `BUDDY_DOCS_ROOT` to your repo’s `docs/` directory |
| Auth / API errors | Fix config with `buddy auth test`; check `--user` / default cred |
| Agent can’t use list tools | Rebuild buddy ≥ response-envelope change; reload MCP |
| `starlark_eval` “if not within a function” | Wrap logic in `def main(): ...` |

---

## Security

- MCP runs as your user and uses your buddy secrets (Keychain / file store).
- Do not point a remote or shared MCP host at production credentials without review.
- Prefer read-only exploration tools first; treat `starlark_eval` like an interactive shell.
