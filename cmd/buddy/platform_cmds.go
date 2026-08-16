package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"buddy/internal/buddy"
	"buddy/internal/clispec"
)

func readJSONFileArg(args []string) (body []byte, rest []string, err error) {
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
	body, err = buddy.ReadJSONBodyFile(file)
	return body, leftover, err
}

func runDeviceGroup(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if len(args) == 0 || args[0] == clispec.SubHelp {
		fmt.Print(helpDeviceGroup)
		return nil
	}
	subcommand := args[0]
	args = args[1:]
	switch subcommand {
	case clispec.SubList:
		listOpts := buddy.DeviceGroupListOpts{Page: buddy.PageOpts{Page: buddy.PageAll}}
		filter, rest, err := extractStringFlag(args, clispec.FlagFilter)
		if err != nil {
			return err
		}
		listOpts.Filter = filter
		if len(rest) > 0 {
			return fmt.Errorf("unknown list flag %q; run '%s'", rest[0], clispec.UsageHelp(clispec.CmdDeviceGroup))
		}
		return buddy.DeviceGroupList(credName, listOpts, log)
	case clispec.SubGet:
		if len(args) < 1 {
			return missingArgHint("group-id", clispec.UsageHelp(clispec.CmdDeviceGroup))
		}
		return buddy.DeviceGroupGet(credName, args[0], log)
	case clispec.SubCreate:
		body, _, err := readJSONFileArg(args)
		if err != nil {
			return err
		}
		return buddy.DeviceGroupCreate(credName, opts, body, log)
	case clispec.SubSet:
		if len(args) < 1 {
			return missingArgHint("group-id", clispec.UsageHelp(clispec.CmdDeviceGroup))
		}
		groupID := args[0]
		body, _, err := readJSONFileArg(args[1:])
		if err != nil {
			return err
		}
		return buddy.DeviceGroupSet(credName, groupID, opts, body, log)
	case clispec.SubDelete:
		if len(args) < 1 {
			return missingArgHint("group-id", clispec.UsageHelp(clispec.CmdDeviceGroup))
		}
		return buddy.DeviceGroupDelete(credName, args[0], opts, log)
	case "members":
		if len(args) < 1 {
			return missingArgHint("group-id", clispec.UsageHelp(clispec.CmdDeviceGroup))
		}
		return buddy.DeviceGroupMembers(credName, args[0], log)
	case "set-members":
		if len(args) < 1 {
			return missingArgHint("group-id", clispec.UsageHelp(clispec.CmdDeviceGroup))
		}
		groupID := args[0]
		body, _, err := readJSONFileArg(args[1:])
		if err != nil {
			return err
		}
		return buddy.DeviceGroupSetMembers(credName, groupID, opts, body, log)
	default:
		return errUnknownSubcommand("device-group", subcommand, []string{
			clispec.SubList, clispec.SubGet, clispec.SubCreate, clispec.SubSet, clispec.SubDelete, "members", "set-members",
		})
	}
}

func runDevice(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if len(args) == 0 || args[0] == clispec.SubHelp {
		fmt.Print(helpDevice)
		return nil
	}
	subcommand := args[0]
	args = args[1:]
	switch subcommand {
	case clispec.SubList:
		listOpts := buddy.DeviceListOpts{Page: buddy.PageOpts{Page: buddy.PageAll}}
		filter, rest, err := extractStringFlag(args, clispec.FlagFilter)
		if err != nil {
			return err
		}
		sort, rest, err := extractStringFlag(rest, "sort")
		if err != nil {
			return err
		}
		listOpts.Filter = filter
		listOpts.Sort = sort
		if len(rest) > 0 {
			return fmt.Errorf("unknown list flag %q; run '%s'", rest[0], clispec.UsageHelp(clispec.CmdDevice))
		}
		return buddy.DeviceList(credName, listOpts, log)
	case clispec.SubGet:
		if len(args) < 1 {
			return missingArgHint("device-id", clispec.UsageHelp(clispec.CmdDevice))
		}
		return buddy.DeviceGet(credName, args[0], log)
	case clispec.SubSet:
		if len(args) < 1 {
			return missingArgHint("device-id", clispec.UsageHelp(clispec.CmdDevice))
		}
		deviceID := args[0]
		body, _, err := readJSONFileArg(args[1:])
		if err != nil {
			return err
		}
		return buddy.DeviceSet(credName, deviceID, opts, body, log)
	case clispec.SubDelete:
		if len(args) < 1 {
			return missingArgHint("device-id", clispec.UsageHelp(clispec.CmdDevice))
		}
		return buddy.DeviceDelete(credName, args[0], opts, log)
	case "applications":
		if len(args) < 1 {
			return missingArgHint("device-id", clispec.UsageHelp(clispec.CmdDevice))
		}
		deviceID := args[0]
		filter, rest, err := extractStringFlag(args[1:], "filter")
		if err != nil {
			return err
		}
		if len(rest) > 0 {
			return fmt.Errorf("unknown applications flag %q", rest[0])
		}
		return buddy.DeviceApplications(credName, deviceID, filter, log)
	case "user":
		if len(args) < 1 {
			return missingArgHint("user-id", clispec.UsageHelp(clispec.CmdDevice))
		}
		listOpts := buddy.DeviceListOpts{Page: buddy.PageOpts{Page: buddy.PageAll}}
		items, err := buddy.DevicesForUserValue(credName, args[0], listOpts, log)
		if err != nil {
			return err
		}
		enc, _ := json.MarshalIndent(items, "", "  ")
		fmt.Println(string(enc))
		return nil
	case "check-in":
		if len(args) < 1 {
			return missingArgHint("device-id", clispec.UsageHelp(clispec.CmdDevice))
		}
		return buddy.DeviceCheckIn(credName, args[0], opts, log)
	case "restart":
		if len(args) < 1 {
			return missingArgHint("device-id", clispec.UsageHelp(clispec.CmdDevice))
		}
		return buddy.DeviceRestart(credName, args[0], opts, log)
	case "shutdown":
		if len(args) < 1 {
			return missingArgHint("device-id", clispec.UsageHelp(clispec.CmdDevice))
		}
		return buddy.DeviceShutdown(credName, args[0], opts, log)
	case "erase":
		if len(args) < 1 {
			return missingArgHint("device-id", clispec.UsageHelp(clispec.CmdDevice))
		}
		return buddy.DeviceErase(credName, args[0], opts, log)
	case "unmanage":
		if len(args) < 1 {
			return missingArgHint("device-id", clispec.UsageHelp(clispec.CmdDevice))
		}
		return buddy.DeviceUnmanage(credName, args[0], opts, log)
	default:
		return errUnknownSubcommand("device", subcommand, []string{
			clispec.SubList, clispec.SubGet, clispec.SubSet, clispec.SubDelete, "applications", "user", "check-in", "restart", "shutdown", "erase", "unmanage",
		})
	}
}

