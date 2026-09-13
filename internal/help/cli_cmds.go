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
  test [--all]                 Prove the secret (ForceRenew + probe); --all tests every context×surface×auth
  authorization [--all]        Print cached authorization details from GET /api/v1/auth
  token                        Show token subcommands (same as token help)
  token show [--raw]           Print Authorization header, or raw bearer with --raw
  token status [--all]         Show disk token cache metadata (no bearer)
  token drop [--all]           Invalidate the disk token cache (including still-valid files)
  token refresh [--raw]        Obtain, keep-alive, or replace the session token from cache state

Flags for 'test':
  --all                        Test every covering auth × context × surface (not just the default session)

Flags for 'authorization':
  --all                        Every still-valid authorization-details cache entry

Flags for 'token show':
  --raw                        Print bearer only (no "Bearer " prefix)

Flags for 'token status' / 'token drop':
  --all                        Every file under ~/.config/buddy/tokens/ (not just the current session)

Flags for 'token refresh':
  --raw                        Print the new bearer token only

Notes:
  auth test always discards the cached token and mints a new one. Exit 0 if the
  secret works (probe 403 is live=token-ok, not a bad secret). Exit non-zero if
  the secret is missing or the token POST fails. cred get secret is presence only.
  Organization workplaces probe GET /licensing/v1/licenses (no buildings
  probe; no X-Environment-Id). 403 is live=token-ok (missing licensing
  privilege, not a bad secret).

  auth authorization prints the last Jamf authorization details cached after
  auth test (GET /api/v1/auth: accessLevel, privilegeSet, privileges, username).
  Session pick and cred status use that cache. Missing or stale cache is ignored.

  auth token refresh uses the derived session (--context / --user) and the disk
  cache: basic keep-alives a usable cached token (Jamf POST /api/v1/auth/keep-alive),
  otherwise obtains; oauth and platform always replace via client credentials;
  command re-runs runCommand.
  auth token drop removes still-valid cache files; cred clean tokens only deletes
  expired or corrupt leftovers. One-shot mint without config is 'buddy token direct'.

Examples:
  buddy auth test
  buddy auth test --all
  buddy --user dev auth test
  buddy auth authorization
  buddy auth authorization --all
  buddy auth token
  buddy auth token show
  buddy auth token show --raw
  buddy --user dev auth token show --raw
  buddy auth token status
  buddy auth token status --all
  buddy auth token drop
  buddy --user dev auth token refresh
  buddy auth token refresh --raw
`
const Token = `buddy token - Mint a bearer token without config

Usage:
  buddy [global-flags] token <subcommand> [args...]

Subcommands:
  direct [args...]             One-shot token mint (no config or vault)

Flags for 'direct':
  --url=<url>                  Jamf Pro or Platform gateway base URL
  --region=<region>            Platform gateway region (builds URL when --url omitted)
  --environment-id=<environment-id>  Platform environment id (preferred GA scope)
  --tenant-id=<tenant-id>      Platform tenant id (legacy single-product scope)
  --username=<username>        Basic auth username
  --password=<password>        Basic auth password
  --client-id=<client-id>      OAuth2 client ID (instance, or Platform with a scope id)
  --client-secret=<client-secret>  OAuth2 client secret
  --oauth2-creds-file=<path>   JSON file with client_id and client_secret

Notes:
  Prints the raw bearer (no "Bearer " prefix). Does not load a named auth or the
  vault. Pass either username+password or client-id+client-secret (not both).
  --environment-id or --tenant-id (or a gateway --url/--region) selects Platform;
  omit both ids for organization scope. --environment-id and --tenant-id are
  mutually exclusive. Tokens still land in tokens/ under a direct-* cache key.
  Secrets on the command line are visible in process lists.

  For a configured session, use 'buddy auth token show --raw' instead.

Examples:
  buddy token direct --url=https://example.jamfcloud.com --username=admin --password=secret
  buddy token direct --region=us --environment-id=<environment-id> --client-id=<client-id> --client-secret=<client-secret>
  buddy token direct --url=https://example.jamfcloud.com --oauth2-creds-file=./client.json
`
const CredShort = `buddy cred - Manage contexts, auths, and secrets

Usage:
  buddy [global-flags] cred <subcommand> [args...]

