package main

import (
	"fmt"
	"os"
	"strings"
)

const helpRoot = `buddy - Jamf Pro management CLI

Usage:
  buddy [global-flags]                              Interactive Starlark shell
  buddy [global-flags] <script.star> [-- args...]   Run a Starlark script
  buddy [global-flags] <command> [subcommand] [args...]
  buddy help [command] [subcommand]

Commands:
  auth     Test authentication for a credential
  cred     Manage credentials in the config file
  policy   Manage Jamf policies (get/set/clone/create/delete/find)
  group    Manage computer groups (list/get/set/create/delete; static membership)
  profile  Manage configuration profiles (list/get/set/clone/redeploy)
  package  Manage package records (list/get/upload/delete/set/history)
  computer Manage computers and MDM commands (find/get/reboot/lock/erase/...)
  mdm      Query MDM commands from Jamf Pro (find with RSQL filter)
  role     Manage API roles (list/get/create/update/delete/privileges)
  client   Manage API integrations (list/get/create/update/delete/credentials/set)
  script   Manage Jamf scripts (list/get/create/upload/download/delete/set)
  setup    Write setup script (role + client for buddy)

Global flags:
  --version           Print version and exit
  -y                  Skip confirmation prompts (buddy CLI and confirm() in scripts)
  -v, --verbose       Verbose output
  -t, --trace         Trace HTTP requests and responses to stderr (headers and bodies; Authorization/Cookie redacted)
  --script-trace      Trace Starlark builtin calls and print() in scripts and the interactive shell
  --user <name>       Credential/user name from config (default: from ~/.config/buddy/config.yaml)

Use 'buddy help starlark' or 'buddy help builtins' for Starlark scripting reference.
Use 'buddy help <command>' or 'buddy <command> help' for command details.
Use 'buddy <command> <subcommand> help' for subcommand details.
`

const helpAuth = `buddy auth - Obtain and test API authentication

Usage:
  buddy [global-flags] auth <subcommand> [args...]

Subcommands:
  test                         GET /api/v1/auth and pretty-print the response
  token [flags]                Print Authorization header or raw bearer token
  refresh [flags] [<token>]    Refresh a bearer token via keep-alive

Flags for 'token':
  --raw                        Print bearer token only (for scripts and token_command)
  --url=<url>                  Jamf Pro base URL (direct auth, no config credential)
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
  buddy auth refresh eyJhbGciOiJIUzI1NiJ9...
  buddy --user dev auth refresh --token=eyJhbGciOiJIUzI1NiJ9...
`

const helpCred = `buddy cred - Manage credentials in the config file

Usage:
  buddy cred <subcommand> [args...]

Subcommands:
  list                          List all credentials (name, URL, auth type)
  get <name>                    Show one credential's details (secrets redacted)
  add <name> <url> [flags]      Add or replace a credential (upsert)
  delete <name>                 Remove a credential
  default [name]                Show current default, or set a new one
  secret <subcommand>           Manage secrets in the platform store (see 'buddy cred secret help')

Flags for 'add':
  --type=command|basic|oauth    Auth type (required)
  --run=<cmd>                   Command to run; stdout is used as the Bearer token (type: command)
  --username=<u>                Username (type: basic)
  --password=<p>                Password inline in config (type: basic; dev/test)
  --client-id=<id>              OAuth client ID (type: oauth)
  --client-secret=<s>           OAuth client secret inline in config (type: oauth; dev/test)

Notes:
  For production, omit --password and --client-secret from 'add' and store secrets with
  'buddy cred secret set'. macOS uses Keychain; Linux uses ~/.config/buddy/secrets/
  (override with BUDDY_SECRET_STORE=file|pass|keychain).
  Inline secrets in config.yaml still work for dev/test. Never commit secrets.

Examples:
  buddy cred list
  buddy cred get default
  buddy cred add sandbox https://example-sandbox.jamfcloud.com --type=oauth --client-id=abc
  buddy cred secret set sandbox --client-secret
  buddy cred secret path
  buddy cred add staging https://example-staging.jamfcloud.com --type=basic --username=admin
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

const helpShell = `buddy - Starlark shell and scripts

