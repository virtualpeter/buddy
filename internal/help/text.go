package help

// Code extracted from cmd/buddy/help.go — keep in sync when editing CLI help.

const Root = `buddy - Jamf Pro management CLI

Usage:
  buddy [global-flags]                              Interactive buddyscript shell
  buddy [global-flags] <script.buddy> [-- args...]  Run a buddyscript script (.buddy preferred; .star still works)
  buddy [global-flags] -e <code> [-- args...]       Eval a buddyscript one-liner
  buddy [global-flags] <command> [subcommand] [args...]
  buddy help [command] [subcommand]

Commands:
  auth     Test authentication for the active tenant/auth
  cred     Manage tenants, auths, and secrets in the config file
  policy   Manage Jamf policies (list/get/set/create/delete)
  category Manage categories (list/get/create/delete)
  macapp   Manage Classic Mac App Store apps (list/get/create/delete)
  app      Manage policy apps (list/get/upgrade)
  group    Manage computer groups (list/get/set/create/delete; static membership)
  profile  Manage configuration profiles (list/get/set/clone/redeploy)
  package  Manage package records (list/get/download/upload/delete/set/history)
  computer Manage computers and MDM commands (list/get/reboot/lock/erase/...)
  mdm      Query MDM commands from Jamf Pro (list with RSQL filter)
  blueprint Manage Platform API blueprints (list/get/create/set/delete/deploy/…)
  device   Manage Platform API devices (inventory + check-in/restart/erase/…)
  device-group Manage Platform API device groups (not Pro computer groups)
  ddm      Platform DDM declaration reporting (read-only)
  benchmark Platform compliance benchmarks (beta)
  role     Manage API roles (list/get/create/update/delete/privileges)
  client   Manage API integrations (list/get/create/update/delete/credentials/set)
  script   Manage Jamf scripts (list/get/create/upload/download/delete/set)
  setup    basic-setup (first tenant/auth); write-script; store-oauth (instance)
  serve    Local docs + notebook + script IDE web UI
  mcp      MCP stdio server for AI tools (setup discovers host configs)
  lsp      Language server for buddyscript (editor integration)
  version  Print version and exit (same as --version)

Global flags:
  --version           Print version and exit
  -e, --eval <code>   Run a buddyscript snippet and exit (see 'buddy help shell')
  -y                  Skip confirmation prompts (buddy CLI and confirm() in scripts)
  -v, --verbose       Verbose output
  -t, --trace         Trace HTTP requests and responses to stderr (headers and bodies; Authorization/Cookie redacted)
  --script-trace      Trace buddyscript builtin calls and print() in scripts and the interactive shell
  --tenant <name>     Tenant from config (default: config.tenant); auto-picks auth
  --user <name>       Pin an auth name (--auth alias)
  --auth <name>       Pin an auth name (same as --user)

First time? Run 'buddy setup basic-setup' (then 'buddy auth test'). See 'buddy setup help'.
Use 'buddy help starlark' or 'buddy help builtins' for buddyscript reference.
Use 'buddy help <command>' or 'buddy <command> help' for command details.
Use 'buddy <command> <subcommand> help' for subcommand details.
`

const Shell = `buddy - buddyscript shell and scripts

Usage:
  buddy [global-flags]                              Interactive buddyscript shell
  buddy [global-flags] [--trace] <script.buddy> [-- args...]
  buddy [global-flags] -e <code> [-- args...]
  buddy [global-flags] --eval=<code> [-- args...]

Arguments:
  <script.buddy>  Path to a buddyscript script (.buddy preferred; .star still works)
  -e, --eval      buddyscript snippet to run (filename for errors: <eval>)
  [-- args...]    Optional arguments passed through to the script (available as args)

Flags:
  --trace         Positional flag before the script path: log each buddy builtin call to
                  stderr with its source position, arguments, and return value. Also logs
                  a trace line to stderr for each buddyscript print() (the message itself
                  goes to stdout). Buddy's error() writes to stderr only.

Notes:
  With no arguments, buddy starts an interactive buddyscript REPL on stdin/stdout.

  -e / --eval runs a one-liner in the normal buddyscript environment (builtins,
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

const Builtins = `buddy built-in functions for buddyscript scripts
===============================================
Builtins return bare values on success and raise on failure.
(MCP tools still wrap {ok, value} at the tool boundary only.)

Conventions: plural nouns enumerate (policies, packages, computers);
singular nouns load one resource (policy, package, computer); singular_verb
mutates (policy_set, policy_put_xml, computer_reboot). Pass either id= or a
natural key (trigger=/name=/name=+version=), not both. Resource IDs first;
payload/options next; tenant?/surface?/auth? always last (source session).
Auth alone soft-switches tenant. Dual-session copy builtins add
to_tenant?/to_surface?/to_auth? for destination (omit = same as source).
*_copy is canonical; *_clone / *_migrate are aliases (migrate ⇒ upsert=True).
Versioned release bumps use change batches (step_clone_policy), not a
policy_clone builtin.