Subcommands:
  add [auth-name] [args...]         Guided context + auth + secret
  set context <name> [args...]      Add or update a context (instance URL and/or Platform gateway)
  set auth <name> [args...]         Add or update an auth (the --user name)
  set secret <auth> [args...]       Store password / client secret in the vault
  status [--test] [--unlock]        Workplace coverage (org → env → tenant)
  list                              List contexts as org → environment → tenant
  get context|auth|channel|secret   Show one item (secrets redacted / presence only)
  use [context]                     Show or set context.default
  help                              Full flags, channel policy, backup, vault, migrate, clean, erase

Notes:
  An auth is a named principal in config.yaml (not a Jamf account). Pin it with
  --user / --auth. Full flags and notes: buddy cred help.

Examples:
  buddy cred add --context=sandbox --url=https://example.jamfcloud.com --client-id=… --client-secret=…
  buddy cred add --paste --context=sandbox
  buddy cred set context sandbox --url=https://example.jamfcloud.com
  buddy cred set auth sandbox-admin --type=basic --username=admin --jamfpro=sandbox
  buddy cred set secret sandbox-admin --password
`
const Cred = `buddy cred - Manage contexts, auths, secrets, and channel policies

Config is split into contexts (named workplace: instance URL and/or
Account environment id + jamfpro.tenantId), auths (secret-bearing principals
linked with jamfpro: / contexts:), and channels (per-client policy: access,
default context, context allowlist). Global --context selects a context
(auto-picks auth; --tenant is a dual-read alias); --user / --auth prefers a
specific auth (Classic follows the pin unless classic_instance_only is on;
ACS and GET /api/v1/policies are always instance).

Usage:
  buddy [global-flags] cred <subcommand> [args...]
  buddy [global-flags] cred backup <file> [--passphrase=] [-y]
  buddy [global-flags] cred restore <file> [--passphrase=] [-y]

Subcommands:
  add [auth-name] [args...]         Guided context + auth + secret (optional live test)
  set context <name> [args...]      Add or update a context (tenant is a dual-read alias)
  set auth <name> [args...]         Add or update an auth (the --user name; link to contexts)
  set secret <auth> [args...]       Store password / client secret in the vault
  status [--test] [--unlock]        Workplace coverage, org → env → tenant (gaps, secret presence, last live check)
  list                              List contexts as org → environment → tenant
  get context <name>                Show context details (tenant is a dual-read alias)
  get auth <name>                   Show auth details (secrets redacted)
  get channel <cli|mcp|ide>
                                    Show channel policy
  get secret <auth>                 Show where secrets are stored (values hidden; presence only)
  use [context]                     Show or set context.default
  use --cli|--mcp|--ide [context]
                                    Show or set channels.<client>.context
  set channel <cli|mcp|ide> [args...]
                                    Set channel access / context / contexts allowlist
  set passphrase [--passphrase=]    Create or rotate the recovery passphrase (backups)
  delete context|auth|secret <name> Remove a context (cascades sole-linked auths+secrets),
                                    auth (+secrets), or stored secrets
  clean secrets                     Review secret-store entries; prompt to delete orphans
  clean tokens                      Delete expired/corrupt files under ~/.config/buddy/tokens/
  erase secrets                     Wipe the vault, data key, recovery passphrase, token wrap/cache, and leftover Keychain items
  store                             Show vault path, data-key backend, and recovery-passphrase backend
  migrate [auth]                    Move inline YAML secrets and old per-auth store items into the vault
  backup <file>                     Encrypt contexts, auths (with secrets), and channels to a file
  restore <file>                    Decrypt a backup; prompt once per auth (or -y)

Flags for 'status':
  --test                            Run auth test --all first (ForceRenew), then print the matrix
  --unlock                          Open the vault so SECRET is stored/missing (default SECRET is locked)

Flags for 'add':
  --context=<name>                  Existing or new context name (--tenant alias)
  --url=<url>                       Jamf Pro instance URL (creating a context)
  --region=<region>                 Platform region (creating a context)
  --environment-id=<environment-id> Account environment UUID (context id)
  --tenant-id=<tenant-id>           Jamf Pro product tenant UUID (jamfpro.tenantId)
  --type=oauth|basic|platform       Auth type (default oauth; platform-only context → platform)
  --scope=organization|environment|tenant
                                    Platform auth Account scope (org: no region)
  --client-id=<client-id>           OAuth client id
  --username=<user>                 Basic username
  --client-secret=<secret>          OAuth secret (non-interactive)
  --password=<password>             Basic password (non-interactive)
  --paste                           Read Jamf client JSON or curl (prompt, or stdin when piped)
  --paste=<blob>                    Same, from a one-line JSON/curl string (- = stdin)
  --from=<file>                     Import secrets-free YAML or ACS plist, then fill secrets
  --org=<name>                      Parent organization workplace (nested orgs: on platform add)
  --environment-name=<name>         Environment workplace when creating a product tenant
  --test                            Force a live auth test after save
  --no-test                         Skip the live test (default when -y / non-TTY)
  -y                                Non-interactive; fail if required fields are missing

