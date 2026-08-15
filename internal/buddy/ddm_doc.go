package buddy

import "buddy/internal/docspec"

func init() {
	docspec.Register(DDMTopic)
}

// DDMTopic is the structured doc source for docs/ddm.
// Edit here, then run `make gendocs`.
var DDMTopic = docspec.Topic{
	ID:      `docs/ddm`,
	Title:   `DDM reporting`,
	Section: `Platform`,
	Keywords: []string{
		`ddm`,
		`declaration`,
		`declarations`,
		`declarative`,
	},
	Summary: `Platform Declarative Device Management (DDM) **declaration reporting** — read-oriented devices / declarations / channels. Requires platform-capable tenant + auth.`,
	Surfaces: []docspec.SurfaceRow{
		{Action: "Report", BuddyScript: "`ddm_*` builtins", CLI: "`buddy ddm declaration|device …`", Notes: "Devices / declarations / channels"},
	},
	Notes: []string{
		"Read-oriented Platform DDM declaration reporting. Requires platform-capable tenant + auth.",
	},
	AgentNotes: []string{
		"Do not treat DDM reporting as a substitute for blueprint deploy or configuration profile push.",
		"Do not invent report paths from schema when `ddm_*` helpers exist.",
	},
	Builtins: []docspec.Builtin{
			{
				Name: `ddm_declarations`,
				Summary: `List DDM declarations`,
				Returns: `list`,
			},
		},
	ExamplesBuddy: []string{
		`def main():
    return ddm_declaration_devices()`,
	},
	Related: []docspec.RelatedLink{
		{Title: `Blueprints`, Ref: `/docs/docs/blueprints`},
		{Title: `API endpoints`, Ref: `/docs/docs/api-endpoints`},
	},
	HelpExport: `ddm`,
	BuiltinsHelp: `DDM REPORT (read-only Platform declaration reporting)
  ddm_declaration_devices(declaration_id, filter?, sort?, page?, page_size?, tenant?, surface?, auth?)
      Does: List devices reporting a given DDM declaration.
      → value: JSON (device report list)
      Params: declaration_id: str; filter?: bare term → name=="*term*", or full RSQL;
        sort?: str; page?: int (omit = all); page_size?: int.
  ddm_device_declarations(device_id, filter?, sort?, page?, page_size?, tenant?, surface?, auth?)
      Does: List DDM declarations reported for a Platform device.
      → value: JSON (declaration report list)
      Params: device_id: str; filter?: bare term → name=="*term*", or full RSQL;
        sort?: str; page?: int (omit = all); page_size?: int.
  ddm_device_channels(device_id, tenant?, surface?, auth?)
      Does: List DDM channels available for a Platform device.
      → value: JSON (channel list)
      Params: device_id: str.

`,
	CLIHelp: `buddy ddm - Jamf Platform DDM declaration reporting (read-only)

Usage:
  buddy [global-flags] ddm <subcommand> [args...]

Subcommands:
  declaration <declaration-id> [--filter=] [--sort=]
      List devices reporting a declaration
  device <device-id> declarations [--filter=] [--sort=]
      List declarations reported for a device
  device <device-id> channels
      List channels available for a device

Notes:
  Requires Platform API Gateway tenant+auth and read:pro:declaration-reporting.
  Useful alongside 'buddy blueprint status' for deployment verification.
  --filter bare term → name=="*term*"; or full RSQL.

Examples:
  buddy ddm declaration com.jamf.ddm.sw-updates
  buddy ddm declaration com.jamf.ddm.sw-updates --filter=lab
  buddy ddm device <device-id> declarations
  buddy ddm device <device-id> channels
`,
}
