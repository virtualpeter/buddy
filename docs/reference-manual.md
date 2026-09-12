# Buddy reference manual

**buddy** is a command-line tool for [Jamf Pro](https://www.jamf.com/products/jamf-pro/) automation: policies, computer groups, configuration profiles, packages, scripts, computers / MDM, API roles, and API clients. It includes a **buddyscript** runtime (built on [Starlark](https://github.com/google/starlark-go)) so multi-step workflows can live in `.buddy` scripts.

**New install?** Start with [Getting started](getting-started.md) (interactive wizard / `buddy cred add`, or [`buddy setup basic-setup`](#first-time-api-setup)), then come back here for the full picture.

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

**buddy** is independent open-source software. It talks to [Jamf](https://www.jamf.com/) Pro and Platform **APIs**. It is not affiliated with, endorsed by, or sponsored by Jamf. Jamf, Jamf Pro, and related names are trademarks of Jamf.

---

## Installation

Build from a checkout with Go 1.26+:

```sh
make buddy
make install    # copies to ~/go/bin/buddy
```

Or run `./dist/buddy` from the build directory.

---

## Configuration and credentials

Default config path: `~/.config/buddy/config.yaml` (override config root with `XDG_CONFIG_HOME`).

On macOS, org defaults may also come from managed preferences (`virtualpete.buddy.cli`); user YAML overlays them. See [Auth, secrets, and tokens](auth.md#org-defaults-macos-managed-preferences).

Config is split into four maps:

- **Context selection** — `context.default` (dual-read `tenant.default` for one release).
- **Channel policies** — `channels.cli|mcp|ide` with optional `access` / `context` / `contexts` / `deny_builtins` / `allow_builtins`. MCP defaults deny `jamf_*`.
- **Contexts** — named workplaces. Default is instance-only (`jamfpro.url`). Platform is additive (`id` + `region`, optional `jamfpro.tenantId`). The config name is not a Jamf environment UUID; see [Auth](auth.md#contexts-and-auths).
- **Auths** — command/basic/oauth/platform principals. Instance auths use `jamfpro:`; platform auths use `scope:` plus `contexts:` or `jamfpro:`. Optional auth `channels:` (`cli`/`mcp`/`ide`; omit = all) limits which buddy clients may use that auth.

Builtin deny/allow is per layer (channel allow cannot unblock a context deny). `deny_builtins: ["*"]` is an allowlist at that layer. `credentials:` / string `tenant:` / `defaults:` / `allow_auths:` and the `serve` / `notebook` client names are removed (use `contexts:` / `auths:` and `channels.ide`). Dual-read `tenants:` / `--tenant` / `tenant=` for one release.

Full walkthrough (secret stores, token cache, instance vs Platform): **[Auth, secrets, and tokens](auth.md)**.

- `buddy cred help` — set/list/get/delete contexts, auths, and channel policies; manage secrets; `cred use` / `cred set channel`
- `buddy cred status` — context × surface coverage (gaps, last live check); SECRET is locked unless `--unlock` or `--test` (`--test` runs `auth test --all` first)
- `buddy cred add` — guided context + auth + secret (optional live test)
- `buddy cred set secret <auth>` — store password/client secret in the vault
- `buddy cred set passphrase` — create or rotate the recovery passphrase (backups; does not wrap the live vault)
- `buddy cred delete auth|context <name>` — remove auth (+store secrets) or context (cascades sole-linked auths)
- `buddy cred clean secrets` — review secret-store entries and prompt to delete orphans
- `buddy cred erase secrets` — wipe the vault, data key, recovery passphrase, token-wrap key, token cache files, and leftover Keychain items
- `buddy cred clean tokens` — delete expired or corrupt files under `~/.config/buddy/tokens/`
- `buddy cred migrate` — move inline YAML secrets and old per-auth store items into the vault
- `buddy cred backup <file>` / `buddy cred restore <file>` — encrypted snapshot of contexts, auths (with secrets), and channels; restore prompts once per auth (Enter restores, or `-y`)
- `buddy setup secrets` — create missing context auths and surface-coverage gaps; store missing passwords / client secrets
- `buddy auth test` — ForceRenew + probe the active session (`--all` for every cell)
- `buddy auth token refresh` — obtain, keep-alive, or replace the session token from cache state (`--raw` prints the bearer)
- `buddy auth authorization` — cached Jamf authorization details from `GET /api/v1/auth` (access level, privilege set, privileges); `--all` dumps every cache entry
- `buddy auth token show` — print the session Authorization header (`--raw` for bearer only)
- `buddy auth token status` — disk cache metadata (no bearer); `--all` lists every `tokens/` file
- `buddy auth token drop` — invalidate the session cache (including still-valid files); `--all` drops every file
- `buddy token direct` — one-shot bearer mint (no config/vault; `--url`/`--region` plus basic or oauth flags)
- `buddy --context <name> ...` — select context (auto-picks auth); `--tenant` is a dual-read alias; `--user`/`--auth` pins an auth

For `basic` auth, buddy exchanges username/password for a Bearer token via Jamf’s `POST /api/v1/auth/token`.

**Platform API Gateway:** put `region` + `id` (Account environment UUID) on the **context**, optional `jamfpro.tenantId`, and link an auth with `type: platform` and `scope: environment` (or `tenant` / `organization`). Instance-only users skip those fields. Auth types are `basic` / `oauth` (instance) or `platform` (gateway oauth). Example:

```sh
buddy cred set context gateway --region=us --environment-id=<environment-id>
buddy cred set auth gateway --type=platform --client-id=<client-id> --contexts=gateway --scope=environment
```

Classic (`/JSSResource/…`) calls are rewritten to `/proclassic/…` (the `/JSSResource` prefix is stripped). Classic follows a platform pin by default (`features.classic_instance_only`, false). Set it true (or `BUDDY_CLASSIC_INSTANCE_ONLY=true`) to force Classic onto instance. ACS (`/api/config-profiles/…`) and undocumented Pro `GET /api/v1/policies` are always instance; they are not on the gateway and are **not** gated by that flag.

**Security:** Omit secrets from YAML and use `buddy cred set secret` (or `buddy cred migrate`). Secrets live in `secrets.enc` under a random data key (Keychain account `dek` on macOS, or `~/.config/buddy/dek` on Linux). `buddy cred set passphrase` stores a recovery passphrase (account `master` / `~/.config/buddy/master`) used for `cred backup` / `restore`, not for every CLI invoke. `BUDDY_SECRET_STORE=file|pass|keychain` selects where those keys live. Never commit `secrets.enc`, `dek`, or `master` to source control.

Full YAML shape and field descriptions are in the [README](../README.md#configuration).

---

## First-time API setup

If you have **no contexts yet**, the preferred path is the interactive wizard (same as `buddy cred add`):

```sh
buddy policy list                     # prompts when config is empty
# or: buddy cred add
buddy auth test
```

You will be asked for a context name (buddy config name), optional Jamf Pro instance URL, optional Platform region + environment id (and optional product tenant id), then auth type and credentials (secrets go in the vault).

`buddy setup basic-setup` remains for **instance URL + basic auth only**:

```sh
buddy setup basic-setup               # prompts for context, URL, username, password
buddy --user <admin-auth> auth test
```

If an MDM profile already provides contexts (preference domain `virtualpete.buddy.cli`), run `buddy setup secrets` instead to create missing auths (including instance/platform coverage gaps) and store client secrets. `buddy cred status` shows remaining holes. In `buddy serve`, **config** (`/config`) edits context/auth metadata only — not vault secrets.

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

Global flags may appear anywhere before `--`. Long flags accept a single dash (`-context` is `--context`); short flags are not combinable (`-vt` is not `-v -t`). Command and subcommand names accept a unique prefix (`buddy ico list`).

**Global flags**

| Flag | Meaning |
|------|---------|
| `--version` | Print version and exit |
| `-y` | Skip confirmation prompts (destructive MDM actions still use extra checks where required) |
| `-v`, `--verbose` | Verbose output |
| `-t`, `--trace` | Trace HTTP to stderr (headers and bodies; Authorization/Cookie and token-shaped values redacted) |
| `--context <name>` | Context from config (auto-picks auth); `--tenant` is a dual-read alias |
| `--user <name>` | Pin an auth (`--auth` alias) |

**Commands** (high level)

| Command | Role |
|---------|------|
| `auth` | Test auth; print session token material |
| `token` | One-shot bearer mint without config (`token direct`) |
| `cred` | Manage `config.yaml` contexts, auths, and platform secrets |
| *(path)* | Execute a script: `buddy script.buddy` |
| `policy` | Policies (list/get/set/create/delete/logs) |
| `group` | Smart/static computer groups (list/get/create/delete; smart/static membership) |
| `profile` | macOS configuration profiles |
| `package` | Package records (upload, metadata, history) |
| `computer` | Computer inventory and MDM actions |
| `computer-search` | Classic advanced computer searches (Jamf Reports) |
| `mdm` | Query MDM command history (RSQL) |
| `mobile-device` | Jamf Pro mobile devices (inventory + MDM actions) |
| `mobile-search` | Advanced mobile-device searches (Jamf Reports) |
| `mobile-group` | Smart/static mobile-device groups |
| `mobile-profile` | Mobile-device configuration profiles |
| `role` | API roles |
| `client` | API integrations / OAuth clients |
| `script` | Jamf script objects |
| `blueprint` | Platform API Blueprints (list/get/create/set/delete/deploy/status; Platform context+auth required) |
| `device` | Platform API devices (inventory + check-in/restart/erase/…; not Pro `computer`) |
| `device-group` | Platform API device groups (not Pro computer groups) |
| `ddm` | Platform DDM declaration reporting (read-only) |
| `benchmark` | Platform compliance benchmarks |
| `setup` | Generate setup scripts and OAuth config helpers |
| `serve` | Local docs browser + config + script IDE |
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
- `/config` — View/edit context and auth **metadata** (defaults, coverage gaps). Vault secrets stay CLI-only (`buddy cred set secret`).
- `/ide` — Single-buffer script runner with open/save, args, optional `--script-trace`-style builtin tracing, Run (file dialect + call `main()` if present), resizable output, copy/save/stash, and a Variables | Libraries sidebar. Open/save is limited to the process cwd or `BUDDY_IDE_ROOT`. Matches CLI file semantics.
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

Uses the active context/auth (`--context` / `--user` to override; `--tenant` dual-read). MCP inherits that session’s Jamf privileges—treat it like the CLI. With **no contexts** configured, `buddy mcp` registers **no tools** and points the host at `buddy cred add` / the interactive CLI wizard (secrets never over MCP).

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
- The active context and auth pin are available as `tenant` and `auth` (override per builtin with `context=` / `tenant=` / `surface=` / `auth=`; `tenant` is the dual-read name of the context).
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
profile_put_xml(id=42, xml_str=xml)  # None; raises on API error
```

`unwrap` is optional JSON-print for CLI/`jq` — not the error model. Scripts that used `r["ok"]` / `r["value"]` must use return values and let errors raise. For rare expected-reject checks, `attempt(fn)` returns a soft `{ok, value, error}` without aborting.

---

## Built-in functions

Buddy registers many functions for policies, groups, profiles, packages, scripts, computers, MDM, roles, clients, JSON/plist/XML transforms (`jq`, `json_flatten`, `xml_escape` / `xml_unescape` / `xml_sort`, payload builders), dotted-numeric version helpers (`version_extract` / `version_compare` / `version_sort`), RSQL filter builders (`rsql_*`), local file I/O, a **declarative workflow** layer (`step_*` + `run_workflow`), and **change batches** (`change_batch` / `plan_change` / `run_change`) for approval-oriented multi-step changes.

**Canonical reference** (always matches your installed `buddy` binary):

```sh
buddy help builtins
```

That output lists every builtin with parameter names, types, and return shapes. **Treat it as the source of truth**; this manual only orients you.

**Conventions** (from the builtins help header):

- **Grammar:** plural nouns enumerate (`policies`, `smart_groups`, `computers`); singular nouns load one (`policy(id=…)` or `policy(trigger=…)`, `smart_group(name=…)`); singular_verb mutates (`policy_set`, `computer_reboot`).
- **Notebook:** Libraries drawer shows Getting started / Advanced; bare builtin names print help; `policies()` lists all.
- Resource IDs come first (`id=` / `ids=` for the subject resource; related IDs stay typed); singular loaders also use `id=` or a natural key; then options; optional `context?` / `tenant?` / `surface?` / `auth?` last (`tenant?` dual-read).
- Use `context="name"` (or `tenant="name"`) and/or `auth="name"` (and optional `surface="instance"|"platform"`) to override session selection for a single call. `cred=` was removed.
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
| `BUDDY_IDE_ROOT` | Root directory for `/ide` open/save (default: process working directory) |
| `BUDDY_LANG` | CLI message locale (wins over `LANG`; unknown tags fall back to `en`) |
| `BUDDY_ACS_PAYLOAD_UUID_<id>` | Bind ACS payload UUID for Classic profile id `<id>` when HTML scrape fails |
| `BUDDY_SECRET_STORE` | Where the vault data key and recovery passphrase live: `file`, `pass`, or `keychain` (default: Keychain on macOS, `dek`/`master` files elsewhere) |
| `BUDDY_VAULT_PASSPHRASE` | Override recovery passphrase (CI/tests; never written). A `dek:v1:…` value overrides the data key instead. |
| `BUDDY_MANAGED_PLIST` | Path to a managed-config plist (replaces Darwin `/Library/Managed Preferences/virtualpete.buddy.cli.plist`) |
| `BUDDY_CLASSIC_INSTANCE_ONLY` | Override `features.classic_instance_only` (`true`/`false`). Default false: Classic may use `/api/proclassic/` (no `/JSSResource` in the path). `true` forces **Classic `/JSSResource` only** onto instance. Does not apply to ACS or `GET /api/v1/policies` (always instance). |

### Build-only (`make buddy-darwin-arm64` / `make app` / `make package`)

These are Makefile variables, not runtime config:

| Variable | Effect |
|----------|--------|
| `BUDDY_CODESIGN_IDENTITY` | Developer ID Application identity used to sign the Darwin/arm64 helper and Buddy.app (default: first match from `security find-identity -v -p codesigning`) |
| `BUDDY_CODESIGN_IDENTIFIER` | CLI helper identifier (default `virtualpete.buddy.cli`); must stay stable so Keychain ACLs survive rebuilds |
| `BUDDY_APP_IDENTIFIER` | Buddy.app identifier (default `virtualpete.buddy`); `Contents/MacOS/Buddy` is reserved for a future GUI |
| `BUDDY_PKG_SIGN_IDENTITY` | Optional Developer ID Installer identity for `make package` (branded product archive + Finder icon from `internal/serve/buddy.png`) |

---

## See also

- [README](../README.md) — quick start, configuration table, command cheat sheet
- [`docs/index.md`](index.md) — About (landing page + docs map)
- [`docs/getting-started.md`](getting-started.md) — Install, `buddy setup`, first commands
- [Auth, secrets, and tokens](auth.md) — tenants, auths, channel policy, secret stores, token cache
- [Change batches howto](/docs/docs/change-batches) — plan / approve / apply for change records
- [Editor / LSP](editor.md) — VS Code/Cursor extension and `buddy lsp`
