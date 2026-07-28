# Buddy Starlark (VS Code / Cursor)

Thin editor client for buddy Starlark (`.buddy` files): syntax highlighting + LSP via `buddy lsp`.

**Full howto** (build VSIX, install into Cursor/VS Code, settings, verify): see [docs/editor.md](../../docs/editor.md).

```sh
# from repo root
make extension-package
cursor --install-extension editors/vscode/buddy-0.1.0.vsix
# or: code --install-extension editors/vscode/buddy-0.1.0.vsix
```

## Quick settings

| Setting | Default | Description |
|---------|---------|-------------|
| `buddy.path` | `buddy` | Path to the buddy executable |
| `buddy.lsp.path` | _(empty)_ | Override LSP command |
| `buddy.associateStar` | `false` | Treat `*.star` as buddy language |
| `buddy.cellMode` | `false` | Notebook/`-e` dialect in the language server |
| `buddy.trace.server` | `off` | LSP client/server trace level |

## Development

```sh
npm install
npm run compile
# Press F5 in VS Code/Cursor with this folder open to launch an Extension Development Host
```
