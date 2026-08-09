package buddy

import "buddy/internal/docspec"

func init() {
	docspec.Register(ProfilePayloadsTopic)
}

// ProfilePayloadsTopic is the structured doc source for docs/profile-payloads.
// Edit here, then run `make gendocs`.
var ProfilePayloadsTopic = docspec.Topic{
	ID:      `docs/profile-payloads`,
	Title:   `Profile payloads`,
	Section: `Resources`,
	Keywords: []string{
		`profile`,
		`profiles`,
		`payload`,
		`payloads`,
		`mcx`,
		`acs`,
		`plist`,
	},
	Summary: `Edit Jamf macOS configuration profile payloads with buddy (PayloadContent and MCX).`,
	Surfaces: []docspec.SurfaceRow{
		{Action: "Load XML", BuddyScript: "`profile_xml(id=…)`", CLI: "`buddy profile get xml`", Notes: "Preserves outer UUID"},
		{Action: "Inspect", BuddyScript: "`profile_xml_extract_payloads` → `payloads_list_identifiers` / `payloads_get_mcx_dict`", CLI: "`buddy profile get list-payloads` / `get payload`", Notes: "PayloadContent vs MCX; CLI is PayloadContent only"},
		{Action: "Patch", BuddyScript: "`profile_xml_patch_payload(xml, domain, new_payload)`", CLI: "`buddy profile set payload`", Notes: "Offline merge for PayloadContent"},
		{Action: "Write back", BuddyScript: "`profile_put_xml(profile_id, xml_str)`", CLI: "`buddy profile set xml`", Notes: "ACS quirks handled inside"},
		{Action: "Clone / copy", BuddyScript: "`profile_clone` / `profile_copy`", CLI: "`buddy profile clone`", Notes: "Randomizes UUIDs; unassigned scope; copy/migrate is BuddyScript-only"},
		{Action: "Create prep", BuddyScript: "`profile_xml_prepare_for_create` → `profile_create_xml`", Notes: "Offline then POST"},
		{Action: "Greenfield", BuddyScript: "`profile_create` + `payloads_set_content`", Notes: ""},
	},
	AgentNotes: []string{
		"Prefer named builtins via buddyscript; do not invent Classic/Pro PUTs from jamf_schema alone.",
		"Prefer `profile_clone` / `profile_copy` over hand-editing UUIDs when creating copies.",
	},
	Builtins: []docspec.Builtin{
			{
				Name: `profile_xml_patch_payload`,
				Summary: `Merge-patch one payload`,
				Returns: `string`,
				Params: []docspec.Param{
					{Name: `profile_xml_str`, Type: `string`, Required: true, Description: `profile_xml_str`},
					{Name: `domain`, Type: `string`, Required: true, Description: `domain`},
				},
			},
			{
				Name: `profile_xml_extract_payloads`,
				Summary: `Extract payloads plist`,
				Returns: `string`,
				Params: []docspec.Param{
					{Name: `profile_xml_str`, Type: `string`, Required: true, Description: `profile_xml_str`},
				},
			},
			{
				Name: `payloads_replace_payload`,
				Summary: `Full-replace payload`,
				Returns: `string`,
				Params: []docspec.Param{
					{Name: `payloads_plist_str`, Type: `string`, Required: true, Description: `payloads_plist_str`},
					{Name: `domain`, Type: `string`, Required: true, Description: `domain`},
				},
			},
		},
	ExamplesBuddy: []string{
		`def main():
    xml = profile_xml(id=42)
    xml = profile_xml_patch_payload(xml, "com.example", {"Key": "New"})
    profile_put_xml(profile_id=42, xml_str=xml)
    return "ok"`,
		`./buddy --tenant=sandbox --user=sandbox-admin scripts/integration_test.buddy
# or: make integration-test`,
	},
	HelpExport: `profile_payloads`,
	BuiltinsHelp: `PROFILE — component
  profile_put_xml(profile_id, xml_str, redeploy?, tenant?, surface?, auth?) → None
  profile_xml_extract_payloads(profile_xml_str)
      → value: str (payloads plist XML from <payloads>; entities unescaped)
      Params: profile_xml_str: str (offline transform; no API call).
  profile_xml_randomize_uuids(profile_xml_str) → value: str
      Params: Offline; new general UUID + remap PayloadUUID keys; PayloadIdentifier unchanged.
  profile_xml_prepare_for_create(profile_xml_str, name, scope_all?, scope_group_ids?) → value: str
      Params: Offline; name, strip id, scope (default unassigned), randomize UUIDs for Classic POST.
  profile_xml_replace_payloads(profile_xml_str, payloads_plist_str)
      → value: str (updated profile XML)
      Params: profile_xml_str: str; payloads_plist_str: str (offline transform).
  profile_xml_set_redeploy_all(profile_xml_str)
      → value: str (updated profile XML)
      Params: profile_xml_str: str. Sets <redeploy_on_update>All</redeploy_on_update>.
  profile_xml_patch_payload(profile_xml_str, domain, new_payload)
      → value: str (updated profile XML)
      Params: profile_xml_str: str; domain: str (PayloadIdentifier or MCX preference domain);
              new_payload: dict or plist/dict XML. PayloadContent: keys merge onto existing payload
              (PayloadUUID preserved). MCX: settings dict only under mcx_preference_settings.
  payloads_list_identifiers(payloads_plist_str)
      → value: list of str
      Params: payloads_plist_str: str (payloads plist XML). PayloadContent PayloadIdentifiers only.
  payloads_collect_uuids(payloads_plist_str) → value: list of str
      Params: Every PayloadUUID in the plist (verify clone/copy remapping).
  payloads_get_payload(payloads_plist_str, domain)
      → value: str (plist XML dict)
      Params: payloads_plist_str: str; domain: str (PayloadIdentifier). PayloadContent only — not MCX.
  payloads_replace_payload(payloads_plist_str, domain, new_payload)
      → value: str (payloads plist XML)
      Params: payloads_plist_str: str; domain: str; new_payload: dict or plist XML. Full replace (not merge).
  payloads_get_mcx_dict(payloads_plist_str, domain)
      → value: str (bare <dict>…</dict>)
      Params: payloads_plist_str: str; domain: str (MCX preference-domain key). Reads mcx_preference_settings.
  payloads_replace_mcx_dict(payloads_plist_str, domain, dict_xml_str)
      → value: str (payloads plist XML)
      Params: payloads_plist_str: str; domain: str; dict_xml_str: str (MCX settings dict).
  payloads_set_content(content, payloads_plist_str?)
      → value: str (payloads plist XML)
      Params: content: list of payload dicts; payloads_plist_str?: str (omit/empty to start fresh).
  payloads_add_payload(payloads_plist_str, domain, new_payload)
      → value: str (payloads plist XML)
      Params: payloads_plist_str: str (empty ok); domain: str; new_payload: dict or plist XML. Appends or replaces by PayloadIdentifier.
  payloads_remove_payload(payloads_plist_str, domain)
      → value: str (payloads plist XML)
      Params: payloads_plist_str: str; domain: str.
  plist_normalise_dict(data_str)
      → value: str (bare <dict>…</dict>)
      Params: data_str: str (bare dict fragment or full plist).
  pppc_fda(bundle_id, code_requirement, comment?)
      → value: dict (SystemPolicyAllFiles service entry)
      Params: bundle_id: str; code_requirement: str; comment?: str. Offline; no API.
  pppc_payload(services, payload_identifier?, payload_uuid?, payload_display_name?)
      → value: dict (com.apple.TCC.configuration-profile-policy payload)
      Params: services: dict of service name → list of app entries (e.g. from
        pppc_fda). Compose Accessibility etc. as additional service keys. Offline.
  prefs_payload(domain, settings, payload_identifier?, payload_uuid?,
                payload_display_name?)
      → value: dict (preference-domain payload; PayloadType == domain)
      Params: domain: str; settings: dict of preference keys. Offline.

`,
}