Flags for 'set context' (alias: set tenant):
  --url=<url>                       Jamf Pro instance URL (and/or platform gateway URL)
  --region=<region>                 Platform API Gateway region (us|eu|apac|…); omit on org workplaces
  --org=<context-name>              Parent organization workplace (environment/instance contexts)
  --environment-id=<environment-id> Account environment UUID (written as context id)
  --tenant-id=<tenant-id>           Jamf Pro product tenant UUID (jamfpro.tenantId)
  --deny-builtins=<a,b>             Block builtins on this context (* = deny all; empty clears)
  --allow-builtins=<a,b>            Override this context's deny (empty clears)

Flags for 'set auth':
  --type=command|basic|oauth|platform
                                    Auth type (required). platform = gateway oauth; oauth = instance oauth
  --jamfpro=<name>                  Instance (and tenant-scoped platform) product link
  --contexts=<a,b>                  Context names this platform auth may use
  --tenants=<a,b>                   Dual-read alias of --contexts
  --scope=organization|environment|tenant
                                    Platform auth Account scope (default environment)
  --access=full|readonly            Default full; readonly preferred for reads
  --channels=cli,mcp,ide|all
                                    Buddy clients that may use this auth (default all)
  --deny-builtins=<a,b>             Block builtins for this auth (* = deny all; empty clears)
  --allow-builtins=<a,b>            Override this auth's deny (empty clears)
  --run=<cmd>                       Command auth
  --username=<user>                 Basic auth
  --password=<password>             Basic password inline (dev/test)
  --client-id=<client-id>           OAuth client id
  --client-secret=<client-secret>   OAuth secret inline (dev/test)

Flags for 'set channel':
  --access=readonly|full            Clamp mutates for this client (omit = full)
  --context=<name>                  Default context for this client
  --contexts=<a,b>                  Optional context allowlist (empty clears)
  --tenant=<name>                   Dual-read alias of --context
  --tenants=<a,b>                   Dual-read alias of --contexts
  --deny-builtins=<a,b>             Extra blocked builtins (MCP already denies jamf_*; * = deny all; empty clears)
  --allow-builtins=<a,b>            Override this channel's deny (allow follows deny; empty clears)

Flags for 'set secret':
  --password[=value]                Store basic-auth password (no-echo prompt if omitted)
  --client-secret[=value]           Store OAuth client secret
  --secret-from-stdin               Read the secret value from stdin

Flags for 'set passphrase':
  --passphrase=<pass>               Recovery passphrase for cred backup/restore (prefer interactive prompt)

Flags for 'backup' / 'restore':
  --passphrase=<pass>               Encryption passphrase (defaults to the stored recovery passphrase)
  -y                                Overwrite backup file / restore all auths without prompting

Notes for 'backup' / 'restore':
  cred backup uses the stored recovery passphrase by default (--passphrase=
  overrides). It writes AES-256-GCM (PBKDF2-SHA256) JSON; auths include store
  secrets. Restore applies contexts/channels, then prompts once per auth (and
  its secrets) unless -y. Enter/y writes that auth from the backup; n leaves
  the existing auth (or absence) in config. Auths not in the backup are kept.
  A backup with no auths restores contexts/channels only. Secrets go to the
  vault under a new local data key; if no recovery passphrase exists yet,
  restore stores the backup passphrase as the recovery phrase.

