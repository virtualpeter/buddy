package buddy

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// RunFirstTimeSetup prints a chooser (TTY) and creates the first workplace(s)
// plus auth/secret. Non-interactive (-y) requires cred add flags / --paste / --from=.
func RunFirstTimeSetup(cfg *Config, opts Opts, log *slog.Logger) (*CredAddResult, error) {
	if cfg == nil {
		return nil, fmt.Errorf("internal error: config is nil")
	}
	if log == nil {
		log = slog.Default()
	}
	if cfg.HasManagedPrefs() {
		fmt.Fprintln(os.Stderr, "Managed preferences already provide workplaces. Fill secrets (Ctrl-C to abort).")
		return finishSetupSecrets(cfg, opts)
	}

	addOpts := CredAddOpts{SkipConfirm: opts.SkipConfirm}
	interactive := StdinIsTerminal() && !opts.SkipConfirm
	if interactive {
		fmt.Fprintln(os.Stderr, "buddy is not configured yet. How do you want to start? (Ctrl-C to abort)")
		fmt.Fprintln(os.Stderr, "  1) Jamf Pro instance (API client JSON, or URL + username/password)")
		fmt.Fprintln(os.Stderr, "  2) Jamf Account / Platform (paste integration curl)")
		fmt.Fprintln(os.Stderr, "  3) Import a quickstart file (YAML or ACS plist, no secrets)")
		fmt.Fprintln(os.Stderr, "  4) This Mac already has an org MDM profile")
		line, err := PromptSecretLine("Choice [1]")
		if err != nil {
			return nil, err
		}
		switch strings.TrimSpace(line) {
		case "", "1":
			addOpts.Kind = "instance"
		case "2":
			addOpts.Kind = "platform"
			addOpts.WantPaste = true
		case "3":
			path, err := PromptSecretLine("Quickstart file path")
			if err != nil {
				return nil, err
			}
			path = strings.TrimSpace(path)
			if path == "" {
				return nil, fmt.Errorf("quickstart path is required")
			}
			if err := ImportQuickstart(cfg, path); err != nil {
				return nil, err
			}
			fmt.Fprintf(os.Stderr, "Imported identity from %s\n", path)
			return finishSetupSecrets(cfg, opts)
		case "4":
			if !cfg.HasManagedPrefs() {
				fmt.Fprintln(os.Stderr, "No managed preferences loaded (preference domain virtualpete.buddy.cli). Continuing to fill secrets if any workplaces exist.")
			}
			return finishSetupSecrets(cfg, opts)
		default:
			return nil, fmt.Errorf("unknown choice %q (want 1, 2, 3, or 4)", strings.TrimSpace(line))
		}
	}

	fmt.Fprintln(os.Stderr, "Set up your first workplace and auth.")
	res, err := CredAdd(cfg, addOpts, log)
	if err != nil {
		return res, err
	}
	fmt.Fprintf(os.Stderr, "Configured workplace %q with auth %q\n", res.TenantName, res.AuthName)
	if res.Tested && res.Live != "" {
		fmt.Fprintf(os.Stderr, "Auth test: live=%s\n", res.Live)
	}
	return res, nil
}

func finishSetupSecrets(cfg *Config, opts Opts) (*CredAddResult, error) {
	res, err := SetupSecrets(cfg, SetupSecretsOpts{SkipConfirm: opts.SkipConfirm})
	out := &CredAddResult{}
	if res != nil && len(res.CreatedAuths) > 0 {
		out.AuthName = res.CreatedAuths[0]
	}
	if d := strings.TrimSpace(cfg.Tenant.Default); d != "" {
		out.TenantName = d
	}
	return out, err
}
