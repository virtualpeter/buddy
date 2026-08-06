# Packages / JCDS

Jamf Pro packages and Jamf Cloud Distribution Service (JCDS) readiness. Package **existence** is not the same as **READY** for attach/download.

## Happy path (preferred)

1. **List / load:** `packages(filter=…)` / `package(id=…)` (or `name=` + `version=`). Bare `filter="Chrome"` → `packageName=="*Chrome*"`; full RSQL still works.
2. **After upload / before policy attach:** `package_wait_ready(id=…, timeout?=…)` — polls until `cloudTransferStatus == "READY"` **and** `sha256` is set (default ~10 minutes); best-effort JCDS refresh between polls.
3. **Check:** `package_ready(id=…)` → `true` only when the package exists, status is `"READY"`, and `sha256` is non-empty. HTTP 404 → `false`, not an error.
4. **Download (Jamf Cloud / JCDS only):** `package_download(id=…, path?=…)` — same READY + sha256 wait, streams via pre-signed URI, verifies hash, retries once on URI 403.

CLI: `buddy package list|get|set|upload|download|delete|…`.

## Folklore encoded in builtins

| Builtin | Behavior |
|---------|----------|
| `package_ready` | READY + sha256; 404 → `false` |
| `package_wait_ready` | Poll until READY and sha256 (default ~10 minutes) |
| `package_upload` | Skip replace when checksums match; if hashes are blank and status is not READY, report pending (do not re-upload) |
| `package_download` | Wait + hash-verify download; not for on-prem non-JCDS DPs |

Do **not** hand-roll: single GET “exists?”, treat blank `sha256` as “needs upload” while transfer is pending, raw JCDS URI save without hash, or policy attach before READY.

```python
def main():
    package_wait_ready(id="42")
    return package_ready(id="42")
```

## Related

- [Policies](policies.md) — attach packages only after READY
- [API endpoints](api-endpoints.md) — Packages + JCDS
- [Change batches](change-batches.md) — versioned package bumps
- Full signatures: [Function index](/docs/builtins)