Usage:
  buddy [global-flags]                              Interactive Starlark shell
  buddy [global-flags] [--trace] <script.star> [-- args...]

Arguments:
  <script.star>   Path to a Starlark (.star) script file
  [-- args...]    Optional arguments passed through to the script (available as args)

Flags:
  --trace         Positional flag before <script.star>: log each buddy builtin call to
                  stderr with its source position, arguments, and return value. Also logs
                  a trace line to stderr for each Starlark print() (the message itself
                  goes to stdout). Buddy's error() writes to stderr only.

Notes:
  With no arguments, buddy starts an interactive Starlark REPL on stdin/stdout.

  Global -y skips interactive confirmation for buddy CLI commands and makes confirm()
  in scripts return True without reading stdin.

  --script-trace enables the same builtin tracing as positional --trace (for scripts
  and the REPL). HTTP tracing (-t / --trace) is separate: use "buddy -t deploy.star"
  to trace Jamf API traffic from builtins.

  Use 'buddy help starlark' and 'buddy help builtins' for language and API reference.

Examples:
  buddy
  buddy deploy.star
  buddy deploy.star -- --dry-run
  buddy --script-trace deploy.star
  buddy -t deploy.star
`

const helpRunStarlark = `Starlark language quick-reference
==================================
Starlark is a deterministic, Python-like scripting language. It is a strict
subset of Python 3 — most Python code that avoids classes, imports, and
generators will work unchanged.

TYPES
  None   bool   int   float   str   bytes   list   dict   tuple

LITERALS
  42        3.14      True   False   None
  "hello"   'world'   """multi\nline"""
  [1, 2]              # list
  {"a": 1}            # dict
  (1, 2)              # tuple (immutable)

VARIABLES & ASSIGNMENT
  x = 1
  a, b = 1, 2         # tuple unpacking
  x += 1              # augmented assignment (+=, -=, *=, //=, %=)

OPERATORS
  Arithmetic: + - * / // % ** (no bit-shift)
  Comparison: == != < <= > >= in not in
  Logical:    and or not
  String:     + (concat)  * (repeat)  % (format)  in (contains)

CONTROL FLOW
  if x > 0:
      print("pos")
  elif x == 0:
      print("zero")
  else:
      print("neg")

  for item in [1, 2, 3]:
      if item == 2: continue
      print(item)

  # break, continue, pass supported; no while loop

FUNCTIONS
  def greet(name, greeting="Hello"):
      return greeting + ", " + name

  def variadic(*args, **kwargs):
      for a in args: print(a)
      for k, v in kwargs.items(): print(k, v)

  greet("Alice")
  greet("Bob", greeting="Hi")

STRING METHODS  (strings are immutable)
  s.upper()  s.lower()  s.title()  s.strip()  s.lstrip()  s.rstrip()
  s.split(sep?)  s.splitlines()  s.join(iterable)
  s.startswith(prefix)  s.endswith(suffix)
  s.replace(old, new)  s.find(sub)  s.count(sub)
  s.format(*args, **kwargs)   # "{0} {name}".format("hi", name="there")
  s[i]  s[i:j]  len(s)  s in other_str

LIST METHODS  (lists are mutable)
  l.append(x)  l.extend(iterable)  l.insert(i, x)
  l.pop(i?)    l.remove(x)        l.clear()
  l.index(x)   l.count(x)        l.reverse()  l.sort(key?, reverse?)
  l[i]  l[i:j]  len(l)  x in l

DICT METHODS  (dicts are mutable)
  d.get(key, default?)  d.setdefault(key, default?)
  d.keys()  d.values()  d.items()
  d.update(other)  d.pop(key, default?)  d.clear()
  d[key]  key in d  len(d)

BUILT-IN FUNCTIONS
  print(*args)         error(*args)     abs(x)           bool(x)
  int(x, base?)        float(x)         str(x)
  len(x)               type(x)          repr(x)
  range(stop)          range(start,stop,step?)
  list(iterable)       dict(**kwargs)   tuple(iterable)
  enumerate(iterable)  zip(*iterables)  reversed(iterable)
  sorted(iterable, key?, reverse?)
  min(iterable)        max(iterable)    sum(iterable, start?)
  any(iterable)        all(iterable)    hasattr(obj, name)
  getattr(obj, name, default?)          dir(x)

  I/O: print() goes to stdout. Buddy adds error(*args, sep=...) like print but to stderr.

