package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"buddy/internal/buddy"
	"buddy/internal/clispec"
	"buddy/internal/help"
	"buddy/internal/script"
)

// Version is set at build time via -ldflags "-X main.Version=..."
var Version = "dev"

// globalCLI holds flags that may appear anywhere on the command line.
type globalCLI struct {
	showVersion bool
	skipConfirm bool
	verbose     bool
	trace       bool
	scriptTrace bool
	tenant      string // --tenant: default tenant for auto auth pick
	user        string // --user / --auth: pin a specific auth
	rejectCred  bool   // --cred seen (removed in v4)
	evalSource  string // -e / --eval snippet
	evalDup     bool   // second -e/--eval seen
	evalMissing bool   // -e/--eval without a following argument
}

func main() {
	globals, filtered := extractGlobalFlags(os.Args)
	os.Args = filtered

	if globals.rejectCred {
		fmt.Fprintln(os.Stderr, "buddy: --cred was removed; use --user (or --auth)")
		os.Exit(2)
	}

	if globals.showVersion || (len(os.Args) > 1 && os.Args[1] == "version") {
		fmt.Println(Version)
		os.Exit(0)
	}
	if globals.evalDup {
		fmt.Fprintln(os.Stderr, "buddy: only one -e / --eval is allowed")
		os.Exit(2)
	}
	if globals.evalMissing {
		fmt.Fprintln(os.Stderr, "buddy: -e / --eval requires a buddyscript snippet")
		os.Exit(2)
	}

	opts := buddy.Opts{
		SkipConfirm: globals.skipConfirm,
		Verbose:     globals.verbose,
		Trace:       globals.trace,
	}

	cfg, err := buddy.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}
	buddy.SetActiveConfig(cfg)

	args := os.Args[1:]
	if len(args) > 0 {
		id, amb, ok := clispec.ResolveCommand(args[0])
		if ok {
			args[0] = id
		} else if len(amb) > 0 {
			fmt.Fprintf(os.Stderr, "%s\n", clispec.T(clispec.MsgAmbiguousCommand, args[0], strings.Join(amb, ", ")))
			fmt.Fprintln(os.Stderr, clispec.T(clispec.MsgRunHelpList))
			os.Exit(1)
		}
	}
	channel := buddy.ChannelCLI
	if len(args) > 0 {
		switch args[0] {
		case clispec.CmdMCP:
			channel = buddy.ChannelMCP
		case clispec.CmdServe:
			// Notebook/IDE set channel per session; keep cli for process --tenant default.
			channel = buddy.ChannelCLI
		}
	}
	buddy.SetActiveChannel(channel)

	tenantName := cfg.DefaultTenant(channel)
	if globals.tenant != "" {
		tenantName = globals.tenant
	}
	authPin := ""
	if globals.user != "" {
		authPin = globals.user
	}
	if globals.tenant != "" && authPin != "" {
		if auth, err := cfg.ResolveAuth(authPin); err == nil && !auth.LinkedTo(tenantName) {
			fmt.Fprintf(os.Stderr, "buddy: auth %q is not linked to tenant %q (linked: %s)\n",
				authPin, tenantName, strings.Join(auth.Tenants, ", "))
			os.Exit(2)
		}
	}
	buddy.SetActiveSelection(tenantName, authPin)
	// credName is the auth pin passed into commands (empty → auto-pick for tenant).
	credName := authPin

	buddy.InitLogger()
	buddy.SetVerbose(globals.verbose || globals.trace)
	buddy.SetHTTPTrace(globals.trace)
	logger := buddy.GetLogger()

	if buddy.ConfigNeedsSetup(cfg) && buddy.StdinIsTerminal() && shouldOfferFirstTimeSetup(args) {
		if _, err := buddy.RunFirstTimeSetup(cfg, opts, logger); err != nil {
			logger.Error("First-time setup failed", "error", err)
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		buddy.SetActiveConfig(cfg)
		tenantName = cfg.DefaultTenant(channel)
		if globals.tenant != "" {
			tenantName = globals.tenant
		}
		buddy.SetActiveSelection(tenantName, authPin)
		credName = authPin
	}

	if globals.evalSource != "" {
		scriptArgs := script.ParseEvalArgs(args)
		if err := script.RunSource(credName, opts, globals.evalSource, scriptArgs, logger, globals.scriptTrace); err != nil {
			logger.Error("Eval failed", "error", err)
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		return
	}

	if len(args) < 1 {
		if err := script.REPL(credName, opts, logger, globals.scriptTrace); err != nil {
			logger.Error("REPL failed", "error", err)
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		return
	}

	if !isKnownCommand(args[0]) && isScriptPath(args[0]) {
		trace := globals.scriptTrace
		posTrace, scriptPath, scriptArgs := script.ParseScriptInvocation(args)
		if posTrace {
			trace = true
		}
		exitOnErr(logger, "Script failed", script.Run(credName, opts, scriptPath, scriptArgs, logger, trace))
		return
	}

	switch args[0] {
	case clispec.CmdHelp:
		if len(args) > 1 {
			printCommandHelp(args[1:])
		} else {
			printUsage()
		}
		os.Exit(0)
	case clispec.CmdVersion:
		// Also handled before config load; keep for completeness if dispatch is reached.
		fmt.Println(Version)
		os.Exit(0)
	case clispec.CmdAuth:
		exitOnErr(logger, "Auth command failed", runAuth(credName, opts, args[1:], logger))
	case clispec.CmdToken:
		exitOnErr(logger, "Token command failed", runToken(args[1:], logger))
	case clispec.CmdCred:
		exitOnErr(logger, "Cred command failed", runCred(cfg, opts, args[1:], logger))
	case clispec.CmdPolicy:
		exitOnErr(logger, "Policy command failed", runPolicy(credName, opts, args[1:], logger))
	case clispec.CmdCategory:
		exitOnErr(logger, "Category command failed", runCategory(credName, opts, args[1:], logger))
	case clispec.CmdMacApp:
		exitOnErr(logger, "Macapp command failed", runMacApp(credName, opts, args[1:], logger))
	case clispec.CmdApp:
		exitOnErr(logger, "App command failed", runApp(credName, opts, args[1:], logger))
	case clispec.CmdGroup:
		exitOnErr(logger, "Group command failed", runGroup(credName, opts, args[1:], logger))
	case clispec.CmdProfile:
		exitOnErr(logger, "Profile command failed", runProfile(credName, opts, args[1:], logger))
	case clispec.CmdPackage:
		exitOnErr(logger, "Package command failed", runPackage(credName, opts, args[1:], logger))
	case clispec.CmdIcon:
		exitOnErr(logger, "Icon command failed", runIcon(credName, opts, args[1:], logger))
	case clispec.CmdComputer:
		exitOnErr(logger, "Computer command failed", runComputer(credName, opts, args[1:], logger))
	case clispec.CmdComputerSearch:
		exitOnErr(logger, "Computer-search command failed", runComputerSearch(credName, opts, args[1:], logger))
	case clispec.CmdRole:
		exitOnErr(logger, "Role command failed", runRole(credName, opts, args[1:], logger))
	case clispec.CmdClient:
		exitOnErr(logger, "Client command failed", runClient(credName, opts, args[1:], logger))
	case clispec.CmdScript:
		exitOnErr(logger, "Script command failed", runJamfScript(credName, opts, args[1:], logger))
	case clispec.CmdExtensionAttribute:
		exitOnErr(logger, "Extension-attribute command failed", runExtensionAttribute(credName, opts, args[1:], logger))
	case clispec.CmdMobileExtAttr:
		exitOnErr(logger, "Mobile-extension-attribute command failed", runMobileExtensionAttribute(credName, opts, args[1:], logger))
	case clispec.CmdMdm:
		exitOnErr(logger, "MDM command failed", runMdm(credName, opts, args[1:], logger))
	case clispec.CmdMobile:
		exitOnErr(logger, "Mobile-device command failed", runMobileDevice(credName, opts, args[1:], logger))
	case clispec.CmdMobileSearch:
		exitOnErr(logger, "Mobile-search command failed", runMobileSearch(credName, opts, args[1:], logger))
	case clispec.CmdMobileGroup:
		exitOnErr(logger, "Mobile-group command failed", runMobileGroup(credName, opts, args[1:], logger))
	case clispec.CmdMobileProf:
		exitOnErr(logger, "Mobile-profile command failed", runMobileProfile(credName, opts, args[1:], logger))
	case clispec.CmdBlueprint:
		exitOnErr(logger, "Blueprint command failed", runBlueprint(credName, opts, args[1:], logger))
	case clispec.CmdDevice:
		exitOnErr(logger, "Device command failed", runDevice(credName, opts, args[1:], logger))
	case clispec.CmdDeviceGroup:
		exitOnErr(logger, "Device-group command failed", runDeviceGroup(credName, opts, args[1:], logger))
	case clispec.CmdDDM:
		exitOnErr(logger, "DDM command failed", runDDM(credName, opts, args[1:], logger))
	case clispec.CmdBenchmark:
		exitOnErr(logger, "Benchmark command failed", runBenchmark(credName, opts, args[1:], logger))
	case clispec.CmdLicense:
		exitOnErr(logger, "License command failed", runLicense(credName, opts, args[1:], logger))
	case clispec.CmdSetup:
		exitOnErr(logger, "Setup command failed", runSetup(cfg, opts, credName, args[1:], logger))
	case clispec.CmdServe:
		exitOnErr(logger, "Serve failed", runServe(credName, opts, args[1:], logger))
	case clispec.CmdMCP:
		exitOnErr(logger, "MCP failed", runMCP(credName, opts, args[1:], logger))
	case clispec.CmdLSP:
		exitOnErr(logger, "LSP failed", runLSP(args[1:], logger))
	default:
		fmt.Fprintf(os.Stderr, "%s", clispec.T(clispec.MsgUnknownCommand, args[0]))
		if s := suggestTopLevelCommand(args[0]); s != "" {
			fmt.Fprint(os.Stderr, clispec.T(clispec.MsgDidYouMean, s))
		}
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, clispec.T(clispec.MsgRunHelpList))
		os.Exit(1)
	}
}

// shouldOfferFirstTimeSetup is false for meta / setup / local-UI commands that
// must work without tenants (or that implement their own unconfigured mode).
func shouldOfferFirstTimeSetup(args []string) bool {
	if len(args) == 0 {
		return true // REPL
	}
	cmd := args[0]
	if !isKnownCommand(cmd) && isScriptPath(cmd) {
		return true
	}
	switch cmd {
	case clispec.CmdHelp, clispec.CmdVersion:
		return false
	case clispec.CmdSetup, clispec.CmdCred, clispec.CmdAuth, clispec.CmdToken:
		return false
	case clispec.CmdServe, clispec.CmdLSP:
		return false
	case clispec.CmdMCP:
		return false // MCP registers zero tools when unconfigured
	default:
		return true
	}
}

func runPolicy(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if err := resolveArg0("policy", args, []string{clispec.SubList, clispec.SubGet, clispec.SubSet, clispec.SubCreate, clispec.SubDelete, clispec.SubLogs, clispec.SubHelp}); err != nil {
		return err
	}
	if len(args) == 0 || args[0] == clispec.SubHelp {
		fmt.Print(helpPolicy)
		return nil
	}
	mode := args[0]
	rest := args[1:]

	// list is valid with no further arguments; handle it before the sub-help fallthrough.
	if mode == "list" {
		if len(rest) > 0 && rest[0] == clispec.SubHelp {
			fmt.Print(helpPolicyList)
			return nil
		}
		return runPolicyList(credName, opts, rest, log)
	}
	if mode == clispec.SubLogs {
		if len(rest) > 0 && rest[0] == clispec.SubHelp {
			fmt.Print(helpPolicyLogs)
			return nil
		}
		return runPolicyLogs(credName, rest, log)
	}

	// "buddy policy <mode> help" — print subcommand help
	if len(rest) == 0 || rest[0] == clispec.SubHelp {
		fmt.Print(policySubHelp(mode))
		return nil
	}

	subcommand := rest[0]
	args = rest[1:]

	switch mode {
	case clispec.SubGet:
		return runPolicyGet(credName, opts, rest, log)
	case clispec.SubSet:
		return runPolicySet(credName, opts, subcommand, args, log)
	case clispec.SubCreate:
		return runPolicyCreate(credName, opts, subcommand, args, log)
	case clispec.SubDelete:
		idStrs := append([]string{subcommand}, args...)
		var policyIDs []int
		for _, s := range idStrs {
			id, err := parseInt(s, "policy-id")
			if err != nil {
				return err
			}
			policyIDs = append(policyIDs, id)
		}
		return buddy.PolicyDelete(credName, opts, policyIDs, log)
	default:
		cands := []string{clispec.SubList, clispec.SubGet, clispec.SubSet, clispec.SubCreate, clispec.SubDelete, "logs"}
		msg := fmt.Sprintf("unknown policy subcommand: %q", mode)
		msg += formatDidYouMean(suggestClosest(mode, cands))
		msg += fmt.Sprintf(" (use: %s)", strings.Join(cands, ", "))
		return fmt.Errorf("%s", msg)
	}
}

func runPolicyList(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	var excludeIDs []int
	var keywords []string
	idsOnly := false
	asJSON := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if val, next, matched, err := takeFlagValue(args, i, clispec.FlagFilter); matched {
			if err != nil {
				return err
			}
			keywords = append(keywords, strings.Fields(val)...)
			i = next - 1
			continue
		}
		if val, next, matched, err := takeFlagValue(args, i, clispec.FlagExcludeIDs); matched {
			if err != nil {
				return err
			}
			for _, s := range strings.Split(val, ",") {
				s = strings.TrimSpace(s)
				if s == "" {
					continue
				}
				id, err := parseInt(s, "exclude-id")
				if err != nil {
					return err
				}
				excludeIDs = append(excludeIDs, id)
			}
			i = next - 1
			continue
		}
		switch {
		case a == clispec.Flag(clispec.FlagIDs):
			idsOnly = true
		case a == clispec.Flag(clispec.FlagJSON):
			asJSON = true
		default:
			return fmt.Errorf("unexpected argument %q; use %s (run '%s' for usage)", a, clispec.FlagEq(clispec.FlagFilter, "keyword"), clispec.UsageHelp(clispec.CmdPolicy, clispec.SubList))
		}
	}
	summaries, err := buddy.PolicyFindSummariesByKeywords(credName, opts, keywords, excludeIDs, log)
	if err != nil {
		return err
	}
	if idsOnly {
		for _, s := range summaries {
			fmt.Println(s.ID)
		}
		return nil
	}
	if asJSON {
		return printJSON(summaries)
	}
	fmt.Printf("%-8s  %s\n", "ID", "Name")
	fmt.Printf("%-8s  %s\n", "--------", "----------------------------------------")
	for _, s := range summaries {
		fmt.Printf("%-8d  %s\n", s.ID, s.Name)
	}
	return nil
}

