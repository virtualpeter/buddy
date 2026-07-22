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

const helpCred = `buddy cred - Manage credentials in the config file

Usage:
  buddy cred <subcommand> [args...]

Subcommands:
  list                          List all credentials (name, URL, auth type, region, tenant)
  get <name>                    Show one credential's details (secrets redacted)
  add [flags]                   Add or replace a credential (upsert; flag-only)
  delete <name>                 Remove a credential
  default [name]                Show current default, or set a new one
  secret <subcommand>           Manage secrets in the platform store (see 'buddy cred secret help')

Flags for 'add':
  --name=<name>                 Credential name (required)
  --url=<url>                   Jamf Pro instance or gateway URL (required unless --region)
  --region=<region>             Platform API Gateway region (us|eu|apac|…); implies oauth + gateway
  --type=command|basic|oauth    Auth type (required unless --region, which defaults to oauth)
  --run=<cmd>                   Command to run; stdout is used as the Bearer token (type: command)
  --username=<u>                Username (type: basic)
  --password=<p>                Password inline in config (type: basic; dev/test)
  --client-id=<id>              OAuth client ID (type: oauth)
  --client-secret=<s>           OAuth client secret inline in config (type: oauth; dev/test)
  --tenant-id=<tenant-id>       Platform tenant id (required with --region or a gateway --url)

Notes:
  Flags accept --flag=value or --flag value. Positionals are rejected for 'add'.
  For production, omit --password and --client-secret from 'add' and store secrets with
  'buddy cred secret set'. macOS uses Keychain; Linux uses ~/.config/buddy/secrets/
  (override with BUDDY_SECRET_STORE=file|pass|keychain).
  Inline secrets in config.yaml still work for dev/test. Never commit secrets.
  Platform credentials: prefer --region=us --tenant-id=<id> --client-id=<id> (url is built as
  https://{region}.apigw.jamf.com). Pass --url only to override (e.g. a proxy). Gateway mode
  is oauth-only; tenantId is required.

Examples:
  buddy cred list
  buddy cred get default
  buddy cred add --name=sandbox --url=https://example-sandbox.jamfcloud.com --type=oauth --client-id=abc
  buddy cred add --name=gateway --region=us --tenant-id=e77c1408-10c8-4007-b177-abc9157fbcaa --client-id=abc
  buddy cred add --name=gateway --region=us --url=https://proxy.example.com --tenant-id=e77c1408-10c8-4007-b177-abc9157fbcaa --client-id=abc
  buddy cred secret set sandbox --client-secret
  buddy cred secret path
  buddy cred add --name=staging --url=https://example-staging.jamfcloud.com --type=basic --username=admin
  buddy cred secret set staging --password
  buddy cred delete staging
  buddy cred default
  buddy cred default dev
`

const helpCredSecret = `buddy cred secret - Manage credential secrets outside config.yaml

Usage:
  buddy cred secret <subcommand> [args...]

Subcommands:
  set <name> [flags]            Store password and/or client secret in the platform store
  get <name>                    Show where secrets are stored (values not shown)
  delete <name> [flags]         Remove secrets from the platform store
  path                          Show active secret backend and location

Flags for 'set':
  --password[=value]              Store basic-auth password (prompt if omitted)
  --client-secret[=value]       Store OAuth client secret (prompt if omitted)
  --secret-from-stdin           Read secret value from stdin (with --password or --client-secret)

Flags for 'delete':
  --password                    Delete stored password only
  --client-secret               Delete stored client secret only

Notes:
  Default store: macOS Keychain; Linux file at ~/.config/buddy/secrets/<name>.json.
  Set BUDDY_SECRET_STORE=pass to use pass(1) instead (Linux).
  Config keeps url, auth type, username, and clientId; secrets load by credential name.

Examples:
  buddy cred secret path
  buddy cred secret set sandbox --client-secret
  buddy cred secret set staging --password=secret
  echo 'my-secret' | buddy cred secret set sandbox --client-secret --secret-from-stdin
  buddy cred secret get sandbox
  buddy cred secret delete sandbox --client-secret
`

const helpPolicy = `buddy policy - Manage Jamf policies

Usage:
  buddy [global-flags] policy <subcommand> [args...]

Subcommands:
  get     Read policy properties (--trigger, enable state, xml)
  set     Modify policy properties (scope, enable/disable, trigger, name, ...)
  clone   Clone a policy with a new version
  create  Create a new policy
  delete  Delete one or more policies
  find    Search policies by keyword

Use 'buddy policy <subcommand> help' for subcommand-specific help.

Examples:
  buddy policy get --trigger=install-chrome
  buddy policy find chrome
  buddy policy delete 42
`

