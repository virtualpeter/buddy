# MCP tools: agent guide and scorecard

How AI agents should use buddy MCP tools, and which tools earn their place in the host tool list.

## Tool priority (do this order)

1. **Typed MCP tool** for the resource (e.g. `smart_group`, `static_group_list_computers`, `profile_xml`).
2. **`starlark_eval` with named builtins** (`smart_group_set_criteria`, `payloads_*`, …) for multi-step or offline transforms not exposed as MCP tools.
3. **`jamf_get` / `jamf_put` / …** only when no typed tool or named builtin exists.

Do **not** invent Classic/Pro HTTP from `jamf_schema` when a typed tool already covers the job. Schema is for discovery; typed tools encode Jamf quirks (pagination, READY polling, criteria vs membership, payload entity encoding).

---

## Jamf hosted MCP vs buddy

Hosts may also connect Jamf’s remote MCP at `https://developer.jamf.com/mcp` (tools like `list-specs`, `search-endpoints`, `get-endpoint`, `execute-request`).

| | **Jamf MCP** | **buddy MCP** |
|--|--------------|---------------|
| Role | Official OpenAPI discovery + generic HAR HTTP | Tenant automation with your buddy config/secrets |
| Best for | Specs across Pro/Classic/Platform/Protect/RISK/Title Editor; codegen | Real changes: groups, profiles/payloads, packages/JCDS, Platform CRUD, Starlark |
| Avoid | Using `execute-request` for workflows buddy already wraps | Inventing Classic profile/group PUT bodies from schema alone |

**Split:** Jamf MCP = “what endpoints exist / look like”; buddy = “do the change on my tenant.” After discovering a path in Jamf MCP, prefer a buddy typed tool or named builtin when one exists.

---

## Groups: pick the right family

| Need | Use | Not |
|------|-----|-----|
| Dynamic membership via **criteria** (Application Title, version, …) | Pro **smart** tools: `smart_groups`, `smart_group`, `smart_group_get_criteria`, `smart_group_update_criterion_value` | static_* / device_group* |
| Explicit **computer ID** membership | Pro **static** tools: `static_groups`, `static_group_list_computers`, `static_group_add/remove/set_computers` | smart_* (no computer list) |
| Platform gateway device groups | `device_groups` / `device_group*` | Pro smart/static computer groups |

**Smart group criterion bump (common release step):**

1. `smart_groups` or `smart_group` (by `id` or `name`) to find the group.
2. `smart_group_get_criteria` to inspect.
3. `smart_group_update_criterion_value` with `name_substring` (e.g. `"version"`) and `new_value`.

**Static membership:**

1. `static_group_list_computers` to read `{id,name,serial}`.
2. `static_group_add_computers` / `static_group_remove_computers` / `static_group_set_computers` with `computer_ids` (ints or strings).

**Create groups:** `smart_group_create` / `static_group_create` with a JSON `body` (`name` required).

---

## Profiles / payloads cookbook

See **[profile-payloads.md](profile-payloads.md)** for the full scorecard, PayloadContent vs MCX, and merge vs full-replace rules.

Classic configuration profiles embed an entity-encoded payloads plist under `<payloads>`. Prefer typed tools:

**Create / clone / copy / redeploy**

1. `payloads_set_content` (optional) — build payloads plist from a PayloadContent array.
2. `profile_create` — greenfield POST (`name` + `payloads_plist_str` or `content`; optional scope/category).
3. `profile_clone` / `profile_copy` — one-shot read/transform/post with **UUID remapping** (do not hand-edit UUIDs).
4. `profile_redeploy` — set redeploy_on_update=All and PUT (no payload change).

**Offline prepare (agent escape hatch)**

1. `profile_xml_randomize_uuids` — remap general + PayloadUUID keys only.
2. `profile_xml_prepare_for_create` — name, strip id, scope (default unassigned), randomize UUIDs.
3. `profile_create_xml` — POST prepared XML.

**Edit existing**

1. `profiles` — list `{id,name}` (optional name `filter`).
2. `profile_xml` — load full Classic XML by `profile_id`.
3. Optional: `profile_xml_extract_payloads` → `payloads_list_identifiers` (inspect PayloadContent domains).
4. `profile_xml_patch_payload` — offline patch (**merges** PayloadContent keys; MCX = settings-only dict).
5. `profile_put_xml` — write XML back (`redeploy` optional).