In the notebook (or shell), evaluate a bare builtin name (no parentheses) to
print its signature, returns, and params. Prefer change_batch + plan_change +
run_change for multi-step Jamf changes (see CHANGE BATCH below).

Paginated list builtins accept page? (0-based) and page_size? (default 200).
Omit page to fetch every page and return the full result set.
Plural list value is usually a list of summary dicts {id, name, ...};
use ids=True on policies/computers for a bare ID list. policies() with no
keywords returns every Classic policy summary.

Each builtin lists its signature, then → value (or None). Indented "Params:"
lines describe arguments; "Returns:" states the value shape on success.

STDLIB
  Complex CLI-style workflows live in embedded buddyscript modules under stdlib/.
  Load them with load() (same mechanism as load("re", ...)):

    load("stdlib/release.buddy", "bump_release")
    out = bump_release(trigger_prefix="install-example-", name="example", version="1.2.3")
    # out is {"policy_id": int}; builtin failures raise

  Modules compose the builtins below (bare returns; failures raise)
  (except setup_app_classic, which returns a list of step dicts for plan_change).
  Browse modules and builtins in the notebook Libraries drawer (buddy serve → /notebook).
  Current modules: stdlib/release.buddy (bump_release), stdlib/app_setup.buddy
  (setup_app_classic).

POLICY
  policies(keywords?, exclude_ids?, ids?=False, tenant?, surface?, auth?)
      → value: list of {id, name} (or list of int if ids=True)
      Params: keywords?: str or list of str — omit or empty to list all Classic
        policies; otherwise filter by name/trigger keywords. exclude_ids?: list of
        int; ids?: bool.
  policy(id=?) | policy(trigger=?, tenant?, surface?, auth?)
      → value: details dict (id, name, enabled, trigger, frequency, scope, packages, scripts)
      Params: pass either id= (int/str) or trigger= (str), not both.
  policy_xml(id, tenant?, surface?, auth?)
      → value: str (raw policy XML)
  policy_set(policy_id, enable?=, name?=, trigger?=, frequency?=, tenant?, surface?, auth?)
      → None
      Params: at least one of enable/name/trigger/frequency.
  policy_put_xml(policy_id, xml_str, tenant?, surface?, auth?)
      → None
  policy_copy(policy_id, name?, trigger?, category_id?, match_trigger?, upsert?=False,
              scope_all?, scope_group_ids?,
              tenant?, surface?, auth?, to_tenant?, to_surface?, to_auth?)
      → value: dict {id, name, trigger} or {id, name, trigger, created} when upsert
      Params: Source session = tenant?/auth?/surface? (or process default).
        Destination = to_tenant?/to_auth?/to_surface? (omit = same as source).
        Default unassigned scope. Remaps category/scripts/packages by name on dest
        (missing scripts/packages fail; missing category → uncategorized).
        upsert=True: upsert by exact name (or trigger_other when match_trigger=True);
        name optional (defaults to source name). Alias: policy_migrate(...).
  policy_migrate(...) → same as policy_copy(..., upsert=True)
  policy_set_ss(policy_id, version?, group_ids?, display_name?,
                description?, install_text?, reinstall_text?,
                force_view?, feature?, self_service?, tenant?, surface?, auth?)
      → None
  policy_set_install(policy_id, version, tenant?, surface?, auth?) → None
  policy_set_package(name, version, policy_id?, trigger?, package_pattern?,
                     disable_source?, keep_source?, tenant?, surface?, auth?) → None
      Params: name: str package name keyword ("" to derive from the source
        trigger); pass trigger= or policy_id= for the source Install/Cache
        policy (one is required). disable_source?/keep_source?: optional bool
        overrides (mutually exclusive). Default: disable source only when
        creating a same-trigger replacement.
  policy_set_scope_all(policy_id, tenant?, surface?, auth?) → None
  policy_create_ss(trigger, display_name, category?, scope_group_id?, script_id?, tenant?, surface?, auth?)
      → value: int (new policy ID)
  policy_create_package(name, version, action?, scope_all?, scope_group_ids?,
                        category?, package_pattern?, tenant?, surface?, auth?)
      → value: dict {id, name, trigger}
      Params: name: str package name keyword. action?: "install" (default) or
        "cache". Defaults match CLI triggers: install-<name>-next /
        cache-<name>-<version>. scope_all defaults true when
        no scope_group_ids. package_pattern?: optional package name glob.
  policy_delete(policy_ids, tenant?, surface?, auth?) → None

