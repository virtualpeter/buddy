# Editor support: LSP and VS Code / Cursor extension

Buddy scripts use the **`.buddy`** file extension. Editor support is two pieces:

1. **`buddy lsp`** — language server over stdio (diagnostics, completions, hover, signature help). Lives in this repo.
2. **BuddyScript** extension — syntax highlighting + LSP client for VS Code and Cursor. Lives in the sibling repo `buddy-extension` (next to this repo): [virtualpeter/buddy-extension](https://github.com/virtualpeter/buddy-extension).

`load("stdlib/<name>.buddy", …)` is reserved (no modules yet). `load("re", …)` is the shipped module.

## What you get

| Feature | Source |
|---------|--------|
| Syntax highlighting | TextMate grammar in buddy-extension |
| Parse / undefined-name diagnostics | `buddy lsp` |
| Completions, hover, signature help | `buddy lsp` (same catalog as `buddy help builtins`) |
| `load()` module checks | `buddy lsp` |

The language server is usually started by the extension, not by hand. See `buddy help lsp` for the CLI surface.

## Prerequisites

Build and install buddy so `buddy lsp` works:

```sh
make buddy
make install    # optional: puts buddy on PATH via ~/go/bin
buddy lsp       # should wait on stdin (Ctrl-C to quit) — confirms the command exists
```

The extension execs `buddy` (or `buddy.path`) and does not embed this binary.

## Build and install the VSIX

The extension is not packaged from this repo. From the sibling folder:

```sh
cd ../buddy-extension
make                 # → buddy-0.1.1.vsix
# make package PUBLISHER=otherorg
cursor --install-extension buddy-0.1.1.vsix
# or: code --install-extension buddy-0.1.1.vsix
```

Or Command Palette → **Extensions: Install from VSIX…**

Open a `.buddy` file — status bar language should show **BuddyScript**. Completions/hover require a working `buddy` on `PATH` (or set `buddy.path` below).

## Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `buddy.path` | `buddy` | Path to the buddy executable (used to launch `buddy lsp`) |
| `buddy.lsp.path` | _(empty)_ | Full override for the LSP command (e.g. `/path/to/buddy lsp`) |
| `buddy.cellMode` | `false` | Notebook / `-e` dialect in the language server |
| `buddy.trace.server` | `off` | LSP client↔server trace (`off` / `messages` / `verbose`) |

Example (User Settings JSON) when buddy is not on `PATH`:

```json
{
  "buddy.path": "/Users/YOU/go/bin/buddy"
}
```

## Verify

1. Open any `.buddy` file (e.g. `examples/bump-release.buddy`).
2. Introduce a typo or unknown name — a diagnostic should appear.
3. Type a builtin name (e.g. `polic`) and confirm completions.
4. Hover a builtin — signature / help text from the catalog.

If nothing happens, set `buddy.trace.server` to `verbose` and check the **Output** / **Buddy Language Server** channel; confirm `buddy.path` points at a binary that includes `lsp`.

## Extension development

Open `../buddy-extension` in VS Code or Cursor, run `make compile`, and press **F5**.

## Related

- `buddy help lsp` — CLI notes for the language server
- [Reference manual](reference-manual.md) — scripting overview
- [Getting started](index.md) — install buddy and connect to Jamf
- Extension: [buddy-extension](https://github.com/virtualpeter/buddy-extension) (sibling folder `../buddy-extension`)
