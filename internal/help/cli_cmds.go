package help

// CLI command help strings shared by `buddy help` and the docs Command index.
const Version = `buddy version - Print version and exit

Usage:
  buddy version
  buddy --version

Notes:
  Same output as --version. Does not load config or contact Jamf.
`
const Auth = `buddy auth - Obtain and test API authentication

Usage:
  buddy [global-flags] auth <subcommand> [args...]

Subcommands:
  test [--all]                 Prove the secret (ForceRenew + probe); --all tests every tenant×surface×auth
  caps [--all]                 Print cached last-check / privileges for the current pin (or every cell)
  token [flags]                Print Authorization header or raw bearer token
  refresh [flags] [<token>]    Refresh a bearer token via keep-alive (instance basic auth)

Flags for 'test':
  --all                        Test every covering auth × tenant × surface (not just the default session)

Flags for 'caps':
  --all                        Dump every still-valid caps cache entry

Flags for 'token':
  --raw                        Print bearer token only (for scripts and token_command)
  --url=<url>                  Jamf Pro or Platform gateway base URL (direct auth)
  --region=<region>            Platform gateway region (direct auth; builds URL when --url omitted)
  --tenant-id=<tenant-id>      Platform API Gateway tenant id (direct auth; requires oauth)
  --username=<u>                 Username (direct basic auth)
  --password=<p>                 Password (direct basic auth)
  --client-id=<id>               OAuth2 client ID (direct auth)
  --client-secret=<s>            OAuth2 client secret (direct auth)
  --oauth2-creds-file=<path>     JSON file with client_id and client_secret

Flags for 'refresh':
  --url=<url>                    Jamf Pro base URL (defaults to --user credential URL)
  --token=<token>                Bearer token to refresh (or pass as positional arg)

Notes:
  auth test always discards the cached token and mints a new one. Exit 0 if the
  secret works (probe 403 is live=token-ok, not a bad secret). Exit non-zero if
  the secret is missing or the token POST fails. cred get secret is presence only.
  Last results are cached under ~/.config/buddy/caps/ for cred status and session pick.

Examples:
  buddy auth test
  buddy auth test --all
  buddy --user dev auth test
  buddy auth caps
  buddy auth token
  buddy auth token --raw
  buddy --user dev auth token --raw
  buddy auth token --raw --url=https://example.jamfcloud.com --username=admin --password=secret
  buddy auth token --raw --region=us --tenant-id=<tenant-id> --client-id=<id> --client-secret=<secret>
  buddy auth refresh eyJhbGciOiJIUzI1NiJ9...
  buddy --user dev auth refresh --token=eyJhbGciOiJIUzI1NiJ9...
`
const Cred = `buddy cred - Manage tenants, auths, secrets, and channel policies

Config is split into tenants (Jamf host / platform tenantId), auths (secret-bearing
principals linked to one or more tenants), and channels (per-client policy:
access, default tenant, tenant allowlist). Global --tenant selects a tenant
(auto-picks auth); --user / --auth pins a specific auth.

Usage:
  buddy cred <subcommand> [args...]
  buddy cred backup <file> [--passphrase=] [-y]
  buddy cred restore <file> [--passphrase=] [-y]

Subcommands:
  status [--test]                   Tenant × surface coverage (gaps, secret presence, last live check)
  add [auth-name] [args...]         Guided tenant + auth + secret (optional live test)
  list                              List tenants, auths, and channel policies
  get tenant <name>                 Show tenant details
  get auth <name>                   Show auth details (secrets redacted)
  get channel <cli|mcp|ide>
                                    Show channel policy
  get secret <auth>                 Show where secrets are stored (values hidden; presence only)
  set tenant <name> [args...]       Add or update a tenant
  set auth <name> [args...]         Add or update an auth (link to tenants)
  set channel <cli|mcp|ide> [args...]
                                    Set channel access / tenant / tenants allowlist
  set secret <auth> [args...]       Store password / client secret in the secure store
  delete tenant|auth|secret <name>  Remove a tenant (cascades sole-linked auths+secrets),
                                    auth (+secrets), or stored secrets
  clean secrets                     Review secret-store entries; prompt to delete orphans
  clean tokens                      Delete expired/corrupt files under ~/.config/buddy/tokens/
  erase secrets                     Wipe every secret from the store (and both macOS Keychain services)
  use [tenant]                      Show or set tenant.default
  use --cli|--mcp|--ide [tenant]
                                    Show or set channels.<client>.tenant
  store                             Show the active secret backend
  migrate [auth]                    Move inline config.yaml secrets into the secure store
  backup <file>                     Encrypt tenants, auths (with secrets), and channels to a file
  restore <file>                    Decrypt a backup; prompt once per auth (or -y)

Flags for 'status':
  --test                            Run auth test --all first (ForceRenew), then print the matrix

Flags for 'add':
  --tenant=<name>                   Existing or new tenant name
  --url=<url>                       Jamf Pro instance URL (creating a tenant)
  --region=<region>                 Platform region (creating a tenant)
  --tenant-id=<tenant-id>           Platform tenant id (creating a tenant)
  --surface=any|instance|platform   Surfaces this auth may use (default from tenant)
  --type=oauth|basic                Auth type (default oauth; platform requires oauth)
  --client-id=<id>                  OAuth client id
  --username=<user>                 Basic username
  --client-secret=<secret>          OAuth secret (non-interactive)
  --password=<password>             Basic password (non-interactive)
  --test                            Force a live auth test after save
  --no-test                         Skip the live test (default when -y / non-TTY)
  -y                                Non-interactive; fail if required fields are missing

Flags for 'set tenant':
  --url=<url>                       Jamf Pro instance URL (and/or platform gateway URL)
  --region=<region>                 Platform API Gateway region (us|eu|apac|…)
  --tenant-id=<tenant-id>           Platform tenant id (with region)

Flags for 'set auth':
  --type=command|basic|oauth        Auth type (required)
  --tenants=<a,b>                   Comma-separated tenant names this auth may use (required)
  --access=full|readonly            Default full; readonly preferred for reads
  --surface=any|platform|instance   Jamf API surface (default any; platform requires oauth)
  --channels=cli,mcp,ide|all
                                    Buddy clients that may use this auth (default all)
  --run=<cmd>                       Command auth
  --username=<user>                 Basic auth
  --password=<password>             Basic password inline (dev/test)
  --client-id=<id>                  OAuth client id
  --client-secret=<secret>          OAuth secret inline (dev/test)

Flags for 'set channel':
  --access=readonly|full            Clamp mutates for this client (omit = full)
  --tenant=<name>                   Default tenant for this client
  --tenants=<a,b>                   Optional tenant allowlist (empty clears)
  --deny-builtins=<a,b,jamf_*>      Block named buddyscript builtins (empty clears)

Flags for 'set secret':
  --password[=value]                Store basic-auth password (no-echo prompt if omitted)
  --client-secret[=value]           Store OAuth client secret
  --secret-from-stdin               Read the secret value from stdin

Flags for 'backup' / 'restore':
  --passphrase=<pass>               Encryption passphrase (prefer interactive prompt)
  -y                                Overwrite backup file / restore all auths without prompting

  Notes:
  cred status is the maintenance view (tenant × surface coverage). cred list is
  the raw dump. cred get secret / status SECRET column are presence only —
  'buddy auth test' proves the secret (ForceRenew). Last live results live in
  ~/.config/buddy/caps/ (same obfuscation style as tokens/).
  A platform clientId may list multiple tenants under --tenants.
  Old credentials: / string tenant: / defaults: / allow_auths: migrate on load.
  tenant.cli|mcp|serve|ide and the serve client channel are removed — use
  channels.<cli|mcp|ide> instead.
  macOS managed preferences (domain virtualpete.buddy.cli) overlay as org
  defaults; config.yaml wins per name. 'buddy cred list' notes managed sources.
  Deleting a managed tenant/auth warns; it returns on next load until the MDM
  profile changes. Fill secrets with 'buddy setup secrets' or 'buddy cred add'.
  cred backup writes AES-256-GCM (PBKDF2-SHA256) JSON; auths include store
  secrets. Restore applies tenants/channels, then prompts once per auth
  (and its secrets) unless -y. Enter/y writes that auth from the backup;
  n leaves the existing auth (or absence) in config. Auths not in the backup
  are kept. A backup with no auths restores tenants/channels only.
  Secrets go to the platform store.
  Secrets are keyed by auth name in the platform store.
  Deleting an auth also removes its password/client-secret from the store.
  Deleting a tenant deletes auths linked only to that tenant (with secrets) and
  unlinks multi-tenant auths; use 'buddy cred clean secrets' for leftover orphans
  or 'buddy cred erase secrets' to wipe the entire store (including Keychain).
  Token files under ~/.config/buddy/tokens/ are purged when expired (or via
  'buddy cred clean tokens'); still-valid entries are reused across CLI runs.
  Auth channels (cli/mcp/ide) default to all. Channel policy access and
  auth access both clamp mutates (most restrictive wins). Restrict MCP e.g.:
  channels.mcp.access=readonly and/or auth channels excluding mcp from full auths.
  channels.mcp.deny_builtins blocks named builtins (exact or trailing-* glob, e.g.
  jamf_*) so MCP agents cannot fall back to generic jamf_get/jamf_put; treat raises
  as coverage gaps.

Examples:
  buddy cred status
  buddy cred status --test
  buddy cred add --tenant=sandbox --url=https://example.jamfcloud.com --client-id=… --client-secret=…
  buddy cred set tenant sandbox --url=https://example.jamfcloud.com --region=apac --tenant-id=…
  buddy cred set auth platform-ro --type=oauth --client-id=… --tenants=sandbox --access=readonly --surface=platform --channels=mcp,ide
  buddy cred set auth sandbox-admin --type=basic --username=admin --tenants=sandbox --surface=instance --channels=cli,ide
  buddy cred set channel mcp --access=readonly --tenant=sandbox --tenants=sandbox
  buddy cred set channel mcp --deny-builtins=jamf_*,computer_erase
  buddy cred set secret platform-ro --client-secret
  buddy cred delete auth sandbox-admin
  buddy cred delete tenant sandbox
  buddy cred clean secrets
  buddy cred clean tokens
  buddy cred erase secrets
  buddy cred backup ./buddy.creds
  buddy cred restore ./buddy.creds
  buddy cred use sandbox
  buddy cred use --mcp sandbox
  buddy --tenant=sandbox policy list --filter=chrome
  buddy --user=sandbox-admin policy set …
`
const PolicyList = `buddy policy list - List policies

Usage:
  buddy policy list [--filter=<keyword>] [--ids] [--exclude-ids=1,2,3]

Flags for 'list':
  --filter=<keyword>    Match policy names (space-separated keywords all required).
                        Hyphenated forms of names are also tried. A single
                        trigger-like keyword (e.g. install-example-next) also
                        resolves via custom trigger lookup.
  --ids                 Print matching policy IDs only (one per line)
  --exclude-ids=1,2,3   Exclude specific policy IDs from results

Examples:
  buddy policy list
  buddy policy list --filter=chrome
  buddy policy list --filter="chrome install" --ids
  buddy policy list --filter=install-chrome-next
  buddy policy list --filter=chrome --exclude-ids=100,101
`
const PolicyGet = `buddy policy get - Read policy properties

Usage:
  buddy policy get <policy-id>
  buddy policy get id <trigger>
  buddy policy get enable <policy-id>
  buddy policy get xml <policy-id>

Subcommands:
  id <trigger>         Resolve custom trigger to policy ID (bare, pipeable)
  enable <policy-id>   Show enabled/disabled state
  xml <policy-id>      Output policy XML (PUT payload)

Notes:
  Plain get <policy-id> prints policy details as JSON (id, name, enabled,
  trigger, scope, packages, scripts).
  get id looks up the policy whose custom trigger (trigger_other) matches
  (with name heuristics as fallback). Prefer this over guessing IDs when
  scripting set package.

Examples:
  buddy policy get 42
  buddy policy get id install-chrome
  buddy policy get enable 42
  buddy policy get xml 42
`
const PolicySet = `buddy policy set - Modify policy properties

Usage:
  buddy policy set ss <policy-id> [version] [flags...]
        Flags: [--display-name=...] [--description=...] [--install-text=...] [--reinstall-text=...]
               [--force-view|--no-force-view] [--feature|--no-feature]
               [--self-service|--no-self-service] [--group-ids=1,2,3]
  buddy policy set scope <policy-id> all
  buddy policy set scope <policy-id> <group-id> [group-id...]  [--group-ids=1,2,3]
  buddy policy set enable <policy-id>
  buddy policy set disable <policy-id>
  buddy policy set trigger <policy-id> <trigger>
  buddy policy set name <policy-id> <new-name>
  buddy policy set frequency <policy-id> <frequency>
  buddy policy set icon <policy-id> <image-file>
  buddy policy set install <policy-id> <version> [--param=4,5,6]
  buddy policy set package <policy-id> <version> [flags]
  buddy policy set package <version> --trigger=<trigger> [flags]
        Flags: [--name=<name>] [--package=<pattern>]
               [--disable-source|--keep-source]
  buddy policy set xml <policy-id>

Notes:
  Source policy for 'set package':
    <policy-id>                 Use this Install/Cache policy directly
    --trigger=<custom-trigger>  Resolve by custom trigger instead of an ID
  Package lookup for 'set package':
    --name=<name>               Package name keyword for the version search
                                (default: derived from the source trigger,
                                e.g. install-<name>-<version>)
    --package=<pattern>         Explicit package name pattern (overrides --name)
  Disable source (shown in confirm plan):
    Default: disable only when creating a replacement that keeps the same
             custom trigger (unversioned). Versioned trigger change leaves
             the old policy enabled. Same version = in-place update (no disable).
    --disable-source            Always disable the source after create
    --keep-source               Never disable the source
  New triggers when creating: install|cache-<name>-<version> if the source
  trigger was versioned; otherwise the source trigger is reused (replacement).

Examples:
  buddy policy set enable 42
  buddy policy set disable 42
  buddy policy set scope 42 all
  buddy policy set scope 42 10 11
  buddy policy set name 42 "Install Chrome 121"
  buddy policy set trigger 42 install-chrome
  buddy policy set frequency 42 "Once per computer"
  buddy policy set install 42 121.0 --param=4,5
  buddy policy set package 42 121.0 --keep-source
  buddy policy set package 121.0 --trigger=install-chrome
  buddy policy set package 42 121.0 --name=chrome
  buddy policy set xml 42 < policy.xml
`
const PolicyCreate = `buddy policy create - Create a new policy

Usage:
  buddy policy create package <name> <version> [flags]
        Flags: [--action=install|cache] [--scope=all|<group-id>[,<group-id>...]]
               [--category=<name>] [--package=<pattern>]
  buddy policy create ss <trigger> <display-name> [--category=<name>] [--script-id=<id>]

Arguments:
  <name>     Package name keyword (used for the package search and to build
             the policy name and trigger)
  <version>  Package version

Notes:
  create package invents triggers: install → install-<name>-next;
  cache → cache-<name>-<version>.
  create ss defaults to no script (--script-id omitted / 0). Pass --script-id
  to attach a Jamf script (parameter4 = <trigger>). Scope is all computers.

Examples:
  buddy policy create package chrome 121.0
  buddy policy create package chrome 121.0 --action=install --scope=all
  buddy policy create ss install-chrome "Install Chrome" --category="Mac Apps"
  buddy policy create ss install-chrome "Install Chrome" --script-id=12
`
const PolicyClone = `buddy policy clone — removed

Versioned policy clone was retired. Prefer:

  • Change batches: step_clone_policy / step_set_package (see docs/change-batches.md)
  • buddyscript: policy_copy + policy_set_package / policy_set_ss / policy_set_install
`
const PolicyFind = `buddy policy find — removed

Keyword search moved to 'buddy policy list':

  buddy policy list [--filter=<keyword>] [--ids] [--exclude-ids=1,2,3]
`
const PolicyDelete = `buddy policy delete - Delete one or more policies

Usage:
  buddy policy delete <policy-id> [policy-id...]

Examples:
  buddy policy delete 42
  buddy policy delete 42 43 44
`
const Setup = `buddy setup - Bootstrap instance config; Jamf Pro role/client + instance oauth

Usage:
  buddy [global-flags] setup basic-setup [--tenant=] [--url=] [--name=] [--username=] [--password=] [-y]
  buddy [global-flags] setup secrets [-y]
  buddy [global-flags] setup managed-plist [path]
  buddy [global-flags] setup write-script [path]
  buddy [global-flags] setup store-oauth [--user=<auth>] [--tenant=<tenant>] [--client-name=buddy-cli] [--name=<auth>] [-y]

Notes:
  write-script and store-oauth target the Jamf Pro *instance* (API roles and
  integrations). Platform gateway oauth cannot create roles or clients.
  If you have no config yet, start with basic-setup (instance URL + basic auth).
  If an MDM profile already provides tenants, run setup secrets instead.

Subcommands:
  basic-setup
    Interactive bootstrap for the first tenant and a basic (username/password)
    instance auth. Stores the password in the platform secret store and sets the
    tenant as default when appropriate. Then prints next steps (auth test, and
    optional write-script → store-oauth for day-to-day oauth).

    Non-interactive: pass --tenant, --url, --username, and --password (and
    optionally --name for the auth; default <tenant>-admin).

  secrets
    For each tenant with no linked auth, prompt to create one (oauth by default).
    Then fill instance/platform coverage gaps (instance URL with no instance/any
    auth, or platform identity with no platform/any oauth). Then prompt for any
    missing client IDs / usernames and store missing passwords or OAuth client
    secrets in the platform secret store.
    Org MDM profiles typically ship tenants and channel policy. Auths in the
    plist (including client IDs and secrets) are accepted if present; prefer
    this command so they stay in the local store. 'buddy setup managed-plist'
    omits auths entirely. 'buddy cred status' shows remaining holes.

    Non-interactive / -y: report gaps and exit non-zero (use a terminal, or
    'buddy cred add' / 'buddy cred set auth' / 'buddy cred set secret').

  managed-plist [path]
    Write tenants and channel policy as XML plist to path, or stdout if path is
    omitted. Omits auths (names, client IDs, and secrets are per-user). Upload
    as Jamf Application & Custom Settings for preference domain
    virtualpete.buddy.cli. User ~/.config/buddy/config.yaml overlays managed
    prefs (user wins per tenant/auth/channel name).

  write-script [path]
    Writes a buddyscript script that creates a Jamf Pro API role with privileges
    needed for buddy and an API client (integration) scoped to that role.
    Default path is setup_buddy_role.buddy.

    Prerequisites: a tenant with an instance URL and an auth that can manage
    API roles/integrations (typically the basic auth from basic-setup). The
    script uses the active session (--tenant / --user).

    Run the generated script with:
      ./<path> -y
      buddy <path> -y
    Then rotate the instance client secret and store it as a buddy oauth auth
    with surface=instance:
      buddy setup store-oauth
      buddy --user <auth-name> auth test
    Or get the client secret once with:
      buddy client credentials <client-id>

    The script is idempotent for role "Buddy" and client "buddy-cli".

    If role creation fails (e.g. privilege names differ on your Jamf version), run
    "buddy role privileges" to see your instance's list and edit the script.

    The role may include destructive privileges (e.g. wipe computer, erase) so a
    single script can cover all buddy features. After setup, review the role in Jamf
    and remove API privileges you do not need before treating the auth as
    production-ready.

  store-oauth
    Rotates OAuth client credentials on a Jamf Pro *instance* API integration
    (invalidates the previous secret) and stores clientId + secret as a buddy
    oauth auth with surface=instance, linked to a tenant. The secret goes in the
    platform secret store (Keychain/file — not Platform API oauth).

    Matching: if an oauth auth already exists for the target tenant with the same
    clientId, that auth is updated in place. Otherwise you are prompted for a new
    auth name (default <tenant>-oauth, or buddy-oauth).

Flags for 'basic-setup':
  --tenant=<name>     Tenant name (default: prompt, or "default")
  --url=<url>         Jamf Pro instance URL
  --name=<auth>       Basic auth name (--auth= / --user= aliases; default <tenant>-admin)
  --username=<user>   Jamf username
  --password=<pass>   Jamf password (prefer interactive prompt)
  -y                  Overwrite existing tenant/auth without confirming

Flags for 'secrets':
  -y                  Do not prompt; print missing auths/secrets and exit non-zero

Flags for 'store-oauth':
  --user=<auth>         Instance-capable auth used to call Jamf (rotate credentials)
  --tenant=<tenant>     Tenant to link the oauth auth to (default: session tenant)
  --client-name=<name>  Jamf Pro API integration display name (default: buddy-cli)
  --name=<auth>         Buddy auth to create/update (--auth= alias)
  -y                    Skip confirmation when rotating the Jamf client secret

Examples:
  buddy setup basic-setup
  buddy setup secrets
  buddy setup managed-plist /tmp/buddy.plist
  buddy --user <admin-auth> auth test
  buddy setup write-script
  buddy setup_buddy_role.buddy -y
  buddy setup store-oauth
  buddy setup store-oauth --tenant=sandbox --name=sandbox-oauth -y
`
const ScriptGet = `buddy script get - Retrieve script information

Usage:
  buddy script get <script-id>           Show full script record (includes scriptContents)
  buddy script get contents <script-id>  Output raw script body only (pipeable)
  buddy script get id <name>             Resolve script name to numeric ID

Examples:
  buddy script get 7
  buddy script get contents 7
  buddy script get contents 7 > myscript.sh
  buddy script get id "Install Chrome"
`
const ScriptSet = `buddy script set - Modify script properties

Usage:
  buddy script set name     <script-id> <value>             Rename script
  buddy script set category <script-id> <category-id>        Set category
  buddy script set priority <script-id> BEFORE|AFTER|AT_REBOOT  Set execution priority
  buddy script set info     <script-id> <value>              Set info field
  buddy script set notes    <script-id> <value>              Set notes field

Examples:
  buddy script set name 7 "Install Chrome"
  buddy script set category 7 3
  buddy script set priority 7 AFTER
  buddy script set notes 7 "Installs Google Chrome"
`
const Serve = `buddy serve - Local docs and script IDE web UI

Usage:
  buddy [global-flags] serve [args...]

Flags for 'serve':
  --addr=<host:port>   Listen address (default: 127.0.0.1:8742; localhost only)
  --docs-root=<path>   Override embedded docs with a live docs/ directory
  --open               Open the docs URL in the default browser

Notes:
  Serves a curated user docs UI (guides, builtins, examples) plus:
  /ide — Single-buffer script runner (open/save, args, trace, Run as file,
         Variables/Libraries sidebar, copy/save/stash output)
  /notebook redirects to /ide.
  AGENTS.md and Cursor rules stay in the repo for contributors; they are not
  listed in the HTML sidebar or embedded for MCP.
  When run from a checkout, serve prefers the repo docs/ tree so edits show
  without a rebuild; otherwise the embedded catalog is used. --docs-root /
  BUDDY_DOCS_ROOT force a filesystem override.
  IDE open/save is limited to the process working directory, or BUDDY_IDE_ROOT
  if set. Run matches CLI file semantics (file dialect, call main() if present,
  args from the form, optional builtin-call trace). Line breakpoints / stepping
  are not supported (Starlark-go has no pause API); a soft breakpoint() builtin
  may come later.
  IDE runs use the same builtins as the CLI. Interactive confirm() and Jamf
  mutate confirms open a dialog in the browser (not the serve terminal).
  Global -y skips confirms entirely. The IDE uses the ide client
  channel with optional channels.ide policy for tenant and access.
  Bind address must be localhost (127.0.0.1, localhost, or ::1).

Examples:
  buddy serve --open
  buddy --tenant=sandbox serve --open
  buddy serve --addr=127.0.0.1:9000
`
const MCP = `buddy mcp - MCP stdio server for AI tools

Usage:
  buddy [global-flags] mcp
  buddy [global-flags] mcp setup [args...]

Subcommands:
  setup   Discover well-known MCP host configs; optionally write a buddy entry

Flags for 'setup':
  --write              Merge mcpServers.buddy into selected configs
  --create             Also create missing well-known files (with --write)
  --path=<file>        Inspect/write a specific MCP config JSON file
  --host=<name>        Filter: cursor | claude-desktop (default: all well-known)

Notes:
  Speaks Model Context Protocol over stdin/stdout for AI hosts (Cursor,
  Claude Desktop, and other MCP clients). Lean surface: buddy_info,
  docs, jamf_schema, buddyscript (primary), plus buddy://docs/... resources.
  Resource automation uses named builtins via buddyscript.
  Uses the active tenant/auth from config (override with --tenant / --user).

  Every tool returns a JSON object {ok, value, error?, count?} — never a bare
  array (hosts require object structuredContent). buddy_info reports binary
  version plus default tenant / auth pin and available tenants/auths for the
  mcp channel (no Jamf call). Live auth: first buddyscript Jamf call or CLI
  buddy auth test.

  Docs and examples are embedded in the binary and exposed as buddy://docs/...
  resources (no BUDDY_DOCS_ROOT required). AGENTS.md and Cursor rules are not
  embedded. Set BUDDY_DOCS_ROOT only to override with live files while developing.
  See docs/mcp.md and make sync-docfs after editing knowledge sources.
  Policy apps: edit internal/buddy/app_doc.go then make gendocs.

  mcp setup (no flags) lists well-known paths and whether buddy is configured.
  Prefer an absolute command path — host subprocesses often omit shell PATH.

Well-known config paths:
  Cursor (user)       ~/.cursor/mcp.json
  Cursor (project)    ./.cursor/mcp.json
  Claude Desktop      ~/Library/Application Support/Claude/claude_desktop_config.json
                      (macOS; Linux ~/.config/Claude/...; Windows %APPDATA%\Claude\...)

Example mcpServers entry:
  {
    "mcpServers": {
      "buddy": {
        "command": "/Users/YOU/go/bin/buddy",
        "args": ["mcp"]
      }
    }
  }

Examples:
  buddy mcp
  buddy mcp setup
  buddy mcp setup --write
  buddy -y mcp setup --write --create --host=cursor
`
const LSP = `buddy lsp - Language server for buddyscript (.buddy)

Usage:
  buddy lsp

Notes:
  Speaks LSP over stdin/stdout for the Cursor/VS Code buddy extension.
  Provides parse diagnostics, undefined-name checks, invalid load() modules,
  completions, hover, and signature help.
  Uses the same builtin catalog as 'buddy help builtins'.

  Scripts use .buddy. load("stdlib/<name>.buddy", …) is reserved (no modules yet).

  Optional initializationOptions.cellMode=true enables -e / REPL dialect
  (top-level if/for and global reassign).

Examples:
  buddy lsp
  # usually launched by the editor extension, not interactively
`
