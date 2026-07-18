package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"

	"buddy/internal/buddy"
	"buddy/internal/help"
	buddymcp "buddy/internal/mcp"
	"buddy/internal/script"
	"buddy/internal/serve"
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
	user        string
	cred        string
	usedCred    bool
}

func main() {
	globals, filtered := extractGlobalFlags(os.Args)
	os.Args = filtered

	if globals.usedCred {
		fmt.Fprintln(os.Stderr, "note: '--cred' is deprecated; use '--user'")
	}

	if globals.showVersion {
		fmt.Println(Version)
		os.Exit(0)
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
	credName := cfg.Cred
	if globals.user != "" {
		credName = globals.user
	} else if globals.cred != "" {
		credName = globals.cred
	}

	args := os.Args[1:]

	buddy.InitLogger()
	buddy.SetVerbose(globals.verbose || globals.trace)
	buddy.SetHTTPTrace(globals.trace)
	logger := buddy.GetLogger()

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
	case "config":
		fmt.Fprintln(os.Stderr, "note: 'config' is deprecated; use 'cred'")
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
		if len(args) > 1 && args[1] == "help" {
			fmt.Print(helpMCP)
			return
		}
		if err := buddymcp.Run(credName, opts, logger, Version); err != nil {
			logger.Error("MCP failed", "error", err)
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %q\n", args[0])
		printUsage()
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
		return runPolicyClone(credName, opts, append([]string{subcommand}, args...), log)
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
	case "find":
		return runPolicyFind(credName, opts, append([]string{subcommand}, args...), log)
	default:
		return fmt.Errorf("policy requires 'get', 'set', 'clone', 'create', 'delete', or 'find', got %q", mode)
	}
}

func runPolicyFind(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if len(args) > 0 && args[0] == "help" {
		fmt.Print(helpPolicyFind)
		return nil
	}
	var excludeIDs []int
	var keywords []string
	forceRefresh := false
	for _, a := range args {
		if strings.HasPrefix(a, "--exclude-ids=") {
			val := strings.TrimPrefix(a, "--exclude-ids=")
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
		} else if a == "-f" || a == "--force-refresh" {
			forceRefresh = true
		} else {
			keywords = append(keywords, a)
		}
	}
	if len(keywords) < 1 {
		return fmt.Errorf("at least one keyword required; run 'buddy policy find help' for usage")
	}
	ids, err := buddy.PolicyFindByKeywords(credName, opts, keywords, excludeIDs, forceRefresh, log)
	if err != nil {
		return err
	}
	for _, id := range ids {
		fmt.Println(id)
	}
	return nil
}

func runPolicyGet(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if len(args) > 0 && args[0] == "help" {
		fmt.Print(helpPolicyGet)
		return nil
	}
	var trigger string
	forceRefresh := false
	var pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-f" || a == "--force-refresh":
			forceRefresh = true
		case a == "--trigger":
			if i+1 >= len(args) {
				return fmt.Errorf("missing value for --trigger")
			}
			trigger = args[i+1]
			i++
		case strings.HasPrefix(a, "--trigger="):
			trigger = strings.TrimPrefix(a, "--trigger=")
		default:
			pos = append(pos, a)
		}
	}
	trigger = strings.TrimSpace(trigger)

	if trigger != "" {
		if len(pos) > 0 {
			return fmt.Errorf("unexpected arguments with --trigger: %v", pos)
		}
		id, err := buddy.PolicyFind(credName, opts, trigger, forceRefresh, log)
		if err != nil {
			return err
		}
		fmt.Println(id)
		return nil
	}

	if len(pos) < 1 {
		return fmt.Errorf("missing arguments: use --trigger <name>, or 'enable <policy-id>', or 'xml <policy-id>'; run 'buddy policy get help'")
	}
	switch pos[0] {
	case "xml":
		if len(pos) < 2 {
			return fmt.Errorf("missing policy-id; run 'buddy policy get help' for usage")
		}
		policyID, err := parseInt(pos[1], "policy-id")
		if err != nil {
			return err
		}
		return buddy.PolicyGetXML(credName, policyID, os.Stdout, log)
	case "enable":
		if len(pos) < 2 {
			return fmt.Errorf("missing policy-id; run 'buddy policy get help' for usage")
		}
		policyID, err := parseInt(pos[1], "policy-id")
		if err != nil {
			return err
		}
		return buddy.PolicyGetEnable(credName, opts, policyID, log)
	default:
		return fmt.Errorf("unknown policy get arguments: %v (use --trigger, or enable, or xml)", args)
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
		var triggerFlag, packageFlag string
		var filtered []string
		for _, a := range args {
			if strings.HasPrefix(a, "--trigger=") {
				triggerFlag = strings.TrimPrefix(a, "--trigger=")
			} else if strings.HasPrefix(a, "--package=") {
				packageFlag = strings.TrimPrefix(a, "--package=")
			} else {
				filtered = append(filtered, a)
			}
		}
		args = filtered
		if len(args) < 2 {
			return fmt.Errorf("missing app or version; run 'buddy policy set help' for usage")
		}
		appName := strings.ToLower(strings.TrimSpace(args[0]))
		newVersion := strings.TrimSpace(args[1])
		policyIDOverride := 0
		if len(args) >= 3 {
			var err error
			policyIDOverride, err = parseInt(args[2], "policy-id")
			if err != nil {
				return err
			}
		}
		if opts.Verbose && triggerFlag != "" {
			fmt.Fprintf(os.Stderr, "Using --trigger=%s to resolve policy\n", triggerFlag)
		}
		return buddy.PolicyPackage(credName, opts, appName, newVersion, policyIDOverride, triggerFlag, packageFlag, log)

	default:
		return fmt.Errorf("unknown policy set subcommand: %q (use: ss, scope, enable, disable, trigger, install, name, frequency, icon, package, xml)", subcommand)
	}
}

