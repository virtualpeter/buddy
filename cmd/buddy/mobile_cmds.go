package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"buddy/internal/buddy"
	"buddy/internal/clispec"
)

// readXMLFileArg reads --file=<path> from args (or stdin when omitted or "-").
func readXMLFileArg(args []string) (body []byte, rest []string, err error) {
	file := "-"
	var leftover []string
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, clispec.FlagPrefix(clispec.FlagFile)):
			file = strings.TrimPrefix(a, clispec.FlagPrefix(clispec.FlagFile))
		case a == "-":
			file = "-"
		default:
			leftover = append(leftover, a)
		}
	}
	if file == "" || file == "-" {
		body, err = io.ReadAll(os.Stdin)
	} else {
		body, err = os.ReadFile(file)
		if err != nil {
			err = fmt.Errorf("read %s: %w", file, err)
		}
	}
	return body, leftover, err
}

func parseMobileDeviceIDArgs(args []string) ([]int, []string, error) {
	var ids []int
	var rest []string
	for _, a := range args {
		if strings.HasPrefix(a, "--") {
			rest = append(rest, a)
			continue
		}
		id, err := parseInt(a, "mobile-device-id")
		if err != nil {
			return nil, nil, err
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, nil, missingArg("mobile-device-id", clispec.UsageHelp(clispec.CmdMobile))
	}
	return ids, rest, nil
}

func runMobileDevice(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if len(args) == 0 || args[0] == clispec.SubHelp {
		fmt.Print(helpMobileDevice)
		return nil
	}
	subcommand := args[0]
	args = args[1:]
	switch subcommand {
	case clispec.SubList:
		fs := flag.NewFlagSet("mobile-device list", flag.ContinueOnError)
		filter := fs.String(clispec.FlagFilter, "", "Client-side substring on id/name/serial/udid/username/model/type")
		asJSON := fs.Bool(clispec.FlagJSON, false, "Print matching rows as JSON")
		if err := fs.Parse(args); err != nil {
			return err
		}
		return buddy.MobileDeviceSearchPrint(credName, *filter, *asJSON, log)
	case clispec.SubGet:
		fs := flag.NewFlagSet("mobile-device get", flag.ContinueOnError)
		detail := fs.Bool("detail", false, "Fetch full inventory detail (/detail)")
		flagArgs, positionals := staticGroupSplitFlags(args)
		if err := fs.Parse(flagArgs); err != nil {
			return err
		}
		if len(positionals) < 1 {
			return missingArg("mobile-device-id", clispec.UsageHelp(clispec.CmdMobile))
		}
		return buddy.MobileDeviceGet(credName, positionals[0], *detail, log)
	case "update-inventory":
		ids, _, err := parseMobileDeviceIDArgs(args)
		if err != nil {
			return err
		}
		return buddy.MobileDeviceUpdateInventory(credName, opts, ids, log)
	case "restart":
		ids, _, err := parseMobileDeviceIDArgs(args)
		if err != nil {
			return err
		}
		return buddy.MobileDeviceRestart(credName, opts, ids, log)
	case "lock":
		message, rest, err := extractStringFlag(args, "message")
		if err != nil {
			return err
		}
		phone, rest, err := extractStringFlag(rest, "phone")
		if err != nil {
			return err
		}
		pin, rest, err := extractStringFlag(rest, "pin")
		if err != nil {
			return err
		}
		ids, _, err := parseMobileDeviceIDArgs(rest)
		if err != nil {
			return err
		}
		return buddy.MobileDeviceLock(credName, opts, ids, message, phone, pin, log)
	case "erase":
		pin, rest, err := extractStringFlag(args, "pin")
		if err != nil {
			return err
		}
		obliteration, rest, err := extractStringFlag(rest, "obliteration")
		if err != nil {
			return err
		}
		preserveDataPlan := false
		disallowProximity := false
		var filtered []string
		for _, a := range rest {
			switch a {
			case "--preserve-data-plan":
				preserveDataPlan = true
			case "--disallow-proximity-setup":
				disallowProximity = true
			default:
				filtered = append(filtered, a)
			}
		}
		ids, _, err := parseMobileDeviceIDArgs(filtered)
		if err != nil {
			return err
		}
		return buddy.MobileDeviceErase(credName, opts, ids, pin, obliteration, preserveDataPlan, disallowProximity, log)
	default:
		return errUnknownSubcommand("mobile-device", subcommand, []string{
			clispec.SubList, clispec.SubGet, "update-inventory", "restart", "lock", "erase",
		})
	}
}

func runMobileGroup(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if len(args) == 0 || args[0] == clispec.SubHelp {
		fmt.Print(helpMobileGroup)
		return nil
	}
	subcommand := args[0]
	args = args[1:]
	switch subcommand {
	case clispec.SubList:
		kind := "smart"
		if len(args) > 0 && !strings.HasPrefix(args[0], "--") {
			kind = args[0]
			args = args[1:]
		}
		filter, rest, err := extractStringFlag(args, clispec.FlagFilter)
		if err != nil {
			return err
		}
		if len(rest) > 0 {
			return fmt.Errorf("unknown list flag %q; run '%s'", rest[0], clispec.UsageHelp(clispec.CmdMobileGroup))
		}
		var groups []buddy.MobileGroupSummary
		switch kind {
		case "smart":
			groups, err = buddy.MobileGroupSearchSmart(credName, filter, buddy.PageOpts{Page: buddy.PageAll}, log)
		case "static":
			groups, err = buddy.MobileGroupSearchStatic(credName, filter, buddy.PageOpts{Page: buddy.PageAll}, log)
		default:
			return errUnknownSubcommand("mobile-group list", kind, []string{"smart", "static"})
		}
		if err != nil {
			return err
		}
		buddy.MobileGroupListPrint(groups)
		return nil
	case clispec.SubGet:
		if len(args) < 1 {
			return missingArg("group-id", clispec.UsageHelp(clispec.CmdMobileGroup))
		}
		groupID := strings.TrimSpace(args[0])
		body, err := buddy.MobileGroupFetchSmartRaw(credName, groupID)
		if err != nil {
			var staticErr error
			body, staticErr = buddy.MobileGroupFetchStaticRaw(credName, groupID)
			if staticErr != nil {
				return fmt.Errorf("not a smart group (%v) nor a static group (%v)", err, staticErr)
			}
		}
		return printRawJSON(body)
	case clispec.SubCreate:
		if len(args) < 1 {
			return fmt.Errorf("usage: buddy mobile-group create <smart|static> [--file=<file>|-]; run '%s'", clispec.UsageHelp(clispec.CmdMobileGroup))
		}
		kind := args[0]
		body, _, err := readJSONFileArg(args[1:])
		if err != nil {
			return err
		}
		var created *buddy.GroupCreateResult
		switch kind {
		case "smart":
			created, err = buddy.MobileGroupCreateSmart(credName, opts, body, log)
		case "static":
			created, err = buddy.MobileGroupCreateStatic(credName, opts, body, log)
		default:
			return errUnknownSubcommand("mobile-group create", kind, []string{"smart", "static"})
		}
		if err != nil {
			return err
		}
		fmt.Printf("Created mobile-device group id=%s\n", created.ID)
		return nil
	case clispec.SubDelete:
		if len(args) < 2 {
			return fmt.Errorf("usage: buddy mobile-group delete <smart|static> <group-id>; run '%s'", clispec.UsageHelp(clispec.CmdMobileGroup))
		}
		kind := args[0]
		groupID := strings.TrimSpace(args[1])
		switch kind {
		case "smart":
			return buddy.MobileGroupDeleteSmart(credName, opts, groupID, log)
		case "static":
			return buddy.MobileGroupDeleteStatic(credName, opts, groupID, log)
		default:
			return errUnknownSubcommand("mobile-group delete", kind, []string{"smart", "static"})
		}
	case "smart":
		return runMobileGroupSmart(credName, opts, args, log)
	case "static":
		return runMobileGroupStatic(credName, opts, args, log)
	default:
		return errUnknownSubcommand("mobile-group", subcommand, []string{
			clispec.SubList, clispec.SubGet, clispec.SubCreate, clispec.SubDelete, "smart", "static",
		})
	}
}

func runMobileGroupSmart(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: buddy mobile-group smart <members|criteria|set-criteria> [args...]; run '%s'", clispec.UsageHelp(clispec.CmdMobileGroup))
	}
	sub := args[0]
	args = args[1:]
	switch sub {
	case "members":
		filter, rest, err := extractStringFlag(args, clispec.FlagFilter)
		if err != nil {
			return err
		}
		if len(rest) < 1 {
			return missingArg("<group-id>", clispec.UsageHelp(clispec.CmdMobileGroup))
		}
		members, err := buddy.MobileSmartGroupFetchMembers(credName, strings.TrimSpace(rest[0]))
		if err != nil {
			return err
		}
		printMobileMembers(buddy.FilterMobileGroupMembers(members, filter))
		return nil
	case "criteria":
		if len(args) < 1 {
			return missingArg("<group-id>", clispec.UsageHelp(clispec.CmdMobileGroup))
		}
		criteria, err := buddy.MobileSmartGroupCriteriaValue(credName, strings.TrimSpace(args[0]))
		if err != nil {
			return err
		}
		enc, err := json.MarshalIndent(criteria, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(enc))
		return nil
	case "set-criteria":
		if len(args) < 1 {
			return missingArg("<group-id>", clispec.UsageHelp(clispec.CmdMobileGroup))
		}
		groupID := strings.TrimSpace(args[0])
		body, rest, err := readJSONFileArg(args[1:])
		if err != nil {
			return err
		}
		if len(rest) > 0 {
			return fmt.Errorf("unexpected argument %q; usage: buddy mobile-group smart set-criteria <group-id> [%s|-]", rest[0], clispec.FlagEq(clispec.FlagFile, "json"))
		}
		return buddy.MobileSmartGroupSetCriteria(credName, opts, groupID, body, log)
	default:
		return errUnknownSubcommand("mobile-group smart", sub, []string{"members", "criteria", "set-criteria"})
	}
}

