# Scripts

Jamf Pro scripts — list summaries vs full body, create/update, and copy/migrate.

## Happy path (preferred)

1. **List:** `scripts(filter=…)` → summaries `{id, name, categoryId, categoryName}` only. Bodies are stripped even if Jamf list JSON includes `scriptContents`.
2. **Full record / body:** `script(id=…)` or `script_get_contents(script_id=…)`.
3. **Create / update:** `script_create` / `script_set` / related put helpers (see builtins).
4. **Copy / migrate:** `script_copy` / `script_migrate` (`upsert=True`); category remapped via destination name resolution.

CLI: `buddy script get|set|…`.

## Do not

- Expect `scripts(...)` to return script bodies — use `script(id=…)` for contents.
- Migrate policies that reference scripts before those scripts exist on the destination (see migrate order under [policies](policies.md) / [categories](categories.md)).

```python
def main():
    rows = scripts(filter="Chrome")
    if not rows:
        return []
    return script(id=rows[0]["id"])
```

## Related

- [Categories](categories.md) — first step in cross-tenant migrate
- [Policies](policies.md) — scripts remapped by name on `policy_copy`
- [API endpoints](api-endpoints.md)
- Full signatures: [Function index](/docs/builtins)
