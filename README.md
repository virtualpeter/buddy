# buddy

<img src="internal/serve/buddy.png" alt="buddy" width="128" height="128">

**buddy** for Jamf Pro is a multipurpose tool with CLI, MCP, and its own scripting language. It covers Classic and Pro api functions (policies, computer groups, configuration profiles, packages/JCDS, scripts, computers and MDM, API roles and clients) and newer features only found via the Platform API (blueprints, devices, device groups, DDM reporting, compliance benchmarks, Account licenses).

**buddy** is independent open-source software (MIT). It talks to [Jamf Pro](https://www.jamf.com/) and Jamf Platform **APIs**. It is not affiliated with, endorsed by, or sponsored by Jamf. Jamf, Jamf Pro, and related names are trademarks of Jamf.

Day-to-day work can follow several different styles - you can work with **buddyscript** (an extended version of Starlark) via an interactive shell, `.buddy` script files, or via a local docs UI with script IDE (`buddy serve`). Alternatively, connect it as an MCP server for AI hosts (`buddy mcp`) and let the agent learn how to use buddyscript.

The only required config is a **context** plus an **auth**; passwords or oauth secrets get stored in the encrypted vault (`secrets.enc`), with a random Keychain data key and a **user-held** recovery passphrase for backups — [docs/auth.md](docs/auth.md). Optional macOS **Buddy Agent** can own integrations and mint tokens instead of the vault. Instance-only (Jamf Pro URL + oauth/basic) is enough; Platform is optional.

New install: if there isn’t a released binary available and you have Go installed you can `make install-signed` (preferred) or `make install` (if you don’t have a signing cert), then run any Jamf command (e.g. `buddy policy list`). On macOS buddy first looks for a running Buddy Agent; otherwise you get the first-run chooser (or [`buddy cred add`](docs/getting-started.md)). 

## Documentation

| Resource | Description |
|----------|-------------|
| [AGENTS.md](AGENTS.md) | Contributor notes for AI agents (layout, conventions) |
| [docs/index.md](docs/index.md) | About: landing page and docs map |
| [docs/getting-started.md](docs/getting-started.md) | Install, `buddy setup`, first commands |
| [docs/releases.md](docs/releases.md) | Notable changes per tagged version |
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

- Go 1.27+
- Optional: `pass` on Linux when using `BUDDY_SECRET_STORE=pass`

## Build

```sh
make buddy
```

Build artefacts go under `dist/` (`DIST=dist` by default). The default binary is `dist/buddy` (`CGO_ENABLED=0`). On macOS that binary stores the vault data key in Keychain via `security(1)` (service `buddy`).

The **signed release** (`make buddy-darwin-arm64` / `make app`) uses Security.framework and a separate Keychain service (`virtualpete.buddy.cli`). Vault unlock prompts once per process (LocalAuthentication on Developer ID CLI; Keychain `userPresence` when entitled). Secrets stay in `secrets.enc`. See [docs/auth.md](docs/auth.md).

```sh
make buddy-darwin-arm64
# or: make app
# or: make install-signed
# or: make package
# Override: BUDDY_CODESIGN_IDENTITY='Developer ID Application: …' make buddy-darwin-arm64
```

| Target | Binary | Keychain | Notes |
|--------|--------|----------|-------|
| `make buddy` | `dist/buddy` | service `buddy` via `security(1)` | Default; no CGO; existing items keep working |
| `make buddy-darwin-arm64` | `dist/buddy-darwin-arm64` | service `virtualpete.buddy.cli` (label Buddy) via Security.framework | Requires Darwin/arm64; Hardened Runtime + Developer ID; identifier `virtualpete.buddy.cli` (`BUDDY_CODESIGN_IDENTIFIER`) must stay stable; unwrap a missing DEK with `dist/buddy-darwin-arm64 cred set passphrase` |
| `make app` | `dist/Buddy.app` | same as helper | CLI at `Contents/Helpers/buddy`; `Contents/MacOS/Buddy` is a SwiftUI shell (`macos/Buddy/`) that hosts `buddy serve --embed` in WKWebView tabs (Docs, Config, IDE) |
| `make install-signed` | `~/go/bin/buddy` → helper | same as `Buddy.app` | Copies `dist/Buddy.app` to `~/Applications`; **symlink** (not a copy) at `~/go/bin/buddy` |
| `make package` | `dist/buddy-<version>.pkg` | same as `Buddy.app` | Branded pkg installs `/Applications/Buddy.app` and a **symlink** at `/usr/local/bin/buddy`; optional `BUDDY_PKG_SIGN_IDENTITY` (Developer ID Installer) |
| `make release` | pkg + linux + windows under `dist/` | same as package | `clean` then `package` + `buddy-linux` + `buddy-windows` (macOS host) |

`make buddy-darwin-arm64` fails if the host is not Darwin/arm64, the identity is missing, the signature is ad-hoc, or TeamIdentifier is unset. Each binary only sees its own DEK until you unwrap with `cred set passphrase` or restore a backup.

```sh
make test         # default store (security CLI on macOS)
make test-signed  # Darwin: compile/test Security.framework backend (no codesign)
make clean        # remove built binaries
make help         # show all targets
```

## Configuration

Config file: `~/.config/buddy/config.yaml` (macOS/Linux; `$XDG_CONFIG_HOME/buddy` when set). On Windows: `%APPDATA%\buddy\config.yaml`.

On macOS, a Jamf **Application & Custom Settings** profile for preference domain `virtualpete.buddy.cli` can ship org defaults (contexts, channel policy). User YAML overlays and wins per name. Auths in the plist are accepted if present; `buddy setup managed-plist` omits them (names and client IDs are per-user). Prefer `buddy setup secrets` to store secrets locally. Override the plist path with `BUDDY_MANAGED_PLIST`.

```yaml
context:
  default: sandbox                 # global fallback

channels:
  mcp:
    access: readonly               # clamp mutates on this client
    context: sandbox               # optional per-client default
    contexts: [sandbox]            # optional allowlist
    # jamf_* is denied on MCP by default; extra denials union with that
    deny_builtins: [computer_erase]
    # allow_builtins: ["*"]        # cowboy: unblock all denials (including jamf_*)
  ide:
    context: sandbox
  # cli omitted → full access; default context = context.default

orgs:
  acme:                            # organization workplace (--context acme)
    # id: …                        # optional Account org UUID; never sent as a header
    environments:
      sandbox:                     # instance-only: URL on the env node, no nested tenant
        url: https://example.jamfcloud.com
      commercial:                  # Account environment
        id: e77c1408-10c8-4007-b177-abc9157fbcaa
        region: us
        tenants:
          prod:
            tenantId: 22222222-…   # product tenant UUID
            url: https://prod.example.jamfcloud.com

auths:
  sandbox-admin:
    type: oauth
    clientId: your_client_id
    jamfpro: sandbox
    channels: [cli, ide]           # omit = all (cli/mcp/ide)
    # clientSecret: buddy cred set secret sandbox-admin --client-secret
  sandbox-platform-ro:
    type: platform
    scope: environment
    clientId: your_integration_client_id
    contexts: [commercial]
    access: readonly
    channels: [mcp, ide]           # restrict MCP/IDE to this readonly auth
  acme-org:
    type: platform
    scope: organization
    clientId: your_org_client_id
    contexts: [acme]

features:
  classic_instance_only: true      # optional: force Classic onto instance. Default false lets Classic use /proclassic. ACS and GET /api/v1/policies are always instance.
```

**Storage:** Nested `orgs:` is the on-disk workplace tree (organization → environment → product tenant). `--context` is only a workplace name; Jamf `scope:` stays on the auth. Instance-only URLs live on the environment node (`orgs.default.environments.sandbox.url` until you rename `default`). Old flat `contexts:` files still load and are folded. Auths hold type/username/clientId and link with `jamfpro:` or `contexts:`. Passwords and OAuth client secrets can stay inline for **dev/test**, or live in the encrypted vault (`secrets.enc`, keyed by **auth name**). The vault data key is one Keychain item (macOS) or `dek` (Linux/Windows). Recovery passphrase is **user-held**: it wraps the DEK inside `secrets.enc` and encrypts `cred backup` / `restore` (not a Keychain/file `master` item).

| Platform | Vault | Data key (`BUDDY_SECRET_STORE`) |
|----------|-------|----------------------------------|
| macOS (`make buddy`) | `~/.config/buddy/secrets.enc` | Keychain service `buddy`, account `dek`, via `security(1)` (default / `keychain`) |
| macOS (`make buddy-darwin-arm64`) | same vault file | Keychain service `virtualpete.buddy.cli` (label Buddy); `cred set passphrase` writes the recovery wrap |
| Linux | same | `~/.config/buddy/dek` (mode 0600), or `pass` (`buddy/dek`) |
| Windows | `%APPDATA%\buddy\secrets.enc` | `%APPDATA%\buddy\dek` |

`BUDDY_VAULT_PASSPHRASE` overrides the recovery passphrase (never written). Optional macOS **Buddy Agent** can own integrations instead of the vault — see [docs/auth.md](docs/auth.md).

```sh
buddy cred set context acme
buddy cred set context sandbox --url=https://example.jamfcloud.com --region=us --environment-id=… --org=acme
buddy cred set auth sandbox-admin --type=oauth --client-id=abc --jamfpro=sandbox --channels=cli,ide
buddy cred set auth sandbox-platform-ro --type=platform --client-id=… --contexts=commercial --scope=environment --access=readonly --channels=mcp,ide
buddy cred set channel mcp --access=readonly --context=sandbox
buddy cred set channel mcp --deny-builtins=computer_erase
buddy cred set channel mcp --allow-builtins=*
buddy cred set context sandbox --deny-builtins=* --allow-builtins=policies,policy
buddy cred set passphrase
buddy cred set secret sandbox-admin --client-secret
buddy cred backup ./buddy.creds
buddy cred restore ./buddy.creds
buddy cred use sandbox
buddy cred use --mcp sandbox
buddy cred store
buddy cred delete auth sandbox-admin   # also clears vault secrets
buddy cred clean secrets               # review/delete orphaned vault entries
buddy cred erase secrets               # wipe vault + DEK + recovery passphrase + token wrap/cache
buddy cred clean tokens                # delete expired/corrupt disk token cache files
```

Config is `context.default` + nested `orgs:` (organization → environment → product tenant) + `auths:` + optional `channels:`. Flat `contexts:` / `tenant:` still load and are folded into `orgs:` (invented org `default` for instance-only). Dual-read `--tenant` / `tenant=` for one release. Older `credentials:`, a bare string `tenant:`, `defaults:`, and `allow_auths:` are rejected. Inline secrets: `buddy cred migrate`. Selection: `--context` (auto-pick auth by operation; `--tenant` alias) or `--user`/`--auth` (pin). Prefer readonly auths for reads when linked. Limit which buddy clients may use an auth with `channels:` (omit = all). Per-client policy lives under top-level `channels:` (`access` / `context` / `contexts` / `deny_builtins` / `allow_builtins`; MCP defaults deny `jamf_*`). The same deny/allow lists may be set on contexts and auths (per-layer; channel allow cannot unblock a context deny). `tenant.cli|mcp|serve|ide|notebook` and the `serve` / `notebook` clients are removed. Rename with `buddy cred rename context|auth`. Recover a broken YAML file with `buddy cred restore-config`.

### First-time setup

If you have **no workplaces yet**, run any interactive Jamf command (or `buddy setup` / `buddy cred add`). On macOS buddy first looks for a running Buddy Agent and uses that document without writing `config.yaml`. A running Agent with no workplaces is an error (add them in Buddy Agent). If the Agent is not there, buddy asks how to start (with a note suggesting Buddy Agent): Jamf Pro instance (API-client JSON, or URL + basic), Jamf Account / Platform (paste curl), a secrets-free quickstart YAML/plist, or an org MDM profile. Linux and Windows go straight to that chooser.

```sh
buddy policy list                     # chooser when config is empty
buddy cred add --paste --context=sandbox
buddy cred add --context=sandbox --url=https://example.jamfcloud.com --type=basic --username=admin
buddy setup import ./quickstart.yaml
pbpaste | buddy cred add --paste --context=sandbox -y
buddy auth test
```

From a working install, share identity without secrets:

```sh
buddy setup export ./quickstart.yaml
buddy setup managed-plist /tmp/buddy.plist
```

If an MDM profile already provides workplaces, run `buddy setup secrets`. `buddy cred status` shows remaining holes. The docs UI **config** page (`buddy serve` → `/config`) edits metadata after CLI setup; it is not first-run. Secrets stay in the terminal vault.

Instance oauth is the JSON Jamf Pro shows when creating an API client. Instance basic is `--type=basic`. API roles and integrations are ordinary `role` / `client` commands — not a setup product path. Review privileges in Jamf before using a wide API role in production.

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
| `--context <name>` | Context from config (auto-picks auth); `--tenant` is a dual-read alias |
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

**Built-in functions:** plural nouns enumerate (`policies`, `packages`, …); singular nouns load one resource (`policy(id=…)` / `policy(trigger=…)`); singular_verb mutates (`policy_set`, `computer_reboot`). Optional `context=` / `tenant=` / `surface=` / `auth=` last (`tenant=` is a dual-read alias; `cred=` removed). Jamf-related builtins return **bare values** and **raise** on failure (MCP tools still wrap `{ok, value}` at the tool boundary). Canonical list: `buddy help builtins`; grammar overview: [docs/reference-manual.md](docs/reference-manual.md#built-in-functions).

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
│   ├── serve/           # buddy serve docs + /config + /ide
│   └── lsp/             # buddy lsp
├── macos/
│   ├── Buddy/           # SwiftUI Buddy.app shell (WKWebView + serve subprocess)
│   └── BuddyAgent/      # Optional menu-bar Agent (config + token minting)
├── examples/            # Runnable .buddy samples
├── docs/                # User guides; docs/generated is from make gendocs
├── Makefile
└── README.md
```
