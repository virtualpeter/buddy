package clispec

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// Message IDs (stable keys; not English-as-id).
const (
	MsgMissingArg          = "cli.missing_arg"
	MsgMissingArgHint      = "cli.missing_arg_hint"
	MsgUnknownCommand      = "cli.unknown_command"
	MsgUnknownSubcommand   = "cli.unknown_subcommand"
	MsgAmbiguousCommand    = "cli.ambiguous_command"
	MsgAmbiguousSubcommand = "cli.ambiguous_subcommand"
	MsgDidYouMean          = "cli.did_you_mean"
	MsgUseCandidates       = "cli.use_candidates"
	MsgRunHelpList         = "cli.run_help_list"
	MsgNoHelp              = "cli.no_help"

	MsgCmdAuth               = "cli.cmd.auth"
	MsgCmdToken              = "cli.cmd.token"
	MsgCmdCred               = "cli.cmd.cred"
	MsgCmdAgent              = "cli.cmd.agent"
	MsgCmdPolicy             = "cli.cmd.policy"
	MsgCmdCategory           = "cli.cmd.category"
	MsgCmdMacApp             = "cli.cmd.macapp"
	MsgCmdApp                = "cli.cmd.app"
	MsgCmdGroup              = "cli.cmd.group"
	MsgCmdProfile            = "cli.cmd.profile"
	MsgCmdPackage            = "cli.cmd.package"
	MsgCmdIcon               = "cli.cmd.icon"
	MsgCmdComputer           = "cli.cmd.computer"
	MsgCmdComputerSearch     = "cli.cmd.computer_search"
	MsgCmdMdm                = "cli.cmd.mdm"
	MsgCmdBlueprint          = "cli.cmd.blueprint"
	MsgCmdDevice             = "cli.cmd.device"
	MsgCmdDeviceGroup        = "cli.cmd.device_group"
	MsgCmdMobile             = "cli.cmd.mobile_device"
	MsgCmdMobileSearch       = "cli.cmd.mobile_search"
	MsgCmdMobileGroup        = "cli.cmd.mobile_group"
	MsgCmdMobileProf         = "cli.cmd.mobile_profile"
	MsgCmdDDM                = "cli.cmd.ddm"
	MsgCmdBenchmark          = "cli.cmd.benchmark"
	MsgCmdLicense            = "cli.cmd.license"
	MsgCmdRole               = "cli.cmd.role"
	MsgCmdClient             = "cli.cmd.client"
	MsgCmdScript             = "cli.cmd.script"
	MsgCmdExtensionAttribute = "cli.cmd.extension_attribute"
	MsgCmdMobileExtAttr      = "cli.cmd.mobile_extension_attribute"
	MsgCmdSetup              = "cli.cmd.setup"
	MsgCmdServe              = "cli.cmd.serve"
	MsgCmdMCP                = "cli.cmd.mcp"
	MsgCmdLSP                = "cli.cmd.lsp"
	MsgCmdVersion            = "cli.cmd.version"

	MsgGFlagVersion     = "cli.gflag.version"
	MsgGFlagEval        = "cli.gflag.eval"
	MsgGFlagSkipConfirm = "cli.gflag.skip_confirm"
	MsgGFlagVerbose     = "cli.gflag.verbose"
	MsgGFlagTrace       = "cli.gflag.trace"
	MsgGFlagScriptTrace = "cli.gflag.script_trace"
	MsgGFlagTenant      = "cli.gflag.tenant"
	MsgGFlagContext     = "cli.gflag.context"
	MsgGFlagUser        = "cli.gflag.user"
	MsgGFlagAuth        = "cli.gflag.auth"
)

// catalogs is locale → message ID → format string. Only English is shipped.
var catalogs = map[string]map[string]string{
	"en": en,
}

