# Star script examples

These `.star` files are [Starlark](https://github.com/google/starlark-go) scripts run with:

```sh
buddy path/to/script.star [-- arguments passed to the script]
```

Arguments after `--` appear in the global `args` list inside the script.

## Included examples

| File | Purpose |
|------|---------|
| [bump-release.star](bump-release.star) | Illustrative workflow: resolve a policy by trigger, then update package install params (adjust IDs and names to your Jamf instance). |

## Full documentation

- [Reference manual](../docs/reference-manual.md) — configuration, CLI, scripting, and links to the API endpoint catalog.
- Canonical **built-in function** list (always matches your installed binary): `buddy help builtins`
- Starlark language subset: `buddy help starlark`

## Advanced / integration testing

The repository also contains [scripts/integration_test.star](../scripts/integration_test.star), a broad integration harness for Jamf test tenants (creates and tears down test policies and groups). It is not a minimal example; read and adapt carefully before running against production.
