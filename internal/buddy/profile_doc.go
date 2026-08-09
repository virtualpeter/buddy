package buddy

import "buddy/internal/docspec"

func init() {
	docspec.Register(ProfilesTopic)
}

// ProfilesTopic is the structured doc source for docs/profiles.
// Edit here, then run `make gendocs`.
var ProfilesTopic = docspec.Topic{
	ID:      `docs/profiles`,
	Title:   `Configuration profiles`,
	Section: `Resources`,
	Keywords: []string{
		`profile`,
		`profiles`,
		`clone`,
		`copy`,
		`redeploy`,
		`acs`,
	},
	Summary: `Jamf Classic macOS configuration profiles — lifecycle (list, get, create, clone, copy, delete, redeploy). For **payload / MCX / ACS** editing, use the dedicated [profile payloads](/docs/docs/profile-payloads) cookbook.`,
	Surfaces: []docspec.SurfaceRow{
		{Action: "List / load", BuddyScript: "`profiles(...)` / `profile(id=…)` / `profile_xml(id=…)`", CLI: "`buddy profile list` / `get xml`", Notes: "XML string is the same Classic document"},
		{Action: "Clone", BuddyScript: "`profile_clone(profile_id, name)`", CLI: "`buddy profile clone`", Notes: "Same tenant; new UUIDs; unassigned scope"},
		{Action: "Copy / migrate", BuddyScript: "`profile_copy(..., to_tenant?, to_auth?)`", Notes: "Source = tenant/auth; dest = to_*"},
		{Action: "Create", BuddyScript: "`profile_xml_prepare_for_create` → `profile_create_xml`", Notes: "Or greenfield `profile_create` + payloads"},
		{Action: "Write back", BuddyScript: "`profile_put_xml(profile_id, xml_str)`", CLI: "`buddy profile set xml`", Notes: "After offline edits"},
		{Action: "Redeploy / ACS", BuddyScript: "`profile_redeploy` / `profile_acs_bind_uuid`", CLI: "`buddy profile redeploy`", Notes: "ACS bind is BuddyScript-only when HTML scrape fails"},
	},
	Notes: []string{
		"Payload / MCX / ACS editing is covered in [profile payloads](/docs/docs/profile-payloads).",
	},
	AgentNotes: []string{
		"Do not hand-edit UUIDs in Classic XML — clone/copy mint new ones; in-place patch keeps PayloadUUID.",
		"Do not invent Classic profile PUT bodies from `jamf_schema` or Jamf MCP `execute-request` (ACS + entity-encoding live in the profile builtins).",
		"Do not skip [profile payloads](/docs/docs/profile-payloads) when editing PayloadContent / MCX.",
	},
	Builtins: []docspec.Builtin{
			{
				Name: `profiles`,
				Summary: `List profiles`,
				Returns: `list`,
				Params: []docspec.Param{
					{Name: `filter`, Type: `string`, Required: false, Description: `filter`},
				},
			},
			{
				Name: `profile`,
				Summary: `Load profile XML`,
				Returns: `string/dict`,
			},
			{
				Name: `profile_copy`,
				Summary: `Copy/migrate profile`,
				Returns: `dict {id,name}`,
				Params: []docspec.Param{
					{Name: `profile_id`, Type: `string`, Required: true, Description: `profile_id`},
				},
			},
			{
				Name: `profile_put_xml`,
				Summary: `Save profile XML`,
				Returns: `None`,
				Params: []docspec.Param{
					{Name: `profile_id`, Type: `string`, Required: true, Description: `profile_id`},
					{Name: `xml_str`, Type: `string`, Required: true, Description: `xml_str`},
				},
			},
		},
	ExamplesBuddy: []string{
		`def main():
    xml = profile_xml(id=42)
    xml = profile_xml_patch_payload(xml, "com.example", {"Key": "New"})
    profile_put_xml(profile_id=42, xml_str=xml)
    return "ok"`,
	},
	Related: []docspec.RelatedLink{
		{Title: `Profile payloads`, Ref: `/docs/docs/profile-payloads`},
		{Title: `Categories`, Ref: `/docs/docs/categories`},
		{Title: `API endpoints`, Ref: `/docs/docs/api-endpoints`},
	},
	HelpExport: `profiles`,
	BuiltinsHelp: `PROFILE
  profiles(filter?)
      → value: list of {id, name}
  profile(id=?) | profile(name=?, tenant?, surface?, auth?)
      → value: XML str (id=) or {id, name} (name=)
  profile_xml(id, tenant?, surface?, auth?) → value: str (raw profile XML)
  profile_create(name, payloads_plist_str?, content?, category?,
                 scope_all?, scope_group_ids?, redeploy_on_update?, tenant?, surface?, auth?)
      → value: int (new profile ID)
      Params: name: str; pass payloads_plist_str and/or content (list of payload
        dicts). scope_all defaults true when scope_group_ids omitted. Classic POST.
  profile_clone(profile_id, name, tenant?, surface?, auth?) → value: dict {id, name}
      Params: Alias of profile_copy without to_* (same-session UUID remap).
  profile_copy(profile_id, name?, category_id?, scope_all?, scope_group_ids?, upsert?=False,
               tenant?, surface?, auth?, to_tenant?, to_surface?, to_auth?) → value: dict {id, name} or +created
      Params: Source = tenant?/auth?; dest = to_* (omit = same session). UUID remap + unassigned scope.
        upsert=True: upsert by exact name (name optional → source name).
  profile_migrate(...) → same as profile_copy(..., upsert=True)
  profile_create_xml(xml_str, name?, tenant?, surface?, auth?) → value: dict {id, name}
      Params: POST prepared XML (after profile_xml_prepare_for_create).
  profile_delete(profile_id, tenant?, surface?, auth?) → None
  profile_redeploy(profile_id, tenant?, surface?, auth?) → None
  profile_acs_bind_uuid(profile_id, payload_uuid) → None
      Params: Seeds process cache classic id → ACS payload UUID (FIELD_EXTERNAL_CONFIGPROFILE_ID).
        Needed when API-token clients cannot scrape legacy HTML; enables read hydrate + ACS writes.
  profile_get_payloads(profile_id) → value: str
  profile_get_payload(profile_id, domain) → value: str
      Params: domain is PayloadIdentifier (PayloadContent only; not MCX — use payloads_get_mcx_dict offline).
  profile_list_payloads(profile_id) → value: list of str
      Params: PayloadIdentifier list (PayloadContent only).
  profile_set_payload(profile_id, domain, dict_file, redeploy?, tenant?, surface?, auth?) → None
      Params: PayloadContent merge or MCX settings dict (same as profile_xml_patch_payload).

`,
	CLIHelp: `buddy profile - Manage macOS configuration profiles

Usage:
  buddy [global-flags] profile <subcommand> [args...]

Subcommands:
  list [--filter=<name>]                                    List configuration profiles
  get xml <profile-id>                                      Output full profile XML
  get payloads <profile-id>                                 Output payloads plist only (pretty-printed)
  get payload <profile-id> <domain>                          Output single PayloadContent payload by PayloadIdentifier
  get list-payloads <profile-id>                            List PayloadIdentifier values (PayloadContent only)
  set xml <profile-id> [--redeploy]                         PUT profile from XML on stdin
  set payload <profile-id> <domain> <dict-file.xml> [--redeploy]   Patch payload (PayloadContent merge or MCX settings)
  clone <profile-id> <new-name>                             Clone profile with new name, unassigned scope
  redeploy <profile-id>                                     Redeploy to all assigned devices

Notes:
  --redeploy sets redeploy_on_update=All (pushes to all assigned devices, not just new ones)
  get payload / list-payloads are PayloadContent only. For MCX inspect use profile XML extract + payloads_get_mcx_dict.
  set payload: PayloadContent merges keys onto the existing payload; MCX dict file is settings-only.

Examples:
  buddy profile list
  buddy profile list --filter=Chrome
  buddy profile get xml 99
  buddy profile get payloads 99
  buddy profile get list-payloads 99
  buddy profile get payload 99 com.google.chrome
  buddy profile clone 99 "New Profile"
  buddy profile set xml 99 < profile.xml
  buddy profile set payload 99 com.google.chrome payload.xml --redeploy
  buddy profile redeploy 99
`,
}
