package main

import (
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"

	"buddy/internal/buddy"
	"buddy/internal/clispec"
)

func runCred(cfg *buddy.Config, opts buddy.Opts, args []string, log *slog.Logger) error {
	if len(args) == 0 || args[0] == clispec.SubHelp {
		fmt.Print(helpCred)
		return nil
	}
	switch args[0] {
	case clispec.SubList:
		return runCredList(cfg)
	case clispec.SubStatus:
		return runCredStatus(cfg, args[1:], log)
	case clispec.SubAdd:
		return runCredAdd(cfg, opts, args[1:], log)
	case clispec.SubGet:
		return runCredGet(cfg, args[1:])
	case clispec.SubSet:
		return runCredSet(cfg, opts, args[1:])
	case clispec.SubDelete:
		return runCredDelete(cfg, opts, args[1:])
	case "use":
		return runCredUse(cfg, args[1:])
	case "allow":
		return fmt.Errorf("'cred allow' was removed; set auth channels with 'buddy cred set auth <name> --channels=cli,mcp,ide' (or --channels=all)")
	case "store":
		return runCredStore()
	case "clean":
		return runCredClean(cfg, opts, args[1:])
	case "erase":
		return runCredErase(opts, args[1:])
	case "migrate":
		return runCredMigrate(cfg, opts, args[1:])
	case "backup":
		return runCredBackup(cfg, opts, args[1:])
	case "restore":
		return runCredRestore(cfg, opts, args[1:])
	default:
		return errUnknownSubcommand("cred", args[0], []string{
			clispec.SubList, clispec.SubStatus, clispec.SubAdd, clispec.SubGet, clispec.SubSet, clispec.SubDelete, "use", "store", "migrate", "clean", "erase", "backup", "restore", clispec.SubHelp,
		})
	}
}

func runCredGet(cfg *buddy.Config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing item; usage: buddy cred get tenant|auth|secret|channel <name> (see '%s')", clispec.UsageHelp(clispec.CmdCred))
	}
	switch args[0] {
	case "tenant":
		if len(args) < 2 {
			return fmt.Errorf("missing tenant name")
		}
		return runCredShowTenant(cfg, args[1])
	case "auth":
		if len(args) < 2 {
			return fmt.Errorf("missing auth name")
		}
		return runCredShowAuth(cfg, args[1])
	case "channel":
		if len(args) < 2 {
			return fmt.Errorf("missing channel name (cli|mcp|ide)")
		}
		return runCredShowChannel(cfg, args[1])
	case "secret":
		if len(args) < 2 {
			return fmt.Errorf("missing auth name")
		}
		return buddy.PrintCredentialSecretStatus(cfg, args[1], os.Stdout)
	default:
		// Convenience: cred get <name> tries auth then tenant.
		if _, ok := cfg.Auths[args[0]]; ok {
			return runCredShowAuth(cfg, args[0])
		}
		if _, ok := cfg.Tenants[args[0]]; ok {
			return runCredShowTenant(cfg, args[0])
		}
		return fmt.Errorf("unknown %q (expected tenant|auth|secret|channel <name>); run 'buddy cred list'", args[0])
	}
}

func runCredSet(cfg *buddy.Config, opts buddy.Opts, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing item; usage: buddy cred set tenant|auth|secret <name> … (see '%s')", clispec.UsageHelp(clispec.CmdCred))
	}
	switch args[0] {
	case "tenant":
		return runCredSetTenant(cfg, opts, args[1:])
	case "auth":
		return runCredSetAuth(cfg, opts, args[1:])
	case "secret":
		if len(args) < 2 {
			return fmt.Errorf("missing auth name")
		}
		return runCredSetSecret(cfg, args[1], args[2:])
	case "channel":
		return runCredSetChannel(cfg, args[1:])
	default:
		return errUnknownSubcommand("cred set", args[0], []string{"tenant", "auth", "channel", "secret"})
	}
}

func runCredDelete(cfg *buddy.Config, opts buddy.Opts, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing item; usage: buddy cred delete tenant|auth|secret <name> (see '%s')", clispec.UsageHelp(clispec.CmdCred))
	}
	switch args[0] {
	case "tenant":
		if len(args) < 2 {
			return fmt.Errorf("missing tenant name")
		}
		return runCredDeleteTenant(cfg, opts, args[1])
	case "auth":
		if len(args) < 2 {
			return fmt.Errorf("missing auth name")
		}
		return runCredDeleteAuth(cfg, opts, args[1])
	case "secret":
		if len(args) < 2 {
			return fmt.Errorf("missing auth name")
		}
		return runCredDeleteSecret(args[1], args[2:])
	default:
		return errUnknownSubcommand("cred delete", args[0], []string{"tenant", "auth", "secret"})
	}
}

