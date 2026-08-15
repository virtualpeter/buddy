# Buddy reference manual

**buddy** is a command-line tool for [Jamf Pro](https://www.jamf.com/products/jamf-pro/) automation: policies, computer groups, configuration profiles, packages, scripts, computers / MDM, API roles, and API clients. It includes a **buddyscript** runtime (built on [Starlark](https://github.com/google/starlark-go)) so multi-step workflows can live in `.buddy` scripts.

**New install?** Start with [`buddy setup basic-setup`](#first-time-api-setup) (also covered in [Getting started](index.md)), then come back here for the full picture.

This manual describes how the pieces fit together. For behavior that must stay in lockstep with the code, buddy exposes the same text via `buddy help` (see [CLI help](#cli-help)).

---

## Table of contents

1. [License](#license)
2. [Installation](#installation)
3. [Configuration and credentials](#configuration-and-credentials)
4. [First-time API setup](#first-time-api-setup)
5. [CLI overview](#cli-overview)
6. [Local docs and script IDE](#local-docs-and-script-ide)
7. [MCP server](#mcp-server)
8. [buddyscript shell and scripts](#buddyscript-shell-and-scripts)
9. [Built-in functions](#built-in-functions)
10. [Change batches](#change-batches)
11. [HTTP API usage](#http-api-usage)
12. [Examples](#examples)
13. [Environment variables](#environment-variables)

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

Config is split into **tenant selection** (`tenant.default`), **channel policies** (`channels.cli|mcp|ide` with optional `access` / `tenant` / `tenants` / `deny_builtins`), **tenants** (instance URL and/or Platform `region`+`tenantId`), and **auths** (command/basic/oauth principals linked to one or more tenants). Optional auth `channels:` (`cli`/`mcp`/`ide`; omit = all) limits which buddy clients may use that auth. Legacy shapes migrate on load; `tenant.cli|mcp|serve|ide|notebook` and the `serve` / `notebook` client names are removed (use `channels.ide`).

Full walkthrough (secret stores, token cache, instance vs Platform): **[Auth, secrets, and tokens](auth.md)**.

- `buddy cred help` — set/list/get/delete tenants, auths, and channel policies; manage secrets; `cred use` / `cred set channel`
- `buddy cred set secret <auth>` — store password/client secret in Keychain, file, or pass
- `buddy cred delete auth|tenant <name>` — remove auth (+store secrets) or tenant (cascades sole-linked auths)
- `buddy cred clean secrets` — review secret-store entries and prompt to delete orphans
- `buddy cred clean tokens` — delete expired or corrupt files under `~/.config/buddy/tokens/`
- `buddy cred migrate` — move inline config.yaml secrets into the secure store
- `buddy auth test` — verify the active tenant/auth session
- `buddy --tenant <name> ...` — select tenant (auto-picks auth); `--user`/`--auth` pins an auth

For `basic` auth, buddy exchanges username/password for a Bearer token via Jamf’s `POST /api/v1/auth/token`.

**Platform API Gateway:** put `region` + `tenantId` on the **tenant**, and link an oauth auth with `surface: platform` (or `any`). Example:

```sh
buddy cred set tenant gateway --region=us --tenant-id=<tenant-id>
buddy cred set auth gateway --type=oauth --client-id=<client-id> --tenants=gateway --surface=platform
```

Classic (`/JSSResource/…`) calls are rewritten for the gateway, but Jamf does not yet expose Classic permissions to Platform integrations — expect `403` on Classic until it does. Prefer an instance-surface auth for Classic-heavy work.

**Security:** Omit secrets from YAML and use `buddy cred set secret` (or `buddy cred migrate`). macOS defaults to Keychain (service `buddy` via `security(1)`, or service `virtualpete.buddy.cli` / label Buddy on `make buddy-darwin-arm64`); Linux defaults to `~/.config/buddy/secrets/`. Override with `BUDDY_SECRET_STORE=file|pass|keychain`. Never commit secrets to source control.

Full YAML shape and field descriptions are in the [README](../README.md#configuration).

---

## First-time API setup

If you have **no config yet**, start with instance basic auth:

```sh
buddy setup basic-setup               # prompts for tenant, URL, username, password
buddy --user <admin-auth> auth test
```

Then (optional) create a Jamf Pro **instance** API role/client and store oauth (Platform gateway oauth cannot manage roles/integrations):

```sh
buddy setup write-script              # writes setup_buddy_role.buddy (or pass a path)
buddy setup_buddy_role.buddy -y       # idempotent: reuses role "Buddy" and client "buddy-cli"
buddy setup store-oauth               # rotate instance client secret; store/update oauth auth (surface=instance)
buddy --user <auth-name> auth test
```

See `buddy setup help` for options. After creation, `buddy client credentials <id>` can reveal the client secret once for manual `buddy cred set auth` setup.

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
| `--tenant <name>` | Tenant from config (auto-picks auth) |
| `--user <name>` | Pin an auth (`--auth` alias) |

**Commands** (high level)

| Command | Role |
|---------|------|
| `auth` | Test auth; print token material |
| `cred` | Manage `config.yaml` tenants, auths, and platform secrets |
| *(path)* | Execute a script: `buddy script.buddy` |
| `policy` | Policies (get/set/clone/create/delete/find) |
| `group` | Smart/static computer groups (list/get/create/delete; smart/static membership) |
| `profile` | macOS configuration profiles |
| `package` | Package records (upload, metadata, history) |
| `computer` | Computer inventory and MDM actions |
| `mdm` | Query MDM command history (RSQL) |
| `role` | API roles |
| `client` | API integrations / OAuth clients |
| `script` | Jamf script objects |
| `blueprint` | Platform API Blueprints (list/get/create/set/delete/deploy/status; gateway credential required) |
| `device` | Platform API devices (inventory + check-in/restart/erase/…; not Pro `computer`) |
| `device-group` | Platform API device groups (not Pro computer groups) |
| `ddm` | Platform DDM declaration reporting (read-only) |
| `benchmark` | Platform compliance benchmarks (beta) |
| `setup` | Generate setup scripts and OAuth config helpers |
| `serve` | Local docs browser + script IDE |
| `mcp` | MCP stdio server for AI tools (`mcp setup` writes host configs) |
| `lsp` | Language server for the Cursor/VS Code buddy extension |

### CLI help

The complete, version-accurate command reference is [Command index](/docs/cli) in the docs UI (same text as the binary):

```sh
buddy help
buddy help policy
buddy policy set help
```

Use these when unsure about positional arguments, flags, or subcommand names.

---

## Local docs and script IDE

```sh
make docs-serve
# or
buddy serve --open
buddy serve --addr=127.0.0.1:8742
```

- Binds **localhost only** (default `127.0.0.1:8742`).
- `/docs/` — Curated user guides (overview, reference, builtins, API, MCP, examples). Contributor/agent material (AGENTS, Cursor rules) is MCP-only.
- `/ide` — Single-buffer script runner with open/save, args, optional `--script-trace`-style builtin tracing, Run (file dialect + call `main()` if present), resizable output, copy/save/stash, and a Variables | Libraries sidebar. Open/save is limited to the process cwd or `BUDDY_IDE_ROOT`. Matches CLI file semantics. `/notebook` redirects here.
- Line breakpoints and stepping are **not** available (Starlark-go has no pause/step API). A soft `breakpoint()` builtin that only stops when the script calls it may be added later.
- Optional `--docs-root` / `BUDDY_DOCS_ROOT` to override embedded markdown with live files while editing.
- Confirmations for `confirm()` and Jamf mutates open a dialog in the browser. Global `-y` skips them. Serve sessions use the `ide` channel.

Open the printed URL in Cursor’s Simple Browser for an in-editor experience.

---

## MCP server

Full guide: **[MCP server (AI tools)](mcp.md)**.

```sh
buddy mcp              # stdio MCP server
buddy mcp setup        # discover well-known host configs
buddy mcp setup --write --create --host=cursor
```

Speaks [Model Context Protocol](https://modelcontextprotocol.io/) over **stdio**.

Add a `mcpServers.buddy` entry (absolute `command` path, `args: ["mcp"]`) to your AI host’s MCP config. `buddy mcp setup` knows Cursor (`~/.cursor/mcp.json`, `.cursor/mcp.json`) and Claude Desktop (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS). Docs resources are embedded (no `BUDDY_DOCS_ROOT` required). See [`examples/mcp.json`](../examples/mcp.json); reload/restart the host after changes.

**Response shape:** every tool returns a JSON **object** `{ok, value, error?, count?}`—never a bare array—so hosts that require object `structuredContent` accept list results.

Curated MCP tools: `get_instructions`, `buddy_info`, `docs`, `jamf_schema`, `buddyscript` (primary automation). Call `get_instructions` once at session start. Pro groups, profiles, policies, package READY/download, Platform CRUD, and related helpers run as **named builtins inside `buddyscript`**. Full table and agent priority: [`docs/mcp.md`](mcp.md) and [`docs/mcp-tools.md`](mcp-tools.md). Resources under `buddy://docs/...` (agents should prefer the `docs` tool).

Uses the active tenant/auth (`--tenant` / `--user` to override). MCP inherits that session’s Jamf privileges—treat it like the CLI.

---

## buddyscript shell and scripts

**Usage**

```sh
buddy [global-flags]                              # interactive buddyscript shell
buddy [global-flags] [--trace] <script.buddy> [-- args...]
```

- With no arguments, buddy starts an interactive buddyscript REPL on stdin/stdout.
- Scripts are **buddyscript** (Starlark-based — a deterministic Python-like language). See **Language** below.
- Scripts use **`.buddy`** (Cursor/VS Code language id + `buddy lsp`). `load("stdlib/<name>.buddy", …)` is reserved (no modules yet).
- `args` after `--` are available as the global list `args`.
- The active tenant and auth pin are available as `tenant` and `auth` (override per builtin with `tenant=` / `surface=` / `auth=`).
- The regex module is exposed as `re` (see `buddy help starlark` for what Starlark provides globally).
- If the script defines `main`, buddy invokes it after load (same as MCP `buddyscript`). Do not call `main()` yourself at top level.
- **`print()`** writes to stdout; buddy adds **`error(*args, ...)`** for stderr (same signature as `print`). Prefer **`error`** for progress, skips, and failures so stdout stays free for machine-readable output; reserve **`print`** for lines meant to be piped or captured as the script’s primary result.
- **Positional `--trace` before the script path** or global **`--script-trace`** logs each builtin call (and optionally buddyscript `print` tracing) to stderr. **HTTP tracing** uses the top-level `-t` / `--trace` on `buddy`: `buddy -t deploy.buddy`.

### Editor / language server

Full howto (local VSIX build, install into Cursor or VS Code, settings): **[Editor support](editor.md)**.

```sh
buddy lsp
# VSIX: sibling checkout ../buddy-extension → make package
```

### Language

See the **BuddyScript** docs section:

- [Overview](buddyscript.md) — runners, `main()`, return contract
- [Syntax](buddyscript-syntax.md) — Starlark subset + [language spec](https://github.com/bazelbuild/starlark/blob/master/spec.md)
- [Stdlib](buddyscript-stdlib.md) — reserved `load("stdlib/….buddy", …)` path (empty; recipes in `examples/`)
- [Function index](/docs/builtins) — every builtin (`buddy help builtins`)
- [Command index](/docs/cli) — every CLI command (`buddy help` / `buddy help <command>`)

Compact terminal cheat sheet:

```sh
buddy help starlark
```

### Result convention

Jamf-related builtins return **bare values** on success and **raise** on failure. There is no `{ok, value, error}` envelope in buddyscript (MCP tools still wrap `{ok, value}` at the tool boundary only).

```python
xml = profile_xml(id=42)          # str
profile_put_xml(profile_id=42, xml_str=xml)  # None; raises on API error
```

`unwrap` is optional JSON-print for CLI/`jq` — not the error model. Scripts that used `r["ok"]` / `r["value"]` must use return values and let errors raise. For rare expected-reject checks, `attempt(fn)` returns a soft `{ok, value, error}` without aborting.

---

## Built-in functions

Buddy registers many functions for policies, groups, profiles, packages, scripts, computers, MDM, roles, clients, JSON/plist/XML transforms (`xml_escape` / `xml_unescape` / `xml_sort`, payload builders), dotted-numeric version helpers (`version_extract` / `version_compare` / `version_sort`), RSQL filter builders (`rsql_*`), local file I/O, a **declarative workflow** layer (`step_*` + `run_workflow`), and **change batches** (`change_batch` / `plan_change` / `run_change`) for approval-oriented multi-step changes.

**Canonical reference** (always matches your installed `buddy` binary):

```sh
buddy help builtins
```

That output lists every builtin with parameter names, types, and return shapes. **Treat it as the source of truth**; this manual only orients you.

**Conventions** (from the builtins help header):

- **Grammar:** plural nouns enumerate (`policies`, `smart_groups`, `computers`); singular nouns load one (`policy(id=…)` or `policy(trigger=…)`, `smart_group(name=…)`); singular_verb mutates (`policy_set`, `computer_reboot`).
- **Notebook:** Libraries drawer shows Getting started / Advanced; bare builtin names print help; `policies()` lists all.
- Resource IDs come first on mutate helpers (`policy_id`, `group_id`, …); singular loaders use `id=` or a natural key; then options; optional `tenant?` / `surface?` / `auth?` last.
- Use `tenant="name"` and/or `auth="name"` (and optional `surface="instance"|"platform"`) to override session selection for a single call. `cred=` was removed.
- Paginated list builtins take `page?` (0-based) and `page_size?` (default 200). **Omit `page`** to walk every page and return the full list.
- Plural lists return summary dicts `{id, name, …}`; use `ids=True` on `policies` / `computers` when you need bare ID lists.

---

## Change batches

For CAB / change-record workflows (author a batch with human notes → resolve a markdown plan with no writes → apply after approval and record outcomes), see:

- [**Change batches howto**](/docs/docs/change-batches)
- Example: [`examples/demochange.buddy`](../examples/demochange.buddy) (starter); [`examples/change-batch-santa.buddy`](../examples/change-batch-santa.buddy) (Santa-shaped)

---

## HTTP API usage

Which Jamf Pro URLs buddy calls (Classic XML vs JSON APIs, pagination, notes) is documented separately:

- [**Jamf Pro API endpoints used by buddy**](/docs/docs/api-endpoints)

Use it when correlating buddy behavior with Jamf’s API or troubleshooting permissions.

---

## Examples

Runnable Star scripts and a short index live under [**examples/**](../examples/README.md):

- [`examples/bump-release.buddy`](../examples/bump-release.buddy) — minimal pattern: `policy(trigger=…)`, then updates (edit triggers/IDs for your tenant).
- [`examples/demochange.buddy`](../examples/demochange.buddy) — easiest change-batch starter (edit SETUP, then plan/apply) ([howto](/docs/docs/change-batches)).
- [`examples/change-batch-santa.buddy`](../examples/change-batch-santa.buddy) — same flow with Santa-shaped triggers/package patterns ([howto](/docs/docs/change-batches)).

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
- [`docs/index.md`](index.md) — Getting started (install, `buddy setup`, first commands)
- [Auth, secrets, and tokens](auth.md) — tenants, auths, channel policy, secret stores, token cache
- [Change batches howto](/docs/docs/change-batches) — plan / approve / apply for change records
- [Editor / LSP](editor.md) — VS Code/Cursor extension and `buddy lsp`
