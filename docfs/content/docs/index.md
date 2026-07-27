# Buddy documentation

**buddy** is a Jamf Pro CLI: policies, groups, profiles, packages, scripts, computers/MDM, and Starlark automation.

Use the sidebar to browse. Start with the [reference manual](/docs/docs/reference-manual) for configuration and commands, and [Starlark builtins](/docs/builtins) for the scripting API.

| Guide | What it’s for |
|-------|----------------|
| [Reference manual](/docs/docs/reference-manual) | Install, config, credentials, CLI, Starlark, environment |
| [Change batches](/docs/docs/change-batches) | Plan → approve → apply workflows for CAB/change records |
| [Starlark builtins](/docs/builtins) | Every built-in function (signatures, params, return values) |
| [Builtins grammar + v4 migration](/docs/docs/builtins-v3) | Plural/singular grammar; removed alias map |
| [API endpoints](/docs/docs/api-endpoints) | Jamf HTTP APIs buddy calls |
| [MCP for AI tools](/docs/docs/mcp) | Use buddy as an MCP server from Cursor, Claude Desktop, and other hosts |
| [Examples index](/docs/examples/README.md) | Runnable `.buddy` samples |
| [Notebook](/notebook) | Interactive Starlark cells against your Jamf tenant/auth |
| [Script IDE](/ide) | Single-buffer runner (open/save, args, trace, Run as file) |

```sh
buddy serve --open          # docs UI + notebook + /ide
buddy help                  # CLI overview
buddy help builtins         # same builtins reference as above
```
