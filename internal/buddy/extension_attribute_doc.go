package buddy

import "buddy/internal/docspec"

func init() {
	docspec.Register(ExtensionAttributesTopic)
}

// ExtensionAttributesTopic is the structured doc source for docs/extension-attributes.
// Edit here, then run `make gendocs`.
var ExtensionAttributesTopic = docspec.Topic{
	ID:      `docs/extension-attributes`,
	Title:   `Extension attributes`,
	Section: `Resources`,
	Keywords: []string{
		`extension attribute`,
		`extension_attribute`,
		`extension_attributes`,
		`ea`,
		`computer-extension-attributes`,
		`scriptContents`,
		`inventoryDisplayType`,
	},
	Summary: `Jamf Pro computer extension attributes — list summaries vs full body, create/update/delete, templates, XML import/export, and copy/migrate. Inventory values on a Mac are under [computers](/docs/docs/computers).`,
	Surfaces: []docspec.SurfaceRow{
		{Action: "List", BuddyScript: "`extension_attributes(filter?)`", CLI: "`buddy extension-attribute list`", Notes: "Summaries `{id, name, enabled, inputType, dataType, inventoryDisplayType}`; bodies stripped"},
		{Action: "Load", BuddyScript: "`extension_attribute(id=…)` / `extension_attribute(name=…)` / `extension_attribute_contents(id=…)`", CLI: "`buddy extension-attribute get` / `get contents`", Notes: "Full Jamf JSON; SCRIPT body only via contents"},
		{Action: "Create / update", BuddyScript: "`extension_attribute_create` / `extension_attribute_put` / `extension_attribute_update`", CLI: "`buddy extension-attribute create` / `set`", Notes: "SCRIPT/TEXT/POPUP/LDAP; PUT dict or replace SCRIPT from file"},
		{Action: "Delete", BuddyScript: "`extension_attribute_delete(id=…)` / `extension_attribute_delete(ids=…)`", CLI: "`buddy extension-attribute delete`", Notes: "One ID or a list (bulk uses delete-multiple)"},
		{Action: "Templates / XML", BuddyScript: "`extension_attribute_templates` / `extension_attribute_template` / `extension_attribute_create_from_template`", CLI: "`templates` / `upload` / `download`", Notes: "Jamf Nation templates; XML multipart upload + download"},
		{Action: "Deps / history", BuddyScript: "`extension_attribute_dependencies` / `extension_attribute_history`", CLI: "`deps` / `history`", Notes: "Smart group / advanced search dependents; audit history"},
		{Action: "Copy / migrate", BuddyScript: "`extension_attribute_copy` / `extension_attribute_migrate`", Notes: "Upsert by name; LDAP mapping passed through (dest must have LDAP)"},
		{Action: "Values on a Mac", BuddyScript: "`computer_extension_attributes` / `computer_set_extension_attribute`", CLI: "`buddy computer eas` / `set ea`", Notes: "Inventory section; TEXT/POPUP writable via PATCH"},
	},
	Notes: []string{
		"Bare `filter` terms become `name==\"*term*\"` on the server. List JSON never yields usable `scriptContents` — load with `extension_attribute(id=…)`.",
		"Migrate order: categories → scripts → **extension attributes** → smart groups (criteria often key off EA names) → policies.",
		"SCRIPT and LDAP inventory values are not writable; TEXT/POPUP values use `computer_set_extension_attribute`.",
	},
	AgentNotes: []string{
		"Do not expect `extension_attributes(...)` to return script bodies — use `extension_attribute(id=…)`.",
		"Definitions are Pro `/api/v1/computer-extension-attributes`. Values are v4 inventory (`section=EXTENSION_ATTRIBUTES` / PATCH detail). Do not invent Classic XML PUT for definitions.",
		"Check `extension_attribute_dependencies` before delete — smart groups and advanced searches may reference the EA.",
		"Mobile device EA definitions are a separate resource (`mobile_extension_attributes`); no SCRIPT input type.",
	},
	Builtins: []docspec.Builtin{
		{
			Name:    `extension_attributes`,
			Summary: `List computer EA summaries`,
			Returns: `list of {id,name,…}`,
			Params: []docspec.Param{
				{Name: `filter`, Type: `string`, Required: false, Description: `filter`},
			},
		},
		{
			Name:    `extension_attribute`,
			Summary: `Load full computer EA`,
			Returns: `dict`,
		},
		{
			Name:    `extension_attribute_copy`,
			Summary: `Copy/migrate computer EA`,
			Returns: `dict`,
			Params: []docspec.Param{
				{Name: `id`, Type: `string`, Required: true, Description: `id`},
			},
		},
		{
			Name:    `extension_attribute_delete`,
			Summary: `Delete one or more computer EAs`,
			Returns: `None`,
			Params: []docspec.Param{
				{Name: `id`, Type: `string|int`, Required: false, Description: `one EA ID (not with ids=)`},
				{Name: `ids`, Type: `string|int|list`, Required: false, Description: `EA ID or list of IDs (not with id=)`},
			},
		},
	},
	ExamplesBuddy: []string{
		`def main():
    rows = extension_attributes(filter="Chrome")
    if not rows:
        return []
    return extension_attribute(id=rows[0]["id"])`,
	},
	Related: []docspec.RelatedLink{
		{Title: `Computers`, Ref: `/docs/docs/computers`},
		{Title: `Smart computer groups`, Ref: `/docs/docs/smart-groups`},
		{Title: `Scripts`, Ref: `/docs/docs/scripts`},
		{Title: `API endpoints`, Ref: `/docs/docs/api-endpoints`},
	},
	HelpExport: `extension-attributes`,
	BuiltinsHelp: `EXTENSION ATTRIBUTE (Jamf Pro computer extension attributes)
  extension_attributes(filter?, page?, page_size?, tenant?, surface?, auth?)
      Does: List Jamf Pro computer extension attributes.
      → value: list of {id, name, enabled, inputType, dataType, inventoryDisplayType}
      Params: filter?: bare term → name=="*term*", or full RSQL. page?: int (omit = all);
        page_size?: int (default 200).
  extension_attribute(id=?) | extension_attribute(name=?, tenant?, surface?, auth?)
      Does: Load one computer EA by id or exact name (case-insensitive).
      → value: dict (full Jamf JSON: id, name, inputType, scriptContents?, …)
      Params: pass either id= or name=, not both.
  extension_attribute_contents(id, tenant?, surface?, auth?)
      Does: Return scriptContents only (fails if inputType is not SCRIPT).
      → value: str
      Params: id: str/int.
  extension_attribute_create(name, input_type?, data_type?, script_contents?, popup_menu_choices?,
                inventory_display_type?, description?, enabled?, ldap_attribute_mapping?,
                ldap_extension_attribute_allowed?, manage_existing_data?,
                tenant?, surface?, auth?)
      Does: Create a computer EA (POST /api/v1/computer-extension-attributes).
      → value: dict (created EA; same shape as extension_attribute(id=…))
      Params: name: str. input_type?: SCRIPT|TEXT|POPUP|DIRECTORY_SERVICE_ATTRIBUTE_MAPPING (default TEXT).
        data_type?: STRING|INTEGER|DATE (default STRING).
        script_contents?: str (SCRIPT). popup_menu_choices?: list[str] (POPUP).
        inventory_display_type?: GENERAL|HARDWARE|OPERATING_SYSTEM|USER_AND_LOCATION|PURCHASING|EXTENSION_ATTRIBUTES
        (default EXTENSION_ATTRIBUTES). description?: str. enabled?: bool (SCRIPT; default true).
        ldap_attribute_mapping?: str. ldap_extension_attribute_allowed?: bool.
        manage_existing_data?: RETAIN|DELETE. tenant?/surface?/auth?: omit for process defaults.
  extension_attribute_update(id, file, tenant?, surface?, auth?)
      Does: Replace a SCRIPT EA's scriptContents from a local file.
      → None
      Params: id: str/int; file: str (filesystem path).
  extension_attribute_put(id, dict, tenant?, surface?, auth?)
      Does: PUT a full computer EA JSON record (fetch-merge field map).
      → value: dict (updated EA)
      Params: id: str/int; dict: fields to merge onto current record.
  extension_attribute_delete(id=?) | extension_attribute_delete(ids=?, tenant?, surface?, auth?)
      Does: Delete one or more computer EAs.
      → None
      Params: pass either id= or ids= (one ID or list), not both.
  extension_attribute_templates(filter?, page?, page_size?, tenant?, surface?, auth?)
      Does: List Jamf Nation computer EA templates.
      → value: list of {templateId, templateName, templateCategoryName}
  extension_attribute_template(id, tenant?, surface?, auth?)
      Does: Load one template (full EA-shaped body).
      → value: dict
  extension_attribute_create_from_template(id, name?, tenant?, surface?, auth?)
      Does: POST a template as a new computer EA (optional rename).
      → value: dict (created EA)
  extension_attribute_history(id, tenant?, surface?, auth?)
      Does: Fetch recent audit history for a computer EA.
      → value: list of {id, username, date, note, details}
  extension_attribute_dependencies(id, tenant?, surface?, auth?)
      Does: List smart group / advanced search objects that depend on this EA.
      → value: list of {id, objectId, nameLocalization, identifiableName, hyperlink}
  extension_attribute_clone(id, name, tenant?, surface?, auth?)
      Does: Clone a computer EA in the same session (alias of extension_attribute_copy without to_*).
      → value: dict (created EA)
  extension_attribute_copy(id, name?, upsert?=False,
              tenant?, surface?, auth?, to_tenant?, to_surface?, to_auth?)
      Does: Copy a computer EA to another (or the same) session.
      → value: dict (dest EA; +created when upsert)
      Params: id: source. name?: dest name (required unless upsert). upsert?: bool (by exact name).
        Source = tenant?/auth?; dest = to_* (omit = same).
      Notes: LDAP mapping is passed through; Jamf returns 400 if destination has no LDAP.
  extension_attribute_migrate(...)
      Does: Alias of extension_attribute_copy with upsert=True.
      → value: dict (dest EA + created bool)

`,
	CLIHelp: `buddy extension-attribute - Manage Jamf Pro computer extension attributes

Usage:
  buddy [global-flags] extension-attribute <subcommand> [args...]

Aliases: ea, eas, extension-attributes

Subcommands:
  list [--filter=<term|rsql>] [--json]                List EAs (bare term → name contains; or RSQL)
  get <ea-id>                                         Show full EA record (includes scriptContents when SCRIPT)
  get contents <ea-id>                                Output SCRIPT body only (pipeable)
  get id <name>                                       Resolve EA name to numeric ID
  history <ea-id>                                     Show recent change history (last 20 entries)
  deps <ea-id>                                        Show smart group / advanced search dependents
  templates [--filter=<term|rsql>] [--json]           List Jamf Nation templates
  create <name> [--input-type=SCRIPT|TEXT|POPUP|DIRECTORY_SERVICE_ATTRIBUTE_MAPPING]
         [--data-type=STRING|INTEGER|DATE] [--file=<path>] [--display=<type>]
         [--description=<text>] [--popup=<a,b,c>] [--ldap=<mapping>] [--enabled=true|false]
  upload <file.xml>                                   Upload Jamf Nation XML (multipart)
  download <ea-id> [outfile]                          Save EA XML export (default: <name>.xml)
  delete <ea-id> [ea-id...]                           Delete one or more EAs
  set name        <ea-id> <value>                     Rename EA
  set enabled     <ea-id> true|false                  Enable/disable (SCRIPT)
  set description <ea-id> <value>                     Set description
  set contents    <ea-id> <value>                     Set SCRIPT body (literal string)
  set display     <ea-id> <type>                      Set inventoryDisplayType

Use 'buddy extension-attribute get help' or 'buddy extension-attribute set help' for more detail.

Examples:
  buddy ea list --filter=Chrome
  buddy ea get 7
  buddy ea create "OS Check" --input-type=SCRIPT --file=os_check.sh
  buddy ea upload nation_export.xml
  buddy ea download 7
  buddy ea delete 7 8
`,
}
