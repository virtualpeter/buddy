# Agent guide: buddy

**buddy** is a Go CLI for Jamf Pro automation: policies, groups, profiles, packages, Platform API, buddyscript, and MCP. It is independent MIT software that calls Jamf APIs; it is not affiliated with or endorsed by Jamf. Config lives at `~/.config/buddy/config.yaml`.

Human docs: [docs/index.md](docs/index.md) (About), [docs/getting-started.md](docs/getting-started.md). Auth/secrets/tokens: [docs/auth.md](docs/auth.md). MCP: [docs/mcp.md](docs/mcp.md). Endpoints: [docs/api-endpoints.md](docs/api-endpoints.md).

## Layout

| Path | Role |
|------|------|
| `cmd/buddy/` | CLI entry, dispatch, help (`help.go`) |
| `internal/clispec/` | CLI command/flag identifiers and English message catalog (`T`) |
| `internal/buddy/` | Jamf API clients, config, auth, secrets (incl. computer/mobile extension attributes) |
| `internal/buddy/tokenauth/` | Token acquisition, cache, 401 refresh |
| `internal/script/` | buddyscript (Starlark) builtins + session |
| `internal/mcp/` | `buddy mcp` stdio server |
| `internal/serve/` | `buddy serve` docs + `/config` + `/ide` |
| `internal/lsp/` | `buddy lsp` |
| `stdlib/` | Reserved `load("stdlib/….buddy")` modules (currently empty) |
| `docs/` | User-facing docs (`docs/generated/` is from `make gendocs`) |
| `packaging/` | Buddy.app Info.plist, pkg postinstall (symlink `/usr/local/bin/buddy`) |
| `.cursor/rules/` | CLI, builtins, and auth conventions |

Build artefacts go under `dist/` (`DIST=dist` by default). `make buddy` → `dist/buddy` (`CGO_ENABLED=0`). Signed Darwin/arm64 helper: `make buddy-darwin-arm64` → `dist/buddy-darwin-arm64`. App bundle: `make app` → `dist/Buddy.app` (CLI at `Contents/Helpers/buddy`, Finder stub at `Contents/MacOS/Buddy`). `make install-signed` copies the app to `~/Applications/Buddy.app` and symlinks `~/go/bin/buddy` at the helper. `make package` writes `dist/buddy-<version>.pkg` (installs `/Applications/Buddy.app` + `/usr/local/bin/buddy` symlink). `make release` cleans `dist/` then builds package + linux + windows. Tests: `go test ./...`. Docs UI: `make docs-serve`. Cross-compile: `make buddy-linux` / `make buddy-windows`. Lite tags: `make buddy-lite`.

Resource cookbooks: edit `internal/buddy/*_doc.go`, then `make gendocs` (writes `docs/generated/{human,agent}/*.md` + `internal/help/*_gen.go`). Do not hand-edit generated markdown. After `docs/` or generated changes: `make sync-docfs`. `AGENTS.md` and `.cursor/rules` are repo/editor only (not embedded for MCP). `BUDDY_DOCS_ROOT` is an optional live override.

## Conventions

- CLI help: `.cursor/rules/buddy-cli.mdc`. Command/flag **names** and user-facing CLI **messages** live in `internal/clispec` (English default; `BUDDY_LANG` / `LANG` fall back to `en`). Do not add a second locale or translate docs/MCP/LSP yet. Identifiers stay English forever.
- Builtins: `.cursor/rules/buddy-builtins.mdc` — plural/singular grammar, resource IDs first, `tenant?`/`context?`/`surface?`/`auth?` last (`context?` dual-read of `tenant?`). Do not reintroduce removed aliases (`package_list`, `policy_find_ids`, …).
- Auth/secrets: `.cursor/rules/buddy-auth-secrets.mdc` and [docs/auth.md](docs/auth.md)
- Scripts use `.buddy`. `load("stdlib/….buddy", …)` is reserved (no modules yet; recipes live in `examples/`).
- Prefer named builtins over inventing Classic PUTs from `jamf_schema`.
- MCP: `get_instructions` first; `docs` → named builtins via `buddyscript` → `jamf_*` last (MCP defaults deny `jamf_*` on the channel layer; tenant/auth may add deny/allow). Envelope `{ok, value}` is MCP-only; builtins return bare values and raise.
- Tests that need a secret store: `secrets.SetStore(mock)`. Vault tests use `NewVaultStore` + `MemoryMaster` (DEK) or `secrets.SetDEKStore` / `SetMasterStore`; default store is `secrets.enc` (random DEK in Keychain/`dek`, recovery passphrase in Keychain/`master`). Signed Darwin: presence-gated DEK + process session unlock — see [docs/auth.md](docs/auth.md).
- No org-specific tenant/auth names, hostnames, or client IDs in docs, help, tests, or commit messages — use generics (`sandbox`, `prod`, `example.jamfcloud.com`).
- Minimal diffs; match existing patterns.

## Platform schema vendoring (reviewed 2026-09-12)

`jamf_schema(api="platform")` embeds OAS JSON from jamfplatform-go-sdk (`internal/buddy/platformschema/`, `make sync-platform-schema`). Do **not** drop that snapshot or fetch Platform specs at runtime.

Reviewed via Jamf MCP (`list-specs` / `list-endpoints` / `get-server-variables`), [developer.jamf.com/platform-api](https://developer.jamf.com/platform-api), and authenticated sandbox GETs against `https://us.api.jamfcloud.com`:

- Official docs + MCP cover the same six native services we vendor. GA servers are `https://{region}.api.jamfcloud.com/{service}` (no `/api` prefix). Scope is `X-Environment-Id` / `X-Tenant-Id`, not `/tenant/{id}` path segments.
- MCP also lists gateway-rewritten `platform-api=Jamf Pro API` and `Classic API` (`/pro`, `/proclassic`). Those are instance APIs; buddy already fetches them live. Do not vendor them.
- No live gateway schema: `{gateway}/schema` and `/openapi.json` are **404** (authenticated and not). Per-service guesses (`/blueprints/openapi.json`, `/devices/schema`, `/pro/schema`, …) are **403** `BAD_PERMISSIONS` even when the real list endpoint on that service is **200** — unknown paths, not a docs privilege. Developer portal `openapi.json` is 404. MCP has no export-full-spec tool. Reference `.md` pages embed per-operation OAS only.
- Token docs show `/auth/token` (getting started) and `/oauth2/token` (migration). Keep the 404 fallback in `tokenauth`.
- Full JSON remains MIT snapshots in [jamfplatform-go-sdk `api/`](https://github.com/Jamf-Concepts/jamfplatform-go-sdk/tree/main/api) (jamf-cli `specs/platform/` is a copy). Same six snake_case files.

Keep the embed so `api="platform"` stays offline and credential-free. Account licensing (`GET /licensing/v1/licenses`) is a live org-gateway call, not in that snapshot. Re-check for a live schema URL if Jamf publishes `{gateway}/schema` or a portal `openapi.json`.
