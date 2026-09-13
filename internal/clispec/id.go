// Package clispec is the single source for CLI command/flag identifiers and
// user-facing message strings. Identifiers are never translated. Messages go
// through T and default to English (see catalog.go).
package clispec

import "strings"

// Top-level commands (never translated).
const (
	CmdHelp               = "help"
	CmdVersion            = "version"
	CmdAuth               = "auth"
	CmdToken              = "token"
	CmdCred               = "cred"
	CmdPolicy             = "policy"
	CmdCategory           = "category"
	CmdMacApp             = "macapp"
	CmdApp                = "app"
	CmdGroup              = "group"
	CmdProfile            = "profile"
	CmdPackage            = "package"
	CmdComputer           = "computer"
	CmdComputerSearch     = "computer-search"
	CmdIcon               = "icon"
	CmdRole               = "role"
	CmdClient             = "client"
	CmdScript             = "script"
	CmdExtensionAttribute = "extension-attribute"
	CmdMobileExtAttr      = "mobile-extension-attribute"
	CmdMdm                = "mdm"
	CmdBlueprint          = "blueprint"
	CmdDevice             = "device"
	CmdDeviceGroup        = "device-group"
	CmdMobile             = "mobile-device"
	CmdMobileSearch       = "mobile-search"
	CmdMobileGroup        = "mobile-group"
	CmdMobileProf         = "mobile-profile"
	CmdDDM                = "ddm"
	CmdBenchmark          = "benchmark"
	CmdLicense            = "license"
	CmdSetup              = "setup"
	CmdServe              = "serve"
	CmdMCP                = "mcp"
	CmdLSP                = "lsp"
	CmdShell              = "shell"
	TopicStarlark         = "starlark"
	TopicBuiltins         = "builtins"
	TopicOverview         = "overview"
)

// Shared subcommands (never translated).
const (
	SubList          = "list"
	SubLogs          = "logs"
	SubGet           = "get"
	SubSet           = "set"
	SubShow          = "show"
	SubCreate        = "create"
	SubPut           = "put"
	SubDelete        = "delete"
	SubResults       = "results"
	SubHelp          = "help"
	SubClone         = "clone"
	SubFind          = "find"
	SubUpdate        = "update"
	SubUpload        = "upload"
	SubStatus        = "status"
	SubDrop          = "drop"
	SubAdd           = "add"
	SubTest          = "test"
	SubAuthorization = "authorization"
	SubDirect        = "direct"
	SubRefresh       = "refresh"
	SubCheck         = "check"
	SubDownload      = "download"
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
	FlagContext     = "context"
	FlagUser        = "user"
	FlagAuth        = "auth"
	FlagEval        = "eval"
	FlagVerbose     = "verbose"
	FlagTrace       = "trace"
	FlagScriptTrace = "script-trace"
	FlagVersion     = "version"
	FlagRefresh     = "refresh"
	FlagDropCache   = "dropcache"
	FlagJSON        = "json"
	FlagFields      = "fields"
	FlagJQ          = "jq"
	FlagCSV         = "csv"
	FlagRaw         = "raw"
	FlagFromJSON    = "fromjson"
	FlagFlatten     = "flatten"
	FlagID          = "id"
	FlagSort        = "sort"
	FlagSince       = "since"
	FlagUntil       = "until"
	FlagComputerIds = "computer-ids"
	FlagExcludeIDs  = "exclude-ids"
	FlagIDs         = "ids"
	FlagPageSize    = "page-size"
	FlagLimit       = "limit"
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
