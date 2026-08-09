package buddy

import "buddy/internal/docspec"

//go:generate go run ../../cmd/gendocs

func init() {
	docspec.Register(AppsTopic)
}

// AppsTopic is the structured doc source for policy apps (buddy app / apps / app / app_upgrade).
// Edit here, then run `make gendocs` (not docs/apps.md).
var AppsTopic = docspec.Topic{
	ID:      "docs/apps",
	Title:   "Policy apps",
	Section: "Resources",
	Keywords: []string{
		"app", "apps", "policy app", "policy apps", "self service",
		"app upgrade", "app import", "app_reconcile", "desired state",
		"install-next", "cache policy",
	},
	Summary: "**Policy apps** are not a Jamf object type. They are a convention: Self Service + packages, discovered and upgraded as one unit via `buddy app` / `apps(...)`.\n\n" +
		"Classic **Mac App Store** records are separate — use [`macapp` / `macapps`](/docs/docs/macapps).",
	Kinds: []docspec.KindRow{
		{
			Kind:    "single",
			Shape:   "One Self Service policy with the Install package attached (often Ongoing)",
			Upgrade: "In-place package swap + SS description version",
		},
		{
			Kind:    "install",
			Shape:   "SS script (param4) calls `install-{app}`, `install-{app}-next`, or `{app}-install`; one enabled install policy holds the package",
			Upgrade: "Create replacement install policy, disable prior",
		},
		{
			Kind:    "cache",
			Shape:   "Same as install, plus versioned `cache-{app}-{version}` policies",
			Upgrade: "Keep old cache; new cache + replace install; optional SS description bump",
		},
	},
	Surfaces: []docspec.SurfaceRow{
		{Action: "List", BuddyScript: "`apps(filter?)`", CLI: "`buddy app list`", Notes: "Hybrid policy list + name heuristics; approximate"},
		{Action: "Get", BuddyScript: "`app(name=…)`", CLI: "`buddy app get`", Notes: "Authoritative Classic XML classify"},
		{Action: "Import", BuddyScript: "", CLI: "`buddy app import <name> [-o path]`", Notes: "Emit desired-state `.buddy` from get"},
		{Action: "Reconcile", BuddyScript: "`app_reconcile(desired, apply?)`", CLI: "run the imported script", Notes: "Diff live vs desired → change-batch plan / apply"},
		{Action: "Upgrade", BuddyScript: "`app_upgrade(name, version, …)`", CLI: "`buddy app upgrade`", Notes: "One-shot imperative bump by kind"},
	},
	Notes: []string{
		"Discovery starts from policies whose **name contains `[SS]`**. List keeps Install…[SS] and/or install/cache siblings; inventory/config `[SS]` stubs are omitted. App slugs treat hyphens and spaces as equivalent (`claude-cli` ↔ `Claude CLI`). When v1 `triggerManual` is present, install siblings with a custom trigger are preferred.",
		"Jamf does **not** publish a Pro policy CRUD API; RSQL on `/api/v1/policies` cannot filter by Self Service (only `id`, `name`, `enabled`, `categoryId`, `executionFrequency`, `triggerCheckin`, `triggerManual`).",
		"Desired-state scripts declare name/kind/version/triggers/package_pattern. Edit `VERSION` then re-run to plan a bump. v1 does not greenfield-create a missing app — use `setup_app_classic` for that.",
		"Wait until the target package is READY (`package_wait_ready` / `package_ready`) before apply/upgrade.",
	},
	AgentNotes: []string{
		"List is approximate; call `app(name=…)` / `buddy app get` before mutate or upgrade.",
		"Prefer `buddy app import` → edit desired → `app_reconcile` for durable version bumps; `app_upgrade` is one-shot imperative.",
		"Do not invent Classic PUTs for app upgrades — use `app_reconcile` / `app_upgrade` (or policy package / Self Service helpers).",
		"Wait until the target package is READY in Jamf (`package_wait_ready` / `package_ready`) before reconcile apply or `app_upgrade`.",
		"Prefer `docs/apps` then named builtins via `buddyscript`; Classic Mac App Store is `macapp` / `macapps`, not `app` / `apps`.",
		"Platform-only auth cannot list apps (Pro `/api/v1/policies` via gateway returns 403); use an instance-surface auth.",
	},
	Builtins: []docspec.Builtin{
		{
			Name:    "apps",
			Summary: "`apps(filter?)` or `buddy app list [--filter=…]`",
			Returns: "list of summary dicts {name, kind, version, ss_policy_id, install_trigger, …}",
			Params: []docspec.Param{
				{Name: "filter", Type: "string", Description: "Client-side substring on name/kind/version/trigger/package"},
				{Name: "tenant", Type: "string", Description: "Session override; omit for process defaults"},
				{Name: "surface", Type: "string", Description: "Session override"},
				{Name: "auth", Type: "string", Description: "Session override"},
			},
				Notes: []string{
				"Name-index from one hybrid policy list ([SS] in name); kind/version may be approximate.",
				"Authoritative classify is `app(name=…)`, not the list row alone.",
			},
		},
		{
			Name:    "app",
			Summary: "`app(name=…)` or `buddy app get <name>`",
			Returns: "dict (summary + linked install/cache policies)",
			Params: []docspec.Param{
				{Name: "name", Type: "string", Required: true, Description: "App identifier (e.g. santa, vscode)"},
				{Name: "tenant", Type: "string", Description: "Session override"},
				{Name: "surface", Type: "string", Description: "Session override"},
				{Name: "auth", Type: "string", Description: "Session override"},
			},
			Notes: []string{"Classic XML classify (excludes inventory-only SS stubs)."},
		},
		{
			Name:    "app_upgrade",
			Summary: "`app_upgrade(name, version, package_pattern?)` or `buddy app upgrade <name> --version=… [--package-pattern=…]`",
			Returns: "None",
			Params: []docspec.Param{
				{Name: "name", Type: "string", Required: true, Description: "App identifier"},
				{Name: "version", Type: "string", Required: true, Description: "Target version"},
				{Name: "package_pattern", Type: "string", Description: "Package name glob"},
				{Name: "tenant", Type: "string", Description: "Session override"},
				{Name: "surface", Type: "string", Description: "Session override"},
				{Name: "auth", Type: "string", Description: "Session override"},
			},
			Notes: []string{
				"One-shot imperative by kind. Prefer import + `app_reconcile` for change-record bumps.",
			},
		},
		{
			Name:    "app_reconcile",
			Summary: "`app_reconcile(desired, apply?=False)` — diff live AppGet vs desired dict; plan_change / optional run_change",
			Returns: "dict {markdown, fingerprints} on plan; None after apply",
			Params: []docspec.Param{
				{Name: "desired", Type: "dict", Required: true, Description: "name, version, kind?, install_trigger?, cache_trigger?, package_pattern?, title?, summary?, risk?, window?"},
				{Name: "apply", Type: "boolean", Description: "When True, run_change after plan (default False)"},
				{Name: "tenant", Type: "string", Description: "Session override"},
				{Name: "surface", Type: "string", Description: "Session override"},
				{Name: "auth", Type: "string", Description: "Session override"},
			},
			Notes: []string{
				"Invented steps mirror install/cache change batches; single uses in-place upgrade_app.",
				"Fails if the app is missing (no greenfield create in v1).",
			},
		},
	},
	CLI: []docspec.CLICommand{
		{
			Name:    "app list",
			Usage:   "buddy [global-flags] app list [--filter=<keyword>]",
			Summary: "Discover policy apps from Self Service policies",
			Params: []docspec.Param{
				{Name: "filter", Type: "string", Description: "Client-side substring on name, kind, version, trigger, or package"},
			},
			Notes: []string{
				"List is a fast name-index from one hybrid policy list; kind/version/trigger may be approximate.",
			},
		},
		{
			Name:    "app get",
			Usage:   "buddy [global-flags] app get <name>",
			Summary: "Show one policy app (kind, linked policies, package)",
			Params: []docspec.Param{
				{Name: "name", Type: "string", Required: true, Description: "App identifier"},
			},
		},
		{
			Name:    "app import",
			Usage:   "buddy [global-flags] app import <name> [--output=<path>|-o <path>]",
			Summary: "Write a desired-state .buddy script from app get",
			Params: []docspec.Param{
				{Name: "name", Type: "string", Required: true, Description: "App identifier"},
				{Name: "output", Type: "string", Description: "Output path (default apps/<slug>.buddy)"},
			},
		},
		{
			Name:    "app upgrade",
			Usage:   "buddy [global-flags] app upgrade <name> --version=<version> [--package-pattern=…]",
			Summary: "Bump app to a new package version by kind (imperative)",
			Params: []docspec.Param{
				{Name: "name", Type: "string", Required: true, Description: "App identifier"},
				{Name: "version", Type: "string", Required: true, Description: "Target version"},
				{Name: "package-pattern", Type: "string", Description: "Optional package name glob (e.g. *santa*2026.3*)"},
			},
		},
	},
	ExamplesBuddy: []string{`def main():
    rows = apps(filter="santa")
    if not rows:
        return None
    detail = app(name=rows[0]["name"])
    # Prefer: buddy app import santa -o apps/santa.buddy
    # Then edit VERSION and run the script (app_reconcile).
    return detail`},
	ExamplesShell: []string{`buddy app list
buddy app get santa
buddy app import santa -o apps/santa.buddy
buddy apps/santa.buddy
buddy apps/santa.buddy -- apply`},
	Related: []docspec.RelatedLink{
		{Title: "Policies", Ref: "/docs/docs/policies"},
		{Title: "Packages / JCDS", Ref: "/docs/docs/packages"},
		{Title: "Mac App Store apps", Ref: "/docs/docs/macapps"},
		{Title: "Change batches", Ref: "/docs/docs/change-batches"},
		{Title: "API endpoints", Ref: "/docs/docs/api-endpoints"},
	},
	HelpExport: "apps",
	BuiltinsHelp: `POLICY APPS (Self Service + packages)
  apps(filter?, tenant?, surface?, auth?)
      → value: list of summary dicts {name, kind, version, ss_policy_id, install_trigger, …}
      Params: filter?: str (client-side substring on name/kind/version/trigger/package).
      Notes: Name-index from one hybrid policy list ([SS] in name); kind/version may be approximate.
             Prefer app(name=…) for authoritative classify.
  app(name, tenant?, surface?, auth?) → value: dict (summary + linked install/cache policies)
      Params: name: str (app identifier, e.g. santa, vscode).
      Notes: Classic XML classify (excludes inventory-only SS stubs).
  app_reconcile(desired, apply?=False, tenant?, surface?, auth?)
      → value: dict {markdown, fingerprints} (plan) or None (after apply)
      Params: desired: dict (name, version required; kind?, install_trigger?, cache_trigger?,
             package_pattern?, title?, summary?, risk?, window?); apply?: bool.
      Notes: Diffs live AppGet vs desired; invents change-batch steps (or noop note).
             Package must already be READY. No greenfield create in v1.
  app_upgrade(name, version, package_pattern?, tenant?, surface?, auth?) → None
      Params: name: str; version: str; package_pattern?: str (package name glob).
      Notes: One-shot imperative by kind. Prefer import + app_reconcile for CAB bumps.
`,
	CLIHelp: `buddy app - Manage policy apps (Self Service + packages)

Usage:
  buddy [global-flags] app <subcommand> [args...]

Subcommands:
  list [--filter=<keyword>]                              Discover policy apps from Self Service policies
  get <name>                                             Show one policy app (kind, linked policies, package)
  import <name> [--output=<path>|-o <path>]               Write desired-state .buddy from get
  upgrade <name> --version=<version> [--package-pattern=…]
                                                         One-shot bump by kind (prefer import + reconcile)

Flags for 'list':
  --filter=<keyword>   Client-side substring on name, kind, version, trigger, or package

Flags for 'import':
  -o, --output=<path>  Output .buddy path (default apps/<slug>.buddy)

Flags for 'upgrade':
  --version=<version>           Target version (required)
  --package-pattern=<glob>      Optional package name glob (e.g. *santa*2026.3*)

Notes:
  List is a fast name-index from one hybrid policy list (Pro /api/v1/policies when
  available, else Classic); kind/version/trigger may be approximate. List keeps
  Install…[SS] and/or rows with install/cache siblings; inventory [SS] stubs omitted.
  Get classifies via Classic XML (authoritative). Discovery starts from names containing [SS]:
    single  — Self Service policy has the Install package attached
    install — SS script calls install-{app} / install-{app}-next / {app}-install
    cache   — install pattern plus versioned cache-{app}-{version} policies
  Non-app Self Service (e.g. inventory-only) is excluded on get.
  Import emits a script that calls app_reconcile (plan by default; -- apply to write).
  Upgrade is imperative (same helpers as change batches). Prefer import for durable bumps.
  Classic Mac App Store records use 'buddy macapp', not this command.

Examples:
  buddy app list
  buddy app list --filter=santa
  buddy app get santa
  buddy app import santa -o apps/santa.buddy
  buddy apps/santa.buddy
  buddy apps/santa.buddy -- apply
  buddy app upgrade santa --version=2026.3 --package-pattern='*santa*2026.3*'
`,
}
