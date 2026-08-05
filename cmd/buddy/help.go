package main

import (
	"fmt"
	"os"
	"strings"

	"buddy/internal/help"
)

const helpAuth = `buddy auth - Obtain and test API authentication

Usage:
  buddy [global-flags] auth <subcommand> [args...]

Subcommands:
  test                         Test auth (instance: GET /api/v1/auth; platform: token + buildings probe)
  token [flags]                Print Authorization header or raw bearer token
  refresh [flags] [<token>]    Refresh a bearer token via keep-alive (instance basic auth)

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

Examples:
  buddy auth test
  buddy auth token
  buddy auth token --raw
  buddy --user dev auth token --raw
  buddy auth token --raw --url=https://example.jamfcloud.com --username=admin --password=secret
  buddy auth token --raw --region=us --tenant-id=<tenant-id> --client-id=<id> --client-secret=<secret>
  buddy auth refresh eyJhbGciOiJIUzI1NiJ9...
  buddy --user dev auth refresh --token=eyJhbGciOiJIUzI1NiJ9...
`

const helpCred = `buddy cred - Manage tenants, auths, secrets, and channel policies

Config is split into tenants (Jamf host / platform tenantId), auths (secret-bearing
principals linked to one or more tenants), and channels (per-client policy:
access, default tenant, tenant allowlist). Global --tenant selects a tenant
(auto-picks auth); --user / --auth pins a specific auth.

Usage:
  buddy cred <subcommand> [args...]

Subcommands:
  list                              List tenants, auths, and channel policies
  get tenant <name>                 Show tenant details
  get auth <name>                   Show auth details (secrets redacted)
  get channel <cli|mcp|ide|notebook>
                                    Show channel policy
  get secret <auth>                 Show where secrets are stored (values hidden)
  set tenant <name> [args...]       Add or update a tenant
  set auth <name> [args...]         Add or update an auth (link to tenants)
  set channel <cli|mcp|ide|notebook> [args...]
                                    Set channel access / tenant / tenants allowlist
  set secret <auth> [args...]       Store password / client secret in the secure store
  delete tenant|auth|secret <name>  Remove a tenant (cascades sole-linked auths+secrets),
                                    auth (+secrets), or stored secrets
  clean secrets                     Review secret-store entries; prompt to delete orphans
  clean tokens                      Delete expired/corrupt files under ~/.config/buddy/tokens/
  use [tenant]                      Show or set tenant.default
  use --cli|--mcp|--ide|--notebook [tenant]
                                    Show or set channels.<client>.tenant
  store                             Show the active secret backend
  migrate [auth]                    Move inline config.yaml secrets into the secure store

Flags for 'set tenant':
  --url=<url>                       Jamf Pro instance URL (and/or platform gateway URL)
  --region=<region>                 Platform API Gateway region (us|eu|apac|…)
  --tenant-id=<tenant-id>           Platform tenant id (with region)

Flags for 'set auth':
  --type=command|basic|oauth        Auth type (required)
  --tenants=<a,b>                   Comma-separated tenant names this auth may use (required)
  --access=full|readonly            Default full; readonly preferred for reads
  --surface=any|platform|instance   Jamf API surface (default any; platform requires oauth)
  --channels=cli,mcp,ide,notebook|all
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

Flags for 'set secret':
  --password[=value]                Store basic-auth password (no-echo prompt if omitted)
  --client-secret[=value]           Store OAuth client secret
  --secret-from-stdin               Read the secret value from stdin

  Notes:
  A platform clientId may list multiple tenants under --tenants.
  Old credentials: / string tenant: / defaults: / allow_auths: migrate on load.
  tenant.cli|mcp|serve|ide and the serve client channel are removed — use
  channels.<cli|mcp|ide|notebook> instead.
  Secrets are keyed by auth name in the platform store.
  Deleting an auth also removes its password/client-secret from the store.
  Deleting a tenant deletes auths linked only to that tenant (with secrets) and
  unlinks multi-tenant auths; use 'buddy cred clean secrets' for leftover orphans.
  Token files under ~/.config/buddy/tokens/ are purged when expired (or via
  'buddy cred clean tokens'); still-valid entries are reused across CLI runs.
  Auth channels (cli/mcp/ide/notebook) default to all. Channel policy access and
  auth access both clamp mutates (most restrictive wins). Restrict MCP e.g.:
  channels.mcp.access=readonly and/or auth channels excluding mcp from full auths.

Examples:
  buddy cred set tenant sandbox --url=https://example.jamfcloud.com --region=apac --tenant-id=…
  buddy cred set auth platform-ro --type=oauth --client-id=… --tenants=sandbox --access=readonly --surface=platform --channels=mcp,notebook
  buddy cred set auth sandbox-admin --type=basic --username=admin --tenants=sandbox --surface=instance --channels=cli,ide
  buddy cred set channel mcp --access=readonly --tenant=sandbox --tenants=sandbox
  buddy cred set secret platform-ro --client-secret
  buddy cred delete auth sandbox-admin
  buddy cred delete tenant sandbox
  buddy cred clean secrets
  buddy cred clean tokens
  buddy cred use sandbox
  buddy cred use --mcp sandbox
  buddy --tenant=sandbox policy list --filter=chrome
  buddy --user=sandbox-admin policy set …
`

