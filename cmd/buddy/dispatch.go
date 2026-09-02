package main

import (
	"os"

	"buddy/internal/clispec"
)

var knownCommands = func() map[string]struct{} {
	m := make(map[string]struct{}, len(clispec.DispatchNames()))
	for _, n := range clispec.DispatchNames() {
		m[n] = struct{}{}
	}
	return m
}()

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
