package buddy

import (
	"fmt"
	"os"
	"strings"
	"text/template"
)

// AppDesired is the declarative target for one policy app (import / reconcile).
// Policy IDs live in apps/index.json — reconcile resolves via AppGet (index first).
type AppDesired struct {
	Name             string `json:"name"`
	Kind             string `json:"kind"` // single | install | cache
	Version          string `json:"version"`
	InstallTrigger   string `json:"install_trigger,omitempty"`
	CacheTrigger     string `json:"cache_trigger,omitempty"`
	UninstallTrigger string `json:"uninstall_trigger,omitempty"`
	PackagePattern   string `json:"package_pattern,omitempty"`
	Title            string `json:"title,omitempty"`
	Summary          string `json:"summary,omitempty"`
	Risk             string `json:"risk,omitempty"`
	Window           string `json:"window,omitempty"`
}

// AppReconcileStep is one change-batch step invented by reconcile (pre-starlark).
type AppReconcileStep struct {
	Type           string // note | set_package | upgrade_app | set_ss
	Name           string
	Version        string
	FromTrigger    string
	PackagePattern string
	Note           string
	KeepSource     bool
	PolicyID       int
}

// AppReconcilePlan is the change header + steps to reach desired from live.
type AppReconcilePlan struct {
	Title   string
	Summary string
	Risk    string
	Window  string
	Steps   []AppReconcileStep
	Noop    bool
}

// AppDesiredFromDetail fills a desired dict from AppGet (import defaults).
func AppDesiredFromDetail(d *PolicyAppDetail) AppDesired {
	if d == nil {
		return AppDesired{}
	}
	ver := strings.TrimSpace(d.Version)
	des := AppDesired{
		Name:             d.Name,
		Kind:             string(d.Kind),
		Version:          ver,
		InstallTrigger:   d.InstallTrigger,
		CacheTrigger:     cacheTriggerFromDetail(d),
		UninstallTrigger: d.UninstallTrigger,
		PackagePattern:   DefaultAppPackagePattern(d.Name, ver),
		Risk:             "low",
	}
	des.Title = fmt.Sprintf("%s %s desired state", d.Name, ver)
	des.Summary = fmt.Sprintf("Reconcile policy app %q (%s) to version %s.", d.Name, d.Kind, ver)
	return des
}

// DefaultAppPackagePattern builds *<slug>*<version>* from name + version.
func DefaultAppPackagePattern(name, version string) string {
	slug := normalizeAppSlugKey(name)
	ver := strings.TrimSpace(version)
	if slug == "" {
		return ""
	}
	if ver == "" {
		return "*" + slug + "*"
	}
	return "*" + slug + "*" + ver + "*"
}

func cacheTriggerFromDetail(d *PolicyAppDetail) string {
	if d == nil {
		return ""
	}
	for _, c := range d.CachePolicies {
		if d.CachePolicyID > 0 && c.ID == d.CachePolicyID && strings.TrimSpace(c.Trigger) != "" {
			return strings.TrimSpace(c.Trigger)
		}
	}
	for _, c := range d.CachePolicies {
		if c.Enabled && strings.TrimSpace(c.Trigger) != "" {
			return strings.TrimSpace(c.Trigger)
		}
	}
	return ""
}

func cacheSourceFromLive(live *PolicyAppDetail, cacheTrigger string) (id int, trigger string) {
	trigger = strings.TrimSpace(cacheTrigger)
	if live == nil {
		return 0, trigger
	}
	want := normalizeTriggerForMatch(trigger)
	for _, c := range live.CachePolicies {
		if want != "" && normalizeTriggerForMatch(c.Trigger) == want {
			return c.ID, strings.TrimSpace(c.Trigger)
		}
	}
	if live.CachePolicyID > 0 {
		if trigger == "" {
			trigger = cacheTriggerFromDetail(live)
		}
		return live.CachePolicyID, trigger
	}
	return 0, trigger
}

func installSourceID(live *PolicyAppDetail, installTrigger string) int {
	if live == nil {
		return 0
	}
	want := normalizeTriggerForMatch(installTrigger)
	for _, p := range live.InstallPolicies {
		if want != "" && normalizeTriggerForMatch(p.Trigger) == want {
			return p.ID
		}
	}
	return live.InstallPolicyID
}

