# API roles

Jamf Pro API roles — list, get, create, put, and privilege catalog. Used when bootstrapping least-privilege API clients (`buddy setup` flows).

## Happy path (preferred)

1. **List / load:** `roles(...)` / `role(id=…)`.
2. **Create / update:** `role_create` / `role_put` (and privilege helpers listed in builtins).
3. **Setup:** `buddy setup write-script` → `setup_buddy_role.buddy` creates a role + client idempotently; then drop privileges you do not need.

CLI: `buddy role …`.

## Do not

- Grant destructive MDM privileges to day-to-day automation clients without review.
- Confuse API roles with Jamf account UI roles — this is the Pro API roles API.

```python
def main():
    return roles(filter='displayName=="Buddy"')
```

## Related

- [API clients](api-clients.md) — integrations that bind to roles
- [Getting started](index.md) — setup path
- [API endpoints](api-endpoints.md) — API Roles
- Full signatures: [Function index](/docs/builtins)
