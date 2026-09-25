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
		`computer_history`,
		`computer_policy_logs`,
		`computer_extension_attributes`,
		`policy logs`,
	},
	Summary: `Jamf Pro computer inventory (v4) and MDM **actions** (reboot, lock, erase, …). Classic **policy execution logs** are ` + "`computer_history`" + ` / ` + "`computer_policy_logs`" + `. For MDM **command history** queries, see [mdm](/docs/docs/mdm). Platform inventory is [devices](/docs/docs/devices) — different API surface.`,
	Surfaces: []docspec.SurfaceRow{
		{Action: "List / find", BuddyScript: "`computers(filter=…, sort?, section?, raw?, page?, page_size?)`", CLI: "`buddy computer list [--sort=] [--json]`", Notes: "v4 dates `lastContact` / `lastCheckIn` / `reportDate`; omit page to fetch all"},
		{Action: "Load", BuddyScript: "`computer(id=…, section?)`", CLI: "`buddy computer get [--section=…]`", Notes: "Passthrough JSON; default GENERAL; `section` → `?section=`"},
		{Action: "Load all sections", BuddyScript: "`computer_detail(id=…)`", CLI: "`buddy computer detail`", Notes: "`GET …/computers-inventory-detail/{id}`"},
		{Action: "EA values", BuddyScript: "`computer_extension_attributes` / `computer_set_extension_attribute`", CLI: "`buddy computer eas` / `set ea`", Notes: "`section=EXTENSION_ATTRIBUTES`; TEXT/POPUP writable via PATCH"},
		{Action: "Policy logs", BuddyScript: "`computer_history` / `computer_policy_logs` / `policy_logs`", CLI: "`buddy computer history` / `logs`", Notes: "Classic computerhistory per Mac; policy_logs needs a bounded computer list. A row has no id and no body; policy_id is the policy; --json is the full Classic record"},
		{Action: "MDM actions", BuddyScript: "`computer_reboot` / `computer_lock` / …", CLI: "`buddy computer reboot|lock|…`", Notes: "Named builtins and matching CLI verbs"},
	},
	Notes: []string{
		"Classic **computer history** is policy/event execution logs (`GET /JSSResource/computerhistory`), not MDM command history and not package/script audit `history`. MDM **command history** is [mdm](/docs/docs/mdm). Platform inventory is [devices](/docs/docs/devices).",
		"List RSQL dates are v4 `general.lastContact` (last MDM/binary/DDM contact), `general.lastCheckIn`, and `general.reportDate` (last inventory). v3 `lastContactTime` and Platform `lastCheckInTime` / `lastInventoryUpdateTime` are not valid on this API.",
	},
	AgentNotes: []string{
		"Do not mix Pro `computer*` with Platform `device*` — different gateways and IDs.",
		"Do not use this page for RSQL history of MDM commands — that is [mdm](/docs/docs/mdm).",
		"Policy execution logs are Classic `computer_history` / `computer_policy_logs` (or `policy_logs(id, computer_id=… | computer_ids=… | limit=…)`). Jamf has no policy-centric logs collection. A policy log row has no id and no body; `policy_id` is the policy; `--json` is the full Classic record (status, user, completed time). Do not invent `jamf_get` or `GET /api/v1/policies` logs — that path is a list index only.",
		"Computer inventory is **v4** (`/api/v4/computers-inventory`). Do not use v3 `lastContactTime` or Platform `lastCheckInTime` / `lastInventoryUpdateTime` (see [api-endpoints](/docs/docs/api-endpoints)).",
		"Date filters: `general.lastContact` (last MDM/binary/DDM contact), `general.lastCheckIn`, `general.reportDate` (last inventory). Unquoted ISO-8601 UTC.",
		"`computers()` summary dicts include `lastContact`, `lastCheckIn`, `reportDate`, `lastEnrolledDate`, `platform` from GENERAL. `raw=True` is Jamf `results[]` passthrough (xor with `ids=True`). List `section=` adds HARDWARE/OS without N+1 gets.",
		"`computer(id)` returns full inventory JSON for requested sections (Jamf default GENERAL). Use `computer_detail(id)` for all sections. Known sections include GENERAL, HARDWARE, OPERATING_SYSTEM, USER_AND_LOCATION, PURCHASING, EXTENSION_ATTRIBUTES.",
		"Prefer named `computer_*` builtins over raw MDM POSTs.",
		"EA inventory values: `computer_extension_attributes(id)` / `computer_set_extension_attribute` (TEXT/POPUP only). Definitions are [extension attributes](/docs/docs/extension-attributes).",
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
			Name:    `computer_history`,
			Summary: `Load Classic computer history`,
			Returns: `dict (flattened history)`,
			Params: []docspec.Param{
				{Name: `id`, Type: `string`, Required: false, Description: `computer id (xor name/serial/udid)`},
			},
		},
		{
			Name:    `computer_policy_logs`,
			Summary: `List flattened policy logs for one computer`,
			Returns: `list of dict`,
			Params: []docspec.Param{
				{Name: `id`, Type: `string`, Required: false, Description: `computer id (xor name/serial/udid)`},
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
		`def main():
    logs = computer_policy_logs(id="101", policy_id=42)
    return {"count": len(logs), "last": logs[0] if logs else None}`,
	},
	Related: []docspec.RelatedLink{
		{Title: `Policies`, Ref: `/docs/docs/policies`},
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
        reportDate is last inventory. v3 lastContactTime and Platform lastCheckInTime are not valid on this API.
        ids and raw are mutually exclusive.
  computer(id=, section?, tenant?, surface?, auth?)
      Does: Load one computer's inventory JSON (requested sections).
      → value: dict (GET /api/v4/computers-inventory/{id} JSON)
      Params: id: str; section?: string or list → ?section= (omit = Jamf GENERAL default).
  computer_detail(id=, tenant?, surface?, auth?)
      Does: Load one computer with all inventory sections.
      → value: dict (GET /api/v4/computers-inventory-detail/{id} JSON)
      Params: id: str.
  computer_extension_attributes(id, filter?, tenant?, surface?, auth?)
      Does: List extension attribute values on a computer (section=EXTENSION_ATTRIBUTES).
      → value: list of {id, name, inputType, dataType, values}
      Params: id: computer id. filter?: case-insensitive substring on name or definition id.
      Notes: Definitions are extension_attribute*; SCRIPT/LDAP values are read-only.
  computer_set_extension_attribute(id, ea_id?, ea_name?, values, tenant?, surface?, auth?)
      Does: Set TEXT/POPUP EA values via PATCH /api/v4/computers-inventory-detail/{id}.
      → None
      Params: id: computer id. pass either ea_id= (definitionId) or ea_name= (exact).
        values: str or list[str]. Rejects SCRIPT/LDAP.
  computer_history(id=?, name=?, serial=?, udid=?, subset?, tenant?, surface?, auth?)
      Does: Load Classic computer history (policy/event execution logs, not MDM or audit history).
      → value: dict (general plus flattened policy_logs and other present subsets)
      Params: pass exactly one of id= / name= / serial= / udid=.
        subset?: str or list — Classic subset names (PolicyLogs, General, Commands, …);
        omit = full document (can be large). tenant?/surface?/auth?: session overrides.
      Notes: GET /JSSResource/computerhistory. Flattened policy_log wrappers.
        A policy log row has no id and no body. policy_id is the policy.
        --json is the full Classic record (status, user, completed time).
        Not GET /api/v1/policies (list index only). Privilege: Read Computer History.
  computer_policy_logs(id=?, name=?, serial=?, udid=?, policy_id?, status?, since?, until?, tenant?, surface?, auth?)
      Does: Fetch Classic PolicyLogs for one computer and return flattened rows.
      → value: list of {policy_id, policy_name, username, date_time, date_completed, date_completed_epoch, date_completed_utc, status, computer_id, …}
      Params: pass exactly one of id= / name= / serial= / udid=.
        policy_id?: int/str (client-side filter). status?: str (case-insensitive exact; omit or None).
        since?/until?: ISO or epoch on date_completed_epoch. tenant?/surface?/auth?: session overrides.
      Notes: Always subset/PolicyLogs. date_completed* aliases date_time*. No fleet fan-out (see policy_logs).
        A policy log row has no id and no body. policy_id is the policy.
        --json is the full Classic record (status, user, completed time).
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
  eas <computer-id> [--filter=<term>] [--json]                List extension attribute values on a computer
  set ea <computer-id> <ea-id|name> <value>                    Set TEXT/POPUP EA value (PATCH inventory-detail)
  history <computer-id> [--subset=PolicyLogs] [--json]        Classic computer history (default PolicyLogs table)
  logs <computer-id> [--policy=<policy-id>] [--status=] [--since=] [--until=] [--json]  Flattened policy execution logs
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

Flags for 'history':
  --subset=<name>[&<name>...]    Classic subset (default PolicyLogs); empty --subset= is full document
  --json                         Print parsed history dict as JSON

Flags for 'logs':
  --policy=<policy-id>           Client-side filter by policy id
  --status=<status>              Client-side filter (case-insensitive exact, e.g. Completed)
  --since=<iso-or-epoch>         Keep rows with date_completed_epoch >= value
  --until=<iso-or-epoch>         Keep rows with date_completed_epoch <= value
  --json                         Print flattened logs as JSON

Notes:
  history and logs print Classic policy log rows. A row has no id and no body.
  policy_id is the policy. --json is the full Classic record (status, user, completed time).

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
  buddy computer eas 101
  buddy computer set ea 101 "Asset Tag" "LAB-42"
  buddy computer history 101
  buddy computer history 101 --json
  buddy computer logs 101 --policy=42
  buddy computer reboot 101 102
  buddy computer lock 101 --message="Lost device" --pin=123456
  buddy computer erase 101 --pin=123456
`,
}
