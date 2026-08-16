//go:build !nolsp

package main

import (
	"buddy/internal/clispec"
	"fmt"
	"log/slog"

	buddylsp "buddy/internal/lsp"
)

func runLSP(args []string, log *slog.Logger) error {
	if len(args) > 0 && args[0] == clispec.SubHelp {
		fmt.Print(helpLSP)
		return nil
	}
	if err := buddylsp.Run(Version); err != nil {
		return err
	}
	_ = log
	return nil
}
