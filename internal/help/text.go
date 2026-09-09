package help

// CLI help bodies. Top-level command names and Root command/flag lists come from clispec.

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
  buddy -e 'unwrap(jq(jamf_schema(filter="smtp"), ".operations[0]"))'
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
mutates (policy_set, policy_put_xml, computer_reboot). Subject resource uses
id= / ids= on load and mutate. Related IDs stay typed (category_id, script_id
on policy create, computer_ids on static-group membership). Pass either id=
or a natural key (trigger=/name=/name=+version=), not both. Resource IDs first;
payload/options next; tenant?/surface?/auth? always last (source session).
Auth alone soft-switches tenant. A pin that cannot serve the call's required
surface (Classic -> instance only while classic_instance_only is on; ACS and
undocumented Pro GET /api/v1/policies -> always instance; Platform
product APIs -> platform) auto-picks another linked auth on the same tenant.
Dual-session copy builtins add
to_tenant?/to_surface?/to_auth? for destination (omit = same as source).
*_copy is canonical; *_clone / *_migrate are aliases (migrate ⇒ upsert=True).
Versioned release bumps use change batches (step_clone_policy), not a
policy_clone builtin.

In the shell, evaluate a bare builtin name (no parentheses) to
print its signature, Does sentence, returns, and params. Prefer
change_batch + plan_change + run_change for multi-step Jamf changes
(see CHANGE BATCH below).

Paginated list builtins accept page? (0-based) and page_size? (default 200).
Omit page to fetch every page and return the full result set.
Plural list value is usually a list of summary dicts {id, name, ...};
use ids=True on policies/computers for a bare ID list. policies() with no
keywords returns every Classic policy summary.

Each builtin lists its signature, a Does: purpose sentence, then → value
(or None). Indented "Params:" lines describe arguments; optional "Notes:"
cover aliases, upsert, READY gates, and other behavior.

STDLIB
  Shared modules use load("stdlib/<name>.buddy", …). None are
  embedded yet. Multi-step recipes live in examples/ (copy and adapt):
  bump-release.buddy, setup-app-classic.buddy, classic-platform-route.buddy.
  Browse builtins in the IDE Libraries sidebar (buddy serve → /ide).

