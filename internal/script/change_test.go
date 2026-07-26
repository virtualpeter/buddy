package script

import (
	"strings"
	"testing"

	"buddy/internal/buddy"

	"go.starlark.net/starlark"
)

func TestFormatChangePlanMarkdown_OmitsCreatePolicyID(t *testing.T) {
	meta := ChangeMeta{
		Title:   "Santa 2026.3",
		Summary: "Bump cache and install.",
		Risk:    "low",
		Window:  " tonight",
	}
	rs := resolvedStep{
		step: &Step{
			Type: stepTypeSetPackage,
			App:  "santa",
			Note: "Leave prior cache enabled.",
		},
		pkgPlan: &buddy.PolicyPackagePlan{
			SourceID: 19, SourceName: "Cache Santa 2026.2", SourceTrigger: "cache-santa-2026.2",
			IsModify: false, NewName: "Cache Santa 2026.3", NewTrigger: "cache-santa-2026.3",
			PkgID: 7, PkgName: "santa-2026.3.pkg",
			DisableSource: false, DisableReason: "trigger changes",
		},
	}
	md := formatChangePlanMarkdown(meta, []resolvedStep{rs})
	if !strings.Contains(md, "# Santa 2026.3") {
		t.Fatalf("missing title:\n%s", md)
	}
	if !strings.Contains(md, "**Create** policy \"Cache Santa 2026.3\"") {
		t.Fatalf("missing create line:\n%s", md)
	}
	if !strings.Contains(md, "Source: \"Cache Santa 2026.2\" (ID 19") {
		t.Fatalf("missing source id:\n%s", md)
	}
	if !strings.Contains(md, "Package: santa-2026.3.pkg (ID 7)") {
		t.Fatalf("missing package id:\n%s", md)
	}
	if !strings.Contains(md, "Note: Leave prior cache enabled.") {
		t.Fatalf("missing note:\n%s", md)
	}
	// Must not invent a created policy id in the plan body.
	if strings.Contains(md, "Created policy") && strings.Contains(md, "(ID ") {
		// Outcomes section isn't present; ensure create line doesn't claim new policy ID.
	}
	createIdx := strings.Index(md, "**Create**")
	chunk := md[createIdx:]
	if end := strings.Index(chunk, "\n2."); end > 0 {
		chunk = chunk[:end]
	}
	// After "**Create** policy "Name"" the next ID mentions should be source/package only.
	if strings.Contains(chunk, "Create policy") && strings.Contains(chunk, "new policy ID") {
		t.Fatalf("plan should not predict create id:\n%s", chunk)
	}
	if strings.Contains(md, "Created policy and profile IDs are assigned by Jamf at apply time") == false {
		t.Fatalf("missing disclaimer:\n%s", md)
	}
}

func TestFormatChangeOutcomes_IncludesCreatedID(t *testing.T) {
	rs := resolvedStep{step: &Step{Type: stepTypeSetPackage, Note: "cache bump"}}
	mut := &buddy.PolicyMutationOutcome{
		Kind: "create", PolicyID: 22, PolicyName: "Cache Santa 2026.3",
		Trigger: "cache-santa-2026.3", PackageID: 7, PackageName: "santa-2026.3.pkg",
	}
	lines := outcomeLines(1, rs, mut)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "Created policy \"Cache Santa 2026.3\" (ID 22)") {
		t.Fatalf("outcomes missing created id:\n%s", joined)
	}
	md := formatChangeOutcomesMarkdown([]ChangeOutcome{{Index: 1, Lines: lines, Outcome: mut}})
	if !strings.Contains(md, "## Outcomes") || !strings.Contains(md, "(ID 22)") {
		t.Fatalf("bad outcomes md:\n%s", md)
	}
}

