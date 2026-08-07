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
		"app upgrade", "install-next", "cache policy",
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
	Concepts: []string{
		"Discovery starts from policies whose **name contains `[SS]`**.",
		"`buddy app list` / `apps()` — one hybrid policy list call (undocumented Pro `GET /api/v1/policies` when available, else Classic `{id,name}`), then **name-only** heuristics for kind/version/trigger. Rows need an `Install…[SS]` name and/or install/cache sibling policies for the app slug (inventory/config `[SS]` stubs are omitted). App slugs treat hyphens and spaces as equivalent (`claude-cli` ↔ `Claude CLI`). When v1 `triggerManual` is present, install siblings with a custom trigger are preferred (Self Service / single rows usually have none). Approximate; `app get` is authoritative.",
		"`buddy app get` / `app()` — authoritative Classic XML classify (Self Service flag, scripts, packages, triggers). Excludes non-apps (no package and no install-like script / inferred install trigger).",
		"Jamf does **not** publish a Pro policy CRUD API; RSQL on `/api/v1/policies` cannot filter by Self Service (only `id`, `name`, `enabled`, `categoryId`, `executionFrequency`, `triggerCheckin`, `triggerManual`).",
		"Greenfield setup (prefs + PPPC + cache + install + optional SS) remains `setup_app_classic` in `stdlib/app_setup.buddy` / change batches — not `app create`.",
	},
	AgentNotes: []string{
		"List is approximate; call `app(name=…)` / `buddy app get` before mutate or upgrade.",
		"Do not invent Classic PUTs for app upgrades — use `app_upgrade` (or policy package / Self Service helpers).",
		"Wait until the target package is READY in Jamf (`package_wait_ready` / `package_ready`) before `app_upgrade`.",
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
				"Dispatches by kind — single in-place package+SS description; install PolicyPackage with disable-source; cache keep prior cache + replace install-next.",
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
			Name:    "app upgrade",
			Usage:   "buddy [global-flags] app upgrade <name> --version=<version> [--package-pattern=…]",
			Summary: "Bump app to a new package version by kind",
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
    # After the package is READY in Jamf:
    # app_upgrade(name="santa", version="2026.3", package_pattern="*santa*2026.3*")
    return detail`},
	ExamplesShell: []string{`buddy app list
buddy app get santa
buddy app upgrade santa --version=2026.3 --package-pattern='*santa*2026.3*'`},
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
  app_upgrade(name, version, package_pattern?, tenant?, surface?, auth?) → None
      Params: name: str; version: str; package_pattern?: str (package name glob).
      Notes: Dispatches by kind — single in-place package+SS description; install PolicyPackage
             with disable-source; cache keep prior cache + replace install-next.
`,
	CLIHelp: `buddy app - Manage policy apps (Self Service + packages)

Usage:
  buddy [global-flags] app <subcommand> [args...]

Subcommands:
  list [--filter=<keyword>]                              Discover policy apps from Self Service policies
  get <name>                                             Show one policy app (kind, linked policies, package)
  upgrade <name> --version=<version> [--package-pattern=…]
                                                         Bump app to a new package version by kind

Flags for 'list':
  --filter=<keyword>   Client-side substring on name, kind, version, trigger, or package

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
  Upgrade composes existing policy package / Self Service helpers (same as change batches).
  Classic Mac App Store records use 'buddy macapp', not this command.

Examples:
  buddy app list
  buddy app list --filter=santa
  buddy app get santa
  buddy app upgrade santa --version=2026.3 --package-pattern='*santa*2026.3*'
`,
}