func runPolicyLogs(credName string, args []string, log *slog.Logger) error {
	fs := flag.NewFlagSet("policy logs", flag.ContinueOnError)
	computer := fs.String("computer", "", "computer id")
	computerIDs := fs.String(clispec.FlagComputerIds, "", "comma-separated computer ids (caller-built candidate set)")
	filter := fs.String(clispec.FlagFilter, "", "computers() filter (bare term or RSQL); v4 inventory bound, not log date")
	lastContact := fs.String("last-contact", "", "ISO datetime; computers with general.lastContact>=value (inventory, not logs since)")
	since := fs.String(clispec.FlagSince, "", "keep log rows with date_completed_epoch >= value (ISO or epoch)")
	until := fs.String(clispec.FlagUntil, "", "keep log rows with date_completed_epoch <= value (ISO or epoch)")
	status := fs.String("status", "", "filter by log status (case-insensitive exact)")
	skip := fs.Int("skip", 0, "computers to skip after the candidate list is built")
	limit := fs.Int(clispec.FlagLimit, 0, "max computers to scan; with no other source uses Classic computers/subset/basic")
	asJSON := fs.Bool(clispec.FlagJSON, false, "Print flattened logs as JSON")
	flagArgs, positionals := staticGroupSplitFlags(args)
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if len(positionals) < 1 {
		return missingArg("policy-id", clispec.UsageHelp(clispec.CmdPolicy))
	}
	policyID, err := parseInt(positionals[0], "policy-id")
	if err != nil {
		return err
	}
	ids := parseCommaSeparatedIDs(*computerIDs)
	return buddy.PolicyLogsPrint(credName, buddy.PolicyLogsQuery{
		PolicyID:    policyID,
		ComputerID:  *computer,
		ComputerIDs: ids,
		Filter:      *filter,
		LastContact: *lastContact,
		Status:      *status,
		Since:       *since,
		Until:       *until,
		Skip:        *skip,
		Limit:       *limit,
	}, *asJSON, log)
}

func runPolicyGet(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if len(args) > 0 && args[0] == clispec.SubHelp {
		fmt.Print(helpPolicyGet)
		return nil
	}
	for _, a := range args {
		if a == clispec.Flag(clispec.FlagTrigger) || strings.HasPrefix(a, clispec.FlagPrefix(clispec.FlagTrigger)) {
			return fmt.Errorf("buddy policy get %s was removed; use 'buddy policy get id <trigger>'", clispec.Flag(clispec.FlagTrigger))
		}
	}
	if len(args) < 1 {
		return fmt.Errorf("missing arguments: use <policy-id>, or 'id <trigger>', 'enable <policy-id>', or 'xml <policy-id>'; run '%s'", clispec.UsageHelp(clispec.CmdPolicy, clispec.SubGet))
	}
	switch args[0] {
	case "id":
		if len(args) < 2 {
			return missingArg("trigger", clispec.UsageHelp(clispec.CmdPolicy, clispec.SubGet))
		}
		trigger := strings.TrimSpace(args[1])
		if trigger == "" {
			return missingArg("trigger", clispec.UsageHelp(clispec.CmdPolicy, clispec.SubGet))
		}
		if len(args) > 2 {
			return fmt.Errorf("unexpected arguments after trigger: %v", args[2:])
		}
		id, err := buddy.PolicyFind(credName, opts, trigger, log)
		if err != nil {
			return err
		}
		fmt.Println(id)
		return nil
	case "xml":
		if len(args) < 2 {
			return missingArg("policy-id", clispec.UsageHelp(clispec.CmdPolicy, clispec.SubGet))
		}
		policyID, err := parseInt(args[1], "policy-id")
		if err != nil {
			return err
		}
		return buddy.PolicyGetXML(credName, policyID, os.Stdout, log)
	case "enable":
		if len(args) < 2 {
			return missingArg("policy-id", clispec.UsageHelp(clispec.CmdPolicy, clispec.SubGet))
		}
		policyID, err := parseInt(args[1], "policy-id")
		if err != nil {
			return err
		}
		return buddy.PolicyGetEnable(credName, opts, policyID, log)
	default:
		policyID, err := parseInt(args[0], "policy-id")
		if err != nil {
			return fmt.Errorf("unknown policy get arguments: %v (use <policy-id>, id, enable, or xml)", args)
		}
		if len(args) > 1 {
			return fmt.Errorf("unexpected arguments after policy-id: %v", args[1:])
		}
		d, err := buddy.PolicyGetDetails(credName, policyID, log)
		if err != nil {
			return err
		}
		b, err := json.MarshalIndent(buddy.PolicyDetailsMap(d), "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}
}

func policySetRequireID(args []string, min int, hint string) (int, error) {
	if len(args) < min {
		return 0, missingArg(hint, clispec.UsageHelp(clispec.CmdPolicy, clispec.SubSet))
	}
	return parseInt(args[0], "policy-id")
}

func runPolicySet(credName string, opts buddy.Opts, subcommand string, args []string, log *slog.Logger) error {
	var err error
	subcommand, err = matchSubcommand("policy set", subcommand, []string{
		"ss", "scope", "enable", "disable", "trigger", "install", "name", "frequency", "icon", "package", "xml", clispec.SubHelp,
	})
	if err != nil {
		return err
	}
	if subcommand == clispec.SubHelp {
		fmt.Print(helpPolicySet)
		return nil
	}
	switch subcommand {
	case "ss":
		ssOpts, policyID, err := parsePolicySSArgs(args)
		if err != nil {
			return err
		}
		return buddy.PolicySSUpdate(credName, opts, policyID, ssOpts, log)

	case "enable", "disable":
		policyID, err := policySetRequireID(args, 1, "policy-id")
		if err != nil {
			return err
		}
		return buddy.PolicyEnable(credName, opts, policyID, subcommand == "enable", log)

	case "trigger":
		policyID, err := policySetRequireID(args, 2, "policy-id or trigger")
		if err != nil {
			return err
		}
		return buddy.PolicyTrigger(credName, opts, policyID, strings.TrimSpace(args[1]), log)

	case "install":
		paramIndices, filtered, err := parseParamArgs(args)
		if err != nil {
			return err
		}
		policyID, err := policySetRequireID(filtered, 2, "policy-id or version")
		if err != nil {
			return err
		}
		return buddy.PolicyInstall(credName, opts, policyID, strings.TrimSpace(filtered[1]), paramIndices, log)

	case "xml":
		policyID, err := policySetRequireID(args, 1, "policy-id")
		if err != nil {
			return err
		}
		return buddy.PolicySetXML(credName, opts, policyID, os.Stdin, log)

	case "scope":
		return policySetScope(credName, opts, args, log)

	case "name":
		policyID, err := policySetRequireID(args, 2, "policy-id or name")
		if err != nil {
			return err
		}
		return buddy.PolicySetName(credName, opts, policyID, strings.TrimSpace(strings.Join(args[1:], " ")), log)

	case "frequency":
		policyID, err := policySetRequireID(args, 2, "policy-id or frequency")
		if err != nil {
			return err
		}
		frequency := strings.TrimSpace(strings.Join(args[1:], " "))
		if frequency == "" {
			return missingArg("frequency", clispec.UsageHelp(clispec.CmdPolicy, clispec.SubSet))
		}
		return buddy.PolicySetFrequency(credName, opts, policyID, frequency, log)

	case "icon":
		policyID, err := policySetRequireID(args, 2, "policy-id or image-file")
		if err != nil {
			return err
		}
		return buddy.PolicySetIcon(credName, opts, policyID, args[1], log)

	case "package":
		return policySetPackage(credName, opts, args, log)

	default:
		return errUnknownSubcommand("policy set", subcommand, []string{
			"ss", "scope", "enable", "disable", "trigger", "install", "name", "frequency", "icon", "package", "xml",
		})
	}
}

func policySetScope(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	var groupIDsFlag string
	var filtered []string
	for _, a := range args {
		if strings.HasPrefix(a, clispec.FlagPrefix(clispec.FlagGroupIDs)) {
			groupIDsFlag = strings.TrimPrefix(a, clispec.FlagPrefix(clispec.FlagGroupIDs))
		} else {
			filtered = append(filtered, a)
		}
	}
	policyID, err := policySetRequireID(filtered, 2, "policy-id or scope")
	if err != nil {
		return err
	}
	if len(filtered) == 2 && strings.ToLower(strings.TrimSpace(filtered[1])) == "all" {
		return buddy.PolicyScopeAll(credName, opts, policyID, log)
	}
	var groupIDs []int
	if groupIDsFlag != "" {
		for _, s := range strings.Split(groupIDsFlag, ",") {
			s = strings.TrimSpace(s)
			id, err := parseInt(s, "group-id")
			if err != nil {
				return err
			}
			groupIDs = append(groupIDs, id)
		}
	}
	for _, a := range filtered[1:] {
		id, err := parseInt(a, "group-id")
		if err != nil {
			return err
		}
		groupIDs = append(groupIDs, id)
	}
	if len(groupIDs) == 0 {
		return fmt.Errorf("at least one computer group ID required (positional or %s)", clispec.FlagEq(clispec.FlagGroupIDs, "1,2,3"))
	}
	return buddy.PolicyScope(credName, opts, policyID, groupIDs, log)
}

func policySetPackage(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	var triggerFlag, packageFlag, nameFlag string
	var disableSource, keepSource bool
	var filtered []string
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, clispec.FlagPrefix(clispec.FlagTrigger)):
			triggerFlag = strings.TrimPrefix(a, clispec.FlagPrefix(clispec.FlagTrigger))
		case strings.HasPrefix(a, "--package="):
			packageFlag = strings.TrimPrefix(a, "--package=")
		case strings.HasPrefix(a, clispec.FlagPrefix(clispec.FlagName)):
			nameFlag = strings.TrimPrefix(a, clispec.FlagPrefix(clispec.FlagName))
		case a == "--disable-source":
			disableSource = true
		case a == "--keep-source":
			keepSource = true
		default:
			filtered = append(filtered, a)
		}
	}
	disableMode, err := parseDisableSourceFlags(disableSource, keepSource)
	if err != nil {
		return err
	}
	policyID := 0
	var version string
	if triggerFlag != "" {
		if len(filtered) < 1 {
			return missingArg("version", clispec.UsageHelp(clispec.CmdPolicy, clispec.SubSet))
		}
		version = strings.TrimSpace(filtered[0])
		if opts.Verbose {
			fmt.Fprintf(os.Stderr, "Using %s%s to resolve policy\n", clispec.FlagPrefix(clispec.FlagTrigger), triggerFlag)
		}
	} else {
		policyID, err = policySetRequireID(filtered, 2, "policy-id or version (or pass "+clispec.FlagEq(clispec.FlagTrigger, "trigger")+" and <version>)")
		if err != nil {
			return err
		}
		version = strings.TrimSpace(filtered[1])
	}
	name := strings.ToLower(strings.TrimSpace(nameFlag))
	return buddy.PolicyPackage(credName, opts, name, version, policyID, triggerFlag, packageFlag, disableMode, log)
}

func parseDisableSourceFlags(disableSource, keepSource bool) (buddy.DisableSourceMode, error) {
	if disableSource && keepSource {
		return buddy.DisableSourceAuto, fmt.Errorf("--disable-source and --keep-source are mutually exclusive")
	}
	if disableSource {
		return buddy.DisableSourceForce, nil
	}
	if keepSource {
		return buddy.DisableSourceKeep, nil
	}
	return buddy.DisableSourceAuto, nil
}

func runPolicyCreate(credName string, opts buddy.Opts, subcommand string, args []string, log *slog.Logger) error {
	var err error
	subcommand, err = matchSubcommand("policy create", subcommand, []string{"package", "ss", clispec.SubHelp})
	if err != nil {
		return err
	}
	if subcommand == clispec.SubHelp {
		fmt.Print(helpPolicyCreate)
		return nil
	}
	switch subcommand {
	case "package":
		var pkgActionFlag, scopeFlag, categoryFlag, packageFlag string
		var filtered []string
		for _, a := range args {
			if strings.HasPrefix(a, "--action=") {
				pkgActionFlag = strings.TrimPrefix(a, "--action=")
			} else if strings.HasPrefix(a, "--scope=") {
				scopeFlag = strings.TrimPrefix(a, "--scope=")
			} else if strings.HasPrefix(a, clispec.FlagPrefix(clispec.FlagCategory)) {
				categoryFlag = strings.TrimPrefix(a, clispec.FlagPrefix(clispec.FlagCategory))
			} else if strings.HasPrefix(a, "--package=") {
				packageFlag = strings.TrimPrefix(a, "--package=")
			} else {
				filtered = append(filtered, a)
			}
		}
		args = filtered
		if len(args) < 2 {
			return missingArg("name or version", clispec.UsageHelp(clispec.CmdPolicy, clispec.SubCreate))
		}
		name := strings.ToLower(strings.TrimSpace(args[0]))
		version := strings.TrimSpace(args[1])
		pkgAction := "install"
		if pkgActionFlag != "" {
			actionNorm := strings.ToLower(strings.TrimSpace(pkgActionFlag))
			if actionNorm != "install" && actionNorm != "cache" {
				return fmt.Errorf("--action must be install or cache, got %q", pkgActionFlag)
			}
			pkgAction = actionNorm
		}
		scopeAll := true
		var scopeGroupIDs []int
		if scopeFlag != "" {
			scopeNorm := strings.ToLower(strings.TrimSpace(scopeFlag))
			if scopeNorm == "all" {
				scopeAll = true
			} else {
				scopeAll = false
				for _, s := range strings.Split(scopeFlag, ",") {
					s = strings.TrimSpace(s)
					id, err := parseInt(s, "group-id")
					if err != nil {
						return err
					}
					scopeGroupIDs = append(scopeGroupIDs, id)
				}
			}
		}
		_, err := buddy.PolicyCreatePackage(credName, opts, name, version, pkgAction, scopeAll, scopeGroupIDs, categoryFlag, packageFlag, log)
		return err
	case "ss":
		var categoryFlag string
		scriptID := 0
		var filtered []string
		for _, a := range args {
			switch {
			case strings.HasPrefix(a, clispec.FlagPrefix(clispec.FlagCategory)):
				categoryFlag = strings.TrimPrefix(a, clispec.FlagPrefix(clispec.FlagCategory))
			case strings.HasPrefix(a, "--script-id="):
				id, err := parseInt(strings.TrimPrefix(a, "--script-id="), "script-id")
				if err != nil {
					return err
				}
				scriptID = id
			default:
				filtered = append(filtered, a)
			}
		}
		args = filtered
		if len(args) < 2 {
			return missingArg("trigger or display-name", clispec.UsageHelp(clispec.CmdPolicy, clispec.SubCreate))
		}
		trigger := strings.TrimSpace(args[0])
		displayName := strings.TrimSpace(strings.Join(args[1:], " "))
		if displayName == "" {
			return missingArg("display-name", clispec.UsageHelp(clispec.CmdPolicy, clispec.SubCreate))
		}
		_, err := buddy.PolicyCreateSS(credName, opts, trigger, displayName, categoryFlag, 0, scriptID, log)
		return err
	default:
		return errUnknownSubcommand("policy create", subcommand, []string{"package", "ss"})
	}
}

