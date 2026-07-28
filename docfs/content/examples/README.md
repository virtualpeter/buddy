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
| [change-batch-santa.buddy](change-batch-santa.buddy) | Change-record style batch: `plan_change` markdown (no predicted create IDs), then optional `run_change` with outcomes. Howto: [docs/change-batches.md](../docs/change-batches.md). |
| [setup-app-classic.buddy](setup-app-classic.buddy) | Classic greenfield new-app batch via `setup_app_classic` (prefs + PPPC profiles, cache/install). Howto: [docs/change-batches.md](../docs/change-batches.md#new-app-setup-classic). |
| [mcp.json](mcp.json) | Example MCP host config for `buddy mcp` (use absolute paths; or run `buddy mcp setup` — see [docs/mcp.md](../docs/mcp.md)). |

## Full documentation

- [Reference manual](../docs/reference-manual.md) — configuration, CLI, scripting, and links to the API endpoint catalog.
- Canonical **built-in function** list (always matches your installed binary): `buddy help builtins`
- Starlark language subset: `buddy help starlark`
