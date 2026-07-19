# Buddy reference manual

**buddy** is a command-line tool for [Jamf Pro](https://www.jamf.com/products/jamf-pro/) automation: policies, computer groups, configuration profiles, packages, scripts, computers / MDM, API roles, and API clients. It includes a [Starlark](https://github.com/google/starlark-go) runtime so multi-step workflows can live in `.buddy` scripts (`.star` still works).

This manual describes how the pieces fit together. For behavior that must stay in lockstep with the code, buddy exposes the same text via `buddy help` (see [CLI help](#cli-help)).

---

## Table of contents

1. [License](#license)
2. [Installation](#installation)
3. [Configuration and credentials](#configuration-and-credentials)
4. [First-time API setup](#first-time-api-setup)
5. [CLI overview](#cli-overview)
6. [Local docs and notebook](#local-docs-and-notebook)
7. [MCP server](#mcp-server)
8. [Starlark shell and scripts](#starlark-shell-and-scripts)
9. [Built-in functions](#built-in-functions)
10. [HTTP API usage](#http-api-usage)
11. [Examples](#examples)
12. [Environment variables](#environment-variables)

---

## License

This project is released under the MIT License. See the [`LICENSE`](../LICENSE) file in the repository root. Copyright © 2026 Peter Viertel.

---

## Installation

Build from a checkout with Go 1.26+:

```sh
make buddy
make install    # copies to ~/go/bin/buddy
```

Or run `./buddy` from the build directory.

---

## Configuration and credentials

Default config path: `~/.config/buddy/config.yaml` (override config root with `XDG_CONFIG_HOME`).

A minimal file defines a default credential name and one or more named credentials (each with a Jamf Pro base URL and auth: `command`, `basic`, or `oauth`). Use:

- `buddy cred help` — add, list, delete credentials; set the default name
- `buddy cred secret help` — store password/client secret in Keychain, file, or pass
- `buddy auth test` — verify the active credential
- `buddy --user <name> ...` — override the default credential for one invocation (`--cred` is deprecated)

For `basic` auth, buddy exchanges username/password for a Bearer token via Jamf’s `POST /api/v1/auth/token`.

**Security:** Omit secrets from YAML and use `buddy cred secret set`. macOS defaults to Keychain; Linux defaults to `~/.config/buddy/secrets/`. Override with `BUDDY_SECRET_STORE=file|pass|keychain`. Inline secrets in config remain supported for dev/test. Never commit credentials to source control.

Full YAML shape and field descriptions are in the [README](../README.md#configuration).

---

## First-time API setup

To create a Jamf API role with the privileges buddy needs and an API client scoped to that role:

```sh
buddy setup write-script              # writes setup_buddy_role.buddy (or pass a path)
buddy setup_buddy_role.buddy -y   # idempotent: reuses an existing "Buddy" role when present
buddy setup configure-oauth -y        # optional: store OAuth in config
```

See `buddy setup help` for options. After creation, `buddy client credentials <id>` can reveal the client secret once for manual config.

The generated automation may request a **wide privilege set** so all buddy features are available. That does **not** mean your organization should keep every privilege on the production API client—especially **destructive** capabilities (wipe computer, erase device, or other Jamf permissions you only need occasionally). Review the **Buddy** (or equivalent) API role in Jamf after setup and **drop privileges** that are not justified for how you will use buddy.

---

## CLI overview

**Usage**

```text
buddy [global-flags] <command> [subcommand] [args...]
buddy help [command] [subcommand]
```

**Global flags**

| Flag | Meaning |
|------|---------|
| `--version` | Print version and exit |
| `-y` | Skip confirmation prompts (destructive MDM actions still use extra checks where required) |
| `-v`, `--verbose` | Verbose output |
| `-t`, `--trace` | Trace HTTP to stderr (headers and bodies; secrets redacted) |
| `--user <name>` | Use a named credential from config (`--cred` deprecated) |

**Commands** (high level)

| Command | Role |
|---------|------|
| `auth` | Test auth; print token material |
| `cred` | Manage `config.yaml` credentials and platform secrets |
| *(path)* | Execute a script: `buddy script.buddy` (`.buddy` preferred; `.star` still works) |
| `policy` | Policies (get/set/clone/create/delete/find) |
| `group` | Smart/static computer groups (list/get/set/create/delete; static membership) |
| `profile` | macOS configuration profiles |
| `package` | Package records (upload, metadata, history) |
| `computer` | Computer inventory and MDM actions |
| `mdm` | Query MDM command history (RSQL) |
| `role` | API roles |
| `client` | API integrations / OAuth clients |
| `script` | Jamf script objects |
| `setup` | Generate setup scripts and OAuth config helpers |
| `serve` | Local docs browser + Starlark notebook UI |
| `mcp` | MCP stdio server for Cursor agents |
| `lsp` | Language server for the Cursor/VS Code buddy extension |

### CLI help

The complete, version-accurate command reference is embedded in the binary:

```sh
buddy help
buddy help policy
buddy policy set help
```

Use these when unsure about positional arguments, flags, or subcommand names.

---

## Local docs and notebook

```sh
make docs-serve
# or
buddy serve --open
buddy serve --addr=127.0.0.1:8742
```

- Binds **localhost only** (default `127.0.0.1:8742`).
- `/docs/` — Curated user guides (overview, reference, builtins, API, MCP, examples). Contributor/agent material (AGENTS, Cursor rules) is MCP-only.
- `/notebook` — Interactive Starlark cells against your configured credential (same builtins as the CLI/REPL).
- Optional `--docs-root` / `BUDDY_DOCS_ROOT` to override embedded markdown with live files while editing.
- Confirmations follow global `-y` (default: prompts still apply for destructive ops).

Open the printed URL in Cursor’s Simple Browser for an in-editor experience.

---

## MCP server

Full guide: **[MCP server (Cursor setup)](mcp.md)**.

```sh
buddy mcp
```

Speaks [Model Context Protocol](https://modelcontextprotocol.io/) over **stdio**.

**Cursor:** add an entry to `.cursor/mcp.json` or `~/.cursor/mcp.json` with an **absolute path** to `buddy` and args `["mcp"]`. Docs resources are embedded (no `BUDDY_DOCS_ROOT` required). See [`examples/mcp.json`](../examples/mcp.json) and reload MCP in Cursor Settings.

**Response shape:** every tool returns a JSON **object** `{ok, value, error?, count?}`—never a bare array—so hosts that require object `structuredContent` (e.g. Cursor) accept list results. `auth_test` defaults to a compact summary (`verbose: true` for full privileges).

Curated tools: `auth_test`, `smart_groups`, `static_groups`, `computers`, `scripts`, `packages`, `policy`, `starlark_eval`. Resources under `buddy://docs/...`.

Uses the active credential (`--user` to override). MCP inherits that credential’s Jamf privileges—treat it like the CLI.

---

## Starlark shell and scripts

**Usage**

```sh
buddy [global-flags]                              # interactive Starlark shell
buddy [global-flags] [--trace] <script.buddy> [-- args...]
```

- With no arguments, buddy starts an interactive Starlark REPL on stdin/stdout.
- Scripts are **Starlark** (a deterministic Python-like language). See **Language** below.
- Prefer the **`.buddy`** file extension for user scripts and stdlib modules (Cursor/VS Code language id + `buddy lsp`). Load with `load("stdlib/<name>.buddy", …)`. Existing **`.star`** paths still run.
- `args` after `--` are available as the global list `args`.
- The active credential name is available as `cred` (override per builtin with `cred=`).
- The regex module is exposed as `re` (see `buddy help starlark` for what Starlark provides globally).
- If the script defines `main`, buddy invokes it after load (same as MCP `starlark_eval`). Do not call `main()` yourself at top level.
- **`print()`** writes to stdout; buddy adds **`error(*args, ...)`** for stderr (same signature as `print`). Prefer **`error`** for progress, skips, and failures so stdout stays free for machine-readable output; reserve **`print`** for lines meant to be piped or captured as the script’s primary result.
- **Positional `--trace` before the script path** or global **`--script-trace`** logs each builtin call (and optionally Starlark `print` tracing) to stderr. **HTTP tracing** uses the top-level `-t` / `--trace` on `buddy`: `buddy -t deploy.buddy`.

### Editor / language server

```sh
buddy lsp
make extension-package   # PUBLISHER=virtualpeter by default → editors/vscode/*.vsix
```

`buddy lsp` speaks LSP over stdio (diagnostics, completions, hover, signature help). The thin editor client lives in [`editors/vscode`](../editors/vscode); see that README for install settings and build-time `PUBLISHER`.

### Language

Starlark quick-reference (types, control flow, string/list/dict methods, what is intentionally absent vs Python):

```sh
buddy help starlark
```

### Result convention

Almost all Jamf-related builtins return a **result dictionary**:

```python
{"ok": True, "value": ...}     # success
{"ok": False, "error": "..."}  # failure
```

Always check `ok` before using `value`. Helpers like `fail()` (if exposed) or explicit checks keep scripts safe.

---

## Built-in functions

Buddy registers many functions for policies, groups, profiles, packages, scripts, computers, MDM, roles, clients, JSON/plist/XML transforms (`xml_escape` / `xml_unescape` / `xml_sort`, payload builders), RSQL filter builders (`rsql_*`), local file I/O, and a **declarative workflow** layer (`step_*` + `run_workflow`).

**Canonical reference** (always matches your installed `buddy` binary):

```sh
buddy help builtins
```

That output lists every builtin with parameter names, types, and return shapes. **Treat it as the source of truth**; this manual only orients you.

**Conventions** (from the builtins help header):

- **v3 grammar:** plural nouns enumerate (`policies`, `smart_groups`, `computers`); singular nouns load one (`policy(id=…)` or `policy(trigger=…)`, `smart_group(name=…)`); singular_verb mutates (`policy_set`, `computer_reboot`). See [builtins-v3.md](builtins-v3.md).
- Resource IDs come first on mutate helpers (`policy_id`, `group_id`, …); singular loaders use `id=` or a natural key; then options; optional `cred?` last.
- Use `cred="name"` to override the active credential for a single call.
- Paginated list builtins take `page?` (0-based) and `page_size?` (default 200). **Omit `page`** to walk every page and return the full list.
- Plural lists return summary dicts `{id, name, …}`; use `ids=True` on `policies` / `computers` when you need bare ID lists.

---

## HTTP API usage

Which Jamf Pro URLs buddy calls (Classic XML vs JSON APIs, pagination, notes) is documented separately:

- [**Jamf Pro API endpoints used by buddy**](api-endpoints.md)

Use it when correlating buddy behavior with Jamf’s API or troubleshooting permissions.

---

## Examples

Runnable Star scripts and a short index live under [**examples/**](../examples/README.md):

- [`examples/bump-release.buddy`](../examples/bump-release.buddy) — minimal pattern: `policy(trigger=…)`, then updates (edit triggers/IDs for your tenant).
- [`docs/builtins-v3.md`](builtins-v3.md) — Starlark plural/singular rename map (v3).
- For broad integration testing against a non-production Jamf instance, see [`scripts/integration_test.buddy`](../scripts/integration_test.buddy) (creates and tears down test resources; read before running).

---

## Environment variables

| Variable | Effect |
|----------|--------|
| `DEBUG` | Enable debug logging when set |
| `XDG_CONFIG_HOME` | Config directory root (default: `~/.config`) |
| `BUDDY_DOCS_ROOT` | Optional live `docs/` override for serve/MCP (default: embedded) |
| `BUDDY_SECRET_STORE` | Secret backend: `file`, `pass`, or `keychain` |

---

## See also

- [README](../README.md) — quick start, configuration table, command cheat sheet
- [`docs/index.md`](index.md) — index of documentation files
