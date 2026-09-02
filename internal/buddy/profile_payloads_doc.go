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
		{Action: "Load XML", BuddyScript: "`profile_xml(id=…)`", CLI: "`buddy profile get xml`", Notes: "Classic envelope; not a bare Apple plist"},
		{Action: "Inspect", BuddyScript: "`profile_xml_extract_payloads` → `payloads_list_identifiers`", CLI: "`buddy profile get list-payloads` / `get payload`", Notes: "List {identifier, type, uuid}; MCX inspect is payloads_get_mcx_dict"},
		{Action: "Patch", BuddyScript: "`profile_xml_patch_payload(xml, identifier, new_payload)`", CLI: "`buddy profile set payload`", Notes: "Offline merge; identifier= is Identifier, unique Type, or unique UUID"},
		{Action: "Write back", BuddyScript: "`profile_put_xml(id, xml_str)`", CLI: "`buddy profile set xml`", Notes: "ACS quirks handled inside"},
		{Action: "Clone / copy", BuddyScript: "`profile_clone` / `profile_copy`", CLI: "`buddy profile clone`", Notes: "Randomizes UUIDs; unassigned scope; copy/migrate is BuddyScript-only"},
		{Action: "Create prep", BuddyScript: "`profile_xml_prepare_for_create` → `profile_create_xml`", Notes: "Offline then POST"},
		{Action: "Greenfield", BuddyScript: "`profile_create` + `payloads_set_content`", Notes: ""},
	},
	AgentNotes: []string{
		"`profile_xml` is the Classic envelope. Pass that string to `profile_xml_patch_payload`. List/get need `profile_xml_extract_payloads` first.",
		"Jamf Apple payloads often have PayloadIdentifier == PayloadUUID; PayloadType is reverse-DNS. `identifier=` accepts Identifier, unique Type, or unique UUID. Always `payloads_list_identifiers` first.",
		"`payloads_get_mcx_dict` is only for ManagedClient / flat mcx_preference_settings preference domains — not `com.apple.MCX` PayloadContent.",
		"Prefer named builtins via buddyscript; do not invent Classic/Pro PUTs from jamf_schema alone.",
		"Prefer `profile_clone` / `profile_copy` over hand-editing UUIDs when creating copies.",
	},
	Builtins: []docspec.Builtin{
		{
			Name:    `profile_xml_patch_payload`,
			Summary: `Merge-patch one payload`,
			Returns: `string`,
			Params: []docspec.Param{
				{Name: `profile_xml_str`, Type: `string`, Required: true, Description: `profile_xml_str`},
				{Name: `identifier`, Type: `string`, Required: true, Description: `PayloadIdentifier, unique PayloadType, unique PayloadUUID, or MCX preference domain`},
			},
		},
		{
			Name:    `profile_xml_extract_payloads`,
			Summary: `Extract payloads plist`,
			Returns: `string`,
			Params: []docspec.Param{
				{Name: `profile_xml_str`, Type: `string`, Required: true, Description: `profile_xml_str`},
			},
		},
		{
			Name:    `payloads_replace_payload`,
			Summary: `Full-replace payload`,
			Returns: `string`,
			Params: []docspec.Param{
				{Name: `payloads_plist_str`, Type: `string`, Required: true, Description: `payloads_plist_str`},
				{Name: `identifier`, Type: `string`, Required: true, Description: `PayloadIdentifier, unique PayloadType, or unique PayloadUUID`},
			},
		},
	},
	ExamplesBuddy: []string{
		`def main():
    xml = profile_xml(id=42)
    plist = profile_xml_extract_payloads(xml)
    rows = payloads_list_identifiers(plist)
    # Jamf Apple payloads: identifier is often a UUID; type is reverse-DNS.
    # Custom payloads (e.g. Chrome): identifier is often reverse-DNS.
    ident = rows[0]["type"]
    xml = profile_xml_patch_payload(xml, identifier=ident, new_payload={"Key": "New"})
    profile_put_xml(id=42, xml_str=xml)
    return rows`,
		`./buddy --tenant=sandbox --user=sandbox-admin scripts/integration_test.buddy
# or: make integration-test`,
	},
	HelpExport: `profile_payloads`,
	BuiltinsHelp: `PROFILE — component
  profile_put_xml(id, xml_str, redeploy?, tenant?, surface?, auth?)
      Does: PUT full profile XML (ACS quirks handled inside).
      → None
      Params: id: int/str; xml_str: str; redeploy?: bool (set redeploy_on_update=All).
  profile_xml_extract_payloads(profile_xml_str)
      Does: Extract the payloads plist from profile XML (entities unescaped).
      → value: str (payloads plist XML from <payloads>)
      Params: profile_xml_str: str (offline transform; no API call).
  profile_xml_randomize_uuids(profile_xml_str)
      Does: Assign a new general UUID and remap PayloadUUID keys (PayloadIdentifier unchanged).
      → value: str (updated profile XML)
      Params: profile_xml_str: str. Offline; no API call.
  profile_xml_prepare_for_create(profile_xml_str, name, scope_all?, scope_group_ids?)
      Does: Prepare profile XML for Classic POST (name, strip id, scope, randomize UUIDs).
      → value: str (POST-ready profile XML)
      Params: profile_xml_str: str; name: str; scope_all?: bool; scope_group_ids?: list of int.
        Offline; default scope is unassigned.
  profile_xml_replace_payloads(profile_xml_str, payloads_plist_str)
      Does: Embed a payloads plist under <payloads> in the profile XML.
      → value: str (updated profile XML)
      Params: profile_xml_str: str; payloads_plist_str: str. Offline.
  profile_xml_set_redeploy_all(profile_xml_str)
      Does: Set <redeploy_on_update>All</redeploy_on_update> on profile XML.
      → value: str (updated profile XML)
      Params: profile_xml_str: str. Offline.
  profile_xml_patch_payload(profile_xml_str, identifier, new_payload)
      Does: Merge-patch one PayloadContent payload or MCX settings dict in profile XML.
      → value: str (updated profile XML)
      Params: profile_xml_str: str; identifier: str (PayloadIdentifier, unique PayloadType,
        unique PayloadUUID, or MCX preference domain if no PayloadContent hit);
              new_payload: dict or plist/dict XML. PayloadContent: keys merge onto existing payload
              (canonical PayloadIdentifier preserved). MCX: settings dict only under mcx_preference_settings.
  payloads_list_identifiers(payloads_plist_str)
      Does: List PayloadContent rows (Identifier, Type, UUID).
      → value: list of {identifier, type, uuid}
      Params: payloads_plist_str: str (payloads plist XML from profile_xml_extract_payloads). PayloadContent only.
  payloads_collect_uuids(payloads_plist_str)
      Does: Collect every PayloadUUID in the plist (verify clone/copy remapping).
      → value: list of str
      Params: payloads_plist_str: str.
  payloads_get_payload(payloads_plist_str, identifier)
      Does: Return the PayloadContent payload selected by identifier=.
      → value: str (plist XML dict)
      Params: payloads_plist_str: str; identifier: str (PayloadIdentifier, unique Type, or unique UUID). Not MCX.
  payloads_replace_payload(payloads_plist_str, identifier, new_payload)
      Does: Fully replace one PayloadContent payload (not a merge).
      → value: str (payloads plist XML)
      Params: payloads_plist_str: str; identifier: str; new_payload: dict or plist XML.
  payloads_get_mcx_dict(payloads_plist_str, domain)
      Does: Return the mcx_preference_settings dict for an MCX preference domain.
      → value: str (bare <dict>…</dict>)
      Params: payloads_plist_str: str; domain: str (MCX preference-domain key, not PayloadType com.apple.MCX).
  payloads_replace_mcx_dict(payloads_plist_str, domain, dict_xml_str)
      Does: Replace mcx_preference_settings for an MCX preference domain.
      → value: str (payloads plist XML)
      Params: payloads_plist_str: str; domain: str; dict_xml_str: str (MCX settings dict).
  payloads_set_content(content, payloads_plist_str?)
      Does: Set the PayloadContent array on a payloads plist (start fresh if omitted).
      → value: str (payloads plist XML)
      Params: content: list of payload dicts; payloads_plist_str?: str (omit/empty to start fresh).
  payloads_add_payload(payloads_plist_str, identifier, new_payload)
      Does: Append a payload, or replace it if identifier= already matches.
      → value: str (payloads plist XML)
      Params: payloads_plist_str: str (empty ok); identifier: str; new_payload: dict or plist XML.
        Sets PayloadIdentifier to identifier when missing on create.
  payloads_remove_payload(payloads_plist_str, identifier)
      Does: Remove the PayloadContent payload selected by identifier=.
      → value: str (payloads plist XML)
      Params: payloads_plist_str: str; identifier: str.
  plist_normalise_dict(data_str)
      Does: Normalise a dict fragment or full plist to a bare <dict>…</dict>.
      → value: str (bare <dict>…</dict>)
      Params: data_str: str (bare dict fragment or full plist).
  pppc_fda(bundle_id, code_requirement, comment?)
      Does: Build a SystemPolicyAllFiles PPPC service entry (offline).
      → value: dict (SystemPolicyAllFiles service entry)
      Params: bundle_id: str; code_requirement: str; comment?: str. No API call.
  pppc_payload(services, payload_identifier?, payload_uuid?, payload_display_name?)
      Does: Build a com.apple.TCC.configuration-profile-policy payload (offline).
      → value: dict (PPPC payload)
      Params: services: dict of service name → list of app entries (e.g. from
        pppc_fda). Compose Accessibility etc. as additional service keys.
        payload_identifier?/payload_uuid?/payload_display_name?: str.
  prefs_payload(domain, settings, payload_identifier?, payload_uuid?,
                payload_display_name?)
      Does: Build a preference-domain payload (PayloadType == domain) (offline).
      → value: dict (preference-domain payload)
      Params: domain: str; settings: dict of preference keys.
        payload_identifier?/payload_uuid?/payload_display_name?: str.

`,
}