GROUP
  smart_groups(filter?, page?, page_size?, tenant?, surface?, auth?)
      → value: list of {id, name, description}
  static_groups(filter?, page?, page_size?, tenant?, surface?, auth?)
      → value: list of {id, name, description}
  smart_group(id=?) | smart_group(name=?, tenant?, surface?, auth?)
      → value: dict (full smart-group JSON)
  static_group(id, tenant?, surface?, auth?)
      → value: dict {id, name, description}
  smart_group_get_version(group_id, tenant?, surface?, auth?) → value: str
  smart_group_set_version(group_id, version, tenant?, surface?, auth?) → None
  group_delete(group_id, tenant?, surface?, auth?) → None
  smart_group_create(body, tenant?, surface?, auth?) → value: dict {"id", "href"?}
  smart_group_clone(group_id, name, site_id?, tenant?, surface?, auth?) → value: dict {"id", "href"?}
      Params: Alias of smart_group_copy without to_* (same-session).
  smart_group_copy(group_id, name?, site_id?, upsert?=False,
                   tenant?, surface?, auth?, to_tenant?, to_surface?, to_auth?)
      → value: dict {"id", "href"?} or +name/created when upsert
      Params: Copy criteria/description; clears siteId unless site_id set.
        Source = tenant?/auth?; dest = to_* (omit = same session).
        upsert=True: upsert by exact name (name optional → source name).
  smart_group_migrate(...) → same as smart_group_copy(..., upsert=True)
  smart_group_put(group_id, body, tenant?, surface?, auth?) → None
  smart_group_get_criteria(group_id, tenant?, surface?, auth?) → value: list[dict]
  smart_group_set_criteria(group_id, criteria, tenant?, surface?, auth?) → None
  smart_group_find_criteria(group_id, name_substring, tenant?, surface?, auth?) → value: list[dict]
  smart_group_update_criterion_value(group_id, name_substring, new_value, tenant?, surface?, auth?) → None
  static_group_list_computers(group_id, filter?, tenant?, surface?, auth?)
      → value: list of {id, name, serial}
  static_group_create(body, tenant?, surface?, auth?) → value: dict {"id", "href"?}
  static_group_add_computers(group_id, computer_ids, tenant?, surface?, auth?) → None
  static_group_remove_computers(group_id, computer_ids, tenant?, surface?, auth?) → None
  static_group_set_computers(group_id, computer_ids, tenant?, surface?, auth?) → None

BLUEPRINT (Platform API Gateway credential required)
  blueprints(search?, page?, page_size?, tenant?, surface?, auth?)
      → value: list of {id, name, description?, created?, updated?}
      Params: search?: str; page?: int (omit = all); page_size?; tenant?/surface?/auth?.
  blueprint(id=, tenant?, surface?, auth?)
      → value: dict (full blueprint JSON)
      Params: id: str (blueprint UUID); tenant?/surface?/auth?.
  blueprint_create(body, tenant?, surface?, auth?) → value: dict
      Params: body: dict (create JSON); tenant?/surface?/auth?.
  blueprint_set(blueprint_id, body, tenant?, surface?, auth?) → None
      Params: blueprint_id: str; body: dict (merge-patch JSON); tenant?/surface?/auth?.
  blueprint_delete(blueprint_id, tenant?, surface?, auth?) → None
  blueprint_deploy(blueprint_id, tenant?, surface?, auth?) → None
  blueprint_undeploy(blueprint_id, tenant?, surface?, auth?) → None
  blueprint_status(blueprint_id, tenant?, surface?, auth?) → value: dict (status report)
  blueprint_components(page?, page_size?, tenant?, surface?, auth?) → value: list[dict]
  blueprint_component(id=, tenant?, surface?, auth?) → value: dict
      Params: id: str (component identifier); tenant?/surface?/auth?.

PLATFORM DEVICE (Platform gateway; distinct from computer/Pro inventory)
  devices(filter?, sort?, page?, page_size?, tenant?, surface?, auth?) → value: list of {id, name, serialNumber?, …}
  device(id=, tenant?, surface?, auth?) → value: dict
  device_set(device_id, body, tenant?, surface?, auth?) → None
  device_delete(device_id, tenant?, surface?, auth?) → None
  device_applications(device_id, filter?, page?, page_size?, tenant?, surface?, auth?) → value: list[dict]
  device_check_in(device_id, tenant?, surface?, auth?) → None
  device_restart(device_id, tenant?, surface?, auth?) → None
  device_shutdown(device_id, tenant?, surface?, auth?) → None
  device_erase(device_id, tenant?, surface?, auth?) → None
  device_unmanage(device_id, tenant?, surface?, auth?) → None

PLATFORM DEVICE GROUP (not Pro smart/static computer groups)
  device_groups(filter?, page?, page_size?, tenant?, surface?, auth?) → value: list of {id, name, …}
  device_group(id=, tenant?, surface?, auth?) → value: dict
  device_group_create(body, tenant?, surface?, auth?) → value: dict
  device_group_set(group_id, body, tenant?, surface?, auth?) → None
  device_group_delete(group_id, tenant?, surface?, auth?) → None
  device_group_members(group_id, tenant?, surface?, auth?) → value: JSON
  device_group_set_members(group_id, body, tenant?, surface?, auth?) → None

