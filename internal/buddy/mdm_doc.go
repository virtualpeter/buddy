package buddy

import "buddy/internal/docspec"

func init() {
	docspec.Register(MdmTopic)
}

// MdmTopic is the structured doc source for docs/mdm.
// Edit here, then run `make gendocs`.
var MdmTopic = docspec.Topic{
	ID:      `docs/mdm`,
	Title:   `MDM command history`,
	Section: `Resources`,
	Keywords: []string{
		`mdm`,
		`mdm_commands`,
		`command`,
		`commands`,
		`history`,
	},
	Summary: `Query Jamf Pro MDM **command history** with RSQL filters. Device **actions** (reboot, lock, erase) live on [computers](/docs/docs/computers) (Pro) or [devices](/docs/docs/devices) (Platform).`,
	Surfaces: []docspec.SurfaceRow{
		{Action: "List / find", BuddyScript: "`mdm_commands(filter=…)`", CLI: "`buddy mdm list --filter=…`", Notes: "Bare term → `status==\"term\"`, or full RSQL"},
	},
	Notes: []string{
		"Device **actions** (reboot, lock, erase) live on [computers](/docs/docs/computers) or Platform [devices](/docs/docs/devices).",
	},
	AgentNotes: []string{
		"Do not treat this as the way to **issue** MDM actions — use `computer_*` / Platform device action builtins.",
		"Do not invent date filters without RSQL range operators (see api-endpoints MDM + RSQL notes).",
	},
	Builtins: []docspec.Builtin{
			{
				Name: `mdm_commands`,
				Summary: `List MDM command history`,
				Returns: `list of dicts`,
				Params: []docspec.Param{
					{Name: `filter`, Type: `string`, Required: true, Description: `filter`},
				},
			},
		},
	ExamplesBuddy: []string{
		`def main():
    return mdm_commands(filter="Pending")`,
	},
	Related: []docspec.RelatedLink{
		{Title: `Computers`, Ref: `/docs/docs/computers`},
		{Title: `API endpoints`, Ref: `/docs/docs/api-endpoints`},
	},
	HelpExport: `mdm`,
	BuiltinsHelp: `MDM
  mdm_commands(filter, sort?, page?, page_size?, limit?, tenant?, surface?, auth?)
      → value: list of MDM command dicts
      Params: filter: str required — bare term → status=="term" (exact), or full RSQL;
        sort?: str; page?: int (omit = all); page_size?; limit?: int.

`,
	CLIHelp: `buddy mdm - Query MDM commands from Jamf Pro

Usage:
  buddy [global-flags] mdm <subcommand> [args...]

Subcommands:
  list   List MDM commands matching an RSQL filter

Flags for 'list':
  --filter=<term|rsql>  Bare term → status=="term" (exact); or full RSQL (required)
  --sort=<expr>         Sort expression (default: dateSent:asc)
  --count               Fetch all matching records and print only the total count (table suppressed)
  --page-size=<n>       Records per API request (default 200; pagination fetches all)
  --limit=<n>           Maximum total records to return (0 = no limit)

RSQL filterable fields:
  uuid, clientManagementId, command, status, clientType, dateSent,
  validAfter, dateCompleted, profileId, profileIdentifier, active

  Date fields require gt/lt/ge/le operators (not ==).
  Bare --filter=Pending is sugar for status=="Pending"; use RSQL for other fields.

Examples:
  buddy mdm list --filter=Pending
  buddy mdm list --filter="status==Pending"
  buddy mdm list --filter="clientManagementId==00000000-0000-4000-8000-000000000002"
  buddy mdm list --filter="status==Pending;clientType==COMPUTER" --sort="dateSent:desc"
  buddy mdm list --filter="dateCompleted=ge=2024-01-01T00:00:00Z;status==Acknowledged"
  buddy mdm list --filter=Pending --count
  buddy mdm list --filter=Pending --limit=50
`,
}
