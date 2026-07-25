# Buddy documentation

**buddy** is a Jamf Pro CLI: policies, groups, profiles, packages, scripts, computers/MDM, and Starlark automation.

Use the sidebar to browse. Start with the [reference manual](/docs/docs/reference-manual) for configuration and commands, and [Starlark builtins](/docs/builtins) for the scripting API.

| Guide | What it’s for |
|-------|----------------|
| [Reference manual](/docs/docs/reference-manual) | Install, config, credentials, CLI, Starlark, environment |
| [Starlark builtins](/docs/builtins) | Every built-in function (signatures, params, return values) |
| [API endpoints](/docs/docs/api-endpoints) | Jamf HTTP APIs buddy calls |
| [MCP for Cursor](/docs/docs/mcp) | Use buddy as an MCP server from Cursor |
| [Notebook](/notebook) | Interactive Starlark cells against your Jamf tenant/auth |
| [Script IDE](/ide) | Single-buffer runner (open/save, args, trace, Run as file) |

```sh
buddy serve --open          # docs UI + notebook + /ide
buddy help                  # CLI overview
buddy help builtins         # same builtins reference as above
```