// PrintAppReconcileProgress writes a live-vs-desired briefing to stderr
// (which policies/packages will be looked up, and which APIs).
func PrintAppReconcileProgress(live *PolicyAppDetail, desired AppDesired, plan *AppReconcilePlan) {
	if live == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "app_reconcile: %s  live=%s (%s) → desired=%s\n",
		live.Name, live.Version, live.Kind, desired.Version)
	fmt.Fprintf(os.Stderr, "  kind: %s\n", live.Kind)
	switch live.Kind {
	case PolicyAppCache:
		fmt.Fprintf(os.Stderr, "  cache source: id=%d name=%q trigger=%q package=%q\n",
			live.CachePolicyID, live.CachePolicyName, cacheTriggerFromDetail(live), live.PackageName)
		if n := len(live.CachePolicies); n > 1 {
			fmt.Fprintf(os.Stderr, "  linked cache policies (%d):\n", n)
			for _, c := range live.CachePolicies {
				en := "off"
				if c.Enabled {
					en = "on"
				}
				mark := ""
				if c.ID == live.CachePolicyID {
					mark = "  <- clone from"
				}
				fmt.Fprintf(os.Stderr, "    id=%d %q trigger=%q version=%s %s%s\n",
					c.ID, c.Name, c.Trigger, c.Version, en, mark)
			}
		}
		fmt.Fprintf(os.Stderr, "  looking for: clone a new versioned cache policy from that source (keep prior cache enabled)\n")
		fmt.Fprintf(os.Stderr, "  install source: id=%d name=%q trigger=%q\n",
			live.InstallPolicyID, live.InstallPolicyName, live.InstallTrigger)
		fmt.Fprintf(os.Stderr, "  looking for: replace the install-next policy package (disable prior install)\n")
	case PolicyAppInstall:
		fmt.Fprintf(os.Stderr, "  install source: id=%d name=%q trigger=%q package=%q\n",
			live.InstallPolicyID, live.InstallPolicyName, live.InstallTrigger, live.PackageName)
		fmt.Fprintf(os.Stderr, "  looking for: replace the install policy package (disable prior)\n")
	case PolicyAppSingle:
		fmt.Fprintf(os.Stderr, "  Self Service policy: id=%d name=%q package=%q\n",
			live.SSPolicyID, live.SSPolicyName, live.PackageName)
		fmt.Fprintf(os.Stderr, "  looking for: in-place package swap on the SS policy\n")
	}
	pattern := strings.TrimSpace(desired.PackagePattern)
	if pattern == "" {
		pattern = DefaultAppPackagePattern(live.Name, desired.Version)
	}
	fmt.Fprintf(os.Stderr, "  package pattern: %s\n", pattern)
	fmt.Fprintf(os.Stderr, "  APIs: Classic GET /JSSResource/policies/id/{id} for source XML; GET /api/v1/packages for the new pkg\n")
	if plan != nil && !plan.Noop {
		fmt.Fprintf(os.Stderr, "  resolving %d change-batch step(s)…\n", len(plan.Steps))
	}
}