func runIcon(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if err := resolveArg0("icon", args, []string{clispec.SubList, clispec.SubGet, clispec.SubUpload, clispec.SubCheck, clispec.SubDownload, clispec.SubHelp}); err != nil {
		return err
	}
	if len(args) == 0 || args[0] == clispec.SubHelp {
		fmt.Print(helpIcon)
		return nil
	}
	subcommand := args[0]
	args = args[1:]

	switch subcommand {
	case clispec.SubList:
		var filter string
		var refresh, dropCache bool
		fs := flag.NewFlagSet("icon list", flag.ContinueOnError)
		fs.StringVar(&filter, clispec.FlagFilter, "", "substring match on name or id")
		fs.BoolVar(&refresh, clispec.FlagRefresh, false, "probe 20 ids past max (skips cached ids)")
		fs.BoolVar(&dropCache, clispec.FlagDropCache, false, "delete the catalog cache and walk from id 1")
		if err := fs.Parse(args); err != nil {
			return err
		}
		res, err := buddy.IconList(credName, buddy.IconListOpts{Filter: filter, Refresh: refresh, DropCache: dropCache, Progress: true}, log)
		if err != nil {
			return err
		}
		fmt.Printf("%-8s  %-40s  %s\n", "ID", "Name", "URL")
		fmt.Printf("%-8s  %-40s  %s\n", "--------", "----------------------------------------", "----------------------------------------")
		for _, icon := range res.Icons {
			fmt.Printf("%-8d  %-40s  %s\n", icon.ID, icon.Name, icon.URL)
		}
		return nil
	case clispec.SubGet:
		if len(args) < 1 {
			return missingArg("icon-id", clispec.UsageHelp(clispec.CmdIcon))
		}
		id, err := strconv.Atoi(args[0])
		if err != nil || id <= 0 {
			return fmt.Errorf("icon-id must be a positive integer")
		}
		icon, err := buddy.IconGet(credName, id)
		if err != nil {
			return err
		}
		return printJSON(icon)
	case clispec.SubUpload:
		if len(args) < 1 {
			return missingArg("file", clispec.UsageHelp(clispec.CmdIcon))
		}
		filePath := args[0]
		if !opts.SkipConfirm {
			if !buddy.ConfirmPrompt(fmt.Sprintf("\nUpload icon %q?", filePath), log) {
				return nil
			}
		}
		icon, err := buddy.IconUpload(credName, filePath)
		if err != nil {
			return err
		}
		return printJSON(icon)
	case clispec.SubCheck:
		if len(args) < 1 {
			return missingArg("file", clispec.UsageHelp(clispec.CmdIcon))
		}
		iconID := 0
		if len(args) >= 2 {
			var err error
			iconID, err = strconv.Atoi(args[1])
			if err != nil || iconID <= 0 {
				return fmt.Errorf("icon-id must be a positive integer")
			}
		}
		res, err := buddy.IconCheckFile(credName, args[0], iconID)
		if err != nil {
			return err
		}
		return printJSON(res)
	case clispec.SubDownload:
		if len(args) < 1 {
			return missingArg("icon-id", clispec.UsageHelp(clispec.CmdIcon))
		}
		id, err := strconv.Atoi(args[0])
		if err != nil || id <= 0 {
			return fmt.Errorf("icon-id must be a positive integer")
		}
		rec, err := buddy.IconGet(credName, id)
		if err != nil {
			return err
		}
		data, err := buddy.IconFetch(credName, rec)
		if err != nil {
			return err
		}
		outPath := ""
		if len(args) >= 2 {
			outPath = args[1]
		} else {
			outPath = iconDownloadFileName(rec, id)
		}
		if err := os.WriteFile(outPath, data, 0o644); err != nil {
			return fmt.Errorf("write icon file: %w", err)
		}
		fmt.Printf("Wrote icon ID %d to %s (%d bytes)\n", id, outPath, len(data))
		return nil
	default:
		return errUnknownSubcommand("icon", subcommand, []string{
			clispec.SubList, clispec.SubGet, clispec.SubUpload, clispec.SubCheck, clispec.SubDownload,
		})
	}
}

