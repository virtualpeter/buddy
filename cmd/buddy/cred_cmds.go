package main

import (
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"

	"buddy/internal/buddy"
)

func runCred(cfg *buddy.Config, opts buddy.Opts, args []string, _ *slog.Logger) error {
	if len(args) == 0 || args[0] == "help" {
		fmt.Print(helpCred)
		return nil
	}
	switch args[0] {
	case "list":
		return runCredList(cfg)
	case "get":
		return runCredGet(cfg, args[1:])
	case "set":
		return runCredSet(cfg, opts, args[1:])
	case "delete":
		return runCredDelete(cfg, opts, args[1:])
	case "use":
		return runCredUse(cfg, args[1:])
	case "allow":
		return fmt.Errorf("'cred allow' was removed; set auth channels with 'buddy cred set auth <name> --channels=cli,mcp,serve' (or --channels=all)")
	case "store":
		return runCredStore()
	case "migrate":
		return runCredMigrate(cfg, opts, args[1:])
	default:
		return fmt.Errorf("unknown cred subcommand: %q; run 'buddy cred help' for usage", args[0])
	}
}

func runCredGet(cfg *buddy.Config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing item; usage: buddy cred get tenant|auth|secret <name> (see 'buddy cred help')")
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
		return fmt.Errorf("unknown %q (expected tenant|auth|secret <name>); run 'buddy cred list'", args[0])
	}
}

func runCredSet(cfg *buddy.Config, opts buddy.Opts, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing item; usage: buddy cred set tenant|auth|secret <name> … (see 'buddy cred help')")
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
	default:
		return fmt.Errorf("unknown set target %q; use tenant, auth, or secret (see 'buddy cred help')", args[0])
	}
}

func runCredDelete(cfg *buddy.Config, opts buddy.Opts, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing item; usage: buddy cred delete tenant|auth|secret <name> (see 'buddy cred help')")
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
		return fmt.Errorf("unknown delete target %q; use tenant, auth, or secret", args[0])
	}
}