const helpPolicy = `buddy policy - Manage Jamf policies

Usage:
  buddy [global-flags] policy <subcommand> [args...]

Subcommands:
  list    List policies (optional keyword filter)
  get     Read policy (details, id by trigger, enable state, xml)
  set     Modify policy properties (scope, enable/disable, trigger, name, ...)
  create  Create a new policy
  delete  Delete one or more policies

Use 'buddy policy <subcommand> help' for subcommand-specific help.

Examples:
  buddy policy list --filter=chrome
  buddy policy get id install-chrome
  buddy policy delete 42
`

const helpPolicyList = `buddy policy list - List policies

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

const helpPolicyGet = `buddy policy get - Read policy properties

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

const helpPolicySet = `buddy policy set - Modify policy properties

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

const helpPolicyCreate = `buddy policy create - Create a new policy

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

const helpPolicyClone = `buddy policy clone — removed

Versioned policy clone was retired. Prefer:

  • Change batches: step_clone_policy / step_set_package (see docs/change-batches.md)
  • buddyscript: policy_copy + policy_set_package / policy_set_ss / policy_set_install
`

const helpPolicyFind = `buddy policy find — removed

Keyword search moved to 'buddy policy list':

  buddy policy list [--filter=<keyword>] [--ids] [--exclude-ids=1,2,3]
`

const helpPolicyDelete = `buddy policy delete - Delete one or more policies

Usage:
  buddy policy delete <policy-id> [policy-id...]

Examples:
  buddy policy delete 42
  buddy policy delete 42 43 44
`

const helpMdm = `buddy mdm - Query MDM commands from Jamf Pro

Usage:
  buddy [global-flags] mdm <subcommand> [args...]

Subcommands:
  list   List MDM commands matching an RSQL filter

Flags for 'list':
  --filter=<rsql>       RSQL filter expression (required; API requires at least one field)
  --sort=<expr>         Sort expression (default: dateSent:asc)
  --count               Fetch all matching records and print only the total count (table suppressed)
  --page-size=<n>       Records per API request (default 200; pagination fetches all)
  --limit=<n>           Maximum total records to return (0 = no limit)

RSQL filterable fields:
  uuid, clientManagementId, command, status, clientType, dateSent,
  validAfter, dateCompleted, profileId, profileIdentifier, active

  Date fields require gt/lt/ge/le operators (not ==).

Examples:
  buddy mdm list --filter="status==Pending"
  buddy mdm list --filter="clientManagementId==00000000-0000-4000-8000-000000000002"
  buddy mdm list --filter="status==Pending;clientType==COMPUTER" --sort="dateSent:desc"
  buddy mdm list --filter="dateCompleted=ge=2024-01-01T00:00:00Z;status==Acknowledged"
  buddy mdm list --filter="status==Pending" --count
  buddy mdm list --filter="status==Pending" --limit=50
`

