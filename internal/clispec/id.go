// Package clispec is the single source for CLI command/flag identifiers and
// user-facing message strings. Identifiers are never translated. Messages go
// through T and default to English (see catalog.go).
package clispec

import "strings"

// Top-level commands (never translated).
const (
	CmdHelp        = "help"
	CmdVersion     = "version"
	CmdAuth        = "auth"
	CmdCred        = "cred"
	CmdPolicy      = "policy"
	CmdCategory    = "category"
	CmdMacApp      = "macapp"
	CmdApp         = "app"
	CmdGroup       = "group"
	CmdProfile     = "profile"
	CmdPackage     = "package"
	CmdComputer    = "computer"
	CmdRole        = "role"
	CmdClient      = "client"
	CmdScript      = "script"
	CmdMdm         = "mdm"
	CmdBlueprint   = "blueprint"
	CmdDevice      = "device"
	CmdDeviceGroup = "device-group"
	CmdDDM         = "ddm"
	CmdBenchmark   = "benchmark"
	CmdSetup       = "setup"
	CmdServe       = "serve"
	CmdMCP         = "mcp"
	CmdLSP         = "lsp"
	CmdShell       = "shell"
	TopicStarlark  = "starlark"
	TopicBuiltins  = "builtins"
	TopicOverview  = "overview"
)

// Shared subcommands (never translated).
const (
	SubList   = "list"
	SubGet    = "get"
	SubSet    = "set"
	SubCreate = "create"
	SubDelete = "delete"
	SubHelp   = "help"
	SubClone  = "clone"
	SubFind   = "find"
	SubUpdate = "update"
	SubUpload = "upload"
	SubStatus = "status"
	SubAdd    = "add"
	SubTest   = "test"
	SubCaps   = "caps"
)

// Flag names without leading dashes (never translated).
const (
	FlagFilter      = "filter"
	FlagFile        = "file"
	FlagName        = "name"
	FlagCategory    = "category"
	FlagTrigger     = "trigger"
	FlagGroupIDs    = "group-ids"
	FlagTenant      = "tenant"
	FlagUser        = "user"
	FlagAuth        = "auth"
	FlagEval        = "eval"
	FlagVerbose     = "verbose"
	FlagTrace       = "trace"
	FlagScriptTrace = "script-trace"
	FlagVersion     = "version"
)

// Short flags (never translated).
const (
	ShortEval        = "e"
	ShortVerbose     = "v"
	ShortTrace       = "t"
	ShortSkipConfirm = "y"
)

// Flag returns the long form (--name).
func Flag(name string) string {
	return "--" + name
}

// FlagPrefix returns --name= for HasPrefix / TrimPrefix parsing.
func FlagPrefix(name string) string {
	return "--" + name + "="
}

// FlagEq returns --name=<placeholder> for usage/error text.
func FlagEq(name, placeholder string) string {
	return "--" + name + "=<" + placeholder + ">"
}

// Short returns the short form (-x).
func Short(name string) string {
	return "-" + name
}

// HelpPath joins command path segments with "." (helpMap keys).
func HelpPath(parts ...string) string {
	return strings.Join(parts, ".")
}

// CatalogID joins command path segments with "-" (CommandCatalog / docs anchors).
func CatalogID(parts ...string) string {
	return strings.Join(parts, "-")
}

// HelpTitle joins command path segments with spaces (CommandCatalog titles).
func HelpTitle(parts ...string) string {
	return strings.Join(parts, " ")
}

// UsageHelp returns "buddy <parts...> help" for missingArg / error hints.
func UsageHelp(parts ...string) string {
	if len(parts) == 0 {
		return "buddy help"
	}
	return "buddy " + strings.Join(parts, " ") + " help"
}
