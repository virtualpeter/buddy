package buddy

import "buddy/internal/docspec"

//go:generate go run ../../cmd/gendocs

func init() {
	docspec.Register(AppsTopic)
}

// AppsTopic is the structured doc source for policy apps (buddy app / apps / app / app_scan / app_import / app_cache / app_upgrade).
// Edit here, then run `make gendocs` (not docs/apps.md).
var AppsTopic = docspec.Topic{
	ID:      "docs/apps",
	Title:   "Policy apps",
	Section: "Resources",
	Keywords: []string{
		"app", "apps", "policy app", "policy apps", "self service",
		"app upgrade", "app import", "app scan", "app cache", "app_scan", "app_import",
		"app_cache", "app_reconcile", "desired state",
		"install-next", "cache policy", "uninstall",
	},
	Summary: "**Policy apps** are not a Jamf object type. They are a curated convention: Self Service + install/cache/uninstall policies, tracked in `apps/index.json` after an interactive import.\n\n" +
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
		{Action: "List", BuddyScript: "`apps(filter?)`", CLI: "`buddy app list`", Notes: "Imported apps only (`apps/index.json`)"},
		{Action: "Scan", BuddyScript: "`app_scan(filter?, refresh?)`", CLI: "`buddy app scan [--refresh]`", Notes: "Heuristic onboarding; uses policy cache when present"},
		{Action: "Get", BuddyScript: "`app(name=…)`", CLI: "`buddy app get`", Notes: "Index bindings first; else Classic classify"},
		{Action: "Import", BuddyScript: "`app_import(name, ss_policy_id, …)`", CLI: "`buddy app import [name]`", Notes: "CLI interview or flags; builtin is flags-only (requires ss_policy_id)"},
		{Action: "Cache", BuddyScript: "`app_cache()` / `app_cache_refresh()`", CLI: "`buddy app cache status|refresh`", Notes: "Full Classic policy XML under `~/.config/buddy/policy-cache/`"},
		{Action: "Reconcile", BuddyScript: "`app_reconcile(desired, apply?)`", CLI: "run the imported script", Notes: "Diff live vs desired → change-batch plan / apply"},
		{Action: "Upgrade", BuddyScript: "`app_upgrade(name, version, …)`", CLI: "`buddy app upgrade`", Notes: "One-shot imperative bump by kind"},
	},
	Notes: []string{
		"**Onboarding:** `app_cache_refresh()` / `buddy app cache refresh` → `app_scan()` / `buddy app scan` → `app_import(name, ss_policy_id, …)` or `buddy app import`. Import writes `apps/index.json` + `apps/<slug>.buddy`.",
		"**List** is the curated registry only. **Scan** is approximate heuristics for finding candidates — not for day-to-day inventory. Scan uses the policy cache when present (prints fetched date); `--refresh` rebuilds it first.",
		"Policy payload cache (age > 3 days prompts refresh) is how import knows true Self Service (`use_for_self_service`), not `[SS]` name markers alone.",
		"Uninstall policies are recorded in the index/interview set; reconcile v1 does not auto-mutate uninstall on version bumps.",
		"Desired-state scripts declare name/kind/version/triggers/package_pattern. Edit `VERSION` then re-run to plan a bump. v1 does not greenfield-create a missing app — see `examples/setup-app-classic.buddy` for that.",
		"Wait until the target package is READY (`package_wait_ready` / `package_ready`) before apply/upgrade.",
	},
	AgentNotes: []string{
		"`apps()` / `buddy app list` = imported registry. Use `app_scan(filter?, refresh?)` only when onboarding (policy cache if present; `refresh=True` to rebuild).",
		"Prefer `app_import(name, ss_policy_id, …)` (or CLI interview) → edit desired → `app_reconcile`. Check `app_cache()` fetched_at; `app_cache_refresh()` if stale.",
		"Do not invent Classic PUTs for app upgrades — use `app_reconcile` / `app_upgrade` (or policy package / Self Service helpers).",
		"Wait until the target package is READY in Jamf (`package_wait_ready` / `package_ready`) before reconcile apply or `app_upgrade`.",
		"Prefer `docs/apps` then named builtins via `buddyscript`; Classic Mac App Store is `macapp` / `macapps`, not `app` / `apps`.",
		"On an environment workplace, undocumented `GET /api/v1/policies` (scan index, ACS, schema) uses a Jamf Pro product-tenant child with instance auth when one exists. Classic XML cache stays on the environment when `/proclassic/` can run. No child → Classic-on-gateway.",
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
				"Reads `apps/index.json` (imported apps only). Empty when nothing imported.",
				"Heuristic discovery is `app_scan()`, not this builtin.",
			},
		},
		{
			Name:    "app",
			Summary: "`app(name=…)` or `buddy app get <name>`",
			Returns: "dict (summary + linked install/cache/uninstall policies)",
			Params: []docspec.Param{
				{Name: "name", Type: "string", Required: true, Description: "App identifier (e.g. santa, vscode)"},
				{Name: "tenant", Type: "string", Description: "Session override"},
				{Name: "surface", Type: "string", Description: "Session override"},
				{Name: "auth", Type: "string", Description: "Session override"},
			},
			Notes: []string{"Uses apps/index.json bindings when present; else Classic XML classify."},
		},
		{
			Name:    "app_scan",
			Summary: "`app_scan(filter?, refresh?)` or `buddy app scan [--filter=…] [--refresh]`",
			Returns: "dict {apps: list of summaries, source: cache|live, cache: {exists, stale, fetched_at, …}, context?}",
			Params: []docspec.Param{
				{Name: "filter", Type: "string", Description: "Client-side substring on name/kind/version/trigger/package"},
				{Name: "refresh", Type: "boolean", Description: "Rebuild the Classic policy cache, then scan it"},
				{Name: "tenant", Type: "string", Description: "Session override"},
				{Name: "surface", Type: "string", Description: "Session override"},
				{Name: "auth", Type: "string", Description: "Session override"},
			},
			Notes: []string{
				"Onboarding aid — approximate. Prefer `apps()` after import.",
				"Uses ~/.config/buddy/policy-cache/ when present. Missing cache falls back to a live policy list.",
				"Environment workplace: v1 policy index uses a Jamf Pro product-tenant child when one exists (same as `buddy policy list`). Otherwise Classic-on-gateway.",
			},
		},
		{
			Name:    "app_import",
			Summary: "`app_import(name, ss_policy_id, …)` or `buddy app import [name] --ss=…`",
			Returns: "dict {name, kind, version, ss_policy_id, script}",
			Params: []docspec.Param{
				{Name: "name", Type: "string", Required: true, Description: "App slug"},
				{Name: "ss_policy_id", Type: "int", Required: true, Description: "Self Service policy ID (no TTY interview)"},
				{Name: "install_policy_ids", Type: "list of int", Description: "Install policy IDs"},
				{Name: "cache_policy_ids", Type: "list of int", Description: "Cache policy IDs"},
				{Name: "uninstall_policy_ids", Type: "list of int", Description: "Uninstall policy IDs"},
				{Name: "package_pattern", Type: "string", Description: "Package name glob"},
				{Name: "install_trigger", Type: "string", Description: "Install trigger"},
				{Name: "cache_trigger", Type: "string", Description: "Cache trigger"},
				{Name: "uninstall_trigger", Type: "string", Description: "Uninstall trigger"},
				{Name: "kind", Type: "string", Description: "single|install|cache (inferred when omitted)"},
				{Name: "version", Type: "string", Description: "Current version"},
				{Name: "output", Type: "string", Description: "Output .buddy path (default apps/<slug>.buddy)"},
				{Name: "tenant", Type: "string", Description: "Session override"},
				{Name: "surface", Type: "string", Description: "Session override"},
				{Name: "auth", Type: "string", Description: "Session override"},
			},
			Notes: []string{
				"Non-interactive only (requires ss_policy_id). CLI interview stays on `buddy app import`.",
				"Writes apps/index.json + desired-state script. Auto-refreshes a missing/stale policy cache (SkipConfirm).",
			},
		},
		{
			Name:    "app_cache",
			Summary: "`app_cache()` or `buddy app cache status`",
			Returns: "dict {exists, stale, fetched_at, age_seconds, count, tenant, dir}",
			Params: []docspec.Param{
				{Name: "tenant", Type: "string", Description: "Session override"},
				{Name: "surface", Type: "string", Description: "Session override"},
				{Name: "auth", Type: "string", Description: "Session override"},
			},
			Notes: []string{"No Jamf call. fetched_at is RFC3339 UTC when the cache exists.", "Environment workplace: cache dir follows Classic-on-gateway when a platform auth can serve `/proclassic/`."},
		},
		{
			Name:    "app_cache_refresh",
			Summary: "`app_cache_refresh()` or `buddy app cache refresh`",
			Returns: "dict {exists, stale, fetched_at, age_seconds, count, tenant, dir} after rebuild",
			Params: []docspec.Param{
				{Name: "tenant", Type: "string", Description: "Session override"},
				{Name: "surface", Type: "string", Description: "Session override"},
				{Name: "auth", Type: "string", Description: "Session override"},
			},
			Notes: []string{"GETs every Classic policy XML into ~/.config/buddy/policy-cache/. Local files only (not a Jamf mutate).", "Environment workplace: Classic XML stays on the environment when `/proclassic/` can run; the v1 index may use a Jamf Pro child."},
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
			Summary: "`app_reconcile(desired, apply?=False, progress?=True)` — diff live AppGet vs desired dict; plan_change / optional run_change",
			Returns: "dict {markdown, fingerprints} on plan; None after apply",
			Params: []docspec.Param{
				{Name: "desired", Type: "dict", Required: true, Description: "name, version, kind?, install_trigger?, cache_trigger?, uninstall_trigger?, package_pattern?, title?, summary?, risk?, window?"},
				{Name: "apply", Type: "boolean", Description: "When True, run_change after plan (default False)"},
				{Name: "progress", Type: "boolean", Description: "Print lookup progress to stderr (default True): which cache/install policy, package pattern, and APIs"},
				{Name: "tenant", Type: "string", Description: "Session override"},
				{Name: "surface", Type: "string", Description: "Session override"},
				{Name: "auth", Type: "string", Description: "Session override"},
			},
			Notes: []string{
				"Invented steps mirror install/cache change batches; single uses in-place upgrade_app.",
				"Cache-kind pins the current cache policy ID (not the install policy) as the clone source; install-next is a separate step.",
				"Fails if the app is missing (no greenfield create in v1). Uninstall is not mutated in v1.",
			},
		},
	},
	CLI: []docspec.CLICommand{
		{
			Name:    "app list",
			Usage:   "buddy [global-flags] app list [--filter=<keyword>]",
			Summary: "List imported policy apps from apps/index.json",
			Params: []docspec.Param{
				{Name: "filter", Type: "string", Description: "Client-side substring on name, kind, version, trigger, or package"},
			},
			Notes: []string{
				"Empty until apps are imported.",
				"Prints the policy cache fetched date (fresh/stale/missing) before the table.",
			},
		},
		{
			Name:    "app scan",
			Usage:   "buddy [global-flags] app scan [--filter=<keyword>] [--refresh]",
			Summary: "Heuristic discovery of policy apps (onboarding aid)",
			Params: []docspec.Param{
				{Name: "filter", Type: "string", Description: "Client-side substring on name, kind, version, trigger, or package"},
				{Name: "refresh", Type: "boolean", Description: "Rebuild the Classic policy cache, then scan it"},
			},
			Notes: []string{
				"Uses ~/.config/buddy/policy-cache/ when present (prints fetched date). No Jamf list call.",
				"Missing cache falls back to a live policy list. --refresh rebuilds the cache first (same as app cache refresh).",
				"On an environment workplace, the v1 policy index uses a Jamf Pro product-tenant child with instance auth when one exists (same as `buddy policy list`).",
				"Approximate; use import to curate real bindings.",
			},
		},
		{
			Name:    "app get",
			Usage:   "buddy [global-flags] app get <name>",
			Summary: "Show one policy app (kind, linked policies, package)",
			Params: []docspec.Param{
				{Name: "name", Type: "string", Required: true, Description: "App identifier"},
			},
			Notes: []string{"Prints the policy cache fetched date before the JSON."},
		},
		{
			Name:    "app import",
			Usage:   "buddy [global-flags] app import [name] [--ss=<policy-id>] [--install=…] [--cache=…] [--uninstall=…] [-o path]",
			Summary: "Interview (or flags) to bind policies and write index + desired-state script",
			Params: []docspec.Param{
				{Name: "name", Type: "string", Description: "App slug (prompted when omitted)"},
				{Name: "ss", Type: "int", Description: "Self Service policy ID (skips interview when set)"},
				{Name: "install", Type: "string", Description: "Comma-separated install policy IDs"},
				{Name: "cache", Type: "string", Description: "Comma-separated cache policy IDs"},
				{Name: "uninstall", Type: "string", Description: "Comma-separated uninstall policy IDs"},
				{Name: "output", Type: "string", Description: "Output path (default apps/<slug>.buddy)"},
			},
			Notes: []string{
				"Prints the policy cache fetched date; prompts refresh if missing/stale > 3 days.",
				"Writes apps/index.json and apps/<slug>.buddy.",
			},
		},
		{
			Name:    "app cache",
			Usage:   "buddy [global-flags] app cache <status|refresh>",
			Summary: "Show or rebuild the Classic policy payload cache",
			Notes: []string{
				"status prints fetched date, age, tenant, and dir.",
				"refresh prints the current fetched date, then rebuilds.",
				"On an environment workplace, Classic XML stays on the environment when `/proclassic/` can run; the v1 index may use a Jamf Pro child.",
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
			Notes: []string{"Prints the policy cache fetched date before applying."},
		},
	},
	ExamplesBuddy: []string{`def main():
    st = app_cache()
    scan = app_scan(filter="santa")
    rows = apps(filter="santa")
    if not rows:
        return {"cache": st, "scan": scan}
    detail = app(name=rows[0]["name"])
    return detail`},
	ExamplesShell: []string{`buddy app cache refresh
buddy app scan --filter=santa
buddy app scan --refresh
buddy app import santa
buddy app list
buddy app get santa
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
      Does: List imported policy apps from apps/index.json.
      → value: list of summary dicts {name, kind, version, ss_policy_id, install_trigger, …}
      Params: filter?: str (client-side substring on name/kind/version/trigger/package).
      Notes: Imported apps only (apps/index.json). Empty until app_import / buddy app import.
             Heuristic discovery is app_scan(), not this builtin.
  app(name, tenant?, surface?, auth?)
      Does: Load one policy app (index bindings, else Classic XML classify).
      → value: dict (summary + linked install/cache/uninstall policies)
      Params: name: str (app identifier, e.g. santa, vscode).
      Notes: Uses apps/index.json when present; else Classic XML classify.
  app_scan(filter?, refresh?, tenant?, surface?, auth?)
      Does: Heuristic discovery of Self Service + install/cache policies (onboarding).
      → value: dict {apps: list of summaries, source: "cache"|"live", cache: status dict}
      Params: filter?: str; refresh?: bool (rebuild policy cache first).
      Notes: Onboarding heuristic. Prefer apps() after import.
  app_import(name, ss_policy_id, install_policy_ids?, cache_policy_ids?, uninstall_policy_ids?,
             package_pattern?, install_trigger?, cache_trigger?, uninstall_trigger?, kind?,
             version?, output?, tenant?, surface?, auth?)
      Does: Bind policies into apps/index.json and write a desired-state .buddy script.
      → value: dict {name, kind, version, ss_policy_id, script}
      Params: name: str; ss_policy_id: int (required; no TTY interview).
             install_policy_ids?/cache_policy_ids?/uninstall_policy_ids?: list of int.
             package_pattern?/install_trigger?/cache_trigger?/uninstall_trigger?/kind?/version?: str.
             output?: str (default apps/<slug>.buddy).
      Notes: Writes apps/index.json + desired .buddy. Missing/stale cache auto-refreshes.
  app_cache(tenant?, surface?, auth?)
      Does: Return Classic policy payload cache status (no Jamf call).
      → value: dict {exists, stale, fetched_at, age_seconds, count, tenant, dir}
      Params: session overrides only.
      Notes: fetched_at is RFC3339 UTC when the cache exists.
  app_cache_refresh(tenant?, surface?, auth?)
      Does: Rebuild the Classic policy XML cache under ~/.config/buddy/policy-cache/.
      → value: dict (same as app_cache, after rebuild)
      Params: session overrides only.
      Notes: GETs every Classic policy XML. Local files only (not a Jamf mutate).
  app_reconcile(desired, apply?=False, progress?=True, tenant?, surface?, auth?)
      Does: Diff live app state vs a desired dict and plan (or apply) a change batch.
      → value: dict {markdown, fingerprints} (plan) or None (after apply)
      Params: desired: dict (name, version required; kind?, install_trigger?, cache_trigger?,
             uninstall_trigger?, package_pattern?, title?, summary?, risk?, window?); apply?: bool;
             progress?: bool (default True — stderr lookup progress: cache/install source, APIs).
      Notes: Invents change-batch steps (or a noop note). Cache-kind clones from the pinned
             cache policy ID (not install-next). Package must already be READY. No greenfield
             create in v1. Uninstall not mutated in v1.
  app_upgrade(name, version, package_pattern?, tenant?, surface?, auth?)
      Does: One-shot bump of an app to a new package version by kind.
      → None
      Params: name: str; version: str; package_pattern?: str (package name glob).
      Notes: Prefer import + app_reconcile for CAB bumps.
`,
	CLIHelp: `buddy app - Manage policy apps (Self Service + packages)

Usage:
  buddy [global-flags] app <subcommand> [args...]

Subcommands:
  list [--filter=<keyword>]                              List imported apps (apps/index.json)
  scan [--filter=<keyword>] [--refresh]                  Heuristic discovery (policy cache if present)
  get <name>                                             Show one policy app (index or classify)
  import [name] [--ss=<policy-id>] [--install=…] [options]   Interview or flags → index + .buddy
  cache <status|refresh>                                 Classic policy payload cache
  upgrade <name> --version=<version> [--package-pattern=…]
                                                         One-shot bump by kind (prefer import + reconcile)

Flags for 'list':
  --filter=<keyword>   Client-side substring on name, kind, version, trigger, or package

Flags for 'scan':
  --filter=<keyword>   Client-side substring on name, kind, version, trigger, or package
  --refresh            Rebuild the policy cache, then scan it

Flags for 'import':
  -o, --output=<path>       Output .buddy path (default apps/<slug>.buddy)
  --ss=<policy-id>          Self Service policy ID (non-interactive when set)
  --install=<policy-id>[,<policy-id>...]    Comma-separated install policy IDs
  --cache=<policy-id>[,<policy-id>...]      Comma-separated cache policy IDs
  --uninstall=<policy-id>[,<policy-id>...]  Comma-separated uninstall policy IDs
  --package-pattern=<glob>  Package name glob
  --install-trigger=…       Install trigger
  --cache-trigger=…         Cache trigger
  --uninstall-trigger=…     Uninstall trigger
  --kind=single|install|cache
  --version=<version>       Current version

Flags for 'upgrade':
  --version=<version>           Target version (required)
  --package-pattern=<glob>      Optional package name glob (e.g. *santa*2026.3*)

Notes:
  Onboarding: buddy app cache refresh → scan → import. Import writes apps/index.json
  and apps/<slug>.buddy. List shows imported apps only; scan is heuristic only.
  App commands print the policy cache fetched date (fresh/stale/missing). cache status
  also prints tenant and dir. Scan uses the cache when present; --refresh rebuilds it
  first. Missing cache falls back to a live policy list.
  Policy cache (~/.config/buddy/policy-cache/) stores Classic XML; import prompts to
  refresh when missing or older than 3 days. True Self Service uses use_for_self_service.
  Uninstall policies are part of the imported set; reconcile v1 does not mutate them.
  Classic Mac App Store records use 'buddy macapp', not this command.

Examples:
  buddy app cache refresh
  buddy app scan --filter=santa
  buddy app import santa
  buddy app list
  buddy app get santa
  buddy apps/santa.buddy
  buddy apps/santa.buddy -- apply
  buddy app upgrade santa --version=2026.3 --package-pattern='*santa*2026.3*'
`,
}
