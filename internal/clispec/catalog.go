package clispec

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// Message IDs (stable keys; not English-as-id).
const (
	MsgMissingArg        = "cli.missing_arg"
	MsgMissingArgHint    = "cli.missing_arg_hint"
	MsgUnknownCommand    = "cli.unknown_command"
	MsgUnknownSubcommand = "cli.unknown_subcommand"
	MsgDidYouMean        = "cli.did_you_mean"
	MsgUseCandidates     = "cli.use_candidates"
	MsgRunHelpList       = "cli.run_help_list"
	MsgNoHelp            = "cli.no_help"

	MsgCmdAuth        = "cli.cmd.auth"
	MsgCmdCred        = "cli.cmd.cred"
	MsgCmdPolicy      = "cli.cmd.policy"
	MsgCmdCategory    = "cli.cmd.category"
	MsgCmdMacApp      = "cli.cmd.macapp"
	MsgCmdApp         = "cli.cmd.app"
	MsgCmdGroup       = "cli.cmd.group"
	MsgCmdProfile     = "cli.cmd.profile"
	MsgCmdPackage     = "cli.cmd.package"
	MsgCmdIcon        = "cli.cmd.icon"
	MsgCmdComputer    = "cli.cmd.computer"
	MsgCmdMdm         = "cli.cmd.mdm"
	MsgCmdBlueprint   = "cli.cmd.blueprint"
	MsgCmdDevice      = "cli.cmd.device"
	MsgCmdDeviceGroup = "cli.cmd.device_group"
	MsgCmdDDM         = "cli.cmd.ddm"
	MsgCmdBenchmark   = "cli.cmd.benchmark"
	MsgCmdRole        = "cli.cmd.role"
	MsgCmdClient      = "cli.cmd.client"
	MsgCmdScript      = "cli.cmd.script"
	MsgCmdSetup       = "cli.cmd.setup"
	MsgCmdServe       = "cli.cmd.serve"
	MsgCmdMCP         = "cli.cmd.mcp"
	MsgCmdLSP         = "cli.cmd.lsp"
	MsgCmdVersion     = "cli.cmd.version"

	MsgGFlagVersion     = "cli.gflag.version"
	MsgGFlagEval        = "cli.gflag.eval"
	MsgGFlagSkipConfirm = "cli.gflag.skip_confirm"
	MsgGFlagVerbose     = "cli.gflag.verbose"
	MsgGFlagTrace       = "cli.gflag.trace"
	MsgGFlagScriptTrace = "cli.gflag.script_trace"
	MsgGFlagTenant      = "cli.gflag.tenant"
	MsgGFlagUser        = "cli.gflag.user"
	MsgGFlagAuth        = "cli.gflag.auth"
)

// catalogs is locale → message ID → format string. Only English is shipped.
var catalogs = map[string]map[string]string{
	"en": en,
}

