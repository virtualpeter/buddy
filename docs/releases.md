# Release notes

buddy did not keep a changelog before v12. This page is the user-facing list of notable changes per tagged release. Version strings come from git tags (`git describe`).

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
