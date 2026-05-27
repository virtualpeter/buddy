package main

import "os"

var knownCommands = map[string]struct{}{
	"help":    {},
	"auth":    {},
	"config":  {},
	"policy":  {},
	"group":   {},
	"profile": {},
	"package": {},
	"computer": {},
	"role":    {},
	"client":  {},
	"script":  {},
	"mdm":     {},
	"setup":   {},
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
