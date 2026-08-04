# BuddyScript overview

**buddyscript** is buddy’s scripting language: Starlark-based, Python-like, and extended with Jamf builtins, stdlib helpers, and CLI/notebook/IDE runners.

Prefer the **`.buddy`** extension (editor language id + `buddy lsp`). Existing **`.star`** paths still run.

## Where it runs

| Surface | How |
|---------|-----|
| CLI shell | `buddy` with no args — interactive REPL |
| Script file | `buddy script.buddy` — calls `main()` if defined |
| One-liner | `buddy -e '…'` |
| Notebook | [`/notebook`](/notebook) — cells share a kernel |
| IDE | [`/ide`](/ide) — single buffer, file dialect + `main()` |
| MCP | tool `buddyscript` — prefer `def main(): …; return …` |

## Core conventions

- **Bare returns:** builtins return values and **raise** on failure (no `{ok, value}` inside scripts). MCP wraps `{ok, value}` only at the tool boundary.
- **`main()`:** after loading a file (CLI/IDE/MCP), buddy calls `main` if present. Do not call `main()` yourself at top level.
- **Control flow:** put `if` / `for` inside functions — Starlark rejects them at top level in file dialect.
- **Session:** globals `tenant` and `auth`; override per call with `tenant=` / `surface=` / `auth=`.
- **I/O:** `print()` → stdout; buddy’s `error()` → stderr (same signature as `print`).

```python
def main():
    rows = policies()
    error("found", len(rows), "policies")
    return rows[0]["name"] if rows else ""
```

## Language guide

| Page | Contents |
|------|----------|
| [Syntax](buddyscript-syntax.md) | Types, control flow, what Starlark omits vs Python, links to the Starlark spec |
| [Stdlib](buddyscript-stdlib.md) | `load("stdlib/….buddy", …)` modules |
| [Function index](/docs/builtins) | Every builtin (signatures, params, returns) |

Also: [Editor / LSP](editor.md), [Change batches](change-batches.md), [Reference manual](reference-manual.md).

## Quick CLI

```sh
buddy help builtins    # function index (matches this binary)
buddy help starlark    # compact syntax cheat sheet
buddy help shell       # running scripts / -e / REPL
```
