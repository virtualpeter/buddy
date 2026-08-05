package buddy

import (
	"testing"
)

func TestLooksLikeInstallTrigger(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"install-chrome-next", true},
		{"install-chrome", true},
		{"vscode-install", true},
		{"vscode-install-next", true},
		{"cache-santa-2026.3", false},
		{"Update Inventory", false},
		{"", false},
	}
	for _, tt := range cases {
		if got := looksLikeInstallTrigger(tt.in); got != tt.want {
			t.Errorf("looksLikeInstallTrigger(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestLooksLikeCacheTrigger(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"cache-santa-2026.3", true},
		{"santa-cache", true},
		{"install-santa-next", false},
		{"", false},
	}
	for _, tt := range cases {
		if got := looksLikeCacheTrigger(tt.in); got != tt.want {
			t.Errorf("looksLikeCacheTrigger(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestInstallTriggerFromScripts(t *testing.T) {
	scripts := []policySubsetScript{{
		ID:         "12",
		Name:       "Software - Trigger Installation",
		Parameter4: "vscode-install",
	}}
	got := installTriggerFromScripts(scripts)
	if got != "vscode-install" {
		t.Fatalf("got %q, want vscode-install", got)
	}
}

func TestPolicyAppNameFromSS(t *testing.T) {
	ss := &resourcePolicy{}
	ss.General.Name = "Install Santa [SS]"
	ss.SelfService.SelfServiceDisplayName = "Install Santa"
	if got := policyAppNameFromSS(ss, ""); got != "santa" {
		t.Errorf("display-based name = %q, want santa", got)
	}
	if got := policyAppNameFromSS(ss, "install-santa-next"); got != "santa" {
		t.Errorf("trigger-based name = %q, want santa", got)
	}

	ss2 := &resourcePolicy{}
	ss2.General.Name = "[SS] Install VSCode - TROUBLESHOOTING"
	ss2.SelfService.SelfServiceDisplayName = ""
	if got := policyAppNameFromSS(ss2, "vscode-install"); got != "vscode" {
		t.Errorf("vscode name = %q, want vscode", got)
	}
}

func TestPolicyAppMatchesFilter(t *testing.T) {
	sum := PolicyAppSummary{Name: "santa", Kind: PolicyAppCache, Version: "2026.3", InstallTrigger: "install-santa-next"}
	if !policyAppMatchesFilter(sum, "santa") {
		t.Fatal("expected match on name")
	}
	if !policyAppMatchesFilter(sum, "cache") {
		t.Fatal("expected match on kind")
	}
	if policyAppMatchesFilter(sum, "chrome") {
		t.Fatal("expected no match")
	}
}

func TestClassifyPolicyAppHelpers(t *testing.T) {
	// Inventory-like: no packages, no install script → no install trigger extraction
	scripts := []policySubsetScript{}
	if got := installTriggerFromScripts(scripts); got != "" {
		t.Fatalf("empty scripts should yield no trigger, got %q", got)
	}
}
