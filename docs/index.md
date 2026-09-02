# Buddy documentation

| Document | Description |
|----------|-------------|
| [../AGENTS.md](../AGENTS.md) | Agent onboarding: auth architecture, secret storage, meliorate integration, project history. |
| [reference-manual.md](reference-manual.md) | Full reference: configuration, CLI, Starlark scripting, builtins overview, and links to examples. |
| [api-endpoints.md](api-endpoints.md) | Jamf Pro HTTP endpoints used by buddy (Classic + Jamf Pro API). |
| [builtins-v3.md](builtins-v3.md) | Starlark v3 plural/singular rename map and migration notes. |
| [mcp.md](mcp.md) | Cursor MCP setup, tools, response shape, and troubleshooting. |

**Browse locally:** `make docs-serve` or `buddy serve --open` (docs + Starlark notebook UI).

In the docs UI, open **[Starlark builtins](/docs/builtins)** for a browsable HTML reference of every builtin (sections, signatures, params). Same content as `buddy help builtins`.

**Examples:** see the [examples/](../examples/) directory for runnable `.star` scripts. For Cursor MCP, see [examples/mcp.json](../examples/mcp.json).