DDM REPORT (read-only Platform declaration reporting)
  ddm_declaration_devices(declaration_id, filter?, sort?, page?, page_size?, tenant?, surface?, auth?) → value: JSON
  ddm_device_declarations(device_id, filter?, sort?, page?, page_size?, tenant?, surface?, auth?) → value: JSON
  ddm_device_channels(device_id, tenant?, surface?, auth?) → value: JSON

COMPLIANCE BENCHMARK (beta Platform API)
  benchmarks(tenant?, surface?, auth?) → value: JSON
  benchmark(id=, tenant?, surface?, auth?) → value: dict
  benchmark_create(body, tenant?, surface?, auth?) → value: dict
  benchmark_delete(benchmark_id, tenant?, surface?, auth?) → None
  benchmark_baselines(tenant?, surface?, auth?) → value: JSON
  benchmark_catalog_rules(baseline?, tenant?, surface?, auth?) → value: JSON
  benchmark_compliance_percentage(benchmark_id, tenant?, surface?, auth?) → value: JSON
  benchmark_report_devices(benchmark_id, tenant?, surface?, auth?) → value: JSON
  benchmark_report_rules(benchmark_id, tenant?, surface?, auth?) → value: JSON

PACKAGE
  packages(filter?, page?, page_size?, tenant?, surface?, auth?)
      → value: list of {id, packageName, fileName}
  package(id=?) | package(name=, version=, pattern?=, tenant?, surface?, auth?)
      → value: full package dict (id=) or {id, name} (name+version)
      Params: id=: str — GET full record (checksums, size, cloudTransferStatus,
        flags, …). Or name=+version= to resolve by name keyword. pattern?:
        optional package name glob/regex override.
  package_ready(id=, tenant?, surface?, auth?) → value: bool
      Params: id=: package id; tenant?/surface?/auth?: str.
      Returns: True iff the package exists, cloudTransferStatus is "READY", and
        sha256 is non-empty; False if missing or not ready. API errors raise.
  package_wait_ready(id=, timeout?, tenant?, surface?, auth?) → value: None
      Params: id=: package id; timeout?: int seconds (default 600); tenant?/surface?/auth?: str.
      Polls until cloudTransferStatus is READY and sha256 is set (nudges JCDS
      refresh-inventory). Prefer after package_upload before attaching to a policy.
  package_download(id=, path?, tenant?, surface?, auth?) → value: None
      Params: id=: package id; path?: local outfile (default fileName); tenant?/surface?/auth?: str.
      Waits READY+sha256, streams JCDS pre-signed URI to temp while hashing
      SHA-256, renames only on match. JCDS only.
  package_upload(file, name?, category_id?, priority?, notes?, info?, reboot?, tenant?, surface?, auth?)
      → value: None
      Params: file: str path (.pkg/.dmg); name?: display name (default basename);
        category_id?: str (default "-1"); priority?: int 1–20 (default 10);
        notes?/info?: str; reboot?: bool (default false); tenant?/surface?/auth?: str.
        Same fileName → update path (GET + checksum compare; skip upload if match;
        if hashes blank and status not READY, raise not-ready — do not re-upload;
        apply metadata flags when they differ). New fileName → create with flags,
        wait until readable, then stream upload.
  package_delete(ids) → None
  package_put(package_id, dict, tenant?, surface?, auth?) → value: dict
      Params: package_id: str; dict: full package body; tenant?/surface?/auth?: str.
      Returns: value: updated package dict. PUT omits server-managed size.

PROFILE
  profiles(filter?)
      → value: list of {id, name}
  profile(id=?) | profile(name=?, tenant?, surface?, auth?)
      → value: XML str (id=) or {id, name} (name=)
  profile_xml(id, tenant?, surface?, auth?) → value: str (raw profile XML)
  profile_create(name, payloads_plist_str?, content?, category?,
                 scope_all?, scope_group_ids?, redeploy_on_update?, tenant?, surface?, auth?)
      → value: int (new profile ID)
      Params: name: str; pass payloads_plist_str and/or content (list of payload
        dicts). scope_all defaults true when scope_group_ids omitted. Classic POST.
  profile_clone(profile_id, name, tenant?, surface?, auth?) → value: dict {id, name}
      Params: Alias of profile_copy without to_* (same-session UUID remap).
  profile_copy(profile_id, name?, category_id?, scope_all?, scope_group_ids?, upsert?=False,
               tenant?, surface?, auth?, to_tenant?, to_surface?, to_auth?) → value: dict {id, name} or +created
      Params: Source = tenant?/auth?; dest = to_* (omit = same session). UUID remap + unassigned scope.
        upsert=True: upsert by exact name (name optional → source name).
  profile_migrate(...) → same as profile_copy(..., upsert=True)
  profile_create_xml(xml_str, name?, tenant?, surface?, auth?) → value: dict {id, name}
      Params: POST prepared XML (after profile_xml_prepare_for_create).
  profile_delete(profile_id, tenant?, surface?, auth?) → None
  profile_redeploy(profile_id, tenant?, surface?, auth?) → None
  profile_acs_bind_uuid(profile_id, payload_uuid) → None
      Params: Seeds process cache classic id → ACS payload UUID (FIELD_EXTERNAL_CONFIGPROFILE_ID).
        Needed when API-token clients cannot scrape legacy HTML; enables read hydrate + ACS writes.
  profile_get_payloads(profile_id) → value: str
  profile_get_payload(profile_id, domain) → value: str
      Params: domain is PayloadIdentifier (PayloadContent only; not MCX — use payloads_get_mcx_dict offline).
  profile_list_payloads(profile_id) → value: list of str
      Params: PayloadIdentifier list (PayloadContent only).
  profile_set_payload(profile_id, domain, dict_file, redeploy?, tenant?, surface?, auth?) → None
      Params: PayloadContent merge or MCX settings dict (same as profile_xml_patch_payload).

