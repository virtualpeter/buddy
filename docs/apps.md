# Policy apps

**Policy apps** are not a Jamf object type. They are a convention: Self Service + packages, discovered and upgraded as one unit via `buddy app` / `apps(...)`.

Classic **Mac App Store** records are separate — use [`macapp` / `macapps`](macapps.md).

## Patterns

| Kind | Shape | Upgrade |
|------|--------|---------|
| **single** | One Self Service policy with the Install package attached (often Ongoing) | In-place package swap + SS description version |
| **install** | SS script (param4) calls `install-{app}`, `install-{app}-next`, or `{app}-install`; one enabled install policy holds the package | Create replacement install policy, disable prior |
| **cache** | Same as install, plus versioned `cache-{app}-{version}` policies | Keep old cache; new cache + replace install; optional SS description bump |

Discovery starts from policies whose **name contains `[SS]`**.

- **`buddy app list` / `apps()`** — one hybrid policy list call (undocumented Pro `GET /api/v1/policies` when available, else Classic `{id,name}`), then **name-only** heuristics for kind/version/trigger. Rows need an `Install…[SS]` name and/or install/cache sibling policies for the app slug (inventory/config `[SS]` stubs are omitted). Approximate; `app get` is authoritative.
- **`buddy app get` / `app()`** — authoritative Classic XML classify (Self Service flag, scripts, packages, triggers). Excludes non-apps (no package and no install-like script / inferred install trigger).

Jamf does **not** publish a Pro policy CRUD API; RSQL on `/api/v1/policies` cannot filter by Self Service (only `id`, `name`, `enabled`, `categoryId`, `executionFrequency`, `triggerCheckin`, `triggerManual`).

## Happy path

1. **List:** `apps(filter?)` or `buddy app list [--filter=…]`
2. **Get:** `app(name=…)` or `buddy app get <name>`
3. **Upgrade:** `app_upgrade(name, version, package_pattern?)` or `buddy app upgrade <name> --version=… [--package-pattern=…]`

```python
def main():
    rows = apps(filter="santa")
    if not rows:
        return None
    detail = app(name=rows[0]["name"])
    # After the package is READY in Jamf:
    # app_upgrade(name="santa", version="2026.3", package_pattern="*santa*2026.3*")
    return detail
```

```sh
buddy app list
buddy app get santa
buddy app upgrade santa --version=2026.3 --package-pattern='*santa*2026.3*'
```

Greenfield setup (prefs + PPPC + cache + install + optional SS) remains `setup_app_classic` in `stdlib/app_setup.buddy` / change batches — not `app create`.

## Related

- [Policies](policies.md) — `policy_set_package`, `policy_set_ss`, change batches
- [Packages / JCDS](packages.md) — wait READY before upgrade
- [Mac App Store apps](macapps.md) — Classic MAS, not policy apps
- [Change batches](change-batches.md) — Santa-style multi-step upgrades
- [API endpoints](api-endpoints.md) — Classic policy paths; hybrid list notes
