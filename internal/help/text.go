package help

// Code extracted from cmd/buddy/help.go — keep in sync when editing CLI help.

const Root = `buddy - Jamf Pro management CLI

Usage:
  buddy [global-flags]                              Interactive Starlark shell
  buddy [global-flags] <script.buddy> [-- args...]  Run a Starlark script (.buddy preferred; .star still works)
  buddy [global-flags] -e <code> [-- args...]       Eval a Starlark one-liner
  buddy [global-flags] <command> [subcommand] [args...]
  buddy help [command] [subcommand]

Commands:
  auth     Test authentication for the active tenant/auth
  cred     Manage tenants, auths, and secrets in the config file
  policy   Manage Jamf policies (get/set/clone/create/delete/find)
  group    Manage computer groups (list/get/set/create/delete; static membership)
  profile  Manage configuration profiles (list/get/set/clone/redeploy)
  package  Manage package records (list/get/upload/delete/set/history)
  computer Manage computers and MDM commands (find/get/reboot/lock/erase/...)
  mdm      Query MDM commands from Jamf Pro (find with RSQL filter)
  blueprint Manage Platform API blueprints (list/get/create/set/delete/deploy/…)
  device   Manage Platform API devices (inventory + check-in/restart/erase/…)
  device-group Manage Platform API device groups (not Pro computer groups)
  ddm      Platform DDM declaration reporting (read-only)
  benchmark Platform compliance benchmarks (beta)
  role     Manage API roles (list/get/create/update/delete/privileges)
  client   Manage API integrations (list/get/create/update/delete/credentials/set)
  script   Manage Jamf scripts (list/get/create/upload/download/delete/set)
  setup    Write setup script (role + client for buddy)
  serve    Local docs + notebook + script IDE web UI
  mcp      MCP stdio server for Cursor agents
  lsp      Language server for buddy Starlark (editor integration)

Global flags:
  --version           Print version and exit
  -e, --eval <code>   Run a Starlark snippet and exit (see 'buddy help shell')
  -y                  Skip confirmation prompts (buddy CLI and confirm() in scripts)
  -v, --verbose       Verbose output
  -t, --trace         Trace HTTP requests and responses to stderr (headers and bodies; Authorization/Cookie redacted)
  --script-trace      Trace Starlark builtin calls and print() in scripts and the interactive shell
  --tenant <name>     Tenant from config (default: config.tenant); auto-picks auth
  --user <name>       Pin an auth name (--auth alias; --cred deprecated)
  --auth <name>       Pin an auth name (same as --user)

Use 'buddy help starlark' or 'buddy help builtins' for Starlark scripting reference.
Use 'buddy help <command>' or 'buddy <command> help' for command details.
Use 'buddy <command> <subcommand> help' for subcommand details.
`

const Shell = `buddy - Starlark shell and scripts

Usage:
  buddy [global-flags]                              Interactive Starlark shell
  buddy [global-flags] [--trace] <script.buddy> [-- args...]
  buddy [global-flags] -e <code> [-- args...]
  buddy [global-flags] --eval=<code> [-- args...]

Arguments:
  <script.buddy>  Path to a buddy Starlark script (.buddy preferred; .star still works)
  -e, --eval      Starlark snippet to run (filename for errors: <eval>)
  [-- args...]    Optional arguments passed through to the script (available as args)

Flags:
  --trace         Positional flag before the script path: log each buddy builtin call to
                  stderr with its source position, arguments, and return value. Also logs
                  a trace line to stderr for each Starlark print() (the message itself
                  goes to stdout). Buddy's error() writes to stderr only.

Notes:
  With no arguments, buddy starts an interactive Starlark REPL on stdin/stdout.

  -e / --eval runs a one-liner in the normal buddy Starlark environment (builtins,
  args, load("stdlib/…")). If main is defined, it is called (same as file scripts).
  If the last statement is an expression, its value is printed to stdout as
  JSON-friendly text (same rendering as notebook Out), unless the expression
  already wrote to stdout (e.g. unwrap or print).

  Global -y skips interactive confirmation for buddy CLI commands and makes confirm()
  in scripts return True without reading stdin.

  --script-trace enables the same builtin tracing as positional --trace (for scripts
  and the REPL). HTTP tracing (-t / --trace) is separate: use "buddy -t deploy.buddy"
  to trace Jamf API traffic from builtins.

  Use 'buddy help starlark' and 'buddy help builtins' for language and API reference.

Examples:
  buddy
  buddy deploy.buddy
  buddy deploy.buddy -- --dry-run
  buddy --script-trace deploy.buddy
  buddy -t deploy.buddy
  buddy -e 'unwrap(jamf_schema(filter="smtp"))'
  buddy -e 'unwrap(jamf_schema(filter="smtp"), pretty=False)' | jq '.operations[0]'
  buddy --eval='print(len(args))' -- foo bar
  buddy -y -e 'unwrap(policies("chrome"))'
`

