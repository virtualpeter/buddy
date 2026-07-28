# Editor support: LSP and VS Code / Cursor extension

Buddy scripts use the **`.buddy`** file extension. Editor support is two pieces:

1. **`buddy lsp`** — language server over stdio (diagnostics, completions, hover, signature help)
2. **Buddy Starlark** extension under [`editors/vscode`](../editors/vscode) — syntax highlighting + LSP client for VS Code and Cursor

Prefer `.buddy` for new scripts and stdlib modules (`load("stdlib/<name>.buddy", …)`). Existing `.star` paths still run; optionally associate them via settings.

## What you get

| Feature | Source |
|---------|--------|
| Syntax highlighting | TextMate grammar in the extension |
| Parse / undefined-name diagnostics | `buddy lsp` |
| Completions, hover, signature help | `buddy lsp` (same catalog as `buddy help builtins`) |
| `load("stdlib/…")` checks | `buddy lsp` |

The language server is usually started by the extension, not by hand. See `buddy help lsp` for the CLI surface.

## Prerequisites

1. Build and install buddy so `buddy lsp` works:

```sh
make buddy
make install    # optional: puts buddy on PATH via ~/go/bin
buddy lsp       # should wait on stdin (Ctrl-C to quit) — confirms the command exists
```

2. Node.js + npm (for packaging the extension).

## Build the VSIX (local)

From the repo root:

```sh
make extension-package
# → editors/vscode/buddy-0.1.0.vsix
# extension id: virtualpete.buddy  (PUBLISHER defaults to virtualpete)
```

Or from the extension folder:

```sh
cd editors/vscode
npm install
npm run package
# PUBLISHER=otherorg npm run package   # optional alternate publisher id
```

The package script compiles TypeScript and runs `vsce package`. The VSIX lands next to `package.json` as `buddy-<version>.vsix`.

## Install into Cursor

**CLI** (from the repo root, adjust the version if needed):

```sh
cursor --install-extension editors/vscode/buddy-0.1.0.vsix
```

**UI:**

1. Open Cursor
2. Command Palette (`Cmd+Shift+P` / `Ctrl+Shift+P`) → **Extensions: Install from VSIX…**
3. Choose `editors/vscode/buddy-0.1.0.vsix`
4. Reload the window if prompted

Open a `.buddy` file — status bar language should show **Buddy Starlark**. Completions/hover require a working `buddy` on `PATH` (or set `buddy.path` below).

## Install into VS Code

```sh
code --install-extension editors/vscode/buddy-0.1.0.vsix
```

Or Command Palette → **Extensions: Install from VSIX…**, same as Cursor.

## Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `buddy.path` | `buddy` | Path to the buddy executable (used to launch `buddy lsp`) |
| `buddy.lsp.path` | _(empty)_ | Full override for the LSP command (e.g. `/path/to/buddy lsp`) |
| `buddy.associateStar` | `false` | Also treat `*.star` as buddy language |
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

## Extension development (optional)

Without packaging a VSIX:

```sh
cd editors/vscode
npm install
npm run compile
```

Open the `editors/vscode` folder in VS Code or Cursor and press **F5** to launch an Extension Development Host.

## Related

- `buddy help lsp` — CLI notes for the language server
- [Reference manual](reference-manual.md) — scripting overview
- [Getting started](index.md) — install buddy and connect to Jamf
- Repo folder: [`editors/vscode/README.md`](../editors/vscode/README.md)
