package buddy

import "buddy/internal/docspec"

func init() {
	docspec.Register(MobileDevicesTopic)
}

// MobileDevicesTopic is the structured doc source for docs/mobile-devices.
// Edit here, then run `make gendocs`.
var MobileDevicesTopic = docspec.Topic{
	ID:      `docs/mobile-devices`,
	Title:   `Mobile devices`,
	Section: `Resources`,
	Keywords: []string{
		`mobile`,
		`mobile device`,
		`mobile_devices`,
		`mobile_device`,
		`ios`,
		`ipad`,
		`iphone`,
		`mobile_device_lock`,
		`mobile_device_erase`,
	},
	Summary: "Jamf Pro mobile-device inventory (iOS/iPadOS/tvOS) via `GET /api/v2/mobile-devices`, plus MDM actions (update inventory, restart, lock, erase) sent as `MOBILE_DEVICE` client-type commands. This is **not** Platform [devices](/docs/docs/devices) and not Pro [computers](/docs/docs/computers).",
	Surfaces: []docspec.SurfaceRow{
		{Action: "List / load", BuddyScript: "`mobile_devices(...)` / `mobile_device(id=… | serial=… | name=… | udid=…)`", CLI: "`buddy mobile-device list` / `get`", Notes: "filter is client-side"},
		{Action: "MDM actions", BuddyScript: "`mobile_device_update_inventory` / `_restart` / `_lock` / `_erase`", CLI: "`buddy mobile-device update-inventory|restart|lock|erase`", Notes: "Sends v2 MDM commands"},
		{Action: "Raw MDM command", BuddyScript: "`mobile_device_send_mdm_command(ids, command_type, command_data?)`", CLI: "—", Notes: "Any commandType"},
	},
	Notes: []string{
		"Pro instance API (`/api/v2/mobile-devices`) — different IDs and gateway from Platform [devices](/docs/docs/devices).",
		"`filter` on list is a client-side substring over id/name/serial/udid/username/model/type (the endpoint has no server-side filter).",
		"MDM actions resolve each device's `managementId` from inventory, then POST `/api/v2/mdm/commands` with `clientType=MOBILE_DEVICE`.",
	},
	AgentNotes: []string{
		"Do not use `computer*` or Platform `device*` builtins for Jamf Pro mobile devices.",
		"`mobile_device_erase` is destructive; it prompts unless -y / SkipConfirm.",
		"Prefer `mobile_device(serial=…)` over listing + client-side matching yourself.",
	},
	Builtins: []docspec.Builtin{
		{
			Name:    `mobile_devices`,
			Summary: `List mobile devices`,
			Returns: `list`,
			Params: []docspec.Param{
				{Name: `filter`, Type: `string`, Required: false, Description: `client-side substring`},
			},
		},
		{
			Name:    `mobile_device`,
			Summary: `Load one mobile device`,
			Returns: `dict`,
			Params: []docspec.Param{
				{Name: `id`, Type: `string`, Required: false, Description: `device id`},
				{Name: `serial`, Type: `string`, Required: false, Description: `serial number`},
			},
		},
	},
	ExamplesBuddy: []string{
		`def main():
    return mobile_devices(filter="iPad")`,
		`def main():
    d = mobile_device(serial = "F9FXH13QJCL7")
    mobile_device_update_inventory(ids = [int(d["id"])])`,
	},
	Related: []docspec.RelatedLink{
		{Title: `Mobile-device groups`, Ref: `/docs/docs/mobile-groups`},
		{Title: `Mobile profiles`, Ref: `/docs/docs/mobile-profiles`},
		{Title: `Computers`, Ref: `/docs/docs/computers`},
		{Title: `Platform devices`, Ref: `/docs/docs/devices`},
	},
	HelpExport: `mobile_devices`,
	BuiltinsHelp: `MOBILE DEVICE (Jamf Pro /api/v2/mobile-devices; iOS/iPadOS/tvOS)
  mobile_devices(filter?, page?, page_size?, tenant?, surface?, auth?)
      Does: List Jamf Pro mobile devices (summary rows).
      → value: list of {id, name, serial, udid, model, model_identifier, username, type, management_id, wifi_mac_address, phone_number}
      Params: filter?: client-side substring over id/name/serial/udid/username/model/type.
        page?: int (omit = all); page_size?: int (default 200).
  mobile_device(id=?, name=?, serial=?, udid=?, detail?, tenant?, surface?, auth?)
      Does: Load one mobile device (pass exactly one of id/name/serial/udid).
      → value: dict (device JSON; full inventory when detail=True via /detail)
      Params: id=/name=/serial=/udid=: str; detail?: bool (default False).
  mobile_device_send_mdm_command(ids, command_type, command_data?, tenant?, surface?, auth?)
      Does: Send a raw v2 MDM command to mobile devices (clientType MOBILE_DEVICE).
      → None
      Params: ids: list of int; command_type: str (e.g. "SETTINGS");
        command_data?: dict merged into the command body.
  mobile_device_update_inventory(ids, tenant?, surface?, auth?)
      Does: Queue DEVICE_INFORMATION (inventory refresh) on each device.
      → None
      Params: ids: list of int.
  mobile_device_restart(ids, tenant?, surface?, auth?)
      Does: Send RESTART_DEVICE to each device (prompts unless -y).
      → None
      Params: ids: list of int.
  mobile_device_lock(ids, message?, phone?, pin?, tenant?, surface?, auth?)
      Does: Send DEVICE_LOCK (prompts unless -y).
      → None
      Params: ids: list of int; message?/phone?/pin?: str.
  mobile_device_erase(ids, pin?, obliteration?, preserve_data_plan?, disallow_proximity_setup?, tenant?, surface?, auth?)
      Does: Send ERASE_DEVICE — permanently wipes devices (type "erase" to confirm unless -y).
      → None
      Params: ids: list of int; pin?: str; obliteration?: str
        (Default|DoNotObliterate|ObliterateWithWarning|Always);
        preserve_data_plan?/disallow_proximity_setup?: bool.

`,
	CLIHelp: `buddy mobile-device - Manage Jamf Pro mobile devices (iOS/iPadOS/tvOS)

Usage:
  buddy [global-flags] mobile-device <subcommand> [args...]

Subcommands:
  list [--filter=<term>] [--json]      List mobile devices (table or JSON)
  get <mobile-device-id> [--detail]    Fetch one device (JSON; --detail = full inventory)
  update-inventory <mobile-device-id> [<mobile-device-id>...]  Queue inventory refresh
  restart <mobile-device-id> [<mobile-device-id>...]           Restart device(s)
  lock <mobile-device-id> [...] [--message=] [--phone=] [--pin=]   Lock device screen(s)
  erase <mobile-device-id> [...] [--pin=] [--obliteration=] [--preserve-data-plan] [--disallow-proximity-setup]  Wipe device(s)

Notes:
  Jamf Pro mobile-device inventory (not Platform 'buddy device', not 'buddy computer').
  --filter is a client-side substring over id/name/serial/udid/username/model/type.
  restart/lock/erase prompt for confirmation unless -y; erase requires typing "erase".

Examples:
  buddy mobile-device list --filter=iPad
  buddy mobile-device get 42 --detail
  buddy mobile-device update-inventory 42 43
  buddy -y mobile-device restart 42
  buddy mobile-device erase 42 --obliteration=DoNotObliterate
`,
}
