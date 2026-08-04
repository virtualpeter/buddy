# Platform devices

Platform API Gateway device inventory and **actions** (check-in, restart, erase, …). This is **not** Pro [computers](computers.md) — different IDs, gateway, and builtins (`device*` vs `computer*`).

## Happy path (preferred)

1. **List / load:** `devices(...)` / `device(id=…)`.
2. **Actions:** named `device_*` action builtins (see builtins catalog) — prefer these over raw device-actions HTTP.
3. CLI: `buddy device …`.

Requires platform-capable tenant + auth.

## Do not

- Use `computer*` builtins against Platform device IDs (or the reverse).
- Issue actions via generic `jamf_put` when a `device_*` helper exists.
- Confuse with [MDM command history](mdm.md) on the Pro instance API.

```python
def main():
    return devices(page_size=50)
```

## Related

- [Computers](computers.md) — Pro inventory / MDM actions
- [Device groups](device-groups.md)
- [API endpoints](api-endpoints.md) — Platform devices + device-actions
- Full signatures: [Starlark builtins](/docs/builtins)