const helpSetup = `buddy setup - Bootstrap instance config; Jamf Pro role/client + instance oauth

Usage:
  buddy [global-flags] setup basic-setup [--tenant=] [--url=] [--name=] [--username=] [--password=] [-y]
  buddy [global-flags] setup write-script [path]
  buddy [global-flags] setup store-oauth [--user=<auth>] [--tenant=<tenant>] [--client-name=buddy-cli] [--name=<auth>] [-y]

Notes:
  write-script and store-oauth target the Jamf Pro *instance* (API roles and
  integrations). Platform gateway oauth cannot create roles or clients.
  If you have no config yet, start with basic-setup (instance URL + basic auth).

Subcommands:
  basic-setup
    Interactive bootstrap for the first tenant and a basic (username/password)
    instance auth. Stores the password in the platform secret store and sets the
    tenant as default when appropriate. Then prints next steps (auth test, and
    optional write-script → store-oauth for day-to-day oauth).

    Non-interactive: pass --tenant, --url, --username, and --password (and
    optionally --name for the auth; default <tenant>-admin).

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

Flags for 'store-oauth':
  --user=<auth>         Instance-capable auth used to call Jamf (rotate credentials)
  --tenant=<tenant>     Tenant to link the oauth auth to (default: session tenant)
  --client-name=<name>  Jamf Pro API integration display name (default: buddy-cli)
  --name=<auth>         Buddy auth to create/update (--auth= alias)
  -y                    Skip confirmation when rotating the Jamf client secret

Examples:
  buddy setup basic-setup
  buddy --user <admin-auth> auth test
  buddy setup write-script
  buddy setup_buddy_role.buddy -y
  buddy setup store-oauth
  buddy setup store-oauth --tenant=sandbox --name=sandbox-oauth -y
`

const helpCategory = `buddy category - Manage Jamf Pro categories

Usage:
  buddy [global-flags] category <subcommand> [args...]

Subcommands:
  list [--filter=<name>]              List categories (id, priority, name)
  get <category-id>                   Show category details (JSON)
  create <name> [--priority=<1-20>]   Create a category
  delete <category-id>                Delete a category

Flags for 'list':
  --filter=<name>   Plain name substring or RSQL expression (e.g. 'name=="*Apps*"')

Flags for 'create':
  --priority=<1-20>   Category priority (default 9)

Examples:
  buddy category list
  buddy category list --filter=apps
  buddy category get 3
  buddy category create "Mac Apps" --priority=9
  buddy category delete 3
`

const helpGroup = `buddy group - Manage Jamf computer groups

Usage:
  buddy [global-flags] group <list|get|set|create|delete|static> [args...]

Subcommands:
  list smart [--filter=<rsql>]           List smart computer groups
  list static [--filter=<rsql>]          List static computer groups
  get <group-id>                         Show group JSON (smart group, falling back to static)
  get version <group-id>                 Show current version criterion (smart group)
  set version <group-id> <version>       Update version criterion (smart group)
  create smart <name> [--description=<text>] [--file=<json>]
                                         Create a smart computer group
  create smart --file=<json>             Create from full smart-group JSON body
  create static <name> [--description=<text>] [--computer-ids=<id>[,<id>...]]
                                         Create a static computer group
  delete <group-id>                      Delete smart group
  static members <group-id> [--filter=<substring>]   List computers in a static group (client-side filter)
  static add <group-id> --computer-ids=<id>[,<id>...]   Add computers to a static group
  static remove <group-id> --computer-ids=<id>[,<id>...]   Remove computers from a static group

Flags for 'list':
  --filter=<rsql>   Plain name substring or RSQL expression (e.g. 'name=="*Chrome*"')

Flags for 'create smart':
  --description=<text>   Optional description
  --file=<json>          Criteria JSON array, or full smart-group body (object). With <name>,
                         a body object may omit name (positional is used). Without <name>,
                         --file must be a full body including name.

Flags for 'create static':
  --description=<text>              Optional description
  --computer-ids=<id>[,<id>...]     Optional initial membership

Flags for 'static members':
  --filter=<substring>   Client-side substring match on computer id, name, or serial

Flags for 'static add' and 'static remove':
  --computer-ids=<id>[,<id>...]   Required. Comma-separated Jamf computer IDs

Notes:
  For static members/add/remove, flags such as --computer-ids and --filter may appear before or after
  <group-id> (Go's flag parser otherwise stops at the first positional).
  Listing static group members prefers GET /api/v3/computer-groups/static-groups/{id} when the response
  includes assignments; otherwise Classic GET /JSSResource/computergroups/id/{id} is used (requires
  Read Computer Groups for that fallback). Updates use PUT on the v3 static group endpoint.

Examples:
  buddy group list smart
  buddy group list static --filter=chrome
  buddy group list smart --filter='name=="*Chrome*"'
  buddy group get 10
  buddy group get version 10
  buddy group set version 10 121.0
  buddy group create smart "Chrome installed" --file=criteria.json
  buddy group create static "Lab Macs" --computer-ids=101,102
  buddy group delete 10
  buddy group static members 42 --filter=lab
  buddy group static add 42 --computer-ids=101,102
  buddy group static remove 42 --computer-ids=101
`

