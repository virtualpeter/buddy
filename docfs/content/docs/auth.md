# Auth, secrets, and tokens

Buddy talks to Jamf using a **tenant** (which environment) plus an **auth** (which principal). Passwords and OAuth client secrets belong in the platform secret store, not in `config.yaml`. Tokens are cached and refreshed automatically.

First-time install: [Getting started](index.md) (`buddy setup basic-setup`). YAML field reference: [README](../README.md#configuration). Jamf URLs and Classic vs Pro vs Platform: [API endpoints](api-endpoints.md).

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

## Where secrets live

Keep production passwords and OAuth client secrets **out of YAML**. Inline secrets are fine for local tests only.

Resolution order: **inline config → platform store → error**. Store keys are **auth names**.

| `BUDDY_SECRET_STORE` | Platform | Location |
|----------------------|----------|----------|
| (default) | macOS | Keychain, service `buddy` |
| (default) | Linux / Windows | `~/.config/buddy/secrets/<auth>.json` (mode `0600`; Windows: `%APPDATA%\buddy\secrets\`) |
| `file` | any | Same as the Linux file store |
| `pass` | Unix | `buddy/<auth>/password` and `buddy/<auth>/client-secret` |
| `keychain` | macOS only | Explicit Keychain |

```sh
buddy cred set secret sandbox-admin                  # password (basic)
buddy cred set secret sandbox-admin --client-secret  # OAuth
buddy cred migrate                                   # move inline YAML secrets into the store
buddy cred clean secrets                             # review / delete orphaned store entries
buddy cred delete auth sandbox-admin                 # also clears store secrets
```

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
buddy auth test                         # verify the active tenant/auth
buddy auth token [--raw] [--tenant=T] [--user=A]
buddy auth refresh --url=URL [TOKEN]    # keep-alive (instance; basic)
buddy cred clean tokens                 # delete expired or corrupt disk cache files
```

`--raw` prints only the bearer token on stdout (no `Authorization:` prefix). Use it when another program should consume the token as a single line.

---

## Instance vs Platform

Call sites still use instance-relative paths (`/api/v1/…`, `/JSSResource/…`). When the session is on a **platform** surface, buddy rewrites them (Pro → `/api/pro/vN/tenant/{id}/…`, Classic → `/api/proclassic/tenant/{id}/JSSResource/…`, Platform product APIs inject `/tenant/{id}` after the version).

Classic path rewrite is implemented, but Jamf Account Platform integrations do not yet expose Classic permissions — expect **403** on `/JSSResource/…` until they do. Prefer an **instance** auth for Classic-heavy work (policies XML, profiles, and similar).

Platform product commands (`buddy blueprint`, `device`, `device-group`, `ddm`, `benchmark`, and the matching builtins) need a platform-capable tenant + auth.

See [API endpoints](api-endpoints.md) for the rewrite table, schema discovery, and which Pro versions buddy uses (computer groups and inventory are **v3**; inventory v4 is in some cloud schemas but not in production yet).
