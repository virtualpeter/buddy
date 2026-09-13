package buddy

import (
	"fmt"
	"log/slog"
	"strings"
)

// CredAddOpts controls buddy cred add. Empty fields are prompted when stdin is a TTY.
type CredAddOpts struct {
	AuthName        string
	TenantName      string
	URL             string
	Region          string
	EnvironmentID   string
	TenantID        string
	Surface         string
	Type            string
	Scope           string // platform: organization | environment | tenant
	ClientID        string
	Username        string
	Password        string
	ClientSecret    string
	WantPaste       bool   // --paste: read JSON/curl (prompt or stdin)
	PasteBlob       string // already-captured blob (--paste=<json-or-curl>)
	FromFile        string // --from=: secrets-free YAML or ACS plist
	OrgName         string // parent organization workplace for nested orgs:
	EnvironmentName string // environment workplace when context is a product tenant
	Kind            string // instance | platform | "" (chooser / flags)
	SkipConfirm     bool
	Test            bool
	NoTest          bool
}

// CredAddResult summarizes a successful cred add.
type CredAddResult struct {
	TenantName    string
	AuthName      string
	CreatedTenant bool
	Tested        bool
	Live          string
}

// CredAdd creates or updates a tenant + auth + store secret in one flow.
func CredAdd(cfg *Config, opts CredAddOpts, log *slog.Logger) (*CredAddResult, error) {
	if cfg == nil {
		return nil, fmt.Errorf("internal error: config is nil")
	}
	if path := strings.TrimSpace(opts.FromFile); path != "" {
		if err := ImportQuickstart(cfg, path); err != nil {
			return nil, err
		}
		sec, err := SetupSecrets(cfg, SetupSecretsOpts{SkipConfirm: opts.SkipConfirm})
		out := &CredAddResult{}
		if d := strings.TrimSpace(cfg.Tenant.Default); d != "" {
			out.TenantName = d
		}
		if sec != nil && len(sec.CreatedAuths) > 0 {
			out.AuthName = sec.CreatedAuths[0]
		}
		return out, err
	}
	if err := resolveCredAddPaste(&opts); err != nil {
		return nil, err
	}
	interactive := StdinIsTerminal() && !opts.SkipConfirm
	res := &CredAddResult{}

	tenantName := strings.TrimSpace(opts.TenantName)
	if tenantName == "" && interactive {
		line, err := PromptSecretLine("Tenant name (buddy config name)")
		if err != nil {
			return nil, err
		}
		tenantName = strings.TrimSpace(line)
	}
	if tenantName == "" {
		return nil, fmt.Errorf("context name required — pass --context=<name> (or --tenant=; or run without -y in a terminal)")
	}
	res.TenantName = tenantName

	tenant, tenantExists := cfg.Tenants[tenantName]
	if !tenantExists {
		url := strings.TrimSpace(opts.URL)
		region := strings.TrimSpace(opts.Region)
		eid := strings.TrimSpace(opts.EnvironmentID)
		tid := strings.TrimSpace(opts.TenantID)
		orgScope := IsOrganizationScope(opts.Scope) || LooksLikeOrgPlatformGateway(url)
		instanceOnly := strings.EqualFold(strings.TrimSpace(opts.Kind), "instance")
		if interactive {
			platformRegional := NormalizeAuthType(opts.Type) == AuthTypePlatform && region != ""
			if url == "" && !orgScope && !platformRegional {
				line, err := PromptSecretLine("Jamf Pro instance URL (blank if platform-only)")
				if err != nil {
					return nil, err
				}
				url = strings.TrimSpace(line)
			}
			if !instanceOnly && region == "" && !orgScope {
				line, err := PromptSecretLine("Platform region (us|eu|apac, blank to skip)")
				if err != nil {
					return nil, err
				}
				region = strings.TrimSpace(line)
			}
			if !instanceOnly && region != "" && eid == "" && tid == "" && !orgScope {
				line, err := PromptSecretLine("Platform environment id (Account environment UUID; blank to skip)")
				if err != nil {
					return nil, err
				}
				eid = strings.TrimSpace(line)
				line, err = PromptSecretLine("Jamf Pro product tenant id (optional; blank to skip)")
				if err != nil {
					return nil, err
				}
				tid = strings.TrimSpace(line)
			}
		}
		if region != "" {
			norm, err := NormalizePlatformRegion(region)
			if err != nil {
				return nil, err
			}
			region = norm
		}
		if newContextIsOrgWorkplace(url, region, eid, tid, opts.Scope, opts.Type) {
			url, region = "", ""
			opts.Scope = PlatformScopeOrganization
			if strings.TrimSpace(opts.Type) == "" {
				opts.Type = AuthTypePlatform
			}
		}
		tenant = TenantConfig{URL: url, Region: region, EnvironmentID: eid, TenantID: tid}
		if err := tenant.Validate(); err != nil {
			return nil, fmt.Errorf("tenant: %w", err)
		}
		if cfg.Tenants == nil {
			cfg.Tenants = map[string]TenantConfig{}
		}
		cfg.Tenants[tenantName] = tenant
		res.CreatedTenant = true
		if err := attachCredAddOrgs(cfg, tenantName, tenant, &opts, interactive); err != nil {
			return nil, err
		}
	}

	typ := NormalizeAuthType(opts.Type)
	if surf := strings.ToLower(strings.TrimSpace(opts.Surface)); surf != "" {
		if surf != SurfaceInstance && surf != SurfacePlatform {
			return nil, fmt.Errorf("surface was folded into type; use --type=oauth|basic|platform (got --surface=%q)", opts.Surface)
		}
		if surf == SurfacePlatform {
			if typ != "" && typ != AuthTypeOAuth && typ != AuthTypePlatform {
				return nil, fmt.Errorf("surface was folded into type; platform oauth is --type=platform")
			}
			typ = AuthTypePlatform
		}
	}
	if typ == "" && interactive {
		def := AuthTypeOAuth
		hint := "oauth|basic"
		if tenant.HasPlatform() && !tenant.HasInstanceURL() {
			def = AuthTypePlatform
			hint = "platform"
		} else if tenant.HasPlatform() {
			hint = "oauth|basic|platform"
		}
		line, err := PromptSecretLine(fmt.Sprintf("Auth type %s [%s]", hint, def))
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(line) != "" {
			typ = NormalizeAuthType(line)
		} else {
			typ = def
		}
	}
	if typ == "" {
		if tenant.HasPlatform() && !tenant.HasInstanceURL() {
			typ = AuthTypePlatform
		} else {
			typ = AuthTypeOAuth
		}
	}
	switch typ {
	case AuthTypeOAuth, AuthTypeBasic, AuthTypePlatform:
	default:
		return nil, fmt.Errorf("auth type must be oauth, basic, or platform (got %q)", typ)
	}
	if typ == AuthTypePlatform && !tenant.HasPlatform() {
		if IsOrganizationScope(opts.Scope) || inferUSOrgAdminPaste(opts, tenant) {
			opts.Scope = PlatformScopeOrganization
		} else {
			return nil, fmt.Errorf("type=platform requires the context to have region (environment/tenant) or --scope=organization")
		}
	}

	authName := strings.TrimSpace(opts.AuthName)
	defName := defaultAuthNameForSurface(tenantName, AuthEntry{Type: typ}.EffectiveSurface(), typ)
	if authName == "" {
		if interactive {
			line, err := PromptSecretLine(fmt.Sprintf("Auth name [%s]", defName))
			if err != nil {
				return nil, err
			}
			if strings.TrimSpace(line) != "" {
				authName = strings.TrimSpace(line)
			} else {
				authName = defName
			}
		} else {
			authName = defName
		}
	}
	if authName == "" {
		return nil, fmt.Errorf("auth name required")
	}
	res.AuthName = authName

	platformScope := ""
	if typ == AuthTypePlatform {
		switch {
		case IsOrganizationScope(opts.Scope) || LooksLikeOrgPlatformGateway(opts.URL) || LooksLikeOrgPlatformGateway(tenant.URL) || tenant.IsOrganizationWorkplace() || inferUSOrgAdminPaste(opts, tenant):
			platformScope = PlatformScopeOrganization
		case strings.TrimSpace(opts.Scope) != "":
			platformScope = AuthEntry{Type: AuthTypePlatform, Scope: opts.Scope}.EffectiveScope()
		}
	}

	auth := AuthEntry{
		Type:    typ,
		Tenants: []string{tenantName},
		Access:  AccessFull,
		Scope:   platformScope,
	}
	if prev, ok := cfg.Auths[authName]; ok {
		auth = prev
		auth.Type = typ
		auth.Surface = ""
		if platformScope != "" {
			auth.Scope = platformScope
		}
		if !auth.LinkedTo(tenantName) {
			auth.Tenants = append(append([]string{}, auth.Tenants...), tenantName)
		}
		if !interactive && !opts.SkipConfirm {
			return nil, fmt.Errorf("auth %q already exists — pass -y to update", authName)
		}
		if interactive && !opts.SkipConfirm {
			if !AskYesNo(fmt.Sprintf("Auth %q already exists. Update it?", authName), true, log) {
				return nil, fmt.Errorf("aborted")
			}
		}
	}

	switch typ {
	case AuthTypeOAuth, AuthTypePlatform:
		cid := strings.TrimSpace(opts.ClientID)
		if cid == "" {
			cid = strings.TrimSpace(auth.ClientID)
		}
		if cid == "" && interactive {
			line, err := PromptSecretLine("OAuth client ID")
			if err != nil {
				return nil, err
			}
			cid = strings.TrimSpace(line)
		}
		if cid == "" {
			return nil, fmt.Errorf("client ID is required for oauth — pass --client-id=")
		}
		auth.ClientID = cid
		auth.Username = ""
		secret := opts.ClientSecret
		if secret == "" && interactive {
			var err error
			secret, err = PromptSecret(fmt.Sprintf("Client secret for %s (leave blank to skip)", authName))
			if err != nil {
				return nil, err
			}
		}
		if strings.TrimSpace(secret) != "" {
			if err := SetCredentialSecret(authName, SecretKindClientSecret, secret); err != nil {
				return nil, fmt.Errorf("store client secret: %w", err)
			}
		}
	case "basic":
		user := strings.TrimSpace(opts.Username)
		if user == "" {
			user = strings.TrimSpace(auth.Username)
		}
		if user == "" && interactive {
			line, err := PromptSecretLine("Jamf username")
			if err != nil {
				return nil, err
			}
			user = strings.TrimSpace(line)
		}
		if user == "" {
			return nil, fmt.Errorf("username is required for basic auth — pass --username=")
		}
		auth.Username = user
		auth.ClientID = ""
		password := opts.Password
		if password == "" && interactive {
			var err error
			password, err = PromptSecret(fmt.Sprintf("Password for %s (leave blank to skip)", user))
			if err != nil {
				return nil, err
			}
		}
		if strings.TrimSpace(password) != "" {
			if err := SetCredentialSecret(authName, SecretKindPassword, password); err != nil {
				return nil, fmt.Errorf("store password: %w", err)
			}
		}
	}
	if err := auth.Validate(); err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}
	if cfg.Auths == nil {
		cfg.Auths = map[string]AuthEntry{}
	}
	cfg.Auths[authName] = auth
	if strings.TrimSpace(cfg.Tenant.Default) == "" {
		cfg.SetDefaultTenant(tenantName)
	}
	if err := SaveConfig(cfg); err != nil {
		return nil, fmt.Errorf("save config: %w", err)
	}

	doTest := interactive && !opts.NoTest
	if opts.Test {
		doTest = true
	}
	if opts.NoTest {
		doTest = false
	}
	if doTest {
		surface := auth.EffectiveSurface()
		err := WithSessionOverrides(SessionOverrides{Tenant: tenantName, Auth: authName, Surface: surface}, func() error {
			_, err := AuthTestValue(authName, log)
			return err
		})
		res.Tested = true
		if err != nil {
			return res, fmt.Errorf("saved, but auth test failed: %w", err)
		}
		tid := ""
		if surface == SurfacePlatform {
			tid = tenant.PlatformScopeKey()
		}
		if rec := LoadCaps(authName, tenantName, surface, tid); rec != nil {
			res.Live = rec.Live
		}
	}
	return res, nil
}

