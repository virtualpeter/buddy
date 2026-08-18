package mcp

// toolPrerequisite is prepended to every tool description except get_instructions.
const toolPrerequisite = "Prerequisite: call get_instructions at least once before using this tool. "

// ServerInstructions is returned in the MCP InitializeResult so hosts can inject
// buddy usage guidance (including how buddy relates to Jamf's hosted MCP).
const ServerInstructions = `buddy is a local Jamf automation MCP using your buddy config/secrets.

Call get_instructions first (firm playbook). Then: docs → buddyscript named builtins → jamf_get/jamf_put only when no builtin exists and not denied.
Prefer one buddyscript def main() over many tiny round-trips (host timeouts); do not micro-batch “just in case.”

Tools: get_instructions, buddy_info, docs, jamf_schema (discovery only), buddyscript (primary).
Before resource work: docs query the object noun (e.g. smart_groups, policies, packages, profile-payloads, blueprints) then get that page.
MCP defaults deny jamf_*. Respect buddy_info.deny_builtins / allow_builtins (channel layer) plus tenant_deny_builtins / auth_deny_builtins. Allow follows deny per layer; deny ["*"] is an allowlist. Blocked builtins raise — treat as a gap, do not invent schema HTTP.
Profiles/ACS: profile_xml / profile_put_xml / profile_xml_patch_payload / profile_acs_bind_uuid — never invent Classic PUT from schema. profile_xml is the Classic envelope; list with payloads_list_identifiers then identifier= (Identifier, unique Type, or unique UUID). MCX helpers only for ManagedClient / mcx_preference_settings.
Packages/JCDS: package_ready / package_wait_ready / package_download.
Groups: smart_group* (criteria + smart_group_list_computers) vs static_group* (membership mutate) — do not mix; Platform device_group* is separate.
buddyscript: prefer def main(): … return …; Starlark has no while/try/class (use for … in / range, or builtins like package_wait_ready for polling); builtins return bare values and raise on failure; MCP wraps {ok,value} only at the tool boundary.
Jamf remote MCP (developer.jamf.com/mcp) is useful for product schemas; prefer buddy builtins for tenant changes.`

// AgentPlaybook is the firm guidance returned by get_instructions.
const AgentPlaybook = `# buddy MCP agent playbook

Call **get_instructions** once at the start of a session before other buddy tools.

## Tools (order of use)

1. **get_instructions** — this playbook (no Jamf call).
2. **docs** — list (fuzzy query) then get object cookbooks before inventing.
3. **buddyscript** — primary automation via **named builtins**.
4. **buddy_info** — session/tenant/auth (no Jamf call).
5. **jamf_schema** — endpoint discovery only; not a license to invent PUTs.

**Priority:** docs → buddyscript named builtins → jamf_get/jamf_put via buddyscript **only when no builtin exists**.

## docs

- action=list with a resource noun; query is **fuzzy** (splits on _ - / spaces). Examples: smart_groups, smart groups, policy, packages, profile-payloads, blueprints.
- Prefer too many hits over none; then action=get the matching cookbook (e.g. docs/smart-groups).
- Do not skip docs and guess builtin names or Classic/Pro HTTP bodies.

## BuddyScript (via buddyscript tool)

- Prefer def main(): … return … (no trailing top-level expression), or a single bare expression.
- Starlark has **no while / try / class**. Use for … in / range. Put waits in builtins (e.g. package_wait_ready), not agent poll loops.
- Top-level if/for must be inside functions.

### Return contract (two layers)

| Layer | Shape |
|-------|--------|
| **Builtins inside scripts** | **Bare values** on success; **raise** on failure. No {ok,value} in Starlark. Do not write r["ok"] or unwrap for correctness. |
| **MCP tools** | {ok, value, …} at the **tool boundary only**. |

For the buddyscript tool: primary output is **value.result** (last expression or main() return). stdout is print/unwrap; when result is set and stdout empty, result is mirrored into stdout.

## Long work / host timeouts

MCP hosts (Claude Desktop, Cursor, …) often time out tool calls. Do **not** preemptively split simple loops into many tiny buddyscript round-trips.

- Prefer **one** def main() that does list → filter → work → return server-side.
- Use named wait builtins (package_wait_ready) instead of agent-driven polling.
- Only re-issue or continue after a **real** timeout/error — do not invent micro-batches “just in case.”
- Multi-step release workflows: change-batch builtins / docs/change-batches — not N one-liner MCP calls.

## Resource rules (do not invent from schema)

- Respect **buddy_info.deny_builtins** / **allow_builtins** (channel) and tenant_deny_builtins / auth_deny_builtins. MCP defaults deny jamf_* on the channel layer. Each layer is independent (channel allow cannot unblock a tenant deny). deny ["*"] is an allowlist at that layer. Blocked builtins raise on call — treat as a coverage gap; do not invent alternate HTTP.
- **Groups:** smart_group* (criteria + smart_group_list_computers) vs static_group* (membership mutate) vs Platform device_group* — do not mix.
- **Profiles/ACS:** profile_xml / profile_put_xml / profile_xml_patch_payload / profile_acs_bind_uuid — never invent Classic profile PUT from jamf_schema. profile_xml is the Classic envelope (not a bare plist). Inspect via profile_xml_extract_payloads → payloads_list_identifiers, then identifier= (PayloadIdentifier, unique PayloadType, or unique PayloadUUID). payloads_get_mcx_dict only for ManagedClient / flat mcx_preference_settings — not com.apple.MCX PayloadContent.
- **Packages/JCDS:** package_ready / package_wait_ready / package_download.
- Jamf remote MCP (developer.jamf.com/mcp) = product schemas; buddy = tenant changes with your config/secrets.
`

