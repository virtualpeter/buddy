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

Authentication is config-driven per tenant + auth (command, basic, or OAuth). See **[Auth, secrets, and tokens](auth.md)**, `buddy auth help`, and `buddy cred help`. For **basic** (username/password), buddy obtains a Bearer token via `POST /api/v1/auth/token` with Basic auth, then uses that token for all API requests. Platform credentials use **OAuth only** (`POST {gateway}/auth/token`, fallback `/oauth2/token`); there is no keep-alive.

### Custom calls from buddyscript

For endpoints without a typed builtin, scripts can call:

- `jamf_get` / `jamf_post` / `jamf_put` / `jamf_patch` / `jamf_delete`
- or `jamf_request(method, path, …)`

Paths are host-relative (`/api/v1/…` or `/JSSResource/…`). Media-type defaults follow the path (JSON for `/api/…`, XML for `/JSSResource/…` and gateway `/api/proclassic/…`). See `buddy help builtins` (section **JAMF API**).

### API schema documents

Each Jamf **instance** host publishes its live Pro/Classic specs (use `jamf_schema` from buddyscript). The **Platform API Gateway** does **not** serve a single live schema, so buddy ships a snapshot of the per-service Platform specs embedded in the binary:

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
- buddyscript list builtins expose `page?` / `page_size?`; **omit `page`** to fetch every page and return the combined list.

Applies to scripts, packages, computers inventory, smart/static group lists, API roles, API integrations, and blueprints. MDM command list uses the same `page` / `page_size` options but ends on a short page (that API’s `totalCount` is not a reliable global total).

---

## Blueprints (Platform API Gateway)

Base path (logical): `/api/blueprints/v1/blueprints`  
Rewritten: `/api/blueprints/v1/tenant/{tenantId}/blueprints`

Requires a credential with `tenantId` and `region` (or gateway `url`, e.g. `https://us.apigw.jamf.com`). CLI: `buddy blueprint …`. buddyscript: `blueprints` / `blueprint` / `blueprint_*`.

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

