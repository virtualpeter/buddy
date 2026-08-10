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

	// SS slug is richer than install trigger (cursor-ide vs install-cursor-next).
	ss3 := &resourcePolicy{}
	ss3.General.Name = "Install Cursor IDE [SS]"
	ss3.SelfService.SelfServiceDisplayName = "Install Cursor IDE"
	if got := policyAppNameFromSS(ss3, "install-cursor-next"); got != "cursor-ide" {
		t.Errorf("cursor name = %q, want cursor-ide", got)
	}
	if got := policyAppNameFromSS(ss3, "cursor-install"); got != "cursor-ide" {
		t.Errorf("cursor reversed trigger name = %q, want cursor-ide", got)
	}
}

func TestChooseAppCanonicalName(t *testing.T) {
	cases := []struct {
		ss, trig, want string
	}{
		{"cursor-ide", "cursor", "cursor-ide"},
		{"cursor", "cursor-ide", "cursor-ide"},
		{"santa", "santa", "santa"},
		{"", "santa", "santa"},
		{"santa", "", "santa"},
		{"claude-cli", "claude", "claude-cli"},
	}
	for _, tt := range cases {
		if got := chooseAppCanonicalName(tt.ss, tt.trig); got != tt.want {
			t.Errorf("chooseAppCanonicalName(%q, %q) = %q, want %q", tt.ss, tt.trig, got, tt.want)
		}
	}
}

func TestAppSlugsCompatible(t *testing.T) {
	if !appSlugsCompatible("cursor-ide", "cursor") {
		t.Fatal("expected cursor-ide ~ cursor")
	}
	if !appSlugsCompatible("Cursor IDE", "cursor-ide") {
		t.Fatal("expected normalized equality")
	}
	if appSlugsCompatible("chrome", "cursor") {
		t.Fatal("chrome should not match cursor")
	}
}

