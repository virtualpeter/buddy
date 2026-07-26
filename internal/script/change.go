package script

import (
	"fmt"
	"log/slog"
	"strings"

	"buddy/internal/buddy"

	"go.starlark.net/starlark"
)

// ChangeMeta is the header for a change-batch document.
type ChangeMeta struct {
	Title   string
	Summary string
	Risk    string
	Window  string
}

// ChangeOutcome is one step's post-apply result for the change record.
type ChangeOutcome struct {
	Index   int
	Lines   []string
	Outcome *buddy.PolicyMutationOutcome
}

func changeMetaFromStarlark(v starlark.Value) (ChangeMeta, error) {
	d, ok := v.(*starlark.Dict)
	if !ok {
		return ChangeMeta{}, fmt.Errorf("change must be a dict from change_batch(...)")
	}
	get := func(key string) (string, error) {
		val, found, err := d.Get(starlark.String(key))
		if err != nil {
			return "", err
		}
		if !found {
			return "", nil
		}
		s, ok := val.(starlark.String)
		if !ok {
			return "", fmt.Errorf("change field %q must be a string", key)
		}
		return string(s), nil
	}
	title, err := get("title")
	if err != nil {
		return ChangeMeta{}, err
	}
	if strings.TrimSpace(title) == "" {
		return ChangeMeta{}, fmt.Errorf("change_batch requires title")
	}
	summary, err := get("summary")
	if err != nil {
		return ChangeMeta{}, err
	}
	risk, err := get("risk")
	if err != nil {
		return ChangeMeta{}, err
	}
	window, err := get("window")
	if err != nil {
		return ChangeMeta{}, err
	}
	typ, _ := get("type")
	if typ != "" && typ != "change_batch" {
		return ChangeMeta{}, fmt.Errorf("expected change_batch dict, got type %q", typ)
	}
	return ChangeMeta{Title: title, Summary: summary, Risk: risk, Window: window}, nil
}

func parseChangeSteps(stepVals []starlark.Value) ([]*Step, error) {
	steps := make([]*Step, 0, len(stepVals))
	for i, v := range stepVals {
		s, err := stepFromStarlark(v)
		if err != nil {
			return nil, fmt.Errorf("step %d: %w", i+1, err)
		}
		steps = append(steps, s)
	}
	if len(steps) == 0 {
		return nil, fmt.Errorf("no steps provided")
	}
	return steps, nil
}

func resolveChangeSteps(credName string, steps []*Step, opts buddy.Opts, logger *slog.Logger) ([]resolvedStep, error) {
	resolved := make([]resolvedStep, 0, len(steps))
	for i, s := range steps {
		rs, err := resolveWorkflowStep(credName, s, opts, logger)
		if err != nil {
			return nil, fmt.Errorf("step %d (%s): %w", i+1, s.Type, err)
		}
		resolved = append(resolved, rs)
	}
	return resolved, nil
}

// driftFingerprint captures intent that should be stable between plan and apply.
// It intentionally omits created policy IDs (unknown until after create).
func driftFingerprint(rs resolvedStep) string {
	s := rs.step
	base := s.Type + "|" + s.Note
	switch {
	case rs.pkgPlan != nil:
		p := rs.pkgPlan
		return fmt.Sprintf("%s|pkg|src=%d|mod=%v|name=%s|trig=%s|pkg=%d|dis=%v|%s",
			base, p.SourceID, p.IsModify, p.NewName, p.NewTrigger, p.PkgID, p.DisableSource, p.DisableReason)
	case rs.clonePlan != nil:
		p := rs.clonePlan
		return fmt.Sprintf("%s|clone|src=%d|name=%s|trig=%s|pkg=%d|dis=%v|%s",
			base, p.SourceID, p.NewName, p.NewTrigger, p.PkgID, p.DisableSource, p.DisableReason)
	case rs.createPkgPlan != nil:
		p := rs.createPkgPlan
		return fmt.Sprintf("%s|create_pkg|name=%s|trig=%s|pkg=%d|action=%s|scope_all=%v|groups=%v",
			base, p.NewName, p.NewTrigger, p.PkgID, p.Action, p.ScopeAll, p.ScopeGroupIDs)
	case rs.createProfile != nil:
		p := rs.createProfile
		return fmt.Sprintf("%s|create_profile|name=%s|domains=%v|scope_all=%v|groups=%v",
			base, p.Name, s.PayloadDomains, p.ScopeAll, p.ScopeGroupIDs)
	default:
		return base + "|" + s.planLine()
	}
}

