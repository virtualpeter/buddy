# Policies

How to list, load, mutate, copy, and migrate Jamf Classic policies with buddy — without inventing Classic PUT bodies from schema alone.

## Happy path (preferred)

1. **List / find:** `policies(filter=…)` or `policies(ids=True)` — summary dicts `{id, name, …}`.
2. **Load one:** `policy(id=…)` **or** `policy(trigger=…)` (not both) — full Classic XML as a dict tree (or use `policy_xml(id=…)` for the raw string).
3. **Mutate:** prefer fat helpers (`policy_set`, `policy_set_ss`, `policy_set_package`, `policy_set_scope_*`, …) over hand-edited XML when they cover the change.
4. **Raw XML write:** `policy_put_xml(policy_id, xml_str)` when you must edit Classic XML directly.
5. **Versioned release bumps:** use [change batches](change-batches.md) (`step_clone_policy` / `step_set_package`) or compose `policy_copy` + set helpers — there is no public `policy_clone` builtin.

CLI: `buddy policy list|get|set|create|clone|find|delete`.

## Copy / migrate

- `policy_copy(policy_id, name?, trigger?, …, upsert?=False, tenant?, …, to_tenant?, to_auth?)` → `{id, name, trigger}` (+ `created` when upsert).
- Source session = `tenant`/`auth`/`surface`; destination = `to_*` (omit = same as source).
- Remaps category/scripts/packages by **name** on destination (missing scripts/packages **fail**; missing category → uncategorized). Default scope is **unassigned** unless `scope_group_ids` / `scope_all` set.
- `policy_migrate(...)` is an alias of `policy_copy(..., upsert=True)`.

Recommended migrate order: [categories](categories.md) → [scripts](scripts.md) → policies (and ensure [packages](packages.md) exist / are READY).

## Do not

- Invent Classic policy PUT/POST bodies from `jamf_schema` or Jamf MCP `execute-request` when a named builtin exists.
- Attach a package before it is JCDS **READY** — see [packages](packages.md).
- Hand-roll versioned release clones when [change batches](change-batches.md) already encode the workflow.

## Recommended builtins

| Prefer | Notes |
|--------|-------|
| `policies` / `policy` / `policy_xml` | List / load |
| `policy_set`, `policy_set_ss`, `policy_set_package`, … | Fat mutators |
| `policy_put_xml` / `policy_create` / `policy_delete` | XML / lifecycle |
| `policy_copy` / `policy_migrate` | Cross-session copy / upsert |

Full signatures: [Function index](/docs/builtins) (`buddy help builtins`). Endpoints: [api-endpoints.md](api-endpoints.md).

```python
def main():
    p = policy(trigger="install-example")
    policy_set(policy_id=p["general"]["id"], enable=True)
    return p["general"]["name"]
```
