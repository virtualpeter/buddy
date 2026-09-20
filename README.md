# buddy

<img src="internal/serve/buddy.png" alt="buddy" width="128" height="128">

**buddy** is a CLI, MCP server, and scripting language for [Jamf Pro](https://www.jamf.com/) and Jamf Platform APIs (policies, groups, profiles, packages, computers, MDM, plus Platform blueprints, devices, DDM, and licenses).

Independent MIT software. Not affiliated with, endorsed by, or sponsored by Jamf. Jamf, Jamf Pro, and related names are trademarks of Jamf.

Work from an interactive **buddyscript** shell, `.buddy` files, **Buddy.app** (docs + script IDE), or `buddy mcp` for AI hosts.

## Install

**macOS:** install the notarized package from [GitHub Releases](https://github.com/virtualpeter/buddy/releases). That puts **Buddy.app** in `/Applications` and `buddy` on `PATH` (`/usr/local/bin/buddy`). The nested **Buddy Agent** is the supported way to store workplaces and secrets.

**Linux / Windows:** use the binaries on the same Releases page, or [build from source](#contributing).

Then [get started](docs/getting-started.md): open Buddy.app, add a workplace in **Agent Settings**, run `buddy policy list`.

## Documentation

| Resource | Description |
|----------|-------------|
| [docs/getting-started.md](docs/getting-started.md) | Install, Buddy Agent, first commands |
| [docs/index.md](docs/index.md) | Docs map |
| [docs/auth.md](docs/auth.md) | Contexts, vault, Agent, tokens |
| [docs/releases.md](docs/releases.md) | Notable changes per tag |
| [docs/reference-manual.md](docs/reference-manual.md) | Config, CLI, buddyscript, env vars |
| [docs/mcp.md](docs/mcp.md) | MCP server for AI hosts |
| [docs/mcp-tools.md](docs/mcp-tools.md) | MCP tool scorecard |
| [docs/generated/human/profile-payloads.md](docs/generated/human/profile-payloads.md) | Profile payload dissect/assemble |
| [docs/editor.md](docs/editor.md) | `buddy lsp`; VSIX is the sibling [buddy-extension](../buddy-extension) checkout |
| [docs/api-endpoints.md](docs/api-endpoints.md) | Jamf HTTP endpoints buddy uses |
| [docs/change-batches.md](docs/change-batches.md) | Plan → approve → apply |
| [examples/](examples/) | Runnable `.buddy` samples |
| [AGENTS.md](AGENTS.md) | Contributor notes for AI agents |

## Usage

```
buddy [flags]                              # interactive buddyscript shell
buddy [flags] <script.buddy> [-- args...]  # run a script
buddy [flags] <command> <subcommand> [args...]
```

Global flags: `-y` skip confirms, `-v` verbose, `--context <name>` (workplace; `--tenant` alias), `--user <name>` pin an auth (`--auth` alias), `--script-trace`.

```sh
buddy policy list
buddy serve --open          # docs + IDE in the browser (Buddy.app is the same surfaces)
buddy help builtins
buddy mcp                   # stdio MCP for AI hosts
```

Builtins: plural nouns enumerate, singular nouns load one resource, `singular_verb` mutates. Session kwargs (`context=` / `tenant=` / `surface=` / `auth=`) last. They return bare values and raise on failure. Grammar: [reference manual](docs/reference-manual.md#built-in-functions). Sample: [examples/bump-release.buddy](examples/bump-release.buddy).

## Contributing

Need **Go 1.27+**. Optional: `pass` on Linux when `BUDDY_SECRET_STORE=pass`.

```sh
make buddy            # dist/buddy (CGO off; macOS vault DEK via security(1))
make test
make help
```

macOS signed artefacts (Developer ID):

```sh
make buddy-darwin-arm64   # dist/buddy-darwin-arm64 (Security.framework)
make app                  # dist/Buddy.app (CLI helper + SwiftUI shell + nested Agent)
make install-signed       # ~/Applications/Buddy.app + symlink ~/go/bin/buddy
make package              # dist/buddy-<version>.pkg
make notarize             # staple app then pkg (needs BUDDY_NOTARY_PROFILE + installer identity)
make release              # abort if dirty; pkg + linux + windows; strip .app from dist
make publish TAG=v1.2.3   # gh release from dist/ (TAG must match HEAD)
```

| Target | Output | Notes |
|--------|--------|-------|
| `make buddy` | `dist/buddy` | Default; Keychain service `buddy` via `security(1)` |
| `make buddy-darwin-arm64` | `dist/buddy-darwin-arm64` | Hardened Runtime; service `virtualpete.buddy.cli`; identifier must stay stable |
| `make app` | `dist/Buddy.app` | Helper at `Contents/Helpers/buddy`; shell hosts `buddy serve --embed` |
| `make install-signed` | `~/Applications/Buddy.app` | Symlink (not a copy) at `~/go/bin/buddy` |
| `make package` | `dist/buddy-<version>.pkg` | `/Applications/Buddy.app` + `/usr/local/bin/buddy`; optional `BUDDY_PKG_SIGN_IDENTITY` |
| `make release` | pkg + linux + windows | Aborts if dirty; notarizes when `BUDDY_NOTARY_PROFILE` is set; removes `.app` from `dist/` |
| `make publish` | GitHub release from `dist/` | `TAG=vX.Y.Z` required and must match HEAD; needs `gh`; optional `REPO=` / `DRAFT=1` |

`make buddy-darwin-arm64` fails unless the host is Darwin/arm64 and the identity is a real Developer ID (not ad-hoc). Each binary only sees its own vault DEK until you unwrap with `cred set passphrase` or restore a backup. See [docs/auth.md](docs/auth.md).

`make release` refuses a dirty working tree, then leaves shippable files in `dist/` (pkg + linux + windows; `Buddy.app` / `BuddyAgent.app` are stripped — they are already inside the pkg). Upload with `make publish TAG=vX.Y.Z` (`gh`; notes are every `docs/releases.md` section from that tag back to the major `.0.0`; `TAG` must be the exact tag on HEAD):

```sh
make release
make publish TAG=v1.2.3
```

```sh
make test-signed   # Darwin: Security.framework tests (no codesign)
make docs-serve    # local docs UI
make gendocs       # regenerate dual docs/help from Go docspecs
make sync-docfs    # refresh embedded docs after editing docs/
```

```
buddy/
├── cmd/buddy/           # CLI entry, dispatch, help
├── cmd/gendocs/         # Regenerates docs/generated and internal/help/*_gen.go
├── internal/
│   ├── buddy/           # Jamf API clients, config, auth, secrets
│   ├── clispec/         # Command/flag identifiers and English catalog
│   ├── script/          # buddyscript runner and builtins
│   ├── mcp/             # buddy mcp stdio server
│   ├── serve/           # buddy serve docs + /config + /ide
│   └── lsp/             # buddy lsp
├── macos/
│   ├── Buddy/           # SwiftUI Buddy.app shell
│   └── BuddyAgent/      # Menu-bar Agent (config + token minting)
├── examples/
├── docs/                # User guides; docs/generated is from make gendocs
└── Makefile
```

[jamf-cli](https://github.com/Jamf-Concepts/jamf-cli) is Jamf’s official CLI. buddy is not a replacement. This repo vendors a snapshot of jamf-cli’s Platform OpenAPI specs (`internal/buddy/platformschema/specs/`, MIT) so `jamf_schema(api="platform")` works offline. Refresh with `make sync-platform-schema`. See [API schema documents](docs/api-endpoints.md#api-schema-documents).

## License

MIT License — see [LICENSE](LICENSE). Copyright © 2026 Peter Viertel.

Jamf, Jamf Pro, and related names are trademarks of Jamf. buddy is not affiliated with, endorsed by, or sponsored by Jamf.

Environment variables: [reference manual](docs/reference-manual.md#environment-variables).
