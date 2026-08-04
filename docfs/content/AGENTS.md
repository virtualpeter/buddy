# Agent guide: buddy

This file captures project context for AI agents (and humans) picking up work on **buddy** without prior chat history.

## What buddy is

**buddy** is a Go CLI for Jamf Pro automation: policies, groups, profiles, packages, Platform API (blueprints, devices, device-groups, DDM report, compliance benchmarks), Starlark workflows, and API auth/token helpers. Config lives at `~/.config/buddy/config.yaml`.

Sibling project **meliorate** (separate repo under `../meliorate`) tracks Jamf config in Git. It can call buddy as an external token helper via `jamfpro.token_command`.

## History (important decisions)

1. **jamftoken removed from meliorate** — trademark: do not reintroduce `jamftoken` as a tool name.
2. **protok / protoken deprecated** — token logic was briefly extracted to a standalone `protoken` project (binary name `protok`). That project is **removed**; all token functionality lives in buddy now.
3. **Platform secret storage** — passwords and OAuth client secrets should live outside `config.yaml` (Keychain on macOS, file or `pass` on Linux). Inline secrets remain supported for dev/test only.

When editing meliorate, point `token_command` at buddy:

```yaml
token_command:
  - buddy
  - auth
  - token
  - --raw
  - --user=default
```

## Repository layout

| Path | Role |
|------|------|
| `cmd/buddy/` | CLI entry (`main.go`), dispatch, help text (`help.go`) |
| `internal/buddy/` | Jamf API clients, config, auth, secret CLI |
| `internal/buddy/tokenauth/` | Token acquisition, caching, reactive refresh on expiry/401 |
| `internal/buddy/secrets/` | Platform secret stores (Keychain, file, pass) |
| `internal/script/` | Starlark engine, builtins, `Session` (REPL/notebook/MCP eval) |
| `internal/lsp/` | `buddy lsp` language server (editor integration) |
| `stdlib/` | Embedded Starlark libraries (`load("stdlib/….buddy", …)`); composed on builtins |
| `editors/vscode/` | Cursor/VS Code extension (language id `buddy`, `.buddy` files) |
| `internal/serve/` | `buddy serve` local docs + notebook + `/ide` script runner |
| `internal/mcp/` | `buddy mcp` stdio MCP server |
| `internal/help/` | Shared CLI help strings (builtins/shell/starlark/root) |
| `docs/` | User-facing reference manual |
| `.cursor/rules/` | Persistent Cursor rules (CLI + Starlark conventions) |

Build: `make buddy` → `./buddy`. Tests: `go test ./...`. Docs UI: `make docs-serve`. Extension VSIX: `make extension-package` (`PUBLISHER` defaults to `virtualpete`).

## MCP (AI tools)

See **[docs/mcp.md](docs/mcp.md)** for MCP host setup and **[docs/mcp-tools.md](docs/mcp-tools.md)** for the lean tool scorecard / agent priority (`docs` → `buddyscript` named builtins → `jamf_*` last). Summary:

- `buddy mcp` speaks stdio MCP for AI hosts (Cursor, Claude Desktop, etc.).
- Lean MCP tools: `buddy_info`, `docs`, `jamf_schema`, `buddyscript`. Resource work uses named builtins via `buddyscript` (including package READY/download).
- `buddy_info` reports binary version plus default tenant/auth, channel policy (`channel_access`, allowlists), and available tenants/auths for the mcp channel (no Jamf call).
- `buddy mcp setup` lists well-known config paths and can `--write` a `mcpServers.buddy` entry (`--create` for missing files). Cursor: `~/.cursor/mcp.json` / `.cursor/mcp.json`; Claude Desktop: `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS).
- Prefer an absolute `command` path and `args: ["mcp"]`.
- Docs/AGENTS/examples/editor rules are embedded in the binary (`docfs/`; refresh with `make sync-docfs`). `BUDDY_DOCS_ROOT` is an optional live override. Agents should use the MCP `docs` tool (list/get); the same pages are also `buddy://docs/...` resources.
- Prefer named builtins (smart criteria vs static membership, profile payload edits) via `buddyscript`; do not invent Classic PUTs from `jamf_schema` alone.
- Tool results are always `{ok, value, ...}` objects (lists under `value`) so hosts accept `structuredContent`. Starlark builtins return bare values and raise on failure; the envelope is MCP-only.
- Reload or restart the AI host after changing MCP config or rebuilding buddy.


## Authentication model

Config is **tenants** + **auths** + optional **channels** policies (breaking change from flat `credentials:` — migrated automatically on load).

