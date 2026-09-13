package buddy

import "buddy/internal/docspec"

func init() {
	docspec.Register(RolesTopic)
}

// RolesTopic is the structured doc source for docs/api-roles.
// Edit here, then run `make gendocs`.
var RolesTopic = docspec.Topic{
	ID:      `docs/api-roles`,
	Title:   `API roles`,
	Section: `Resources`,
	Keywords: []string{
		`role`,
		`roles`,
		`api role`,
		`privileges`,
		`privilege`,
	},
	Summary: "Jamf Pro API roles — list, get, create, put, and privilege catalog. Pair with API clients for least-privilege instance oauth.",
	Surfaces: []docspec.SurfaceRow{
		{Action: "List / load", BuddyScript: "`roles(...)` / `role(id=…)`", CLI: "`buddy role list` / `get`", Notes: ""},
		{Action: "Create / update", BuddyScript: "`role_create` / `role_put`", CLI: "`buddy role create` / `update`", Notes: "Privilege helpers in builtins"},
	},
	AgentNotes: []string{
		"Do not grant destructive MDM privileges to day-to-day automation clients without review.",
		"Do not confuse API roles with Jamf account UI roles — this is the Pro API roles API.",
	},
	Builtins: []docspec.Builtin{
		{
			Name:    `roles`,
			Summary: `List API roles`,
			Returns: `list`,
			Params: []docspec.Param{
				{Name: `filter`, Type: `string`, Required: false, Description: `filter`},
			},
		},
		{
			Name:    `role`,
			Summary: `Load one role`,
			Returns: `dict`,
			Params: []docspec.Param{
				{Name: `id`, Type: `string`, Required: true, Description: `id`},
			},
		},
	},
	ExamplesBuddy: []string{
		`def main():
    return roles(filter="Buddy")`,
	},
	Related: []docspec.RelatedLink{
		{Title: `API clients`, Ref: `/docs/docs/api-clients`},
		{Title: `Getting started`, Ref: `/docs/docs/getting-started`},
		{Title: `API endpoints`, Ref: `/docs/docs/api-endpoints`},
	},
	HelpExport: `roles`,
	BuiltinsHelp: `ROLE
  role_privileges(tenant?, surface?, auth?)
      Does: List all available Jamf Pro API privilege names.
      → value: list of str
  roles(filter?, full?=False, page?, page_size?, tenant?, surface?, auth?)
      Does: List API roles.
      → value: list of {id, display_name} (or full JSON list if full=True)
      Params: filter?: bare term → displayName=="*term*", or full RSQL. full?: bool.
        page?: int (omit = all); page_size?: int.
  role(id=, tenant?, surface?, auth?)
      Does: Load one API role JSON document.
      → value: dict (full role record)
      Params: id: str/int (role ID).
  role_put(id, dict, tenant?, surface?, auth?)
      Does: PUT a full API role JSON body (name and privileges).
      → value: dict (updated role)
      Params: id: str/int; dict: full role body.
  role_create(display_name, privileges, tenant?, surface?, auth?)
      Does: Create an API role with the given privilege names.
      → value: str (new role ID)
      Params: display_name: str; privileges: list of str.

`,
	CLIHelp: `buddy role - Manage Jamf Pro API roles

Usage:
  buddy [global-flags] role <subcommand> [args...]

Subcommands:
  list [--filter=<term|rsql>] [--json]             List API roles (always JSON; --json accepted)
  get <role-id>                           Show API role details
  create <displayName> [priv1 priv2...]   Create API role with optional privileges
  update <role-id> <displayName> [priv1...]    Update API role name and privileges
  delete <role-id>                        Delete API role
  privileges                              List all available privilege names

Examples:
  buddy role list
  buddy role list --filter=Buddy
  buddy role list --filter='displayName=="*Buddy*"'
  buddy role get 5
  buddy role create "Read Policies" "Read Policies"
  buddy role update 5 "Read Policies" "Read Policies" "Read Scripts"
  buddy role delete 5
  buddy role privileges
`,
}