func runMobileGroupStatic(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: buddy mobile-group static <members|add|remove|set> [args...]; run '%s'", clispec.UsageHelp(clispec.CmdMobileGroup))
	}
	sub := args[0]
	args = args[1:]
	switch sub {
	case "members":
		filter, rest, err := extractStringFlag(args, clispec.FlagFilter)
		if err != nil {
			return err
		}
		if len(rest) < 1 {
			return missingArg("<group-id>", clispec.UsageHelp(clispec.CmdMobileGroup))
		}
		members, err := buddy.MobileStaticGroupFetchMembers(credName, strings.TrimSpace(rest[0]))
		if err != nil {
			return err
		}
		printMobileMembers(buddy.FilterMobileGroupMembers(members, filter))
		return nil
	case clispec.SubAdd, "remove", clispec.SubSet:
		deviceIDsFlag, rest, err := extractStringFlag(args, "device-ids")
		if err != nil {
			return err
		}
		if len(rest) < 1 {
			return missingArg("<group-id>", clispec.UsageHelp(clispec.CmdMobileGroup))
		}
		if deviceIDsFlag == "" {
			return fmt.Errorf("buddy mobile-group static %s requires --device-ids=<id>[,<id>...]", sub)
		}
		groupID := strings.TrimSpace(rest[0])
		ids := parseCommaSeparatedIDs(deviceIDsFlag)
		switch sub {
		case clispec.SubAdd:
			return buddy.MobileStaticGroupAssign(credName, opts, groupID, ids, nil, log)
		case "remove":
			return buddy.MobileStaticGroupAssign(credName, opts, groupID, nil, ids, log)
		default:
			return buddy.MobileStaticGroupSetDevices(credName, opts, groupID, ids, log)
		}
	default:
		return errUnknownSubcommand("mobile-group static", sub, []string{"members", clispec.SubAdd, "remove", clispec.SubSet})
	}
}

