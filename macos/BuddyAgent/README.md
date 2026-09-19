# Buddy Agent (macOS)

Optional menu-bar app that **owns its own Jamf workplaces and auths** — identity
in `agent-config.json`, secrets in its Keychain — and mints bearer tokens for the
`buddy` CLI over a unix-domain socket. buddy never pushes a secret to the Agent,
and the Agent never returns one: the CLI receives config (identity) or
`{access_token, expires_at}`.

Config is served **while the Agent is locked**; unlock gates minting only. When
buddy asks a locked Agent for a token it sets `allow_unlock`, and the Agent
raises the presence prompt itself while the CLI waits on the socket — no trip to
the menu bar mid-command. An empty or missing `config.yaml` on macOS uses a
running Agent in memory (the file stays absent). When the Agent is not running
and `features.use_agent` is off, buddy falls through to first-run setup / the
in-process `secrets.enc` vault.

It complements **Buddy.app**: the Swift shell at `Contents/MacOS/Buddy` (`macos/Buddy/`) hosts docs/IDE serve UI; this Agent owns config/secrets and mints tokens. The Agent installs no separate LaunchAgent — it nests under `Buddy.app/Contents/Library/LoginItems/` and can register **Open at Login** via `SMAppService`.

## Build and run

Two builds of the same sources, and the difference is not cosmetic.

```sh
make agent-app     # Developer ID .app → dist/BuddyAgent.app; presence: acl
make agent-dev     # unsigned SwiftPM  → .build/debug/BuddyAgent; presence: simulated
```

`make agent-app` drives [`BuddyAgent.xcodeproj`](BuddyAgent.xcodeproj) through
`xcodebuild archive` + `-exportArchive` with `method: developer-id`. Archive and
export rather than a plain build: with automatic signing only the export step
picks the Developer ID identity and embeds the provisioning profile. That
profile is the whole point — it authorizes `keychain-access-groups`, which is
what lets the Agent put a `userPresence` ACL on each secret. The build script
fails if the result is missing the profile, the entitlement, a hardened runtime,
or a secure timestamp.

The Xcode target uses a **synchronized folder group** on `Sources/BuddyAgent`,
so a new `.swift` file is picked up by both builds with no file-list edit.
`Package.swift` is the dev loop only; it is never shipped.