WHAT IS NOT IN STARLARK (vs Python)
  No:  import  class  try/except  while  lambda  yield  global  nonlocal
  No:  set  frozenset  bytearray  file I/O  os/sys modules
  No:  is  is not  (use == != instead)

Use 'buddy help builtins' for buddy-specific functions.
`

const helpRunBuiltins = `buddy built-in functions for Starlark scripts
===============================================
All functions return a result dict: {"ok": True, "value": ...}
                                 or {"ok": False, "error": "..."}
Check ok before using value.

Conventions: Resource IDs come first (e.g. policy_id, group_id, script_id,
computer_id); payload/options next; cred? is always last and optional.
Use cred= to override the active credential for that call.

Each builtin lists its signature, then → value (or None). Indented "Params:"
lines describe arguments; "Returns:" states the value shape when ok.

POLICY — read
  policy_get_id(trigger, force_refresh?)
      → value: int policy ID
      Params: trigger: str (policy trigger name). force_refresh?: bool (bypass cache). Returns: policy ID.
  policy_get_enable(policy_id)
      → value: "true" or "false"
      Params: policy_id: int. Returns: enabled state string.
  policy_get_details(policy_id, cred?)
      → value: dict with keys name, enabled, trigger, frequency, scope, ...
      Params: policy_id: int. cred?: credential name. Returns: dict of policy details.

POLICY — write
  policy_set_ss(policy_id, version?, group_ids?, display_name?,
                description?, install_text?, reinstall_text?,
                force_view?, feature?, self_service?, cred?)
      → None
      Params: policy_id: int; version?: str; group_ids?: list of int; display_name?, description?, install_text?, reinstall_text?: str; force_view?, feature?, self_service?: bool; cred?: credential name.
  policy_set_enable(policy_id, cred?)
      → None
      Params: policy_id: int. cred?: credential name.
  policy_set_disable(policy_id, cred?)
      → None
      Params: policy_id: int. cred?: credential name.
  policy_set_trigger(policy_id, trigger, cred?)
      → None
      Params: policy_id: int; trigger: str. cred?: credential name.
  policy_set_name(policy_id, new_name, cred?)
      → None
      Params: policy_id: int; new_name: str. cred?: credential name.
  policy_set_frequency(policy_id, frequency, cred?)
      → None
      Params: policy_id: int; frequency: str (e.g. "Once per computer", "Ongoing"). cred?: credential name.
  policy_set_install(policy_id, version, cred?)
      → None
      Params: policy_id: int; version: str. cred?: credential name.
  policy_set_package(app, version, policy_id?, trigger?,
                     package_pattern?, cred?)
      → None
      Params: app: str (e.g. "chrome"); version: str; policy_id?: int; trigger?: str; package_pattern?: str; cred?: credential name.
  policy_set_scope_all(policy_id, cred?)
      → None
      Params: policy_id: int. cred?: credential name.
  policy_clone(policy_id, version, cred?)
      → None
      Params: policy_id: int; version: str. cred?: credential name.
  policy_create_ss(trigger, display_name, category?, scope_group_id?, script_id?, cred?)
      → value: int (new policy ID)
      Params: trigger: str; display_name: str; category?: str; scope_group_id?: int (0 = all computers); script_id?: int (Jamf script ID, default 0 = no script attached; trigger is passed as parameter4 when script_id > 0). cred?: credential name.
  policy_delete(policy_ids, cred?)
      → None
      Params: policy_ids: list of int. cred?: credential name.
  policy_find_ids(keywords, exclude_ids?, force_refresh?, cred?)
      → value: list of int policy IDs
      Params: keywords: str or list; exclude_ids?: list of int; force_refresh?: bool. cred?: credential name.

POLICY — component
  policy_fetch_xml(policy_id, cred?)
      → value: str (raw policy XML)
      Params: policy_id: int. cred?: credential name.
  policy_put_xml(policy_id, xml_str, cred?)
      → None
      Params: policy_id: int; xml_str: str (full policy XML body). cred?: credential name.

