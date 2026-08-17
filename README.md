# buddy

**buddy** is a Jamf Pro automation CLI. It covers Classic and Pro resources (policies, computer groups, configuration profiles, packages/JCDS, scripts, computers and MDM, API roles and clients) and the Platform API (blueprints, devices, device groups, DDM reporting, compliance benchmarks).

Day-to-day work is **buddyscript** (Starlark): an interactive shell, `.buddy` files, a local docs UI with script IDE (`buddy serve`), and an MCP server for AI hosts (`buddy mcp`). Config is tenants + auths; secrets belong in Keychain or the file store — [docs/auth.md](docs/auth.md).

New install: `make buddy` then [`buddy setup basic-setup`](docs/index.md). Builtin signatures always match the binary: `buddy help builtins`.

## Documentation

| Resource | Description |
|----------|-------------|
| [AGENTS.md](AGENTS.md) | Contributor notes for AI agents (layout, conventions) |
| [docs/index.md](docs/index.md) | Getting started: install, `buddy setup`, first commands |
| [docs/auth.md](docs/auth.md) | Tenants, auths, secrets, and token caching |
| [docs/reference-manual.md](docs/reference-manual.md) | Full reference: config, CLI, scripting, builtins overview, API doc link |
| [docs/change-batches.md](docs/change-batches.md) | Plan → approve → apply change-record workflows |
| [docs/mcp.md](docs/mcp.md) | MCP server setup for AI hosts |
| [docs/mcp-tools.md](docs/mcp-tools.md) | MCP tool scorecard and agent priority |
| [docs/profile-payloads.md](docs/profile-payloads.md) | Profile payload dissect/assemble |
| [docs/editor.md](docs/editor.md) | `buddy lsp`; VSIX is the sibling [buddy-extension](../buddy-extension) checkout |
| [docs/api-endpoints.md](docs/api-endpoints.md) | Jamf HTTP endpoints used by buddy |
| [examples/](examples/) | Runnable `.buddy` examples and index |

## Prerequisites

- Go 1.26+
- Optional: `pass` on Linux when using `BUDDY_SECRET_STORE=pass`

## Build

```sh
make buddy
```

The binary is built to `./buddy` (`CGO_ENABLED=0`). On macOS that binary stores secrets in Keychain via `security(1)` under service `buddy` (writes use `security -i` so the secret is not on argv). Linux/Windows use the file store. See [docs/auth.md](docs/auth.md).

`security(1)` is rebuild-stable and needs no CGO, but Keychain ACLs attach to `/usr/bin/security`, not to buddy. Any process running as you can read those items the same way. The **signed release** calls Security.framework instead so the ACL is the creating app: only a binary signed with the same Developer ID and identifier can read secrets without a prompt. Rebuilds and upgrades keep working because the designated requirement is the Team ID, not the binary hash (ad-hoc / unsigned `go build` output would prompt on every rebuild — that is why this path requires a stable Developer ID).

The signed binary uses a **different Keychain service** (`virtualpete.buddy.cli`, Keychain Access label **Buddy**) so it never shares items or ACLs with the CLI store. Development can keep using `./buddy`; production uses `./buddy-darwin-arm64` after `cred set secret` (or migrate) against that binary. `make install-signed` copies it to `~/go/bin/buddy`.

```sh
BUDDY_CODESIGN_IDENTITY='Developer ID Application: …' make buddy-darwin-arm64
# or: BUDDY_CODESIGN_IDENTITY='Developer ID Application: …' make install-signed
# or: BUDDY_CODESIGN_IDENTITY='Developer ID Application: …' make package
```

| Target | Binary | Keychain | Notes |
|--------|--------|----------|-------|
| `make buddy` | `./buddy` | service `buddy` via `security(1)` | Default; no CGO; existing items keep working |
| `make buddy-darwin-arm64` | `./buddy-darwin-arm64` | service `virtualpete.buddy.cli` (label Buddy) via Security.framework | Requires Darwin/arm64 and `BUDDY_CODESIGN_IDENTITY`; identifier `virtualpete.buddy.cli` (`BUDDY_CODESIGN_IDENTIFIER`) must stay stable; re-store secrets with `./buddy-darwin-arm64 cred set secret` |
| `make install-signed` | `~/go/bin/buddy` | same as `buddy-darwin-arm64` | Builds the signed binary, then copies it over `~/go/bin/buddy` |
| `make package` | `/usr/local/bin/buddy` (via pkg) | same as `buddy-darwin-arm64` | Builds `buddy-<version>.pkg`; optional `BUDDY_PKG_SIGN_IDENTITY` (Developer ID Installer) |