// Tool description bodies (prerequisite prepended at registration except get_instructions).
const (
	descGetInstructions = "Firm buddy MCP playbook: tool priority, docs-first pathfinding, BuddyScript dialect, " +
		"return contract (bare builtins vs MCP {ok,value}), and long-work guidance. " +
		"Call once at session start before other buddy tools. No Jamf call. Returns {ok, value: markdown}."

	descBuddyInfo = toolPrerequisite +
		"Buddy binary version and MCP session selection: default tenant, optional auth pin, " +
		"and which tenants/auths are available on the mcp channel (respects auth channels:). " +
		"Includes deny_builtins (MCP defaults jamf_*) and allow_builtins from channels.mcp, " +
		"plus tenant_deny_builtins / auth_deny_builtins for the process default session. " +
		"Does not call Jamf. Live auth is proven by the first buddyscript Jamf call, or CLI buddy auth test."

	descDocs = toolPrerequisite +
		"Read buddy embedded documentation and design rules (same catalog as buddy://docs/… resources). " +
		"Prefer this before inventing buddy conventions. " +
		"action=list (default) returns [{id,title,section,uri},…]; optional query fuzzy-filters id/title/section/keywords " +
		"(case-insensitive; splits on _ - / spaces — try smart_groups, smart groups, policy, packages, blueprints; may return multiple hits). " +
		"action=get requires id and returns {id,title,uri,markdown}. List then get."

	descJamfSchema = toolPrerequisite +
		"Discover Jamf API operations (Pro OpenAPI, Classic Swagger, embedded Platform specs). " +
		"Default api=all. Use filter (e.g. \"blueprints\" or \"policy delete\") to narrow. " +
		"Returns {ok, value: {api, count, operations:[...], …}}. Set full=true only for parameter/response schemas. " +
		"Schema is discovery only — not a license to PUT profiles/groups/packages. " +
		"After finding a path, prefer a named builtin via buddyscript; " +
		"use jamf_get/jamf_put via buddyscript only when no builtin exists. " +
		"Never invent Classic profile XML PUTs or smart/static group bodies from schema alone (use profile_* / smart_group* / static_group*)."

	descBuddyscript = toolPrerequisite +
		"Primary Jamf automation: evaluate buddy Starlark with named builtins in a fresh session. " +
		"Returns {ok, value: {stdout, stderr, result, error}} at the MCP boundary only. " +
		"Starlark builtins return bare values and raise on failure (no {ok,value} inside scripts). " +
		"PRIMARY OUTPUT is value.result (last expression or main() return). stdout is print/unwrap; " +
		"when result is set and stdout empty, result is mirrored into stdout. " +
		"Prefer one def main() that does the full loop server-side — do not micro-batch round-trips for fear of host timeouts. " +
		"Prefer def main(): … return … (no trailing top-level expr), or a single bare expression. " +
		"Before resource work: docs query object noun (policies, packages, smart-groups, profile-payloads, blueprints). " +
		"Profiles/ACS: profile_xml / profile_put_xml / profile_xml_patch_payload — never jamf_put Classic profile XML. " +
		"profile_xml is the Classic envelope; payloads_list_identifiers then identifier= (Identifier / unique Type / unique UUID). " +
		"MCX helpers only for ManagedClient / mcx_preference_settings. " +
		"Packages: package_wait_ready / package_download. " +
		"Groups: smart_group* (criteria + smart_group_list_computers) vs static_group* (membership mutate) — do not mix; Platform device_group* is separate. " +
		"jamf_get/jamf_put only when no builtin exists. Top-level if/for must be inside functions. " +
		"Starlark has no while/try/class (use for … in / range, or builtins like package_wait_ready for polling)."
)