func runCredUse(cfg *buddy.Config, args []string) error {
	surface, rest, err := parseCredChannelFlags(args)
	if err != nil {
		return err
	}
	if len(rest) == 0 {
		if surface == "" {
			fmt.Println(cfg.Tenant.Default)
		} else {
			fmt.Println(cfg.DefaultTenant(surface))
		}
		return nil
	}
	if len(rest) != 1 {
		return fmt.Errorf("usage: buddy cred use [tenant] or buddy cred use --cli|--mcp|--ide [tenant]")
	}
	name := rest[0]
	if _, ok := cfg.Tenants[name]; !ok {
		return fmt.Errorf("tenant %q not found; run 'buddy cred list'", name)
	}
	if surface == "" {
		cfg.SetDefaultTenant(name)
		if err := buddy.SaveConfig(cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}
		fmt.Printf("tenant.default set to %q\n", name)
		return nil
	}
	if err := cfg.SetChannelPolicy(surface, "", name, nil, false, true, false, nil, false); err != nil {
		return err
	}
	if err := buddy.SaveConfig(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	fmt.Printf("channels.%s.tenant set to %q\n", surface, name)
	return nil
}

// parseCredChannelFlags extracts --cli|--mcp|--ide (--notebook aliases --ide).
func parseCredChannelFlags(args []string) (channel string, rest []string, err error) {
	for _, a := range args {
		switch a {
		case "--cli":
			if channel != "" {
				return "", nil, fmt.Errorf("only one of --cli|--mcp|--ide allowed")
			}
			channel = buddy.ChannelCLI
		case "--mcp":
			if channel != "" {
				return "", nil, fmt.Errorf("only one of --cli|--mcp|--ide allowed")
			}
			channel = buddy.ChannelMCP
		case "--ide":
			if channel != "" {
				return "", nil, fmt.Errorf("only one of --cli|--mcp|--ide allowed")
			}
			channel = buddy.ChannelIDE
		case "--notebook":
			if channel != "" {
				return "", nil, fmt.Errorf("only one of --cli|--mcp|--ide allowed")
			}
			fmt.Fprintln(os.Stderr, "warning: --notebook is deprecated; use --ide")
			channel = buddy.ChannelIDE
		case "--serve":
			return "", nil, fmt.Errorf("--serve was removed; use --ide")
		default:
			rest = append(rest, a)
		}
	}
	return channel, rest, nil
}

func resolveCredChannelName(name string) (string, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == buddy.ChannelNotebook {
		fmt.Fprintln(os.Stderr, "warning: channel notebook is deprecated; using ide")
		return buddy.ChannelIDE, nil
	}
	switch name {
	case buddy.ChannelCLI, buddy.ChannelMCP, buddy.ChannelIDE:
		return name, nil
	default:
		return "", fmt.Errorf("unknown channel %q (want cli, mcp, or ide)", name)
	}
}

func runCredShowChannel(cfg *buddy.Config, name string) error {
	resolved, err := resolveCredChannelName(name)
	if err != nil {
		return err
	}
	name = resolved
	p := cfg.ChannelPolicy(name)
	fmt.Printf("channel:  %s\n", name)
	fmt.Printf("access:   %s\n", p.EffectiveAccess())
	def := cfg.DefaultTenant(name)
	fmt.Printf("tenant:   %s", def)
	if strings.TrimSpace(p.Tenant) == "" {
		fmt.Printf(" (from tenant.default)")
	}
	fmt.Println()
	if len(p.Tenants) == 0 {
		fmt.Printf("tenants:  (all)\n")
	} else {
		fmt.Printf("tenants:  %s\n", strings.Join(p.Tenants, ", "))
	}
	if len(p.DenyBuiltins) == 0 {
		fmt.Printf("deny_builtins:  (none)\n")
	} else {
		fmt.Printf("deny_builtins:  %s\n", strings.Join(p.DenyBuiltins, ", "))
	}
	return nil
}

func runCredSetChannel(cfg *buddy.Config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: buddy cred set channel <cli|mcp|ide> [--access=readonly|full] [--tenant=…] [--tenants=a,b] [--deny-builtins=a,b,jamf_*]")
	}
	name, err := resolveCredChannelName(args[0])
	if err != nil {
		return err
	}
	var access, tenant, tenantsCSV, denyCSV string
	var setAccess, setTenant, setTenants, setDeny bool
	for _, a := range args[1:] {
		switch {
		case strings.HasPrefix(a, "--access="):
			access = strings.TrimPrefix(a, "--access=")
			setAccess = true
		case strings.HasPrefix(a, "--tenant="):
			tenant = strings.TrimPrefix(a, "--tenant=")
			setTenant = true
		case a == "--tenants=" || a == "--tenants":
			tenantsCSV = ""
			setTenants = true
		case strings.HasPrefix(a, "--tenants="):
			tenantsCSV = strings.TrimPrefix(a, "--tenants=")
			setTenants = true
		case a == "--deny-builtins=" || a == "--deny-builtins":
			denyCSV = ""
			setDeny = true
		case strings.HasPrefix(a, "--deny-builtins="):
			denyCSV = strings.TrimPrefix(a, "--deny-builtins=")
			setDeny = true
		default:
			return fmt.Errorf("unknown flag %q; usage: buddy cred set channel <name> [--access=…] [--tenant=…] [--tenants=…] [--deny-builtins=…]", a)
		}
	}
	if !setAccess && !setTenant && !setTenants && !setDeny {
		return fmt.Errorf("specify at least one of --access, --tenant, --tenants, --deny-builtins")
	}
	var tenants []string
	if setTenants && tenantsCSV != "" {
		for _, p := range strings.Split(tenantsCSV, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				tenants = append(tenants, p)
			}
		}
	}
	var denyBuiltins []string
	if setDeny && denyCSV != "" {
		for _, p := range strings.Split(denyCSV, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				denyBuiltins = append(denyBuiltins, p)
			}
		}
	}
	if setTenant && tenant != "" {
		if _, ok := cfg.Tenants[tenant]; !ok {
			return fmt.Errorf("tenant %q not found; run 'buddy cred list'", tenant)
		}
	}
	if setTenants {
		for _, tn := range tenants {
			if _, ok := cfg.Tenants[tn]; !ok {
				return fmt.Errorf("tenant %q not found; run 'buddy cred list'", tn)
			}
		}
	}
	if err := cfg.SetChannelPolicy(name, access, tenant, tenants, setAccess, setTenant, setTenants, denyBuiltins, setDeny); err != nil {
		return err
	}
	if err := cfg.ValidateTenantSelectionAndChannels(); err != nil {
		return err
	}
	if err := buddy.SaveConfig(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	fmt.Printf("Updated channels.%s\n", name)
	return runCredShowChannel(cfg, name)
}

func runCredStatus(cfg *buddy.Config, args []string, log *slog.Logger) error {
	test := false
	for _, a := range args {
		switch a {
		case "--test":
			test = true
		case "-y":
			// live tests do not prompt
		default:
			return fmt.Errorf("unknown argument %q; usage: buddy cred status [--test]", a)
		}
	}
	var testErr error
	if test {
		results, err := buddy.AuthTestAll(cfg, log)
		if len(results) > 0 {
			buddy.PrintAuthTestAll(results)
			fmt.Println()
		}
		testErr = err
	}
	buddy.FormatCoverageReport(buddy.BuildCoverageReport(cfg), os.Stdout)
	return testErr
}

func runCredAdd(cfg *buddy.Config, opts buddy.Opts, args []string, log *slog.Logger) error {
	addOpts, err := parseCredAddArgs(args, opts.SkipConfirm)
	if err != nil {
		return err
	}
	res, err := buddy.CredAdd(cfg, addOpts, log)
	if res != nil {
		if res.CreatedTenant {
			fmt.Printf("Added tenant %q\n", res.TenantName)
		}
		fmt.Printf("Added auth %q for tenant %q\n", res.AuthName, res.TenantName)
		if res.Tested && res.Live != "" {
			fmt.Printf("auth test: live=%s\n", res.Live)
		}
	}
	return err
}

