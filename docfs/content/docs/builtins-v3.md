# Starlark builtins v3 (plural / singular)

Breaking redesign: **plural nouns enumerate**, **singular nouns load one resource**, **singular_verb** mutates. Result dicts remain `{"ok", "value"|"error"}`.

Temporary **deprecated aliases** warn once per process on stderr (`old_name → new_name`), then call the new implementation. Aliases will be removed in a later minor release.

## Grammar

| Form | Role | Example |
|------|------|---------|
| plural noun | enumerate / search | `policies(...)`, `packages(...)`, `computers(...)` |
| singular noun | load one | `policy(id=…)` / `policy(trigger=…)`, `package(id=…)` |
| singular_verb | mutate / action | `policy_set(...)`, `policy_enable(...)`, `computer_reboot(...)` |

Plural `value` is a **list of summary dicts** (at least `id`, plus `name` when available). Use `[x["id"] for x in r["value"]]` when you need bare IDs. Optional `ids=True` on `policies` / `computers` returns `list` of ints for migration ease.

Singular load accepts **either** `id=` **or** a natural key (`trigger=`, `name=`, `app=`+`version=`), not both.

## Rename map

### Policy

| Old | New |
|-----|-----|
| `policy_find_ids` | `policies(keywords=…, ids?=False, …)` |
| `policy_get_id` | `policy(trigger=…)` → details (includes id) |
| `policy_get_details` | `policy(id=…)` |
| `policy_get_enable` | use `policy(id=…)["value"]["enabled"]` (alias kept) |
| `policy_fetch_xml` | `policy_xml(id=…)` |
| `policy_put_xml` | `policy_put_xml` (unchanged) |
| `policy_set_enable` / `policy_set_disable` / `policy_set_name` / `policy_set_trigger` / `policy_set_frequency` | `policy_set(policy_id, enable?=…, name?=…, trigger?=…, frequency?=…)` |
| `policy_set_ss`, `policy_set_package`, `policy_set_install`, `policy_set_scope_all` | unchanged (fat helpers) |
| `policy_clone`, `policy_create_ss`, `policy_delete`, `policy_template` | unchanged |

### Profile

| Old | New |
|-----|-----|
| `profile_list` | `profiles(filter?=…)` |
| `profile_find_id` | `profile(name=…)` → `{id, name}` |
| `profile_fetch_xml` | `profile_xml(id=…)` or `profile(id=…)` for XML string when id given |
| `profile_get_payloads` etc. | unchanged offline/online payload helpers |

Singular `profile(id=…)` returns XML string (same as today’s fetch). `profile(name=…)` returns `{id, name}`.

### Groups

| Old | New |
|-----|-----|
| `group_list_smart` | `smart_groups(filter?=…, page?=…, page_size?=…)` |
| `group_list_static` | `static_groups(...)` |
| `group_find_id` / `group_fetch` / `smart_group_fetch` | `smart_group(id=…)` or `smart_group(name=…)` |
| `group_put` | prefer `smart_group_put` (alias) |
| `group_get_version` / `group_set_version` | `smart_group_get_version` / `smart_group_set_version` (aliases from old) |
| `group_delete` | unchanged |
| other `smart_group_*` / `static_group_*` | unchanged |

### Package / script

| Old | New |
|-----|-----|
| `package_list` | `packages(filter?=…, page?=…)` |
| `package_find_id` | `package(app=…, version=…, pattern?=…)` → `{id, name}` |
| `package_fetch` | `package(id=…)` → full dict |
| `script_list` | `scripts(...)` |
| `script_get_id` | `script(name=…)` → id string |
| `script_fetch` | `script(id=…)` → full dict |
| `script_get_contents` | unchanged |

### Computer / MDM

| Old | New |
|-----|-----|
| `computer_find_ids` | `computers(filter?=…, ids?=False, …)` |
| `computer_get` / `computer_fetch` | `computer(id=…)` |
| `mdm_command_find` / `mdm_fetch_commands` | `mdm_commands(...)` |

### Role / client

| Old | New |
|-----|-----|
| `role_list` / `role_fetch_list` | `roles(full?=False, page?=…)` |
| `role_fetch` | `role(id=…)` |
| `role_privileges_list` | `role_privileges(...)` |
| `client_fetch_list` | `clients(...)` |
| `client_fetch` | `client(id=…)` |

### Blueprints (Platform API Gateway)

New in the Platform API Gateway surface — these are additions, not renames. They
require a **gateway credential** (`region` like `us`, or a `url` such as
`https://us.apigw.jamf.com`, plus `tenantId`). Same `{"ok", "value"|"error"}` shape.

| Builtin | Role |
|---------|------|
| `blueprints(search?, page?, page_size?, cred?)` | enumerate → list of `{id, name, description?, …}` (omit `page` = all) |
| `blueprint(id=, cred?)` | load one → full blueprint dict |
| `blueprint_create(body, cred?)` | create → response dict |
| `blueprint_set(blueprint_id, body, cred?)` | merge-patch update → None |
| `blueprint_delete(blueprint_id, cred?)` | delete → None |
| `blueprint_deploy(blueprint_id, cred?)` | deploy → None |
| `blueprint_undeploy(blueprint_id, cred?)` | undeploy → None |
| `blueprint_status(blueprint_id, cred?)` | deployment report → dict |
| `blueprint_components(page?, page_size?, cred?)` | enumerate components → list[dict] |
| `blueprint_component(id=, cred?)` | load one component → dict |

Follows the same conventions: resource IDs first on mutate/action helpers, `cred?`
last. The CLI mirror is `buddy blueprint <subcommand>` (see `buddy blueprint help`).

### Unchanged families

`rsql_*`, `plist_*`, `payloads_*`, `file_*`, `json_*`, `xml_*`, most `step_*`, computer MDM action verbs (`computer_reboot`, …).

## Alias removal

Deprecated aliases emit one stderr warning per process and will be **removed in a later minor release**. After removal, a checklist test should fail if any forbidden old name remains in `makeBuiltins`. Migrate scripts using this document before upgrading past that release.
