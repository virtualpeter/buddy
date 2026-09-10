package main

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"buddy/internal/buddy"
	"buddy/internal/clispec"
)

func runComputerSearch(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if err := resolveArg0("computer-search", args, []string{clispec.SubList, clispec.SubGet, clispec.SubResults, clispec.SubCreate, clispec.SubPut, clispec.SubDelete, clispec.SubHelp}); err != nil {
		return err
	}
	if len(args) == 0 || args[0] == clispec.SubHelp {
		fmt.Print(helpComputerSearch)
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
		asJSON, rest := extractBareFlag(rest, clispec.FlagJSON)
		if len(rest) > 0 {
			return fmt.Errorf("unknown list flag %q; run '%s'", rest[0], clispec.UsageHelp(clispec.CmdComputerSearch))
		}
		return buddy.ComputerSearchList(credName, filter, asJSON, log)

	case clispec.SubGet:
		if len(args) < 1 {
			return missingArg("search-id", clispec.UsageHelp(clispec.CmdComputerSearch))
		}
		if args[0] == "id" {
			if len(args) < 2 {
				return missingArg("name", clispec.UsageHelp(clispec.CmdComputerSearch))
			}
			id, err := buddy.ComputerSearchGetID(credName, args[1], log)
			if err != nil {
				return err
			}
			fmt.Println(id)
			return nil
		}
		return buddy.ComputerSearchGet(credName, args[0], log)

	case clispec.SubResults:
		if len(args) < 1 {
			return missingArg("search-id", clispec.UsageHelp(clispec.CmdComputerSearch))
		}
		id := args[0]
		filter, rest, err := extractStringFlag(args[1:], clispec.FlagFilter)
		if err != nil {
			return err
		}
		fields, rest, err := extractStringFlag(rest, clispec.FlagFields)
		if err != nil {
			return err
		}
		jqExpr, rest, err := extractStringFlag(rest, clispec.FlagJQ)
		if err != nil {
			return err
		}
		fromJSON, rest, err := extractStringFlag(rest, clispec.FlagFromJSON)
		if err != nil {
			return err
		}
		flatten, rest, err := extractStringFlag(rest, clispec.FlagFlatten)
		if err != nil {
			return err
		}
		asJSON, rest := extractBareFlag(rest, clispec.FlagJSON)
		asCSV, rest := extractBareFlag(rest, clispec.FlagCSV)
		asRaw, rest := extractBareFlag(rest, clispec.FlagRaw)
		includeID, rest := extractBareFlag(rest, clispec.FlagID)
		if len(rest) > 0 {
			return fmt.Errorf("unknown results flag %q; run '%s'", rest[0], clispec.UsageHelp(clispec.CmdComputerSearch))
		}
		return buddy.ComputerSearchResultsPrint(credName, id, filter, buddy.SearchResultPrintOpts{
			Fields:   fields,
			JQ:       jqExpr,
			FromJSON: fromJSON,
			Flatten:  flatten,
			JSON:     asJSON,
			CSV:      asCSV,
			Raw:      asRaw,
			ID:       includeID,
		}, log)

	case clispec.SubCreate:
		return runComputerSearchCreate(credName, opts, args, log)

	case clispec.SubPut:
		return runSearchPut(credName, opts, args, log, clispec.CmdComputerSearch, buddy.ComputerSearchPut)

	case clispec.SubDelete:
		if len(args) < 1 {
			return missingArg("search-id", clispec.UsageHelp(clispec.CmdComputerSearch))
		}
		return buddy.ComputerSearchDelete(credName, opts, args, log)

	default:
		return errUnknownSubcommand("computer-search", subcommand, []string{
			clispec.SubList, clispec.SubGet, clispec.SubResults, clispec.SubCreate, clispec.SubPut, clispec.SubDelete,
		})
	}
}

func runComputerSearchCreate(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	body, err := parseSearchCreateBody(args, clispec.CmdComputerSearch)
	if err != nil {
		return err
	}
	_, err = buddy.ComputerSearchCreate(credName, opts, body, log)
	return err
}

func parseSearchCreateBody(args []string, cmd string) (map[string]interface{}, error) {
	file, rest, err := extractStringFlag(args, clispec.FlagFile)
	if err != nil {
		return nil, err
	}
	if file != "" {
		if len(rest) > 0 {
			return nil, fmt.Errorf("create: extra args with --file; run '%s'", clispec.UsageHelp(cmd))
		}
		return parseSearchBodyFile(file)
	}
	if len(rest) < 1 {
		return nil, missingArg("name or --file=", clispec.UsageHelp(cmd))
	}
	if len(rest) > 1 {
		return nil, fmt.Errorf("create: extra args %q; run '%s'", rest[1], clispec.UsageHelp(cmd))
	}
	return map[string]interface{}{"name": rest[0]}, nil
}

func parseSearchBodyFile(path string) (map[string]interface{}, error) {
	raw, err := buddy.ReadJSONBodyFile(path)
	if err != nil {
		return nil, err
	}
	var body map[string]interface{}
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, fmt.Errorf("parse search JSON: %w", err)
	}
	if body == nil {
		return nil, fmt.Errorf("parse search JSON: expected object")
	}
	return body, nil
}

func runSearchPut(credName string, opts buddy.Opts, args []string, log *slog.Logger, cmd string, put func(string, buddy.Opts, string, map[string]interface{}, *slog.Logger) error) error {
	if len(args) < 1 {
		return missingArg("search-id", clispec.UsageHelp(cmd))
	}
	id := args[0]
	file, rest, err := extractStringFlag(args[1:], clispec.FlagFile)
	if err != nil {
		return err
	}
	if file == "" {
		return missingArg(clispec.FlagEq(clispec.FlagFile, "json"), clispec.UsageHelp(cmd))
	}
	if len(rest) > 0 {
		return fmt.Errorf("put: extra args %q; run '%s'", rest[0], clispec.UsageHelp(cmd))
	}
	body, err := parseSearchBodyFile(file)
	if err != nil {
		return err
	}
	return put(credName, opts, id, body, log)
}