func TestDriftFingerprint_StableWithoutCreateID(t *testing.T) {
	rs := resolvedStep{
		step: &Step{Type: stepTypeSetPackage, Note: "n"},
		pkgPlan: &buddy.PolicyPackagePlan{
			SourceID: 19, IsModify: false, NewName: "A", NewTrigger: "t",
			PkgID: 7, DisableSource: false, DisableReason: "trigger changes",
		},
	}
	a := driftFingerprint(rs)
	b := driftFingerprint(rs)
	if a != b {
		t.Fatalf("fingerprint not stable: %q vs %q", a, b)
	}
	if strings.Contains(a, "PolicyID=22") {
		t.Fatalf("fingerprint should not include created id: %q", a)
	}
}

func TestStepNoteFromStarlark(t *testing.T) {
	d := stepDict(stepTypeNote, "note", "SS already points at install-santa-next")
	s, err := stepFromStarlark(d)
	if err != nil {
		t.Fatal(err)
	}
	if s.Type != stepTypeNote || s.Note == "" {
		t.Fatalf("got %+v", s)
	}
}

func TestStepSetPackageNote(t *testing.T) {
	d := stepDict(stepTypeSetPackage, "app", "santa", "version", "1", "from_trigger", "t", "note", "hello")
	s, err := stepFromStarlark(d)
	if err != nil {
		t.Fatal(err)
	}
	if s.Note != "hello" {
		t.Fatalf("note=%q", s.Note)
	}
}

func TestFormatChangePlan_CreatePackageAndProfileOmitsCreateIDs(t *testing.T) {
	meta := ChangeMeta{Title: "New app setup"}
	rs := []resolvedStep{
		{
			step: &Step{Type: stepTypeCreatePackagePolicy, Note: "cache"},
			createPkgPlan: &buddy.PolicyCreatePackagePlan{
				Action: "cache", NewName: "Cache Example 1.0", NewTrigger: "cache-example-1.0",
				PkgID: 3, PkgName: "example-1.0.pkg", ScopeAll: true,
			},
		},
		{
			step: &Step{
				Type: stepTypeCreateProfile, PolicyName: "Example PPPC",
				PayloadDomains: []string{"com.apple.TCC.configuration-profile-policy"},
				Note: "pppc",
			},
			createProfile: &buddy.ProfileCreateSpec{Name: "Example PPPC", ScopeAll: true},
		},
	}
	md := formatChangePlanMarkdown(meta, rs)
	if !strings.Contains(md, "**Create** cache policy") {
		t.Fatalf("missing create package line:\n%s", md)
	}
	if !strings.Contains(md, "**Create** configuration profile") {
		t.Fatalf("missing create profile line:\n%s", md)
	}
	if strings.Contains(md, "(ID 44)") || strings.Contains(md, "Created configuration profile \"Example PPPC\" (ID") {
		t.Fatalf("plan must not include created IDs:\n%s", md)
	}
	// Package ID in plan is OK (known before create); new policy/profile IDs are not.
	createIdx := strings.Index(md, "**Create** cache policy")
	if createIdx < 0 {
		t.Fatalf("missing create package:\n%s", md)
	}
}

func TestOutcomeLines_CreateProfile(t *testing.T) {
	rs := resolvedStep{step: &Step{Type: stepTypeCreateProfile, Note: "n"}}
	mut := &buddy.PolicyMutationOutcome{Kind: "create_profile", PolicyID: 44, PolicyName: "Example PPPC"}
	joined := strings.Join(outcomeLines(1, rs, mut), "\n")
	if !strings.Contains(joined, "Created configuration profile \"Example PPPC\" (ID 44)") {
		t.Fatalf("outcomes:\n%s", joined)
	}
}

func TestChangeBatchBuiltin_RequiresTitle(t *testing.T) {
	_, err := changeBatchBuiltin(nil, nil, nil, []starlark.Tuple{
		{starlark.String("title"), starlark.String("")},
	})
	if err == nil {
		t.Fatal("expected error for empty title")
	}
}
