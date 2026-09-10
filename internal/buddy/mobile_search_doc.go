package buddy

import "buddy/internal/docspec"

func init() {
	docspec.Register(MobileSearchesTopic)
}

// MobileSearchesTopic is the structured doc source for docs/mobile-searches.
// Edit here, then run `make gendocs`.
var MobileSearchesTopic = docspec.Topic{
	ID:      `docs/mobile-searches`,
	Title:   `Mobile advanced searches`,
	Section: `Resources`,
	Keywords: []string{
		`mobile search`,
		`mobile-search`,
		`mobile_search`,
		`mobile_searches`,
		`advanced mobile device search`,
		`advanced-mobile-device-searches`,
		`advancedmobiledevicesearches`,
		`mobile-acs`,
		`jamf reports`,
	},
	Summary: "Jamf **Reports** for mobile devices: Pro JSON CRUD for the saved search definition (`/api/v1/advanced-mobile-device-searches`) and Classic `GET` for result rows (`mobile_devices` on `/JSSResource/advancedmobiledevicesearches`). Pro GET has **no membership**. Split like computer searches: `mobile_search` is definition-only; `mobile_search_devices` runs the search.",
	Surfaces: []docspec.SurfaceRow{
		{Action: "List", BuddyScript: "`mobile_searches(filter?, page?, page_size?)`", CLI: "`buddy mobile-search list`", Notes: "Pro paginated `{id, name}`; omit page = all; client-side filter"},
		{Action: "Load definition", BuddyScript: "`mobile_search(id=…)` / `mobile_search(name=…)`", CLI: "`buddy mobile-search get` / `get id <name>`", Notes: "Pro JSON; no devices"},
		{Action: "Run / results", BuddyScript: "`mobile_search_devices(id, filter?, fields?)`", CLI: "`buddy mobile-search results [--fields=…] [--csv] [--id]`", Notes: "CSV uses spaced headers and omits id unless `--id`; `--fromjson`/`--flatten` for JSON-string cells"},
		{Action: "Create", BuddyScript: "`mobile_search_create(body)`", CLI: "`create --file=` or `create <name>`", Notes: "Pro camelCase `andOr` / `searchType`"},
		{Action: "Replace", BuddyScript: "`mobile_search_put(id, body)`", CLI: "`put <search-id> --file=`", Notes: "Full Pro PUT"},
		{Action: "Delete", BuddyScript: "`mobile_search_delete(id=…)` / `ids=…`", CLI: "`delete <search-id> […]`", Notes: "One or more IDs"},
	},
	Notes: []string{
		"Definition CRUD is Pro `/api/v1/advanced-mobile-device-searches`. Result rows come from Classic `/JSSResource/advancedmobiledevicesearches/id/{id}` (`mobile_devices`).",
		"Pro list is paginated; CLI list fetches all pages. `filter` on list is a client-side substring on id/name.",
		"Pass Jamf JSON through: Pro camelCase `andOr` / `searchType`. This is not Classic user searches and not Pro advanced-user-content-searches (VPP content).",
	},
	AgentNotes: []string{
		"Do not use Pro GET `/api/v1/advanced-mobile-device-searches/{id}` for results — that document has no membership. Use Classic `mobile_devices` via `mobile_search_devices` / `results`.",
		"Do not treat Pro `advanced-user-content-searches` as user inventory searches (Classic `advancedusersearches`). Content searches are VPP (Content Name, Price) and are out of this slice.",
		"Do not mix with `computer_search*` or Platform `ddm_*` / `benchmark_*` reports.",
		"Do not add `buddy report`. User searches wait until a user inventory command exists.",
	},
	Builtins: []docspec.Builtin{
		{
			Name:    `mobile_searches`,
			Summary: `List advanced mobile-device searches`,
			Returns: `list of {id, name}`,
			Params: []docspec.Param{
				{Name: `filter`, Type: `string`, Required: false, Description: `client-side substring on id/name`},
				{Name: `page`, Type: `int`, Required: false, Description: `0-based; omit to fetch all pages`},
				{Name: `page_size`, Type: `int`, Required: false, Description: `defaults to 200`},
			},
		},
		{
			Name:    `mobile_search`,
			Summary: `Load one Pro search definition`,
			Returns: `dict`,
		},
		{
			Name:    `mobile_search_devices`,
			Summary: `Run a saved search and return Classic device rows`,
			Returns: `list of dict`,
			Params: []docspec.Param{
				{Name: `id`, Type: `string`, Required: true, Description: `search-id`},
				{Name: `filter`, Type: `string`, Required: false, Description: `client-side substring on any result field`},
				{Name: `fields`, Type: `string|list`, Required: false, Description: `display-field names to keep`},
			},
		},
		{
			Name:    `mobile_search_create`,
			Summary: `Create a Pro advanced mobile-device search`,
			Returns: `dict {id, name}`,
			Params: []docspec.Param{
				{Name: `body`, Type: `dict`, Required: true, Description: `Pro JSON (name required)`},
			},
		},
		{
			Name:    `mobile_search_put`,
			Summary: `Replace a Pro advanced mobile-device search`,
			Returns: `None`,
			Params: []docspec.Param{
				{Name: `id`, Type: `string`, Required: true, Description: `search-id`},
				{Name: `body`, Type: `dict`, Required: true, Description: `full Pro JSON`},
			},
		},
		{
			Name:    `mobile_search_delete`,
			Summary: `Delete one or more mobile searches`,
			Returns: `None`,
		},
	},
	ExamplesBuddy: []string{
		`def main():
    rows = mobile_searches(filter="iPad")
    if not rows:
        return []
    sid = rows[0]["id"]
    defn = mobile_search(id=sid)
    devices = mobile_search_devices(id=sid)
    return {"name": defn.get("name"), "matches": len(devices)}`,
	},
	Related: []docspec.RelatedLink{
		{Title: `Mobile devices`, Ref: `/docs/docs/mobile-devices`},
		{Title: `Computer advanced searches`, Ref: `/docs/docs/computer-searches`},
		{Title: `Mobile-device groups`, Ref: `/docs/docs/mobile-groups`},
		{Title: `API endpoints`, Ref: `/docs/docs/api-endpoints`},
	},
	HelpExport: `mobile-searches`,
	BuiltinsHelp: `MOBILE SEARCH (Pro advanced-mobile-device-searches + Classic results)
  mobile_searches(filter?, page?, page_size?, tenant?, surface?, auth?)
      Does: List Pro advanced mobile-device searches.
      → value: list of {id, name}
      Params: filter?: str (client-side substring on id/name); page?: int (0-based; omit = all pages); page_size?: int (default 200).
  mobile_search(id=?) | mobile_search(name=?, tenant?, surface?, auth?)
      Does: Load one Pro advanced mobile-device search definition (no devices).
      → value: dict (full Pro JSON)
      Params: Pass either id or name, not both. Name is an exact, case-insensitive list match.
  mobile_search_devices(id, filter?, fields?, tenant?, surface?, auth?)
      Does: Return Classic result rows (GET advancedmobiledevicesearches; Pro GET has no membership).
      → value: list of dict (display-field keys kept)
      Params: id: str (search-id); filter?: str (client-side substring on any result field); fields?: str or list of str (comma-separated JSON keys; omit = all).
  mobile_search_create(body, tenant?, surface?, auth?)
      Does: POST a Pro advanced mobile-device search.
      → value: dict {id, name}
      Params: body: dict (name required). Pro camelCase andOr / searchType.
  mobile_search_put(id, body, tenant?, surface?, auth?)
      Does: PUT a full Pro advanced mobile-device search.
      → None
      Params: id: str; body: dict (full definition JSON).
  mobile_search_delete(id=?) | mobile_search_delete(ids=?, tenant?, surface?, auth?)
      Does: Delete one or more advanced mobile-device searches.
      → None
      Params: Pass either id (one or list) or ids (list), not both.

`,
	CLIHelp: `buddy mobile-search - Manage advanced mobile-device searches (Jamf Reports)

Usage:
  buddy [global-flags] mobile-search <subcommand> [args...]

Aliases: mobile-searches, mobile-acs

Subcommands:
  list [--filter=<term>] [--json]              List saved searches (id, name)
  get <search-id>                              Show Pro definition JSON
  get id <name>                                Resolve exact name to ID
  results <search-id> [--filter=<term>] [--fields=<name>[,<name>...]] [--fromjson=<name>[,<name>...]] [--flatten=<name>[,<name>...]] [--json|--csv] [--id] [--jq=<expr>] [--raw]
                                               Run the search; table of display fields
  create --file=<json>                         Create from Pro JSON
  create <name>                                Create a named empty search
  put <search-id> --file=<json>                Replace Pro definition
  delete <search-id> [search-id...]            Delete one or more searches

Flags for 'list':
  --filter=<term>   Client-side substring on id or name
  --json            Print JSON instead of a table

Flags for 'results':
  --filter=<term>                 Client-side substring on any result field
  --fields=<name>[,<name>...]     Columns to show (JSON keys); omit = all
  --json                          Print full result-row JSON (no truncation)
  --csv                           Print RFC 4180 CSV (spaces in headers; no id unless --id; no count footer)
  --id                            Include the id column in CSV
  --jq=<expr>                     Apply a jq expression after --fields
  --fromjson=<name>[,<name>...]   Parse JSON-string cells into objects (for jq / --json)
  --flatten=<name>[,<name>...]    Explode JSON objects into dotted columns (parses strings)
  --raw                           Print jq stream like jq -r (requires --jq)

Flags for 'create' / 'put':
  --file=<json>     Path to a JSON object, or - for stdin

Notes:
  Definitions: Pro /api/v1/advanced-mobile-device-searches. Results: Classic
  GET /JSSResource/advancedmobiledevicesearches (mobile_devices). Not user
  searches and not Pro advanced-user-content-searches (VPP content).
  --json, --csv, and --raw are mutually exclusive. --raw requires --jq.
  CSV headers use spaces instead of underscores. The id column is omitted unless --id.

Examples:
  buddy mobile-search list --filter=iPad
  buddy mobile-acs get 88
  buddy mobile-search results 88 --json
  buddy mobile-acs results 88 --fields=name
  buddy mobile-acs results 88 --fields=name --csv
  buddy mobile-acs results 88 --csv --id
  buddy mobile-search results 88 --jq='.[].name' --raw
  buddy mobile-acs results 88 --fromjson=Wi_Fi_History --flatten=Wi_Fi_History --csv
  buddy mobile-search create "Lab iPads"
  buddy mobile-search put 88 --file=search.json
  buddy mobile-search delete 88
`,
}