var en = map[string]string{
	MsgMissingArg:          "missing %s; run '%s' for usage",
	MsgMissingArgHint:      "missing %s; run '%s'",
	MsgUnknownCommand:      "Unknown command: %q",
	MsgUnknownSubcommand:   "unknown %s subcommand: %q",
	MsgAmbiguousCommand:    "Ambiguous command: %q (could be: %s)",
	MsgAmbiguousSubcommand: "ambiguous %s subcommand: %q (could be: %s)",
	MsgDidYouMean:          "\nDid you mean %q?",
	MsgUseCandidates:       " (use: %s)",
	MsgRunHelpList:         "Run 'buddy help' for a list of commands.",
	MsgNoHelp:              "No help available for %q\n\n",

	MsgCmdAuth:               "Test authentication for the active tenant/auth",
	MsgCmdToken:              "Mint a bearer token with one-shot flags (no config)",
	MsgCmdCred:               "Manage tenants, auths, and secrets in the config file",
	MsgCmdAgent:              "Inspect the macOS Buddy Agent (config, integrations, tokens)",
	MsgCmdPolicy:             "Manage Jamf policies (list/get/set/create/delete/logs)",
	MsgCmdCategory:           "Manage categories (list/get/create/delete)",
	MsgCmdMacApp:             "Manage Classic Mac App Store apps (list/get/create/delete)",
	MsgCmdApp:                "Manage policy apps (list/scan/get/import/cache/upgrade)",
	MsgCmdGroup:              "Manage computer groups (list/get/create/delete; smart/static membership)",
	MsgCmdProfile:            "Manage configuration profiles (list/get/set/create/delete/clone/redeploy)",
	MsgCmdPackage:            "Manage package records (list/get/download/upload/delete/set/history)",
	MsgCmdIcon:               "Manage Self Service icons (list/get/upload/check/download)",
	MsgCmdComputer:           "Manage computers and MDM commands (list/get/reboot/lock/erase/...)",
	MsgCmdComputerSearch:     "Manage Classic advanced computer searches (Jamf Reports)",
	MsgCmdMdm:                "Query MDM commands from Jamf Pro (list with RSQL filter)",
	MsgCmdBlueprint:          "Manage Platform API blueprints (list/get/create/set/delete/deploy/…)",
	MsgCmdDevice:             "Manage Platform API devices (inventory + check-in/restart/erase/…)",
	MsgCmdDeviceGroup:        "Manage Platform API device groups (not Pro computer groups)",
	MsgCmdMobile:             "Manage Jamf Pro mobile devices (list/get/update-inventory/restart/lock/erase)",
	MsgCmdMobileSearch:       "Manage advanced mobile-device searches (Jamf Reports)",
	MsgCmdMobileGroup:        "Manage mobile-device groups (smart criteria; static membership)",
	MsgCmdMobileProf:         "Manage mobile-device configuration profiles (list/get/put/create/delete)",
	MsgCmdDDM:                "Platform DDM declaration reporting (read-only)",
	MsgCmdBenchmark:          "Platform compliance benchmarks",
	MsgCmdLicense:            "List Jamf Account licenses (organization workplace)",
	MsgCmdRole:               "Manage API roles (list/get/create/update/delete/privileges)",
	MsgCmdClient:             "Manage API integrations (list/get/create/update/delete/credentials/set)",
	MsgCmdScript:             "Manage Jamf scripts (list/get/create/upload/download/delete/set)",
	MsgCmdExtensionAttribute: "Manage computer extension attributes (list/get/create/upload/download/delete/set)",
	MsgCmdMobileExtAttr:      "Manage mobile-device extension attributes (list/get/create/delete)",
	MsgCmdSetup:              "First-run chooser; import; export; secrets; managed-plist",
	MsgCmdServe:              "Local docs + config + script IDE web UI",
	MsgCmdMCP:                "MCP stdio server for AI tools (setup discovers host configs)",
	MsgCmdLSP:                "Language server for buddyscript (editor integration)",
	MsgCmdVersion:            "Print version and exit (same as --version)",

	MsgGFlagVersion:     "Print version and exit",
	MsgGFlagEval:        "Run a buddyscript snippet and exit (see 'buddy help shell')",
	MsgGFlagSkipConfirm: "Skip confirmation prompts (buddy CLI and confirm() in scripts)",
	MsgGFlagVerbose:     "Verbose output",
	MsgGFlagTrace:       "Trace HTTP requests and responses to stderr (headers and bodies; Authorization/Cookie and token-shaped values redacted)",
	MsgGFlagScriptTrace: "Trace buddyscript builtin calls and print() in scripts and the interactive shell",
	MsgGFlagTenant:      "Alias of --context (dual-read)",
	MsgGFlagContext:     "Context from config (auto-picks auth); --tenant is a dual-read alias",
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
