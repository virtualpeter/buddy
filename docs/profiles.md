# Configuration profiles

Jamf Classic macOS configuration profiles — lifecycle (list, get, create, clone, copy, delete, redeploy). For **payload / MCX / ACS** editing, use the dedicated [profile payloads](profile-payloads.md) cookbook.

## Happy path (preferred)

1. **List / load:** `profiles(...)` / `profile(id=…)` or `profile_xml(id=…)` (XML string; same Classic document).
2. **Clone (same tenant):** `profile_clone(profile_id, name)` — randomizes general + PayloadUUID; default unassigned scope.
3. **Copy / migrate:** `profile_copy(..., to_tenant?, to_auth?)` — source = `tenant`/`auth`; dest = `to_*`.
4. **Create from prepared XML:** `profile_xml_prepare_for_create` → `profile_create_xml`, or greenfield `profile_create` with `payloads_set_content`.
5. **Write back:** `profile_put_xml(profile_id, xml_str)` after offline edits.
6. **Redeploy / ACS:** `profile_redeploy`; `profile_acs_bind_uuid` when legacy HTML scrape fails for ACS payload UUID.

CLI: `buddy profile …`.

## Do not

- Hand-edit UUIDs in Classic XML — clone/copy mint new ones; in-place patch keeps PayloadUUID.
- Invent Classic profile PUT bodies from `jamf_schema` or Jamf MCP `execute-request` (ACS + entity-encoding live in the profile builtins).
- Skip [profile payloads](profile-payloads.md) when editing PayloadContent / MCX.

## Payload edits

See **[profile-payloads.md](profile-payloads.md)**:

```python
def main():
    xml = profile_xml(id=42)
    xml = profile_xml_patch_payload(xml, "com.example", {"Key": "New"})
    profile_put_xml(profile_id=42, xml_str=xml)
    return "ok"
```

## Related

- [Profile payloads](profile-payloads.md) — dissect / assemble
- [Categories](categories.md) — category remap on copy
- [API endpoints](api-endpoints.md) — Configuration Profiles + ACS
- Full signatures: [Function index](/docs/builtins)