func printJSON(v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

func iconDownloadFileName(rec buddy.Icon, id int) string {
	name := strings.TrimSpace(rec.Name)
	name = strings.ReplaceAll(name, "\\", "/")
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	if name == "" || name == "." || name == ".." {
		return fmt.Sprintf("icon-%d.png", id)
	}
	return name
}

func runCategory(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if err := resolveArg0("category", args, []string{clispec.SubList, clispec.SubGet, clispec.SubCreate, clispec.SubDelete, clispec.SubHelp}); err != nil {
		return err
	}
	if len(args) == 0 || args[0] == clispec.SubHelp {
		fmt.Print(helpCategory)
		return nil
	}
	subcommand := args[0]
	args = args[1:]

	switch subcommand {
	case clispec.SubList:
		var filter string
		fs := flag.NewFlagSet("category list", flag.ContinueOnError)
		fs.StringVar(&filter, clispec.FlagFilter, "", "name substring or RSQL filter")
		asJSON := fs.Bool(clispec.FlagJSON, false, "Print matching rows as JSON")
		if err := fs.Parse(args); err != nil {
			return err
		}
		cats, err := buddy.CategorySearch(credName, filter, buddy.PageOpts{Page: buddy.PageAll}, log)
		if err != nil {
			return err
		}
		if *asJSON {
			b, err := json.MarshalIndent(cats, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(b))
			return nil
		}
		fmt.Printf("%-8s  %-8s  %s\n", "ID", "Priority", "Name")
		fmt.Printf("%-8s  %-8s  %s\n", "--------", "--------", "----------------------------------------")
		for _, c := range cats {
			fmt.Printf("%-8s  %-8d  %s\n", c.ID, c.Priority, c.Name)
		}
		return nil
	case clispec.SubGet:
		if len(args) < 1 {
			return missingArg("category-id", clispec.UsageHelp(clispec.CmdCategory))
		}
		c, err := buddy.CategoryFetch(credName, args[0])
		if err != nil {
			return err
		}
		b, err := json.MarshalIndent(c, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	case clispec.SubCreate:
		var priority int
		fs := flag.NewFlagSet("category create", flag.ContinueOnError)
		fs.IntVar(&priority, "priority", 9, "category priority (Jamf default 9)")
		flagArgs, positionals := staticGroupSplitFlags(args)
		if err := fs.Parse(flagArgs); err != nil {
			return err
		}
		if len(positionals) < 1 {
			return missingArg("<name>", clispec.UsageHelp(clispec.CmdCategory))
		}
		name := strings.TrimSpace(strings.Join(positionals, " "))
		_, err := buddy.CategoryPost(credName, opts, buddy.CategoryDetail{Name: name, Priority: priority}, log)
		return err
	case clispec.SubDelete:
		if len(args) < 1 {
			return missingArg("category-id", clispec.UsageHelp(clispec.CmdCategory))
		}
		return buddy.CategoryDelete(credName, opts, args[0], log)
	default:
		return errUnknownSubcommand("category", subcommand, []string{clispec.SubList, clispec.SubGet, clispec.SubCreate, clispec.SubDelete})
	}
}

func runMacApp(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if err := resolveArg0("macapp", args, []string{clispec.SubList, clispec.SubGet, clispec.SubCreate, clispec.SubDelete, clispec.SubHelp}); err != nil {
		return err
	}
	if len(args) == 0 || args[0] == clispec.SubHelp {
		fmt.Print(helpMacApp)
		return nil
	}
	subcommand := args[0]
	args = args[1:]

	switch subcommand {
	case clispec.SubList:
		var filter string
		fs := flag.NewFlagSet("macapp list", flag.ContinueOnError)
		fs.StringVar(&filter, clispec.FlagFilter, "", "substring matched against name, version, or bundle_id")
		if err := fs.Parse(args); err != nil {
			return err
		}
		apps, err := buddy.MacAppSearch(credName, filter, log)
		if err != nil {
			return err
		}
		fmt.Printf("%-8s  %-40s  %-16s  %s\n", "ID", "Name", "Version", "Bundle ID")
		fmt.Printf("%-8s  %-40s  %-16s  %s\n", "--------", "----------------------------------------", "----------------", "--------------------------------")
		for _, a := range apps {
			fmt.Printf("%-8d  %-40s  %-16s  %s\n", a.ID, truncatePad(a.Name, 40), truncatePad(a.Version, 16), a.BundleID)
		}
		return nil
	case clispec.SubGet:
		if len(args) < 1 {
			return missingArg("macapp-id", clispec.UsageHelp(clispec.CmdMacApp))
		}
		d, err := buddy.MacAppFetch(credName, args[0])
		if err != nil {
			return err
		}
		b, err := json.MarshalIndent(d, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	case clispec.SubCreate:
		var name, version, bundleID, url, category string
		var free bool
		fs := flag.NewFlagSet("macapp create", flag.ContinueOnError)
		fs.StringVar(&name, clispec.FlagName, "", "display name (required)")
		fs.StringVar(&version, "version", "", "version string (required)")
		fs.StringVar(&bundleID, "bundle-id", "", "bundle identifier (required)")
		fs.StringVar(&url, "url", "", "App Store URL (required)")
		fs.BoolVar(&free, "free", false, "mark as free App Store app")
		fs.StringVar(&category, clispec.FlagCategory, "", "category name (exact)")
		if err := fs.Parse(args); err != nil {
			return err
		}
		_, err := buddy.MacAppCreate(credName, opts, buddy.MacAppCreateSpec{
			Name:     name,
			Version:  version,
			BundleID: bundleID,
			URL:      url,
			IsFree:   free,
			Category: category,
		}, log)
		return err
	case clispec.SubDelete:
		if len(args) < 1 {
			return missingArg("macapp-id", clispec.UsageHelp(clispec.CmdMacApp))
		}
		var ids []int
		for _, s := range args {
			id, err := parseInt(s, "macapp-id")
			if err != nil {
				return err
			}
			ids = append(ids, id)
		}
		return buddy.MacAppDelete(credName, opts, ids, log)
	default:
		return errUnknownSubcommand("macapp", subcommand, []string{clispec.SubList, clispec.SubGet, clispec.SubCreate, clispec.SubDelete})
	}
}

func truncatePad(s string, width int) string {
	if len(s) <= width {
		return s
	}
	if width <= 3 {
		return s[:width]
	}
	return s[:width-3] + "..."
}

func printAppScanSource(res buddy.AppScanResult) {
	fmt.Println(buddy.FormatPolicyCacheLine(res.Cache))
	if res.Source == "cache" {
		if res.Cache.Stale {
			fmt.Println("  Pass --refresh to rebuild, or: buddy app cache refresh")
		}
	} else {
		fmt.Println("  Scanned live policy list. Faster next time: buddy app scan --refresh")
	}
	if res.IndexContext != "" {
		fmt.Printf("  Policy index via product tenant %s\n", res.IndexContext)
	}
}

func runApp(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if err := resolveArg0("app", args, []string{clispec.SubList, "scan", clispec.SubGet, "import", "cache", "upgrade", clispec.SubHelp}); err != nil {
		return err
	}
	if len(args) == 0 || args[0] == clispec.SubHelp {
		fmt.Print(helpApp)
		return nil
	}
	subcommand := args[0]
	args = args[1:]

	switch subcommand {
	case clispec.SubList:
		var filter string
		fs := flag.NewFlagSet("app list", flag.ContinueOnError)
		fs.StringVar(&filter, clispec.FlagFilter, "", "substring matched against name, kind, version, trigger, or package")
		if err := fs.Parse(args); err != nil {
			return err
		}
		buddy.PrintPolicyCacheLine(credName)
		apps, err := buddy.AppListImported(buddy.DefaultAppsDir(), filter)
		if err != nil {
			return err
		}
		if len(apps) == 0 {
			fmt.Println("No imported apps (apps/index.json empty or missing).")
			fmt.Println("Onboard with: buddy app cache refresh && buddy app scan && buddy app import <name>")
			return nil
		}
		fmt.Printf("%-24s  %-8s  %-12s  %-8s  %-8s  %-8s  %-8s  %s\n",
			"Name", "Kind", "Version", "SS", "Install", "Cache", "Uninst", "Trigger")
		fmt.Printf("%-24s  %-8s  %-12s  %-8s  %-8s  %-8s  %-8s  %s\n",
			"------------------------", "--------", "------------", "--------", "--------", "--------", "--------", "----------------")
		idx, _ := buddy.LoadAppIndex(buddy.DefaultAppsDir())
		for _, a := range apps {
			inst, cache, uninst := a.InstallPolicyID, a.CachePolicyID, 0
			if idx != nil {
				if e, ok := idx.Apps[a.Name]; ok {
					if len(e.UninstallPolicyIDs) > 0 {
						uninst = e.UninstallPolicyIDs[0]
					}
					if len(e.InstallPolicyIDs) > 0 {
						inst = e.InstallPolicyIDs[0]
					}
					if len(e.CachePolicyIDs) > 0 {
						cache = e.CachePolicyIDs[0]
					}
				}
			}
			fmt.Printf("%-24s  %-8s  %-12s  %-8d  %-8d  %-8d  %-8d  %s\n",
				truncatePad(a.Name, 24), truncatePad(string(a.Kind), 8), truncatePad(a.Version, 12),
				a.SSPolicyID, inst, cache, uninst, a.InstallTrigger)
		}
		return nil
	case "scan":
		var filter string
		var refresh bool
		fs := flag.NewFlagSet("app scan", flag.ContinueOnError)
		fs.StringVar(&filter, clispec.FlagFilter, "", "substring matched against name, kind, version, trigger, or package")
		fs.BoolVar(&refresh, "refresh", false, "rebuild the policy cache, then scan it")
		if err := fs.Parse(args); err != nil {
			return err
		}
		res, err := buddy.AppScan(credName, buddy.AppScanOpts{Filter: filter, Refresh: refresh}, log)
		if err != nil {
			return err
		}
		printAppScanSource(res)
		fmt.Printf("%-30s  %-8s  %-12s  %-8s  %-24s  %s\n", "Name", "Kind", "Version", "SS ID", "Install trigger", "Package")
		fmt.Printf("%-30s  %-8s  %-12s  %-8s  %-24s  %s\n", "------------------------------", "--------", "------------", "--------", "------------------------", "--------------------------------")
		for _, a := range res.Apps {
			fmt.Printf("%-30s  %-8s  %-12s  %-8d  %-24s  %s\n",
				truncatePad(a.Name, 30), truncatePad(string(a.Kind), 8), truncatePad(a.Version, 12),
				a.SSPolicyID, truncatePad(a.InstallTrigger, 24), a.PackageName)
		}
		fmt.Println("\nHeuristic scan only — import with: buddy app import <name>")
		return nil
	case clispec.SubGet:
		if len(args) < 1 {
			return missingArg("app name", clispec.UsageHelp(clispec.CmdApp))
		}
		buddy.PrintPolicyCacheLine(credName)
		d, err := buddy.AppGet(credName, args[0], log)
		if err != nil {
			return err
		}
		b, err := json.MarshalIndent(d, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	case "upgrade":
		if len(args) < 1 {
			return missingArg("app name", clispec.UsageHelp(clispec.CmdApp))
		}
		name := args[0]
		var version, packagePattern string
		fs := flag.NewFlagSet("app upgrade", flag.ContinueOnError)
		fs.StringVar(&version, "version", "", "target version (required)")
		fs.StringVar(&packagePattern, "package-pattern", "", "optional package name glob")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(version) == "" {
			return fmt.Errorf("--version is required; run '%s' for usage", clispec.UsageHelp(clispec.CmdApp))
		}
		buddy.PrintPolicyCacheLine(credName)
		return buddy.AppUpgrade(credName, opts, name, version, packagePattern, log)
	case "import":
		var name string
		rest := args
		if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
			name = rest[0]
			rest = rest[1:]
		}
		var output, installCSV, cacheCSV, uninstallCSV, packagePattern, installTrig, cacheTrig, uninstallTrig, kind, version string
		var ssID int
		fs := flag.NewFlagSet("app import", flag.ContinueOnError)
		fs.StringVar(&output, "output", "", "output .buddy path (default apps/<slug>.buddy)")
		fs.StringVar(&output, "o", "", "shorthand for --output")
		fs.IntVar(&ssID, "ss", 0, "Self Service policy ID (non-interactive)")
		fs.StringVar(&installCSV, "install", "", "comma-separated install policy IDs")
		fs.StringVar(&cacheCSV, "cache", "", "comma-separated cache policy IDs")
		fs.StringVar(&uninstallCSV, "uninstall", "", "comma-separated uninstall policy IDs")
		fs.StringVar(&packagePattern, "package-pattern", "", "package name glob")
		fs.StringVar(&installTrig, "install-trigger", "", "install trigger")
		fs.StringVar(&cacheTrig, "cache-trigger", "", "cache trigger")
		fs.StringVar(&uninstallTrig, "uninstall-trigger", "", "uninstall trigger")
		fs.StringVar(&kind, "kind", "", "single|install|cache")
		fs.StringVar(&version, "version", "", "current version")
		if err := fs.Parse(rest); err != nil {
			return err
		}
		installIDs, err := buddy.ParseIntCSVList(installCSV)
		if err != nil {
			return err
		}
		cacheIDs, err := buddy.ParseIntCSVList(cacheCSV)
		if err != nil {
			return err
		}
		uninstallIDs, err := buddy.ParseIntCSVList(uninstallCSV)
		if err != nil {
			return err
		}
		_, err = buddy.AppImport(credName, opts, buddy.AppImportOpts{
			Name:               name,
			Output:             output,
			SSPolicyID:         ssID,
			InstallPolicyIDs:   installIDs,
			CachePolicyIDs:     cacheIDs,
			UninstallPolicyIDs: uninstallIDs,
			PackagePattern:     packagePattern,
			InstallTrigger:     installTrig,
			CacheTrigger:       cacheTrig,
			UninstallTrigger:   uninstallTrig,
			Kind:               kind,
			Version:            version,
		}, log)
		return err
	case "cache":
		if err := resolveArg0("app cache", args, []string{"status", "refresh", clispec.SubHelp}); err != nil {
			return err
		}
		if len(args) == 0 || args[0] == clispec.SubHelp {
			fmt.Println("Usage: buddy [global-flags] app cache <status|refresh>")
			return nil
		}
		switch args[0] {
		case "status":
			return buddy.PrintPolicyCacheStatus(credName)
		case "refresh":
			buddy.PrintPolicyCacheLine(credName)
			return buddy.PolicyCacheRefresh(credName, log)
		default:
			return errUnknownSubcommand("app cache", args[0], []string{"status", "refresh"})
		}
	default:
		return errUnknownSubcommand("app", subcommand, []string{clispec.SubList, "scan", clispec.SubGet, "import", "cache", "upgrade"})
	}
}

func runGroup(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if err := resolveArg0("group", args, []string{clispec.SubList, clispec.SubGet, clispec.SubCreate, clispec.SubDelete, "smart", "static", clispec.SubHelp}); err != nil {
		return err
	}
	if len(args) == 0 || args[0] == clispec.SubHelp {
		fmt.Print(helpGroup)
		return nil
	}
	mode := args[0]
	args = args[1:]

	switch mode {
	case clispec.SubList:
		if len(args) == 0 {
			return fmt.Errorf("usage: buddy group list smart|static [%s]", clispec.FlagEq(clispec.FlagFilter, "rsql"))
		}
		groupType := args[0]
		if resolved, err := matchSubcommand("group list", groupType, []string{"smart", "static"}); err != nil {
			return err
		} else {
			groupType = resolved
		}
		filter := ""
		fs := flag.NewFlagSet("group list "+groupType, flag.ContinueOnError)
		fs.StringVar(&filter, clispec.FlagFilter, "", "name filter (plain substring or RSQL)")
		asJSON := fs.Bool(clispec.FlagJSON, false, "Print matching rows as JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		switch groupType {
		case "smart":
			return buddy.GroupListSmart(credName, filter, *asJSON, log)
		case "static":
			return buddy.GroupListStatic(credName, filter, *asJSON, log)
		default:
			return errUnknownSubcommand("group list", groupType, []string{"smart", "static"})
		}
	case clispec.SubGet:
		if len(args) < 1 {
			return missingArg("group-id", clispec.UsageHelp(clispec.CmdGroup))
		}
		return buddy.GroupGet(credName, strings.TrimSpace(args[0]), log)
	case clispec.SubDelete:
		if len(args) < 1 {
			return missingArg("group-id", clispec.UsageHelp(clispec.CmdGroup))
		}
		groupID := strings.TrimSpace(args[0])
		return buddy.GroupDelete(credName, opts, groupID, log)
	case clispec.SubCreate:
		return runGroupCreate(credName, opts, args, log)
	case "smart":
		if len(args) == 0 {
			return fmt.Errorf("usage: buddy group smart <members|criteria|set-criteria|version|set-version> ...; run '%s' for usage", clispec.UsageHelp(clispec.CmdGroup))
		}
		sub := args[0]
		args = args[1:]
		if resolved, err := matchSubcommand("group smart", sub, []string{"members", "criteria", "set-criteria", "version", "set-version"}); err != nil {
			return err
		} else {
			sub = resolved
		}
		switch sub {
		case "members":
			var filter string
			fs := flag.NewFlagSet("group smart members", flag.ContinueOnError)
			fs.StringVar(&filter, clispec.FlagFilter, "", "client-side substring filter on id, name, or serial")
			flagArgs, positionals := staticGroupSplitFlags(args)
			if err := fs.Parse(flagArgs); err != nil {
				return err
			}
			if len(positionals) < 1 {
				return missingArg("group-id", clispec.UsageHelp(clispec.CmdGroup))
			}
			groupID := strings.TrimSpace(positionals[0])
			return buddy.GroupSmartListMembers(credName, groupID, filter, log)
		case "criteria":
			if len(args) < 1 {
				return missingArg("group-id", clispec.UsageHelp(clispec.CmdGroup))
			}
			return buddy.GroupSmartPrintCriteria(credName, strings.TrimSpace(args[0]), log)
		case "set-criteria":
			if len(args) < 1 {
				return missingArg("group-id", clispec.UsageHelp(clispec.CmdGroup))
			}
			groupID := strings.TrimSpace(args[0])
			body, rest, err := readJSONFileArg(args[1:])
			if err != nil {
				return err
			}
			if len(rest) > 0 {
				return fmt.Errorf("unexpected argument %q; usage: buddy group smart set-criteria <group-id> [%s|-]", rest[0], clispec.FlagEq(clispec.FlagFile, "json"))
			}
			return buddy.GroupSmartSetCriteria(credName, opts, groupID, body, log)
		case "version":
			if len(args) < 1 {
				return missingArg("group-id", clispec.UsageHelp(clispec.CmdGroup))
			}
			return buddy.GroupGetVersion(credName, opts, strings.TrimSpace(args[0]), log)
		case "set-version":
			if len(args) < 2 {
				return missingArg("group-id or version", clispec.UsageHelp(clispec.CmdGroup))
			}
			groupID := strings.TrimSpace(args[0])
			newVersion := strings.TrimSpace(args[1])
			return buddy.GroupVersion(credName, opts, groupID, newVersion, log)
		default:
			return errUnknownSubcommand("group smart", sub, []string{"members", "criteria", "set-criteria", "version", "set-version"})
		}
	case "static":
		if len(args) == 0 {
			return fmt.Errorf("usage: buddy group static <members|add|remove|set> ...; run '%s' for usage", clispec.UsageHelp(clispec.CmdGroup))
		}
		sub := args[0]
		args = args[1:]
		if resolved, err := matchSubcommand("group static", sub, []string{"members", clispec.SubAdd, "remove", clispec.SubSet}); err != nil {
			return err
		} else {
			sub = resolved
		}
		switch sub {
		case "members":
			var filter string
			fs := flag.NewFlagSet("group static members", flag.ContinueOnError)
			fs.StringVar(&filter, clispec.FlagFilter, "", "client-side substring filter on id, name, or serial")
			flagArgs, positionals := staticGroupSplitFlags(args)
			if err := fs.Parse(flagArgs); err != nil {
				return err
			}
			if len(positionals) < 1 {
				return missingArg("group-id", clispec.UsageHelp(clispec.CmdGroup))
			}
			groupID := strings.TrimSpace(positionals[0])
			return buddy.GroupStaticListMembers(credName, groupID, filter, log)
		case "add", "remove", clispec.SubSet:
			var computerIDsFlag string
			fs := flag.NewFlagSet("group static "+sub, flag.ContinueOnError)
			fs.StringVar(&computerIDsFlag, clispec.FlagComputerIds, "", "comma-separated computer IDs")
			flagArgs, positionals := staticGroupSplitFlags(args)
			if err := fs.Parse(flagArgs); err != nil {
				return err
			}
			if len(positionals) < 1 {
				return missingArg("group-id", clispec.UsageHelp(clispec.CmdGroup))
			}
			seenIDs := false
			for _, a := range args {
				if a == clispec.Flag(clispec.FlagComputerIds) || strings.HasPrefix(a, clispec.FlagPrefix(clispec.FlagComputerIds)) {
					seenIDs = true
					break
				}
			}
			if !seenIDs {
				return fmt.Errorf("buddy group static %s requires %s", sub, clispec.FlagEq(clispec.FlagComputerIds, "computer-id"))
			}
			groupID := strings.TrimSpace(positionals[0])
			ids := parseCommaSeparatedIDs(computerIDsFlag)
			if sub != clispec.SubSet && len(ids) == 0 {
				return fmt.Errorf("no computer IDs in %s", clispec.Flag(clispec.FlagComputerIds))
			}
			var changed bool
			var err error
			switch sub {
			case "add":
				changed, err = buddy.StaticGroupAddComputers(credName, opts, groupID, ids, log)
			case "remove":
				changed, err = buddy.StaticGroupRemoveComputers(credName, opts, groupID, ids, log)
			default:
				changed, err = buddy.StaticGroupSetComputers(credName, opts, groupID, ids, log)
			}
			if err != nil {
				return err
			}
			if changed {
				fmt.Printf("Updated static group %s.\n", groupID)
			}
			return nil
		default:
			return errUnknownSubcommand("group static", sub, []string{"members", clispec.SubAdd, "remove", clispec.SubSet})
		}
	default:
		cands := []string{clispec.SubList, clispec.SubGet, clispec.SubCreate, clispec.SubDelete, "smart", "static"}
		msg := fmt.Sprintf("unknown group subcommand: %q", mode)
		msg += formatDidYouMean(suggestClosest(mode, cands))
		msg += fmt.Sprintf(" (use: %s)", strings.Join(cands, ", "))
		return fmt.Errorf("%s", msg)
	}
}

func runGroupCreate(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: buddy group create smart|static ...; run '%s' for usage", clispec.UsageHelp(clispec.CmdGroup))
	}
	kind := args[0]
	if resolved, err := matchSubcommand("group create", kind, []string{"smart", "static"}); err != nil {
		return err
	} else {
		kind = resolved
	}
	args = args[1:]
	switch kind {
	case "smart":
		return runGroupCreateSmart(credName, opts, args, log)
	case "static":
		var description, computerIDsFlag string
		fs := flag.NewFlagSet("group create static", flag.ContinueOnError)
		fs.StringVar(&description, "description", "", "optional group description")
		fs.StringVar(&computerIDsFlag, clispec.FlagComputerIds, "", "comma-separated computer IDs (optional)")
		flagArgs, positionals := staticGroupSplitFlags(args)
		if err := fs.Parse(flagArgs); err != nil {
			return err
		}
		if len(positionals) < 1 {
			return missingArg("<name>", clispec.UsageHelp(clispec.CmdGroup))
		}
		name := strings.TrimSpace(positionals[0])
		var ids []string
		if computerIDsFlag != "" {
			ids = parseCommaSeparatedIDs(computerIDsFlag)
			if len(ids) == 0 {
				return fmt.Errorf("no computer IDs in --computer-ids")
			}
		}
		created, err := buddy.GroupCreateStaticNamed(credName, opts, name, description, ids, log)
		if err != nil {
			return err
		}
		if created == nil {
			return nil
		}
		fmt.Printf("Created static group %q (ID %s)\n", name, created.ID)
		return nil
	default:
		return errUnknownSubcommand("group create", kind, []string{"smart", "static"})
	}
}

func runGroupCreateSmart(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	var description, filePath string
	fs := flag.NewFlagSet("group create smart", flag.ContinueOnError)
	fs.StringVar(&description, "description", "", "optional group description")
	fs.StringVar(&filePath, clispec.FlagFile, "", "JSON file: full body, or criteria array")
	flagArgs, positionals := staticGroupSplitFlags(args)
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if filePath != "" && len(positionals) == 0 {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("read %s: %w", clispec.Flag(clispec.FlagFile), err)
		}
		created, err := buddy.GroupCreateSmart(credName, opts, data, log)
		if err != nil {
			return err
		}
		return printCreatedSmartGroup(created, "")
	}
	if len(positionals) < 1 {
		return missingArg("<name>", clispec.UsageHelp(clispec.CmdGroup))
	}
	name := strings.TrimSpace(positionals[0])
	payload, criteriaJSON, err := parseSmartGroupCreateFile(filePath, name, description)
	if err != nil {
		return err
	}
	if payload != nil {
		created, err := buddy.GroupCreateSmart(credName, opts, payload, log)
		if err != nil {
			return err
		}
		return printCreatedSmartGroup(created, name)
	}
	created, err := buddy.GroupCreateSmartNamed(credName, opts, name, description, criteriaJSON, log)
	if err != nil {
		return err
	}
	return printCreatedSmartGroup(created, name)
}

func parseSmartGroupCreateFile(filePath, name, description string) (fullBody, criteriaJSON []byte, err error) {
	if filePath == "" {
		return nil, nil, nil
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", clispec.Flag(clispec.FlagFile), err)
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, data, nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, nil, fmt.Errorf("parse %s: %w", clispec.Flag(clispec.FlagFile), err)
	}
	if _, ok := raw["name"]; !ok {
		nameJSON, err := json.Marshal(name)
		if err != nil {
			return nil, nil, err
		}
		raw["name"] = nameJSON
	}
	if description != "" {
		descJSON, err := json.Marshal(description)
		if err != nil {
			return nil, nil, err
		}
		raw["description"] = descJSON
	}
	if _, ok := raw["id"]; !ok {
		raw["id"] = json.RawMessage("-1")
	}
	payload, err := json.Marshal(raw)
	if err != nil {
		return nil, nil, err
	}
	return payload, nil, nil
}

func printCreatedSmartGroup(created *buddy.GroupCreateResult, name string) error {
	if created == nil {
		return nil
	}
	if name == "" {
		fmt.Printf("Created smart group (ID %s)\n", created.ID)
		return nil
	}
	fmt.Printf("Created smart group %q (ID %s)\n", name, created.ID)
	return nil
}

func parseComputerIDs(args []string) ([]int, []string, error) {
	var ids []int
	var rest []string
	for _, a := range args {
		if strings.HasPrefix(a, "--") {
			rest = append(rest, a)
			continue
		}
		id, err := parseInt(a, "computer-id")
		if err != nil {
			return nil, nil, err
		}
		ids = append(ids, id)
	}
	return ids, rest, nil
}

func requireComputerIDs(args []string) ([]int, []string, error) {
	if len(args) < 1 {
		return nil, nil, missingArg("computer-id", clispec.UsageHelp(clispec.CmdComputer))
	}
	return parseComputerIDs(args)
}

func runComputerIDs(args []string, fn func([]int) error) error {
	ids, _, err := requireComputerIDs(args)
	if err != nil {
		return err
	}
	return fn(ids)
}

// computerEAKeyIsID reports whether s looks like a numeric EA definition id.
func computerEAKeyIsID(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func parseInt(s, name string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid %s %q: must be positive integer", name, s)
	}
	return n, nil
}

func parseCommaSeparatedIDs(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// staticGroupSplitFlags separates flag tokens from positional arguments before flag.Parse.
// The standard library stops parsing at the first non-flag argument, so without this,
// "buddy group static remove 5 --computer-ids=1" would never set --computer-ids.
func staticGroupSplitFlags(args []string) (flagArgs []string, positional []string) {
	i := 0
	for i < len(args) {
		a := args[i]
		if strings.HasPrefix(a, "-") {
			flagArgs = append(flagArgs, a)
			if strings.HasPrefix(a, "--") && strings.Contains(a, "=") {
				i++
				continue
			}
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				flagArgs = append(flagArgs, args[i])
			}
			i++
			continue
		}
		positional = append(positional, a)
		i++
	}
	return flagArgs, positional
}

// parseParamArgs extracts --param=4,5,6 or --param 4,5,6 from args.
// Returns paramIndices, remaining args, and error.
func parseParamArgs(args []string) ([]int, []string, error) {
	paramIndices := []int{4, 5, 6}
	var filtered []string
	i := 0
	for i < len(args) {
		a := args[i]
		if strings.HasPrefix(a, "--param=") {
			val := strings.TrimPrefix(a, "--param=")
			var err error
			paramIndices, err = parseParamVal(val)
			if err != nil {
				return nil, nil, err
			}
		} else if a == "--param" {
			i++
			if i >= len(args) {
				return nil, nil, fmt.Errorf("%s requires a value (e.g. %s=4,5,6)", a, a)
			}
			var err error
			paramIndices, err = parseParamVal(args[i])
			if err != nil {
				return nil, nil, err
			}
			i++ // skip the value
		} else {
			filtered = append(filtered, a)
		}
		i++
	}
	return paramIndices, filtered, nil
}

func parseParamVal(val string) ([]int, error) {
	var paramIndices []int
	for _, p := range strings.Split(val, ",") {
		p = strings.TrimSpace(p)
		n, err := strconv.Atoi(p)
		if err != nil || n < 4 || n > 11 {
			return nil, fmt.Errorf("invalid --param=%q: must be 4-11 or comma-separated (e.g. 4,5,6)", val)
		}
		paramIndices = append(paramIndices, n)
	}
	if len(paramIndices) == 0 {
		paramIndices = []int{4, 5, 6}
	}
	return paramIndices, nil
}

func parsePolicySSArgs(args []string) (buddy.PolicySSOpts, int, error) {
	var ssOpts buddy.PolicySSOpts
	var groupIDsFlag string
	var positional []string

	for _, a := range args {
		switch {
		case strings.HasPrefix(a, clispec.FlagPrefix(clispec.FlagGroupIDs)):
			groupIDsFlag = strings.TrimPrefix(a, clispec.FlagPrefix(clispec.FlagGroupIDs))
		case strings.HasPrefix(a, "--display-name="):
			v := strings.TrimPrefix(a, "--display-name=")
			ssOpts.DisplayName = &v
		case strings.HasPrefix(a, "--description="):
			v := strings.TrimPrefix(a, "--description=")
			ssOpts.Description = &v
		case strings.HasPrefix(a, "--install-text="):
			v := strings.TrimPrefix(a, "--install-text=")
			ssOpts.InstallText = &v
		case strings.HasPrefix(a, "--reinstall-text="):
			v := strings.TrimPrefix(a, "--reinstall-text=")
			ssOpts.ReinstallText = &v
		case a == "--force-view":
			v := true
			ssOpts.ForceView = &v
		case a == "--no-force-view":
			v := false
			ssOpts.ForceView = &v
		case a == "--feature":
			v := true
			ssOpts.Feature = &v
		case a == "--no-feature":
			v := false
			ssOpts.Feature = &v
		case a == "--self-service":
			v := true
			ssOpts.SelfService = &v
		case a == "--no-self-service":
			v := false
			ssOpts.SelfService = &v
		default:
			positional = append(positional, a)
		}
	}

	if groupIDsFlag != "" {
		for _, s := range strings.Split(groupIDsFlag, ",") {
			s = strings.TrimSpace(s)
			id, err := parseInt(s, "group-id")
			if err != nil {
				return ssOpts, 0, err
			}
			ssOpts.GroupIDs = append(ssOpts.GroupIDs, id)
		}
	}

	if len(positional) < 1 {
		return ssOpts, 0, missingArg("policy-id", clispec.UsageHelp(clispec.CmdPolicy, clispec.SubSet))
	}
	policyID, err := parseInt(positional[0], "policy-id")
	if err != nil {
		return ssOpts, 0, err
	}
	if len(positional) >= 2 {
		ssOpts.Version = strings.TrimSpace(positional[1])
	}

	hasFlags := ssOpts.DisplayName != nil || ssOpts.Description != nil ||
		ssOpts.InstallText != nil || ssOpts.ReinstallText != nil ||
		ssOpts.ForceView != nil || ssOpts.Feature != nil ||
		ssOpts.SelfService != nil || len(ssOpts.GroupIDs) > 0

	if ssOpts.Version == "" && !hasFlags {
		return ssOpts, 0, missingArg("version or flags", clispec.UsageHelp(clispec.CmdPolicy, clispec.SubSet))
	}

	return ssOpts, policyID, nil
}

func runProfile(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if err := resolveArg0("profile", args, []string{clispec.SubList, clispec.SubGet, clispec.SubSet, clispec.SubCreate, clispec.SubDelete, clispec.SubClone, "redeploy", clispec.SubHelp}); err != nil {
		return err
	}
	if len(args) == 0 || args[0] == clispec.SubHelp {
		fmt.Print(helpProfile)
		return nil
	}
	mode := args[0]
	rest := args[1:]

	// "buddy profile <mode> help"
	if len(rest) > 0 && rest[0] == clispec.SubHelp {
		fmt.Print(helpProfile)
		return nil
	}
	args = rest

	switch mode {
	case clispec.SubClone:
		if len(args) < 2 {
			return missingArg("profile-id or name", clispec.UsageHelp(clispec.CmdProfile))
		}
		profileID, err := parseInt(args[0], "profile-id")
		if err != nil {
			return err
		}
		name := strings.TrimSpace(strings.Join(args[1:], " "))
		if name == "" {
			return missingArg("name", clispec.UsageHelp(clispec.CmdProfile))
		}
		_, err = buddy.ProfileClone(credName, opts, profileID, name, log)
		return err
	case clispec.SubList:
		filter, rest, err := extractStringFlag(args, clispec.FlagFilter)
		if err != nil {
			return err
		}
		asJSON, rest := extractBareFlag(rest, clispec.FlagJSON)
		if len(rest) > 0 {
			return fmt.Errorf("unknown list flag %q; run '%s'", rest[0], clispec.UsageHelp(clispec.CmdProfile))
		}
		return buddy.ProfileList(credName, filter, asJSON, log)
	case clispec.SubCreate:
		name, rest, err := extractStringFlag(args, clispec.FlagName)
		if err != nil {
			return err
		}
		body, leftover, err := readXMLFileArg(rest)
		if err != nil {
			return err
		}
		if len(leftover) > 0 {
			return fmt.Errorf("unexpected argument %q; usage: buddy profile create [--name=<name>] [--file=<xml>|-]", leftover[0])
		}
		_, err = buddy.ProfilePostXML(credName, opts, body, name, log)
		return err
	case clispec.SubDelete:
		if len(args) < 1 {
			return missingArg("profile-id", clispec.UsageHelp(clispec.CmdProfile))
		}
		profileID, err := parseInt(args[0], "profile-id")
		if err != nil {
			return err
		}
		return buddy.ProfileDelete(credName, opts, profileID, log)
	case "redeploy":
		if len(args) < 1 {
			return missingArg("profile-id", clispec.UsageHelp(clispec.CmdProfile))
		}
		profileID, err := parseInt(args[0], "profile-id")
		if err != nil {
			return err
		}
		return buddy.ProfileRedeploy(credName, opts, profileID, log)
	case clispec.SubGet:
		if len(args) < 1 {
			return missingArg("subcommand", clispec.UsageHelp(clispec.CmdProfile))
		}
		return runProfileGet(credName, opts, args[0], args[1:], log)
	case clispec.SubSet:
		if len(args) < 1 {
			return missingArg("subcommand", clispec.UsageHelp(clispec.CmdProfile))
		}
		return runProfileSet(credName, opts, args[0], args[1:], log)
	default:
		return errUnknownSubcommand("profile", mode, []string{clispec.SubList, clispec.SubGet, clispec.SubSet, clispec.SubCreate, clispec.SubDelete, clispec.SubClone, "redeploy"})
	}
}

func runProfileGet(credName string, opts buddy.Opts, subcommand string, args []string, log *slog.Logger) error {
	var err error
	subcommand, err = matchSubcommand("profile get", subcommand, []string{"xml", "payloads", "payload", "list-payloads", clispec.SubHelp})
	if err != nil {
		return err
	}
	if subcommand == clispec.SubHelp {
		fmt.Print(helpProfile)
		return nil
	}
	switch subcommand {
	case "xml":
		if len(args) < 1 {
			return missingArg("profile-id", clispec.UsageHelp(clispec.CmdProfile))
		}
		profileID, err := parseInt(args[0], "profile-id")
		if err != nil {
			return err
		}
		return buddy.ProfileGetXML(credName, profileID, os.Stdout, log)
	case "payloads":
		if len(args) < 1 {
			return missingArg("profile-id", clispec.UsageHelp(clispec.CmdProfile))
		}
		profileID, err := parseInt(args[0], "profile-id")
		if err != nil {
			return err
		}
		return buddy.ProfileGetPayloads(credName, profileID, os.Stdout, log)
	case "payload":
		if len(args) < 2 {
			return missingArg("profile-id or identifier", clispec.UsageHelp(clispec.CmdProfile))
		}
		profileID, err := parseInt(args[0], "profile-id")
		if err != nil {
			return err
		}
		identifier := strings.TrimSpace(args[1])
		return buddy.ProfileGetPayloadByDomain(credName, profileID, identifier, os.Stdout, log)
	case "list-payloads":
		if len(args) < 1 {
			return missingArg("profile-id", clispec.UsageHelp(clispec.CmdProfile))
		}
		profileID, err := parseInt(args[0], "profile-id")
		if err != nil {
			return err
		}
		rows, err := buddy.ProfileListPayloads(credName, profileID, log)
		if err != nil {
			return err
		}
		fmt.Printf("%-36s  %-40s  %s\n", "IDENTIFIER", "TYPE", "UUID")
		for _, r := range rows {
			fmt.Printf("%-36s  %-40s  %s\n", r.Identifier, r.Type, r.UUID)
		}
		return nil
	default:
		return errUnknownSubcommand("profile get", subcommand, []string{"xml", "payloads", "payload", "list-payloads"})
	}
}

func runProfileSet(credName string, opts buddy.Opts, subcommand string, args []string, log *slog.Logger) error {
	var err error
	subcommand, err = matchSubcommand("profile set", subcommand, []string{"xml", "payload", clispec.SubHelp})
	if err != nil {
		return err
	}
	if subcommand == clispec.SubHelp {
		fmt.Print(helpProfile)
		return nil
	}
	// Extract --redeploy flag from args before positional parsing.
	redeploy := false
	var filtered []string
	for _, a := range args {
		if a == "--redeploy" {
			redeploy = true
		} else {
			filtered = append(filtered, a)
		}
	}
	args = filtered

	switch subcommand {
	case "xml":
		if len(args) < 1 {
			return missingArg("profile-id", clispec.UsageHelp(clispec.CmdProfile))
		}
		profileID, err := parseInt(args[0], "profile-id")
		if err != nil {
			return err
		}
		return buddy.ProfileSetXML(credName, opts, profileID, os.Stdin, redeploy, log)
	case "payload":
		if len(args) < 3 {
			return missingArg("profile-id, identifier, or dict-file", clispec.UsageHelp(clispec.CmdProfile))
		}
		profileID, err := parseInt(args[0], "profile-id")
		if err != nil {
			return err
		}
		identifier := strings.TrimSpace(args[1])
		dictFile := args[2]
		return buddy.ProfileSetPayload(credName, opts, profileID, identifier, dictFile, redeploy, log)
	default:
		return errUnknownSubcommand("profile set", subcommand, []string{"xml", "payload"})
	}
}

func runPackage(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if err := resolveArg0("package", args, []string{clispec.SubList, clispec.SubGet, "download", clispec.SubDelete, clispec.SubSet, "history", clispec.SubUpload, "scan", clispec.SubHelp}); err != nil {
		return err
	}
	if len(args) == 0 || args[0] == clispec.SubHelp {
		fmt.Print(helpPackage)
		return nil
	}
	subcommand := args[0]
	args = args[1:]

	switch subcommand {
	case clispec.SubUpload:
		if len(args) < 1 {
			return missingArg("file argument", clispec.UsageHelp(clispec.CmdPackage))
		}
		filePath, attrs, upOpts, err := parsePackageUploadArgs(args)
		if err != nil {
			return err
		}
		return buddy.PackageUpload(credName, opts, filePath, attrs, upOpts, log)
	case clispec.SubList:
		filter, rest, err := extractStringFlag(args, clispec.FlagFilter)
		if err != nil {
			return err
		}
		sortSpec, rest, err := extractStringFlag(rest, clispec.FlagSort)
		if err != nil {
			return err
		}
		sortOrder, err := parsePackageListSort(sortSpec)
		if err != nil {
			return err
		}
		asJSON, rest := extractBareFlag(rest, clispec.FlagJSON)
		if len(rest) > 0 {
			return fmt.Errorf("unknown list flag %q; run '%s'", rest[0], clispec.UsageHelp(clispec.CmdPackage))
		}
		return buddy.PackageList(credName, filter, sortOrder, asJSON, log)
	case clispec.SubGet:
		if len(args) < 1 {
			return missingArg("package-id", clispec.UsageHelp(clispec.CmdPackage))
		}
		return buddy.PackageGet(credName, args[0], log)
	case "download":
		if len(args) < 1 {
			return missingArg("package-id", clispec.UsageHelp(clispec.CmdPackage))
		}
		outfile := ""
		if len(args) >= 2 {
			outfile = args[1]
		}
		return buddy.PackageDownload(credName, args[0], outfile, log)
	case clispec.SubDelete:
		if len(args) < 1 {
			return missingArg("package-id", clispec.UsageHelp(clispec.CmdPackage))
		}
		return buddy.PackageDelete(credName, opts, args, log)
	case clispec.SubSet:
		if len(args) < 1 {
			return missingArg("package-id or property", clispec.UsageHelp(clispec.CmdPackage))
		}
		return runPackageSet(credName, opts, args, log)
	case "history":
		if len(args) < 1 {
			return missingArg("package-id", clispec.UsageHelp(clispec.CmdPackage))
		}
		return buddy.PackageHistory(credName, args[0], 20, log)
	case "scan":
		classSpec, rest, err := extractStringFlag(args, "class")
		if err != nil {
			return err
		}
		filter, _, err := extractStringFlag(rest, "filter")
		if err != nil {
			return err
		}
		return buddy.PackageScanPrint(credName, filter, classSpec, log)
	default:
		return errUnknownSubcommand("package", subcommand, []string{clispec.SubList, clispec.SubGet, "download", clispec.SubDelete, clispec.SubSet, "history", clispec.SubUpload, "scan"})
	}
}

// parsePackageListSort maps --sort= values to an order string for PackageList.
// "" means no sort. "version" defaults to desc (latest first).
func parsePackageListSort(spec string) (order string, err error) {
	spec = strings.TrimSpace(strings.ToLower(spec))
	if spec == "" {
		return "", nil
	}
	switch spec {
	case "version", "version:desc":
		return "desc", nil
	case "version:asc":
		return "asc", nil
	default:
		return "", fmt.Errorf("invalid --sort=%s (use version, version:asc, or version:desc)", spec)
	}
}

func parsePackageUploadArgs(args []string) (filePath string, attrs buddy.PackageAttrs, upOpts buddy.PackageUploadOptions, err error) {
	filtered := make([]string, 0, len(args))
	for _, a := range args {
		switch {
		case a == "--force":
			upOpts.Force = true
		case a == "--wait-ready":
			upOpts.WaitReady = true
		case strings.HasPrefix(a, "--wait-ready="):
			upOpts.WaitReady = true
			v := strings.TrimSpace(strings.TrimPrefix(a, "--wait-ready="))
			if v == "" || strings.EqualFold(v, "true") {
				break
			}
			if strings.EqualFold(v, "false") {
				upOpts.WaitReady = false
				break
			}
			sec, e := strconv.Atoi(v)
			if e != nil || sec < 0 {
				return "", buddy.PackageAttrs{}, buddy.PackageUploadOptions{}, fmt.Errorf("--wait-ready= expects true/false or seconds >= 0, got %q", v)
			}
			upOpts.WaitReadyTimeout = time.Duration(sec) * time.Second
		case strings.HasPrefix(a, "--wait-ready-timeout="):
			upOpts.WaitReady = true
			v := strings.TrimSpace(strings.TrimPrefix(a, "--wait-ready-timeout="))
			sec, e := strconv.Atoi(v)
			if e != nil || sec < 0 {
				return "", buddy.PackageAttrs{}, buddy.PackageUploadOptions{}, fmt.Errorf("--wait-ready-timeout= expects seconds >= 0, got %q", v)
			}
			upOpts.WaitReadyTimeout = time.Duration(sec) * time.Second
		default:
			filtered = append(filtered, a)
		}
	}
	attrs, positional, err := parsePackageAttrFlags(filtered)
	if err != nil {
		return "", buddy.PackageAttrs{}, buddy.PackageUploadOptions{}, err
	}
	if len(positional) < 1 {
		return "", buddy.PackageAttrs{}, buddy.PackageUploadOptions{}, missingArg("file argument", clispec.UsageHelp(clispec.CmdPackage))
	}
	if len(positional) > 1 {
		return "", buddy.PackageAttrs{}, buddy.PackageUploadOptions{}, fmt.Errorf("unexpected arguments after file: %v", positional[1:])
	}
	filePath = strings.TrimSpace(positional[0])
	if filePath == "" {
		return "", buddy.PackageAttrs{}, buddy.PackageUploadOptions{}, missingArg("file argument", clispec.UsageHelp(clispec.CmdPackage))
	}
	return filePath, attrs, upOpts, nil
}

// parsePackageAttrFlags parses shared upload/set metadata flags.
func parsePackageAttrFlags(args []string) (attrs buddy.PackageAttrs, positional []string, err error) {
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, clispec.FlagPrefix(clispec.FlagName)):
			v := strings.TrimSpace(strings.TrimPrefix(a, clispec.FlagPrefix(clispec.FlagName)))
			attrs.Name = &v
		case strings.HasPrefix(a, "--notes="):
			v := strings.TrimSpace(strings.TrimPrefix(a, "--notes="))
			attrs.Notes = &v
		case strings.HasPrefix(a, "--info="):
			v := strings.TrimSpace(strings.TrimPrefix(a, "--info="))
			attrs.Info = &v
		case strings.HasPrefix(a, clispec.FlagPrefix(clispec.FlagCategory)):
			v := strings.TrimSpace(strings.TrimPrefix(a, clispec.FlagPrefix(clispec.FlagCategory)))
			attrs.CategoryID = &v
		case strings.HasPrefix(a, "--priority="):
			v := strings.TrimSpace(strings.TrimPrefix(a, "--priority="))
			n, e := parseInt(v, "priority")
			if e != nil {
				return buddy.PackageAttrs{}, nil, e
			}
			attrs.Priority = &n
		case strings.HasPrefix(a, "--reboot="):
			v := strings.TrimSpace(strings.TrimPrefix(a, "--reboot="))
			b, e := parsePackageBoolFlag(v, "reboot")
			if e != nil {
				return buddy.PackageAttrs{}, nil, e
			}
			attrs.Reboot = &b
		case strings.HasPrefix(a, "--"):
			return buddy.PackageAttrs{}, nil, fmt.Errorf("unknown flag %s (use %s, --notes=, --info=, %s, --priority=, --reboot=)", a, clispec.FlagPrefix(clispec.FlagName), clispec.FlagPrefix(clispec.FlagCategory))
		default:
			positional = append(positional, a)
		}
	}
	if err := attrs.Validate(); err != nil {
		return buddy.PackageAttrs{}, nil, err
	}
	return attrs, positional, nil
}

