package buddy

import (
	"strings"
	"testing"
)

func TestDefaultAppPackagePattern(t *testing.T) {
	got := DefaultAppPackagePattern("Santa", "2026.3")
	if got != "*santa*2026.3*" {
		t.Fatalf("pattern=%q", got)
	}
}

func TestAppDesiredFromDetail(t *testing.T) {
	d := &PolicyAppDetail{
		PolicyAppSummary: PolicyAppSummary{
			Name:           "santa",
			Kind:           PolicyAppCache,
			Version:        "2026.2",
			InstallTrigger: "install-santa-next",
			CachePolicyID:  10,
		},
		CachePolicies: []PolicyAppRef{
			{ID: 10, Trigger: "cache-santa-2026.2", Enabled: true},
		},
	}
	des := AppDesiredFromDetail(d)
	if des.Name != "santa" || des.Kind != "cache" || des.Version != "2026.2" {
		t.Fatalf("desired=%+v", des)
	}
	if des.CacheTrigger != "cache-santa-2026.2" {
		t.Fatalf("cache_trigger=%q", des.CacheTrigger)
	}
	if des.PackagePattern != "*santa*2026.2*" {
		t.Fatalf("package_pattern=%q", des.PackagePattern)
	}
	if !strings.Contains(des.Title, "santa") {
		t.Fatalf("title=%q", des.Title)
	}
}

func TestBuildAppReconcilePlanSameVersion(t *testing.T) {
	live := &PolicyAppDetail{
		PolicyAppSummary: PolicyAppSummary{
			Name: "santa", Kind: PolicyAppCache, Version: "2026.3",
			InstallTrigger: "install-santa-next", CachePolicyID: 1,
		},
		CachePolicies: []PolicyAppRef{{ID: 1, Trigger: "cache-santa-2026.3", Enabled: true}},
	}
	plan, err := BuildAppReconcilePlan(live, AppDesired{Name: "santa", Version: "2026.3", Kind: "cache"})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Noop || len(plan.Steps) != 1 || plan.Steps[0].Type != "note" {
		t.Fatalf("plan=%+v", plan)
	}
}

func TestBuildAppReconcilePlanCacheBump(t *testing.T) {
	live := &PolicyAppDetail{
		PolicyAppSummary: PolicyAppSummary{
			Name: "santa", Kind: PolicyAppCache, Version: "2026.2",
			InstallTrigger: "install-santa-next", CachePolicyID: 1, SSPolicyID: 99,
		},
		SSDescription: "Santa 2026.2",
		CachePolicies: []PolicyAppRef{{ID: 1, Trigger: "cache-santa-2026.2", Enabled: true}},
	}
	plan, err := BuildAppReconcilePlan(live, AppDesired{
		Name: "santa", Version: "2026.3", Kind: "cache",
		PackagePattern: "*santa*2026.3*",
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Noop {
		t.Fatal("expected mutate plan")
	}
	if len(plan.Steps) < 2 {
		t.Fatalf("steps=%+v", plan.Steps)
	}
	if plan.Steps[0].Type != "set_package" || !plan.Steps[0].KeepSource || plan.Steps[0].FromTrigger != "cache-santa-2026.2" {
		t.Fatalf("cache step=%+v", plan.Steps[0])
	}
	if plan.Steps[1].Type != "set_package" || plan.Steps[1].FromTrigger != "install-santa-next" {
		t.Fatalf("install step=%+v", plan.Steps[1])
	}
	foundSS := false
	for _, s := range plan.Steps {
		if s.Type == "set_ss" && s.PolicyID == 99 && s.Version == "2026.3" {
			foundSS = true
		}
	}
	if !foundSS {
		t.Fatalf("expected set_ss step; steps=%+v", plan.Steps)
	}
}

func TestBuildAppReconcilePlanInstallBump(t *testing.T) {
	live := &PolicyAppDetail{
		PolicyAppSummary: PolicyAppSummary{
			Name: "app", Kind: PolicyAppInstall, Version: "1.0",
			InstallTrigger: "install-app-next",
		},
	}
	plan, err := BuildAppReconcilePlan(live, AppDesired{Name: "app", Version: "2.0"})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Steps) != 1 || plan.Steps[0].Type != "set_package" {
		t.Fatalf("steps=%+v", plan.Steps)
	}
}

func TestBuildAppReconcilePlanSingleBump(t *testing.T) {
	live := &PolicyAppDetail{
		PolicyAppSummary: PolicyAppSummary{
			Name: "tool", Kind: PolicyAppSingle, Version: "1.0", SSPolicyID: 5,
		},
	}
	plan, err := BuildAppReconcilePlan(live, AppDesired{Name: "tool", Version: "1.1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Steps) != 1 || plan.Steps[0].Type != "upgrade_app" {
		t.Fatalf("steps=%+v", plan.Steps)
	}
}

func TestBuildAppReconcilePlanKindRewriteRejected(t *testing.T) {
	live := &PolicyAppDetail{
		PolicyAppSummary: PolicyAppSummary{Name: "x", Kind: PolicyAppSingle, Version: "1"},
	}
	_, err := BuildAppReconcilePlan(live, AppDesired{Name: "x", Version: "2", Kind: "cache"})
	if err == nil {
		t.Fatal("expected kind rewrite error")
	}
}

func TestBuildAppReconcilePlanCompatibleNames(t *testing.T) {
	live := &PolicyAppDetail{
		PolicyAppSummary: PolicyAppSummary{
			Name: "cursor-ide", Kind: PolicyAppInstall, Version: "1.0",
			InstallTrigger: "install-cursor-next",
		},
	}
	plan, err := BuildAppReconcilePlan(live, AppDesired{Name: "cursor", Version: "1.0", Kind: "install"})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Noop {
		t.Fatalf("expected noop for same version, plan=%+v", plan)
	}
}

func TestRenderAppImportScript(t *testing.T) {
	body, err := RenderAppImportScript(AppDesired{
		Name: "santa", Kind: "cache", Version: "2026.2",
		InstallTrigger: "install-santa-next",
		CacheTrigger:   "cache-santa-2026.2",
		PackagePattern: "*santa*2026.2*",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"#!/usr/bin/env buddy",
		`NAME             = "santa"`,
		`VERSION          = "2026.2"`,
		`CACHE_TRIGGER    = "cache-santa-2026.2"`,
		"app_reconcile",
		"apply = True",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("script missing %q\n%s", want, body)
		}
	}
}
