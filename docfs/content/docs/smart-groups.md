# Smart computer groups

Pro v3 smart computer groups — dynamic membership via **criteria**. Do not use `static_group*` or Platform `device_group*` for this.

## Happy path (preferred)

1. **List:** `smart_groups(search=…)` — summary list.
2. **Load:** `smart_group(id=…)` **or** `smart_group(name=…)` (not both) — full JSON document.
3. **Criteria:** `smart_group_get_criteria(group_id)` → list of criterion dicts; `smart_group_set_criteria` / `smart_group_find_criteria` / `smart_group_update_criterion_value` for targeted edits.
4. **Create / replace:** `smart_group_create(body)` / `smart_group_put(group_id, body)`.
5. **Copy / migrate:** `smart_group_copy` / `smart_group_migrate` (upsert by exact name); clears `siteId` unless `site_id` set.

CLI: `buddy group …` (smart family).

## Criterion bump example

```python
def main():
    smart_group_update_criterion_value(
        group_id="123",
        name_substring="version",
        new_value="2.0.0",
    )
    return "ok"
```

## Do not

- Mix with `static_group*` (explicit computer IDs) or Platform `device_group*`.
- Invent criteria PUT bodies from `jamf_schema` alone — use the criteria helpers.
- Assume Classic computergroup XML is the write path for smart groups (buddy uses `/api/v3/computer-groups/smart-groups`).

## Related

- [Static computer groups](static-groups.md) — membership by computer ID
- [Platform device groups](device-groups.md) — Platform gateway only
- [API endpoints](api-endpoints.md) — Computer Groups section
- Full signatures: [Function index](/docs/builtins)
