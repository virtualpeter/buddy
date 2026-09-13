package main

import (
	"fmt"
	"log/slog"
	"strings"

	"buddy/internal/buddy"
	"buddy/internal/clispec"
)

func runExtensionAttribute(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if err := resolveArg0("extension-attribute", args, []string{
		clispec.SubList, clispec.SubGet, "history", "deps", "templates",
		clispec.SubCreate, clispec.SubUpload, clispec.SubDownload, clispec.SubDelete, clispec.SubSet, clispec.SubHelp,
	}); err != nil {
		return err
	}
	if len(args) == 0 || args[0] == clispec.SubHelp {
		fmt.Print(helpExtensionAttribute)
		return nil
	}
	subcommand := args[0]
	rest := args[1:]

	if (subcommand == clispec.SubGet || subcommand == clispec.SubSet) && (len(rest) == 0 || rest[0] == clispec.SubHelp) {
		fmt.Print(extensionAttributeSubHelp(subcommand))
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
			return fmt.Errorf("unknown list flag %q; run '%s'", rest[0], clispec.UsageHelp(clispec.CmdExtensionAttribute))
		}
		return buddy.ExtensionAttributeList(credName, filter, asJSON, log)

	case clispec.SubGet:
		if len(args) < 1 {
			return missingArg("ea-id", clispec.UsageHelp(clispec.CmdExtensionAttribute, clispec.SubGet))
		}
		return runExtensionAttributeGet(credName, args[0], args[1:], log)

	case "history":
		if len(args) < 1 {
			return missingArg("ea-id", clispec.UsageHelp(clispec.CmdExtensionAttribute))
		}
		return buddy.ExtensionAttributeHistory(credName, args[0], 20, log)

	case "deps":
		if len(args) < 1 {
			return missingArg("ea-id", clispec.UsageHelp(clispec.CmdExtensionAttribute))
		}
		return buddy.ExtensionAttributeDependencies(credName, args[0], log)

	case "templates":
		filter, rest, err := extractStringFlag(args, clispec.FlagFilter)
		if err != nil {
			return err
		}
		asJSON, rest := extractBareFlag(rest, clispec.FlagJSON)
		if len(rest) > 0 {
			return fmt.Errorf("unknown templates flag %q; run '%s'", rest[0], clispec.UsageHelp(clispec.CmdExtensionAttribute))
		}
		return buddy.ExtensionAttributeTemplatesList(credName, filter, asJSON, log)

	case clispec.SubCreate:
		return runExtensionAttributeCreate(credName, opts, args, log)

	case clispec.SubUpload:
		if len(args) < 1 {
			return missingArg("file", clispec.UsageHelp(clispec.CmdExtensionAttribute))
		}
		_, err := buddy.ExtensionAttributeUploadXML(credName, opts, args[0], log)
		return err

	case clispec.SubDownload:
		if len(args) < 1 {
			return missingArg("ea-id", clispec.UsageHelp(clispec.CmdExtensionAttribute))
		}
		outfile := ""
		if len(args) >= 2 {
			outfile = strings.TrimSpace(args[1])
		}
		return buddy.ExtensionAttributeDownloadXML(credName, args[0], outfile, log)

	case clispec.SubDelete:
		if len(args) < 1 {
			return missingArg("ea-id", clispec.UsageHelp(clispec.CmdExtensionAttribute))
		}
		return buddy.ExtensionAttributeDelete(credName, opts, args, log)

	case clispec.SubSet:
		if len(args) < 3 {
			return missingArg("property, ea-id, or value", clispec.UsageHelp(clispec.CmdExtensionAttribute, clispec.SubSet))
		}
		return runExtensionAttributeSet(credName, opts, args, log)

	default:
		return errUnknownSubcommand("extension-attribute", subcommand, []string{
			clispec.SubList, clispec.SubGet, "history", "deps", "templates",
			clispec.SubCreate, clispec.SubUpload, clispec.SubDownload, clispec.SubDelete, clispec.SubSet,
		})
	}
}

func runExtensionAttributeGet(credName string, subcommand string, args []string, log *slog.Logger) error {
	if subcommand == clispec.SubHelp {
		fmt.Print(helpExtensionAttributeGet)
		return nil
	}
	switch subcommand {
	case "contents":
		if len(args) < 1 {
			return missingArg("ea-id", clispec.UsageHelp(clispec.CmdExtensionAttribute, clispec.SubGet))
		}
		return buddy.ExtensionAttributeGetContents(credName, args[0], log)
	case "id":
		if len(args) < 1 {
			return missingArg("name", clispec.UsageHelp(clispec.CmdExtensionAttribute, clispec.SubGet))
		}
		id, err := buddy.ExtensionAttributeGetID(credName, args[0], log)
		if err != nil {
			return err
		}
		fmt.Println(id)
		return nil
	default:
		return buddy.ExtensionAttributeGet(credName, subcommand, log)
	}
}

func runExtensionAttributeCreate(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	var inputType, dataType, display, description, file, popup, ldap, enabled string
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
		case strings.HasPrefix(a, "--file="):
			file = strings.TrimPrefix(a, "--file=")
		case strings.HasPrefix(a, "--popup="):
			popup = strings.TrimPrefix(a, "--popup=")
		case strings.HasPrefix(a, "--ldap="):
			ldap = strings.TrimPrefix(a, "--ldap=")
		case strings.HasPrefix(a, "--enabled="):
			enabled = strings.TrimPrefix(a, "--enabled=")
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) < 1 {
		return missingArg("name", clispec.UsageHelp(clispec.CmdExtensionAttribute))
	}
	spec := buddy.ExtensionAttributeCreateSpec{
		Name:                 positional[0],
		InputType:            inputType,
		DataType:             dataType,
		InventoryDisplayType: display,
		Description:          description,
		File:                 file,
		PopupChoices:         buddy.ParsePopupChoices(popup),
		LDAPAttributeMapping: ldap,
	}
	if enabled != "" {
		b, err := parseEnabledFlag(enabled)
		if err != nil {
			return err
		}
		spec.Enabled = &b
	}
	return buddy.ExtensionAttributeCreate(credName, opts, spec, log)
}

func parseEnabledFlag(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "1", "yes", "y", "on":
		return true, nil
	case "false", "0", "no", "n", "off":
		return false, nil
	default:
		return false, fmt.Errorf("--enabled must be true or false, got %q", s)
	}
}

func runExtensionAttributeSet(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if len(args) > 0 && args[0] == clispec.SubHelp {
		fmt.Print(helpExtensionAttributeSet)
		return nil
	}
	property := args[0]
	eaID := strings.TrimSpace(args[1])
	value := strings.TrimSpace(args[2])

	switch property {
	case "name":
		return buddy.ExtensionAttributeSetName(credName, eaID, opts, value, log)
	case "enabled":
		return buddy.ExtensionAttributeSetEnabled(credName, eaID, opts, value, log)
	case "description":
		return buddy.ExtensionAttributeSetDescription(credName, eaID, opts, value, log)
	case "contents":
		return buddy.ExtensionAttributeSetContents(credName, eaID, opts, value, log)
	case "display":
		return buddy.ExtensionAttributeSetDisplay(credName, eaID, opts, value, log)
	default:
		return fmt.Errorf("unknown extension-attribute set property: %q (use: name, enabled, description, contents, display)", property)
	}
}

func extensionAttributeSubHelp(action string) string {
	switch action {
	case clispec.SubGet:
		return helpExtensionAttributeGet
	case clispec.SubSet:
		return helpExtensionAttributeSet
	default:
		return helpExtensionAttribute
	}
}
