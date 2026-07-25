# Jamf Pro API Endpoints Used by Buddy

For CLI scripting and configuration, start with the [reference manual](reference-manual.md).

Buddy communicates with Jamf through direct HTTP requests (no third-party SDK).

- **Classic API** — XML-based, path prefix `/JSSResource/`
- **Jamf Pro API** — JSON-based, path prefix `/api/v1/` or `/api/v2/` / `/api/v3/`
- **Platform API Gateway (optional)** — set `region` (e.g. `us`) and/or gateway `url` `https://{us|eu|apac}.apigw.jamf.com`, plus OAuth client credentials and `tenantId` on the credential

Call sites still use instance-relative paths (`/api/v1/…`, `/JSSResource/…`). When the credential has `tenantId` set, buddy rewrites them:

| Caller path | Gateway path |
|-------------|--------------|
| `/api/vN/rest…` | `/api/pro/vN/tenant/{tenantId}/rest…` |
| `/JSSResource/…` | `/api/proclassic/tenant/{tenantId}/JSSResource/…` |
| `/api/blueprints/vN/…` | inject `/tenant/{tenantId}` after the version |

**Limitation (Classic / proclassic):** Buddy rewrites Classic paths for the gateway, but Jamf Account Platform integrations do not yet expose Classic permissions. In practice, Pro JSON and Blueprints work through the gateway when the integration is scoped correctly; Classic calls (`/JSSResource/…`, profiles, Classic policy XML, etc.) typically return `403 BAD_PERMISSIONS` until Jamf adds those privileges. Prefer an instance credential for Classic-heavy workflows for now.

Authentication is config-driven per credential (command, basic, or OAuth). See `buddy auth help` and `buddy cred help`. For **basic** (username/password), buddy obtains a Bearer token via `POST /api/v1/auth/token` with Basic auth, then uses that token for all API requests. Platform credentials use **OAuth only** (`POST {gateway}/auth/token`, fallback `/oauth2/token`); there is no keep-alive.

### Custom calls from Starlark

For endpoints without a typed builtin, scripts can call:

- `jamf_get` / `jamf_post` / `jamf_put` / `jamf_patch` / `jamf_delete`
- or `jamf_request(method, path, …)`

Paths are host-relative (`/api/v1/…` or `/JSSResource/…`). Media-type defaults follow the path (JSON for `/api/…`, XML for `/JSSResource/…` and gateway `/api/proclassic/…`). See `buddy help builtins` (section **JAMF API**).

### API schema documents

Each Jamf **instance** host publishes its live Pro/Classic specs (use `jamf_schema` from Starlark). The **Platform API Gateway** does **not** serve a single live schema, so buddy ships a snapshot of the per-service Platform specs embedded in the binary:

| API | `api=` | Source | Format |
|-----|--------|--------|--------|
| Jamf Pro API | `pro` | live `/api/schema` | OpenAPI 3 JSON |
| Classic API | `classic` | live `/classicapi/doc/swagger.yaml` | Swagger 2 YAML |
| Platform API Gateway | `platform` | embedded specs (blueprints, devices, device-groups, device-actions, ddm/report, compliance-benchmarks) | OpenAPI 3.x JSON |

`api="both"` searches pro+classic; `api="all"` (the default) also includes platform. Platform paths are prefixed with each service's gateway base (e.g. `/api/blueprints/v1/tenant/{tenantId}/blueprints`) and tagged with the service segment, so `jamf_schema(filter="blueprints deploy")` finds them. `api="platform"` needs no credential.

**Authentication:** none of these schema documents require authentication. The Pro `/api/schema` and Classic `/classicapi/doc/swagger.yaml` endpoints are served publicly by the Jamf host (only *data* endpoints need a bearer token), and the Platform specs are embedded in the binary. buddy fetches Pro/Classic schemas with **no `Authorization` header**, using the credential only to resolve the host base URL — so schema discovery works even when the credential has no stored secret.

Example: `jamf_schema(filter="policy delete")` searches Pro, Classic, and Platform (default `api="all"`). MCP exposes the same helper as tool `jamf_schema`.