Notes:
  cred status is the maintenance view (org → environment → product-tenant,
  then SURFACE). SURFACE is instance vs Platform gateway — not Jamf Account
  scope (that lives on the auth as SCOPE=organization|environment|tenant).
  cred list prints the same tree (ORG / ENVIRONMENT / CONTEXT); Account
  UUIDs stay on cred get. Organization-scoped auths only cover the
  organization workplace (not environments or product tenants). Default SECRET is locked
  (does not open the vault / Touch ID).
  cred status --unlock or --test classifies stored/missing. cred get secret
  is presence only — 'buddy auth test' proves the secret (ForceRenew). Last
  authorization details from GET /api/v1/auth live in ~/.config/buddy/caps/
  (same obfuscation style as tokens/).
  A platform clientId may list multiple contexts under --contexts.
  credentials: / string tenant: / defaults: / allow_auths: and the notebook
  / serve client names are removed — use contexts:/auths: and channels.<cli|mcp|ide>.
  Dual-read tenants: / tenant.default / --tenant / tenant= for one release;
  loading config rewrites those keys to context: / contexts: on disk.
  Optional orgs: YAML (organization → environments → product tenants) expands
  into those same workplaces; --context still picks a workplace name. Nested
  orgs.*.environments.*.tenants is Jamf's product-tenant list, not the root
  tenants: dual-read. cred add --paste with a US curl and blank env id still
  creates an org workplace under the name you passed (same object as orgs.<name>).
  macOS managed preferences (domain virtualpete.buddy.cli) overlay as org
  defaults; config.yaml wins per name. 'buddy cred list' notes managed sources.
  Deleting a managed context/auth warns; it returns on next load until the MDM
  profile changes. Fill secrets with 'buddy setup secrets' or 'buddy cred add'.
  Passwords and client secrets live in ~/.config/buddy/secrets.enc (AES-256-GCM
  with a random data key). The data key is one Keychain item on macOS (account
  "dek"), or ~/.config/buddy/dek on Linux/Windows. A recovery passphrase (Keychain
  account "master", or ~/.config/buddy/master) encrypts cred backup files only;
  day-to-day CLI does not prompt for it or for Touch ID. BUDDY_SECRET_STORE
  selects where those keys live (keychain, file, pass). BUDDY_VAULT_PASSPHRASE
  overrides the recovery passphrase (never written). cred set passphrase creates
  or rotates the recovery phrase (Touch ID on the signed binary).
  Deleting an auth also removes its password/client-secret from the vault.
  Deleting a context deletes auths linked only to that context (with secrets) and
  unlinks multi-context auths; use 'buddy cred clean secrets' for leftover orphans
  or 'buddy cred erase secrets' to wipe the vault, data key, recovery passphrase,
  token-wrap key, on-disk token cache, and leftover per-auth Keychain items
  (both macOS services).
  cred migrate moves inline YAML secrets into the vault and, with no auth name,
  imports leftover per-auth Keychain/file/pass entries.
  Token files under ~/.config/buddy/tokens/ are purged when expired (or via
  'buddy cred clean tokens'); still-valid entries are reused across CLI runs.
  'buddy auth token drop' invalidates still-valid cache files for a session.
  Auth channels (cli/mcp/ide) default to all. Channel policy access and
  auth access both clamp mutates (most restrictive wins). Restrict MCP e.g.:
  channels.mcp.access=readonly and/or auth channels excluding mcp from full auths.
  MCP denies jamf_* builtins by default on the channel layer (union with channels.mcp.deny_builtins).
  deny_builtins / allow_builtins also apply on contexts and auths; each layer is independent
  (channel allow cannot unblock a context deny). deny_builtins: ["*"] is an allowlist at that layer.
  Extra deny_builtins (exact or trailing-* glob) still block named builtins so MCP
  agents cannot fall back to generic jamf_get/jamf_put; treat raises as coverage gaps.
  cred add --paste accepts the JSON Jamf Pro shows when creating an API client,
  or the curl sample from a Platform API client. Organization-admin is the US
  gateway (us.api.jamfcloud.com) with no environment/tenant header — leave
  environment id blank. There is no api.jamfcloud.com host (NXDOMAIN); paste
  still maps that sample URL to org-against-US. A curl plus --environment-id=
  (any region) is environment-scoped. Interactive cred add offers a paste step
  first. buddy cred set context acme (no --region) defines an organization
  workplace (same object as orgs.acme). Environment/instance contexts may set
  --org=acme. Auth scope: is organization|environment|tenant; --context is only
  the workplace name.

