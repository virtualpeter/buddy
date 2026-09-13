package main

import (
	"fmt"
	"log/slog"

	"buddy/internal/buddy"
	"buddy/internal/clispec"
)

func runMobileSearch(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if err := resolveArg0("mobile-search", args, []string{clispec.SubList, clispec.SubGet, clispec.SubResults, clispec.SubCreate, clispec.SubPut, clispec.SubDelete, clispec.SubHelp}); err != nil {
		return err
	}
	if len(args) == 0 || args[0] == clispec.SubHelp {
		fmt.Print(helpMobileSearch)
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
			return fmt.Errorf("unknown list flag %q; run '%s'", rest[0], clispec.UsageHelp(clispec.CmdMobileSearch))
		}
		return buddy.MobileSearchList(credName, filter, asJSON, log)

	case clispec.SubGet:
		if len(args) < 1 {
			return missingArg("search-id", clispec.UsageHelp(clispec.CmdMobileSearch))
		}
		if args[0] == "id" {
			if len(args) < 2 {
				return missingArg("name", clispec.UsageHelp(clispec.CmdMobileSearch))
			}
			id, err := buddy.MobileSearchGetID(credName, args[1], log)
			if err != nil {
				return err
			}
			fmt.Println(id)
			return nil
		}
		return buddy.MobileSearchGet(credName, args[0], log)

	case clispec.SubResults:
		if len(args) < 1 {
			return missingArg("search-id", clispec.UsageHelp(clispec.CmdMobileSearch))
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
			return fmt.Errorf("unknown results flag %q; run '%s'", rest[0], clispec.UsageHelp(clispec.CmdMobileSearch))
		}
		return buddy.MobileSearchResultsPrint(credName, id, filter, buddy.SearchResultPrintOpts{
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
		return runMobileSearchCreate(credName, opts, args, log)

	case clispec.SubPut:
		return runSearchPut(credName, opts, args, log, clispec.CmdMobileSearch, buddy.MobileSearchPut)

	case clispec.SubDelete:
		if len(args) < 1 {
			return missingArg("search-id", clispec.UsageHelp(clispec.CmdMobileSearch))
		}
		return buddy.MobileSearchDelete(credName, opts, args, log)

	default:
		return errUnknownSubcommand("mobile-search", subcommand, []string{
			clispec.SubList, clispec.SubGet, clispec.SubResults, clispec.SubCreate, clispec.SubPut, clispec.SubDelete,
		})
	}
}

func runMobileSearchCreate(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	body, err := parseSearchCreateBody(args, clispec.CmdMobileSearch)
	if err != nil {
		return err
	}
	_, err = buddy.MobileSearchCreate(credName, opts, body, log)
	return err
}