func printMobileMembers(members []buddy.MobileDeviceMember) {
	fmt.Printf("%-8s  %-40s  %s\n", "ID", "Name", "Serial")
	fmt.Printf("%-8s  %-40s  %s\n", "--------", "----------------------------------------", "--------------")
	for _, m := range members {
		fmt.Printf("%-8s  %-40s  %s\n", m.ID, m.Name, m.Serial)
	}
	fmt.Printf("\n%d device(s)\n", len(members))
}

func printRawJSON(body []byte) error {
	var v interface{}
	if err := json.Unmarshal(body, &v); err != nil {
		fmt.Println(string(body))
		return nil
	}
	enc, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(enc))
	return nil
}

func runMobileProfile(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if len(args) == 0 || args[0] == clispec.SubHelp {
		fmt.Print(helpMobileProfile)
		return nil
	}
	subcommand := args[0]
	args = args[1:]
	switch subcommand {
	case clispec.SubList:
		filter, _, err := extractStringFlag(args, clispec.FlagFilter)
		if err != nil {
			return err
		}
		return buddy.MobileProfileList(credName, filter, log)
	case clispec.SubGet:
		if len(args) < 1 {
			return missingArg("profile-id", clispec.UsageHelp(clispec.CmdMobileProf))
		}
		profileID, err := parseInt(args[0], "profile-id")
		if err != nil {
			return err
		}
		return buddy.MobileProfileGetXML(credName, profileID, os.Stdout, log)
	case "put":
		redeploy := false
		var filtered []string
		for _, a := range args {
			if a == "--redeploy" {
				redeploy = true
			} else {
				filtered = append(filtered, a)
			}
		}
		if len(filtered) < 1 {
			return missingArg("profile-id", clispec.UsageHelp(clispec.CmdMobileProf))
		}
		profileID, err := parseInt(filtered[0], "profile-id")
		if err != nil {
			return err
		}
		body, _, err := readXMLFileArg(filtered[1:])
		if err != nil {
			return err
		}
		return buddy.MobileProfilePutXML(credName, opts, profileID, body, redeploy, log)
	case clispec.SubCreate:
		body, _, err := readXMLFileArg(args)
		if err != nil {
			return err
		}
		id, err := buddy.MobileProfileCreateXML(credName, opts, body, log)
		if err != nil {
			return err
		}
		fmt.Printf("Created mobile profile id=%d\n", id)
		return nil
	case clispec.SubDelete:
		if len(args) < 1 {
			return missingArg("profile-id", clispec.UsageHelp(clispec.CmdMobileProf))
		}
		profileID, err := parseInt(args[0], "profile-id")
		if err != nil {
			return err
		}
		return buddy.MobileProfileDelete(credName, opts, profileID, log)
	default:
		return errUnknownSubcommand("mobile-profile", subcommand, []string{
			clispec.SubList, clispec.SubGet, "put", clispec.SubCreate, clispec.SubDelete,
		})
	}
}
