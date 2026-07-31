# buddy

CLI tool for managing Jamf Pro policies and smart groups. Automates common tasks like version bumps, policy cloning, scope changes, and package updates via the Jamf Pro API.

Includes a Starlark scripting engine for composing multi-step workflows.

## Documentation

| Resource | Description |
|----------|-------------|
| [AGENTS.md](AGENTS.md) | Agent/contributor context: auth, secrets, token caching, meliorate integration |
| [docs/index.md](docs/index.md) | Getting started: install, `buddy setup`, first commands |
| [docs/reference-manual.md](docs/reference-manual.md) | Full reference: config, CLI, scripting, builtins overview, API doc link |
| [docs/change-batches.md](docs/change-batches.md) | Plan → approve → apply change-record workflows |
| [docs/mcp.md](docs/mcp.md) | MCP server setup for AI hosts |
| [docs/mcp-tools.md](docs/mcp-tools.md) | MCP tool scorecard and agent priority |
| [docs/profile-payloads.md](docs/profile-payloads.md) | Profile payload dissect/assemble |
| [docs/editor.md](docs/editor.md) | `buddy lsp` + VS Code/Cursor extension (build & install VSIX) |
| [docs/api-endpoints.md](docs/api-endpoints.md) | Jamf HTTP endpoints used by buddy |
| [examples/](examples/) | Runnable `.buddy` examples and index |

Complete builtin signatures always match your installed binary: `buddy help builtins`.

## Prerequisites

- Go 1.26+
- Optional: `pass` on Linux when using `BUDDY_SECRET_STORE=pass`

## Build

```sh
make buddy
```

The binary is built to `./buddy`.

```sh
make clean    # remove built binary
make help     # show all targets
```

## Configuration

Config file: `~/.config/buddy/config.yaml`

```yaml
tenant:
  default: sandbox                 # global fallback

channels:
  mcp:
    access: readonly               # clamp mutates on this client
    tenant: sandbox                # optional per-client default
    tenants: [sandbox]             # optional allowlist
  notebook:
    tenant: sandbox
  ide:
    tenant: sandbox
  # cli omitted → full access; default tenant = tenant.default

tenants:
  sandbox:
    url: https://your-instance.jamfcloud.com   # instance (Pro/Classic)
    region: us                                  # optional Platform gateway
    tenantId: e77c1408-10c8-4007-b177-abc9157fbcaa

auths:
  sandbox-admin:
    type: oauth
    clientId: your_client_id
    tenants: [sandbox]
    channels: [cli, ide]           # omit = all (cli/mcp/ide/notebook)
    # clientSecret: buddy cred set secret sandbox-admin --client-secret
  sandbox-platform-ro:
    type: oauth
    clientId: your_integration_client_id
    tenants: [sandbox]
    access: readonly
    surface: platform
    channels: [mcp, notebook]      # restrict MCP/notebook to this readonly auth
```

**Storage:** Tenants hold host identity; auths hold type/username/clientId and link to tenants. Passwords and OAuth client secrets can stay inline for **dev/test**, or be stored outside the file (keyed by **auth name**):

| Platform | Default store | Override |
|----------|---------------|----------|
| macOS | Keychain (service `buddy`) | `BUDDY_SECRET_STORE=keychain` |
| Linux | `~/.config/buddy/secrets/<auth>.json` (mode 0600) | `BUDDY_SECRET_STORE=file` or `pass` |

```sh
buddy cred set tenant sandbox --url=https://example.jamfcloud.com --region=us --tenant-id=…
buddy cred set auth sandbox-admin --type=oauth --client-id=abc --tenants=sandbox --channels=cli,ide
buddy cred set auth sandbox-platform-ro --type=oauth --client-id=… --tenants=sandbox --access=readonly --surface=platform --channels=mcp,notebook
buddy cred set channel mcp --access=readonly --tenant=sandbox
buddy cred set secret sandbox-admin --client-secret
buddy cred use sandbox
buddy cred use --mcp sandbox
buddy cred store
buddy cred delete auth sandbox-admin   # also clears store secrets
buddy cred clean secrets               # review/delete orphaned store entries
```

