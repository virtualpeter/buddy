# API clients

Jamf Pro API integrations (OAuth clients) — list, get, create, put. Pair with [API roles](api-roles.md) and `buddy setup store-oauth` for instance-surface oauth auths.

## Happy path (preferred)

1. **List / load:** `clients(...)` / `client(id=…)`.
2. **Create / update:** `client_create` / `client_put` (see builtins).
3. **Store secret in buddy:** `buddy setup store-oauth` rotates a Jamf Pro *instance* API client secret into a buddy oauth auth (`surface=instance`). Platform gateway oauth cannot manage API roles/integrations.

CLI: `buddy client …`.

## Do not

- Put production client secrets only in `config.yaml` when the platform secret store exists.
- Use Platform gateway oauth expecting Classic/Pro role management — use an instance-surface auth for that.

```python
def main():
    return clients(filter="buddy-cli")
```

## Related

- [API roles](api-roles.md)
- [Getting started](index.md) / reference manual — credentials
- [API endpoints](api-endpoints.md) — API Integrations
- Full signatures: [Function index](/docs/builtins)
