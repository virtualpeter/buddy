package script

import (
	"strings"
	"testing"

	"buddy/internal/buddy"

	"go.starlark.net/starlark"
)

func TestStepFromStarlark_NotDict(t *testing.T) {
	_, err := stepFromStarlark(starlark.String("x"))
	if err == nil {
		t.Fatal("expected error for non-dict value")
	}
	if !strings.Contains(err.Error(), "workflow step must be a dict") {
		t.Errorf("error = %v", err)
	}
}

func TestStepFromStarlark_MissingType(t *testing.T) {
	d := new(starlark.Dict)
	_ = d.SetKey(starlark.String("name"), starlark.String("A"))
	_, err := stepFromStarlark(d)
	if err == nil {
		t.Fatal("expected error when type is missing")
	}
	if !strings.Contains(err.Error(), "missing required field \"type\"") {
		t.Errorf("error = %v", err)
	}
}

func TestStepFromStarlark_UnknownType(t *testing.T) {
	d := stepDict("unknown_step")
	_, err := stepFromStarlark(d)
	if err == nil {
		t.Fatal("expected error for unknown step type")
	}
	if !strings.Contains(err.Error(), "unknown workflow step type") {
		t.Errorf("error = %v", err)
	}
}

func TestStepFromStarlark_SetPackage_Valid(t *testing.T) {
	d := stepDict(stepTypeSetPackage, "name", "MyApp", "version", "1.0.0", "from_trigger", "Enroll")
	s, err := stepFromStarlark(d)
	if err != nil {
		t.Fatalf("stepFromStarlark: %v", err)
	}
	if s.Type != stepTypeSetPackage || s.Name != "MyApp" || s.Version != "1.0.0" || s.FromTrigger != "Enroll" {
		t.Errorf("got Type=%q Name=%q Version=%q FromTrigger=%q", s.Type, s.Name, s.Version, s.FromTrigger)
	}
	if s.DisableMode != buddy.DisableSourceAuto {
		t.Errorf("DisableMode = %v, want Auto", s.DisableMode)
	}
}

func TestStepFromStarlark_SetPackage_KeepSource(t *testing.T) {
	d := stepDict(stepTypeSetPackage, "name", "MyApp", "version", "1.0.0", "from_trigger", "install-app")
	_ = d.SetKey(starlark.String("keep_source"), starlark.True)
	s, err := stepFromStarlark(d)
	if err != nil {
		t.Fatalf("stepFromStarlark: %v", err)
	}
	if s.DisableMode != buddy.DisableSourceKeep {
		t.Fatalf("DisableMode = %v, want Keep", s.DisableMode)
	}
}

func TestStepFromStarlark_SetPackage_PolicyID(t *testing.T) {
	d := stepDict(stepTypeSetPackage, "name", "santa", "version", "2026.3", "from_trigger", "cache-santa-2026.2", "id", 10)
	s, err := stepFromStarlark(d)
	if err != nil {
		t.Fatalf("stepFromStarlark: %v", err)
	}
	if s.PolicyID != 10 {
		t.Fatalf("PolicyID = %d, want 10", s.PolicyID)
	}
}

func TestStepFromStarlark_SetPackage_DisableSource(t *testing.T) {
	d := stepDict(stepTypeSetPackage, "name", "MyApp", "version", "1.0.0", "from_trigger", "install-app")
	_ = d.SetKey(starlark.String("disable_source"), starlark.True)
	s, err := stepFromStarlark(d)
	if err != nil {
		t.Fatalf("stepFromStarlark: %v", err)
	}
	if s.DisableMode != buddy.DisableSourceForce {
		t.Fatalf("DisableMode = %v, want Force", s.DisableMode)
	}
}

func TestStepFromStarlark_SetPackage_ConflictingDisableFlags(t *testing.T) {
	d := stepDict(stepTypeSetPackage, "name", "MyApp", "version", "1.0.0", "from_trigger", "install-app")
	_ = d.SetKey(starlark.String("disable_source"), starlark.True)
	_ = d.SetKey(starlark.String("keep_source"), starlark.True)
	_, err := stepFromStarlark(d)
	if err == nil {
		t.Fatal("expected error for conflicting flags")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error = %v", err)
	}
}

func TestStepFromStarlark_ClonePolicy_KeepSource(t *testing.T) {
	d := stepDict(stepTypeClonePolicy, "from_trigger", "install-app", "version", "2.0")
	_ = d.SetKey(starlark.String("keep_source"), starlark.True)
	s, err := stepFromStarlark(d)
	if err != nil {
		t.Fatalf("stepFromStarlark: %v", err)
	}
	if s.DisableMode != buddy.DisableSourceKeep {
		t.Fatalf("DisableMode = %v, want Keep", s.DisableMode)
	}
}

