package buddy

import "buddy/internal/docspec"

func init() {
	docspec.Register(BlueprintsTopic)
}

// BlueprintsTopic is the structured doc source for docs/blueprints.
// Edit here, then run `make gendocs`.
var BlueprintsTopic = docspec.Topic{
	ID:      `docs/blueprints`,
	Title:   `Blueprints`,
	Section: `Platform`,
	Keywords: []string{
		`blueprint`,
		`blueprints`,
		`deploy`,
		`undeploy`,
		`platform`,
	},
	Summary: "Platform API Gateway blueprints — CRUD, deploy/undeploy/status, components. Requires a **platform-capable** tenant + auth (`region`/`url` + `tenantId`).",
	Surfaces: []docspec.SurfaceRow{
		{Action: "List / load", BuddyScript: "`blueprints(search=…)` / `blueprint(id=…)`", CLI: "`buddy blueprint list` / `get`", Notes: ""},
		{Action: "Create / patch", BuddyScript: "`blueprint_create(body)` / `blueprint_set(id, body)`", CLI: "`buddy blueprint create` / `set`", Notes: "`blueprint_set` is merge-patch"},
		{Action: "Lifecycle", BuddyScript: "`blueprint_deploy` / undeploy / status …", CLI: "`buddy blueprint deploy|undeploy|status`", Notes: "See builtins catalog"},
	},
	Notes: []string{
		"Requires a **platform-capable** tenant + auth (`region`/`url` + `tenantId`).",
	},
	AgentNotes: []string{
		"Do not call blueprint builtins with an instance-only session — you need Platform surface credentials.",
		"Do not invent deploy payloads from `jamf_schema` when named builtins exist.",
		"Do not confuse with Classic/Pro policies or configuration profiles.",
	},
	Builtins: []docspec.Builtin{
			{
				Name: `blueprints`,
				Summary: `List blueprints`,
				Returns: `list`,
				Params: []docspec.Param{
					{Name: `filter`, Type: `string`, Required: false, Description: `filter`},
				},
			},
			{
				Name: `blueprint`,
				Summary: `Load one blueprint`,
				Returns: `dict`,
				Params: []docspec.Param{
					{Name: `id`, Type: `string`, Required: true, Description: `id`},
				},
			},
		},
	ExamplesBuddy: []string{
		`def main():
    rows = blueprints(search="example")
    return rows`,
	},
	Related: []docspec.RelatedLink{
		{Title: `Platform devices`, Ref: `/docs/docs/devices`},
		{Title: `device groups`, Ref: `/docs/docs/device-groups`},
		{Title: `API endpoints`, Ref: `/docs/docs/api-endpoints`},
		{Title: `MCP`, Ref: `/docs/docs/mcp`},
	},
	HelpExport: `blueprints`,
	BuiltinsHelp: `BLUEPRINT (Platform API Gateway credential required)
  blueprints(search?, page?, page_size?, tenant?, surface?, auth?)
      → value: list of {id, name, description?, created?, updated?}
      Params: search?: str; page?: int (omit = all); page_size?; tenant?/surface?/auth?.
  blueprint(id=, tenant?, surface?, auth?)
      → value: dict (full blueprint JSON)
      Params: id: str (blueprint UUID); tenant?/surface?/auth?.
  blueprint_create(body, tenant?, surface?, auth?) → value: dict
      Params: body: dict (create JSON); tenant?/surface?/auth?.
  blueprint_set(blueprint_id, body, tenant?, surface?, auth?) → None
      Params: blueprint_id: str; body: dict (merge-patch JSON); tenant?/surface?/auth?.
  blueprint_delete(blueprint_id, tenant?, surface?, auth?) → None
  blueprint_deploy(blueprint_id, tenant?, surface?, auth?) → None
  blueprint_undeploy(blueprint_id, tenant?, surface?, auth?) → None
  blueprint_status(blueprint_id, tenant?, surface?, auth?) → value: dict (status report)
  blueprint_components(page?, page_size?, tenant?, surface?, auth?) → value: list[dict]
  blueprint_component(id=, tenant?, surface?, auth?) → value: dict
      Params: id: str (component identifier); tenant?/surface?/auth?.

`,
	CLIHelp: `buddy blueprint - Manage Jamf Platform API Blueprints

Usage:
  buddy [global-flags] blueprint <subcommand> [args...]

Subcommands:
  list                          List blueprints (table)
  get <blueprint-id>            Fetch one blueprint (JSON)
  create [--file=<path>|-]      Create from JSON body (file or stdin)
  set <blueprint-id> [--file=|-]  PATCH blueprint (merge-patch+json)
  delete <blueprint-id>         Delete a blueprint
  deploy <blueprint-id>         Deploy a blueprint
  undeploy <blueprint-id>       Undeploy a blueprint
  status <blueprint-id>         Deployment status report (JSON)
  components [identifier]       List components, or get one by identifier

Flags for 'list':
  --search=<text>               Search name/description
  --sort=<field:dir>            Sort (default created:desc)

Notes:
  Requires a Platform API Gateway credential (region like us, or url like https://us.apigw.jamf.com
  and tenantId). See 'buddy cred help'.

Examples:
  buddy blueprint list
  buddy blueprint list --search=chrome
  buddy blueprint get e77c1408-10c8-4007-b177-abc9157fbcaa
  buddy blueprint create --file=bp.json
  buddy blueprint set <blueprint-id> --file=patch.json
  buddy blueprint deploy <blueprint-id>
  buddy blueprint status <blueprint-id>
  buddy blueprint components
`,
}