PROFILE — component
  profile_put_xml(profile_id, xml_str, redeploy?, tenant?, surface?, auth?) → None
  profile_xml_extract_payloads(profile_xml_str)
      → value: str (payloads plist XML from <payloads>; entities unescaped)
      Params: profile_xml_str: str (offline transform; no API call).
  profile_xml_randomize_uuids(profile_xml_str) → value: str
      Params: Offline; new general UUID + remap PayloadUUID keys; PayloadIdentifier unchanged.
  profile_xml_prepare_for_create(profile_xml_str, name, scope_all?, scope_group_ids?) → value: str
      Params: Offline; name, strip id, scope (default unassigned), randomize UUIDs for Classic POST.
  profile_xml_replace_payloads(profile_xml_str, payloads_plist_str)
      → value: str (updated profile XML)
      Params: profile_xml_str: str; payloads_plist_str: str (offline transform).
  profile_xml_set_redeploy_all(profile_xml_str)
      → value: str (updated profile XML)
      Params: profile_xml_str: str. Sets <redeploy_on_update>All</redeploy_on_update>.
  profile_xml_patch_payload(profile_xml_str, domain, new_payload)
      → value: str (updated profile XML)
      Params: profile_xml_str: str; domain: str (PayloadIdentifier or MCX preference domain);
              new_payload: dict or plist/dict XML. PayloadContent: keys merge onto existing payload
              (PayloadUUID preserved). MCX: settings dict only under mcx_preference_settings.
  payloads_list_identifiers(payloads_plist_str)
      → value: list of str
      Params: payloads_plist_str: str (payloads plist XML). PayloadContent PayloadIdentifiers only.
  payloads_collect_uuids(payloads_plist_str) → value: list of str
      Params: Every PayloadUUID in the plist (verify clone/copy remapping).
  payloads_get_payload(payloads_plist_str, domain)
      → value: str (plist XML dict)
      Params: payloads_plist_str: str; domain: str (PayloadIdentifier). PayloadContent only — not MCX.
  payloads_replace_payload(payloads_plist_str, domain, new_payload)
      → value: str (payloads plist XML)
      Params: payloads_plist_str: str; domain: str; new_payload: dict or plist XML. Full replace (not merge).
  payloads_get_mcx_dict(payloads_plist_str, domain)
      → value: str (bare <dict>…</dict>)
      Params: payloads_plist_str: str; domain: str (MCX preference-domain key). Reads mcx_preference_settings.
  payloads_replace_mcx_dict(payloads_plist_str, domain, dict_xml_str)
      → value: str (payloads plist XML)
      Params: payloads_plist_str: str; domain: str; dict_xml_str: str (MCX settings dict).
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
  pppc_fda(bundle_id, code_requirement, comment?)
      → value: dict (SystemPolicyAllFiles service entry)
      Params: bundle_id: str; code_requirement: str; comment?: str. Offline; no API.
  pppc_payload(services, payload_identifier?, payload_uuid?, payload_display_name?)
      → value: dict (com.apple.TCC.configuration-profile-policy payload)
      Params: services: dict of service name → list of app entries (e.g. from
        pppc_fda). Compose Accessibility etc. as additional service keys. Offline.
  prefs_payload(domain, settings, payload_identifier?, payload_uuid?,
                payload_display_name?)
      → value: dict (preference-domain payload; PayloadType == domain)
      Params: domain: str; settings: dict of preference keys. Offline.

