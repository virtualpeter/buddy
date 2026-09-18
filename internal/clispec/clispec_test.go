package clispec

import (
	"strings"
	"testing"
)

func TestFlagHelpers(t *testing.T) {
	if got := Flag(FlagFilter); got != "--filter" {
		t.Fatalf("Flag = %q", got)
	}
	if got := FlagPrefix(FlagFilter); got != "--filter=" {
		t.Fatalf("FlagPrefix = %q", got)
	}
	if got := FlagEq(FlagFilter, "rsql"); got != "--filter=<rsql>" {
		t.Fatalf("FlagEq = %q", got)
	}
	if got := Short(ShortEval); got != "-e" {
		t.Fatalf("Short = %q", got)
	}
	if got := HelpPath(CmdPolicy, SubList); got != "policy.list" {
		t.Fatalf("HelpPath = %q", got)
	}
	if got := CatalogID(CmdPolicy, SubList); got != "policy-list" {
		t.Fatalf("CatalogID = %q", got)
	}
	if got := UsageHelp(CmdPolicy); got != "buddy policy help" {
		t.Fatalf("UsageHelp = %q", got)
	}
	if got := UsageHelp(CmdPolicy, SubGet); got != "buddy policy get help" {
		t.Fatalf("UsageHelp nested = %q", got)
	}
}

func TestTEnglishAndFallback(t *testing.T) {
	t.Setenv("BUDDY_LANG", "")
	t.Setenv("LANG", "C")
	if got := T(MsgMissingArg, "policy-id", "buddy policy help"); got != "missing policy-id; run 'buddy policy help' for usage" {
		t.Fatalf("T missing_arg = %q", got)
	}
	t.Setenv("BUDDY_LANG", "fr")
	if got := T(MsgRunHelpList); got != en[MsgRunHelpList] {
		t.Fatalf("fr fallback = %q", got)
	}
	t.Setenv("BUDDY_LANG", "en-US")
	if got := ResolveLocale(); got != "en" {
		t.Fatalf("ResolveLocale en-US = %q", got)
	}
}

func TestParseLang(t *testing.T) {
	cases := map[string]string{
		"en_US.UTF-8": "en",
		"fr_FR":       "fr",
		"de":          "de",
		"C":           "en",
		"POSIX":       "en",
		"":            "en",
	}
	for in, want := range cases {
		if got := parseLang(in); got != want {
			t.Errorf("parseLang(%q) = %q want %q", in, got, want)
		}
	}
}

func TestMissingKeyPanicsInTest(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for missing key")
		}
	}()
	_ = T("cli.no_such_key")
}

func TestEnglishCatalogComplete(t *testing.T) {
	ids := []string{
		MsgMissingArg, MsgMissingArgHint, MsgUnknownCommand, MsgUnknownSubcommand,
		MsgAmbiguousCommand, MsgAmbiguousSubcommand,
		MsgDidYouMean, MsgUseCandidates, MsgRunHelpList, MsgNoHelp,
		MsgCmdAuth, MsgCmdToken, MsgCmdCred, MsgCmdAgent, MsgCmdPolicy, MsgCmdCategory, MsgCmdMacApp, MsgCmdApp,
		MsgCmdGroup, MsgCmdProfile, MsgCmdPackage, MsgCmdIcon, MsgCmdComputer, MsgCmdComputerSearch, MsgCmdMdm,
		MsgCmdMobile, MsgCmdMobileSearch, MsgCmdMobileGroup, MsgCmdMobileProf,
		MsgCmdBlueprint, MsgCmdDevice, MsgCmdDeviceGroup, MsgCmdDDM, MsgCmdBenchmark,
		MsgCmdLicense, MsgCmdRole, MsgCmdClient, MsgCmdScript, MsgCmdExtensionAttribute, MsgCmdMobileExtAttr,
		MsgCmdSetup, MsgCmdServe, MsgCmdMCP,
		MsgCmdLSP, MsgCmdVersion,
		MsgGFlagVersion, MsgGFlagEval, MsgGFlagSkipConfirm, MsgGFlagVerbose,
		MsgGFlagTrace, MsgGFlagScriptTrace, MsgGFlagContext, MsgGFlagTenant, MsgGFlagUser, MsgGFlagAuth,
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if _, ok := en[id]; !ok {
			t.Errorf("en missing %s", id)
		}
		seen[id] = true
	}
	for id := range en {
		if !seen[id] {
			t.Errorf("en has extra key %s", id)
		}
	}
}

func TestDispatchNamesIncludeHelpAndRoot(t *testing.T) {
	names := DispatchNames()
	found := map[string]bool{}
	for _, n := range names {
		if found[n] {
			t.Fatalf("duplicate dispatch name %q", n)
		}
		found[n] = true
	}
	if !found[CmdHelp] {
		t.Fatal("dispatch missing help")
	}
	for _, c := range RootCommands() {
		if !found[c.ID] {
			t.Errorf("dispatch missing %s", c.ID)
		}
		if strings.TrimSpace(T(c.Summary)) == "" {
			t.Errorf("empty summary for %s", c.ID)
		}
	}
}