func parseCredAddArgs(args []string, skipConfirm bool) (buddy.CredAddOpts, error) {
	out := buddy.CredAddOpts{SkipConfirm: skipConfirm}
	stringFlags := map[string]*string{
		"name":          &out.AuthName,
		"tenant":        &out.TenantName,
		"url":           &out.URL,
		"region":        &out.Region,
		"tenant-id":     &out.TenantID,
		"surface":       &out.Surface,
		"type":          &out.Type,
		"client-id":     &out.ClientID,
		"username":      &out.Username,
		"password":      &out.Password,
		"client-secret": &out.ClientSecret,
	}
	pos := ""
	posSet := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "-y":
			out.SkipConfirm = true
			continue
		case "--test":
			out.Test = true
			continue
		case "--no-test":
			out.NoTest = true
			continue
		}
		if !strings.HasPrefix(a, "-") {
			if posSet {
				return buddy.CredAddOpts{}, fmt.Errorf("unexpected argument %q", a)
			}
			pos, posSet = a, true
			continue
		}
		fname, val, hasVal, err := parseFlag(a)
		if err != nil {
			return buddy.CredAddOpts{}, err
		}
		dest, ok := stringFlags[fname]
		if !ok {
			return buddy.CredAddOpts{}, fmt.Errorf("unknown flag --%s", fname)
		}
		if !hasVal {
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return buddy.CredAddOpts{}, fmt.Errorf("flag --%s requires a value", fname)
			}
			i++
			val = args[i]
		}
		*dest = val
	}
	if posSet {
		if out.AuthName != "" && out.AuthName != pos {
			return buddy.CredAddOpts{}, fmt.Errorf("auth name given twice")
		}
		out.AuthName = pos
	}
	return out, nil
}

func runCredList(cfg *buddy.Config) error {
	if len(cfg.Tenants) == 0 && len(cfg.Auths) == 0 {
		fmt.Printf("No tenants/auths configured. Use 'buddy cred set tenant|auth …' (see '%s').\n", clispec.UsageHelp(clispec.CmdCred))
		return nil
	}
	fmt.Println("TENANTS")
	fmt.Printf("%-18s  %-36s  %-8s  %s\n", "NAME", "URL", "REGION", "TENANT_ID")
	fmt.Printf("%-18s  %-36s  %-8s  %s\n", "------------------", "------------------------------------", "--------", "---------")
	tnames := make([]string, 0, len(cfg.Tenants))
	for n := range cfg.Tenants {
		tnames = append(tnames, n)
	}
	sort.Strings(tnames)
	for _, name := range tnames {
		t := cfg.Tenants[name]
		marker := ""
		if name == cfg.Tenant.Default {
			marker = " *"
		}
		url := t.InstanceBaseURL()
		if url == "" {
			url = t.GatewayBaseURL()
		}
		if url == "" {
			url = "-"
		}
		region := t.EffectiveRegion()
		if region == "" {
			region = "-"
		}
		tid := t.EffectiveTenantID()
		if tid == "" {
			tid = "-"
		}
		fmt.Printf("%-18s  %-36s  %-8s  %s%s\n", name, url, region, tid, marker)
	}
	fmt.Printf("\n* = default tenant\n\n")
	fmt.Println("AUTHS")
	fmt.Printf("%-18s  %-10s  %-10s  %-14s  %s\n", "NAME", "TYPE", "ACCESS", "CHANNELS", "TENANTS")
	fmt.Printf("%-18s  %-10s  %-10s  %-14s  %s\n", "------------------", "----------", "----------", "--------------", "-------")
	anames := make([]string, 0, len(cfg.Auths))
	for n := range cfg.Auths {
		anames = append(anames, n)
	}
	sort.Strings(anames)
	for _, name := range anames {
		a := cfg.Auths[name]
		ch := "all"
		if len(a.Channels) > 0 {
			ch = strings.Join(a.Channels, ",")
		}
		fmt.Printf("%-18s  %-10s  %-10s  %-14s  %s\n", name, a.Type, a.EffectiveAccess(), ch, strings.Join(a.Tenants, ","))
	}
	sel := cfg.Channels
	if len(sel) > 0 {
		fmt.Printf("\nCHANNEL POLICIES\n")
		names := make([]string, 0, len(sel))
		for n := range sel {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			p := sel[n]
			parts := []string{"access=" + p.EffectiveAccess()}
			if t := strings.TrimSpace(p.Tenant); t != "" {
				parts = append(parts, "tenant="+t)
			}
			if len(p.Tenants) > 0 {
				parts = append(parts, "tenants="+strings.Join(p.Tenants, ","))
			}
			fmt.Printf("  %-10s  %s\n", n, strings.Join(parts, "  "))
		}
	}
	if buddy.SecretStoreAvailable() {
		var inline []string
		for name, a := range cfg.Auths {
			if a.Password != "" || a.ClientSecret != "" {
				inline = append(inline, name)
			}
		}
		sort.Strings(inline)
		if len(inline) > 0 {
			fmt.Fprintf(os.Stderr, "\nwarning: inline secrets in config.yaml for: %s\n", strings.Join(inline, ", "))
			fmt.Fprintf(os.Stderr, "move them to the secure store with: buddy cred migrate\n")
		}
	}
	if sum := cfg.ManagedPrefsSummary(); sum != "" {
		fmt.Printf("\n%s\n", sum)
	}
	return nil
}

func runCredShowTenant(cfg *buddy.Config, name string) error {
	t, err := cfg.ResolveTenant(name)
	if err != nil {
		return err
	}
	fmt.Printf("tenant:   %s\n", name)
	if u := t.InstanceBaseURL(); u != "" {
		fmt.Printf("url:      %s\n", u)
	}
	if r := t.EffectiveRegion(); r != "" {
		fmt.Printf("region:   %s\n", r)
	}
	if gw := t.GatewayBaseURL(); gw != "" && t.InstanceBaseURL() == "" {
		fmt.Printf("gateway:  %s\n", gw)
	}
	if tid := t.EffectiveTenantID(); tid != "" {
		fmt.Printf("tenantId: %s\n", tid)
	}
	fmt.Printf("default:  %v\n", name == cfg.Tenant.Default)
	linked := cfg.AuthsForTenant(name)
	sort.Strings(linked)
	fmt.Printf("auths:    %s\n", strings.Join(linked, ", "))
	rep := buddy.BuildCoverageReport(cfg)
	for _, cell := range rep.Cells {
		if cell.Tenant != name {
			continue
		}
		names := make([]string, 0, len(cell.Auths))
		for _, a := range cell.Auths {
			names = append(names, a.Name)
		}
		line := strings.Join(names, ", ")
		if line == "" {
			line = "—"
		}
		if cell.Gap {
			fmt.Printf("  %s: %s  GAP (%s)\n", cell.Surface, line, cell.GapReason)
		} else {
			fmt.Printf("  %s: %s\n", cell.Surface, line)
		}
	}
	return nil
}

