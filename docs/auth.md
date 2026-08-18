# Auth, secrets, and tokens

Buddy talks to Jamf using a **tenant** (which environment) plus an **auth** (which principal). Passwords and OAuth client secrets belong in the platform secret store, not in `config.yaml`. Tokens are cached and refreshed automatically.

First-time install: [Getting started](index.md) (`buddy setup basic-setup` or, with an org MDM profile, `buddy setup secrets` / `buddy cred add`). YAML field reference: [README](../README.md#configuration). Jamf URLs and Classic vs Pro vs Platform: [API endpoints](api-endpoints.md).

Maintenance: `buddy cred status` shows each tenant × surface (instance URL vs platform identity), covering auths, whether a secret is **stored**, and the last `auth test` result. Gaps are tenants that have an identity but no covering auth (or no secret). Low-level `buddy cred set tenant|auth|secret` still works.

---

## Tenants and auths

A **tenant** is a Jamf environment: an instance `url` and/or Platform `region` + `tenantId`.

An **auth** is a secret-bearing principal linked to one or more tenants (`tenants: […]`). Types:

| Type | What you configure | How buddy authenticates |
|------|--------------------|-------------------------|
| `basic` | `username`; password in the secret store (or inline for tests) | Username/password → `POST /api/v1/auth/token` |
| `oauth` | `clientId`; client secret in the store | Client credentials → instance `/api/oauth/token`, or Platform `{gateway}/auth/token` |
| `command` | `runCommand` | Runs the command each time; its stdout is the bearer token |

Optional auth fields:

- `access`: `full` (default) or `readonly` — readonly auths are preferred for reads; mutates need `full`.
- `surface`: `any` (default), `platform`, or `instance` — which Jamf API surface this auth may use.
- `channels:` — which buddy clients may use this auth (`cli`, `mcp`, `ide`). Omit the list to allow all.

Legacy config shapes (`credentials:`, string `tenant:`, `defaults:`, `allow_auths:`) migrate automatically on load. Only the new form is written back.

---

## Choosing a session

| Mechanism | Effect |
|-----------|--------|
| `tenant.default` | Global fallback tenant |
| `channels.<cli\|mcp\|ide>.tenant` | Per-client default |
| `--tenant` | Select tenant (auto-picks an auth for the operation) |
| `--user` / `--auth` | Pin a specific auth |
| buddyscript / MCP `tenant=` / `surface=` / `auth=` | Per-call override (`cred=` was removed) |

When auto-picking, buddy prefers a **readonly** auth for reads, requires **full** for mutates, **platform** for blueprints and other Platform product APIs, and **instance** for Classic / schema work.

Direct one-off flags (`--url`, `--tenant-id`, `--username`, `--password`, `--client-id`, `--client-secret`) bypass config for a single command.

---

## Channel policy

Top-level `channels.<client>` can clamp what that client is allowed to do:

- `access`: `readonly` or `full` — the most restrictive of channel vs auth wins for mutates.
- `tenant` / `tenants` — default and allowlist for that client.
- `deny_builtins` — exact builtin names or trailing-`*` globs (for example `jamf_*`). Blocked builtins raise; they are a gap, not a cue to invent HTTP from the schema.

The IDE uses the `ide` client channel (there is no shared `serve` channel). Legacy `channels.notebook` migrates onto `channels.ide` on load.

```sh
buddy cred set channel mcp --access=readonly --tenant=sandbox
buddy cred set channel mcp --deny-builtins=jamf_*,computer_erase
buddy cred use --mcp sandbox
```

---

## Org defaults (macOS managed preferences)

Buddy can load config from a Jamf Application & Custom Settings (or MCX) payload for preference domain **`virtualpete.buddy.cli`** (the same reverse-DNS as the signed binary / Keychain service). This is org **defaults**, not enforced policy: `~/.config/buddy/config.yaml` overlays and wins per tenant, auth, and channel name.

Search order:

1. `BUDDY_MANAGED_PLIST` if set (tests and non-Darwin drop-in; a missing path disables discovery)
2. Else on macOS: `/Library/Managed Preferences/virtualpete.buddy.cli.plist`, then `~/Library/Managed Preferences/virtualpete.buddy.cli.plist` (user-managed overlays computer)
3. User `config.yaml`

Typical profile content: `tenant.default`, `tenants`, and `channels` policy. Auths (`clientId`, usernames, secrets, even auth *names*) are per-user: **accepted** if present in the plist, but `buddy setup managed-plist` omits the whole `auths` map. Prefer `buddy setup secrets` for client IDs and secrets (managed plists are typically world-readable).

`buddy setup secrets` creates an auth for any tenant that has none, fills instance/platform coverage gaps (instance URL with no instance/`any` auth, or platform identity with no platform/`any` oauth), then prompts for missing client IDs and store secrets. `buddy cred add` is the one-shot guided path for a named tenant+auth. `buddy setup managed-plist [path]` dumps tenants and channel policy as XML plist for Jamf ACS.

`buddy cred list` prints a `managed: …` line when prefs contributed. `buddy cred status` is the coverage matrix. Deleting a managed tenant or auth from YAML warns: it comes back on the next load until the profile changes or you keep a local override.

---

## Where secrets live

Keep production passwords and OAuth client secrets **out of YAML**. Inline secrets are fine for local tests only.

Resolution order: **inline config → platform store → error**. Store keys are **auth names**.

| `BUDDY_SECRET_STORE` | Platform | Location |
|----------------------|----------|----------|
| (default) | macOS (`make buddy`) | Keychain, service `buddy`, via `security(1)` |
| (default) | macOS (`make buddy-darwin-arm64`) | Keychain, service `virtualpete.buddy.cli` (label Buddy), via Security.framework |
| (default) | Linux / Windows | `~/.config/buddy/secrets/<auth>.json` (mode `0600`; Windows: `%APPDATA%\buddy\secrets\`) |
| `file` | any | Same as the Linux file store |
| `pass` | Unix | `buddy/<auth>/password` and `buddy/<auth>/client-secret` |
| `keychain` | macOS only | Explicit Keychain (same backend as the binary: CLI or SecItem) |

The default Darwin binary writes Keychain items with `security -i` (secret on stdin, not argv) under service `buddy`. That is convenient for development (no CGO, survives `go build`), but the ACL is on `/usr/bin/security`, so any process running as you can read the same items.

The signed release binary (`make buddy-darwin-arm64`, build tag `secitem`) uses Security.framework so the ACL is buddy’s Developer ID: only that signed binary can read without a prompt, and upgrades keep working as long as the Team ID and identifier stay the same. It uses a **different** service (`virtualpete.buddy.cli`, Keychain Access label **Buddy**) so the two never share ACL or items. A signed binary does not see secrets stored by `./buddy`; run `buddy cred set secret` (or `buddy cred migrate`) again against `./buddy-darwin-arm64`. `make install-signed` copies that binary to `~/go/bin/buddy`. `buddy cred erase secrets` deletes items from **both** services.

`make buddy-darwin-arm64` requires Darwin/arm64 and `BUDDY_CODESIGN_IDENTITY` (a Developer ID Application identity). The identifier defaults to `virtualpete.buddy.cli` (`BUDDY_CODESIGN_IDENTIFIER`) and must stay stable across releases so Keychain ACLs survive rebuilds. The Keychain service is hardcoded to that same reverse-DNS string (not read from the running binary), so overriding the identifier at sign time still stores items under `virtualpete.buddy.cli`. Ad-hoc signatures are rejected.

```sh
buddy cred status                               # tenant × surface coverage (no Jamf call)
buddy cred add --tenant=sandbox --url=https://example.jamfcloud.com --client-id=… --client-secret=…
buddy cred set secret sandbox-admin                  # password (basic)
buddy cred set secret sandbox-admin --client-secret  # OAuth
buddy cred migrate                                   # move inline YAML secrets into the store
buddy cred backup ./buddy.creds                      # passphrase prompt; AES-GCM file
buddy cred restore ./buddy.creds                     # passphrase; confirm each auth
buddy cred clean secrets                             # review / delete orphaned store entries
buddy cred erase secrets                             # wipe all store secrets (both macOS Keychain services)
buddy cred delete auth sandbox-admin                 # also clears store secrets
```

`buddy cred get secret` and the SECRET column on `cred status` are **presence only**. `buddy auth test` discards any cached token and mints a new one: missing secret / token POST failure is an error; probe 403 after a good token is `live=token-ok` (the secret works, the role lacks that probe).

`buddy cred backup` / `restore` encrypt tenants, auths (including store secrets), and channel policy with a passphrase (AES-256-GCM, PBKDF2-SHA256). Restore applies tenants and channels, then prompts once per auth unless `-y` (Enter/y writes that auth from the backup; `n` leaves the existing config entry). Auths not in the backup are kept. Secrets go back to the platform store.

Config and tokens live under `~/.config/buddy` on macOS/Linux (`$XDG_CONFIG_HOME/buddy` when set), and `%APPDATA%\buddy` on Windows.

---

## How tokens are cached

Buddy reuses a bearer token until shortly before it expires (about 15 seconds of clock skew), then obtains a new one. There is no proactive keep-alive while the token is still valid.

| Auth type | Token source | Refresh |
|-----------|--------------|---------|
| `basic` | `/api/v1/auth/token` (treated as ~18 minutes, ahead of Jamf’s 20-minute limit) | Keep-alive on HTTP 401 or `buddy auth refresh`; otherwise re-auth |
| `oauth` (instance) | `/api/oauth/token`; expiry from `expires_in` | Re-auth with client credentials when expired or on 401 |
| `oauth` (Platform) | `{gateway}/auth/token` (fallback `/oauth2/token`); ~900s | Re-auth; no keep-alive |
| `command` | External command each time | Not cached on disk |

Authenticated HTTP retries **once** after a refresh if Jamf returns 401.

Tokens are also written under `~/.config/buddy/tokens/` (mode `0600`) so later CLI invocations can reuse them. The file is obfuscated enough to discourage casual `cat`; it is **not** a secret store. Expired, URL-mismatched, or corrupt files are deleted on load. `command` auth skips the disk cache.

```sh
buddy auth test                         # ForceRenew + probe the active tenant/auth
buddy auth test --all                   # every covering auth × tenant × surface
buddy auth caps                         # last-check / privileges cache for the current pin
buddy auth token [--raw] [--tenant=T] [--user=A]
buddy auth refresh --url=URL [TOKEN]    # keep-alive (instance; basic)
buddy cred clean tokens                 # delete expired or corrupt disk cache files
```

`--raw` prints only the bearer token on stdout (no `Authorization:` prefix). Use it when another program should consume the token as a single line.

Last `auth test` results (token ok / probe, and instance privilege names when `/api/v1/auth` succeeded) are cached under `~/.config/buddy/caps/` with the same 0600 + obfuscation style as tokens. Buddy uses that cache when auto-picking among auths and when an auth is `surface: any` on a tenant that has both instance and platform identities: prefer the surface whose last check minted a token. Missing or stale cache is ignored (fail open).

---

## Instance vs Platform

Call sites still use instance-relative paths (`/api/v1/…`, `/JSSResource/…`). When the session is on a **platform** surface, buddy rewrites them (Pro → `/api/pro/vN/tenant/{id}/…`, Classic → `/api/proclassic/tenant/{id}/JSSResource/…`, Platform product APIs inject `/tenant/{id}` after the version).

Classic path rewrite is implemented, but Jamf Account Platform integrations do not yet expose Classic permissions — expect **403** on `/JSSResource/…` until they do. Prefer an **instance** auth for Classic-heavy work (policies XML, profiles, and similar).

Platform product commands (`buddy blueprint`, `device`, `device-group`, `ddm`, `benchmark`, and the matching builtins) need a platform-capable tenant + auth.

See [API endpoints](api-endpoints.md) for the rewrite table, schema discovery, and which Pro versions buddy uses (computer groups and inventory are **v3**; inventory v4 is in some cloud schemas but not in production yet).
