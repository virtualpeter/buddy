package buddy

import "buddy/internal/docspec"

func init() {
	docspec.Register(ComputersTopic)
}

// ComputersTopic is the structured doc source for docs/computers.
// Edit here, then run `make gendocs`.
var ComputersTopic = docspec.Topic{
	ID:      `docs/computers`,
	Title:   `Computers`,
	Section: `Resources`,
	Keywords: []string{
		`computer`,
		`computers`,
		`inventory`,
		`reboot`,
		`lock`,
		`erase`,
		`mdm`,
	},
	Summary: `Jamf Pro computer inventory (v3) and MDM **actions** (reboot, lock, erase, …). For MDM **command history** queries, see [mdm](/docs/docs/mdm). Platform inventory is [devices](/docs/docs/devices) — different API surface.`,
	Concepts: []string{
		"**List / find:** `computers(filter=…, page?, page_size?)` — omit `page` to fetch all pages. Buddy uses **inventory v3**.",
		"**Load one:** `computer(id=…)` (and detail-slice helpers where listed in builtins).",
		"**MDM actions:** `computer_reboot`, `computer_lock`, and related named builtins.",
	},
	AgentNotes: []string{
		"Do not mix Pro `computer*` with Platform `device*` — different gateways and IDs.",
		"Do not use this page for RSQL history of MDM commands — that is [mdm](/docs/docs/mdm).",
		"Do not invent computers-inventory **v4** paths from schema; buddy and production stay on v3 for now (see [api-endpoints](/docs/docs/api-endpoints)).",
		"Prefer named `computer_*` builtins over raw MDM POSTs.",
	},
	Builtins: []docspec.Builtin{
			{
				Name: `computers`,
				Summary: `List computers`,
				Returns: `list of summary dicts`,
				Params: []docspec.Param{
					{Name: `filter`, Type: `string`, Required: false, Description: `filter`},
				},
			},
			{
				Name: `computer`,
				Summary: `Load one computer`,
				Returns: `dict`,
				Params: []docspec.Param{
					{Name: `id`, Type: `string`, Required: true, Description: `id`},
				},
			},
			{
				Name: `computer_reboot`,
				Summary: `Reboot computers`,
				Returns: `None`,
				Params: []docspec.Param{
					{Name: `computer_ids`, Type: `list`, Required: true, Description: `computer_ids`},
				},
			},
		},
	ExamplesBuddy: []string{
		`def main():
    rows = computers(filter="MacBook", page_size=50)
    return rows`,
	},
	Related: []docspec.RelatedLink{
		{Title: `MDM command history`, Ref: `/docs/docs/mdm`},
		{Title: `Static computer groups`, Ref: `/docs/docs/static-groups`},
		{Title: `Platform devices`, Ref: `/docs/docs/devices`},
	},
	HelpExport: `computers`,
	BuiltinsHelp: `COMPUTER
  computers(filter?, ids?=False, page?, page_size?, tenant?, surface?, auth?)
      → value: list of summary dicts (or list of int if ids=True)
      Params: filter?: bare term → general.name=="*term*", or full RSQL.
  computer(id, tenant?, surface?, auth?)
      → value: dict (id, name, serialNumber, udid, managementId, ...)
  computer_send_mdm_command(computer_ids, command_type, command_data?, tenant?, surface?, auth?) → None
  computer_reboot(computer_ids, tenant?, surface?, auth?) → None
  computer_redeploy(computer_ids, tenant?, surface?, auth?) → None
  computer_shutdown(computer_ids, tenant?, surface?, auth?) → None
  computer_lock(computer_ids, message?, phone?, pin?, tenant?, surface?, auth?) → None
  computer_logout(computer_ids, tenant?, surface?, auth?) → None
  computer_erase(computer_ids, pin?, obliteration?, tenant?, surface?, auth?) → None
  computer_device_info(computer_ids, queries?, tenant?, surface?, auth?) → value: dict or list
  computer_device_location(computer_ids, tenant?, surface?, auth?) → value: dict or list
  computer_security_info(computer_ids, tenant?, surface?, auth?) → value: dict or list
  computer_installed_apps(computer_ids, identifiers?, tenant?, surface?, auth?) → value: dict or list
  computer_managed_apps(computer_ids, identifiers?, tenant?, surface?, auth?) → value: dict or list
  computer_certificates(computer_ids, tenant?, surface?, auth?) → value: dict or list

`,
	CLIHelp: `buddy computer - Manage computers and send MDM commands

Usage:
  buddy [global-flags] computer <subcommand> [args...]

Subcommands:
  list [--filter=<term|rsql>] [--management-id=<uuid>]    List computers (bare term → general.name contains; --management-id looks up by MDM management UUID)
  get <computer-id>                                   Show computer details
  reboot <computer-id> [computer-id...]               Send RESTART_DEVICE MDM command
  redeploy <computer-id> [computer-id...]            Trigger Jamf management framework redeploy
  shutdown <computer-id> [computer-id...]            Send SHUT_DOWN_DEVICE MDM command
  lock <computer-id> [computer-id...] [--message=...] [--phone=...] [--pin=...]  Send DEVICE_LOCK (extra confirm)
  logout <computer-id> [computer-id...]               Send LOG_OUT_USER MDM command
  erase <computer-id> [computer-id...] [--pin=...] [--obliteration=...] Send ERASE_DEVICE (type 'erase' to confirm)
  device-info <computer-id> [computer-id...] [--queries=DeviceName,...]  Send DEVICE_INFORMATION (queued)
  device-location <computer-id> [computer-id...]      Send DEVICE_LOCATION (queued)
  security-info <computer-id> [computer-id...]        Send SECURITY_INFO (queued)
  installed-apps <computer-id> [computer-id...] [--identifiers=...]     Send INSTALLED_APPLICATION_LIST (queued)
  managed-apps <computer-id> [computer-id...] [--identifiers=...]       Send MANAGED_APPLICATION_LIST (queued)
  certificates <computer-id> [computer-id...]         Send CERTIFICATE_LIST (queued)

Examples:
  buddy computer list
  buddy computer list --filter=Example
  buddy computer list --filter='general.name=="*Example*"'
  buddy computer list --management-id=00000000-0000-4000-8000-000000000003
  buddy computer get 101
  buddy computer reboot 101 102
  buddy computer lock 101 --message="Lost device" --pin=123456
  buddy computer erase 101 --pin=123456
`,
}