// BuildAppReconcilePlan diffs live AppGet vs desired and invents change-batch steps.
// Does not call Jamf beyond using the provided live detail. Fails if the app is
// missing fields needed for a bump, or if kind rewrite is requested.
func BuildAppReconcilePlan(live *PolicyAppDetail, desired AppDesired) (*AppReconcilePlan, error) {
	if live == nil {
		return nil, fmt.Errorf("app not found (v1 reconcile does not greenfield-create)")
	}
	desired.Name = strings.TrimSpace(desired.Name)
	desired.Version = strings.TrimSpace(desired.Version)
	desired.Kind = strings.TrimSpace(desired.Kind)
	if desired.Name == "" {
		return nil, fmt.Errorf("desired.name is required")
	}
	if desired.Version == "" {
		return nil, fmt.Errorf("desired.version is required")
	}
	if !appSlugsCompatible(desired.Name, live.Name) {
		return nil, fmt.Errorf("desired.name %q does not match live app %q", desired.Name, live.Name)
	}
	liveKind := string(live.Kind)
	if desired.Kind == "" {
		desired.Kind = liveKind
	}
	if !strings.EqualFold(desired.Kind, liveKind) {
		return nil, fmt.Errorf("kind rewrite not supported in v1 (desired %q, live %q)", desired.Kind, liveKind)
	}
	desired.Kind = liveKind

	pattern := strings.TrimSpace(desired.PackagePattern)
	if pattern == "" {
		pattern = DefaultAppPackagePattern(live.Name, desired.Version)
	}
	installTrigger := strings.TrimSpace(desired.InstallTrigger)
	if installTrigger == "" {
		installTrigger = live.InstallTrigger
	}
	cacheTrigger := strings.TrimSpace(desired.CacheTrigger)
	if cacheTrigger == "" {
		cacheTrigger = cacheTriggerFromDetail(live)
	}

	title := strings.TrimSpace(desired.Title)
	if title == "" {
		title = fmt.Sprintf("%s %s reconcile", live.Name, desired.Version)
	}
	summary := strings.TrimSpace(desired.Summary)
	if summary == "" {
		summary = fmt.Sprintf("Move policy app %q (%s) to version %s.", live.Name, live.Kind, desired.Version)
	}
	risk := strings.TrimSpace(desired.Risk)
	if risk == "" {
		risk = "low"
	}

	plan := &AppReconcilePlan{
		Title:   title,
		Summary: summary,
		Risk:    risk,
		Window:  strings.TrimSpace(desired.Window),
	}

	sameVersion := CompareVersions(live.Version, desired.Version) == 0
	if sameVersion && live.Version != "" {
		plan.Noop = true
		plan.Steps = []AppReconcileStep{{
			Type: "note",
			Note: fmt.Sprintf("Already at version %s (%s); no package policy changes.", desired.Version, live.Kind),
		}}
		return plan, nil
	}
	if sameVersion && live.Version == "" {
		// Live has no version — treat as needing bump toward desired.
	}

	switch PolicyAppKind(desired.Kind) {
	case PolicyAppSingle:
		plan.Steps = []AppReconcileStep{{
			Type:           "upgrade_app",
			Name:           live.Name,
			Version:        desired.Version,
			PackagePattern: pattern,
			Note:           "In-place Self Service package swap + description version bump.",
		}}
	case PolicyAppInstall:
		if installTrigger == "" {
			return nil, fmt.Errorf("app %q: missing install_trigger for install-kind bump", live.Name)
		}
		installID := installSourceID(live, installTrigger)
		plan.Steps = []AppReconcileStep{{
			Type:           "set_package",
			Name:           live.Name,
			Version:        desired.Version,
			FromTrigger:    installTrigger,
			PackagePattern: pattern,
			PolicyID:       installID,
			Note:           "Replace install policy; disable prior (same trigger).",
		}}
	case PolicyAppCache:
		if looksLikeInstallTrigger(cacheTrigger) && !looksLikeCacheTrigger(cacheTrigger) {
			return nil, fmt.Errorf("app %q: cache_trigger %q looks like an install trigger; set CACHE_TRIGGER to the versioned cache policy (e.g. cache-%s-%s), not the install-next policy",
				live.Name, cacheTrigger, strings.ToLower(normalizeAppSlugKey(live.Name)), live.Version)
		}
		if cacheTrigger == "" {
			return nil, fmt.Errorf("app %q: missing cache_trigger for cache-kind bump", live.Name)
		}
		cacheID, cacheTrigger := cacheSourceFromLive(live, cacheTrigger)
		if cacheID == 0 && live.CachePolicyID > 0 {
			cacheID = live.CachePolicyID
		}
		if installTrigger == "" {
			installTrigger = "install-" + strings.ToLower(normalizeAppSlugKey(live.Name)) + "-next"
		}
		installID := installSourceID(live, installTrigger)
		plan.Steps = []AppReconcileStep{
			{
				Type:           "set_package",
				Name:           live.Name,
				Version:        desired.Version,
				FromTrigger:    cacheTrigger,
				PackagePattern: pattern,
				KeepSource:     true,
				PolicyID:       cacheID,
				Note:           "Versioned cache; leave prior cache enabled for rollback.",
			},
			{
				Type:           "set_package",
				Name:           live.Name,
				Version:        desired.Version,
				FromTrigger:    installTrigger,
				PackagePattern: pattern,
				PolicyID:       installID,
				Note:           "Unversioned install-next replacement; disable prior install policy.",
			},
		}
		if live.SSPolicyID > 0 && ExtractVersion(live.SSDescription) != "" {
			plan.Steps = append(plan.Steps, AppReconcileStep{
				Type:     "set_ss",
				PolicyID: live.SSPolicyID,
				Version:  desired.Version,
				Note:     "Bump Self Service description version.",
			})
		} else {
			plan.Steps = append(plan.Steps, AppReconcileStep{
				Type: "note",
				Note: "Self Service already invokes the install trigger; no SS description version to bump.",
			})
		}
	default:
		return nil, fmt.Errorf("app %q has unknown kind %q", live.Name, desired.Kind)
	}
	return plan, nil
}

