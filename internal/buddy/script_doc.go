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
		`script_clone`,
		`script_delete`,
		`script_check`,
		`scripts_check`,
		`script_diff`,
		`script_download`,
		`script_rename`,
		`script_history`,
		`script_local_filename`,
		`script_local_stem`,
		`scriptContents`,
	},
	Summary: `Jamf Pro scripts — list summaries vs full body, create/update/delete, and copy/migrate.`,
	Surfaces: []docspec.SurfaceRow{
		{Action: "List", BuddyScript: "`scripts(filter?)`", CLI: "`buddy script list`", Notes: "Summaries `{id, name, categoryId, categoryName, localFilename}`. Omit page to fetch every page. `--page` fetches one page."},
		{Action: "Load", BuddyScript: "`script(id=…)` / `script(name=…)` / `script(file=…)` / `script_contents(id=…)`", CLI: "`buddy script get` / `get contents` / `get id`", Notes: "`script()` and the get commands accept an id, an exact name, or a local filename. The local filename matches with or without the file on disk. `script_contents(id)` returns the body for an id."},
		{Action: "Create / update", BuddyScript: "`script_create` / `script_put` / `script_update`", CLI: "`buddy script create` / `upload|update` / `put`", Notes: "CLI create reads a file. `script_create` takes the body as a string. Both accept os requirements and parameter4–parameter11, and both refuse a taken name or download filename. `upload` and `update` replace the script that owns the file. `put` merges known JSON keys onto the current record."},
		{Action: "Check", BuddyScript: "`script_check(file, id?)`", CLI: "`buddy script check <file> [<script-id>]`", Notes: "SHA-256 of the local file against the script body. The file basename is slugged to its download name. `missing` is ok=False. Exit 0 only on a match."},
		{Action: "Directory check", BuddyScript: "`scripts_check(dir)`", CLI: "", Notes: "Compare every file in a directory with Jamf by download filename. Rows are `match`, `mismatch`, `missing`, or `remote-only`."},
		{Action: "Diff", BuddyScript: "`script_diff(file, id?, name?)`", CLI: "`buddy script diff <file> [<script>]`", Notes: "Same text as the diff command. The Jamf body is the first file and the local file is the second. Exit 0 only when the bytes match."},
		{Action: "Download / rename / history", BuddyScript: "`script_download` / `script_rename` / `script_history`", CLI: "`buddy script download` / `rename` / `history`", Notes: "Download writes the download filename when path is omitted. Rename of a local file also renames that file. History accepts an id, exact name, or local file."},
		{Action: "Delete", BuddyScript: "`script_delete(id=…)` / `script_delete(ids=…)` / `script_delete(name=…)` / `script_delete(file=…)`", CLI: "`buddy script delete`", Notes: "One mode per call. The local file is left in place."},
		{Action: "Copy / migrate", BuddyScript: "`script_copy` / `script_clone` / `script_migrate`", CLI: "`buddy script copy` / `clone` / `migrate`", Notes: "The builtins take a source id. The CLI accepts an id, exact name, or local file. Clone stays in the same session. Copy and migrate remap category by name. Migrate upserts by exact name."},
	},
	Notes: []string{
		"Bare `filter` terms become `name==\"*term*\"` on the server. List results omit `scriptContents`. Load a body with `script(id=…)`, `script(name=…)`, or `script(file=…)`.",
	},
	AgentNotes: []string{
		"scripts() returns summaries with localFilename and no script body. Load a body with script(id=), script(name=), or script(file=).",
		"Migrate scripts before policies that reference them (see migrate order under [policies](/docs/docs/policies) / [categories](/docs/docs/categories)).",
	},
	Builtins: []docspec.Builtin{
		{
			Name:    `scripts`,
			Summary: `List script summaries`,
			Returns: `list of {id,name,localFilename,…}`,
			Params: []docspec.Param{
				{Name: `filter`, Type: `string`, Required: false, Description: `bare term or RSQL`},
				{Name: `page`, Type: `integer`, Required: false, Description: `0-based page; omit to fetch every page`},
				{Name: `page_size`, Type: `integer`, Required: false, Description: `page size (default 200)`},
			},
		},
		{
			Name:    `script`,
			Summary: `Load one full script`,
			Returns: `dict`,
			Params: []docspec.Param{
				{Name: `id`, Type: `string|int`, Required: false, Description: `script id`},
				{Name: `name`, Type: `string`, Required: false, Description: `exact Jamf name, or a local filename when that exact name is absent`},
				{Name: `file`, Type: `string`, Required: false, Description: `local path; basename is matched as a download filename`},
			},
			Notes: []string{`Pass exactly one of id, name, or file.`},
		},
		{
			Name:    `script_local_filename`,
			Summary: `Download filename for a Jamf script name`,
			Returns: `str`,
			Params: []docspec.Param{
				{Name: `name`, Type: `string`, Required: true, Description: `Jamf script name`},
			},
		},
		{
			Name:    `script_local_stem`,
			Summary: `Stem of a download filename`,
			Returns: `str`,
			Params: []docspec.Param{
				{Name: `filename`, Type: `string`, Required: true, Description: `basename that is already a download name`},
			},
		},
		{
			Name:    `script_check`,
			Summary: `Compare a local file to a script body`,
			Returns: `dict {ok, class, id, name, file, sha256, localSha256}`,
			Params: []docspec.Param{
				{Name: `file`, Type: `string`, Required: true, Description: `local file`},
				{Name: `id`, Type: `string|int`, Required: false, Description: `script id; omit to use the download filename`},
			},
			Notes: []string{`class is match, mismatch, or missing. missing is ok=False and does not raise.`},
		},
		{
			Name:    `scripts_check`,
			Summary: `Compare a directory of files with Jamf scripts`,
			Returns: `list of {class, path, file, id, name}`,
			Params: []docspec.Param{
				{Name: `dir`, Type: `string`, Required: true, Description: `directory of script files`},
			},
			Notes: []string{`class is match, mismatch, missing, or remote-only. Subdirectories are skipped. A local file owned by several scripts raises. Two local files that slug to one name raise.`},
		},
		{
			Name:    `script_diff`,
			Summary: `Diff a local file against a script body`,
			Returns: `dict {ok, class, id, name, file, diff}`,
			Params: []docspec.Param{
				{Name: `file`, Type: `string`, Required: true, Description: `local file`},
				{Name: `id`, Type: `string|int`, Required: false, Description: `script id`},
				{Name: `name`, Type: `string`, Required: false, Description: `exact Jamf name or local filename`},
			},
			Notes: []string{`Pass id= or name=, not both. Omit both to use the file's download name. class is match, differ, or missing. diff is the same text as the diff command, empty on match and missing. < lines are the Jamf body. > lines are the local file.`},
		},
		{
			Name:    `script_download`,
			Summary: `Write a script body to a local file`,
			Returns: `str`,
			Params: []docspec.Param{
				{Name: `id`, Type: `string|int`, Required: false, Description: `script id`},
				{Name: `name`, Type: `string`, Required: false, Description: `exact Jamf name or local filename`},
				{Name: `path`, Type: `string`, Required: false, Description: `output path; omit to use the download filename`},
			},
			Notes: []string{`Pass exactly one of id or name.`},
		},
		{
			Name:    `script_rename`,
			Summary: `Rename a script`,
			Returns: `None`,
			Params: []docspec.Param{
				{Name: `name`, Type: `string`, Required: true, Description: `new Jamf name`},
				{Name: `id`, Type: `string|int`, Required: false, Description: `script id`},
				{Name: `file`, Type: `string`, Required: false, Description: `local download file; that file is renamed too`},
				{Name: `from`, Type: `string`, Required: false, Description: `exact current Jamf name or local filename`},
			},
			Notes: []string{`Pass exactly one of id, file, or from.`},
		},
		{
			Name:    `script_history`,
			Summary: `Recent audit history for one script`,
			Returns: `list of {id, username, date, note, details}`,
			Params: []docspec.Param{
				{Name: `id`, Type: `string|int`, Required: false, Description: `script id`},
				{Name: `name`, Type: `string`, Required: false, Description: `exact Jamf name or local filename`},
				{Name: `file`, Type: `string`, Required: false, Description: `local download file`},
				{Name: `limit`, Type: `integer`, Required: false, Description: `default 20`},
			},
			Notes: []string{`Pass exactly one of id, name, or file.`},
		},
		{
			Name:    `script_copy`,
			Summary: `Copy a script to this session or another`,
			Returns: `dict`,
			Params: []docspec.Param{
				{Name: `id`, Type: `string`, Required: true, Description: `source script id`},
				{Name: `name`, Type: `string`, Required: false, Description: `destination name; required unless upsert`},
				{Name: `category_id`, Type: `string`, Required: false, Description: `destination category`},
				{Name: `upsert`, Type: `boolean`, Required: false, Description: `upsert by exact name`},
				{Name: `to_tenant`, Type: `string`, Required: false, Description: `destination tenant`},
				{Name: `to_surface`, Type: `string`, Required: false, Description: `destination surface`},
				{Name: `to_auth`, Type: `string`, Required: false, Description: `destination auth`},
			},
			Notes: []string{`script_clone is the same session. script_migrate is upsert=True.`},
		},
		{
			Name:    `script_delete`,
			Summary: `Delete one or more scripts`,
			Returns: `None`,
			Params: []docspec.Param{
				{Name: `id`, Type: `string|int`, Required: false, Description: `one script ID`},
				{Name: `ids`, Type: `string|int|list`, Required: false, Description: `script ID or list of IDs`},
				{Name: `name`, Type: `string`, Required: false, Description: `exact Jamf name or local filename`},
				{Name: `file`, Type: `string`, Required: false, Description: `local download file; the file is left in place`},
			},
			Notes: []string{`Pass exactly one of id, ids, name, or file.`},
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
      → value: list of {id, name, categoryId, categoryName, localFilename}
      Params: filter?: bare term → name=="*term*", or full RSQL. page?: int (omit = all);
        page_size?: int (default 200). localFilename is the download name.
  script(id=?) | script(name=?) | script(file=?, tenant?, surface?, auth?)
      Does: Load one script by id, exact name, or local filename.
      → value: dict (full Jamf JSON: id, name, categoryId, scriptContents, …)
      Params: pass exactly one of id=, name=, or file=. name= is an exact Jamf name, or a
        local filename when no script has that exact name. restart-jamf-agent.sh finds
        Restart Jamf Agent!. file= matches by the download name of its basename, so
        notes.py finds notes.py. The file does not have to exist. Zero or several
        download-name hits raise and list ids.
  script_local_filename(name)
      Does: Return the download filename for a Jamf script name. No API call.
      → value: str
      Params: name: str. Install Chrome.SH → Install-Chrome.SH. notes.py → notes.py.
        Letter case is kept. A lookup still matches that name in any case.
  script_local_stem(filename)
      Does: Return the stem of a download filename. No API call.
      → value: str
      Params: filename: str. install-chrome.sh → install-chrome. notes.py → notes.
      Notes: The basename must already be a download name. For a .sh name,
        script_local_filename(script_local_stem(filename)) is that filename again.
        notes.py is a download name; its stem is notes, and script_local_filename("notes")
        is notes.sh. A name with spaces raises. This is the stem, not the original Jamf name.
  script_contents(id, tenant?, surface?, auth?)
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
      Notes: Fails when that exact name already exists, or when another script already
        downloads to script_local_filename(name). Jamf POST returns {id, href}.
        The builtin returns the posted script with id applied (plus any extra Script fields in the
        POST body). No follow-up GET.
  script_update(id?, file, name?, create?=False, tenant?, surface?, auth?)
      Does: Replace a script's body from a local file.
      → None
      Params: file: str (filesystem path, required). Pass id= or name= to choose the script.
        Omit both to update the script that owns that download filename. name= is an exact
        Jamf name or a local filename. The file's basename is slugged the same way.
        create?: bool. With no id= or name=, a missing script is created and named from
        the basename. Default false still raises when the script is absent.
  script_put(id, dict, tenant?, surface?, auth?)
      Does: Fetch the script, merge known keys from dict, and PUT the record.
      → value: dict (updated script; PUT body confirms the write)
      Params: id: str/int. dict keys: name, categoryId, info, notes, osRequirements, priority,
        parameter4–parameter11, scriptContents. Omitted keys stay as they are. Other keys are ignored.
  script_check(file, id?, tenant?, surface?, auth?)
      Does: SHA-256 the local file against the script body.
      → value: dict {ok, class, id, name, file, sha256, localSha256}
      Params: file: str. id?: str/int. Omit id to resolve the download name of the basename.
      Notes: class is match, mismatch, or missing. Missing is ok=False and does not raise.
        Several owners raise. Bytes are compared as stored. notes.py matches notes.py.
  scripts_check(dir, tenant?, surface?, auth?)
      Does: Compare each file in a directory with the Jamf script that downloads as that filename.
      → value: list of {class, path, file, id, name}
      Params: dir: str. Subdirectories are skipped.
      Notes: class is match, mismatch, missing (local file, no Jamf script), or remote-only
        (Jamf script, no local file). path is empty for remote-only. id and name are empty
        when missing. Rows are sorted by file, then id. A local file whose download name
        is owned by several scripts raises. Two local files that slug to one name raise.
        Remote-only rows are still listed when they share a name. Bytes are compared as stored.
  script_diff(file, id?, name?, tenant?, surface?, auth?)
      Does: Diff a local file against the script body.
      → value: dict {ok, class, id, name, file, diff}
      Params: file: str. Pass id= or name=, not both. Omit both to resolve the file's
        download name. name= is an exact Jamf name or a local filename. id= is a script id.
      Notes: class is match, differ, or missing. diff is the same text as the diff command,
        with the Jamf body as the first file and the local file as the second. Empty on
        match and missing. Missing is ok=False and does not raise. Several owners raise.
        Bytes are compared as stored.
  script_download(id?, path?, name?, tenant?, surface?, auth?)
      Does: Write a script body to a local file.
      → value: str (path written)
      Params: pass exactly one of id= or name=. name= is an exact Jamf name or a local
        filename. path?: str. Omit path to use the download filename.
  script_rename(id?, name, file?, from?, tenant?, surface?, auth?)
      Does: Rename a script. name is the new Jamf name.
      → None
      Params: pass exactly one of id=, file=, or from=. from= is the exact current Jamf name
        or a local filename.
        file= is a local download file and is renamed to the new download name.
      Notes: Refuses an existing name or a download filename owned by another script.
        file= also refuses when the destination path already exists.
  script_history(id?, name?, file?, limit?, tenant?, surface?, auth?)
      Does: Return recent audit history for one script.
      → value: list of {id, username, date, note, details}
      Params: pass exactly one of id=, name=, or file=. name= is an exact Jamf name or a
        local filename. limit?: int (default 20).
  script_delete(id=?) | script_delete(ids=?) | script_delete(name=?) | script_delete(file=?, tenant?, surface?, auth?)
      Does: Delete one or more Jamf Pro scripts (DELETE /api/v1/scripts/{id}).
      → None
      Params: pass exactly one of id= (one script), ids= (one ID or a list), name= (exact name
        or local filename), or file= (local file). IDs are str/int. A file argument does not delete the local file.
        tenant?/surface?/auth?: omit for process defaults.
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
  list                                              List scripts
  get <script>                                      Show full script record (includes body)
  get contents <script>                             Output raw script body only (pipeable)
  get id <script>                                   Print the numeric ID
  history <script>                                  Show recent change history (last 20 entries)
  create <file>
  create <name> <file>
  upload|update <file> [--name=<new-name>]          Replace the script that downloads as this filename
  upload|update <script-id>|<name> <file> [--name=<new-name>]
                                                    Replace by id, exact name, or local filename
  check <file> [<script-id>]                        SHA-256 the local file against the script body
  diff <file> [<script>]                            Diff the local file against the script body
  download <script-id>|<name> [outfile]             Save script body (default: download name)
  rename <script> <new-name>                        Rename by id, exact name, or local file
  delete <script> [<script>...]                     Delete by id, exact name, or local file
  put <script> <json-file>                          Merge known JSON fields onto the script
  clone <script> <name>                             Copy in the same session
  copy <script>                                     Copy to this session or another
  migrate <script>                                  Copy with upsert by exact name
  set name     <script> <value>                     Rename script (same identity as rename)
  set category <script-id> <category-id>            Set category
  set priority <script-id> BEFORE|AFTER|AT_REBOOT  Set execution priority
  set info     <script-id> <value>                  Set info field
  set notes    <script-id> <value>                  Set notes field

Flags for 'list':
  --filter=<term|rsql>   Bare term → name=="*term*"; or full RSQL
  --json                 Print JSON rows, including localFilename
  --page=<n>             Fetch this 0-based page only (omit to fetch every page)
  --page-size=<n>        Page size (default 200)

Flags for 'create':
  --category=<category-id>
  --priority=BEFORE|AFTER|AT_REBOOT
  --info=<text>
  --notes=<text>
  --os-requirements=<text>
  --parameter4=<text> … --parameter11=<text>

Flags for 'clone':
  --category=<category-id>   Category on the new script

Flags for 'copy':
  --name=<name>              Destination name. Required when --upsert is omitted
  --category=<category-id>   Category on the destination
  --upsert                   Upsert by exact name
  --to-tenant=<tenant>       Destination session
  --to-surface=<surface>
  --to-auth=<auth>

Flags for 'migrate':
  --name=<name>              Destination name. Omit to keep the source name
  --category=<category-id>   Category on the destination
  --to-tenant=<tenant>       Destination session
  --to-surface=<surface>
  --to-auth=<auth>

Notes:
  <script> is an id, an exact Jamf name, or a local filename.
  The local filename is the download name from list. restart-jamf-agent.sh finds
  Restart Jamf Agent! with or without that file in the working directory.
  An existing file matches by slugging its basename, so Install Chrome.SH finds
  Install-Chrome.SH. A trailing two-character extension is kept: notes.py stays notes.py.
  Letter case is kept. A lookup matches the download name in any case, so
  tabninekiller.sh finds TabNineKiller.sh.
  get id prints only the id. An existing file matches by download name. Any other
  get id token is an exact name, then a local filename. A numeric string that is
  not an existing file stays an exact name.
  Download names keep letter case. Spaces and punctuation become hyphens. A trailing
  two-character extension is kept (.sh, .py), in its own case. Anything else gets .sh.
  Install Chrome.SH downloads as Install-Chrome.SH. notes.py downloads as notes.py.
  Restart Jamf Agent! downloads as Restart-Jamf-Agent.sh. A .sh that is not at the
  end stays punctuation.
  create <file> accepts only that form and stores the filename as the Jamf name.
  create fails when the name or that download name is already used. upload and update
  are the same and fail when no script owns the file. check prints the id and exits
  0 only when the SHA-256 values match. diff prints the same text as the diff command
  and exits 0 only when the bytes match. The Jamf body is the first file. < lines are
  Jamf and > lines are the local file. The optional script is an id, an exact name, or
  a local filename.
  rename of a local file also renames that file.
  delete leaves the local file in place. More than one match is an error.
  list's Local Filename column is that download filename. --json adds localFilename to each summary.
  A file lookup asks Jamf for names containing the filename's tokens, then compares
  download names. When that page has no match, or Jamf rejects the filter, it lists every script once.

Examples:
  buddy script list --filter=Chrome
  buddy script list --page=0 --page-size=50
  buddy script get install-chrome.sh
  buddy script get id "Install Chrome"
  buddy script history install-chrome.sh
  buddy script create install-chrome.sh --priority=AFTER --parameter4=hello
  buddy script create "Install Chrome" install-chrome.sh
  buddy script update install-chrome.sh
  buddy script check install-chrome.sh
  buddy script diff restart-jamf-agent.sh
  buddy script put 7 script.json
  buddy script clone 7 "Install Chrome copy"
  buddy script copy 7 --name="Install Chrome" --to-tenant=sandbox
  buddy script migrate install-chrome.sh
  buddy script download "Restart Jamf Agent!"
  buddy script rename install-chrome.sh "Install Chrome"
  buddy script delete install-chrome.sh
`,
}
