# Getting started with buddy

**buddy** is a Jamf Pro CLI and buddyscript tool: policies, groups, profiles, packages, scripts, computers/MDM, Platform API (blueprints, devices, …), plus a local docs UI, notebook, and script IDE.

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

Already have tenants/auths? Use `buddy cred …` and `buddy auth test` instead — details in the [reference manual](/docs/docs/reference-manual) and [Auth, secrets, and tokens](/docs/docs/auth).

## 3. Try it

```sh
buddy policy list                     # or: group list, package list, …
buddy serve --open                    # docs + notebook + /ide in the browser
buddy help                            # CLI overview
buddy help builtins                   # buddyscript API reference
```

In the notebook, open **Libraries** → **Getting started** for a short list of builtins to try in cells.

## Resources (by object type)

| Guide | What it’s for |
|-------|----------------|
| [Policies](/docs/docs/policies) | Classic policies: list/set/copy/migrate |
| [Smart computer groups](/docs/docs/smart-groups) | Criteria-based membership |
| [Static computer groups](/docs/docs/static-groups) | Explicit computer ID membership |
| [Configuration profiles](/docs/docs/profiles) | Profile lifecycle (clone/copy/redeploy) |
| [Profile payloads](/docs/docs/profile-payloads) | Dissect/assemble PayloadContent vs MCX |
| [Packages / JCDS](/docs/docs/packages) | READY gate, wait, download |
| [Scripts](/docs/docs/scripts) | List vs body; copy/migrate |
| [Categories](/docs/docs/categories) | Category copy (migrate step 1) |
| [Mac App Store apps](/docs/docs/macapps) | Classic macapplications (list/get/create/delete) |
| [Policy apps](/docs/docs/apps) | Self Service + packages (list/get/upgrade) |
| [Computers](/docs/docs/computers) | Inventory + MDM actions |
| [MDM command history](/docs/docs/mdm) | Query command history (RSQL) |
| [API roles](/docs/docs/api-roles) | Pro API roles |
| [API clients](/docs/docs/api-clients) | Pro API integrations / oauth clients |

## Platform

| Guide | What it’s for |
|-------|----------------|
| [Blueprints](/docs/docs/blueprints) | Platform blueprints CRUD / deploy |
| [Platform devices](/docs/docs/devices) | Platform inventory + actions |
| [Device groups](/docs/docs/device-groups) | Platform device groups |
| [DDM reporting](/docs/docs/ddm) | Declaration reporting |
| [Compliance benchmarks](/docs/docs/benchmarks) | Benchmarks (beta) |

## Guides

| Guide | What it’s for |
|-------|----------------|
| [Auth, secrets, and tokens](/docs/docs/auth) | Tenants, auths, channel policy, secret stores, token cache |
| [Reference manual](/docs/docs/reference-manual) | Config, credentials, CLI, buddyscript, environment |
| [BuddyScript overview](/docs/docs/buddyscript) | Language, runners, conventions |
| [BuddyScript syntax](/docs/docs/buddyscript-syntax) | Starlark subset + external spec links |
| [BuddyScript stdlib](/docs/docs/buddyscript-stdlib) | `load("stdlib/….buddy", …)` |
| [Function index](/docs/builtins) | Every builtin (signatures, params, returns) |
| [Change batches](/docs/docs/change-batches) | Plan → approve → apply workflows |
| [API endpoints](/docs/docs/api-endpoints) | Jamf HTTP APIs buddy calls |
| [MCP for AI tools](/docs/docs/mcp) | buddy as an MCP server (Cursor, Claude Desktop, …) |
| [MCP tools scorecard](/docs/docs/mcp-tools) | Agent tool priority and pathfinding |
| [Editor / LSP](/docs/docs/editor) | `buddy lsp` + VS Code/Cursor extension (build & install VSIX) |
| [Examples index](/docs/examples/README.md) | Runnable `.buddy` samples |
| [Notebook](/notebook) | Interactive buddyscript against your tenant/auth |
| [Script IDE](/ide) | Single-buffer runner (open/save, args, trace) |
