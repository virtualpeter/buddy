package buddy

import "buddy/internal/docspec"

func init() {
	docspec.Register(PackagesTopic)
}

// PackagesTopic is the structured doc source for docs/packages.
// Edit here, then run `make gendocs`.
var PackagesTopic = docspec.Topic{
	ID:      `docs/packages`,
	Title:   `Packages / JCDS`,
	Section: `Resources`,
	Keywords: []string{
		`package`,
		`packages`,
		`jcds`,
		`ready`,
		`download`,
		`upload`,
		`cloudTransferStatus`,
	},
	Summary: `Jamf Pro packages and Jamf Cloud Distribution Service (JCDS) readiness. Package **existence** is not the same as **READY** for attach/download.`,
	Surfaces: []docspec.SurfaceRow{
		{Action: "List / load", BuddyScript: "`packages(filter=…)` / `package(id=…)`", CLI: "`buddy package list` / `get`", Notes: "Or `name=` + `version=`; bare filter → `packageName==\"*term*\"`"},
		{Action: "Wait READY", BuddyScript: "`package_wait_ready(id=…, timeout?=…)`", Notes: "Polls until READY and sha256 set (~10 min default)"},
		{Action: "Check READY", BuddyScript: "`package_ready(id=…)`", Notes: "`true` only when READY + sha256; HTTP 404 → `false`"},
		{Action: "Download", BuddyScript: "`package_download(id=…, path?=…)`", CLI: "`buddy package download`", Notes: "Jamf Cloud / JCDS only; hash-verified"},
	},
	Notes: []string{
		"Package **existence** is not the same as **READY** for attach or download. Full RSQL still works on list filters.",
	},
	AgentNotes: []string{
		"Prefer `package_ready` / `package_wait_ready` / `package_download` — do not hand-roll single GET “exists?”, treat blank sha256 as needs-upload while transfer is pending, or save a raw JCDS URI without hash verify.",
		"Do not attach a package to a policy before READY.",
		"Do not invent Classic/Pro package PUTs from jamf_schema when named builtins exist.",
	},
	Builtins: []docspec.Builtin{
			{
				Name: `packages`,
				Summary: `List packages`,
				Returns: `list of summary dicts`,
				Params: []docspec.Param{
					{Name: `filter`, Type: `string`, Required: false, Description: `filter`},
				},
			},
			{
				Name: `package`,
				Summary: `Load one package`,
				Returns: `dict`,
			},
			{
				Name: `package_ready`,
				Summary: `READY+sha256 check`,
				Returns: `bool`,
			},
			{
				Name: `package_wait_ready`,
				Summary: `Poll until READY`,
				Returns: `None`,
			},
			{
				Name: `package_download`,
				Summary: `JCDS download`,
				Returns: `None`,
			},
		},
	ExamplesBuddy: []string{
		`def main():
    package_wait_ready(id="42")
    return package_ready(id="42")`,
	},
	Related: []docspec.RelatedLink{
		{Title: `Policies`, Ref: `/docs/docs/policies`},
		{Title: `API endpoints`, Ref: `/docs/docs/api-endpoints`},
		{Title: `Change batches`, Ref: `/docs/docs/change-batches`},
	},
	HelpExport: `packages`,
	BuiltinsHelp: `PACKAGE
  packages(filter?, page?, page_size?, tenant?, surface?, auth?)
      → value: list of {id, packageName, fileName}
      Params: filter?: bare term → packageName=="*term*", or full RSQL.
  package(id=?) | package(name=, version=, pattern?=, tenant?, surface?, auth?)
      → value: full package dict (id=) or {id, name} (name+version)
      Params: id=: str — GET full record (checksums, size, cloudTransferStatus,
        flags, …). Or name=+version= to resolve by name keyword. pattern?:
        optional package name glob/regex override.
  package_ready(id=, tenant?, surface?, auth?) → value: bool
      Params: id=: package id; tenant?/surface?/auth?: str.
      Returns: True iff the package exists, cloudTransferStatus is "READY", and
        sha256 is non-empty; False if missing or not ready. API errors raise.
  package_wait_ready(id=, timeout?, tenant?, surface?, auth?) → value: None
      Params: id=: package id; timeout?: int seconds (default 600); tenant?/surface?/auth?: str.
      Polls until cloudTransferStatus is READY and sha256 is set (nudges JCDS
      refresh-inventory). Prefer after package_upload before attaching to a policy.
  package_download(id=, path?, tenant?, surface?, auth?) → value: None
      Params: id=: package id; path?: local outfile (default fileName); tenant?/surface?/auth?: str.
      Waits READY+sha256, streams JCDS pre-signed URI to temp while hashing
      SHA-256, renames only on match. JCDS only.
  package_upload(file, name?, category_id?, priority?, notes?, info?, reboot?, tenant?, surface?, auth?)
      → value: None
      Params: file: str path (.pkg/.dmg); name?: display name (default basename);
        category_id?: str (default "-1"); priority?: int 1–20 (default 10);
        notes?/info?: str; reboot?: bool (default false); tenant?/surface?/auth?: str.
        Same fileName → update path (GET + checksum compare; skip upload if match;
        if hashes blank and status not READY, raise not-ready — do not re-upload;
        apply metadata flags when they differ). New fileName → create with flags,
        wait until readable, then stream upload.
  package_delete(ids) → None
  package_put(package_id, dict, tenant?, surface?, auth?) → value: dict
      Params: package_id: str; dict: full package body; tenant?/surface?/auth?: str.
      Returns: value: updated package dict. PUT omits server-managed size.

`,
	CLIHelp: `buddy package - Manage Jamf Pro package records

Usage:
  buddy [global-flags] package <subcommand> [args...]

Subcommands:
  list [--filter=<term|rsql>] [--sort=version[:asc|desc]]  List packages
  get <package-id>                                    Show package record details
  download <package-id> [outfile]                     Download package file via JCDS (SHA-256 verified)
  history <package-id>                                Show recent change history (last 20 entries)
  delete <package-id> [package-id...]                Delete one or more package records
  upload <file> [attr-flags…]                        Upload .pkg or .dmg (create or update)
  set <package-id> [attr-flags…]                     Update package metadata

Flags for 'list':
  --filter=<term|rsql>   Bare term → packageName=="*term*"; or full RSQL

Flags for 'upload' / 'set':
  --name=<display-name>       Package display name (upload default: filename)
  --notes=<text>              Notes field
  --info=<text>               Info field
  --category=<category-id>    Category ID (-1 to clear; upload default: -1)
  --priority=<1-20>           Install priority (upload default: 10)
  --reboot=true|false         rebootRequired flag (upload default: false)

Notes:
  --sort=version sorts client-side by a dotted version extracted from the package
  name (latest first). Use --sort=version:asc for oldest first.
  get prints the full package record (checksums, size, cloudTransferStatus, flags, …)
  plus lastChanged fields from history.
  download waits until cloudTransferStatus is READY and sha256 is set, fetches a
  JCDS pre-signed URI, streams to a temp file while hashing SHA-256, and renames
  only on match. JCDS only (Jamf Cloud). Default outfile is the package fileName.
  upload looks up fileName first: existing record → compare checksums and skip or
  replace the file (and apply attr flags when they differ); if checksums are empty
  and cloudTransferStatus is not READY, report not ready (do not re-upload — wait
  with wait-ready); new fileName → create metadata with attr flags, wait until
  readable, then upload.
  wait-ready / package_ready require cloudTransferStatus READY and a non-empty sha256.
  Only .pkg and .dmg are supported.

Examples:
  buddy package list
  buddy package list --filter=Chrome
  buddy package list --filter='packageName=="*Chrome*"'
  buddy package list --filter='packageName=="*Chrome*"' --sort=version
  buddy package get 55
  buddy package download 55
  buddy package download 55 ./Chrome-121.pkg
  buddy package history 55
  buddy package upload Chrome-121.pkg --name="Google Chrome 121" --category=3 --priority=10
  buddy package upload Chrome-121.pkg --notes="rollout" --reboot=false
  buddy package set 55 --name="Google Chrome 121" --priority=5 --reboot=true
  buddy package delete 55 56
`,
}
