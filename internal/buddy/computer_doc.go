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
		`computer_detail`,
		`inventory`,
		`section`,
		`lastContact`,
		`lastCheckIn`,
		`reportDate`,
		`reboot`,
		`lock`,
		`erase`,
		`mdm`,
	},
	Summary: `Jamf Pro computer inventory (v4) and MDM **actions** (reboot, lock, erase, …). For MDM **command history** queries, see [mdm](/docs/docs/mdm). Platform inventory is [devices](/docs/docs/devices) — different API surface.`,
	Surfaces: []docspec.SurfaceRow{
		{Action: "List / find", BuddyScript: "`computers(filter=…, sort?, section?, raw?, page?, page_size?)`", CLI: "`buddy computer list [--sort=] [--json]`", Notes: "v4 dates `lastContact` / `lastCheckIn` / `reportDate`; omit page to fetch all"},
		{Action: "Load", BuddyScript: "`computer(id=…, section?)`", CLI: "`buddy computer get [--section=…]`", Notes: "Passthrough JSON; default GENERAL; `section` → `?section=`"},
		{Action: "Load all sections", BuddyScript: "`computer_detail(id=…)`", CLI: "`buddy computer detail`", Notes: "`GET …/computers-inventory-detail/{id}`"},
		{Action: "MDM actions", BuddyScript: "`computer_reboot` / `computer_lock` / …", CLI: "`buddy computer reboot|lock|…`", Notes: "Named builtins and matching CLI verbs"},
	},
	Notes: []string{
		"MDM **command history** is [mdm](/docs/docs/mdm). Platform inventory is [devices](/docs/docs/devices).",
		"List RSQL dates are v4 `general.lastContact` (last MDM/binary/DDM contact), `general.lastCheckIn`, and `general.reportDate` (last inventory). Do not use v3 `lastContactTime` or Platform `lastCheckInTime` / `lastInventoryUpdateTime`.",
	},
	AgentNotes: []string{
		"Do not mix Pro `computer*` with Platform `device*` — different gateways and IDs.",
		"Do not use this page for RSQL history of MDM commands — that is [mdm](/docs/docs/mdm).",
		"Computer inventory is **v4** (`/api/v4/computers-inventory`). Do not use v3 `lastContactTime` or Platform `lastCheckInTime` / `lastInventoryUpdateTime` (see [api-endpoints](/docs/docs/api-endpoints)).",
		"Date filters: `general.lastContact` (last MDM/binary/DDM contact), `general.lastCheckIn`, `general.reportDate` (last inventory). Unquoted ISO-8601 UTC.",
		"`computers()` summary dicts include `lastContact`, `lastCheckIn`, `reportDate`, `lastEnrolledDate`, `platform` from GENERAL. `raw=True` is Jamf `results[]` passthrough (xor with `ids=True`). List `section=` adds HARDWARE/OS without N+1 gets.",
		"`computer(id)` returns full inventory JSON for requested sections (Jamf default GENERAL). Use `computer_detail(id)` for all sections.",
		"Prefer named `computer_*` builtins over raw MDM POSTs.",
	},
	Builtins: []docspec.Builtin{
		{
			Name:    `computers`,
			Summary: `List computers`,
			Returns: `list of summary dicts (or ids / raw results)`,
			Params: []docspec.Param{
				{Name: `filter`, Type: `string`, Required: false, Description: `bare term or v4 RSQL`},
				{Name: `ids`, Type: `bool`, Required: false, Description: `return list of int (xor with raw)`},
				{Name: `raw`, Type: `bool`, Required: false, Description: `Jamf results[] passthrough (xor with ids)`},
				{Name: `sort`, Type: `string`, Required: false, Description: `sort= (e.g. general.reportDate:desc)`},
				{Name: `section`, Type: `string|list`, Required: false, Description: `list GET section=; omit for GENERAL`},
			},
		},
		{
			Name:    `computer`,
			Summary: `Load inventory (passthrough JSON)`,
			Returns: `dict (Jamf inventory document)`,
			Params: []docspec.Param{
				{Name: `id`, Type: `string`, Required: true, Description: `computer id`},
				{Name: `section`, Type: `string|list`, Required: false, Description: `inventory section(s); omit for GENERAL`},
			},
		},
		{
			Name:    `computer_detail`,
			Summary: `Load all inventory sections`,
			Returns: `dict (Jamf inventory document)`,
			Params: []docspec.Param{
				{Name: `id`, Type: `string`, Required: true, Description: `computer id`},
			},
		},
		{
			Name:    `computer_reboot`,
			Summary: `Reboot computers`,
			Returns: `None`,
			Params: []docspec.Param{
				{Name: `ids`, Type: `list`, Required: true, Description: `ids`},
			},
		},
	},
	ExamplesBuddy: []string{
		`def main():
    general = computer(id="101")
    hw = computer(id="101", section=["GENERAL", "HARDWARE"])
    full = computer_detail(id="101")
    return {"name": general["general"]["name"], "sections": list(full.keys())}`,
		`def main():
    recent = computers(
        filter=rsql_ge("general.reportDate", rsql_days_ago(7)),
        sort="general.reportDate:desc",
    )
    seen = computers(
        filter=rsql_ge("general.lastContact", rsql_days_ago(1)),
        sort="general.lastContact:desc",
    )
    return {"inventory": len(recent), "contact": len(seen)}`,
	},
	Related: []docspec.RelatedLink{
		{Title: `MDM command history`, Ref: `/docs/docs/mdm`},
		{Title: `Static computer groups`, Ref: `/docs/docs/static-groups`},
		{Title: `Platform devices`, Ref: `/docs/docs/devices`},
	},
	HelpExport: `computers`,
	BuiltinsHelp: `COMPUTER
  computers(filter?, ids?=False, raw?=False, sort?, section?, page?, page_size?, tenant?, surface?, auth?)
      Does: List computers from Pro inventory v4.
      → value: list of summary dicts; list of int if ids=True; Jamf results[] dicts if raw=True
      Params: filter?: bare term → general.name=="*term*", or full RSQL (v4: general.lastContact, general.lastCheckIn, general.reportDate, …).
        ids?: bool (xor with raw). raw?: bool — unchanged Jamf results[] (xor with ids).
        sort?: string — sort= (e.g. general.lastContact:desc); omit = Jamf general.name:asc.
        section?: string or list → repeated section= on the list GET (omit = GENERAL).
        page?: int (omit = all); page_size?: int (default 200).
      Notes: Summary dicts include lastContact, lastCheckIn, reportDate, lastEnrolledDate, platform from GENERAL.
        reportDate is last inventory. Do not use v3 lastContactTime or Platform lastCheckInTime.
        ids and raw are mutually exclusive.
  computer(id, section?, tenant?, surface?, auth?)
      Does: Load one computer's inventory JSON (requested sections).
      → value: dict (GET /api/v4/computers-inventory/{id} JSON)
      Params: id: str; section?: string or list → ?section= (omit = Jamf GENERAL default).
  computer_detail(id, tenant?, surface?, auth?)
      Does: Load one computer with all inventory sections.
      → value: dict (GET /api/v4/computers-inventory-detail/{id} JSON)
      Params: id: str.
  computer_send_mdm_command(ids, command_type, command_data?, tenant?, surface?, auth?)
      Does: Queue a named MDM command on one or more computers.
      → None
      Params: ids: list of int/str; command_type: str (Jamf MDM command name);
        command_data?: dict (command payload when required).
  computer_reboot(ids, tenant?, surface?, auth?)
      Does: Queue RESTART_DEVICE on the given computers.
      → None
      Params: ids: list of int/str.
  computer_redeploy(ids, tenant?, surface?, auth?)
      Does: Trigger Jamf management framework redeploy on the given computers.
      → None
      Params: ids: list of int/str.
  computer_shutdown(ids, tenant?, surface?, auth?)
      Does: Queue SHUT_DOWN_DEVICE on the given computers.
      → None
      Params: ids: list of int/str.
  computer_lock(ids, message?, phone?, pin?, tenant?, surface?, auth?)
      Does: Queue DEVICE_LOCK (extra confirm) on the given computers.
      → None
      Params: ids: list of int/str; message?/phone?: str shown on lock screen;
        pin?: str (6-digit PIN when required).
  computer_logout(ids, tenant?, surface?, auth?)
      Does: Queue LOG_OUT_USER on the given computers.
      → None
      Params: ids: list of int/str.
  computer_erase(ids, pin?, obliteration?, tenant?, surface?, auth?)
      Does: Queue ERASE_DEVICE (type 'erase' to confirm) on the given computers.
      → None
      Params: ids: list of int/str; pin?: str; obliteration?: str (obliteration behavior).
  computer_device_info(ids, queries?, tenant?, surface?, auth?)
      Does: Queue DEVICE_INFORMATION MDM (results arrive later in command history).
      → None
      Params: ids: list of int/str; queries?: list of str (e.g. DeviceName).
  computer_device_location(ids, tenant?, surface?, auth?)
      Does: Queue DEVICE_LOCATION MDM (results arrive later in command history).
      → None
      Params: ids: list of int/str.
  computer_security_info(ids, tenant?, surface?, auth?)
      Does: Queue SECURITY_INFO MDM (results arrive later in command history).
      → None
      Params: ids: list of int/str.
  computer_installed_apps(ids, identifiers?, tenant?, surface?, auth?)
      Does: Queue INSTALLED_APPLICATION_LIST MDM (results arrive later in command history).
      → None
      Params: ids: list of int/str; identifiers?: list of str (bundle IDs to query).
  computer_managed_apps(ids, identifiers?, tenant?, surface?, auth?)
      Does: Queue MANAGED_APPLICATION_LIST MDM (results arrive later in command history).
      → None
      Params: ids: list of int/str; identifiers?: list of str (bundle IDs to query).
  computer_certificates(ids, tenant?, surface?, auth?)
      Does: Queue CERTIFICATE_LIST MDM (results arrive later in command history).
      → None
      Params: ids: list of int/str.

`,
	CLIHelp: `buddy computer - Manage computers and send MDM commands

Usage:
  buddy [global-flags] computer <subcommand> [args...]

Subcommands:
  list [args...]                                              List computers
  get <computer-id> [--section=<name>[,<name>...]]            Show inventory JSON (default GENERAL)
  detail <computer-id>                                        Show all inventory sections (inventory-detail)
  reboot <computer-id> [computer-id...]                       Send RESTART_DEVICE MDM command
  redeploy <computer-id> [computer-id...]                     Trigger Jamf management framework redeploy
  shutdown <computer-id> [computer-id...]                     Send SHUT_DOWN_DEVICE MDM command
  lock <computer-id> [computer-id...] [--message=...] [--phone=...] [--pin=...]  Send DEVICE_LOCK (extra confirm)
  logout <computer-id> [computer-id...]                       Send LOG_OUT_USER MDM command
  erase <computer-id> [computer-id...] [--pin=...] [--obliteration=...] Send ERASE_DEVICE (type 'erase' to confirm)
  device-info <computer-id> [computer-id...] [--queries=DeviceName,...]  Send DEVICE_INFORMATION (queued)
  device-location <computer-id> [computer-id...]              Send DEVICE_LOCATION (queued)
  security-info <computer-id> [computer-id...]                Send SECURITY_INFO (queued)
  installed-apps <computer-id> [computer-id...] [--identifiers=...]     Send INSTALLED_APPLICATION_LIST (queued)
  managed-apps <computer-id> [computer-id...] [--identifiers=...]       Send MANAGED_APPLICATION_LIST (queued)
  certificates <computer-id> [computer-id...]                 Send CERTIFICATE_LIST (queued)

Flags for 'list':
  --filter=<term|rsql>           Bare term → general.name contains; or full RSQL
  --sort=<field:dir>             Jamf sort= (e.g. general.lastContact:desc); omit for name
  --section=<name>[,<name>...]   Extra inventory sections on the list GET (omit = GENERAL)
  --json                         Print Jamf results[] as JSON
  --management-id=<uuid>         Look up by MDM management UUID

Flags for 'get':
  --section=<name>[,<name>...]   Inventory sections (e.g. GENERAL,HARDWARE); omit for GENERAL

Examples:
  buddy computer list
  buddy computer list --filter=Example
  buddy computer list --filter='general.name=="*Example*"'
  buddy computer list --filter='general.lastContact=ge=2026-01-01T00:00:00Z' --sort=general.lastContact:desc
  buddy computer list --json
  buddy computer list --management-id=00000000-0000-4000-8000-000000000003
  buddy computer get 101
  buddy computer get 101 --section=GENERAL,HARDWARE
  buddy computer detail 101
  buddy computer reboot 101 102
  buddy computer lock 101 --message="Lost device" --pin=123456
  buddy computer erase 101 --pin=123456
`,
}
