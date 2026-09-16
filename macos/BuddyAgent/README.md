# Buddy Agent (macOS)

Optional menu-bar app that **owns its own Jamf integrations** — workplace, auth,
and the secret, which lives in its Keychain — and mints bearer tokens for the
`buddy` CLI over a unix-domain socket. buddy never pushes a secret to the Agent,
and the Agent never returns one: the CLI receives config (identity) or
`{access_token, expires_at}`.

Config is served **while the Agent is locked**; unlock gates minting only. When
the Agent is not running and `features.use_agent` is off, buddy behaves exactly
as it does today (in-process `secrets.enc` vault).

Still SwiftPM-only: no Xcode app target, no Developer ID signing, and therefore
no `userPresence` biometric gate yet (unlock is a menu click). It does not
replace `Contents/MacOS/Buddy` and installs no LaunchAgent.

## Build and run

```sh
cd macos/BuddyAgent
swift build
BUDDY_AGENT_ALLOW_UNSIGNED=1 .build/debug/BuddyAgent    # unsigned dev run
```

A padlock appears in the menu bar (`lock.fill` locked, `lock.open.fill`
unlocked) with a state header, Unlock, Lock, **Open Buddy Agent...**, Copy
Socket Path, and Quit. Unlock is the only way to enable minting; the CLI cannot
unlock the Agent.

The window has three tabs:

- **Integrations** — add, edit, and delete workplace+auth pairs. The secret
  field is write-only; rows show `stored` or `missing`, never a value. **Import
  from buddy** takes either file an existing setup can provide:
  - *Config only* — the JSON from `buddy setup export --json`: workplaces and
    auth names, no secrets, so each secret is still typed in here.
  - *Config and secrets* — the encrypted file from `buddy cred backup`. The
    Agent asks for that backup's passphrase, decrypts it in memory
    (PBKDF2-SHA256 + AES-GCM, same envelope `buddy cred restore` reads), and
    writes each secret straight into its own Keychain. The passphrase is used
    once and never stored; the CLI is not involved.
- **Tokens** — bearers minted this session and when they expire. Lock clears them.
- **Status** — socket path, config path, version, peer-check mode, warnings.

From the CLI side:

```sh
buddy agent status          # present / unlocked / version / peer check / counts
buddy agent config          # the document the Agent serves (no secrets)
buddy agent integrations    # what it owns, with secret presence
buddy agent tokens          # cached bearer metadata
buddy agent lock            # relock and drop cached bearers
BUDDY_NO_AGENT=1 buddy …    # kill switch: ignore the Agent entirely
```

Set `features.use_agent: true` (or `BUDDY_USE_AGENT=1`) to make buddy take its
workplaces and auths from the Agent. That layer sits beneath managed preferences
and your own `config.yaml`, is never written to disk, and its auths show
`SECRET=agent` in `buddy cred status`.

## Storage

| What | Where |
|---|---|
| Integrations (identity only) | `~/Library/Application Support/Buddy/agent-config.json` (0600) |
| Secrets | Keychain generic passwords, service `virtualpete.buddy.agent`, account `<auth>/client_secret` or `<auth>/password` |
| Socket | `~/Library/Application Support/Buddy/agent.sock` (directory 0700, socket 0600) |

Items are `kSecAttrAccessibleWhenUnlockedThisDeviceOnly`. A `userPresence`
`SecAccessControl` — the real biometric gate — needs the signed app target. If a
Keychain write fails (a build with no code signature at all), the Agent keeps
that secret in memory for the session and says so on the Status tab.

## Wire protocol

Newline-delimited JSON. `BUDDY_AGENT_SOCK` overrides the path on both sides.

