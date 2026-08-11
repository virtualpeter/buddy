package buddy

import "buddy/internal/docspec"

func init() {
	docspec.Register(SmartGroupsTopic)
	docspec.Register(StaticGroupsTopic)
}

// SmartGroupsTopic is the structured doc source for docs/smart-groups.
// Edit here, then run `make gendocs`.
var SmartGroupsTopic = docspec.Topic{
	ID:      `docs/smart-groups`,
	Title:   `Smart computer groups`,
	Section: `Resources`,
	Keywords: []string{
		`smart`,
		`smart_group`,
		`smart_groups`,
		`criteria`,
		`membership`,
		`group`,
		`groups`,
	},
	Summary: "Pro v3 smart computer groups — dynamic membership via **criteria**. Distinct from static computer groups (`static_group*`) and Platform device groups (`device_group*`).",
	Surfaces: []docspec.SurfaceRow{
		{Action: "List", BuddyScript: "`smart_groups(search=…)`", CLI: "`buddy group list smart`", Notes: "Summary list"},
		{Action: "Load", BuddyScript: "`smart_group(id=…)` / `smart_group(name=…)`", CLI: "`buddy group get`", Notes: "Full JSON; pass id or name, not both"},
		{Action: "List members", BuddyScript: "`smart_group_list_computers(group_id, filter?)`", CLI: "`buddy group smart members`", Notes: "Pro membership endpoint; IDs only (`name`/`serial` empty)"},
		{Action: "Criteria", BuddyScript: "`smart_group_get_criteria` / `smart_group_set_criteria` / …", CLI: "`buddy group smart criteria` / `set-criteria`", Notes: "Round-trip JSON array; version helpers: `version` / `set-version`"},
		{Action: "Create / replace", BuddyScript: "`smart_group_create` / `smart_group_put`", CLI: "`buddy group create smart`", Notes: "Full body PUT; create from JSON file supported"},
		{Action: "Copy / migrate", BuddyScript: "`smart_group_copy` / `smart_group_migrate`", Notes: "Upsert by exact name; clears siteId unless set"},
	},
	AgentNotes: []string{
		"Do not mix with `static_group*` (explicit computer IDs) or Platform `device_group*`.",
		"Do not invent criteria PUT bodies from `jamf_schema` alone — use the criteria helpers.",
		"Effective membership: `smart_group_list_computers` / `GET /api/v3/computer-groups/smart-group-membership/{id}` (IDs only).",
		"Do not assume Classic computergroup XML is the write path for smart groups (buddy uses `/api/v3/computer-groups/smart-groups`).",
	},
	Builtins: []docspec.Builtin{
		{
			Name:    `smart_groups`,
			Summary: `List smart groups`,
			Returns: `list`,
			Params: []docspec.Param{
				{Name: `filter`, Type: `string`, Required: false, Description: `filter`},
			},
		},
		{
			Name:    `smart_group`,
			Summary: `Load smart group`,
			Returns: `dict`,
		},
		{
			Name:    `smart_group_list_computers`,
			Summary: `List effective membership`,
			Returns: `list of {id,name,serial}`,
			Params: []docspec.Param{
				{Name: `group_id`, Type: `string`, Required: true, Description: `group_id`},
				{Name: `filter`, Type: `string`, Required: false, Description: `client-side substring on id/name/serial`},
			},
		},
		{
			Name:    `smart_group_set_criteria`,
			Summary: `Replace criteria`,
			Returns: `None`,
			Params: []docspec.Param{
				{Name: `group_id`, Type: `string`, Required: true, Description: `group_id`},
			},
		},
		{
			Name:    `smart_group_copy`,
			Summary: `Copy/migrate smart group`,
			Returns: `dict`,
			Params: []docspec.Param{
				{Name: `group_id`, Type: `string`, Required: true, Description: `group_id`},
			},
		},
	},
	ExamplesBuddy: []string{
		`def main():
    members = smart_group_list_computers(group_id="123")
    smart_group_update_criterion_value(
        group_id="123",
        name_substring="version",
        new_value="2.0.0",
    )
    return members`,
	},
	Related: []docspec.RelatedLink{
		{Title: `Static computer groups`, Ref: `/docs/docs/static-groups`},
		{Title: `Platform device groups`, Ref: `/docs/docs/device-groups`},
		{Title: `API endpoints`, Ref: `/docs/docs/api-endpoints`},
	},
	HelpExport: `groups`,
	BuiltinsHelp: `GROUP
  smart_groups(filter?, page?, page_size?, tenant?, surface?, auth?)
      → value: list of {id, name, description}
      Params: filter?: bare term → name=="*term*", or full RSQL.
  static_groups(filter?, page?, page_size?, tenant?, surface?, auth?)
      → value: list of {id, name, description}
      Params: filter?: bare term → name=="*term*", or full RSQL.
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
  smart_group_list_computers(group_id, filter?, tenant?, surface?, auth?)
      → value: list of {id, name, serial}
      Params: Pro GET smart-group-membership; name/serial empty (IDs only).
  static_group_list_computers(group_id, filter?, tenant?, surface?, auth?)
      → value: list of {id, name, serial}
  static_group_create(body, tenant?, surface?, auth?) → value: dict {"id", "href"?}
  static_group_add_computers(group_id, computer_ids, tenant?, surface?, auth?) → None
  static_group_remove_computers(group_id, computer_ids, tenant?, surface?, auth?) → None
  static_group_set_computers(group_id, computer_ids, tenant?, surface?, auth?) → None

`,
	CLIHelp: `buddy group - Manage Jamf computer groups

Usage:
  buddy [global-flags] group <list|get|create|delete|smart|static> [args...]

Subcommands:
  list smart [--filter=<term|rsql>]      List smart computer groups
  list static [--filter=<term|rsql>]     List static computer groups
  get <group-id>                         Show group JSON (smart group, falling back to static)
  create smart <name> [--description=<text>] [--file=<json>]
                                         Create a smart computer group
  create smart --file=<json>             Create from full smart-group JSON body
  create static <name> [--description=<text>] [--computer-ids=<id>[,<id>...]]
                                         Create a static computer group
  delete <group-id>                      Delete smart group
  smart members <group-id> [--filter=<substring>]   List effective membership (Pro IDs)
  smart criteria <group-id>              Show smart group criteria JSON
  smart set-criteria <group-id> [--file=<json>|-]
                                         Replace criteria from JSON array (file or stdin)
  smart version <group-id>               Show current version criterion
  smart set-version <group-id> <version> Update version criterion
  static members <group-id> [--filter=<substring>]   List computers in a static group (client-side filter)
  static add <group-id> --computer-ids=<id>[,<id>...]   Add computers to a static group
  static remove <group-id> --computer-ids=<id>[,<id>...]   Remove computers from a static group

Flags for 'list':
  --filter=<term|rsql>   Bare term → name=="*term*"; or full RSQL (e.g. 'name=="*Chrome*"')

Flags for 'create smart':
  --description=<text>   Optional description
  --file=<json>          Criteria JSON array, or full smart-group body (object). With <name>,
                         a body object may omit name (positional is used). Without <name>,
                         --file must be a full body including name.

Flags for 'create static':
  --description=<text>              Optional description
  --computer-ids=<id>[,<id>...]     Optional initial membership

Flags for 'smart members' and 'static members':
  --filter=<substring>   Client-side substring match on computer id, name, or serial

Flags for 'smart set-criteria':
  --file=<json>|-        Criteria JSON array (default: stdin when omitted or '-')

Flags for 'static add' and 'static remove':
  --computer-ids=<id>[,<id>...]   Required. Comma-separated Jamf computer IDs

Notes:
  For smart/static members and static add/remove, flags such as --computer-ids and --filter may appear
  before or after <group-id> (Go's flag parser otherwise stops at the first positional).
  Smart membership uses GET /api/v3/computer-groups/smart-group-membership/{id} (computer IDs only).
  Criteria round-trip: buddy group smart criteria <id> > crit.json ; edit ; buddy group smart set-criteria <id> --file=crit.json
  Listing static group members prefers GET /api/v3/computer-groups/static-groups/{id} when the response
  includes assignments; otherwise Classic GET /JSSResource/computergroups/id/{id} is used (requires
  Read Computer Groups for that fallback). Updates use PUT on the v3 static group endpoint.

Examples:
  buddy group list smart
  buddy group list static --filter=chrome
  buddy group list smart --filter='name=="*Chrome*"'
  buddy group get 10
  buddy group smart members 10
  buddy group smart criteria 10
  buddy group smart set-criteria 10 --file=criteria.json
  buddy group smart criteria 10 | buddy -y group smart set-criteria 10 -
  buddy group smart version 10
  buddy group smart set-version 10 121.0
  buddy group create smart "Chrome installed" --file=criteria.json
  buddy group create static "Lab Macs" --computer-ids=101,102
  buddy group delete 10
  buddy group static members 42 --filter=lab
  buddy group static add 42 --computer-ids=101,102
  buddy group static remove 42 --computer-ids=101
`,
}