func runPolicyClone(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if len(args) > 0 && args[0] == "help" {
		fmt.Print(helpPolicyClone)
		return nil
	}
	var packageFlag string
	var cloneFiltered []string
	for _, a := range args {
		if strings.HasPrefix(a, "--package=") {
			packageFlag = strings.TrimPrefix(a, "--package=")
		} else {
			cloneFiltered = append(cloneFiltered, a)
		}
	}
	args = cloneFiltered
	paramIndices, filtered, err := parseParamArgs(args)
	if err != nil {
		return err
	}
	args = filtered
	if len(args) < 2 {
		return fmt.Errorf("missing policy-id or version; run 'buddy policy clone help' for usage")
	}
	policyID, err := parseInt(args[0], "policy-id")
	if err != nil {
		return err
	}
	newVersion := strings.TrimSpace(args[1])
	return buddy.PolicyClone(credName, opts, policyID, "", "", newVersion, packageFlag, paramIndices, log)
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
			return fmt.Errorf("missing app or version; run 'buddy policy create help' for usage")
		}
		appName := strings.ToLower(strings.TrimSpace(args[0]))
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
		return buddy.PolicyCreatePackage(credName, opts, appName, version, pkgAction, scopeAll, scopeGroupIDs, categoryFlag, packageFlag, log)
	case "ss":
		var categoryFlag string
		var filtered []string
		for _, a := range args {
			if strings.HasPrefix(a, "--category=") {
				categoryFlag = strings.TrimPrefix(a, "--category=")
			} else {
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
		// Default to all-computers scope and legacy script ID 51 for CLI.
		_, err := buddy.PolicyCreateSS(credName, opts, trigger, displayName, categoryFlag, 0, 51, log)
		return err
	default:
		return fmt.Errorf("unknown policy create subcommand: %q (use: package, ss)", subcommand)
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
			return fmt.Errorf("group list requires 'smart' or 'static', got %q", groupType)
		}
	case "get":
		if len(args) < 2 || args[0] != "version" {
			return fmt.Errorf("missing 'version' or group-id; run 'buddy group help' for usage")
		}
		groupID := strings.TrimSpace(args[1])
		return buddy.GroupGetVersion(credName, opts, groupID, log)
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
			return fmt.Errorf("unknown group static subcommand %q (use: members, add, remove)", sub)
		}
	default:
		return fmt.Errorf("group requires 'list', 'get', 'set', 'create', 'delete', or 'static', got %q", mode)
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
		return fmt.Errorf("group create requires 'smart' or 'static', got %q", kind)
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
		return buddy.ProfileClone(credName, opts, profileID, name, log)
	case "list":
		var filter string
		var filtered []string
		for _, a := range args {
			if strings.HasPrefix(a, "--filter=") {
				filter = strings.TrimPrefix(a, "--filter=")
			} else {
				filtered = append(filtered, a)
			}
		}
		_ = filtered
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
		return fmt.Errorf("profile requires 'list', 'get', 'set', 'clone', or 'redeploy', got %q", mode)
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
		return fmt.Errorf("unknown profile get subcommand: %q (use: xml, payloads, payload, list-payloads)", subcommand)
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
		return fmt.Errorf("unknown profile set subcommand: %q (use: xml, payload)", subcommand)
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
		filePath, nameOverride, categoryID, priority, err := parsePackageUploadArgs(args)
		if err != nil {
			return err
		}
		return buddy.PackageUpload(credName, opts, filePath, nameOverride, categoryID, priority, log)
	case "list":
		var filter string
		var filtered []string
		for _, a := range args {
			if strings.HasPrefix(a, "--filter=") {
				filter = strings.TrimPrefix(a, "--filter=")
			} else {
				filtered = append(filtered, a)
			}
		}
		_ = filtered
		return buddy.PackageList(credName, filter, log)
	case "get":
		if len(args) < 1 {
			return fmt.Errorf("missing package-id; run 'buddy package help' for usage")
		}
		return buddy.PackageGet(credName, args[0], log)
	case "delete":
		if len(args) < 1 {
			return fmt.Errorf("missing package-id; run 'buddy package help' for usage")
		}
		return buddy.PackageDelete(credName, opts, args, log)
	case "set":
		if len(args) < 2 {
			return fmt.Errorf("missing property or package-id; run 'buddy package help' for usage")
		}
		return runPackageSet(credName, opts, args, log)
	case "history":
		if len(args) < 1 {
			return fmt.Errorf("missing package-id; run 'buddy package help' for usage")
		}
		return buddy.PackageHistory(credName, args[0], 20, log)
	default:
		return fmt.Errorf("unknown package subcommand: %q (use: list, get, delete, set, history, upload)", subcommand)
	}
}