Legacy shapes (`credentials:`, string `tenant:`, `defaults:`, `allow_auths:`) migrate automatically on load (breaking; only the new form is written back). Inline secrets: `buddy cred migrate`. Selection: `--tenant` (auto-pick auth by operation) or `--user`/`--auth` (pin). Prefer readonly auths for reads when linked. Limit which buddy clients may use an auth with `channels:` (omit = all). Per-client policy lives under top-level `channels:` (`access` / `tenant` / `tenants`); `tenant.cli|mcp|serve|ide` and the `serve` client are removed.
### Setting up a role and client for buddy

If you have **no config yet**, bootstrap an instance tenant + basic auth first:

```sh
buddy setup basic-setup               # prompts for tenant, URL, username, password
buddy --user <admin-auth> auth test
```

Then (optional) create a Jamf Pro **instance** API role/client and store oauth (Platform gateway oauth cannot manage roles/integrations):

```sh
buddy setup write-script              # writes setup_buddy_role.buddy (or specify a path)
buddy setup_buddy_role.buddy -y       # idempotent: reuses role "Buddy" and client "buddy-cli"
buddy setup store-oauth               # rotate instance client secret; store/update oauth auth (surface=instance)
buddy --user <auth-name> auth test
```

Alternatively, after the script you can run `buddy client credentials <id>` to get the client secret (shown once) and add an auth manually with `buddy cred set auth …`.

The generated script fetches privilege names from your Jamf instance at runtime and reuses an existing role named "Buddy" (and client "buddy-cli") if present.

The role the script targets may include **destructive or high-impact privileges** (for example wiping a computer, sending erase MDM commands, or similar Jamf permissions buddy can use in automation). That keeps a single setup path flexible, but it is **not** always appropriate to grant every such privilege to the API client: for day-to-day or production use, review the role in Jamf and **remove privileges you do not actually need**, then narrow the role before distributing credentials.

## Usage

```
buddy [flags]                              # interactive Starlark shell
buddy [flags] <script.buddy> [-- args...]  # run a script (.buddy preferred; .star still works)
buddy [flags] <command> <subcommand> [args...]
```

### Global Flags

| Flag            | Description                     |
|-----------------|---------------------------------|
| `-y`            | Skip confirmation prompts       |
| `-v, --verbose` | Verbose output (show payloads)  |
| `--tenant <name>` | Tenant from config (auto-picks auth) |
| `--user <name>` | Pin an auth (`--auth` alias) |
| `--script-trace` | Trace Starlark builtin calls in scripts and the shell |

### Policy Commands

#### Get

```sh
buddy policy get --trigger=<trigger> [-f] # Find policy ID by trigger (-f to force cache refresh)
buddy policy get enable <policy-id>       # Show enabled/disabled state
buddy policy get xml <policy-id>          # Output policy XML to stdout
```

#### Set

```sh
buddy policy set ss <policy-id> <version> [--group-ids=1,2,3]   # Update self-service description
buddy policy set scope <policy-id> <group-id> [group-id...]      # Set scope to computer groups
buddy policy set enable <policy-id>                               # Enable policy
buddy policy set disable <policy-id>                              # Disable policy
buddy policy set trigger <policy-id> <trigger>                    # Set custom trigger
buddy policy set install <policy-id> <version> [--param=4,5,6]   # Update script param versions
buddy policy set package <app> <version> [policy-id] [--trigger=<trigger>] [--package=<pattern>]  # Update package policy (--trigger for non-standard triggers; --package overrides search pattern)
buddy policy set xml <policy-id>                                  # PUT policy from XML on stdin
```

#### Clone

```sh
buddy policy clone <policy-id> <version> [--param=4,5,6] [--package=<pattern>]   # Clone policy with new version, disable source
```

### Group Commands

```sh
buddy group get version <group-id>                # Show current version criterion
buddy group set version <group-id> <version>      # Update version criterion
```

### Scripting