Examples:
  buddy cred add --context=sandbox --url=https://example.jamfcloud.com --client-id=… --client-secret=…
  buddy cred add --context=sandbox --url=https://example.jamfcloud.com --type=basic --username=… --password=…
  buddy cred add --paste --context=sandbox
  pbpaste | buddy cred add --paste --context=sandbox -y
  buddy cred add --from=./quickstart.yaml
  buddy cred set context acme
  buddy cred set auth org --type=platform --client-id=… --contexts=acme --scope=organization
  buddy cred set context sandbox --url=https://example.jamfcloud.com --region=us --environment-id=… --org=acme
  buddy cred set auth sandbox-admin --type=basic --username=admin --jamfpro=sandbox --channels=cli,ide
  buddy cred set auth platform-ro --type=platform --client-id=… --contexts=sandbox --scope=environment --access=readonly --channels=mcp,ide
  buddy cred set secret platform-ro --client-secret
  buddy cred status
  buddy cred status --unlock
  buddy cred status --test
  buddy cred use sandbox
  buddy cred use --mcp sandbox
  buddy cred set channel mcp --access=readonly --context=sandbox --contexts=sandbox
  buddy cred set channel mcp --deny-builtins=computer_erase
  buddy cred set channel mcp --allow-builtins=*
  buddy cred set context prod --deny-builtins=* --allow-builtins=policies,policy
  buddy cred set passphrase
  buddy cred delete auth sandbox-admin
  buddy cred delete tenant sandbox
  buddy cred clean secrets
  buddy cred clean tokens
  buddy cred erase secrets
  buddy cred backup ./buddy.creds
  buddy cred restore ./buddy.creds
  buddy --context=sandbox policy list --filter=chrome
  buddy --user=sandbox-admin policy set …
`
const PolicyList = `buddy policy list - List policies

Usage:
  buddy [global-flags] policy list [--filter=<keyword>] [--ids] [--exclude-ids=1,2,3] [--json]

Flags for 'list':
  --filter=<keyword>    Match policy names (space-separated keywords all required).
                        Hyphenated forms of names are also tried. A single
                        trigger-like keyword (e.g. install-example-next) also
                        resolves via custom trigger lookup.
  --ids                 Print matching policy IDs only (one per line)
  --exclude-ids=1,2,3   Exclude specific policy IDs from results
  --json                Print matching rows as JSON

Examples:
  buddy policy list
  buddy policy list --filter=chrome
  buddy policy list --filter="chrome install" --ids
  buddy policy list --filter=install-chrome-next
  buddy policy list --filter=chrome --exclude-ids=100,101
  buddy policy list --json
`
const PolicyGet = `buddy policy get - Read policy properties

Usage:
  buddy [global-flags] policy get <policy-id>
  buddy [global-flags] policy get id <trigger>
  buddy [global-flags] policy get enable <policy-id>
  buddy [global-flags] policy get xml <policy-id>

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
  buddy [global-flags] policy set ss <policy-id> [version]
        Flags: [--display-name=...] [--description=...] [--install-text=...] [--reinstall-text=...]
               [--force-view|--no-force-view] [--feature|--no-feature]
               [--self-service|--no-self-service] [--group-ids=1,2,3]
  buddy [global-flags] policy set scope <policy-id> all
  buddy [global-flags] policy set scope <policy-id> <group-id> [group-id...]  [--group-ids=1,2,3]
  buddy [global-flags] policy set enable <policy-id>
  buddy [global-flags] policy set disable <policy-id>
  buddy [global-flags] policy set trigger <policy-id> <trigger>
  buddy [global-flags] policy set name <policy-id> <new-name>
  buddy [global-flags] policy set frequency <policy-id> <frequency>
  buddy [global-flags] policy set icon <policy-id> <image-file>
  buddy [global-flags] policy set install <policy-id> <version> [--param=4,5,6]
  buddy [global-flags] policy set package <policy-id> <version>
  buddy [global-flags] policy set package <version> --trigger=<trigger>
        Flags: [--name=<name>] [--package=<pattern>]
               [--disable-source|--keep-source]
  buddy [global-flags] policy set xml <policy-id>

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
  buddy policy set icon 42 ./app.png
  buddy policy set install 42 121.0 --param=4,5
  buddy policy set package 42 121.0 --keep-source
  buddy policy set package 121.0 --trigger=install-chrome
  buddy policy set package 42 121.0 --name=chrome
  buddy policy set xml 42 < policy.xml
`
const PolicyCreate = `buddy policy create - Create a new policy

Usage:
  buddy [global-flags] policy create package <name> <version>
        Flags: [--action=install|cache] [--scope=all|<group-id>[,<group-id>...]]
               [--category=<name>] [--package=<pattern>]
  buddy [global-flags] policy create ss <trigger> <display-name> [--category=<name>] [--script-id=<script-id>]