// StaticGroupsTopic is the structured doc source for docs/static-groups.
// Edit here, then run `make gendocs`.
var StaticGroupsTopic = docspec.Topic{
	ID:      `docs/static-groups`,
	Title:   `Static computer groups`,
	Section: `Resources`,
	Keywords: []string{
		`static`,
		`static_group`,
		`static_groups`,
		`membership`,
		`assignments`,
		`group`,
		`groups`,
	},
	Summary: "Pro v3 static computer groups — membership is an explicit list of **computer IDs**. Distinct from smart computer groups (`smart_group*`) and Platform device groups (`device_group*`).",
	Surfaces: []docspec.SurfaceRow{
		{Action: "List groups", BuddyScript: "`static_groups(search=…)`", CLI: "`buddy group list static`", Notes: ""},
		{Action: "List members", BuddyScript: "`static_group_list_computers(group_id, filter?)`", CLI: "`buddy group static members`", Notes: "`[{id, name, serial}, …]`; name may be empty"},
		{Action: "Mutate membership", BuddyScript: "`static_group_add_computers` / `remove` / `set`", CLI: "`buddy group static add|remove`", Notes: "Read-modify-write; add/remove no-op when already satisfied; no CLI set-all"},
		{Action: "Create", BuddyScript: "`static_group_create(body)`", CLI: "`buddy group create static`", Notes: "`name` required; `assignments` optional"},
	},
	AgentNotes: []string{
		"Do not use smart-group criteria APIs for static membership (or the reverse).",
		"Do not confuse with Platform [device groups](/docs/docs/device-groups).",
		"Do not invent `/api/v3/computer-groups/static-groups/{id}` PUT bodies from schema when the membership helpers exist.",
	},
	Builtins: []docspec.Builtin{
		{
			Name:    `static_group_list_computers`,
			Summary: `List membership`,
			Returns: `list of {id,name,serial}`,
			Params: []docspec.Param{
				{Name: `group_id`, Type: `string`, Required: true, Description: `group_id`},
			},
		},
		{
			Name:    `static_group_add_computers`,
			Summary: `Add members`,
			Returns: `None`,
			Params: []docspec.Param{
				{Name: `group_id`, Type: `string`, Required: true, Description: `group_id`},
				{Name: `computer_ids`, Type: `list`, Required: true, Description: `computer_ids`},
			},
		},
		{
			Name:    `static_group_set_computers`,
			Summary: `Replace membership`,
			Returns: `None`,
			Params: []docspec.Param{
				{Name: `group_id`, Type: `string`, Required: true, Description: `group_id`},
				{Name: `computer_ids`, Type: `list`, Required: true, Description: `computer_ids`},
			},
		},
	},
	ExamplesBuddy: []string{
		`def main():
    members = static_group_list_computers(group_id="10")
    static_group_add_computers(group_id="10", computer_ids=["42", "43"])
    return members`,
	},
	Related: []docspec.RelatedLink{
		{Title: `Smart computer groups`, Ref: `/docs/docs/smart-groups`},
		{Title: `Computers`, Ref: `/docs/docs/computers`},
		{Title: `API endpoints`, Ref: `/docs/docs/api-endpoints`},
	},
}
