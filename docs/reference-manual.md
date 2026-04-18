# Buddy reference manual

**buddy** is a command-line tool for [Jamf Pro](https://www.jamf.com/products/jamf-pro/) automation: policies, computer groups, configuration profiles, packages, scripts, computers / MDM, API roles, and API clients. It includes a [Starlark](https://github.com/google/starlark-go) runtime so multi-step workflows can live in `.star` scripts.

This manual describes how the pieces fit together. For behavior that must stay in lockstep with the code, buddy exposes the same text via `buddy help` (see [CLI help](#cli-help)).

---

## Table of contents

1. [License](#license)
2. [Installation and build](#installation-and-build)
3. [Configuration and credentials](#configuration-and-credentials)
4. [First-time API setup](#first-time-api-setup)
5. [CLI overview](#cli-overview)
6. [Starlark scripts (`buddy run`)](#starlark-scripts-buddy-run)
7. [Built-in functions](#built-in-functions)
8. [HTTP API usage](#http-api-usage)
9. [Examples](#examples)
10. [Environment variables](#environment-variables)

---

## License

This project is released under the MIT License. See the [`LICENSE`](../LICENSE) file in the repository root. Copyright © 2026 Peter Viertel.

---

## Installation and build

**Requirements**

- Go 1.26 or newer
- Optional: a helper command that prints a Bearer token for `command` auth (see [Configuration](#configuration-and-credentials))

**Build**

```sh
make buddy
```

The binary is written to `./buddy`. `make clean` removes it; `make help` lists Makefile targets.

---

## Configuration and credentials

Default config path: `~/.config/buddy/config.yaml` (override config root with `XDG_CONFIG_HOME`).

A minimal file defines a default credential name and one or more named credentials (each with a Jamf Pro base URL and auth: `command`, `basic`, or `oauth`). Use:

- `buddy config help` — add, list, delete credentials; set the default name
- `buddy auth test` — verify the active credential
- `buddy --cred <name> ...` — override the default credential for one invocation

For `basic` auth, buddy exchanges username/password for a Bearer token via Jamf’s `POST /api/v1/auth/token`.

**Security:** Storing **passwords, OAuth client secrets, or tokens as plain text inside the YAML file** is practical for **development and test tenants**, but it is **not** a good pattern for production or high-value Jamf environments. For those, use **`command` authentication**: buddy runs an external command whose stdout is the Bearer token—point that command at a credential helper or store (for example **jamftoken**, the macOS keychain, or a team vault script) so secrets are not persisted in `config.yaml`. Regardless of method, keep the config file off shared disks where possible, use restrictive file permissions, and never commit credentials to source control.

Full YAML shape and field descriptions are in the [README](../README.md#configuration).

---

## First-time API setup

To create a Jamf API role with the privileges buddy needs and an API client scoped to that role:

```sh
buddy setup write-script              # writes setup_buddy_role.star (or pass a path)
buddy run setup_buddy_role.star -y    # idempotent: reuses an existing "Buddy" role when present
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
| `--cred <name>` | Use a named credential from config |

**Commands** (high level)

| Command | Role |
|---------|------|
| `auth` | Test auth; print token material |
| `config` | Manage `config.yaml` credentials |
| `run` | Execute a `.star` script |
| `policy` | Policies (get/set/clone/create/delete/find) |
| `group` | Smart/static computer groups |
| `profile` | macOS configuration profiles |
| `package` | Package records (upload, metadata, history) |
| `computer` | Computer inventory and MDM actions |
| `mdm` | Query MDM command history (RSQL) |
| `role` | API roles |
| `client` | API integrations / OAuth clients |
| `script` | Jamf script objects |
| `setup` | Generate setup scripts and OAuth config helpers |

### CLI help

The complete, version-accurate command reference is embedded in the binary:

```sh
buddy help
buddy help policy
buddy policy set help
```

Use these when unsure about positional arguments, flags, or subcommand names.

---

## Starlark scripts (`buddy run`)

**Usage**

```sh
buddy [global-flags] run [--trace] <script.star> [-- args...]
```

- Scripts are **Starlark** (a deterministic Python-like language). See **Language** below.
- `args` after `--` are available as the global list `args`.
- The active credential name is available as `cred` (override per builtin with `cred=`).
- The regex module is exposed as `re` (see `buddy run help starlark` for what Starlark provides globally).
- **`print()`** writes to stdout; buddy adds **`error(*args, ...)`** for stderr (same signature as `print`). Prefer **`error`** for progress, skips, and failures so stdout stays free for machine-readable output; reserve **`print`** for lines meant to be piped or captured as the script’s primary result.
- **`--trace` on `run`** logs each builtin call (and optionally Starlark `print` tracing) to stderr. **HTTP tracing** uses the top-level `-t` / `--trace` on `buddy` before `run`: `buddy -t run script.star`.

### Language

Starlark quick-reference (types, control flow, string/list/dict methods, what is intentionally absent vs Python):

```sh
buddy run help starlark
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

Buddy registers many functions for policies, groups, profiles, packages, scripts, computers, MDM, roles, clients, JSON/plist, local file I/O, and a **declarative workflow** layer (`step_*` + `run_workflow`).

**Canonical reference** (always matches your installed `buddy` binary):

```sh
buddy run help builtins
```

That output lists every builtin with parameter names, types, and return shapes. **Treat it as the source of truth**; this manual only orients you.

**Conventions** (from the builtins help header):

- Resource IDs come first (`policy_id`, `group_id`, …), then options, then optional `cred?` last.
- Use `cred="name"` to override the active credential for a single call.

---

## HTTP API usage

Which Jamf Pro URLs buddy calls (Classic XML vs JSON APIs, pagination, notes) is documented separately:

- [**Jamf Pro API endpoints used by buddy**](api-endpoints.md)

Use it when correlating buddy behavior with Jamf’s API or troubleshooting permissions.

---

## Examples

Runnable Star scripts and a short index live under [**examples/**](../examples/README.md):

- [`examples/bump-release.star`](../examples/bump-release.star) — minimal pattern: `policy_get_id`, then updates (edit triggers/IDs for your tenant).
- For broad integration testing against a non-production Jamf instance, see [`scripts/integration_test.star`](../scripts/integration_test.star) (creates and tears down test resources; read before running).

---

## Environment variables

| Variable | Effect |
|----------|--------|
| `DEBUG` | Enable debug logging when set |
| `XDG_CONFIG_HOME` | Config directory root (default: `~/.config`) |

---

## See also

- [README](../README.md) — quick start, configuration table, command cheat sheet
- [`docs/index.md`](index.md) — index of documentation files
