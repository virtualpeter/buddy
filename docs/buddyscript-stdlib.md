# BuddyScript stdlib

`load("stdlib/<name>.buddy", …)` is reserved for shared modules shipped with the binary. **There are no modules yet.**

Compose [builtins](/docs/builtins) in your own `.buddy` files (bare returns; failures raise). Copy a recipe from [`examples/`](../examples/) when you want a starting point:

| Example | Role |
|---------|------|
| [`examples/bump-release.buddy`](../examples/bump-release.buddy) | Resolve a policy by trigger, then set package / install / Self Service (optional smart-group version) |
| [`examples/setup-app-classic.buddy`](../examples/setup-app-classic.buddy) | Build `step_*` dicts (prefs + PPPC profiles, cache/install, optional SS) for `plan_change` / `run_change` |
| [`examples/classic-platform-route.buddy`](../examples/classic-platform-route.buddy) | GET-only Classic-on-Platform routing smoke (gateway `/api/proclassic/` vs instance ACS / policy list) |

```python
load("re", "match")  # starlib regex; also predeclared as re
```

Promote a pattern into `stdlib/` only when it is shared enough to ship with the binary. Until then, keep workflows in `examples/` or your own scripts.

Full Jamf primitive API: [Function index](/docs/builtins).

Next: [Syntax](buddyscript-syntax.md) · [Overview](buddyscript.md) · [Function index](/docs/builtins)
