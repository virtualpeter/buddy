package buddy

import "buddy/internal/docspec"

func init() {
	docspec.Register(MacappsTopic)
}

// MacappsTopic is the structured doc source for docs/macapps.
// Edit here, then run `make gendocs`.
var MacappsTopic = docspec.Topic{
	ID:      `docs/macapps`,
	Title:   `Mac App Store apps`,
	Section: `Resources`,
	Keywords: []string{
		`macapp`,
		`macapps`,
		`macapplications`,
		`mac-app-store`,
		`mac app store`,
		`app store`,
	},
	Summary: "Jamf **Mac App Store Apps** objects live on Classic `/JSSResource/macapplications`. There is no Pro JSON catalog CRUD for the same resource. Classic privileges on an instance-surface auth are required.\n\nBare `buddy app` / `apps(...)` manage **policy apps** (Self Service + packages) — see [Policy apps](/docs/docs/apps). Those names are a different resource family from Mac App Store records.",
	Surfaces: []docspec.SurfaceRow{
		{Action: "List", BuddyScript: "`macapps(filter?)`", CLI: "`buddy macapp list`", Notes: "Client-side substring; Classic list has no RSQL"},
		{Action: "Load", BuddyScript: "`macapp(id=…)` / `macapp(name=…)`", CLI: "`buddy macapp get`", Notes: "Full Classic record"},
		{Action: "Create", BuddyScript: "`macapp_create(...)`", CLI: "`buddy macapp create`", Notes: "Requires name, version, bundle_id, url"},
		{Action: "Delete", BuddyScript: "`macapp_delete(macapp_id)`", CLI: "`buddy macapp delete`", Notes: ""},
	},
	Notes: []string{
		"Classic `/JSSResource/macapplications` only — no Pro JSON catalog CRUD. Instance-surface auth with Classic privileges is required.",
	},
	AgentNotes: []string{
		"Do not invent Pro “Mac App Store” or “Jamf Apps” list/create paths — they are not exposed for this object.",
		"Do not confuse this with VPP locations/content (`/api/v1/volume-purchasing-locations`) or `jamf-package` (Protect/Connect only).",
		"Do not use `app` / `apps` for MAS records — those builtins are policy apps; see [Policy apps](/docs/docs/apps).",
		"Prefer an instance-surface auth (Classic privileges).",
	},
	Builtins: []docspec.Builtin{
			{
				Name: `macapps`,
				Summary: `List Mac App Store apps`,
				Returns: `list of {id,name,version,bundle_id}`,
				Params: []docspec.Param{
					{Name: `filter`, Type: `string`, Required: false, Description: `filter`},
				},
			},
			{
				Name: `macapp`,
				Summary: `Load one MAS app`,
				Returns: `dict`,
			},
			{
				Name: `macapp_create`,
				Summary: `Create MAS app`,
				Returns: `dict {id,name}`,
				Params: []docspec.Param{
					{Name: `name`, Type: `string`, Required: true, Description: `name`},
					{Name: `version`, Type: `string`, Required: true, Description: `version`},
					{Name: `bundle_id`, Type: `string`, Required: true, Description: `bundle_id`},
					{Name: `url`, Type: `string`, Required: true, Description: `url`},
				},
			},
			{
				Name: `macapp_delete`,
				Summary: `Delete MAS app`,
				Returns: `None`,
				Params: []docspec.Param{
					{Name: `macapp_id`, Type: `string`, Required: true, Description: `macapp_id`},
				},
			},
		},
	ExamplesBuddy: []string{
		`def main():
    rows = macapps(filter="TextWrangler")
    if not rows:
        return macapp_create(
            name="TextWrangler.app",
            version="5.5.2",
            bundle_id="com.barebones.textwrangler",
            url="https://itunes.apple.com/app/id404010395",
            free=True,
        )
    return macapp(id=rows[0]["id"])`,
	},
	Related: []docspec.RelatedLink{
		{Title: `Categories`, Ref: `/docs/docs/categories`},
		{Title: `API endpoints`, Ref: `/docs/docs/api-endpoints`},
	},
	HelpExport: `macapps`,
	BuiltinsHelp: `MAC APP STORE (Classic macapplications)
  macapps(filter?, tenant?, surface?, auth?)
      Does: List Classic Mac App Store app records.
      → value: list of {id, name, version, bundle_id}
      Params: filter?: str (client-side substring on name/version/bundle_id); Classic list has no RSQL.
  macapp(id=?) | macapp(name=?, tenant?, surface?, auth?)
      Does: Load one Mac App Store app by id or exact name.
      → value: dict (full resource: general/scope/self_service plus flat fields)
      Params: Pass either id or name, not both.
  macapp_create(name, version, bundle_id, url, free?=False, category?, tenant?, surface?, auth?)
      Does: Create a Classic Mac App Store app record.
      → value: dict {id, name}
      Params: name/version/bundle_id/url: str (required); free?: bool; category?: str (exact category name).
  macapp_delete(macapp_id, tenant?, surface?, auth?)
      Does: Delete a Classic Mac App Store app record.
      → None
      Params: macapp_id: int or str.

`,
	CLIHelp: `buddy macapp - Manage Classic Mac App Store apps

Usage:
  buddy [global-flags] macapp <subcommand> [args...]

Subcommands:
  list [--filter=<keyword>]           List Mac App Store apps (id, name, version, bundle id)
  get <macapp-id>                     Show mac app details (JSON)
  create --name= --version= --bundle-id= --url= [--free] [--category=<name>]
                                      Create a Mac App Store app record
  delete <macapp-id> [macapp-id...]   Delete one or more Mac App Store apps

Flags for 'list':
  --filter=<keyword>   Client-side substring on name, version, or bundle_id (Classic list has no RSQL)

Flags for 'create':
  --name=<name>             Display name (required)
  --version=<version>       Version string (required)
  --bundle-id=<bundle-id>   Bundle identifier (required)
  --url=<url>               App Store URL (required)
  --free                    Mark as free App Store app
  --category=<name>         Category name (exact match)

Notes:
  Uses Classic /JSSResource/macapplications (instance surface). There is no Pro MAS catalog CRUD.
  For policy apps (package + Self Service), use 'buddy app'.

Examples:
  buddy macapp list
  buddy macapp list --filter=chrome
  buddy macapp get 42
  buddy macapp create --name="TextWrangler.app" --version=5.5.2 --bundle-id=com.barebones.textwrangler --url=https://itunes.apple.com/app/id404010395 --free
  buddy macapp delete 42
`,
}