func TestResolvedWorkflowPlanLines_IncludeDisable(t *testing.T) {
	// Mirror how runWorkflow builds plan lines from a package plan (no Jamf call).
	plan := &buddy.PolicyPackagePlan{
		SourceID: 12, SourceName: "Install Foo", SourceTrigger: "install-foo",
		IsModify: false, NewName: "Install Foo 2", NewTrigger: "install-foo-2.0",
		PkgID: 99, PkgName: "Foo-2.pkg",
		DisableSource: false, DisableReason: "trigger changes",
	}
	plan.Actions = []string{
		`Source policy "Install Foo" (ID 12, trigger install-foo)`,
		`Create policy "Install Foo 2" with trigger install-foo-2.0, package Foo-2.pkg (ID 99)`,
		buddy.FormatDisableAction(plan.DisableSource, plan.DisableReason),
	}
	lines := append([]string{"set_package        Foo 2.0"}, plan.Actions...)
	joined := strings.Join(lines, "\n")
	for _, want := range []string{
		"set_package",
		`Source policy "Install Foo" (ID 12, trigger install-foo)`,
		"Create policy",
		"Disable source: no (trigger changes)",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("plan missing %q\n%s", want, joined)
		}
	}
}

func TestStepFromStarlark_SetPackage_MissingField(t *testing.T) {
	// missing from_trigger
	d := stepDict(stepTypeSetPackage, "name", "A", "version", "1.0")
	_, err := stepFromStarlark(d)
	if err == nil {
		t.Fatal("expected error when from_trigger missing")
	}
	if !strings.Contains(err.Error(), "name, version, from_trigger") {
		t.Errorf("error = %v", err)
	}
}

func TestStepFromStarlark_UpgradeApp_Valid(t *testing.T) {
	d := stepDict(stepTypeUpgradeApp, "name", "tool", "version", "1.1", "package_pattern", "*tool*1.1*")
	s, err := stepFromStarlark(d)
	if err != nil {
		t.Fatalf("stepFromStarlark: %v", err)
	}
	if s.Type != stepTypeUpgradeApp || s.Name != "tool" || s.Version != "1.1" || s.PackagePattern != "*tool*1.1*" {
		t.Errorf("got %+v", s)
	}
}

func TestStepFromStarlark_SetSS_Valid(t *testing.T) {
	d := stepDict(stepTypeSetSS, "id", 10, "version", "2.0")
	s, err := stepFromStarlark(d)
	if err != nil {
		t.Fatalf("stepFromStarlark: %v", err)
	}
	if s.Type != stepTypeSetSS || s.PolicyID != 10 || s.Version != "2.0" {
		t.Errorf("got Type=%q PolicyID=%d Version=%q", s.Type, s.PolicyID, s.Version)
	}
}

func TestStepFromStarlark_SetSS_MissingVersion(t *testing.T) {
	d := stepDict(stepTypeSetSS, "id", 10)
	_, err := stepFromStarlark(d)
	if err == nil {
		t.Fatal("expected error when version missing")
	}
	if !strings.Contains(err.Error(), "id, version") {
		t.Errorf("error = %v", err)
	}
}

func TestStepFromStarlark_EnablePolicy_Valid(t *testing.T) {
	d := stepDict(stepTypeEnablePolicy, "id", 5)
	s, err := stepFromStarlark(d)
	if err != nil {
		t.Fatalf("stepFromStarlark: %v", err)
	}
	if s.Type != stepTypeEnablePolicy || s.PolicyID != 5 {
		t.Errorf("got Type=%q PolicyID=%d", s.Type, s.PolicyID)
	}
}

func TestStepFromStarlark_SetScope_Valid(t *testing.T) {
	l := starlark.NewList([]starlark.Value{starlark.MakeInt(1), starlark.MakeInt(2)})
	d := stepDict(stepTypeSetScope, "id", 7, "group_ids", l)
	s, err := stepFromStarlark(d)
	if err != nil {
		t.Fatalf("stepFromStarlark: %v", err)
	}
	if s.Type != stepTypeSetScope || s.PolicyID != 7 || len(s.GroupIDs) != 2 || s.GroupIDs[0] != 1 || s.GroupIDs[1] != 2 {
		t.Errorf("got Type=%q PolicyID=%d GroupIDs=%v", s.Type, s.PolicyID, s.GroupIDs)
	}
}