func runCredShowAuth(cfg *buddy.Config, name string) error {
	a, err := cfg.ResolveAuth(name)
	if err != nil {
		return err
	}
	fmt.Printf("auth:     %s\n", name)
	fmt.Printf("type:     %s\n", a.Type)
	fmt.Printf("access:   %s\n", a.EffectiveAccess())
	if len(a.Channels) == 0 {
		fmt.Printf("channels: all\n")
	} else {
		fmt.Printf("channels: %s\n", strings.Join(a.Channels, ", "))
	}
	fmt.Printf("tenants:  %s\n", strings.Join(a.Tenants, ", "))
	storeAvailable := buddy.SecretStoreAvailable()
	inlineWarn := false
	switch a.Type {
	case "command":
		fmt.Printf("command:  %s\n", a.RunCommand)
	case "basic":
		fmt.Printf("username: %s\n", a.Username)
		if a.Password != "" {
			fmt.Printf("password: %s (inline in config)\n", redact(a.Password))
			inlineWarn = storeAvailable
		} else if ok, _ := buddy.HasCredentialSecret(name, buddy.SecretKindPassword); ok {
			fmt.Printf("password: (secure store)\n")
		} else {
			fmt.Printf("password: (not set)\n")
		}
	case "oauth", "platform":
		fmt.Printf("clientId:     %s\n", a.ClientID)
		if a.ClientSecret != "" {
			fmt.Printf("clientSecret: %s (inline in config)\n", redact(a.ClientSecret))
			inlineWarn = storeAvailable
		} else if ok, _ := buddy.HasCredentialSecret(name, buddy.SecretKindClientSecret); ok {
			fmt.Printf("clientSecret: (secure store)\n")
		} else {
			fmt.Printf("clientSecret: (not set)\n")
		}
	}
	if inlineWarn {
		fmt.Fprintf(os.Stderr, "warning: secret is stored inline — move with: buddy cred migrate %s\n", name)
	}
	return nil
}

func redact(s string) string {
	if len(s) <= 4 {
		return strings.Repeat("*", len(s))
	}
	return s[:4] + strings.Repeat("*", len(s)-4)
}

func runCredSetTenant(cfg *buddy.Config, opts buddy.Opts, args []string) error {
	parsed, err := parseCredSetTenantArgs(args, opts.SkipConfirm)
	if err != nil {
		return err
	}
	if err := parsed.Tenant.Validate(); err != nil {
		return err
	}
	_, existing := cfg.Tenants[parsed.Name]
	if existing && !parsed.SkipConfirm {
		if !buddy.AskYesNo(fmt.Sprintf("Tenant %q already exists. Overwrite?", parsed.Name), false, nil) {
			return nil
		}
	}
	if cfg.Tenants == nil {
		cfg.Tenants = map[string]buddy.TenantConfig{}
	}
	cfg.Tenants[parsed.Name] = parsed.Tenant
	if strings.TrimSpace(cfg.Tenant.Default) == "" {
		cfg.Tenant.Default = parsed.Name
	}
	if err := buddy.SaveConfig(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	if existing {
		fmt.Printf("Updated tenant %q\n", parsed.Name)
	} else {
		fmt.Printf("Added tenant %q\n", parsed.Name)
	}
	return nil
}

func runCredSetAuth(cfg *buddy.Config, opts buddy.Opts, args []string) error {
	parsed, err := parseCredSetAuthArgs(args, opts.SkipConfirm)
	if err != nil {
		return err
	}
	if err := parsed.Auth.Validate(); err != nil {
		return err
	}
	for _, tn := range parsed.Auth.Tenants {
		if _, ok := cfg.Tenants[tn]; !ok {
			return fmt.Errorf("tenant %q not found — create it with 'buddy cred set tenant %s …' first", tn, tn)
		}
	}
	prev, existing := cfg.Auths[parsed.Name]
	if existing && !parsed.SkipConfirm {
		if !buddy.AskYesNo(fmt.Sprintf("Auth %q already exists. Overwrite?", parsed.Name), false, nil) {
			return nil
		}
	}
	if existing && !parsed.ChannelsSet {
		parsed.Auth.Channels = prev.Channels
	}
	if cfg.Auths == nil {
		cfg.Auths = map[string]buddy.AuthEntry{}
	}
	cfg.Auths[parsed.Name] = parsed.Auth
	if err := buddy.SaveConfig(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	if existing {
		fmt.Printf("Updated auth %q\n", parsed.Name)
	} else {
		fmt.Printf("Added auth %q\n", parsed.Name)
	}
	cred := buddy.ComposeCredential(cfg.Tenants[parsed.Auth.Tenants[0]], parsed.Auth, parsed.Auth.EffectiveSurface())
	return offerStoreSecret(parsed.Name, cred, parsed.SkipConfirm)
}

func offerStoreSecret(authName string, cred buddy.CredentialConfig, skipConfirm bool) error {
	kind := buddy.SecretKindClientSecret
	var label, kindName, flagHint string
	switch cred.Auth.Type {
	case "basic":
		if cred.Auth.Password != "" {
			return nil
		}
		kind = buddy.SecretKindPassword
		label = fmt.Sprintf("Basic auth password for %q", authName)
		kindName = "password"
		flagHint = "--password"
	case "oauth", "platform":
		if cred.Auth.ClientSecret != "" {
			return nil
		}
		kind = buddy.SecretKindClientSecret
		label = fmt.Sprintf("OAuth client secret for %q", authName)
		kindName = "client secret"
		flagHint = "--client-secret"
	default:
		return nil
	}
	if ok, _ := buddy.HasCredentialSecret(authName, kind); ok {
		return nil
	}
	printHint := func() {
		fmt.Fprintf(os.Stderr, "Note: no %s stored — add one with: buddy cred set secret %s %s\n", kindName, authName, flagHint)
	}
	if skipConfirm || !buddy.StdinIsTerminal() || !buddy.SecretStoreAvailable() {
		printHint()
		return nil
	}
	value, err := buddy.PromptSecret(label + " (leave blank to skip)")
	if err != nil {
		return err
	}
	if strings.TrimSpace(value) == "" {
		printHint()
		return nil
	}
	if err := buddy.SetCredentialSecret(authName, kind, value); err != nil {
		return err
	}
	fmt.Printf("Stored %s for auth %q in the secure store\n", kindName, authName)
	return nil
}

type tenantSetParsed struct {
	Name        string
	Tenant      buddy.TenantConfig
	SkipConfirm bool
}

func parseCredSetTenantArgs(args []string, skipConfirm bool) (tenantSetParsed, error) {
	var name, url, region, tenantID string
	flags := map[string]*string{
		"url":       &url,
		"region":    &region,
		"tenant-id": &tenantID,
	}
	pos := ""
	posSet := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "-y" {
			skipConfirm = true
			continue
		}
		if !strings.HasPrefix(a, "-") {
			if posSet {
				return tenantSetParsed{}, fmt.Errorf("unexpected argument %q", a)
			}
			pos, posSet = a, true
			continue
		}
		fname, val, hasVal, err := parseFlag(a)
		if err != nil {
			return tenantSetParsed{}, err
		}
		if fname == "name" {
			name = val
			if !hasVal {
				if i+1 >= len(args) {
					return tenantSetParsed{}, fmt.Errorf("--name requires a value")
				}
				i++
				name = args[i]
			}
			continue
		}
		dest, ok := flags[fname]
		if !ok {
			return tenantSetParsed{}, fmt.Errorf("unknown flag --%s", fname)
		}
		if !hasVal {
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return tenantSetParsed{}, fmt.Errorf("flag --%s requires a value", fname)
			}
			i++
			val = args[i]
		}
		*dest = val
	}
	if posSet {
		if name != "" && name != pos {
			return tenantSetParsed{}, fmt.Errorf("tenant name given twice")
		}
		name = pos
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return tenantSetParsed{}, fmt.Errorf("tenant name required; usage: buddy cred set tenant <name> [--url=…] [--region=…] [--tenant-id=…]")
	}
	region = strings.TrimSpace(region)
	if region != "" {
		norm, err := buddy.NormalizePlatformRegion(region)
		if err != nil {
			return tenantSetParsed{}, err
		}
		region = norm
	}
	return tenantSetParsed{
		Name: name,
		Tenant: buddy.TenantConfig{
			URL:      strings.TrimSpace(url),
			Region:   region,
			TenantID: strings.TrimSpace(tenantID),
		},
		SkipConfirm: skipConfirm,
	}, nil
}

