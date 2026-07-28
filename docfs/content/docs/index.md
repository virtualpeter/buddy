# Getting started with buddy

**buddy** is a Jamf Pro CLI and Starlark scripting tool: policies, groups, profiles, packages, scripts, computers/MDM, Platform API (blueprints, devices, …), plus a local docs UI, notebook, and script IDE.

## 1. Install

```sh
make buddy
make install    # optional: ~/go/bin/buddy
```

## 2. Connect to Jamf (`buddy setup`)

If you have **no config yet**, do not hand-edit `config.yaml` first — bootstrap with setup:

```sh
buddy setup basic-setup               # prompts: tenant name, Jamf URL, username, password
buddy auth test                       # verify the session
```

That writes `~/.config/buddy/config.yaml`, stores the password in the platform secret store (Keychain on macOS), and sets a default tenant.

**Optional — API client (oauth) for day-to-day use** (instance only; not Platform gateway):

```sh
buddy setup write-script              # writes setup_buddy_role.buddy
buddy setup_buddy_role.buddy -y       # creates role "Buddy" + client "buddy-cli" (idempotent)
buddy setup store-oauth               # rotates client secret into a buddy oauth auth
buddy --user <auth-name> auth test
```

See `buddy setup help` for flags. After setup, review the Jamf API role and **drop privileges** you do not need (especially destructive MDM actions).

Already have tenants/auths? Use `buddy cred …` and `buddy auth test` instead — details in the [reference manual](/docs/docs/reference-manual).

## 3. Try it

```sh
buddy policy list                     # or: group list, package list, …
buddy serve --open                    # docs + notebook + /ide in the browser
buddy help                            # CLI overview
buddy help builtins                   # Starlark API reference
```

In the notebook, open **Libraries** → **Getting started** for a short list of builtins to try in cells.

## Guides

| Guide | What it’s for |
|-------|----------------|
| [Reference manual](/docs/docs/reference-manual) | Config, credentials, CLI, Starlark, environment |
| [Change batches](/docs/docs/change-batches) | Plan → approve → apply workflows |
| [Starlark builtins](/docs/builtins) | Every built-in (signatures, params, returns) |
| [API endpoints](/docs/docs/api-endpoints) | Jamf HTTP APIs buddy calls |
| [MCP for AI tools](/docs/docs/mcp) | buddy as an MCP server (Cursor, Claude Desktop, …) |
| [Editor / LSP](/docs/docs/editor) | `buddy lsp` + VS Code/Cursor extension (build & install VSIX) |
| [Examples index](/docs/examples/README.md) | Runnable `.buddy` samples |
| [Notebook](/notebook) | Interactive Starlark against your tenant/auth |
| [Script IDE](/ide) | Single-buffer runner (open/save, args, trace) |