func TestStepFromStarlark_SetScope_MissingGroupIDs(t *testing.T) {
	d := stepDict(stepTypeSetScope, "id", 7)
	_, err := stepFromStarlark(d)
	if err == nil {
		t.Fatal("expected error when group_ids missing")
	}
	if !strings.Contains(err.Error(), "id, group_ids") {
		t.Errorf("error = %v", err)
	}
}

func TestStepFromStarlark_CreatePolicy_Minimal(t *testing.T) {
	d := stepDict(stepTypeCreatePolicy, "name", "My Policy", "trigger", "Login", "script_id", 42)
	s, err := stepFromStarlark(d)
	if err != nil {
		t.Fatalf("stepFromStarlark: %v", err)
	}
	if s.Type != stepTypeCreatePolicy || s.PolicyName != "My Policy" || s.PolicyTrigger != "Login" || s.ScriptID != 42 {
		t.Errorf("got Type=%q Name=%q Trigger=%q ScriptID=%d", s.Type, s.PolicyName, s.PolicyTrigger, s.ScriptID)
	}
}

func TestStepFromStarlark_CreatePolicy_VersionSubstitution(t *testing.T) {
	d := stepDict(stepTypeCreatePolicy, "name", "App {version}", "trigger", "v{version}", "script_id", 1, "version", "2.0")
	s, err := stepFromStarlark(d)
	if err != nil {
		t.Fatalf("stepFromStarlark: %v", err)
	}
	if s.PolicyName != "App 2.0" || s.PolicyTrigger != "v2.0" {
		t.Errorf("got Name=%q Trigger=%q, want {version} substituted", s.PolicyName, s.PolicyTrigger)
	}
}

func TestStepFromStarlark_SetProfilePayload_Valid(t *testing.T) {
	d := stepDict(stepTypeSetProfilePayload, "id", 99, "domain", "com.example.app", "dict_file", "/path/to/dict.plist")
	s, err := stepFromStarlark(d)
	if err != nil {
		t.Fatalf("stepFromStarlark: %v", err)
	}
	if s.Type != stepTypeSetProfilePayload || s.ProfileID != 99 || s.Domain != "com.example.app" || s.DictFile != "/path/to/dict.plist" {
		t.Errorf("got Type=%q ProfileID=%d Domain=%q DictFile=%q", s.Type, s.ProfileID, s.Domain, s.DictFile)
	}
}

func TestStepFromStarlark_SetProfilePayload_MissingField(t *testing.T) {
	d := stepDict(stepTypeSetProfilePayload, "id", 99, "domain", "com.example")
	_, err := stepFromStarlark(d)
	if err == nil {
		t.Fatal("expected error when dict_file missing")
	}
	if !strings.Contains(err.Error(), "id, domain, dict_file") {
		t.Errorf("error = %v", err)
	}
}

func TestStepFromStarlark_CreatePackagePolicy(t *testing.T) {
	d := stepDict(stepTypeCreatePackagePolicy, "name", "example", "version", "1.0", "action", "cache", "note", "n")
	s, err := stepFromStarlark(d)
	if err != nil {
		t.Fatalf("stepFromStarlark: %v", err)
	}
	if s.Type != stepTypeCreatePackagePolicy || s.Name != "example" || s.Action != "cache" || !s.ScopeAll {
		t.Errorf("got %+v", s)
	}
}

func TestStepFromStarlark_CreateProfile(t *testing.T) {
	plist := `<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict><key>PayloadContent</key><array><dict><key>PayloadIdentifier</key><string>com.example.app</string></dict></array></dict></plist>`
	d := stepDict(stepTypeCreateProfile, "name", "Prefs", "payloads_plist_str", plist, "scope_all", true)
	s, err := stepFromStarlark(d)
	if err != nil {
		t.Fatalf("stepFromStarlark: %v", err)
	}
	if s.Type != stepTypeCreateProfile || s.PolicyName != "Prefs" || len(s.PayloadDomains) != 1 {
		t.Errorf("got %+v", s)
	}
}

func TestStepFromStarlark_CreateSS(t *testing.T) {
	d := stepDict(stepTypeCreateSS, "trigger", "install-example-next", "display_name", "Install Example")
	s, err := stepFromStarlark(d)
	if err != nil {
		t.Fatalf("stepFromStarlark: %v", err)
	}
	if s.Type != stepTypeCreateSS || s.Trigger != "install-example-next" || s.DisplayName != "Install Example" {
		t.Errorf("got %+v", s)
	}
}
