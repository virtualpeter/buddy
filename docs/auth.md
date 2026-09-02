# Auth, secrets, and tokens

Buddy talks to Jamf using a **tenant** (which environment) plus an **auth** (which principal). Passwords and OAuth client secrets belong in the encrypted vault, not in `config.yaml`. Tokens are cached and refreshed automatically.

First-time install: [Getting started](getting-started.md) (`buddy setup basic-setup` or, with an org MDM profile, `buddy setup secrets` / `buddy cred add`). YAML field reference: [README](../README.md#configuration). Jamf URLs and Classic vs Pro vs Platform: [API endpoints](api-endpoints.md).

Maintenance: `buddy cred status` shows each tenant × surface (instance URL vs platform identity), covering auths, whether a secret is **stored**, and the last `auth test` result. Gaps are tenants that have an identity but no covering auth (or no secret). Low-level `buddy cred set tenant|auth|secret` still works.

---

## Tenants and auths

A **tenant** is a Jamf environment: an instance `url` and/or Platform `region` + `tenantId`.

An **auth** is a secret-bearing principal linked to one or more tenants (`tenants: […]`). Types:

| Type | What you configure | How buddy authenticates |
|------|--------------------|-------------------------|
| `basic` | `username`; password in the secret store (or inline for tests) | Username/password → instance `POST /api/v1/auth/token` |
| `oauth` | `clientId`; client secret in the store | Client credentials → instance `/api/oauth/token` |
| `platform` | `clientId`; client secret in the store | Client credentials → Platform `{gateway}/auth/token` |
| `command` | `runCommand` | Runs the command each time; its stdout is the bearer token |

Optional auth fields:

- `access`: `full` (default) or `readonly` — readonly auths are preferred for reads; mutates need `full`.
- `channels:` — which buddy clients may use this auth (`cli`, `mcp`, `ide`). Omit the list to allow all.

`type` encodes the Jamf surface: `basic` / `oauth` / `command` are instance; `platform` is the API Gateway. An OAuth client is never valid on both. Legacy `surface: platform` on `type: oauth` migrates to `type: platform` on load.

`credentials:`, a bare string `tenant:`, `defaults:`, and `allow_auths:` are rejected — use `tenants:` / `auths:` / `tenant.default` / `channels.`.

---

## Choosing a session

| Mechanism | Effect |
|-----------|--------|
| `tenant.default` | Global fallback tenant |
| `channels.<cli\|mcp\|ide>.tenant` | Per-client default |
| `--tenant` | Select tenant (auto-picks an auth for the operation) |
| `--user` / `--auth` | Pin a specific auth |
| buddyscript / MCP `tenant=` / `surface=` / `auth=` | Per-call override (`cred=` was removed) |

When auto-picking, buddy:

- Prefers a **readonly** auth for reads
- Requires **full** for mutates
- Uses **platform** for blueprints and other Platform product APIs
- Uses **instance** for Classic / schema work

You can keep a platform-readonly auth and an instance-full auth on the **same tenant** — reads use the pin; mutates fall over to the full auth.

`--user` / `--auth` (and buddyscript `auth=`) is a **preference**, not a lock:

- Classic (while `classic_instance_only` is on), ACS, schema, and undocumented Pro `GET /api/v1/policies` need an instance auth
- Platform product APIs need platform oauth
- If the pin cannot serve that surface, or the pin is **readonly** and the op mutates, buddy auto-picks another linked auth on the **same tenant**
- Channel `access: readonly` is the hard clamp (MCP/IDE will not mutate)
- `surface=` biases published Pro/JSON calls that can use either surface, and Classic when `classic_instance_only` is off; it never sends ACS or `GET /api/v1/policies` through the gateway (clone/copy destination sessions included)

Direct one-off flags (`--url`, `--tenant-id`, `--username`, `--password`, `--client-id`, `--client-secret`) bypass config for a single command.

---

## Channel policy

Top-level `channels.<client>` can clamp what that client is allowed to do:

- `access`: `readonly` or `full` — the most restrictive of channel vs auth wins for mutates.
- `tenant` / `tenants` — default and allowlist for that client.
- `deny_builtins` / `allow_builtins` — also valid on **tenants.** and **auths.** (same list/glob form). Each layer is independent: a builtin is blocked if **any** layer matches deny and that layer’s allow does not match. Channel allow cannot unblock a tenant deny.

MCP already denies `jamf_*` on the channel layer even when the YAML list is empty. `deny_builtins: ["*"]` is deny-all at that layer (explicit allowlist when paired with `allow_builtins`). Quote `"*"` so YAML does not treat it as an alias. Blocked builtins raise; they are a gap, not a cue to invent HTTP from the schema.

The IDE uses the `ide` client channel (there is no `serve` or `notebook` channel).

```sh
buddy cred set channel mcp --access=readonly --tenant=sandbox
buddy cred set channel mcp --deny-builtins=computer_erase
buddy cred set channel mcp --allow-builtins=*
buddy cred set tenant prod --deny-builtins=* --allow-builtins=policies,policy
buddy cred set auth mcp-ro --type=oauth --tenants=prod --client-id=… --deny-builtins=computer_erase
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

`buddy setup secrets` creates an auth for any tenant that has none, fills instance/platform coverage gaps (instance URL with no instance auth, or platform identity with no platform oauth), then prompts for missing client IDs and store secrets. `buddy cred add` is the one-shot guided path for a named tenant+auth. `buddy setup managed-plist [path]` dumps tenants and channel policy as XML plist for Jamf ACS.

`buddy cred list` prints a `managed: …` line when prefs contributed. `buddy cred status` is the coverage matrix. Deleting a managed tenant or auth from YAML warns: it comes back on the next load until the profile changes or you keep a local override.

---

## Where secrets live

Keep production passwords and OAuth client secrets **out of YAML**. Inline secrets are fine for local tests only.

Resolution order: **inline config → vault → error**. Vault keys are **auth names**.

Passwords and client secrets are stored in **`~/.config/buddy/secrets.enc`** (AES-256-GCM) next to `config.yaml`. A **random 256-bit data key (DEK)** unlocks the vault on each CLI run (no PBKDF2, no Touch ID). A **recovery passphrase** is only for `cred backup` / `restore` and for unwrapping that DEK onto a new binary or Mac.

| `BUDDY_SECRET_STORE` | Platform | Data key (DEK) | Recovery passphrase |
|----------------------|----------|----------------|---------------------|
| (default) | macOS (`make buddy`) | Keychain, service `buddy`, account `dek`, via `security(1)` | same service, account `master` |
| (default) | macOS (`make buddy-darwin-arm64`) | Keychain, service `virtualpete.buddy.cli` (label Buddy), account `dek`, `AfterFirstUnlockThisDeviceOnly`, via Security.framework | same service, account `master` |
| (default) | Linux / Windows | `~/.config/buddy/dek` (mode `0600`) | `~/.config/buddy/master` (mode `0600`; Windows: `%APPDATA%\buddy\…`) |
| `file` | any | Same as the Linux DEK file | Same as the Linux master file |
| `pass` | Unix | `pass show buddy/dek` | `pass show buddy/master` |
| `keychain` | macOS only | Explicit Keychain (same backend as the binary: CLI or SecItem) | same |

`BUDDY_VAULT_PASSPHRASE` overrides the recovery passphrase for CI/tests (never written). If the value is a tagged DEK (`dek:v1:…`), it overrides the data key instead.

**Default Darwin binary (`make buddy`)**

- Writes Keychain items with `security -i` (secret on stdin, not argv) under service `buddy`
- Convenient for development (no CGO, survives `go build`)
- ACL is on `/usr/bin/security`, so any process running as you can read the same items

**Signed release (`make buddy-darwin-arm64`, build tag `secitem`)**

- Uses Security.framework so the ACL is buddy’s Developer ID (only that signed binary can read without a prompt)
- Upgrades keep working as long as the Team ID and identifier stay the same
- Separate Keychain service (`virtualpete.buddy.cli`, Keychain Access label **Buddy**) — never shares ACL or items with `./buddy`; the vault file is shared
- DEK uses `AfterFirstUnlockThisDeviceOnly` (available after login; screen lock does not prompt Terminal)
- Touch ID / user presence only when creating or unwrapping the DEK, setting the recovery passphrase, or restoring a backup — not on ordinary commands such as `buddy policy list`
- Unsigned `./buddy` cannot use a meaningful app ACL; biometrics are a signed-release feature

A signed binary does not see a DEK stored by `./buddy`. Use the same recovery passphrase with `buddy cred set passphrase` against `./buddy-darwin-arm64` (unwraps the DEK from the vault file into the signed Keychain), or `cred backup` / `restore`. `make install-signed` copies that binary to `~/go/bin/buddy`. `buddy cred erase secrets` deletes the vault, the DEK, the recovery passphrase, and leftover per-auth items from **both** Keychain services.

**Codesign**

- Requires Darwin/arm64 and `BUDDY_CODESIGN_IDENTITY` (a Developer ID Application identity)
- Identifier defaults to `virtualpete.buddy.cli` (`BUDDY_CODESIGN_IDENTIFIER`) and must stay stable so Keychain ACLs survive rebuilds
- The Keychain service is hardcoded to that reverse-DNS string (not read from the running binary), so overriding the identifier at sign time still stores items under `virtualpete.buddy.cli`
- Ad-hoc signatures are rejected

Older vault files used PBKDF2 with the recovery passphrase as the wrapping secret. The first CLI run after upgrade migrates them to a DEK (PBKDF2 stays on backup files and on the optional DEK wrap inside `secrets.enc`). Older installs stored one Keychain/file/pass item per secret. `buddy cred migrate` copies those into the vault (and can delete the old items).

```sh
buddy cred status                               # tenant × surface coverage (no Jamf call)
buddy cred add --tenant=sandbox --url=https://example.jamfcloud.com --client-id=… --client-secret=…
buddy cred set passphrase                        # create / rotate the recovery passphrase (backups)
buddy cred set secret sandbox-admin                  # password (basic)
buddy cred set secret sandbox-admin --client-secret  # OAuth
buddy cred migrate                                   # inline YAML + old per-auth store → vault
buddy cred backup ./buddy.creds                      # uses stored recovery passphrase by default; AES-GCM file
buddy cred restore ./buddy.creds                     # recovery passphrase; confirm each auth; new local DEK
buddy cred clean secrets                             # review / delete orphaned vault entries
buddy cred erase secrets                             # wipe vault + DEK + recovery passphrase (+ leftover Keychain items)
buddy cred delete auth sandbox-admin                 # also clears vault secrets
```

`buddy cred get secret` and the SECRET column on `cred status` are **presence only**. `buddy auth test` discards any cached token and mints a new one: missing secret / token POST failure is an error; probe 403 after a good token is `live=token-ok` (the secret works, the role lacks that probe).

`buddy cred backup` / `restore` encrypt tenants, auths (including vault secrets), and channel policy with a **recovery passphrase** (AES-256-GCM, PBKDF2-SHA256). The DEK and any biometric/Keychain material are never written into `buddy.creds`. Backup defaults to the stored recovery passphrase (`--passphrase=` overrides). Restore applies tenants and channels, then prompts once per auth unless `-y` (Enter/y writes that auth from the backup; `n` leaves the existing config entry). Auths not in the backup are kept. Secrets go back to the vault under a **new** local DEK. If no recovery passphrase is stored yet, restore saves the backup passphrase as the recovery phrase.

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

Last `auth test` results (token ok / probe, and instance privilege names when `/api/v1/auth` succeeded) are cached under `~/.config/buddy/caps/` with the same 0600 + obfuscation style as tokens. Buddy uses that cache when auto-picking among auths. Missing or stale cache is ignored (fail open). Legacy `type: oauth` + `surface: platform` migrates to `type: platform` on load.

---

## Instance vs Platform

Call sites still use instance-relative paths (`/api/v1/…`, `/JSSResource/…`). When the session is on a **platform** surface, buddy rewrites them (Pro → `/api/pro/vN/tenant/{id}/…`, Classic → `/api/proclassic/tenant/{id}/…` with the `/JSSResource` prefix stripped, Platform product APIs inject `/tenant/{id}` after the version).

Classic follows a platform pin / auto-pick by default (`OpClassic` / `OpClassicWrite` → `/api/proclassic/tenant/{id}/…`, no `/JSSResource`). Auto-pick among mixed auths still prefers an instance auth when one is linked. Set `features.classic_instance_only: true` (or `BUDDY_CLASSIC_INSTANCE_ONLY=true`) to force **Classic `/JSSResource` only** onto instance even when the pin is platform. A gateway `403` means the OAuth client lacks Classic scopes.

That toggle does **not** apply to surfaces that are never published on the gateway. These always use the instance URL (a platform pin auto-picks an instance auth on the same tenant):

- **ACS** `/api/config-profiles/…` — Jamf Pro UI API; gateway POST is `404 page not found`
- **Undocumented Pro policy list** `GET /api/v1/policies` — not in published OpenAPI; gateway is `403`. Policy writes and detail stay Classic XML (`/JSSResource/policies`)
- **Schema discovery** `/api/schema` and Classic swagger — instance host only

```yaml
features:
  classic_instance_only: true    # optional lock; default false = Classic may use /api/proclassic/
```

Platform product commands (`buddy blueprint`, `device`, `device-group`, `ddm`, `benchmark`, and the matching builtins) need a platform-capable tenant + auth.

See [API endpoints](api-endpoints.md) for the rewrite table, schema discovery, and which Pro versions buddy uses (computer inventory is **v4**; computer groups stay **v3**).
