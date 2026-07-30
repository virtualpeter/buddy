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

---

## Profiles / payloads cookbook

See **[profile-payloads.md](profile-payloads.md)** for the full scorecard, PayloadContent vs MCX, and merge vs full-replace rules.

Classic configuration profiles embed an entity-encoded payloads plist under `<payloads>`. Prefer typed tools:

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
| `policy` | Looks like full Starlark `policy` | Description: trigger → `{id,trigger}` only |
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
| `profiles` | List profile summaries |
| `profile_xml` | Load Classic XML |
| `profile_put_xml` | Write Classic XML |
| `profile_xml_patch_payload` | Offline domain patch |
| `profile_xml_extract_payloads` | Offline: payloads plist from profile XML |
| `payloads_list_identifiers` | Offline: PayloadIdentifier list |

### Defer

`package` get/upload/delete; `script` / contents; full `policy`/`policies`; computer MDM; smart/static create/delete.

### Cut / do not add

- No first-class MCP `jamf_get`/`jamf_post`/… (keeps bypass costly).
- Do not dump every Starlark builtin onto MCP.
- Do not prune Platform tools in this cycle.