GROUP
  group_list_smart(filter?, cred?)
      → value: list of dicts with id, name, description
      Params: filter?: str (name substring or RSQL). cred?: credential name.
  group_list_static(filter?, cred?)
      → value: list of dicts with id, name, description
      Params: filter?: str. cred?: credential name.
  group_find_id(name, cred?)
      → value: dict {"id": str, "name": str} (searches smart groups)
      Params: name: str. cred?: credential name.
  group_get_version(group_id, cred?)
      → value: str (version criterion)
      Params: group_id: str. cred?: credential name.
  group_set_version(group_id, version, cred?)
      → None
      Params: group_id: str; version: str. cred?: credential name.
  group_delete(group_id, cred?)
      → None
      Params: group_id: str. cred?: credential name.
  group_fetch(group_id, cred?)
      → value: dict (full smart-group JSON)
      Params: group_id: str. cred?: credential name.
  group_put(group_id, dict, cred?)
      → None (use group_id="-1" to create; prefer smart_group_create)
      Params: group_id: str; dict: Starlark dict (group body). cred?: credential name.

  smart_group_fetch(group_id, cred?)
      → value: dict (full smart-group JSON from /api/v2/computer-groups/smart-groups/{id})
      Params: group_id: str; cred?: credential name.
  smart_group_create(body, cred?)
      → value: dict {"id": str, "href"?: str}
      Params: body: dict (smart group JSON; typically name + criteria; id: -1 optional). cred?: credential name.
  smart_group_put(group_id, body, cred?)
      → None
      Params: group_id: str; body: dict/list tree (full smart group JSON); cred?: credential name.
  smart_group_get_criteria(group_id, cred?)
      → value: list[dict] (criteria array)
      Params: group_id: str; cred?: credential name. Each criterion: {name: str, priority: int, andOr: str, searchType: str, value: str, openingParen?: bool, closingParen?: bool}.
  smart_group_set_criteria(group_id, criteria, cred?)
      → None
      Params: group_id: str; criteria: list[dict] as above; cred?: credential name.
  smart_group_find_criteria(group_id, name_substring, cred?)
      → value: list[dict] (criteria whose name contains name_substring, case-insensitive)
      Params: group_id: str; name_substring: str; cred?: credential name.
  smart_group_update_criterion_value(group_id, name_substring, new_value, cred?)
      → None
      Params: group_id: str; name_substring: str; new_value: str; cred?: credential name.

  static_group_list_computers(group_id, filter?, cred?)
      → value: list of dicts with id, name, serial (name/serial may be empty if API returns IDs only)
      Params: group_id: str; filter?: str (client-side substring on id, name, serial). cred?: credential name.
  static_group_create(body, cred?)
      → value: dict {"id": str, "href"?: str}
      Params: body: dict (static group JSON; name required; assignments?: list of computer id str). cred?: credential name.
  static_group_add_computers(group_id, computer_ids, cred?)
      → None
      Params: group_id: str; computer_ids: list of int or str. cred?: credential name.
  static_group_remove_computers(group_id, computer_ids, cred?)
      → None
      Params: group_id: str; computer_ids: list of int or str. cred?: credential name.
  static_group_set_computers(group_id, computer_ids, cred?)
      → None
      Params: group_id: str; computer_ids: list of int or str (replaces full membership). cred?: credential name.

PACKAGE
  package_list(filter?)
      → value: list of dicts
      Params: filter?: str (RSQL). Returns: list of package records.
  package_find_id(app, version, pattern?)
      → value: dict {"id": str, "name": str}
      Params: app: str; version: str; pattern?: str. Returns: match dict.
  package_upload(file, name?, category_id?, priority?, cred?)
      → value: dict (created package)
      Params: file: str (path); name?, category_id?, priority?: optional. cred?: credential name.
  package_delete(ids)
      → None
      Params: ids: list of str (package IDs). Returns: None.
  package_fetch(package_id, cred?)
      → value: dict (full package JSON)
      Params: package_id: str. cred?: credential name.
  package_put(package_id, dict, cred?)
      → value: dict (updated package JSON)
      Params: package_id: str; dict: Starlark dict (fields to update). cred?: credential name.