const Starlark = `Starlark language quick-reference
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

MODULES
  load("re", "match")                      # starlib regex (also predeclared as re)
  load("stdlib/release.buddy", "bump_release")  # buddy stdlib (see help builtins)

WHAT IS NOT IN STARLARK (vs Python)
  No:  import  class  try/except  while  lambda  yield  global  nonlocal
  No:  set  frozenset  bytearray  file I/O  os/sys modules
  No:  is  is not  (use == != instead)
  Use load() for buddy stdlib / re — not Python import.

Use 'buddy help builtins' for buddy-specific functions.
`

const Builtins = `buddy built-in functions for Starlark scripts
===============================================
All functions return a result dict: {"ok": True, "value": ...}
                                 or {"ok": False, "error": "..."}
Check ok before using value.

Conventions (v3): plural nouns enumerate (policies, packages, computers);
singular nouns load one resource (policy, package, computer); singular_verb
mutates (policy_set, policy_put_xml, computer_reboot). Pass either id= or a
natural key (trigger=/name=/app=+version=), not both. Resource IDs first;
payload/options next; cred? always last. See docs/builtins-v3.md for the
full old→new rename map. Deprecated aliases warn once on stderr.

Paginated list builtins accept page? (0-based) and page_size? (default 200).
Omit page to fetch every page and return the full result set.
Plural list value is usually a list of summary dicts {id, name, ...};
use ids=True on policies/computers for a bare ID list.

Each builtin lists its signature, then → value (or None). Indented "Params:"
lines describe arguments; "Returns:" states the value shape when ok.

STDLIB
  Complex CLI-style workflows live in embedded Starlark modules under stdlib/.
  Load them with Starlark load() (same mechanism as load("re", ...)):

    load("stdlib/release.buddy", "bump_release")
    r = bump_release(trigger_prefix="install-app-", app="app", version="1.2.3")
    if not r["ok"]:
      fail(r["error"])

  Modules compose the builtins below and return the same {"ok","value","error"} dicts.
  Browse modules and builtins in the notebook Libraries drawer (buddy serve → /notebook).
  Current modules: stdlib/release.buddy (bump_release).

POLICY
  policies(keywords, exclude_ids?, force_refresh?, ids?=False, cred?)
      → value: list of {id, name} (or list of int if ids=True)
      Params: keywords: str or list of str; exclude_ids?: list of int; force_refresh?: bool; ids?: bool.
  policy(id=?) | policy(trigger=?, force_refresh?=, cred?)
      → value: details dict (id, name, enabled, trigger, frequency, scope, packages, scripts)
      Params: pass either id= (int/str) or trigger= (str), not both.
  policy_xml(id, cred?)
      → value: str (raw policy XML)
  policy_set(policy_id, enable?=, name?=, trigger?=, frequency?=, cred?)
      → None
      Params: at least one of enable/name/trigger/frequency.
  policy_put_xml(policy_id, xml_str, cred?)
      → None
  policy_set_ss(policy_id, version?, group_ids?, display_name?,
                description?, install_text?, reinstall_text?,
                force_view?, feature?, self_service?, cred?)
      → None
  policy_set_install(policy_id, version, cred?) → None
  policy_set_package(app, version, policy_id?, trigger?, package_pattern?, cred?) → None
  policy_set_scope_all(policy_id, cred?) → None
  policy_clone(policy_id, version, cred?) → None
  policy_create_ss(trigger, display_name, category?, scope_group_id?, script_id?, cred?)
      → value: int (new policy ID)
  policy_delete(policy_ids, cred?) → None

GROUP
  smart_groups(filter?, page?, page_size?, cred?)
      → value: list of {id, name, description}
  static_groups(filter?, page?, page_size?, cred?)
      → value: list of {id, name, description}
  smart_group(id=?) | smart_group(name=?, cred?)
      → value: dict (full smart-group JSON)
  static_group(id, cred?)
      → value: dict {id, name, description}
  smart_group_get_version(group_id, cred?) → value: str
  smart_group_set_version(group_id, version, cred?) → None
  group_delete(group_id, cred?) → None
  smart_group_create(body, cred?) → value: dict {"id", "href"?}
  smart_group_put(group_id, body, cred?) → None
  smart_group_get_criteria(group_id, cred?) → value: list[dict]
  smart_group_set_criteria(group_id, criteria, cred?) → None
  smart_group_find_criteria(group_id, name_substring, cred?) → value: list[dict]
  smart_group_update_criterion_value(group_id, name_substring, new_value, cred?) → None
  static_group_list_computers(group_id, filter?, cred?)
      → value: list of {id, name, serial}
  static_group_create(body, cred?) → value: dict {"id", "href"?}
  static_group_add_computers(group_id, computer_ids, cred?) → None
  static_group_remove_computers(group_id, computer_ids, cred?) → None
  static_group_set_computers(group_id, computer_ids, cred?) → None

BLUEPRINT (Platform API Gateway credential required)
  blueprints(search?, page?, page_size?, cred?)
      → value: list of {id, name, description?, created?, updated?}
      Params: search?: str; page?: int (omit = all); page_size?; cred?.
  blueprint(id=, cred?)
      → value: dict (full blueprint JSON)
      Params: id: str (blueprint UUID); cred?.
  blueprint_create(body, cred?) → value: dict
      Params: body: dict (create JSON); cred?.
  blueprint_set(blueprint_id, body, cred?) → None
      Params: blueprint_id: str; body: dict (merge-patch JSON); cred?.
  blueprint_delete(blueprint_id, cred?) → None
  blueprint_deploy(blueprint_id, cred?) → None
  blueprint_undeploy(blueprint_id, cred?) → None
  blueprint_status(blueprint_id, cred?) → value: dict (status report)
  blueprint_components(page?, page_size?, cred?) → value: list[dict]
  blueprint_component(id=, cred?) → value: dict
      Params: id: str (component identifier); cred?.

PLATFORM DEVICE (Platform gateway; distinct from computer/Pro inventory)
  devices(filter?, sort?, page?, page_size?, cred?) → value: list of {id, name, serialNumber?, …}
  device(id=, cred?) → value: dict
  device_set(device_id, body, cred?) → None
  device_delete(device_id, cred?) → None
  device_applications(device_id, filter?, page?, page_size?, cred?) → value: list[dict]
  device_check_in(device_id, cred?) → None
  device_restart(device_id, cred?) → None
  device_shutdown(device_id, cred?) → None
  device_erase(device_id, cred?) → None
  device_unmanage(device_id, cred?) → None

PLATFORM DEVICE GROUP (not Pro smart/static computer groups)
  device_groups(filter?, page?, page_size?, cred?) → value: list of {id, name, …}
  device_group(id=, cred?) → value: dict
  device_group_create(body, cred?) → value: dict
  device_group_set(group_id, body, cred?) → None
  device_group_delete(group_id, cred?) → None
  device_group_members(group_id, cred?) → value: JSON
  device_group_set_members(group_id, body, cred?) → None

DDM REPORT (read-only Platform declaration reporting)
  ddm_declaration_devices(declaration_id, filter?, sort?, page?, page_size?, cred?) → value: JSON
  ddm_device_declarations(device_id, filter?, sort?, page?, page_size?, cred?) → value: JSON
  ddm_device_channels(device_id, cred?) → value: JSON

COMPLIANCE BENCHMARK (beta Platform API)
  benchmarks(cred?) → value: JSON
  benchmark(id=, cred?) → value: dict
  benchmark_create(body, cred?) → value: dict
  benchmark_delete(benchmark_id, cred?) → None
  benchmark_baselines(cred?) → value: JSON
  benchmark_catalog_rules(baseline?, cred?) → value: JSON
  benchmark_compliance_percentage(benchmark_id, cred?) → value: JSON
  benchmark_report_devices(benchmark_id, cred?) → value: JSON
  benchmark_report_rules(benchmark_id, cred?) → value: JSON

PACKAGE
  packages(filter?, page?, page_size?, cred?)
      → value: list of {id, packageName, fileName}
  package(id=?) | package(app=, version=, pattern?=, cred?)
      → value: full package dict (id=) or {id, name} (app+version)
  package_upload(file, name?, category_id?, priority?, cred?) → value: dict
  package_delete(ids) → None
  package_put(package_id, dict, cred?) → value: dict

PROFILE
  profiles(filter?)
      → value: list of {id, name}
  profile(id=?) | profile(name=?, cred?)
      → value: XML str (id=) or {id, name} (name=)
  profile_xml(id, cred?) → value: str (raw profile XML)
  profile_clone(profile_id, name, cred?) → value: dict
  profile_redeploy(profile_id) → None
  profile_get_payloads(profile_id) → value: str
  profile_get_payload(profile_id, domain) → value: str
  profile_list_payloads(profile_id) → value: list of str
  profile_set_payload(profile_id, domain, dict_file, redeploy?) → None
  profile_set_xml(profile_id, xml_file, redeploy?) → None

PROFILE — component
  profile_put_xml(profile_id, xml_str, redeploy?, cred?) → None
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
  scripts(filter?, page?, page_size?, cred?)
      → value: list of {id, name, categoryId, categoryName}
  script(id=?) | script(name=?, cred?)
      → value: full script dict (id=) or id string (name=)
  script_get_contents(script_id, cred?) → value: str
  script_update(script_id, file, cred?) → None
  script_put(script_id, dict, cred?) → value: dict

COMPUTER
  computers(filter?, ids?=False, page?, page_size?, cred?)
      → value: list of summary dicts (or list of int if ids=True)
  computer(id, cred?)
      → value: dict (id, name, serialNumber, udid, managementId, ...)
  computer_send_mdm_command(computer_ids, command_type, command_data?, cred?) → None
  computer_reboot(computer_ids, cred?) → None
  computer_redeploy(computer_ids, cred?) → None
  computer_shutdown(computer_ids, cred?) → None
  computer_lock(computer_ids, message?, phone?, pin?, cred?) → None
  computer_logout(computer_ids, cred?) → None
  computer_erase(computer_ids, pin?, obliteration?, cred?) → None
  computer_device_info(computer_ids, queries?, cred?) → value: dict or list
  computer_device_location(computer_ids, cred?) → value: dict or list
  computer_security_info(computer_ids, cred?) → value: dict or list
  computer_installed_apps(computer_ids, identifiers?, cred?) → value: dict or list
  computer_managed_apps(computer_ids, identifiers?, cred?) → value: dict or list
  computer_certificates(computer_ids, cred?) → value: dict or list

MDM
  mdm_commands(filter, sort?, page?, page_size?, limit?, cred?)
      → value: list of MDM command dicts
      Params: filter: str (RSQL, required); sort?: str; page?: int (omit = all); page_size?; limit?: int.

ROLE
  role_privileges(cred?) → value: list of str
  roles(full?=False, page?, page_size?, cred?)
      → value: list of {id, display_name} (or full JSON list if full=True)
  role(id, cred?) → value: dict
  role_put(role_id, dict, cred?) → value: dict
  role_create(display_name, privileges, cred?) → value: str (role ID)

CLIENT
  clients(page?, page_size?, cred?) → value: list of dicts
  client(id, cred?) → value: dict
  client_put(client_id, dict, cred?) → value: dict
  client_create(display_name, scopes, cred?) → value: dict {id, client_id}

JAMF API
  Low-level authenticated HTTP against the credential’s Jamf host. Prefer typed
  builtins when they exist; use these for endpoints buddy does not wrap yet.
  Path is host-relative (must start with /). Defaults: /JSSResource… → XML
  Classic API; /api/… → JSON Jamf Pro API.
  value is always {status: int, body: str|dict|list|bytes, content_type: str}.
  On non-2xx: ok=false, error describes the status, and value is still set.

  jamf_get(path, accept?, parse?=True, cred?)
      → value: {status, body, content_type}
      Params: path: str; accept?: str; parse?: bool (JSON→dict/list when possible).
  jamf_post(path, body?, content_type?, accept?, parse?=True, cred?)
      → value: {status, body, content_type}
      Params: body?: str|bytes|dict|list (dict/list JSON-encoded).
  jamf_put(path, body?, content_type?, accept?, parse?=True, cred?)
      → value: {status, body, content_type}
  jamf_patch(path, body?, content_type?, accept?, parse?=True, cred?)
      → value: {status, body, content_type}
  jamf_delete(path, accept?, parse?=True, cred?)
      → value: {status, body, content_type}
  jamf_request(method, path, body?, content_type?, accept?, parse?=True, cred?)
      → value: {status, body, content_type}
      Params: method: GET|POST|PUT|PATCH|DELETE|HEAD.
  jamf_schema(api?="all", filter?="", full?=False, cred?)
      → value: {api, count, operations, source?, title?, pro?, classic?, platform?, document?}
      Params: api?: "pro" (/api/schema OpenAPI), "classic" (/classicapi/doc/swagger.yaml),
        "platform" (embedded Platform API Gateway specs; no cred needed), "both"
        (pro+classic), or "all" (default; pro+classic+platform). filter?: case-insensitive;
        space-separated terms AND-matched against path, method, tags, summary, operationId,
        description. full?: include filtered schema document(s) (large). Default is a
        compact operations catalog only.
      Note: no schema requires authentication — Pro/Classic schema docs are public on the
        Jamf host (cred is only used to resolve the base URL) and Platform specs are embedded.
      Returns: operations is a list of {api?, path, method, summary?, operationId?, tags?}.

SERIALIZATION
  Data flow (important):
    • Jamf builtins return a result envelope: {"ok": bool, "value": …} or
      {"ok": False, "error": "…"}. That envelope is a Starlark dict, not a JSON string.
    • print(x) and bare Starlark display look JSON-ish but are NOT JSON.
      Notebook "Out" pretty-prints dicts/lists as real JSON text.
    • For jq / stdout JSON, use unwrap (prints value on ok, returns it; soft on error):
        data = unwrap(jamf_schema(filter="smtp"))
        buddy script.buddy | jq '.operations[0]'
    • json_encode / json_pretty return JSON strings *inside* the envelope — prefer
      unwrap when you want stdout bytes for tools.

  unwrap(result, pretty?=True)
      → value on ok, or None on error (soft; does not exit)
      Params: result: usually a builtin return dict (plain values also accepted).
        On ok=True: writes JSON of value to stdout and returns value.
        On ok=False: writes error to stderr and returns None (script continues).
        pretty?=True indents; pretty=False for compact jq input.
  json_decode(json_str)
      → value: dict or list
      Params: json_str: str. Returns: Starlark dict/list from JSON.
  json_encode(value)
      → value: str (JSON)
      Params: value: Starlark dict, list, or primitive. Returns: compact JSON string
      inside the envelope (not printed). Prefer unwrap for stdout.
  json_pretty(value)
      → value: str (indented JSON)
      Params: value: Starlark dict, list, or primitive. Returns indented JSON string
      inside the envelope. Prefer unwrap(result) to print real newlines to stdout.
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
  auth(renew?=False, cred?)
      → value: dict {cred, url, authType, authenticationType, username, accessLevel,
        privilegeSet, privilegeCount, sites, expiresAt?, renewed}
      Params: renew?: bool — if True, discard cached token and obtain a fresh one first.
        cred?: str. Uses GET /api/v1/auth; does not return the raw bearer token.
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
  r = policy(trigger="install-chrome")
  if not r["ok"]:
      error("not found:", r["error"])
      exit(1)
  policy_id = r["value"]["id"]
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
