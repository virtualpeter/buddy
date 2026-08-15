package buddy

import "buddy/internal/docspec"

func init() {
	docspec.Register(ClientsTopic)
}

// ClientsTopic is the structured doc source for docs/api-clients.
// Edit here, then run `make gendocs`.
var ClientsTopic = docspec.Topic{
	ID:      `docs/api-clients`,
	Title:   `API clients`,
	Section: `Resources`,
	Keywords: []string{
		`client`,
		`clients`,
		`integration`,
		`integrations`,
		`oauth`,
		`api client`,
	},
	Summary: "Jamf Pro API integrations (OAuth clients) — list, get, create, put. Pair with [API roles](/docs/docs/api-roles) and `buddy setup store-oauth` for instance-surface oauth auths.",
	Surfaces: []docspec.SurfaceRow{
		{Action: "List / load", BuddyScript: "`clients(...)` / `client(id=…)`", CLI: "`buddy client list` / `get`", Notes: ""},
		{Action: "Create / update", BuddyScript: "`client_create` / `client_put`", CLI: "`buddy client create` / `update`", Notes: ""},
		{Action: "Store secret", BuddyScript: "", CLI: "`buddy setup store-oauth`", Notes: "Instance API client → buddy oauth auth (`surface=instance`)"},
	},
	Notes: []string{
		"Pair with [API roles](/docs/docs/api-roles). Platform gateway oauth cannot manage API roles/integrations.",
	},
	AgentNotes: []string{
		"Do not put production client secrets only in `config.yaml` when the platform secret store exists.",
		"Do not use Platform gateway oauth expecting Classic/Pro role management — use an instance-surface auth for that.",
	},
	Builtins: []docspec.Builtin{
			{
				Name: `clients`,
				Summary: `List API clients`,
				Returns: `list`,
				Params: []docspec.Param{
					{Name: `filter`, Type: `string`, Required: false, Description: `filter`},
				},
			},
			{
				Name: `client`,
				Summary: `Load one client`,
				Returns: `dict`,
				Params: []docspec.Param{
					{Name: `id`, Type: `string`, Required: true, Description: `id`},
				},
			},
		},
	ExamplesBuddy: []string{
		`def main():
    return clients(filter="buddy-cli")`,
	},
	Related: []docspec.RelatedLink{
		{Title: `API roles`, Ref: `/docs/docs/api-roles`},
		{Title: `Getting started`, Ref: `/docs/docs/index`},
		{Title: `API endpoints`, Ref: `/docs/docs/api-endpoints`},
	},
	HelpExport: `clients`,
	BuiltinsHelp: `CLIENT
  clients(filter?, page?, page_size?, tenant?, surface?, auth?)
      Does: List Jamf Pro API integrations (oauth clients).
      → value: list of dicts (id, displayName, …)
      Params: filter?: bare term → displayName=="*term*", or full RSQL. page?: int (omit = all);
        page_size?: int.
  client(id, tenant?, surface?, auth?)
      Does: Load one API integration JSON document.
      → value: dict (full client record)
      Params: id: str/int (integration ID).
  client_put(client_id, dict, tenant?, surface?, auth?)
      Does: PUT a full API integration JSON body (name and role scopes).
      → value: dict (updated client)
      Params: client_id: str/int; dict: full client body.
  client_create(display_name, scopes, tenant?, surface?, auth?)
      Does: Create an API integration with the given role scopes.
      → value: dict {id, client_id}
      Params: display_name: str; scopes: list of str (role display names or IDs).

`,
	CLIHelp: `buddy client - Manage Jamf Pro API integrations

Usage:
  buddy [global-flags] client <subcommand> [args...]

Subcommands:
  list [--filter=<term|rsql>]                           List API integrations (bare → displayName contains)
  get <client-id>                                   Show API integration details
  create <displayName> [scope1 scope2...]           Create integration with role scopes
  update <client-id> <displayName> [scope1 scope2...]  Update integration name and scopes
  delete <client-id>                                Delete API integration
  credentials <client-id>                           Generate new OAuth client credentials
  set token-lifetime <client-id> <seconds>          Set access token lifetime in seconds

Examples:
  buddy client list
  buddy client list --filter=buddy
  buddy client list --filter='displayName=="*buddy*"'
  buddy client get 3
  buddy client create "My App" "Read Policies"
  buddy client update 3 "My App" "Read Policies" "Read Scripts"
  buddy client delete 3
  buddy client credentials 3
  buddy client set token-lifetime 3 3600
`,
}
