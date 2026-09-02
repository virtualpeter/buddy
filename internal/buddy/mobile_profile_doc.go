package buddy

import "buddy/internal/docspec"

func init() {
	docspec.Register(MobileProfilesTopic)
}

// MobileProfilesTopic is the structured doc source for docs/mobile-profiles.
// Edit here, then run `make gendocs`.
var MobileProfilesTopic = docspec.Topic{
	ID:      `docs/mobile-profiles`,
	Title:   `Mobile profiles`,
	Section: `Resources`,
	Keywords: []string{
		`mobile profile`,
		`mobile_profiles`,
		`mobile_profile_xml`,
		`mobile_profile_put_xml`,
		`mobiledeviceconfigurationprofiles`,
	},
	Summary: "Jamf Pro **mobile-device** configuration profiles via Classic `/JSSResource/mobiledeviceconfigurationprofiles` (XML envelope). The offline payload pipeline — `profile_xml_extract_payloads`, `payloads_*`, `plist_*` — works on mobile profile XML exactly as it does for [computer profiles](/docs/docs/profiles).",
	Surfaces: []docspec.SurfaceRow{
		{Action: "List", BuddyScript: "`mobile_profiles(filter?)`", CLI: "`buddy mobile-profile list`", Notes: ""},
		{Action: "Fetch XML", BuddyScript: "`mobile_profile_xml(id=…|name=…)`", CLI: "`buddy mobile-profile get <profile-id>`", Notes: "Classic envelope"},
		{Action: "Save XML", BuddyScript: "`mobile_profile_put_xml(id, xml_str, redeploy?)`", CLI: "`buddy mobile-profile put <profile-id> [--redeploy]`", Notes: ""},
		{Action: "Create / delete", BuddyScript: "`mobile_profile_create_xml` / `mobile_profile_delete`", CLI: "`buddy mobile-profile create|delete`", Notes: ""},
		{Action: "Edit payloads", BuddyScript: "offline `profile_xml_*` / `payloads_*` helpers", CLI: "—", Notes: "Same helpers as computer profiles"},
	},
	Notes: []string{
		"Classic XML API — the envelope wraps a payloads plist, same as computer profiles.",
		"`redeploy=True` sets `<redeploy_on_update>All</redeploy_on_update>` before the PUT.",
		"Offline helpers (`profile_xml_extract_payloads`, `profile_xml_patch_payload`, `payloads_*`) take XML strings and never call the API, so they work on mobile profile XML unchanged.",
	},
	AgentNotes: []string{
		"Use `mobile_profile_xml` / `mobile_profile_put_xml` for mobile profiles — `profile_xml` / `profile_put_xml` hit the computer endpoint.",
		"Clone by fetching XML, running `profile_xml_prepare_for_create(xml, name)`, then `mobile_profile_create_xml`.",
	},
	Builtins: []docspec.Builtin{
		{
			Name:    `mobile_profiles`,
			Summary: `List mobile profiles`,
			Returns: `list`,
			Params: []docspec.Param{
				{Name: `filter`, Type: `string`, Required: false, Description: `name substring`},
			},
		},
		{
			Name:    `mobile_profile_xml`,
			Summary: `Fetch profile XML`,
			Returns: `str`,
			Params: []docspec.Param{
				{Name: `id`, Type: `string`, Required: false, Description: `profile id`},
			},
		},
	},
	ExamplesBuddy: []string{
		`def main():
    return mobile_profiles(filter="WiFi")`,
		`def main():
    xml = mobile_profile_xml(name = "WiFi - Corp")
    xml = profile_xml_patch_payload(xml, identifier="com.apple.wifi.managed", new_payload={"SSID_STR": "Corp-New"})
    pid = mobile_profiles(filter="WiFi - Corp")[0]["id"]
    mobile_profile_put_xml(id = pid, xml_str = xml, redeploy = True)`,
	},
	Related: []docspec.RelatedLink{
		{Title: `Computer profiles`, Ref: `/docs/docs/profiles`},
		{Title: `Profile payloads`, Ref: `/docs/docs/profile-payloads`},
		{Title: `Mobile devices`, Ref: `/docs/docs/mobile-devices`},
	},
	HelpExport: `mobile_profiles`,
	BuiltinsHelp: `MOBILE PROFILES (Classic /JSSResource/mobiledeviceconfigurationprofiles; XML)
  mobile_profiles(filter?, tenant?, surface?, auth?)
      Does: List mobile-device configuration profiles.
      → value: list of {id, name}
      Params: filter?: case-insensitive name substring (client-side).
  mobile_profile_xml(id=?) | mobile_profile_xml(name=?, tenant?, surface?, auth?)
      Does: Fetch one mobile profile as Classic XML (envelope with payloads plist).
      → value: str (XML)
      Params: pass either id= or name= (exact match preferred, unique substring fallback).
  mobile_profile_put_xml(id, xml_str, redeploy?, tenant?, surface?, auth?)
      Does: PUT profile XML back; redeploy=True sets redeploy_on_update=All first.
      → None
      Params: id: int; xml_str: str; redeploy?: bool (default False).
  mobile_profile_create_xml(xml_str, tenant?, surface?, auth?)
      Does: POST new mobile profile XML.
      → value: dict {"id"}
      Params: xml_str: str (prepare with profile_xml_prepare_for_create for clones).
  mobile_profile_delete(id, tenant?, surface?, auth?)
      Does: Delete a mobile profile.
      → None
      Params: id: int.

  Offline payload editing (profile_xml_extract_payloads, profile_xml_patch_payload,
  payloads_*) works on mobile profile XML — see help topic profile-payloads.

`,
	CLIHelp: `buddy mobile-profile - Manage mobile-device configuration profiles (Classic XML)

Usage:
  buddy [global-flags] mobile-profile <subcommand> [args...]

Subcommands:
  list [--filter=<term>]                       List profiles (table)
  get <profile-id>                             Print profile XML to stdout
  put <profile-id> [--file=<file>|-] [--redeploy]  PUT profile XML (stdin default)
  create [--file=<file>|-]                     POST new profile XML (stdin default)
  delete <profile-id>                          Delete a profile

Notes:
  Jamf Pro mobile-device profiles (not 'buddy profile', which is computers).
  --redeploy sets redeploy_on_update=All so existing devices receive the update.
  Payload editing is scriptable via the offline profile_xml_* / payloads_* builtins.

Examples:
  buddy mobile-profile list --filter=WiFi
  buddy mobile-profile get 12 > profile.xml
  buddy mobile-profile put 12 --file=profile.xml --redeploy
  buddy mobile-profile create --file=new-profile.xml
`,
}