Arguments:
  <name>     Package name keyword (used for the package search and to build
             the policy name and trigger)
  <version>  Package version

Notes:
  create package invents triggers: install → install-<name>-next;
  cache → cache-<name>-<version>.
  create ss defaults to no script (--script-id omitted / 0). Pass --script-id=<script-id>
  to attach a Jamf script (parameter4 = <trigger>). Scope is all computers.

Examples:
  buddy policy create package chrome 121.0
  buddy policy create package chrome 121.0 --action=install --scope=all
  buddy policy create ss install-chrome "Install Chrome" --category="Mac Apps"
  buddy policy create ss install-chrome "Install Chrome" --script-id=12
`
const PolicyLogs = `buddy policy logs - Policy execution logs (Classic computer history)

Usage:
  buddy [global-flags] policy logs <policy-id> [--computer=<computer-id>] [--computer-ids=<computer-id>[,<computer-id>...]] [--filter=<term|rsql>] [--last-contact=<iso>] [--since=<iso-or-epoch>] [--until=<iso-or-epoch>] [--status=<status>] [--skip=<n>] [--limit=<n>] [--json]

Flags for 'logs':
  --computer=<computer-id>        Single computer to scan
  --computer-ids=<computer-id>[,<computer-id>...]   Caller-built candidate computer set
  --filter=<term|rsql>            computers() inventory filter (v4; bare term → name)
  --last-contact=<iso>            Computers with general.lastContact >= value (inventory, not "logs since")
  --since=<iso|epoch>             Keep rows with date_completed_epoch >= value
  --until=<iso|epoch>             Keep rows with date_completed_epoch <= value
  --status=<status>               Filter by log status (case-insensitive exact)
  --skip=<n>                      Computers to skip after the candidate list is built
  --limit=<n>                     Max computers to scan (alone: Classic computers/subset/basic)
  --json                          Print flattened logs as JSON

Notes:
  Jamf publishes policy execution logs per computer (Classic computerhistory
  PolicyLogs subset), so buddy scans a bounded computer set and filters by
  policy ID client-side. Pass exactly one computer source; an unconstrained
  fleet walk is rejected.

Examples:
  buddy policy logs 42 --computer=101
  buddy policy logs 42 --last-contact=2026-08-01T00:00:00Z --since=2026-08-01
  buddy policy logs 42 --limit=200 --status=Failed --json
`
const PolicyDelete = `buddy policy delete - Delete one or more policies

Usage:
  buddy [global-flags] policy delete <policy-id> [policy-id...]

Examples:
  buddy policy delete 42
  buddy policy delete 42 43 44
`
const Setup = `buddy setup - First-run chooser, quickstart import/export, secrets, MDM plist

Usage:
  buddy [global-flags] setup
  buddy [global-flags] setup import <file>
  buddy [global-flags] setup export [path]
  buddy [global-flags] setup secrets [-y]
  buddy [global-flags] setup managed-plist [path]

Subcommands:
  import <file>             Merge secrets-free YAML or ACS plist; then fill secrets
  export [path]             Write secrets-free identity YAML (stdout if path omitted)
  secrets [-y]              Fill missing auths and secrets for existing workplaces
  managed-plist [path]      Write org MDM plist (same identity as export; no auths)

Flags for 'secrets':
  -y                        Do not prompt; print missing auths/secrets and exit non-zero

Notes:
  Empty config: any interactive Jamf command, or 'buddy setup' / 'buddy cred add',
  asks how to start: Jamf Pro instance (API-client JSON, or URL + basic), Jamf
  Account / Platform (paste curl), a quickstart file, or an org MDM profile.
  --context is a workplace name; auth scope: is organization|environment|tenant.
  Optional orgs: YAML expands into those workplaces. A US curl with blank env id
  is an org workplace (same object as orgs.<name>).

  From a working install, setup export writes the mail-around YAML (orgs: /
  leftover contexts: / context.default / channels; no auths). Recipients run
  setup import. managed-plist is the same identity as ACS XML for domain
  virtualpete.buddy.cli. import and cred add --from= accept either file.
  Then setup secrets stores passwords and client secrets in the vault.

  If managed preferences already provide workplaces (domain virtualpete.buddy.cli),
  run setup secrets. User config.yaml overlays managed prefs (user wins per name).

  Instance basic auth: cred add --type=basic --url= --username= --password=.
  Instance oauth: paste the JSON Jamf Pro shows when creating an API client.