func parsePackageBoolFlag(value, name string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "yes", "1":
		return true, nil
	case "false", "no", "0":
		return false, nil
	default:
		return false, fmt.Errorf("%s must be true or false, got %q", name, value)
	}
}

func runComputerHistory(credName string, args []string, log *slog.Logger) error {
	fs := flag.NewFlagSet("computer history", flag.ContinueOnError)
	subset := fs.String("subset", "PolicyLogs", "Classic history subset (default PolicyLogs); omit path segment with --subset=")
	asJSON := fs.Bool(clispec.FlagJSON, false, "Print parsed history dict as JSON")
	flagArgs, positionals := staticGroupSplitFlags(args)
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if len(positionals) < 1 {
		return missingArg("computer-id", clispec.UsageHelp(clispec.CmdComputer))
	}
	var subsets []string
	if *subset != "" {
		subsets = []string{*subset}
	}
	return buddy.ComputerHistoryPrint(credName, buddy.ComputerHistoryRef{ID: positionals[0]}, subsets, *asJSON, log)
}

func runComputerLogs(credName string, args []string, log *slog.Logger) error {
	fs := flag.NewFlagSet("computer logs", flag.ContinueOnError)
	policy := fs.String("policy", "", "filter by policy id")
	status := fs.String("status", "", "filter by log status (case-insensitive exact)")
	since := fs.String(clispec.FlagSince, "", "keep log rows with date_completed_epoch >= value (ISO or epoch)")
	until := fs.String(clispec.FlagUntil, "", "keep log rows with date_completed_epoch <= value (ISO or epoch)")
	asJSON := fs.Bool(clispec.FlagJSON, false, "Print flattened logs as JSON")
	flagArgs, positionals := staticGroupSplitFlags(args)
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if len(positionals) < 1 {
		return missingArg("computer-id", clispec.UsageHelp(clispec.CmdComputer))
	}
	var policyID *int
	if strings.TrimSpace(*policy) != "" {
		n, err := parseInt(*policy, "policy-id")
		if err != nil {
			return err
		}
		policyID = &n
	}
	return buddy.ComputerPolicyLogsPrint(credName, buddy.ComputerHistoryRef{ID: positionals[0]}, buddy.PolicyLogFilter{
		PolicyID: policyID,
		Status:   *status,
		Since:    *since,
		Until:    *until,
	}, *asJSON, log)
}

