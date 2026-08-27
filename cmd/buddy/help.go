package main

import (
	"fmt"
	"os"
	"strings"

	"buddy/internal/clispec"
	"buddy/internal/help"
)

var helpVersion = help.Version

var helpAuth = help.Auth

var helpCred = help.Cred

// helpPolicy is generated (make gendocs).
var helpPolicy = help.PoliciesCLIHelp

var helpPolicyList = help.PolicyList

var helpPolicyGet = help.PolicyGet

var helpPolicySet = help.PolicySet

var helpPolicyCreate = help.PolicyCreate

var helpPolicyClone = help.PolicyClone

var helpPolicyFind = help.PolicyFind

var helpPolicyDelete = help.PolicyDelete

// helpMdm is generated (make gendocs).
var helpMdm = help.MdmCLIHelp

var helpSetup = help.Setup

// helpCategory is generated (make gendocs).
var helpCategory = help.CategoriesCLIHelp

// helpMacApp is generated (make gendocs).
var helpMacApp = help.MacappsCLIHelp

// helpApp is generated from internal/buddy/app_doc.go (make gendocs).
var helpApp = help.AppsCLIHelp

// helpGroup is generated (make gendocs).
var helpGroup = help.GroupsCLIHelp

// helpProfile is generated (make gendocs).
var helpProfile = help.ProfilesCLIHelp

// helpBlueprint is generated (make gendocs).
var helpBlueprint = help.BlueprintsCLIHelp

// helpDevice is generated (make gendocs).
var helpDevice = help.DevicesCLIHelp

// helpDeviceGroup is generated (make gendocs).
var helpDeviceGroup = help.DeviceGroupsCLIHelp

// helpDDM is generated (make gendocs).
var helpDDM = help.DdmCLIHelp

// helpBenchmark is generated (make gendocs).
var helpBenchmark = help.BenchmarksCLIHelp

// helpPackage is generated (make gendocs).
var helpPackage = help.PackagesCLIHelp

// helpIcon is generated (make gendocs).
var helpIcon = help.IconsCLIHelp

// helpComputer is generated (make gendocs).
var helpComputer = help.ComputersCLIHelp

// helpRole is generated (make gendocs).
var helpRole = help.RolesCLIHelp

// helpClient is generated (make gendocs).
var helpClient = help.ClientsCLIHelp

// helpScript is generated (make gendocs).
var helpScript = help.ScriptsCLIHelp

var helpScriptGet = help.ScriptGet

var helpScriptSet = help.ScriptSet

// printCommandHelp routes 'buddy help [command] [subcommand]' to the appropriate help text.
func printCommandHelp(path []string) {
	key := strings.Join(path, ".")
	helpMap := map[string]string{
		clispec.CmdAuth:       helpAuth,
		clispec.CmdCred:       helpCred,
		clispec.CmdVersion:    helpVersion,
		clispec.CmdShell:      help.Shell,
		clispec.TopicStarlark: help.Starlark,
		clispec.TopicBuiltins: help.Builtins,
		clispec.CmdPolicy:     helpPolicy,
		clispec.HelpPath(clispec.CmdPolicy, clispec.SubList):   helpPolicyList,
		clispec.HelpPath(clispec.CmdPolicy, clispec.SubGet):    helpPolicyGet,
		clispec.HelpPath(clispec.CmdPolicy, clispec.SubSet):    helpPolicySet,
		clispec.HelpPath(clispec.CmdPolicy, clispec.SubCreate): helpPolicyCreate,
		clispec.HelpPath(clispec.CmdPolicy, clispec.SubClone):  helpPolicyClone,
		clispec.HelpPath(clispec.CmdPolicy, clispec.SubFind):   helpPolicyFind,
		clispec.HelpPath(clispec.CmdPolicy, clispec.SubDelete): helpPolicyDelete,
		clispec.CmdCategory:    helpCategory,
		clispec.CmdMacApp:      helpMacApp,
		clispec.CmdApp:         helpApp,
		clispec.CmdGroup:       helpGroup,
		clispec.CmdProfile:     helpProfile,
		clispec.CmdPackage:     helpPackage,
		clispec.CmdIcon:        helpIcon,
		clispec.CmdComputer:    helpComputer,
		clispec.CmdMdm:         helpMdm,
		clispec.CmdBlueprint:   helpBlueprint,
		clispec.CmdDevice:      helpDevice,
		clispec.CmdDeviceGroup: helpDeviceGroup,
		clispec.CmdDDM:         helpDDM,
		clispec.CmdBenchmark:   helpBenchmark,
		clispec.CmdRole:        helpRole,
		clispec.CmdClient:      helpClient,
		clispec.CmdScript:      helpScript,
		clispec.HelpPath(clispec.CmdScript, clispec.SubGet): helpScriptGet,
		clispec.HelpPath(clispec.CmdScript, clispec.SubSet): helpScriptSet,
		clispec.CmdSetup: helpSetup,
		clispec.CmdServe: helpServe,
		clispec.CmdMCP:   helpMCP,
		clispec.CmdLSP:   helpLSP,
	}
	if text, ok := helpMap[key]; ok {
		fmt.Print(text)
	} else {
		fmt.Fprintf(os.Stderr, "%s", clispec.T(clispec.MsgNoHelp, strings.Join(path, " ")))
		printUsage()
	}
}

// policySubHelp returns the help text for a policy subcommand.
func policySubHelp(mode string) string {
	switch mode {
	case clispec.SubList:
		return helpPolicyList
	case clispec.SubGet:
		return helpPolicyGet
	case clispec.SubSet:
		return helpPolicySet
	case clispec.SubCreate:
		return helpPolicyCreate
	case clispec.SubClone:
		return helpPolicyClone
	case clispec.SubFind:
		return helpPolicyFind
	case clispec.SubDelete:
		return helpPolicyDelete
	default:
		return helpPolicy
	}
}

// scriptSubHelp returns the help text for a script subcommand.
func scriptSubHelp(action string) string {
	switch action {
	case clispec.SubGet:
		return helpScriptGet
	case clispec.SubSet:
		return helpScriptSet
	default:
		return helpScript
	}
}
