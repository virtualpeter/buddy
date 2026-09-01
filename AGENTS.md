# Agent guide: buddy

**buddy** is a Go CLI for Jamf Pro automation: policies, groups, profiles, packages, Platform API, buddyscript, and MCP. Config lives at `~/.config/buddy/config.yaml`.

Human docs: [docs/index.md](docs/index.md). Auth/secrets/tokens: [docs/auth.md](docs/auth.md). MCP: [docs/mcp.md](docs/mcp.md). Endpoints: [docs/api-endpoints.md](docs/api-endpoints.md).

## Layout

| Path | Role |
|------|------|
| `cmd/buddy/` | CLI entry, dispatch, help (`help.go`) |
| `internal/clispec/` | CLI command/flag identifiers and English message catalog (`T`) |
| `internal/buddy/` | Jamf API clients, config, auth, secrets |
| `internal/buddy/tokenauth/` | Token acquisition, cache, 401 refresh |
| `internal/script/` | buddyscript (Starlark) builtins + session |
| `internal/mcp/` | `buddy mcp` stdio server |
| `internal/serve/` | `buddy serve` docs + `/ide` |
| `internal/lsp/` | `buddy lsp` |
| `stdlib/` | Reserved `load("stdlib/….buddy")` modules (currently empty) |
| `docs/` | User-facing docs (`docs/generated/` is from `make gendocs`) |
| `.cursor/rules/` | CLI, builtins, and auth conventions |

Build: `make buddy` → `./buddy` (`CGO_ENABLED=0`). Signed Darwin/arm64: `make buddy-darwin-arm64` → `./buddy-darwin-arm64`; `make install-signed` copies that to `~/go/bin/buddy`; `make package` wraps it in a pkg that installs `/usr/local/bin/buddy`. Tests: `go test ./...`. Docs UI: `make docs-serve`. Cross-compile: `make buddy-linux` / `make buddy-windows`. Lite tags: `make buddy-lite`.

Resource cookbooks: edit `internal/buddy/*_doc.go`, then `make gendocs` (writes `docs/generated/{human,agent}/*.md` + `internal/help/*_gen.go`). Do not hand-edit generated markdown. After `docs/` or generated changes: `make sync-docfs`. `AGENTS.md` and `.cursor/rules` are repo/editor only (not embedded for MCP). `BUDDY_DOCS_ROOT` is an optional live override.

## Conventions

- CLI help: `.cursor/rules/buddy-cli.mdc`. Command/flag **names** and user-facing CLI **messages** live in `internal/clispec` (English default; `BUDDY_LANG` / `LANG` fall back to `en`). Do not add a second locale or translate docs/MCP/LSP yet. Identifiers stay English forever.
- Builtins: `.cursor/rules/buddy-builtins.mdc` — plural/singular grammar, resource IDs first, `tenant?`/`surface?`/`auth?` last. Do not reintroduce removed aliases (`package_list`, `policy_find_ids`, …).
- Auth/secrets: `.cursor/rules/buddy-auth-secrets.mdc` and [docs/auth.md](docs/auth.md)
- Scripts use `.buddy`. `load("stdlib/….buddy", …)` is reserved (no modules yet; recipes live in `examples/`).
- Prefer named builtins over inventing Classic PUTs from `jamf_schema`.
- MCP: `get_instructions` first; `docs` → named builtins via `buddyscript` → `jamf_*` last (MCP defaults deny `jamf_*` on the channel layer; tenant/auth may add deny/allow). Envelope `{ok, value}` is MCP-only; builtins return bare values and raise.
- Tests that need a secret store: `secrets.SetStore(mock)`. Vault tests use `NewVaultStore` + `MemoryMaster` (DEK) or `secrets.SetDEKStore` / `SetMasterStore`; default store is `secrets.enc` (random DEK in Keychain/`dek`, recovery passphrase in Keychain/`master`).
- No org-specific tenant/auth names, hostnames, or client IDs in docs, help, tests, or commit messages — use generics (`sandbox`, `prod`, `example.jamfcloud.com`).
- Minimal diffs; match existing patterns.

## Platform schema vendoring (reviewed 2026-09-01)

`jamf_schema(api="platform")` embeds OAS JSON from jamf-cli (`internal/buddy/platformschema/`, `make sync-platform-schema`). Do **not** drop that snapshot or fetch Platform specs at runtime.

Reviewed via Jamf MCP (`list-specs` / `list-endpoints` / `get-server-variables`) and [developer.jamf.com/platform-api](https://developer.jamf.com/platform-api):

- Official docs + MCP cover the same six native services we vendor. Path lists match. Declaration Reporting docs hide two unfiltered routes that jamf-cli still marks `deprecated`.
- MCP also lists gateway-rewritten `platform-api=Jamf Pro API` and `Classic API` (`/api/pro`, proclassic). Those are instance APIs; buddy already fetches them live. Do not vendor them.
- No live gateway schema: `{gateway}/api/schema` and `/openapi.json` are 404; paths under `/api/{service}/` are 401 even when fake. Developer portal `openapi.json` is missing (404 / SPA HTML). MCP has no export-full-spec tool; search for `schema` / `openapi` endpoints is empty.
- Token docs still show `/auth/token` (getting started) and `/oauth2/token` (migration). Keep the 404 fallback in `tokenauth`.
- jamf-cli `specs/platform/` is still the only anonymous raw OAS (MIT, same six snake_case JSON files).

**TODO:** After Platform API Gateway is GA (docs still said Beta on 2026-09-01), re-check for a live schema URL or downloadable OAS. If one exists, consider dropping jamf-cli vendoring; until then keep the embed so `api="platform"` stays offline and credential-free.