Do **not** invent a Classic PUT body from `jamf_schema` alone.

For extract / decode / full replace / MCX get, use `docs` (`query: "profile-payloads"`) and `starlark_eval` with `payloads_*` / `plist_*`.

---

## Scorecard (keep / fix / expand / cut)

Value criteria: encodes Jamf/buddy knowledge; completes a common task without raw HTTP; description distinguishes near-duplicates; return shape is what agents need next.

### Keep (high value)

| Tool | Value | Notes |
|------|-------|-------|
| `buddy_info` | high | Session/policy without Jamf call |
| `auth_test` | high | Compact by default |
| `docs` | high | Conventions before inventing |
| `jamf_schema` | high | Discovery only — then prefer typed tools |
| `package_ready` / `package_wait_ready` / `package_download` | high | JCDS/READY/SHA knowledge |
| Platform `blueprint*` / `device*` / `device_group*` / `ddm_*` / `benchmark*` | high | Full typed CRUD already |

### Describe-fix (done in this hardening pass)

| Tool | Confusion | Fix |
|------|-----------|-----|
| `smart_groups` / `static_groups` | Near-identical; no criteria vs membership | Descriptions contrast models; point at get/membership tools |
| `policy` | Looks like full Starlark `policy` | Fixed: `id` xor `trigger` → details dict (same shape as Starlark) |
| `computers` / `scripts` / `packages` | List-only stubs | Descriptions say summaries; singular/mutate via Starlark until expanded |
| `starlark_eval` / `jamf_schema` | Encouraged raw `jamf_get` | Prefer typed → named builtins → `jamf_*` last |

### Expand (Tier 1 — this pass)

| Tool | Purpose |
|------|---------|
| `smart_group` | Full smart group JSON by `id` or `name` |
| `smart_group_get_criteria` | Criteria array |
| `smart_group_update_criterion_value` | Bump criterion value by name substring |
| `static_group_list_computers` | Membership `{id,name,serial}` |
| `static_group_add_computers` / `_remove_computers` / `_set_computers` | Membership mutates |
| `smart_group_create` / `static_group_create` | Create Pro computer groups |
| `profiles` | List profile summaries |
| `profile_create` / `profile_clone` / `profile_copy` / `profile_redeploy` / `profile_create_xml` / `profile_delete` | Create / copy / lifecycle |
| `profile_xml_randomize_uuids` / `profile_xml_prepare_for_create` | Offline UUID + create prep |
| `payloads_collect_uuids` | Verify UUID remap after clone/copy |
| `profile_xml` | Load Classic XML |
| `profile_put_xml` | Write Classic XML |
| `profile_xml_patch_payload` | Offline domain patch |
| `profile_xml_extract_payloads` | Offline: payloads plist from profile XML |
| `payloads_list_identifiers` | Offline: PayloadIdentifier list |
| `payloads_set_content` | Offline: build PayloadContent plist for create |
| `policies` / `policy` | List summaries; load details by `id` xor `trigger` |
| `policy_xml` / `policy_put_xml` | Classic XML get/put |
| `policy_set` / `policy_delete` | Basic field edits / delete |
| `policy_copy` / `policy_migrate` | Cross-session copy (POST) / upsert migrate |

### Object-type coverage (MCP vs Starlark)

| Family | MCP today | Still via `starlark_eval` / defer |
|--------|-----------|-----------------------------------|
| Platform blueprint / device / device_group / ddm / benchmark | Near-full CRUD + actions | — |
| Pro smart/static groups | List, get, criteria bump, membership mutate, **create** | Delete; full `smart_group_put` / set_criteria |
| Profiles | List, create, clone, **copy**, redeploy, XML get/put, patch, inspect, set_content, delete | Advanced `payloads_*` / MCX get |
| Packages | List + ready/wait/download | get/upload/delete/put |
| Scripts | List | get/contents/update/put |
| Policies | List, get (id\|trigger), XML get/put, set, delete, **copy**, **migrate** | Release helpers (`policy_set_ss` / `_package` / versioned `policy_clone` / create_*) |
| Computers | List summaries | MDM actions |

### Defer

`package` get/upload/delete; `script` / contents; computer MDM; group delete.

### Cut / do not add

- No first-class MCP `jamf_get`/`jamf_post`/… (keeps bypass costly).
- Do not dump every Starlark builtin onto MCP.
- Do not prune Platform tools in this cycle.
