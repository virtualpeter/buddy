# Agent guide: buddy

This file captures project context for AI agents (and humans) picking up work on **buddy** without prior chat history.

## What buddy is

**buddy** is a Go CLI for Jamf Pro automation: policies, groups, profiles, packages, Platform API Blueprints, Starlark workflows, and API auth/token helpers. Config lives at `~/.config/buddy/config.yaml`.

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

(`--cred=default` still works but prints a deprecation note on stderr.)

## Repository layout

| Path | Role |
|------|------|
| `cmd/buddy/` | CLI entry (`main.go`), dispatch, help text (`help.go`) |
| `internal/buddy/` | Jamf API clients, config, auth, secret CLI |
| `internal/buddy/tokenauth/` | Token acquisition, caching, proactive/reactive refresh |
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

Build: `make buddy` → `./buddy`. Tests: `go test ./...`. Docs UI: `make docs-serve`. Extension VSIX: `make extension-package` (`PUBLISHER` defaults to `virtualpeter`).

## MCP (Cursor)

See **[docs/mcp.md](docs/mcp.md)** for Cursor setup. Summary:

- Config: `.cursor/mcp.json` or `~/.cursor/mcp.json` with absolute `command` path and `args: ["mcp"]`.
- Docs/AGENTS/examples/Cursor rules are embedded in the binary (`docfs/`; refresh with `make sync-docfs`). `BUDDY_DOCS_ROOT` is an optional live override. Agents should use the MCP `docs` tool (list/get); the same pages are also `buddy://docs/...` resources.
- Tool results are always `{ok, value, ...}` objects (lists under `value`) so Cursor accepts `structuredContent`.
- Reload MCP in Cursor Settings after changing config or rebuilding buddy.


## Authentication model

Each named credential in config has `url` + `auth`:

| `auth.type` | Config fields | Secret storage |
|-------------|---------------|----------------|
| `command` | `runCommand` (shell string) | N/A — command prints token |
| `basic` | `username`; password inline or in store | `buddy cred set secret <cred>` |
| `oauth` | `clientId`; secret inline or in store | `buddy cred set secret <cred> --client-secret` |

Optional credential fields `region` (e.g. `us`) and/or gateway `url`, plus `tenantId`, enable Platform API Gateway mode: OAuth only, path rewrite for Pro/Classic, and Blueprints. With `region` set, the base URL defaults to `https://{region}.apigw.jamf.com` unless `url` overrides it (proxy).

**Classic parity:** path rewrite for `/JSSResource/…` is implemented, but Platform integrations do not yet offer Classic permissions in Jamf Account — expect `403` on Classic until Jamf exposes them. Use an instance credential for Classic-heavy work; Pro + Blueprints are the supported gateway surfaces today.

Resolution order for secrets: **inline config → platform store → error** (`ResolveAuthWithSecrets` in `internal/buddy/auth_resolve.go`).

### Secret backends

Selected by platform or `BUDDY_SECRET_STORE`:

| Value | Platform | Location |
|-------|----------|----------|
| (default) | macOS | Keychain, service `buddy` |
| (default) | Linux | `~/.config/buddy/secrets/<cred>.json` (0600) |
| `file` | any | Same as Linux default |
| `pass` | any | `buddy/<cred>/password`, `buddy/<cred>/client-secret` |
| `keychain` | macOS only | Explicit Keychain |

CLI: `buddy cred set|get|delete secret <cred>`, `buddy cred store`, `buddy cred migrate`. `buddy setup configure-oauth` stores client secret in the platform store automatically.

## Token caching

Three layers:

1. **Process-local cache** — `credAuthCache` in `internal/buddy/common.go` holds one `tokenauth.Authenticator` per credential name for the lifetime of the buddy process.
2. **In-authenticator cache** — each `Authenticator` stores `token` + `tokenExpiry`.
3. **On-disk cache** — `~/.config/buddy/tokens/<cache-key>.json` (mode `0600`), keyed by credential name (or a `direct-…` key for one-off auth flags). Reused across CLI invocations when expiry is more than ~3 minutes away. Skipped for `command` auth. Invalidated on failed refresh / full re-auth.

