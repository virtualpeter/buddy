package main

import (
	"fmt"
	"os"
	"strings"

	"buddy/internal/help"
)

const helpVersion = `buddy version - Print version and exit

Usage:
  buddy version
  buddy --version

Notes:
  Same output as --version. Does not load config or contact Jamf.
`

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

// helpPolicy is generated (make gendocs).
var helpPolicy = help.PoliciesCLIHelp


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

// helpMdm is generated (make gendocs).
var helpMdm = help.MdmCLIHelp


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

// helpCategory is generated (make gendocs).
var helpCategory = help.CategoriesCLIHelp


// helpMacApp is generated (make gendocs).
var helpMacApp = help.MacappsCLIHelp


// helpApp is generated from internal/buddy/app_doc.go (make gendocs).
var helpApp = help.AppsCLIHelp

// helpGroup is generated (make gendocs).
var helpGroup = help.GroupsCLIHelp


// helpProfile is generated (make gendocs).
var helpProfile = help.ProfilesCLIHelp


// helpBlueprint is generated (make gendocs).
var helpBlueprint = help.BlueprintsCLIHelp


// helpDevice is generated (make gendocs).
var helpDevice = help.DevicesCLIHelp


// helpDeviceGroup is generated (make gendocs).
var helpDeviceGroup = help.DeviceGroupsCLIHelp


// helpDDM is generated (make gendocs).
var helpDDM = help.DdmCLIHelp


// helpBenchmark is generated (make gendocs).
var helpBenchmark = help.BenchmarksCLIHelp


// helpPackage is generated (make gendocs).
var helpPackage = help.PackagesCLIHelp


// helpComputer is generated (make gendocs).
var helpComputer = help.ComputersCLIHelp


// helpRole is generated (make gendocs).
var helpRole = help.RolesCLIHelp


// helpClient is generated (make gendocs).
var helpClient = help.ClientsCLIHelp


// helpScript is generated (make gendocs).
var helpScript = help.ScriptsCLIHelp


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
		"version":       helpVersion,
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
		"macapp":        helpMacApp,
		"app":           helpApp,
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
