# Profile payloads: dissect and assemble

How to edit Jamf macOS configuration profile payloads with buddy — without inventing Classic PUT bodies.

## Happy path (preferred)

**Edit existing**

1. `profile_xml(id=…)` or MCP `profile_xml` — full Classic XML (preserves outer UUID).
2. Optional inspect: `profile_xml_extract_payloads` → `payloads_list_identifiers` (PayloadContent) or `payloads_get_mcx_dict` (MCX).
3. `profile_xml_patch_payload(xml, domain, new_payload)` — offline patch (**merge** for PayloadContent).
4. `profile_put_xml(profile_id, xml_str)` — write back.

**Create / clone / copy**

1. Prefer **`profile_clone`** (same tenant) or **`profile_copy`** (`from_tenant`/`from_auth` → destination `tenant`/`auth`) — buddy randomizes general + PayloadUUID values and sets unassigned scope by default.
2. Or offline: `profile_xml_prepare_for_create` → `profile_create_xml`.
3. Or greenfield: `profile_create` with `payloads_set_content` / `content`.

Do **not** hand-edit UUIDs in Classic XML — patch preserves PayloadUUID on in-place edits; clone/copy mint new ones.

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
| MCP `profile_create` / `profile_clone` / `profile_copy` / `profile_redeploy` / `payloads_set_content` | **expand** (create path) |
| `profile_xml_randomize_uuids` / `profile_xml_prepare_for_create` / `profile_create_xml` / `profile_delete` | **expand** (UUID + lifecycle) |
| `payloads_collect_uuids` | **expand** (verify remap) |

## Sandbox integration test

Requires a source profile named `buddy-test-source-profile` on a non-production tenant:

```sh
./buddy --tenant=sandbox --user=sandbox-admin scripts/integration_test.buddy
# or: make integration-test
```

Optional: set `TEST_COPY_DEST_TENANT` in the script for cross-tenant `profile_copy` smoke.

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

## MCX ampersands and Classic encoding

Jamf Application & Custom Settings (ACS) **shows** `&amp;` in the editor and **blocks Apply** on a bare `&`. Coders should supply the same Apply form in preference strings:

```python
ExampleURL = "https://example.com/search?q=foo&amp;bar=baz"  # correct (Jamf UI form)
# ExampleURL = "https://example.com/search?q=foo&bar=baz"   # bare '&' — avoid
```

Buddy does **not** silently turn bare `&` into `&amp;`. At write time it **rejects**:

1. **Bare `&` in decoded MCX preference strings** (fail-fast before any ACS/Classic call) — supply Jamf ACS Apply form `&amp;` in the string value. (Bare `&` is not enterable via the Jamf ACS UI Apply path.)
2. **Bare or under-encoded `&` in `<payloads>` XML** (entities must be well-formed; Jamf outer nesting needs `&amp;amp;` for an inner plist `&amp;`).

When preference strings use Apply-form `&amp;` (or payloads encoding is otherwise incompatible with Classic), Classic `POST`/`PUT` may return **409 Conflict**. Buddy then uses Jamf's undocumented **ACS API** (`/api/config-profiles/macos/...` — UI Application & Custom Settings path):

| Operation | Published path (default) | When ACS-preferred |
|-----------|-------------------------|-------------------------|
| Edit / `profile_put_xml` / `set payload` | Classic PUT | `PUT /api/config-profiles/macos/{payloadUUID}` then Classic payloads sync + metadata |
| Clone / copy / `profile_create_xml` | Classic POST | Classic shell + `POST /api/config-profiles/macos` attach, then Classic payloads sync |

After ACS attach, Classic sync must **retain** ampersand-related settings with correct `&amp;` / outer `&amp;amp;` encoding so the ACS tab stays complete. Stripping those keys is treated as failure when amps were required.

**Reads hydrate ACS when Classic MCX is empty.** Some Jamf tenants store preference-domain text (ACS “Application & Custom Settings”) on `/api/config-profiles/macos/{payloadUUID}` while Classic only has an empty ManagedClient shell for that domain. On `profile_xml` / `profile get` / payload inspect / clone-copy source fetch, buddy resolves the ACS document (via `profile/{id}`, then legacy HTML `FIELD_EXTERNAL_CONFIGPROFILE_ID`, then process cache / `BUDDY_ACS_PAYLOAD_UUID_<id>` / `profile_acs_bind_uuid`) and fills **empty** Classic `mcx_preference_settings` from `forced.plist`. Non-empty Classic domains are left alone (Classic-full profiles like Apply-form Edge stay Classic-backed). Jamf UI may use non-standard `<value>` tags in Forced plists; buddy normalizes those for Classic embedding and preserves the raw Forced blob on ACS PUT when settings are unchanged. **Note:** API-token clients often get an SPA shell without `FIELD_EXTERNAL_CONFIGPROFILE_ID` — bind the UUID once (browser form value) so hydrate and ACS writes work.

**Writes:** Apply-form `&amp;` (or Classic 409 with MCX) still prefer the ACS API. Additionally, if an ACS document is already linked for the Classic id and the put includes MCX, buddy uses the ACS path so a hydrated roundtrip does not wipe UI-only ACS content.

**Redeploy:** Classic `redeploy_on_update=All` is applied at most once per operation. A 409 Classic response applies nothing; buddy runs the ACS payload write, then Classic payloads sync and a final metadata PUT for redeploy/scope/name.

**Attach vs edit:** When no ACS payload UUID exists yet, buddy attaches ACS via `PUT …/macos/profile/{classicId}` or `POST …/macos` (full desired MCX body). Subsequent edits (UUID already known) use GET-merge-PUT once. Keep Classic general `<uuid>` stable across PUT updates when copying between instances ([orphaned profiles](https://grahamrpugh.com/2020/04/27/managing-profiles-between-multiple-jamf-instances.html)); buddy’s clone/copy UUID remapping is for *new* profiles, not in-place updates.

**Clone scope:** `profile_clone` defaults to **unassigned** scope (`all_computers=false`). Jamf's UI clone copies source scope; buddy intentionally does not unless you set scope on copy/create.

**Debug logging:** Path selection and hydrate skips are logged at `slog.Debug` — visible with `-v` / `-t` on the CLI or MCP host verbose logging.

**Scoped profiles:** ACS payload saves may push to assigned devices even when `redeploy=false` (no equivalent field on the ACS API). Buddy logs a Debug warning when saving a scoped eligible profile.

**Integration contract:** `scripts/integration_test.buddy` phase A/B exercises Classic-only ACS create/clone/edit; phase C covers Apply-form `&amp;` put (ACS-eligible + Classic retention), bare-`&` reject, Apply-form `profile_create`, and under-encoded payloads reject. Empty-Classic / ACS-full hydrate unit coverage: `profile_acs_hydrate_test.go`. Live same-tenant permutations on a refactored ACS profile (default Classic id=7, mutate+restore): `scripts/integration_test_acs_profile7.buddy` / `make integration-test-acs7`. **Read-only inventory** of which profiles need ACS hydrate / UUID bind: `scripts/scan_acs_profile_handling.buddy` / `make scan-acs-profiles` (class `needs_bind` = unreliable ACS read with API auth until the ACS payload UUID is known).
