# BuddyScript syntax

BuddyScript is **Starlark** plus buddy builtins and `load()` modules. Most Python that avoids classes, imports, generators, and `try`/`except` works.

Put top-level control flow inside a function (especially `def main():` for file scripts). Notebook cells allow top-level `if`/`for`.

## External Starlark references

| Resource | Why |
|----------|-----|
| [Starlark language spec](https://github.com/bazelbuild/starlark/blob/master/spec.md) | Canonical language definition (Bazel / starlark-go share this dialect) |
| [Starlark for Bazel](https://bazel.build/rules/language) | Readable overview of types, functions, and differences from Python |
| [starlark-go](https://github.com/google/starlark-go) | Interpreter buddy embeds |

CLI cheat sheet (same material, terminal-sized):

```sh
buddy help starlark
```

## Types and literals

`None` · `bool` · `int` · `float` · `str` · `bytes` · `list` · `dict` · `tuple`

`bytes` concatenate with `+` (and `+=`), including when both sides are variables — the same rule as `str` / `list` / `tuple`.

```python
x = 42
name = "chrome"
items = [1, 2, 3]
meta = {"id": "10", "name": "Demo"}
a, b = 1, 2
blob = file_read_bytes("icon.png") + b"\n"
```

## Control flow and functions

```python
def main():
    for p in policies():
        if p["name"] == "skip":
            continue
        print(p["id"], p["name"])
    return None
```

Supported: `if` / `elif` / `else`, `for`, `break`, `continue`, `pass`, `def`, `return`, `*args` / `**kwargs`.

**Not in Starlark (vs Python):** `import`, `class`, `try`/`except`, `while`, `lambda`, `yield`, `global`/`nonlocal`, `set`, `is` / `is not` (use `==` / `!=`), file/`os`/`sys` modules. Use `load()` for `re` — not Python `import`.

## Strings, lists, dicts

Familiar methods: `s.split`, `s.format`, `l.append`, `d.get`, `d.keys`, slicing, `len`, `in`, and the usual builtins (`range`, `enumerate`, `sorted`, `min`/`max`, …).

## Modules

```python
load("re", "match")
```

`re` is also predeclared. The `load("stdlib/….buddy", …)` path is reserved and currently empty; recipes live in [`examples/`](../examples/). See [Stdlib](buddyscript-stdlib.md).

## Buddy additions (not stock Starlark)

- Jamf and offline helpers — [Function index](/docs/builtins)
- `error(*args)` — like `print` but to stderr
- `confirm(…)` — interactive yes/no (skipped with global `-y`)
- Session kwargs `context` / `tenant` / `surface` / `auth` on builtins (`tenant` is a dual-read alias of `context`)

Next: [Stdlib](buddyscript-stdlib.md) · [Function index](/docs/builtins) · [Overview](buddyscript.md)
