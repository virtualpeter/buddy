//go:build noserve

package main

import (
	"fmt"
	"log/slog"
	"os"

	"buddy/internal/buddy"
)

func runServe(_ string, _ buddy.Opts, args []string, _ *slog.Logger) error {
	if len(args) > 0 && args[0] == "help" {
		fmt.Print(helpServe)
		fmt.Fprintln(os.Stderr, "\nNote: this binary was built with -tags noserve; serve is not linked.")
		return nil
	}
	return fmt.Errorf("buddy serve is not included in this build (rebuild without -tags noserve)")
}