`make buddy-darwin-arm64` fails if the host is not Darwin/arm64, the identity is missing, the signature is ad-hoc, or TeamIdentifier is unset. You can keep both binaries; each only sees its own service, so a secret stored with one is invisible to the other until you store it again.

```sh
make test         # default store (security CLI on macOS)
make test-signed  # Darwin: compile/test Security.framework backend (no codesign)
make clean        # remove built binaries
make help         # show all targets
```

## Configuration

Config file: `~/.config/buddy/config.yaml` (macOS/Linux; `$XDG_CONFIG_HOME/buddy` when set). On Windows: `%APPDATA%\buddy\config.yaml`.

```yaml
tenant:
  default: sandbox                 # global fallback

channels:
  mcp:
    access: readonly               # clamp mutates on this client
    tenant: sandbox                # optional per-client default
    tenants: [sandbox]             # optional allowlist
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
    channels: [cli, ide]           # omit = all (cli/mcp/ide)
    # clientSecret: buddy cred set secret sandbox-admin --client-secret
  sandbox-platform-ro:
    type: oauth
    clientId: your_integration_client_id
    tenants: [sandbox]
    access: readonly
    surface: platform
    channels: [mcp, ide]           # restrict MCP/IDE to this readonly auth
```

**Storage:** Tenants hold host identity; auths hold type/username/clientId and link to tenants. Passwords and OAuth client secrets can stay inline for **dev/test**, or be stored outside the file (keyed by **auth name**):

| Platform | Default store | Override |
|----------|---------------|----------|
| macOS (`make buddy`) | Keychain service `buddy` via `security(1)` | `BUDDY_SECRET_STORE=keychain` |
| macOS (`make buddy-darwin-arm64`) | Keychain service `virtualpete.buddy.cli` (label Buddy) via Security.framework | `BUDDY_SECRET_STORE=keychain` (re-store secrets; items do not overlap) |
| Linux | `~/.config/buddy/secrets/<auth>.json` (mode 0600) | `BUDDY_SECRET_STORE=file` or `pass` |
| Windows | `%APPDATA%\buddy\secrets\<auth>.json` | `BUDDY_SECRET_STORE=file` |

```sh
buddy cred set tenant sandbox --url=https://example.jamfcloud.com --region=us --tenant-id=…
buddy cred set auth sandbox-admin --type=oauth --client-id=abc --tenants=sandbox --channels=cli,ide
buddy cred set auth sandbox-platform-ro --type=oauth --client-id=… --tenants=sandbox --access=readonly --surface=platform --channels=mcp,ide
buddy cred set channel mcp --access=readonly --tenant=sandbox
buddy cred set channel mcp --deny-builtins=jamf_*,computer_erase
buddy cred set secret sandbox-admin --client-secret
buddy cred use sandbox
buddy cred use --mcp sandbox
buddy cred store
buddy cred delete auth sandbox-admin   # also clears store secrets
buddy cred clean secrets               # review/delete orphaned store entries
buddy cred clean tokens                # delete expired/corrupt disk token cache files
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
buddy [flags]                              # interactive buddyscript shell
buddy [flags] <script.buddy> [-- args...]  # run a script
buddy [flags] <command> <subcommand> [args...]
```

### Global Flags

| Flag            | Description                     |
|-----------------|---------------------------------|
| `-y`            | Skip confirmation prompts       |
| `-v, --verbose` | Verbose output (show payloads)  |
| `--tenant <name>` | Tenant from config (auto-picks auth) |
| `--user <name>` | Pin an auth (`--auth` alias) |
| `--script-trace` | Trace buddyscript builtin calls in scripts and the shell |

### Policy Commands

#### Get

```sh
buddy policy get <policy-id>              # Show policy details (JSON)
buddy policy get id <trigger>             # Resolve custom trigger to policy ID
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
buddy policy set package <policy-id> <version> [--name=<name>] [--package=<pattern>]  # Update package policy (--name picks the package search keyword; --package overrides the search pattern)
buddy policy set package <version> --trigger=<trigger>            # Same, resolving the policy by custom trigger
buddy policy set xml <policy-id>                                  # PUT policy from XML on stdin
```

#### List

```sh
buddy policy list [--filter=<keyword>] [--ids]    # List policies (keyword filter; --ids for scripting)
```