| Request | Unlock | Response |
|---|---|---|
| `{"op":"status"}` | no | `{"ok":true,"present":true,"unlocked":false,"version":"0.2.0","peer_check":"codesign","integration_count":1,"token_count":0}` |
| `{"op":"config"}` | no | `{"ok":true,"config":{…}}` — config.yaml-shaped, no secrets |
| `{"op":"integrations"}` | no | `{"ok":true,"integrations":[{"context":…,"auth":…,"secret":"stored"}]}` |
| `{"op":"tokens"}` | no | `{"ok":true,"tokens":[{"auth":…,"base_url":…,"expires_at":…}]}` |
| `{"op":"token","auth":"…","base_url":"…","kind":"oauth\|platform\|basic","force":false}` | **yes** | `{"ok":true,"access_token":"…","expires_at":"RFC3339"}` |
| `{"op":"lock"}` | no | status |
| `{"op":"unlock"}` | — | `{"ok":false,"error":"denied"}` — tray only |

Failures are `{"ok":false,"error":"<code>","message":"…"}`. Codes:

| Code | CLI behaviour |
|---|---|
| `unsupported` | falls back to the in-process vault — unless the auth is Agent-owned, which fails |
| `locked` | command fails — buddy does not open the vault behind a running Agent |
| `denied` | command fails (peer codesign rejected, or unlock attempted over the socket) |
| `bad_request`, `mint_failed` | command fails |

## Peer check

On accept the Agent resolves the peer's audit token (PID as fallback) and
requires `anchor apple generic`, the Agent's own Team ID, and an identifier in
`virtualpete.buddy` / `virtualpete.buddy.cli`. Unsigned callers are rejected.

| Variable | Use |
|---|---|
| `BUDDY_AGENT_TEAM_ID` | Require this Team ID instead of the Agent's own (unsigned dev builds have none) |
| `BUDDY_AGENT_ALLOW_UNSIGNED=1` | Skip the peer check — local dev only, never ship |
| `BUDDY_AGENT_SPIKE_CLIENT_SECRET` | Dev-only fallback secret when nothing is stored for that auth |

## Wrapping in an Xcode app target

SwiftPM is enough to run and iterate. Move to an Xcode **macOS App** target for
Developer ID signing, `userPresence` Keychain ACLs, and notarisation:

1. Xcode → File → New → Project → macOS → App, name `BuddyAgent`, location
   `macos/BuddyAgent/` (keep `Sources/BuddyAgent/` as the target's group).
2. Delete the template's `App.swift`/`ContentView.swift`; add the files from
   `Sources/BuddyAgent/` (rename `main.swift` only if you prefer `@main`).
3. Set the target Info.plist from [`Resources/Info.plist`](Resources/Info.plist)
   (`LSUIElement` = YES) and Code Signing Entitlements to
   [`BuddyAgent.entitlements`](BuddyAgent.entitlements).
4. Signing → your Team, Developer ID or Development. Bundle identifier
   `virtualpete.buddy.agent` (the peer allowlist covers the *callers*
   `virtualpete.buddy` / `virtualpete.buddy.cli`, not the Agent itself).
5. Run, then re-run the soak below with the signed build and no
   `BUDDY_AGENT_ALLOW_UNSIGNED`.

## Manual soak

Full checklist including the Go side: [`docs/auth.md`](../../docs/auth.md)
§ Optional Buddy Agent.

1. Start the Agent locked, add an integration in the window, then
   `buddy agent integrations` → the row with `secret stored`, no prompt.
2. `buddy agent config` → the served document. With `features.use_agent: true`,
   `buddy cred status` shows that context with `SECRET=agent`.
3. `buddy auth token drop` then `buddy auth test` → fails with
   `buddy agent is locked` (no Touch ID prompt, vault not opened).
4. Unlock from the tray → `buddy auth token drop` then `buddy auth test` →
   token minted by the Agent, still no vault prompt.
5. Quit the Agent → with `use_agent` on, buddy aborts telling you to start it;
   with it off, vault-backed auths fall back exactly as before.
6. `BUDDY_NO_AGENT=1 buddy auth test` → today's behaviour with the Agent up.