func runComputer(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if err := resolveArg0("computer", args, []string{
		clispec.SubList, clispec.SubGet, "detail", "history", "logs", "eas", clispec.SubSet, "reboot", "redeploy", "shutdown", "lock", "logout", "erase",
		"device-info", "device-location", "security-info", "installed-apps", "managed-apps", "certificates", clispec.SubHelp,
	}); err != nil {
		return err
	}
	if len(args) == 0 || args[0] == clispec.SubHelp {
		fmt.Print(helpComputer)
		return nil
	}
	subcommand := args[0]
	args = args[1:]

	switch subcommand {
	case clispec.SubList:
		fs := flag.NewFlagSet("computer list", flag.ContinueOnError)
		filter := fs.String(clispec.FlagFilter, "", "RSQL filter expression")
		sort := fs.String(clispec.FlagSort, "", "Jamf sort= (e.g. general.lastContact:desc)")
		sectionFlag := fs.String("section", "", "inventory section(s), comma-separated; omit for Jamf GENERAL default")
		mgmtID := fs.String("management-id", "", "Look up computer by MDM management UUID (clientManagementId)")
		asJSON := fs.Bool(clispec.FlagJSON, false, "Print Jamf results[] as JSON")
		if err := fs.Parse(args); err != nil {
			return err
		}
		f := *filter
		if *mgmtID != "" {
			mgmtFilter := fmt.Sprintf(`general.managementId=="%s"`, *mgmtID)
			if f != "" {
				f = f + ";" + mgmtFilter
			} else {
				f = mgmtFilter
			}
		}
		var sections []string
		if *sectionFlag != "" {
			sections = []string{*sectionFlag}
		}
		q := buddy.ComputerListQuery{Filter: f, Sort: *sort, Sections: sections}
		return buddy.ComputerSearchPrint(credName, q, *asJSON, log)
	case clispec.SubGet:
		if len(args) < 1 {
			return missingArg("computer-id", clispec.UsageHelp(clispec.CmdComputer))
		}
		fs := flag.NewFlagSet("computer get", flag.ContinueOnError)
		var sectionFlag string
		fs.StringVar(&sectionFlag, "section", "", "inventory section(s), comma-separated (e.g. GENERAL,HARDWARE); omit for Jamf GENERAL default")
		flagArgs, positionals := staticGroupSplitFlags(args)
		if err := fs.Parse(flagArgs); err != nil {
			return err
		}
		if len(positionals) < 1 {
			return missingArg("computer-id", clispec.UsageHelp(clispec.CmdComputer))
		}
		var sections []string
		if sectionFlag != "" {
			sections = []string{sectionFlag}
		}
		return buddy.ComputerGet(credName, positionals[0], sections, log)
	case "detail":
		if len(args) < 1 {
			return missingArg("computer-id", clispec.UsageHelp(clispec.CmdComputer))
		}
		return buddy.ComputerDetail(credName, args[0], log)
	case "history":
		return runComputerHistory(credName, args, log)
	case "logs":
		return runComputerLogs(credName, args, log)
	case "reboot":
		return runComputerIDs(args, func(ids []int) error {
			return buddy.ComputerReboot(credName, opts, ids, log)
		})
	case "redeploy":
		return runComputerIDs(args, func(ids []int) error {
			return buddy.ComputerRedeploy(credName, opts, ids, log)
		})
	case "shutdown":
		return runComputerIDs(args, func(ids []int) error {
			return buddy.ComputerShutdown(credName, opts, ids, log)
		})
	case "lock":
		computerIDs, rest, err := requireComputerIDs(args)
		if err != nil {
			return err
		}
		var message, phone, pin string
		for _, r := range rest {
			if strings.HasPrefix(r, "--message=") {
				message = strings.TrimPrefix(r, "--message=")
			} else if strings.HasPrefix(r, "--phone=") {
				phone = strings.TrimPrefix(r, "--phone=")
			} else if strings.HasPrefix(r, "--pin=") {
				pin = strings.TrimPrefix(r, "--pin=")
			}
		}
		return buddy.ComputerLock(credName, opts, computerIDs, message, phone, pin, log)
	case "logout":
		return runComputerIDs(args, func(ids []int) error {
			return buddy.ComputerLogOut(credName, opts, ids, log)
		})
	case "erase":
		computerIDs, rest, err := requireComputerIDs(args)
		if err != nil {
			return err
		}
		var pin, obliteration string
		for _, r := range rest {
			if strings.HasPrefix(r, "--pin=") {
				pin = strings.TrimPrefix(r, "--pin=")
			} else if strings.HasPrefix(r, "--obliteration=") {
				obliteration = strings.TrimPrefix(r, "--obliteration=")
			}
		}
		return buddy.ComputerErase(credName, opts, computerIDs, pin, obliteration, false, false, log)
	case "device-info":
		computerIDs, rest, err := requireComputerIDs(args)
		if err != nil {
			return err
		}
		var queries []string
		for _, r := range rest {
			if strings.HasPrefix(r, "--queries=") {
				q := strings.TrimPrefix(r, "--queries=")
				queries = strings.Split(q, ",")
				for i := range queries {
					queries[i] = strings.TrimSpace(queries[i])
				}
				break
			}
		}
		return buddy.ComputerDeviceInfo(credName, opts, computerIDs, queries, log)
	case "device-location":
		return runComputerIDs(args, func(ids []int) error {
			return buddy.ComputerDeviceLocation(credName, opts, ids, log)
		})
	case "security-info":
		return runComputerIDs(args, func(ids []int) error {
			return buddy.ComputerSecurityInfo(credName, opts, ids, log)
		})
	case "installed-apps":
		computerIDs, rest, err := requireComputerIDs(args)
		if err != nil {
			return err
		}
		var identifiers []string
		for _, r := range rest {
			if strings.HasPrefix(r, "--identifiers=") {
				idStr := strings.TrimPrefix(r, "--identifiers=")
				identifiers = strings.Split(idStr, ",")
				for i := range identifiers {
					identifiers[i] = strings.TrimSpace(identifiers[i])
				}
				break
			}
		}
		return buddy.ComputerInstalledApps(credName, opts, computerIDs, identifiers, log)
	case "managed-apps":
		computerIDs, rest, err := requireComputerIDs(args)
		if err != nil {
			return err
		}
		var identifiers []string
		for _, r := range rest {
			if strings.HasPrefix(r, "--identifiers=") {
				idStr := strings.TrimPrefix(r, "--identifiers=")
				identifiers = strings.Split(idStr, ",")
				for i := range identifiers {
					identifiers[i] = strings.TrimSpace(identifiers[i])
				}
				break
			}
		}
		return buddy.ComputerManagedApps(credName, opts, computerIDs, identifiers, log)
	case "certificates":
		return runComputerIDs(args, func(ids []int) error {
			return buddy.ComputerCertificates(credName, opts, ids, log)
		})
	case "eas":
		if len(args) < 1 {
			return missingArg("computer-id", clispec.UsageHelp(clispec.CmdComputer))
		}
		filter, rest, err := extractStringFlag(args[1:], clispec.FlagFilter)
		if err != nil {
			return err
		}
		asJSON, rest := extractBareFlag(rest, clispec.FlagJSON)
		if len(rest) > 0 {
			return fmt.Errorf("unknown eas flag %q; run '%s'", rest[0], clispec.UsageHelp(clispec.CmdComputer))
		}
		return buddy.ComputerExtensionAttributesList(credName, args[0], filter, asJSON, log)
	case clispec.SubSet:
		if len(args) < 1 {
			return missingArg("property", clispec.UsageHelp(clispec.CmdComputer))
		}
		if args[0] != "ea" {
			return fmt.Errorf("unknown computer set property: %q (use: ea)", args[0])
		}
		if len(args) < 4 {
			return missingArg("computer-id, ea-id|name, or value", clispec.UsageHelp(clispec.CmdComputer))
		}
		spec := buddy.ComputerSetExtensionAttributeSpec{
			ComputerID: args[1],
			Values:     []string{args[3]},
		}
		key := strings.TrimSpace(args[2])
		if computerEAKeyIsID(key) {
			spec.EAID = key
		} else {
			spec.EAName = key
		}
		return buddy.ComputerSetExtensionAttribute(credName, opts, spec, log)
	default:
		return errUnknownSubcommand("computer", subcommand, []string{
			clispec.SubList, clispec.SubGet, "detail", "history", "logs", "eas", clispec.SubSet, "reboot", "redeploy", "shutdown", "lock", "logout", "erase",
			"device-info", "device-location", "security-info", "installed-apps", "managed-apps", "certificates",
		})
	}
}