### Group Commands

```sh
buddy group smart members <group-id> [--filter=…]  # Effective membership (computer IDs)
buddy group smart criteria <group-id>               # Show criteria JSON
buddy group smart set-criteria <group-id> [--file=|-]  # Replace criteria (JSON array)
buddy group smart version <group-id>                # Show current version criterion
buddy group smart set-version <group-id> <version>  # Update version criterion
buddy group static members <group-id> [--filter=…]  # Static group membership
```

### Scripting

buddy runs **buddyscript** on a [Starlark](https://github.com/google/starlark-go) interpreter with builtins for policies, groups, profiles, packages, scripts, computers, MDM, roles, clients, JSON/plist/XML helpers, RSQL filter builders, and more.

```sh
buddy                              # interactive buddyscript shell
buddy <script.buddy> [-- args...]
```

Scripts use **`.buddy`**. Editor support: [docs/editor.md](docs/editor.md) (`buddy lsp`; VSIX in sibling `buddy-extension`).

**Built-in functions:** plural nouns enumerate (`policies`, `packages`, …); singular nouns load one resource (`policy(id=…)` / `policy(trigger=…)`); singular_verb mutates (`policy_set`, `computer_reboot`). Optional `tenant=` / `surface=` / `auth=` last (`cred=` removed). Jamf-related builtins return **bare values** and **raise** on failure (MCP tools still wrap `{ok, value}` at the tool boundary). Canonical list: `buddy help builtins`; grammar overview: [docs/reference-manual.md](docs/reference-manual.md#built-in-functions).

```sh
buddy help starlark   # language subset
buddy help builtins   # full builtin reference (canonical)
buddy help lsp        # language server for the editor extension
make docs-serve       # local docs + script IDE (buddy serve --open)
buddy mcp             # MCP stdio server for AI tools (see docs/mcp.md)
buddy mcp setup       # discover/write well-known MCP host configs
make gendocs          # regenerate dual docs/help from Go docspecs (Resources + Platform)
make sync-docfs       # refresh embedded docs after editing docs/ (runs gendocs)
```

**Globals:** `tenant` and `auth` (active session selection; empty `auth` = auto-pick), `args` (arguments after `--`), `re` (regex). Use **`error(...)`** for status and diagnostic lines (stderr); use **`print(...)`** only when you intentionally write script output to stdout (e.g. for piping). If the script defines `main`, buddy calls it after loading—do **not** add a trailing `main()` (that would run it twice).

Sample scripts: [examples/](examples/).

#### Example (illustrative)

```python
# Edit TRIGGER_PREFIX / PACKAGE_NAME for your policies; see examples/bump-release.buddy for a fuller sample.
# Usage: buddy examples/bump-release.buddy -- 2.4.27

TRIGGER_PREFIX = "install-exampleapp-"
PACKAGE_NAME = "exampleapp"

def main():
    version = args[0]

    policy_id = policy(trigger=TRIGGER_PREFIX + version)["id"]
    policy_set_package(name=PACKAGE_NAME, version=version, policy_id=policy_id)
    # If install params or self-service live on other policies, add policy_set_install / policy_set_ss
    # with those policy IDs. To bump a smart-group version criterion: smart_group_set_version(group_id="...", version=version).
```

A runnable file with the same ideas (and optional steps you can uncomment) lives at [examples/bump-release.buddy](examples/bump-release.buddy).

## Acknowledgements

[jamf-cli](https://github.com/Jamf-Concepts/jamf-cli) is Jamf’s official CLI. buddy started before that project was known of publicly; there is some overlap in what both tools talk to, but buddy is not an attempt to replace jamf-cli.

The only use of jamf-cli in this repo is a vendored snapshot of its Platform API OpenAPI specs (`internal/buddy/platformschema/specs/`, MIT). The Platform API Gateway does not publish a single live schema the way Jamf Pro (`/api/schema`) and Classic (`/classicapi/doc/swagger.yaml`) do, and I have not found a better official source in Jamf’s documentation. When an update is needed refresh the copies with `make sync-platform-schema` and commit the result.

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
│   ├── script/         # buddyscript runner and builtins
│   └── lsp/            # buddy lsp (editor language server)
├── scripts/
│   └── integration_test.buddy  # Broad Jamf integration harness (non-production)
├── examples/            # Runnable .buddy samples (see examples/README.md)
├── docs/                # Reference manual, API catalog
├── go.mod
├── go.sum
├── Makefile
└── README.md
```
