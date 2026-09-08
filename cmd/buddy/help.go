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

var helpPolicyDelete = help.PolicyDelete

var helpPolicyLogs = help.PolicyLogs

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

// helpMobileDevice is generated (make gendocs).
var helpMobileDevice = help.MobileDevicesCLIHelp

// helpMobileGroup is generated (make gendocs).
var helpMobileGroup = help.MobileGroupsCLIHelp

// helpMobileProfile is generated (make gendocs).
var helpMobileProfile = help.MobileProfilesCLIHelp

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

// helpExtensionAttribute is generated (make gendocs).
var helpExtensionAttribute = help.ExtensionAttributesCLIHelp

var helpExtensionAttributeGet = help.ExtensionAttributeGet

var helpExtensionAttributeSet = help.ExtensionAttributeSet

// helpMobileExtAttr is generated (make gendocs).
var helpMobileExtAttr = help.MobileExtensionAttributesCLIHelp

// printCommandHelp routes 'buddy help [command] [subcommand]' to the appropriate help text.
func printCommandHelp(path []string) {
	if len(path) > 0 {
		path = append([]string(nil), path...)
		path[0] = clispec.CanonicalCommand(path[0])
	}
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
		clispec.HelpPath(clispec.CmdPolicy, clispec.SubDelete): helpPolicyDelete,
		clispec.HelpPath(clispec.CmdPolicy, clispec.SubLogs):   helpPolicyLogs,
		clispec.CmdCategory:    helpCategory,
		clispec.CmdMacApp:      helpMacApp,
		clispec.CmdApp:         helpApp,
		clispec.CmdGroup:       helpGroup,
		clispec.CmdProfile:     helpProfile,
		clispec.CmdPackage:     helpPackage,
		clispec.CmdIcon:        helpIcon,
		clispec.CmdComputer:    helpComputer,
		clispec.CmdMdm:         helpMdm,
		clispec.CmdMobile:      helpMobileDevice,
		clispec.CmdMobileGroup: helpMobileGroup,
		clispec.CmdMobileProf:  helpMobileProfile,
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
		clispec.CmdExtensionAttribute: helpExtensionAttribute,
		clispec.HelpPath(clispec.CmdExtensionAttribute, clispec.SubGet): helpExtensionAttributeGet,
		clispec.HelpPath(clispec.CmdExtensionAttribute, clispec.SubSet): helpExtensionAttributeSet,
		clispec.CmdMobileExtAttr: helpMobileExtAttr,
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
	case clispec.SubDelete:
		return helpPolicyDelete
	case clispec.SubLogs:
		return helpPolicyLogs
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
