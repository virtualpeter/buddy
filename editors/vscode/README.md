# Buddy Starlark (VS Code / Cursor)

Editor support for buddy Starlark scripts using the `.buddy` file extension.

## Features

- Syntax highlighting for `.buddy`
- Language server via `buddy lsp` (diagnostics, completions, hover, signature help)
- Optional association of `*.star` with the buddy language (`buddy.associateStar`)

## Requirements

- `buddy` on your `PATH` (or set `buddy.path` to the binary)
- Buddy built with the `lsp` command (`buddy lsp`)

## Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `buddy.path` | `buddy` | Path to the buddy executable |
| `buddy.lsp.path` | _(empty)_ | Override LSP command (e.g. `/path/to/buddy lsp`) |
| `buddy.associateStar` | `false` | Treat `*.star` as buddy language |
| `buddy.cellMode` | `false` | Notebook/`-e` dialect in the language server |
| `buddy.trace.server` | `off` | LSP client/server trace level |

## Package a VSIX

Publisher is set at package time (default `virtualpeter`):

```sh
cd editors/vscode
npm install
npm run package
# → virtualpeter.buddy-0.1.0.vsix

PUBLISHER=otherorg npm run package
# → otherorg.buddy-0.1.0.vsix
```

Install in Cursor/VS Code:

```sh
code --install-extension virtualpeter.buddy-0.1.0.vsix
# or
cursor --install-extension virtualpeter.buddy-0.1.0.vsix
```

## Development

```sh
npm install
npm run compile
# Press F5 in VS Code/Cursor with this folder open to launch an Extension Development Host
```
