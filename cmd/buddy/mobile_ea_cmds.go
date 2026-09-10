package main

import (
	"fmt"
	"log/slog"
	"strings"

	"buddy/internal/buddy"
	"buddy/internal/clispec"
)

func runMobileExtensionAttribute(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if err := resolveArg0("mobile-extension-attribute", args, []string{clispec.SubList, clispec.SubGet, clispec.SubCreate, clispec.SubDelete, clispec.SubHelp}); err != nil {
		return err
	}
	if len(args) == 0 || args[0] == clispec.SubHelp {
		fmt.Print(helpMobileExtAttr)
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
			return fmt.Errorf("unknown list flag %q; run '%s'", rest[0], clispec.UsageHelp(clispec.CmdMobileExtAttr))
		}
		return buddy.MobileExtensionAttributeList(credName, filter, asJSON, log)

	case clispec.SubGet:
		if len(args) < 1 {
			return missingArg("ea-id", clispec.UsageHelp(clispec.CmdMobileExtAttr))
		}
		if args[0] == "id" {
			if len(args) < 2 {
				return missingArg("name", clispec.UsageHelp(clispec.CmdMobileExtAttr))
			}
			id, err := buddy.MobileExtensionAttributeGetID(credName, args[1], log)
			if err != nil {
				return err
			}
			fmt.Println(id)
			return nil
		}
		return buddy.MobileExtensionAttributeGet(credName, args[0], log)

	case clispec.SubCreate:
		return runMobileExtensionAttributeCreate(credName, opts, args, log)

	case clispec.SubDelete:
		if len(args) < 1 {
			return missingArg("ea-id", clispec.UsageHelp(clispec.CmdMobileExtAttr))
		}
		return buddy.MobileExtensionAttributeDelete(credName, opts, args, log)

	default:
		return errUnknownSubcommand("mobile-extension-attribute", subcommand, []string{
			clispec.SubList, clispec.SubGet, clispec.SubCreate, clispec.SubDelete,
		})
	}
}

func runMobileExtensionAttributeCreate(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	var inputType, dataType, display, description, popup, ldap string
	var positional []string
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "--input-type="):
			inputType = strings.TrimPrefix(a, "--input-type=")
		case strings.HasPrefix(a, "--data-type="):
			dataType = strings.TrimPrefix(a, "--data-type=")
		case strings.HasPrefix(a, "--display="):
			display = strings.TrimPrefix(a, "--display=")
		case strings.HasPrefix(a, "--description="):
			description = strings.TrimPrefix(a, "--description=")
		case strings.HasPrefix(a, "--popup="):
			popup = strings.TrimPrefix(a, "--popup=")
		case strings.HasPrefix(a, "--ldap="):
			ldap = strings.TrimPrefix(a, "--ldap=")
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) < 1 {
		return missingArg("name", clispec.UsageHelp(clispec.CmdMobileExtAttr))
	}
	return buddy.MobileExtensionAttributeCreate(credName, opts, buddy.MobileExtensionAttributeCreateSpec{
		Name:                 positional[0],
		InputType:            inputType,
		DataType:             dataType,
		InventoryDisplayType: display,
		Description:          description,
		PopupChoices:         buddy.ParsePopupChoices(popup),
		LDAPAttributeMapping: ldap,
	}, log)
}