- **Tenant** — Jamf environment: instance `url` and/or Platform `region` + `tenantId`.
- **Auth** — secret-bearing principal (`command` / `basic` / `oauth`) linked to one or more tenants via `tenants: […]`. A platform clientId may list multiple tenants. Optional `access` (`full`|`readonly`) and `surface` (`any`|`platform`|`instance`).
- **Tenant selection** — `tenant.default` plus optional `channels.<cli|mcp|ide|notebook>.tenant`. Global `--tenant` still wins; `--user` / `--auth` pins an auth. Starlark/MCP use tenant-first session kwargs (`tenant=` / `surface=` / `auth=`); `cred=` was removed.
- **Channel policy** — optional `channels.<client>.access` (`readonly`|`full`), `tenant`, and `tenants` allowlist. Clamps mutates and filters selectable tenants (most restrictive with auth access wins).
- **Auth channels** — optional `channels: [cli, mcp, ide, notebook]` on each auth (omit = all). Filters auto-pick and rejects pins / UI choices. Notebook and IDE are separate clients (no shared `serve` channel).

| `auth.type` | Config fields | Secret storage |
|-------------|---------------|----------------|
| `command` | `runCommand` | N/A — command prints token |
| `basic` | `username`; password inline or in store | `buddy cred set secret <auth>` |
| `oauth` | `clientId`; secret inline or in store | `buddy cred set secret <auth> --client-secret` |

`ResolveSession` (`internal/buddy/session.go`) picks auth by operation class (prefer readonly for reads; require full for mutates; platform for blueprints; instance for classic/schema). Path rewrite uses the tenant’s `tenantId`; base URL comes from the composed surface.

**Classic parity:** path rewrite for `/JSSResource/…` is implemented, but Platform integrations do not yet offer Classic permissions in Jamf Account — expect `403` on Classic until Jamf exposes them. Prefer an instance-surface auth for Classic-heavy work.

Resolution order for secrets: **inline config → platform store → error** (`ResolveAuthWithSecrets`; store keys are **auth names**).

### Secret backends

Selected by platform or `BUDDY_SECRET_STORE`:

| Value | Platform | Location |
|-------|----------|----------|
| (default) | macOS | Keychain, service `buddy` |
| (default) | Linux | `~/.config/buddy/secrets/<cred>.json` (0600) |
| `file` | any | Same as Linux default |
| `pass` | any | `buddy/<cred>/password`, `buddy/<cred>/client-secret` |
| `keychain` | macOS only | Explicit Keychain |

CLI: `buddy cred set tenant|auth|channel|secret …` (auth supports `--channels=`), `buddy cred delete tenant|auth|secret …` (auth delete clears store secrets; tenant delete cascades sole-linked auths), `buddy cred clean secrets` (review/delete orphaned store entries), `buddy cred clean tokens` (delete expired/corrupt disk token cache files), `buddy cred use <tenant>`, `buddy cred use --cli|--mcp|--ide|--notebook <tenant>`, `buddy cred store`, `buddy cred migrate`. `buddy setup basic-setup` bootstraps the first instance tenant + basic auth; `buddy setup store-oauth` rotates a Jamf Pro *instance* API client's secret into a buddy oauth auth with `surface=instance` (Platform gateway oauth cannot manage API roles/integrations).

## Token caching

Three layers:

1. **Process-local cache** — `credAuthCache` in `internal/buddy/common.go` holds one `tokenauth.Authenticator` per `auth@tenant` for the lifetime of the buddy process.
2. **In-authenticator cache** — each `Authenticator` stores `token` + `tokenExpiry`.
3. **On-disk cache** — `~/.config/buddy/tokens/<cache-key>.json` (mode `0600`), keyed by auth@tenant (platform adds `@platform:{tenantId}`). Bearer is AES-GCM–obfuscated with a compiled-in key (discourages casual `cat`; not a real secret store). Reused across CLI invocations while expiry is more than ~15s away (clock skew). Skipped for `command` auth. Expired, URL-mismatched, undecodable, or corrupt files are deleted on load; `buddy cred clean tokens` sweeps the directory. Invalidated on failed refresh / full re-auth.

`GetToken()` flow (`internal/buddy/tokenauth/auth.go`):

- **command auth**: runs external command each time (no long-lived or disk cache).
- **basic auth**: username/password → `/api/v1/auth/token`; expiry treated as ~18 minutes (refresh before Jamf’s 20-minute limit).
- **oauth** (instance): client credentials → `/api/oauth/token`; expiry from `expires_in`.
- **oauth** (Platform API Gateway, credential `tenantId` set): client credentials → `{gateway}/auth/token` (fallback `/oauth2/token`); ~900s expiry; **no keep-alive**.
- **Disk hydrate**: if memory is empty and `CacheKey` is set, load a still-usable disk token before hitting Jamf.
- **Reuse until expiry**: tokens are used until expiry minus a small skew (~15s). No proactive keep-alive or OAuth re-auth while the token is still usable.
- **Reactive refresh**: on HTTP 401, `PerformRequest` refreshes once (OAuth/command: re-auth; basic: keep-alive or re-auth unless keep-alive probe still succeeds) and retries the request once. Explicit `buddy auth refresh` also uses keep-alive for basic tokens.

CLI surfaces:

```sh
buddy auth test                              # verify active tenant/auth session
buddy auth token [--raw] [--tenant=T] [--user=A]  # Authorization header or raw bearer
buddy auth refresh --url=URL [TOKEN]         # keep-alive refresh (instance; direct flags supported)
buddy cred clean tokens                      # delete expired/corrupt disk token cache files
```