`GetToken()` flow (`internal/buddy/tokenauth/auth.go`):

- **command auth**: runs external command each time (no long-lived or disk cache).
- **basic auth**: username/password → `/api/v1/auth/token`; expiry treated as ~18 minutes (refresh before Jamf’s 20-minute limit).
- **oauth** (instance): client credentials → `/api/oauth/token`; expiry from `expires_in`.
- **oauth** (Platform API Gateway, credential `tenantId` set): client credentials → `{gateway}/auth/token` (fallback `/oauth2/token`); ~900s expiry; **no keep-alive** — re-auth when near expiry.
- **Disk hydrate**: if memory is empty and `CacheKey` is set, load a still-valid disk token before hitting Jamf.
- **Proactive refresh**: if token expires within 3 minutes, call `/api/v1/auth/keep-alive` before it expires (instance basic/oauth only; skipped for Platform).
- **Reactive refresh**: on HTTP 401, `RefreshToken()` retries once (keep-alive or full re-auth); disk cache cleared when re-authing.

CLI surfaces:

```sh
buddy auth test                         # verify credential
buddy auth token [--raw] [--user=NAME]  # print Authorization header or raw bearer
buddy auth refresh --url=URL [TOKEN]    # keep-alive refresh (instance; direct flags supported)
```

Direct auth flags (`--url`, `--tenant-id`, `--username`, `--password`, `--client-id`, `--client-secret`) bypass config for one-off use.

When `tenantId` is set, request paths are rewritten via `RewriteJamfPath` / `ResolveEndpoint` (Pro → `/api/pro/vN/tenant/{id}/…`, Classic → `/api/proclassic/tenant/{id}/JSSResource/…`). Blueprints: `buddy blueprint`, Starlark `blueprints` / `blueprint_*`, and first-class MCP `blueprint*` tools (gateway credential required).

## Jamf Pro API versions (groups / inventory)

| Resource | Buddy uses | Notes |
|----------|------------|--------|
| Computer groups (smart/static) | `/api/v3/computer-groups/...` | Migrated from v2 (v2 deprecated in cloud schema ~2026-05-28). |
| Computer inventory | `/api/v3/computers-inventory` | **Stay on v3.** Cloud schema exposes v4 and deprecates v3 (~2026-07-14), but production Jamf does not support v4 yet. Do not add a v4 path (or v4→v3 fallback) until prod does; then remap filters/sort (`lastContactTime` → `lastCheckIn` / `lastContact`). |

See [`docs/api-endpoints.md`](docs/api-endpoints.md) for the full endpoint map.

## Integration as meliorate token helper

meliorate runs `token_command` as a subprocess and expects **only the raw bearer token on stdout** (no log lines). Use:

```sh
buddy auth token --raw --user=default
```

buddy resolves secrets from the platform store at runtime, so meliorate config does not need plaintext secrets when buddy holds them.

## Conventions (follow existing rules)

- **CLI help**: see `.cursor/rules/buddy-cli.mdc` — usage lines, subcommand naming, placeholders.
- **Starlark builtins**: see `.cursor/rules/buddy-builtins.mdc` and `docs/builtins-v3.md` — v3 plural/singular grammar (`policies` / `policy` / `policy_set`), resource IDs first on mutate helpers, `cred?` last. Deprecated aliases warn once on stderr.
- **Starlark stdlib**: multi-step helpers in `stdlib/*.buddy` via `load("stdlib/release.buddy", "bump_release")` (etc.). Notebook Libraries drawer browses builtins + stdlib.
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

## Open follow-ups (optional)

- Add dedicated `buddy auth help refresh` block in `help.go` if refresh usage is unclear.
- Ensure meliorate docs/examples consistently reference buddy (not legacy names).
