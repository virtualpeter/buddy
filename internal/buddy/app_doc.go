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
		"app upgrade", "app import", "app scan", "app cache", "app_reconcile", "desired state",
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
		{Action: "Scan", BuddyScript: "", CLI: "`buddy app scan`", Notes: "Heuristic onboarding discovery ([SS] name index)"},
		{Action: "Get", BuddyScript: "`app(name=…)`", CLI: "`buddy app get`", Notes: "Index bindings first; else Classic classify"},
		{Action: "Import", BuddyScript: "", CLI: "`buddy app import [name]`", Notes: "Interview (or `--ss=` flags) → index + desired `.buddy`"},
		{Action: "Cache", BuddyScript: "", CLI: "`buddy app cache status|refresh`", Notes: "Full Classic policy XML under `~/.config/buddy/policy-cache/`"},
		{Action: "Reconcile", BuddyScript: "`app_reconcile(desired, apply?)`", CLI: "run the imported script", Notes: "Diff live vs desired → change-batch plan / apply"},
		{Action: "Upgrade", BuddyScript: "`app_upgrade(name, version, …)`", CLI: "`buddy app upgrade`", Notes: "One-shot imperative bump by kind"},
	},
	Notes: []string{
		"**Onboarding:** `buddy app cache refresh` → `buddy app scan` → `buddy app import <name>` (picks SS/install/cache/uninstall IDs). Import writes `apps/index.json` + `apps/<slug>.buddy`.",
		"**List** is the curated registry only. **Scan** is approximate heuristics for finding candidates — not for day-to-day inventory.",
		"Policy payload cache (age > 3 days prompts refresh) is how import knows true Self Service (`use_for_self_service`), not `[SS]` name markers alone.",
		"Uninstall policies are recorded in the index/interview set; reconcile v1 does not auto-mutate uninstall on version bumps.",
		"Desired-state scripts declare name/kind/version/triggers/package_pattern. Edit `VERSION` then re-run to plan a bump. v1 does not greenfield-create a missing app — use `setup_app_classic` for that.",
		"Wait until the target package is READY (`package_wait_ready` / `package_ready`) before apply/upgrade.",
	},
	AgentNotes: []string{
		"`apps()` / `buddy app list` = imported registry. Use `buddy app scan` only when onboarding.",
		"Prefer `buddy app import` (interview or `--ss=`/`--install=`/…) → edit desired → `app_reconcile`. Ensure policy cache is fresh.",
		"Do not invent Classic PUTs for app upgrades — use `app_reconcile` / `app_upgrade` (or policy package / Self Service helpers).",
		"Wait until the target package is READY in Jamf (`package_wait_ready` / `package_ready`) before reconcile apply or `app_upgrade`.",
		"Prefer `docs/apps` then named builtins via `buddyscript`; Classic Mac App Store is `macapp` / `macapps`, not `app` / `apps`.",
		"Platform-only auth cannot list/scan apps (Pro `/api/v1/policies` via gateway returns 403); use an instance-surface auth.",
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
				"Heuristic discovery is CLI `buddy app scan`, not this builtin.",
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
				{Name: "desired", Type: "dict", Required: true, Description: "name, version, kind?, install_trigger?, cache_trigger?, uninstall_trigger?, package_pattern?, title?, summary?, risk?, window?"},
				{Name: "apply", Type: "boolean", Description: "When True, run_change after plan (default False)"},
				{Name: "tenant", Type: "string", Description: "Session override"},
				{Name: "surface", Type: "string", Description: "Session override"},
				{Name: "auth", Type: "string", Description: "Session override"},
			},
			Notes: []string{
				"Invented steps mirror install/cache change batches; single uses in-place upgrade_app.",
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
			Notes: []string{"Empty until apps are imported."},
		},
		{
			Name:    "app scan",
			Usage:   "buddy [global-flags] app scan [--filter=<keyword>]",
			Summary: "Heuristic discovery of [SS] policy apps (onboarding aid)",
			Params: []docspec.Param{
				{Name: "filter", Type: "string", Description: "Client-side substring on name, kind, version, trigger, or package"},
			},
			Notes: []string{"Approximate name-index; use import to curate real bindings."},
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
			Usage:   "buddy [global-flags] app import [name] [--ss=<id>] [--install=…] [--cache=…] [--uninstall=…] [-o path]",
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
				"Ensures policy cache (prompts refresh if missing/stale > 3 days).",
				"Writes apps/index.json and apps/<slug>.buddy.",
			},
		},
		{
			Name:    "app cache",
			Usage:   "buddy [global-flags] app cache <status|refresh>",
			Summary: "Show or rebuild the Classic policy payload cache",
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
    # Prefer: buddy app import santa
    # Then edit VERSION and run the script (app_reconcile).
    return detail`},
	ExamplesShell: []string{`buddy app cache refresh
buddy app scan --filter=santa
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
      → value: list of summary dicts {name, kind, version, ss_policy_id, install_trigger, …}
      Params: filter?: str (client-side substring on name/kind/version/trigger/package).
      Notes: Imported apps only (apps/index.json). Empty until buddy app import.
             Heuristic discovery is CLI: buddy app scan.
  app(name, tenant?, surface?, auth?) → value: dict (summary + linked install/cache/uninstall policies)
      Params: name: str (app identifier, e.g. santa, vscode).
      Notes: Uses apps/index.json when present; else Classic XML classify.
  app_reconcile(desired, apply?=False, tenant?, surface?, auth?)
      → value: dict {markdown, fingerprints} (plan) or None (after apply)
      Params: desired: dict (name, version required; kind?, install_trigger?, cache_trigger?,
             uninstall_trigger?, package_pattern?, title?, summary?, risk?, window?); apply?: bool.
      Notes: Diffs live AppGet vs desired; invents change-batch steps (or noop note).
             Package must already be READY. No greenfield create in v1. Uninstall not mutated in v1.
  app_upgrade(name, version, package_pattern?, tenant?, surface?, auth?) → None
      Params: name: str; version: str; package_pattern?: str (package name glob).
      Notes: One-shot imperative by kind. Prefer import + app_reconcile for CAB bumps.
`,
	CLIHelp: `buddy app - Manage policy apps (Self Service + packages)

Usage:
  buddy [global-flags] app <subcommand> [args...]

Subcommands:
  list [--filter=<keyword>]                              List imported apps (apps/index.json)
  scan [--filter=<keyword>]                              Heuristic [SS] discovery (onboarding)
  get <name>                                             Show one policy app (index or classify)
  import [name] [--ss=<id>] [--install=…] [options]      Interview or flags → index + .buddy
  cache <status|refresh>                                 Classic policy payload cache
  upgrade <name> --version=<version> [--package-pattern=…]
                                                         One-shot bump by kind (prefer import + reconcile)

Flags for 'list' / 'scan':
  --filter=<keyword>   Client-side substring on name, kind, version, trigger, or package

Flags for 'import':
  -o, --output=<path>       Output .buddy path (default apps/<slug>.buddy)
  --ss=<id>                 Self Service policy ID (non-interactive when set)
  --install=<ids>           Comma-separated install policy IDs
  --cache=<ids>             Comma-separated cache policy IDs
  --uninstall=<ids>         Comma-separated uninstall policy IDs
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
