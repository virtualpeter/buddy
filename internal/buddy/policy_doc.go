package buddy

import "buddy/internal/docspec"

func init() {
	docspec.Register(PoliciesTopic)
}

// PoliciesTopic is the structured doc source for docs/policies.
// Edit here, then run `make gendocs`.
var PoliciesTopic = docspec.Topic{
	ID:      `docs/policies`,
	Title:   `Policies`,
	Section: `Resources`,
	Keywords: []string{
		`policy`,
		`policies`,
		`policy_set`,
		`policy_copy`,
		`policy_migrate`,
		`policy_logs`,
		`trigger`,
		`classic`,
	},
	Summary: `List, load, mutate, copy, and migrate Jamf Classic policies with buddy.`,
	Surfaces: []docspec.SurfaceRow{
		{Action: "List / find", BuddyScript: "`policies(keywords=…)` / `policies(ids=True)`", CLI: "`buddy policy list`", Notes: "Summary dicts `{id, name, …}`"},
		{Action: "Load", BuddyScript: "`policy(id=…)` / `policy(trigger=…)` / `policy_xml(id=…)`", CLI: "`buddy policy get`", Notes: "Full Classic XML as dict or string"},
		{Action: "Logs", BuddyScript: "`policy_logs(id, computer_id)`", CLI: "`buddy policy logs <policy-id> --computer=<computer-id>`", Notes: "Classic computerhistory; requires a computer (no fleet fan-out)"},
		{Action: "Mutate", BuddyScript: "`policy_set*` / `policy_put_xml`", CLI: "`buddy policy set`", Notes: "Multi-step helpers for common fields; raw XML when needed"},
		{Action: "Copy / migrate", BuddyScript: "`policy_copy` / `policy_migrate`", Notes: "Cross-session; remap by name"},
	},
	Notes: []string{
		"Versioned release bumps use [change batches](/docs/docs/change-batches) (`step_clone_policy` / `step_set_package`) or compose `policy_copy` + set helpers — there is no public `policy_clone` builtin.",
		"Policy **execution** logs are Classic computerhistory (`policy_logs` / `computer_policy_logs`). There is no fleet-wide “logs for policy N” API and `GET /api/v1/policies` is a list index only.",
	},
	AgentNotes: []string{
		"Do not invent Classic policy PUT/POST bodies from `jamf_schema` or Jamf MCP `execute-request` when a named builtin exists.",
		"Prefer fat `policy_set*` helpers over hand-edited XML when they cover the change.",
		"Wait until a package is JCDS **READY** before attach — see [packages](/docs/docs/packages).",
		"Do not hand-roll versioned release clones when [change batches](/docs/docs/change-batches) already encode the workflow.",
		"Policy execution logs require a computer: `policy_logs(id, computer_id)` or `computer_policy_logs`. Do not invent `jamf_get` or `/api/v1/policies` logs.",
	},
	Builtins: []docspec.Builtin{
			{
				Name: `policies`,
				Summary: `List/search policies`,
				Returns: `list of summary dicts`,
				Params: []docspec.Param{
					{Name: `keywords`, Type: `string`, Required: false, Description: `keywords`},
				},
			},
			{
				Name: `policy`,
				Summary: `Load one policy`,
				Returns: `dict`,
			},
			{
				Name: `policy_copy`,
				Summary: `Copy/migrate policy`,
				Returns: `dict {id,name,trigger}`,
				Params: []docspec.Param{
					{Name: `id`, Type: `string`, Required: true, Description: `id`},
				},
			},
			{
				Name: `policy_set`,
				Summary: `Mutate policy fields`,
				Returns: `None`,
				Params: []docspec.Param{
					{Name: `id`, Type: `string`, Required: true, Description: `id`},
				},
			},
			{
				Name: `policy_logs`,
				Summary: `List flattened policy logs for one computer`,
				Returns: `list of dict`,
				Params: []docspec.Param{
					{Name: `id`, Type: `string`, Required: true, Description: `policy id`},
					{Name: `computer_id`, Type: `string`, Required: true, Description: `computer id`},
				},
			},
		},
	ExamplesBuddy: []string{
		`def main():
    p = policy(trigger="install-example")
    policy_set(id=p["general"]["id"], enable=True)
    return p["general"]["name"]`,
		`def main():
    return policy_logs(id=42, computer_id="101")`,
	},
	Related: []docspec.RelatedLink{
		{Title: `Computers`, Ref: `/docs/docs/computers`},
	},
	HelpExport: `policies`,
	BuiltinsHelp: `POLICY
  policies(keywords?, exclude_ids?, ids?=False, tenant?, surface?, auth?)
      Does: List Classic policies, optionally filtered by name/trigger keywords.
      → value: list of {id, name} (or list of int if ids=True)
      Params: keywords?: str or list of str — omit or empty to list all Classic
        policies; otherwise filter by name/trigger keywords. exclude_ids?: list of
        int (drop those IDs). ids?: bool (return bare IDs). tenant?/surface?/auth?:
        session overrides.
  policy(id=?) | policy(trigger=?, tenant?, surface?, auth?)
      Does: Load one Classic policy by id or custom trigger.
      → value: details dict (id, name, enabled, trigger, frequency, scope, packages, scripts)
      Params: pass either id= (int/str) or trigger= (str), not both.
  policy_xml(id, tenant?, surface?, auth?)
      Does: Fetch the raw Classic policy XML document.
      → value: str (raw policy XML)
      Params: id: int/str (policy ID).
  policy_set(id, enable?=, name?=, trigger?=, frequency?=, tenant?, surface?, auth?)
      Does: Patch general policy fields (enable, name, trigger, frequency).
      → None
      Params: id: int/str; at least one of enable?/name?/trigger?/frequency?.
  policy_put_xml(id, xml_str, tenant?, surface?, auth?)
      Does: Replace a policy by PUTting full Classic XML.
      → None
      Params: id: int/str; xml_str: str (full policy XML).
  policy_copy(id, name?, trigger?, category_id?, match_trigger?, upsert?=False,
              scope_all?, scope_group_ids?,
              tenant?, surface?, auth?, to_tenant?, to_surface?, to_auth?)
      Does: Copy a policy to another (or the same) session, remapping category/scripts/packages by name.
      → value: dict {id, name, trigger} or {id, name, trigger, created} when upsert
      Params: id: source policy. name?/trigger?: dest identity. category_id?: dest category.
        match_trigger?: bool (upsert match on trigger_other). upsert?: bool.
        scope_all?/scope_group_ids?: dest scope (default unassigned).
        Source session = tenant?/auth?/surface? (or process default).
        Destination = to_tenant?/to_auth?/to_surface? (omit = same as source).
      Notes: Missing scripts/packages on dest fail; missing category → uncategorized.
        upsert=True: upsert by exact name (or trigger_other when match_trigger=True);
        name optional (defaults to source name). Alias: policy_migrate(...).
  policy_migrate(...)
      Does: Alias of policy_copy with upsert=True.
      → value: dict {id, name, trigger, created}
  policy_set_ss(id, version?, group_ids?, display_name?,
                description?, install_text?, reinstall_text?,
                force_view?, feature?, self_service?, tenant?, surface?, auth?)
      Does: Update a policy's Self Service display (name, description, buttons, feature, scope groups).
      → None
      Params: id: int; version?: str (shown in Self Service); group_ids?: list of int
        (Self Service group scope); display_name?/description?/install_text?/reinstall_text?: str;
        force_view?/feature?/self_service?: bool.
  policy_set_install(id, version, tenant?, surface?, auth?)
      Does: Point an Install policy at the package matching name+version (after READY).
      → None
      Params: id: int; version: str (package version to attach).
  policy_set_package(name, version, id?, trigger?, package_pattern?,
                     disable_source?, keep_source?, tenant?, surface?, auth?)
      Does: Attach a package to an Install/Cache policy, optionally cloning a same-trigger replacement.
      → None
      Params: name: str package name keyword ("" to derive from the source
        trigger); version: str; pass trigger= or id= for the source Install/Cache
        policy (one is required). package_pattern?: package name glob.
        disable_source?/keep_source?: optional bool overrides (mutually exclusive).
      Notes: Default: disable source only when creating a same-trigger replacement.
  policy_set_scope_all(id, tenant?, surface?, auth?)
      Does: Set the policy scope to all computers (unrestricted).
      → None
      Params: id: int/str.
  policy_create_ss(trigger, display_name, category?, scope_group_id?, script_id?, tenant?, surface?, auth?)
      Does: Create a Self Service policy that runs a script (parameter4 = trigger).
      → value: int (new policy ID)
      Params: trigger: str; display_name: str; category?: str; scope_group_id?: int;
        script_id?: int/str.
  policy_create_package(name, version, action?, scope_all?, scope_group_ids?,
                        category?, package_pattern?, tenant?, surface?, auth?)
      Does: Create an Install or Cache policy for a named package version.
      → value: dict {id, name, trigger}
      Params: name: str package name keyword. version: str. action?: "install" (default) or
        "cache". scope_all?: bool (defaults true when no scope_group_ids).
        scope_group_ids?: list of int. category?: str. package_pattern?: package name glob.
      Notes: Default triggers: install-<name>-next / cache-<name>-<version>.
  policy_delete(ids, tenant?, surface?, auth?)
      Does: Delete one or more Classic policies.
      → None
      Params: ids: int/str or list of int/str.
  policy_logs(id, computer_id, status?, tenant?, surface?, auth?)
      Does: List flattened Classic policy execution logs for one policy on one computer.
      → value: list of {policy_id, policy_name, username, date_time, date_time_epoch, date_time_utc, status}
      Params: id: int/str (policy ID). computer_id: int/str (required; no fleet fan-out).
        status?: str (case-insensitive exact). tenant?/surface?/auth?: session overrides.
      Notes: Same GET as computer_policy_logs (subset/PolicyLogs), filtered by policy id.
        Not GET /api/v1/policies. See computer_history / computer_policy_logs.

`,
	CLIHelp: `buddy policy - Manage Jamf policies

Usage:
  buddy [global-flags] policy <subcommand> [args...]

Subcommands:
  list    List policies (optional keyword filter)
  get     Read policy (details, id by trigger, enable state, xml)
  logs    Policy execution logs for one computer (Classic computerhistory)
  set     Modify policy properties (scope, enable/disable, trigger, name, ...)
  create  Create a new policy
  delete  Delete one or more policies

Use 'buddy policy <subcommand> help' for subcommand-specific help.

Flags for 'logs':
  --computer=<computer-id>       Required computer to read Classic history from
  --status=<status>              Client-side filter (case-insensitive exact)
  --json                         Print flattened logs as JSON

Examples:
  buddy policy list --filter=chrome
  buddy policy get id install-chrome
  buddy policy logs 42 --computer=101
  buddy policy delete 42
`,
}
