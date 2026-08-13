package buddy

import (
	"strings"
	"testing"
)

func TestResolveCloneDisable(t *testing.T) {
	tests := []struct {
		name    string
		mode    DisableSourceMode
		disable bool
		reason  string
	}{
		{"auto", DisableSourceAuto, true, "clone default"},
		{"keep", DisableSourceKeep, false, "keep-source"},
		{"force", DisableSourceForce, true, "disable-source"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, reason := ResolveCloneDisable(tt.mode)
			if got != tt.disable || reason != tt.reason {
				t.Fatalf("ResolveCloneDisable(%v) = %v %q, want %v %q", tt.mode, got, reason, tt.disable, tt.reason)
			}
		})
	}
}

func TestResolvePackageDisable(t *testing.T) {
	tests := []struct {
		name          string
		isModify      bool
		newTrigger    string
		sourceTrigger string
		mode          DisableSourceMode
		disable       bool
		reason        string
	}{
		{
			name:     "modify in place",
			isModify: true, newTrigger: "install-app", sourceTrigger: "install-app",
			mode: DisableSourceAuto, disable: false, reason: "same version — in-place update",
		},
		{
			name:     "same trigger replacement",
			isModify: false, newTrigger: "install-app", sourceTrigger: "install-app",
			mode: DisableSourceAuto, disable: true, reason: "same trigger — replacement",
		},
		{
			name:     "versioned trigger change",
			isModify: false, newTrigger: "cache-app-2.0", sourceTrigger: "cache-app-1.0",
			mode: DisableSourceAuto, disable: false, reason: "trigger changes",
		},
		{
			name:     "force disable on versioned",
			isModify: false, newTrigger: "cache-app-2.0", sourceTrigger: "cache-app-1.0",
			mode: DisableSourceForce, disable: true, reason: "disable-source",
		},
		{
			name:     "keep on same trigger",
			isModify: false, newTrigger: "install-app", sourceTrigger: "install-app",
			mode: DisableSourceKeep, disable: false, reason: "keep-source",
		},
		{
			name:     "case insensitive trigger match",
			isModify: false, newTrigger: "Install-App", sourceTrigger: "install-app",
			mode: DisableSourceAuto, disable: true, reason: "same trigger — replacement",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, reason := ResolvePackageDisable(tt.isModify, tt.newTrigger, tt.sourceTrigger, tt.mode)
			if got != tt.disable || reason != tt.reason {
				t.Fatalf("got %v %q, want %v %q", got, reason, tt.disable, tt.reason)
			}
		})
	}
}

func TestFormatDisableAction(t *testing.T) {
	if got := FormatDisableAction(true, "clone default"); got != "Disable source: yes (clone default)" {
		t.Fatalf("got %q", got)
	}
	if got := FormatDisableAction(false, "trigger changes"); got != "Disable source: no (trigger changes)" {
		t.Fatalf("got %q", got)
	}
}

func TestBuildPackagePlanActions(t *testing.T) {
	modify := &PolicyPackagePlan{
		SourceID: 12, SourceName: "Install Foo 1.0", SourceTrigger: "install-foo",
		IsModify: true, NewName: "Install Foo 1.0", NewTrigger: "install-foo",
		PkgID: 99, PkgName: "Foo-1.0.pkg",
		DisableSource: false, DisableReason: "same version — in-place update",
	}
	actions := buildPackagePlanActions(modify)
	if len(actions) != 3 {
		t.Fatalf("want 3 actions, got %d: %v", len(actions), actions)
	}
	if !strings.Contains(actions[0], `Source policy "Install Foo 1.0" (ID 12, trigger install-foo)`) {
		t.Errorf("source line: %q", actions[0])
	}
	if !strings.Contains(actions[1], "Update policy") || !strings.Contains(actions[1], "Foo-1.0.pkg") {
		t.Errorf("update line: %q", actions[1])
	}
	if actions[2] != "Disable source: no (same version — in-place update)" {
		t.Errorf("disable line: %q", actions[2])
	}

	create := &PolicyPackagePlan{
		SourceID: 12, SourceName: "Cache Foo 1.0", SourceTrigger: "cache-foo-1.0",
		IsModify: false, NewName: "Cache Foo 2.0", NewTrigger: "cache-foo-2.0",
		PkgID: 100, PkgName: "Foo-2.0.pkg",
		DisableSource: false, DisableReason: "trigger changes",
	}
	actions = buildPackagePlanActions(create)
	if !strings.Contains(actions[1], `Create policy "Cache Foo 2.0"`) || !strings.Contains(actions[1], "cache-foo-2.0") {
		t.Errorf("create line: %q", actions[1])
	}
	if actions[2] != "Disable source: no (trigger changes)" {
		t.Errorf("disable line: %q", actions[2])
	}
}

