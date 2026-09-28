package buddy

import (
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strings"

	"buddy/internal/buddy/agent"
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
		if hint := firstTimeSetupAgentHint(); hint != "" {
			fmt.Fprintln(os.Stderr, hint)
		}
		options := firstTimeSetupOptions()
		for _, opt := range options {
			fmt.Fprintf(os.Stderr, "  %s) %s\n", opt.key, opt.label)
		}
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
			return restoreSetupBackup(cfg, opts)
		case "5":
			if runtime.GOOS != "darwin" {
				return nil, fmt.Errorf("unknown choice %q (want %s)", strings.TrimSpace(line), setupChoiceList(options))
			}
			if !cfg.HasManagedPrefs() {
				fmt.Fprintln(os.Stderr, "No managed preferences loaded (preference domain virtualpete.buddy.cli). Continuing to fill secrets if any workplaces exist.")
			}
			return finishSetupSecrets(cfg, opts)
		default:
			return nil, fmt.Errorf("unknown choice %q (want %s)", strings.TrimSpace(line), setupChoiceList(options))
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

// firstTimeSetupAgentHint is the macOS-only note when first-run setup still
// runs because no Buddy Agent was found (or it had no workplaces yet).
func firstTimeSetupAgentHint() string {
	if runtime.GOOS != "darwin" || !agent.Enabled() {
		return ""
	}
	return "On this Mac, Buddy Agent (menu bar) can own workplaces and secrets instead — add them there, then re-run this command."
}

type setupOption struct {
	key   string
	label string
}

func firstTimeSetupOptions() []setupOption {
	opts := []setupOption{
		{"1", "Jamf Pro instance (API client JSON, or URL + username/password)"},
		{"2", "Jamf Account / Platform (paste integration curl)"},
		{"3", "Import a quickstart file (YAML or ACS plist, no secrets)"},
		{"4", "Restore a cred backup"},
	}
	if runtime.GOOS == "darwin" {
		opts = append(opts, setupOption{"5", "This Mac already has an org MDM profile"})
	}
	return opts
}

func setupChoiceList(opts []setupOption) string {
	keys := make([]string, len(opts))
	for i, opt := range opts {
		keys[i] = opt.key
	}
	if len(keys) == 0 {
		return ""
	}
	if len(keys) == 1 {
		return keys[0]
	}
	return strings.Join(keys[:len(keys)-1], ", ") + ", or " + keys[len(keys)-1]
}

func restoreSetupBackup(cfg *Config, opts Opts) (*CredAddResult, error) {
	path, err := PromptSecretLine("Cred backup file")
	if err != nil {
		return nil, err
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("cred backup path is required")
	}
	res, err := RestoreCreds(cfg, CredBackupOpts{
		Path:        path,
		SkipConfirm: opts.SkipConfirm,
	})
	if err != nil {
		return nil, err
	}
	if res != nil {
		fmt.Fprintf(os.Stderr, "Restored %s (%d tenant(s), %d auth(s), %d secret(s) in store)\n",
			res.Path, res.Tenants, res.Auths, res.Stored)
	}
	out := &CredAddResult{}
	if d := strings.TrimSpace(cfg.Tenant.Default); d != "" {
		out.TenantName = d
	}
	return out, nil
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
