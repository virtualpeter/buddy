# buddy

CLI tool for managing Jamf Pro policies and smart groups. Automates common tasks like version bumps, policy cloning, scope changes, and package updates via the Jamf Pro API.

Includes a Starlark scripting engine for composing multi-step workflows.

## Documentation

| Resource | Description |
|----------|-------------|
| [AGENTS.md](AGENTS.md) | Agent/contributor context: auth, secrets, token caching, meliorate integration |
| [docs/index.md](docs/index.md) | Short index of the docs in this folder |
| [docs/reference-manual.md](docs/reference-manual.md) | Full reference: config, CLI, scripting, builtins overview, API doc link |
| [docs/builtins-v3.md](docs/builtins-v3.md) | Starlark v3 plural/singular rename map (breaking) |
| [docs/api-endpoints.md](docs/api-endpoints.md) | Jamf HTTP endpoints used by buddy |
| [examples/](examples/) | Runnable `.buddy` examples and index |

Complete builtin signatures always match your installed binary: `buddy help builtins`.

## Breaking change: Starlark builtins v3

Jamf-related Starlark builtins use a **plural / singular** grammar (`policies` / `policy` / `policy_set`). Old names remain as temporary deprecated aliases (one stderr warning per process). Full map: [docs/builtins-v3.md](docs/builtins-v3.md).

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
cred: default

credentials:
  default:
    url: https://your-instance.jamfcloud.com
    auth:
      type: oauth
      clientId: your_client_id
      # clientSecret stored via: buddy cred set secret default --client-secret

  # Optional: Jamf Platform API Gateway (Pro JSON + Blueprints; Classic not yet usable —
  # Jamf Account integrations do not expose Classic/proclassic permissions yet)
  gateway:
    region: us
    tenantId: e77c1408-10c8-4007-b177-abc9157fbcaa
    auth:
      type: oauth
      clientId: your_integration_client_id
```

**Credential storage:** Config holds URL and/or `region` (Platform API Gateway), optional `tenantId`, auth type, username, and client ID. Passwords and OAuth client secrets can stay inline for **dev/test**, or be stored outside the file:

| Platform | Default store | Override |
|----------|---------------|----------|
| macOS | Keychain (service `buddy`) | `BUDDY_SECRET_STORE=keychain` |
| Linux | `~/.config/buddy/secrets/<name>.json` (mode 0600) | `BUDDY_SECRET_STORE=file` or `pass` |

```sh
buddy cred set sandbox --url=https://example.jamfcloud.com --type=oauth --client-id=abc
buddy cred set gateway --region=us --tenant-id=e77c1408-10c8-4007-b177-abc9157fbcaa --client-id=abc
buddy cred set secret sandbox --client-secret
buddy cred store
```

Inline `password` / `clientSecret` in YAML still works; move them into the secure store with `buddy cred migrate`. `buddy setup configure-oauth` stores the client secret in the platform store automatically. Set `BUDDY_SECRET_STORE=pass` on Linux to use [pass](https://www.passwordstore.org/) (`buddy/<cred>/password`, `buddy/<cred>/client-secret`).

| Field         | Description                              | Default     |
|---------------|------------------------------------------|-------------|
| `cred`        | Active credential name                   | `default`   |
| `credentials` | Map of named credentials (`url` and/or `region`, auth; optional `tenantId` for Platform API Gateway) | — |

Override the active credential per-invocation with `--user <name>` (`--cred` is a deprecated alias).

Each credential specifies its own URL (or Platform `region`) and authentication method (`command`, `basic`, or `oauth`). Use `buddy cred help` to manage credentials, and `buddy auth test` to verify a credential works. For `basic` (username/password), buddy exchanges credentials for a Bearer token via Jamf's `/api/v1/auth/token` so API calls succeed. Credentials with `region`/`tenantId` (or a gateway URL) target the Platform API Gateway (OAuth only; token via `/auth/token`). Pro JSON and Blueprints work through the gateway; Classic (`/JSSResource`) is rewritten but not practically usable yet because Platform integrations lack Classic permissions.

### Setting up a role and client for buddy

To create a Jamf API role with the permissions buddy needs and an API client (integration) scoped to that role:

```sh
buddy setup write-script              # writes setup_buddy_role.buddy (or specify a path)
buddy setup_buddy_role.buddy -y    # create the role and client (idempotent: reuses existing "Buddy" role)
buddy setup configure-oauth -y        # stores clientId in config and client secret in platform store
```

Alternatively, after step 2 you can run `buddy client credentials <id>` to get the client secret (shown once) and add it to config manually.

The generated script fetches privilege names from your Jamf instance at runtime and reuses an existing role named "Buddy" if present.

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
| `--user <name>` | Credential/user name from config (`--cred` deprecated) |
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

Prefer the **`.buddy`** extension for scripts and stdlib modules (`load("stdlib/<name>.buddy", …)`) so the Cursor/VS Code extension language id applies. Existing `.star` paths still run. Editor support: `buddy lsp` plus [editors/vscode](editors/vscode) (`make extension-package`).

**Built-in functions (v3):** plural nouns enumerate (`policies`, `packages`, …); singular nouns load one resource (`policy(id=…)` / `policy(trigger=…)`); singular_verb mutates (`policy_set`, `computer_reboot`). Optional `cred=` last. Every Jamf-related builtin returns `{"ok": true, "value": ...}` or `{"ok": false, "error": "..."}` unless documented otherwise. See [docs/builtins-v3.md](docs/builtins-v3.md) for the old→new rename map (deprecated aliases warn once on stderr).

```sh
buddy help starlark   # language subset
buddy help builtins   # full builtin reference (canonical)
buddy help lsp        # language server for the editor extension
make docs-serve       # local docs + Starlark notebook (buddy serve --open)
buddy mcp             # MCP stdio server (docs/AGENTS/rules embedded; see docs/mcp.md)
make sync-docfs       # refresh embedded knowledge after editing docs/rules/AGENTS
```

**Globals:** `cred` (active credential name), `args` (arguments after `--`), `re` (regex). Use **`error(...)`** for status and diagnostic lines (stderr); use **`print(...)`** only when you intentionally write script output to stdout (e.g. for piping). If the script defines `main`, buddy calls it after loading—do **not** add a trailing `main()` (that would run it twice).

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
