# buddy

<p class="about-hero"><img src="/buddy.png" width="168" height="168" alt="buddy"></p>

**buddy** for Jamf Pro is a multipurpose tool with CLI, MCP, and its own scripting language. It covers Classic and Pro api functions (policies, computer groups, configuration profiles, packages/JCDS, scripts, computers and MDM, API roles and clients) and newer features only found via the Platform API (blueprints, devices, device groups, DDM reporting, compliance benchmarks).

**buddy** is independent open-source software (MIT). It talks to [Jamf Pro](https://www.jamf.com/) and Jamf Platform **APIs**. It is not affiliated with, endorsed by, or sponsored by Jamf. Jamf, Jamf Pro, and related names are trademarks of Jamf.

Day-to-day work can follow several different styles — you can work with **buddyscript** (an extended version of Starlark) via an interactive shell, `.buddy` script files, or via this local docs UI with script IDE (`buddy serve`). Alternatively, connect it as an MCP server for AI hosts (`buddy mcp`) and let the agent learn how to use buddyscript.

The only required config is to define tenants + auths; passwords or oauth secrets get stored in the encrypted vault (`secrets.enc`), with a random Keychain data key and a recovery passphrase for backups — [Auth, secrets, and tokens](auth.md).

## Start here

<ul class="about-jump">
<li><a href="/docs/docs/getting-started">Getting started</a><span>Install, <code>buddy setup</code>, and your first commands</span></li>
<li><a href="/ide">Script IDE</a><span>Open, edit, and run <code>.buddy</code> scripts in the browser</span></li>
<li><a href="/docs/docs/auth">Auth, secrets, and tokens</a><span>Tenants, vault, channel policy, token cache</span></li>
<li><a href="/docs/docs/reference-manual">Reference manual</a><span>Config, CLI, buddyscript, environment</span></li>
<li><a href="/docs/docs/mcp">MCP for AI tools</a><span>Use buddy from Cursor, Claude Desktop, and other hosts</span></li>
<li><a href="/docs/builtins">Function index</a><span>Every buddyscript builtin</span></li>
<li><a href="/docs/cli">Command index</a><span>Every <code>buddy</code> CLI command</span></li>
<li><a href="/docs/examples/README.md">Examples</a><span>Runnable <code>.buddy</code> samples</span></li>
</ul>

## Resources (by object type)

| Guide | What it’s for |
|-------|----------------|
| [Policies](policies.md) | Classic policies: list/set/copy/migrate |
| [Smart computer groups](smart-groups.md) | Criteria-based membership |
| [Static computer groups](static-groups.md) | Explicit computer ID membership |
| [Configuration profiles](profiles.md) | Profile lifecycle (clone/copy/redeploy) |
| [Profile payloads](profile-payloads.md) | Dissect/assemble PayloadContent vs MCX |
| [Packages / JCDS](packages.md) | READY gate, wait, download |
| [Icons](icons.md) | Self Service icons (list via weekly catalog cache; get/upload/check/download) |
| [Scripts](scripts.md) | List vs body; copy/migrate |
| [Categories](categories.md) | Category copy (migrate step 1) |
| [Mac App Store apps](macapps.md) | Classic macapplications (list/get/create/delete) |
| [Policy apps](apps.md) | Self Service + packages (list/get/upgrade) |
| [Computers](computers.md) | Inventory, Classic policy logs, MDM actions |
| [MDM command history](mdm.md) | Query command history (RSQL) |
| [Mobile devices](mobile-devices.md) | iOS/iPadOS/tvOS inventory + MDM actions |
| [Mobile-device groups](mobile-groups.md) | Smart criteria / static membership |
| [Mobile profiles](mobile-profiles.md) | Mobile configuration profiles (Classic XML) |
| [API roles](api-roles.md) | Pro API roles |
| [API clients](api-clients.md) | Pro API integrations / oauth clients |

### Platform

| Guide | What it’s for |
|-------|----------------|
| [Blueprints](blueprints.md) | Platform blueprints CRUD / deploy |
| [Platform devices](devices.md) | Platform inventory + actions |
| [Device groups](device-groups.md) | Platform device groups |
| [DDM reporting](ddm.md) | Declaration reporting |
| [Compliance benchmarks](benchmarks.md) | Benchmarks |

## Guides

| Guide | What it’s for |
|-------|----------------|
| [BuddyScript overview](buddyscript.md) | Language, runners, conventions |
| [BuddyScript syntax](buddyscript-syntax.md) | Starlark subset + external spec links |
| [BuddyScript stdlib](buddyscript-stdlib.md) | Reserved `load("stdlib/….buddy", …)` path (empty; recipes in `examples/`) |
| [Change batches](change-batches.md) | Plan → approve → apply workflows |
| [API endpoints](api-endpoints.md) | Jamf HTTP APIs buddy calls |
| [MCP tools scorecard](mcp-tools.md) | Agent tool priority and pathfinding |
| [Editor / LSP](editor.md) | `buddy lsp`; VSIX is the sibling `buddy-extension` checkout |