PROFILE
  profile_list(filter?)
      → value: list of dicts
      Params: filter?: str (name substring). Returns: list of profile records.
  profile_find_id(name)
      → value: dict {"id": int, "name": str}
      Params: name: str.
  profile_clone(profile_id, name, cred?)
      → value: dict (new profile)
      Params: profile_id: int; name: str (new name). cred?: credential name.
  profile_redeploy(profile_id)
      → None
      Params: profile_id: int.
  profile_get_payloads(profile_id)
      → value: str (payloads plist XML)
      Params: profile_id: int.
  profile_get_payload(profile_id, domain)
      → value: str (single payload plist)
      Params: profile_id: int; domain: str (PayloadIdentifier).
  profile_list_payloads(profile_id)
      → value: list of str (PayloadIdentifiers)
      Params: profile_id: int.
  profile_set_payload(profile_id, domain, dict_file, redeploy?)
      → None
      Params: profile_id: int; domain: str; dict_file: str (path to plist XML); redeploy?: bool.
  profile_set_xml(profile_id, xml_file, redeploy?)
      → None
      Params: profile_id: int; xml_file: str (path); redeploy?: bool.

PROFILE — component
  profile_fetch_xml(profile_id, cred?)
      → value: str (raw profile XML)
      Params: profile_id: int. cred?: credential name.
  profile_put_xml(profile_id, xml_str, redeploy?, cred?)
      → None
      Params: profile_id: int; xml_str: str; redeploy?: bool. cred?: credential name.
  profile_xml_extract_payloads(profile_xml_str)
      → value: str (payloads plist XML from <payloads>; entities unescaped)
      Params: profile_xml_str: str (offline transform; no API call).
  profile_xml_replace_payloads(profile_xml_str, payloads_plist_str)
      → value: str (updated profile XML)
      Params: profile_xml_str: str; payloads_plist_str: str (offline transform).
  profile_xml_set_redeploy_all(profile_xml_str)
      → value: str (updated profile XML)
      Params: profile_xml_str: str. Sets <redeploy_on_update>All</redeploy_on_update>.
  profile_xml_patch_payload(profile_xml_str, domain, new_payload)
      → value: str (updated profile XML)
      Params: profile_xml_str: str; domain: str; new_payload: dict or plist/dict XML (PayloadContent or MCX).
  payloads_list_identifiers(payloads_plist_str)
      → value: list of str
      Params: payloads_plist_str: str (payloads plist XML).
  payloads_get_payload(payloads_plist_str, domain)
      → value: str (plist XML dict)
      Params: payloads_plist_str: str; domain: str (PayloadIdentifier).
  payloads_replace_payload(payloads_plist_str, domain, new_payload)
      → value: str (payloads plist XML)
      Params: payloads_plist_str: str; domain: str; new_payload: dict or plist XML.
  payloads_replace_mcx_dict(payloads_plist_str, domain, dict_xml_str)
      → value: str (payloads plist XML)
      Params: payloads_plist_str: str; domain: str; dict_xml_str: str.
  payloads_set_content(content, payloads_plist_str?)
      → value: str (payloads plist XML)
      Params: content: list of payload dicts; payloads_plist_str?: str (omit/empty to start fresh).
  payloads_add_payload(payloads_plist_str, domain, new_payload)
      → value: str (payloads plist XML)
      Params: payloads_plist_str: str (empty ok); domain: str; new_payload: dict or plist XML. Appends or replaces by PayloadIdentifier.
  payloads_remove_payload(payloads_plist_str, domain)
      → value: str (payloads plist XML)
      Params: payloads_plist_str: str; domain: str.
  plist_normalise_dict(data_str)
      → value: str (bare <dict>…</dict>)
      Params: data_str: str (bare dict fragment or full plist).

SCRIPT (Jamf Pro scripts)
  script_list(filter?)
      → value: list of dicts
      Params: filter?: str (name substring). Returns: list of script records.
  script_get_id(name)
      → value: str script ID
      Params: name: str.
  script_get_contents(script_id, cred?)
      → value: str (script body)
      Params: script_id: str. cred?: credential name.
  script_update(script_id, file, cred?)
      → None
      Params: script_id: str; file: str (path to script file). cred?: credential name.
  script_fetch(script_id, cred?)
      → value: dict (full script JSON)
      Params: script_id: str. cred?: credential name.
  script_put(script_id, dict, cred?)
      → value: dict (updated script JSON)
      Params: script_id: str; dict: Starlark dict (fields to update). cred?: credential name.