func formatChangePlanMarkdown(meta ChangeMeta, resolved []resolvedStep) string {
	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(meta.Title)
	b.WriteString("\n\n")
	var metaBits []string
	if meta.Risk != "" {
		metaBits = append(metaBits, "Risk: "+meta.Risk)
	}
	if meta.Window != "" {
		metaBits = append(metaBits, "Window: "+meta.Window)
	}
	if len(metaBits) > 0 {
		b.WriteString(strings.Join(metaBits, " | "))
		b.WriteString("\n\n")
	}
	if strings.TrimSpace(meta.Summary) != "" {
		b.WriteString(strings.TrimSpace(meta.Summary))
		b.WriteString("\n\n")
	}
	b.WriteString("## Planned changes\n\n")
	b.WriteString("Created policy and profile IDs are assigned by Jamf at apply time and are recorded in Outcomes.\n\n")
	for i, rs := range resolved {
		b.WriteString(formatChangeStepPlan(i+1, rs))
		b.WriteString("\n")
	}
	return b.String()
}

func formatChangeStepPlan(n int, rs resolvedStep) string {
	var b strings.Builder
	s := rs.step
	if s.Type == stepTypeNote {
		fmt.Fprintf(&b, "%d. **Note:** %s\n", n, s.Note)
		return b.String()
	}
	if rs.pkgPlan != nil {
		p := rs.pkgPlan
		if p.IsModify {
			fmt.Fprintf(&b, "%d. **Update** policy %q (ID %d)\n", n, p.NewName, p.SourceID)
			fmt.Fprintf(&b, "   - Set package to %s (ID %d)\n", p.PkgName, p.PkgID)
		} else {
			fmt.Fprintf(&b, "%d. **Create** policy %q with trigger `%s`\n", n, p.NewName, p.NewTrigger)
			fmt.Fprintf(&b, "   - Source: %q (ID %d, trigger `%s`)\n", p.SourceName, p.SourceID, p.SourceTrigger)
			fmt.Fprintf(&b, "   - Package: %s (ID %d)\n", p.PkgName, p.PkgID)
		}
		fmt.Fprintf(&b, "   - %s\n", buddy.FormatDisableAction(p.DisableSource, p.DisableReason))
	} else if rs.clonePlan != nil {
		p := rs.clonePlan
		fmt.Fprintf(&b, "%d. **Clone** to new policy %q with trigger `%s`\n", n, p.NewName, p.NewTrigger)
		fmt.Fprintf(&b, "   - Source: %q (ID %d, trigger `%s`)\n", p.SourceName, p.SourceID, p.SourceTrigger)
		if p.PkgName != "" {
			fmt.Fprintf(&b, "   - Package: %s (ID %d)", p.PkgName, p.PkgID)
			if p.OldPkgName != "" {
				fmt.Fprintf(&b, " (replaces %s)", p.OldPkgName)
			}
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "   - %s\n", buddy.FormatDisableAction(p.DisableSource, p.DisableReason))
	} else if rs.createPkgPlan != nil {
		p := rs.createPkgPlan
		fmt.Fprintf(&b, "%d. **Create** %s policy %q with trigger `%s`\n", n, p.Action, p.NewName, p.NewTrigger)
		fmt.Fprintf(&b, "   - Package: %s (ID %d)\n", p.PkgName, p.PkgID)
		if p.ScopeAll || len(p.ScopeGroupIDs) == 0 {
			b.WriteString("   - Scope: all computers\n")
		} else {
			fmt.Fprintf(&b, "   - Scope: groups %v\n", p.ScopeGroupIDs)
		}
	} else if rs.createProfile != nil {
		p := rs.createProfile
		fmt.Fprintf(&b, "%d. **Create** configuration profile %q\n", n, p.Name)
		domains := s.PayloadDomains
		if len(domains) == 0 {
			domains = []string{"(unknown)"}
		}
		fmt.Fprintf(&b, "   - Payload domains: %s\n", strings.Join(domains, ", "))
		if p.ScopeAll || len(p.ScopeGroupIDs) == 0 {
			b.WriteString("   - Scope: all computers\n")
		} else {
			fmt.Fprintf(&b, "   - Scope: groups %v\n", p.ScopeGroupIDs)
		}
	} else if s.Type == stepTypeCreateSS {
		fmt.Fprintf(&b, "%d. **Create** Self Service policy %q\n", n, s.DisplayName)
		fmt.Fprintf(&b, "   - Script parameter trigger: `%s`\n", s.Trigger)
	} else {
		fmt.Fprintf(&b, "%d. %s\n", n, s.planLine())
	}
	if s.Note != "" && s.Type != stepTypeNote {
		fmt.Fprintf(&b, "   - Note: %s\n", s.Note)
	}
	return b.String()
}

