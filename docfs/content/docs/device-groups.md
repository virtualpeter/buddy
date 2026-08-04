# Platform device groups

Platform API Gateway device groups and membership. **Not** Pro [smart](smart-groups.md) or [static](static-groups.md) computer groups.

## Happy path (preferred)

1. **List / load:** `device_groups(...)` / `device_group(id=…)`.
2. **Membership / mutate:** `device_group*` helpers in the builtins catalog.
3. CLI: `buddy device-group …`.

Requires platform-capable tenant + auth.

## Do not

- Use `smart_group*` / `static_group*` for Platform device groups (or the reverse).
- Invent membership PUT bodies from schema when named builtins exist.

```python
def main():
    return device_groups()
```

## Related

- [Smart computer groups](smart-groups.md), [Static computer groups](static-groups.md)
- [Platform devices](devices.md)
- [API endpoints](api-endpoints.md) — Platform device groups
- Full signatures: [Function index](/docs/builtins)
