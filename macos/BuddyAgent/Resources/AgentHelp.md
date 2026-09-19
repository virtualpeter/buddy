# Buddy Agent Help

Buddy Agent is a menu-bar app that **owns your Jamf workplaces, auths, and secrets** and mints bearer tokens for the `buddy` CLI over a local socket. The CLI never receives a secret — only config (identity) or `{access_token, expires_at}`.

## Quick start

1. **Launch** Buddy Agent (from Buddy.app’s Login Items copy, `dist/BuddyAgent.app`, or your dev build).
2. Click the **padlock** in the menu bar → **Settings…** (⌘,).
3. Add **workplaces** (org → environment → tenant) and **auths** (oauth, basic, or platform).
4. Store each auth’s **secret** in the auth editor (write-only; rows show `stored` or `missing`).
5. On a Mac with no `config.yaml`, run any buddy command — it uses this Agent in memory and does not write that file. A running Agent with no workplaces is an error (add them here). `buddy agent enable` writes `features.use_agent: true` so a *missing* Agent is a hard error; `buddy agent disable` writes `false` so empty config does not rediscover the Agent. `BUDDY_USE_AGENT=1` is the env override.
6. Run `buddy cred status` — Agent-owned auths show `SECRET=agent`.

Optional: **Open at Login** in the menu bar menu so the Agent starts with your session.

## Menu bar

| Item | Action |
|------|--------|
| Summary line | Lock state and auth count |
| Unlock / Lock Agent | Same presence gate as minting (⌘L when Unlock is shown) |
| Settings… | Setup, Policy, Log, and Status window (⌘,) |
| Open Buddy… | Launches Buddy.app when installed |
| Open at Login | Registers with System Settings → Login Items |
| Help… | This guide (⌘?) |
| About… | Version and license |
| Quit | Stops the Agent (⌘Q) |

The icon is **closed padlock** when locked, **open padlock** when unlocked.

## Settings window

### Setup tab

**Workplaces** — Org tree matching `buddy cred list`: organization → environment → product-tenant. Select a row to add children. Set a default context with the radio control.

**Auths** — One row per auth name:

- **oauth / basic** → Jamf Pro token minting
- **platform** → Jamf Account gateway; pick scope (environment, organization, tenant)
- Link each auth to one or more workplaces
- **Secret** — stored in the Mac Keychain (signed app) or in memory only (unsigned dev build)
- **Bearer** — click the key icon to mint (when unlocked) or view cached token expiry

**Import / Export**

- *Config only* — JSON from `buddy setup export --json` (no secrets; type secrets in the Agent)
- *Config and secrets* — encrypted backup from `buddy cred backup` / `buddy cred restore` (same format; passphrase asked once)

### Policy tab

Buddy YAML policy the Agent serves (same keys as `config.yaml`):

- `features.classic_instance_only` — force Classic API on Jamf Pro
- `channels.cli|mcp|ide` — default workplace, readonly access, workplace allowlist, `deny_builtins` / `allow_builtins`

Changes are held in a draft until you click **Save**; **Revert** discards unsaved edits. MDM-managed policy fields show a lock icon and cannot be edited here.

Per-workplace and per-auth deny/allow lists are edited in the Setup tab workplace and auth sheets (each sheet has its own **Save**).

Buddy still applies MCP’s runtime default deny of `jamf_*` unless you override it in channel or auth policy.

### Log tab

**Activity** log: config edits, token mints, buddy socket requests, and keychain diagnostics. Newest entries appear at the bottom. The same events are also in Unified Log (`subsystem == "virtualpete.buddy.agent"`).

### Status tab

Socket path, config file path, workplace and auth counts, **config integrity**, managed prefs (when MDM locks apply), peer-check mode, and presence mode.

**Config integrity** — `ok` / `sealed` / `missing` / `tampered` / `unavailable` (unsigned dev builds cannot seal). The Agent HMAC-seals `agent-config.json` in Keychain when you save. Minting requires unlock and a matching seal. If the file was edited outside the Agent, Status warns you — review Setup/Policy and **Save** to re-seal, or restore from backup.

**Auto-lock when idle** — After no minting for the chosen interval, the Agent relocks and drops cached bearers. Turn off only if you accept staying unlocked until you lock manually.

**Allow unlock prompts from buddy** — When on, a locked Agent can show Touch ID / password when buddy asks for a token (`allow_unlock`). When off, unlock only from the menu bar.

## Lock vs unlock

| State | Config served to buddy | Minting |
|-------|------------------------|---------|
| Locked | Yes | No — buddy may trigger an unlock prompt if enabled |
| Unlocked | Yes | Yes — bearers cached until expiry, lock, or idle timeout |

Lock (menu or idle timer) **clears all cached bearers** and ends the presence session. Secrets and the config seal remain in Keychain; only the session flag, token cache, and in-memory seal cache are cleared.

## Buddy CLI

```sh
buddy agent status          # present, unlocked, version, counts, config_integrity
buddy agent config          # document the Agent serves (no secrets)
buddy agent integrations    # auths with secret stored/missing
buddy agent tokens          # cached bearer metadata
buddy agent lock            # relock from the CLI
buddy agent enable          # write features.use_agent: true
buddy agent disable         # write features.use_agent: false
buddy agent force           # write features.force_agent: true
buddy agent force --off     # write features.force_agent: false
BUDDY_NO_AGENT=1 buddy …    # ignore the Agent for this command
```

An empty or missing `config.yaml` already uses this document when the Agent is running. `buddy agent enable` writes `features.use_agent: true` (never in the Agent document) so that layer stays in play with a local file and a missing Agent is a hard error. `buddy agent disable` writes `false`. `buddy agent force` writes `features.force_agent: true` (YAML, MDM, or `BUDDY_FORCE_AGENT`) — requires the Agent and ignores the rest of config.yaml; managed prefs still apply. `buddy agent force --off` writes `false`.

## Storage

| What | Where |
|------|--------|
| Workplaces, auths, policy (identity) | `~/Library/Application Support/Buddy/agent-config.json` |
| Secrets | Keychain, service `virtualpete.buddy.agent` |
| Config seal | Keychain (HMAC key + signature, same presence ACL as secrets) |
| Socket | `~/Library/Application Support/Buddy/agent.sock` |

## Troubleshooting

**Buddy says Agent refused / not present** — Start Buddy Agent; use the signed helper inside Buddy.app or a matching signed build.

**Secret shows missing** — Edit the auth and enter the secret, or import a cred backup.

**Mint fails when locked** — Unlock from the menu, or ensure “Allow unlock prompts from buddy” is on in Status.

**Keychain / memory warning on Status** — Unsigned dev builds keep secrets in memory only (cleared on lock/quit). Use `make agent-app` / `dist/BuddyAgent.app` for Keychain ACL + Touch ID on secrets.

**Config integrity missing or tampered** — Open Settings, review Setup/Policy, and **Save** (Policy or a workplace/auth sheet). Unlock first if minting still fails with a seal error.

**MDM locks on Policy or Setup** — Jamf managed preferences for `virtualpete.buddy.cli` override Agent fields at runtime; locked rows cannot be edited in the window.

**Orange warnings on Status** — Read the label; unsigned peer check, config tamper, and dev-only env vars are called out there.

For developer build, env overrides, and wire protocol details, see `README.md` in the Buddy Agent source tree.