func runPackageSet(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	attrs, positional, err := parsePackageAttrFlags(args)
	if err != nil {
		return err
	}
	if len(positional) != 1 {
		return fmt.Errorf("usage: buddy package set <package-id> [%s] [--notes=] [--info=] [%s] [--priority=] [--reboot=true|false]", clispec.FlagPrefix(clispec.FlagName), clispec.FlagPrefix(clispec.FlagCategory))
	}
	return buddy.PackageSetAttrs(credName, strings.TrimSpace(positional[0]), opts, attrs, log)
}

func runRole(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if err := resolveArg0("role", args, []string{clispec.SubList, clispec.SubGet, clispec.SubCreate, clispec.SubUpdate, clispec.SubDelete, "privileges", clispec.SubHelp}); err != nil {
		return err
	}
	if len(args) == 0 || args[0] == clispec.SubHelp {
		fmt.Print(helpRole)
		return nil
	}
	subcommand := args[0]
	args = args[1:]

	switch subcommand {
	case clispec.SubList:
		filter, rest, err := extractStringFlag(args, clispec.FlagFilter)
		if err != nil {
			return err
		}
		_, rest = extractBareFlag(rest, clispec.FlagJSON) // RoleList is always JSON
		if len(rest) > 0 {
			return fmt.Errorf("unknown list flag %q; run '%s'", rest[0], clispec.UsageHelp(clispec.CmdRole))
		}
		return buddy.RoleList(credName, filter, log)
	case clispec.SubGet:
		if len(args) < 1 {
			return missingArg("role-id", clispec.UsageHelp(clispec.CmdRole))
		}
		return buddy.RoleGet(credName, args[0], log)
	case clispec.SubCreate:
		if len(args) < 1 {
			return missingArg("displayName", clispec.UsageHelp(clispec.CmdRole))
		}
		displayName := args[0]
		privileges := args[1:]
		if privileges == nil {
			privileges = []string{}
		}
		return buddy.RoleCreate(credName, opts, displayName, privileges, log)
	case clispec.SubUpdate:
		if len(args) < 2 {
			return missingArg("role-id or displayName", clispec.UsageHelp(clispec.CmdRole))
		}
		roleID := args[0]
		displayName := args[1]
		privileges := args[2:]
		if privileges == nil {
			privileges = []string{}
		}
		return buddy.RoleUpdate(credName, roleID, opts, displayName, privileges, log)
	case clispec.SubDelete:
		if len(args) < 1 {
			return missingArg("role-id", clispec.UsageHelp(clispec.CmdRole))
		}
		return buddy.RoleDelete(credName, args[0], opts, log)
	case "privileges":
		return buddy.RolePrivileges(credName, log)
	default:
		return errUnknownSubcommand("role", subcommand, []string{clispec.SubList, clispec.SubGet, clispec.SubCreate, clispec.SubUpdate, clispec.SubDelete, "privileges"})
	}
}

func runClient(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if err := resolveArg0("client", args, []string{clispec.SubList, clispec.SubGet, clispec.SubCreate, clispec.SubUpdate, clispec.SubDelete, "credentials", clispec.SubSet, clispec.SubHelp}); err != nil {
		return err
	}
	if len(args) == 0 || args[0] == clispec.SubHelp {
		fmt.Print(helpClient)
		return nil
	}
	subcommand := args[0]
	args = args[1:]

	switch subcommand {
	case clispec.SubList:
		filter, rest, err := extractStringFlag(args, clispec.FlagFilter)
		if err != nil {
			return err
		}
		_, rest = extractBareFlag(rest, clispec.FlagJSON) // ClientList is always JSON
		if len(rest) > 0 {
			return fmt.Errorf("unknown list flag %q; run '%s'", rest[0], clispec.UsageHelp(clispec.CmdClient))
		}
		return buddy.ClientList(credName, filter, log)
	case clispec.SubGet:
		if len(args) < 1 {
			return missingArg("client-id", clispec.UsageHelp(clispec.CmdClient))
		}
		return buddy.ClientGet(credName, args[0], log)
	case clispec.SubCreate:
		if len(args) < 1 {
			return missingArg("displayName", clispec.UsageHelp(clispec.CmdClient))
		}
		displayName := args[0]
		scopes := args[1:]
		if scopes == nil {
			scopes = []string{}
		}
		return buddy.ClientCreate(credName, opts, displayName, scopes, log)
	case clispec.SubUpdate:
		if len(args) < 2 {
			return missingArg("client-id or displayName", clispec.UsageHelp(clispec.CmdClient))
		}
		clientID := args[0]
		displayName := args[1]
		scopes := args[2:]
		if scopes == nil {
			scopes = []string{}
		}
		return buddy.ClientUpdate(credName, clientID, opts, displayName, scopes, log)
	case clispec.SubDelete:
		if len(args) < 1 {
			return missingArg("client-id", clispec.UsageHelp(clispec.CmdClient))
		}
		return buddy.ClientDelete(credName, args[0], opts, log)
	case "credentials":
		if len(args) < 1 {
			return missingArg("client-id", clispec.UsageHelp(clispec.CmdClient))
		}
		return buddy.ClientCredentials(credName, args[0], opts, log)
	case clispec.SubSet:
		if len(args) < 3 {
			return missingArg("property, client-id, or value", clispec.UsageHelp(clispec.CmdClient))
		}
		return runClientSet(credName, opts, args, log)
	default:
		return errUnknownSubcommand("client", subcommand, []string{clispec.SubList, clispec.SubGet, clispec.SubCreate, clispec.SubUpdate, clispec.SubDelete, "credentials", clispec.SubSet})
	}
}

func runClientSet(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	property := args[0]
	clientID := args[1]
	value := args[2]
	switch property {
	case "token-lifetime":
		secs, err := strconv.Atoi(value)
		if err != nil || secs <= 0 {
			return fmt.Errorf("token-lifetime must be a positive integer (seconds), got %q", value)
		}
		return buddy.ClientSetTokenLifetime(credName, clientID, opts, secs, log)
	default:
		return fmt.Errorf("unknown client set property: %q (use: token-lifetime)", property)
	}
}

func runJamfScript(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if err := resolveArg0("script", args, []string{clispec.SubList, clispec.SubGet, "history", clispec.SubCreate, clispec.SubUpload, clispec.SubUpdate, "download", clispec.SubDelete, clispec.SubSet, clispec.SubHelp}); err != nil {
		return err
	}
	if len(args) == 0 || args[0] == clispec.SubHelp {
		fmt.Print(helpScript)
		return nil
	}
	subcommand := args[0]
	rest := args[1:]

	// clispec.UsageHelp(clispec.CmdScript, clispec.SubGet) / clispec.UsageHelp(clispec.CmdScript, clispec.SubSet)
	if (subcommand == clispec.SubGet || subcommand == clispec.SubSet) && (len(rest) == 0 || rest[0] == clispec.SubHelp) {
		fmt.Print(scriptSubHelp(subcommand))
		return nil
	}
	args = rest

	switch subcommand {
	case clispec.SubList:
		filter, rest, err := extractStringFlag(args, clispec.FlagFilter)
		if err != nil {
			return err
		}
		asJSON, rest := extractBareFlag(rest, clispec.FlagJSON)
		if len(rest) > 0 {
			return fmt.Errorf("unknown list flag %q; run '%s'", rest[0], clispec.UsageHelp(clispec.CmdScript))
		}
		return buddy.ScriptList(credName, filter, asJSON, log)

	case clispec.SubGet:
		if len(args) < 1 {
			return missingArg("script-id", clispec.UsageHelp(clispec.CmdScript, clispec.SubGet))
		}
		return runJamfScriptGet(credName, opts, args[0], args[1:], log)

	case "history":
		if len(args) < 1 {
			return missingArg("script-id", clispec.UsageHelp(clispec.CmdScript))
		}
		return buddy.ScriptHistory(credName, args[0], 20, log)

	case clispec.SubCreate:
		var category, priority, info, notes string
		var positional []string
		for _, a := range args {
			switch {
			case strings.HasPrefix(a, clispec.FlagPrefix(clispec.FlagCategory)):
				category = strings.TrimPrefix(a, clispec.FlagPrefix(clispec.FlagCategory))
			case strings.HasPrefix(a, "--priority="):
				priority = strings.TrimPrefix(a, "--priority=")
			case strings.HasPrefix(a, "--info="):
				info = strings.TrimPrefix(a, "--info=")
			case strings.HasPrefix(a, "--notes="):
				notes = strings.TrimPrefix(a, "--notes=")
			default:
				positional = append(positional, a)
			}
		}
		if len(positional) < 2 {
			return missingArg("name or file", clispec.UsageHelp(clispec.CmdScript))
		}
		return buddy.ScriptCreate(credName, opts, positional[0], positional[1], category, priority, info, notes, log)

	case "download":
		if len(args) < 1 {
			return missingArg("script-id", clispec.UsageHelp(clispec.CmdScript))
		}
		outfile := ""
		if len(args) >= 2 {
			outfile = strings.TrimSpace(args[1])
		}
		return buddy.ScriptDownload(credName, args[0], outfile, log)

	case "upload":
		var nameFlag string
		var positional []string
		for _, a := range args {
			if strings.HasPrefix(a, clispec.FlagPrefix(clispec.FlagName)) {
				nameFlag = strings.TrimPrefix(a, clispec.FlagPrefix(clispec.FlagName))
			} else {
				positional = append(positional, a)
			}
		}
		if len(positional) < 2 {
			return missingArg("script-id or file", clispec.UsageHelp(clispec.CmdScript))
		}
		if err := buddy.ScriptUpdate(credName, opts, positional[0], positional[1], log); err != nil {
			return err
		}
		if nameFlag != "" {
			return buddy.ScriptSetName(credName, positional[0], opts, nameFlag, log)
		}
		return nil

	case clispec.SubDelete:
		if len(args) < 1 {
			return missingArg("script-id", clispec.UsageHelp(clispec.CmdScript))
		}
		return buddy.ScriptDelete(credName, opts, args, log)

	case clispec.SubSet:
		if len(args) < 3 {
			return missingArg("property, script-id, or value", clispec.UsageHelp(clispec.CmdScript, clispec.SubSet))
		}
		return runJamfScriptSet(credName, opts, args, log)

	default:
		return errUnknownSubcommand("script", subcommand, []string{clispec.SubList, clispec.SubGet, "history", clispec.SubCreate, clispec.SubUpload, clispec.SubUpdate, "download", clispec.SubDelete, clispec.SubSet})
	}
}

func runJamfScriptGet(credName string, opts buddy.Opts, subcommand string, args []string, log *slog.Logger) error {
	if subcommand == clispec.SubHelp {
		fmt.Print(helpScriptGet)
		return nil
	}
	switch subcommand {
	case "contents":
		if len(args) < 1 {
			return missingArg("script-id", clispec.UsageHelp(clispec.CmdScript, clispec.SubGet))
		}
		return buddy.ScriptGetContents(credName, args[0], log)
	case "id":
		if len(args) < 1 {
			return missingArg("name", clispec.UsageHelp(clispec.CmdScript, clispec.SubGet))
		}
		id, err := buddy.ScriptGetID(credName, args[0], log)
		if err != nil {
			return err
		}
		fmt.Println(id)
		return nil
	default:
		// treat subcommand as a script ID
		return buddy.ScriptGet(credName, subcommand, log)
	}
}

func runJamfScriptSet(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if len(args) > 0 && args[0] == clispec.SubHelp {
		fmt.Print(helpScriptSet)
		return nil
	}
	property := args[0]
	scriptID := strings.TrimSpace(args[1])
	value := strings.TrimSpace(args[2])

	switch property {
	case "name":
		return buddy.ScriptSetName(credName, scriptID, opts, value, log)
	case "category":
		return buddy.ScriptSetCategory(credName, scriptID, opts, value, log)
	case "priority":
		return buddy.ScriptSetPriority(credName, scriptID, opts, value, log)
	case "info":
		return buddy.ScriptSetInfo(credName, scriptID, opts, value, log)
	case "notes":
		return buddy.ScriptSetNotes(credName, scriptID, opts, value, log)
	default:
		return fmt.Errorf("unknown script set property: %q (use: name, category, priority, info, notes)", property)
	}
}

func printUsage() {
	fmt.Fprint(os.Stderr, help.Root)
}

func runBlueprint(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if err := resolveArg0("blueprint", args, []string{clispec.SubList, clispec.SubGet, clispec.SubCreate, clispec.SubSet, clispec.SubDelete, "deploy", "undeploy", "status", "components", clispec.SubHelp}); err != nil {
		return err
	}
	if len(args) == 0 || args[0] == clispec.SubHelp {
		fmt.Print(helpBlueprint)
		return nil
	}
	subcommand := args[0]
	args = args[1:]

	switch subcommand {
	case clispec.SubList:
		listOpts := buddy.BlueprintListOpts{Page: buddy.PageOpts{Page: buddy.PageAll}}
		for _, a := range args {
			switch {
			case strings.HasPrefix(a, "--search="):
				listOpts.Search = strings.TrimPrefix(a, "--search=")
			case strings.HasPrefix(a, "--sort="):
				listOpts.Sort = strings.TrimPrefix(a, "--sort=")
			default:
				return fmt.Errorf("unknown list flag %q; run '%s'", a, clispec.UsageHelp(clispec.CmdBlueprint))
			}
		}
		return buddy.BlueprintList(credName, listOpts, log)
	case clispec.SubGet:
		if len(args) < 1 {
			return missingArgHint("blueprint-id", clispec.UsageHelp(clispec.CmdBlueprint))
		}
		return buddy.BlueprintGet(credName, args[0], log)
	case clispec.SubCreate:
		file := ""
		for _, a := range args {
			switch {
			case strings.HasPrefix(a, clispec.FlagPrefix(clispec.FlagFile)):
				file = strings.TrimPrefix(a, clispec.FlagPrefix(clispec.FlagFile))
			case a == "-":
				file = "-"
			default:
				return fmt.Errorf("unknown create arg %q; use %s or - for stdin", a, clispec.FlagEq(clispec.FlagFile, "path"))
			}
		}
		if file == "" {
			file = "-"
		}
		body, err := buddy.ReadJSONBodyFile(file)
		if err != nil {
			return err
		}
		return buddy.BlueprintCreate(credName, opts, body, log)
	case clispec.SubSet:
		if len(args) < 1 {
			return missingArgHint("blueprint-id", clispec.UsageHelp(clispec.CmdBlueprint))
		}
		blueprintID := args[0]
		file := "-"
		for _, a := range args[1:] {
			switch {
			case strings.HasPrefix(a, clispec.FlagPrefix(clispec.FlagFile)):
				file = strings.TrimPrefix(a, clispec.FlagPrefix(clispec.FlagFile))
			case a == "-":
				file = "-"
			default:
				return fmt.Errorf("unknown set arg %q; use %s or - for stdin", a, clispec.FlagEq(clispec.FlagFile, "path"))
			}
		}
		body, err := buddy.ReadJSONBodyFile(file)
		if err != nil {
			return err
		}
		return buddy.BlueprintSet(credName, blueprintID, opts, body, log)
	case clispec.SubDelete:
		if len(args) < 1 {
			return missingArgHint("blueprint-id", clispec.UsageHelp(clispec.CmdBlueprint))
		}
		return buddy.BlueprintDelete(credName, args[0], opts, log)
	case "deploy":
		if len(args) < 1 {
			return missingArgHint("blueprint-id", clispec.UsageHelp(clispec.CmdBlueprint))
		}
		return buddy.BlueprintDeploy(credName, args[0], opts, log)
	case "undeploy":
		if len(args) < 1 {
			return missingArgHint("blueprint-id", clispec.UsageHelp(clispec.CmdBlueprint))
		}
		return buddy.BlueprintUndeploy(credName, args[0], opts, log)
	case "status":
		if len(args) < 1 {
			return missingArgHint("blueprint-id", clispec.UsageHelp(clispec.CmdBlueprint))
		}
		return buddy.BlueprintStatus(credName, args[0], log)
	case "components":
		identifier := ""
		if len(args) >= 1 {
			identifier = args[0]
		}
		return buddy.BlueprintComponents(credName, identifier, log)
	default:
		return errUnknownSubcommand("blueprint", subcommand, []string{
			clispec.SubList, clispec.SubGet, clispec.SubCreate, clispec.SubSet, clispec.SubDelete, "deploy", "undeploy", "status", "components",
		})
	}
}

