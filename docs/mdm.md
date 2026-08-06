# MDM command history

Query Jamf Pro MDM **command history** with RSQL filters. Device **actions** (reboot, lock, erase) live on [computers](computers.md) (Pro) or [devices](devices.md) (Platform).

## Happy path (preferred)

1. **List / find:** `mdm_commands(filter=…)` — bare term → `status=="term"`, or full RSQL (status, clientManagementId, dates, …).
2. CLI: `buddy mdm list --filter=…` (e.g. `--filter=Pending`).

Build filters with offline `rsql_*` helpers when useful (see builtins / [api-endpoints](api-endpoints.md) RSQL section).

## Do not

- Treat this as the way to **issue** MDM actions — use `computer_*` / Platform device action builtins.
- Invent date filters without RSQL range operators (see api-endpoints MDM + RSQL notes).

```python
def main():
    return mdm_commands(filter="Pending")
```

## Related

- [Computers](computers.md) — issue actions
- [API endpoints](api-endpoints.md) — MDM Commands + RSQL
- Full signatures: [Function index](/docs/builtins)
