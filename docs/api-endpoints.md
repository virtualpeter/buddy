# Jamf Pro API Endpoints Used by Buddy

For CLI scripting and configuration, start with the [reference manual](reference-manual.md).

Buddy communicates with Jamf through direct HTTP requests (no third-party SDK).

- **Classic API** — XML-based, path prefix `/JSSResource/`
- **Jamf Pro API** — JSON-based, path prefix `/api/v1/` or `/api/v2/` / `/api/v3/`
- **Platform API Gateway** — first-class surface for blueprints, devices, device groups, DDM, compliance benchmarks, and organization Account APIs (licenses). Set `region` (currently `us`, `eu`, `apac`; the list may grow) and/or gateway `url` `https://{region}.api.jamfcloud.com`, plus OAuth client credentials. Scope is **`id` / `environmentId` (preferred)** or `jamfpro.tenantId` on the **buddy context** (the named workplace `--context` selects — not Jamf’s environment). Organization-scoped integrations use a platform auth with `scope: organization`: same US gateway (`https://us.api.jamfcloud.com`), no `X-Environment-Id` / `X-Tenant-Id`. There is no `api.jamfcloud.com` host (it does not resolve). The org workplace YAML has no region. Optional nested `orgs:` YAML expands into those same workplaces. Scope lives on the **auth**, so a context may hold both an environment `id` and a product `tenantId`. The retired beta host `*.apigw.jamf.com` is refused. See [Auth](auth.md#contexts-and-auths).

Call sites still use instance-relative paths (`/api/v1/…`, `/JSSResource/…`). On a platform session buddy rewrites them and sends scope in a header (`X-Environment-Id` or `X-Tenant-Id`), not a path segment:

| Caller path | Gateway path |
|-------------|--------------|
| `/api/vN/rest…` | `/pro/vN/rest…` |
| `/JSSResource/…` | `/proclassic/…` (the `/JSSResource` prefix is stripped) |
| `/api/blueprints/vN/…` | `/blueprints/vN/…` (drop `/api`; same for devices, device-groups, licensing, sso, …) |

**Limitation (Classic / proclassic):** Classic-on-Platform is `/proclassic/…` with `/JSSResource` stripped (Jamf’s Classic OpenAPI). Buddy sends Classic through the gateway when the session is platform (`features.classic_instance_only` default **false**). Auto-pick among mixed auths still prefers instance. Set `features.classic_instance_only: true` (or `BUDDY_CLASSIC_INSTANCE_ONLY=true`) to force Classic onto an instance auth. A gateway `403` means the client lacks Classic privileges, not a bad path.

That toggle does **not** control ACS or the undocumented Pro policy list — those never work on the gateway (see ACS and Policies notes below). If a Platform OAuth client lacks Classic scopes, pin an instance auth or set `classic_instance_only: true`.

Authentication is config-driven per context + auth (command, basic, or OAuth). See **[Auth, secrets, and tokens](auth.md)**, `buddy auth help`, and `buddy cred help`. For **basic** (username/password), buddy obtains a Bearer token via `POST /api/v1/auth/token` with Basic auth, then uses that token for all API requests. Platform credentials use **OAuth only** (`POST {gateway}/auth/token`, fallback `/oauth2/token`); there is no keep-alive.

### Custom calls from buddyscript

For endpoints without a typed builtin, scripts can call:

- `jamf_get` / `jamf_post` / `jamf_put` / `jamf_patch` / `jamf_delete`
- or `jamf_request(method, path, …)`

Paths are host-relative (`/api/v1/…` or `/JSSResource/…`). Media-type defaults follow the path (JSON for `/api/…`, XML for `/JSSResource/…` and gateway `/proclassic/…`). See `buddy help builtins` (section **JAMF API**).

### API schema documents

Each Jamf **instance** host publishes its live Pro/Classic specs (use `jamf_schema` from buddyscript). The **Platform API Gateway** does **not** serve a single live schema, so buddy ships a snapshot of the per-service Platform specs embedded in the binary:

| API | `api=` | Source | Format |
|-----|--------|--------|--------|
| Jamf Pro API | `pro` | live `/api/schema` | OpenAPI 3 JSON |
| Classic API | `classic` | live `/classicapi/doc/swagger.yaml` | Swagger 2 YAML |
| Platform API Gateway | `platform` | embedded specs (blueprints, devices, device-groups, device-actions, ddm/report, compliance-benchmarks) | OpenAPI 3.x JSON |

`api="both"` searches pro+classic; `api="all"` (the default) also includes platform. Platform paths are prefixed with each service's gateway base (e.g. `/blueprints/v1/blueprints`) and tagged with the service segment, so `jamf_schema(filter="blueprints deploy")` finds them. `api="platform"` needs no credential.

**Authentication:** none of these schema documents require authentication. The Pro `/api/schema` and Classic `/classicapi/doc/swagger.yaml` endpoints are served publicly by the Jamf host (only *data* endpoints need a bearer token), and the Platform specs are embedded in the binary. buddy fetches Pro/Classic schemas with **no `Authorization` header**, using the credential only to resolve the host base URL — so schema discovery works even when the credential has no stored secret.

Example: `jamf_schema(filter="policy delete")` searches Pro, Classic, and Platform (default `api="all"`). MCP exposes the same helper as tool `jamf_schema`.

Official discovery is [Jamf’s Platform API docs](https://developer.jamf.com/platform-api) and the remote MCP at `https://developer.jamf.com/mcp` (the six native product services: blueprints, devices, device-groups, device-actions, ddm/report, compliance-benchmarks, plus Account APIs such as licensing and SSO). Those are HTML / MCP query surfaces — they do **not** publish a downloadable OpenAPI document for the six product services. Authenticated GETs of `{gateway}/schema` and `/openapi.json` are **404**; per-service `/openapi.json` is **403** (unknown path, not a missing docs privilege). buddy therefore vendors the per-service OAS JSON from [`Jamf-Concepts/jamfplatform-go-sdk`](https://github.com/Jamf-Concepts/jamfplatform-go-sdk/tree/main/api) (MIT; jamf-cli `specs/platform/` is a copy of that tree) into `internal/buddy/platformschema/specs/` so `jamf_schema(api="platform")` works offline with no credential. Refresh with `make sync-platform-schema` and commit the result. Account licensing is called live (`GET /licensing/v1/licenses`); it is **not** in that embed.

Do not vendor the MCP’s `platform-api=Jamf Pro API` / `Classic API` copies — those are gateway-rewritten instance APIs; buddy already loads them live from the Jamf Pro host. Keep the SDK embed until Jamf publishes a live schema URL.

### Pagination

Jamf Pro JSON list endpoints that return `{totalCount, results}` are fetched via `fetchJamfPaged` (`internal/buddy/pagination.go`):

- Query params: `page` (0-based) and `page-size` (default **200**).
- CLI table printers and internal resolvers (find-by-name, etc.) request **all pages**.
- buddyscript list builtins expose `page?` / `page_size?`; **omit `page`** to fetch every page and return the combined list.

Applies to scripts, packages, computers inventory, smart/static group lists, API roles, API integrations, and blueprints. MDM command list uses the same `page` / `page_size` options but ends on a short page (that API’s `totalCount` is not a reliable global total).

### Contents

- [Blueprints](#blueprints-platform-api-gateway)
- [Account licenses](#account-licenses-licensing)
- [Platform devices](#platform-devices-apidevices)
- [Platform device actions](#platform-device-actions-apidevice-actions)
- [Platform device groups](#platform-device-groups-apidevice-groups)
- [DDM declaration reporting](#ddm-declaration-reporting-apiddmreport)
- [Compliance benchmarks](#compliance-benchmarks-apicompliance-benchmarks)
- [Scripts](#scripts-jamf-pro-api-v1)
- [Policies](#policies-classic-api)
- [Packages](#packages-jamf-pro-api-v1)
- [Icons](#icons-jamf-pro-api-v1)
- [Mac App Store apps](#mac-app-store-apps-classic-api)
- [Configuration profiles](#configuration-profiles-classic-api)
- [API roles](#api-roles-jamf-pro-api-v1)
- [API integrations / clients](#api-integrations--clients-jamf-pro-api-v1)
- [Computers](#computers-jamf-pro-api-v4v2v1)
- [Computer history](#computer-history-classic-api)
- [MDM commands](#mdm-commands-jamf-pro-api-v2)
- [Mobile devices](#mobile-devices-jamf-pro-api-v2)
- [Mobile-device groups](#mobile-device-groups-jamf-pro-api-v2)
- [Mobile profiles](#mobile-profiles-classic-api)
- [Computer groups](#computer-groups-jamf-pro-api-v3)
- [Categories](#categories-jamf-pro-api-v1)
- [Request flow](#request-flow-per-operation)
- [RSQL](#rsql-filter-syntax)
- [Base URLs](#base-urls)

---

## Blueprints (Platform API Gateway)

Base path (logical): `/api/blueprints/v1/blueprints`  
Rewritten: `/blueprints/v1/blueprints` plus `X-Environment-Id` or `X-Tenant-Id`

Requires a **buddy context** with Jamf Account environment `id` (preferred) or `jamfpro.tenantId`, plus `region` (or gateway `url`, e.g. `https://us.api.jamfcloud.com`) and a linked `type: platform` auth. CLI: `buddy blueprint …`. buddyscript: `blueprints` / `blueprint` / `blueprint_*`.

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

## Account licenses (`/licensing`)

Organization-scoped Jamf Account API. Logical path `/api/licensing/v1/licenses` rewrites to `/licensing/v1/licenses` on `https://us.api.jamfcloud.com`. The org is taken from the access token; **no** `X-Environment-Id` / `X-Tenant-Id`. Not paginated (one GET; empty org is still **200** `{totalCount:0, results:[]}`). Privilege: licensing on the org integration.

CLI: `buddy license list`. buddyscript: `licenses()`. `buddy auth test` on an organization workplace uses this GET as the live probe (200 → `live=ok`; 403 → `live=token-ok`).

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET` | `/api/licensing/v1/licenses` | `LicenseList`, `licenses`, `auth test` | List licenses for the organization |

When the current context is an environment or product tenant with `org:`, `buddy license list` / `licenses()` use that parent organization workplace automatically. `buddy auth test` stays on the selected workplace. Do not rewrite this path to `/pro/…`. Environment / product-tenant Platform APIs (blueprints, devices, …) cannot run on an org workplace.

---

## Platform devices (`/api/devices`)

Logical paths drop `/api` on the gateway (same as blueprints). Scope is a header. Distinct from Jamf Pro `computer` inventory.

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

Read-only. CLI: `buddy ddm …`. buddyscript: `ddm_*`.

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET` | `…/declarations/{id}/devices` | `DDMDeclarationDevices` | Devices for declaration |
| `GET` | `…/devices/{id}/declarations` | `DDMDeviceDeclarations` | Declarations for device |
| `GET` | `…/devices/{id}/channels` | `DDMDeviceChannels` | Channels for device |

## Compliance benchmarks (`/api/compliance-benchmarks`)

CLI: `buddy benchmark …`. buddyscript: `benchmark` / `benchmark_*`.

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
| `GET` | `/api/v1/scripts` | `ScriptSearch`, `ScriptList`, `ScriptGetID` | List scripts (paginated; all pages by default); bare filter → `name=="*term*"` server-side, RSQL = pass-through |
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
- Jamf’s list OpenAPI model may include `scriptContents` on each row; buddy’s `ScriptSearch` / buddyscript `scripts()` strip bodies and return `{id,name,categoryId,categoryName}` only. Use `GET /api/v1/scripts/{id}` (`script(id=…)`, `script_contents`) for the script body.

---

## Extension attributes (Jamf Pro API v1)

### Computer definitions

Base path: `/api/v1/computer-extension-attributes`

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET` | `/api/v1/computer-extension-attributes` | `ExtensionAttributeSearch`, `ExtensionAttributeList`, `ExtensionAttributeGetID` | List EAs (paginated); bare filter → `name=="*term*"`; list strips `scriptContents` |
| `GET` | `/api/v1/computer-extension-attributes/{id}` | `extensionAttributeGetFull`, `ExtensionAttributeGet`, `ExtensionAttributeUpdate`, `ExtensionAttributeSet*` | Fetch full EA including `scriptContents` when SCRIPT |
| `POST` | `/api/v1/computer-extension-attributes` | `ExtensionAttributePostDetail`, `ExtensionAttributeCreate` | Create EA (SCRIPT/TEXT/POPUP/LDAP) |
| `PUT` | `/api/v1/computer-extension-attributes/{id}` | `extensionAttributePut`, `ExtensionAttributePutFields`, `ExtensionAttributeSet*` | Replace EA record |
| `DELETE` | `/api/v1/computer-extension-attributes/{id}` | `extensionAttributeDeleteOne` | Delete one EA |
| `POST` | `/api/v1/computer-extension-attributes/delete-multiple` | `ExtensionAttributeDelete` (multi) | Bulk delete by ids |
| `GET` | `/api/v1/computer-extension-attributes/{id}/history` | `ExtensionAttributeHistory` | Audit history |
| `GET` | `/api/v1/computer-extension-attributes/{id}/data-dependency` | `ExtensionAttributeDependencies` | Smart group / advanced search dependents |
| `GET` | `/api/v1/computer-extension-attributes/templates` | `ExtensionAttributeTemplateSearch` | Jamf Nation templates |
| `GET` | `/api/v1/computer-extension-attributes/templates/{id}` | `ExtensionAttributeFetchTemplate` | One template (EA-shaped body) |
| `POST` | `/api/v1/computer-extension-attributes/upload` | `ExtensionAttributeUploadXML` | Multipart XML import |
| `GET` | `/api/v1/computer-extension-attributes/{id}/download` | `ExtensionAttributeDownloadXML` | XML export |

### Mobile definitions

Base path: `/api/v1/mobile-device-extension-attributes`

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET` | `/api/v1/mobile-device-extension-attributes` | `MobileExtensionAttributeSearch` | List mobile EAs (no SCRIPT) |
| `GET` | `/api/v1/mobile-device-extension-attributes/{id}` | `mobileExtensionAttributeGetFull` | Fetch one |
| `POST` | `/api/v1/mobile-device-extension-attributes` | `MobileExtensionAttributePostDetail` | Create |
| `PUT` | `/api/v1/mobile-device-extension-attributes/{id}` | `mobileExtensionAttributePut` | Update |
| `DELETE` | `/api/v1/mobile-device-extension-attributes/{id}` | `MobileExtensionAttributeDelete` | Delete |

### Inventory values (computers)

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET` | `/api/v4/computers-inventory/{id}?section=EXTENSION_ATTRIBUTES` | `ComputerFetchExtensionAttributes` | Read EA values on a Mac |
| `PATCH` | `/api/v4/computers-inventory-detail/{id}` | `ComputerSetExtensionAttribute` | Set TEXT/POPUP values (`extensionAttributes: [{definitionId, values}]`) |

### Notes

- Computer EA list summaries never include `scriptContents` (same rule as scripts).
- SCRIPT and LDAP inventory values are not writable; TEXT/POPUP are.
- Migrate order: categories → scripts → **extension attributes** → smart groups → policies.
- Classic `/computerextensionattributes` XML is not used for CRUD.

---

## Policies (Classic API)

Base path: `/JSSResource/policies`

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET` | `/JSSResource/policies` | `PolicyFind`, `PolicyFindByKeywords`, `PolicyPackage`, `PolicyInstall`, `AppSearch` (fallback) | Fetch full policy list (id + name) for trigger/name matching and cache |
| `GET` | `/JSSResource/policies/id/{id}` | most operations | Fetch full policy detail (triggers, scripts, scope, self-service, etc.) |
| `PUT` | `/JSSResource/policies/id/{id}` | `PolicySS`, `PolicyScope`, `PolicyScopeAll`, `PolicyEnable`, `PolicyTrigger`, `PolicyInstall`, `PolicyPackage`, `PolicySetXML`, `PolicyClone` (disable source) | Update an existing policy in place |
| `POST` | `/JSSResource/policies/id/0` | `PolicyClone`, `PolicyPackage` (new install policy), `PolicyCreateFromTemplate`, `PolicyCreatePackage`, `PolicyCreateSS` | Create a new policy (Jamf convention: POST to id/0) |
| `DELETE` | `/JSSResource/policies/id/{id}` | `PolicyDelete` | Delete a policy by ID |

### Notes

- **Hybrid list (undocumented Pro):** `getPolicyList` prefers `GET /api/v1/policies` when an instance session can be resolved (paginated `results`/`totalCount`; summary: `id`, `name`, `enabled`, `categoryId`, `executionFrequency`, `triggerCheckin`, `triggerManual`, `updateInventory`). Environment-only workplaces (no Jamf Pro URL) skip that probe. App scan and the policy payload cache (`app_scan`, `buddy app cache refresh`) are the exception: if the current workplace is an environment with a Jamf Pro product-tenant child that has instance auth, they relocate to that child for the v1 index (and XML cache). On v1 failure or empty, falls back to Classic. Not in published OpenAPI; not used for writes or detail. RSQL filter fields are those keys only — **no** Self Service boolean. The Platform gateway returns **403** for this path; buddy never sends it through `/pro/`. That is **not** `features.classic_instance_only` (that flag is Classic `/JSSResource` only). Classic fallback may still use `/proclassic/policies`.
- `GET /JSSResource/policies` returns a summary list `{id, name}` only; detail fields (trigger, scripts, scope) require a subsequent `GET /JSSResource/policies/id/{id}`.
- Buddy caches the summary list in process memory per resolved context+auth session (~60s TTL) so multi-step scripts and change batches don't re-fetch it for every trigger lookup. Policy creates/deletes in the same process invalidate the entry; there is no on-disk cache.
- Phase 1 of trigger resolution fetches detail for a small set of name-matched candidate policies to verify `trigger_other` exactly, rather than relying on name heuristics alone.

---

## Packages (Jamf Pro API v1)

Base path: `/api/v1/packages`

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET` | `/api/v1/packages` | `PackageList`, `PackageSearch`, `PackageFindID` | List packages; supports optional RSQL `filter` query param |
| `GET` | `/api/v1/packages/{id}` | `PackageGet`, `PackageFetchFull`, `PackageReady`, `PackageWaitReady`, `PackageDownload` | Fetch single package record (includes `cloudTransferStatus`, checksums) |
| `POST` | `/api/v1/packages` | `PackageUpload` | Create new package record (metadata) |
| `POST` | `/api/v1/packages/{id}/upload` | `PackageUpload` | Upload package file (multipart/form-data) |
| `DELETE` | `/api/v1/packages/{id}` | `PackageDelete` (single) | Delete a single package record |
| `POST` | `/api/v1/packages/delete-multiple` | `PackageDelete` (batch) | Delete multiple packages by ID array |
| `PUT` | `/api/v1/packages/{id}` | `PackageSetAttrs` (`packagePut`) | Update package fields: name, notes, info, priority, category, rebootRequired |
| `GET` | `/api/v1/packages/{id}/history` | `PackageGet` (last-changed enrichment), `PackageHistory`, `packageGetLatestHistory` | Fetch package audit history; sorted `date:desc` |
| `GET` | `/api/v1/jcds/files/{fileName}` | `PackageDownload` | JCDS pre-signed download URI (`{"uri":…}`) |
| `POST` | `/api/v1/jcds/refresh-inventory?file-name=…` | `PackageWaitReady` / `PackageDownload` wait | Best-effort nudge while waiting for READY |

### Notes

#### List / get / READY

- `PackageGet` prints the full v1 package object (including `md5` / `sha256` / `sha3512` / `hashType` / `hashValue` / `size` / `cloudTransferStatus` when present), then adds `lastChanged` / `lastChangedBy` from `history?sort=date:desc&page-size=1`.
- `cloudTransferStatus` is server-managed. Jamf’s UI “Availability Pending” clears when the value is `"READY"` (case-sensitive). Anything else (including empty) means not ready for policy use / JCDS download. A successful `GET /api/v1/packages/{id}` alone does **not** mean the package is usable. While transfer is pending, `sha256` is often blank while `hashValue` is already set (typically `hashType` SHA3_512) — that is **not** a signal to re-upload. A record with **no `hashValue`** (and no comparable md5/sha256) usually means the multipart upload failed after the metadata create — re-upload is appropriate (`--force` / `force=True` overrides skip/not-ready guards).
- `PackageReady` / buddyscript `package_ready` return true only when the package exists, `cloudTransferStatus == "READY"`, and `sha256` is non-empty (404 → false). `PackageWaitReady` / `package_wait_ready` poll for both with backoff (default 10 minutes) and best-effort POST `jcds/refresh-inventory?file-name=…` between polls. Use after upload / before attaching the package to a policy.

#### Download

- `PackageDownload` / `package_download` / `buddy package download`: wait until READY **and** non-empty `sha256` → GET JCDS pre-signed URI → stream to a temp file beside the destination while hashing SHA-256 → rename only on match; delete temp on failure; retry once on URI HTTP 403. **JCDS only** (Jamf Cloud); not for on-prem non-JCDS distribution points. Verify is always on (no skip flag). Do not invent this loop from OpenAPI alone — call the builtin.

#### Upload

- `PackageUpload` looks up `fileName` first. If a record exists, it GETs the full object, compares local MD5/SHA-256 to server checksums, and **skips** the file transfer when they match; if comparable checksums are empty, status is not READY, and `hashValue` is set, `--wait-ready` / `wait_ready=True` **skips the file transfer and polls until READY** (re-run after an upload that omitted the flag); without wait-ready that case **errors as not ready** instead of treating blank hashes as “needs upload”; if there is **no `hashValue`**, it treats the record as a failed upload and streams a replacement; `--force` / `force=True` re-uploads regardless. Metadata flags (`--name`, `--notes`, `--info`, `--category`, `--priority`, `--reboot`) apply on create and on update when they differ (including metadata-only updates when the file is unchanged). If no record exists, it POSTs metadata (no confirm prompt), polls GET until the ID is readable (Jamf Cloud create is async), **waits 5s** so the upload endpoint can catch up, then streams the multipart upload. After upload, scripts should use `package_wait_ready` before attaching the package to a policy.

#### Scan / set / find / delete

- `PackageScan` / `packages_scan` / `buddy package scan` pages the packages list and classifies incomplete records from list fields: `failed-upload` (no `hashValue` and no md5/sha256), `pending` (`hashValue` set, not READY), `unusable-ready` (READY but empty `sha256`). Optional `--class=` / `class=` narrows; default is all three.
- `packagePut` / `PackageSetAttrs` (used by `buddy package set <id> --…`) omit server-managed `size` / `cloudTransferStatus` and client-only history fields from the PUT body so a metadata update does not blank Jamf’s size.
- `PackageFindID` is shared between the CLI (`policy set package`) and buddyscript (`package(name=..., version=...)` builtin). List/search walks all pages unless a single `page` is requested from buddyscript.
- The `delete-multiple` body is `{"ids": ["1","2",...]}`.

---

## Icons (Jamf Pro API v1)

Base path: `/api/v1/icon`

There is **no list** endpoint (`GET /api/v1/icon` is 405) and no published delete. Icon ids are per-tenant. Buddy lists icons by walking `GET /api/v1/icon/{id}` (404s are holes) and storing a weekly JSON catalog at `~/.config/buddy/catalog-cache/<tenant>/icons.json` (`fetched_at` TTL 7 days; two auths on the same tenant share the file). A missing cache walks from id 1 until 20 consecutive 404s past the last hit. A stale cache or `--refresh` / `refresh=True` keeps cached rows and only walks 20 ids past the previous high-water mark (grow-on-hit). `--dropcache` / `dropcache=True` ignores the catalog and walks from 1 again. `POST /api/v1/icon` upserts that row and bumps `max_id` without resetting `fetched_at`. This is not a Jamf collection API — do not invent pagination, and do not scrape Classic `policies.html`. ICS stores image bytes by SHA-256 (`/icon/hash_<hex>`); a successful ICS HEAD does not mint a tenant id.

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET` | `/api/v1/icon/{id}` | `IconGet`, `icon(id=)`, `IconList` / `icons()` (id-walk) | Metadata `{id, name, url}` (`name` is live even when OpenAPI omits it). `url` is ICS (`…/icon/hash_<sha256>`) or the instance download path (Jamf may emit a double slash after the host). |
| `POST` | `/api/v1/icon` | `IconUpload`, `icon_upload`, `policy_copy` (cross-tenant remap) | Multipart field `file`; returns `{id, name, url}`. Buddy skips POST when the catalog cache already has this file’s sha256 and that id still GETs (lowest live id). |
| `GET` | `/api/v1/icon/download/{id}` | `IconDownload` fallback only | Image bytes when metadata `url` is missing. **500s for ICS-backed icons** — fetch `url` unauthenticated instead (no bearer; a token on the instance url can 401). |

`icon_download` / `buddy icon download` follows metadata `url` first (ICS or instance path, no Jamf token), then the Pro download path.

`icon_check` / `buddy icon check <file>` is **not** a Jamf list API: SHA-256 the local file, find the lowest catalog-cache id with that hash, and HEAD that row’s `url` (unauthenticated). Missing cache is an error — run `buddy icon list` / `icons()` first. ICS 403 AccessDenied is not present. `buddy icon check <file> <icon-id>` compares SHA-256 to that icon’s downloaded bytes (cache optional). Attach to a policy with Classic `PUT` `self_service_icon.id` (`PolicySetIcon` / `policy_set_icon`).

---

## Mac App Store apps (Classic API)

Base path: `/JSSResource/macapplications`

Jamf’s **Mac App Store Apps** object. There is no matching Pro JSON catalog for list/get/create. Prefer an instance-surface credential (Classic privileges). Policy apps use `buddy app` / `apps(...)` (discovered from policies), not this endpoint.

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET` | `/JSSResource/macapplications` | `MacAppSearch` | List all Mac App Store apps (id + name; XML) |
| `GET` | `/JSSResource/macapplications/id/{id}` | `MacAppFetch`, `MacAppSearch` (enrich), `MacAppDelete` | Fetch full detail (JSON) |
| `GET` | `/JSSResource/macapplications/name/{name}` | `MacAppFetchByName` | Fetch by exact name (JSON) |
| `POST` | `/JSSResource/macapplications/id/0` | `MacAppCreate` | Create (name, version, bundle_id, url required) |
| `DELETE` | `/JSSResource/macapplications/id/{id}` | `MacAppDelete` | Delete by id |

### Notes

- List summaries from Jamf are `{id, name}` only. `MacAppSearch` / `buddy macapp list` / `macapps()` follow up with per-row detail GETs so the table can show `version` and `bundle_id`. `--filter` / `filter=` is client-side (name, version, bundle_id); Classic has no RSQL on this collection.
- Detail shape includes `general` (id, name, version, bundle_id, url, is_free, category, site), `scope`, and `self_service` (incl. VPP license flags). Full replace (`set`) is not exposed in this slice.
- Optional create `--category` / `category=` resolves via exact category name (`CategoryFindIDByExactName`).
- Related but separate: Pro VPP locations (`/api/v1/volume-purchasing-locations`) and `jamf-package` (Protect/Connect only) — not wired as `macapp`.

---

## Configuration Profiles (Classic API)

Base path: `/JSSResource/osxconfigurationprofiles`

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET` | `/JSSResource/osxconfigurationprofiles` | `ProfileList`, `ProfileSearch`, `ProfileFindID` | List all configuration profiles (id + name) |
| `GET` | `/JSSResource/osxconfigurationprofiles/id/{id}` | `ProfileGetXML`, `ProfileGetPayloads`, `ProfileGetPayloadByDomain`, `ProfileListPayloads`, `ProfileSetPayload`, `ProfileRedeploy`, `ProfileClone` | Fetch full profile XML (`Accept: application/xml`; `jamfGET` would prefer JSON) |
| `POST` | `/JSSResource/osxconfigurationprofiles/id/0` | `ProfileClone`, `ProfileCreate` | Create a new profile (clone with empty scope, or greenfield create with payloads + scope) |
| `PUT` | `/JSSResource/osxconfigurationprofiles/id/{id}` | `ProfileSetXML`, `ProfileSetPayload`, `ProfileRedeploy` | Replace full profile XML (Classic-only when ACS is not linked; see ACS write path) |

### ACS API (Application & Custom Settings)

Jamf Application & Custom Settings (ACS) lives on undocumented `/api/config-profiles/macos/...` JSON endpoints (the path still contains `macos` as a platform segment). Buddy uses this surface whenever Classic cannot store the MCX preference strings the ACS UI shows.

- **Always instance.** ACS is the Jamf Pro UI API, not a Platform product API. Gateway POST is `404`; buddy always routes these calls to an instance auth. That is **not** `features.classic_instance_only`.
- **Do not invent Classic PUT from schema.** Live checks: `scripts/integration_test.buddy` (phase C) and `scripts/integration_test_acs_profile7.buddy`.
- Builtins and payload editing: [profile payloads](/docs/docs/profile-payloads).

#### Ampersand forms

Two different ampersands show up in MCX:

| Form | What it is | Classic | Buddy |
|------|------------|---------|--------|
| **Apply-form** | The preference *string* contains the five characters `&amp;` (Jamf ACS **Apply**) | Often **409** on PUT/POST | Write via ACS; keep Classic visible with the same Apply-form encoding |
| **Literal `&`** | Decoded setting is `Here &` or `…?q=foo&bar=` (XML still writes `&amp;` as an entity) | **409** | Write via ACS when a document is linked (or create/clone falls through after Classic reject/409). Classic-only PUT still rejects — no auto-convert |
| **Under-encoded outer `<payloads>`** | Nested plist `&` encoded as a single `&amp;` instead of `&amp;amp;` | **409** | Always rejected locally |

Buddy does **not** rewrite one form into the other on the ACS document. Classic visibility after an ACS write may seed Apply-form (`Here &amp;` in the Classic string) so the key is not dropped from Classic XML.

**Merging ACS into Classic XML (hydrate).** Classic `GET` often has empty or stale MCX while the ACS UI still has the real preference dict. On `profile_xml` / `get xml`, buddy fetches ACS `Forced` and copies those settings into the Classic `<payloads>` document it returns — so the caller sees what the UI shows, not the stub. Code and logs call this **hydrate**. The merge is read-only: buddy must not PUT that merged document back to Classic (that would overwrite Classic with a view that mixed two sources). `ProfileSetPayload` (`set payload`) skips the merge and uses the raw Classic GET.

#### Why Classic-only PUT wiped ACS keys

On several tenants `GET /api/config-profiles/macos/profile/{classicId}` returns **200** `{name, level}` only — no `payloadUUID`, no `Forced`. The ACS UI still holds settings. A later Classic PUT of a dict **without** `&` (or a stub that omitted those keys) returned **200** and replaced Classic payloads. The next process then had no ACS UUID, so the merge above did nothing, and `profile_xml` showed Classic only — preference keys that contain `&` were gone.

Fixes:

1. **Resolve an ACS handle even from a name/level shell** (`profile:{id}` sentinel), then **POST-attach** when `PUT /profile/{id}` is **405**. After a profile-scoped PUT **200**, re-read `Forced`; if it is still empty, POST-attach (clones can “succeed” with empty Forced).
2. **Persist real ACS document UUIDs** in `~/.config/buddy/acs-payload-uuids` (and honor `BUDDY_ACS_PAYLOAD_UUID_{id}` / `profile_acs_bind_uuid`) so the next process can fetch `Forced` and merge it into Classic XML.
3. **Classic payloads sync after ACS** starts from the server’s Classic XML (before any ACS merge) and never writes the merged view back. Refuse a sync that **omits** a current `&` key *or* **replaces** that value with a no-`&` string. If a bare-`&` Classic PUT **409**s, retry **Apply-form** encoding so the key stays in Classic.
4. **On read:** fill empty Classic MCX from ACS `Forced`; if ACS settings differ from Classic, ACS wins.

#### Write path (PUT)

`profile_put_xml` / `ProfileSetXML` (`profileClassicPut`):

1. If MCX is present and an ACS UUID or `profile:{id}` shell resolves — **or** the XML is Apply-form eligible — skip Classic-first and write ACS (`forced.plist` on `PUT /api/config-profiles/macos/{payloadUUID}`).
2. Else Classic PUT of encoding-validated XML. Bare `&` in decoded MCX **rejects locally** when ACS is not linked.
3. Classic **409** with MCX still falls back to ACS.
4. After ACS: Classic payloads sync (guards above), then a **metadata-only** Classic PUT for name / scope / `redeploy_on_update`.

#### Write path (POST / clone / create)

`profile_create` / `profile_clone` (`profileClassicPost`):

1. Try Classic `POST …/id/0` when outer encoding is valid (including Apply-form).
2. Bare `&` in decoded MCX, or Classic **409** with MCX → Classic shell + ACS attach (`POST /api/config-profiles/macos` with `classicProfileId`).
3. Same Classic payloads sync + Apply-form seed if Classic XML (after merging ACS `Forced`) is still missing the MCX domain.

Do not invent Classic profile PUT bodies from `jamf_schema`. Live checks: `scripts/integration_test.buddy` (phase C create/edit/clone/delete with Apply-form and literal `&`) and `scripts/integration_test_acs_profile7.buddy` (same-tenant ACS document).

ACS `/api/config-profiles/…` is the Jamf Pro **instance** UI API. It is not a Platform product API and is **not** rewritten to the gateway (a gateway POST returns `404 page not found`). Buddy always routes these calls to an instance auth. That is **not** `features.classic_instance_only` — that flag only gates Classic `/JSSResource` → `/api/proclassic/`.

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET` | `/api/config-profiles/macos/profile/{classicId}` | `resolveACSPayloadUUID`, `profileACSPut` | Resolve ACS payload UUID / Forced. Often `{name, level}` only — treat as shell, not “no ACS” |
| `PUT` | `/api/config-profiles/macos/profile/{classicId}` | `profileACSCreatePayload` | Attach ACS to Classic profile (some tenants return **405**; buddy falls back to POST) |
| `GET` | `/api/config-profiles/macos/{payloadUUID}` | read merge, `profileACSPut` | Fetch current ACS JSON document (`Forced` / `forced.plist`) |
| `PUT` | `/api/config-profiles/macos/{payloadUUID}` | `profileACSPut` | Save MCX payload content (`forced.plist`) |
| `POST` | `/api/config-profiles/macos` | create/attach fallback | Create or attach ACS (`classicProfileId` + `name` link to Classic profile) |
| `POST` | `/api/config-profiles/plist/can-convert-to-plist` | ACS write | Validate plist before save |
| `GET` | `/api/config-profiles/macos/generated-uuids?count=N` | (optional) | Pre-generate UUIDs (UI uses; POST accepts `payloadUUID: null`) |

Builtins and payload editing: [profile payloads](/docs/docs/profile-payloads).

### Notes

- List/get/clone still go through Classic XML. There is no public v1/v2 profile CRUD; ACS JSON is the undocumented UI API for Application & Custom Settings only.
- `--redeploy` sets `redeploy_on_update` to `All` in the XML before the **metadata** Classic PUT, causing Jamf to push the profile to all currently-assigned devices rather than only new assignments.
- **Payload handling**: The profile XML contains a `<general><payloads>` element whose content is entity-encoded plist XML. Buddy supports PayloadContent (`identifier=` matches PayloadIdentifier, unique PayloadType, or unique PayloadUUID) and MCX (`mcx_preference_settings` preference domains). **Get/list** (`get payload`, `list-payloads`, `payloads_get_payload`, `payloads_list_identifiers`) are PayloadContent only and list `{identifier, type, uuid}`; **set/patch** merge PayloadContent keys or replace MCX settings. See [profile payloads](/docs/docs/profile-payloads).
- **UUID preservation**: When updating a profile via `set xml` or `set payload`, the existing profile UUID is preserved (the fetched body is only modified in the payloads section or redeploy flag). Preserving the same UUID avoids duplicate or orphaned profiles on devices when updating from another instance.

---

## API Roles (Jamf Pro API v1)

Base path: `/api/v1/api-roles`

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET` | `/api/v1/api-roles` | `RoleList`, `RoleListValues` | List API roles (paginated; bare filter → `displayName=="*term*"`) |
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
| `GET` | `/api/v1/api-integrations` | `ClientList`, `ClientListValues` | List API integrations (paginated; bare filter → `displayName=="*term*"`) |
| `GET` | `/api/v1/api-integrations/{id}` | `ClientGet`, `clientGetRaw` | Fetch a single API integration |
| `POST` | `/api/v1/api-integrations` | `ClientCreate` | Create a new API integration |
| `PUT` | `/api/v1/api-integrations/{id}` | `ClientUpdate`, `ClientSetTokenLifetime` | Update an API integration (name, scopes, token lifetime) |
| `DELETE` | `/api/v1/api-integrations/{id}` | `ClientDelete` | Delete an API integration |
| `POST` | `/api/v1/api-integrations/{id}/client-credentials` | `ClientCredentials` | Generate a new OAuth client secret (invalidates the previous one) |

### Notes

- `ClientUpdate` and `ClientSetTokenLifetime` both use a fetch-then-PUT pattern (`clientGetRaw` first) to preserve fields not being modified.
- The generated client secret from `client-credentials` is shown only once and cannot be retrieved again.

---

## Computers (Jamf Pro API v4/v2/v1)

Base paths: `/api/v4/computers-inventory`, `/api/v2/mdm/commands`, `/api/v1/jamf-management-framework/redeploy`

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET` | `/api/v4/computers-inventory` | `ComputerSearch`, `ComputerSearchRaw`, `ComputerSearchPrint`, `computers` | List computers (paginated; all pages by default); optional RSQL `filter`, `sort`, repeated `section=` |
| `GET` | `/api/v4/computers-inventory/{id}` | `ComputerGet`, `ComputerGetValue`, `computer`, `computerGetManagementID`, `ComputerFetchExtensionAttributes` | Fetch computer inventory (default GENERAL; optional `?section=`; use `EXTENSION_ATTRIBUTES` for EA values) |
| `GET` | `/api/v4/computers-inventory-detail/{id}` | `ComputerDetail`, `ComputerDetailValue`, `computer_detail` | Fetch all inventory sections |
| `PATCH` | `/api/v4/computers-inventory-detail/{id}` | `ComputerSetExtensionAttribute`, `computer_set_extension_attribute` | Set TEXT/POPUP EA values (`extensionAttributes: [{definitionId, values}]`) |
| `POST` | `/api/v2/mdm/commands` | `ComputerReboot`, `ComputerShutdown`, `ComputerLock`, `ComputerLogOut`, `ComputerErase`, `ComputerDeviceInfo`, `ComputerDeviceLocation`, `ComputerSecurityInfo`, `ComputerInstalledApps`, `ComputerManagedApps`, `ComputerCertificates` | Send MDM command (requires managementId from inventory) |
| `POST` | `/api/v1/jamf-management-framework/redeploy/{id}` | `ComputerRedeploy` | Trigger Jamf Management Framework redeploy on a computer |

### Notes

- Inventory uses **v4** (`/api/v4/computers-inventory`). v3 is deprecated (Jamf Pro 11.30). RSQL/sort date fields are `general.lastContact` (renamed from v3 `lastContactTime`), `general.lastCheckIn` (new), and `general.reportDate` (unchanged).
- All MDM commands fetch each computer's `managementId` via `GET /api/v4/computers-inventory/{id}?section=GENERAL`, then POST to `/api/v2/mdm/commands` with `clientData: [{managementId, clientType: "COMPUTER"}]` and `commandData: {commandType: "..."}` plus optional fields.
- **Device control commands** (destructive; require confirmation unless `-y`): `RESTART_DEVICE`, `SHUT_DOWN_DEVICE`, `LOG_OUT_USER`. **DEVICE_LOCK** and **ERASE_DEVICE** require extra confirmation (LOCK: "Proceed?"; ERASE: type "erase" to confirm).
- **Information/query commands** (non-destructive; queued; results reported asynchronously by device): `DEVICE_INFORMATION`, `DEVICE_LOCATION`, `SECURITY_INFO`, `INSTALLED_APPLICATION_LIST`, `MANAGED_APPLICATION_LIST`, `CERTIFICATE_LIST`.
- `ComputerRedeploy` is a simple POST with no body; no v3 endpoint exists, so Buddy uses v1.
- RSQL filter examples: `general.name=="*Example*"`, `hardware.serialNumber=="C02EXAMPLE0001"`, `udid=="00000000-0000-4000-8000-000000000001"`, `general.lastContact=ge=2026-01-01T00:00:00Z`, `general.lastCheckIn=ge=2026-01-01T00:00:00Z`, `general.reportDate=ge=2026-01-01T00:00:00Z`. Sort examples: `general.lastContact:desc`, `general.lastCheckIn:desc`, `general.reportDate:desc` (omit = Jamf `general.name:asc`).
- List `section=` is the same enum as get (`GENERAL`, `HARDWARE`, `OPERATING_SYSTEM`, …). Omit = GENERAL only. `computers(raw=True)` / `buddy computer list --json` returns Jamf `results[]` unchanged so extra sections are usable without N+1 `computer()` calls.
- Date field names are **v4**: `general.lastContact` (last MDM/binary/DDM contact), `general.lastCheckIn`, and `general.reportDate` (last inventory). Do not use v3 `lastContactTime` or Platform `lastCheckInTime` / `lastInventoryUpdateTime` on this API.

---

## Computer history (Classic API)

Base path: `/JSSResource/computerhistory`

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET` | `/JSSResource/computerhistory/{key}/{value}` | `ComputerHistoryGet`, `computer_history` | Full Classic computer history (`key` = `id`, `name`, `serialnumber`, or `udid`) |
| `GET` | `/JSSResource/computerhistory/{key}/{value}/subset/{subset}` | `ComputerHistoryGet`, `ComputerPolicyLogs`, `computer_history`, `computer_policy_logs`, `policy_logs` | Subset (e.g. `PolicyLogs`; combine with `&`) |
| `GET` | `/JSSResource/computers/subset/basic` | `policy_logs` (`limit=` scan) | Classic ID+name list (faster than Pro v4 inventory for candidate sets) |

### Notes

- This is **policy/event execution history**, not MDM command history (`GET /api/v2/mdm/commands`) and not package/script audit `GET …/history`.
- **Not** undocumented Pro `GET /api/v1/policies` — that path is a paginated **list index** only (`id`, `name`, `enabled`, …) and has no logs.
- Lookup is per computer (id, name, serial, or udid). There is no published “all logs for policy N” collection. `policy_logs(id, computer_id)` still `GET`s that Mac’s full PolicyLogs subset and filters `policy_id` client-side. Bounded scans: `computer_ids=`, `limit=` (Classic `GET /JSSResource/computers/subset/basic`), or v4 `last_contact`/`filter` (`computers()` inventory — lastContact is not log date). Use `since=`/`until=` on `date_completed_epoch`. Unconstrained fleet scan is rejected. Does not expand policy scope.
- Log rows expose Classic `date_time*` and `date_completed*` (same instant; UI “Date Completed”).
- Accept is `application/json`. Buddy unwraps the `computer_history` envelope and flattens `{policy_log: {…}}` wrappers (and Classic XML-style `{size, policy_log}` bags). XML is a parse fallback only.
- Subset tokens: `General`, `PolicyLogs`, `Commands`, `Audits`, `ComputerUsageLogs`, `CasperRemoteLogs`, `ScreenSharingLogs`, `CasperImagingLogs`, `UserLocation`, `MacAppStoreApplications`.
- CLI `buddy computer history` defaults to `--subset=PolicyLogs` (full documents can be large). Privilege: **Read Computer History**. Classic routing (`OpClassic` / proclassic), not `OpProUI`.

---

## Advanced computer searches (Classic API)

Jamf’s **Reports** UI for Macs is Classic advanced computer searches. There is no Pro JSON CRUD for this object. Buddy’s `computer_search` / `buddy computer-search` treat **definition** and **results** as separate calls (`computers` on GET-by-id can be huge).

Base path: `/JSSResource/advancedcomputersearches`

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET` | `/JSSResource/advancedcomputersearches` | `ComputerSearchSearch`, `computer_searches` | List `{id, name}` (JSON Accept; XML fallback) |
| `GET` | `/JSSResource/advancedcomputersearches/id/{id}` | `ComputerSearchFetch`, `computer_search` | Definition JSON; buddy **strips `computers`** |
| `GET` | `/JSSResource/advancedcomputersearches/name/{name}` | `ComputerSearchFetchByName` | Exact Classic name |
| `GET` | `/JSSResource/computerreports/id/{id}` | `ComputerSearchComputers`, `computer_search_computers` | Results-only `Computer` rows (same saved-search IDs) |
| `POST` | `/JSSResource/advancedcomputersearches/id/0` | `ComputerSearchCreate`, `computer_search_create` | Create (JSON body; `computers` stripped) |
| `PUT` | `/JSSResource/advancedcomputersearches/id/{id}` | `ComputerSearchPut`, `computer_search_put` | Full replace (JSON; `computers` stripped) |
| `DELETE` | `/JSSResource/advancedcomputersearches/id/{id}` | `ComputerSearchDelete`, `computer_search_delete` | Delete by id |

### Notes

- `computerreports` is **not** a second resource noun — it is the results path for the same advanced-search IDs. If that GET fails, buddy falls back to ACS `computers`.
- Pass Classic JSON through (`and_or` / `search_type`). List `--filter` / `filter=` is client-side on `{id, name}`.
- Not DDM / benchmark / blueprint status reports, not per-Mac `computerhardwaresoftwarereports`, and not `buddy report`.

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

## Mobile Devices (Jamf Pro API v2)

Base path: `/api/v2/mobile-devices`

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET` | `/api/v2/mobile-devices` | `MobileDeviceSearch`, `mobile_devices` | List mobile devices (paginated; **no server-side filter** — buddy filters client-side) |
| `GET` | `/api/v2/mobile-devices/{id}` | `MobileDeviceGet`, `mobile_device`, `mobileDeviceGetManagementID` | Fetch one device (summary record incl. `managementId`) |
| `GET` | `/api/v2/mobile-devices/{id}/detail` | `MobileDeviceGet --detail`, `mobile_device(detail=True)` | Full inventory detail |
| `POST` | `/api/v2/mdm/commands` | `MobileDeviceUpdateInventory`, `MobileDeviceRestart`, `MobileDeviceLock`, `MobileDeviceErase`, `mobile_device_send_mdm_command` | Send MDM command with `clientType: "MOBILE_DEVICE"` |

### Notes

- The list endpoint has no RSQL `filter`; buddy's `--filter` / `filter=` is a client-side substring over id/name/serial/udid/username/model/type.
- MDM actions resolve each device's `managementId` from `GET /api/v2/mobile-devices/{id}`, then POST `/api/v2/mdm/commands` — same send path as computers, different `clientType`.
- Command types used: `DEVICE_INFORMATION` (update-inventory), `RESTART_DEVICE`, `DEVICE_LOCK` (optional `message`/`phoneNumber`/`pin`), `ERASE_DEVICE` (optional `pin`, `obliterationBehavior`, `preserveDataPlan`, `disallowProximitySetup`).

---

## Advanced mobile-device searches (Pro + Classic)

Definitions: Pro `/api/v1/advanced-mobile-device-searches` (paginated JSON CRUD). **No membership in the Pro schema.** Results: Classic `GET /JSSResource/advancedmobiledevicesearches/id/{id}` → `mobile_devices`. Buddy’s `mobile_search` / `buddy mobile-search` split definition vs results like computer searches.

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET` | `/api/v1/advanced-mobile-device-searches` | `MobileSearchSearch`, `mobile_searches` | Paginated `{id, name}` (omit page = all) |
| `GET` | `/api/v1/advanced-mobile-device-searches/{id}` | `MobileSearchFetch`, `mobile_search` | Pro definition JSON (no devices) |
| `POST` | `/api/v1/advanced-mobile-device-searches` | `MobileSearchCreate`, `mobile_search_create` | Create (Pro camelCase `andOr` / `searchType`) |
| `PUT` | `/api/v1/advanced-mobile-device-searches/{id}` | `MobileSearchPut`, `mobile_search_put` | Full replace |
| `DELETE` | `/api/v1/advanced-mobile-device-searches/{id}` | `MobileSearchDelete`, `mobile_search_delete` | Delete by id |
| `GET` | `/JSSResource/advancedmobiledevicesearches/id/{id}` | `MobileSearchDevices`, `mobile_search_devices` | Classic result rows (`mobile_devices`) |

### Notes

- Result rows are Classic `mobile_devices` only; Pro GET has no membership. List `filter=` is client-side on id/name (Pro list has no RSQL in this slice).
- Classic swagger’s “Pro equivalent” note for **user** searches points at `advanced-user-content-searches` (VPP/content). That is **not** Classic `advancedusersearches` (user inventory) and is not wired here.
- Not DDM / benchmark / patch reports.

---

## Mobile-Device Groups (Jamf Pro API v2)

Base path: `/api/v2/mobile-device-groups`

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET` | `/api/v2/mobile-device-groups/smart-groups` | `MobileGroupSearchSmart`, `mobile_smart_groups` | List smart groups (paginated, RSQL filter; bare → `groupName=="*term*"`) |
| `GET` | `/api/v2/mobile-device-groups/static-groups` | `MobileGroupSearchStatic`, `mobile_static_groups` | List static groups |
| `GET` | `/api/v2/mobile-device-groups/smart-groups/{id}` | `mobile_smart_group`, criteria helpers | Fetch smart group (name, criteria) |
| `GET` | `/api/v2/mobile-device-groups/static-groups/{id}` | `mobile_static_group` | Fetch static group |
| `GET` | `/api/v2/mobile-device-groups/smart-group-membership/{id}` | `MobileSmartGroupFetchMembers`, `mobile_smart_group_devices` | Effective smart-group membership |
| `GET` | `/api/v2/mobile-device-groups/static-group-membership/{id}` | `MobileStaticGroupFetchMembers`, `mobile_static_group_devices` | Static-group membership |
| `POST` | `/api/v2/mobile-device-groups/smart-groups` | `MobileGroupCreateSmart`, `mobile_smart_group_create` | Create smart group |
| `POST` | `/api/v2/mobile-device-groups/static-groups` | `MobileGroupCreateStatic`, `mobile_static_group_create` | Create static group |
| `PUT` | `/api/v2/mobile-device-groups/smart-groups/{id}` | `MobileGroupPutSmartRaw`, `MobileSmartGroupSetCriteria` | Update smart group (criteria) |
| `PATCH` | `/api/v2/mobile-device-groups/static-groups/{id}` | `MobileStaticGroupAssign`, `MobileStaticGroupSetDevices` | Mutate static membership (`assignments: [{mobileDeviceId, selected}]`) |
| `DELETE` | `/api/v2/mobile-device-groups/{smart,static}-groups/{id}` | `MobileGroupDeleteSmart` / `MobileGroupDeleteStatic` | Delete a group |

### Notes

- Summary rows use `groupId` / `groupName` / `groupDescription` (unlike computer groups' `id` / `name`).
- Static membership mutates are read-modify-write: buddy reads current membership, then PATCHes an `assignments` array of `{mobileDeviceId, selected}` rows.
- Criteria share the computer smart-group shape: `{name, priority, andOr, searchType, value, openingParen?, closingParen?}`.

---

## Mobile Profiles (Classic API)

Base path: `/JSSResource/mobiledeviceconfigurationprofiles`

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET` | `/JSSResource/mobiledeviceconfigurationprofiles` | `MobileProfileSearch`, `mobile_profiles` | List profiles (XML; buddy filters by name client-side) |
| `GET` | `/JSSResource/mobiledeviceconfigurationprofiles/id/{id}` | `MobileProfileFetchXML`, `mobile_profile_xml` | Fetch profile XML envelope |
| `PUT` | `/JSSResource/mobiledeviceconfigurationprofiles/id/{id}` | `MobileProfilePutXML`, `mobile_profile_put_xml` | Update profile XML (optional redeploy_on_update=All) |
| `POST` | `/JSSResource/mobiledeviceconfigurationprofiles/id/0` | `MobileProfileCreateXML`, `mobile_profile_create_xml` | Create profile from XML |
| `DELETE` | `/JSSResource/mobiledeviceconfigurationprofiles/id/{id}` | `MobileProfileDelete`, `mobile_profile_delete` | Delete profile |

### Notes

- The XML envelope mirrors macOS computer profiles (root `configuration_profile`, entity-encoded `<payloads>` plist), so the offline `profile_xml_*` / `payloads_*` helpers work on mobile profile XML unchanged.
- Classic routing follows the same instance/proclassic rules as other `/JSSResource` calls (see Notes at the end of this document).

---

## Computer Groups (Jamf Pro API v3)

Base path: `/api/v3/computer-groups`

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET` | `/api/v3/computer-groups/smart-groups` | `GroupSearchSmart`, `GroupListSmart`, `GroupFindID` | List smart groups (paginated, RSQL filter on `id`, `name`, `siteId`) |
| `GET` | `/api/v3/computer-groups/static-groups` | `GroupSearchStatic`, `GroupListStatic` | List static groups (paginated, RSQL filter on `id`, `name`, `siteId`) |
| `GET` | `/api/v3/computer-groups/smart-groups/{id}` | `GroupGetVersion`, `GroupVersion`, `GroupDelete`, criteria helpers | Fetch smart group detail (name, criteria) |
| `GET` | `/api/v3/computer-groups/smart-group-membership/{id}` | `SmartGroupFetchMembers`, `GroupSmartListMembers` | Effective smart group membership (computer IDs) |
| `POST` | `/api/v3/computer-groups/smart-groups` | `GroupCreateSmart`, `GroupCreateSmartNamed` | Create a smart computer group |
| `PUT` | `/api/v3/computer-groups/smart-groups/{id}` | `GroupVersion`, `SmartGroupSetCriteria` | Update smart group (criteria / version criterion) |
| `DELETE` | `/api/v3/computer-groups/smart-groups/{id}` | `GroupDelete` | Delete a smart computer group |
| `POST` | `/api/v3/computer-groups/static-groups` | `GroupCreateStatic`, `GroupCreateStaticNamed` | Create a static computer group |
| `GET` | `/api/v3/computer-groups/static-groups/{id}` | `StaticGroupFetchMembers`, `StaticGroupPutAssignments` | Fetch static group (including assignments when present) |
| `PUT` | `/api/v3/computer-groups/static-groups/{id}` | `StaticGroupPutAssignments` | Update static group membership |

### Notes

- Computer groups use **v3** (`/api/v3/computer-groups/...`). Cloud schema deprecated v2 around 2026-05-28.
- Both list endpoints are paginated (`page`, `page-size=200`) and support RSQL `filter` server-side. Bare `--filter` values become `name=="*term*"`.
- Group `id` values are JSON strings (e.g. `"920"`), consistent with scripts and packages.
- `GroupFindID` searches smart groups only (used for version/scope operations on smart groups).

---

## Categories (Jamf Pro API v1)

Base path: `/api/v1/categories`

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET` | `/api/v1/categories` (paginated) | `CategorySearch`, `CategoryFindIDByExactName`, policy create helpers | List categories |
| `GET` | `/api/v1/categories/{id}` | `CategoryFetch` | Get one category |
| `POST` | `/api/v1/categories` | `CategoryPost`, clone/copy/migrate | Create (`name` + `priority` required; id ignored) |
| `PUT` | `/api/v1/categories/{id}` | `CategoryPut`, migrate upsert | Update category |

`ResolveCategoryOnDest` looks up a source category **name** on the destination (soft: missing → uncategorized). Used by script/policy/profile copy.

---

## Request Flow per Operation

```
buddy script list [--filter=<term|rsql>]
  └─ GET /api/v1/scripts                     (page-size=200; bare → name=="*term*")

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

buddy script upload <id> <file>
  └─ GET /api/v1/scripts/{id}                (fetch current)
  └─ PUT /api/v1/scripts/{id}                (replace scriptContents)

buddy script delete <id> [id...]
  └─ GET /api/v1/scripts/{id}                (fetch name for confirmation, per ID)
  └─ DELETE /api/v1/scripts/{id}             (delete, per ID)

buddy script set <property> <id> <value>
  └─ GET /api/v1/scripts/{id}                (fetch current)
  └─ PUT /api/v1/scripts/{id}                (update with modified field)

buddy policy get <policy-id>
  └─ GET /JSSResource/policies/id/{id}      (detail → JSON summary)

buddy policy get id <trigger>
  └─ GET /JSSResource/policies              (list, process-cached)
  └─ GET /JSSResource/policies/id/{id}      (detail, per candidate, for exact trigger match)

buddy policy set ss <id> <version>
  └─ GET /JSSResource/policies/id/{id}      (fetch current)
  └─ PUT /JSSResource/policies/id/{id}      (update self-service description + optional scope)

buddy policy set install <id> <version>
  └─ GET /JSSResource/policies/id/{id}      (fetch current)
  └─ PUT /JSSResource/policies/id/{id}      (update script parameters)

buddy policy set package <policy-id> <version> [--name=<name>] [--package=<pattern>]
buddy policy set package <version> --trigger=<trigger> [--name=<name>] [--package=<pattern>]
  └─ GET /JSSResource/policies              (list, cached; only when --trigger set, for PolicyFind)
  └─ GET /JSSResource/policies/id/{id}      (fetch source policy)
  └─ GET /api/v1/packages                   (find matching package)
  └─ PUT /JSSResource/policies/id/{id}      (update package on source policy)
  └─ POST /JSSResource/policies/id/0        (create new versioned install policy, if needed)
  └─ PUT /JSSResource/policies/id/{id}      (disable old install policy)

  Use --trigger to resolve the source policy by custom trigger instead of an ID.
  --name sets the package search keyword (default: derived from the source trigger).
  Use --package to override the default name+version search pattern. Supports:
    - Exact: "FalconSensorMacOS-7.32.20408.pkg"
    - Template: "FalconSensorMacOS-{version}.pkg" (substitutes version)
    - Regex: "re:FalconSensor.*\.pkg" (client-side regex filter)

buddy policy create package <name> <version> [--action=install|cache] [--scope=all|1,2,3] [--category=name] [--package=<pattern>]
  └─ GET /api/v1/packages                   (PackageFindID; --package overrides search pattern)
  └─ GET /JSSResource/policies              (uniquePolicyName)
  └─ GET /api/v1/categories                  (if --category set)
  └─ POST /JSSResource/policies/id/0

buddy policy create ss <trigger> <display-name> [--category=name]
  └─ GET /api/v1/categories                  (if --category set)
  └─ POST /JSSResource/policies/id/0

buddy policy delete <id> [id ...]
  └─ GET /JSSResource/policies/id/{id}      (fetch name for confirmation, per ID)
  └─ DELETE /JSSResource/policies/id/{id}   (delete, per ID)

buddy policy list [--filter=<keyword>] [--ids] [--exclude-ids=1,2,3]
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

buddy group list smart [--filter=<term|rsql>]
  └─ GET /api/v3/computer-groups/smart-groups       (paginated; bare → name=="*term*")

buddy group list static [--filter=<term|rsql>]
  └─ GET /api/v3/computer-groups/static-groups      (paginated; bare → name=="*term*")

buddy group smart members <group-id>
  └─ GET /api/v3/computer-groups/smart-group-membership/{id}

buddy group smart criteria <group-id>
  └─ GET /api/v3/computer-groups/smart-groups/{id}

buddy group smart set-criteria <group-id> [--file=|-]
  └─ GET /api/v3/computer-groups/smart-groups/{id}
  └─ PUT /api/v3/computer-groups/smart-groups/{id}

buddy group smart version <group-id>
  └─ GET /api/v3/computer-groups/smart-groups/{id}

buddy group smart set-version <group-id> <version>
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
  └─ GET /JSSResource/osxconfigurationprofiles/id/{id}   (Accept: application/xml)
  └─ GET /api/config-profiles/macos/profile/{id}         (optional: resolve ACS UUID)
  └─ GET /api/config-profiles/macos/{payloadUUID}        (merge ACS Forced into the XML returned to the caller)

buddy profile get payloads <id>
  └─ GET /JSSResource/osxconfigurationprofiles/id/{id}   (extract payloads plist; same ACS merge as get xml)

buddy profile get payload <id> <identifier>
  └─ GET /JSSResource/osxconfigurationprofiles/id/{id}   (extract one PayloadContent payload by identifier=)

buddy profile get list-payloads <id>
  └─ GET /JSSResource/osxconfigurationprofiles/id/{id}   (list {identifier, type, uuid})

buddy profile clone <profile-id> <name>
  └─ GET /JSSResource/osxconfigurationprofiles/id/{id}   (same ACS merge as get xml)
  └─ POST /JSSResource/osxconfigurationprofiles/id/0     (or Classic shell + ACS attach on 409 / bare '&')

buddy profile redeploy <id>
  └─ GET /JSSResource/osxconfigurationprofiles/id/{id}   (fetch current XML)
  └─ PUT /JSSResource/osxconfigurationprofiles/id/{id}   (set redeploy_on_update=All; ACS path if MCX linked)

buddy profile set xml <id> [--redeploy]
  └─ PUT /api/config-profiles/macos/{payloadUUID}        (ACS-linked or Apply-form / Classic 409)
  └─ POST /api/config-profiles/macos                     (attach when PUT /profile/{id} is 405)
  └─ PUT /JSSResource/osxconfigurationprofiles/id/{id}   (Classic payloads sync; Apply-form fallback)
  └─ PUT /JSSResource/osxconfigurationprofiles/id/{id}   (metadata: name / scope / redeploy)

buddy profile set payload <id> <identifier> <file> [--redeploy]
  └─ GET /JSSResource/osxconfigurationprofiles/id/{id}   (raw Classic XML; no ACS merge)
  └─ PUT …                                              (same write path as set xml)

buddy package list [--filter=<term|rsql>]
  └─ GET /api/v1/packages                         (bare → packageName=="*term*")

buddy package upload <file> [--name=] [--notes=] [--info=] [--category=] [--priority=] [--reboot=]
  └─ GET  /api/v1/packages?filter=fileName=="…"  (reuse existing record if present)
  ├─ if existing:
  │    GET  /api/v1/packages/{id}               (summary + checksum compare)
  │    PUT  /api/v1/packages/{id}               (when attr flags differ; may be metadata-only)
  │    POST /api/v1/packages/{id}/upload        (streamed multipart, only when file differs)
  └─ if new:
       POST /api/v1/packages                    (create metadata with attr flags)
       GET  /api/v1/packages/{id}               (poll until readable — Jamf Cloud create is async)
       POST /api/v1/packages/{id}/upload        (streamed multipart file upload)

buddy package get <id>
  └─ GET /api/v1/packages/{id}               (full record, including checksums/size)
  └─ GET /api/v1/packages/{id}/history       (sort=date:desc&page-size=1, for lastChanged/lastChangedBy)

buddy package history <id>
  └─ GET /api/v1/packages/{id}/history       (sort=date:desc&page-size=20)

buddy package delete <id> [id2 ...]
  └─ DELETE /api/v1/packages/{id}            (single ID)
  └─ POST /api/v1/packages/delete-multiple   (multiple IDs)

buddy package set <package-id> [--name=] [--notes=] [--info=] [--category=] [--priority=] [--reboot=]
  └─ GET /api/v1/packages/{id}               (fetch full record)
  └─ PUT /api/v1/packages/{id}               (update set fields)

buddy computer list [--filter=<term|rsql>] [--sort=<field:dir>] [--section=HARDWARE] [--json]
  └─ GET /api/v4/computers-inventory          (bare → general.name=="*term*"; optional sort/section; page-size=200)

buddy computer history <id> [--subset=PolicyLogs]
buddy computer logs <id> [--policy=] [--status=] [--since=] [--until=]
  └─ GET /JSSResource/computerhistory/id/{id}[/subset/PolicyLogs]  (per computer; logs flatten PolicyLogs)

buddy policy logs <policy-id> --computer=<id>
buddy policy logs <policy-id> --computer-ids=<id,id,...>
buddy policy logs <policy-id> --limit=<n> [--skip=] [--since=]
  ├─ GET /JSSResource/computers/subset/basic     (when --limit= with no other computer source)
  ├─ GET /api/v4/computers-inventory             (only when --last-contact= / --filter=)
  └─ GET /JSSResource/computerhistory/id/{id}/subset/PolicyLogs  (per candidate; client-side policy_id)

buddy computer get <id> [--section=GENERAL,HARDWARE]
  └─ GET /api/v4/computers-inventory/{id}[?section=…]

buddy computer detail <id>
  └─ GET /api/v4/computers-inventory-detail/{id}

buddy computer reboot <id> [id...]
  └─ GET /api/v4/computers-inventory/{id}?section=GENERAL  (per ID, for managementId)
  └─ POST /api/v2/mdm/commands                (RESTART_DEVICE payload, per ID)

buddy computer shutdown <id> [id...]
  └─ GET /api/v4/computers-inventory/{id}?section=GENERAL
  └─ POST /api/v2/mdm/commands                (SHUT_DOWN_DEVICE)

buddy computer lock <id> [id...] [--message=...] [--phone=...] [--pin=...]
  └─ GET /api/v4/computers-inventory/{id}?section=GENERAL
  └─ POST /api/v2/mdm/commands                (DEVICE_LOCK; extra confirm)

buddy computer logout <id> [id...]
  └─ GET /api/v4/computers-inventory/{id}?section=GENERAL
  └─ POST /api/v2/mdm/commands                (LOG_OUT_USER)

buddy computer erase <id> [id...] [--pin=...] [--obliteration=...]
  └─ GET /api/v4/computers-inventory/{id}?section=GENERAL
  └─ POST /api/v2/mdm/commands                (ERASE_DEVICE; type "erase" to confirm)

buddy computer device-info <id> [id...] [--queries=...]
buddy computer device-location <id> [id...]
buddy computer security-info <id> [id...]
buddy computer installed-apps <id> [id...] [--identifiers=...]
buddy computer managed-apps <id> [id...] [--identifiers=...]
buddy computer certificates <id> [id...]
  └─ GET /api/v4/computers-inventory/{id}?section=GENERAL
  └─ POST /api/v2/mdm/commands                (respective commandType; queued, results async)

buddy computer redeploy <id> [id...]
  └─ POST /api/v1/jamf-management-framework/redeploy/{id}  (per ID)

buddy mdm list --filter=<term|rsql> [--sort=<expr>]
  └─ GET /api/v2/mdm/commands          (paginated; bare → status=="term"; filter required)

buddy role list [--filter=<term|rsql>]
  └─ GET /api/v1/api-roles                    (bare → displayName=="*term*")

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

buddy client list [--filter=<term|rsql>]
  └─ GET /api/v1/api-integrations             (bare → displayName=="*term*")

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

Several Jamf Pro / Platform list endpoints accept a `filter` query parameter using RSQL (RESTful Service Query Language).

### One-rule `--filter` / `filter=` UX

Buddy treats list filters the same way across RSQL-backed resources:

| Input | Behavior |
|---|---|
| empty | No `filter=` param (MDM still requires a non-empty filter) |
| contains an RSQL operator (`==`, `!=`, `=lt=`, `=gt=`, `=le=`, `=ge=`, `=in=`, `=out=`) | Pass through to the API unchanged |
| otherwise (simple term) | Wrap with the resource default field (see table below) |

Examples of simple terms:

```bash
buddy package list --filter=Chrome          # → packageName=="*Chrome*"
buddy computer list --filter=Example        # → general.name=="*Example*"
buddy script list --filter=Chrome           # → name=="*Chrome*"
buddy mdm list --filter=Pending             # → status=="Pending" (exact)
buddy device list --filter=Mac              # → name=="*Mac*"
```

Classic lists (policies, profiles, macapps, apps) and blueprint `--search` are unchanged (client-side / API-native search — not RSQL wrap).

### Simple-term defaults

| Resource | Simple term becomes |
|---|---|
| packages | `packageName=="*term*"` |
| computers | `general.name=="*term*"` |
| scripts / categories / smart+static groups | `name=="*term*"` |
| roles / clients (API integrations) | `displayName=="*term*"` |
| Platform devices / device-groups / device apps / DDM reports | `name=="*term*"` |
| mdm | `status=="term"` (exact enum match, not contains) |

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
# Simple term (auto-wrapped)
buddy package list --filter=ExampleApp

# Explicit RSQL
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

# Computers by name, serial, UDID, last contact, last check-in, or last inventory (v4 field names)
buddy computer list --filter=Example
buddy computer list --filter='general.name=="*Example*"'
buddy computer list --filter='hardware.serialNumber=="C02EXAMPLE0001"'
buddy computer list --filter='udid=="00000000-0000-4000-8000-000000000001"'
buddy computer list --filter='general.lastContact=ge=2026-01-01T00:00:00Z' --sort=general.lastContact:desc
buddy computer list --filter='general.lastCheckIn=ge=2026-01-01T00:00:00Z' --sort=general.lastCheckIn:desc
buddy computer list --filter='general.reportDate=ge=2026-01-01T00:00:00Z' --sort=general.reportDate:desc
buddy computer list --json

# MDM commands by status (bare or RSQL)
buddy mdm list --filter=Pending
buddy mdm list --filter='status==Pending'
buddy mdm list --filter='clientManagementId==00000000-0000-4000-8000-000000000002;status==Pending'

# MDM commands completed after a date (date fields require range operators)
buddy mdm list --filter='dateCompleted=ge=2024-01-01T00:00:00Z;status==Acknowledged'
```

In buddyscript scripts, prefer the `rsql_*` builders (see `buddy help builtins`) so dates and quoting stay correct:

```python
since = rsql_days_ago(7)
filt = rsql_and([
    rsql_ge("dateCompleted", since),
    rsql_eq("status", "Acknowledged"),
])
mdm_commands(filter=filt)

# last inventory in the last 7 days, newest first (v3)
computers(
    filter=rsql_ge("general.reportDate", rsql_days_ago(7)),
    sort="general.reportDate:desc",
)
# last MDM/binary/DDM contact
computers(
    filter=rsql_ge("general.lastContact", rsql_days_ago(1)),
    sort="general.lastContact:desc",
)
```

### Filterable Fields by Resource

| Resource | Filterable fields |
|---|---|
| `/v1/packages` | `id`, `fileName`, `packageName`, `categoryId`, `info`, `notes`, `manifestFileName`, `cloudTransferStatus` |
| `/v4/computers-inventory` | `id`, `udid`, `general.name`, `general.lastContact`, `general.lastCheckIn`, `general.reportDate`, `general.lastEnrolledDate`, `general.lastCloudBackupDate`, `general.mdmCertificateExpiration`, `general.managementId`, `operatingSystem.version`, `hardware.model`, `hardware.serialNumber`, … (see OpenAPI). Sort: `general.lastContact`, `general.lastCheckIn`, `general.reportDate`, `general.name`, `udid`, `id`, … |
| `/v2/mdm/commands` | `uuid`, `clientManagementId`, `command`, `status`, `clientType`, `dateSent`, `validAfter`, `dateCompleted`, `profileId`, `profileIdentifier`, `active` |
| `/v1/scripts`, `/v1/categories`, computer-groups | `id`, `name`, … (see OpenAPI) |
| `/v1/api-integrations` | `id`, `displayName` |
| `/v1/api-roles` | `id`, `displayName` |

### Notes

- Wildcard `*` is only meaningful inside string equality (`==` / `!=`).
- String values are case-insensitive in Jamf's RSQL implementation.
- Computer inventory date filters (`general.lastContact`, `general.lastCheckIn`, `general.reportDate`, …) use **unquoted** ISO-8601 UTC (`2026-01-01T00:00:00Z`). `rsql_ge` + `rsql_days_ago` already produce that.
- Quote RSQL values in the shell to prevent glob expansion: `--filter='packageName=="*Chrome*"'`. Bare simple terms like `--filter=Chrome` need no quotes.
- The RSQL grammar is specified at [github.com/jirutka/rsql-parser](https://github.com/jirutka/rsql-parser).

---

## Base URLs

Instance URLs live on **contexts** in `~/.config/buddy/config.yaml` (`contexts.<name>.jamfpro.url`). Platform gateway URLs are derived from context `region` (or an explicit gateway `url`). There is no hardcoded host. See `buddy cred help` and [Auth, secrets, and tokens](auth.md).
