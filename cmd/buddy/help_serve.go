package main

const helpServe = `buddy serve - Local docs and Starlark notebook web UI

Usage:
  buddy [global-flags] serve [args...]

Flags for 'serve':
  --addr=<host:port>   Listen address (default: 127.0.0.1:8742; localhost only)
  --docs-root=<path>   Path to docs/ directory (default: find repo docs/ or BUDDY_DOCS_ROOT)
  --open               Open the docs URL in the default browser

Notes:
  Serves Markdown docs, CLI help (builtins/shell/starlark), and a notebook UI at /notebook.
  Starlark cells use the same builtins as the CLI; confirmations follow -y / --yes.
  Bind address must be localhost (127.0.0.1, localhost, or ::1).

Examples:
  buddy serve --open
  buddy serve --addr=127.0.0.1:9000
`

const helpMCP = `buddy mcp - MCP stdio server for Cursor agents

Usage:
  buddy [global-flags] mcp

Notes:
  Speaks Model Context Protocol over stdin/stdout (for Cursor mcp.json).
  Exposes curated read-oriented tools plus starlark_eval and docs resources.
  Uses the active credential from config (override with --user).

  Every tool returns a JSON object {ok, value, error?, count?} — never a bare
  array (Cursor requires object structuredContent). auth_test defaults to a
  compact summary; pass verbose=true for the full privileges payload.

  Prefer an absolute path to buddy in mcp.json; set BUDDY_DOCS_ROOT to your
  repo docs/ so buddy://docs/... resources resolve. See docs/mcp.md.

Example Cursor mcp.json entry:
  {
    "mcpServers": {
      "buddy": {
        "command": "/Users/YOU/go/bin/buddy",
        "args": ["mcp"],
        "env": { "BUDDY_DOCS_ROOT": "/Users/YOU/work/buddy/docs" }
      }
    }
  }
`
