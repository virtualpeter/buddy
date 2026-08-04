# Categories

Jamf Pro categories — mostly Starlark-first (no fat CLI surface). Used heavily when copying scripts/policies across tenants.

## Happy path (preferred)

1. **List / load:** `categories(...)` / `category(id=…)` or by name where supported.
2. **Copy / migrate:** `category_copy(..., upsert=True)` / `category_migrate` — Pro `{name, priority}` body; upsert by exact name.
3. **Migrate order:** categories → [scripts](scripts.md) → [policies](policies.md) / [profiles](profiles.md). Missing categories on policy copy become **uncategorized**; prefer creating them first.

## Do not

- Skip category copy when migrating policies that should keep category assignment.
- Invent category POST/PUT from schema when `category_copy` exists.

```python
def main():
    return category_copy(category_id="5", upsert=True, to_tenant="sandbox")
```

## Related

- [Scripts](scripts.md), [Policies](policies.md), [Profiles](profiles.md)
- [API endpoints](api-endpoints.md) — Categories
- Full signatures: [Starlark builtins](/docs/builtins)
