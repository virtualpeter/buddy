# buddy

<img src="internal/serve/buddy.png" alt="buddy" width="128" height="128">

**buddy** for Jamf Pro is a multipurpose tool with CLI, MCP, and its own scripting language. It covers Classic and Pro api functions (policies, computer groups, configuration profiles, packages/JCDS, scripts, computers and MDM, API roles and clients) and newer features only found via the Platform API (blueprints, devices, device groups, DDM reporting, compliance benchmarks).

**buddy** is independent open-source software (MIT). It talks to [Jamf Pro](https://www.jamf.com/) and Jamf Platform **APIs**. It is not affiliated with, endorsed by, or sponsored by Jamf. Jamf, Jamf Pro, and related names are trademarks of Jamf.

Day-to-day work can follow several different styles - you can work with **buddyscript** (an extended version of Starlark) via an interactive shell, `.buddy` script files, or via a local docs UI with script IDE (`buddy serve`). Alternatively, connect it as an MCP server for AI hosts (`buddy mcp`) and let the agent learn how to use buddyscript.

The only required config is to define tenants + auths; passwords or oauth secrets get stored in the encrypted vault (`secrets.enc`), with a random Keychain data key and a recovery passphrase for backups — [docs/auth.md](docs/auth.md).

New install: if there isn’t a released binary available and you have Go installed you can `make install-signed` (preferred) or `make install` (if you don’t have a signing cert), then [`buddy setup basic-setup`](docs/getting-started.md). 

## Documentation

| Resource | Description |
|----------|-------------|
| [AGENTS.md](AGENTS.md) | Contributor notes for AI agents (layout, conventions) |
| [docs/index.md](docs/index.md) | About: landing page and docs map |
| [docs/getting-started.md](docs/getting-started.md) | Install, `buddy setup`, first commands |
| [docs/auth.md](docs/auth.md) | Tenants, auths, secrets, and token caching |
| [docs/reference-manual.md](docs/reference-manual.md) | Full reference: config, CLI, scripting, builtins overview, API doc link |
| [docs/change-batches.md](docs/change-batches.md) | Plan → approve → apply change-record workflows |
| [docs/mcp.md](docs/mcp.md) | MCP server setup for AI hosts |
| [docs/mcp-tools.md](docs/mcp-tools.md) | MCP tool scorecard and agent priority |
| [docs/generated/human/profile-payloads.md](docs/generated/human/profile-payloads.md) | Profile payload dissect/assemble |
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

The binary is built to `./buddy` (`CGO_ENABLED=0`). On macOS the default binary stores the vault data key and recovery passphrase in Keychain via `security(1)` (service `buddy`).

The **signed release** (`make buddy-darwin-arm64`) uses Security.framework and a separate Keychain service (`virtualpete.buddy.cli`) so only buddy can read the DEK without a prompt. Secrets stay in `secrets.enc`. See [docs/auth.md](docs/auth.md) for Keychain ACLs, Touch ID, and `cred set passphrase`.

```sh
BUDDY_CODESIGN_IDENTITY='Developer ID Application: …' make buddy-darwin-arm64
# or: BUDDY_CODESIGN_IDENTITY='Developer ID Application: …' make install-signed
# or: BUDDY_CODESIGN_IDENTITY='Developer ID Application: …' make package
```

| Target | Binary | Keychain | Notes |
|--------|--------|----------|-------|
| `make buddy` | `./buddy` | service `buddy` via `security(1)` | Default; no CGO; existing items keep working |
| `make buddy-darwin-arm64` | `./buddy-darwin-arm64` | service `virtualpete.buddy.cli` (label Buddy) via Security.framework | Requires Darwin/arm64 and `BUDDY_CODESIGN_IDENTITY`; identifier `virtualpete.buddy.cli` (`BUDDY_CODESIGN_IDENTIFIER`) must stay stable; store the recovery passphrase with `./buddy-darwin-arm64 cred set passphrase` |
| `make install-signed` | `~/go/bin/buddy` | same as `buddy-darwin-arm64` | Builds the signed binary, then copies it over `~/go/bin/buddy` |
| `make package` | `/usr/local/bin/buddy` (via pkg) | same as `buddy-darwin-arm64` | Branded `buddy-<version>.pkg` (installer UI + Finder icon); optional `BUDDY_PKG_SIGN_IDENTITY` (Developer ID Installer) |

`make buddy-darwin-arm64` fails if the host is not Darwin/arm64, the identity is missing, the signature is ad-hoc, or TeamIdentifier is unset. Each binary only sees its own DEK until you unwrap with `cred set passphrase` or restore a backup.

```sh
make test         # default store (security CLI on macOS)
make test-signed  # Darwin: compile/test Security.framework backend (no codesign)
make clean        # remove built binaries
make help         # show all targets
```

## Configuration

Config file: `~/.config/buddy/config.yaml` (macOS/Linux; `$XDG_CONFIG_HOME/buddy` when set). On Windows: `%APPDATA%\buddy\config.yaml`.

On macOS, a Jamf **Application & Custom Settings** profile for preference domain `virtualpete.buddy.cli` can ship org defaults (tenants, channel policy). User YAML overlays and wins per name. Auths in the plist are accepted if present; `buddy setup managed-plist` omits them (names and client IDs are per-user). Prefer `buddy setup secrets` to store secrets locally. Override the plist path with `BUDDY_MANAGED_PLIST`.

```yaml
tenant:
  default: sandbox                 # global fallback

channels:
  mcp:
    access: readonly               # clamp mutates on this client
    tenant: sandbox                # optional per-client default
    tenants: [sandbox]             # optional allowlist
    # jamf_* is denied on MCP by default; extra denials union with that
    deny_builtins: [computer_erase]
    # allow_builtins: ["*"]        # cowboy: unblock all denials (including jamf_*)
  ide:
    tenant: sandbox
  # cli omitted → full access; default tenant = tenant.default

tenants:
  sandbox:
    url: https://your-instance.jamfcloud.com   # instance (Pro/Classic)
    region: us                                  # Platform gateway (us, eu, apac, …; list may grow)
    tenantId: e77c1408-10c8-4007-b177-abc9157fbcaa
    # deny_builtins: ["*"]         # tenant-layer allowlist
    # allow_builtins: [policies, policy]

auths:
  sandbox-admin:
    type: oauth
    clientId: your_client_id
    tenants: [sandbox]
    channels: [cli, ide]           # omit = all (cli/mcp/ide)
    # clientSecret: buddy cred set secret sandbox-admin --client-secret
  sandbox-platform-ro:
    type: platform
    clientId: your_integration_client_id
    tenants: [sandbox]
    access: readonly
    channels: [mcp, ide]           # restrict MCP/IDE to this readonly auth

features:
  classic_instance_only: true      # optional: force Classic onto instance. Default false lets Classic use /api/proclassic. ACS and GET /api/v1/policies are always instance.
```

**Storage:** Tenants hold host identity; auths hold type/username/clientId and link to tenants. Passwords and OAuth client secrets can stay inline for **dev/test**, or live in the encrypted vault (`secrets.enc`, keyed by **auth name**). The vault data key is one Keychain item (macOS) or `dek` (Linux/Windows); backups use a recovery passphrase (`master`):

| Platform | Vault | Data key / recovery (`BUDDY_SECRET_STORE`) |
|----------|-------|--------------------------------------------|
| macOS (`make buddy`) | `~/.config/buddy/secrets.enc` | Keychain service `buddy`, accounts `dek` / `master`, via `security(1)` (default / `keychain`) |
| macOS (`make buddy-darwin-arm64`) | same vault file | Keychain service `virtualpete.buddy.cli` (label Buddy); `cred set passphrase` unwraps the DEK |
| Linux | same | `~/.config/buddy/dek` and `master` (mode 0600), or `pass` (`buddy/dek`, `buddy/master`) |
| Windows | `%APPDATA%\buddy\secrets.enc` | `%APPDATA%\buddy\dek` and `master` |

`BUDDY_VAULT_PASSPHRASE` overrides the recovery passphrase (never written).

```sh
buddy cred set tenant sandbox --url=https://example.jamfcloud.com --region=us --tenant-id=…
buddy cred set auth sandbox-admin --type=oauth --client-id=abc --tenants=sandbox --channels=cli,ide
buddy cred set auth sandbox-platform-ro --type=platform --client-id=… --tenants=sandbox --access=readonly --channels=mcp,ide
buddy cred set channel mcp --access=readonly --tenant=sandbox
buddy cred set channel mcp --deny-builtins=computer_erase
buddy cred set channel mcp --allow-builtins=*
buddy cred set tenant sandbox --deny-builtins=* --allow-builtins=policies,policy
buddy cred set passphrase
buddy cred set secret sandbox-admin --client-secret
buddy cred backup ./buddy.creds
buddy cred restore ./buddy.creds
buddy cred use sandbox
buddy cred use --mcp sandbox
buddy cred store
buddy cred delete auth sandbox-admin   # also clears vault secrets
buddy cred clean secrets               # review/delete orphaned vault entries
buddy cred erase secrets               # wipe vault + DEK + recovery passphrase (+ leftover Keychain items)
buddy cred clean tokens                # delete expired/corrupt disk token cache files
```

Config is `tenant.default` + `tenants:` + `auths:` + optional `channels:`. Older `credentials:`, a bare string `tenant:`, `defaults:`, and `allow_auths:` are rejected. Inline secrets: `buddy cred migrate`. Selection: `--tenant` (auto-pick auth by operation) or `--user`/`--auth` (pin). Prefer readonly auths for reads when linked. Limit which buddy clients may use an auth with `channels:` (omit = all). Per-client policy lives under top-level `channels:` (`access` / `tenant` / `tenants` / `deny_builtins` / `allow_builtins`; MCP defaults deny `jamf_*`). The same deny/allow lists may be set on tenants and auths (per-layer; channel allow cannot unblock a tenant deny). `tenant.cli|mcp|serve|ide|notebook` and the `serve` / `notebook` clients are removed.
### Setting up a role and client for buddy

If you have **no config yet**, bootstrap an instance tenant + basic auth first:

```sh
buddy setup basic-setup               # prompts for tenant, URL, username, password
buddy --user <admin-auth> auth test
```

If an MDM profile already provides tenants, run `buddy setup secrets` instead (creates missing auths and fills instance/platform coverage gaps). `buddy cred status` shows remaining holes.

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
    policy_set_package(name=PACKAGE_NAME, version=version, id=policy_id)
    # If install params or self-service live on other policies, add policy_set_install / policy_set_ss
    # with those policy IDs. To bump a smart-group version criterion: smart_group_set_version(id="...", version=version).
```

A runnable file with the same ideas (and optional steps you can uncomment) lives at [examples/bump-release.buddy](examples/bump-release.buddy).

## Acknowledgements

[jamf-cli](https://github.com/Jamf-Concepts/jamf-cli) is Jamf’s official CLI. buddy started before that project was known of publicly; there is some overlap in what both tools talk to, but buddy is not an attempt to replace jamf-cli.

The only use of jamf-cli in this repo is a vendored snapshot of its Platform API OpenAPI specs (`internal/buddy/platformschema/specs/`, MIT). Jamf documents those APIs on [developer.jamf.com/platform-api](https://developer.jamf.com/platform-api) and the [developer MCP](https://developer.jamf.com/mcp), but the gateway still has no live schema like Jamf Pro (`/api/schema`) or Classic (`/classicapi/doc/swagger.yaml`), and the portal does not host downloadable OpenAPI JSON. Keep the snapshot so `jamf_schema(api="platform")` works offline. Refresh with `make sync-platform-schema` and commit the result. See [API schema documents](docs/api-endpoints.md#api-schema-documents).

## License

MIT License — see [LICENSE](LICENSE). Copyright © 2026 Peter Viertel.

## Trademark

Jamf, Jamf Pro, and related names are trademarks of Jamf. buddy is not affiliated with, endorsed by, or sponsored by Jamf. See [License](#license).

## Environment Variables

See the [reference manual](docs/reference-manual.md#environment-variables) for the full table (`DEBUG`, `XDG_CONFIG_HOME`, `BUDDY_*` runtime and build-only vars). Common ones:

| Variable          | Description                              |
|-------------------|------------------------------------------|
| `DEBUG`           | Enable debug logging when set            |
| `XDG_CONFIG_HOME` | Override config directory (default `~/.config`) |
| `BUDDY_MANAGED_PLIST` | Path to org-managed config plist (replaces Darwin Managed Preferences) |
| `BUDDY_CLASSIC_INSTANCE_ONLY` | `false` (default) allows Classic via Platform; `true` forces Classic onto instance auth |

## Project Structure

```
buddy/
├── cmd/buddy/           # CLI entry, dispatch, help
├── cmd/gendocs/         # Regenerates docs/generated and internal/help/*_gen.go
├── internal/
│   ├── buddy/           # Jamf API clients, config, auth, secrets
│   │   └── secrets/     # Encrypted vault (secrets.enc) + Keychain/file DEK
│   ├── clispec/         # Command/flag identifiers and English catalog
│   ├── help/            # CLI help strings (some generated)
│   ├── script/          # buddyscript runner and builtins
│   ├── mcp/             # buddy mcp stdio server
│   ├── serve/           # buddy serve docs + /ide
│   └── lsp/             # buddy lsp
├── examples/            # Runnable .buddy samples
├── docs/                # User guides; docs/generated is from make gendocs
├── Makefile
└── README.md
```
