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
	},
	Summary: `Jamf Pro Self Service icons. Tenant-local ids; ICS stores bytes by SHA-256. No list API.`,
	Surfaces: []docspec.SurfaceRow{
		{Action: "Load", BuddyScript: "`icon(id=…)`", CLI: "`buddy icon get <icon-id>`", Notes: "`{id, name, url}` from `GET /api/v1/icon/{id}`"},
		{Action: "Upload", BuddyScript: "`icon_upload(file)`", CLI: "`buddy icon upload <file>`", Notes: "POST multipart; Jamf returns `{id, name, url}`"},
		{Action: "Check ICS", BuddyScript: "`icon_check(file, icon_id=?)`", CLI: "`buddy icon check <file> [<icon-id>]`", Notes: "Local SHA-256 + ICS HEAD; optional icon-id compares tenant bytes (instance-only icons are not on ICS)"},
		{Action: "Download", BuddyScript: "`icon_download(id, path)`", CLI: "`buddy icon download <icon-id> [path]`", Notes: "Follows metadata `url` (ICS, no token). Pro `download/{id}` 500s for ICS-backed icons."},
		{Action: "Attach to policy", BuddyScript: "`policy_set_icon(id, file=… | icon_id=…)`", CLI: "`buddy policy set icon <policy-id> <image-file>`", Notes: "Classic PUT `self_service_icon.id`"},
	},
	Notes: []string{
		"There is **no list API** (`GET /api/v1/icon` is 405). Do not probe sequential ids.",
		"ICS URLs look like `https://ics.services.jamfcloud.com/icon/hash_<sha256>`. Hosts vary (`ics`, `use1.ics`, `use2.ics`). Some records use the instance download path (`https://<tenant>.jamfcloud.com/api/v1/icon/download/{id}`) instead of a hash URL — Jamf often emits a double slash after the host; buddy normalizes it.",
		"`icon_check` SHA-256s the **local** file and HEADs ICS (no Jamf token). ICS 403 (S3 AccessDenied) is treated as not present — the same response as a missing hash. Jamf may re-encode PNGs, so check can miss even after a successful upload of that file. After upload, Jamf’s returned `url` is the source of truth.",
		"Some tenant icons are **instance-only**: metadata `url` is `/api/v1/icon/download/{id}` with no `hash_`. Those bytes are not on ICS, so `icon_check(file)` is `present=false` / `ics=false` even right after `icon_download`. Pass `icon_id=` / `buddy icon check <file> <icon-id>` to compare SHA-256 to that icon’s downloaded bytes (`present=true`, `ics` still false, `url` is the instance path).",
		"`icon_download` / `IconDownload` GETs that `url` **without** a Jamf token (ICS and the instance download path are public). `GET /api/v1/icon/download/{id}` through the Pro API **500s** for ICS-backed icons; sending a bearer token to the instance `url` can 401. The Pro download path is only a fallback.",
		"A 200 on ICS does **not** mint a tenant icon id. Attach still needs `icon_upload` / `POST /api/v1/icon` on that tenant. Cross-tenant `policy_copy` re-uploads bytes; it does not skip POST because ICS HEAD was 200.",
	},
	AgentNotes: []string{
		"Do not invent `icons()` or Classic icon CRUD from schema. Use `icon` / `icon_upload` / `icon_download` / `policy_set_icon`.",
		"Do not skip dest POST on policy copy because ICS already has the hash.",
		"Pass either `file` or `icon_id` to `policy_set_icon`, not both.",
		"icon_check without icon_id= is ICS-only. Instance download URLs are not on ICS; pass icon_id= to compare bytes.",
	},
	Builtins: []docspec.Builtin{
		{
			Name:    `icon`,
			Summary: `Load one icon`,
			Returns: `dict {id, name, url}`,
			Params: []docspec.Param{
				{Name: `id`, Type: `int`, Required: true, Description: `icon id`},
			},
		},
		{
			Name:    `icon_upload`,
			Summary: `Upload an icon file`,
			Returns: `dict {id, name, url}`,
		},
		{
			Name:    `icon_check`,
			Summary: `SHA-256 + ICS HEAD`,
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
	},
	Related: []docspec.RelatedLink{
		{Title: `Policies`, Ref: `/docs/docs/policies`},
		{Title: `API endpoints`, Ref: `/docs/docs/api-endpoints`},
	},
	HelpExport: `icons`,
	BuiltinsHelp: `ICON (Jamf Pro Self Service icons)
  icon(id=, tenant?, surface?, auth?)
      Does: Load one icon by tenant-local id.
      → value: dict {id, name, url}
      Params: id=: int/str. tenant?/surface?/auth?: session overrides.
      Notes: No icons() list — GET /api/v1/icon is 405. OpenAPI often omits name;
        live GET includes the filename the UI shows.
  icon_upload(file, tenant?, surface?, auth?)
      Does: Multipart-upload a local image; Jamf assigns a tenant icon id (ICS hash is opaque).
      → value: dict {id, name, url}
      Params: file: str (filesystem path). tenant?/surface?/auth?: omit for process defaults.
  icon_check(file, icon_id=?, tenant?, surface?, auth?)
      Does: SHA-256 the local file and HEAD ICS (unauthenticated) to see if that hash URL exists.
        Optional icon_id= also compares bytes to that tenant icon.
      → value: dict {url, present, sha256, ics} (+ icon_id when compared)
      Params: file: str (filesystem path). icon_id?: int (tenant icon to byte-compare).
        tenant?/surface?/auth?: only with icon_id=; omit for process defaults.
      Notes: ICS-only when icon_id omitted. Probes ics.services then use2.ics. First 200/206
        wins. ICS 403 AccessDenied is not present (same as missing). If none, present=false,
        ics=false, url is the canonical ics.services hash URL. Instance-only icons (download
        path, no hash_) are never on ICS — pass icon_id= after download to verify bytes.
        Jamf may re-encode PNGs (ICS false negative). present/ics do not mint a tenant id —
        still POST to attach.
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
  get <icon-id>                 Show icon metadata (id, name, url) as JSON
  upload <file>                 Upload a local image; print {id, name, url} JSON
  check <file> [<icon-id>]      SHA-256 + ICS HEAD; optional icon-id compares tenant bytes

Notes:
  There is no list subcommand (Jamf has no icon catalog API).
  check without <icon-id> only probes ICS (not a tenant id; does not skip upload).
  Instance-only icons (url is /api/v1/icon/download/{id}, no hash_) are not on ICS —
  check <file> is present=false; pass the icon-id to verify downloaded bytes.
  get's url is the fetchable image (ICS or instance). Do not GET /api/v1/icon/download/{id}
  for ICS-backed icons — that endpoint 500s; use download (or GET the url with no token).
  Attach an icon to a policy with: buddy policy set icon <policy-id> <image-file>

Examples:
  buddy icon get 2
  buddy icon upload ./santa.png
  buddy icon check ./santa.png
  buddy icon check ./systemsetting.png 1
  buddy icon download 2
  buddy icon download 2 ./santa.png
  buddy policy set icon 21 ./santa.png
`,
}