func runDDM(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	_ = opts
	if len(args) == 0 || args[0] == clispec.SubHelp {
		fmt.Print(helpDDM)
		return nil
	}
	switch args[0] {
	case "declaration":
		if len(args) < 2 {
			return fmt.Errorf("usage: buddy ddm declaration <declaration-id> [--filter=] [--sort=]")
		}
		id := args[1]
		filter, rest, err := extractStringFlag(args[2:], "filter")
		if err != nil {
			return err
		}
		sort, rest, err := extractStringFlag(rest, "sort")
		if err != nil {
			return err
		}
		if len(rest) > 0 {
			return fmt.Errorf("unknown flag %q", rest[0])
		}
		return buddy.DDMDeclarationDevices(credName, id, filter, sort, log)
	case "device":
		if len(args) < 2 {
			return fmt.Errorf("usage: buddy ddm device <device-id> <declarations|channels> [--filter=] [--sort=]")
		}
		deviceID := args[1]
		if len(args) < 3 {
			return fmt.Errorf("usage: buddy ddm device <device-id> <declarations|channels>")
		}
		switch args[2] {
		case "channels":
			return buddy.DDMDeviceChannels(credName, deviceID, log)
		case "declarations":
			filter, rest, err := extractStringFlag(args[3:], "filter")
			if err != nil {
				return err
			}
			sort, rest, err := extractStringFlag(rest, "sort")
			if err != nil {
				return err
			}
			if len(rest) > 0 {
				return fmt.Errorf("unknown flag %q", rest[0])
			}
			return buddy.DDMDeviceDeclarations(credName, deviceID, filter, sort, log)
		default:
			return errUnknownSubcommand("ddm device", args[2], []string{"declarations", "channels"})
		}
	default:
		return errUnknownSubcommand("ddm", args[0], []string{"declaration", "device"})
	}
}

func runBenchmark(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if len(args) == 0 || args[0] == clispec.SubHelp {
		fmt.Print(helpBenchmark)
		return nil
	}
	switch args[0] {
	case clispec.SubList:
		return buddy.BenchmarkList(credName, log)
	case clispec.SubGet:
		if len(args) < 2 {
			return missingArgHint("benchmark-id", clispec.UsageHelp(clispec.CmdBenchmark))
		}
		return buddy.BenchmarkGet(credName, args[1], log)
	case clispec.SubCreate:
		body, _, err := readJSONFileArg(args[1:])
		if err != nil {
			return err
		}
		return buddy.BenchmarkCreate(credName, opts, body, log)
	case clispec.SubDelete:
		if len(args) < 2 {
			return missingArgHint("benchmark-id", clispec.UsageHelp(clispec.CmdBenchmark))
		}
		return buddy.BenchmarkDelete(credName, args[1], opts, log)
	case "baselines":
		return buddy.BenchmarkBaselines(credName, log)
	case "rules":
		baseline := ""
		for _, a := range args[1:] {
			if strings.HasPrefix(a, "--baseline=") {
				baseline = strings.TrimPrefix(a, "--baseline=")
			} else {
				return fmt.Errorf("unknown rules flag %q", a)
			}
		}
		return buddy.BenchmarkCatalogRules(credName, baseline, log)
	case "compliance-percentage":
		if len(args) < 2 {
			return missingArgHint("benchmark-id", clispec.UsageHelp(clispec.CmdBenchmark))
		}
		return buddy.BenchmarkCompliancePercentage(credName, args[1], log)
	case "report-devices":
		if len(args) < 2 {
			return missingArgHint("benchmark-id", clispec.UsageHelp(clispec.CmdBenchmark))
		}
		return buddy.BenchmarkReportDevices(credName, args[1], log)
	case "report-rules":
		if len(args) < 2 {
			return missingArgHint("benchmark-id", clispec.UsageHelp(clispec.CmdBenchmark))
		}
		return buddy.BenchmarkReportRules(credName, args[1], log)
	default:
		return errUnknownSubcommand("benchmark", args[0], []string{
			clispec.SubList, clispec.SubGet, clispec.SubCreate, clispec.SubDelete, "baselines", "rules", "compliance-percentage", "report-devices", "report-rules",
		})
	}
}
