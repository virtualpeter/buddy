# Auth, secrets, and tokens

Buddy talks to Jamf using a **context** (a named workplace in config) plus an **auth** (which principal). Passwords and OAuth client secrets belong in the encrypted vault, not in `config.yaml`. Tokens are cached and refreshed automatically.

First-time install: [Getting started](getting-started.md). With an empty or missing config, buddy on macOS first looks for a running Buddy Agent and uses that document in memory (`config.yaml` stays absent). If the Agent is running but has no workplaces, buddy errors and asks you to add them in Buddy Agent. If the Agent is not there, any interactive Jamf command (or `buddy setup` / `buddy cred add`) runs the chooser — on a Mac, with a note suggesting Buddy Agent — and Linux/Windows keep that chooser as the first step: instance JSON or basic, Platform curl, a secrets-free quickstart file, or an org MDM profile. `buddy cred add --paste` accepts the JSON Jamf Pro shows when creating an API client, or the curl sample from a Platform API client. `buddy setup export` writes that YAML from a working config; `buddy setup import` / `cred add --from=` load it. With an org MDM profile, use `buddy setup secrets`. The docs UI **config** page is a workplace tree (rename with the pencil, delete with the trash — parents that still have children are blocked). Secrets stay in the terminal vault. YAML field reference: [README](../README.md#configuration). Jamf URLs and Classic vs Pro vs Platform: [API endpoints](api-endpoints.md).

Maintenance: `buddy cred status` and `buddy cred list` walk workplaces as **org → environment → product-tenant** (ORG / ENVIRONMENT / CONTEXT). SURFACE on status is instance URL vs Platform gateway — not Jamf Account scope (that lives on the auth). Account UUIDs stay on `buddy cred get`. Covering auths, whether a secret is **locked** / **stored** / missing, and the last `auth test` result. Default status does not open the vault (`SECRET=locked`); `--unlock` or `--test` classifies stored vs missing. Gaps are workplaces that have an identity but no covering auth (or no secret after unlock). Low-level `buddy cred set context|auth|secret` still works (`tenant` is a dual-read alias). `buddy cred rename context|auth` renames a workplace or auth (vault secrets move with the auth). If `config.yaml` will not parse, `buddy cred restore-config` copies the last-good `config.yaml.ok`. Loading also rewrites leading tabs to spaces.

---

## Contexts and auths

Jamf **scope** is organization / environment / tenant — which Account header, if any, an API client sends. It lives on the **auth** (`scope: organization|environment|tenant`). Buddy **context** is only a named workplace you pass to `--context` (dual-read `--tenant`). Those are different words on purpose.

A context is a named buddy workplace (e.g. `sandbox`). The default path is **instance-only**: a Jamf Pro URL and an `oauth`/`basic` auth. On disk that lives under nested `orgs:` (an invented organization named `default` until you rename it). No Account environment, region, or platform auth is required.

When you adopt Jamf Account / Platform, Jamf’s hierarchy is **organization → environment → product tenant**. Nested `orgs:` YAML is the on-disk shape; load expands it into the same flat workplace map `--context` already uses. Auths stay in `auths:`. Old flat `contexts:` files are folded into `orgs:` on load.

| Nested node | Workplace (`--context`) | What you get |
|-------------|-------------------------|--------------|
| `orgs.acme` | `acme` | Organization workplace: no region, no env/tenant ids. Tokens go to `https://us.api.jamfcloud.com` with **no** `X-Environment-Id` / `X-Tenant-Id`. `orgs.acme.id` is stored for humans / future Account APIs only (never sent as a header). |
| `environments.commercial` | `commercial` | Environment workplace: `region` + `id`, parent org `acme`. |
| `tenants.sandbox` | `sandbox` | Product-tenant workplace: `jamfpro.tenantId` + optional URL; inherits environment id + region; parent org `acme`. |

Org, environment, and nested tenant names share **one** workplace namespace. A leftover flat `contexts:` key with the same name overlays (instance URL + nested tenant ids = today’s hybrid) and is then folded into the tree. A name used twice in the nested tree, or an overlay with two different URLs / environment ids, is an error.

Root dual-read `tenants:` is the old name for workplaces (one release). Nested `orgs.*.environments.*.tenants` is Jamf’s product-tenant list inside the orgs tree — not a second root `tenants:` meaning.

| Auth `scope:` | Header | What it is |
|---------------|--------|------------|
| `environment` (default for `type: platform`) | `X-Environment-Id` | Account environment. Prefer this for GA product APIs (blueprints, devices, …). Link with `contexts: [commercial]`. |
| `tenant` | `X-Tenant-Id` | One Jamf Pro / School / Protect / Security Cloud product tenant. Link with `jamfpro:` or `contexts:`. |
| `organization` | *(none)* | Organization-scoped Account APIs. Link with `contexts: [acme]`. **Only the organization workplace** — not inherited by environments or product tenants. Product builtins still need an environment- or tenant-scoped auth. |

Copy ids from Jamf Account → Integrations. Do not use a jamfcloud.com hostname or the buddy context name as an environment/tenant UUID. There is no separate org hostname: `api.jamfcloud.com` does not resolve. Organization-scoped clients use `https://us.api.jamfcloud.com/auth/token` and send **no** environment or tenant header. Environment- and tenant-scoped clients use `https://{region}.api.jamfcloud.com` plus `X-Environment-Id` or `X-Tenant-Id`. Some Jamf UI curl samples still show the non-resolving host; paste treats that as organization-against-US. `buddy cred add --paste --context=acme` with a US curl and blank environment id still creates an org workplace named `acme` (the same object as `orgs.acme`). Beta `*.apigw.jamf.com` hosts are refused. Beta credentials cannot be migrated — register a new integration.

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

`credentials:`, a bare string `tenant:`/`context:`, `defaults:`, and `allow_auths:` are rejected. Dual-read `tenants:` / `tenant.default` / auth `tenants:` for one release. Loading config rewrites those keys to `context.default` / `jamfpro:` / auth `contexts:` + `scope:`, and folds leftover flat `contexts:` into `orgs:`.

Instance-only example (invented org `default` — rename it in the config page or with `buddy cred rename context default acme`):

```yaml
context:
  default: sandbox
orgs:
  default:
    environments:
      sandbox:
        url: https://example.jamfcloud.com
auths:
  sandgod:
    type: oauth
    jamfpro: sandbox
    clientId: …
```

Optional Platform tree (expands to workplaces `acme`, `commercial`, and `sandbox`):

```yaml
orgs:
  acme:
    id: 00000000-0000-0000-0000-000000000000   # optional Account org UUID; never sent as a header
    environments:
      commercial:
        id: 11111111-1111-1111-1111-111111111111
        region: us
        tenants:
          sandbox:
            tenantId: 22222222-2222-2222-2222-222222222222
            url: https://example.jamfcloud.com          # optional
auths:
  acme-org:
    type: platform
    scope: organization
    contexts: [acme]
    clientId: …
  env-plat:
    type: platform
    scope: environment
    contexts: [commercial]
    clientId: …
  sandgod:
    type: oauth
    jamfpro: sandbox
```

Old flat `contexts:` files still load once and are rewritten into `orgs:`. `buddy cred set context acme` (no `--region`) creates an organization workplace. `--context=sandbox` still sees `acme-org` for Account org APIs when the child has `org: acme` (derived from `orgs:`).

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

Typical profile content: `context.default`, `contexts` / `orgs:`, and `channels` policy. Auths (`clientId`, usernames, secrets, even auth *names*) are per-user: **accepted** if present in the plist, but `buddy setup managed-plist` omits the whole `auths` map. The same identity document is a mail-around YAML quickstart (`buddy setup export` writes it; `buddy setup import` / `cred add --from=` load it). Prefer `buddy setup secrets` for client IDs and secrets (managed plists are typically world-readable).

`buddy setup secrets` creates an auth for any context that has none, fills instance/platform coverage gaps (instance URL with no instance auth, or platform identity with no platform oauth), then prompts for missing client IDs and store secrets. `buddy cred add` is the one-shot guided path for a named context+auth. `buddy setup export [path]` dumps secrets-free YAML from a working config. `buddy setup import <file>` merges that YAML or an ACS plist, then runs secrets. `buddy setup managed-plist [path]` dumps the same identity as XML plist for Jamf ACS.

`buddy cred list` prints a `managed: …` line when prefs contributed. `buddy cred status` is the coverage matrix. Deleting a managed context or auth from YAML warns: it comes back on the next load until the profile changes or you keep a local override.

---

## Where secrets live

Keep production passwords and OAuth client secrets **out of YAML**. Inline secrets are fine for local tests only.

Resolution order: **inline config → vault → error**. Vault keys are **auth names**.

Passwords and client secrets are stored in **`~/.config/buddy/secrets.enc`** (AES-256-GCM) next to `config.yaml`. A **random 256-bit data key (DEK)** decrypts the vault. After the DEK is loaded once into **process memory**, further secret reads in that process do not call Keychain again (no PBKDF2). A **user-held recovery passphrase** wraps that DEK inside `secrets.enc` and encrypts `cred backup` / `restore` — it is **not** stored in Keychain/file.

| `BUDDY_SECRET_STORE` | Platform | Data key (DEK) | Recovery |
|----------------------|----------|----------------|----------|
| (default) | macOS (`make buddy`) | Keychain, service `buddy`, account `dek`, via `security(1)` | DEK wrap in `secrets.enc` (typed passphrase; leftover Keychain `master` is deleted once a wrap exists) |
| (default) | macOS (`make buddy-darwin-arm64`) | Keychain, service `virtualpete.buddy.cli` (label Buddy), account `dek`, `AfterFirstUnlockThisDeviceOnly` + LocalAuthentication presence gate (SecAccessControl when entitled), via Security.framework | Same wrap in `secrets.enc` |
| (default) | Linux / Windows | `~/.config/buddy/dek` (mode `0600`) | Wrap in `secrets.enc` (leftover `master` file deleted once wrap exists) |
| `file` | any | Same as the Linux DEK file | Same wrap model |
| `pass` | Unix | `pass show buddy/dek` | Wrap in `secrets.enc` |
| `keychain` | macOS only | Explicit Keychain (same backend as the binary: CLI or SecItem) | Wrap in `secrets.enc` |

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
- **Presence unlock:** ideally Keychain `SecAccessControl` `userPresence`. A bare Developer ID CLI **cannot** store that ACL today (`errSecMissingEntitlement` — needs `keychain-access-groups` + provisioning profile / `.app` packaging). Buddy falls back to **LocalAuthentication** (`evaluatePolicy` device-owner; no Mac-unlock reuse; unavailable policy fails closed) once per process before releasing the DEK into memory, then reads an `AfterFirstUnlockThisDeviceOnly` Keychain item
- First vault use in a process loads the DEK into memory (**process session**): CLI, `buddy mcp`, and `buddy serve` unlock lazily on the first vault-backed secret resolve. A valid token cache skips the vault. `cred status` / `cred store` alone do not unlock the vault (`cred status --unlock` or `--test` does).
- Escape: `BUDDY_VAULT_UNLOCK=after-first-unlock` skips the LocalAuthentication gate (and ACL attempts)
- **Cancel means locked:** if the DEK item exists but presence / LA fails or is cancelled, buddy does **not** fall back to the recovery passphrase. Recovery still works when the DEK is genuinely absent via typed `buddy cred set passphrase` or `BUDDY_VAULT_PASSPHRASE`
- **User-held recovery:** `cred set passphrase` writes a DEK wrap into `secrets.enc` only. Older Keychain/`master` (or `master` file) items are deleted once a wrap is present. `cred backup` / `restore` always require a typed passphrase (or `--passphrase=` / `BUDDY_VAULT_PASSPHRASE`) — never a machine-stored default
- Unsigned `dist/buddy` cannot use a meaningful app ACL; presence prompts are a signed-release feature. An unlocked mcp/serve process still holds the DEK in RAM
- Hard biometrics belong in the optional macOS Buddy Agent (Swift tray, below); the Go CLI stays soft LA + AFU DEK until then

### Optional Buddy Agent

An optional macOS menu-bar **Buddy Agent** (Swift, [`macos/BuddyAgent/`](../macos/BuddyAgent/)) **owns its own workplaces and auths**: identity in `agent-config.json`, secrets in its Keychain under service `virtualpete.buddy.agent`. buddy never sends a secret to the Agent, and the Agent never returns one: the CLI dials a **unix-domain socket** with a **Team-ID codesign peer check** and receives config (identity) or `{access_token, expires_at}`. Shipped inside **Buddy.app** at `Contents/Library/LoginItems/BuddyAgent.app` (optional **Open at Login**); no separate LaunchAgent plist. When the Agent is active, **`buddy serve` `/config`** is read-only in the browser or Buddy.app window — edit workplaces in Agent Settings instead. No new entitlements on `Helpers/buddy`.

**Config is served while the Agent is locked.** Workplaces and auth names are identity, not secrets, so resolving a session never prompts. Unlock gates **minting** only.

| Op | Needs unlock | Returns |
|---|---|---|
| `status` | no | present, unlocked, version, peer check, integration/token counts, presence mode, idle lock, `config_integrity`, optional `managed_prefs` |
| `config` | no | config.yaml-shaped document, secrets stripped |
| `integrations` | no | per-auth summary with `secret: stored/missing` |
| `tokens` | no | cached bearer metadata, never a bearer |
| `token` | **yes — the Agent prompts** | `access_token` + `expires_at` |
| `lock` | no | relocks and drops cached bearers |

**A locked Agent raises the prompt itself.** When buddy needs a bearer it sets `allow_unlock`, and the Agent puts up a LocalAuthentication prompt and holds the connection open until it is answered (60s, `BUDDY_AGENT_UNLOCK_TIMEOUT`; buddy waits 90s). A successful prompt unlocks the **session**, which then relocks after 15 minutes with no minting (`BUDDY_AGENT_IDLE_LOCK`, `0` = never) — locking also drops the Agent's cached bearers. Concurrent commands share one prompt. `BUDDY_AGENT_NO_PROMPT=1` opts out and a locked Agent simply fails, which is the pre-0.3 behaviour. The tray `Unlock` item goes through the same gate.

**What the prompt actually buys depends on how the Agent was built.** `buddy agent status` reports it:

| `presence:` | Build | What the prompt does |
|---|---|---|
| `Keychain userPresence` | Developer ID app (`make agent-app`) | Authenticates against the `SecAccessControl` guarding the secret, so Touch ID **releases the keychain item**. The Agent holds that `LAContext` for the session; locking or the idle relock invalidates it, which genuinely revokes access |
| `LocalAuthentication, unsigned agent` | SwiftPM (`make agent-dev`) | `evaluatePolicy` returns pass/fail and the Agent chooses to honour it. Secrets stay **in process memory** only (cleared on Lock / Quit) — there is no login-keychain fallback |
| `unavailable` | either | No device-owner authentication on this Mac; the menu bar is the only way in |

The difference is the entitlement. A `userPresence` ACL only works on the **data protection** keychain, which on macOS needs `keychain-access-groups` — a restricted entitlement authorized by a provisioning profile. Xcode automatic signing embeds one; a `swift build` binary has none, so the Agent detects that at launch and keeps secrets in memory rather than failing open into the login keychain. The Status tab shows which backend is in use.

`buddy agent status | config [--json] | integrations | tokens | lock` inspects it (`buddy auth agent` is an alias of `agent status`). There is no `unlock` and no `push`: unlock is a tray action, and secrets are typed into the Agent window — or imported by it. The Agent's **Setup** tab **Import from buddy** reads two files an existing setup already has:

| File | Carries | In the Agent |
|---|---|---|
| `buddy setup export --json` | workplaces and auth names, no secrets | seeds the rows; each secret is typed in afterwards |
| `buddy cred backup <file>` | the same identity **plus** the secrets | asks for that backup's passphrase and writes the secrets into its own Keychain |

The Agent opens the backup itself (the `buddy cred restore` envelope: PBKDF2-SHA256 then AES-256-GCM), so no secret passes through the CLI or the socket. The passphrase is used once, in memory. `internal/buddy/cred_backup_test.go` pins the payload keys that reader depends on.

**After importing, hand the entries over.** `config.yaml` still holds the same contexts and auths, and the user file wins per name, so the local copy silently shadows the Agent's. `buddy cred status` says so, and `buddy agent prune` removes the duplicates — `--dry-run` lists them first, `--secrets` also deletes the vault copies the Agent has made redundant (that one opens the vault). A workplace stays until the workplaces under it are shared too, and anything the Agent does not serve is left alone. Run prune to completion and `config.yaml` shrinks to `features.use_agent: true`. If you never had a local file, it stays absent — a running Agent is enough.

**`features.use_agent`** (`buddy agent enable`, or `BUDDY_USE_AGENT=1`) folds the Agent's document into buddy's config as a layer **beneath** managed preferences and your own `config.yaml` — the user file still wins per name, and `SaveConfig` never writes the Agent's entries to disk. `buddy agent disable` writes `features.use_agent: false` so an empty file does not rediscover the Agent. **`features.force_agent`** (`buddy agent force`, MDM plist, or `BUDDY_FORCE_AGENT=1`) also requires the Agent, and **ignores the rest of `config.yaml`** (workplaces, auths, channels, other features). Managed preferences still overlay the Agent. MDM `force_agent: true` cannot be turned off in the user file (`buddy agent force --off` writes false locally). With `use_agent` or `force_agent` on:

- an absent Agent (or `BUDDY_NO_AGENT=1`) is a **hard error**, not a silent fallback to the local vault;
- Agent-owned auths show `SECRET=agent` in `buddy cred status`, and buddy never resolves a vault secret for them;
- `buddy cred set|delete|rename` refuse those contexts and auths, pointing at the Agent window;
- an otherwise empty `config.yaml` does not trigger first-run setup — a contactable Agent with no workplaces errors instead (`Buddy Agent needs config`).

Without the flag, a configured `config.yaml` does not dial the socket during load. An empty or missing file on macOS still asks a running Agent once and uses that document in memory without writing `features.use_agent`. A contactable Agent with no workplaces is the same error (add them in Buddy Agent). If the Agent is absent, first-run setup runs as before, with a note to use Buddy Agent. Linux and Windows have no Agent, so they keep the chooser. After that, the Agent is only consulted when a bearer is minted.

Order of precedence when a bearer is needed:

1. Valid disk token under `~/.config/buddy/tokens/` — the Agent is not dialled at all. **Agent-owned auths skip this step**: they always ask the Agent, and any file an older buddy left is deleted
2. Agent, when `Enabled()` (macOS, `BUDDY_NO_AGENT` unset) and the credential has no inline secret
3. In-process `secrets.enc` vault, as before

**A bearer the Agent minted is never written to disk.** It lives in that one buddy process, so locking the Agent stops the next command instead of leaving buddy a usable copy for the rest of the token's life. The Agent keeps its own cache, so repeat commands cost a socket round trip, not a Jamf one. A 401 (or `buddy auth token refresh`) goes back to the Agent with `force`, bypassing that cache, rather than falling through to the vault for a secret buddy may not hold.

| Agent result | buddy |
|---|---|
| Socket absent | falls back to the vault — unless the auth is Agent-owned, which fails |
| `unsupported` (the Agent does not know that auth) | same: fallback, or fail when Agent-owned |
| `locked` (prompting opted out or a pre-0.3 Agent) | **fails** — a running Agent is never bypassed by unlocking the local vault |
| `unlock_declined` | fails; the prompt was answered with no |
| `unlock_timeout` | fails; nobody answered the prompt |
| `unlock_unavailable` | fails; this Mac cannot prompt, so unlock from the menu bar |
| `denied` / `bad_request` / `mint_failed` | fails |
| `config_tampered` | fails — `agent-config.json` was edited outside the Agent or no longer matches its Keychain seal |

| Variable | Effect |
|---|---|
| `BUDDY_USE_AGENT` | Overrides `features.use_agent` (`1`/`0`); `buddy agent enable` / `disable` write the YAML flag |
| `BUDDY_FORCE_AGENT` | Overrides `features.force_agent` (`1`/`0`): require the Agent and ignore `config.yaml` (MDM still applies). `buddy agent force` / `force --off` write the YAML flag |
| `BUDDY_NO_AGENT=1` | Ignore the Agent entirely; bit-identical to the pre-Agent behaviour |
| `BUDDY_AGENT_SOCK` | Socket path override (both sides); default `~/Library/Application Support/Buddy/agent.sock` |
| `BUDDY_AGENT_NO_PROMPT=1` | buddy never asks the Agent to prompt; a locked Agent just fails |
| `BUDDY_AGENT_UNLOCK_TIMEOUT` | Agent-side seconds the prompt stays up (default 60, `0` disables prompting) |
| `BUDDY_AGENT_IDLE_LOCK` | Agent-side seconds of no minting before it relocks (default 900, `0` = never) |
| `BUDDY_AGENT_FAKE_PRESENCE` | `allow`/`deny` answers without a prompt, for soaks; dev Agents only (`BUDDY_AGENT_ALLOW_UNSIGNED=1`) |

**Building and shipping it.** `make agent-app` archives and exports `dist/BuddyAgent.app` with Developer ID automatic signing, then refuses the result unless it carries a hardened runtime, a secure timestamp, an `embedded.provisionprofile`, and the `keychain-access-groups` entitlement — without those the Agent would silently lose the presence ACL. `make app` nests that bundle at `Buddy.app/Contents/Library/LoginItems/BuddyAgent.app` (the `SMAppService` location, for a later "Open at Login") and **verifies rather than re-signs** it: `codesign --force` there would strip the profile. The pkg carries it with no extra `/Applications` entry and no postinstall change. `make agent-dev` is the unsigned SwiftPM loop over the same sources.

`make notarize` submits and staples in the order that leaves both artefacts stapled — `Buddy.app` first, then a pkg built from the stapled app. It needs `BUDDY_NOTARY_PROFILE` (`xcrun notarytool store-credentials`) and `BUDDY_PKG_SIGN_IDENTITY` (a **Developer ID Installer** identity; the Application one will not sign a pkg). With `BUDDY_NOTARY_PROFILE` set, `make release` notarizes instead of just packaging, and the `--timestamp=none` fallback in the signing script becomes a hard failure rather than a quietly un-notarizable build.

That order is load-bearing, so it is enforced rather than assumed: `notarize` packages the app **on disk** instead of re-entering `make package`, because `sync-docfs` is phony and a recursive make would rebuild `Buddy.app` and discard the ticket just stapled to it. `build-pkg.sh` refuses to package an unstapled app whenever `BUDDY_NOTARY_PROFILE` is set, which catches that class of mistake instead of shipping a pkg whose inner app needs a network round trip on first launch.

Apple's queue is unpredictable — the same build can clear as a zipped app in seconds and sit for hours as an installer package — so the wait is bounded by `BUDDY_NOTARY_WAIT` (default `30m`). A timeout is "not yet", not a rejection: the submitted file stays valid, and `make staple` finishes the run once Apple answers. `stapler` fetches the ticket by cdhash, so that needs no submission id and no resubmission, but it only works on the exact files that were submitted — rebuilding first invalidates the ticket. A genuine rejection is reported separately, with `notarytool log` output inline.

Still out of scope: `SMAppService` login-item registration and an "Open at Login" toggle, auto-starting a stopped Agent, an update channel, and sandboxed / App Store distribution.

**Manual soak** (Agent running from `dist/BuddyAgent.app`, or `make agent-dev` for the unsigned path):

1. Add a workplace and auth in the Agent **Setup** tab while it is locked, then `buddy agent integrations` → the row with `secret stored`, no prompt
2. `buddy agent config` → the served document; with an empty `config.yaml` (or `features.use_agent: true`), `buddy cred status` shows the same context with `SECRET=agent`
3. `buddy auth token drop` then `buddy auth test` with the Agent locked → the Agent prompts, and approving it mints without touching the tray; `buddy agent status` then shows `unlocked: true`
4. Decline the prompt → `unlock declined at the presence prompt`, the Agent stays locked, and the vault was never opened
5. `BUDDY_AGENT_NO_PROMPT=1 buddy auth test` against a locked Agent → `buddy agent is locked`, with no prompt at all
6. `BUDDY_AGENT_IDLE_LOCK=5`, unlock, wait → `buddy agent status` shows it relocked and its cached bearers gone
7. Quit the Agent → with `use_agent` on (yaml/env), buddy aborts with "the Buddy Agent is not running". With no flag and no `config.yaml`, the next command falls through to first-run setup. Vault-backed auths in a local file keep working.
8. `BUDDY_NO_AGENT=1` → same abort with `use_agent` on; without the flag, the vault path with the Agent untouched

Unattended runs of steps 3–6 can use `BUDDY_AGENT_FAKE_PRESENCE=allow|deny` instead of a finger, but that hook is dev-only and cannot prove the prompt works — do one real pass by hand.

On a signed Agent, four more that only signing can demonstrate:

9. `buddy agent status` → `presence: Keychain userPresence`, and the Status tab says "data protection keychain, presence ACL"
10. An unsigned `dist/buddy` against it → `buddy agent refused this buddy`, naming `Contents/Helpers/buddy` and `BUDDY_AGENT_ALLOW_UNSIGNED=1`. Use the helper inside `Buddy.app`
11. Mint, `buddy agent lock`, mint again → two prompts. The first `LAContext` was invalidated, so the second read genuinely needs presence again rather than re-checking a flag
12. `spctl -a -vvv -t install` on the stapled pkg, then install it on a Mac that has never seen the app — quarantine is what ended the original Go entitlement experiment, so it is worth proving once

Do not re-ship Go helper Keychain Sharing / Secure Enclave packaging as the primary unlock story.

A signed binary does not see a DEK stored by `dist/buddy`. Use the same recovery passphrase with `buddy cred set passphrase` against `dist/buddy-darwin-arm64` (unwraps the DEK from the vault file into the signed Keychain), or `cred backup` / `restore`. `make install-signed` puts Buddy.app in `~/Applications` and **symlinks** `~/go/bin/buddy` at `Contents/Helpers/buddy`. `buddy cred erase secrets` deletes the vault, the DEK, any leftover Keychain/file master item, the token-wrap key, token cache files, and leftover per-auth items from **both** Keychain services. Leftover `~/.config/buddy/dek.se` from an abandoned Secure Enclave experiment is unused and safe to delete.

**Codesign**

- Requires Darwin/arm64. Signs with the first valid Developer ID Application from `security find-identity -v -p codesigning`; set `BUDDY_CODESIGN_IDENTITY` to override
- Identifier defaults to `virtualpete.buddy.cli` (`BUDDY_CODESIGN_IDENTIFIER`) and must stay stable so Keychain ACLs survive rebuilds
- The Keychain service is hardcoded to that reverse-DNS string (not read from the running binary), so overriding the identifier at sign time still stores items under `virtualpete.buddy.cli`
- Ad-hoc signatures are rejected

Older vault files used PBKDF2 with the recovery passphrase as the wrapping secret. The first CLI run after upgrade migrates them to a DEK (PBKDF2 stays on backup files and on the optional DEK wrap inside `secrets.enc`). Older installs stored one Keychain/file/pass item per secret. `buddy cred migrate` copies those into the vault (and can delete the old items).

```sh
buddy cred status                               # org → env → tenant coverage (SECRET locked; no vault)
buddy cred status --unlock                      # classify SECRET as stored/missing (may Touch ID)
buddy cred add --context=sandbox --url=https://example.jamfcloud.com --client-id=… --client-secret=…
buddy cred add --paste --context=sandbox          # Jamf Pro JSON or Platform curl
pbpaste | buddy cred add --paste --context=sandbox -y
buddy cred add --from=./quickstart.yaml
buddy setup export ./quickstart.yaml
buddy setup import ./quickstart.yaml
buddy cred set passphrase                        # create / rotate user-held recovery wrap in secrets.enc
buddy cred set secret sandbox-admin                  # password (basic)
buddy cred set secret sandbox-admin --client-secret  # OAuth
buddy cred migrate                                   # inline YAML + old per-auth store → vault
buddy cred backup ./buddy.creds                      # typed passphrase (prompt / --passphrase=); AES-GCM file
buddy cred restore ./buddy.creds                     # typed passphrase; confirm each auth; new local DEK
buddy cred clean secrets                             # review / delete orphaned vault entries
buddy cred erase secrets                             # wipe vault + DEK + leftover keys + token wrap/cache
buddy cred delete auth sandbox-admin                 # also clears vault secrets
```

`buddy cred get secret` and the SECRET column on `cred status` (with `--unlock`) are **presence only**. `buddy auth test` discards any cached token and mints a new one: missing secret / token POST failure is an error; probe 403 after a good token is `live=token-ok` (the secret works, the role lacks that probe). On an **organization** workplace the probe is `GET /licensing/v1/licenses` (org from the token; no `X-Environment-Id` / `X-Tenant-Id`). 200 is `live=ok`; 403 is `live=token-ok` (need the licensing privilege on the org integration). Buildings is skipped — that Pro path needs a scope header. Use `--context` on an environment or product tenant to probe Pro.

`buddy cred backup` / `restore` encrypt contexts, auths (including vault secrets), and channel policy with a **typed passphrase** (AES-256-GCM, PBKDF2-SHA256). The DEK and any biometric/Keychain material are never written into `buddy.creds`. Backup never defaults to a machine-stored Keychain/file secret — you must prompt or pass `--passphrase=` (CI may use `BUDDY_VAULT_PASSPHRASE`). Restore applies contexts and channels, then prompts once per auth unless `-y` (Enter/y writes that auth from the backup; `n` leaves the existing config entry). Auths not in the backup are kept. Secrets go back to the vault under a **new** local DEK. If `secrets.enc` has no recovery wrap yet, restore wraps the DEK with the backup passphrase (still not stored in Keychain).

Config and tokens live under `~/.config/buddy` on macOS/Linux (`$XDG_CONFIG_HOME/buddy` when set), and `%APPDATA%\buddy` on Windows.

---

## How tokens are cached

Buddy reuses a bearer token until shortly before it expires (about 15 seconds of clock skew), then obtains a new one. There is no proactive keep-alive while the token is still valid. On CLI, the vault (Touch ID / presence unlock) is opened only when a token must be **minted or refreshed** — a still-valid disk cache under `~/.config/buddy/tokens/` skips the vault entirely. `buddy mcp` / `buddy serve` also unlock lazily on first vault-backed secret resolve (not at process start).

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

When the selected context is an **environment** workplace (no Jamf Pro URL) and the op cannot run there — no platform auth, `classic_instance_only`, or an instance-only path — buddy uses the first Jamf Pro product-tenant child (tree order) that has an instance URL and instance auth. Classic and published Pro stay on the environment when a platform auth can serve them (`/proclassic/`, `/pro/`). `buddy auth test` (`OpAny`) and Platform product APIs (blueprints, devices, licenses) do not walk down.

That toggle does **not** apply to surfaces that are never published on the gateway. These always use the instance URL (a platform pin auto-picks an instance auth on the same context, or a Jamf Pro child when the workplace is environment-only):

- **ACS** `/api/config-profiles/…` — Jamf Pro UI API; gateway POST is `404 page not found`
- **Undocumented Pro policy list** `GET /api/v1/policies` — not in published OpenAPI; gateway is `403`. Environment workplaces with no Jamf Pro child skip this probe and list via Classic (`/proclassic/policies` on the gateway). When a product-tenant child has instance auth, the v1 list runs on that child. Policy writes and detail stay Classic XML (`/JSSResource/policies`)
- **Schema discovery** `/api/schema` and Classic swagger — instance host only

```yaml
features:
  classic_instance_only: true    # optional lock; default false = Classic may use /proclassic/
```

Platform product commands (`buddy blueprint`, `device`, `device-group`, `ddm`, `benchmark`, and the matching builtins) need a platform-capable context + auth (`id`/`region` plus `type: platform`).

See [API endpoints](api-endpoints.md) for the rewrite table, schema discovery, and which Pro versions buddy uses (computer inventory is **v4**; computer groups stay **v3**).
