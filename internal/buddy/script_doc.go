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
		`script_delete`,
		`scriptContents`,
	},
	Summary: `Jamf Pro scripts — list summaries vs full body, create/update/delete, and copy/migrate.`,
	Surfaces: []docspec.SurfaceRow{
		{Action: "List", BuddyScript: "`scripts(filter?)`", CLI: "`buddy script list`", Notes: "Summaries `{id, name, categoryId, categoryName}` only; bodies stripped"},
		{Action: "Load", BuddyScript: "`script(id=…)` / `script(name=…)` / `script_get_contents(id=…)`", CLI: "`buddy script get` / `get contents`", Notes: "Full Jamf JSON dict (id= and name=); or body only"},
		{Action: "Create / update", BuddyScript: "`script_create` / `script_put` / `script_update`", CLI: "`buddy script create` / `upload`", Notes: "Create from string body; PUT dict or replace from file"},
		{Action: "Delete", BuddyScript: "`script_delete(id=…)` / `script_delete(ids=…)`", CLI: "`buddy script delete`", Notes: "One ID or a list"},
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
			Name:    `scripts`,
			Summary: `List script summaries`,
			Returns: `list of {id,name,…}`,
			Params: []docspec.Param{
				{Name: `filter`, Type: `string`, Required: false, Description: `filter`},
			},
		},
		{
			Name:    `script`,
			Summary: `Load full script`,
			Returns: `dict`,
		},
		{
			Name:    `script_copy`,
			Summary: `Copy/migrate script`,
			Returns: `dict`,
			Params: []docspec.Param{
				{Name: `id`, Type: `string`, Required: true, Description: `id`},
			},
		},
		{
			Name:    `script_delete`,
			Summary: `Delete one or more scripts`,
			Returns: `None`,
			Params: []docspec.Param{
				{Name: `id`, Type: `string|int`, Required: false, Description: `one script ID (not with ids=)`},
				{Name: `ids`, Type: `string|int|list`, Required: false, Description: `script ID or list of IDs (not with id=)`},
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
      Does: List Jamf Pro scripts.
      → value: list of {id, name, categoryId, categoryName}
      Params: filter?: bare term → name=="*term*", or full RSQL. page?: int (omit = all);
        page_size?: int (default 200).
  script(id=?) | script(name=?, tenant?, surface?, auth?)
      Does: Load one script by id or exact name (case-insensitive).
      → value: dict (full Jamf JSON: id, name, categoryId, scriptContents, …)
      Params: pass either id= or name=, not both. name= uses RSQL name=="…" and
        returns that list row when it includes scriptContents; otherwise GET by id.
  script_get_contents(id, tenant?, surface?, auth?)
      Does: Return the raw script body only.
      → value: str
      Params: id: str/int.
  script_create(name, script_contents, category_id?, priority?, info?, notes?, os_requirements?,
                parameter4?, parameter5?, parameter6?, parameter7?, parameter8?,
                parameter9?, parameter10?, parameter11?, tenant?, surface?, auth?)
      Does: Create a Jamf Pro script (POST /api/v1/scripts). Body is script_contents (not a file).
      → value: dict (created script; same shape as script(id=…))
      Params: name: str; script_contents: str (same field as script(id=…)["scriptContents"]). category_id?: str. priority?: BEFORE|AFTER|AT_REBOOT.
        info?/notes?/os_requirements?: str. parameter4?…parameter11?: str (Jamf script parameters).
        tenant?/surface?/auth?: omit for process defaults.
      Notes: Jamf POST returns {id, href}. The builtin returns the posted script with id applied
        (plus any extra Script fields in the POST body). No follow-up GET.
  script_update(id, file, tenant?, surface?, auth?)
      Does: Replace a script's body from a local file.
      → None
      Params: id: str/int; file: str (filesystem path).
  script_put(id, dict, tenant?, surface?, auth?)
      Does: PUT a full script JSON record.
      → value: dict (updated script; PUT body confirms the write)
      Params: id: str/int; dict: full script body.
  script_delete(id=?) | script_delete(ids=?, tenant?, surface?, auth?)
      Does: Delete one or more Jamf Pro scripts (DELETE /api/v1/scripts/{id}).
      → None
      Params: pass either id= (one script) or ids= (one ID or list of IDs), not both.
        IDs are str/int. tenant?/surface?/auth?: omit for process defaults.
  script_clone(id, name, category_id?, tenant?, surface?, auth?)
      Does: Clone a script in the same session (alias of script_copy without to_*).
      → value: dict (created script; same shape as script(id=…))
      Params: id: str/int; name: str; category_id?: str.
  script_copy(id, name?, category_id?, upsert?=False,
              tenant?, surface?, auth?, to_tenant?, to_surface?, to_auth?)
      Does: Copy a script to another (or the same) session, remapping category by name.
      → value: dict (dest script; same shape as script(id=…); +created when upsert)
      Params: id: source. name?: dest name. category_id?: dest category.
        upsert?: bool (upsert by exact name). Source = tenant?/auth?; dest = to_* (omit = same).
      Notes: Create is POST (posted script + id). Existing dest is PUT (response confirms the write).
  script_migrate(...)
      Does: Alias of script_copy with upsert=True.
      → value: dict (dest script + created bool)

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
