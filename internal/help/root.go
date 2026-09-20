package help

import (
	"strings"

	"buddy/internal/clispec"
)

const rootPreamble = `buddy - Jamf Pro management CLI

Usage:
  buddy [global-flags]                              Interactive buddyscript shell
  buddy [global-flags] <script.buddy> [-- args...]  Run a buddyscript script
  buddy [global-flags] -e <code> [-- args...]       Eval a buddyscript one-liner
  buddy [global-flags] <command> [subcommand] [args...]
  buddy help [command] [subcommand]

Commands:
`

const rootFooter = `
First time? On macOS install the notarized package, open Buddy.app, and add a workplace in Agent Settings. Without an Agent, any Jamf command (or 'buddy setup' / 'buddy cred add') opens the CLI chooser. Then 'buddy auth test'. See 'buddy setup help'.
Use 'buddy help starlark' or 'buddy help builtins' for buddyscript reference.
Use 'buddy help <command>' or 'buddy <command> help' for command details.
Use 'buddy <command> <subcommand> help' for subcommand details.

Notes:
  Global flags may appear anywhere before -- (buddy policy list --context sandbox -v).
  Long flags accept a single dash (-context is --context). Short flags are not combinable (-vt is not -v -t).
  Command and subcommand names accept a unique prefix (buddy ico list).
`

// Root is buddy help with no arguments (also the Command index overview).
var Root = buildRoot()

func buildRoot() string {
	var b strings.Builder
	b.WriteString(rootPreamble)
	cmds := clispec.RootCommands()
	width := 0
	for _, c := range cmds {
		if n := len(c.ID); n > width {
			width = n
		}
	}
	for _, c := range cmds {
		b.WriteString("  ")
		b.WriteString(c.ID)
		b.WriteString(strings.Repeat(" ", width-len(c.ID)+1))
		b.WriteString(clispec.T(c.Summary))
		b.WriteByte('\n')
	}
	b.WriteString("\nGlobal flags:\n")
	for _, f := range clispec.GlobalFlags() {
		left := formatGlobalFlagLeft(f)
		b.WriteString("  ")
		b.WriteString(left)
		if pad := 20 - len(left); pad > 0 {
			b.WriteString(strings.Repeat(" ", pad))
		} else {
			b.WriteByte(' ')
		}
		b.WriteString(clispec.T(f.Summary))
		b.WriteByte('\n')
	}
	b.WriteString(rootFooter)
	return b.String()
}

func formatGlobalFlagLeft(f clispec.GlobalFlag) string {
	var b strings.Builder
	if f.Short != "" && f.Name != "" {
		b.WriteString(clispec.Short(f.Short))
		b.WriteString(", ")
		b.WriteString(clispec.Flag(f.Name))
	} else if f.Short != "" {
		b.WriteString(clispec.Short(f.Short))
	} else {
		b.WriteString(clispec.Flag(f.Name))
	}
	if f.Arg != "" {
		b.WriteByte(' ')
		b.WriteString(f.Arg)
	}
	return b.String()
}