func TestCommandAliases(t *testing.T) {
	a := CommandAliases()
	if a["policies"] != CmdPolicy || a["blueprints"] != CmdBlueprint {
		t.Fatalf("aliases = %v", a)
	}
	if a["licenses"] != CmdLicense {
		t.Fatalf("licenses alias = %v", a)
	}
	if a["ea"] != CmdExtensionAttribute || a["eas"] != CmdExtensionAttribute || a["extension-attributes"] != CmdExtensionAttribute {
		t.Fatalf("ea aliases = %v", a)
	}
	if a["mobile-ea"] != CmdMobileExtAttr {
		t.Fatalf("mobile-ea alias = %v", a)
	}
	if a["acs"] != CmdComputerSearch || a["computer-searches"] != CmdComputerSearch {
		t.Fatalf("computer-search aliases = %v", a)
	}
	if a["mobile-acs"] != CmdMobileSearch || a["mobile-searches"] != CmdMobileSearch {
		t.Fatalf("mobile-search aliases = %v", a)
	}
}

func TestCanonicalCommand(t *testing.T) {
	cases := map[string]string{
		"ea":                   CmdExtensionAttribute,
		"eas":                  CmdExtensionAttribute,
		"EA":                   CmdExtensionAttribute,
		"extension-attributes": CmdExtensionAttribute,
		"extension-attribute":  CmdExtensionAttribute,
		"  ea  ":               CmdExtensionAttribute,
		"policies":             CmdPolicy,
		"policy":               CmdPolicy,
		"mobile-ea":            CmdMobileExtAttr,
		"mobile-eas":           CmdMobileExtAttr,
		"acs":                  CmdComputerSearch,
		"computer-searches":    CmdComputerSearch,
		"mobile-acs":           CmdMobileSearch,
		"mobile-searches":      CmdMobileSearch,
		"xyzzy":                "xyzzy",
		"":                     "",
	}
	for in, want := range cases {
		if got := CanonicalCommand(in); got != want {
			t.Errorf("CanonicalCommand(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMatchToken(t *testing.T) {
	cands := []string{"list", "get", "logs", "help"}
	if got, amb := MatchToken("li", cands); got != "list" || amb != nil {
		t.Fatalf("li → %q %v want list", got, amb)
	}
	if got, amb := MatchToken("l", cands); got != "" || len(amb) != 2 {
		t.Fatalf("l → %q %v want ambiguous list,logs", got, amb)
	}
	if got, amb := MatchToken("LIST", cands); got != "list" || amb != nil {
		t.Fatalf("LIST exact → %q %v", got, amb)
	}
	if got, amb := MatchToken("xyzzy", cands); got != "" || amb != nil {
		t.Fatalf("unknown → %q %v", got, amb)
	}
}

func TestResolveCommand(t *testing.T) {
	id, amb, ok := ResolveCommand("ico")
	if !ok || id != CmdIcon || amb != nil {
		t.Fatalf("ico → %q %v ok=%v want icon", id, amb, ok)
	}
	id, amb, ok = ResolveCommand("pol")
	if !ok || id != CmdPolicy {
		t.Fatalf("pol → %q %v ok=%v want policy", id, amb, ok)
	}
	id, amb, ok = ResolveCommand("ea")
	if !ok || id != CmdExtensionAttribute {
		t.Fatalf("ea alias → %q ok=%v", id, ok)
	}
	id, amb, ok = ResolveCommand("c")
	if ok || id != "" || len(amb) < 2 {
		t.Fatalf("c → id=%q amb=%v ok=%v want ambiguous", id, amb, ok)
	}
	id, amb, ok = ResolveCommand("xyzzy")
	if ok || amb != nil || id != "xyzzy" {
		t.Fatalf("xyzzy → %q %v ok=%v", id, amb, ok)
	}
	id, amb, ok = ResolveCommand("icons")
	if !ok || id != CmdIcon {
		t.Fatalf("icons alias → %q ok=%v", id, ok)
	}
}

func TestCatalogEntriesStable(t *testing.T) {
	entries := CatalogEntries()
	if entries[0].ID != TopicOverview {
		t.Fatalf("first entry %q", entries[0].ID)
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if e.ID == "" || e.Title == "" || e.Level < 2 {
			t.Fatalf("bad entry %+v", e)
		}
		if seen[e.ID] {
			t.Fatalf("duplicate catalog id %q", e.ID)
		}
		seen[e.ID] = true
	}
	for _, id := range []string{CmdPolicy, CatalogID(CmdPolicy, SubList), CmdDeviceGroup, CmdShell} {
		if !seen[id] {
			t.Errorf("catalog missing %s", id)
		}
	}
}