var en = map[string]string{
	MsgMissingArg:        "missing %s; run '%s' for usage",
	MsgMissingArgHint:    "missing %s; run '%s'",
	MsgUnknownCommand:    "Unknown command: %q",
	MsgUnknownSubcommand: "unknown %s subcommand: %q",
	MsgDidYouMean:        "\nDid you mean %q?",
	MsgUseCandidates:     " (use: %s)",
	MsgRunHelpList:       "Run 'buddy help' for a list of commands.",
	MsgNoHelp:            "No help available for %q\n\n",

	MsgCmdAuth:        "Test authentication for the active tenant/auth",
	MsgCmdCred:        "Manage tenants, auths, and secrets in the config file",
	MsgCmdPolicy:      "Manage Jamf policies (list/get/set/create/delete)",
	MsgCmdCategory:    "Manage categories (list/get/create/delete)",
	MsgCmdMacApp:      "Manage Classic Mac App Store apps (list/get/create/delete)",
	MsgCmdApp:         "Manage policy apps (list/get/upgrade)",
	MsgCmdGroup:       "Manage computer groups (list/get/create/delete; smart/static membership)",
	MsgCmdProfile:     "Manage configuration profiles (list/get/set/clone/redeploy)",
	MsgCmdPackage:     "Manage package records (list/get/download/upload/delete/set/history)",
	MsgCmdIcon:        "Manage Self Service icons (get/upload/check/download)",
	MsgCmdComputer:    "Manage computers and MDM commands (list/get/reboot/lock/erase/...)",
	MsgCmdMdm:         "Query MDM commands from Jamf Pro (list with RSQL filter)",
	MsgCmdBlueprint:   "Manage Platform API blueprints (list/get/create/set/delete/deploy/…)",
	MsgCmdDevice:      "Manage Platform API devices (inventory + check-in/restart/erase/…)",
	MsgCmdDeviceGroup: "Manage Platform API device groups (not Pro computer groups)",
	MsgCmdDDM:         "Platform DDM declaration reporting (read-only)",
	MsgCmdBenchmark:   "Platform compliance benchmarks (beta)",
	MsgCmdRole:        "Manage API roles (list/get/create/update/delete/privileges)",
	MsgCmdClient:      "Manage API integrations (list/get/create/update/delete/credentials/set)",
	MsgCmdScript:      "Manage Jamf scripts (list/get/create/upload/download/delete/set)",
	MsgCmdSetup:       "basic-setup; secrets; managed-plist; write-script; store-oauth",
	MsgCmdServe:       "Local docs + script IDE web UI",
	MsgCmdMCP:         "MCP stdio server for AI tools (setup discovers host configs)",
	MsgCmdLSP:         "Language server for buddyscript (editor integration)",
	MsgCmdVersion:     "Print version and exit (same as --version)",

	MsgGFlagVersion:     "Print version and exit",
	MsgGFlagEval:        "Run a buddyscript snippet and exit (see 'buddy help shell')",
	MsgGFlagSkipConfirm: "Skip confirmation prompts (buddy CLI and confirm() in scripts)",
	MsgGFlagVerbose:     "Verbose output",
	MsgGFlagTrace:       "Trace HTTP requests and responses to stderr (headers and bodies; Authorization/Cookie redacted)",
	MsgGFlagScriptTrace: "Trace buddyscript builtin calls and print() in scripts and the interactive shell",
	MsgGFlagTenant:      "Tenant from config (default: config.tenant); auto-picks auth",
	MsgGFlagUser:        "Pin an auth name (--auth alias)",
	MsgGFlagAuth:        "Pin an auth name (same as --user)",
}

// ResolveLocale returns the active language tag (primary subtag).
// BUDDY_LANG wins over LANG. Unknown or empty tags fall back to en.
func ResolveLocale() string {
	if v := strings.TrimSpace(os.Getenv("BUDDY_LANG")); v != "" {
		return parseLang(v)
	}
	if v := strings.TrimSpace(os.Getenv("LANG")); v != "" {
		return parseLang(v)
	}
	return "en"
}

func parseLang(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, ".@"); i >= 0 {
		s = s[:i]
	}
	s = strings.ReplaceAll(s, "_", "-")
	if s == "" || strings.EqualFold(s, "C") || strings.EqualFold(s, "POSIX") {
		return "en"
	}
	base := strings.ToLower(strings.Split(s, "-")[0])
	if base == "" {
		return "en"
	}
	return base
}

// T formats the message for id using the active locale, falling back to English.
// A missing key returns the id; under go test it panics so catalogs stay complete.
func T(id string, args ...any) string {
	loc := ResolveLocale()
	tmpl, ok := lookup(loc, id)
	if !ok {
		tmpl, ok = lookup("en", id)
	}
	if !ok {
		if testing.Testing() {
			panic("clispec: missing message " + id)
		}
		tmpl = id
	}
	if len(args) == 0 {
		return tmpl
	}
	return fmt.Sprintf(tmpl, args...)
}

func lookup(locale, id string) (string, bool) {
	m, ok := catalogs[locale]
	if !ok {
		return "", false
	}
	s, ok := m[id]
	return s, ok
}

// MessageIDs returns every English catalog key (for completeness tests).
func MessageIDs() []string {
	out := make([]string, 0, len(en))
	for id := range en {
		out = append(out, id)
	}
	return out
}
