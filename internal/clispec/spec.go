package clispec

import "strings"

// Command is a top-level CLI command. ID is never translated; Summary is a message ID.
type Command struct {
	ID      string
	Summary string
	Aliases []string
}

// CatalogEntry is one CommandCatalog / docs index row.
type CatalogEntry struct {
	ID    string
	Title string
	Level int
}

// GlobalFlag is a process-wide flag shown in Root help and parsed by extractGlobalFlags.
type GlobalFlag struct {
	Name    string // long name without dashes; empty if short-only
	Short   string // single letter; empty if long-only
	Arg     string // "<code>", "<name>", or empty
	Summary string // message ID
}

// RootCommands is the Commands: table in buddy help (excludes "help" itself).
func RootCommands() []Command {
	return []Command{
		{ID: CmdAuth, Summary: MsgCmdAuth, Aliases: []string{"auths"}},
		{ID: CmdToken, Summary: MsgCmdToken},
		{ID: CmdCred, Summary: MsgCmdCred, Aliases: []string{"creds", "credentials"}},
		{ID: CmdAgent, Summary: MsgCmdAgent},
		{ID: CmdPolicy, Summary: MsgCmdPolicy, Aliases: []string{"policies"}},
		{ID: CmdCategory, Summary: MsgCmdCategory, Aliases: []string{"categories"}},
		{ID: CmdMacApp, Summary: MsgCmdMacApp, Aliases: []string{"macapps"}},
		{ID: CmdApp, Summary: MsgCmdApp, Aliases: []string{"apps"}},
		{ID: CmdGroup, Summary: MsgCmdGroup, Aliases: []string{"groups"}},
		{ID: CmdProfile, Summary: MsgCmdProfile, Aliases: []string{"profiles"}},
		{ID: CmdPackage, Summary: MsgCmdPackage, Aliases: []string{"packages"}},
		{ID: CmdIcon, Summary: MsgCmdIcon, Aliases: []string{"icons"}},
		{ID: CmdComputer, Summary: MsgCmdComputer, Aliases: []string{"computers"}},
		{ID: CmdComputerSearch, Summary: MsgCmdComputerSearch, Aliases: []string{"computer-searches", "acs"}},
		{ID: CmdMdm, Summary: MsgCmdMdm},
		{ID: CmdMobile, Summary: MsgCmdMobile, Aliases: []string{"mobile-devices"}},
		{ID: CmdMobileSearch, Summary: MsgCmdMobileSearch, Aliases: []string{"mobile-searches", "mobile-acs"}},
		{ID: CmdMobileGroup, Summary: MsgCmdMobileGroup, Aliases: []string{"mobile-groups"}},
		{ID: CmdMobileProf, Summary: MsgCmdMobileProf, Aliases: []string{"mobile-profiles"}},
		{ID: CmdBlueprint, Summary: MsgCmdBlueprint, Aliases: []string{"blueprints"}},
		{ID: CmdDevice, Summary: MsgCmdDevice, Aliases: []string{"devices"}},
		{ID: CmdDeviceGroup, Summary: MsgCmdDeviceGroup, Aliases: []string{"device-groups"}},
		{ID: CmdDDM, Summary: MsgCmdDDM},
		{ID: CmdBenchmark, Summary: MsgCmdBenchmark, Aliases: []string{"benchmarks"}},
		{ID: CmdLicense, Summary: MsgCmdLicense, Aliases: []string{"licenses"}},
		{ID: CmdRole, Summary: MsgCmdRole, Aliases: []string{"roles"}},
		{ID: CmdClient, Summary: MsgCmdClient, Aliases: []string{"clients"}},
		{ID: CmdScript, Summary: MsgCmdScript, Aliases: []string{"scripts"}},
		{ID: CmdExtensionAttribute, Summary: MsgCmdExtensionAttribute, Aliases: []string{"ea", "eas", "extension-attributes"}},
		{ID: CmdMobileExtAttr, Summary: MsgCmdMobileExtAttr, Aliases: []string{"mobile-ea", "mobile-eas", "mobile-extension-attributes"}},
		{ID: CmdSetup, Summary: MsgCmdSetup},
		{ID: CmdServe, Summary: MsgCmdServe},
		{ID: CmdMCP, Summary: MsgCmdMCP},
		{ID: CmdLSP, Summary: MsgCmdLSP},
		{ID: CmdPublish, Summary: MsgCmdPublish},
		{ID: CmdVersion, Summary: MsgCmdVersion},
	}
}

// DispatchNames is every canonical token accepted as argv[0] (Root commands plus help).
// Documented aliases are resolved to these IDs by CanonicalCommand.
func DispatchNames() []string {
	out := make([]string, 0, len(RootCommands())+1)
	out = append(out, CmdHelp)
	for _, c := range RootCommands() {
		out = append(out, c.ID)
	}
	return out
}

// CommandAliases maps documented alias tokens (plurals and short names) to a dispatch name.
func CommandAliases() map[string]string {
	out := map[string]string{}
	for _, c := range RootCommands() {
		for _, a := range c.Aliases {
			out[a] = c.ID
		}
	}
	return out
}

// CanonicalCommand returns the dispatch ID for a command or documented alias
// (case-insensitive). Unknown tokens are returned unchanged.
func CanonicalCommand(name string) string {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		return name
	}
	for _, n := range DispatchNames() {
		if n == key {
			return n
		}
	}
	if id, ok := CommandAliases()[key]; ok {
		return id
	}
	return name
}