func runCredUse(cfg *buddy.Config, args []string) error {
	surface, rest, err := parseCredSurfaceFlags(args, true)
	if err != nil {
		return err
	}
	key := "default"
	if surface != "" {
		key = surface
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
		return fmt.Errorf("usage: buddy cred use [tenant] or buddy cred use --cli|--mcp|--ide|--serve [tenant]")
	}
	name := rest[0]
	if _, ok := cfg.Tenants[name]; !ok {
		return fmt.Errorf("tenant %q not found; run 'buddy cred list'", name)
	}
	if err := cfg.Tenant.SetChannel(key, name); err != nil {
		return err
	}
	if err := buddy.SaveConfig(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	fmt.Printf("tenant.%s set to %q\n", key, name)
	return nil
}

// parseCredSurfaceFlags extracts --cli|--mcp|--ide|--serve.
func parseCredSurfaceFlags(args []string, allowIDE bool) (surface string, rest []string, err error) {
	for _, a := range args {
		switch a {
		case "--cli":
			if surface != "" {
				return "", nil, fmt.Errorf("only one of --cli|--mcp|--ide|--serve allowed")
			}
			surface = buddy.ChannelCLI
		case "--mcp":
			if surface != "" {
				return "", nil, fmt.Errorf("only one of --cli|--mcp|--ide|--serve allowed")
			}
			surface = buddy.ChannelMCP
		case "--serve":
			if surface != "" {
				return "", nil, fmt.Errorf("only one of --cli|--mcp|--ide|--serve allowed")
			}
			surface = buddy.ChannelServe
		case "--ide":
			if !allowIDE {
				return "", nil, fmt.Errorf("--ide is only valid for 'buddy cred use'")
			}
			if surface != "" {
				return "", nil, fmt.Errorf("only one of --cli|--mcp|--ide|--serve allowed")
			}
			surface = buddy.ChannelIDE
		default:
			rest = append(rest, a)
		}
	}
	return surface, rest, nil
}

func runCredList(cfg *buddy.Config) error {
	if len(cfg.Tenants) == 0 && len(cfg.Auths) == 0 {
		fmt.Println("No tenants/auths configured. Use 'buddy cred set tenant|auth …' (see 'buddy cred help').")
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
	fmt.Printf("%-18s  %-8s  %-10s  %-10s  %-14s  %s\n", "NAME", "TYPE", "ACCESS", "SURFACE", "CHANNELS", "TENANTS")
	fmt.Printf("%-18s  %-8s  %-10s  %-10s  %-14s  %s\n", "------------------", "--------", "----------", "----------", "--------------", "-------")
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
		fmt.Printf("%-18s  %-8s  %-10s  %-10s  %-14s  %s\n", name, a.Type, a.EffectiveAccess(), a.EffectiveSurface(), ch, strings.Join(a.Tenants, ","))
	}
	sel := cfg.Tenant
	if sel.CLI != "" || sel.MCP != "" || sel.Serve != "" || sel.IDE != "" {
		fmt.Printf("\nTENANT SELECTION\n")
		fmt.Printf("  default  %s\n", sel.Default)
		if sel.CLI != "" {
			fmt.Printf("  cli      %s\n", sel.CLI)
		}
		if sel.MCP != "" {
			fmt.Printf("  mcp      %s\n", sel.MCP)
		}
		if sel.Serve != "" {
			fmt.Printf("  serve    %s\n", sel.Serve)
		}
		if sel.IDE != "" {
			fmt.Printf("  ide      %s\n", sel.IDE)
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
	fmt.Printf("surface:  %s\n", a.EffectiveSurface())
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
	case "oauth":
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
		fmt.Printf("Tenant %q already exists. Overwrite? [y/N] ", parsed.Name)
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(strings.TrimSpace(response)) != "y" {
			fmt.Println("Aborted.")
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
		fmt.Printf("Auth %q already exists. Overwrite? [y/N] ", parsed.Name)
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(strings.TrimSpace(response)) != "y" {
			fmt.Println("Aborted.")
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
	case "oauth":
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
		return authSetParsed{}, fmt.Errorf("--type is required (command, basic, or oauth)")
	}
	if strings.TrimSpace(tenants) == "" {
		return authSetParsed{}, fmt.Errorf("--tenants is required (comma-separated tenant names)")
	}
	tenantList := splitCSV(tenants)
	entry := buddy.AuthEntry{
		Type:    authType,
		Tenants: tenantList,
		Access:  strings.TrimSpace(access),
		Surface: strings.TrimSpace(surface),
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
	case "oauth":
		if clientID == "" {
			return authSetParsed{}, fmt.Errorf("--client-id is required for type oauth")
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
	for an, a := range cfg.Auths {
		if a.LinkedTo(name) {
			return fmt.Errorf("tenant %q is still linked from auth %q — unlink/delete that auth first", name, an)
		}
	}
	if !opts.SkipConfirm {
		fmt.Printf("Delete tenant %q? [y/N] ", name)
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(strings.TrimSpace(response)) != "y" {
			fmt.Println("Aborted.")
			return nil
		}
	}
	delete(cfg.Tenants, name)
	cleared := false
	if cfg.Tenant.Default == name {
		cfg.Tenant.Default = buddy.DefaultTenant
		cleared = true
	}
	if cfg.Tenant.CLI == name {
		cfg.Tenant.CLI = ""
	}
	if cfg.Tenant.MCP == name {
		cfg.Tenant.MCP = ""
	}
	if cfg.Tenant.Serve == name {
		cfg.Tenant.Serve = ""
	}
	if cfg.Tenant.IDE == name {
		cfg.Tenant.IDE = ""
	}
	if cleared {
		fmt.Printf("Note: tenant.default was %q; reset to %q\n", name, buddy.DefaultTenant)
	}
	if err := buddy.SaveConfig(cfg); err != nil {
		return err
	}
	fmt.Printf("Deleted tenant %q\n", name)
	return nil
}

func runCredDeleteAuth(cfg *buddy.Config, opts buddy.Opts, name string) error {
	if _, ok := cfg.Auths[name]; !ok {
		return fmt.Errorf("auth %q not found", name)
	}
	if !opts.SkipConfirm {
		fmt.Printf("Delete auth %q? [y/N] ", name)
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(strings.TrimSpace(response)) != "y" {
			fmt.Println("Aborted.")
			return nil
		}
	}
	delete(cfg.Auths, name)
	if err := buddy.SaveConfig(cfg); err != nil {
		return err
	}
	fmt.Printf("Deleted auth %q\n", name)
	return nil
}

func secretSelectionForAuthType(authType, authName string) (wantPassword, wantClientSecret bool, err error) {
	switch authType {
	case "basic":
		return true, false, nil
	case "oauth":
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
			return fmt.Errorf("unknown flag %q; run 'buddy cred help'", a)
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

func runCredStore() error {
	kind, detail, err := buddy.SecretStoreInfo()
	if err != nil {
		return err
	}
	fmt.Printf("store: %s\n", kind)
	fmt.Printf("path:  %s\n", detail)
	return nil
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
