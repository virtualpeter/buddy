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
		{Action: "Upload", BuddyScript: "`package_upload(file, …, wait_ready?, force?)`", CLI: "`buddy package upload [--wait-ready] [--force]`", Notes: "Create/update by fileName; waits until READY by default (Ctrl-C leaves the upload); --wait-ready=false stops after the file transfer; --force re-uploads"},
		{Action: "Check", BuddyScript: "`package_check(file, id?)`", CLI: "`buddy package check <file> [<package-id>]`", Notes: "Read-only local checksum vs Jamf; exit 1 unless match and usable"},
		{Action: "Scan issues", BuddyScript: "`packages_scan(class?=…)`", CLI: "`buddy package scan [--class=…]`", Notes: "failed-upload / pending / unusable-ready from list fields"},
		{Action: "Wait READY", BuddyScript: "`package_wait_ready(id=…, timeout?=…)`", Notes: "Polls until READY and sha256 set (~10 min default)"},
		{Action: "Check READY", BuddyScript: "`package_ready(id=…)`", Notes: "`true` only when READY + sha256; HTTP 404 → `false`"},
		{Action: "Download", BuddyScript: "`package_download(id=…, path?=…)`", CLI: "`buddy package download`", Notes: "Jamf Cloud / JCDS only; hash-verified"},
	},
	Notes: []string{
		"Package **existence** is not the same as **READY** for attach or download. Full RSQL still works on list filters.",
	},
	AgentNotes: []string{
		"Prefer `package_ready` / `package_wait_ready` / `package_download` — do not hand-roll single GET “exists?”, treat blank sha256 as needs-upload while transfer is pending (hashValue set, usually SHA3), or save a raw JCDS URI without hash verify.",
		"A package record with **no hashValue**, or hashValue set to SHA3-512 of an empty file (`a69f73cc…`) and no size, means the upload stored no bytes — re-upload is appropriate. Mid-transfer has a real hashValue while sha256 is still empty. The default `package_upload` / `buddy package upload` on that pending record skips the file transfer and polls until READY (do not treat it as a failed upload). `wait_ready=False` / `--wait-ready=false` errors instead of waiting and does not re-upload.",
		"After creating a new package record, upload waits until GET succeeds, then streams the multipart POST. A 201 is followed by a short GET loop (about 15s) until the record shows a size or a hash other than SHA3-512 of empty bytes. A 404/500/502/503, a connection reset, or that empty record retries the whole POST (pauses 1s, 2s, 4s, 8s, 16s; six attempts). That retry is not controlled by wait_ready.",
		"package_upload waits until READY by default. wait_ready=False returns once the file transfer finishes. Ctrl-C during the READY poll stops waiting; the upload is already complete.",
		"package_check compares a local .pkg/.dmg to the Jamf record (by fileName, or id=). It does not upload or wait. ok is true only when a checksum matches and the package is READY with sha256.",
		"Use `packages_scan` / `buddy package scan` to find failed-upload, pending, and unusable-ready records from the list API.",
		"Do not attach a package to a policy before READY.",
		"Do not invent Classic/Pro package PUTs from jamf_schema when named builtins exist.",
	},
	Builtins: []docspec.Builtin{
		{
			Name:    `packages`,
			Summary: `List packages`,
			Returns: `list of summary dicts`,
			Params: []docspec.Param{
				{Name: `filter`, Type: `string`, Required: false, Description: `filter`},
			},
		},
		{
			Name:    `package`,
			Summary: `Load one package`,
			Returns: `dict`,
		},
		{
			Name:    `package_ready`,
			Summary: `READY+sha256 check`,
			Returns: `bool`,
		},
		{
			Name:    `package_wait_ready`,
			Summary: `Poll until READY`,
			Returns: `None`,
		},
		{
			Name:    `package_download`,
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
      Does: List package records from Jamf Pro.
      → value: list of {id, packageName, fileName}
      Params: filter?: bare term → packageName=="*term*", or full RSQL. page?: int (omit = all);
        page_size?: int (default 200).
  package(id=?) | package(name=, version=, pattern?=, tenant?, surface?, auth?)
      Does: Load one package by id, or resolve {id, name} by name+version.
      → value: full package dict (id=) or {id, name} (name+version)
      Params: id=: str — GET full record (checksums, size, cloudTransferStatus,
        flags, …). Or name=+version= to resolve by name keyword. pattern?:
        optional package name glob/regex override.
  package_ready(id=, tenant?, surface?, auth?)
      Does: Check whether a package is JCDS READY with a sha256 checksum.
      → value: bool
      Params: id=: package id.
      Notes: True iff the package exists, cloudTransferStatus is "READY", and
        sha256 is non-empty; False if missing or not ready. API errors raise.
  package_wait_ready(id=, timeout?, tenant?, surface?, auth?)
      Does: Poll until the package is READY with sha256 set (nudges JCDS refresh-inventory).
      → None
      Params: id=: package id; timeout?: int seconds (default 600).
      Notes: Prefer after package_upload before attaching to a policy.
  package_download(id=, path?, tenant?, surface?, auth?)
      Does: Download a JCDS package to disk after READY+sha256, verifying the hash.
      → None
      Params: id=: package id; path?: local outfile (default fileName).
      Notes: Streams JCDS pre-signed URI to temp while hashing SHA-256; renames only on match. JCDS only.
  packages_scan(class?, filter?, tenant?, surface?, auth?)
      Does: Scan the packages list for incomplete or unusable records.
      → value: list of {id, packageName, fileName, cloudTransferStatus, class, …}
      Params: class?: "failed-upload" | "pending" | "unusable-ready" | "all" (default)
        or comma-separated; filter?: bare term or RSQL (same as packages).
      Notes: failed-upload (no hashValue, or SHA3-512 of an empty file, and no md5/sha256),
        pending (real hashValue set, not READY), unusable-ready (READY but sha256 empty).
  package_upload(file, name?, category_id?, priority?, notes?, info?, reboot?, wait_ready?, wait_ready_timeout?, force?, tenant?, surface?, auth?)
      Does: Create or update a package record and stream the file to JCDS.
      → None
      Params: file: str path (.pkg/.dmg); name?: display name (default basename);
        category_id?: str (default "-1"); priority?: int 1–20 (default 10);
        notes?/info?: str; reboot?: bool (default false);
        wait_ready?: bool — poll until READY+sha256 after the file transfer or
        skip (default true). False stops once the upload finishes and does not
        change upload retries. Ctrl-C during the poll returns success.
        wait_ready_timeout?: int seconds (default 600; implies wait_ready);
        force?: bool — re-upload even if checksums match or the record looks
        mid-transfer (default false).
      Notes: Same fileName → update path (GET + checksum compare; skip upload if match;
        if hashes blank, status not READY, and a real hashValue is set, wait when
        wait_ready is true — otherwise raise not-ready and do not re-upload;
        no hashValue, or SHA3-512 of an empty file, → treat as failed upload and
        re-upload; apply metadata flags when they differ). New fileName → create
        with flags (no confirm), wait until readable, then stream upload.
        404/500/502/503, connection resets, and a 201 that leaves the empty-file
        hash with no size retry the POST (1s, 2s, 4s, 8s, 16s; six attempts)
        whether or not wait_ready is set.
  package_check(file, id?, tenant?, surface?, auth?)
      Does: Compare a local .pkg/.dmg to a Jamf package record. Does not upload or wait.
      → value: dict {ok, class, id, packageName, fileName, recordFileName,
        cloudTransferStatus, match, matchedBy, localMd5, localSha256, fileNameDiffers}
      Params: file: str path (.pkg/.dmg); id?: package id (omit to look up fileName).
      Notes: ok is true only when a server checksum matches and the package is
        READY with sha256. class is ok, mismatch, missing, pending, failed-upload,
        or unusable-ready. I/O and API failures raise; a missing record does not.
  package_delete(ids, tenant?, surface?, auth?)
      Does: Delete one or more package records.
      → None
      Params: ids: str/int or list of package IDs.
  package_put(id, dict, tenant?, surface?, auth?)
      Does: PUT a full package JSON body (omits server-managed size).
      → value: dict (updated package record)
      Params: id: str; dict: full package body.

`,
	CLIHelp: `buddy package - Manage Jamf Pro package records

Usage:
  buddy [global-flags] package <subcommand> [args...]

Subcommands:
  list [--filter=<term|rsql>] [--sort=version[:asc|desc]] [--json]  List packages
  get <package-id>                                    Show package record details
  download <package-id> [outfile]                     Download package file via JCDS (SHA-256 verified)
  history <package-id>                                Show recent change history (last 20 entries)
  check <file> [<package-id>]                        Compare a local .pkg/.dmg to Jamf (no upload)
  scan [--class=…] [--filter=…]                       Find incomplete/bad package records
  delete <package-id> [package-id...]                Delete one or more package records
  upload <file> [attr-flags…]                        Upload .pkg or .dmg (create or update)
  set <package-id> [attr-flags…]                     Update package metadata

Flags for 'list':
  --filter=<term|rsql>   Bare term → packageName=="*term*"; or full RSQL

Flags for 'scan':
  --class=<name[,name]>  failed-upload | pending | unusable-ready | all (default)
  --filter=<term|rsql>   Same as list (optional narrow)

Flags for 'upload' / 'set':
  --name=<display-name>       Package display name (upload default: filename)
  --notes=<text>              Notes field
  --info=<text>               Info field
  --category=<category-id>    Category ID (-1 to clear; upload default: -1)
  --priority=<1-20>           Install priority (upload default: 10)
  --reboot=true|false         rebootRequired flag (upload default: false)

Flags for 'upload' only:
  --wait-ready[=<seconds>]    After the file transfer or a skip, poll until
                              cloudTransferStatus=READY and sha256 is set.
                              Default is on (timeout 600s / 10 minutes).
                              Ctrl-C stops the wait; the upload is already complete.
  --wait-ready=false          Do not poll after the upload finishes. Does not
                              change retries during the upload.
  --wait-ready-timeout=<sec>  Same as --wait-ready=<sec> (implies --wait-ready)
  --force                     Re-upload even if checksums match or the package
                              looks mid-transfer (hashValue set, sha256 empty)

Arguments for 'check':
  <file>                      Local .pkg or .dmg
  <package-id>                Optional record to compare (default: look up fileName)

Notes:
  --sort=version sorts client-side by a dotted version extracted from the package
  name (latest first). Use --sort=version:asc for oldest first.
  get prints the full package record (checksums, size, cloudTransferStatus, flags, …)
  plus lastChanged fields from history.
  download waits until cloudTransferStatus is READY and sha256 is set, fetches a
  JCDS pre-signed URI, streams to a temp file while hashing SHA-256, and renames
  only on match. JCDS only (Jamf Cloud). Default outfile is the package fileName.
  upload looks up fileName first: existing record → compare checksums and skip or
  replace the file (and apply attr flags when they differ); if comparable checksums
  are empty, status is not READY, and hashValue is set, the default wait skips
  the file transfer and polls until READY; --wait-ready=false skips the POST
  and the poll and errors (do not re-upload); a record with no hashValue, or
  hashValue set to SHA3-512 of an empty file and no size, is a failed upload
  and is re-uploaded; --force overrides skip/not-ready guards.
  New fileName → create metadata with attr flags (no y/N prompt), wait until
  readable, then stream the upload. After a 201, buddy polls the record for
  about 15s until it shows a size or a hash other than SHA3-512 of empty bytes.
  404/500/502/503, connection resets, and that empty record retry the whole POST
  (pauses 1s, 2s, 4s, 8s, 16s; six attempts). --wait-ready does not turn those
  retries on or off.
  check hashes the local file, compares md5/sha256 to the record, and exits 0
  only when they match and the package is READY with sha256. It does not upload
  or wait. Exit 1 reports mismatch, missing, pending, failed-upload, or
  unusable-ready.
  scan classifies from list fields: failed-upload (no hashValue, or SHA3-512 of
  an empty file, and no md5/sha256), pending (real hashValue set, not READY),
  unusable-ready (READY, sha256 empty).
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
  buddy package scan
  buddy package scan --class=failed-upload
  buddy package upload Chrome-121.pkg --name="Google Chrome 121" --category=3 --priority=10
  buddy package upload Chrome-121.pkg --notes="rollout" --reboot=false
  buddy package upload Chrome-121.pkg
  buddy package upload Chrome-121.pkg --wait-ready=900
  buddy package upload Chrome-121.pkg --wait-ready=false
  buddy package upload Chrome-121.pkg --force
  buddy package check Chrome-121.pkg
  buddy package check Chrome-121.pkg 55
  buddy package set 55 --name="Google Chrome 121" --priority=5 --reboot=true
  buddy package delete 55 56
`,
}