The embedded Platform specs are vendored from [`Jamf-Concepts/jamf-cli`](https://github.com/Jamf-Concepts/jamf-cli/tree/main/specs/platform) (MIT) into `internal/buddy/platformschema/specs/`. Refresh them with `make sync-platform-schema` and commit the result.

### Pagination

Jamf Pro JSON list endpoints that return `{totalCount, results}` are fetched via `fetchJamfPaged` (`internal/buddy/pagination.go`):

- Query params: `page` (0-based) and `page-size` (default **200**).
- CLI table printers and internal resolvers (find-by-name, etc.) request **all pages**.
- Starlark list builtins expose `page?` / `page_size?`; **omit `page`** to fetch every page and return the combined list.

Applies to scripts, packages, computers inventory, smart/static group lists, API roles, API integrations, and blueprints. MDM command list uses the same `page` / `page_size` options but ends on a short page (that API’s `totalCount` is not a reliable global total).

---

## Blueprints (Platform API Gateway)

Base path (logical): `/api/blueprints/v1/blueprints`  
Rewritten: `/api/blueprints/v1/tenant/{tenantId}/blueprints`

Requires a credential with `tenantId` and `region` (or gateway `url`, e.g. `https://us.apigw.jamf.com`). CLI: `buddy blueprint …`. Starlark: `blueprints` / `blueprint` / `blueprint_*`.

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET` | `/api/blueprints/v1/blueprints` | `BlueprintList`, `blueprints` | List blueprints (paged; `search`, `sort`) |
| `GET` | `/api/blueprints/v1/blueprints/{id}` | `BlueprintGet`, `blueprint` | Fetch one blueprint |
| `POST` | `/api/blueprints/v1/blueprints` | `BlueprintCreate`, `blueprint_create` | Create |
| `PATCH` | `/api/blueprints/v1/blueprints/{id}` | `BlueprintSet`, `blueprint_set` | Merge-patch update |
| `DELETE` | `/api/blueprints/v1/blueprints/{id}` | `BlueprintDelete`, `blueprint_delete` | Delete |
| `POST` | `…/blueprints/{id}/deploy` | `BlueprintDeploy`, `blueprint_deploy` | Deploy |
| `POST` | `…/blueprints/{id}/undeploy` | `BlueprintUndeploy`, `blueprint_undeploy` | Undeploy |
| `GET` | `…/blueprints/{id}/report` | `BlueprintStatus`, `blueprint_status` | Status report |
| `GET` | `/api/blueprints/v1/blueprint-components` | `BlueprintComponents`, `blueprint_components` | List components |
| `GET` | `/api/blueprints/v1/blueprint-components/{id}` | `BlueprintComponent`, `blueprint_component` | Get component |

---

## Platform devices (`/api/devices`)

Logical paths inject `/tenant/{tenantId}` after `v1` (same as blueprints). Distinct from Jamf Pro `computer` inventory.

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET` | `/api/devices/v1/devices` | `DeviceList`, `devices` | List devices (paged; RSQL `filter`) |
| `GET` | `/api/devices/v1/devices/{id}` | `DeviceGet`, `device` | Fetch one |
| `PATCH` | `/api/devices/v1/devices/{id}` | `DeviceSet`, `device_set` | Update |
| `DELETE` | `/api/devices/v1/devices/{id}` | `DeviceDelete`, `device_delete` | Delete |
| `GET` | `…/devices/{id}/applications` | `DeviceApplications`, `device_applications` | Installed apps |
| `GET` | `/api/devices/v1/users/{id}/devices` | `DevicesForUser`, `buddy device user` | Devices for user |

## Platform device actions (`/api/device-actions`)

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `POST` | `/api/device-actions/v1/devices/{id}/check-in` | `DeviceCheckIn`, `device_check_in` | Check-in |
| `POST` | `…/restart` | `DeviceRestart`, `device_restart` | Restart |
| `POST` | `…/shutdown` | `DeviceShutdown`, `device_shutdown` | Shutdown |
| `POST` | `…/erase` | `DeviceErase`, `device_erase` | Erase |
| `POST` | `…/unmanage` | `DeviceUnmanage`, `device_unmanage` | Unmanage |

## Platform device groups (`/api/device-groups`)

Not Jamf Pro computer groups (`buddy group` / `smart_group` / `static_group`).

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET`/`POST` | `/api/device-groups/v1/device-groups` | list/create | List / create |
| `GET`/`PATCH`/`DELETE` | `…/device-groups/{id}` | get/set/delete | CRUD |
| `GET`/`PATCH` | `…/device-groups/{id}/members` | members / set-members | Membership |
| `GET` | `/api/device-groups/v1/devices/{id}/device-groups` | `DeviceGroupsForDevice` | Groups for device |

## DDM declaration reporting (`/api/ddm/report`)

Read-only. CLI: `buddy ddm …`. Starlark: `ddm_*`.

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET` | `…/declarations/{id}/devices` | `DDMDeclarationDevices` | Devices for declaration |
| `GET` | `…/devices/{id}/declarations` | `DDMDeviceDeclarations` | Declarations for device |
| `GET` | `…/devices/{id}/channels` | `DDMDeviceChannels` | Channels for device |

## Compliance benchmarks (`/api/compliance-benchmarks`, beta)

CLI: `buddy benchmark …`. Starlark: `benchmark` / `benchmark_*`.

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET`/`POST` | `/api/compliance-benchmarks/v1/benchmarks` | list/create | Tenant benchmarks |
| `GET`/`DELETE` | `…/benchmarks/{id}` | get/delete | One benchmark |
| `GET` | `…/baselines`, `…/rules` | baselines / catalog rules | Catalog |
| `GET` | `…/benchmarks/{id}/compliance-percentage` | report | Compliance % |
| `GET` | `…/benchmarks/{id}/devices`, `…/rules` | report | Report subresources |

---

## Scripts (Jamf Pro API v1)

Base path: `/api/v1/scripts`

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET` | `/api/v1/scripts` | `ScriptSearch`, `ScriptList`, `ScriptGetID` | List scripts (paginated; all pages by default); plain filter = client-side name substring, RSQL = server `filter` |
| `GET` | `/api/v1/scripts/{id}` | `scriptGetFull`, `ScriptGet`, `ScriptGetContents`, `ScriptGetContentsStr`, `ScriptUpdate`, `ScriptSet*` | Fetch full script record including `scriptContents` |
| `POST` | `/api/v1/scripts` | `ScriptCreate` | Create a new script |
| `PUT` | `/api/v1/scripts/{id}` | `scriptPut`, `ScriptUpdate`, `ScriptSet*` | Replace a script record (fetch-then-PUT pattern) |
| `DELETE` | `/api/v1/scripts/{id}` | `scriptDeleteOne`, `ScriptDelete` | Delete a script by ID |
| `GET` | `/api/v1/scripts/{id}/history` | `ScriptHistory` | Fetch audit history; sorted `date:desc` |

### Notes

- Script `id` values are strings (numeric, e.g. `"42"`), consistent with packages, roles, and clients.
- `priority` values in the v1 API are uppercase: `BEFORE`, `AFTER`, `AT_REBOOT` (Classic API used `Before`, `After`, `At Reboot`).
- `ScriptCreate` and `ScriptUpdate` read script body from a local file and send it as the `scriptContents` JSON field.
- `ScriptSet*` functions use a fetch-then-PUT pattern, preserving all fields not being modified.
- The list endpoint does not return `scriptContents`; a separate `GET /api/v1/scripts/{id}` call is required to retrieve the script body.

---

## Policies (Classic API)

Base path: `/JSSResource/policies`

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET` | `/JSSResource/policies` | `PolicyFind`, `PolicyFindByKeywords`, `PolicyPackage`, `PolicyInstall` | Fetch full policy list (id + name) for trigger/name matching and cache |
| `GET` | `/JSSResource/policies/id/{id}` | most operations | Fetch full policy detail (triggers, scripts, scope, self-service, etc.) |
| `PUT` | `/JSSResource/policies/id/{id}` | `PolicySS`, `PolicyScope`, `PolicyScopeAll`, `PolicyEnable`, `PolicyTrigger`, `PolicyInstall`, `PolicyPackage`, `PolicySetXML`, `PolicyClone` (disable source) | Update an existing policy in place |
| `POST` | `/JSSResource/policies/id/0` | `PolicyClone`, `PolicyPackage` (new install policy), `PolicyCreateFromTemplate`, `PolicyCreatePackage`, `PolicyCreateSS` | Create a new policy (Jamf convention: POST to id/0) |
| `DELETE` | `/JSSResource/policies/id/{id}` | `PolicyDelete` | Delete a policy by ID |

### Notes

- `GET /JSSResource/policies` returns a summary list `{id, name}` only; detail fields (trigger, scripts, scope) require a subsequent `GET /JSSResource/policies/id/{id}`.
- Buddy's policy cache stores the summary list locally (`~/.buddy-policy-cache-<cred>.json`) to avoid re-fetching on every invocation. The cache is bypassed on `force_refresh=True` or when the cache file is absent/stale.
- Phase 1 of trigger resolution fetches detail for a small set of name-matched candidate policies to verify `trigger_other` exactly, rather than relying on name heuristics alone.

---

## Packages (Jamf Pro API v1)

Base path: `/api/v1/packages`

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET` | `/api/v1/packages` | `PackageList`, `PackageSearch`, `PackageFindID` | List packages; supports optional RSQL `filter` query param |
| `GET` | `/api/v1/packages/{id}` | `PackageGet`, `packageGetFull` | Fetch single package record |
| `POST` | `/api/v1/packages` | `PackageUpload` | Create new package record (metadata) |
| `POST` | `/api/v1/packages/{id}/upload` | `PackageUpload` | Upload package file (multipart/form-data) |
| `DELETE` | `/api/v1/packages/{id}` | `PackageDelete` (single) | Delete a single package record |
| `POST` | `/api/v1/packages/delete-multiple` | `PackageDelete` (batch) | Delete multiple packages by ID array |
| `PUT` | `/api/v1/packages/{id}` | `PackageSet*` (`packagePut`) | Update package fields: name, notes, info, priority, category, rebootRequired |
| `GET` | `/api/v1/packages/{id}/history` | `PackageGet` (last-changed enrichment), `PackageHistory`, `packageGetLatestHistory` | Fetch package audit history; sorted `date:desc` |

### Notes

- `PackageGet` makes two API calls: one for the record and one for `history?sort=date:desc&page-size=1` to inject `lastChanged` / `lastChangedBy` into the output.
- `PackageFindID` is shared between the CLI (`policy set package`) and DSL (`package(app=..., version=...)` builtin). List/search walks all pages unless a single `page` is requested from Starlark.
- The `delete-multiple` body is `{"ids": ["1","2",...]}`.

---

## Configuration Profiles (Classic API)

Base path: `/JSSResource/osxconfigurationprofiles`

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET` | `/JSSResource/osxconfigurationprofiles` | `ProfileList`, `ProfileSearch`, `ProfileFindID` | List all configuration profiles (id + name) |
| `GET` | `/JSSResource/osxconfigurationprofiles/id/{id}` | `ProfileGetXML`, `ProfileGetPayloads`, `ProfileGetPayloadByDomain`, `ProfileListPayloads`, `ProfileSetPayload`, `ProfileRedeploy`, `ProfileClone` | Fetch full profile XML |
| `POST` | `/JSSResource/osxconfigurationprofiles/id/0` | `ProfileClone` | Create a new profile (clone with new name, no scope) |
| `PUT` | `/JSSResource/osxconfigurationprofiles/id/{id}` | `ProfileSetXML`, `ProfileSetPayload`, `ProfileRedeploy` | Replace full profile XML |

### Notes

- All profile operations use the Classic API (XML). There is no v1/v2 profile endpoint in use.
- `--redeploy` sets `redeploy_on_update` to `All` in the XML before PUT, causing Jamf to push the profile to all currently-assigned devices rather than only new assignments.
- **Payload handling**: The profile XML contains a `<general><payloads>` element whose content is entity-encoded plist XML. Buddy parses this to support `get payloads`, `get payload <domain>`, and `list-payloads`. Both PayloadContent (PayloadIdentifier) and MCX-style (domain key + `mcx_preference_settings`) payloads are supported for get/set.
- **UUID preservation**: When updating a profile via `set xml` or `set payload`, the existing profile UUID is preserved (the fetched body is only modified in the payloads section or redeploy flag). Preserving the same UUID avoids duplicate or orphaned profiles on devices when updating from another instance.

---

## API Roles (Jamf Pro API v1)

Base path: `/api/v1/api-roles`

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET` | `/api/v1/api-roles` | `RoleList`, `RoleListValues` | List API roles (paginated; all pages by default) |
| `GET` | `/api/v1/api-roles/{id}` | `RoleGet` | Fetch a single API role |
| `POST` | `/api/v1/api-roles` | `RoleCreate` | Create a new API role |
| `PUT` | `/api/v1/api-roles/{id}` | `RoleUpdate` | Replace an API role's name and privilege list |
| `DELETE` | `/api/v1/api-roles/{id}` | `RoleDelete` | Delete an API role |
| `GET` | `/api/v1/api-role-privileges` | `RolePrivileges` | List all privilege strings available for assignment |

---

## API Integrations / Clients (Jamf Pro API v1)

Base path: `/api/v1/api-integrations`

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET` | `/api/v1/api-integrations` | `ClientList`, `ClientListValues` | List API integrations (paginated; all pages by default) |
| `GET` | `/api/v1/api-integrations/{id}` | `ClientGet`, `clientGetRaw` | Fetch a single API integration |
| `POST` | `/api/v1/api-integrations` | `ClientCreate` | Create a new API integration |
| `PUT` | `/api/v1/api-integrations/{id}` | `ClientUpdate`, `ClientSetTokenLifetime` | Update an API integration (name, scopes, token lifetime) |
| `DELETE` | `/api/v1/api-integrations/{id}` | `ClientDelete` | Delete an API integration |
| `POST` | `/api/v1/api-integrations/{id}/client-credentials` | `ClientCredentials` | Generate a new OAuth client secret (invalidates the previous one) |

### Notes

- `ClientUpdate` and `ClientSetTokenLifetime` both use a fetch-then-PUT pattern (`clientGetRaw` first) to preserve fields not being modified.
- The generated client secret from `client-credentials` is shown only once and cannot be retrieved again.

---

## Computers (Jamf Pro API v3/v2/v1)

Base paths: `/api/v3/computers-inventory`, `/api/v2/mdm/commands`, `/api/v1/jamf-management-framework/redeploy`

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET` | `/api/v3/computers-inventory` | `ComputerSearch`, `ComputerSearchPrint` | List computers (paginated; all pages by default); optional RSQL `filter` |
| `GET` | `/api/v3/computers-inventory/{id}` | `ComputerGet`, `ComputerGetValue`, `computerGetManagementID` | Fetch single computer (optionally `?section=GENERAL` for MDM commands) |
| `POST` | `/api/v2/mdm/commands` | `ComputerReboot`, `ComputerShutdown`, `ComputerLock`, `ComputerLogOut`, `ComputerErase`, `ComputerDeviceInfo`, `ComputerDeviceLocation`, `ComputerSecurityInfo`, `ComputerInstalledApps`, `ComputerManagedApps`, `ComputerCertificates` | Send MDM command (requires managementId from inventory) |
| `POST` | `/api/v1/jamf-management-framework/redeploy/{id}` | `ComputerRedeploy` | Trigger Jamf Management Framework redeploy on a computer |

### Notes

- Inventory stays on **v3** even though some cloud Jamf schemas already advertise v4 (and deprecate v3). Production Jamf does not support inventory v4 yet — do not migrate until it does (then remap RSQL/sort fields such as `general.lastContactTime` → `general.lastCheckIn` / `general.lastContact`).
- All MDM commands fetch each computer's `managementId` via `GET /api/v3/computers-inventory/{id}?section=GENERAL`, then POST to `/api/v2/mdm/commands` with `clientData: [{managementId, clientType: "COMPUTER"}]` and `commandData: {commandType: "..."}` plus optional fields.
- **Device control commands** (destructive; require confirmation unless `-y`): `RESTART_DEVICE`, `SHUT_DOWN_DEVICE`, `LOG_OUT_USER`. **DEVICE_LOCK** and **ERASE_DEVICE** require extra confirmation (LOCK: "Proceed?"; ERASE: type "erase" to confirm).
- **Information/query commands** (non-destructive; queued; results reported asynchronously by device): `DEVICE_INFORMATION`, `DEVICE_LOCATION`, `SECURITY_INFO`, `INSTALLED_APPLICATION_LIST`, `MANAGED_APPLICATION_LIST`, `CERTIFICATE_LIST`.
- `ComputerRedeploy` is a simple POST with no body; no v3 endpoint exists, so Buddy uses v1.
- RSQL filter examples: `general.name=="*Example*"`, `hardware.serialNumber=="C02EXAMPLE0001"`, `udid=="00000000-0000-4000-8000-000000000001"`.

---

## MDM Commands (Jamf Pro API v2)

Base path: `/api/v2/mdm/commands`

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET` | `/api/v2/mdm/commands` | `MdmCommandFind`, `MdmCommandFindPrint` | List MDM commands (paginated, RSQL filter required by API) |
| `POST` | `/api/v2/mdm/commands` | `computerSendMDMCommand` (computer-specific senders) | Queue an MDM command for one or more managed clients |

### Notes

- `GET /api/v2/mdm/commands` **requires at least one RSQL filter field** — the API rejects requests with no `filter` parameter. Buddy enforces this client-side with a non-empty check before the request is sent.
- The endpoint is paginated (`page` 0-based, `page-size` default 200). When fetching all pages, `MdmCommandFind` stops on a short page (`totalCount` from this API is not a reliable global total).
- `sort` defaults to `dateSent:asc` (API default). Supported sort fields match the RSQL filterable fields.
- Required privilege: `"View MDM command information in Jamf Pro API"`.
- Date fields (`dateSent`, `validAfter`, `dateCompleted`) require range operators (`=gt=`, `=lt=`, `=ge=`, `=le=`); equality (`==`) is not supported for dates.
- `commandState` values: `PENDING`, `ACKNOWLEDGED`, `NOT_NOW`, `ERROR`.
- `clientType` values: `COMPUTER`, `MOBILE_DEVICE`, `TV`, `VISION_PRO`, `WATCH`, `COMPUTER_USER`, `MOBILE_DEVICE_USER`.

### RSQL Filterable Fields

`uuid`, `clientManagementId`, `command`, `status`, `clientType`, `dateSent`, `validAfter`, `dateCompleted`, `profileId`, `profileIdentifier`, `active`

---

## Computer Groups (Jamf Pro API v3)

Base path: `/api/v3/computer-groups`

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET` | `/api/v3/computer-groups/smart-groups` | `GroupSearchSmart`, `GroupListSmart`, `GroupFindID` | List smart groups (paginated, RSQL filter on `id`, `name`, `siteId`) |
| `GET` | `/api/v3/computer-groups/static-groups` | `GroupSearchStatic`, `GroupListStatic` | List static groups (paginated, RSQL filter on `id`, `name`, `siteId`) |
| `GET` | `/api/v3/computer-groups/smart-groups/{id}` | `GroupGetVersion`, `GroupVersion`, `GroupDelete` | Fetch smart group detail (name, criteria) |
| `POST` | `/api/v3/computer-groups/smart-groups` | `GroupCreateSmart`, `GroupCreateSmartNamed` | Create a smart computer group |
| `PUT` | `/api/v3/computer-groups/smart-groups/{id}` | `GroupVersion` | Update smart group criteria (version value) |
| `DELETE` | `/api/v3/computer-groups/smart-groups/{id}` | `GroupDelete` | Delete a smart computer group |
| `POST` | `/api/v3/computer-groups/static-groups` | `GroupCreateStatic`, `GroupCreateStaticNamed` | Create a static computer group |
| `GET` | `/api/v3/computer-groups/static-groups/{id}` | `StaticGroupFetchMembers`, `StaticGroupPutAssignments` | Fetch static group (including assignments when present) |
| `PUT` | `/api/v3/computer-groups/static-groups/{id}` | `StaticGroupPutAssignments` | Update static group membership |

### Notes

- Both list endpoints are paginated (`page`, `page-size=200`) and support RSQL `filter` server-side. Plain text `--filter` values are matched client-side by name substring.
- Group `id` values are JSON strings (e.g. `"920"`), consistent with scripts and packages.
- `GroupFindID` searches smart groups only (used for version/scope operations on smart groups).

---

## Categories (Jamf Pro API v1)

Base path: `/api/v1/categories`

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET` | `/api/v1/categories` (paginated) | `PolicyCreateFromTemplate`, `PolicyCreatePackage`, `PolicyCreateSS` | List all categories; filtered client-side to resolve a category name to its numeric ID |

---

## Request Flow per Operation

```
buddy script list [--filter=<name>]
  └─ GET /api/v1/scripts                     (page-size=200; client-side name filter)

buddy script get <id>
  └─ GET /api/v1/scripts/{id}

buddy script get contents <id>
  └─ GET /api/v1/scripts/{id}

buddy script get id <name>
  └─ GET /api/v1/scripts                     (page-size=200; exact name match)

buddy script history <id>
  └─ GET /api/v1/scripts/{id}/history        (sort=date:desc&page-size=20)

buddy script create <name> <file> [flags]
  └─ POST /api/v1/scripts

buddy script update <id> <file>
  └─ GET /api/v1/scripts/{id}                (fetch current)
  └─ PUT /api/v1/scripts/{id}                (replace scriptContents)

buddy script delete <id> [id...]
  └─ GET /api/v1/scripts/{id}                (fetch name for confirmation, per ID)
  └─ DELETE /api/v1/scripts/{id}             (delete, per ID)

buddy script set <property> <id> <value>
  └─ GET /api/v1/scripts/{id}                (fetch current)
  └─ PUT /api/v1/scripts/{id}                (update with modified field)

buddy policy get --trigger=<trigger>
  └─ GET /JSSResource/policies              (list, cached)
  └─ GET /JSSResource/policies/id/{id}      (detail, per candidate, for exact trigger match)

buddy policy set ss <id> <version>
  └─ GET /JSSResource/policies/id/{id}      (fetch current)
  └─ PUT /JSSResource/policies/id/{id}      (update self-service description + optional scope)

buddy policy set install <id> <version>
  └─ GET /JSSResource/policies/id/{id}      (fetch current)
  └─ PUT /JSSResource/policies/id/{id}      (update script parameters)

buddy policy set package <app> <version> [policy-id] [--trigger=<trigger>] [--package=<pattern>]
  └─ GET /JSSResource/policies              (list, for source policy lookup; or PolicyFind when --trigger set)
  └─ GET /JSSResource/policies/id/{id}      (detail, for candidates)
  └─ GET /api/v1/packages                   (find matching package)
  └─ PUT /JSSResource/policies/id/{id}      (update package on source policy)
  └─ POST /JSSResource/policies/id/0        (create new versioned install policy, if needed)
  └─ PUT /JSSResource/policies/id/{id}      (disable old install policy)

  Use --trigger when the policy uses a non-standard trigger (e.g. filebeat-install instead of cache-filebeat-1.2.3).
  Use --package to override the default app+version search pattern. Supports:
    - Exact: "FalconSensorMacOS-7.32.20408.pkg"
    - Template: "FalconSensorMacOS-{version}.pkg" (substitutes version)
    - Regex: "re:FalconSensor.*\.pkg" (client-side regex filter)

buddy policy create package <app> <version> [--action=install|cache] [--scope=all|1,2,3] [--category=name] [--package=<pattern>]
  └─ GET /api/v1/packages                   (PackageFindID; --package overrides search pattern)
  └─ GET /JSSResource/policies              (uniquePolicyName)
  └─ GET /api/v1/categories                  (if --category set)
  └─ POST /JSSResource/policies/id/0

buddy policy create ss <trigger> <display-name> [--category=name]
  └─ GET /api/v1/categories                  (if --category set)
  └─ POST /JSSResource/policies/id/0

buddy policy clone <id> <version> [--package=<pattern>]
  └─ GET /JSSResource/policies/id/{id}      (fetch source)
  └─ GET /JSSResource/policies              (uniqueness check for new name)
  └─ POST /JSSResource/policies/id/0        (create new policy)
  └─ PUT /JSSResource/policies/id/{id}      (disable source)

buddy policy delete <id> [id ...]
  └─ GET /JSSResource/policies/id/{id}      (fetch name for confirmation, per ID)
  └─ DELETE /JSSResource/policies/id/{id}   (delete, per ID)

buddy policy find <keyword> [keyword...] [--exclude-ids=1,2,3]
  └─ GET /JSSResource/policies             (or cached)

buddy policy set scope <id> <group-id | all>
  └─ GET /JSSResource/policies/id/{id}
  └─ PUT /JSSResource/policies/id/{id}

buddy policy set trigger <id> <trigger>
  └─ GET /JSSResource/policies/id/{id}
  └─ PUT /JSSResource/policies/id/{id}

buddy policy set frequency <id> <frequency>
  └─ GET /JSSResource/policies/id/{id}
  └─ PUT /JSSResource/policies/id/{id}

buddy policy set enable/disable <id>
  └─ GET /JSSResource/policies/id/{id}
  └─ PUT /JSSResource/policies/id/{id}

buddy policy set xml <id>                   (XML piped from stdin)
  └─ PUT /JSSResource/policies/id/{id}      (raw XML PUT, no prior GET)

buddy group list smart [--filter=<rsql>]
  └─ GET /api/v3/computer-groups/smart-groups       (paginated; RSQL or client-side substring filter)

buddy group list static [--filter=<rsql>]
  └─ GET /api/v3/computer-groups/static-groups      (paginated; RSQL or client-side substring filter)

buddy group get version <group-id>
  └─ GET /api/v3/computer-groups/smart-groups/{id}

buddy group set version <group-id> <version>
  └─ GET /api/v3/computer-groups/smart-groups/{id}
  └─ PUT /api/v3/computer-groups/smart-groups/{id}

buddy group delete <group-id>
  └─ GET /api/v3/computer-groups/smart-groups/{id}
  └─ DELETE /api/v3/computer-groups/smart-groups/{id}

buddy group create smart …
  └─ POST /api/v3/computer-groups/smart-groups

buddy group create static …
  └─ POST /api/v3/computer-groups/static-groups

step_create_policy (template-based)
  └─ GET /api/v1/categories                 (only if category field is set)
  └─ POST /JSSResource/policies/id/0

buddy profile list [--filter=name]
  └─ GET /JSSResource/osxconfigurationprofiles

buddy profile get xml <id>
  └─ GET /JSSResource/osxconfigurationprofiles/id/{id}

buddy profile get payloads <id>
  └─ GET /JSSResource/osxconfigurationprofiles/id/{id}   (extract and output payloads plist)

buddy profile get payload <id> <domain>
  └─ GET /JSSResource/osxconfigurationprofiles/id/{id}   (extract single payload by PayloadIdentifier)

buddy profile get list-payloads <id>
  └─ GET /JSSResource/osxconfigurationprofiles/id/{id}   (list PayloadIdentifier values)

buddy profile clone <profile-id> <name>
  └─ GET /JSSResource/osxconfigurationprofiles/id/{id}
  └─ POST /JSSResource/osxconfigurationprofiles/id/0

buddy profile redeploy <id>
  └─ GET /JSSResource/osxconfigurationprofiles/id/{id}   (fetch current XML)
  └─ PUT /JSSResource/osxconfigurationprofiles/id/{id}   (set redeploy_on_update=All)

buddy profile set xml <id> [--redeploy]
  └─ PUT /JSSResource/osxconfigurationprofiles/id/{id}   (raw XML from stdin)

buddy profile set payload <id> <domain> <file> [--redeploy]
  └─ GET /JSSResource/osxconfigurationprofiles/id/{id}   (fetch current XML)
  └─ PUT /JSSResource/osxconfigurationprofiles/id/{id}   (replace with updated payload)

buddy package list [--filter=<rsql>]
  └─ GET /api/v1/packages

buddy package upload <file>
  └─ POST /api/v1/packages              (create metadata)
  └─ POST /api/v1/packages/{id}/upload  (multipart file upload)

buddy package get <id>
  └─ GET /api/v1/packages/{id}
  └─ GET /api/v1/packages/{id}/history       (sort=date:desc&page-size=1, for lastChanged/lastChangedBy)

buddy package history <id>
  └─ GET /api/v1/packages/{id}/history       (sort=date:desc&page-size=20)

buddy package delete <id> [id2 ...]
  └─ DELETE /api/v1/packages/{id}            (single ID)
  └─ POST /api/v1/packages/delete-multiple   (multiple IDs)

buddy package set <property> <id> <value>
  └─ GET /api/v1/packages/{id}               (fetch full record)
  └─ PUT /api/v1/packages/{id}               (update with modified field)

buddy computer find [--filter=<rsql>]
  └─ GET /api/v3/computers-inventory          (optional filter, page-size=200)

buddy computer get <id>
  └─ GET /api/v3/computers-inventory/{id}

buddy computer reboot <id> [id...]
  └─ GET /api/v3/computers-inventory/{id}?section=GENERAL  (per ID, for managementId)
  └─ POST /api/v2/mdm/commands                (RESTART_DEVICE payload, per ID)

buddy computer shutdown <id> [id...]
  └─ GET /api/v3/computers-inventory/{id}?section=GENERAL
  └─ POST /api/v2/mdm/commands                (SHUT_DOWN_DEVICE)

buddy computer lock <id> [id...] [--message=...] [--phone=...] [--pin=...]
  └─ GET /api/v3/computers-inventory/{id}?section=GENERAL
  └─ POST /api/v2/mdm/commands                (DEVICE_LOCK; extra confirm)

buddy computer logout <id> [id...]
  └─ GET /api/v3/computers-inventory/{id}?section=GENERAL
  └─ POST /api/v2/mdm/commands                (LOG_OUT_USER)

buddy computer erase <id> [id...] [--pin=...] [--obliteration=...]
  └─ GET /api/v3/computers-inventory/{id}?section=GENERAL
  └─ POST /api/v2/mdm/commands                (ERASE_DEVICE; type "erase" to confirm)

buddy computer device-info <id> [id...] [--queries=...]
buddy computer device-location <id> [id...]
buddy computer security-info <id> [id...]
buddy computer installed-apps <id> [id...] [--identifiers=...]
buddy computer managed-apps <id> [id...] [--identifiers=...]
buddy computer certificates <id> [id...]
  └─ GET /api/v3/computers-inventory/{id}?section=GENERAL
  └─ POST /api/v2/mdm/commands                (respective commandType; queued, results async)

buddy computer redeploy <id> [id...]
  └─ POST /api/v1/jamf-management-framework/redeploy/{id}  (per ID)

buddy mdm find --filter=<rsql> [--sort=<expr>]
  └─ GET /api/v2/mdm/commands          (paginated; RSQL filter required by API)

buddy role list
  └─ GET /api/v1/api-roles

buddy role get <id>
  └─ GET /api/v1/api-roles/{id}

buddy role create <name> [priv...]
  └─ POST /api/v1/api-roles

buddy role update <id> <name> [priv...]
  └─ PUT /api/v1/api-roles/{id}

buddy role delete <id>
  └─ DELETE /api/v1/api-roles/{id}

buddy role privileges
  └─ GET /api/v1/api-role-privileges

buddy client list
  └─ GET /api/v1/api-integrations

buddy client get <id>
  └─ GET /api/v1/api-integrations/{id}

buddy client create <name> [scope...]
  └─ POST /api/v1/api-integrations

buddy client update <id> <name> [scope...]
  └─ GET /api/v1/api-integrations/{id}       (preserve existing fields)
  └─ PUT /api/v1/api-integrations/{id}

buddy client delete <id>
  └─ DELETE /api/v1/api-integrations/{id}

buddy client credentials <id>
  └─ POST /api/v1/api-integrations/{id}/client-credentials

buddy client set token-lifetime <id> <seconds>
  └─ GET /api/v1/api-integrations/{id}       (preserve existing fields)
  └─ PUT /api/v1/api-integrations/{id}
```

---

## RSQL Filter Syntax

Several Jamf Pro API v1/v2 endpoints accept a `filter` query parameter using RSQL (RESTful Service Query Language). Buddy passes these through directly on commands like `buddy package list --filter=...`.

### Operators

| Operator | Meaning | Example |
|---|---|---|
| `==` | equals (supports `*` glob wildcards) | `packageName=="*ExampleApp*"` |
| `!=` | not equals | `packageName!="*beta*"` |
| `=lt=` | less than | `id=lt=100` |
| `=gt=` | greater than | `id=gt=50` |
| `=le=` | less than or equal | `id=le=100` |
| `=ge=` | greater than or equal | `id=ge=50` |
| `=in=` | value in list | `id=in=(1,2,3)` |
| `=out=` | value not in list | `id=out=(99,100)` |
| `;` | AND (both conditions must match) | `packageName=="*ExampleApp*";fileName=="*arm64*"` |
| `,` | OR (either condition matches) | `packageName=="*ExampleApp*",packageName=="*OtherApp*"` |

### Examples

```bash
# All packages whose name contains a placeholder product name
buddy package list --filter='packageName=="*ExampleApp*"'

# ExampleApp packages built for Apple Silicon
buddy package list --filter='packageName=="*ExampleApp*";fileName=="*arm64*"'

# Exclude beta/test packages
buddy package list --filter='packageName=="*ExampleApp*";packageName!="*beta*"'

# Multiple apps
buddy package list --filter='packageName=="*ExampleApp*",packageName=="*OtherApp*"'

# Packages ready in cloud distribution
buddy package list --filter='cloudTransferStatus=="READY"'

# Packages by specific IDs
buddy package list --filter='id=in=(10,11,12)'

# API integrations by name
buddy client list --filter='displayName=="*CI*"'

# API roles by name
buddy role list  # (role list does not currently support --filter; use get <id>)

# Computers by name, serial, or UDID
buddy computer find --filter='general.name=="*Example*"'
buddy computer find --filter='hardware.serialNumber=="C02EXAMPLE0001"'
buddy computer find --filter='udid=="00000000-0000-4000-8000-000000000001"'

# MDM commands by status
buddy mdm find --filter='status==Pending'
buddy mdm find --filter='clientManagementId==00000000-0000-4000-8000-000000000002;status==Pending'

# MDM commands completed after a date (date fields require range operators)
buddy mdm find --filter='dateCompleted=ge=2024-01-01T00:00:00Z;status==Acknowledged'
```

In Starlark scripts, prefer the `rsql_*` builders (see `buddy help builtins`) so dates and quoting stay correct:

```python
since = rsql_days_ago(7)["value"]
filt = rsql_and([
    rsql_ge("dateCompleted", since)["value"],
    rsql_eq("status", "Acknowledged")["value"],
])["value"]
mdm_commands(filter=filt)
```

### Filterable Fields by Resource

| Resource | Filterable fields |
|---|---|
| `/v1/packages` | `id`, `fileName`, `packageName`, `categoryId`, `info`, `notes`, `manifestFileName`, `cloudTransferStatus` |
| `/v3/computers-inventory` | `id`, `udid`, `general.name`, `hardware.serialNumber`, `general.managementId`, etc. |
| `/v2/mdm/commands` | `uuid`, `clientManagementId`, `command`, `status`, `clientType`, `dateSent`, `validAfter`, `dateCompleted`, `profileId`, `profileIdentifier`, `active` |
| `/v1/api-integrations` | `id`, `displayName` |
| `/v1/api-roles` | `id`, `displayName` |

### Notes

- Wildcard `*` is only meaningful inside string equality (`==` / `!=`).
- String values are case-insensitive in Jamf's RSQL implementation.
- Always quote the filter value in the shell to prevent glob expansion: `--filter='...'`.
- The RSQL grammar is specified at [github.com/jirutka/rsql-parser](https://github.com/jirutka/rsql-parser).

---

## Base URLs

Base URLs are configured per credential in `~/.config/buddy/config.yaml`. Each named credential specifies its own `url` field. There is no hardcoded URL logic.

See `buddy cred help` for how to add or update credentials.