const helpProfile = `buddy profile - Manage macOS configuration profiles

Usage:
  buddy [global-flags] profile <subcommand> [args...]

Subcommands:
  list [--filter=<name>]                                    List configuration profiles
  get xml <profile-id>                                      Output full profile XML
  get payloads <profile-id>                                 Output payloads plist only (pretty-printed)
  get payload <profile-id> <domain>                          Output single PayloadContent payload by PayloadIdentifier
  get list-payloads <profile-id>                            List PayloadIdentifier values (PayloadContent only)
  set xml <profile-id> [--redeploy]                         PUT profile from XML on stdin
  set payload <profile-id> <domain> <dict-file.xml> [--redeploy]   Patch payload (PayloadContent merge or MCX settings)
  clone <profile-id> <new-name>                             Clone profile with new name, unassigned scope
  redeploy <profile-id>                                     Redeploy to all assigned devices

Notes:
  --redeploy sets redeploy_on_update=All (pushes to all assigned devices, not just new ones)
  get payload / list-payloads are PayloadContent only. For MCX inspect use profile XML extract + payloads_get_mcx_dict.
  set payload: PayloadContent merges keys onto the existing payload; MCX dict file is settings-only.

Examples:
  buddy profile list
  buddy profile list --filter=Chrome
  buddy profile get xml 99
  buddy profile get payloads 99
  buddy profile get list-payloads 99
  buddy profile get payload 99 com.google.chrome
  buddy profile clone 99 "New Profile"
  buddy profile set xml 99 < profile.xml
  buddy profile set payload 99 com.google.chrome payload.xml --redeploy
  buddy profile redeploy 99
`

const helpBlueprint = `buddy blueprint - Manage Jamf Platform API Blueprints

Usage:
  buddy [global-flags] blueprint <subcommand> [args...]

Subcommands:
  list                          List blueprints (table)
  get <blueprint-id>            Fetch one blueprint (JSON)
  create [--file=<path>|-]      Create from JSON body (file or stdin)
  set <blueprint-id> [--file=|-]  PATCH blueprint (merge-patch+json)
  delete <blueprint-id>         Delete a blueprint
  deploy <blueprint-id>         Deploy a blueprint
  undeploy <blueprint-id>       Undeploy a blueprint
  status <blueprint-id>         Deployment status report (JSON)
  components [identifier]       List components, or get one by identifier

Flags for 'list':
  --search=<text>               Search name/description
  --sort=<field:dir>            Sort (default created:desc)

Notes:
  Requires a Platform API Gateway credential (region like us, or url like https://us.apigw.jamf.com
  and tenantId). See 'buddy cred help'.

Examples:
  buddy blueprint list
  buddy blueprint list --search=chrome
  buddy blueprint get e77c1408-10c8-4007-b177-abc9157fbcaa
  buddy blueprint create --file=bp.json
  buddy blueprint set <blueprint-id> --file=patch.json
  buddy blueprint deploy <blueprint-id>
  buddy blueprint status <blueprint-id>
  buddy blueprint components
`

const helpDevice = `buddy device - Manage Jamf Platform API devices (inventory + actions)

Usage:
  buddy [global-flags] device <subcommand> [args...]

Subcommands:
  list [--filter=<rsql>] [--sort=]     List Platform devices (table)
  get <device-id>                      Fetch one device (JSON)
  set <device-id> [--file=|-]          PATCH device JSON
  delete <device-id>                   Delete a device
  applications <device-id> [--filter=] List installed applications
  user <user-id>                       List devices for a user
  check-in <device-id>                 Request device check-in
  restart <device-id>                  Restart device
  shutdown <device-id>                 Shut down device
  erase <device-id>                    Erase device
  unmanage <device-id>                 Remove remote management

Notes:
  Platform Devices API (not Jamf Pro computer inventory). Use 'buddy computer' for Pro/MDM.
  Requires Platform API Gateway tenant+auth. Actions need execute:pro:device-actions.

Examples:
  buddy device list --filter='name=="*Mac*"'
  buddy device get <device-id>
  buddy device applications <device-id>
  buddy device restart <device-id>
`

