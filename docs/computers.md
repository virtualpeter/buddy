# Computers

Jamf Pro computer inventory (v3) and MDM **actions** (reboot, lock, erase, …). For MDM **command history** queries, see [mdm](mdm.md). Platform inventory is [devices](devices.md) — different API surface.

## Happy path (preferred)

1. **List / find:** `computers(filter=…, page?, page_size?)` — omit `page` to fetch all pages. Stay on **inventory v3** (do not invent v4 paths until production supports them).
2. **Load one:** `computer(id=…)` (and detail-slice helpers where listed in builtins).
3. **MDM actions:** `computer_reboot`, `computer_lock`, … — prefer named builtins over raw MDM POSTs.

CLI: `buddy computer …`.

## Do not

- Mix Pro `computer*` with Platform `device*` — different gateways and IDs.
- Use this page for RSQL history of MDM commands — that is [mdm](mdm.md).
- Jump to computers-inventory **v4** from schema alone; buddy and production stay on v3 for now (see [api-endpoints](api-endpoints.md) / AGENTS).

```python
def main():
    rows = computers(filter='general.name=="*MacBook*"', page_size=50)
    return rows
```

## Related

- [MDM command history](mdm.md)
- [Static computer groups](static-groups.md) — membership by computer ID
- [Platform devices](devices.md)
- Full signatures: [Function index](/docs/builtins)
