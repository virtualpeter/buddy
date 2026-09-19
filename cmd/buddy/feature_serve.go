//go:build !noserve

package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"buddy/internal/buddy"
	"buddy/internal/clispec"
	"buddy/internal/serve"
)

func runServe(credName string, opts buddy.Opts, args []string, log *slog.Logger) error {
	if len(args) > 0 && args[0] == clispec.SubHelp {
		fmt.Print(helpServe)
		return nil
	}
	var addr, docsRoot string
	var open, embed bool
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.StringVar(&addr, "addr", "127.0.0.1:8742", "Listen address (localhost only)")
	fs.StringVar(&docsRoot, "docs-root", "", "Override embedded docs with a live docs/ directory")
	fs.BoolVar(&open, "open", false, "Open docs in the default browser")
	fs.BoolVar(&embed, "embed", false, "Embedded chrome: hide logo and docs/ide/config nav (?embed=1)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	tenant, _ := buddy.ActiveSelection()
	return serve.Run(ctx, serve.Options{
		Addr:       addr,
		DocsRoot:   docsRoot,
		Open:       open,
		Embed:      embed,
		Version:    Version,
		TenantName: tenant,
		CredName:   credName,
		Opts:       opts,
		Logger:     log,
	})
}
