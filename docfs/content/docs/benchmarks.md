# Compliance benchmarks

Platform compliance benchmarks (beta) — baselines, catalog rules, reports. Requires platform-capable tenant + auth.

## Happy path (preferred)

1. **List / load / mutate:** `benchmark*` builtins (CRUD-ish, baselines, reports — see catalog).
2. CLI: `buddy benchmark …`.

Treat as beta: prefer named builtins; confirm behavior on your tenant before automation at scale.

## Do not

- Invent benchmark HTTP from schema alone when builtins exist.
- Confuse with Pro smart-group “compliance” criteria or MDM commands.

```python
def main():
    return benchmarks()
```

## Related

- [API endpoints](api-endpoints.md) — Compliance benchmarks
- [MCP](mcp.md) — Platform notes
- Full signatures: [Starlark builtins](/docs/builtins)
