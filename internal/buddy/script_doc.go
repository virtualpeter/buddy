package buddy

import "buddy/internal/docspec"

func init() {
	docspec.Register(ScriptsTopic)
}

// ScriptsTopic is the structured doc source for docs/scripts.
// Edit here, then run `make gendocs`.
var ScriptsTopic = docspec.Topic{
	ID:      `docs/scripts`,
	Title:   `Scripts`,
	Section: `Resources`,
	Keywords: []string{
		`script`,
		`scripts`,
		`script_copy`,
		`script_migrate`,
		`scriptContents`,
	},
	Summary: `Jamf Pro scripts — list summaries vs full body, create/update, and copy/migrate.`,
	Surfaces: []docspec.SurfaceRow{
		{Action: "List", BuddyScript: "`scripts(filter?)`", CLI: "`buddy script list`", Notes: "Summaries `{id, name, categoryId, categoryName}` only; bodies stripped"},
		{Action: "Load", BuddyScript: "`script(id=…)` / `script_get_contents(script_id=…)`", CLI: "`buddy script get` / `get contents`", Notes: "Full record / body"},
		{Action: "Create / update", BuddyScript: "`script_put` / `script_update`", CLI: "`buddy script create` / `upload`", Notes: "Full dict or from file"},
		{Action: "Copy / migrate", BuddyScript: "`script_copy` / `script_migrate`", Notes: "Upsert by name; category remapped on destination"},
	},
	Notes: []string{
		"Bare `filter` terms become `name==\"*term*\"` on the server. List JSON never yields usable `scriptContents` — load with `script(id=…)`.",
	},
	AgentNotes: []string{
		"Do not expect `scripts(...)` to return script bodies — use `script(id=…)` for contents.",
		"Migrate scripts before policies that reference them (see migrate order under [policies](/docs/docs/policies) / [categories](/docs/docs/categories)).",
	},
	Builtins: []docspec.Builtin{
			{
				Name: `scripts`,
				Summary: `List script summaries`,
				Returns: `list of {id,name,…}`,
				Params: []docspec.Param{
					{Name: `filter`, Type: `string`, Required: false, Description: `filter`},
				},
			},
			{
				Name: `script`,
				Summary: `Load full script`,
				Returns: `dict`,
			},
			{
				Name: `script_copy`,
				Summary: `Copy/migrate script`,
				Returns: `dict`,
				Params: []docspec.Param{
					{Name: `script_id`, Type: `string`, Required: true, Description: `script_id`},
				},
			},
		},
	ExamplesBuddy: []string{
		`def main():
    rows = scripts(filter="Chrome")
    if not rows:
        return []
    return script(id=rows[0]["id"])`,
	},
	Related: []docspec.RelatedLink{
		{Title: `Categories`, Ref: `/docs/docs/categories`},
		{Title: `Policies`, Ref: `/docs/docs/policies`},
		{Title: `API endpoints`, Ref: `/docs/docs/api-endpoints`},
	},
	HelpExport: `scripts`,
	BuiltinsHelp: `SCRIPT (Jamf Pro scripts)
  scripts(filter?, page?, page_size?, tenant?, surface?, auth?)
      → value: list of {id, name, categoryId, categoryName}
      Params: filter?: bare term → name=="*term*", or full RSQL.
  script(id=?) | script(name=?, tenant?, surface?, auth?)
      → value: full script dict (id=) or id string (name=)
  script_get_contents(script_id, tenant?, surface?, auth?) → value: str
  script_update(script_id, file, tenant?, surface?, auth?) → None
  script_put(script_id, dict, tenant?, surface?, auth?) → value: dict
  script_clone(script_id, name, category_id?, tenant?, surface?, auth?) → value: dict {id, name}
      Params: Alias of script_copy without to_*.
  script_copy(script_id, name?, category_id?, upsert?=False,
              tenant?, surface?, auth?, to_tenant?, to_surface?, to_auth?) → value: dict {id, name} or +created
      Params: Source = tenant?/auth?; dest = to_* (omit = same). upsert=True upserts by name.
  script_migrate(...) → same as script_copy(..., upsert=True)

`,
	CLIHelp: `buddy script - Manage Jamf Pro scripts

Usage:
  buddy [global-flags] script <subcommand> [args...]

Subcommands:
  list [--filter=<term|rsql>]                         List scripts (bare term → name contains; or RSQL)
  get <script-id>                                     Show full script record (includes body)
  get contents <script-id>                            Output raw script body only (pipeable)
  get id <name>                                       Resolve script name to numeric ID
  history <script-id>                                 Show recent change history (last 20 entries)
  create <name> <file> [flags]                        Create a new script from a local file
        [--category=<category-id>] [--priority=BEFORE|AFTER|AT_REBOOT] [--info=<text>] [--notes=<text>]
  upload <script-id> <file> [--name=<new-name>]       Upload (replace) contents of an existing script
  update <script-id> <file> [--name=<new-name>]       Alias for upload
  download <script-id> [outfile]                      Save script body to a local file (default: <name>.sh)
  delete <script-id> [script-id...]                   Delete one or more scripts
  set name     <script-id> <value>                    Rename script
  set category <script-id> <category-id>              Set category
  set priority <script-id> BEFORE|AFTER|AT_REBOOT    Set execution priority
  set info     <script-id> <value>                    Set info field
  set notes    <script-id> <value>                    Set notes field

Use 'buddy script get help' or 'buddy script set help' for more detail.

Examples:
  buddy script list --filter=Chrome
  buddy script list --filter='name=="*Chrome*"'
  buddy script get 7
  buddy script create "Install Chrome" chrome.sh --priority=AFTER
  buddy script upload 7 chrome.sh
  buddy script download 7
  buddy script delete 7 8
`,
}
