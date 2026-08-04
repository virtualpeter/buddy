# Blueprints

Platform API Gateway blueprints — CRUD, deploy/undeploy/status, components. Requires a **platform-capable** tenant + auth (`region`/`url` + `tenantId`).

## Happy path (preferred)

1. **List / load:** `blueprints(search=…)` / `blueprint(id=…)`.
2. **Create / patch:** `blueprint_create(body)` / `blueprint_set(blueprint_id, body)` — `blueprint_set` is a merge-patch.
3. **Lifecycle:** deploy / undeploy / status helpers listed in builtins (`blueprint_deploy`, …).
4. CLI: `buddy blueprint …`.

Mutating Platform calls in MCP skip interactive confirm (host tool approval is the guard). Readonly auths are rejected for mutates.

## Do not

- Call blueprint builtins with an instance-only session — you need Platform surface credentials.
- Invent deploy payloads from `jamf_schema` when named builtins exist.
- Confuse with Classic/Pro policies or configuration profiles.

```python
def main():
    rows = blueprints(search="example")
    return rows
```

## Related

- [Platform devices](devices.md), [device groups](device-groups.md)
- [API endpoints](api-endpoints.md) — Blueprints
- [MCP](mcp.md) — Platform builtin notes
- Full signatures: [Function index](/docs/builtins)