// GlobalFlags is the Global flags: table in buddy help, in display order.
func GlobalFlags() []GlobalFlag {
	return []GlobalFlag{
		{Name: FlagVersion, Summary: MsgGFlagVersion},
		{Name: FlagEval, Short: ShortEval, Arg: "<code>", Summary: MsgGFlagEval},
		{Short: ShortSkipConfirm, Summary: MsgGFlagSkipConfirm},
		{Name: FlagVerbose, Short: ShortVerbose, Summary: MsgGFlagVerbose},
		{Name: FlagTrace, Short: ShortTrace, Summary: MsgGFlagTrace},
		{Name: FlagScriptTrace, Summary: MsgGFlagScriptTrace},
		{Name: FlagContext, Arg: "<name>", Summary: MsgGFlagContext},
		{Name: FlagTenant, Arg: "<name>", Summary: MsgGFlagTenant},
		{Name: FlagUser, Arg: "<name>", Summary: MsgGFlagUser},
		{Name: FlagAuth, Arg: "<name>", Summary: MsgGFlagAuth},
	}
}

// CatalogEntries is the ordered Command index (IDs/titles/levels). Bodies stay in internal/help.
func CatalogEntries() []CatalogEntry {
	return []CatalogEntry{
		{ID: TopicOverview, Title: "Overview", Level: 2},
		{ID: CmdAuth, Title: CmdAuth, Level: 2},
		{ID: CmdToken, Title: CmdToken, Level: 2},
		{ID: CmdCred, Title: CmdCred, Level: 2},
		{ID: CmdAgent, Title: CmdAgent, Level: 2},
		{ID: CmdPolicy, Title: CmdPolicy, Level: 2},
		{ID: CatalogID(CmdPolicy, SubList), Title: HelpTitle(CmdPolicy, SubList), Level: 3},
		{ID: CatalogID(CmdPolicy, SubGet), Title: HelpTitle(CmdPolicy, SubGet), Level: 3},
		{ID: CatalogID(CmdPolicy, SubSet), Title: HelpTitle(CmdPolicy, SubSet), Level: 3},
		{ID: CatalogID(CmdPolicy, SubCreate), Title: HelpTitle(CmdPolicy, SubCreate), Level: 3},
		{ID: CatalogID(CmdPolicy, SubDelete), Title: HelpTitle(CmdPolicy, SubDelete), Level: 3},
		{ID: CatalogID(CmdPolicy, SubLogs), Title: HelpTitle(CmdPolicy, SubLogs), Level: 3},
		{ID: CmdCategory, Title: CmdCategory, Level: 2},
		{ID: CmdMacApp, Title: CmdMacApp, Level: 2},
		{ID: CmdApp, Title: CmdApp, Level: 2},
		{ID: CmdGroup, Title: CmdGroup, Level: 2},
		{ID: CmdProfile, Title: CmdProfile, Level: 2},
		{ID: CmdPackage, Title: CmdPackage, Level: 2},
		{ID: CmdIcon, Title: CmdIcon, Level: 2},
		{ID: CmdComputer, Title: CmdComputer, Level: 2},
		{ID: CmdComputerSearch, Title: CmdComputerSearch, Level: 2},
		{ID: CmdMdm, Title: CmdMdm, Level: 2},
		{ID: CmdMobile, Title: CmdMobile, Level: 2},
		{ID: CmdMobileSearch, Title: CmdMobileSearch, Level: 2},
		{ID: CmdMobileGroup, Title: CmdMobileGroup, Level: 2},
		{ID: CmdMobileProf, Title: CmdMobileProf, Level: 2},
		{ID: CmdBlueprint, Title: CmdBlueprint, Level: 2},
		{ID: CmdDevice, Title: CmdDevice, Level: 2},
		{ID: CmdDeviceGroup, Title: CmdDeviceGroup, Level: 2},
		{ID: CmdDDM, Title: CmdDDM, Level: 2},
		{ID: CmdBenchmark, Title: CmdBenchmark, Level: 2},
		{ID: CmdLicense, Title: CmdLicense, Level: 2},
		{ID: CmdRole, Title: CmdRole, Level: 2},
		{ID: CmdClient, Title: CmdClient, Level: 2},
		{ID: CmdScript, Title: CmdScript, Level: 2},
		{ID: CatalogID(CmdScript, SubGet), Title: HelpTitle(CmdScript, SubGet), Level: 3},
		{ID: CatalogID(CmdScript, SubSet), Title: HelpTitle(CmdScript, SubSet), Level: 3},
		{ID: CmdExtensionAttribute, Title: CmdExtensionAttribute, Level: 2},
		{ID: CatalogID(CmdExtensionAttribute, SubGet), Title: HelpTitle(CmdExtensionAttribute, SubGet), Level: 3},
		{ID: CatalogID(CmdExtensionAttribute, SubSet), Title: HelpTitle(CmdExtensionAttribute, SubSet), Level: 3},
		{ID: CmdMobileExtAttr, Title: CmdMobileExtAttr, Level: 2},
		{ID: CmdSetup, Title: CmdSetup, Level: 2},
		{ID: CmdServe, Title: CmdServe, Level: 2},
		{ID: CmdMCP, Title: CmdMCP, Level: 2},
		{ID: CmdLSP, Title: CmdLSP, Level: 2},
		{ID: CmdPublish, Title: CmdPublish, Level: 2},
		{ID: CmdVersion, Title: CmdVersion, Level: 2},
		{ID: CmdShell, Title: CmdShell, Level: 2},
	}
}
