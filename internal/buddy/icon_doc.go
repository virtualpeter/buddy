package buddy

import "buddy/internal/docspec"

func init() {
	docspec.Register(IconsTopic)
}

// IconsTopic is the structured doc source for docs/icons.
// Edit here, then run `make gendocs`.
var IconsTopic = docspec.Topic{
	ID:      `docs/icons`,
	Title:   `Icons`,
	Section: `Resources`,
	Keywords: []string{
		`icon`,
		`icons`,
		`self service`,
		`self_service_icon`,
		`ics`,
		`catalog-cache`,
	},
	Summary: `Jamf Pro Self Service icons. Tenant-local ids; ICS stores bytes by SHA-256. List is a weekly catalog cache (id-walk); Jamf has no collection GET.`,
	Surfaces: []docspec.SurfaceRow{
		{Action: "List", BuddyScript: "`icons(filter?, refresh?=False, dropcache?=False, progress?=False)`", CLI: "`buddy icon list`", Notes: "Weekly JSON cache keyed by tenant; `--refresh` probes 20 past max; `--dropcache` rescans from 1. CLI (and `progress=True` / `-v`) prints cache age, max id, and live scan progress."},
		{Action: "Load", BuddyScript: "`icon(id=…)` / `icon(name=…)`", CLI: "`buddy icon get <icon-id>`", Notes: "`id=` is cache-first then live GET; `name=` is cache only"},
		{Action: "Upload", BuddyScript: "`icon_upload(file)`", CLI: "`buddy icon upload <file>`", Notes: "POST multipart unless catalog cache already has this sha256 (returns lowest live id)"},
		{Action: "Check", BuddyScript: "`icon_check(file, icon_id=?)`", CLI: "`buddy icon check <file> [<icon-id>]`", Notes: "SHA-256 → lowest catalog id; HEAD that url. Missing cache: run list first. Optional icon-id byte-compares."},
		{Action: "Download", BuddyScript: "`icon_download(id, path)`", CLI: "`buddy icon download <icon-id> [path]`", Notes: "Follows metadata `url` (ICS, no token). Pro `download/{id}` 500s for ICS-backed icons."},
		{Action: "Attach to policy", BuddyScript: "`policy_set_icon(id, file=… | icon_id=…)`", CLI: "`buddy policy set icon <policy-id> <image-file>`", Notes: "Classic PUT `self_service_icon.id`"},
	},
	Notes: []string{
		"Jamf has **no list API** (`GET /api/v1/icon` is 405) and no published delete. `icons()` / `buddy icon list` keep a weekly JSON catalog under `~/.config/buddy/catalog-cache/<tenant>/icons.json` (TTL 7 days; two auths on the same tenant share it). A missing cache walks `GET /api/v1/icon/{id}` from 1; a stale cache or `--refresh` keeps cached rows and only probes 20 ids past `max_id` (grow-on-hit). `--dropcache` / `dropcache=True` ignores the on-disk catalog and walks from 1 again. Cached ids are not re-GETed. `icon_upload` upserts the new id (and bumps `max_id`) so the next refresh does not have to rediscover it. First walk is one request per probed id (can take minutes). `buddy icon list` prints a warning, cache age + max id, and live scan progress on stderr; `icons()` does the same only with `progress=True` or `-v`. Do not scrape Classic `policies.html`.",
		"ICS URLs look like `https://ics.services.jamfcloud.com/icon/hash_<sha256>`. Hosts vary by region (`ics`, `use1.ics`, `use2.ics`, `usw2.ics`, `apse2.ics`, `apne1.ics`, `euw2.ics`, `euc1.ics`). Some records use the instance download path (`https://<tenant>.jamfcloud.com/api/v1/icon/download/{id}`) instead of a hash URL — Jamf often emits a double slash after the host; buddy normalizes it. Catalog rows store sha256 from a `hash_` URL, or (on refresh only) by hashing an unauthenticated GET of an instance `url`.",
		"`icon_check` SHA-256s the **local** file and looks up the catalog cache. Missing cache is an error — run `buddy icon list` / `icons()` first. The lowest id with that sha256 is returned; buddy HEADs that row’s `url` (no token). `present` is that probe 200/206; `ics` is true only for a live `hash_` URL. Duplicate catalog hashes use the lowest id. Unknown hash → `present=false` (no icon_id). Optional `icon_id=` / `buddy icon check <file> <icon-id>` byte-compares that tenant icon (cache optional). Jamf may re-encode PNGs, so a local hash can miss the stored sha256.",
		"Some tenant icons are **instance-only**: metadata `url` is `/api/v1/icon/download/{id}` with no `hash_`. After `icon list`, `icon_check(file)` HEADs that instance url. Or pass `icon_id=` / `buddy icon check <file> <icon-id>` to compare SHA-256 to that icon’s downloaded bytes.",
		"`icon_download` / `IconDownload` GETs that `url` **without** a Jamf token (ICS and the instance download path are public). `GET /api/v1/icon/download/{id}` through the Pro API **500s** for ICS-backed icons; sending a bearer token to the instance `url` can 401. The Pro download path is only a fallback.",
		"A 200 on ICS does **not** mint a tenant icon id. Attach still needs `icon_upload` / `POST /api/v1/icon` on that tenant. Cross-tenant `policy_copy` re-uploads bytes; it does not skip POST because ICS HEAD was 200.",
	},
	AgentNotes: []string{
		"Use `icons()` (weekly catalog cache; not a Jamf list API) / `icon` / `icon_upload` / `icon_download` / `policy_set_icon`. Do not invent Classic icon CRUD from schema. Do not scrape `policies.html`.",
		"Do not skip dest POST on policy copy because ICS already has the hash. `icon_upload` may reuse a dest catalog-cache id when the local sha256 matches.",
		"Pass either `file` or `icon_id` to `policy_set_icon`, not both.",
		"`icon(name=)` is cache-only (missing cache, unknown name, or duplicate names raise). `icon(id=)` is cache-first then live GET.",
		"icon_check without icon_id= needs the catalog cache (run icons() / icon list first). Lowest matching sha256 id; HEAD that url. icon_id= byte-compares and does not require a cache.",
	},
	Builtins: []docspec.Builtin{
		{
			Name:    `icons`,
			Summary: `List icons from the weekly catalog cache`,
			Returns: `list of {id, name, url, sha256?}`,
			Params: []docspec.Param{
				{Name: `filter`, Type: `string`, Description: `substring match on name or id`},
				{Name: `refresh`, Type: `boolean`, Description: `probe 20 ids past max even if the cache is fresh`},
				{Name: `dropcache`, Type: `boolean`, Description: `ignore the on-disk catalog and walk from id 1`},
				{Name: `progress`, Type: `boolean`, Description: `print cache status and live scan progress on stderr (CLI list always does this)`},
			},
		},
		{
			Name:    `icon`,
			Summary: `Load one icon`,
			Returns: `dict {id, name, url, sha256?}`,
			Params: []docspec.Param{
				{Name: `id`, Type: `int`, Description: `icon id (cache-first, then live GET)`},
				{Name: `name`, Type: `string`, Description: `exact filename from the catalog cache`},
			},
		},
		{
			Name:    `icon_upload`,
			Summary: `Upload an icon file, or reuse a catalog-cache id`,
			Returns: `dict {id, name, url, sha256?}`,
		},
		{
			Name:    `icon_check`,
			Summary: `SHA-256 against catalog cache; HEAD that url`,
			Returns: `dict {url, present, sha256, ics, icon_id?}`,
		},
		{
			Name:    `icon_download`,
			Summary: `Download icon bytes`,
			Returns: `None`,
		},
	},
	ExamplesBuddy: []string{
		`def main():
    rec = icon_upload(file="santa.png")
    policy_set_icon(id=21, icon_id=rec["id"])
    return rec`,
		`def main():
    rows = icons(filter="santa")
    return icon(name="santa.png") if rows else None`,
	},
	Related: []docspec.RelatedLink{
		{Title: `Policies`, Ref: `/docs/docs/policies`},
		{Title: `API endpoints`, Ref: `/docs/docs/api-endpoints`},
	},
	HelpExport: `icons`,
	BuiltinsHelp: `ICON (Jamf Pro Self Service icons)
  icons(filter?, refresh?=False, dropcache?=False, progress?=False, tenant?, surface?, auth?)
      Does: List tenant icons from the weekly catalog cache (id-walk when stale, refresh=True, or dropcache=True).
      → value: list of {id, name, url, sha256?}
      Params: filter?: substring on name or id. refresh?: bool (default False). dropcache?: bool
        (default False; walk from 1). progress?: bool (default False; also on with process -v).
        tenant?/surface?/auth?: session overrides; omit for process defaults.
      Notes: Jamf has no GET /api/v1/icon collection (405) and no published delete. Cache lives
        under ~/.config/buddy/catalog-cache/<tenant>/icons.json (TTL 7 days; shared by auths on
        that tenant). No page/page_size. Missing cache walks from id 1. Stale/refresh keeps
        cached rows and probes 20 ids past max. dropcache=True ignores the catalog and walks
        from 1. icon_upload upserts the new id. First walk can take minutes. CLI always prints
        cache age, max id, and live scan progress on stderr; icons() only with progress=True or -v.
  icon(id=?) | icon(name=?, tenant?, surface?, auth?)
      Does: Load one icon by tenant-local id (cache-first) or exact name (cache only).
      → value: dict {id, name, url, sha256?}
      Params: pass either id= or name=, not both. tenant?/surface?/auth?: session overrides.
      Notes: id= miss does a live GET and upserts that row (does not reset fetched_at).
        name= raises if the cache is missing, the name is unknown, or several icons share it.
  icon_upload(file, tenant?, surface?, auth?)
      Does: Multipart-upload a local image, or return the lowest live catalog-cache id
        whose sha256 matches the file (no POST).
      → value: dict {id, name, url, sha256?}
      Params: file: str (filesystem path). tenant?/surface?/auth?: omit for process defaults.
      Notes: Cache miss (or stale cache without this hash) POSTs; Jamf assigns a new tenant
        id. Upserts that row (max_id bumped; fetched_at unchanged). Jamf may re-encode
        PNGs so a local hash can miss the stored sha256 and still POST.
  icon_check(file, icon_id=?, tenant?, surface?, auth?)
      Does: SHA-256 the local file, find the lowest catalog-cache id with that
        hash, and HEAD that row's url. Optional icon_id= byte-compares that tenant icon.
      → value: dict {url, present, sha256, ics} (+ icon_id when known)
      Params: file: str (filesystem path). icon_id?: int (tenant icon to byte-compare).
        tenant?/surface?/auth?: session overrides.
      Notes: Missing cache raises — run icons() / buddy icon list first. Duplicate
        sha256 uses the lowest id. present is the url probe (200/206). ICS 403 is
        not present. ics is true only for a live hash_ URL. Unknown hash returns
        present=false (no icon_id). icon_id= does not require a cache. Jamf may
        re-encode PNGs (local hash miss). present/ics do not mint a tenant id.
  icon_download(id, path, tenant?, surface?, auth?)
      Does: Download icon image bytes to a local path (follows metadata url; Pro download API is fallback).
      → None
      Params: id: int/str; path: str (destination file). tenant?/surface?/auth?: omit for
        process defaults.
      Notes: GET /api/v1/icon/download/{id} 500s for ICS-backed icons. Do not send a
        Jamf token to the metadata url.

`,
	CLIHelp: `buddy icon - Manage Jamf Pro Self Service icons

Usage:
  buddy [global-flags] icon <subcommand> [args...]

Subcommands:
  list                          List icons from the weekly catalog cache
  get <icon-id>                 Show icon metadata (id, name, url) as JSON
  upload <file>                 Upload or reuse by catalog sha256; print {id, name, url} JSON
  check <file> [<icon-id>]      SHA-256 vs catalog cache; HEAD that url (or byte-compare icon-id)
  download <icon-id> [path]     Write image bytes (metadata url; Pro download is fallback)

Flags for 'list':
  --filter=<keyword>            Substring match on name or id
  --refresh                     Probe 20 ids past max (skips cached ids)
  --dropcache                   Ignore the catalog cache and walk from id 1

Notes:
  Jamf has no icon list API (GET /api/v1/icon is 405) and no published delete. A
  missing cache walks GET /api/v1/icon/{id} from 1. Stale cache or --refresh keeps
  cached rows and probes 20 ids past max. --dropcache ignores the catalog and
  walks from 1 (first walk can take minutes). Stderr shows a warning, cache age +
  icon count + max id, and live scan progress. Cache:
  ~/.config/buddy/catalog-cache/<tenant>/icons.json (shared by auths on that tenant)
  upload hashes the file and returns the lowest live cache id with that sha256
  (no POST) when icon list has already seen it; otherwise POSTs and upserts the cache.
  check without <icon-id> needs the catalog cache (run list first). Lowest id with
  that file sha256; HEAD that url. Missing hash is present=false. Optional
  <icon-id> byte-compares downloaded bytes (cache optional).
  get's url is the fetchable image (ICS or instance). Do not GET /api/v1/icon/download/{id}
  for ICS-backed icons — that endpoint 500s; use download (or GET the url with no token).
  Attach an icon to a policy with: buddy policy set icon <policy-id> <image-file>

Examples:
  buddy icon list
  buddy icon list --filter=santa --refresh
  buddy icon list --dropcache
  buddy icon get 2
  buddy icon upload ./santa.png
  buddy icon check ./santa.png
  buddy icon check ./systemsetting.png 1
  buddy icon download 2
  buddy icon download 2 ./santa.png
  buddy policy set icon 21 ./santa.png
`,
}