func TestParsePolicyIDFromPostBody(t *testing.T) {
	if id := parsePolicyIDFromPostBody([]byte(`<policy><id>42</id></policy>`)); id != 42 {
		t.Fatalf("xml wrap id=%d", id)
	}
	if id := parsePolicyIDFromPostBody([]byte(`{"id": 99}`)); id != 99 {
		t.Fatalf("json id=%d", id)
	}
	if id := parsePolicyIDFromPostBody([]byte(`{"policy":{"id":"7"}}`)); id != 7 {
		t.Fatalf("nested json id=%d", id)
	}
	if id := parsePolicyIDFromPostBody([]byte(``)); id != 0 {
		t.Fatalf("empty id=%d", id)
	}
}

func TestLooksLikeTriggerKeyword(t *testing.T) {
	if !looksLikeTriggerKeyword("install-chrome-next") {
		t.Fatal("expected install-chrome-next to look like trigger")
	}
	if !looksLikeTriggerKeyword("cache-app-1.0") {
		t.Fatal("expected cache-app-1.0 to look like trigger")
	}
	if looksLikeTriggerKeyword("chrome") {
		t.Fatal("chrome alone should not look like trigger")
	}
	if looksLikeTriggerKeyword("install") {
		t.Fatal("bare install should not look like trigger")
	}
}

func TestFindByNameAndVersionPrefersCachePolicy(t *testing.T) {
	policies := []policyListItem{
		{ID: 2, Name: "Install Santa 2026.3"},
		{ID: 3, Name: "Cache Santa 2026.3"},
		{ID: 21, Name: "Install Santa [SS]"},
	}
	id, ok := findByNameAndVersion(policies, "cache-santa-2026.3")
	if !ok || id != 3 {
		t.Fatalf("cache trigger picked ID %d ok=%v, want cache policy 3 (not install 2)", id, ok)
	}
	id, ok = findByNameAndVersion(policies, "install-santa-next")
	if !ok || id != 2 {
		t.Fatalf("install-next picked ID %d ok=%v, want install policy 2", id, ok)
	}
}

func TestExtractVersionFromTriggerWithoutExactName(t *testing.T) {
	if got := extractVersionFromTrigger("cache-santa-2026.3", "santa-daemon"); got != "2026.3" {
		t.Fatalf("got %q, want 2026.3 from parseTrigger even when name does not match", got)
	}
	if got := extractVersionFromTrigger("install-santa-next", "santa"); got != "" {
		t.Fatalf("unversioned install-next should have empty version, got %q", got)
	}
}

func TestPackageSourceRoleError(t *testing.T) {
	install := &resourcePolicy{}
	install.General.ID = 23
	install.General.Name = "Install Santa Next"
	install.General.TriggerOther = "install-santa-next"
	install.PackageConfiguration.Packages = []policySubsetPackageConfigurationPackage{{ID: 1, Name: "Santa-2026.2.pkg", Action: "Install"}}

	err := packageSourceRoleError("cache-santa-2026.2", install)
	if err == nil {
		t.Fatal("expected error when cache trigger resolves to install policy")
	}

	cache := &resourcePolicy{}
	cache.General.ID = 10
	cache.General.Name = "Cache Santa 2026.2"
	cache.General.TriggerOther = "cache-santa-2026.2"
	cache.PackageConfiguration.Packages = []policySubsetPackageConfigurationPackage{{ID: 1, Name: "Santa-2026.2.pkg", Action: "Cache"}}
	if err := packageSourceRoleError("cache-santa-2026.2", cache); err != nil {
		t.Fatalf("cache→cache should be ok: %v", err)
	}
}
