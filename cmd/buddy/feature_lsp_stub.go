//go:build nolsp

package main

import (
	"buddy/internal/clispec"
	"fmt"
	"log/slog"
	"os"
)

func runLSP(args []string, _ *slog.Logger) error {
	if len(args) > 0 && args[0] == clispec.SubHelp {
		fmt.Print(helpLSP)
		fmt.Fprintln(os.Stderr, "\nNote: this binary was built with -tags nolsp; lsp is not linked.")
		return nil
	}
	return fmt.Errorf("buddy lsp is not included in this build (rebuild without -tags nolsp)")
}
