package buddy

import "buddy/internal/docspec"

func init() {
	docspec.Register(DevicesTopic)
}

// DevicesTopic is the structured doc source for docs/devices.
// Edit here, then run `make gendocs`.
var DevicesTopic = docspec.Topic{
	ID:      `docs/devices`,
	Title:   `Platform devices`,
	Section: `Platform`,
	Keywords: []string{
		`device`,
		`devices`,
		`platform`,
		`device_action`,
		`check-in`,
		`restart`,
	},
	Summary: "Platform API Gateway device inventory and **actions** (check-in, restart, erase, …). This is **not** Pro [computers](/docs/docs/computers) — different IDs, gateway, and builtins (`device*` vs `computer*`).",
	Surfaces: []docspec.SurfaceRow{
		{Action: "List / load", BuddyScript: "`devices(...)` / `device(id=…)`", CLI: "`buddy device list` / `get`", Notes: ""},
		{Action: "Actions", BuddyScript: "`device_*` action builtins", CLI: "`buddy device check-in|restart|erase|…`", Notes: "Check-in, restart, erase, …"},
	},
	Notes: []string{
		"Platform gateway inventory — not Pro [computers](/docs/docs/computers) (`device*` vs `computer*`).",
	},
	AgentNotes: []string{
		"Do not use `computer*` builtins against Platform device IDs (or the reverse).",
		"Prefer `device_*` helpers over raw device-actions HTTP / generic `jamf_put`.",
		"Do not confuse with [MDM command history](/docs/docs/mdm) on the Pro instance API.",
	},
	Builtins: []docspec.Builtin{
			{
				Name: `devices`,
				Summary: `List platform devices`,
				Returns: `list`,
				Params: []docspec.Param{
					{Name: `filter`, Type: `string`, Required: false, Description: `filter`},
				},
			},
			{
				Name: `device`,
				Summary: `Load one device`,
				Returns: `dict`,
				Params: []docspec.Param{
					{Name: `id`, Type: `string`, Required: true, Description: `id`},
				},
			},
		},
	ExamplesBuddy: []string{
		`def main():
    return devices(page_size=50)`,
	},
	Related: []docspec.RelatedLink{
		{Title: `Computers`, Ref: `/docs/docs/computers`},
		{Title: `Device groups`, Ref: `/docs/docs/device-groups`},
		{Title: `API endpoints`, Ref: `/docs/docs/api-endpoints`},
	},
	HelpExport: `devices`,
	BuiltinsHelp: `PLATFORM DEVICE (Platform gateway; distinct from computer/Pro inventory)
  devices(filter?, sort?, page?, page_size?, tenant?, surface?, auth?)
      Does: List Platform API devices (not Pro computer inventory).
      → value: list of {id, name, serialNumber?, …}
      Params: filter?: bare term → name=="*term*", or full RSQL. sort?: str.
        page?: int (omit = all); page_size?: int.
  device(id=, tenant?, surface?, auth?)
      Does: Fetch one Platform device JSON document.
      → value: dict (full device record)
      Params: id=: str (device UUID).
  device_set(device_id, body, tenant?, surface?, auth?)
      Does: PATCH a Platform device with a JSON body.
      → None
      Params: device_id: str; body: dict (merge-patch JSON).
  device_delete(device_id, tenant?, surface?, auth?)
      Does: Delete a Platform device record.
      → None
      Params: device_id: str.
  device_applications(device_id, filter?, page?, page_size?, tenant?, surface?, auth?)
      Does: List applications installed on a Platform device.
      → value: list of dicts
      Params: device_id: str; filter?: bare term → name=="*term*", or full RSQL.
        page?: int (omit = all); page_size?: int.
  device_check_in(device_id, tenant?, surface?, auth?)
      Does: Request a Platform device check-in.
      → None
      Params: device_id: str.
  device_restart(device_id, tenant?, surface?, auth?)
      Does: Restart a Platform device.
      → None
      Params: device_id: str.
  device_shutdown(device_id, tenant?, surface?, auth?)
      Does: Shut down a Platform device.
      → None
      Params: device_id: str.
  device_erase(device_id, tenant?, surface?, auth?)
      Does: Erase a Platform device.
      → None
      Params: device_id: str.
  device_unmanage(device_id, tenant?, surface?, auth?)
      Does: Remove remote management from a Platform device.
      → None
      Params: device_id: str.

`,
	CLIHelp: `buddy device - Manage Jamf Platform API devices (inventory + actions)

Usage:
  buddy [global-flags] device <subcommand> [args...]

Subcommands:
  list [--filter=<term|rsql>] [--sort=]    List Platform devices (table)
  get <device-id>                      Fetch one device (JSON)
  set <device-id> [--file=|-]          PATCH device JSON
  delete <device-id>                   Delete a device
  applications <device-id> [--filter=] List installed applications
  user <user-id>                       List devices for a user
  check-in <device-id>                 Request device check-in
  restart <device-id>                  Restart device
  shutdown <device-id>                 Shut down device
  erase <device-id>                    Erase device
  unmanage <device-id>                 Remove remote management

Notes:
  Platform Devices API (not Jamf Pro computer inventory). Use 'buddy computer' for Pro/MDM.
  Requires Platform API Gateway tenant+auth. Actions need execute:pro:device-actions.
  --filter bare term → name=="*term*"; or full RSQL.

Examples:
  buddy device list --filter=Mac
  buddy device list --filter='name=="*Mac*"'
  buddy device get <device-id>
  buddy device applications <device-id>
  buddy device restart <device-id>
`,
}
