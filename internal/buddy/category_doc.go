package buddy

import "buddy/internal/docspec"

func init() {
	docspec.Register(CategoriesTopic)
}

// CategoriesTopic is the structured doc source for docs/categories.
// Edit here, then run `make gendocs`.
var CategoriesTopic = docspec.Topic{
	ID:      `docs/categories`,
	Title:   `Categories`,
	Section: `Resources`,
	Keywords: []string{
		`category`,
		`categories`,
		`category_copy`,
		`category_migrate`,
	},
	Summary: `Jamf Pro categories. Used heavily when copying scripts/policies across tenants.`,
	Surfaces: []docspec.SurfaceRow{
		{Action: "List / load", BuddyScript: "`categories(...)` / `category(id=…)`", CLI: "`buddy category list` / `get`", Notes: "Or by name where supported"},
		{Action: "Copy / migrate", BuddyScript: "`category_copy` / `category_migrate`", Notes: "Pro `{name, priority}`; upsert by exact name"},
	},
	Notes: []string{
		"Typical migrate order: categories → [scripts](/docs/docs/scripts) → [policies](/docs/docs/policies) / [profiles](/docs/docs/profiles). Missing categories on policy copy become **uncategorized**.",
	},
	AgentNotes: []string{
		"Create/copy categories before migrating policies that should keep category assignment.",
		"Do not invent category POST/PUT from schema when `category_copy` exists.",
		"Do not skip category copy when migrating policies that should keep category assignment.",
	},
	Builtins: []docspec.Builtin{
			{
				Name: `categories`,
				Summary: `List categories`,
				Returns: `list of summary dicts`,
			},
			{
				Name: `category`,
				Summary: `Load one category`,
				Returns: `dict`,
				Params: []docspec.Param{
					{Name: `id`, Type: `string`, Required: true, Description: `id`},
				},
			},
			{
				Name: `category_copy`,
				Summary: `Copy/migrate category`,
				Returns: `dict`,
				Params: []docspec.Param{
					{Name: `category_id`, Type: `string`, Required: true, Description: `category_id`},
				},
			},
		},
	ExamplesBuddy: []string{
		`def main():
    return category_copy(category_id="5", upsert=True, to_tenant="sandbox")`,
	},
	Related: []docspec.RelatedLink{
		{Title: `Scripts`, Ref: `/docs/docs/scripts`},
		{Title: `Policies`, Ref: `/docs/docs/policies`},
		{Title: `Profiles`, Ref: `/docs/docs/profiles`},
		{Title: `API endpoints`, Ref: `/docs/docs/api-endpoints`},
	},
	HelpExport: `categories`,
	BuiltinsHelp: `CATEGORY (Jamf Pro categories)
  categories(filter?, page?, page_size?, tenant?, surface?, auth?)
      → value: list of {id, name, priority}
      Params: filter?: bare term → name=="*term*", or full RSQL.
  category(id=?) | category(name=?, tenant?, surface?, auth?) → value: dict {id, name, priority}
  category_clone(category_id, name, priority?, tenant?, surface?, auth?) → value: dict {id, name, priority?}
      Params: Alias of category_copy without to_*.
  category_copy(category_id, name?, priority?, upsert?=False,
                tenant?, surface?, auth?, to_tenant?, to_surface?, to_auth?) → value: dict …
  category_migrate(...) → same as category_copy(..., upsert=True)

`,
	CLIHelp: `buddy category - Manage Jamf Pro categories

Usage:
  buddy [global-flags] category <subcommand> [args...]

Subcommands:
  list [--filter=<term|rsql>]              List categories (id, priority, name)
  get <category-id>                   Show category details (JSON)
  create <name> [--priority=<1-20>]   Create a category
  delete <category-id>                Delete a category

Flags for 'list':
  --filter=<term|rsql>   Bare term → name=="*term*"; or full RSQL (e.g. 'name=="*Apps*"')

Flags for 'create':
  --priority=<1-20>   Category priority (default 9)

Examples:
  buddy category list
  buddy category list --filter=apps
  buddy category get 3
  buddy category create "Mac Apps" --priority=9
  buddy category delete 3
`,
}
