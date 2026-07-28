package main

const helpServe = `buddy serve - Local docs, notebook, and script IDE web UI

Usage:
  buddy [global-flags] serve [args...]

Flags for 'serve':
  --addr=<host:port>   Listen address (default: 127.0.0.1:8742; localhost only)
  --docs-root=<path>   Override embedded docs with a live docs/ directory
  --open               Open the docs URL in the default browser

Notes:
  Serves a curated user docs UI (guides, builtins, examples) plus:
  /notebook — REPL-style Starlark cells (explore builtins interactively)
  /ide      — Single-buffer script runner (open/save, args, trace, Run as file)
  Contributor/agent knowledge (AGENTS, Cursor rules, migration notes) is
  embedded for MCP only — not listed in the HTML sidebar.
  When run from a checkout, serve prefers the repo docs/ tree so edits show
  without a rebuild; otherwise the embedded catalog is used. --docs-root /
  BUDDY_DOCS_ROOT force a filesystem override.
  IDE open/save is limited to the process working directory, or BUDDY_IDE_ROOT
  if set. Run matches CLI file semantics (file dialect, call main() if present,
  args from the form, optional builtin-call trace). Line breakpoints / stepping
  are not supported (Starlark-go has no pause API); a soft breakpoint() builtin
  may come later.
  Starlark cells and IDE runs use the same builtins as the CLI; confirmations
  follow -y / --yes. Notebook/IDE pick a tenant and optional auth pin (same as
  --tenant / --user); empty auth auto-picks for the tenant.
  Bind address must be localhost (127.0.0.1, localhost, or ::1).

Examples:
  buddy serve --open
  buddy --tenant=sandbox serve --open
  buddy serve --addr=127.0.0.1:9000
`

const helpMCP = `buddy mcp - MCP stdio server for AI tools

Usage:
  buddy [global-flags] mcp
  buddy [global-flags] mcp setup [args...]

Subcommands:
  setup   Discover well-known MCP host configs; optionally write a buddy entry

Flags for 'setup':
  --write              Merge mcpServers.buddy into selected configs
  --create             Also create missing well-known files (with --write)
  --path=<file>        Inspect/write a specific MCP config JSON file
  --host=<name>        Filter: cursor | claude-desktop (default: all well-known)

Notes:
  Speaks Model Context Protocol over stdin/stdout for AI hosts (Cursor,
  Claude Desktop, and other MCP clients). Exposes curated Jamf tools plus
  starlark_eval, docs (embedded knowledge catalog), and buddy://docs/... resources.
  Uses the active tenant/auth from config (override with --tenant / --user).

  Every tool returns a JSON object {ok, value, error?, count?} — never a bare
  array (hosts require object structuredContent). buddy_info reports binary
  version plus default tenant / auth pin and available tenants/auths for the
  mcp channel. auth_test defaults to a compact summary; pass verbose=true for
  the full privileges payload.

  Docs, AGENTS.md, examples, and editor rules are embedded in the binary and
  exposed as buddy://docs/... resources (no BUDDY_DOCS_ROOT required). Set
  BUDDY_DOCS_ROOT only to override with live files while developing. See
  docs/mcp.md and make sync-docfs after editing knowledge sources.

  mcp setup (no flags) lists well-known paths and whether buddy is configured.
  Prefer an absolute command path — host subprocesses often omit shell PATH.

Well-known config paths:
  Cursor (user)       ~/.cursor/mcp.json
  Cursor (project)    ./.cursor/mcp.json
  Claude Desktop      ~/Library/Application Support/Claude/claude_desktop_config.json
                      (macOS; Linux ~/.config/Claude/...; Windows %APPDATA%\Claude\...)

Example mcpServers entry:
  {
    "mcpServers": {
      "buddy": {
        "command": "/Users/YOU/go/bin/buddy",
        "args": ["mcp"]
      }
    }
  }

Examples:
  buddy mcp
  buddy mcp setup
  buddy mcp setup --write
  buddy -y mcp setup --write --create --host=cursor
`

const helpLSP = `buddy lsp - Language server for buddy Starlark (.buddy)

Usage:
  buddy lsp

Notes:
  Speaks LSP over stdin/stdout for the Cursor/VS Code buddy extension.
  Provides parse diagnostics, undefined-name checks, invalid load() modules,
  completions, hover, and signature help.
  Uses the same builtin catalog as 'buddy help builtins' and embedded stdlib/.

  Prefer the .buddy file extension for user scripts. Embedded stdlib modules
  and stdlib use .buddy (e.g. load("stdlib/<name>.buddy", …)). Existing .star paths still run.

  Optional initializationOptions.cellMode=true enables notebook/-e dialect
  (top-level if/for and global reassign).

Examples:
  buddy lsp
  # usually launched by the editor extension, not interactively
`