func runAuth(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if err := resolveArg0("auth", args, []string{"token", clispec.SubTest, clispec.SubAuthorization, clispec.SubRefresh, "caps", clispec.SubHelp}); err != nil {
		return err
	}
	if len(args) == 0 || args[0] == clispec.SubHelp {
		fmt.Print(helpAuth)
		return nil
	}
	switch args[0] {
	case clispec.SubTest:
		all := false
		for _, a := range args[1:] {
			switch a {
			case "--all":
				all = true
			case clispec.SubHelp:
				fmt.Print(helpAuth)
				return nil
			default:
				return fmt.Errorf("unknown argument %q; usage: buddy auth test [--all]", a)
			}
		}
		if all {
			cfg := buddy.GetActiveConfig()
			results, err := buddy.AuthTestAll(cfg, log)
			if len(results) == 0 {
				return fmt.Errorf("no covering auths to test — run 'buddy cred status'")
			}
			buddy.PrintAuthTestAll(results)
			return err
		}
		return buddy.AuthTest(credName, log)
	case clispec.SubAuthorization:
		all := false
		for _, a := range args[1:] {
			switch a {
			case "--all":
				all = true
			case clispec.SubHelp:
				fmt.Print(helpAuth)
				return nil
			default:
				return fmt.Errorf("unknown argument %q; usage: buddy auth authorization [--all]", a)
			}
		}
		return buddy.PrintAuthCaps(buddy.GetActiveConfig(), credName, all)
	case "token":
		if len(args) > 1 && args[1] == clispec.SubHelp {
			fmt.Print(helpAuth)
			return nil
		}
		return runAuthToken(credName, args[1:], log)
	case clispec.SubRefresh:
		return fmt.Errorf("moved to 'buddy auth token refresh'; see 'buddy auth help'")
	case "caps":
		return fmt.Errorf("unknown auth subcommand %q (use: authorization)", args[0])
	default:
		return errUnknownSubcommand("auth", args[0], []string{"token", clispec.SubTest, clispec.SubAuthorization, clispec.SubHelp})
	}
}

func runToken(args []string, log *slog.Logger) error {
	if err := resolveArg0("token", args, []string{clispec.SubDirect, clispec.SubHelp}); err != nil {
		return err
	}
	if len(args) == 0 || args[0] == clispec.SubHelp {
		fmt.Print(helpToken)
		return nil
	}
	switch args[0] {
	case clispec.SubDirect:
		if len(args) > 1 && args[1] == clispec.SubHelp {
			fmt.Print(helpToken)
			return nil
		}
		return runTokenDirect(args[1:], log)
	default:
		return errUnknownSubcommand("token", args[0], []string{clispec.SubDirect, clispec.SubHelp})
	}
}

func runTokenDirect(args []string, log *slog.Logger) error {
	fs := flag.NewFlagSet("token direct", flag.ContinueOnError)
	baseURL := fs.String("url", "", "Jamf Pro or Platform API Gateway base URL")
	region := fs.String("region", "", "Platform API Gateway region (builds URL when --url omitted)")
	environmentID := fs.String("environment-id", "", "Platform environment id (preferred)")
	tenantID := fs.String("tenant-id", "", "Platform tenant id (legacy single-product scope)")
	username := fs.String("username", "", "Username for basic auth")
	password := fs.String("password", "", "Password for basic auth")
	clientID := fs.String("client-id", "", "OAuth2 client ID")
	clientSecret := fs.String("client-secret", "", "OAuth2 client secret")
	oauth2CredsFile := fs.String("oauth2-creds-file", "", "Path to OAuth2 credentials JSON file")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	direct := buddy.DirectAuthOpts{
		BaseURL:         *baseURL,
		Region:          *region,
		EnvironmentID:   *environmentID,
		TenantID:        *tenantID,
		Username:        *username,
		Password:        *password,
		ClientID:        *clientID,
		ClientSecret:    *clientSecret,
		OAuth2CredsFile: *oauth2CredsFile,
	}
	if !direct.UsesDirectAuth() {
		return fmt.Errorf("direct auth requires --url or --region plus username/password or client-id/client-secret; see 'buddy token help'")
	}
	return buddy.PrintRawToken("", direct, log)
}

func runAuthToken(credName string, args []string, log *slog.Logger) error {
	if err := resolveArg0("auth token", args, []string{clispec.SubShow, clispec.SubStatus, clispec.SubDrop, clispec.SubRefresh, clispec.SubHelp}); err != nil {
		return err
	}
	if len(args) == 0 || args[0] == clispec.SubHelp {
		fmt.Print(helpAuth)
		return nil
	}
	switch args[0] {
	case clispec.SubShow:
		if len(args) > 1 && args[1] == clispec.SubHelp {
			fmt.Print(helpAuth)
			return nil
		}
		return runAuthTokenShow(credName, args[1:], log)
	case clispec.SubStatus:
		return runAuthTokenStatus(credName, args[1:])
	case clispec.SubDrop:
		return runAuthTokenDrop(credName, args[1:])
	case clispec.SubRefresh:
		if len(args) > 1 && args[1] == clispec.SubHelp {
			fmt.Print(helpAuth)
			return nil
		}
		return runAuthTokenRefresh(credName, args[1:], log)
	default:
		if movedDirectAuthFlags(args) {
			return fmt.Errorf("direct auth flags moved to 'buddy token direct'; see 'buddy token help'")
		}
		if strings.HasPrefix(args[0], "-") {
			return fmt.Errorf("print a token with 'buddy auth token show [--raw]'; see 'buddy auth help'")
		}
		return errUnknownSubcommand("auth token", args[0], []string{clispec.SubShow, clispec.SubStatus, clispec.SubDrop, clispec.SubRefresh, clispec.SubHelp})
	}
}

func runAuthTokenShow(credName string, args []string, log *slog.Logger) error {
	fs := flag.NewFlagSet("auth token show", flag.ContinueOnError)
	raw := fs.Bool("raw", false, "Print bearer token only (no \"Bearer \" prefix)")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *raw {
		return buddy.PrintRawToken(credName, buddy.DirectAuthOpts{}, log)
	}
	return buddy.GetToken(credName)
}

func movedDirectAuthFlags(args []string) bool {
	for _, a := range args {
		name, _, _ := strings.Cut(a, "=")
		switch name {
		case "--url", "--region", "--environment-id", "--tenant-id", "--username", "--password",
			"--client-id", "--client-secret", "--oauth2-creds-file":
			return true
		}
	}
	return false
}

func parseAuthAllFlag(args []string, usage string) (all bool, err error) {
	for _, a := range args {
		switch a {
		case "--all":
			all = true
		case clispec.SubHelp:
			return false, errAuthHelp
		default:
			return false, fmt.Errorf("unknown argument %q; usage: %s", a, usage)
		}
	}
	return all, nil
}

var errAuthHelp = fmt.Errorf("auth help")

func runAuthTokenStatus(credName string, args []string) error {
	all, err := parseAuthAllFlag(args, "buddy auth token status [--all]")
	if err != nil {
		if err == errAuthHelp {
			fmt.Print(helpAuth)
			return nil
		}
		return err
	}
	return buddy.PrintTokenStatus(credName, all)
}

func runAuthTokenDrop(credName string, args []string) error {
	all, err := parseAuthAllFlag(args, "buddy auth token drop [--all]")
	if err != nil {
		if err == errAuthHelp {
			fmt.Print(helpAuth)
			return nil
		}
		return err
	}
	return buddy.DropTokenCache(credName, all)
}

func runAuthTokenRefresh(credName string, args []string, log *slog.Logger) error {
	fs := flag.NewFlagSet("auth token refresh", flag.ContinueOnError)
	raw := fs.Bool("raw", false, "Print bearer token only (no \"Bearer \" prefix)")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected argument %q; usage: buddy auth token refresh [--raw]", fs.Arg(0))
	}
	res, err := buddy.AuthRefresh(credName, log)
	if err != nil {
		return err
	}
	if *raw {
		fmt.Println(res.Token)
		return nil
	}
	buddy.PrintAuthRefresh(res)
	return nil
}

func runSetup(cfg *buddy.Config, opts buddy.Opts, credName string, args []string, log *slog.Logger) error {
	setupSubs := []string{"import", "export", "secrets", "managed-plist", clispec.SubHelp}
	if err := resolveArg0("setup", args, setupSubs); err != nil {
		return err
	}
	if len(args) == 0 {
		if buddy.ConfigNeedsSetup(cfg) {
			_, err := buddy.RunFirstTimeSetup(cfg, opts, log)
			return err
		}
		fmt.Print(helpSetup)
		return nil
	}
	if args[0] == clispec.SubHelp {
		fmt.Print(helpSetup)
		return nil
	}
	subcommand := args[0]
	args = args[1:]
	switch subcommand {
	case "import":
		if len(args) == 0 {
			return fmt.Errorf("setup import requires a YAML or plist path")
		}
		path := args[0]
		if err := buddy.ImportQuickstart(cfg, path); err != nil {
			return err
		}
		fmt.Printf("Imported identity from %s\n", path)
		fmt.Println()
		fmt.Println("Creates a missing auth for any workplace that has none, then stores")
		fmt.Println("missing passwords / OAuth client secrets in the vault.")
		fmt.Println()
		res, err := buddy.SetupSecrets(cfg, buddy.SetupSecretsOpts{SkipConfirm: opts.SkipConfirm})
		printSetupSecrets(res)
		if err != nil {
			return err
		}
		fmt.Println()
		fmt.Println("Try: buddy auth test")
		return nil
	case "export":
		path := ""
		if len(args) > 0 {
			path = args[0]
		}
		data, err := buddy.MarshalQuickstartYAML(cfg)
		if err != nil {
			return fmt.Errorf("quickstart yaml: %w", err)
		}
		if path == "" {
			_, err := os.Stdout.Write(data)
			return err
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return err
		}
		fmt.Printf("Wrote %s\n", path)
		fmt.Println("Secrets-free identity (orgs/contexts/channels; no auths). Recipients: buddy setup import")
		return nil
	case "secrets":
		fmt.Println("buddy setup secrets")
		fmt.Println()
		fmt.Println("Creates a missing auth for any tenant that has none, then stores")
		fmt.Println("missing passwords / OAuth client secrets in the vault.")
		fmt.Println("MDM profiles can include client IDs or secrets; this command fills gaps.")
		fmt.Println()
		res, err := buddy.SetupSecrets(cfg, buddy.SetupSecretsOpts{SkipConfirm: opts.SkipConfirm})
		printSetupSecrets(res)
		if err != nil {
			return err
		}
		fmt.Println()
		fmt.Println("Try: buddy auth test")
		return nil
	case "managed-plist":
		path := ""
		if len(args) > 0 {
			path = args[0]
		}
		data, err := buddy.MarshalManagedPlist(cfg)
		if err != nil {
			return fmt.Errorf("managed plist: %w", err)
		}
		if path == "" {
			_, err := os.Stdout.Write(data)
			return err
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return err
		}
		fmt.Printf("Wrote %s\n", path)
		fmt.Printf("Upload as Jamf Application & Custom Settings for domain %s\n", buddy.ManagedPrefsDomain)
		fmt.Println("Auths omitted (per-user); tenants and channel policy are included.")
		return nil
	default:
		return errUnknownSubcommand("setup", subcommand, setupSubs)
	}
}

func printSetupSecrets(res *buddy.SetupSecretsResult) {
	if res == nil {
		return
	}
	for _, name := range res.CreatedAuths {
		fmt.Printf("Created auth %q\n", name)
	}
	for _, name := range res.UpdatedAuths {
		fmt.Printf("Updated auth %q\n", name)
	}
	for _, s := range res.StoredSecrets {
		fmt.Printf("Stored %s\n", s)
	}
	if len(res.Missing) > 0 {
		return
	}
	if len(res.CreatedAuths) == 0 && len(res.StoredSecrets) == 0 && len(res.UpdatedAuths) == 0 {
		fmt.Println("All tenants have an auth and required secrets are stored.")
	}
}

func runMdm(credName string, _ buddy.Opts, args []string, log *slog.Logger) error {
	if err := resolveArg0("mdm", args, []string{clispec.SubList, clispec.SubHelp}); err != nil {
		return err
	}
	if len(args) == 0 || args[0] == clispec.SubHelp {
		fmt.Print(helpMdm)
		return nil
	}
	subcommand := args[0]
	args = args[1:]

	switch subcommand {
	case clispec.SubList:
		var filter, sort string
		var pageSize, limit int
		var count bool
		fs := flag.NewFlagSet("mdm list", flag.ContinueOnError)
		fs.StringVar(&filter, clispec.FlagFilter, "", "RSQL filter (required; at least one field)")
		fs.StringVar(&sort, clispec.FlagSort, "dateSent:asc", "Sort expression (e.g. dateSent:desc)")
		fs.IntVar(&pageSize, clispec.FlagPageSize, 200, "Records per API request; pagination iterates until all results are fetched")
		fs.IntVar(&limit, clispec.FlagLimit, 0, "Maximum total records to return (0 = no limit)")
		fs.BoolVar(&count, "count", false, "Print only the total count of matching records (single API call)")
		asJSON := fs.Bool(clispec.FlagJSON, false, "Print matching rows as JSON")
		if err := fs.Parse(args); err != nil {
			return err
		}
		if filter == "" {
			return fmt.Errorf("%s is required; run '%s' for usage", clispec.Flag(clispec.FlagFilter), clispec.UsageHelp(clispec.CmdMdm))
		}
		pageOpts := buddy.PageOpts{Page: buddy.PageAll, PageSize: pageSize}
		if count {
			records, err := buddy.MdmCommandFind(credName, filter, sort, pageOpts, limit, log)
			if err != nil {
				return err
			}
			fmt.Printf("%d\n", len(records))
			return nil
		}
		return buddy.MdmCommandFindPrint(credName, filter, sort, pageOpts, limit, *asJSON, log)
	default:
		return errUnknownSubcommand("mdm", subcommand, []string{clispec.SubList})
	}
}