const helpDeviceGroup = `buddy device-group - Manage Jamf Platform API device groups

Usage:
  buddy [global-flags] device-group <subcommand> [args...]

Subcommands:
  list [--filter=<rsql>]               List device groups (table)
  get <group-id>                       Fetch one group (JSON)
  create [--file=<path>|-]             Create from JSON body
  set <group-id> [--file=|-]           PATCH group JSON
  delete <group-id>                    Delete a group
  members <group-id>                   List group members (JSON)
  set-members <group-id> [--file=|-]   PATCH membership (static groups)

Notes:
  Distinct from Jamf Pro computer groups ('buddy group' / smart_group / static_group).
  Requires Platform API Gateway tenant+auth.

Examples:
  buddy device-group list
  buddy device-group get <group-id>
  buddy device-group members <group-id>
`

const helpDDM = `buddy ddm - Jamf Platform DDM declaration reporting (read-only)

Usage:
  buddy [global-flags] ddm <subcommand> [args...]

Subcommands:
  declaration <declaration-id> [--filter=] [--sort=]
      List devices reporting a declaration
  device <device-id> declarations [--filter=] [--sort=]
      List declarations reported for a device
  device <device-id> channels
      List channels available for a device

Notes:
  Requires Platform API Gateway tenant+auth and read:pro:declaration-reporting.
  Useful alongside 'buddy blueprint status' for deployment verification.

Examples:
  buddy ddm declaration com.jamf.ddm.sw-updates
  buddy ddm device <device-id> declarations
  buddy ddm device <device-id> channels
`

const helpBenchmark = `buddy benchmark - Jamf Platform compliance benchmarks (beta)

Usage:
  buddy [global-flags] benchmark <subcommand> [args...]

Subcommands:
  list                                   List tenant benchmarks (JSON)
  get <benchmark-id>                     Fetch one benchmark
  create [--file=<path>|-]               Create from JSON body
  delete <benchmark-id>                  Delete a benchmark
  baselines                              List mSCP baselines
  rules [--baseline=<id>]                Catalog rules for a baseline
  compliance-percentage <benchmark-id>   Overall compliance percentage
  report-devices <benchmark-id>          Devices for a benchmark report
  report-rules <benchmark-id>            Rules stats for a benchmark

Notes:
  Beta Platform API. Requires Platform gateway tenant+auth and
  read|create|delete:pro:compliance-benchmarks as applicable.

Examples:
  buddy benchmark list
  buddy benchmark baselines
  buddy benchmark get <benchmark-id>
`

const helpPackage = `buddy package - Manage Jamf Pro package records

Usage:
  buddy [global-flags] package <subcommand> [args...]

Subcommands:
  list [--filter=<rsql>] [--sort=version[:asc|desc]]  List packages
  get <package-id>                                    Show package record details
  download <package-id> [outfile]                     Download package file via JCDS (SHA-256 verified)
  history <package-id>                                Show recent change history (last 20 entries)
  delete <package-id> [package-id...]                Delete one or more package records
  upload <file> [attr-flags…]                        Upload .pkg or .dmg (create or update)
  set <package-id> [attr-flags…]                     Update package metadata

Flags for 'upload' / 'set':
  --name=<display-name>       Package display name (upload default: filename)
  --notes=<text>              Notes field
  --info=<text>               Info field
  --category=<category-id>    Category ID (-1 to clear; upload default: -1)
  --priority=<1-20>           Install priority (upload default: 10)
  --reboot=true|false         rebootRequired flag (upload default: false)

Notes:
  --sort=version sorts client-side by a dotted version extracted from the package
  name (latest first). Use --sort=version:asc for oldest first.
  get prints the full package record (checksums, size, cloudTransferStatus, flags, …)
  plus lastChanged fields from history.
  download waits until cloudTransferStatus is READY and sha256 is set, fetches a
  JCDS pre-signed URI, streams to a temp file while hashing SHA-256, and renames
  only on match. JCDS only (Jamf Cloud). Default outfile is the package fileName.
  upload looks up fileName first: existing record → compare checksums and skip or
  replace the file (and apply attr flags when they differ); new fileName → create
  metadata with attr flags, wait until readable, then upload.
  Only .pkg and .dmg are supported.

Examples:
  buddy package list
  buddy package list --filter='packageName=="*Chrome*"'
  buddy package list --filter='packageName=="*Chrome*"' --sort=version
  buddy package get 55
  buddy package download 55
  buddy package download 55 ./Chrome-121.pkg
  buddy package history 55
  buddy package upload Chrome-121.pkg --name="Google Chrome 121" --category=3 --priority=10
  buddy package upload Chrome-121.pkg --notes="rollout" --reboot=false
  buddy package set 55 --name="Google Chrome 121" --priority=5 --reboot=true
  buddy package delete 55 56
`

