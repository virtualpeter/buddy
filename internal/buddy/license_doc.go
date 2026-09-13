package buddy

import "buddy/internal/docspec"

func init() {
	docspec.Register(LicensesTopic)
}

// LicensesTopic is the structured doc source for docs/licenses.
// Edit here, then run `make gendocs`.
var LicensesTopic = docspec.Topic{
	ID:      `docs/licenses`,
	Title:   `Licenses`,
	Section: `Platform`,
	Keywords: []string{
		`license`,
		`licenses`,
		`licensing`,
		`account`,
		`organization`,
	},
	Summary: "Jamf Account licenses (organization workplace) — read-only list from `GET /licensing/v1/licenses`. Org is resolved from the access token; no `X-Environment-Id` / `X-Tenant-Id`.",
	Surfaces: []docspec.SurfaceRow{
		{Action: "List", BuddyScript: "`licenses()`", CLI: "`buddy license list`", Notes: "Not paginated; empty org is still 200"},
	},
	Notes: []string{
		"Organization-scoped Account API. Requires a **platform** auth on an **organization** workplace (no environment or product tenant id).",
		"Privilege: licensing on the org integration. The same GET is the `buddy auth test` probe on an organization workplace (403 → `live=token-ok`).",
		"Do not send `orgs.*.id` as a header. Environment / product-tenant Platform APIs (blueprints, devices, …) cannot run on an org workplace.",
	},
	AgentNotes: []string{
		"Do not call `licenses()` with an environment or product-tenant session — it will raise. Use `--context` / `tenant=` on the organization workplace.",
		"Do not invent SSO, Partners, Audit, or AI Governance calls from this page. Licensing is read-only; there is no `license_set`.",
		"Do not rewrite `/api/licensing/…` to `/pro/…`. It stays on the org gateway as `/licensing/v1/licenses`.",
	},
	Builtins: []docspec.Builtin{
		{
			Name:    `licenses`,
			Summary: `List Account licenses`,
			Returns: `list`,
			Params: []docspec.Param{
				{Name: `tenant`, Type: `string`, Required: false, Description: `organization workplace (dual-read of context)`},
				{Name: `context`, Type: `string`, Required: false, Description: `organization workplace`},
				{Name: `surface`, Type: `string`, Required: false, Description: `omit for process defaults`},
				{Name: `auth`, Type: `string`, Required: false, Description: `omit for process defaults`},
			},
		},
	},
	ExamplesBuddy: []string{
		`def main():
    return licenses()`,
	},
	Related: []docspec.RelatedLink{
		{Title: `Auth`, Ref: `/docs/docs/auth`},
		{Title: `API endpoints`, Ref: `/docs/docs/api-endpoints`},
		{Title: `Blueprints`, Ref: `/docs/docs/blueprints`},
	},
	HelpExport: `licenses`,
	BuiltinsHelp: `LICENSE (Account API; organization workplace)
  licenses(tenant?, context?, surface?, auth?)
      Does: List Jamf Account licenses for the organization on the access token.
      → value: list of dicts (productName, sku, licenseType, purchasedSeats, startDate, endDate, …)
      Params: tenant? / context?: organization workplace; surface? / auth?: omit for process defaults.
      Notes: Not paginated. Empty org returns []. Requires platform auth with no X-Environment-Id / X-Tenant-Id.

`,
	CLIHelp: `buddy license - Jamf Account licenses (organization workplace)

Usage:
  buddy [global-flags] license <subcommand> [args...]

Subcommands:
  list
      List licenses for the organization (from the access token)

Notes:
  Organization-scoped Account API. Requires a platform auth on an organization
  workplace (no X-Environment-Id / X-Tenant-Id). Privilege: licensing on the
  org integration. Empty org still returns 200 with no rows. The same GET is
  the 'buddy auth test' probe on an organization workplace.

Examples:
  buddy license list
  buddy --context default license list
`,
}
