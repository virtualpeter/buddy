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
		{Action: "List members", BuddyScript: "`smart_group_computers(id, filter?)`", CLI: "`buddy group smart members`", Notes: "Pro membership endpoint; IDs only (`name`/`serial` empty)"},
		{Action: "Criteria", BuddyScript: "`smart_group_criteria` / `smart_group_set_criteria` / …", CLI: "`buddy group smart criteria` / `set-criteria`", Notes: "Round-trip JSON array; version helpers: `version` / `set-version`"},
		{Action: "Create / replace", BuddyScript: "`smart_group_create` / `smart_group_put`", CLI: "`buddy group create smart`", Notes: "Full body PUT; create from JSON file supported"},
		{Action: "Copy / migrate", BuddyScript: "`smart_group_copy` / `smart_group_migrate`", Notes: "Upsert by exact name; clears siteId unless set"},
	},
	AgentNotes: []string{
		"Do not mix with `static_group*` (explicit computer IDs) or Platform `device_group*`.",
		"Do not invent criteria PUT bodies from `jamf_schema` alone — use the criteria helpers.",
		"Effective membership: `smart_group_computers` / `GET /api/v3/computer-groups/smart-group-membership/{id}` (IDs only).",
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
			Name:    `smart_group_computers`,
			Summary: `List effective membership`,
			Returns: `list of {id,name,serial}`,
			Params: []docspec.Param{
				{Name: `id`, Type: `string`, Required: true, Description: `id`},
				{Name: `filter`, Type: `string`, Required: false, Description: `client-side substring on id/name/serial`},
			},
		},
		{
			Name:    `smart_group_set_criteria`,
			Summary: `Replace criteria`,
			Returns: `None`,
			Params: []docspec.Param{
				{Name: `id`, Type: `string`, Required: true, Description: `id`},
			},
		},
		{
			Name:    `smart_group_copy`,
			Summary: `Copy/migrate smart group`,
			Returns: `dict`,
			Params: []docspec.Param{
				{Name: `id`, Type: `string`, Required: true, Description: `id`},
			},
		},
	},
	ExamplesBuddy: []string{
		`def main():
    members = smart_group_computers(id="123")
    smart_group_update_criterion_value(
        id="123",
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
      Does: List smart computer groups (criteria-based).
      → value: list of {id, name, description}
      Params: filter?: bare term → name=="*term*", or full RSQL. page?: int (omit = all);
        page_size?: int (default 200).
  static_groups(filter?, page?, page_size?, tenant?, surface?, auth?)
      Does: List static computer groups (explicit membership).
      → value: list of {id, name, description}
      Params: filter?: bare term → name=="*term*", or full RSQL. page?: int (omit = all);
        page_size?: int (default 200).
  smart_group(id=?) | smart_group(name=?, tenant?, surface?, auth?)
      Does: Load one smart computer group (full JSON: name, criteria, siteId, …).
      → value: dict (full smart-group JSON)
      Params: pass either id= (str) or name= (str), not both.
  static_group(id=, tenant?, surface?, auth?)
      Does: Load one static computer group summary.
      → value: dict {id, name, description}
      Params: id: str (group ID).
  smart_group_version(id, tenant?, surface?, auth?)
      Does: Read the current version criterion value from a smart group.
      → value: str
      Params: id: str.
  smart_group_set_version(id, version, tenant?, surface?, auth?)
      Does: Set matching version criteria on a smart group to a new value.
      → None
      Params: id: str; version: str.
  smart_group_delete(id, tenant?, surface?, auth?)
      Does: Delete a smart computer group.
      → None
      Params: id: str.
  smart_group_create(body, tenant?, surface?, auth?)
      Does: Create a smart computer group from a JSON body.
      → value: dict {"id", "href"?}
      Params: body: dict (name required; criteria optional; id: -1 optional create convention).
  smart_group_clone(id, name, site_id?, tenant?, surface?, auth?)
      Does: Clone a smart group in the same session (alias of smart_group_copy without to_*).
      → value: dict {"id", "href"?}
      Params: id: str; name: str (new group name); site_id?: str.
      Notes: Alias of smart_group_copy without to_* (same-session).
  smart_group_copy(id, name?, site_id?, upsert?=False,
                   tenant?, surface?, auth?, to_tenant?, to_surface?, to_auth?)
      Does: Copy a smart group's criteria/description to another (or the same) session.
      → value: dict {"id", "href"?} or +name/created when upsert
      Params: id: str; name?: dest name (optional → source name on upsert);
        site_id?: dest site (clears siteId unless set). upsert?: bool.
        Source = tenant?/auth?; dest = to_* (omit = same session).
      Notes: upsert=True: upsert by exact name.
  smart_group_migrate(...)
      Does: Alias of smart_group_copy with upsert=True.
      → value: dict {"id", "href"?, "name"?, "created"?}
  smart_group_put(id, body, tenant?, surface?, auth?)
      Does: Replace a smart group's full JSON document (name, criteria, siteId, …).
      → None
      Params: id: str; body: dict (same shape as smart_group(id=…)).
      Notes: PUT /api/v3/computer-groups/smart-groups/{id}. SkipConfirm.
  smart_group_criteria(id, filter?, tenant?, surface?, auth?)
      Does: Return the criteria array from a smart group (or [] if missing).
      → value: list of dict {name, priority, andOr, searchType, value, openingParen?, closingParen?}
      Params: id: str; filter?: case-insensitive substring matched against criterion name.
  smart_group_set_criteria(id, criteria, tenant?, surface?, auth?)
      Does: Replace a smart group's criteria array, preserving other fields.
      → None
      Params: id: str; criteria: list of criteria dicts (same keys as smart_group_criteria).
  smart_group_update_criterion_value(id, name_substring, new_value, tenant?, surface?, auth?)
      Does: Set value= on every criterion whose name contains name_substring.
      → None
      Params: id: str; name_substring: str; new_value: str.
      Notes: Raises if no criteria match.
  smart_group_computers(id, filter?, tenant?, surface?, auth?)
      Does: List effective smart-group membership (Pro IDs).
      → value: list of {id, name, serial}
      Params: id: str; filter?: client-side substring on id/name/serial.
      Notes: Pro GET smart-group-membership; name/serial empty (IDs only).
  static_group_computers(id, filter?, tenant?, surface?, auth?)
      Does: List computers assigned to a static group.
      → value: list of {id, name, serial}
      Params: id: str; filter?: client-side substring on id/name/serial.
      Notes: name may be empty when the API returns only assignment IDs.
  static_group_create(body, tenant?, surface?, auth?)
      Does: Create a static computer group.
      → value: dict {"id", "href"?}
      Params: body: dict (name required; assignments optional list of computer id strings;
        description?/siteId? optional).
  static_group_add_computers(id, computer_ids, tenant?, surface?, auth?)
      Does: Merge computer IDs into a static group's membership (no-op if already members).
      → None
      Params: id: str; computer_ids: list of int or str.
  static_group_remove_computers(id, computer_ids, tenant?, surface?, auth?)
      Does: Remove computer IDs from a static group's membership (no-op if none match).
      → None
      Params: id: str; computer_ids: list of int or str.
  static_group_set_computers(id, computer_ids, tenant?, surface?, auth?)
      Does: Replace a static group's entire membership list.
      → None
      Params: id: str; computer_ids: list of int or str.

`,
	CLIHelp: `buddy group - Manage Jamf computer groups

Usage:
  buddy [global-flags] group <list|get|create|delete|smart|static> [args...]

Subcommands:
  list smart [--filter=<term|rsql>] [--json]    List smart computer groups
  list static [--filter=<term|rsql>] [--json]   List static computer groups
  get <group-id>                         Show group JSON (smart group, falling back to static)
  create smart <name> [--description=<text>] [--file=<json>]
                                         Create a smart computer group
  create smart --file=<json>             Create from full smart-group JSON body
  create static <name> [--description=<text>] [--computer-ids=<computer-id>[,<computer-id>...]]
                                         Create a static computer group
  delete <group-id>                      Delete smart group
  smart members <group-id> [--filter=<substring>]   List effective membership (Pro IDs)
  smart criteria <group-id>              Show smart group criteria JSON
  smart set-criteria <group-id> [--file=<json>|-]
                                         Replace criteria from JSON array (file or stdin)
  smart version <group-id>               Show current version criterion
  smart set-version <group-id> <version> Update version criterion
  static members <group-id> [--filter=<substring>]   List computers in a static group (client-side filter)
  static add <group-id> --computer-ids=<computer-id>[,<computer-id>...]   Add computers to a static group
  static remove <group-id> --computer-ids=<computer-id>[,<computer-id>...]   Remove computers from a static group
  static set <group-id> --computer-ids=<computer-id>[,<computer-id>...]   Replace static group membership (empty list clears)

Flags for 'list':
  --filter=<term|rsql>   Bare term → name=="*term*"; or full RSQL (e.g. 'name=="*Chrome*"')

Flags for 'create smart':
  --description=<text>   Optional description
  --file=<json>          Criteria JSON array, or full smart-group body (object). With <name>,
                         a body object may omit name (positional is used). Without <name>,
                         --file must be a full body including name.

Flags for 'create static':
  --description=<text>              Optional description
  --computer-ids=<computer-id>[,<computer-id>...]     Optional initial membership

Flags for 'smart members' and 'static members':
  --filter=<substring>   Client-side substring match on computer id, name, or serial

Flags for 'smart set-criteria':
  --file=<json>|-        Criteria JSON array (default: stdin when omitted or '-')

Flags for 'static add', 'static remove', and 'static set':
  --computer-ids=<computer-id>[,<computer-id>...]   Required. Comma-separated Jamf computer IDs (set may be empty to clear)

Notes:
  For smart/static members and static add/remove/set, flags such as --computer-ids and --filter may appear
  before or after <group-id> (Go's flag parser otherwise stops at the first positional).
  Smart membership uses GET /api/v3/computer-groups/smart-group-membership/{id} (computer IDs only).
  Criteria round-trip: buddy group smart criteria <group-id> > crit.json ; edit ; buddy group smart set-criteria <group-id> --file=crit.json
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
  buddy group static set 42 --computer-ids=101,102
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
		{Action: "List members", BuddyScript: "`static_group_computers(id, filter?)`", CLI: "`buddy group static members`", Notes: "`[{id, name, serial}, …]`; name may be empty"},
		{Action: "Mutate membership", BuddyScript: "`static_group_add_computers` / `remove` / `set`", CLI: "`buddy group static add|remove|set`", Notes: "Read-modify-write; add/remove no-op when already satisfied; set replaces the list"},
		{Action: "Create", BuddyScript: "`static_group_create(body)`", CLI: "`buddy group create static`", Notes: "`name` required; `assignments` optional"},
	},
	AgentNotes: []string{
		"Do not use smart-group criteria APIs for static membership (or the reverse).",
		"Do not confuse with Platform [device groups](/docs/docs/device-groups).",
		"Do not invent `/api/v3/computer-groups/static-groups/{id}` PUT bodies from schema when the membership helpers exist.",
	},
	Builtins: []docspec.Builtin{
		{
			Name:    `static_group_computers`,
			Summary: `List membership`,
			Returns: `list of {id,name,serial}`,
			Params: []docspec.Param{
				{Name: `id`, Type: `string`, Required: true, Description: `id`},
			},
		},
		{
			Name:    `static_group_add_computers`,
			Summary: `Add members`,
			Returns: `None`,
			Params: []docspec.Param{
				{Name: `id`, Type: `string`, Required: true, Description: `id`},
				{Name: `computer_ids`, Type: `list`, Required: true, Description: `computer_ids`},
			},
		},
		{
			Name:    `static_group_set_computers`,
			Summary: `Replace membership`,
			Returns: `None`,
			Params: []docspec.Param{
				{Name: `id`, Type: `string`, Required: true, Description: `id`},
				{Name: `computer_ids`, Type: `list`, Required: true, Description: `computer_ids`},
			},
		},
	},
	ExamplesBuddy: []string{
		`def main():
    members = static_group_computers(id="10")
    static_group_add_computers(id="10", computer_ids=["42", "43"])
    return members`,
	},
	Related: []docspec.RelatedLink{
		{Title: `Smart computer groups`, Ref: `/docs/docs/smart-groups`},
		{Title: `Computers`, Ref: `/docs/docs/computers`},
		{Title: `API endpoints`, Ref: `/docs/docs/api-endpoints`},
	},
}