func formatChangeOutcomesMarkdown(outcomes []ChangeOutcome) string {
	var b strings.Builder
	b.WriteString("## Outcomes\n\n")
	if len(outcomes) == 0 {
		b.WriteString("(no steps applied)\n")
		return b.String()
	}
	for _, o := range outcomes {
		for _, line := range o.Lines {
			b.WriteString(line)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

func outcomeLines(n int, rs resolvedStep, mut *buddy.PolicyMutationOutcome) []string {
	s := rs.step
	if s.Type == stepTypeNote {
		return []string{fmt.Sprintf("%d. Note (no change): %s", n, s.Note)}
	}
	if mut == nil {
		return []string{fmt.Sprintf("%d. Applied: %s", n, s.planLine())}
	}
	var lines []string
	switch mut.Kind {
	case "update":
		lines = append(lines, fmt.Sprintf("%d. Updated policy %q (ID %d) → package %s (ID %d)",
			n, mut.PolicyName, mut.PolicyID, mut.PackageName, mut.PackageID))
	case "create":
		lines = append(lines, fmt.Sprintf("%d. Created policy %q (ID %d) with trigger `%s`",
			n, mut.PolicyName, mut.PolicyID, mut.Trigger))
		if mut.PackageName != "" {
			lines = append(lines, fmt.Sprintf("   - Package: %s (ID %d)", mut.PackageName, mut.PackageID))
		}
	case "create_profile":
		lines = append(lines, fmt.Sprintf("%d. Created configuration profile %q (ID %d)",
			n, mut.PolicyName, mut.PolicyID))
	default:
		lines = append(lines, fmt.Sprintf("%d. Applied: %s", n, s.planLine()))
	}
	if mut.DisabledSourceID > 0 {
		lines = append(lines, fmt.Sprintf("   - Disabled source %q (ID %d)", mut.DisabledSourceName, mut.DisabledSourceID))
	}
	if s.Note != "" {
		lines = append(lines, fmt.Sprintf("   - Note: %s", s.Note))
	}
	return lines
}

// planChange resolves steps and returns markdown + drift fingerprints (no writes).
func planChange(credName string, changeVal starlark.Value, stepVals []starlark.Value, logger *slog.Logger) (markdown string, fingerprints []string, err error) {
	meta, err := changeMetaFromStarlark(changeVal)
	if err != nil {
		return "", nil, err
	}
	steps, err := parseChangeSteps(stepVals)
	if err != nil {
		return "", nil, err
	}
	opts := buddy.Opts{SkipConfirm: true, Verbose: scriptOpts.Verbose}
	resolved, err := resolveChangeSteps(credName, steps, opts, logger)
	if err != nil {
		return "", nil, err
	}
	return formatChangePlanMarkdown(meta, resolved), fingerprintsForResolved(resolved), nil
}

// runChange re-resolves, warns on drift, confirms, applies, and returns plan+outcomes markdown.
func runChange(credName string, changeVal starlark.Value, stepVals []starlark.Value, plannedFingerprints []string, logger *slog.Logger) (string, error) {
	meta, err := changeMetaFromStarlark(changeVal)
	if err != nil {
		return "", err
	}
	steps, err := parseChangeSteps(stepVals)
	if err != nil {
		return "", err
	}
	opts := buddy.Opts{SkipConfirm: true, Verbose: scriptOpts.Verbose}
	resolved, err := resolveChangeSteps(credName, steps, opts, logger)
	if err != nil {
		return "", err
	}

	planMD := formatChangePlanMarkdown(meta, resolved)
	fmt.Print(planMD)

	if len(plannedFingerprints) > 0 {
		var drifts []string
		if len(plannedFingerprints) != len(resolved) {
			drifts = append(drifts, fmt.Sprintf("step count changed: planned %d, now %d", len(plannedFingerprints), len(resolved)))
		} else {
			for i, rs := range resolved {
				fp := driftFingerprint(rs)
				if fp != plannedFingerprints[i] {
					drifts = append(drifts, fmt.Sprintf("step %d (%s) resolved differently than at plan time", i+1, rs.step.Type))
				}
			}
		}
		if len(drifts) > 0 {
			fmt.Println("## Drift warning")
			fmt.Println()
			fmt.Println("Resolved state differs from the earlier plan:")
			for _, d := range drifts {
				fmt.Printf("- %s\n", d)
			}
			fmt.Println()
			if !runnerOpts.SkipConfirm {
				if !buddy.ConfirmPrompt("Proceed despite drift?", logger) {
					return planMD + "\n(aborted on drift)\n", nil
				}
			}
		}
	}

	if !runnerOpts.SkipConfirm {
		if !buddy.ConfirmPrompt("Apply this change batch?", logger) {
			return planMD + "\n(aborted before apply)\n", nil
		}
	}

	outcomes := make([]ChangeOutcome, 0, len(resolved))
	for i, rs := range resolved {
		mut, err := applyChangeStep(credName, rs, opts, logger)
		if err != nil {
			partial := formatChangeOutcomesMarkdown(outcomes)
			return planMD + "\n" + partial, fmt.Errorf("step %d (%s): %w", i+1, rs.step.Type, err)
		}
		outcomes = append(outcomes, ChangeOutcome{
			Index:   i + 1,
			Lines:   outcomeLines(i+1, rs, mut),
			Outcome: mut,
		})
	}

	outMD := formatChangeOutcomesMarkdown(outcomes)
	fmt.Print(outMD)
	return planMD + "\n" + outMD, nil
}

func applyChangeStep(credName string, rs resolvedStep, opts buddy.Opts, logger *slog.Logger) (*buddy.PolicyMutationOutcome, error) {
	switch rs.step.Type {
	case stepTypeNote:
		return nil, nil
	case stepTypeSetPackage:
		if rs.pkgPlan != nil {
			return buddy.ApplyPolicyPackagePlan(credName, opts, rs.pkgPlan, logger)
		}
	case stepTypeClonePolicy:
		if rs.clonePlan != nil {
			return buddy.ApplyPolicyClonePlan(credName, opts, rs.clonePlan, logger)
		}
	case stepTypeCreatePackagePolicy:
		if rs.createPkgPlan != nil {
			return buddy.ApplyPolicyCreatePackagePlan(credName, opts, rs.createPkgPlan, logger)
		}
	case stepTypeCreateProfile:
		if rs.createProfile != nil {
			out, err := buddy.ProfileCreate(credName, opts, *rs.createProfile, logger)
			if err != nil {
				return nil, err
			}
			if out == nil {
				return nil, nil
			}
			return &buddy.PolicyMutationOutcome{
				Kind:       "create_profile",
				PolicyID:   out.ProfileID,
				PolicyName: out.Name,
			}, nil
		}
	case stepTypeCreateSS:
		s := rs.step
		id, err := buddy.PolicyCreateSS(credName, opts, s.Trigger, s.DisplayName, s.Category, s.ScopeGroupID, s.ScriptID, logger)
		if err != nil {
			return nil, err
		}
		if id == 0 {
			return nil, nil
		}
		return &buddy.PolicyMutationOutcome{
			Kind:       "create",
			PolicyID:   id,
			PolicyName: s.DisplayName + " [SS]",
			Trigger:    s.Trigger,
		}, nil
	}
	if err := executeStep(credName, rs.step, opts, logger); err != nil {
		return nil, err
	}
	return nil, nil
}

// fingerprintsForResolved returns drift fingerprints for optional plan→apply handoff.
func fingerprintsForResolved(resolved []resolvedStep) []string {
	out := make([]string, len(resolved))
	for i, rs := range resolved {
		out[i] = driftFingerprint(rs)
	}
	return out
}
