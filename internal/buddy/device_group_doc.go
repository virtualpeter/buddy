package buddy

import "buddy/internal/docspec"

func init() {
	docspec.Register(DeviceGroupsTopic)
}

// DeviceGroupsTopic is the structured doc source for docs/device-groups.
// Edit here, then run `make gendocs`.
var DeviceGroupsTopic = docspec.Topic{
	ID:      `docs/device-groups`,
	Title:   `Platform device groups`,
	Section: `Platform`,
	Keywords: []string{
		`device_group`,
		`device_groups`,
		`device-group`,
		`platform`,
		`membership`,
	},
	Summary: `Platform API Gateway device groups and membership. **Not** Pro [smart](/docs/docs/smart-groups) or [static](/docs/docs/static-groups) computer groups.`,
	Surfaces: []docspec.SurfaceRow{
		{Action: "List / load", BuddyScript: "`device_groups(...)` / `device_group(id=…)`", CLI: "`buddy device-group list` / `get`", Notes: ""},
		{Action: "Membership", BuddyScript: "`device_group*` helpers", CLI: "`buddy device-group members` / `set-members`", Notes: "See builtins catalog"},
	},
	Notes: []string{
		"Not Pro [smart](/docs/docs/smart-groups) or [static](/docs/docs/static-groups) computer groups.",
	},
	AgentNotes: []string{
		"Do not use `smart_group*` / `static_group*` for Platform device groups (or the reverse).",
		"Do not invent membership PUT bodies from schema when named builtins exist.",
	},
	Builtins: []docspec.Builtin{
			{
				Name: `device_groups`,
				Summary: `List platform device groups`,
				Returns: `list`,
				Params: []docspec.Param{
					{Name: `filter`, Type: `string`, Required: false, Description: `filter`},
				},
			},
			{
				Name: `device_group`,
				Summary: `Load one device group`,
				Returns: `dict`,
				Params: []docspec.Param{
					{Name: `id`, Type: `string`, Required: true, Description: `id`},
				},
			},
		},
	ExamplesBuddy: []string{
		`def main():
    return device_groups()`,
	},
	Related: []docspec.RelatedLink{
		{Title: `Smart computer groups`, Ref: `/docs/docs/smart-groups`},
		{Title: `Static computer groups`, Ref: `/docs/docs/static-groups`},
		{Title: `Platform devices`, Ref: `/docs/docs/devices`},
		{Title: `API endpoints`, Ref: `/docs/docs/api-endpoints`},
	},
	HelpExport: `device_groups`,
	BuiltinsHelp: `PLATFORM DEVICE GROUP (not Pro smart/static computer groups)
  device_groups(filter?, page?, page_size?, tenant?, surface?, auth?)
      Does: List Platform API device groups.
      → value: list of {id, name, …}
      Params: filter?: bare term → name=="*term*", or full RSQL. page?: int (omit = all);
        page_size?: int.
  device_group(id=, tenant?, surface?, auth?)
      Does: Fetch one Platform device group JSON document.
      → value: dict (full group record)
      Params: id=: str (group UUID).
  device_group_create(body, tenant?, surface?, auth?)
      Does: Create a Platform device group from a JSON body.
      → value: dict (created group, including id)
      Params: body: dict (create JSON).
  device_group_set(id, body, tenant?, surface?, auth?)
      Does: PATCH a Platform device group with a JSON body.
      → None
      Params: id: str; body: dict (merge-patch JSON).
  device_group_delete(id, tenant?, surface?, auth?)
      Does: Delete a Platform device group.
      → None
      Params: id: str.
  device_group_members(id, tenant?, surface?, auth?)
      Does: List members of a Platform device group.
      → value: JSON (membership document)
      Params: id: str.
  device_group_set_members(id, body, tenant?, surface?, auth?)
      Does: PATCH static Platform device-group membership.
      → None
      Params: id: str; body: dict (membership JSON).

`,
	CLIHelp: `buddy device-group - Manage Jamf Platform API device groups

Usage:
  buddy [global-flags] device-group <subcommand> [args...]

Subcommands:
  list [--filter=<term|rsql>]              List device groups (table)
  get <group-id>                       Fetch one group (JSON)
  create [--file=<path>|-]             Create from JSON body
  set <group-id> [--file=|-]           PATCH group JSON
  delete <group-id>                    Delete a group
  members <group-id>                   List group members (JSON)
  set-members <group-id> [--file=|-]   PATCH membership (static groups)

Notes:
  Distinct from Jamf Pro computer groups ('buddy group' / smart_group / static_group).
  Requires Platform API Gateway context+auth.

Examples:
  buddy device-group list
  buddy device-group get <group-id>
  buddy device-group members <group-id>
`,
}