func newContextIsOrgWorkplace(url, region, eid, tid, scope, typ string) bool {
	if strings.TrimSpace(eid) != "" || strings.TrimSpace(tid) != "" {
		return false
	}
	if looksLikeInstanceURL(url) {
		return false
	}
	if IsOrganizationScope(scope) || LooksLikeOrgPlatformGateway(url) {
		return true
	}
	r := strings.ToLower(strings.TrimSpace(region))
	if r == "" {
		r = RegionFromGatewayURL(url)
	}
	return r == OrgPlatformRegion && (typ == "" || NormalizeAuthType(typ) == AuthTypePlatform)
}

func inferUSOrgAdminPaste(opts CredAddOpts, tenant TenantConfig) bool {
	if tenant.HasScopeID() || strings.TrimSpace(opts.EnvironmentID) != "" || strings.TrimSpace(opts.TenantID) != "" {
		return false
	}
	if looksLikeInstanceURL(opts.URL) {
		return false
	}
	r := strings.ToLower(strings.TrimSpace(opts.Region))
	if r == "" {
		r = tenant.EffectiveRegion()
	}
	if r == "" {
		r = RegionFromGatewayURL(opts.URL)
	}
	return r == OrgPlatformRegion && (opts.Type == "" || NormalizeAuthType(opts.Type) == AuthTypePlatform)
}

func looksLikeInstanceURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	return !LooksLikePlatformGateway(raw) && !LooksLikeRetiredPlatformGateway(raw)
}

func attachCredAddOrgs(cfg *Config, name string, tenant TenantConfig, opts *CredAddOpts, interactive bool) error {
	if cfg == nil || opts == nil {
		return nil
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	if tenant.IsOrganizationWorkplace() {
		if cfg.Orgs == nil {
			cfg.Orgs = map[string]OrgConfig{}
		}
		if _, ok := cfg.Orgs[name]; !ok {
			cfg.Orgs[name] = OrgConfig{}
		}
		return expandOrgs(cfg)
	}
	if !tenant.HasPlatform() {
		return nil
	}
	orgName := strings.TrimSpace(opts.OrgName)
	if orgName == "" && interactive {
		line, err := PromptSecretLine("Parent organization name (blank to skip nested orgs:)")
		if err != nil {
			return err
		}
		orgName = strings.TrimSpace(line)
		opts.OrgName = orgName
	}
	if orgName == "" {
		return nil
	}
	envName := strings.TrimSpace(opts.EnvironmentName)
	product := tenant.EffectiveTenantID() != "" || tenant.HasInstanceURL()
	if product && envName == "" && interactive {
		line, err := PromptSecretLine("Environment workplace name (blank to skip nested orgs:)")
		if err != nil {
			return err
		}
		envName = strings.TrimSpace(line)
		opts.EnvironmentName = envName
	}
	if cfg.Orgs == nil {
		cfg.Orgs = map[string]OrgConfig{}
	}
	org := cfg.Orgs[orgName]
	if org.Environments == nil {
		org.Environments = map[string]OrgEnvironmentConfig{}
	}
	if product {
		if envName == "" || envName == name || envName == orgName {
			cfg.Orgs[orgName] = org
			return expandOrgs(cfg)
		}
		env := org.Environments[envName]
		if eid := tenant.EffectiveEnvironmentID(); eid != "" {
			env.ID = eid
		}
		if r := strings.TrimSpace(tenant.Region); r != "" {
			env.Region = r
		}
		if env.Tenants == nil {
			env.Tenants = map[string]OrgProductTenant{}
		}
		env.Tenants[name] = OrgProductTenant{TenantID: tenant.EffectiveTenantID(), URL: tenant.URL}
		org.Environments[envName] = env
		cfg.Orgs[orgName] = org
		return expandOrgs(cfg)
	}
	if tenant.EffectiveEnvironmentID() == "" {
		cfg.Orgs[orgName] = org
		return expandOrgs(cfg)
	}
	if name == orgName {
		return fmt.Errorf("orgs: environment name %q cannot match organization %q", name, orgName)
	}
	env := org.Environments[name]
	env.ID = tenant.EffectiveEnvironmentID()
	env.Region = tenant.Region
	org.Environments[name] = env
	cfg.Orgs[orgName] = org
	return expandOrgs(cfg)
}
