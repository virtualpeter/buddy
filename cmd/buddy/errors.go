package main

import (
	"fmt"
	"log/slog"
	"os"

	"buddy/internal/clispec"
)

func missingArg(what, helpCmd string) error {
	return fmt.Errorf("%s", clispec.T(clispec.MsgMissingArg, what, helpCmd))
}

func missingArgHint(what, helpCmd string) error {
	return fmt.Errorf("%s", clispec.T(clispec.MsgMissingArgHint, what, helpCmd))
}

func exitOnErr(logger *slog.Logger, msg string, err error) {
	if err == nil {
		return
	}
	logger.Error(msg, "error", err)
	fmt.Fprintf(os.Stderr, "%v\n", err)
	os.Exit(1)
}
