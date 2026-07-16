# buddy

CLI tool for managing Jamf Pro policies and smart groups. Automates common tasks like version bumps, policy cloning, scope changes, and package updates via the Jamf Pro API.

Includes a Starlark scripting engine for composing multi-step workflows.

## Documentation

| Resource | Description |
|----------|-------------|
| [AGENTS.md](AGENTS.md) | Agent/contributor context: auth, secrets, token caching, meliorate integration |
| [docs/index.md](docs/index.md) | Short index of the docs in this folder |
| [docs/reference-manual.md](docs/reference-manual.md) | Full reference: config, CLI, scripting, builtins overview, API doc link |
| [docs/api-endpoints.md](docs/api-endpoints.md) | Jamf HTTP endpoints used by buddy |
| [examples/](examples/) | Runnable `.star` examples and index |

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
cred: default

credentials:
  default:
    url: https://your-instance.jamfcloud.com
    auth:
      type: oauth
      clientId: your_client_id
      # clientSecret stored via: buddy cred secret set default --client-secret
```

**Credential storage:** Config holds URL, auth type, username, and client ID. Passwords and OAuth client secrets can stay inline for **dev/test**, or be stored outside the file:

| Platform | Default store | Override |
|----------|---------------|----------|
| macOS | Keychain (service `buddy`) | `BUDDY_SECRET_STORE=keychain` |
| Linux | `~/.config/buddy/secrets/<name>.json` (mode 0600) | `BUDDY_SECRET_STORE=file` or `pass` |

```sh
buddy cred add sandbox https://example.jamfcloud.com --type=oauth --client-id=abc
buddy cred secret set sandbox --client-secret
buddy cred secret path
```

Inline `password` / `clientSecret` in YAML still works. `buddy setup configure-oauth` stores the client secret in the platform store automatically. Set `BUDDY_SECRET_STORE=pass` on Linux to use [pass](https://www.passwordstore.org/) (`buddy/<cred>/password`, `buddy/<cred>/client-secret`).

| Field         | Description                              | Default     |
|---------------|------------------------------------------|-------------|
| `cred`        | Active credential name                   | `default`   |
| `credentials` | Map of named credentials (url + auth)    | —           |

Override the active credential per-invocation with `--user <name>` (`--cred` is a deprecated alias).

Each credential specifies its own URL and authentication method (`command`, `basic`, or `oauth`). Use `buddy cred help` to manage credentials, and `buddy auth test` to verify a credential works. For `basic` (username/password), buddy exchanges credentials for a Bearer token via Jamf's `/api/v1/auth/token` so API calls succeed.

### Setting up a role and client for buddy

To create a Jamf API role with the permissions buddy needs and an API client (integration) scoped to that role:

```sh
buddy setup write-script              # writes setup_buddy_role.star (or specify a path)
buddy setup_buddy_role.star -y     # create the role and client (idempotent: reuses existing "Buddy" role)
buddy setup configure-oauth -y        # stores clientId in config and client secret in platform store
```

Alternatively, after step 2 you can run `buddy client credentials <id>` to get the client secret (shown once) and add it to config manually.

The generated script fetches privilege names from your Jamf instance at runtime and reuses an existing role named "Buddy" if present.

The role the script targets may include **destructive or high-impact privileges** (for example wiping a computer, sending erase MDM commands, or similar Jamf permissions buddy can use in automation). That keeps a single setup path flexible, but it is **not** always appropriate to grant every such privilege to the API client: for day-to-day or production use, review the role in Jamf and **remove privileges you do not actually need**, then narrow the role before distributing credentials.

## Usage

```
buddy [flags]                              # interactive Starlark shell
buddy [flags] <script.star> [-- args...]   # run a script
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

buddy embeds a [Starlark](https://github.com/google/starlark-go) interpreter with builtins for policies, groups, profiles, packages, scripts, computers, MDM, roles, clients, and more.

```sh
buddy                              # interactive Starlark shell
buddy <script.star> [-- args...]
```

**Built-in functions:** resource IDs and options first; optional `cred=` last. Every Jamf-related builtin returns `{"ok": true, "value": ...}` or `{"ok": false, "error": "..."}` unless documented otherwise.

```sh
buddy help starlark   # language subset
buddy help builtins   # full builtin reference (canonical)
```

**Globals:** `cred` (active credential name), `args` (arguments after `--`), `re` (regex). Use **`error(...)`** for status and diagnostic lines (stderr); use **`print(...)`** only when you intentionally write script output to stdout (e.g. for piping).

Sample scripts: [examples/](examples/).

#### Example (illustrative)

```python
# Edit TRIGGER_PREFIX / APP_NAME for your policies; see examples/bump-release.star for a fuller sample.
# Usage: buddy examples/bump-release.star -- 2.4.27

TRIGGER_PREFIX = "install-exampleapp-"
APP_NAME = "exampleapp"

def main():
    version = args[0]

    r = policy_get_id(trigger=TRIGGER_PREFIX + version, force_refresh=True)
    if not r["ok"]:
        error("Policy not found:", r["error"])
        exit(1)

    policy_id = r["value"]
    policy_set_package(app=APP_NAME, version=version, policy_id=policy_id)
    # If install params or self-service live on other policies, add policy_set_install / policy_set_ss
    # with those policy IDs. To bump a smart-group version criterion: group_set_version(group_id="...", version=version).

main()
```

A runnable file with the same ideas (and optional steps you can uncomment) lives at [examples/bump-release.star](examples/bump-release.star).

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
│   └── script/         # Starlark runner and builtins
│       ├── builtins.go
│       ├── script.go
│       └── workflow.go
├── scripts/
│   └── integration_test.star   # Broad Jamf integration harness (non-production)
├── examples/            # Runnable .star samples (see examples/README.md)
├── docs/                # Reference manual, API catalog
├── go.mod
├── go.sum
├── Makefile
└── README.md
```
