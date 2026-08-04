# DDM reporting

Platform Declarative Device Management (DDM) **declaration reporting** — read-oriented devices / declarations / channels. Requires platform-capable tenant + auth.

## Happy path (preferred)

1. Use `ddm_*` builtins (see catalog) for report queries.
2. CLI: `buddy ddm …`.

## Do not

- Treat DDM reporting as a substitute for blueprint deploy or configuration profile push.
- Invent report paths from schema when `ddm_*` helpers exist.

```python
def main():
    return ddm_declaration_devices()
```

Exact names: `ddm_declaration_devices`, `ddm_device_declarations`, `ddm_device_channels` — see [Function index](/docs/builtins).

## Related

- [Blueprints](blueprints.md)
- [API endpoints](api-endpoints.md) — DDM declaration reporting
- Full signatures: [Function index](/docs/builtins)
