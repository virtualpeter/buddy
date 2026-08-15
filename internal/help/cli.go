package help

import (
	"strings"
)

// CommandHelp is one CLI command or subcommand help block.
type CommandHelp struct {
	ID    string // anchor, e.g. "policy", "policy-list", "device-group"
	Title string // heading, e.g. "policy", "policy list"
	Body  string
	Level int // 2 = command, 3 = subcommand
}

// CommandCatalog is the ordered CLI help set shown by `buddy help` / Command index.
func CommandCatalog() []CommandHelp {
	return []CommandHelp{
		{ID: "overview", Title: "Overview", Body: Root, Level: 2},
		{ID: "auth", Title: "auth", Body: Auth, Level: 2},
		{ID: "cred", Title: "cred", Body: Cred, Level: 2},
		{ID: "policy", Title: "policy", Body: PoliciesCLIHelp, Level: 2},
		{ID: "policy-list", Title: "policy list", Body: PolicyList, Level: 3},
		{ID: "policy-get", Title: "policy get", Body: PolicyGet, Level: 3},
		{ID: "policy-set", Title: "policy set", Body: PolicySet, Level: 3},
		{ID: "policy-create", Title: "policy create", Body: PolicyCreate, Level: 3},
		{ID: "policy-delete", Title: "policy delete", Body: PolicyDelete, Level: 3},
		{ID: "category", Title: "category", Body: CategoriesCLIHelp, Level: 2},
		{ID: "macapp", Title: "macapp", Body: MacappsCLIHelp, Level: 2},
		{ID: "app", Title: "app", Body: AppsCLIHelp, Level: 2},
		{ID: "group", Title: "group", Body: GroupsCLIHelp, Level: 2},
		{ID: "profile", Title: "profile", Body: ProfilesCLIHelp, Level: 2},
		{ID: "package", Title: "package", Body: PackagesCLIHelp, Level: 2},
		{ID: "computer", Title: "computer", Body: ComputersCLIHelp, Level: 2},
		{ID: "mdm", Title: "mdm", Body: MdmCLIHelp, Level: 2},
		{ID: "blueprint", Title: "blueprint", Body: BlueprintsCLIHelp, Level: 2},
		{ID: "device", Title: "device", Body: DevicesCLIHelp, Level: 2},
		{ID: "device-group", Title: "device-group", Body: DeviceGroupsCLIHelp, Level: 2},
		{ID: "ddm", Title: "ddm", Body: DdmCLIHelp, Level: 2},
		{ID: "benchmark", Title: "benchmark", Body: BenchmarksCLIHelp, Level: 2},
		{ID: "role", Title: "role", Body: RolesCLIHelp, Level: 2},
		{ID: "client", Title: "client", Body: ClientsCLIHelp, Level: 2},
		{ID: "script", Title: "script", Body: ScriptsCLIHelp, Level: 2},
		{ID: "script-get", Title: "script get", Body: ScriptGet, Level: 3},
		{ID: "script-set", Title: "script set", Body: ScriptSet, Level: 3},
		{ID: "setup", Title: "setup", Body: Setup, Level: 2},
		{ID: "serve", Title: "serve", Body: Serve, Level: 2},
		{ID: "mcp", Title: "mcp", Body: MCP, Level: 2},
		{ID: "lsp", Title: "lsp", Body: LSP, Level: 2},
		{ID: "version", Title: "version", Body: Version, Level: 2},
		{ID: "shell", Title: "shell", Body: Shell, Level: 2},
	}
}

// CLIMarkdown converts CommandCatalog into structured Markdown for the docs UI
// (same text as `buddy help` / `buddy help <command>`, with per-command anchors).
func CLIMarkdown() string {
	var b strings.Builder
	b.WriteString("# Command index\n\n")
	b.WriteString("Canonical reference for buddy’s CLI commands ")
	b.WriteString("(same text as `buddy help` and `buddy help <command>`, rendered for browsing).\n\n")
	for _, c := range CommandCatalog() {
		tag := "h2"
		if c.Level == 3 {
			tag = "h3"
		}
		b.WriteString("<")
		b.WriteString(tag)
		b.WriteString(" id=\"")
		b.WriteString(c.ID)
		b.WriteString("\"><code>")
		b.WriteString(htmlEscapeText(c.Title))
		b.WriteString("</code></")
		b.WriteString(tag)
		b.WriteString(">\n\n```\n")
		b.WriteString(strings.TrimSpace(c.Body))
		b.WriteString("\n```\n\n")
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
