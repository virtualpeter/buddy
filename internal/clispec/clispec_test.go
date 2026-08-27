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
		MsgDidYouMean, MsgUseCandidates, MsgRunHelpList, MsgNoHelp,
		MsgCmdAuth, MsgCmdCred, MsgCmdPolicy, MsgCmdCategory, MsgCmdMacApp, MsgCmdApp,
		MsgCmdGroup, MsgCmdProfile, MsgCmdPackage, MsgCmdIcon, MsgCmdComputer, MsgCmdMdm,
		MsgCmdBlueprint, MsgCmdDevice, MsgCmdDeviceGroup, MsgCmdDDM, MsgCmdBenchmark,
		MsgCmdRole, MsgCmdClient, MsgCmdScript, MsgCmdSetup, MsgCmdServe, MsgCmdMCP,
		MsgCmdLSP, MsgCmdVersion,
		MsgGFlagVersion, MsgGFlagEval, MsgGFlagSkipConfirm, MsgGFlagVerbose,
		MsgGFlagTrace, MsgGFlagScriptTrace, MsgGFlagTenant, MsgGFlagUser, MsgGFlagAuth,
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
