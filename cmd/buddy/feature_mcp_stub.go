//go:build nomcp

package main

import (
	"fmt"
	"log/slog"
	"os"

	"buddy/internal/buddy"
)

func runMCP(_ string, _ buddy.Opts, args []string, _ *slog.Logger) error {
	if len(args) > 0 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		fmt.Print(helpMCP)
		fmt.Fprintln(os.Stderr, "\nNote: this binary was built with -tags nomcp; mcp is not linked.")
		return nil
	}
	return fmt.Errorf("buddy mcp is not included in this build (rebuild without -tags nomcp)")
}
