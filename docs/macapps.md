# Mac App Store apps (Classic)

Jamf **Mac App Store Apps** objects live on Classic `/JSSResource/macapplications`. There is no Pro JSON catalog CRUD for the same resource. Prefer an instance-surface auth (Classic privileges required).

Bare `buddy app` / `apps(...)` manage **policy apps** (Self Service + packages) — see [apps.md](apps.md). Do not use those names for Mac App Store records.

## Happy path (preferred)

1. **List:** `macapps(filter?)` or `buddy macapp list [--filter=…]` — client-side substring on name / version / bundle_id (Classic list has no RSQL).
2. **Load:** `macapp(id=…)` or `macapp(name=…)` / `buddy macapp get <macapp-id>`.
3. **Create:** `macapp_create(name, version, bundle_id, url, free?=False, category?)` or CLI `buddy macapp create --name= --version= --bundle-id= --url= [--free] [--category=<name>]`.
4. **Delete:** `macapp_delete(macapp_id)` / `buddy macapp delete <macapp-id> […]`.

Create requires `name`, `version`, `bundle_id`, and App Store `url`. Optional `category` is an exact category name. Full replace (`set`) / copy-migrate are not in this slice.

```python
def main():
    rows = macapps(filter="TextWrangler")
    if not rows:
        return macapp_create(
            name="TextWrangler.app",
            version="5.5.2",
            bundle_id="com.barebones.textwrangler",
            url="https://itunes.apple.com/app/id404010395",
            free=True,
        )
    return macapp(id=rows[0]["id"])
```

## Do not

- Invent Pro “Mac App Store” or “Jamf Apps” list/create paths — they are not exposed for this object.
- Confuse this with VPP locations/content (`/api/v1/volume-purchasing-locations`) or `jamf-package` (Protect/Connect only).
- Use `app` / `apps` for policy apps (policy + package), not for MAS records — see [apps.md](apps.md).

## Related

- [Categories](categories.md) — optional create `--category`
- [API endpoints](api-endpoints.md) — Mac App Store apps (Classic)
- Full signatures: [Function index](/docs/builtins)