// RenderAppImportScript writes a runnable desired-state .buddy for one app.
func RenderAppImportScript(desired AppDesired) (string, error) {
	if strings.TrimSpace(desired.Name) == "" {
		return "", fmt.Errorf("desired.name is required")
	}
	if desired.PackagePattern == "" {
		desired.PackagePattern = DefaultAppPackagePattern(desired.Name, desired.Version)
	}
	if desired.Title == "" {
		desired.Title = fmt.Sprintf("%s %s desired state", desired.Name, desired.Version)
	}
	if desired.Summary == "" {
		desired.Summary = fmt.Sprintf("Reconcile policy app %q to version %s.", desired.Name, desired.Version)
	}
	if desired.Risk == "" {
		desired.Risk = "low"
	}
	var b strings.Builder
	if err := appImportTmpl.Execute(&b, desired); err != nil {
		return "", err
	}
	return b.String(), nil
}

var appImportTmpl = template.Must(template.New("app_import").Parse(`#!/usr/bin/env buddy
# Desired-state config for {{.Name}} (generated by buddy app import).
# Policy IDs live in apps/index.json; this script is the version bump surface.
# Edit VERSION / PACKAGE_PATTERN (and triggers if needed), then plan or apply.
#
# Usage:
#   buddy this-script.buddy              # plan only (default)
#   buddy this-script.buddy -- apply     # plan + apply (after approval)
#   buddy -y this-script.buddy -- apply  # apply without prompts
#
# Howto: docs/change-batches.md and docs/apps

def main():
    # ===================== DESIRED STATE (edit me) =====================
    NAME             = {{printf "%q" .Name}}
    KIND             = {{printf "%q" .Kind}}
    VERSION          = {{printf "%q" .Version}}
    INSTALL_TRIGGER  = {{printf "%q" .InstallTrigger}}
    CACHE_TRIGGER    = {{printf "%q" .CacheTrigger}}
    UNINSTALL_TRIGGER = {{printf "%q" .UninstallTrigger}}
    PACKAGE_PATTERN  = {{printf "%q" .PackagePattern}}

    TITLE   = {{printf "%q" .Title}}
    SUMMARY = {{printf "%q" .Summary}}
    RISK    = {{printf "%q" .Risk}}
    WINDOW  = {{printf "%q" .Window}}
    # ===================================================================

    desired = {
        "name": NAME,
        "kind": KIND,
        "version": VERSION,
        "install_trigger": INSTALL_TRIGGER,
        "cache_trigger": CACHE_TRIGGER,
        "uninstall_trigger": UNINSTALL_TRIGGER,
        "package_pattern": PACKAGE_PATTERN,
        "title": TITLE,
        "summary": SUMMARY,
        "risk": RISK,
        "window": WINDOW,
    }

    apply = False
    if len(args) > 0 and args[0] in ("apply", "run"):
        apply = True

    return app_reconcile(desired = desired, apply = apply)
`))