const helpComputer = `buddy computer - Manage computers and send MDM commands

Usage:
  buddy [global-flags] computer <subcommand> [args...]

Subcommands:
  list [--filter=<rsql>] [--management-id=<uuid>]    List computers (RSQL filter optional; --management-id looks up by MDM management UUID)
  get <computer-id>                                   Show computer details
  reboot <computer-id> [computer-id...]               Send RESTART_DEVICE MDM command
  redeploy <computer-id> [computer-id...]            Trigger Jamf management framework redeploy
  shutdown <computer-id> [computer-id...]            Send SHUT_DOWN_DEVICE MDM command
  lock <computer-id> [computer-id...] [--message=...] [--phone=...] [--pin=...]  Send DEVICE_LOCK (extra confirm)
  logout <computer-id> [computer-id...]               Send LOG_OUT_USER MDM command
  erase <computer-id> [computer-id...] [--pin=...] [--obliteration=...] Send ERASE_DEVICE (type 'erase' to confirm)
  device-info <computer-id> [computer-id...] [--queries=DeviceName,...]  Send DEVICE_INFORMATION (queued)
  device-location <computer-id> [computer-id...]      Send DEVICE_LOCATION (queued)
  security-info <computer-id> [computer-id...]        Send SECURITY_INFO (queued)
  installed-apps <computer-id> [computer-id...] [--identifiers=...]     Send INSTALLED_APPLICATION_LIST (queued)
  managed-apps <computer-id> [computer-id...] [--identifiers=...]       Send MANAGED_APPLICATION_LIST (queued)
  certificates <computer-id> [computer-id...]         Send CERTIFICATE_LIST (queued)

Examples:
  buddy computer list
  buddy computer list --filter='general.name=="*Example*"'
  buddy computer list --management-id=00000000-0000-4000-8000-000000000003
  buddy computer get 101
  buddy computer reboot 101 102
  buddy computer lock 101 --message="Lost device" --pin=123456
  buddy computer erase 101 --pin=123456
`

const helpRole = `buddy role - Manage Jamf Pro API roles

Usage:
  buddy [global-flags] role <subcommand> [args...]

Subcommands:
  list                                    List all API roles
  get <role-id>                           Show API role details
  create <displayName> [priv1 priv2...]   Create API role with optional privileges
  update <role-id> <displayName> [priv1...]    Update API role name and privileges
  delete <role-id>                        Delete API role
  privileges                              List all available privilege names

Examples:
  buddy role list
  buddy role get 5
  buddy role create "Read Policies" "Read Policies"
  buddy role update 5 "Read Policies" "Read Policies" "Read Scripts"
  buddy role delete 5
  buddy role privileges
`

const helpClient = `buddy client - Manage Jamf Pro API integrations

Usage:
  buddy [global-flags] client <subcommand> [args...]

Subcommands:
  list                                              List all API integrations
  get <client-id>                                   Show API integration details
  create <displayName> [scope1 scope2...]           Create integration with role scopes
  update <client-id> <displayName> [scope1 scope2...]  Update integration name and scopes
  delete <client-id>                                Delete API integration
  credentials <client-id>                           Generate new OAuth client credentials
  set token-lifetime <client-id> <seconds>          Set access token lifetime in seconds

Examples:
  buddy client list
  buddy client get 3
  buddy client create "My App" "Read Policies"
  buddy client update 3 "My App" "Read Policies" "Read Scripts"
  buddy client delete 3
  buddy client credentials 3
  buddy client set token-lifetime 3 3600
`