`

const builtinsTail = `JAMF API
  Low-level authenticated HTTP against the credential’s Jamf host. Prefer typed
  builtins when they exist; use these for endpoints buddy does not wrap yet.
  Path is host-relative (must start with /). Defaults: /JSSResource… → XML
  Classic API; /api/… → JSON Jamf Pro API.
  HTTP helpers return {status: int, body: str|dict|list|bytes, content_type: str}.
  On non-2xx: raises with status/path (and a short body preview).
  jamf_url resolves the rewritten URL only (no HTTP).

  jamf_url(path, tenant?, surface?, auth?)
      Does: Resolve the full Jamf URL for path (host + Platform rewrite) without sending HTTP.
      → value: str
      Params: path: str (must start with /). Same session pick as jamf_get (Classic / ACS / Pro UI / published Pro).
  jamf_get(path, accept?, parse?=True, tenant?, surface?, auth?)
      Does: Authenticated GET against the Jamf host.
      → value: {status, body, content_type}
      Params: path: str (must start with /); accept?: str; parse?: bool (JSON→dict/list when possible).
  jamf_post(path, body?, content_type?, accept?, parse?=True, tenant?, surface?, auth?)
      Does: Authenticated POST against the Jamf host.
      → value: {status, body, content_type}
      Params: path: str; body?: str|bytes|dict|list (dict/list JSON-encoded);
        content_type?: str; accept?: str; parse?: bool.
  jamf_put(path, body?, content_type?, accept?, parse?=True, tenant?, surface?, auth?)
      Does: Authenticated PUT against the Jamf host.
      → value: {status, body, content_type}
      Params: path: str; body?: str|bytes|dict|list; content_type?: str; accept?: str; parse?: bool.
  jamf_patch(path, body?, content_type?, accept?, parse?=True, tenant?, surface?, auth?)
      Does: Authenticated PATCH against the Jamf host.
      → value: {status, body, content_type}
      Params: path: str; body?: str|bytes|dict|list; content_type?: str; accept?: str; parse?: bool.
  jamf_delete(path, accept?, parse?=True, tenant?, surface?, auth?)
      Does: Authenticated DELETE against the Jamf host.
      → value: {status, body, content_type}
      Params: path: str; accept?: str; parse?: bool.
  jamf_request(method, path, body?, content_type?, accept?, parse?=True, tenant?, surface?, auth?)
      Does: Authenticated HTTP with an explicit method.
      → value: {status, body, content_type}
      Params: method: GET|POST|PUT|PATCH|DELETE|HEAD; path: str; body?: str|bytes|dict|list;
        content_type?: str; accept?: str; parse?: bool.
  jamf_schema(api?="all", filter?="", full?=False, tenant?, surface?, auth?)
      Does: Search Jamf OpenAPI/Swagger operations (Pro, Classic, and/or embedded Platform).
      → value: {api, count, operations, source?, title?, pro?, classic?, platform?, document?}
      Params: api?: "pro" (/api/schema OpenAPI), "classic" (/classicapi/doc/swagger.yaml),
        "platform" (embedded Platform API Gateway specs; no cred needed), "both"
        (pro+classic), or "all" (default; pro+classic+platform). filter?: case-insensitive;
        space-separated terms AND-matched against path, method, tags, summary, operationId,
        description. full?: include filtered schema document(s) (large). Default is a
        compact operations catalog only.
      Notes: No schema requires authentication — Pro/Classic schema docs are public on the
        Jamf host (cred is only used to resolve the base URL) and Platform specs are embedded.
        operations is a list of {api?, path, method, summary?, operationId?, tags?}.