Read-only. CLI: `buddy ddm …`. buddyscript: `ddm_*`.

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET` | `…/declarations/{id}/devices` | `DDMDeclarationDevices` | Devices for declaration |
| `GET` | `…/devices/{id}/declarations` | `DDMDeviceDeclarations` | Declarations for device |
| `GET` | `…/devices/{id}/channels` | `DDMDeviceChannels` | Channels for device |

## Compliance benchmarks (`/api/compliance-benchmarks`, beta)

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
- Jamf’s list OpenAPI model may include `scriptContents` on each row; buddy’s `ScriptSearch` / buddyscript `scripts()` strip bodies and return `{id,name,categoryId,categoryName}` only. Use `GET /api/v1/scripts/{id}` (`script(id=…)`, `script_get_contents`) for the script body.

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

- **Hybrid list (undocumented Pro):** `getPolicyList` prefers `GET /api/v1/policies` (paginated `results`/`totalCount`; summary: `id`, `name`, `enabled`, `categoryId`, `executionFrequency`, `triggerCheckin`, `triggerManual`, `updateInventory`). On failure or empty, falls back to Classic. Not in published OpenAPI; not used for writes or detail. RSQL filter fields are those keys only — **no** Self Service boolean.
- `GET /JSSResource/policies` returns a summary list `{id, name}` only; detail fields (trigger, scripts, scope) require a subsequent `GET /JSSResource/policies/id/{id}`.
- Buddy caches the summary list in process memory per resolved tenant+auth session (~60s TTL) so multi-step scripts and change batches don't re-fetch it for every trigger lookup. Policy creates/deletes in the same process invalidate the entry; there is no on-disk cache.
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

- `PackageGet` prints the full v1 package object (including `md5` / `sha256` / `sha3512` / `hashType` / `hashValue` / `size` / `cloudTransferStatus` when present), then adds `lastChanged` / `lastChangedBy` from `history?sort=date:desc&page-size=1`.
- `cloudTransferStatus` is server-managed. Jamf’s UI “Availability Pending” clears when the value is `"READY"` (case-sensitive). Anything else (including empty) means not ready for policy use / JCDS download. A successful `GET /api/v1/packages/{id}` alone does **not** mean the package is usable. While transfer is pending, `sha256` is often blank while `hashValue` is already set (typically `hashType` SHA3_512) — that is **not** a signal to re-upload. A record with **no `hashValue`** (and no comparable md5/sha256) usually means the multipart upload failed after the metadata create — re-upload is appropriate (`--force` / `force=True` overrides skip/not-ready guards).
- `PackageReady` / buddyscript `package_ready` return true only when the package exists, `cloudTransferStatus == "READY"`, and `sha256` is non-empty (404 → false). `PackageWaitReady` / `package_wait_ready` poll for both with backoff (default 10 minutes) and best-effort POST `jcds/refresh-inventory?file-name=…` between polls. Use after upload / before attaching the package to a policy.
- `PackageDownload` / `package_download` / `buddy package download`: wait until READY **and** non-empty `sha256` → GET JCDS pre-signed URI → stream to a temp file beside the destination while hashing SHA-256 → rename only on match; delete temp on failure; retry once on URI HTTP 403. **JCDS only** (Jamf Cloud); not for on-prem non-JCDS distribution points. Verify is always on (no skip flag). Do not invent this loop from OpenAPI alone — call the builtin.
- `PackageUpload` looks up `fileName` first. If a record exists, it GETs the full object, compares local MD5/SHA-256 to server checksums, and **skips** the file transfer when they match; if comparable checksums are empty, status is not READY, and `hashValue` is set, `--wait-ready` / `wait_ready=True` **skips the file transfer and polls until READY** (re-run after an upload that omitted the flag); without wait-ready that case **errors as not ready** instead of treating blank hashes as “needs upload”; if there is **no `hashValue`**, it treats the record as a failed upload and streams a replacement; `--force` / `force=True` re-uploads regardless. Metadata flags (`--name`, `--notes`, `--info`, `--category`, `--priority`, `--reboot`) apply on create and on update when they differ (including metadata-only updates when the file is unchanged). If no record exists, it POSTs metadata (no confirm prompt), polls GET until the ID is readable (Jamf Cloud create is async), **waits 5s** so the upload endpoint can catch up, then streams the multipart upload. After upload, scripts should use `package_wait_ready` before attaching the package to a policy.
- `PackageScan` / `packages_scan` / `buddy package scan` pages the packages list and classifies incomplete records from list fields: `failed-upload` (no `hashValue` and no md5/sha256), `pending` (`hashValue` set, not READY), `unusable-ready` (READY but empty `sha256`). Optional `--class=` / `class=` narrows; default is all three.
- `packagePut` / `PackageSetAttrs` (used by `buddy package set <id> --…`) omit server-managed `size` / `cloudTransferStatus` and client-only history fields from the PUT body so a metadata update does not blank Jamf’s size.
- `PackageFindID` is shared between the CLI (`policy set package`) and buddyscript (`package(name=..., version=...)` builtin). List/search walks all pages unless a single `page` is requested from buddyscript.
- The `delete-multiple` body is `{"ids": ["1","2",...]}`.

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
- **Payload handling**: The profile XML contains a `<general><payloads>` element whose content is entity-encoded plist XML. Buddy supports PayloadContent (`identifier=` matches PayloadIdentifier, unique PayloadType, or unique PayloadUUID) and MCX (`mcx_preference_settings` preference domains). **Get/list** (`get payload`, `list-payloads`, `profile_get_payload`, `payloads_list_identifiers`) are PayloadContent only and list `{identifier, type, uuid}`; **set/patch** merge PayloadContent keys or replace MCX settings. See [profile payloads](/docs/docs/profile-payloads).
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

## Computers (Jamf Pro API v3/v2/v1)

Base paths: `/api/v3/computers-inventory`, `/api/v2/mdm/commands`, `/api/v1/jamf-management-framework/redeploy`

| Method | Endpoint | Used by | Purpose |
|--------|----------|---------|---------|
| `GET` | `/api/v3/computers-inventory` | `ComputerSearch`, `ComputerSearchRaw`, `ComputerSearchPrint`, `computers` | List computers (paginated; all pages by default); optional RSQL `filter`, `sort`, repeated `section=` |
| `GET` | `/api/v3/computers-inventory/{id}` | `ComputerGet`, `ComputerGetValue`, `computer`, `computerGetManagementID` | Fetch computer inventory (default GENERAL; optional `?section=`) |
| `GET` | `/api/v3/computers-inventory-detail/{id}` | `ComputerDetail`, `ComputerDetailValue`, `computer_detail` | Fetch all inventory sections |
| `POST` | `/api/v2/mdm/commands` | `ComputerReboot`, `ComputerShutdown`, `ComputerLock`, `ComputerLogOut`, `ComputerErase`, `ComputerDeviceInfo`, `ComputerDeviceLocation`, `ComputerSecurityInfo`, `ComputerInstalledApps`, `ComputerManagedApps`, `ComputerCertificates` | Send MDM command (requires managementId from inventory) |
| `POST` | `/api/v1/jamf-management-framework/redeploy/{id}` | `ComputerRedeploy` | Trigger Jamf Management Framework redeploy on a computer |

### Notes

- Inventory stays on **v3** even though some cloud Jamf schemas already advertise v4 (and deprecate v3). Production Jamf does not support inventory v4 yet — do not migrate until it does (then remap RSQL/sort fields such as `general.lastContactTime` → `general.lastCheckIn` / `general.lastContact`).
- All MDM commands fetch each computer's `managementId` via `GET /api/v3/computers-inventory/{id}?section=GENERAL`, then POST to `/api/v2/mdm/commands` with `clientData: [{managementId, clientType: "COMPUTER"}]` and `commandData: {commandType: "..."}` plus optional fields.
- **Device control commands** (destructive; require confirmation unless `-y`): `RESTART_DEVICE`, `SHUT_DOWN_DEVICE`, `LOG_OUT_USER`. **DEVICE_LOCK** and **ERASE_DEVICE** require extra confirmation (LOCK: "Proceed?"; ERASE: type "erase" to confirm).
- **Information/query commands** (non-destructive; queued; results reported asynchronously by device): `DEVICE_INFORMATION`, `DEVICE_LOCATION`, `SECURITY_INFO`, `INSTALLED_APPLICATION_LIST`, `MANAGED_APPLICATION_LIST`, `CERTIFICATE_LIST`.
- `ComputerRedeploy` is a simple POST with no body; no v3 endpoint exists, so Buddy uses v1.
- RSQL filter examples: `general.name=="*Example*"`, `hardware.serialNumber=="C02EXAMPLE0001"`, `udid=="00000000-0000-4000-8000-000000000001"`, `general.lastContactTime=ge=2026-01-01T00:00:00Z`, `general.reportDate=ge=2026-01-01T00:00:00Z`. Sort examples: `general.lastContactTime:desc`, `general.reportDate:desc` (omit = Jamf `general.name:asc`).
- List `section=` is the same enum as get (`GENERAL`, `HARDWARE`, `OPERATING_SYSTEM`, …). Omit = GENERAL only. `computers(raw=True)` / `buddy computer list --json` returns Jamf `results[]` unchanged so extra sections are usable without N+1 `computer()` calls.
- Date field names are **v3**: `general.lastContactTime` (last MDM/binary contact) and `general.reportDate` (last inventory). Do not use v4 (`lastCheckIn` / `lastContact`) or Platform (`lastCheckInTime` / `lastInventoryUpdateTime`) names on this API.

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

buddy script update <id> <file>
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
  └─ GET /api/v3/computers-inventory          (bare → general.name=="*term*"; optional sort/section; page-size=200)

buddy computer get <id> [--section=GENERAL,HARDWARE]
  └─ GET /api/v3/computers-inventory/{id}[?section=…]

buddy computer detail <id>
  └─ GET /api/v3/computers-inventory-detail/{id}

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

# Computers by name, serial, UDID, last contact, or last inventory (v3 field names)
buddy computer list --filter=Example
buddy computer list --filter='general.name=="*Example*"'
buddy computer list --filter='hardware.serialNumber=="C02EXAMPLE0001"'
buddy computer list --filter='udid=="00000000-0000-4000-8000-000000000001"'
buddy computer list --filter='general.lastContactTime=ge=2026-01-01T00:00:00Z' --sort=general.lastContactTime:desc
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
# last MDM/binary contact
computers(
    filter=rsql_ge("general.lastContactTime", rsql_days_ago(1)),
    sort="general.lastContactTime:desc",
)
```

