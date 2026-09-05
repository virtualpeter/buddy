package help

import (
	"strings"

	"buddy/internal/clispec"
)

// CommandHelp is one CLI command or subcommand help block.
type CommandHelp struct {
	ID    string // anchor, e.g. "policy", "policy-list", "device-group"
	Title string // heading, e.g. "policy", "policy list"
	Body  string
	Level int // 2 = command, 3 = subcommand
}

func catalogBodies() map[string]string {
	return map[string]string{
		clispec.TopicOverview: Root,
		clispec.CmdAuth:       Auth,
		clispec.CmdToken:      Token,
		clispec.CmdCred:       Cred,
		clispec.CmdPolicy:     PoliciesCLIHelp,
		clispec.CatalogID(clispec.CmdPolicy, clispec.SubList):   PolicyList,
		clispec.CatalogID(clispec.CmdPolicy, clispec.SubGet):    PolicyGet,
		clispec.CatalogID(clispec.CmdPolicy, clispec.SubSet):    PolicySet,
		clispec.CatalogID(clispec.CmdPolicy, clispec.SubCreate): PolicyCreate,
		clispec.CatalogID(clispec.CmdPolicy, clispec.SubDelete): PolicyDelete,
		clispec.CatalogID(clispec.CmdPolicy, clispec.SubLogs):   PolicyLogs,
		clispec.CmdCategory:    CategoriesCLIHelp,
		clispec.CmdMacApp:      MacappsCLIHelp,
		clispec.CmdApp:         AppsCLIHelp,
		clispec.CmdGroup:       GroupsCLIHelp,
		clispec.CmdProfile:     ProfilesCLIHelp,
		clispec.CmdPackage:     PackagesCLIHelp,
		clispec.CmdIcon:        IconsCLIHelp,
		clispec.CmdComputer:    ComputersCLIHelp,
		clispec.CmdMdm:         MdmCLIHelp,
		clispec.CmdMobile:      MobileDevicesCLIHelp,
		clispec.CmdMobileGroup: MobileGroupsCLIHelp,
		clispec.CmdMobileProf:  MobileProfilesCLIHelp,
		clispec.CmdBlueprint:   BlueprintsCLIHelp,
		clispec.CmdDevice:      DevicesCLIHelp,
		clispec.CmdDeviceGroup: DeviceGroupsCLIHelp,
		clispec.CmdDDM:         DdmCLIHelp,
		clispec.CmdBenchmark:   BenchmarksCLIHelp,
		clispec.CmdRole:        RolesCLIHelp,
		clispec.CmdClient:      ClientsCLIHelp,
		clispec.CmdScript:      ScriptsCLIHelp,
		clispec.CatalogID(clispec.CmdScript, clispec.SubGet):             ScriptGet,
		clispec.CatalogID(clispec.CmdScript, clispec.SubSet):             ScriptSet,
		clispec.CmdExtensionAttribute:                                    ExtensionAttributesCLIHelp,
		clispec.CatalogID(clispec.CmdExtensionAttribute, clispec.SubGet): ExtensionAttributeGet,
		clispec.CatalogID(clispec.CmdExtensionAttribute, clispec.SubSet): ExtensionAttributeSet,
		clispec.CmdMobileExtAttr:                                         MobileExtensionAttributesCLIHelp,
		clispec.CmdSetup:                                                 Setup,
		clispec.CmdServe:                                                 Serve,
		clispec.CmdMCP:                                                   MCP,
		clispec.CmdLSP:                                                   LSP,
		clispec.CmdVersion:                                               Version,
		clispec.CmdShell:                                                 Shell,
	}
}

// CommandCatalog is the ordered CLI help set shown by `buddy help` / Command index.
func CommandCatalog() []CommandHelp {
	bodies := catalogBodies()
	entries := clispec.CatalogEntries()
	out := make([]CommandHelp, 0, len(entries))
	for _, e := range entries {
		out = append(out, CommandHelp{ID: e.ID, Title: e.Title, Body: bodies[e.ID], Level: e.Level})
	}
	return out
}

// CLIMarkdown converts CommandCatalog into structured Markdown for the docs UI
// (same text as `buddy help` / `buddy help <command>`, with per-command anchors).
func CLIMarkdown() string {
	var b strings.Builder
	b.WriteString("# Command index\n\n")
	b.WriteString("Canonical reference for buddy’s CLI commands ")
	b.WriteString("(same text as `buddy help` and `buddy help <command>`, rendered for browsing).\n\n")
	cat := CommandCatalog()
	for i := 0; i < len(cat); {
		c := cat[i]
		j := i + 1
		if c.Level == 2 {
			for j < len(cat) && cat[j].Level == 3 {
				j++
			}
			if j > i+1 {
				b.WriteString(`<h2 id="section-`)
				b.WriteString(htmlEscapeText(c.ID))
				b.WriteString(`">`)
				b.WriteString(htmlEscapeText(c.Title))
				b.WriteString("</h2>\n\n")
			}
		}
		for _, item := range cat[i:j] {
			b.WriteString(renderCLIEntryHTML(item))
		}
		i = j
	}
	return b.String()
}

// CommandAnchorIDs returns the set of Command index fragment ids.
func CommandAnchorIDs() map[string]bool {
	cat := CommandCatalog()
	out := make(map[string]bool, len(cat))
	for _, c := range cat {
		out[c.ID] = true
	}
	return out
}