SERIALIZATION
  Data flow (important):
    • Jamf builtins return bare buddyscript values (dict/list/str/None/…). Failures raise.
      MCP tools wrap {ok, value} only at the tool boundary.
    • print(x) and bare buddyscript display look JSON-ish but are NOT JSON.
      Notebook "Out" pretty-prints dicts/lists as real JSON text.
    • For jq / stdout JSON, use unwrap (optional helper — not the error model):
        data = unwrap(jamf_schema(filter="smtp"))
        buddy script.buddy | jq '.operations[0]'
        unwrap(jq(jamf_schema(filter="smtp"), ".operations[0]"))
    • json_encode / json_pretty return JSON strings — prefer unwrap when you want
      stdout bytes for tools.

  unwrap(result, pretty?=True)
      Does: Print JSON of result to stdout (for jq / piping) and return the same value.
      → value: result (after printing JSON to stdout)
      Params: result: any buddyscript value. pretty?: bool (True indents; False for compact jq input).
      Notes: JSON-print only; not the error model.
  attempt(fn)
      Does: Call a zero-arg function and catch failure as an envelope instead of raising.
      → value: {"ok": bool, "value": …, "error": str}
      Params: fn: zero-arg callable.
      Notes: Prefer bare builtin calls; use attempt only for expected-reject / optional paths.
  json_decode(json_str)
      Does: Parse a JSON string into a dict or list.
      → value: dict or list
      Params: json_str: str.
  json_encode(value)
      Does: Encode a value as compact JSON (not printed).
      → value: str (JSON)
      Params: value: dict, list, or primitive. Prefer unwrap for stdout.
  json_pretty(value)
      Does: Encode a value as indented JSON (not printed).
      → value: str (indented JSON)
      Params: value: dict, list, or primitive. Prefer unwrap(result) to print real newlines to stdout.
  json_flatten(value, prefix?, sep?)
      Does: Hoist nested object keys into a flat dict with dotted names.
      → value: dict
      Params: value: dict; prefix?: str (prepended to keys); sep?: str (default ".").
      Notes: Recurses objects only. Arrays and scalars stay one value. Pair with
        json_decode when a field is a JSON string (e.g. Wi_Fi_History).
  jq(value, expr)
      Does: Apply a jq expression to a dict, list, or primitive (gojq; in-process).
      → value: the collected result (0 values → []; 1 value → that value; N values → list)
      Params: value: JSON-like dict/list/primitive. expr: jq filter string.
      Notes: Strings are already unquoted (jq -r). Use json_encode if you want JSON text.
        Not a replacement for unwrap / piping to jq(1). Object key order is not preserved.
  plist_decode(plist_xml_str)
      Does: Parse plist XML into a dict or list.
      → value: dict or list
      Params: plist_xml_str: str (plist XML).
  plist_encode(value)
      Does: Encode a value as plist XML.
      → value: str (plist XML)
      Params: value: dict, list, or primitive.
  uuid_new()
      Does: Generate a random version-4 UUID (122 random bits; RFC 9562 / RFC 4122 variant 1).
      → value: str (8-4-4-4-12 lowercase hex, e.g. 550e8400-e29b-41d4-a716-446655440000)
      Params: none (offline).
      Notes: Canonical text form; not time/MAC version 1. Same layout Apple/Jamf use for PayloadUUID.
  uuid_is(s)
      Does: Return True if s is canonical UUID text (36-char 8-4-4-4-12 hex).
      → value: bool
      Params: s: str. Offline. Any version (including nil UUID). False for reverse-DNS PayloadType,
        urn:uuid:, {braces}, or 32-char hex without hyphens.
  xml_escape(s)
      Does: Entity-encode & < > for nesting text inside XML element content.
      → value: str
      Params: s: str.
  xml_unescape(s)
      Does: Reverse XML/HTML character entities (&amp; &lt; &gt; &quot; &#39; and numeric).
      → value: str
      Params: s: str.
  xml_sort(xml_str, element?, key?, attr?, only?, order?, ignore_case?, numeric?, recursive?)
      Does: Sort child elements of an XML document or fragment.
      → value: str (sorted XML)
      Params: xml_str: str; element?: str (parent tag or path; default document root);
        key?: str (child tag text, or plist <key>/<string> pair); attr?: str (sort by attribute);
        only?: str (only reorder children with this tag); order?: "asc"|"desc" (default asc);
        ignore_case?: bool; numeric?: bool; recursive?: bool (all matching bare tags).

VERSION — dotted-numeric compare / extract / sort
  version_extract(s)
      Does: Extract a dotted-numeric version from freeform text (package name, trigger, …).
      → value: str (extracted version, or "" if none)
      Params: s: str (bare version or freeform text).
  version_compare(a, b)
      Does: Compare two dotted-numeric versions.
      → value: int (-1, 0, 1)
      Params: a, b: str (bare versions or freeform). Missing/empty versions sort lowest.
  version_sort(items, key?, order?)
      Does: Sort strings or dicts by extracted dotted-numeric version.
      → value: list (same element type as items)
      Params: items: list of str or list of dict; key?: str (required for dicts — field to extract from, e.g. "packageName");
        order?: "asc"|"desc" (default asc). Unversioned items sort after versioned (asc) / before (desc).

RSQL — filter builders for Jamf list/find APIs
  rsql_escape(s)
      Does: Escape \ and " for use inside an RSQL quoted value.
      → value: str
      Params: s: str.
  rsql_quote(s)
      Does: Escape and wrap a string in double quotes for RSQL.
      → value: str
      Params: s: str.
  rsql_eq(field, value)
      Does: Build field==value (strings quoted; numbers/bools unquoted).
      → value: str (clause)
      Params: field: str; value: str (quoted), int/float/bool (unquoted).
  rsql_ne(field, value)
      Does: Build field!=value (same quoting as rsql_eq).
      → value: str (clause)
      Params: field: str; value: str/int/float/bool.
  rsql_lt(field, value)
      Does: Build field=lt=value (strings left unquoted — use for ISO datetimes).
      → value: str (clause)
      Params: field: str; value: str/int/float.
  rsql_gt(field, value)
      Does: Build field=gt=value (strings left unquoted — use for ISO datetimes).
      → value: str (clause)
      Params: field: str; value: str/int/float.
  rsql_le(field, value)
      Does: Build field=le=value (strings left unquoted — use for ISO datetimes).
      → value: str (clause)
      Params: field: str; value: str/int/float.
  rsql_ge(field, value)
      Does: Build field=ge=value (strings left unquoted — use for ISO datetimes).
      → value: str (clause)
      Params: field: str; value: str/int/float.
  rsql_contains(field, substring)
      Does: Build field=="*substring*" (substring quote-escaped; * remains a wildcard).
      → value: str (field=="*substring*")
      Params: field: str; substring: str.
  rsql_in(field, values)
      Does: Build field=in=(…).
      → value: str (field=in=(...))
      Params: field: str; values: list of str/int/float/bool.
  rsql_and(clauses)
      Does: Join RSQL clauses with ";" (AND).
      → value: str
      Params: clauses: list of str.
  rsql_or(clauses)
      Does: Join RSQL clauses with "," (OR).
      → value: str
      Params: clauses: list of str.
  rsql_datetime(year, month, day, hour?, minute?, second?)
      Does: Format a UTC ISO-8601 timestamp for RSQL datetime filters.
      → value: str (UTC ISO-8601, e.g. 2024-01-15T00:00:00Z)
      Params: year/month/day: int; hour?/minute?/second?: int (default 0).
  rsql_now()
      Does: Current UTC time as ISO-8601 for RSQL filters.
      → value: str (current UTC ISO-8601)
  rsql_days_ago(days)
      Does: UTC timestamp that many days before now (ISO-8601).
      → value: str (UTC ISO-8601)
      Params: days: int or float.
  rsql_hours_ago(hours)
      Does: UTC timestamp that many hours before now (ISO-8601).
      → value: str (UTC ISO-8601)
      Params: hours: int or float.

FILE I/O — local filesystem
  file_read(path)
      Does: Read a local file as UTF-8 text.
      → value: str (file contents)
      Params: path: str (filesystem path).
      Notes: Fails if the file is not valid UTF-8 (use file_read_bytes).
  file_read_bytes(path)
      Does: Read a local file as raw bytes.
      → value: bytes
      Params: path: str.
  file_write(path, data)
      Does: Create or overwrite a local file (mode 0644).
      → None
      Params: path: str; data: str or bytes (written as UTF-8 bytes for str).

DECLARATIVE WORKFLOW (step constructors + executor)
  Build a list of steps then pass to run_workflow(). Each step_* returns a step dict.
  run_workflow resolves set_package / clone_policy against Jamf before Confirm
  (source id/name/trigger, package, modify-vs-create, disable yes/no + reason).

  For enterprise change records, prefer change_batch + plan_change + run_change
  (human notes, markdown plan without predicted create IDs, outcomes after apply).

  step_set_package(name, version, from_trigger, package_pattern?,
                   id?, disable_source?, keep_source?, note?)
      Does: Step: attach a package version to an Install/Cache policy (from a source trigger).
      → value: step dict
      Params: name: str package name keyword; version: str; from_trigger: str
        (required source selector); id?: int (pin source policy, skip trigger search);
        package_pattern?, disable_source?, keep_source?, note?: optional.
  step_clone_policy(from_trigger, version, name?, old_version?,
                    package_pattern?, params?, keep_source?, note?)
      Does: Step: clone a policy for a new version (versioned release bump).
      → value: step dict
      Params: from_trigger: str; version: str; name?, old_version?, package_pattern?,
        params?, keep_source?, note?: optional.
  step_note(note)
      Does: Step: prose-only note (no Jamf write).
      → value: step dict
      Params: note: str.
  step_with_note(step, note)
      Does: Attach a human note to any step dict.
      → value: step dict
      Params: step: step dict; note: str.
  step_set_ss(id, version, group_ids?)
      Does: Step: update Self Service display fields on a policy.
      → value: step dict
      Params: id: int; version?: str; group_ids?: list of int.
  step_set_group_version(id, version)
      Does: Step: set a smart group's version criterion.
      → value: step dict
      Params: id: int or str; version: str.
  step_set_install(id, version, params?)
      Does: Step: point an Install policy at a package version.
      → value: step dict
      Params: id: int; version: str; params?: dict.
  step_enable_policy(id)
      Does: Step: enable a policy.
      → value: step dict
      Params: id: int.
  step_disable_policy(id)
      Does: Step: disable a policy.
      → value: step dict
      Params: id: int.
  step_set_trigger(id, trigger)
      Does: Step: set a policy's custom trigger.
      → value: step dict
      Params: id: int; trigger: str.
  step_set_scope(id, group_ids)
      Does: Step: set a policy's computer-group scope.
      → value: step dict
      Params: id: int; group_ids: list of int.
  step_set_scope_all(id)
      Does: Step: set a policy's scope to all computers.
      → value: step dict
      Params: id: int.
  step_delete_policy(ids)
      Does: Step: delete one or more policies.
      → value: step dict
      Params: ids: list of int.
  step_set_profile_payload(id, identifier, dict_file, redeploy?)
      Does: Step: merge-patch a profile payload from a dict file.
      → value: step dict
      Params: id: int; identifier: str; dict_file: str; redeploy?: bool.
  step_set_profile_xml(id, xml_file, redeploy?)
      Does: Step: PUT a profile from a local XML file.
      → value: step dict
      Params: id: int; xml_file: str; redeploy?: bool.
  step_create_package_policy(name, version, action?, scope_all?, scope_group_ids?,
                             category?, package_pattern?, note?)
      Does: Step: create an Install or Cache policy for a package version.
      → value: step dict
      Params: name/version: str; action?: "install"|"cache"; scope_all?: bool;
        scope_group_ids?: list of int; category?: str; package_pattern?: str; note?: str.
      Notes: Plan resolves the package; created policy ID only in Outcomes.
  step_create_profile(name, payloads_plist_str?, content?, category?,
                      scope_all?, scope_group_ids?, redeploy_on_update?, note?)
      Does: Step: create a configuration profile from payloads/content.
      → value: step dict
      Params: name: str; payloads_plist_str and/or content; category?: str;
        scope_all?: bool; scope_group_ids?: list of int; redeploy_on_update?: bool; note?: str.
      Notes: Plan lists payload domains (no new profile ID until apply).
  step_create_ss(trigger, display_name, category?, scope_group_id?, script_id?, note?)
      Does: Step: create a Self Service script policy (parameter4 = trigger).
      → value: step dict
      Params: trigger: str; display_name: str; category?: str; scope_group_id?: int;
        script_id?: int/str; note?: str.
  step_create_policy(template, version)
      Does: Step: create a policy from a policy_template dict.
      → value: step dict
      Params: template: dict (from policy_template); version: str.

  policy_template(name, trigger, script_id, script_priority?,
                  params?, frequency?, category?,
                  scope_group_ids?, scope_all?,
                  recon?, distribution_point?)
      Does: Build a template dict for step_create_policy.
      → value: template dict
      Params: name: str; trigger: str; script_id: str; script_priority?, params?, frequency?,
        category?, scope_group_ids?, scope_all?, recon?, distribution_point?: optional.

  run_workflow(steps)
      Does: Resolve, confirm, and execute a list of step dicts (stops on first failure).
      → None on success; on failure returns {ok: False, error: str}
      Params: steps: list of step dicts.
      Notes: Resolves package/clone plans first, prints a detailed plan, then one Proceed?
        confirm (skipped with -y). Prefer change_batch + plan_change + run_change for CAB.

CHANGE BATCH (plan document + apply with outcomes)
  Prefer this over ad-hoc mutates when applying several related Jamf changes.
  change_batch(title, summary?, risk?, window?)
      Does: Create a change-record dict (title and optional metadata).
      → value: change dict
      Params: title: str (required); summary?/risk?/window?: optional str metadata
        for the change record markdown.
  plan_change(change, steps)
      Does: Resolve steps against Jamf (no writes) and print a markdown plan.
      → value: {markdown: str, fingerprints: list[str]}
      Params: change: from change_batch; steps: list of step dicts.
      Notes: Created policy/profile IDs are NOT predicted (unknown until apply). Pass the
        return value to run_change as planned= for drift detection. See docs/change-batches.md
        for version-bump vs Classic new-app setup.
  run_change(change, steps, planned?)
      Does: Re-resolve, confirm, apply steps, and append Outcomes (created/updated IDs).
      → value: str (full markdown: plan + outcomes)
      Params: change: from change_batch; steps: list of step dicts; planned?: dict from plan_change.
      Notes: If planned fingerprints differ, warns and confirms (warn+confirm), then confirms apply.

MISC
  auth(renew?=False, tenant?, surface?, auth?)
      Does: Report the active session (whoami) without returning the bearer token.
      → value: dict {cred, url, authType, authenticationType, username, accessLevel,
        privilegeSet, privilegeCount, sites, expiresAt?, renewed}
      Params: renew?: bool — if True, discard cached token and obtain a fresh one first.
        tenant?/surface?/auth?: str.
      Notes: Uses GET /api/v1/auth.
  error(*args, sep?)
      Does: Print values to stderr (same as print, but stderr).
      → None
      Params: *args: values to print; sep?: str (default " ").
  confirm(msg?, default?, yes?, no?)
      Does: Prompt on stdin and return True/False (skipped as True when buddy -y).
      → value: bool
      Params: msg?: str — prompt question (default "Proceed?");
               yes?: str — accept token for yes (default "y");
               no?: str — accept token for no (default "n");
               default?: str — Enter alone selects this; must be yes or no (default = no).
      Notes: Prompt looks like "<msg> [y/N]: " (default token uppercased). Example:
        confirm("Wipe disks?", yes="confirm", no="abort")           # [confirm/ABORT]
        confirm("Wipe disks?", yes="confirm", no="abort", default="confirm")  # [CONFIRM/abort]
        Returns True for the yes token (or Enter when default is yes), False for no / Enter-default-no.
        Invalid input re-prompts. buddy prints "Aborted." when the answer is no.
        If buddy was run with global -y, returns True without prompting.
        To stop the script when the user declines, use: if not confirm("..."): exit(1).
  exit(code?)
      Does: Exit the script immediately (does not return).
      → does not return
      Params: code?: int (exit code, default 0).

EXAMPLE — imperative style
    policy_id = policy(trigger="install-chrome")["id"]
    policy_set_install(id=policy_id, version="130.0", auth="default")

EXAMPLE — declarative workflow
    steps = [
        step_set_package(name="Google Chrome", version="130.0",
                         from_trigger="install-chrome"),
        step_set_group_version(id=42, version="130.0"),
        step_enable_policy(id=101),
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
	IconsBuiltinsSection +
	ProfilesBuiltinsSection +
	ProfilePayloadsBuiltinsSection +
	ScriptsBuiltinsSection +
	ExtensionAttributesBuiltinsSection +
	MobileExtensionAttributesBuiltinsSection +
	CategoriesBuiltinsSection +
	MacappsBuiltinsSection +
	AppsBuiltinsSection +
	ComputersBuiltinsSection +
	ComputerSearchesBuiltinsSection +
	MdmBuiltinsSection +
	MobileDevicesBuiltinsSection +
	MobileSearchesBuiltinsSection +
	MobileGroupsBuiltinsSection +
	MobileProfilesBuiltinsSection +
	RolesBuiltinsSection +
	ClientsBuiltinsSection +
	builtinsTail
