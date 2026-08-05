package main

import "os"

var knownCommands = map[string]struct{}{
	"help":          {},
	"version":       {},
	"auth":          {},
	"cred":          {},
	"policy":        {},
	"category":      {},
	"macapp":        {},
	"group":         {},
	"profile":       {},
	"package":       {},
	"computer":      {},
	"role":          {},
	"client":        {},
	"script":        {},
	"mdm":           {},
	"blueprint":     {},
	"device":        {},
	"device-group":  {},
	"ddm":           {},
	"benchmark":     {},
	"setup":         {},
	"serve":         {},
	"mcp":           {},
	"lsp":           {},
}

func isKnownCommand(name string) bool {
	_, ok := knownCommands[name]
	return ok
}

func isScriptPath(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}