SCRIPT (Jamf Pro scripts)
  scripts(filter?, page?, page_size?, tenant?, surface?, auth?)
      → value: list of {id, name, categoryId, categoryName}
  script(id=?) | script(name=?, tenant?, surface?, auth?)
      → value: full script dict (id=) or id string (name=)
  script_get_contents(script_id, tenant?, surface?, auth?) → value: str
  script_update(script_id, file, tenant?, surface?, auth?) → None
  script_put(script_id, dict, tenant?, surface?, auth?) → value: dict
  script_clone(script_id, name, category_id?, tenant?, surface?, auth?) → value: dict {id, name}
      Params: Alias of script_copy without to_*.
  script_copy(script_id, name?, category_id?, upsert?=False,
              tenant?, surface?, auth?, to_tenant?, to_surface?, to_auth?) → value: dict {id, name} or +created
      Params: Source = tenant?/auth?; dest = to_* (omit = same). upsert=True upserts by name.
  script_migrate(...) → same as script_copy(..., upsert=True)

CATEGORY (Jamf Pro categories)
  categories(filter?, page?, page_size?, tenant?, surface?, auth?)
      → value: list of {id, name, priority}
  category(id=?) | category(name=?, tenant?, surface?, auth?) → value: dict {id, name, priority}
  category_clone(category_id, name, priority?, tenant?, surface?, auth?) → value: dict {id, name, priority?}
      Params: Alias of category_copy without to_*.
  category_copy(category_id, name?, priority?, upsert?=False,
                tenant?, surface?, auth?, to_tenant?, to_surface?, to_auth?) → value: dict …
  category_migrate(...) → same as category_copy(..., upsert=True)

MAC APP STORE (Classic macapplications)
  macapps(filter?, tenant?, surface?, auth?)
      → value: list of {id, name, version, bundle_id}
      Params: filter?: str (client-side substring on name/version/bundle_id); Classic list has no RSQL.
  macapp(id=?) | macapp(name=?, tenant?, surface?, auth?) → value: dict (full resource)
      Params: Pass either id or name, not both. Returns general/scope/self_service plus flat fields.
  macapp_create(name, version, bundle_id, url, free?=False, category?, tenant?, surface?, auth?)
      → value: dict {id, name}
      Params: name/version/bundle_id/url: str (required); free?: bool; category?: str (exact category name).
  macapp_delete(macapp_id, tenant?, surface?, auth?) → None
      Params: macapp_id: int or str.

POLICY APPS (Self Service + packages)
  apps(filter?, tenant?, surface?, auth?)
      → value: list of summary dicts {name, kind, version, ss_policy_id, install_trigger, …}
      Params: filter?: str (client-side substring on name/kind/version/trigger/package).
      Notes: Discovers policies whose name contains [SS]; kinds are single|install|cache.
  app(name, tenant?, surface?, auth?) → value: dict (summary + linked install/cache policies)
      Params: name: str (app identifier, e.g. santa, vscode).
  app_upgrade(name, version, package_pattern?, tenant?, surface?, auth?) → None
      Params: name: str; version: str; package_pattern?: str (package name glob).
      Notes: Dispatches by kind — single in-place package+SS description; install PolicyPackage
             with disable-source; cache keep prior cache + replace install-next.

COMPUTER
  computers(filter?, ids?=False, page?, page_size?, tenant?, surface?, auth?)
      → value: list of summary dicts (or list of int if ids=True)
  computer(id, tenant?, surface?, auth?)
      → value: dict (id, name, serialNumber, udid, managementId, ...)
  computer_send_mdm_command(computer_ids, command_type, command_data?, tenant?, surface?, auth?) → None
  computer_reboot(computer_ids, tenant?, surface?, auth?) → None
  computer_redeploy(computer_ids, tenant?, surface?, auth?) → None
  computer_shutdown(computer_ids, tenant?, surface?, auth?) → None
  computer_lock(computer_ids, message?, phone?, pin?, tenant?, surface?, auth?) → None
  computer_logout(computer_ids, tenant?, surface?, auth?) → None
  computer_erase(computer_ids, pin?, obliteration?, tenant?, surface?, auth?) → None
  computer_device_info(computer_ids, queries?, tenant?, surface?, auth?) → value: dict or list
  computer_device_location(computer_ids, tenant?, surface?, auth?) → value: dict or list
  computer_security_info(computer_ids, tenant?, surface?, auth?) → value: dict or list
  computer_installed_apps(computer_ids, identifiers?, tenant?, surface?, auth?) → value: dict or list
  computer_managed_apps(computer_ids, identifiers?, tenant?, surface?, auth?) → value: dict or list
  computer_certificates(computer_ids, tenant?, surface?, auth?) → value: dict or list

MDM
  mdm_commands(filter, sort?, page?, page_size?, limit?, tenant?, surface?, auth?)
      → value: list of MDM command dicts
      Params: filter: str (RSQL, required); sort?: str; page?: int (omit = all); page_size?; limit?: int.

