# Static computer groups

Pro v3 static computer groups — membership is an explicit list of **computer IDs**. Do not use `smart_group*` or Platform `device_group*` for this.

## Happy path (preferred)

1. **List groups:** `static_groups(search=…)`.
2. **List members:** `static_group_list_computers(group_id, filter?)` → `[{id, name, serial}, …]` (`name` may be empty when the API returns only assignment IDs).
3. **Mutate membership:** `static_group_add_computers` / `static_group_remove_computers` / `static_group_set_computers` (read-modify-write; add/remove are no-ops when already satisfied).
4. **Create:** `static_group_create(body)` — `name` required; `assignments` optional list of computer id strings.

CLI: `buddy group …` (static family).

## Membership example

```python
def main():
    members = static_group_list_computers(group_id="10")
    static_group_add_computers(group_id="10", computer_ids=["42", "43"])
    return members
```

## Do not

- Use smart-group criteria APIs for static membership (or the reverse).
- Confuse with Platform [device groups](device-groups.md).
- Invent `/api/v3/computer-groups/static-groups/{id}` PUT bodies from schema when the membership helpers exist.

## Related

- [Smart computer groups](smart-groups.md) — criteria-based membership
- [Computers](computers.md) — inventory IDs used in assignments
- [API endpoints](api-endpoints.md)
- Full signatures: [Starlark builtins](/docs/builtins)
