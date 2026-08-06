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

	"buddy/internal/buddy"
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
	channel := buddy.ChannelCLI
	if len(args) > 0 {
		switch args[0] {
		case "mcp":
			channel = buddy.ChannelMCP
		case "serve":
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
		if err := script.Run(credName, opts, scriptPath, scriptArgs, logger, trace); err != nil {
			logger.Error("Script failed", "error", err)
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		return
	}

	switch args[0] {
	case "help":
		if len(args) > 1 {
			printCommandHelp(args[1:])
		} else {
			printUsage()
		}
		os.Exit(0)
	case "version":
		// Also handled before config load; keep for completeness if dispatch is reached.
		fmt.Println(Version)
		os.Exit(0)
	case "auth":
		if err := runAuth(credName, opts, args[1:], logger); err != nil {
			logger.Error("Auth command failed", "error", err)
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	case "cred":
		if err := runCred(cfg, opts, args[1:], logger); err != nil {
			logger.Error("Cred command failed", "error", err)
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	case "policy":
		if err := runPolicy(credName, opts, args[1:], buddy.GetLogger()); err != nil {
			logger.Error("Policy command failed", "error", err)
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	case "category":
		if err := runCategory(credName, opts, args[1:], buddy.GetLogger()); err != nil {
			logger.Error("Category command failed", "error", err)
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	case "macapp":
		if err := runMacApp(credName, opts, args[1:], buddy.GetLogger()); err != nil {
			logger.Error("Macapp command failed", "error", err)
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	case "app":
		if err := runApp(credName, opts, args[1:], buddy.GetLogger()); err != nil {
			logger.Error("App command failed", "error", err)
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	case "group":
		if err := runGroup(credName, opts, args[1:], buddy.GetLogger()); err != nil {
			logger.Error("Group command failed", "error", err)
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	case "profile":
		if err := runProfile(credName, opts, args[1:], buddy.GetLogger()); err != nil {
			logger.Error("Profile command failed", "error", err)
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	case "package":
		if err := runPackage(credName, opts, args[1:], buddy.GetLogger()); err != nil {
			logger.Error("Package command failed", "error", err)
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	case "computer":
		if err := runComputer(credName, opts, args[1:], buddy.GetLogger()); err != nil {
			logger.Error("Computer command failed", "error", err)
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	case "role":
		if err := runRole(credName, opts, args[1:], buddy.GetLogger()); err != nil {
			logger.Error("Role command failed", "error", err)
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	case "client":
		if err := runClient(credName, opts, args[1:], buddy.GetLogger()); err != nil {
			logger.Error("Client command failed", "error", err)
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	case "script":
		if err := runJamfScript(credName, opts, args[1:], buddy.GetLogger()); err != nil {
			logger.Error("Script command failed", "error", err)
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	case "mdm":
		if err := runMdm(credName, opts, args[1:], buddy.GetLogger()); err != nil {
			logger.Error("MDM command failed", "error", err)
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	case "blueprint":
		if err := runBlueprint(credName, opts, args[1:], buddy.GetLogger()); err != nil {
			logger.Error("Blueprint command failed", "error", err)
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	case "device":
		if err := runDevice(credName, opts, args[1:], buddy.GetLogger()); err != nil {
			logger.Error("Device command failed", "error", err)
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	case "device-group":
		if err := runDeviceGroup(credName, opts, args[1:], buddy.GetLogger()); err != nil {
			logger.Error("Device-group command failed", "error", err)
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	case "ddm":
		if err := runDDM(credName, opts, args[1:], buddy.GetLogger()); err != nil {
			logger.Error("DDM command failed", "error", err)
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	case "benchmark":
		if err := runBenchmark(credName, opts, args[1:], buddy.GetLogger()); err != nil {
			logger.Error("Benchmark command failed", "error", err)
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	case "setup":
		if err := runSetup(cfg, opts, credName, args[1:], logger); err != nil {
			logger.Error("Setup command failed", "error", err)
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	case "serve":
		if err := runServe(credName, opts, args[1:], logger); err != nil {
			logger.Error("Serve failed", "error", err)
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	case "mcp":
		if err := runMCP(credName, opts, args[1:], logger); err != nil {
			logger.Error("MCP failed", "error", err)
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	case "lsp":
		if err := runLSP(args[1:], logger); err != nil {
			logger.Error("LSP failed", "error", err)
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %q", args[0])
		if s := suggestTopLevelCommand(args[0]); s != "" {
			fmt.Fprintf(os.Stderr, "\nDid you mean %q?", s)
		}
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Run 'buddy help' for a list of commands.")
		os.Exit(1)
	}
}

func runPolicy(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if len(args) == 0 || args[0] == "help" {
		fmt.Print(helpPolicy)
		return nil
	}
	mode := args[0]
	rest := args[1:]

	// list is valid with no further arguments; handle it before the sub-help fallthrough.
	if mode == "list" {
		if len(rest) > 0 && rest[0] == "help" {
			fmt.Print(helpPolicyList)
			return nil
		}
		return runPolicyList(credName, opts, rest, log)
	}
	if mode == "find" {
		return fmt.Errorf("buddy policy find was removed; use 'buddy policy list [--filter=<keyword>] [--ids]'")
	}

	// "buddy policy <mode> help" — print subcommand help
	if len(rest) == 0 || rest[0] == "help" {
		fmt.Print(policySubHelp(mode))
		return nil
	}

	subcommand := rest[0]
	args = rest[1:]

	switch mode {
	case "get":
		return runPolicyGet(credName, opts, rest, log)
	case "set":
		return runPolicySet(credName, opts, subcommand, args, log)
	case "clone":
		return fmt.Errorf("buddy policy clone was removed; use change batches (step_clone_policy / step_set_package) or policy_copy + policy_set_package in buddyscript")
	case "create":
		return runPolicyCreate(credName, opts, subcommand, args, log)
	case "delete":
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
		cands := []string{"list", "get", "set", "create", "delete"}
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
	for i := 0; i < len(args); i++ {
		a := args[i]
		if val, next, matched, err := takeFlagValue(args, i, "filter"); matched {
			if err != nil {
				return err
			}
			keywords = append(keywords, strings.Fields(val)...)
			i = next - 1
			continue
		}
		if val, next, matched, err := takeFlagValue(args, i, "exclude-ids"); matched {
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
		case a == "--ids":
			idsOnly = true
		default:
			return fmt.Errorf("unexpected argument %q; use --filter=<keyword> (run 'buddy policy list help' for usage)", a)
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
	fmt.Printf("%-8s  %s\n", "ID", "Name")
	fmt.Printf("%-8s  %s\n", "--------", "----------------------------------------")
	for _, s := range summaries {
		fmt.Printf("%-8d  %s\n", s.ID, s.Name)
	}
	return nil
}

func runPolicyGet(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if len(args) > 0 && args[0] == "help" {
		fmt.Print(helpPolicyGet)
		return nil
	}
	for _, a := range args {
		if a == "--trigger" || strings.HasPrefix(a, "--trigger=") {
			return fmt.Errorf("buddy policy get --trigger was removed; use 'buddy policy get id <trigger>'")
		}
	}
	if len(args) < 1 {
		return fmt.Errorf("missing arguments: use <policy-id>, or 'id <trigger>', 'enable <policy-id>', or 'xml <policy-id>'; run 'buddy policy get help'")
	}
	switch args[0] {
	case "id":
		if len(args) < 2 {
			return fmt.Errorf("missing trigger; run 'buddy policy get help' for usage")
		}
		trigger := strings.TrimSpace(args[1])
		if trigger == "" {
			return fmt.Errorf("missing trigger; run 'buddy policy get help' for usage")
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
			return fmt.Errorf("missing policy-id; run 'buddy policy get help' for usage")
		}
		policyID, err := parseInt(args[1], "policy-id")
		if err != nil {
			return err
		}
		return buddy.PolicyGetXML(credName, policyID, os.Stdout, log)
	case "enable":
		if len(args) < 2 {
			return fmt.Errorf("missing policy-id; run 'buddy policy get help' for usage")
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

func runPolicySet(credName string, opts buddy.Opts, subcommand string, args []string, log *slog.Logger) error {
	if subcommand == "help" {
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

	case "enable":
		if len(args) < 1 {
			return fmt.Errorf("missing policy-id; run 'buddy policy set help' for usage")
		}
		policyID, err := parseInt(args[0], "policy-id")
		if err != nil {
			return err
		}
		return buddy.PolicyEnable(credName, opts, policyID, true, log)

	case "disable":
		if len(args) < 1 {
			return fmt.Errorf("missing policy-id; run 'buddy policy set help' for usage")
		}
		policyID, err := parseInt(args[0], "policy-id")
		if err != nil {
			return err
		}
		return buddy.PolicyEnable(credName, opts, policyID, false, log)

	case "trigger":
		if len(args) < 2 {
			return fmt.Errorf("missing policy-id or trigger; run 'buddy policy set help' for usage")
		}
		policyID, err := parseInt(args[0], "policy-id")
		if err != nil {
			return err
		}
		newTrigger := strings.TrimSpace(args[1])
		return buddy.PolicyTrigger(credName, opts, policyID, newTrigger, log)

	case "install":
		paramIndices, filtered, err := parseParamArgs(args)
		if err != nil {
			return err
		}
		args = filtered
		if len(args) < 2 {
			return fmt.Errorf("missing policy-id or version; run 'buddy policy set help' for usage")
		}
		policyID, err := parseInt(args[0], "policy-id")
		if err != nil {
			return err
		}
		newVersion := strings.TrimSpace(args[1])
		return buddy.PolicyInstall(credName, opts, policyID, newVersion, paramIndices, log)

	case "xml":
		if len(args) < 1 {
			return fmt.Errorf("missing policy-id; run 'buddy policy set help' for usage")
		}
		policyID, err := parseInt(args[0], "policy-id")
		if err != nil {
			return err
		}
		return buddy.PolicySetXML(credName, opts, policyID, os.Stdin, log)

	case "scope":
		var groupIDsFlag string
		var filtered []string
		for _, a := range args {
			if strings.HasPrefix(a, "--group-ids=") {
				groupIDsFlag = strings.TrimPrefix(a, "--group-ids=")
			} else {
				filtered = append(filtered, a)
			}
		}
		args = filtered
		if len(args) < 2 {
			return fmt.Errorf("missing policy-id or scope; run 'buddy policy set help' for usage")
		}
		policyID, err := parseInt(args[0], "policy-id")
		if err != nil {
			return err
		}
		// "all" sets scope to All Computers
		if len(args) == 2 && strings.ToLower(strings.TrimSpace(args[1])) == "all" {
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
		for _, a := range args[1:] {
			id, err := parseInt(a, "group-id")
			if err != nil {
				return err
			}
			groupIDs = append(groupIDs, id)
		}
		if len(groupIDs) == 0 {
			return fmt.Errorf("at least one computer group ID required (positional or --group-ids=1,2,3)")
		}
		return buddy.PolicyScope(credName, opts, policyID, groupIDs, log)

	case "name":
		if len(args) < 2 {
			return fmt.Errorf("missing policy-id or name; run 'buddy policy set help' for usage")
		}
		policyID, err := parseInt(args[0], "policy-id")
		if err != nil {
			return err
		}
		newName := strings.TrimSpace(strings.Join(args[1:], " "))
		return buddy.PolicySetName(credName, opts, policyID, newName, log)

	case "frequency":
		if len(args) < 2 {
			return fmt.Errorf("missing policy-id or frequency; run 'buddy policy set help' for usage")
		}
		policyID, err := parseInt(args[0], "policy-id")
		if err != nil {
			return err
		}
		frequency := strings.TrimSpace(strings.Join(args[1:], " "))
		if frequency == "" {
			return fmt.Errorf("missing frequency; run 'buddy policy set help' for usage")
		}
		return buddy.PolicySetFrequency(credName, opts, policyID, frequency, log)

	case "icon":
		if len(args) < 2 {
			return fmt.Errorf("missing policy-id or image-file; run 'buddy policy set help' for usage")
		}
		policyID, err := parseInt(args[0], "policy-id")
		if err != nil {
			return err
		}
		filePath := args[1]
		return buddy.PolicySetIcon(credName, opts, policyID, filePath, log)

	case "package":
		var triggerFlag, packageFlag, nameFlag string
		var disableSource, keepSource bool
		var filtered []string
		for _, a := range args {
			switch {
			case strings.HasPrefix(a, "--trigger="):
				triggerFlag = strings.TrimPrefix(a, "--trigger=")
			case strings.HasPrefix(a, "--package="):
				packageFlag = strings.TrimPrefix(a, "--package=")
			case strings.HasPrefix(a, "--name="):
				nameFlag = strings.TrimPrefix(a, "--name=")
			case a == "--disable-source":
				disableSource = true
			case a == "--keep-source":
				keepSource = true
			default:
				filtered = append(filtered, a)
			}
		}
		args = filtered
		disableMode, err := parseDisableSourceFlags(disableSource, keepSource)
		if err != nil {
			return err
		}
		policyID := 0
		var version string
		if triggerFlag != "" {
			if len(args) < 1 {
				return fmt.Errorf("missing version; run 'buddy policy set help' for usage")
			}
			version = strings.TrimSpace(args[0])
			if opts.Verbose {
				fmt.Fprintf(os.Stderr, "Using --trigger=%s to resolve policy\n", triggerFlag)
			}
		} else {
			if len(args) < 2 {
				return fmt.Errorf("missing policy-id or version (or pass --trigger=<trigger> and <version>); run 'buddy policy set help' for usage")
			}
			policyID, err = parseInt(args[0], "policy-id")
			if err != nil {
				return err
			}
			version = strings.TrimSpace(args[1])
		}
		name := strings.ToLower(strings.TrimSpace(nameFlag))
		return buddy.PolicyPackage(credName, opts, name, version, policyID, triggerFlag, packageFlag, disableMode, log)

	default:
		return errUnknownSubcommand("policy set", subcommand, []string{
			"ss", "scope", "enable", "disable", "trigger", "install", "name", "frequency", "icon", "package", "xml",
		})
	}
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
	if subcommand == "help" {
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
			} else if strings.HasPrefix(a, "--category=") {
				categoryFlag = strings.TrimPrefix(a, "--category=")
			} else if strings.HasPrefix(a, "--package=") {
				packageFlag = strings.TrimPrefix(a, "--package=")
			} else {
				filtered = append(filtered, a)
			}
		}
		args = filtered
		if len(args) < 2 {
			return fmt.Errorf("missing name or version; run 'buddy policy create help' for usage")
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
			case strings.HasPrefix(a, "--category="):
				categoryFlag = strings.TrimPrefix(a, "--category=")
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
			return fmt.Errorf("missing trigger or display-name; run 'buddy policy create help' for usage")
		}
		trigger := strings.TrimSpace(args[0])
		displayName := strings.TrimSpace(strings.Join(args[1:], " "))
		if displayName == "" {
			return fmt.Errorf("missing display-name; run 'buddy policy create help' for usage")
		}
		_, err := buddy.PolicyCreateSS(credName, opts, trigger, displayName, categoryFlag, 0, scriptID, log)
		return err
	default:
		return errUnknownSubcommand("policy create", subcommand, []string{"package", "ss"})
	}
}

func runCategory(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if len(args) == 0 || args[0] == "help" {
		fmt.Print(helpCategory)
		return nil
	}
	subcommand := args[0]
	args = args[1:]

	switch subcommand {
	case "list":
		var filter string
		fs := flag.NewFlagSet("category list", flag.ContinueOnError)
		fs.StringVar(&filter, "filter", "", "name substring or RSQL filter")
		if err := fs.Parse(args); err != nil {
			return err
		}
		cats, err := buddy.CategorySearch(credName, filter, buddy.PageOpts{Page: buddy.PageAll}, log)
		if err != nil {
			return err
		}
		fmt.Printf("%-8s  %-8s  %s\n", "ID", "Priority", "Name")
		fmt.Printf("%-8s  %-8s  %s\n", "--------", "--------", "----------------------------------------")
		for _, c := range cats {
			fmt.Printf("%-8s  %-8d  %s\n", c.ID, c.Priority, c.Name)
		}
		return nil
	case "get":
		if len(args) < 1 {
			return fmt.Errorf("missing category-id; run 'buddy category help' for usage")
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
	case "create":
		var priority int
		fs := flag.NewFlagSet("category create", flag.ContinueOnError)
		fs.IntVar(&priority, "priority", 9, "category priority (Jamf default 9)")
		flagArgs, positionals := staticGroupSplitFlags(args)
		if err := fs.Parse(flagArgs); err != nil {
			return err
		}
		if len(positionals) < 1 {
			return fmt.Errorf("missing <name>; run 'buddy category help' for usage")
		}
		name := strings.TrimSpace(strings.Join(positionals, " "))
		_, err := buddy.CategoryPost(credName, opts, buddy.CategoryDetail{Name: name, Priority: priority}, log)
		return err
	case "delete":
		if len(args) < 1 {
			return fmt.Errorf("missing category-id; run 'buddy category help' for usage")
		}
		return buddy.CategoryDelete(credName, opts, args[0], log)
	default:
		return errUnknownSubcommand("category", subcommand, []string{"list", "get", "create", "delete"})
	}
}

func runMacApp(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if len(args) == 0 || args[0] == "help" {
		fmt.Print(helpMacApp)
		return nil
	}
	subcommand := args[0]
	args = args[1:]

	switch subcommand {
	case "list":
		var filter string
		fs := flag.NewFlagSet("macapp list", flag.ContinueOnError)
		fs.StringVar(&filter, "filter", "", "substring matched against name, version, or bundle_id")
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
	case "get":
		if len(args) < 1 {
			return fmt.Errorf("missing macapp-id; run 'buddy macapp help' for usage")
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
	case "create":
		var name, version, bundleID, url, category string
		var free bool
		fs := flag.NewFlagSet("macapp create", flag.ContinueOnError)
		fs.StringVar(&name, "name", "", "display name (required)")
		fs.StringVar(&version, "version", "", "version string (required)")
		fs.StringVar(&bundleID, "bundle-id", "", "bundle identifier (required)")
		fs.StringVar(&url, "url", "", "App Store URL (required)")
		fs.BoolVar(&free, "free", false, "mark as free App Store app")
		fs.StringVar(&category, "category", "", "category name (exact)")
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
	case "delete":
		if len(args) < 1 {
			return fmt.Errorf("missing macapp-id; run 'buddy macapp help' for usage")
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
		return errUnknownSubcommand("macapp", subcommand, []string{"list", "get", "create", "delete"})
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

func runApp(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if len(args) == 0 || args[0] == "help" {
		fmt.Print(helpApp)
		return nil
	}
	subcommand := args[0]
	args = args[1:]

	switch subcommand {
	case "list":
		var filter string
		fs := flag.NewFlagSet("app list", flag.ContinueOnError)
		fs.StringVar(&filter, "filter", "", "substring matched against name, kind, version, trigger, or package")
		if err := fs.Parse(args); err != nil {
			return err
		}
		apps, err := buddy.AppSearch(credName, filter, log)
		if err != nil {
			return err
		}
		fmt.Printf("%-16s  %-8s  %-12s  %-8s  %-24s  %s\n", "Name", "Kind", "Version", "SS ID", "Install trigger", "Package")
		fmt.Printf("%-16s  %-8s  %-12s  %-8s  %-24s  %s\n", "----------------", "--------", "------------", "--------", "------------------------", "--------------------------------")
		for _, a := range apps {
			fmt.Printf("%-16s  %-8s  %-12s  %-8d  %-24s  %s\n",
				truncatePad(a.Name, 16), truncatePad(string(a.Kind), 8), truncatePad(a.Version, 12),
				a.SSPolicyID, truncatePad(a.InstallTrigger, 24), a.PackageName)
		}
		return nil
	case "get":
		if len(args) < 1 {
			return fmt.Errorf("missing app name; run 'buddy app help' for usage")
		}
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
			return fmt.Errorf("missing app name; run 'buddy app help' for usage")
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
			return fmt.Errorf("--version is required; run 'buddy app help' for usage")
		}
		return buddy.AppUpgrade(credName, opts, name, version, packagePattern, log)
	default:
		return errUnknownSubcommand("app", subcommand, []string{"list", "get", "upgrade"})
	}
}

func runGroup(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if len(args) == 0 || args[0] == "help" {
		fmt.Print(helpGroup)
		return nil
	}
	mode := args[0]
	args = args[1:]

	switch mode {
	case "list":
		if len(args) == 0 {
			return fmt.Errorf("usage: buddy group list smart|static [--filter=<rsql>]")
		}
		groupType := args[0]
		filter := ""
		fs := flag.NewFlagSet("group list "+groupType, flag.ContinueOnError)
		fs.StringVar(&filter, "filter", "", "name filter (plain substring or RSQL)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		switch groupType {
		case "smart":
			return buddy.GroupListSmart(credName, filter, log)
		case "static":
			return buddy.GroupListStatic(credName, filter, log)
		default:
			return errUnknownSubcommand("group list", groupType, []string{"smart", "static"})
		}
	case "get":
		if len(args) >= 2 && args[0] == "version" {
			groupID := strings.TrimSpace(args[1])
			return buddy.GroupGetVersion(credName, opts, groupID, log)
		}
		if len(args) < 1 || args[0] == "version" {
			return fmt.Errorf("missing group-id; run 'buddy group help' for usage")
		}
		return buddy.GroupGet(credName, strings.TrimSpace(args[0]), log)
	case "set":
		if len(args) < 3 || args[0] != "version" {
			return fmt.Errorf("missing 'version', group-id, or version value; run 'buddy group help' for usage")
		}
		groupID := strings.TrimSpace(args[1])
		newVersion := strings.TrimSpace(args[2])
		return buddy.GroupVersion(credName, opts, groupID, newVersion, log)
	case "delete":
		if len(args) < 1 {
			return fmt.Errorf("missing group-id; run 'buddy group help' for usage")
		}
		groupID := strings.TrimSpace(args[0])
		return buddy.GroupDelete(credName, opts, groupID, log)
	case "create":
		return runGroupCreate(credName, opts, args, log)
	case "static":
		if len(args) == 0 {
			return fmt.Errorf("usage: buddy group static <members|add|remove> ...; run 'buddy group help' for usage")
		}
		sub := args[0]
		args = args[1:]
		switch sub {
		case "members":
			var filter string
			fs := flag.NewFlagSet("group static members", flag.ContinueOnError)
			fs.StringVar(&filter, "filter", "", "client-side substring filter on id, name, or serial")
			flagArgs, positionals := staticGroupSplitFlags(args)
			if err := fs.Parse(flagArgs); err != nil {
				return err
			}
			if len(positionals) < 1 {
				return fmt.Errorf("missing <group-id>; run 'buddy group help' for usage")
			}
			groupID := strings.TrimSpace(positionals[0])
			return buddy.GroupStaticListMembers(credName, groupID, filter, log)
		case "add":
			var computerIDsFlag string
			fs := flag.NewFlagSet("group static add", flag.ContinueOnError)
			fs.StringVar(&computerIDsFlag, "computer-ids", "", "comma-separated computer IDs")
			flagArgs, positionals := staticGroupSplitFlags(args)
			if err := fs.Parse(flagArgs); err != nil {
				return err
			}
			if len(positionals) < 1 {
				return fmt.Errorf("missing <group-id>; run 'buddy group help' for usage")
			}
			if computerIDsFlag == "" {
				return fmt.Errorf("buddy group static add requires --computer-ids=<id>[,<id>...]")
			}
			groupID := strings.TrimSpace(positionals[0])
			ids := parseCommaSeparatedIDs(computerIDsFlag)
			if len(ids) == 0 {
				return fmt.Errorf("no computer IDs in --computer-ids")
			}
			changed, err := buddy.StaticGroupAddComputers(credName, opts, groupID, ids, log)
			if err != nil {
				return err
			}
			if changed {
				fmt.Printf("Updated static group %s.\n", groupID)
			}
			return nil
		case "remove":
			var computerIDsFlag string
			fs := flag.NewFlagSet("group static remove", flag.ContinueOnError)
			fs.StringVar(&computerIDsFlag, "computer-ids", "", "comma-separated computer IDs")
			flagArgs, positionals := staticGroupSplitFlags(args)
			if err := fs.Parse(flagArgs); err != nil {
				return err
			}
			if len(positionals) < 1 {
				return fmt.Errorf("missing <group-id>; run 'buddy group help' for usage")
			}
			if computerIDsFlag == "" {
				return fmt.Errorf("buddy group static remove requires --computer-ids=<id>[,<id>...]")
			}
			groupID := strings.TrimSpace(positionals[0])
			ids := parseCommaSeparatedIDs(computerIDsFlag)
			if len(ids) == 0 {
				return fmt.Errorf("no computer IDs in --computer-ids")
			}
			changed, err := buddy.StaticGroupRemoveComputers(credName, opts, groupID, ids, log)
			if err != nil {
				return err
			}
			if changed {
				fmt.Printf("Updated static group %s.\n", groupID)
			}
			return nil
		default:
			return errUnknownSubcommand("group static", sub, []string{"members", "add", "remove"})
		}
	default:
		cands := []string{"list", "get", "set", "create", "delete", "static"}
		msg := fmt.Sprintf("unknown group subcommand: %q", mode)
		msg += formatDidYouMean(suggestClosest(mode, cands))
		msg += fmt.Sprintf(" (use: %s)", strings.Join(cands, ", "))
		return fmt.Errorf("%s", msg)
	}
}

func runGroupCreate(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: buddy group create smart|static ...; run 'buddy group help' for usage")
	}
	kind := args[0]
	args = args[1:]
	switch kind {
	case "smart":
		var description, filePath string
		fs := flag.NewFlagSet("group create smart", flag.ContinueOnError)
		fs.StringVar(&description, "description", "", "optional group description")
		fs.StringVar(&filePath, "file", "", "JSON file: full body, or criteria array")
		flagArgs, positionals := staticGroupSplitFlags(args)
		if err := fs.Parse(flagArgs); err != nil {
			return err
		}
		if filePath != "" && len(positionals) == 0 {
			// Full body from file (must include name).
			data, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("read --file: %w", err)
			}
			created, err := buddy.GroupCreateSmart(credName, opts, data, log)
			if err != nil {
				return err
			}
			if created == nil {
				return nil
			}
			fmt.Printf("Created smart group (ID %s)\n", created.ID)
			return nil
		}
		if len(positionals) < 1 {
			return fmt.Errorf("missing <name>; run 'buddy group help' for usage")
		}
		name := strings.TrimSpace(positionals[0])
		var criteriaJSON []byte
		if filePath != "" {
			data, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("read --file: %w", err)
			}
			trimmed := bytes.TrimSpace(data)
			if len(trimmed) > 0 && trimmed[0] == '{' {
				// Full body with name override from positional if body lacks name.
				var raw map[string]json.RawMessage
				if err := json.Unmarshal(data, &raw); err != nil {
					return fmt.Errorf("parse --file: %w", err)
				}
				if _, ok := raw["name"]; !ok {
					nameJSON, err := json.Marshal(name)
					if err != nil {
						return err
					}
					raw["name"] = nameJSON
				}
				if description != "" {
					descJSON, err := json.Marshal(description)
					if err != nil {
						return err
					}
					raw["description"] = descJSON
				}
				if _, ok := raw["id"]; !ok {
					raw["id"] = json.RawMessage("-1")
				}
				payload, err := json.Marshal(raw)
				if err != nil {
					return err
				}
				created, err := buddy.GroupCreateSmart(credName, opts, payload, log)
				if err != nil {
					return err
				}
				if created == nil {
					return nil
				}
				fmt.Printf("Created smart group %q (ID %s)\n", name, created.ID)
				return nil
			}
			criteriaJSON = data
		}
		created, err := buddy.GroupCreateSmartNamed(credName, opts, name, description, criteriaJSON, log)
		if err != nil {
			return err
		}
		if created == nil {
			return nil
		}
		fmt.Printf("Created smart group %q (ID %s)\n", name, created.ID)
		return nil
	case "static":
		var description, computerIDsFlag string
		fs := flag.NewFlagSet("group create static", flag.ContinueOnError)
		fs.StringVar(&description, "description", "", "optional group description")
		fs.StringVar(&computerIDsFlag, "computer-ids", "", "comma-separated computer IDs (optional)")
		flagArgs, positionals := staticGroupSplitFlags(args)
		if err := fs.Parse(flagArgs); err != nil {
			return err
		}
		if len(positionals) < 1 {
			return fmt.Errorf("missing <name>; run 'buddy group help' for usage")
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

// parseParamArgs extracts --param=4,5,6 or --param 4,5,6 (and --params alias) from args.
// Returns paramIndices, remaining args, and error.
func parseParamArgs(args []string) ([]int, []string, error) {
	paramIndices := []int{4, 5, 6}
	var filtered []string
	i := 0
	for i < len(args) {
		a := args[i]
		if strings.HasPrefix(a, "--param=") || strings.HasPrefix(a, "--params=") {
			val := a
			if strings.HasPrefix(a, "--param=") {
				val = strings.TrimPrefix(a, "--param=")
			} else {
				val = strings.TrimPrefix(a, "--params=")
			}
			var err error
			paramIndices, err = parseParamVal(val)
			if err != nil {
				return nil, nil, err
			}
		} else if a == "--param" || a == "--params" {
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
		case strings.HasPrefix(a, "--group-ids="):
			groupIDsFlag = strings.TrimPrefix(a, "--group-ids=")
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
		return ssOpts, 0, fmt.Errorf("missing policy-id; run 'buddy policy set help' for usage")
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
		return ssOpts, 0, fmt.Errorf("missing version or flags; run 'buddy policy set help' for usage")
	}

	return ssOpts, policyID, nil
}

func runProfile(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if len(args) == 0 || args[0] == "help" {
		fmt.Print(helpProfile)
		return nil
	}
	mode := args[0]
	rest := args[1:]

	// "buddy profile <mode> help"
	if len(rest) > 0 && rest[0] == "help" {
		fmt.Print(helpProfile)
		return nil
	}
	args = rest

	switch mode {
	case "clone":
		if len(args) < 2 {
			return fmt.Errorf("missing profile-id or name; run 'buddy profile help' for usage")
		}
		profileID, err := parseInt(args[0], "profile-id")
		if err != nil {
			return err
		}
		name := strings.TrimSpace(strings.Join(args[1:], " "))
		if name == "" {
			return fmt.Errorf("missing name; run 'buddy profile help' for usage")
		}
		_, err = buddy.ProfileClone(credName, opts, profileID, name, log)
		return err
	case "list":
		filter, _, err := extractStringFlag(args, "filter")
		if err != nil {
			return err
		}
		return buddy.ProfileList(credName, filter, log)
	case "redeploy":
		if len(args) < 1 {
			return fmt.Errorf("missing profile-id; run 'buddy profile help' for usage")
		}
		profileID, err := parseInt(args[0], "profile-id")
		if err != nil {
			return err
		}
		return buddy.ProfileRedeploy(credName, opts, profileID, log)
	case "get":
		if len(args) < 1 {
			return fmt.Errorf("missing subcommand; run 'buddy profile help' for usage")
		}
		return runProfileGet(credName, opts, args[0], args[1:], log)
	case "set":
		if len(args) < 1 {
			return fmt.Errorf("missing subcommand; run 'buddy profile help' for usage")
		}
		return runProfileSet(credName, opts, args[0], args[1:], log)
	default:
		return errUnknownSubcommand("profile", mode, []string{"list", "get", "set", "clone", "redeploy"})
	}
}

func runProfileGet(credName string, opts buddy.Opts, subcommand string, args []string, log *slog.Logger) error {
	if subcommand == "help" {
		fmt.Print(helpProfile)
		return nil
	}
	switch subcommand {
	case "xml":
		if len(args) < 1 {
			return fmt.Errorf("missing profile-id; run 'buddy profile help' for usage")
		}
		profileID, err := parseInt(args[0], "profile-id")
		if err != nil {
			return err
		}
		return buddy.ProfileGetXML(credName, profileID, os.Stdout, log)
	case "payloads":
		if len(args) < 1 {
			return fmt.Errorf("missing profile-id; run 'buddy profile help' for usage")
		}
		profileID, err := parseInt(args[0], "profile-id")
		if err != nil {
			return err
		}
		return buddy.ProfileGetPayloads(credName, profileID, os.Stdout, log)
	case "payload":
		if len(args) < 2 {
			return fmt.Errorf("missing profile-id or domain; run 'buddy profile help' for usage")
		}
		profileID, err := parseInt(args[0], "profile-id")
		if err != nil {
			return err
		}
		domain := strings.TrimSpace(args[1])
		return buddy.ProfileGetPayloadByDomain(credName, profileID, domain, os.Stdout, log)
	case "list-payloads":
		if len(args) < 1 {
			return fmt.Errorf("missing profile-id; run 'buddy profile help' for usage")
		}
		profileID, err := parseInt(args[0], "profile-id")
		if err != nil {
			return err
		}
		domains, err := buddy.ProfileListPayloads(credName, profileID, log)
		if err != nil {
			return err
		}
		for _, d := range domains {
			fmt.Println(d)
		}
		return nil
	default:
		return errUnknownSubcommand("profile get", subcommand, []string{"xml", "payloads", "payload", "list-payloads"})
	}
}

func runProfileSet(credName string, opts buddy.Opts, subcommand string, args []string, log *slog.Logger) error {
	if subcommand == "help" {
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
			return fmt.Errorf("missing profile-id; run 'buddy profile help' for usage")
		}
		profileID, err := parseInt(args[0], "profile-id")
		if err != nil {
			return err
		}
		return buddy.ProfileSetXML(credName, opts, profileID, os.Stdin, redeploy, log)
	case "payload":
		if len(args) < 3 {
			return fmt.Errorf("missing profile-id, domain, or dict-file; run 'buddy profile help' for usage")
		}
		profileID, err := parseInt(args[0], "profile-id")
		if err != nil {
			return err
		}
		domain := strings.TrimSpace(args[1])
		dictFile := args[2]
		return buddy.ProfileSetPayload(credName, opts, profileID, domain, dictFile, redeploy, log)
	default:
		return errUnknownSubcommand("profile set", subcommand, []string{"xml", "payload"})
	}
}

func runPackage(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if len(args) == 0 || args[0] == "help" {
		fmt.Print(helpPackage)
		return nil
	}
	subcommand := args[0]
	args = args[1:]

	switch subcommand {
	case "upload":
		if len(args) < 1 {
			return fmt.Errorf("missing file argument; run 'buddy package help' for usage")
		}
		filePath, attrs, err := parsePackageUploadArgs(args)
		if err != nil {
			return err
		}
		return buddy.PackageUpload(credName, opts, filePath, attrs, log)
	case "list":
		filter, rest, err := extractStringFlag(args, "filter")
		if err != nil {
			return err
		}
		sortSpec, _, err := extractStringFlag(rest, "sort")
		if err != nil {
			return err
		}
		sortOrder, err := parsePackageListSort(sortSpec)
		if err != nil {
			return err
		}
		return buddy.PackageList(credName, filter, sortOrder, log)
	case "get":
		if len(args) < 1 {
			return fmt.Errorf("missing package-id; run 'buddy package help' for usage")
		}
		return buddy.PackageGet(credName, args[0], log)
	case "download":
		if len(args) < 1 {
			return fmt.Errorf("missing package-id; run 'buddy package help' for usage")
		}
		outfile := ""
		if len(args) >= 2 {
			outfile = args[1]
		}
		return buddy.PackageDownload(credName, args[0], outfile, log)
	case "delete":
		if len(args) < 1 {
			return fmt.Errorf("missing package-id; run 'buddy package help' for usage")
		}
		return buddy.PackageDelete(credName, opts, args, log)
	case "set":
		if len(args) < 1 {
			return fmt.Errorf("missing package-id or property; run 'buddy package help' for usage")
		}
		return runPackageSet(credName, opts, args, log)
	case "history":
		if len(args) < 1 {
			return fmt.Errorf("missing package-id; run 'buddy package help' for usage")
		}
		return buddy.PackageHistory(credName, args[0], 20, log)
	default:
		return errUnknownSubcommand("package", subcommand, []string{"list", "get", "download", "delete", "set", "history", "upload"})
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

func parsePackageUploadArgs(args []string) (filePath string, attrs buddy.PackageAttrs, err error) {
	attrs, positional, err := parsePackageAttrFlags(args)
	if err != nil {
		return "", buddy.PackageAttrs{}, err
	}
	if len(positional) < 1 {
		return "", buddy.PackageAttrs{}, fmt.Errorf("missing file argument; run 'buddy package help' for usage")
	}
	if len(positional) > 1 {
		return "", buddy.PackageAttrs{}, fmt.Errorf("unexpected arguments after file: %v", positional[1:])
	}
	filePath = strings.TrimSpace(positional[0])
	if filePath == "" {
		return "", buddy.PackageAttrs{}, fmt.Errorf("missing file argument; run 'buddy package help' for usage")
	}
	return filePath, attrs, nil
}

// parsePackageAttrFlags parses shared upload/set metadata flags.
func parsePackageAttrFlags(args []string) (attrs buddy.PackageAttrs, positional []string, err error) {
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "--name="):
			v := strings.TrimSpace(strings.TrimPrefix(a, "--name="))
			attrs.Name = &v
		case strings.HasPrefix(a, "--notes="):
			v := strings.TrimSpace(strings.TrimPrefix(a, "--notes="))
			attrs.Notes = &v
		case strings.HasPrefix(a, "--info="):
			v := strings.TrimSpace(strings.TrimPrefix(a, "--info="))
			attrs.Info = &v
		case strings.HasPrefix(a, "--category="):
			v := strings.TrimSpace(strings.TrimPrefix(a, "--category="))
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
			return buddy.PackageAttrs{}, nil, fmt.Errorf("unknown flag %s (use --name=, --notes=, --info=, --category=, --priority=, --reboot=)", a)
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

func runComputer(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if len(args) == 0 || args[0] == "help" {
		fmt.Print(helpComputer)
		return nil
	}
	subcommand := args[0]
	args = args[1:]

	parseComputerIDs := func(args []string) ([]int, []string, error) {
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

	switch subcommand {
	case "find":
		return fmt.Errorf("buddy computer find was removed; use 'buddy computer list [--filter=<rsql>]'")
	case "list":
		fs := flag.NewFlagSet("computer list", flag.ContinueOnError)
		filter := fs.String("filter", "", "RSQL filter expression")
		mgmtID := fs.String("management-id", "", "Look up computer by MDM management UUID (clientManagementId)")
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
		return buddy.ComputerSearchPrint(credName, f, log)
	case "get":
		if len(args) < 1 {
			return fmt.Errorf("missing computer-id; run 'buddy computer help' for usage")
		}
		return buddy.ComputerGet(credName, args[0], log)
	case "reboot":
		if len(args) < 1 {
			return fmt.Errorf("missing computer-id; run 'buddy computer help' for usage")
		}
		computerIDs, _, err := parseComputerIDs(args)
		if err != nil {
			return err
		}
		return buddy.ComputerReboot(credName, opts, computerIDs, log)
	case "redeploy":
		if len(args) < 1 {
			return fmt.Errorf("missing computer-id; run 'buddy computer help' for usage")
		}
		computerIDs, _, err := parseComputerIDs(args)
		if err != nil {
			return err
		}
		return buddy.ComputerRedeploy(credName, opts, computerIDs, log)
	case "shutdown":
		if len(args) < 1 {
			return fmt.Errorf("missing computer-id; run 'buddy computer help' for usage")
		}
		computerIDs, _, err := parseComputerIDs(args)
		if err != nil {
			return err
		}
		return buddy.ComputerShutdown(credName, opts, computerIDs, log)
	case "lock":
		if len(args) < 1 {
			return fmt.Errorf("missing computer-id; run 'buddy computer help' for usage")
		}
		computerIDs, rest, err := parseComputerIDs(args)
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
		if len(args) < 1 {
			return fmt.Errorf("missing computer-id; run 'buddy computer help' for usage")
		}
		computerIDs, _, err := parseComputerIDs(args)
		if err != nil {
			return err
		}
		return buddy.ComputerLogOut(credName, opts, computerIDs, log)
	case "erase":
		if len(args) < 1 {
			return fmt.Errorf("missing computer-id; run 'buddy computer help' for usage")
		}
		computerIDs, rest, err := parseComputerIDs(args)
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
		if len(args) < 1 {
			return fmt.Errorf("missing computer-id; run 'buddy computer help' for usage")
		}
		computerIDs, rest, err := parseComputerIDs(args)
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
		if len(args) < 1 {
			return fmt.Errorf("missing computer-id; run 'buddy computer help' for usage")
		}
		computerIDs, _, err := parseComputerIDs(args)
		if err != nil {
			return err
		}
		return buddy.ComputerDeviceLocation(credName, opts, computerIDs, log)
	case "security-info":
		if len(args) < 1 {
			return fmt.Errorf("missing computer-id; run 'buddy computer help' for usage")
		}
		computerIDs, _, err := parseComputerIDs(args)
		if err != nil {
			return err
		}
		return buddy.ComputerSecurityInfo(credName, opts, computerIDs, log)
	case "installed-apps":
		if len(args) < 1 {
			return fmt.Errorf("missing computer-id; run 'buddy computer help' for usage")
		}
		computerIDs, rest, err := parseComputerIDs(args)
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
		if len(args) < 1 {
			return fmt.Errorf("missing computer-id; run 'buddy computer help' for usage")
		}
		computerIDs, rest, err := parseComputerIDs(args)
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
		if len(args) < 1 {
			return fmt.Errorf("missing computer-id; run 'buddy computer help' for usage")
		}
		computerIDs, _, err := parseComputerIDs(args)
		if err != nil {
			return err
		}
		return buddy.ComputerCertificates(credName, opts, computerIDs, log)
	default:
		return errUnknownSubcommand("computer", subcommand, []string{
			"list", "get", "reboot", "redeploy", "shutdown", "lock", "logout", "erase",
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
		return fmt.Errorf("usage: buddy package set <package-id> [--name=] [--notes=] [--info=] [--category=] [--priority=] [--reboot=true|false]")
	}
	return buddy.PackageSetAttrs(credName, strings.TrimSpace(positional[0]), opts, attrs, log)
}

func runRole(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if len(args) == 0 || args[0] == "help" {
		fmt.Print(helpRole)
		return nil
	}
	subcommand := args[0]
	args = args[1:]

	switch subcommand {
	case "list":
		filter, rest, err := extractStringFlag(args, "filter")
		if err != nil {
			return err
		}
		if len(rest) > 0 {
			return fmt.Errorf("unknown list flag %q; run 'buddy role help'", rest[0])
		}
		return buddy.RoleList(credName, filter, log)
	case "get":
		if len(args) < 1 {
			return fmt.Errorf("missing role-id; run 'buddy role help' for usage")
		}
		return buddy.RoleGet(credName, args[0], log)
	case "create":
		if len(args) < 1 {
			return fmt.Errorf("missing displayName; run 'buddy role help' for usage")
		}
		displayName := args[0]
		privileges := args[1:]
		if privileges == nil {
			privileges = []string{}
		}
		return buddy.RoleCreate(credName, opts, displayName, privileges, log)
	case "update":
		if len(args) < 2 {
			return fmt.Errorf("missing role-id or displayName; run 'buddy role help' for usage")
		}
		roleID := args[0]
		displayName := args[1]
		privileges := args[2:]
		if privileges == nil {
			privileges = []string{}
		}
		return buddy.RoleUpdate(credName, roleID, opts, displayName, privileges, log)
	case "delete":
		if len(args) < 1 {
			return fmt.Errorf("missing role-id; run 'buddy role help' for usage")
		}
		return buddy.RoleDelete(credName, args[0], opts, log)
	case "privileges":
		return buddy.RolePrivileges(credName, log)
	default:
		return errUnknownSubcommand("role", subcommand, []string{"list", "get", "create", "update", "delete", "privileges"})
	}
}

func runClient(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if len(args) == 0 || args[0] == "help" {
		fmt.Print(helpClient)
		return nil
	}
	subcommand := args[0]
	args = args[1:]

	switch subcommand {
	case "list":
		filter, rest, err := extractStringFlag(args, "filter")
		if err != nil {
			return err
		}
		if len(rest) > 0 {
			return fmt.Errorf("unknown list flag %q; run 'buddy client help'", rest[0])
		}
		return buddy.ClientList(credName, filter, log)
	case "get":
		if len(args) < 1 {
			return fmt.Errorf("missing client-id; run 'buddy client help' for usage")
		}
		return buddy.ClientGet(credName, args[0], log)
	case "create":
		if len(args) < 1 {
			return fmt.Errorf("missing displayName; run 'buddy client help' for usage")
		}
		displayName := args[0]
		scopes := args[1:]
		if scopes == nil {
			scopes = []string{}
		}
		return buddy.ClientCreate(credName, opts, displayName, scopes, log)
	case "update":
		if len(args) < 2 {
			return fmt.Errorf("missing client-id or displayName; run 'buddy client help' for usage")
		}
		clientID := args[0]
		displayName := args[1]
		scopes := args[2:]
		if scopes == nil {
			scopes = []string{}
		}
		return buddy.ClientUpdate(credName, clientID, opts, displayName, scopes, log)
	case "delete":
		if len(args) < 1 {
			return fmt.Errorf("missing client-id; run 'buddy client help' for usage")
		}
		return buddy.ClientDelete(credName, args[0], opts, log)
	case "credentials":
		if len(args) < 1 {
			return fmt.Errorf("missing client-id; run 'buddy client help' for usage")
		}
		return buddy.ClientCredentials(credName, args[0], opts, log)
	case "set":
		if len(args) < 3 {
			return fmt.Errorf("missing property, client-id, or value; run 'buddy client help' for usage")
		}
		return runClientSet(credName, opts, args, log)
	default:
		return errUnknownSubcommand("client", subcommand, []string{"list", "get", "create", "update", "delete", "credentials", "set"})
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
	if len(args) == 0 || args[0] == "help" {
		fmt.Print(helpScript)
		return nil
	}
	subcommand := args[0]
	rest := args[1:]

	// "buddy script get help" / "buddy script set help"
	if (subcommand == "get" || subcommand == "set") && (len(rest) == 0 || rest[0] == "help") {
		fmt.Print(scriptSubHelp(subcommand))
		return nil
	}
	args = rest

	switch subcommand {
	case "list":
		filter, _, err := extractStringFlag(args, "filter")
		if err != nil {
			return err
		}
		return buddy.ScriptList(credName, filter, log)

	case "get":
		if len(args) < 1 {
			return fmt.Errorf("missing script-id; run 'buddy script get help' for usage")
		}
		return runJamfScriptGet(credName, opts, args[0], args[1:], log)

	case "history":
		if len(args) < 1 {
			return fmt.Errorf("missing script-id; run 'buddy script help' for usage")
		}
		return buddy.ScriptHistory(credName, args[0], 20, log)

	case "create":
		var category, priority, info, notes string
		var positional []string
		for _, a := range args {
			switch {
			case strings.HasPrefix(a, "--category="):
				category = strings.TrimPrefix(a, "--category=")
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
			return fmt.Errorf("missing name or file; run 'buddy script help' for usage")
		}
		return buddy.ScriptCreate(credName, opts, positional[0], positional[1], category, priority, info, notes, log)

	case "download":
		if len(args) < 1 {
			return fmt.Errorf("missing script-id; run 'buddy script help' for usage")
		}
		outfile := ""
		if len(args) >= 2 {
			outfile = strings.TrimSpace(args[1])
		}
		return buddy.ScriptDownload(credName, args[0], outfile, log)

	case "upload", "update":
		var nameFlag string
		var positional []string
		for _, a := range args {
			if strings.HasPrefix(a, "--name=") {
				nameFlag = strings.TrimPrefix(a, "--name=")
			} else {
				positional = append(positional, a)
			}
		}
		if len(positional) < 2 {
			return fmt.Errorf("missing script-id or file; run 'buddy script help' for usage")
		}
		if err := buddy.ScriptUpdate(credName, opts, positional[0], positional[1], log); err != nil {
			return err
		}
		if nameFlag != "" {
			return buddy.ScriptSetName(credName, positional[0], opts, nameFlag, log)
		}
		return nil

	case "delete":
		if len(args) < 1 {
			return fmt.Errorf("missing script-id; run 'buddy script help' for usage")
		}
		return buddy.ScriptDelete(credName, opts, args, log)

	case "set":
		if len(args) < 3 {
			return fmt.Errorf("missing property, script-id, or value; run 'buddy script set help' for usage")
		}
		return runJamfScriptSet(credName, opts, args, log)

	default:
		return errUnknownSubcommand("script", subcommand, []string{"list", "get", "history", "create", "upload", "update", "download", "delete", "set"})
	}
}

func runJamfScriptGet(credName string, opts buddy.Opts, subcommand string, args []string, log *slog.Logger) error {
	if subcommand == "help" {
		fmt.Print(helpScriptGet)
		return nil
	}
	switch subcommand {
	case "contents":
		if len(args) < 1 {
			return fmt.Errorf("missing script-id; run 'buddy script get help' for usage")
		}
		return buddy.ScriptGetContents(credName, args[0], log)
	case "id":
		if len(args) < 1 {
			return fmt.Errorf("missing name; run 'buddy script get help' for usage")
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
	if len(args) > 0 && args[0] == "help" {
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

// extractGlobalFlags removes buddy global flags from argv (any position) and returns the rest.
func extractGlobalFlags(argv []string) (globalCLI, []string) {
	var g globalCLI
	if len(argv) == 0 {
		return g, argv
	}
	out := []string{argv[0]}
	for i := 1; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--version":
			g.showVersion = true
		case a == "-y":
			g.skipConfirm = true
		case a == "-v", a == "--verbose":
			g.verbose = true
		case a == "-t", a == "--trace":
			g.trace = true
		case a == "--script-trace":
			g.scriptTrace = true
		case a == "-e", a == "--eval":
			if g.evalSource != "" {
				g.evalDup = true
			}
			if i+1 >= len(argv) {
				g.evalMissing = true
			} else {
				i++
				if g.evalSource == "" {
					g.evalSource = argv[i]
				}
			}
		case strings.HasPrefix(a, "--eval="):
			if g.evalSource != "" {
				g.evalDup = true
			} else {
				g.evalSource = strings.TrimPrefix(a, "--eval=")
				if g.evalSource == "" {
					g.evalMissing = true
				}
			}
		case strings.HasPrefix(a, "--tenant="):
			g.tenant = strings.TrimPrefix(a, "--tenant=")
		case a == "--tenant":
			if i+1 < len(argv) {
				i++
				g.tenant = argv[i]
			}
		case strings.HasPrefix(a, "--user="):
			g.user = strings.TrimPrefix(a, "--user=")
		case a == "--user":
			if i+1 < len(argv) {
				i++
				g.user = argv[i]
			}
		case strings.HasPrefix(a, "--auth="):
			g.user = strings.TrimPrefix(a, "--auth=")
		case a == "--auth":
			if i+1 < len(argv) {
				i++
				g.user = argv[i]
			}
		case a == "--cred", strings.HasPrefix(a, "--cred="):
			g.rejectCred = true
			if a == "--cred" && i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "-") {
				i++ // consume value so it is not treated as a command
			}
		default:
			out = append(out, a)
		}
	}
	return g, out
}

func printUsage() {
	fmt.Fprint(os.Stderr, help.Root)
}

func runBlueprint(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if len(args) == 0 || args[0] == "help" {
		fmt.Print(helpBlueprint)
		return nil
	}
	subcommand := args[0]
	args = args[1:]

	switch subcommand {
	case "list":
		listOpts := buddy.BlueprintListOpts{Page: buddy.PageOpts{Page: buddy.PageAll}}
		for _, a := range args {
			switch {
			case strings.HasPrefix(a, "--search="):
				listOpts.Search = strings.TrimPrefix(a, "--search=")
			case strings.HasPrefix(a, "--sort="):
				listOpts.Sort = strings.TrimPrefix(a, "--sort=")
			default:
				return fmt.Errorf("unknown list flag %q; run 'buddy blueprint help'", a)
			}
		}
		return buddy.BlueprintList(credName, listOpts, log)
	case "get":
		if len(args) < 1 {
			return fmt.Errorf("missing blueprint-id; run 'buddy blueprint help'")
		}
		return buddy.BlueprintGet(credName, args[0], log)
	case "create":
		file := ""
		for _, a := range args {
			switch {
			case strings.HasPrefix(a, "--file="):
				file = strings.TrimPrefix(a, "--file=")
			case a == "-":
				file = "-"
			default:
				return fmt.Errorf("unknown create arg %q; use --file=<path> or - for stdin", a)
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
	case "set":
		if len(args) < 1 {
			return fmt.Errorf("missing blueprint-id; run 'buddy blueprint help'")
		}
		blueprintID := args[0]
		file := "-"
		for _, a := range args[1:] {
			switch {
			case strings.HasPrefix(a, "--file="):
				file = strings.TrimPrefix(a, "--file=")
			case a == "-":
				file = "-"
			default:
				return fmt.Errorf("unknown set arg %q; use --file=<path> or - for stdin", a)
			}
		}
		body, err := buddy.ReadJSONBodyFile(file)
		if err != nil {
			return err
		}
		return buddy.BlueprintSet(credName, blueprintID, opts, body, log)
	case "delete":
		if len(args) < 1 {
			return fmt.Errorf("missing blueprint-id; run 'buddy blueprint help'")
		}
		return buddy.BlueprintDelete(credName, args[0], opts, log)
	case "deploy":
		if len(args) < 1 {
			return fmt.Errorf("missing blueprint-id; run 'buddy blueprint help'")
		}
		return buddy.BlueprintDeploy(credName, args[0], opts, log)
	case "undeploy":
		if len(args) < 1 {
			return fmt.Errorf("missing blueprint-id; run 'buddy blueprint help'")
		}
		return buddy.BlueprintUndeploy(credName, args[0], opts, log)
	case "status":
		if len(args) < 1 {
			return fmt.Errorf("missing blueprint-id; run 'buddy blueprint help'")
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
			"list", "get", "create", "set", "delete", "deploy", "undeploy", "status", "components",
		})
	}
}

func runAuth(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if len(args) == 0 || args[0] == "help" {
		fmt.Print(helpAuth)
		return nil
	}
	switch args[0] {
	case "test":
		return buddy.AuthTest(credName, log)
	case "token":
		if len(args) > 1 && args[1] == "help" {
			fmt.Print(helpAuth)
			return nil
		}
		return runAuthToken(credName, args[1:], log)
	case "refresh":
		if len(args) > 1 && args[1] == "help" {
			fmt.Print(helpAuth)
			return nil
		}
		return runAuthRefresh(credName, args[1:], log)
	default:
		return errUnknownSubcommand("auth", args[0], []string{"token", "refresh", "test", "help"})
	}
}

func runAuthToken(credName string, args []string, log *slog.Logger) error {
	fs := flag.NewFlagSet("auth token", flag.ContinueOnError)
	raw := fs.Bool("raw", false, "Print bearer token only (no \"Bearer \" prefix)")
	baseURL := fs.String("url", "", "Jamf Pro or Platform API Gateway base URL (direct auth)")
	region := fs.String("region", "", "Platform API Gateway region (direct auth; builds URL when --url omitted)")
	tenantID := fs.String("tenant-id", "", "Platform API Gateway tenant id (direct auth)")
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
		TenantID:        *tenantID,
		Username:        *username,
		Password:        *password,
		ClientID:        *clientID,
		ClientSecret:    *clientSecret,
		OAuth2CredsFile: *oauth2CredsFile,
	}

	if *raw || direct.UsesDirectAuth() {
		return buddy.PrintRawToken(credName, direct, log)
	}

	return buddy.GetToken(credName)
}

func runAuthRefresh(credName string, args []string, log *slog.Logger) error {
	fs := flag.NewFlagSet("auth refresh", flag.ContinueOnError)
	baseURL := fs.String("url", "", "Jamf Pro base URL (defaults to --user credential URL)")
	tokenFlag := fs.String("token", "", "Bearer token to refresh via keep-alive")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	tokenValue := strings.TrimSpace(*tokenFlag)
	if tokenValue == "" && fs.NArg() > 0 {
		tokenValue = strings.TrimSpace(fs.Arg(0))
	}
	if tokenValue == "" {
		return fmt.Errorf("token is required; run 'buddy auth refresh help' for usage")
	}

	urlValue := strings.TrimSpace(*baseURL)
	if urlValue == "" {
		var err error
		urlValue, err = buddy.GetBaseURL(credName)
		if err != nil {
			return fmt.Errorf("Jamf Pro URL is required (use --url or configure --user): %w", err)
		}
	}

	refreshed, err := buddy.RefreshRawToken(urlValue, tokenValue, log)
	if err != nil {
		return err
	}
	fmt.Println(refreshed)
	return nil
}

func runSetup(cfg *buddy.Config, opts buddy.Opts, credName string, args []string, log *slog.Logger) error {
	if len(args) == 0 || args[0] == "help" {
		fmt.Print(helpSetup)
		return nil
	}
	subcommand := args[0]
	args = args[1:]
	switch subcommand {
	case "basic-setup":
		bopts := buddy.BasicSetupOpts{SkipConfirm: opts.SkipConfirm}
		for _, a := range args {
			switch {
			case strings.HasPrefix(a, "--tenant="):
				bopts.TenantName = strings.TrimSpace(strings.TrimPrefix(a, "--tenant="))
			case strings.HasPrefix(a, "--url="):
				bopts.URL = strings.TrimSpace(strings.TrimPrefix(a, "--url="))
			case strings.HasPrefix(a, "--name="), strings.HasPrefix(a, "--auth="), strings.HasPrefix(a, "--user="):
				// --user here means the new auth name (not API pin); setup is bootstrapping.
				if strings.HasPrefix(a, "--name=") {
					bopts.AuthName = strings.TrimSpace(strings.TrimPrefix(a, "--name="))
				} else if strings.HasPrefix(a, "--auth=") {
					bopts.AuthName = strings.TrimSpace(strings.TrimPrefix(a, "--auth="))
				} else {
					bopts.AuthName = strings.TrimSpace(strings.TrimPrefix(a, "--user="))
				}
			case strings.HasPrefix(a, "--username="):
				bopts.Username = strings.TrimSpace(strings.TrimPrefix(a, "--username="))
			case strings.HasPrefix(a, "--password="):
				bopts.Password = strings.TrimPrefix(a, "--password=")
			case a == "-y":
				bopts.SkipConfirm = true
			}
		}
		fmt.Println("buddy setup basic-setup")
		fmt.Println()
		fmt.Println("This configures your first Jamf Pro *instance* tenant and a basic")
		fmt.Println("(username/password) auth in ~/.config/buddy/config.yaml.")
		fmt.Println("The password is stored in the platform secret store (Keychain/file),")
		fmt.Println("not inline in config. surface=instance (not Platform gateway oauth).")
		fmt.Println()
		fmt.Println("Afterward you can auth test with basic, and optionally create a Jamf")
		fmt.Println("API role/client and store instance oauth (buddy setup write-script /")
		fmt.Println("store-oauth).")
		fmt.Println()
		res, err := buddy.SetupBasic(cfg, bopts)
		if err != nil {
			return err
		}
		if res == nil {
			return nil
		}
		fmt.Printf("Configured tenant %q (%s) and basic auth %q (user %s, surface=instance).\n",
			res.TenantName, res.URL, res.AuthName, res.Username)
		fmt.Printf("Password stored in the platform secret store. Default tenant is %q.\n", cfg.Tenant.Default)
		fmt.Println()
		fmt.Println("Try:")
		fmt.Printf("  buddy --user %s auth test\n", res.AuthName)
		fmt.Println()
		fmt.Println("Optional — create a Jamf Pro instance API role/client and store oauth:")
		fmt.Println("  buddy setup write-script")
		fmt.Printf("  buddy setup_buddy_role.buddy -y   # uses auth %q\n", res.AuthName)
		fmt.Println("  buddy setup store-oauth")
		fmt.Println("  buddy --user <oauth-auth> auth test")
		return nil
	case "write-script":
		path := "setup_buddy_role.buddy"
		if len(args) > 0 {
			path = args[0]
		}
		if err := buddy.WriteSetupScript(path); err != nil {
			return fmt.Errorf("write setup script: %w", err)
		}
		fmt.Printf("Wrote %s\n", path)
		fmt.Printf("Run with: ./%s -y   (or: buddy %s -y)\n", path, path)
		return nil
	case "store-oauth":
		apiAuth := credName
		clientDisplayName := "buddy-cli"
		var authName, tenantName string
		configureOpts := opts
		for _, a := range args {
			switch {
			case strings.HasPrefix(a, "--user="):
				apiAuth = strings.TrimPrefix(a, "--user=")
			case strings.HasPrefix(a, "--client-name="):
				clientDisplayName = strings.TrimPrefix(a, "--client-name=")
			case strings.HasPrefix(a, "--name="):
				authName = strings.TrimSpace(strings.TrimPrefix(a, "--name="))
			case strings.HasPrefix(a, "--auth="):
				authName = strings.TrimSpace(strings.TrimPrefix(a, "--auth="))
			case strings.HasPrefix(a, "--tenant="):
				tenantName = strings.TrimSpace(strings.TrimPrefix(a, "--tenant="))
			case a == "-y":
				configureOpts.SkipConfirm = true
			}
		}
		res, err := buddy.SetupStoreOAuth(cfg, buddy.StoreOAuthOpts{
			APIAuth:           apiAuth,
			ClientDisplayName: clientDisplayName,
			AuthName:          authName,
			TenantName:        tenantName,
			Opts:              configureOpts,
		}, log)
		if err != nil {
			return err
		}
		if res == nil {
			return nil // user declined
		}
		if res.Updated {
			fmt.Printf("Updated instance oauth auth %q on tenant %q (surface=instance; secret in platform store). Use: buddy --user %s ...\n",
				res.AuthName, res.TenantName, res.AuthName)
		} else {
			fmt.Printf("Created instance oauth auth %q on tenant %q (surface=instance; secret in platform store). Use: buddy --user %s ...\n",
				res.AuthName, res.TenantName, res.AuthName)
		}
		return nil
	default:
		return errUnknownSubcommand("setup", subcommand, []string{"basic-setup", "write-script", "store-oauth"})
	}
}

func runMdm(credName string, _ buddy.Opts, args []string, log *slog.Logger) error {
	if len(args) == 0 || args[0] == "help" {
		fmt.Print(helpMdm)
		return nil
	}
	subcommand := args[0]
	args = args[1:]

	switch subcommand {
	case "find":
		return fmt.Errorf("buddy mdm find was removed; use 'buddy mdm list --filter=<rsql>'")
	case "list":
		var filter, sort string
		var pageSize, limit int
		var count bool
		fs := flag.NewFlagSet("mdm list", flag.ContinueOnError)
		fs.StringVar(&filter, "filter", "", "RSQL filter (required; at least one field)")
		fs.StringVar(&sort, "sort", "dateSent:asc", "Sort expression (e.g. dateSent:desc)")
		fs.IntVar(&pageSize, "page-size", 200, "Records per API request; pagination iterates until all results are fetched")
		fs.IntVar(&limit, "limit", 0, "Maximum total records to return (0 = no limit)")
		fs.BoolVar(&count, "count", false, "Print only the total count of matching records (single API call)")
		if err := fs.Parse(args); err != nil {
			return err
		}
		if filter == "" {
			return fmt.Errorf("--filter is required; run 'buddy mdm help' for usage")
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
		return buddy.MdmCommandFindPrint(credName, filter, sort, pageOpts, limit, log)
	default:
		return errUnknownSubcommand("mdm", subcommand, []string{"list"})
	}
}