const helpPolicyGet = `buddy policy get - Read policy properties

Usage:
  buddy policy get --trigger=<trigger> [-f|--force-refresh]
  buddy policy get enable <policy-id>
  buddy policy get xml <policy-id>

Subcommands:
  enable <policy-id>   Show enabled/disabled state
  xml <policy-id>      Output policy XML (PUT payload)

Flags for '--trigger' lookup:
  --trigger=<name>     Custom trigger name (also: --trigger <name>)
  -f, --force-refresh   Bypass local policy cache

Examples:
  buddy policy get --trigger=install-chrome
  buddy policy get --trigger install-chrome -f
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
  buddy policy set package <app> <version> [policy-id] [--trigger=<trigger>] [--package=<pattern>]
  buddy policy set xml <policy-id>

Examples:
  buddy policy set enable 42
  buddy policy set disable 42
  buddy policy set scope 42 all
  buddy policy set scope 42 10 11
  buddy policy set name 42 "Install Chrome 121"
  buddy policy set trigger 42 install-chrome
  buddy policy set frequency 42 "Once per computer"
  buddy policy set install 42 121.0 --param=4,5
  buddy policy set package chrome 121.0
  buddy policy set xml 42 < policy.xml
`

const helpPolicyCreate = `buddy policy create - Create a new policy

Usage:
  buddy policy create package <app> <version> [flags]
        Flags: [--action=install|cache] [--scope=all|<group-id>[,<group-id>...]]
               [--category=<name>] [--package=<pattern>]
  buddy policy create ss <trigger> <display-name> [--category=<name>]

Examples:
  buddy policy create package chrome 121.0
  buddy policy create package chrome 121.0 --action=install --scope=all
  buddy policy create ss install-chrome "Install Chrome" --category="Mac Apps"
`

const helpPolicyClone = `buddy policy clone - Clone a policy with a new version

Usage:
  buddy policy clone <policy-id> <version> [--param=4,5,6] [--package=<pattern>]

Examples:
  buddy policy clone 42 121.0
  buddy policy clone 42 121.0 --param=4,5
  buddy policy clone 42 121.0 --package=Chrome
`

const helpPolicyFind = `buddy policy find - Search policies by keyword

Usage:
  buddy policy find <keyword> [keyword...] [--exclude-ids=1,2,3] [-f|--force-refresh]

Arguments:
  <keyword>             One or more keywords to match against policy names/triggers
  --exclude-ids=1,2,3   Exclude specific policy IDs from results
  -f, --force-refresh   Bypass local cache and re-fetch from Jamf

Examples:
  buddy policy find chrome
  buddy policy find chrome install
  buddy policy find chrome --exclude-ids=100,101 -f
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
  find   Search MDM commands using an RSQL filter

Flags for 'find':
  --filter=<rsql>   RSQL filter expression (required; API requires at least one field)
  --sort=<expr>     Sort expression (default: dateSent:asc)
  --count           Fetch all matching records and print only the total count (table suppressed)

RSQL filterable fields:
  uuid, clientManagementId, command, status, clientType, dateSent,
  validAfter, dateCompleted, profileId, profileIdentifier, active

  Date fields require gt/lt/ge/le operators (not ==).

Examples:
  buddy mdm find --filter="status==Pending"
  buddy mdm find --filter="clientManagementId==00000000-0000-4000-8000-000000000002"
  buddy mdm find --filter="status==Pending;clientType==COMPUTER" --sort="dateSent:desc"
  buddy mdm find --filter="dateCompleted=ge=2024-01-01T00:00:00Z;status==Acknowledged"
  buddy mdm find --filter="status==Pending" --count
`