type authSetParsed struct {
	Name        string
	Auth        buddy.AuthEntry
	SkipConfirm bool
	ChannelsSet bool
}

func parseCredSetAuthArgs(args []string, skipConfirm bool) (authSetParsed, error) {
	var (
		name, authType, runCommand, username, password, clientID, clientSecret string
		tenants, access, surface, channels                                     string
		channelsSet                                                            bool
	)
	stringFlags := map[string]*string{
		"name":          &name,
		"type":          &authType,
		"run":           &runCommand,
		"username":      &username,
		"password":      &password,
		"client-id":     &clientID,
		"client-secret": &clientSecret,
		"tenants":       &tenants,
		"access":        &access,
		"surface":       &surface,
		"channels":      &channels,
	}
	pos := ""
	posSet := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "-y" {
			skipConfirm = true
			continue
		}
		if !strings.HasPrefix(a, "-") {
			if posSet {
				return authSetParsed{}, fmt.Errorf("unexpected argument %q", a)
			}
			pos, posSet = a, true
			continue
		}
		fname, val, hasVal, err := parseFlag(a)
		if err != nil {
			return authSetParsed{}, err
		}
		dest, ok := stringFlags[fname]
		if !ok {
			return authSetParsed{}, fmt.Errorf("unknown flag --%s", fname)
		}
		if !hasVal {
			// Allow --channels= / --channels with empty next → all
			if fname == "channels" {
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
					i++
					val = args[i]
				} else {
					val = "all"
				}
				channelsSet = true
				*dest = val
				continue
			}
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return authSetParsed{}, fmt.Errorf("flag --%s requires a value", fname)
			}
			i++
			val = args[i]
		}
		if fname == "channels" {
			channelsSet = true
		}
		*dest = val
	}
	if posSet {
		if name != "" && name != pos {
			return authSetParsed{}, fmt.Errorf("auth name given twice")
		}
		name = pos
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return authSetParsed{}, fmt.Errorf("auth name required; usage: buddy cred set auth <name> --type=… --tenants=…")
	}
	if authType == "" {
		return authSetParsed{}, fmt.Errorf("--type is required (command, basic, oauth, or platform)")
	}
	if strings.TrimSpace(tenants) == "" {
		return authSetParsed{}, fmt.Errorf("--tenants is required (comma-separated tenant names)")
	}
	tenantList := splitCSV(tenants)
	authType = strings.ToLower(strings.TrimSpace(authType))
	surf := strings.ToLower(strings.TrimSpace(surface))
	if surf != "" {
		if surf == buddy.SurfacePlatform && (authType == "" || authType == buddy.AuthTypeOAuth || authType == buddy.AuthTypePlatform) {
			authType = buddy.AuthTypePlatform
		} else if surf == buddy.SurfaceInstance {
			// ignored — instance is type oauth/basic/command
		} else {
			return authSetParsed{}, fmt.Errorf("auth surface was folded into type; use --type=platform (not --surface=%s)", surface)
		}
	}
	entry := buddy.AuthEntry{
		Type:    authType,
		Tenants: tenantList,
		Access:  strings.TrimSpace(access),
	}
	if channelsSet {
		ch, err := buddy.ParseAuthChannels(channels)
		if err != nil {
			return authSetParsed{}, err
		}
		entry.Channels = ch
	}
	switch authType {
	case "command":
		if runCommand == "" {
			return authSetParsed{}, fmt.Errorf("--run is required for type command")
		}
		entry.RunCommand = runCommand
	case "basic":
		if username == "" {
			return authSetParsed{}, fmt.Errorf("--username is required for type basic")
		}
		entry.Username = username
		entry.Password = password
	case "oauth", "platform":
		if clientID == "" {
			return authSetParsed{}, fmt.Errorf("--client-id is required for type %s", authType)
		}
		entry.ClientID = clientID
		entry.ClientSecret = clientSecret
	default:
		return authSetParsed{}, fmt.Errorf("unknown auth type %q", authType)
	}
	return authSetParsed{Name: name, Auth: entry, SkipConfirm: skipConfirm, ChannelsSet: channelsSet}, nil
}