func parsePackageUploadArgs(args []string) (filePath, nameOverride, categoryID string, priority int, err error) {
	priority = 10
	categoryID = "-1"
	var positional []string
	for _, a := range args {
		if strings.HasPrefix(a, "--name=") {
			nameOverride = strings.TrimSpace(strings.TrimPrefix(a, "--name="))
		} else if strings.HasPrefix(a, "--category=") {
			categoryID = strings.TrimSpace(strings.TrimPrefix(a, "--category="))
		} else if strings.HasPrefix(a, "--priority=") {
			v := strings.TrimSpace(strings.TrimPrefix(a, "--priority="))
			priority, err = parseInt(v, "priority")
			if err != nil {
				return "", "", "", 0, err
			}
		} else {
			positional = append(positional, a)
		}
	}
	if len(positional) < 1 {
		return "", "", "", 0, fmt.Errorf("missing file argument; run 'buddy package help' for usage")
	}
	filePath = strings.TrimSpace(positional[0])
	if filePath == "" {
		return "", "", "", 0, fmt.Errorf("missing file argument; run 'buddy package help' for usage")
	}
	return filePath, nameOverride, categoryID, priority, nil
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
		fs := flag.NewFlagSet("computer find", flag.ContinueOnError)
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
		return fmt.Errorf("unknown computer subcommand: %q (use: find, get, reboot, redeploy, shutdown, lock, logout, erase, device-info, device-location, security-info, installed-apps, managed-apps, certificates)", subcommand)
	}
}