buddy embeds a [Starlark](https://github.com/google/starlark-go) interpreter with builtins for policies, groups, profiles, packages, scripts, computers, MDM, roles, clients, JSON/plist/XML helpers, RSQL filter builders, and more.

```sh
buddy                              # interactive Starlark shell
buddy <script.buddy> [-- args...]  # .buddy preferred; any existing path (including .star) still runs
```

Prefer the **`.buddy`** extension for scripts and stdlib modules (`load("stdlib/<name>.buddy", …)`) so the Cursor/VS Code extension language id applies. Existing `.star` paths still run. Editor support: [docs/editor.md](docs/editor.md) (`buddy lsp` + `make extension-package`).

**Built-in functions:** plural nouns enumerate (`policies`, `packages`, …); singular nouns load one resource (`policy(id=…)` / `policy(trigger=…)`); singular_verb mutates (`policy_set`, `computer_reboot`). Optional `tenant=` / `surface=` / `auth=` last (`cred=` removed). Every Jamf-related builtin returns `{"ok": true, "value": ...}` or `{"ok": false, "error": "..."}` unless documented otherwise. Canonical list: `buddy help builtins`; grammar overview: [docs/reference-manual.md](docs/reference-manual.md#built-in-functions).

```sh
buddy help starlark   # language subset
buddy help builtins   # full builtin reference (canonical)
buddy help lsp        # language server for the editor extension
make docs-serve       # local docs + Starlark notebook (buddy serve --open)
buddy mcp             # MCP stdio server for AI tools (see docs/mcp.md)
buddy mcp setup       # discover/write well-known MCP host configs
make sync-docfs       # refresh embedded knowledge after editing docs/rules/AGENTS
```

**Globals:** `tenant` and `auth` (active session selection; empty `auth` = auto-pick), `args` (arguments after `--`), `re` (regex). Use **`error(...)`** for status and diagnostic lines (stderr); use **`print(...)`** only when you intentionally write script output to stdout (e.g. for piping). If the script defines `main`, buddy calls it after loading—do **not** add a trailing `main()` (that would run it twice).

Sample scripts: [examples/](examples/).

#### Example (illustrative)

```python
# Edit TRIGGER_PREFIX / APP_NAME for your policies; see examples/bump-release.buddy for a fuller sample.
# Usage: buddy examples/bump-release.buddy -- 2.4.27

TRIGGER_PREFIX = "install-exampleapp-"
APP_NAME = "exampleapp"

def main():
    version = args[0]

    r = policy(trigger=TRIGGER_PREFIX + version, force_refresh=True)
    if not r["ok"]:
        error("Policy not found:", r["error"])
        exit(1)

    policy_id = r["value"]["id"]
    policy_set_package(app=APP_NAME, version=version, policy_id=policy_id)
    # If install params or self-service live on other policies, add policy_set_install / policy_set_ss
    # with those policy IDs. To bump a smart-group version criterion: smart_group_set_version(group_id="...", version=version).
```

A runnable file with the same ideas (and optional steps you can uncomment) lives at [examples/bump-release.buddy](examples/bump-release.buddy).

## License

MIT License — see [LICENSE](LICENSE). Copyright © 2026 Peter Viertel.

## Environment Variables

| Variable          | Description                              |
|-------------------|------------------------------------------|
| `DEBUG`           | Enable debug logging when set            |
| `XDG_CONFIG_HOME` | Override config directory (default `~/.config`) |

## Project Structure

```
buddy/
├── LICENSE
├── cmd/buddy/
│   ├── main.go         # CLI entry point
│   └── help.go         # Help text
├── internal/
│   ├── buddy/          # Library: config, API client, policy/group/computer/profile/package/role/mdm/Jamf script
│   │   ├── apiclient.go
│   │   ├── auth.go
│   │   ├── common.go    # Logger, HTTP helpers (PerformRequest, GetBaseURL)
│   │   ├── config.go
│   │   ├── computer.go
│   │   ├── group.go
│   │   ├── mdm.go
│   │   ├── package.go
│   │   ├── policy.go
│   │   ├── policy_types.go
│   │   ├── profile.go
│   │   ├── role.go
│   │   ├── setup.go      # Setup script generator (role + client)
│   │   └── script_jamf.go
│   ├── script/         # Starlark runner and builtins
│   └── lsp/            # buddy lsp (editor language server)
├── editors/vscode/      # Cursor/VS Code extension (.buddy)
├── scripts/
│   └── integration_test.buddy  # Broad Jamf integration harness (non-production)
├── examples/            # Runnable .buddy samples (see examples/README.md)
├── docs/                # Reference manual, API catalog
├── go.mod
├── go.sum
├── Makefile
└── README.md
```