### Filterable Fields by Resource

| Resource | Filterable fields |
|---|---|
| `/v1/packages` | `id`, `fileName`, `packageName`, `categoryId`, `info`, `notes`, `manifestFileName`, `cloudTransferStatus` |
| `/v3/computers-inventory` | `id`, `udid`, `general.name`, `general.lastContactTime`, `general.reportDate`, `general.lastEnrolledDate`, `general.lastCloudBackupDate`, `general.mdmCertificateExpiration`, `general.managementId`, `operatingSystem.version`, `hardware.model`, `hardware.serialNumber`, … (see OpenAPI). Sort: `general.lastContactTime`, `general.reportDate`, `general.name`, `udid`, `id`, … |
| `/v2/mdm/commands` | `uuid`, `clientManagementId`, `command`, `status`, `clientType`, `dateSent`, `validAfter`, `dateCompleted`, `profileId`, `profileIdentifier`, `active` |
| `/v1/scripts`, `/v1/categories`, computer-groups | `id`, `name`, … (see OpenAPI) |
| `/v1/api-integrations` | `id`, `displayName` |
| `/v1/api-roles` | `id`, `displayName` |

### Notes

- Wildcard `*` is only meaningful inside string equality (`==` / `!=`).
- String values are case-insensitive in Jamf's RSQL implementation.
- Computer inventory date filters (`general.lastContactTime`, `general.reportDate`, …) use **unquoted** ISO-8601 UTC (`2026-01-01T00:00:00Z`). `rsql_ge` + `rsql_days_ago` already produce that.
- Quote RSQL values in the shell to prevent glob expansion: `--filter='packageName=="*Chrome*"'`. Bare simple terms like `--filter=Chrome` need no quotes.
- The RSQL grammar is specified at [github.com/jirutka/rsql-parser](https://github.com/jirutka/rsql-parser).

---

## Base URLs

Base URLs are configured per credential in `~/.config/buddy/config.yaml`. Each named credential specifies its own `url` field. There is no hardcoded URL logic.

See `buddy cred help` for how to add or update credentials.
