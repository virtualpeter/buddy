package buddy

import "buddy/internal/docspec"

func init() {
	docspec.Register(MobileExtensionAttributesTopic)
}

// MobileExtensionAttributesTopic is the structured doc source for docs/mobile-extension-attributes.
var MobileExtensionAttributesTopic = docspec.Topic{
	ID:      `docs/mobile-extension-attributes`,
	Title:   `Mobile extension attributes`,
	Section: `Resources`,
	Keywords: []string{
		`mobile extension attribute`,
		`mobile_extension_attribute`,
		`mobile_extension_attributes`,
		`mobile-ea`,
	},
	Summary: `Jamf Pro mobile-device extension attributes — list/get/create/put/delete and copy/migrate. No SCRIPT input type (TEXT/POPUP/LDAP only). Computer EAs are under [extension attributes](/docs/docs/extension-attributes).`,
	Surfaces: []docspec.SurfaceRow{
		{Action: "List", BuddyScript: "`mobile_extension_attributes(filter?)`", CLI: "`buddy mobile-extension-attribute list`", Notes: "Summaries `{id, name, inputType, dataType, inventoryDisplayType}`"},
		{Action: "Load", BuddyScript: "`mobile_extension_attribute(id=…)` / `mobile_extension_attribute(name=…)`", CLI: "`buddy mobile-extension-attribute get`", Notes: "Full Jamf JSON"},
		{Action: "Create / update", BuddyScript: "`mobile_extension_attribute_create` / `mobile_extension_attribute_put`", CLI: "`create`", Notes: "TEXT/POPUP/LDAP only"},
		{Action: "Delete", BuddyScript: "`mobile_extension_attribute_delete`", CLI: "`delete`", Notes: "One ID or a list"},
		{Action: "Copy / migrate", BuddyScript: "`mobile_extension_attribute_copy` / `mobile_extension_attribute_migrate`", Notes: "Upsert by name"},
	},
	Notes: []string{
		"Mobile EAs have no SCRIPT, templates, or XML upload/download — use computer EAs for inventory scripts.",
	},
	AgentNotes: []string{
		"Do not invent SCRIPT for mobile EAs. Computer definitions are `extension_attribute*`.",
	},
	Builtins: []docspec.Builtin{
		{Name: `mobile_extension_attributes`, Summary: `List mobile EA summaries`, Returns: `list of {id,name,…}`},
		{Name: `mobile_extension_attribute`, Summary: `Load full mobile EA`, Returns: `dict`},
		{Name: `mobile_extension_attribute_copy`, Summary: `Copy/migrate mobile EA`, Returns: `dict`},
	},
	ExamplesBuddy: []string{
		`def main():
    return mobile_extension_attributes(filter="Department")`,
	},
	Related: []docspec.RelatedLink{
		{Title: `Extension attributes`, Ref: `/docs/docs/extension-attributes`},
		{Title: `Mobile devices`, Ref: `/docs/docs/mobile-devices`},
	},
	HelpExport: `mobile-extension-attributes`,
	BuiltinsHelp: `MOBILE EXTENSION ATTRIBUTE (Jamf Pro mobile-device extension attributes)
  mobile_extension_attributes(filter?, page?, page_size?, tenant?, surface?, auth?)
      Does: List mobile-device extension attributes.
      → value: list of {id, name, inputType, dataType, inventoryDisplayType}
      Params: filter?: bare term → name=="*term*", or full RSQL.
  mobile_extension_attribute(id=?) | mobile_extension_attribute(name=?, tenant?, surface?, auth?)
      Does: Load one mobile EA by id or exact name.
      → value: dict (full Jamf JSON)
  mobile_extension_attribute_create(name, input_type?, data_type?, popup_menu_choices?,
                inventory_display_type?, description?, ldap_attribute_mapping?,
                ldap_extension_attribute_allowed?, tenant?, surface?, auth?)
      Does: Create a mobile EA (POST /api/v1/mobile-device-extension-attributes).
      → value: dict
      Params: input_type?: TEXT|POPUP|DIRECTORY_SERVICE_ATTRIBUTE_MAPPING (default TEXT). No SCRIPT.
  mobile_extension_attribute_put(id, dict, tenant?, surface?, auth?)
      Does: PUT a full mobile EA JSON record (fetch-merge field map).
      → value: dict
  mobile_extension_attribute_delete(id=?) | mobile_extension_attribute_delete(ids=?, …)
      Does: Delete one or more mobile EAs.
      → None
  mobile_extension_attribute_clone(id, name, tenant?, surface?, auth?)
      Does: Same-session alias of mobile_extension_attribute_copy.
      → value: dict
  mobile_extension_attribute_copy(id, name?, upsert?=False,
              tenant?, surface?, auth?, to_tenant?, to_surface?, to_auth?)
      Does: Copy a mobile EA to another (or the same) session.
      → value: dict (+created when upsert)
  mobile_extension_attribute_migrate(...)
      Does: Alias of mobile_extension_attribute_copy with upsert=True.
      → value: dict

`,
	CLIHelp: `buddy mobile-extension-attribute - Manage Jamf Pro mobile-device extension attributes

Usage:
  buddy [global-flags] mobile-extension-attribute <subcommand> [args...]

Aliases: mobile-ea, mobile-eas, mobile-extension-attributes

Subcommands:
  list [--filter=<term|rsql>] [--json]                List mobile EAs
  get <ea-id>                                         Show full EA record
  get id <name>                                       Resolve name to ID
  create <name> [--input-type=TEXT|POPUP|DIRECTORY_SERVICE_ATTRIBUTE_MAPPING]
         [--data-type=STRING|INTEGER|DATE] [--display=<type>]
         [--description=<text>] [--popup=<a,b,c>] [--ldap=<mapping>]
  delete <ea-id> [ea-id...]                           Delete one or more EAs

Examples:
  buddy mobile-ea list --filter=Dept
  buddy mobile-ea create "Building" --input-type=POPUP --popup=A,B,C
  buddy mobile-ea delete 3
`,
}