const helpSetup = `buddy setup - Write scripts and configure OAuth for buddy

Usage:
  buddy setup write-script [path]
  buddy setup configure-oauth [--user=<name>] [--client-name=buddy-cli] [--name=buddy-oauth] [-y]

  write-script [path]
    Writes a Starlark script that creates a Jamf API role with privileges needed
    for buddy and an API client (integration) scoped to that role. Default path
    is setup_buddy_role.buddy.

    Run the generated script with:
      ./<path> -y
      buddy <path> -y
    Then add an OAuth credential to config (see configure-oauth below), or get
    the client secret (shown once) with:
      buddy client credentials <client-id>

    If role creation fails (e.g. privilege names differ on your Jamf version), run
    "buddy role privileges" to see your instance's list and edit the script.

    The role may include destructive privileges (e.g. wipe computer, erase) so a
    single script can cover all buddy features. After setup, review the role in Jamf
    and remove API privileges you do not need before treating the credential as
    production-ready.

  configure-oauth
    Finds the API integration by display name (default: buddy-cli), generates new
    OAuth client credentials via the Jamf API, and adds a credential to
    ~/.config/buddy/config.yaml with type oauth so you can use it without
    storing the secret elsewhere.

    Flags for 'configure-oauth':
      --user=<name>         Credential to use for API calls (default: current/--user)
      --client-name=<name>  API integration display name to find (default: buddy-cli)
      --name=<name>         Name for the new config credential (default: buddy-oauth)
      -y                    Skip confirmation (recommended in scripts)

    Example:
      buddy setup configure-oauth -y
      buddy --user buddy-oauth auth test
`

const helpGroup = `buddy group - Manage Jamf computer groups

Usage:
  buddy [global-flags] group <list|get|set|create|delete|static> [args...]

Subcommands:
  list smart [--filter=<rsql>]           List smart computer groups
  list static [--filter=<rsql>]          List static computer groups
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
  get payload <profile-id> <domain>                          Output single payload by PayloadIdentifier
  get list-payloads <profile-id>                            List PayloadIdentifier values (domains)
  set xml <profile-id> [--redeploy]                         PUT profile from XML on stdin
  set payload <profile-id> <domain> <dict-file.xml> [--redeploy]   Replace payload for domain (PayloadIdentifier or MCX)
  clone <profile-id> <new-name>                             Clone profile with new name, no scope
  redeploy <profile-id>                                     Redeploy to all assigned devices

Notes:
  --redeploy sets redeploy_on_update=All (pushes to all assigned devices, not just new ones)
  get payload / set payload work with both PayloadContent (PayloadIdentifier) and MCX-style payloads.

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

const helpPackage = `buddy package - Manage Jamf Pro package records

Usage:
  buddy [global-flags] package <subcommand> [args...]

Subcommands:
  list [--filter=<rsql>]                              List packages
  get <package-id>                                    Show package record details
  history <package-id>                                Show recent change history (last 20 entries)
  delete <package-id> [package-id...]                Delete one or more package records
  upload <file> [--name=...] [--category=<category-id>] [--priority=1-20]  Upload .pkg or .dmg
  set name     <package-id> <value>                   Set display name
  set notes    <package-id> <value>                   Set notes field
  set info     <package-id> <value>                   Set info field
  set priority <package-id> <1-10>                    Set install priority
  set category <package-id> <category-id>             Set category (-1 to clear)
  set reboot   <package-id> <true|false>              Set rebootRequired flag

Examples:
  buddy package list
  buddy package list --filter='packageName=="*Chrome*"'
  buddy package get 55
  buddy package history 55
  buddy package upload Chrome-121.pkg --name="Google Chrome 121" --category=3
  buddy package set name 55 "Google Chrome 121"
  buddy package set priority 55 5
  buddy package delete 55 56
`

const helpComputer = `buddy computer - Manage computers and send MDM commands

Usage:
  buddy [global-flags] computer <subcommand> [args...]

Subcommands:
  find [--filter=<rsql>] [--management-id=<uuid>]    List computers (RSQL filter optional; --management-id looks up by MDM management UUID)
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
  buddy computer find
  buddy computer find --filter='general.name=="*Example*"'
  buddy computer find --management-id=00000000-0000-4000-8000-000000000003
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
		"cred.secret":   helpCredSecret,
		"config":        helpCred, // deprecated alias
		"config.secret": helpCredSecret,
		"shell":         help.Shell,
		"starlark":      help.Starlark,
		"builtins":      help.Builtins,
		"policy":        helpPolicy,
		"policy.get":    helpPolicyGet,
		"policy.set":    helpPolicySet,
		"policy.create": helpPolicyCreate,
		"policy.clone":  helpPolicyClone,
		"policy.find":   helpPolicyFind,
		"policy.delete": helpPolicyDelete,
		"group":         helpGroup,
		"profile":       helpProfile,
		"package":       helpPackage,
		"computer":      helpComputer,
		"mdm":           helpMdm,
		"blueprint":     helpBlueprint,
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