func runPackageSet(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	property := args[0]
	if len(args) < 3 {
		return fmt.Errorf("missing package-id or value for 'set %s'; run 'buddy package help' for usage", property)
	}
	packageID := strings.TrimSpace(args[1])
	value := strings.TrimSpace(args[2])

	switch property {
	case "name":
		return buddy.PackageSetName(credName, packageID, opts, value, log)
	case "notes":
		return buddy.PackageSetNotes(credName, packageID, opts, value, log)
	case "info":
		return buddy.PackageSetInfo(credName, packageID, opts, value, log)
	case "priority":
		n, err := parseInt(value, "priority")
		if err != nil {
			return err
		}
		return buddy.PackageSetPriority(credName, packageID, opts, n, log)
	case "category":
		return buddy.PackageSetCategory(credName, packageID, opts, value, log)
	case "reboot":
		var b bool
		switch strings.ToLower(value) {
		case "true", "yes", "1":
			b = true
		case "false", "no", "0":
			b = false
		default:
			return fmt.Errorf("reboot value must be true or false, got %q", value)
		}
		return buddy.PackageSetReboot(credName, packageID, opts, b, log)
	default:
		return fmt.Errorf("unknown package set property: %q (use: name, notes, info, priority, category, reboot)", property)
	}
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
		return buddy.RoleList(credName, log)
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
		return fmt.Errorf("unknown role subcommand: %q (use: list, get, create, update, delete, privileges)", subcommand)
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
		return buddy.ClientList(credName, log)
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
		return fmt.Errorf("unknown client subcommand: %q (use: list, get, create, update, delete, credentials, set)", subcommand)
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
		var filter string
		for _, a := range args {
			if strings.HasPrefix(a, "--filter=") {
				filter = strings.TrimPrefix(a, "--filter=")
			}
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
		return fmt.Errorf("unknown script subcommand: %q (use: list, get, history, create, upload, update, download, delete, set)", subcommand)
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
		case strings.HasPrefix(a, "--user="):
			g.user = strings.TrimPrefix(a, "--user=")
		case a == "--user":
			if i+1 < len(argv) {
				i++
				g.user = argv[i]
			}
		case strings.HasPrefix(a, "--cred="):
			g.cred = strings.TrimPrefix(a, "--cred=")
			g.usedCred = true
		case a == "--cred":
			g.usedCred = true
			if i+1 < len(argv) {
				i++
				g.cred = argv[i]
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
		return fmt.Errorf("unknown auth subcommand: %q; run 'buddy auth help' for usage", args[0])
	}
}

func runAuthToken(credName string, args []string, log *slog.Logger) error {
	fs := flag.NewFlagSet("auth token", flag.ContinueOnError)
	raw := fs.Bool("raw", false, "Print bearer token only (no \"Bearer \" prefix)")
	baseURL := fs.String("url", "", "Jamf Pro base URL (direct auth, no config credential)")
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

func runCred(cfg *buddy.Config, opts buddy.Opts, args []string, _ *slog.Logger) error {
	if len(args) == 0 || args[0] == "help" {
		fmt.Print(helpCred)
		return nil
	}
	switch args[0] {
	case "list":
		return runConfigList(cfg)
	case "get":
		if len(args) < 2 {
			return fmt.Errorf("missing credential name; run 'buddy cred help' for usage")
		}
		return runConfigGet(cfg, args[1])
	case "add":
		return runConfigAdd(cfg, opts, args[1:])
	case "delete":
		if len(args) < 2 {
			return fmt.Errorf("missing credential name; run 'buddy cred help' for usage")
		}
		return runConfigDelete(cfg, opts, args[1])
	case "default":
		if len(args) < 2 {
			fmt.Println(cfg.Cred)
			return nil
		}
		return runConfigSetDefault(cfg, args[1])
	case "secret":
		return runConfigSecret(cfg, args[1:])
	default:
		return fmt.Errorf("unknown cred subcommand: %q; run 'buddy cred help' for usage", args[0])
	}
}

func runConfigList(cfg *buddy.Config) error {
	if len(cfg.Credentials) == 0 {
		fmt.Println("No credentials configured. Use 'buddy cred add' to add one.")
		return nil
	}
	fmt.Printf("%-20s  %-45s  %s\n", "NAME", "URL", "TYPE")
	fmt.Printf("%-20s  %-45s  %s\n", "--------------------", "---------------------------------------------", "-------")
	for name, cred := range cfg.Credentials {
		marker := ""
		if name == cfg.Cred {
			marker = " *"
		}
		fmt.Printf("%-20s  %-45s  %s%s\n", name, cred.URL, cred.Auth.Type, marker)
	}
	fmt.Printf("\n* = default credential\n")
	return nil
}

func runConfigGet(cfg *buddy.Config, name string) error {
	cred, err := cfg.ResolveCredential(name)
	if err != nil {
		return err
	}
	isDefault := "no"
	if name == cfg.Cred {
		isDefault = "yes"
	}
	fmt.Printf("name:     %s\n", name)
	fmt.Printf("url:      %s\n", cred.URL)
	fmt.Printf("type:     %s\n", cred.Auth.Type)
	switch cred.Auth.Type {
	case "command":
		fmt.Printf("command:  %s\n", cred.Auth.RunCommand)
	case "basic":
		fmt.Printf("username: %s\n", cred.Auth.Username)
		if cred.Auth.Password != "" {
			fmt.Printf("password: %s (inline in config)\n", redact(cred.Auth.Password))
		} else if ok, _ := buddy.HasCredentialSecret(name, buddy.SecretKindPassword); ok {
			fmt.Printf("password: (platform store)\n")
		} else {
			fmt.Printf("password: (not set)\n")
		}
	case "oauth":
		fmt.Printf("clientId:     %s\n", cred.Auth.ClientID)
		if cred.Auth.ClientSecret != "" {
			fmt.Printf("clientSecret: %s (inline in config)\n", redact(cred.Auth.ClientSecret))
		} else if ok, _ := buddy.HasCredentialSecret(name, buddy.SecretKindClientSecret); ok {
			fmt.Printf("clientSecret: (platform store)\n")
		} else {
			fmt.Printf("clientSecret: (not set)\n")
		}
	}
	fmt.Printf("default:  %s\n", isDefault)
	return nil
}

// redact masks a secret string, showing only the first 4 characters.
func redact(s string) string {
	if len(s) <= 4 {
		return strings.Repeat("*", len(s))
	}
	return s[:4] + strings.Repeat("*", len(s)-4)
}

func runConfigAdd(cfg *buddy.Config, opts buddy.Opts, args []string) error {
	// Parse positional: name url, then flags
	var positional []string
	authType := ""
	runCommand := ""
	username := ""
	password := ""
	clientID := ""
	clientSecret := ""
	skipConfirm := opts.SkipConfirm

	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "--type="):
			authType = strings.TrimPrefix(a, "--type=")
		case strings.HasPrefix(a, "--run="):
			runCommand = strings.TrimPrefix(a, "--run=")
		case strings.HasPrefix(a, "--username="):
			username = strings.TrimPrefix(a, "--username=")
		case strings.HasPrefix(a, "--password="):
			password = strings.TrimPrefix(a, "--password=")
		case strings.HasPrefix(a, "--client-id="):
			clientID = strings.TrimPrefix(a, "--client-id=")
		case strings.HasPrefix(a, "--client-secret="):
			clientSecret = strings.TrimPrefix(a, "--client-secret=")
		case a == "-y":
			skipConfirm = true
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) < 2 {
		return fmt.Errorf("usage: buddy cred add <name> <url> --type=<type> [flags]; run 'buddy cred help' for details")
	}
	credName := positional[0]
	credURL := positional[1]

	if authType == "" {
		return fmt.Errorf("--type is required (command, basic, or oauth); run 'buddy cred help' for details")
	}

	auth := buddy.AuthConfig{Type: authType}
	switch authType {
	case "command":
		if runCommand == "" {
			return fmt.Errorf("--run is required for auth type 'command'")
		}
		auth.RunCommand = runCommand
	case "basic":
		if username == "" {
			return fmt.Errorf("--username is required for auth type 'basic'")
		}
		auth.Username = username
		auth.Password = password
	case "oauth":
		if clientID == "" {
			return fmt.Errorf("--client-id is required for auth type 'oauth'")
		}
		auth.ClientID = clientID
		auth.ClientSecret = clientSecret
	default:
		return fmt.Errorf("unknown auth type %q (supported: command, basic, oauth)", authType)
	}

	_, existing := cfg.Credentials[credName]
	if existing && !skipConfirm {
		fmt.Printf("Credential %q already exists. Overwrite? [y/N] ", credName)
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(strings.TrimSpace(response)) != "y" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	if cfg.Credentials == nil {
		cfg.Credentials = make(map[string]buddy.CredentialConfig)
	}
	cfg.Credentials[credName] = buddy.CredentialConfig{URL: credURL, Auth: auth}

	if err := buddy.SaveConfig(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	if existing {
		fmt.Printf("Updated credential %q\n", credName)
	} else {
		fmt.Printf("Added credential %q\n", credName)
	}
	if authType == "basic" && password == "" {
		fmt.Fprintf(os.Stderr, "Note: no password in config — store one with: buddy cred secret set %s --password\n", credName)
	}
	if authType == "oauth" && clientSecret == "" {
		fmt.Fprintf(os.Stderr, "Note: no client secret in config — store one with: buddy cred secret set %s --client-secret\n", credName)
	}
	return nil
}

func runConfigDelete(cfg *buddy.Config, opts buddy.Opts, name string) error {
	if _, ok := cfg.Credentials[name]; !ok {
		return fmt.Errorf("credential %q not found", name)
	}
	if !opts.SkipConfirm {
		fmt.Printf("Delete credential %q? [y/N] ", name)
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(strings.TrimSpace(response)) != "y" {
			fmt.Println("Aborted.")
			return nil
		}
	}
	delete(cfg.Credentials, name)
	if cfg.Cred == name {
		cfg.Cred = buddy.DefaultCred
		fmt.Printf("Note: default credential was %q; reset to %q\n", name, buddy.DefaultCred)
	}
	if err := buddy.SaveConfig(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	fmt.Printf("Deleted credential %q\n", name)
	return nil
}

func runConfigSetDefault(cfg *buddy.Config, name string) error {
	if _, ok := cfg.Credentials[name]; !ok {
		return fmt.Errorf("credential %q not found; run 'buddy cred list' to see available credentials", name)
	}
	cfg.Cred = name
	if err := buddy.SaveConfig(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	fmt.Printf("Default credential set to %q\n", name)
	return nil
}

func runConfigSecret(cfg *buddy.Config, args []string) error {
	if len(args) == 0 || args[0] == "help" {
		fmt.Print(helpCredSecret)
		return nil
	}
	switch args[0] {
	case "set":
		if len(args) < 2 {
			return fmt.Errorf("missing credential name; run 'buddy cred secret help' for usage")
		}
		return runConfigSecretSet(cfg, args[1], args[2:])
	case "get":
		if len(args) < 2 {
			return fmt.Errorf("missing credential name; run 'buddy cred secret help' for usage")
		}
		return buddy.PrintCredentialSecretStatus(cfg, args[1], os.Stdout)
	case "delete":
		if len(args) < 2 {
			return fmt.Errorf("missing credential name; run 'buddy cred secret help' for usage")
		}
		return runConfigSecretDelete(args[1], args[2:])
	case "path":
		return runConfigSecretPath()
	default:
		return fmt.Errorf("unknown cred secret subcommand: %q; run 'buddy cred secret help' for usage", args[0])
	}
}

func runConfigSecretSet(cfg *buddy.Config, credName string, args []string) error {
	if _, err := cfg.ResolveCredential(credName); err != nil {
		return err
	}
	var password, clientSecret string
	wantPassword := false
	wantClientSecret := false
	secretFromStdin := false
	for _, a := range args {
		switch {
		case a == "--secret-from-stdin":
			secretFromStdin = true
		case strings.HasPrefix(a, "--password="):
			wantPassword = true
			password = strings.TrimPrefix(a, "--password=")
		case a == "--password":
			wantPassword = true
		case strings.HasPrefix(a, "--client-secret="):
			wantClientSecret = true
			clientSecret = strings.TrimPrefix(a, "--client-secret=")
		case a == "--client-secret":
			wantClientSecret = true
		default:
			return fmt.Errorf("unknown flag %q; run 'buddy cred secret help' for usage", a)
		}
	}
	if !wantPassword && !wantClientSecret {
		wantPassword = true
		wantClientSecret = true
	}
	if secretFromStdin {
		val, err := buddy.ReadSecretFromStdin()
		if err != nil {
			return err
		}
		if wantPassword && password == "" {
			password = val
		}
		if wantClientSecret && clientSecret == "" {
			clientSecret = val
		}
	}
	if wantPassword && password == "" {
		var err error
		password, err = buddy.PromptSecretLine("Password")
		if err != nil {
			return err
		}
	}
	if wantClientSecret && clientSecret == "" {
		var err error
		clientSecret, err = buddy.PromptSecretLine("Client secret")
		if err != nil {
			return err
		}
	}
	if err := buddy.SetCredentialSecrets(credName, password, clientSecret); err != nil {
		return err
	}
	fmt.Printf("Stored secret(s) for credential %q\n", credName)
	return nil
}

func runConfigSecretDelete(credName string, args []string) error {
	deletePassword := false
	deleteClientSecret := false
	for _, a := range args {
		switch a {
		case "--password":
			deletePassword = true
		case "--client-secret":
			deleteClientSecret = true
		default:
			return fmt.Errorf("unknown flag %q; run 'buddy cred secret help' for usage", a)
		}
	}
	if !deletePassword && !deleteClientSecret {
		deletePassword = true
		deleteClientSecret = true
	}
	if err := buddy.DeleteCredentialSecrets(credName, deletePassword, deleteClientSecret); err != nil {
		return err
	}
	fmt.Printf("Deleted secret(s) for credential %q from platform store\n", credName)
	return nil
}

func runConfigSecretPath() error {
	kind, detail, err := buddy.SecretStoreInfo()
	if err != nil {
		return err
	}
	fmt.Printf("store: %s\n", kind)
	fmt.Printf("path:  %s\n", detail)
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
	case "write-script":
		path := "setup_buddy_role.star"
		if len(args) > 0 {
			path = args[0]
		}
		if err := buddy.WriteSetupScript(path); err != nil {
			return fmt.Errorf("write setup script: %w", err)
		}
		fmt.Printf("Wrote %s\n", path)
		fmt.Println("Run with: buddy", path, "-y")
		return nil
	case "configure-oauth":
		credNameForAPI := credName
		clientDisplayName := "buddy-cli"
		newCredName := "buddy-oauth"
		configureOpts := opts
		usedSetupCred := false
		for _, a := range args {
			switch {
			case strings.HasPrefix(a, "--user="):
				credNameForAPI = strings.TrimPrefix(a, "--user=")
			case strings.HasPrefix(a, "--cred="):
				credNameForAPI = strings.TrimPrefix(a, "--cred=")
				usedSetupCred = true
			case strings.HasPrefix(a, "--client-name="):
				clientDisplayName = strings.TrimPrefix(a, "--client-name=")
			case strings.HasPrefix(a, "--name="):
				newCredName = strings.TrimPrefix(a, "--name=")
			case a == "-y":
				configureOpts.SkipConfirm = true
			}
		}
		if usedSetupCred {
			fmt.Fprintln(os.Stderr, "note: '--cred' is deprecated; use '--user'")
		}
		finalCredName, err := buddy.SetupConfigureOAuth(cfg, credNameForAPI, clientDisplayName, newCredName, configureOpts, log)
		if err != nil {
			return err
		}
		if finalCredName == "" {
			return nil // user declined
		}
		if finalCredName == newCredName {
			fmt.Printf("Added credential %q. Use: buddy --user %s ...\n", finalCredName, finalCredName)
		} else {
			fmt.Printf("Added credential %q (name %q in use). Use: buddy --user %s ...\n", finalCredName, newCredName, finalCredName)
		}
		return nil
	default:
		return fmt.Errorf("unknown setup subcommand: %q (use: write-script, configure-oauth)", subcommand)
	}
}

func runServe(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if len(args) > 0 && args[0] == "help" {
		fmt.Print(helpServe)
		return nil
	}
	var addr, docsRoot string
	var open bool
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.StringVar(&addr, "addr", "127.0.0.1:8742", "Listen address (localhost only)")
	fs.StringVar(&docsRoot, "docs-root", "", "Override embedded docs with a live docs/ directory")
	fs.BoolVar(&open, "open", false, "Open docs in the default browser")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return serve.Run(ctx, serve.Options{
		Addr:     addr,
		DocsRoot: docsRoot,
		Open:     open,
		CredName: credName,
		Opts:     opts,
		Logger:   log,
	})
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
		var filter, sort string
		var pageSize, limit int
		var count bool
		fs := flag.NewFlagSet("mdm find", flag.ContinueOnError)
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
		return fmt.Errorf("unknown mdm subcommand: %q (use: find); run 'buddy mdm help' for usage", subcommand)
	}
}
