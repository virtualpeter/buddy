package buddy

import "buddy/internal/docspec"

func init() {
	docspec.Register(MobileGroupsTopic)
}

// MobileGroupsTopic is the structured doc source for docs/mobile-groups.
// Edit here, then run `make gendocs`.
var MobileGroupsTopic = docspec.Topic{
	ID:      `docs/mobile-groups`,
	Title:   `Mobile-device groups`,
	Section: `Resources`,
	Keywords: []string{
		`mobile group`,
		`mobile_smart_group`,
		`mobile_static_group`,
		`mobile_smart_groups`,
		`mobile_static_groups`,
		`mobile device group`,
	},
	Summary: "Jamf Pro smart and static **mobile-device** groups via the v2 JSON API (`/api/v2/mobile-device-groups/…`). Smart groups have criteria; static groups have explicit membership. These are separate from computer [groups](/docs/docs/smart-groups) and Platform [device groups](/docs/docs/device-groups).",
	Surfaces: []docspec.SurfaceRow{
		{Action: "List", BuddyScript: "`mobile_smart_groups(...)` / `mobile_static_groups(...)`", CLI: "`buddy mobile-group list smart|static`", Notes: ""},
		{Action: "Load one", BuddyScript: "`mobile_smart_group(id=…|name=…)` / `mobile_static_group(…)`", CLI: "`buddy mobile-group get <group-id>`", Notes: "CLI tries smart, then static"},
		{Action: "Criteria", BuddyScript: "`mobile_smart_group_criteria` / `mobile_smart_group_set_criteria`", CLI: "`buddy mobile-group smart criteria|set-criteria`", Notes: "Same criteria shape as computer smart groups"},
		{Action: "Members", BuddyScript: "`mobile_smart_group_devices` / `mobile_static_group_devices`", CLI: "`buddy mobile-group smart|static members`", Notes: ""},
		{Action: "Static membership", BuddyScript: "`mobile_static_group_add_devices` / `_remove_devices` / `_set_devices`", CLI: "`buddy mobile-group static add|remove|set`", Notes: "Read-modify-write"},
		{Action: "Create / delete", BuddyScript: "`mobile_smart_group_create` / `mobile_static_group_create` / `*_delete`", CLI: "`buddy mobile-group create|delete smart|static`", Notes: ""},
	},
	Notes: []string{
		"Smart criteria use the same `{name, priority, andOr, searchType, value}` shape as computer smart groups.",
		"Static membership mutates are read-modify-write on the group's assignments array.",
		"Do not mix with computer `smart_group*` / `static_group*` builtins or Platform `device_group*`.",
	},
	AgentNotes: []string{
		"Membership rows may return IDs only (empty name/serial) depending on the Jamf version.",
		"Use `mobile_smart_group_set_criteria` for criteria edits; do not PUT hand-built JSON unless you fetched it first.",
	},
	Builtins: []docspec.Builtin{
		{
			Name:    `mobile_smart_groups`,
			Summary: `List mobile smart groups`,
			Returns: `list`,
			Params: []docspec.Param{
				{Name: `filter`, Type: `string`, Required: false, Description: `name filter`},
			},
		},
		{
			Name:    `mobile_static_groups`,
			Summary: `List mobile static groups`,
			Returns: `list`,
			Params: []docspec.Param{
				{Name: `filter`, Type: `string`, Required: false, Description: `name filter`},
			},
		},
	},
	ExamplesBuddy: []string{
		`def main():
    return mobile_smart_groups(filter="iPad")`,
		`def main():
    gid = mobile_smart_groups(filter="Latest iOS")[0]["id"]
    crit = mobile_smart_group_criteria(id = gid)
    for c in crit:
        if "OS Version" in c["name"]:
            c["value"] = "18.0"
    mobile_smart_group_set_criteria(id = gid, criteria = crit)`,
	},
	Related: []docspec.RelatedLink{
		{Title: `Mobile devices`, Ref: `/docs/docs/mobile-devices`},
		{Title: `Computer smart groups`, Ref: `/docs/docs/smart-groups`},
		{Title: `Platform device groups`, Ref: `/docs/docs/device-groups`},
	},
	HelpExport: `mobile_groups`,
	BuiltinsHelp: `MOBILE-DEVICE GROUPS (Jamf Pro v2 JSON; smart criteria + static membership)
  mobile_smart_groups(filter?, page?, page_size?, tenant?, surface?, auth?)
      Does: List smart mobile-device groups.
      → value: list of {id, name, description, count}
      Params: filter?: groupName filter (bare term or RSQL).
        page?: int (omit = all); page_size?: int (default 200).
  mobile_static_groups(filter?, page?, page_size?, tenant?, surface?, auth?)
      Does: List static mobile-device groups.
      → value: list of {id, name, description, count}
      Params: same as mobile_smart_groups.
  mobile_smart_group(id=?) | mobile_smart_group(name=?, tenant?, surface?, auth?)
      Does: Load one smart mobile-device group (full JSON).
      → value: dict
      Params: pass either id= or name= (exact, case-insensitive), not both.
  mobile_static_group(id=?) | mobile_static_group(name=?, tenant?, surface?, auth?)
      Does: Load one static mobile-device group (full JSON).
      → value: dict
      Params: pass either id= or name=, not both.
  mobile_smart_group_create(body, tenant?, surface?, auth?)
      Does: Create a smart mobile-device group.
      → value: dict {"id", "href"?}
      Params: body: dict (name required; criteria optional).
  mobile_smart_group_put(id, body, tenant?, surface?, auth?)
      Does: Replace a smart group document (PUT full JSON).
      → None
      Params: id: str; body: dict as returned by mobile_smart_group(id=…).
  mobile_smart_group_delete(id, tenant?, surface?, auth?)
      Does: Delete a smart mobile-device group.
      → None
      Params: id: str.
  mobile_smart_group_criteria(id, filter?, tenant?, surface?, auth?)
      Does: Return the group's criteria array.
      → value: list of {name, priority, andOr, searchType, value, …}
      Params: id: str; filter?: case-insensitive substring on criterion name.
  mobile_smart_group_set_criteria(id, criteria, tenant?, surface?, auth?)
      Does: Replace the group's criteria (other fields preserved).
      → None
      Params: id: str; criteria: list of criteria dicts.
  mobile_smart_group_devices(id, filter?, tenant?, surface?, auth?)
      Does: List devices currently in a smart group.
      → value: list of {id, name, serial}
      Params: id: str; filter?: substring on id/name/serial.
  mobile_static_group_create(body, tenant?, surface?, auth?)
      Does: Create a static mobile-device group.
      → value: dict {"id", "href"?}
      Params: body: dict (name required; assignments optional list of device-id strings).
  mobile_static_group_delete(id, tenant?, surface?, auth?)
      Does: Delete a static mobile-device group.
      → None
      Params: id: str.
  mobile_static_group_devices(id, filter?, tenant?, surface?, auth?)
      Does: List static-group membership.
      → value: list of {id, name, serial}
      Params: id: str; filter?: substring on id/name/serial.
  mobile_static_group_add_devices(id, device_ids, tenant?, surface?, auth?)
      Does: Add devices to a static group (merge; no-op when already members).
      → None
      Params: id: str; device_ids: list of int or str.
  mobile_static_group_remove_devices(id, device_ids, tenant?, surface?, auth?)
      Does: Remove devices from a static group.
      → None
      Params: id: str; device_ids: list of int or str.
  mobile_static_group_set_devices(id, device_ids, tenant?, surface?, auth?)
      Does: Replace the entire static-group membership.
      → None
      Params: id: str; device_ids: list of int or str.

`,
	CLIHelp: `buddy mobile-group - Manage Jamf Pro mobile-device groups (smart + static)

Usage:
  buddy [global-flags] mobile-group <subcommand> [args...]

Subcommands:
  list <smart|static> [--filter=<term>]   List groups (table)
  get <group-id>                          Fetch one group (JSON; tries smart, then static)
  create <smart|static> [--file=<file>|-] Create group from JSON (stdin default)
  delete <smart|static> <group-id>        Delete a group
  smart members <group-id> [--filter=<term>]        List smart-group membership
  smart criteria <group-id>                          Print criteria (JSON)
  smart set-criteria <group-id> [--file=<file>|-]    Replace criteria from JSON array
  static members <group-id> [--filter=<term>]        List static-group membership
  static add <group-id> --device-ids=<device-id>[,<device-id>...]     Add devices
  static remove <group-id> --device-ids=<device-id>[,<device-id>...]  Remove devices
  static set <group-id> --device-ids=<device-id>[,<device-id>...]     Replace membership

Notes:
  Jamf Pro mobile-device groups — separate from 'buddy group' (computers) and
  'buddy device-group' (Platform). Criteria use the same JSON shape as computer
  smart groups.

Examples:
  buddy mobile-group list smart --filter=iPad
  buddy mobile-group smart criteria 12
  buddy mobile-group smart set-criteria 12 --file=criteria.json
  buddy mobile-group static add 7 --device-ids=42,43
`,
}
