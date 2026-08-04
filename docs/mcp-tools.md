# MCP tools: agent guide and scorecard

How AI agents should use buddy MCP tools after the lean-surface cut.

## Tool priority (do this order)

1. **`docs`** — conventions and cookbooks before inventing (`query: "mcp-tools"`, `"profile-payloads"`, `"api-endpoints"`, `"builtins"`).
2. **`buddyscript`** with **named builtins** (`smart_group_*`, `policy_migrate`, `profile_xml_*`, `package_wait_ready`, `blueprint_*`, …) for Jamf work.
3. **`jamf_get` / `jamf_put` / …** via `buddyscript` only when no named builtin exists.

Do **not** invent Classic/Pro HTTP from `jamf_schema` when a named builtin already covers the job. Schema is for discovery; builtins encode Jamf quirks (pagination, READY polling, criteria vs membership, payload entity encoding, ACS).

Typed per-resource MCP tools (smart/static groups, profiles, policies, packages READY/download, Platform CRUD, list stubs) were **removed** to cut host schema token cost. The same capabilities remain as Starlark builtins — lean MCP did **not** remove ACS/profile quirk handling.

---

## Pathfinding map

| Task | Docs query first | Named builtins (via `buddyscript`) | Do not |
|------|------------------|--------------------------------------|--------|
| Edit profile / ACS prefs | `profile-payloads` | `profile_xml` → `profile_xml_patch_payload` → `profile_put_xml`; `profile_acs_bind_uuid` if needed | Classic PUT from schema / Jamf `execute-request` |
| Create/clone profile | `profile-payloads` | `profile_create` / `profile_clone` / `profile_copy` / `profile_create_xml` | Hand-edit UUIDs |
| Package after upload | `api-endpoints`, `mcp-tools` | `package_wait_ready` / `package_ready` / `package_download` | Attach/download before READY |
| Smart group criteria | `mcp-tools` | `smart_group*` / `smart_group_get_criteria` / `smart_group_update_criterion_value` | `static_group*` / invent criteria PUT |
| Static membership | `mcp-tools` | `static_group_list_computers` / add/remove/set | `smart_group*` |
| Unknown endpoint | `jamf_schema` then `builtins` | Prefer named builtin; else `jamf_get`/`jamf_put` | Schema as license to PUT profiles/groups |

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
| `buddy_info` | Session/channel without Jamf call |
| `docs` | Embedded conventions / cookbooks |
| `jamf_schema` | Endpoint discovery only |
| `buddyscript` | **Primary** automation (all Pro + Platform builtins) |

### Starlark return contract

- **Inside Starlark:** builtins return **bare values**; failures **raise**. No `{ok, value, error}` ceremony; do not use `unwrap` for correctness.
- **MCP tool boundary only:** `buddyscript` returns `{ok, value: {stdout, stderr, result, error}}`.
- **`unwrap`** is optional JSON-print for CLI/`jq` (legacy envelopes still peel; `ok=false` raises).

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

## Packages / JCDS (via buddyscript) — do not invent from schema

Jamf Cloud package availability is **not** “GET package returned 200.” After upload the record often exists while JCDS is still syncing (UI **Availability Pending**). Agents that attach the package to a policy or download too early produce client failures.

**Folklore encoded in builtins** (also CLI `buddy package …`; details in [api-endpoints.md](api-endpoints.md)):

| Builtin | Behavior |
|---------|----------|
| `package_ready(id=…)` | `true` only when the package exists **and** `cloudTransferStatus == "READY"` (case-sensitive). HTTP 404 → `false`, not an error. |
| `package_wait_ready(id=…, timeout?=…)` | Polls with backoff (default ~10 minutes). Between polls, best-effort `POST /api/v1/jcds/refresh-inventory?file-name=…` to nudge availability. Use **after upload / before policy attach**. |
| `package_download(id=…, path?=…)` | Waits until READY **and** non-empty `sha256` → fetches a JCDS pre-signed URI (short TTL) → streams to a temp file while hashing SHA-256 → renames only on match; deletes temp on failure; retries once on URI HTTP 403. **Jamf Cloud / JCDS only** — not on-prem non-JCDS DPs. Verify is always on. |

Do **not** hand-roll: single GET “exists?”, raw JCDS URI save without hash, or policy attach before READY.

```python
def main():
    package_wait_ready(id="42")
    return package_ready(id="42")
```

---

## Groups: pick the right family (via buddyscript)

| Need | Use | Not |
|------|-----|-----|
| Dynamic membership via **criteria** | `smart_groups` / `smart_group` / `smart_group_get_criteria` / `smart_group_update_criterion_value` | static_* / device_group* |
| Explicit **computer ID** membership | `static_groups` / `static_group_list_computers` / add/remove/set | smart_* |
| Platform gateway device groups | `device_groups` / `device_group*` | Pro smart/static computer groups |

**Smart group criterion bump:**

```python
def main():
    smart_group_update_criterion_value(
        group_id="123",
        name_substring="version",
        new_value="2.0.0",
    )
    return "ok"
```

**Static membership:** `static_group_list_computers` then `static_group_add_computers` / `_remove_computers` / `_set_computers`.

---

## Profiles / payloads (via buddyscript)

See **[profile-payloads.md](profile-payloads.md)**. ACS / entity-encoding / UUID remap still live in Go helpers + these builtins (lean MCP only removed typed tool wrappers):

- Create/clone/copy: `profile_create`, `profile_clone`, `profile_copy` (UUID remap; do not hand-edit UUIDs).
- Edit: `profile_xml` → `profile_xml_patch_payload` → `profile_put_xml`.
- Advanced extract/replace/MCX: `payloads_*` / `plist_*`.
- ACS seed: `profile_acs_bind_uuid` when legacy HTML scrape fails.

Do **not** invent a Classic PUT body from `jamf_schema` alone.

```python
def main():
    xml = profile_xml(id=42)
    xml = profile_xml_patch_payload(xml, "com.example", {"Key": "New"})
    profile_put_xml(profile_id=42, xml_str=xml)
    return "ok"
```

---

## Scripts list vs body

- `scripts(...)` → summaries `{id, name, categoryId, categoryName}` only (bodies stripped even if Jamf list JSON includes `scriptContents`).
- Full record / body: `script(id=…)` or `script_get_contents(script_id=…)`.

---

## Cross-tenant migrate order (via buddyscript)

1. `category_migrate`
2. `script_migrate` (and ensure packages exist by name — wait for READY with `package_wait_ready` when uploading)
3. `policy_copy` / `policy_migrate` / `profile_copy`

Missing script/package names on policy copy **fail**; missing categories become uncategorized. Scope stays unassigned unless `scope_group_ids` set.

---

## Scorecard

### Keep (MCP tools)

| Tool | Notes |
|------|-------|
| `buddy_info` / `docs` / `jamf_schema` | Meta / discovery |
| `buddyscript` | Primary surface |

### Cut from MCP (still via `buddyscript`)

Former typed tools: Pro smart/static groups, profiles/payloads helpers, policies, categories/scripts clone-copy-migrate, list stubs (`computers`, `packages`, …), `package_ready` / `package_wait_ready` / `package_download`, all Platform `blueprint*` / `device*` / `ddm_*` / `benchmark*`.

### Do not add

- First-class MCP `jamf_get`/`jamf_post`/…
- Restoring dozens of typed profile/group tools (pathfinding is docs + named builtins)
