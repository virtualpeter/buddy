package buddy

import "buddy/internal/docspec"

func init() {
	docspec.Register(ComputerSearchesTopic)
}

// ComputerSearchesTopic is the structured doc source for docs/computer-searches.
// Edit here, then run `make gendocs`.
var ComputerSearchesTopic = docspec.Topic{
	ID:      `docs/computer-searches`,
	Title:   `Computer advanced searches`,
	Section: `Resources`,
	Keywords: []string{
		`computer search`,
		`computer-search`,
		`computer_search`,
		`computer_searches`,
		`advanced computer search`,
		`advancedcomputersearches`,
		`computerreports`,
		`acs`,
		`jamf reports`,
	},
	Summary: "Jamf **Reports** for Macs are Classic **advanced computer searches**. Buddy lists and mutates the saved search definition (`criteria`, `display_fields`) and runs it for result rows. `get` / `computer_search` strip the embedded `computers` array so a definition load stays small; `results` / `computer_search_computers` returns those rows. There is no Pro JSON CRUD for this object.",
	Surfaces: []docspec.SurfaceRow{
		{Action: "List", BuddyScript: "`computer_searches(filter?)`", CLI: "`buddy computer-search list`", Notes: "Classic `{id, name}`; client-side substring on id/name"},
		{Action: "Load definition", BuddyScript: "`computer_search(id=…)` / `computer_search(name=…)`", CLI: "`buddy computer-search get` / `get id <name>`", Notes: "`computers` stripped"},
		{Action: "Run / results", BuddyScript: "`computer_search_computers(id, filter?, fields?)`", CLI: "`buddy computer-search results [--fields=…] [--csv] [--id]`", Notes: "CSV uses spaced headers and omits id unless `--id`; `--fromjson`/`--flatten` for JSON-string cells"},
		{Action: "Create", BuddyScript: "`computer_search_create(body)`", CLI: "`create --file=` or `create <name>`", Notes: "Classic JSON (`and_or` / `search_type`); POST `…/id/0`"},
		{Action: "Replace", BuddyScript: "`computer_search_put(id, body)`", CLI: "`put <search-id> --file=`", Notes: "Full PUT; `computers` stripped before send"},
		{Action: "Delete", BuddyScript: "`computer_search_delete(id=…)` / `ids=…`", CLI: "`delete <search-id> […]`", Notes: "One or more IDs"},
	},
	Notes: []string{
		"Jamf’s Reports UI is this saved-search object, not DDM / benchmark / blueprint status reports.",
		"Definition CRUD is Classic `/JSSResource/advancedcomputersearches`. Result rows prefer Classic `/JSSResource/computerreports` for the same search ID, then fall back to the ACS `computers` array.",
		"Pass Jamf JSON through: Classic snake_case `and_or` / `search_type`. Criteria helpers like `smart_group_criteria` are not in this slice.",
	},
	AgentNotes: []string{
		"Do not treat `computerreports` as a second resource noun — it is the results path for the same advanced-search IDs.",
		"Do not invent a Pro JSON catalog for advanced computer searches; there is none.",
		"Do not return `computers` from `computer_search` / `get` — use `computer_search_computers` / `results`.",
		"Do not conflate this with per-Mac Classic `computerhardwaresoftwarereports` / `computerapplicationusage`, fleet `computerapplications`, or Platform `ddm_*` / `benchmark_*` / `blueprint_status`.",
		"Do not add `buddy report`. Copy/migrate is not in this slice.",
	},
	Builtins: []docspec.Builtin{
		{
			Name:    `computer_searches`,
			Summary: `List advanced computer searches`,
			Returns: `list of {id, name}`,
			Params: []docspec.Param{
				{Name: `filter`, Type: `string`, Required: false, Description: `client-side substring on id/name`},
			},
		},
		{
			Name:    `computer_search`,
			Summary: `Load one search definition (no computers)`,
			Returns: `dict`,
		},
		{
			Name:    `computer_search_computers`,
			Summary: `Run a saved search and return result rows`,
			Returns: `list of dict (display fields kept)`,
			Params: []docspec.Param{
				{Name: `id`, Type: `string`, Required: true, Description: `search-id`},
				{Name: `filter`, Type: `string`, Required: false, Description: `client-side substring on any result field`},
				{Name: `fields`, Type: `string|list`, Required: false, Description: `display-field names to keep (JSON keys or Computer Name)`},
			},
		},
		{
			Name:    `computer_search_create`,
			Summary: `Create an advanced computer search`,
			Returns: `dict {id, name}`,
			Params: []docspec.Param{
				{Name: `body`, Type: `dict`, Required: true, Description: `Classic JSON (name required)`},
			},
		},
		{
			Name:    `computer_search_put`,
			Summary: `Replace an advanced computer search`,
			Returns: `None`,
			Params: []docspec.Param{
				{Name: `id`, Type: `string`, Required: true, Description: `search-id`},
				{Name: `body`, Type: `dict`, Required: true, Description: `full Classic JSON`},
			},
		},
		{
			Name:    `computer_search_delete`,
			Summary: `Delete one or more advanced computer searches`,
			Returns: `None`,
		},
	},
	ExamplesBuddy: []string{
		`def main():
    rows = computer_searches(filter="Chrome")
    if not rows:
        return []
    sid = rows[0]["id"]
    defn = computer_search(id=sid)
    comps = computer_search_computers(id=sid, filter="lab")
    return {"name": defn.get("name"), "matches": len(comps)}`,
	},
	Related: []docspec.RelatedLink{
		{Title: `Computers`, Ref: `/docs/docs/computers`},
		{Title: `Mobile advanced searches`, Ref: `/docs/docs/mobile-searches`},
		{Title: `Smart computer groups`, Ref: `/docs/docs/smart-groups`},
		{Title: `API endpoints`, Ref: `/docs/docs/api-endpoints`},
	},
	HelpExport: `computer-searches`,
	BuiltinsHelp: `COMPUTER SEARCH (Classic advancedcomputersearches / Reports)
  computer_searches(filter?, tenant?, surface?, auth?)
      Does: List Classic advanced computer searches.
      → value: list of {id, name}
      Params: filter?: str (client-side substring on id/name). Classic list has no RSQL.
  computer_search(id=?) | computer_search(name=?, tenant?, surface?, auth?)
      Does: Load one advanced computer search definition (computers stripped).
      → value: dict (Classic JSON: name, criteria, display_fields, site, …)
      Params: Pass either id or name, not both.
  computer_search_computers(id, filter?, fields?, tenant?, surface?, auth?)
      Does: Return result rows for a saved search (computerreports, then ACS computers).
      → value: list of dict (display-field keys kept, e.g. Computer_Name)
      Params: id: str (search-id); filter?: str (client-side substring on any result field); fields?: str or list of str (comma-separated JSON keys or "Computer Name"; omit = all).
  computer_search_create(body, tenant?, surface?, auth?)
      Does: POST a Classic advanced computer search (JSON to …/id/0).
      → value: dict {id, name}
      Params: body: dict (name required; computers stripped). Classic snake_case and_or / search_type.
  computer_search_put(id, body, tenant?, surface?, auth?)
      Does: PUT a full Classic advanced computer search (computers stripped).
      → None
      Params: id: str; body: dict (full definition JSON).
  computer_search_delete(id=?) | computer_search_delete(ids=?, tenant?, surface?, auth?)
      Does: Delete one or more advanced computer searches.
      → None
      Params: Pass either id (one or list) or ids (list), not both.

`,
	CLIHelp: `buddy computer-search - Manage Classic advanced computer searches (Jamf Reports)

Usage:
  buddy [global-flags] computer-search <subcommand> [args...]

Aliases: computer-searches, acs

Subcommands:
  list [--filter=<term>] [--json]              List saved searches (id, name)
  get <search-id>                              Show definition JSON (no computers)
  get id <name>                                Resolve exact name to ID
  results <search-id> [--filter=<term>] [--fields=<name>[,<name>...]] [--fromjson=<name>[,<name>...]] [--flatten=<name>[,<name>...]] [--json|--csv] [--id] [--jq=<expr>] [--raw]
                                               Run the search; table of display fields
  create --file=<json>                         Create from Classic JSON
  create <name>                                Create a named empty search
  put <search-id> --file=<json>                Replace definition (computers stripped)
  delete <search-id> [search-id...]            Delete one or more searches

Flags for 'list':
  --filter=<term>   Client-side substring on id or name
  --json            Print JSON instead of a table

Flags for 'results':
  --filter=<term>                 Client-side substring on any result field
  --fields=<name>[,<name>...]     Columns to show (JSON keys or "Computer Name"); omit = all
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
  Classic /JSSResource/advancedcomputersearches. Results prefer computerreports
  for the same search-id. This is the Jamf Reports UI, not DDM/benchmark reports.
  --json, --csv, and --raw are mutually exclusive. --raw requires --jq.
  CSV headers use spaces instead of underscores. The id column is omitted unless --id.

Examples:
  buddy computer-search list --filter=Chrome
  buddy acs get 42
  buddy computer-search results 42 --filter=lab --json
  buddy acs results 219 --fields=Computer_Name,Username,Last_Check_in
  buddy acs results 219 --fields=Computer_Name,Username,Last_Check_in --csv
  buddy acs results 219 --csv --id
  buddy acs results 219 --jq='map({name: .Computer_Name, user: .Username})' --csv
  buddy acs results 219 --jq='.[].Computer_Name' --raw
  buddy acs results 219 --fields=Computer_Name,Wi_Fi_History --fromjson=Wi_Fi_History --flatten=Wi_Fi_History --csv
  buddy computer-search create "Lab Macs"
  buddy computer-search put 42 --file=search.json
  buddy computer-search delete 42
`,
}
