//go:build !nomcp

package main

import (
	"flag"
	"fmt"
	"log/slog"

	"buddy/internal/buddy"
	"buddy/internal/clispec"
	buddymcp "buddy/internal/mcp"
)

func runMCP(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if len(args) > 0 && (args[0] == clispec.SubHelp || args[0] == "-h" || args[0] == "--help") {
		fmt.Print(helpMCP)
		return nil
	}
	if len(args) > 0 && args[0] == "setup" {
		var write, create bool
		var path, host string
		fs := flag.NewFlagSet("mcp setup", flag.ContinueOnError)
		fs.BoolVar(&write, "write", false, "Merge buddy into selected MCP host configs")
		fs.BoolVar(&create, "create", false, "Create missing well-known config files when writing")
		fs.StringVar(&path, "path", "", "Write/inspect a specific MCP config file")
		fs.StringVar(&host, "host", "", "Filter hosts: cursor, claude-desktop (default: all)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		return buddymcp.SetupMCP(buddymcp.SetupOpts{
			Write:  write,
			Create: create,
			Path:   path,
			Host:   host,
		}, opts, log)
	}
	if len(args) > 0 {
		return errUnknownSubcommand("mcp", args[0], []string{"setup", clispec.SubHelp})
	}
	return buddymcp.Run(credName, opts, log, Version)
}
