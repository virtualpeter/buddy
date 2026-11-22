# MCP tools: agent guide and scorecard

How AI agents should use buddy MCP tools after the lean-surface cut.

## Tool priority (do this order)

0. **`get_instructions`** — call once at session start (firm playbook: dialect, return contract, docs-first, long-work guidance).
1. **`docs`** — object-type cookbooks before inventing. Query a **resource noun** (`policy`, `packages`, `smart_groups` / `smart groups`, `blueprints`, …) or a page id (`profile-payloads`, `mcp-tools`), then `get` that page. Do **not** get `builtins` (too large). To learn one function, `buddyscript` the **bare name** (no parentheses) — e.g. source `computer_history` prints signature / Does / params.
2. **`buddyscript`** with **named builtins** (`smart_group_*`, `policy_migrate`, `profile_xml_*`, `package_wait_ready`, `blueprint_*`, …) for Jamf work. Prefer **one** `def main()` over many tiny round-trips (host timeouts).
3. **`jamf_get` / `jamf_put` / …** via `buddyscript` only when no named builtin exists **and** they are not blocked (`buddy_info.deny_builtins` / `allow_builtins`, plus context/auth lists). MCP defaults deny `jamf_*` on the channel layer; each layer is independent (`deny_builtins: ["*"]` is an allowlist at that layer).

Do **not** invent Classic/Pro HTTP from `jamf_schema` when a named builtin already covers the job. Schema is for discovery; builtins encode Jamf quirks (pagination, READY polling, criteria vs membership, payload entity encoding, ACS). When a call raises *blocked by … deny_builtins*, treat it as a gap: use a named builtin or stop — do not invent an alternate HTTP path. Channel `allow_builtins` cannot unblock a context or auth deny.

Typed per-resource MCP tools (smart/static groups, profiles, policies, packages READY/download, Platform CRUD, list stubs) were **removed** to cut host schema token cost. The same capabilities remain as buddyscript builtins — lean MCP did **not** remove ACS/profile quirk handling.

---

## Pathfinding map

| Task | Docs query first | Named builtins (via `buddyscript`) | Do not |
|------|------------------|--------------------------------------|--------|
| Policies | `policies` / `policy` | `policies` / `policy` / `policy_set*` / `policy_copy` | Invent Classic policy PUT from schema |
| Smart group criteria | `smart-groups` / `smart` | `smart_group*` / `smart_group_update_criterion_value` | `static_group*` / invent criteria PUT |
| Smart group members | `smart-groups` / `smart` | `smart_group_computers` | Classic computergroup XML / invent from schema |
| Static membership | `static-groups` / `static` | `static_group_computers` / add/remove/set | `smart_group*` for mutate |
| Edit profile / ACS prefs | `profile-payloads` | `profile_xml` → `payloads_list_identifiers` → `profile_xml_patch_payload(identifier=)` → `profile_put_xml` | Classic PUT from schema; `payloads_get_mcx_dict` on `com.apple.MCX` PayloadContent |
| Create/clone profile | `profiles` / `profile-payloads` | `profile_create` / `profile_clone` / `profile_copy` | Hand-edit UUIDs |
| Package after upload | `packages` / `jcds` | `package_wait_ready` / `package_ready` / `package_download` | Attach/download before READY |
| Self Service icons | `icons` / `icon` | `icons` / `icon` / `icon_upload` / `icon_check` / `icon_download` / `policy_set_icon` | Skip dest POST because ICS HEAD was 200; scrape Classic `policies.html` |
| Scripts | `scripts` | `scripts` (summaries include `localFilename`, no body) / `script(id=|name=|file=)` / `script_check` / `scripts_check` / `script_diff` / `script_update` / `script_copy` | Expect bodies from `scripts()`. Pass a file path to `script_create` (the body is a string). `script_update(file=..., create=True)` creates from that file when no script owns the download name. `script_put` replaces only the keys it knows. `name=` accepts a local filename |
| Categories | `categories` | `category_copy(..., upsert=True)` first in migrate order | Skip before policy migrate |
| Computers / MDM actions | `computers` | `computers` / `computer_*` actions | Mix with Platform `device*` |
| Policy execution logs | `computers` / `policies` | `computer_history` / `computer_policy_logs` / `policy_logs(id, computer_id=… or computer_ids=… or limit=…)` | `jamf_get` computerhistory; `/api/v1/policies` logs; unconstrained fleet scan; treating `last_contact` as logs-since |
| MDM history | `mdm` | `mdm_commands` | Use history APIs to *issue* actions |
| API roles / clients | `api-roles` / `api-clients` | `role*` / `client*` | Store secrets only in config.yaml |
| Blueprints | `blueprints` | `blueprint*` | Instance-only session |
| Platform devices / groups | `devices` / `device-groups` | `device*` / `device_group*` | Pro smart/static groups |
| DDM / benchmarks | `ddm` / `benchmarks` | `ddm_*` / `benchmark*` | Invent from schema alone |
| Unknown endpoint | `jamf_schema` then `builtins` | Prefer named builtin; else `jamf_get`/`jamf_put` | Schema as license to PUT profiles/groups |

