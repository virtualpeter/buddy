# Release notes

buddy did not keep a changelog before v12. This page is the user-facing list of notable changes per tagged release. Version strings come from git tags (`git describe`). Maintainers add a section with `make tag`.

## v12.1.4

- Treat a missing or cleared config as first-run setup instead of a failed start.
- Add make tag and finish package/publish around notes and notarization.

## v12.1.3

"We regret the inconvenience"

No new product features. This release is documentation: install and first-run copy now point at the notarized macOS package and Buddy Agent ([Getting started](getting-started.md)).

## v12.1.0

"The one with an agent"

Since **v12.0.0**. Human docs: [Auth](auth.md) (Agent + vault recovery), [Getting started](getting-started.md), [BuddyScript syntax](buddyscript-syntax.md).

### Buddy Agent (macOS)

- Optional menu-bar **Buddy Agent** owns its own workplaces, auths, and secrets (Keychain service `virtualpete.buddy.agent`). buddy never sends a secret to it and never gets one back — only identity and minted bearers over the unix socket.
- Config and integrations are served **while locked**. Unlock (tray or a `token` request with `allow_unlock`) gates minting only. Idle relock drops cached bearers so `agent lock` stops the next process.
- `features.use_agent` / `BUDDY_USE_AGENT` folds the Agent document under managed prefs and `config.yaml`. `features.force_agent` / `BUDDY_FORCE_AGENT` / MDM `force_agent: true` does the same but skips the user file (MDM true is sticky). An absent Agent is then fatal.
- `buddy agent status|config|integrations|tokens|lock|prune` inspects and hands over shadowed user-file entries. `buddy agent enable|disable` writes `use_agent`; `buddy agent force [--off]` writes `force_agent`.
- Empty or missing `config.yaml` on macOS uses a running Agent in memory without writing `use_agent`. A contactable Agent with no workplaces errors until you add them there.
- Import identity from `buddy setup export --json`, or identity **and** secrets from a `buddy cred backup` file (the Agent decrypts that envelope itself).
- Signed `make agent-app` / nested LoginItem in Buddy.app is the `userPresence` ACL path. `make agent-dev` is unsigned (in-memory secrets, simulated presence). No auto-start or sandboxing yet.

### Buddy.app

- **Buddy.app** (`Contents/MacOS/Buddy`) is a SwiftUI shell (`macos/Buddy/`) that hosts `buddy serve --embed` (Docs, Config, IDE tabs; native appearance menu; Agent Settings shortcut). The app starts the nested Agent. The **notarized package** from GitHub Releases is the macOS install path (`/Applications/Buddy.app` + `/usr/local/bin/buddy`).
- `buddy serve --embed` (or `?embed=1`) strips in-page nav for embed hosts. When the Agent owns workplaces, `/config` is read-only (`config_readonly`) and Buddy.app hides the Config tab — edit identity in Agent Settings.

### Secrets

- Recovery passphrase is **user-held**: it wraps the DEK inside `secrets.enc` and encrypts `cred backup` / `restore`. It is not stored in Keychain or a `master` file (legacy `master` items are deleted once a wrap exists). Cancelled Touch ID / LA does not fall through to recovery.
- `cred backup` / `restore` always need a typed `--passphrase=` or a prompt (`BUDDY_VAULT_PASSPHRASE` for CI).

### Build and runtime

- Source builds need **Go 1.27+**. `uuid_new()` is still an RFC 9562 **v4** UUID (stdlib `uuid.NewV4`); `uuid_is` still requires the 36-character hyphenated form.
- Starlark: `bytes + bytes` works at runtime (not only on literals). Excessively nested brackets are rejected instead of crashing eval/LSP.
- Docs UI (`buddy serve`): goldmark past the link/image XSS advisory; chroma 2.27 for fenced highlighting (Try-it still reads the python source).
- MCP stdio SDK 1.8 (same protocol revision; transport hardening). XML sort uses etree 1.8.

### Upgrade notes

- Building from a checkout older than Go 1.27 will fail.
- If you still have a Keychain/file `master` recovery item, run `buddy cred set passphrase` once so the wrap lives in `secrets.enc` and the leftover item is dropped.
- Vault installs stay on the vault until you import into the Agent and `buddy agent prune`. `features.use_agent` / `BUDDY_USE_AGENT` is the explicit hand-over; the notarized **Buddy.app** starts the nested Agent, and an empty macOS config uses a running Agent without writing that flag.

## v12.0.0

Notable features since the v11 line (last tag **v11.4.1**).

### Contexts and Account organizations

- Workplaces are **contexts** (`context:` / `--context`). Nested `orgs:` is the on-disk shape (organization → environment → product tenant). Flat `contexts:` / `tenant:` still load and are rewritten. `--tenant` remains a dual-read alias for this release.
- Instance-only configs fold into `orgs.default.environments.<name>.url`. Rename `default` in the config tree (pencil) or `buddy cred rename context`.
- The config page is a tree (not blank-cell tables): pencil rename, trash delete (parents with children are blocked). `buddy cred restore-config` recovers `config.yaml.ok` after a YAML parse failure; leading tabs are rewritten to spaces on load.

### Account licenses

- `buddy license list` (alias `licenses`) and buddyscript `licenses()` list Jamf Account licenses (`GET /licensing/v1/licenses`).
- Organization `buddy auth test` probes that same GET (200 → `live=ok`; 403 → `live=token-ok`).
- From an environment or product-tenant child with `org:`, license list uses the parent organization workplace automatically. `auth test` stays on the selected workplace.

### Classic / Pro when the context is an environment

- Instance-only APIs (undocumented `GET /api/v1/policies`, ACS, schema) use the first Jamf Pro product-tenant child that has an instance URL and instance auth.
- Classic `/proclassic/` and published Pro `/pro/` stay on the environment when a platform auth can serve them. If there is no platform auth (or `classic_instance_only` is on), those calls use the child too.
- `buddy auth test` and Platform product APIs (blueprints, devices, licenses) do not walk down to a Pro child.

### Setup and Buddy.app

- First-run is a chooser: paste Jamf Pro JSON / Account curl, import a secrets-free YAML/plist, or fill secrets for an existing MDM profile. `buddy setup export` / `import` / `secrets` / `managed-plist` replace the old write-script / store-oauth / basic-setup helpers.
- `buddy cred add --paste` / `--from=` share that parser.
- `make install-signed` installs **Buddy.app** under `~/Applications` with `~/go/bin/buddy` as a symlink into the helper. The Finder stub no longer bounce-waits in the Dock. Vault unlock is lazy (mcp/serve and `cred status` do not prompt at start).

### CLI

- Unique command prefixes (`buddy ico list`) and single-dash long flags (`-context` is `--context`). Short flags are still not combinable.

### Upgrade notes

- Re-run `buddy setup` / `buddy cred add` if you relied on the removed first-run helpers.
- Prefer `--context`; `--tenant` remains a dual-read alias for this release.
- Instance-only (Jamf Pro URL + oauth/basic) is unchanged and enough. Account/Platform orgs are additive.

See [Auth](auth.md) and [Getting started](getting-started.md).