const helpScript = `buddy script - Manage Jamf Pro scripts

Usage:
  buddy [global-flags] script <subcommand> [args...]

Subcommands:
  list [--filter=<name>]                              List scripts (optional name substring filter)
  get <script-id>                                     Show full script record (includes body)
  get contents <script-id>                            Output raw script body only (pipeable)
  get id <name>                                       Resolve script name to numeric ID
  history <script-id>                                 Show recent change history (last 20 entries)
  create <name> <file> [flags]                        Create a new script from a local file
        [--category=<category-id>] [--priority=BEFORE|AFTER|AT_REBOOT] [--info=<text>] [--notes=<text>]
  upload <script-id> <file> [--name=<new-name>]       Upload (replace) contents of an existing script
  update <script-id> <file> [--name=<new-name>]       Alias for upload
  download <script-id> [outfile]                      Save script body to a local file (default: <name>.sh)
  delete <script-id> [script-id...]                   Delete one or more scripts
  set name     <script-id> <value>                    Rename script
  set category <script-id> <category-id>              Set category
  set priority <script-id> BEFORE|AFTER|AT_REBOOT    Set execution priority
  set info     <script-id> <value>                    Set info field
  set notes    <script-id> <value>                    Set notes field

Use 'buddy script get help' or 'buddy script set help' for more detail.

Examples:
  buddy script list --filter=Chrome
  buddy script get 7
  buddy script create "Install Chrome" chrome.sh --priority=AFTER
  buddy script upload 7 chrome.sh
  buddy script download 7
  buddy script delete 7 8
`

const helpScriptGet = `buddy script get - Retrieve script information

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

const helpScriptSet = `buddy script set - Modify script properties

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

// printCommandHelp routes 'buddy help [command] [subcommand]' to the appropriate help text.
func printCommandHelp(path []string) {
	key := strings.Join(path, ".")
	helpMap := map[string]string{
		"auth":          helpAuth,
		"cred":          helpCred,
		"shell":         help.Shell,
		"starlark":      help.Starlark,
		"builtins":      help.Builtins,
		"policy":        helpPolicy,
		"policy.list":   helpPolicyList,
		"policy.get":    helpPolicyGet,
		"policy.set":    helpPolicySet,
		"policy.create": helpPolicyCreate,
		"policy.clone":  helpPolicyClone,
		"policy.find":   helpPolicyFind,
		"policy.delete": helpPolicyDelete,
		"category":      helpCategory,
		"group":         helpGroup,
		"profile":       helpProfile,
		"package":       helpPackage,
		"computer":      helpComputer,
		"mdm":           helpMdm,
		"blueprint":     helpBlueprint,
		"device":        helpDevice,
		"device-group":  helpDeviceGroup,
		"ddm":           helpDDM,
		"benchmark":     helpBenchmark,
		"role":          helpRole,
		"client":        helpClient,
		"script":        helpScript,
		"script.get":    helpScriptGet,
		"script.set":    helpScriptSet,
		"setup":         helpSetup,
		"serve":         helpServe,
		"mcp":           helpMCP,
		"lsp":           helpLSP,
	}
	if text, ok := helpMap[key]; ok {
		fmt.Print(text)
	} else {
		fmt.Fprintf(os.Stderr, "No help available for %q\n\n", strings.Join(path, " "))
		printUsage()
	}
}

// policySubHelp returns the help text for a policy subcommand.
func policySubHelp(mode string) string {
	switch mode {
	case "list":
		return helpPolicyList
	case "get":
		return helpPolicyGet
	case "set":
		return helpPolicySet
	case "create":
		return helpPolicyCreate
	case "clone":
		return helpPolicyClone
	case "find":
		return helpPolicyFind
	case "delete":
		return helpPolicyDelete
	default:
		return helpPolicy
	}
}

// scriptSubHelp returns the help text for a script subcommand.
func scriptSubHelp(action string) string {
	switch action {
	case "get":
		return helpScriptGet
	case "set":
		return helpScriptSet
	default:
		return helpScript
	}
}
