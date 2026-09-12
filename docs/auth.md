# Auth, secrets, and tokens

Buddy talks to Jamf using a **context** (a named workplace in config) plus an **auth** (which principal). Passwords and OAuth client secrets belong in the encrypted vault, not in `config.yaml`. Tokens are cached and refreshed automatically.

First-time install: [Getting started](getting-started.md). With an empty config, any interactive Jamf command (or `buddy cred add`) runs the setup wizard. With an org MDM profile, use `buddy setup secrets` / `buddy cred add`. The docs UI **config** page edits context/auth metadata only — secrets stay in the terminal vault. YAML field reference: [README](../README.md#configuration). Jamf URLs and Classic vs Pro vs Platform: [API endpoints](api-endpoints.md).

Maintenance: `buddy cred status` shows each context × surface (instance URL vs platform identity), covering auths, whether a secret is **locked** / **stored** / missing, and the last `auth test` result. Default status does not open the vault (`SECRET=locked`); `--unlock` or `--test` classifies stored vs missing. Gaps are contexts that have an identity but no covering auth (or no secret after unlock). Low-level `buddy cred set context|auth|secret` still works (`tenant` is a dual-read alias).

---

## Contexts and auths

A **context** is a named buddy workplace (e.g. `sandbox`). The default path is **instance-only**: a Jamf Pro `jamfpro.url` and an `oauth`/`basic` auth. No Account environment, region, or platform auth is required.

When you adopt Jamf Account / Platform, add fields on the **same name**:

| YAML | Header | What it is |
|------|--------|------------|
| `contexts.<name>.id` (or `environmentId`) | `X-Environment-Id` | Jamf Account **platform environment** — a group of product tenants. Prefer this for GA integrations (blueprints, devices, …). |
| `contexts.<name>.jamfpro.tenantId` | `X-Tenant-Id` | One Jamf Pro / School / Protect / Security Cloud product tenant. Used when a platform auth’s `scope` is `tenant`. |
| `auths.<name>.scope: organization` | *(none)* | Organization-scoped Account APIs; product builtins need an environment or product tenant id. |

`id` and `jamfpro.tenantId` may both be set — they are different objects. Scope lives on the **auth** (`organization` / `environment` / `tenant`), not XOR’d onto the workplace row.

Copy ids from Jamf Account → Integrations. Do not use the jamfcloud.com hostname or the buddy context name. Beta `*.apigw.jamf.com` hosts are refused; use `https://{region}.api.jamfcloud.com`. Beta credentials cannot be migrated — register a new integration.

An **auth** is a secret-bearing principal. Instance auths attach with `jamfpro: sandbox`. Platform auths use `type: platform`, `scope:`, and `contexts: [sandbox]` (environment) or `jamfpro:` (tenant-scoped). Types:

| Type | What you configure | How buddy authenticates |
|------|--------------------|-------------------------|
| `basic` | `username`; password in the secret store (or inline for tests) | Username/password → instance `POST /api/v1/auth/token` |
| `oauth` | `clientId`; client secret in the store | Client credentials → instance `/api/oauth/token` |
| `platform` | `clientId`; client secret in the store; `scope:` | Client credentials → Platform `{gateway}/auth/token` |
| `command` | `runCommand` | Runs the command each time; its stdout is the bearer token |

Optional auth fields:

- `access`: `full` (default) or `readonly` — readonly auths are preferred for reads; mutates need `full`.
- `channels:` — which buddy clients may use this auth (`cli`, `mcp`, `ide`). Omit the list to allow all.

`type` encodes the Jamf surface: `basic` / `oauth` / `command` are instance; `platform` is the API Gateway. An OAuth client is never valid on both. Legacy `surface: platform` on `type: oauth` migrates to `type: platform` on load.

`credentials:`, a bare string `tenant:`/`context:`, `defaults:`, and `allow_auths:` are rejected. Dual-read `tenants:` / `tenant.default` / auth `tenants:` for one release; saves write `contexts:` / `context.default` / `jamfpro:` / `contexts:` + `scope:`.

Instance-only example:

```yaml
context:
  default: sandbox
contexts:
  sandbox:
    jamfpro:
      url: https://example.jamfcloud.com
auths:
  sandgod:
    type: oauth
    jamfpro: sandbox
    clientId: …
```

Same name after adding Platform:

```yaml
contexts:
  sandbox:
    id: 11111111-1111-1111-1111-111111111111
    region: us
    jamfpro:
      url: https://example.jamfcloud.com
      tenantId: 22222222-2222-2222-2222-222222222222
auths:
  sandgod:
    type: oauth
    jamfpro: sandbox
  sandbox-platform:
    type: platform
    scope: environment
    contexts: [sandbox]
    clientId: …
```

---

## Choosing a session

| Mechanism | Effect |
|-----------|--------|
| `context.default` | Global fallback context |
| `channels.<cli\|mcp\|ide>.context` | Per-client default |
| `--context` | Select context (auto-picks an auth for the operation); `--tenant` is a dual-read alias |
| `--user` / `--auth` | Pin a specific auth |
| buddyscript / MCP `context=` / `tenant=` / `surface=` / `auth=` | Per-call override (`cred=` was removed) |

When auto-picking, buddy:

- Prefers a **readonly** auth for reads
- Requires **full** for mutates
- Uses **platform** for blueprints and other Platform product APIs
- Uses **instance** for Classic / schema work

You can keep a platform-readonly auth and an instance-full auth on the **same context** — reads use the pin; mutates fall over to the full auth.

`--user` / `--auth` (and buddyscript `auth=`) is a **preference**, not a lock:

- Classic (while `classic_instance_only` is on), ACS, schema, and undocumented Pro `GET /api/v1/policies` need an instance auth
- Platform product APIs need platform oauth
- If the pin cannot serve that surface, or the pin is **readonly** and the op mutates, buddy auto-picks another linked auth on the **same context**
- Channel `access: readonly` is the hard clamp (MCP/IDE will not mutate)
- `surface=` biases published Pro/JSON calls that can use either surface, and Classic when `classic_instance_only` is off; it never sends ACS or `GET /api/v1/policies` through the gateway (clone/copy destination sessions included)

Direct one-off flags (`--url`, `--environment-id`, `--tenant-id`, `--username`, `--password`, `--client-id`, `--client-secret`) bypass config for a single command.

---

## Channel policy

Top-level `channels.<client>` can clamp what that client is allowed to do:

- `access`: `readonly` or `full` — the most restrictive of channel vs auth wins for mutates.
- `context` / `contexts` — default and allowlist for that client (`tenant` / `tenants` dual-read).
- `deny_builtins` / `allow_builtins` — also valid on **contexts.** and **auths.** (same list/glob form). Each layer is independent: a builtin is blocked if **any** layer matches deny and that layer’s allow does not match. Channel allow cannot unblock a context deny.

MCP already denies `jamf_*` on the channel layer even when the YAML list is empty. `deny_builtins: ["*"]` is deny-all at that layer (explicit allowlist when paired with `allow_builtins`). Quote `"*"` so YAML does not treat it as an alias. Blocked builtins raise; they are a gap, not a cue to invent HTTP from the schema.

The IDE uses the `ide` client channel (there is no `serve` or `notebook` channel).

```sh
buddy cred set channel mcp --access=readonly --context=sandbox
buddy cred set channel mcp --deny-builtins=computer_erase
buddy cred set channel mcp --allow-builtins=*
buddy cred set context prod --deny-builtins=* --allow-builtins=policies,policy
buddy cred set auth mcp-ro --type=oauth --jamfpro=prod --client-id=… --deny-builtins=computer_erase
buddy cred use --mcp sandbox
```

---

## Org defaults (macOS managed preferences)

Buddy can load config from a Jamf Application & Custom Settings (or MCX) payload for preference domain **`virtualpete.buddy.cli`** (the same reverse-DNS as the signed binary / Keychain service). This is org **defaults**, not enforced policy: `~/.config/buddy/config.yaml` overlays and wins per context, auth, and channel name.

Search order:

1. `BUDDY_MANAGED_PLIST` if set (tests and non-Darwin drop-in; a missing path disables discovery)
2. Else on macOS: `/Library/Managed Preferences/virtualpete.buddy.cli.plist`, then `~/Library/Managed Preferences/virtualpete.buddy.cli.plist` (user-managed overlays computer)
3. User `config.yaml`

Typical profile content: `context.default`, `contexts`, and `channels` policy. Auths (`clientId`, usernames, secrets, even auth *names*) are per-user: **accepted** if present in the plist, but `buddy setup managed-plist` omits the whole `auths` map. Prefer `buddy setup secrets` for client IDs and secrets (managed plists are typically world-readable).

`buddy setup secrets` creates an auth for any context that has none, fills instance/platform coverage gaps (instance URL with no instance auth, or platform identity with no platform oauth), then prompts for missing client IDs and store secrets. `buddy cred add` is the one-shot guided path for a named context+auth. `buddy setup managed-plist [path]` dumps contexts and channel policy as XML plist for Jamf ACS.

`buddy cred list` prints a `managed: …` line when prefs contributed. `buddy cred status` is the coverage matrix. Deleting a managed context or auth from YAML warns: it comes back on the next load until the profile changes or you keep a local override.

---

## Where secrets live

Keep production passwords and OAuth client secrets **out of YAML**. Inline secrets are fine for local tests only.

Resolution order: **inline config → vault → error**. Vault keys are **auth names**.

Passwords and client secrets are stored in **`~/.config/buddy/secrets.enc`** (AES-256-GCM) next to `config.yaml`. A **random 256-bit data key (DEK)** decrypts the vault. After the DEK is loaded once into **process memory**, further secret reads in that process do not call Keychain again (no PBKDF2). A **recovery passphrase** is only for `cred backup` / `restore` and for unwrapping that DEK onto a new binary or Mac.

| `BUDDY_SECRET_STORE` | Platform | Data key (DEK) | Recovery passphrase |
|----------------------|----------|----------------|---------------------|
| (default) | macOS (`make buddy`) | Keychain, service `buddy`, account `dek`, via `security(1)` | same service, account `master` |
| (default) | macOS (`make buddy-darwin-arm64`) | Keychain, service `virtualpete.buddy.cli` (label Buddy), account `dek`, `AfterFirstUnlockThisDeviceOnly` + LocalAuthentication presence gate (SecAccessControl when entitled), via Security.framework | same service, account `master` (no presence ACL) |
| (default) | Linux / Windows | `~/.config/buddy/dek` (mode `0600`) | `~/.config/buddy/master` (mode `0600`; Windows: `%APPDATA%\buddy\…`) |
| `file` | any | Same as the Linux DEK file | Same as the Linux master file |
| `pass` | Unix | `pass show buddy/dek` | `pass show buddy/master` |
| `keychain` | macOS only | Explicit Keychain (same backend as the binary: CLI or SecItem) | same |

Escape hatches (never written to disk as config):

- `BUDDY_VAULT_PASSPHRASE` — overrides the recovery passphrase for CI/tests. If the value is a tagged DEK (`dek:v1:…`), it overrides the data key instead (skips Keychain).
- `BUDDY_VAULT_UNLOCK=after-first-unlock` — on signed Darwin only, store/read the DEK **without** `userPresence` (legacy accessibility; useful for CI). Default is presence-gated.

**Default Darwin binary (`make buddy`)**

- Writes Keychain items with `security -i` (secret on stdin, not argv) under service `buddy`
- Convenient for development (no CGO, survives `go build`)
- ACL is on `/usr/bin/security`, so any process running as you can read the same items
- No Touch ID / presence gate (Linux/Windows-style file or CLI Keychain strength)

**Signed release (`make buddy-darwin-arm64`, build tag `secitem`)**

- Uses Security.framework so the ACL is buddy’s Developer ID
- Upgrades keep working as long as the Team ID and identifier stay the same
- Separate Keychain service (`virtualpete.buddy.cli`, Keychain Access label **Buddy**) — never shares ACL or items with `dist/buddy`; the vault file is shared
- **Presence unlock:** ideally Keychain `SecAccessControl` `userPresence`. A bare Developer ID CLI **cannot** store that ACL today (`errSecMissingEntitlement` — needs `keychain-access-groups` + provisioning profile / `.app` packaging). Buddy falls back to **LocalAuthentication** (`evaluatePolicy` device-owner) once per process before releasing the DEK into memory, then reads an `AfterFirstUnlockThisDeviceOnly` Keychain item
- First vault use in a process loads the DEK into memory (**process session**): CLI, `buddy mcp`, and `buddy serve` unlock lazily on the first vault-backed secret resolve. A valid token cache skips the vault. `cred status` / `cred store` alone do not unlock the vault (`cred status --unlock` or `--test` does).
- Escape: `BUDDY_VAULT_UNLOCK=after-first-unlock` skips the LocalAuthentication gate (and ACL attempts)
- Recovery `master` item stays non-presence so vault writes do not prompt for every save
- Unsigned `dist/buddy` cannot use a meaningful app ACL; presence prompts are a signed-release feature. An unlocked mcp/serve process still holds the DEK in RAM

A signed binary does not see a DEK stored by `dist/buddy`. Use the same recovery passphrase with `buddy cred set passphrase` against `dist/buddy-darwin-arm64` (unwraps the DEK from the vault file into the signed Keychain), or `cred backup` / `restore`. `make install-signed` puts Buddy.app in `~/Applications` and **symlinks** `~/go/bin/buddy` at `Contents/Helpers/buddy`. `buddy cred erase secrets` deletes the vault, the DEK, the recovery passphrase, the token-wrap key, token cache files, and leftover per-auth items from **both** Keychain services.

**Codesign**

- Requires Darwin/arm64. Signs with the first valid Developer ID Application from `security find-identity -v -p codesigning`; set `BUDDY_CODESIGN_IDENTITY` to override
- Identifier defaults to `virtualpete.buddy.cli` (`BUDDY_CODESIGN_IDENTIFIER`) and must stay stable so Keychain ACLs survive rebuilds
- The Keychain service is hardcoded to that reverse-DNS string (not read from the running binary), so overriding the identifier at sign time still stores items under `virtualpete.buddy.cli`
- Ad-hoc signatures are rejected

Older vault files used PBKDF2 with the recovery passphrase as the wrapping secret. The first CLI run after upgrade migrates them to a DEK (PBKDF2 stays on backup files and on the optional DEK wrap inside `secrets.enc`). Older installs stored one Keychain/file/pass item per secret. `buddy cred migrate` copies those into the vault (and can delete the old items).

```sh
buddy cred status                               # context × surface coverage (SECRET locked; no vault)
buddy cred status --unlock                      # classify SECRET as stored/missing (may Touch ID)
buddy cred add --context=sandbox --url=https://example.jamfcloud.com --client-id=… --client-secret=…
buddy cred set passphrase                        # create / rotate the recovery passphrase (backups)
buddy cred set secret sandbox-admin                  # password (basic)
buddy cred set secret sandbox-admin --client-secret  # OAuth
buddy cred migrate                                   # inline YAML + old per-auth store → vault
buddy cred backup ./buddy.creds                      # uses stored recovery passphrase by default; AES-GCM file
buddy cred restore ./buddy.creds                     # recovery passphrase; confirm each auth; new local DEK
buddy cred clean secrets                             # review / delete orphaned vault entries
buddy cred erase secrets                             # wipe vault + DEK + recovery passphrase + token wrap/cache
buddy cred delete auth sandbox-admin                 # also clears vault secrets
```

`buddy cred get secret` and the SECRET column on `cred status` are **presence only**. `buddy auth test` discards any cached token and mints a new one: missing secret / token POST failure is an error; probe 403 after a good token is `live=token-ok` (the secret works, the role lacks that probe).

`buddy cred backup` / `restore` encrypt contexts, auths (including vault secrets), and channel policy with a **recovery passphrase** (AES-256-GCM, PBKDF2-SHA256). The DEK and any biometric/Keychain material are never written into `buddy.creds`. Backup defaults to the stored recovery passphrase (`--passphrase=` overrides). Restore applies contexts and channels, then prompts once per auth unless `-y` (Enter/y writes that auth from the backup; `n` leaves the existing config entry). Auths not in the backup are kept. Secrets go back to the vault under a **new** local DEK. If no recovery passphrase is stored yet, restore saves the backup passphrase as the recovery phrase.

Config and tokens live under `~/.config/buddy` on macOS/Linux (`$XDG_CONFIG_HOME/buddy` when set), and `%APPDATA%\buddy` on Windows.

---

## How tokens are cached

Buddy reuses a bearer token until shortly before it expires (about 15 seconds of clock skew), then obtains a new one. There is no proactive keep-alive while the token is still valid. On CLI, the vault (Touch ID / presence unlock) is opened only when a token must be **minted or refreshed** — a still-valid disk cache under `~/.config/buddy/tokens/` skips the vault entirely. `buddy mcp` / `buddy serve` still unlock once at process start.

| Auth type | Token source | Refresh |
|-----------|--------------|---------|
| `basic` | `/api/v1/auth/token` (treated as ~18 minutes, ahead of Jamf’s 20-minute limit) | Keep-alive on HTTP 401 or `buddy auth token refresh` when a cached token is still usable; otherwise obtain |
| `oauth` (instance) | `/api/oauth/token`; expiry from `expires_in` | `buddy auth token refresh` replaces via client credentials; also on expiry or 401 |
| `platform` | `{gateway}/auth/token` (fallback `/oauth2/token`); ~900s | Same as oauth; no keep-alive |
| `command` | External command each time | `buddy auth token refresh` re-runs `runCommand`; not cached on disk |

Authenticated HTTP retries **once** after a refresh if Jamf returns 401.

Tokens are also written under `~/.config/buddy/tokens/` (mode `0600`) so later CLI invocations can reuse them. On macOS the ciphertext is AES-GCM wrapped with a random key in Keychain account `token-wrap` (no Touch ID / `userPresence`; not the vault DEK). Linux and Windows use a compiled-in obfuscation key — enough to discourage casual `cat`, not a secret store. Expired, URL-mismatched, or corrupt files are deleted on load. `command` auth skips the disk cache. `buddy cred erase secrets` deletes the wrap key and the token files.

Debug logs, `--trace`, and `--script-trace` pass through a redaction filter (JWTs, `Bearer` values, and fields such as `access_token`, `token`, and `client_secret`). `buddy auth token show --raw` still prints the bearer to stdout.

```sh
buddy auth test                         # ForceRenew + probe the active context/auth
buddy auth test --all                   # every covering auth × context × surface
buddy auth authorization [--all]        # cached GET /api/v1/auth details (privileges, access level)
buddy auth token show [--raw] [--context=C] [--user=A]
buddy auth token status [--all]         # disk cache metadata (no bearer)
buddy auth token drop [--all]           # invalidate cache (including still-valid files)
buddy auth token refresh [--raw]        # obtain / keep-alive / replace from cache state
buddy token direct --url=… --username=… --password=…   # one-shot mint, no config
buddy cred clean tokens                 # delete expired or corrupt disk cache files
```

`auth token refresh` uses the derived session (`--context` / `--user`) and the on-disk cache: **obtain** if there is no usable token, **refresh** (Jamf keep-alive) for basic when a cached token is still usable, **replace** for oauth/platform (client credentials) and command (`runCommand`). `--raw` on `auth token refresh` or `auth token show` prints only the bearer. `token drop` removes still-valid files; `cred clean tokens` only sweeps expired or corrupt leftovers. One-shot mint without config is `buddy token direct`.

Last `auth test` results (token ok / probe, and Jamf authorization details when `GET /api/v1/auth` succeeded: access level, privilege set, privileges) are cached under `~/.config/buddy/caps/` with the same 0600 + compiled-in obfuscation style (not the token wrap key). Buddy uses that cache when auto-picking among auths. Missing or stale cache is ignored (fail open). Legacy `type: oauth` + `surface: platform` migrates to `type: platform` on load.

---

## Instance vs Platform

Call sites still use instance-relative paths (`/api/v1/…`, `/JSSResource/…`). When the session is on a **platform** surface, buddy rewrites them (Pro → `/pro/vN/…`, Classic → `/proclassic/…` with the `/JSSResource` prefix stripped, Platform product APIs drop `/api`) and sends `X-Environment-Id` or `X-Tenant-Id`.

Classic follows a platform pin / auto-pick by default (`OpClassic` / `OpClassicWrite` → `/proclassic/…`, no `/JSSResource`). Auto-pick among mixed auths still prefers an instance auth when one is linked. Set `features.classic_instance_only: true` (or `BUDDY_CLASSIC_INSTANCE_ONLY=true`) to force **Classic `/JSSResource` only** onto instance even when the pin is platform. A gateway `403` means the OAuth client lacks Classic scopes.

That toggle does **not** apply to surfaces that are never published on the gateway. These always use the instance URL (a platform pin auto-picks an instance auth on the same context):

- **ACS** `/api/config-profiles/…` — Jamf Pro UI API; gateway POST is `404 page not found`
- **Undocumented Pro policy list** `GET /api/v1/policies` — not in published OpenAPI; gateway is `403`. Policy writes and detail stay Classic XML (`/JSSResource/policies`)
- **Schema discovery** `/api/schema` and Classic swagger — instance host only

```yaml
features:
  classic_instance_only: true    # optional lock; default false = Classic may use /proclassic/
```

Platform product commands (`buddy blueprint`, `device`, `device-group`, `ddm`, `benchmark`, and the matching builtins) need a platform-capable context + auth (`id`/`region` plus `type: platform`).

See [API endpoints](api-endpoints.md) for the rewrite table, schema discovery, and which Pro versions buddy uses (computer inventory is **v4**; computer groups stay **v3**).