func parseFlag(a string) (name, val string, hasVal bool, err error) {
	if !strings.HasPrefix(a, "--") {
		return "", "", false, fmt.Errorf("unknown flag %q", a)
	}
	name = strings.TrimPrefix(a, "--")
	if j := strings.IndexByte(name, '='); j >= 0 {
		return name[:j], name[j+1:], true, nil
	}
	return name, "", false, nil
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func runCredDeleteTenant(cfg *buddy.Config, opts buddy.Opts, name string) error {
	if _, ok := cfg.Tenants[name]; !ok {
		return fmt.Errorf("tenant %q not found", name)
	}
	var deleteAuths []string
	var unlinkAuths []string
	for an, a := range cfg.Auths {
		if !a.LinkedTo(name) {
			continue
		}
		if len(a.Tenants) == 1 {
			deleteAuths = append(deleteAuths, an)
		} else {
			unlinkAuths = append(unlinkAuths, an)
		}
	}
	sort.Strings(deleteAuths)
	sort.Strings(unlinkAuths)

	if !opts.SkipConfirm {
		fmt.Printf("Delete tenant %q?\n", name)
		if len(deleteAuths) > 0 {
			fmt.Printf("  Will delete auth(s) only linked here (and their store secrets): %s\n", strings.Join(deleteAuths, ", "))
		}
		if len(unlinkAuths) > 0 {
			fmt.Printf("  Will unlink auth(s) that also cover other tenants: %s\n", strings.Join(unlinkAuths, ", "))
		}
		if !buddy.AskYesNo("Proceed?", false, nil) {
			return nil
		}
	}

	for _, an := range deleteAuths {
		if err := deleteAuthRecord(cfg, an); err != nil {
			return err
		}
		fmt.Printf("Deleted auth %q (and related secrets)\n", an)
	}
	for _, an := range unlinkAuths {
		a := cfg.Auths[an]
		var kept []string
		for _, t := range a.Tenants {
			if t != name {
				kept = append(kept, t)
			}
		}
		a.Tenants = kept
		cfg.Auths[an] = a
		fmt.Printf("Unlinked auth %q from tenant %q\n", an, name)
	}

	delete(cfg.Tenants, name)
	cleared := false
	if cfg.Tenant.Default == name {
		cfg.Tenant.Default = buddy.DefaultTenant
		cleared = true
	}
	for ch, p := range cfg.Channels {
		changed := false
		if p.Tenant == name {
			p.Tenant = ""
			changed = true
		}
		if len(p.Tenants) > 0 {
			var kept []string
			for _, t := range p.Tenants {
				if t != name {
					kept = append(kept, t)
				}
			}
			if len(kept) != len(p.Tenants) {
				p.Tenants = kept
				changed = true
			}
		}
		if changed {
			cfg.Channels[ch] = p
		}
	}
	if cleared {
		fmt.Printf("Note: tenant.default was %q; reset to %q\n", name, buddy.DefaultTenant)
	}
	if err := buddy.SaveConfig(cfg); err != nil {
		return err
	}
	fmt.Printf("Deleted tenant %q\n", name)
	if cfg.TenantFromManagedPrefs(name) {
		fmt.Fprintf(os.Stderr, "warning: tenant %q is still provided by managed preferences; it will return on next load unless you override it in config.yaml or remove it from the MDM profile\n", name)
	}
	return nil
}

func runCredDeleteAuth(cfg *buddy.Config, opts buddy.Opts, name string) error {
	if _, ok := cfg.Auths[name]; !ok {
		return fmt.Errorf("auth %q not found", name)
	}
	if !opts.SkipConfirm {
		if !buddy.AskYesNo(fmt.Sprintf("Delete auth %q and related secrets from the secure store?", name), false, nil) {
			return nil
		}
	}
	if err := deleteAuthRecord(cfg, name); err != nil {
		return err
	}
	if err := buddy.SaveConfig(cfg); err != nil {
		return err
	}
	fmt.Printf("Deleted auth %q (and related secrets)\n", name)
	if cfg.AuthFromManagedPrefs(name) {
		fmt.Fprintf(os.Stderr, "warning: auth %q is still provided by managed preferences; it will return on next load unless you override it in config.yaml or remove it from the MDM profile\n", name)
	}
	return nil
}

// deleteAuthRecord removes an auth from cfg and deletes password + client-secret from the store.
func deleteAuthRecord(cfg *buddy.Config, name string) error {
	delete(cfg.Auths, name)
	if err := buddy.DeleteCredentialSecrets(name, true, true); err != nil {
		// Store may be unavailable; still remove the auth record.
		fmt.Fprintf(os.Stderr, "warning: could not delete secrets for %q: %v\n", name, err)
	}
	return nil
}

func secretSelectionForAuthType(authType, authName string) (wantPassword, wantClientSecret bool, err error) {
	switch authType {
	case "basic":
		return true, false, nil
	case "oauth", "platform":
		return false, true, nil
	case "command":
		return false, false, fmt.Errorf("auth %q uses command auth and has no stored secret", authName)
	default:
		return false, false, fmt.Errorf("auth %q has no type; set one with 'buddy cred set auth %s …' first", authName, authName)
	}
}

func runCredSetSecret(cfg *buddy.Config, authName string, args []string) error {
	auth, err := cfg.ResolveAuth(authName)
	if err != nil {
		return err
	}
	var password, clientSecret string
	wantPassword := false
	wantClientSecret := false
	secretFromStdin := false
	for _, a := range args {
		switch {
		case a == "--secret-from-stdin":
			secretFromStdin = true
		case strings.HasPrefix(a, "--password="):
			wantPassword = true
			password = strings.TrimPrefix(a, "--password=")
		case a == "--password":
			wantPassword = true
		case strings.HasPrefix(a, "--client-secret="):
			wantClientSecret = true
			clientSecret = strings.TrimPrefix(a, "--client-secret=")
		case a == "--client-secret":
			wantClientSecret = true
		default:
			return fmt.Errorf("unknown flag %q; run '%s'", a, clispec.UsageHelp(clispec.CmdCred))
		}
	}
	if !wantPassword && !wantClientSecret {
		wantPassword, wantClientSecret, err = secretSelectionForAuthType(auth.Type, authName)
		if err != nil {
			return err
		}
	}
	if secretFromStdin {
		val, err := buddy.ReadSecretFromStdin()
		if err != nil {
			return err
		}
		if wantPassword && password == "" {
			password = val
		}
		if wantClientSecret && clientSecret == "" {
			clientSecret = val
		}
	}
	if wantPassword && password == "" {
		password, err = buddy.PromptSecret(fmt.Sprintf("Basic auth password for %q", authName))
		if err != nil {
			return err
		}
	}
	if wantClientSecret && clientSecret == "" {
		clientSecret, err = buddy.PromptSecret(fmt.Sprintf("OAuth client secret for %q", authName))
		if err != nil {
			return err
		}
	}
	if err := buddy.SetCredentialSecrets(authName, password, clientSecret); err != nil {
		return err
	}
	fmt.Printf("Stored secret(s) for auth %q\n", authName)
	return nil
}

func runCredDeleteSecret(authName string, args []string) error {
	deletePassword := false
	deleteClientSecret := false
	for _, a := range args {
		switch a {
		case "--password":
			deletePassword = true
		case "--client-secret":
			deleteClientSecret = true
		default:
			return fmt.Errorf("unknown flag %q", a)
		}
	}
	if !deletePassword && !deleteClientSecret {
		deletePassword = true
		deleteClientSecret = true
	}
	if err := buddy.DeleteCredentialSecrets(authName, deletePassword, deleteClientSecret); err != nil {
		return err
	}
	fmt.Printf("Deleted secret(s) for auth %q from the secure store\n", authName)
	return nil
}

func runCredBackup(cfg *buddy.Config, opts buddy.Opts, args []string) error {
	path, passphrase, err := parseCredBackupArgs("backup", args)
	if err != nil {
		return err
	}
	res, err := buddy.BackupCreds(cfg, buddy.CredBackupOpts{
		Path:        path,
		Passphrase:  passphrase,
		SkipConfirm: opts.SkipConfirm,
	})
	if err != nil {
		return err
	}
	if res == nil {
		return nil
	}
	fmt.Printf("Wrote encrypted backup %s (%d tenant(s), %d auth(s))\n", res.Path, res.Tenants, res.Auths)
	for _, m := range res.MissingSecrets {
		fmt.Fprintf(os.Stderr, "warning: missing %s (not in config or store)\n", m)
	}
	return nil
}

func runCredRestore(cfg *buddy.Config, opts buddy.Opts, args []string) error {
	path, passphrase, err := parseCredBackupArgs("restore", args)
	if err != nil {
		return err
	}
	res, err := buddy.RestoreCreds(cfg, buddy.CredBackupOpts{
		Path:        path,
		Passphrase:  passphrase,
		SkipConfirm: opts.SkipConfirm,
	})
	if err != nil {
		return err
	}
	if res == nil {
		return nil
	}
	fmt.Printf("Restored %s (%d tenant(s), %d auth(s), %d secret(s) in store)\n",
		res.Path, res.Tenants, res.Auths, res.Stored)
	if res.Skipped > 0 {
		fmt.Printf("Skipped %d auth(s)\n", res.Skipped)
	}
	return nil
}

func parseCredBackupArgs(verb string, args []string) (path, passphrase string, err error) {
	var positional []string
	for _, a := range args {
		switch {
		case a == clispec.SubHelp:
			return "", "", fmt.Errorf("usage: buddy cred %s <file> [--passphrase=] [-y]", verb)
		case strings.HasPrefix(a, "--passphrase="):
			passphrase = strings.TrimPrefix(a, "--passphrase=")
		case a == clispec.Short(clispec.ShortSkipConfirm):
			// global -y is already in opts; ignore if repeated
		case strings.HasPrefix(a, "-"):
			return "", "", fmt.Errorf("unknown flag %q; usage: buddy cred %s <file> [--passphrase=] [-y]", a, verb)
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) != 1 {
		return "", "", fmt.Errorf("usage: buddy cred %s <file> [--passphrase=] [-y]", verb)
	}
	return positional[0], passphrase, nil
}

func runCredStore() error {
	kind, detail, err := buddy.SecretStoreInfo()
	if err != nil {
		return err
	}
	fmt.Printf("store: %s\n", kind)
	fmt.Printf("path:  %s\n", detail)
	return nil
}

func runCredClean(cfg *buddy.Config, opts buddy.Opts, args []string) error {
	if len(args) == 0 || args[0] == clispec.SubHelp {
		fmt.Print(`buddy cred clean - Review and remove orphaned secrets or stale tokens

Usage:
  buddy [global-flags] cred clean secrets [-y]
  buddy [global-flags] cred clean tokens

Subcommands:
  secrets   List secret-store entries whose auth name is not in config; prompt to delete each
  tokens    Delete expired or corrupt files under ~/.config/buddy/tokens/

Notes:
  secrets: compares the platform secret store to auths: in config.yaml. Orphans are
  usually left behind after deleting an auth without clearing the store (older buddy)
  or renaming an auth.
  tokens: removes bearer cache files that are expired, corrupt, undecodable, or
  within the skew window. Still-valid tokens are kept. No prompt (safe anytime).

Examples:
  buddy cred clean secrets
  buddy cred clean secrets -y
  buddy cred clean tokens
`)
		return nil
	}
	switch args[0] {
	case "secrets":
		for _, a := range args[1:] {
			if a == "-y" {
				opts.SkipConfirm = true
			}
		}
		return runCredCleanSecrets(cfg, opts)
	case "tokens":
		return runCredCleanTokens()
	default:
		return errUnknownSubcommand("cred clean", args[0], []string{"secrets", "tokens"})
	}
}

func runCredCleanTokens() error {
	res, err := buddy.CleanExpiredTokenCache()
	if err != nil {
		return fmt.Errorf("clean token cache: %w", err)
	}
	if res.Dir != "" {
		fmt.Printf("Token cache: %s\n", res.Dir)
	}
	fmt.Printf("Deleted %d stale file(s); kept %d still-valid.\n", res.Deleted, res.Kept)
	return nil
}

func runCredErase(opts buddy.Opts, args []string) error {
	if len(args) == 0 || args[0] == clispec.SubHelp {
		fmt.Print(`buddy cred erase - Wipe every secret from the platform store

Usage:
  buddy [global-flags] cred erase secrets [-y]

Subcommands:
  secrets   Delete all secret-store entries, including those still linked to auths

Notes:
  Unlike 'cred clean secrets' (orphans only), this removes every password and
  client-secret buddy stored. Auths, tenants, and channels in config.yaml are
  not changed. Re-store with 'buddy cred set secret' or 'buddy setup secrets'.
  On macOS, both Keychain services are cleared (CLI "buddy" and signed
  "virtualpete.buddy.cli") so leftovers from the other binary are not left behind.
  -y skips the confirmation prompt.

Examples:
  buddy cred erase secrets
  buddy cred erase secrets -y
`)
		return nil
	}
	switch args[0] {
	case "secrets":
		for _, a := range args[1:] {
			if a == "-y" {
				opts.SkipConfirm = true
			}
		}
		return runCredEraseSecrets(opts)
	default:
		return errUnknownSubcommand("cred erase", args[0], []string{"secrets"})
	}
}

func runCredEraseSecrets(opts buddy.Opts) error {
	if !buddy.SecretStoreAvailable() {
		return fmt.Errorf("no secure secret store available (see 'buddy cred store')")
	}
	kind, detail, err := buddy.SecretStoreInfo()
	if err != nil {
		return err
	}
	entries, err := buddy.ListCredentialSecrets()
	if err != nil {
		return fmt.Errorf("list secrets: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].AuthName != entries[j].AuthName {
			return entries[i].AuthName < entries[j].AuthName
		}
		return entries[i].Kind.String() < entries[j].Kind.String()
	})
	kcGroups, err := buddy.ListBuddyKeychainSecrets()
	if err != nil {
		return fmt.Errorf("list keychain: %w", err)
	}
	activeKC := buddy.ActiveKeychainService()
	var extraKC []buddy.KeychainSecretGroup
	for _, g := range kcGroups {
		if kind == "keychain" && g.Service == activeKC {
			continue
		}
		extraKC = append(extraKC, g)
	}

	if len(entries) == 0 && len(extraKC) == 0 {
		fmt.Printf("No secrets in the store (%s).\n", detail)
		return nil
	}

	fmt.Printf("This will delete ALL secrets from the store (%s: %s).\n", kind, detail)
	fmt.Println("Auths in config.yaml are kept; only stored passwords / client-secrets are removed.")
	if len(entries) > 0 {
		fmt.Println()
		fmt.Println("Active store:")
		for _, e := range entries {
			fmt.Printf("  %-24s  %s\n", e.AuthName, e.Kind.String())
		}
	}
	for _, g := range extraKC {
		fmt.Println()
		fmt.Printf("Keychain service %q:\n", g.Service)
		ents := g.Entries
		sort.Slice(ents, func(i, j int) bool {
			if ents[i].AuthName != ents[j].AuthName {
				return ents[i].AuthName < ents[j].AuthName
			}
			return ents[i].Kind.String() < ents[j].Kind.String()
		})
		for _, e := range ents {
			fmt.Printf("  %-24s  %s\n", e.AuthName, e.Kind.String())
		}
	}
	fmt.Println()
	if !opts.SkipConfirm {
		if !buddy.ConfirmPrompt("Delete ALL of these secrets?", nil) {
			return nil
		}
	}
	storeN, kcN, err := buddy.EraseCredentialSecrets()
	if err != nil {
		return fmt.Errorf("erase secrets: %w", err)
	}
	fmt.Printf("Deleted %d secret(s) from the store.\n", storeN)
	if kcN > 0 {
		fmt.Printf("Deleted %d Keychain item(s) across buddy services.\n", kcN)
	}
	return nil
}

func runCredCleanSecrets(cfg *buddy.Config, opts buddy.Opts) error {
	if !buddy.SecretStoreAvailable() {
		return fmt.Errorf("no secure secret store available (see 'buddy cred store')")
	}
	entries, err := buddy.ListCredentialSecrets()
	if err != nil {
		return fmt.Errorf("list secrets: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].AuthName != entries[j].AuthName {
			return entries[i].AuthName < entries[j].AuthName
		}
		return entries[i].Kind.String() < entries[j].Kind.String()
	})

	var linked []secretsEntryView
	for _, e := range entries {
		if _, ok := cfg.Auths[e.AuthName]; ok {
			linked = append(linked, secretsEntryView{AuthName: e.AuthName, Kind: e.Kind.String()})
		}
	}
	orphans, err := buddy.OrphanCredentialSecrets(cfg)
	if err != nil {
		return err
	}
	sort.Slice(orphans, func(i, j int) bool {
		if orphans[i].AuthName != orphans[j].AuthName {
			return orphans[i].AuthName < orphans[j].AuthName
		}
		return orphans[i].Kind.String() < orphans[j].Kind.String()
	})

	fmt.Printf("Secret store review (%d entries)\n", len(entries))
	if len(linked) > 0 {
		fmt.Println()
		fmt.Println("Linked to an auth in config:")
		for _, e := range linked {
			fmt.Printf("  %-24s  %s\n", e.AuthName, e.Kind)
		}
	}
	if len(orphans) == 0 {
		fmt.Println()
		fmt.Println("No orphaned secrets.")
		return nil
	}
	fmt.Println()
	fmt.Println("Orphaned (no matching auth in config):")
	for _, e := range orphans {
		fmt.Printf("  %-24s  %s\n", e.AuthName, e.Kind.String())
	}
	fmt.Println()

	deleted := 0
	for _, e := range orphans {
		kindName := e.Kind.String()
		if !opts.SkipConfirm {
			if !buddy.AskYesNo(fmt.Sprintf("Delete orphaned %s for %q?", kindName, e.AuthName), false, nil) {
				continue
			}
		}
		delPW := e.Kind == buddy.SecretKindPassword
		delCS := e.Kind == buddy.SecretKindClientSecret
		if err := buddy.DeleteCredentialSecrets(e.AuthName, delPW, delCS); err != nil {
			return fmt.Errorf("delete %s for %q: %w", kindName, e.AuthName, err)
		}
		fmt.Printf("  deleted %s for %q\n", kindName, e.AuthName)
		deleted++
	}
	fmt.Printf("\nRemoved %d orphaned secret(s).\n", deleted)
	return nil
}