COMPUTER
  computer_find_ids(filter?, cred?)
      → value: list of int IDs
      Params: filter?: str (RSQL). cred?: credential name.
  computer_get(computer_id, cred?)
      → value: dict (id, name, serialNumber, udid, managementId, ...)
      Params: computer_id: int or str. cred?: credential name.
  computer_fetch(computer_id, cred?)
      → value: dict (alias of computer_get)
      Params: computer_id: int or str. cred?: credential name.
  computer_send_mdm_command(computer_ids, command_type, command_data?, cred?)
      → None
      Params: computer_ids: list of int; command_type: str (e.g. "Reboot", "Erase"); command_data?: dict. cred?: credential name.
  computer_reboot(computer_ids, cred?)
      → None
      Params: computer_ids: list of int. cred?: credential name.
  computer_redeploy(computer_ids, cred?)
      → None
      Params: computer_ids: list of int. cred?: credential name.
  computer_shutdown(computer_ids, cred?)
      → None
      Params: computer_ids: list of int. cred?: credential name.
  computer_lock(computer_ids, message?, phone?, pin?, cred?)
      → None
      Params: computer_ids: list of int; message?, phone?, pin?: optional str. cred?: credential name.
  computer_logout(computer_ids, cred?)
      → None
      Params: computer_ids: list of int. cred?: credential name.
  computer_erase(computer_ids, pin?, obliteration?, cred?)
      → None
      Params: computer_ids: list of int; pin?: str; obliteration?: str. cred?: credential name.
  computer_device_info(computer_ids, queries?, cred?)
      → value: dict or list (queued command result)
      Params: computer_ids: list of int; queries?: list of str. cred?: credential name.
  computer_device_location(computer_ids, cred?)
      → value: dict or list (queued command result)
      Params: computer_ids: list of int. cred?: credential name.
  computer_security_info(computer_ids, cred?)
      → value: dict or list (queued command result)
      Params: computer_ids: list of int. cred?: credential name.
  computer_installed_apps(computer_ids, identifiers?, cred?)
      → value: dict or list (queued command result)
      Params: computer_ids: list of int; identifiers?: list. cred?: credential name.
  computer_managed_apps(computer_ids, identifiers?, cred?)
      → value: dict or list (queued command result)
      Params: computer_ids: list of int; identifiers?: list. cred?: credential name.
  computer_certificates(computer_ids, cred?)
      → value: dict or list (queued command result)
      Params: computer_ids: list of int. cred?: credential name.

MDM
  mdm_command_find(filter, sort?, page_size?, limit?, cred?)
      → value: list of dicts (MDM command records)
      Params: filter: str (RSQL, required); sort?, page_size?, limit?: optional. cred?: credential name.
  mdm_fetch_commands(filter, sort?, page_size?, limit?, cred?)
      → value: list of dicts (MDM command records)
      Params: filter?: str (RSQL); sort?, page_size?, limit?: optional. cred?: credential name.

ROLE
  role_privileges_list(cred?)
      → value: list of str (privilege names)
      Params: cred?: credential name.
  role_list(cred?)
      → value: list of dicts with id, display_name
      Params: cred?: credential name.
  role_fetch_list(cred?)
      → value: list of dicts (full roles JSON)
      Params: cred?: credential name.
  role_fetch(role_id, cred?)
      → value: dict (full role JSON)
      Params: role_id: str. cred?: credential name.
  role_put(role_id, dict, cred?)
      → value: dict (updated role JSON)
      Params: role_id: str; dict: Starlark dict (role body). cred?: credential name.
  role_create(display_name, privileges, cred?)
      → value: str (role ID). Also accepts dict form.
      Params: display_name: str; privileges: list of str; cred?: credential name.

CLIENT
  client_fetch_list(cred?)
      → value: list of dicts (full clients JSON)
      Params: cred?: credential name.
  client_fetch(client_id, cred?)
      → value: dict (full client JSON)
      Params: client_id: str or int. cred?: credential name.
  client_put(client_id, dict, cred?)
      → value: dict (updated client JSON)
      Params: client_id: str or int; dict: Starlark dict. cred?: credential name.
  client_create(display_name, scopes, cred?)
      → value: dict {id, client_id}. Also accepts dict form.
      Params: display_name: str; scopes: list of str; cred?: credential name.