func TestInstallTriggerCandidates(t *testing.T) {
	got := installTriggerCandidates("cursor-ide")
	wantSome := []string{"install-cursor-ide-next", "install-cursor", "cursor-install", "cursor-ide-install"}
	for _, w := range wantSome {
		found := false
		for _, g := range got {
			if g == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("installTriggerCandidates missing %q; got %v", w, got)
		}
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

func TestPolicyAppNameFromListName(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Install Santa [SS]", "santa"},
		{"[SS] Install VSCode - TROUBLESHOOTING", "vscode"},
		{"Update Inventory [SS]", "update-inventory"},
		{"Chrome [SS]", "chrome"},
		{"Install Claude CLI [SS]", "claude-cli"},
		{"Install Cursor IDE [SS]", "cursor-ide"},
		{"Install claude-cli [SS]", "claude-cli"},
	}
	for _, tt := range cases {
		if got := policyAppNameFromListName(tt.in); got != tt.want {
			t.Errorf("policyAppNameFromListName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestSummarizePolicyAppFromList(t *testing.T) {
	list := []policyListItem{
		{ID: 1, Name: "Install Santa [SS]"},
		{ID: 2, Name: "Cache Santa 2026.2"},
		{ID: 3, Name: "Install Santa Next"},
		{ID: 4, Name: "Install Foo [SS]"},
		{ID: 5, Name: "Install Foo"},
		{ID: 6, Name: "Install Chrome Unrelated"},
		{ID: 7, Name: "Update Inventory [SS]"},
	}
	santa, ok := summarizePolicyAppFromList(list[0], list)
	if !ok || santa.Name != "santa" {
		t.Fatalf("santa: ok=%v name=%q", ok, santa.Name)
	}
	if santa.Kind != PolicyAppCache {
		t.Errorf("santa kind = %q, want cache", santa.Kind)
	}
	if santa.Version != "2026.2" {
		t.Errorf("santa version = %q, want 2026.2", santa.Version)
	}
	if santa.CachePolicyID != 2 || santa.InstallPolicyID != 3 {
		t.Errorf("santa ids cache=%d install=%d", santa.CachePolicyID, santa.InstallPolicyID)
	}
	if santa.InstallTrigger != "install-santa-next" {
		t.Errorf("santa trigger = %q", santa.InstallTrigger)
	}

	foo, ok := summarizePolicyAppFromList(list[3], list)
	if !ok || foo.Kind != PolicyAppInstall {
		t.Fatalf("foo: ok=%v kind=%q", ok, foo.Kind)
	}

	inv, ok := summarizePolicyAppFromList(list[6], list)
	if ok {
		t.Fatalf("inventory stub should be excluded, got kind=%q name=%q", inv.Kind, inv.Name)
	}

	// Single Install-named SS without siblings still listed.
	chromeList := []policyListItem{{ID: 10, Name: "Install Chrome [SS]"}}
	chrome, ok := summarizePolicyAppFromList(chromeList[0], chromeList)
	if !ok || chrome.Kind != PolicyAppSingle || chrome.Name != "chrome" {
		t.Fatalf("chrome single: ok=%v kind=%q name=%q", ok, chrome.Kind, chrome.Name)
	}

	// Bare [SS] without Install prefix and without siblings → excluded.
	bare := []policyListItem{{ID: 11, Name: "VPN Config [SS]"}}
	if _, ok := summarizePolicyAppFromList(bare[0], bare); ok {
		t.Fatal("non-Install [SS] without siblings should be excluded")
	}

	// Hyphenated app slug matches spaced sibling names; prefer sibling with custom trigger.
	trig := "install-claude-cli-next"
	empty := ""
	claudeList := []policyListItem{
		{ID: 20, Name: "Install Claude CLI [SS]", TriggerManual: &empty},
		{ID: 21, Name: "Cache Claude CLI 1.2.3"},
		{ID: 22, Name: "Install Claude CLI", TriggerManual: &trig},
		{ID: 23, Name: "Install Claude CLI Copy", TriggerManual: &empty},
	}
	claude, ok := summarizePolicyAppFromList(claudeList[0], claudeList)
	if !ok || claude.Name != "claude-cli" {
		t.Fatalf("claude: ok=%v name=%q", ok, claude.Name)
	}
	if claude.Kind != PolicyAppCache {
		t.Errorf("claude kind = %q, want cache", claude.Kind)
	}
	if claude.InstallPolicyID != 22 {
		t.Errorf("claude install id = %d, want 22 (custom trigger preferred)", claude.InstallPolicyID)
	}
	if claude.InstallTrigger != "install-claude-cli-next" {
		t.Errorf("claude trigger = %q, want install-claude-cli-next", claude.InstallTrigger)
	}
}

func TestPolicyNameContainsAppSlug(t *testing.T) {
	if !policyNameContainsAppSlug("Cache Santa 2026.2", "santa") {
		t.Fatal("expected santa in Cache Santa")
	}
	if !policyNameContainsAppSlug("install-santa-next", "santa") {
		t.Fatal("expected santa in install-santa-next")
	}
	if policyNameContainsAppSlug("Install Maintain Tools", "ai") {
		t.Fatal("short slug ai must not match Maintain")
	}
	if policyNameContainsAppSlug("Install Foo", "santa") {
		t.Fatal("santa must not match Install Foo")
	}
	if !policyNameContainsAppSlug("Cache Claude CLI 1.0", "claude-cli") {
		t.Fatal("claude-cli must match Cache Claude CLI")
	}
	if !policyNameContainsAppSlug("install-cursor-ide-next", "cursor-ide") {
		t.Fatal("cursor-ide must match install-cursor-ide-next")
	}
	if !policyNameContainsAppSlug("Install Cursor-IDE Next", "cursor ide") {
		t.Fatal("spaces/hyphens equivalent")
	}
}

func TestPolicyAppMatchesFilterDashed(t *testing.T) {
	sum := PolicyAppSummary{Name: "claude-cli", Kind: PolicyAppCache, SSPolicyName: "Install Claude CLI [SS]"}
	if !policyAppMatchesFilter(sum, "claude-cli") {
		t.Fatal("exact dashed filter")
	}
	if !policyAppMatchesFilter(sum, "claude cli") {
		t.Fatal("spaced filter should match dashed name")
	}
	if !policyAppMatchesFilter(sum, "claude") {
		t.Fatal("prefix token filter")
	}
}

func TestPolicyListNameHasAppHint(t *testing.T) {
	if !policyListNameHasAppHint("Install Santa Next", "santa") {
		t.Fatal("expected santa match")
	}
	if policyListNameHasAppHint("Install Foo", "santa") {
		t.Fatal("should not match Install Foo for santa")
	}
	if policyListNameHasAppHint("Cache Chrome 1.0", "santa") {
		t.Fatal("should not match unrelated cache for santa")
	}
	if !policyListNameHasAppHint("anything", "") {
		t.Fatal("empty hint matches all")
	}
	if !policyListNameHasAppHint("Install Claude CLI Next", "claude-cli") {
		t.Fatal("dashed hint must match spaced policy name")
	}
}

func TestParseV1PolicyListResults(t *testing.T) {
	raw := []byte(`[
		{"id":"1","name":"Update Inventory","enabled":true,"categoryId":"-1","executionFrequency":"EXECUTION_FREQUENCY_ONCE_PER_DAY","triggerCheckin":true,"triggerManual":"","updateInventory":true},
		{"id":2,"name":"Install Santa [SS]","enabled":false,"categoryId":5}
	]`)
	items, err := parseV1PolicyListResults(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("len=%d", len(items))
	}
	if items[0].ID != 1 || items[0].Name != "Update Inventory" {
		t.Errorf("item0 = %+v", items[0])
	}
	if items[0].Enabled == nil || !*items[0].Enabled {
		t.Error("expected enabled=true")
	}
	if items[1].ID != 2 || items[1].CategoryID != "5" {
		t.Errorf("item1 = %+v", items[1])
	}
	if items[1].Enabled == nil || *items[1].Enabled {
		t.Error("expected enabled=false")
	}
}