ROLE
  role_privileges(tenant?, surface?, auth?) → value: list of str
  roles(full?=False, page?, page_size?, tenant?, surface?, auth?)
      → value: list of {id, display_name} (or full JSON list if full=True)
  role(id, tenant?, surface?, auth?) → value: dict
  role_put(role_id, dict, tenant?, surface?, auth?) → value: dict
  role_create(display_name, privileges, tenant?, surface?, auth?) → value: str (role ID)

CLIENT
  clients(page?, page_size?, tenant?, surface?, auth?) → value: list of dicts
  client(id, tenant?, surface?, auth?) → value: dict
  client_put(client_id, dict, tenant?, surface?, auth?) → value: dict
  client_create(display_name, scopes, tenant?, surface?, auth?) → value: dict {id, client_id}

JAMF API
  Low-level authenticated HTTP against the credential’s Jamf host. Prefer typed
  builtins when they exist; use these for endpoints buddy does not wrap yet.
  Path is host-relative (must start with /). Defaults: /JSSResource… → XML
  Classic API; /api/… → JSON Jamf Pro API.
  value is always {status: int, body: str|dict|list|bytes, content_type: str}.
  On non-2xx: raises with status/path (and a short body preview).

  jamf_get(path, accept?, parse?=True, tenant?, surface?, auth?)
      → value: {status, body, content_type}
      Params: path: str; accept?: str; parse?: bool (JSON→dict/list when possible).
  jamf_post(path, body?, content_type?, accept?, parse?=True, tenant?, surface?, auth?)
      → value: {status, body, content_type}
      Params: body?: str|bytes|dict|list (dict/list JSON-encoded).
  jamf_put(path, body?, content_type?, accept?, parse?=True, tenant?, surface?, auth?)
      → value: {status, body, content_type}
  jamf_patch(path, body?, content_type?, accept?, parse?=True, tenant?, surface?, auth?)
      → value: {status, body, content_type}
  jamf_delete(path, accept?, parse?=True, tenant?, surface?, auth?)
      → value: {status, body, content_type}
  jamf_request(method, path, body?, content_type?, accept?, parse?=True, tenant?, surface?, auth?)
      → value: {status, body, content_type}
      Params: method: GET|POST|PUT|PATCH|DELETE|HEAD.
  jamf_schema(api?="all", filter?="", full?=False, tenant?, surface?, auth?)
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
    • Jamf builtins return bare buddyscript values (dict/list/str/None/…). Failures raise.
      MCP tools wrap {ok, value} only at the tool boundary.
    • print(x) and bare buddyscript display look JSON-ish but are NOT JSON.
      Notebook "Out" pretty-prints dicts/lists as real JSON text.
    • For jq / stdout JSON, use unwrap (optional helper — not the error model):
        data = unwrap(jamf_schema(filter="smtp"))
        buddy script.buddy | jq '.operations[0]'
    • json_encode / json_pretty return JSON strings — prefer unwrap when you want
      stdout bytes for tools.

  unwrap(result, pretty?=True)
      → prints JSON of result to stdout and returns result
      Params: result: any buddyscript value. Optional CLI/jq helper.
        Legacy {"ok","value"/"error"} envelopes are peeled; ok=false raises.
        pretty?=True indents; pretty=False for compact jq input.
  attempt(fn)
      → {"ok": bool, "value": …, "error": str}
      Params: fn: zero-arg callable. Soft wrapper — returns ok=false instead of raising.
        Prefer bare builtin calls; use attempt only for expected-reject / optional paths.
  json_decode(json_str)
      → value: dict or list
      Params: json_str: str. Returns: dict/list from JSON.
  json_encode(value)
      → value: str (JSON)
      Params: value: dict, list, or primitive. Returns: compact JSON string
      (not printed). Prefer unwrap for stdout.
  json_pretty(value)
      → value: str (indented JSON)
      Params: value: dict, list, or primitive. Returns indented JSON string
      (not printed). Prefer unwrap(result) to print real newlines to stdout.
  plist_decode(plist_xml_str)
      → value: dict or list
      Params: plist_xml_str: str (plist XML). Returns: dict/list.
  plist_encode(value)
      → value: str (plist XML)
      Params: value: dict, list, or primitive. Returns: plist XML string.
  uuid_new() → value: str
      Params: Offline; random RFC 4122 UUID (for payload builders).
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

VERSION — dotted-numeric compare / extract / sort
  version_extract(s)
      → value: str
      Params: s: str (bare version or freeform text like a package name). Returns extracted dotted version, or "" if none.
  version_compare(a, b)
      → value: int (-1, 0, 1)
      Params: a, b: str (bare versions or freeform). Missing/empty versions sort lowest.
  version_sort(items, key?, order?)
      → value: list
      Params: items: list of str or list of dict; key?: str (required for dicts — field to extract from, e.g. "packageName"); order?: "asc"|"desc" (default asc). Unversioned items sort after versioned (asc) / before (desc).

RSQL — filter builders for Jamf list/find APIs
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

