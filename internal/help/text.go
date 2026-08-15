package help

// Code extracted from cmd/buddy/help.go — keep in sync when editing CLI help.

const Root = `buddy - Jamf Pro management CLI

Usage:
  buddy [global-flags]                              Interactive buddyscript shell
  buddy [global-flags] <script.buddy> [-- args...]  Run a buddyscript script
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
  group    Manage computer groups (list/get/create/delete; smart/static membership)
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
  serve    Local docs + script IDE web UI
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
  <script.buddy>  Path to a buddyscript script
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
  args, load("re", …)). If main is defined, it is called (same as file scripts).
  If the last statement is an expression, its value is printed to stdout as
  JSON-friendly text (same rendering as IDE result), unless the expression
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

WHAT IS NOT IN STARLARK (vs Python)
  No:  import  class  try/except  while  lambda  yield  global  nonlocal
  No:  set  frozenset  bytearray  file I/O  os/sys modules
  No:  is  is not  (use == != instead)
  Use load() for re — not Python import.

Use 'buddy help builtins' for buddy-specific functions.
`

const builtinsHead = `buddy built-in functions for buddyscript scripts
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

In the shell, evaluate a bare builtin name (no parentheses) to
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
  load("stdlib/<name>.buddy", …) is reserved for shared modules. None are
  embedded yet. Multi-step recipes live in examples/ (copy and adapt):
  bump-release.buddy, setup-app-classic.buddy. Browse builtins in the IDE
  Libraries sidebar (buddy serve → /ide).

`

const builtinsTail = `JAMF API
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
                   policy_id?, disable_source?, keep_source?, note?)
      Params: name: str package name keyword; version: str; from_trigger: str
        (required source selector); policy_id?: int (pin source policy, skip trigger search);
        package_pattern?, disable_source?, keep_source?, note?: optional. Returns: step dict.
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
        version-bump vs Classic new-app setup.
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

// Builtins is the buddyscript help text (resource sections from *_gen.go).
var Builtins = builtinsHead +
	PoliciesBuiltinsSection +
	GroupsBuiltinsSection +
	BlueprintsBuiltinsSection +
	DevicesBuiltinsSection +
	DeviceGroupsBuiltinsSection +
	DdmBuiltinsSection +
	BenchmarksBuiltinsSection +
	PackagesBuiltinsSection +
	ProfilesBuiltinsSection +
	ProfilePayloadsBuiltinsSection +
	ScriptsBuiltinsSection +
	CategoriesBuiltinsSection +
	MacappsBuiltinsSection +
	AppsBuiltinsSection +
	ComputersBuiltinsSection +
	MdmBuiltinsSection +
	RolesBuiltinsSection +
	ClientsBuiltinsSection +
	builtinsTail
