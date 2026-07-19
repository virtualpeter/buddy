# Script examples

These `.buddy` files are [Starlark](https://github.com/google/starlark-go) scripts run with:

```sh
./path/to/script.buddy [-- arguments passed to the script]
buddy path/to/script.buddy [-- arguments passed to the script]
```

User scripts start with `#!/usr/bin/env buddy` and are executable. Embedded stdlib modules omit the shebang (loaded via `load("stdlib/….buddy", …)`, not run as programs). Both use **`.buddy`** for the editor language id. Buddy still runs any existing file path (including legacy `.star` scripts).

Arguments after `--` appear in the global `args` list inside the script.

## Included examples

| File | Purpose |
|------|---------|
| [bump-release.buddy](bump-release.buddy) | Illustrative workflow: resolve a policy by trigger, then update package install params (adjust IDs and names to your Jamf instance). |
| [mcp.json](mcp.json) | Example Cursor MCP config for `buddy mcp` (use absolute paths; see [docs/mcp.md](../docs/mcp.md)). |

## Full documentation

- [Reference manual](../docs/reference-manual.md) — configuration, CLI, scripting, and links to the API endpoint catalog.
- [Starlark builtins v3 rename map](../docs/builtins-v3.md) — plural/singular grammar and old→new names.
- Canonical **built-in function** list (always matches your installed binary): `buddy help builtins`
- Starlark language subset: `buddy help starlark`

## Advanced / integration testing

The repository also contains [scripts/integration_test.buddy](../scripts/integration_test.buddy), a broad integration harness for Jamf test tenants (creates and tears down test policies and groups). It is not a minimal example; read and adapt carefully before running against production.