FILE I/O — local filesystem
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
  run_workflow resolves set_package / clone_policy against Jamf before Confirm
  (source id/name/trigger, package, modify-vs-create, disable yes/no + reason).

  For enterprise change records, prefer change_batch + plan_change + run_change
  (human notes, markdown plan without predicted create IDs, outcomes after apply).

  step_set_package(name, version, from_trigger, package_pattern?,
                   disable_source?, keep_source?, note?)
      Params: name: str package name keyword; version: str; from_trigger: str
        (required source selector); package_pattern?, disable_source?,
        keep_source?, note?: optional. Returns: step dict.
  step_clone_policy(from_trigger, version, name?, old_version?,
                    package_pattern?, params?, keep_source?, note?)
      Params: from_trigger: str; version: str; name?, old_version?, package_pattern?,
        params?, keep_source?, note?: optional. Returns: step dict.
  step_note(note) → step dict (prose-only; no Jamf write)
  step_with_note(step, note) → step dict (attach note to any step)
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
  step_create_package_policy(name, version, action?, scope_all?, scope_group_ids?,
                             category?, package_pattern?, note?)
      Params: name/version: str; action?: "install"|"cache". Plan resolves package;
        created policy ID only in Outcomes. Returns: step dict.
  step_create_profile(name, payloads_plist_str?, content?, category?,
                      scope_all?, scope_group_ids?, redeploy_on_update?, note?)
      Params: name: str; payloads_plist_str and/or content. Plan lists payload
        domains (no new profile ID). Returns: step dict.
  step_create_ss(trigger, display_name, category?, scope_group_id?, script_id?, note?)
      Params: trigger: str (script parameter4); display_name: str. Returns: step dict.
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
      Params: steps: list of step dicts. Resolves package/clone plans first, prints
        a detailed plan, then one Proceed? confirm (skipped with -y). Executes in
        order; stops on first failure.

CHANGE BATCH (plan document + apply with outcomes)
  Prefer this over ad-hoc mutates when applying several related Jamf changes.
  change_batch(title, summary?, risk?, window?)
      → value: change dict
      Params: title: str (required); summary?/risk?/window?: optional str metadata
        for the change record markdown.
  plan_change(change, steps)
      → value: {markdown: str, fingerprints: list[str]}
      Params: change: from change_batch; steps: list of step dicts.
        Resolves against Jamf (no writes). Prints markdown. Created policy/profile
        IDs are NOT predicted (unknown until apply). Pass the return value to
        run_change as planned= for drift detection. See docs/change-batches.md for
        version-bump vs Classic new-app setup (setup_app_classic).
  run_change(change, steps, planned?)
      → value: str (full markdown: plan + outcomes)
      Params: planned?: dict from plan_change. Re-resolves; if planned fingerprints
        differ, warns and confirms (warn+confirm). Then confirms apply, executes,
        and appends Outcomes with created/updated policy and profile IDs.

MISC
  auth(renew?=False, tenant?, surface?, auth?)
      → value: dict {cred, url, authType, authenticationType, username, accessLevel,
        privilegeSet, privilegeCount, sites, expiresAt?, renewed}
      Params: renew?: bool — if True, discard cached token and obtain a fresh one first.
        tenant?/surface?/auth?: str. Uses GET /api/v1/auth; does not return the raw bearer token.
  error(*args, sep?)
      → None (writes to stderr like print)
      Params: *args: values to print; sep?: str (default " ").
  confirm(msg?, default?, yes?, no?)
      → value: bool
      Params: msg?: str — prompt question (default "Proceed?");
               yes?: str — accept token for yes (default "y");
               no?: str — accept token for no (default "n");
               default?: str — Enter alone selects this; must be yes or no (default = no).
      Prompt looks like "<msg> [y/N]: " (default token uppercased). Example:
        confirm("Wipe disks?", yes="confirm", no="abort")           # [confirm/ABORT]
        confirm("Wipe disks?", yes="confirm", no="abort", default="confirm")  # [CONFIRM/abort]
      Returns True for the yes token (or Enter when default is yes), False for no / Enter-default-no.
      Invalid input re-prompts. buddy prints "Aborted." when the answer is no.
      If buddy was run with global -y, returns True without prompting (same SkipConfirm behavior as buddy CLI confirms).
      To stop the script when the user declines, use: if not confirm("..."): exit(1).
  exit(code?)
      Does not return; exits the script immediately.
      Params: code?: int (exit code, default 0).

EXAMPLE — imperative style
  policy_id = policy(trigger="install-chrome")["id"]
  policy_set_install(policy_id=policy_id, version="130.0", auth="default")

EXAMPLE — declarative workflow
  steps = [
      step_set_package(name="Google Chrome", version="130.0",
                       from_trigger="install-chrome"),
      step_set_group_version(group_id=42, version="130.0"),
      step_enable_policy(policy_id=101),
  ]
  run_workflow(steps)
`
