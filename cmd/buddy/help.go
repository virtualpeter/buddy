package main

import (
	"fmt"
	"os"
	"strings"

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
		"auth":          helpAuth,
		"cred":          helpCred,
		"version":       helpVersion,
		"shell":         help.Shell,
		"starlark":      help.Starlark,
		"builtins":      help.Builtins,
		"policy":        helpPolicy,
		"policy.list":   helpPolicyList,
		"policy.get":    helpPolicyGet,
		"policy.set":    helpPolicySet,
		"policy.create": helpPolicyCreate,
		"policy.clone":  helpPolicyClone,
		"policy.find":   helpPolicyFind,
		"policy.delete": helpPolicyDelete,
		"category":      helpCategory,
		"macapp":        helpMacApp,
		"app":           helpApp,
		"group":         helpGroup,
		"profile":       helpProfile,
		"package":       helpPackage,
		"computer":      helpComputer,
		"mdm":           helpMdm,
		"blueprint":     helpBlueprint,
		"device":        helpDevice,
		"device-group":  helpDeviceGroup,
		"ddm":           helpDDM,
		"benchmark":     helpBenchmark,
		"role":          helpRole,
		"client":        helpClient,
		"script":        helpScript,
		"script.get":    helpScriptGet,
		"script.set":    helpScriptSet,
		"setup":         helpSetup,
		"serve":         helpServe,
		"mcp":           helpMCP,
		"lsp":           helpLSP,
	}
	if text, ok := helpMap[key]; ok {
		fmt.Print(text)
	} else {
		fmt.Fprintf(os.Stderr, "No help available for %q\n\n", strings.Join(path, " "))
		printUsage()
	}
}

// policySubHelp returns the help text for a policy subcommand.
func policySubHelp(mode string) string {
	switch mode {
	case "list":
		return helpPolicyList
	case "get":
		return helpPolicyGet
	case "set":
		return helpPolicySet
	case "create":
		return helpPolicyCreate
	case "clone":
		return helpPolicyClone
	case "find":
		return helpPolicyFind
	case "delete":
		return helpPolicyDelete
	default:
		return helpPolicy
	}
}

// scriptSubHelp returns the help text for a script subcommand.
func scriptSubHelp(action string) string {
	switch action {
	case "get":
		return helpScriptGet
	case "set":
		return helpScriptSet
	default:
		return helpScript
	}
}