Object cookbooks live under `docs/<name>` (see [About](index.md) Resources / Platform tables).

---

## Jamf hosted MCP vs buddy

Hosts may also connect Jamf’s remote MCP at `https://developer.jamf.com/mcp` (tools like `list-specs`, `search-endpoints`, `get-endpoint`, `execute-request`).

| | **Jamf MCP** | **buddy MCP** |
|--|--------------|---------------|
| Role | Official OpenAPI discovery + generic HAR HTTP | Tenant automation with your buddy config/secrets |
| Best for | Specs across Pro/Classic/Platform/Protect/RISK/Title Editor; codegen | Real changes via `buddyscript` builtins |
| Avoid | Using `execute-request` for workflows buddy already wraps | Inventing Classic profile/group PUT bodies or JCDS download loops from schema alone |

**Split:** Jamf MCP = “what endpoints exist / look like”; buddy = “do the change on my tenant.” After discovering a path in Jamf MCP, prefer a buddy named builtin via `buddyscript` when one exists.

---

## Lean MCP tool surface (keep)

| Tool | Role |
|------|------|
| `get_instructions` | Firm playbook; call once before other tools |
| `buddy_info` | Session/channel without Jamf call |
| `docs` | Embedded conventions / object cookbooks |
| `jamf_schema` | Endpoint discovery only |
| `buddyscript` | **Primary** automation (all Pro + Platform builtins) |

### buddyscript return contract

- **In buddyscript:** builtins return **bare values**; failures **raise**. No `{ok, value, error}` ceremony; do not use `unwrap` for correctness.
- **MCP tool boundary only:** `buddyscript` returns `{ok, value: {stdout, stderr, result, error}}`.
- **`unwrap`** is optional JSON-print for CLI/`jq`.

### `buddyscript` output contract

Returns `{ok, value: {stdout, stderr, result, error}}`.

- **Primary output is `value.result`** — last expression or `main()` return.
- **`stdout`** is from `print` / `unwrap` only. When `result` is set and `stdout` is empty, buddy **mirrors `result` into `stdout`** so agents that only read stdout still see data.
- Assignment-only with no `return` / bare expression yields empty both.
- Prefer `def main(): … return …` with **no** trailing top-level expression (a trailing expression sets `result` and skips calling `main`). Or use a single bare expression.

```python
def main():
    return scripts(filter="Chrome")
```

---

## Object cookbooks (pointers)

Inline folklore moved into per-object pages — `docs` list matches **keywords** as well as id/title/section:

| Topic | Docs page |
|-------|-----------|
| Packages / JCDS READY | [packages.md](/docs/docs/packages) |
| Smart vs static vs device groups | [smart-groups.md](/docs/docs/smart-groups), [static-groups.md](/docs/docs/static-groups), [device-groups.md](/docs/docs/device-groups) |
| Profiles / payloads / ACS | [profiles.md](/docs/docs/profiles), [profile-payloads.md](/docs/docs/profile-payloads) |
| Scripts list vs body | [scripts.md](/docs/docs/scripts) |
| Extension attributes | [extension-attributes.md](/docs/docs/extension-attributes) |
| Cross-tenant copy / upsert | [categories.md](/docs/docs/categories) → [scripts.md](/docs/docs/scripts) → [extension-attributes.md](/docs/docs/extension-attributes) → [policies.md](/docs/docs/policies) / [profiles.md](/docs/docs/profiles); prefer `*_copy(..., upsert=True)` |

---

## Scorecard

### Keep (MCP tools)

| Tool | Notes |
|------|-------|
| `get_instructions` | Call once at session start |
| `buddy_info` / `docs` / `jamf_schema` | Meta / discovery |
| `buddyscript` | Primary surface |

### Cut from MCP (still via `buddyscript`)

Former typed tools: Pro smart/static groups, profiles/payloads helpers, policies, categories/scripts clone-copy-migrate, list stubs (`computers`, `packages`, …), `package_ready` / `package_wait_ready` / `package_download`, all Platform `blueprint*` / `device*` / `ddm_*` / `benchmark*`.

### Do not add

- First-class MCP `jamf_get`/`jamf_post`/…
- Restoring dozens of typed profile/group tools (pathfinding is docs + named builtins)