type secretsEntryView struct {
	AuthName string
	Kind     string
}

func runCredMigrate(cfg *buddy.Config, opts buddy.Opts, args []string) error {
	if !buddy.SecretStoreAvailable() {
		return fmt.Errorf("no secure secret store available; nothing to migrate (see 'buddy cred store')")
	}
	var names []string
	if len(args) > 0 {
		if _, err := cfg.ResolveAuth(args[0]); err != nil {
			return err
		}
		names = append(names, args[0])
	} else {
		for name := range cfg.Auths {
			names = append(names, name)
		}
		sort.Strings(names)
	}

	log := buddy.GetLogger()
	found := 0
	migrated := 0
	for _, name := range names {
		auth := cfg.Auths[name]
		changed := false
		migrate := func(isPassword bool, kindName, value string) error {
			found++
			kind := buddy.SecretKindClientSecret
			if isPassword {
				kind = buddy.SecretKindPassword
			}
			prompt := fmt.Sprintf("Move %s for auth %q from config.yaml into the secure store?", kindName, name)
			if !opts.SkipConfirm && !buddy.ConfirmPrompt(prompt, log) {
				return nil
			}
			if err := buddy.SetCredentialSecret(name, kind, value); err != nil {
				return fmt.Errorf("store %s for %q: %w", kindName, name, err)
			}
			if isPassword {
				auth.Password = ""
			} else {
				auth.ClientSecret = ""
			}
			changed = true
			migrated++
			fmt.Printf("Migrated %s for %q to the secure store\n", kindName, name)
			return nil
		}
		if auth.Password != "" {
			if err := migrate(true, "password", auth.Password); err != nil {
				return err
			}
		}
		if auth.ClientSecret != "" {
			if err := migrate(false, "client secret", auth.ClientSecret); err != nil {
				return err
			}
		}
		if changed {
			cfg.Auths[name] = auth
		}
	}
	if found == 0 {
		fmt.Println("No inline secrets found in config.yaml.")
		return nil
	}
	if migrated == 0 {
		fmt.Println("No secrets migrated.")
		return nil
	}
	if err := buddy.SaveConfig(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	fmt.Printf("Migrated %d secret(s); config.yaml updated.\n", migrated)
	return nil
}