Examples:
  buddy setup
  buddy cred add --paste --context=sandbox
  buddy cred add --context=sandbox --url=https://example.jamfcloud.com --type=basic --username=admin --password=…
  buddy setup export ./quickstart.yaml
  buddy setup import ./quickstart.yaml
  buddy setup secrets
  buddy setup managed-plist /tmp/buddy.plist
  buddy auth test
`
const ScriptGet = `buddy script get - Retrieve script information

Usage:
  buddy [global-flags] script get <script-id>           Show full script record (includes scriptContents)
  buddy [global-flags] script get contents <script-id>  Output raw script body only (pipeable)
  buddy [global-flags] script get id <name>             Resolve script name to numeric ID

Examples:
  buddy script get 7
  buddy script get contents 7
  buddy script get contents 7 > myscript.sh
  buddy script get id "Install Chrome"
`
const ScriptSet = `buddy script set - Modify script properties

Usage:
  buddy [global-flags] script set name     <script-id> <value>             Rename script
  buddy [global-flags] script set category <script-id> <category-id>        Set category
  buddy [global-flags] script set priority <script-id> BEFORE|AFTER|AT_REBOOT  Set execution priority
  buddy [global-flags] script set info     <script-id> <value>              Set info field
  buddy [global-flags] script set notes    <script-id> <value>              Set notes field

Examples:
  buddy script set name 7 "Install Chrome"
  buddy script set category 7 3
  buddy script set priority 7 AFTER
  buddy script set notes 7 "Installs Google Chrome"
`
const ExtensionAttributeGet = `buddy extension-attribute get - Retrieve computer extension attribute information

Usage:
  buddy [global-flags] extension-attribute get <ea-id>           Show full EA record
  buddy [global-flags] extension-attribute get contents <ea-id>  Output SCRIPT body only (pipeable)
  buddy [global-flags] extension-attribute get id <name>         Resolve EA name to numeric ID

Aliases: ea, eas

Examples:
  buddy ea get 7
  buddy ea get contents 7
  buddy ea get contents 7 > check.sh
  buddy ea get id "OS Version Check"
`
const ExtensionAttributeSet = `buddy extension-attribute set - Modify computer extension attribute properties

Usage:
  buddy [global-flags] extension-attribute set name        <ea-id> <value>       Rename EA
  buddy [global-flags] extension-attribute set enabled     <ea-id> true|false    Enable/disable (SCRIPT)
  buddy [global-flags] extension-attribute set description <ea-id> <value>       Set description
  buddy [global-flags] extension-attribute set contents    <ea-id> <value>       Set SCRIPT body
  buddy [global-flags] extension-attribute set display     <ea-id> <type>        Set inventoryDisplayType

Examples:
  buddy ea set name 7 "OS Version Check"
  buddy ea set enabled 7 false
  buddy ea set display 7 EXTENSION_ATTRIBUTES
`
const Serve = `buddy serve - Local docs, config, and script IDE web UI

Usage:
  buddy [global-flags] serve [args...]

Flags for 'serve':
  --addr=<host:port>   Listen address (default: 127.0.0.1:8742; localhost only)
  --docs-root=<path>   Override embedded docs with a live docs/ directory
  --open               Open the docs URL in the default browser

Notes:
  Serves a curated user docs UI (guides, builtins, examples) plus:
  /config — Workplace/auth metadata editor (defaults, coverage). First-run is
            the CLI chooser (buddy setup / cred add); secrets stay in the vault.
  /ide — Single-buffer script runner (open/save, args, trace, Run as file,
         Variables/Libraries sidebar, copy/save/stash output)
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
  channel with optional channels.ide policy for context and access.
  Bind address must be localhost (127.0.0.1, localhost, or ::1).

Examples:
  buddy serve --open
  buddy --context=sandbox serve --open
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
  Uses the active context/auth from config (override with --context / --user;
  --tenant is a dual-read alias).
  With no contexts configured, buddy mcp registers no tools and instructs the
  With no contexts configured, buddy mcp registers no tools and instructs the
  host to run CLI setup first (buddy setup / buddy cred add).

  Every tool returns a JSON object {ok, value, error?, count?} — never a bare
  array (hosts require object structuredContent). buddy_info reports binary
  version plus default context / auth pin (JSON fields context / tenant, contexts / tenants) for the
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
