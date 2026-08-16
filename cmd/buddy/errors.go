package main

import (
	"fmt"
	"log/slog"
	"os"
)

func missingArg(what, helpCmd string) error {
	return fmt.Errorf("missing %s; run '%s' for usage", what, helpCmd)
}

func missingArgHint(what, helpCmd string) error {
	return fmt.Errorf("missing %s; run '%s'", what, helpCmd)
}

func exitOnErr(logger *slog.Logger, msg string, err error) {
	if err == nil {
		return
	}
	logger.Error(msg, "error", err)
	fmt.Fprintf(os.Stderr, "%v\n", err)
	os.Exit(1)
}
