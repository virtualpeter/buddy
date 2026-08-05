# BuddyScript stdlib

Multi-step helpers live as embedded **`.buddy`** modules. Load them like `re`:

```python
load("stdlib/release.buddy", "bump_release")

def main():
    return bump_release(
        trigger_prefix = "install-app-",
        name = "app",
        version = "1.2.3",
    )
```

Modules compose [builtins](/docs/builtins) (bare returns; failures raise). Browse the same catalog from the notebook **Libraries** drawer.

## Modules

### `stdlib/release.buddy`

Release / version bump helpers.

| Symbol | Role |
|--------|------|
| `bump_release` | Resolve policy by trigger, set package / install / self-service, optional smart-group version |

Returns e.g. `{"policy_id": int}`.

### `stdlib/app_setup.buddy`

Classic greenfield “new app” step builders for change batches (does **not** write to Jamf by itself).

| Symbol | Role |
|--------|------|
| `setup_app_classic` | Build a list of `step_*` dicts (prefs + PPPC profiles, cache/install policies, optional SS) for `plan_change` / `run_change` |

See [Change batches](change-batches.md) and [`examples/setup-app-classic.buddy`](../examples/setup-app-classic.buddy).

## Tips

- Prefer `load("stdlib/<name>.buddy", "symbol")` — same paths the notebook inserts.
- Keep fat one-off workflows in your own `.buddy` files; promote repeated patterns into stdlib only when shared.
- Full Jamf primitive API: [Function index](/docs/builtins).

Next: [Syntax](buddyscript-syntax.md) · [Overview](buddyscript.md) · [Function index](/docs/builtins)
