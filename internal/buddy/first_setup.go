package buddy

import (
	"fmt"
	"log/slog"
	"os"
)

// RunFirstTimeSetup prints a short intro and runs CredAdd interactively to
// create the first tenant + auth (+ optional vault secret and auth test).
func RunFirstTimeSetup(cfg *Config, opts Opts, log *slog.Logger) (*CredAddResult, error) {
	if cfg == nil {
		return nil, fmt.Errorf("internal error: config is nil")
	}
	if log == nil {
		log = slog.Default()
	}
	fmt.Fprintln(os.Stderr, "buddy is not configured yet. Set up your first tenant and auth (Ctrl-C to abort).")
	res, err := CredAdd(cfg, CredAddOpts{
		SkipConfirm: opts.SkipConfirm,
	}, log)
	if err != nil {
		return res, err
	}
	fmt.Fprintf(os.Stderr, "Configured tenant %q with auth %q\n", res.TenantName, res.AuthName)
	if res.Tested && res.Live != "" {
		fmt.Fprintf(os.Stderr, "Auth test: live=%s\n", res.Live)
	}
	return res, nil
}