SERIALIZATION
  json_decode(json_str)
      → value: dict or list
      Params: json_str: str. Returns: Starlark dict/list from JSON.
  json_encode(value)
      → value: str (JSON)
      Params: value: Starlark dict, list, or primitive. Returns: JSON string.
  plist_decode(plist_xml_str)
      → value: dict or list
      Params: plist_xml_str: str (plist XML). Returns: Starlark dict/list.
  plist_encode(value)
      → value: str (plist XML)
      Params: value: Starlark dict, list, or primitive. Returns: plist XML string.
  xml_escape(s)
      → value: str
      Params: s: str. Entity-encodes & < > for nesting text (XML/JSON/plist) inside an XML element.
  xml_unescape(s)
      → value: str
      Params: s: str. Reverses XML/HTML character entities (&amp; &lt; &gt; &quot; &#39; and numeric).
  xml_sort(xml_str, element?, key?, attr?, only?, order?, ignore_case?, numeric?, recursive?)
      → value: str (sorted XML)
      Params: xml_str: str; element?: str (parent tag or path; default document root);
        key?: str (child tag text, or plist <key>/<string> pair); attr?: str (sort by attribute);
        only?: str (only reorder children with this tag); order?: "asc"|"desc" (default asc);
        ignore_case?: bool; numeric?: bool; recursive?: bool (all matching bare tags).

RSQL (offline filter builders for Jamf list/find APIs)
  rsql_escape(s)
      → value: str
      Params: s: str. Escapes \ and " for use inside an RSQL quoted value.
  rsql_quote(s)
      → value: str
      Params: s: str. Escapes and wraps in double quotes.
  rsql_eq(field, value)
      → value: str (clause)
      Params: field: str; value: str (quoted), int/float/bool (unquoted).
  rsql_ne(field, value)
      → value: str (clause)
      Params: same as rsql_eq.
  rsql_lt(field, value)
      → value: str (clause)
      Params: field: str; value: str/int/float (strings left unquoted — use for ISO datetimes).
  rsql_gt(field, value)
      → value: str (clause)
      Params: same as rsql_lt.
  rsql_le(field, value)
      → value: str (clause)
      Params: same as rsql_lt.
  rsql_ge(field, value)
      → value: str (clause)
      Params: same as rsql_lt.
  rsql_contains(field, substring)
      → value: str (field=="*substring*")
      Params: field: str; substring: str (quote-escaped; * remains a wildcard).
  rsql_in(field, values)
      → value: str (field=in=(...))
      Params: field: str; values: list of str/int/float/bool.
  rsql_and(clauses)
      → value: str
      Params: clauses: list of str (joined with ";").
  rsql_or(clauses)
      → value: str
      Params: clauses: list of str (joined with ",").
  rsql_datetime(year, month, day, hour?, minute?, second?)
      → value: str (UTC ISO-8601, e.g. 2024-01-15T00:00:00Z)
      Params: year/month/day: int; hour?/minute?/second?: int (default 0).
  rsql_now()
      → value: str (current UTC ISO-8601)
  rsql_days_ago(days)
      → value: str (UTC ISO-8601)
      Params: days: int or float.
  rsql_hours_ago(hours)
      → value: str (UTC ISO-8601)
      Params: hours: int or float.

FILE I/O (local filesystem)
  file_read(path)
      → value: str (file contents as UTF-8 text)
      Params: path: str (filesystem path). Returns: full file contents. Fails if the file is not valid UTF-8 (use file_read_bytes).
  file_read_bytes(path)
      → value: bytes
      Params: path: str. Returns: raw file contents as bytes.
  file_write(path, data)
      → None
      Params: path: str; data: str or bytes (written as UTF-8 bytes for str). Creates or overwrites the file with mode 0644.

DECLARATIVE WORKFLOW (step constructors + executor)
  Build a list of steps then pass to run_workflow(). Each step_* returns a step dict.

  step_set_package(app, version, from_trigger, package_pattern?)
      Params: app: str; version: str; from_trigger: str; package_pattern?: str. Returns: step dict.
  step_clone_policy(from_trigger, version, app?, old_version?,
                    package_pattern?, params?)
      Params: from_trigger: str; version: str; app?, old_version?, package_pattern?, params?: optional. Returns: step dict.
  step_set_ss(policy_id, version, group_ids?)
      Params: policy_id: int; version?: str; group_ids?: list of int. Returns: step dict.
  step_set_group_version(group_id, version)
      Params: group_id: int or str; version: str. Returns: step dict.
  step_set_install(policy_id, version, params?)
      Params: policy_id: int; version: str; params?: dict. Returns: step dict.
  step_enable_policy(policy_id)
      Params: policy_id: int. Returns: step dict.
  step_disable_policy(policy_id)
      Params: policy_id: int. Returns: step dict.
  step_set_trigger(policy_id, trigger)
      Params: policy_id: int; trigger: str. Returns: step dict.
  step_set_scope(policy_id, group_ids)
      Params: policy_id: int; group_ids: list of int. Returns: step dict.
  step_set_scope_all(policy_id)
      Params: policy_id: int. Returns: step dict.
  step_delete_policy(policy_ids)
      Params: policy_ids: list of int. Returns: step dict.
  step_set_profile_payload(profile_id, domain, dict_file, redeploy?)
      Params: profile_id: int; domain: str; dict_file: str; redeploy?: bool. Returns: step dict.
  step_set_profile_xml(profile_id, xml_file, redeploy?)
      Params: profile_id: int; xml_file: str; redeploy?: bool. Returns: step dict.
  step_create_policy(template, version)
      Params: template: dict (from policy_template); version: str. Returns: step dict.

  policy_template(name, trigger, script_id, script_priority?,
                  params?, frequency?, category?,
                  scope_group_ids?, scope_all?,
                  recon?, distribution_point?)
      → value: template dict for use with step_create_policy()
      Params: name: str; trigger: str; script_id: str; script_priority?, params?, frequency?, category?, scope_group_ids?, scope_all?, recon?, distribution_point?: optional.

  run_workflow(steps)
      → value: None on success; on failure returns {ok: False, error: str}
      Params: steps: list of step dicts. Executes in order; stops on first failure.

MISC
  error(*args, sep?)
      → None (writes to stderr like print, but does not return a result dict)
      Params: *args: values to print; sep?: str (default " ").
  confirm(msg?)
      → value: bool
      Params: msg?: str — optional description printed before "Proceed? [y/N]" on stdin.
      Returns True if the user enters y or yes, or False otherwise (buddy also prints "Aborted.").
      If buddy was run with global -y, returns True without prompting (same SkipConfirm behavior as buddy CLI confirms).
      To stop the script when the user declines, use: if not confirm("..."): exit(1).
  exit(code?)
      Does not return; exits the script immediately.
      Params: code?: int (exit code, default 0).

EXAMPLE — imperative style
  id_result = policy_get_id(trigger="install-chrome")
  if not id_result["ok"]:
      print("not found:", id_result["error"])
      exit(1)
  policy_id = id_result["value"]
  policy_set_install(policy_id=policy_id, version="130.0", cred="default")

EXAMPLE — declarative workflow
  steps = [
      step_set_package(app="Google Chrome", version="130.0",
                       from_trigger="install-chrome"),
      step_set_group_version(group_id=42, version="130.0"),
      step_enable_policy(policy_id=101),
  ]
  run_workflow(steps)
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
    is setup_buddy_role.star.

    Run the generated script with:
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
  Listing static group members prefers GET /api/v2/computer-groups/static-groups/{id} when the response
  includes assignments; otherwise Classic GET /JSSResource/computergroups/id/{id} is used (requires
  Read Computer Groups for that fallback). Updates use PUT on the v2 static group endpoint.

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
		"shell":         helpShell,
		"starlark":      helpRunStarlark,
		"builtins":      helpRunBuiltins,
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
		"role":          helpRole,
		"client":        helpClient,
		"script":        helpScript,
		"script.get":    helpScriptGet,
		"script.set":    helpScriptSet,
		"setup":         helpSetup,
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
