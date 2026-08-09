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
		`trigger`,
		`classic`,
	},
	Summary: `List, load, mutate, copy, and migrate Jamf Classic policies with buddy.`,
	Surfaces: []docspec.SurfaceRow{
		{Action: "List / find", BuddyScript: "`policies(keywords=…)` / `policies(ids=True)`", CLI: "`buddy policy list`", Notes: "Summary dicts `{id, name, …}`"},
		{Action: "Load", BuddyScript: "`policy(id=…)` / `policy(trigger=…)` / `policy_xml(id=…)`", CLI: "`buddy policy get`", Notes: "Full Classic XML as dict or string"},
		{Action: "Mutate", BuddyScript: "`policy_set*` / `policy_put_xml`", CLI: "`buddy policy set`", Notes: "Multi-step helpers for common fields; raw XML when needed"},
		{Action: "Copy / migrate", BuddyScript: "`policy_copy` / `policy_migrate`", Notes: "Cross-session; remap by name"},
	},
	Notes: []string{
		"Versioned release bumps use [change batches](/docs/docs/change-batches) (`step_clone_policy` / `step_set_package`) or compose `policy_copy` + set helpers — there is no public `policy_clone` builtin.",
	},
	AgentNotes: []string{
		"Do not invent Classic policy PUT/POST bodies from `jamf_schema` or Jamf MCP `execute-request` when a named builtin exists.",
		"Prefer fat `policy_set*` helpers over hand-edited XML when they cover the change.",
		"Wait until a package is JCDS **READY** before attach — see [packages](/docs/docs/packages).",
		"Do not hand-roll versioned release clones when [change batches](/docs/docs/change-batches) already encode the workflow.",
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
					{Name: `policy_id`, Type: `string`, Required: true, Description: `policy_id`},
				},
			},
			{
				Name: `policy_set`,
				Summary: `Mutate policy fields`,
				Returns: `None`,
				Params: []docspec.Param{
					{Name: `policy_id`, Type: `string`, Required: true, Description: `policy_id`},
				},
			},
		},
	ExamplesBuddy: []string{
		`def main():
    p = policy(trigger="install-example")
    policy_set(policy_id=p["general"]["id"], enable=True)
    return p["general"]["name"]`,
	},
	HelpExport: `policies`,
	BuiltinsHelp: `POLICY
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

`,
	CLIHelp: `buddy policy - Manage Jamf policies

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
`,
}