Direct auth flags (`--url`, `--tenant-id`, `--username`, `--password`, `--client-id`, `--client-secret`) bypass config for one-offs.

When the resolved session uses a platform surface, paths are rewritten via `RewriteJamfPath` / `ResolveEndpoint` (Pro → `/api/pro/vN/tenant/{id}/…`, Classic → `/api/proclassic/tenant/{id}/JSSResource/…`, Platform product APIs → inject `/tenant/{id}` after version for blueprints/devices/device-groups/device-actions/ddm/report/compliance-benchmarks). Platform product commands require a platform-capable tenant+auth (`buddy blueprint|device|device-group|ddm|benchmark`, matching Starlark/MCP tools).

## Jamf Pro API versions (groups / inventory)

| Resource | Buddy uses | Notes |
|----------|------------|--------|
| Computer groups (smart/static) | `/api/v3/computer-groups/...` | Migrated from v2 (v2 deprecated in cloud schema ~2026-05-28). |
| Computer inventory | `/api/v3/computers-inventory` | **Stay on v3.** Cloud schema exposes v4 and deprecates v3 (~2026-07-14), but production Jamf does not support v4 yet. Do not add a v4 path (or v4→v3 fallback) until prod does; then remap filters/sort (`lastContactTime` → `lastCheckIn` / `lastContact`). |

See [`docs/api-endpoints.md`](docs/api-endpoints.md) for the full endpoint map.

## API schemas (jamf_schema)

`jamf_schema` (Starlark builtin + MCP tool) discovers endpoints. Pro (`/api/schema`) and Classic (`/classicapi/doc/swagger.yaml`) are fetched **live** from the credential's host. The **Platform API Gateway has no live schema endpoint**, so its per-service OpenAPI specs are vendored (MIT, from `Jamf-Concepts/jamf-cli`) into `internal/buddy/platformschema/specs/` and embedded via `go:embed`. Refresh with `make sync-platform-schema` and commit. `api="all"` (default) covers pro+classic+platform; `api="platform"` needs no credential.

None of the schema documents require authentication: the Pro/Classic schema endpoints are public on the Jamf host (only data endpoints need a bearer token) and Platform specs are embedded. buddy fetches Pro/Classic schemas with no `Authorization` header (`JamfRequestOpts.Unauthenticated` → `PerformRequestNoAuth`), using the credential only to resolve the base URL — so schema discovery works even when the credential has no stored secret.

## Integration as meliorate token helper

meliorate runs `token_command` as a subprocess and expects **only the raw bearer token on stdout** (no log lines). Use:

```sh
buddy auth token --raw --user=default
```

buddy resolves secrets from the platform store at runtime, so meliorate config does not need plaintext secrets when buddy holds them.

## Conventions (follow existing rules)

- **CLI help**: see `.cursor/rules/buddy-cli.mdc` — usage lines, subcommand naming, placeholders.
- **Starlark builtins**: see `.cursor/rules/buddy-builtins.mdc` and [docs/reference-manual.md](docs/reference-manual.md#built-in-functions) — plural/singular grammar (`policies` / `policy` / `policy_set`), resource IDs first on mutate helpers, `tenant?` / `surface?` / `auth?` last. Do not reintroduce removed legacy aliases (`package_list`, `policy_find_ids`, …).
- **Starlark stdlib**: multi-step helpers in `stdlib/*.buddy` via `load("stdlib/release.buddy", "bump_release")` (etc.). Notebook Libraries drawer browses builtins + stdlib (Getting started / Advanced).
- **Script file extension**: user scripts and stdlib modules use `.buddy` (editor language id / LSP). Keep accepting `.star` (and any path) for legacy scripts.
- **User docs**: `README.md`, `docs/reference-manual.md` — keep in sync when adding commands or secret behavior.
- **Scope**: minimal diffs; match existing patterns; no over-engineering.

## Testing

- `internal/buddy/auth_test.go` — direct auth opts, OAuth creds file
- `internal/buddy/auth_resolve_test.go` — secret resolution with mock store
- `internal/buddy/secrets/file_test.go` — file store round-trips
- `cmd/buddy/*_test.go`, `internal/script/*_test.go` — CLI and Starlark

Use `secrets.SetStore(mock)` in tests to override the platform store.

## What not to do

- Do not recreate `protoken` / `protok` / `jamftoken` — extend buddy `auth` subcommands instead.
- Do not put production secrets only in `config.yaml` when platform storage exists.
- Do not break `buddy auth token --raw` stdout contract (meliorate and other tools depend on it).
- Do not put org-specific tenant/auth names, hostnames, or client IDs in docs, help examples, tests, or **commit messages** — use generics (`sandbox`, `prod`, `example.jamfcloud.com`).

## Open follow-ups (optional)

- Add dedicated `buddy auth help refresh` block in `help.go` if refresh usage is unclear.
- Ensure meliorate docs/examples consistently reference buddy (not legacy names).