Version strings come from the same git tags as the CLI (`git describe` → wire
`status.version`; the tag's semver prefix → About / `CFBundleShortVersionString`).
`scripts/gen-agent-version.sh` writes `AgentVersion.generated.swift` and
`Version.xcconfig`; `make agent-dev` and `make agent-app` run it, and the Xcode
target has a **Generate Version** build phase for direct builds.

```sh
BUDDY_AGENT_ALLOW_UNSIGNED=1 macos/BuddyAgent/.build/debug/BuddyAgent    # dev run
open dist/BuddyAgent.app                                                # signed run
```

An unsigned Agent has no `keychain-access-groups` entitlement, so the data
protection keychain refuses it and secrets stay **in process memory** only
(cleared on Lock / Quit). `evaluatePolicy` answers pass/fail for unlock; there
is no login-keychain fallback and no migrate-on-first-signed-launch path. The
Status tab and `buddy agent status` both say which backend is in force. A signed
launch also silently deletes any leftover login-keychain items from earlier
spike builds (without reading them, so it does not prompt).

A signed Agent also enforces the peer check for real, which means an unsigned
`dist/buddy` is refused. Use `Buddy.app/Contents/Helpers/buddy`, or start the
Agent with `BUDDY_AGENT_ALLOW_UNSIGNED=1` while developing.

## Shipping

`make app` nests the signed Agent at
`Buddy.app/Contents/Library/LoginItems/BuddyAgent.app` — the `SMAppService`
location, for a later "Open at Login" — and **verifies rather than re-signs**
it. `codesign --force` there would re-sign without the Agent's entitlements and
quietly cost it the ACL; the outer signature seals nested bundles by hash, so
copying is enough. `make notarize` then staples `Buddy.app` and the pkg built
from it, given `BUDDY_NOTARY_PROFILE` and a Developer ID **Installer** identity
in `BUDDY_PKG_SIGN_IDENTITY`.

Nothing may rebuild the app between those two steps — that would drop the
ticket, so `notarize` packages the app on disk rather than re-entering
`make package`, and `build-pkg.sh` refuses an unstapled app while notarizing.
The wait on Apple is capped by `BUDDY_NOTARY_WAIT` (default `30m`); if it
expires, the submission is still live and `make staple` finishes the run once
Apple answers. Do not rebuild before stapling.

A padlock appears in the menu bar (`lock.fill` locked, `lock.open.fill`
unlocked). The menu opens with a **Buddy Agent** title (app icon), lock state,
Unlock / Lock, **Settings…**, **Open Buddy…** (launches Buddy.app when
found), **Open at Login** (`SMAppService.mainApp` — Apple's API for registering
this process to launch at login; System Settings approval when required),
**Help** (⌘?), **About Buddy Agent…**, and Quit. Unlock goes through the same
presence gate a token request does; the CLI still cannot unlock the Agent over
the socket, it can only ask the Agent to put up its own prompt.

`make agent-app` stamps `AppIcon.icns` from `internal/serve/buddy.png` (same
logo as Buddy.app) and the copyright string from the MIT LICENSE.

The window has four tabs. User-facing setup guide: `Resources/AgentHelp.md`
(in-app via **Help** in the menu bar or the **?** icon on the Setup tab).

- **Setup** — add, edit, and delete workplaces and auths. The secret
  field is write-only; rows show `stored` or `missing`, never a value. The
  key icon on an auth row mints or shows cached bearer metadata (when unlocked).
  **Import from buddy** takes either file an existing setup can provide:
  - *Config only* — the JSON from `buddy setup export --json`: workplaces and
    auth names, no secrets, so each secret is still typed in here.
  - *Config and secrets* — the encrypted file from `buddy cred backup`. The
    Agent asks for that backup's passphrase, decrypts it in memory
    (PBKDF2-SHA256 + AES-GCM, same envelope `buddy cred restore` reads), and
    writes each secret straight into its own Keychain. The passphrase is used
    once and never stored; the CLI is not involved.
  **Export for buddy** (beside Import) writes the same two formats: identity JSON
  for `buddy setup import`, or an encrypted cred backup for `buddy cred restore`.
- **Policy** — `features.classic_instance_only` and `channels` policy (draft until **Save**). MDM-locked fields are read-only. `features.use_agent` / `force_agent` are client-side only (buddy YAML, MDM, or env) and are never served on the wire. `force_agent` requires the Agent and ignores the rest of config.yaml; managed prefs still apply.
- **Log** — Activity log (config, mint, buddy socket, keychain diagnostics); also
  in Unified Log.
- **Status** — socket path, config path, config integrity, managed prefs, version,
  peer-check mode, presence mode, idle lock, unlock-prompt preference, warnings.

From the CLI side:

```sh
buddy agent status          # present / unlocked / version / peer check / counts / config_integrity
buddy agent config          # the document the Agent serves (no secrets)
buddy agent integrations    # what it owns, with secret presence
buddy agent tokens          # cached bearer metadata
buddy agent lock            # relock and drop cached bearers
buddy agent enable          # write features.use_agent: true
buddy agent disable         # write features.use_agent: false
buddy agent force           # write features.force_agent: true
buddy agent force --off     # write features.force_agent: false
BUDDY_NO_AGENT=1 buddy …    # kill switch: ignore the Agent entirely
```

An empty or missing `config.yaml` already takes workplaces and auths from a
running Agent (in memory; the file is not created). `buddy agent enable`
writes `features.use_agent: true` when you also have a local file, or when you
want a missing Agent to be a hard error (`BUDDY_USE_AGENT=1` is the env
override). `buddy agent disable` writes `false`. `buddy agent force` writes
`features.force_agent: true` (ignore the rest of config.yaml; MDM still
applies). That layer sits beneath
managed preferences and your own `config.yaml`, is never written to disk by
buddy from the Agent document, and its auths show `SECRET=agent` in
`buddy cred status`.

## Storage

| What | Where |
|---|---|
| Workplaces, auths, policy (identity only) | `~/Library/Application Support/Buddy/agent-config.json` (0600) |
| Secrets | Keychain generic passwords, service `virtualpete.buddy.agent`, account `<auth>/client_secret` or `<auth>/password` |
| Config seal | Keychain (HMAC key + signature, same presence ACL as secrets) |
| Socket | `~/Library/Application Support/Buddy/agent.sock` (directory 0700, socket 0600) |

Items are `kSecAttrAccessibleWhenUnlockedThisDeviceOnly`. A `userPresence`
`SecAccessControl` — the real biometric gate — needs the signed app target. If a
Keychain write fails (a build with no code signature at all), the Agent keeps
that secret in memory for the session and says so on the Status tab.

## Unlock

`UnlockGate` ([AgentUnlock.swift](Sources/BuddyAgent/AgentUnlock.swift)) is the
one presence path: the tray Unlock item and an incoming `token` with
`allow_unlock` both call it. It coalesces concurrent callers onto a single
prompt, uses a fresh `LAContext` per prompt with
`touchIDAuthenticationAllowableReuseDuration = 0` (otherwise the recent Mac login
satisfies the policy with no UI), and imposes the timeout LocalAuthentication
lacks by invalidating the context. A success unlocks the session; the session
then relocks on an idle timer, which also drops every cached bearer.

| Variable | Use |
|---|---|
| `BUDDY_AGENT_UNLOCK_TIMEOUT` | Dev override for unlock-prompt seconds (`0` disables buddy-triggered prompts). When unset, the Status tab preference applies. |
| `BUDDY_AGENT_IDLE_LOCK` | Dev override for idle relock seconds (`0` = never). When unset, the Status tab preference applies. |
| `BUDDY_AGENT_FAKE_PRESENCE` | `allow` / `deny` answers without a prompt, for unattended soaks; needs `BUDDY_AGENT_ALLOW_UNSIGNED=1` |

`status` reports the mode: `simulated` (LocalAuthentication, today), `acl`
(Keychain `userPresence`, once signed), or `unavailable` — a Mac with no
device-owner authentication, where the Agent fails closed and the tray is the
only way in.

## Wire protocol

Newline-delimited JSON. `BUDDY_AGENT_SOCK` overrides the path on both sides.

| Request | Unlock | Response |
|---|---|---|
| `{"op":"status"}` | no | `{"ok":true,"present":true,"unlocked":false,"version":"v12.0.0-9-ge63c95a","peer_check":"codesign","integration_count":1,"token_count":0,"presence":"simulated","idle_lock_seconds":900,"unlock_timeout_seconds":60,"config_integrity":"sealed","managed_prefs":"…"}` (optional) |
| `{"op":"config"}` | no | `{"ok":true,"config":{…}}` — config.yaml-shaped, no secrets |
| `{"op":"integrations"}` | no | `{"ok":true,"integrations":[{"context":…,"auth":…,"secret":"stored"}]}` |
| `{"op":"tokens"}` | no | `{"ok":true,"tokens":[{"auth":…,"base_url":…,"expires_at":…}]}` |
| `{"op":"token","auth":"…","base_url":"…","kind":"oauth\|platform\|basic","force":false,"allow_unlock":true}` | **yes** | `{"ok":true,"access_token":"…","expires_at":"RFC3339"}` |
| `{"op":"lock"}` | no | status |
| `{"op":"unlock"}` | — | `{"ok":false,"error":"denied"}` — tray only |

`allow_unlock` is what lets the Agent prompt; without it (or from a pre-0.3 CLI)
a locked Agent answers `locked` straight away, as it always did.

Failures are `{"ok":false,"error":"<code>","message":"…"}`. Codes:

| Code | CLI behaviour |
|---|---|
| `unsupported` | falls back to the in-process vault — unless the auth is Agent-owned, which fails |
| `locked` | command fails — buddy does not open the vault behind a running Agent |
| `unlock_declined` | command fails; the prompt was answered with no |
| `unlock_timeout` | command fails; the prompt went unanswered |
| `unlock_unavailable` | command fails; no prompt is possible, so unlock from the tray |
| `denied` | command fails (peer codesign rejected, or unlock attempted over the socket) |
| `bad_request`, `mint_failed` | command fails |
| `config_tampered` | command fails — `agent-config.json` no longer matches its Keychain seal |

## Peer check

On accept the Agent resolves the peer's audit token (PID as fallback) and
requires `anchor apple generic`, the Agent's own Team ID, and an identifier in
`virtualpete.buddy` / `virtualpete.buddy.cli`. Unsigned callers are rejected.

| Variable | Use |
|---|---|
| `BUDDY_AGENT_TEAM_ID` | Require this Team ID instead of the Agent's own (unsigned dev builds have none) |
| `BUDDY_AGENT_ALLOW_UNSIGNED=1` | Skip the peer check — local dev only, never ship |
| `BUDDY_AGENT_SPIKE_CLIENT_SECRET` | Dev-only fallback secret when nothing is stored for that auth; ignored on a signed Agent |

## Manual soak

Full checklist including the Go side: [`docs/auth.md`](../../docs/auth.md)
§ Optional Buddy Agent.

1. Start the Agent locked, add a workplace and auth in **Setup**, then
   `buddy agent integrations` → the row with `secret stored`, no prompt.
2. `buddy agent config` → the served document. With an empty `config.yaml` (or
   `features.use_agent: true`), `buddy cred status` shows that context with
   `SECRET=agent`.
3. `buddy auth token drop` then `buddy auth test` with the Agent locked → the
   Agent prompts; approve it and the token is minted, `buddy agent status` then
   shows `unlocked: true`. The vault is never opened either way.
4. Repeat and decline → `unlock declined at the presence prompt`, Agent still
   locked. `BUDDY_AGENT_UNLOCK_TIMEOUT=0` gives the plain `locked` error with no
   prompt at all.
5. Run with `BUDDY_AGENT_IDLE_LOCK=5`, unlock, wait → `buddy agent status` shows
   it relocked and its cached bearers gone.
6. Quit the Agent → with `use_agent` on (yaml/env), buddy aborts telling you
   to start it. With no flag and no `config.yaml`, the next command falls
   through to first-run setup. Vault-backed auths in a local file keep working.
7. `BUDDY_NO_AGENT=1 buddy auth test` → today's behaviour with the Agent up.

`BUDDY_AGENT_FAKE_PRESENCE=allow|deny` runs steps 3–5 unattended, but it is
dev-only and cannot prove the prompt works — do one real pass.

On `dist/BuddyAgent.app`, four more that only the signed build can show:

8. `buddy agent status` → `presence: Keychain userPresence`; the Status tab says
   "data protection keychain, presence ACL".
9. An unsigned `dist/buddy` → `buddy agent refused this buddy`, naming the
   helper inside `Buddy.app` and `BUDDY_AGENT_ALLOW_UNSIGNED=1`.
10. Mint, `buddy agent lock`, mint again → **two** prompts. Locking invalidated
    the authenticated `LAContext`, so the keychain genuinely wants presence
    again; on the unsigned build this only clears a flag.
11. `spctl -a -vvv -t install` on the stapled pkg, then install on a Mac that has
    never seen the app — quarantine is what ended the earlier Go entitlement
    experiment.
