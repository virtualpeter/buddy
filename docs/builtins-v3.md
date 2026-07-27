# Starlark builtins grammar + v3→v4 migration

Buddy Starlark uses **plural nouns** to enumerate, **singular nouns** to load one resource, and **singular_verb** to mutate. Result dicts remain `{"ok", "value"|"error"}`.

As of **v4**, the old `*_list` / `*_fetch` / `*_get_*` aliases are **removed** (not registered). Use only the names in the **New** column below. This page is a migration reference, not a list of still-working aliases.

## Grammar

| Form | Role | Example |
|------|------|---------|
| plural noun | enumerate / search | `policies(...)`, `packages(...)`, `computers(...)` |
| singular noun | load one | `policy(id=…)` / `policy(trigger=…)`, `package(id=…)` |
| singular_verb | mutate / action | `policy_set(...)`, `computer_reboot(...)` |

Plural `value` is a **list of summary dicts** (at least `id`, plus `name` when available). Optional `ids=True` on `policies` / `computers` returns a list of ints.

Singular load accepts **either** `id=` **or** a natural key (`trigger=`, `name=`, `app=`+`version=`), not both.

`policies()` with no keywords returns every Classic policy summary.

## Migration map (old names removed in v4)

### Policy

| Old (removed) | Use instead |
|---------------|-------------|
| `policy_find_ids` | `policies(…, ids=True)` |
| `policy_get_id` | `policy(trigger=…)` → details dict (includes id); was a bare int |
| `policy_get_details` | `policy(id=…)` |
| `policy_get_enable` | `policy(id=…)["value"]["enabled"]` (bool; old returned `"true"`/`"false"` strings) |
| `policy_fetch_xml` | `policy_xml(id=…)` |
| `policy_set_enable` / `policy_set_disable` / `policy_set_name` / `policy_set_trigger` / `policy_set_frequency` | `policy_set(policy_id, enable?=…, name?=…, trigger?=…, frequency?=…)` |
| `policy_set_ss`, `policy_set_package`, `policy_set_install`, `policy_set_scope_all` | unchanged (fat helpers) |
| `policy_clone`, `policy_create_ss`, `policy_delete`, `policy_template` | unchanged |

### Profile

| Old (removed) | Use instead |
|---------------|-------------|
| `profile_list` | `profiles(filter?=…)` |
| `profile_find_id` | `profile(name=…)` → `{id, name}` |
| `profile_fetch_xml` | `profile_xml(id=…)` or `profile(id=…)` |
| payload helpers | unchanged |

### Groups

| Old (removed) | Use instead |
|---------------|-------------|
| `group_list_smart` | `smart_groups(…)` |
| `group_list_static` | `static_groups(...)` |
| `group_find_id` / `group_fetch` / `smart_group_fetch` | `smart_group(id=…)` or `smart_group(name=…)` (full JSON; `group_find_id` was a summary) |
| `group_put` | `smart_group_put` (PUT only; create with `smart_group_create`, not `group_id="-1"`) |
| `group_get_version` / `group_set_version` | `smart_group_get_version` / `smart_group_set_version` |
| `group_delete` | unchanged |

### Package / script

| Old (removed) | Use instead |
|---------------|-------------|
| `package_list` | `packages(…)` |
| `package_find_id` | `package(app=…, version=…)` |
| `package_fetch` | `package(id=…)` |
| `script_list` | `scripts(...)` |
| `script_get_id` | `script(name=…)` |
| `script_fetch` | `script(id=…)` |

### Computer / MDM

| Old (removed) | Use instead |
|---------------|-------------|
| `computer_find_ids` | `computers(…, ids=True)` |
| `computer_get` / `computer_fetch` | `computer(id=…)` |
| `mdm_command_find` / `mdm_fetch_commands` | `mdm_commands(...)` |

### Role / client

| Old (removed) | Use instead |
|---------------|-------------|
| `role_list` / `role_fetch_list` | `roles(full?=False, …)` |
| `role_fetch` | `role(id=…)` |
| `role_privileges_list` | `role_privileges(...)` |
| `client_fetch_list` | `clients(...)` |
| `client_fetch` | `client(id=…)` |

### Unchanged families

`rsql_*`, `plist_*`, `payloads_*`, `file_*`, `json_*`, `xml_*`, most `step_*`, computer MDM action verbs, Platform blueprints/devices/benchmarks, and fat policy helpers.

## CLI renames (v4)

| Removed | Use instead |
|---------|-------------|
| `--cred=` | `--user=` / `--auth=` |
| `buddy config` | `buddy cred` |
| `buddy setup configure-oauth` | `buddy setup store-oauth` |

## Discoverability

- Notebook Libraries drawer: **Getting started**, resource sections, **Advanced** (`step_*`, fat helpers, offline transforms).
- Evaluate a bare builtin name (no `()`) in the notebook/shell to print signature / returns / params.
- Prefer `change_batch` + `plan_change` + `run_change` for multi-step applies (see [change-batches.md](change-batches.md)).
