# Profile payloads: dissect and assemble

How to edit Jamf macOS configuration profile payloads with buddy — without inventing Classic PUT bodies.

## Happy path (preferred)

**Edit existing**

1. `profile_xml(id=…)` or MCP `profile_xml` — full Classic XML (preserves outer UUID).
2. Optional inspect: `profile_xml_extract_payloads` → `payloads_list_identifiers` (PayloadContent) or `payloads_get_mcx_dict` (MCX).
3. `profile_xml_patch_payload(xml, domain, new_payload)` — offline patch (**merge** for PayloadContent).
4. `profile_put_xml(profile_id, xml_str)` — write back.

**Create / clone**

1. Optional: MCP `payloads_set_content` (or Starlark) to build a payloads plist from PayloadContent dicts.
2. MCP/Starlark `profile_create` (`name` + `payloads_plist_str` or `content`).
3. Or `profile_clone` to copy an existing profile under a new name with **unassigned** scope (`all_computers=false`, empty targets). Jamf quirk: omitting `<scope>` / `<all_computers>` on Classic POST can default to all computers — buddy always sends an explicit unassigned block.

Do **not** hand-build Classic XML from `jamf_schema` / Jamf hosted MCP `execute-request`.

## PayloadContent vs MCX

| Kind | How domain appears | `new_payload` / dict file shape | Inspect |
|------|--------------------|----------------------------------|---------|
| **PayloadContent** | `PayloadIdentifier` in `PayloadContent` array | Keys to set/override; **patch merges** onto existing (keeps PayloadUUID etc.). Full replace only via `payloads_replace_payload`. | `payloads_list_identifiers`, `payloads_get_payload` |
| **MCX** | Preference-domain `<key>` + `mcx_preference_settings` | **Settings dict only** (contents of `mcx_preference_settings`) | `payloads_get_mcx_dict` |

`profile_xml_patch_payload` / CLI `set payload` / MCP patch: try PayloadContent first, then MCX.

**Get/list honesty:** online `profile_get_payload` / `profile_list_payloads` / CLI `get payload` / `get list-payloads` are **PayloadContent only**. For MCX inspect use extract → `payloads_get_mcx_dict`.

## Recommended names

| Prefer | Also exists | Notes |
|--------|-------------|-------|
| `profile_xml` / `profile_put_xml` | `profile(id=)` same XML; `profile_set_xml` file path | String path for scripts/MCP |
| `profile_xml_patch_payload` | `profile_set_payload` (file + online) | Patch = merge; file set uses same merge |
| `payloads_replace_payload` | — | **Full** dict replace (advanced) |

## Scorecard (dissect / assemble)

| Cluster | Action |
|---------|--------|
| `profile_xml`, `profile_put_xml`, `profile_xml_patch_payload` (+ MCP) | keep + harden (merge, docs) |
| `profile_xml_extract/replace_payloads`, `payloads_*`, `plist_*` | keep; advanced path |
| `profile_get_payload(s)` / list / CLI get | describe-fix: PayloadContent only |
| `profile` vs `profile_xml`, `set_xml` vs `put_xml` | keep; prefer table above |
| `prefs_payload` / `pppc_*` | keep (builders) |
| Missing `<payloads>` on replace | **error** (no silent no-op) |
| `payloads_get_mcx_dict` | **expand** (MCX inspect) |
| MCP `profile_xml_extract_payloads`, `payloads_list_identifiers` | **expand** (inspect before patch) |
| MCP `profile_create` / `profile_clone` / `profile_redeploy` / `payloads_set_content` | **expand** (create path) |

## Advanced Starlark pattern

```python
xml = unwrap(profile_xml(id=42))
payloads = unwrap(profile_xml_extract_payloads(xml))
chrome = unwrap(plist_decode(unwrap(payloads_get_payload(payloads, "com.google.chrome"))))
chrome["SomeKey"] = "NewValue"
payloads = unwrap(payloads_replace_payload(payloads, "com.google.chrome", chrome))  # full replace
xml = unwrap(profile_xml_replace_payloads(xml, payloads))
unwrap(profile_put_xml(profile_id=42, xml_str=xml))
```

For a one-key change, prefer **patch** (merge) instead of full replace.
