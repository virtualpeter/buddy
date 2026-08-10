package buddy

import (
	"fmt"
	"strings"
	"text/template"
)

// AppDesired is the declarative target for one policy app (import / reconcile).
// IDs are not desired state — reconcile always re-resolves via AppGet + triggers.
type AppDesired struct {
	Name           string `json:"name"`
	Kind           string `json:"kind"` // single | install | cache
	Version        string `json:"version"`
	InstallTrigger string `json:"install_trigger,omitempty"`
	CacheTrigger   string `json:"cache_trigger,omitempty"`
	PackagePattern string `json:"package_pattern,omitempty"`
	Title          string `json:"title,omitempty"`
	Summary        string `json:"summary,omitempty"`
	Risk           string `json:"risk,omitempty"`
	Window         string `json:"window,omitempty"`
}

// AppReconcileStep is one change-batch step invented by reconcile (pre-starlark).
type AppReconcileStep struct {
	Type            string // note | set_package | upgrade_app | set_ss
	Name            string
	Version         string
	FromTrigger     string
	PackagePattern  string
	Note            string
	KeepSource      bool
	PolicyID        int
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
		Name:           d.Name,
		Kind:           string(d.Kind),
		Version:        ver,
		InstallTrigger: d.InstallTrigger,
		CacheTrigger:   cacheTriggerFromDetail(d),
		PackagePattern: DefaultAppPackagePattern(d.Name, ver),
		Risk:           "low",
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
		plan.Steps = []AppReconcileStep{{
			Type:           "set_package",
			Name:           live.Name,
			Version:        desired.Version,
			FromTrigger:    installTrigger,
			PackagePattern: pattern,
			Note:           "Replace install policy; disable prior (same trigger).",
		}}
	case PolicyAppCache:
		if cacheTrigger == "" {
			return nil, fmt.Errorf("app %q: missing cache_trigger for cache-kind bump", live.Name)
		}
		if installTrigger == "" {
			installTrigger = "install-" + strings.ToLower(normalizeAppSlugKey(live.Name)) + "-next"
		}
		plan.Steps = []AppReconcileStep{
			{
				Type:           "set_package",
				Name:           live.Name,
				Version:        desired.Version,
				FromTrigger:    cacheTrigger,
				PackagePattern: pattern,
				KeepSource:     true,
				Note:           "Versioned cache; leave prior cache enabled for rollback.",
			},
			{
				Type:           "set_package",
				Name:           live.Name,
				Version:        desired.Version,
				FromTrigger:    installTrigger,
				PackagePattern: pattern,
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
