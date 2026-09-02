package buddy

import (
	"fmt"
	"log/slog"
	"strings"
)

// CredAddOpts controls buddy cred add. Empty fields are prompted when stdin is a TTY.
type CredAddOpts struct {
	AuthName     string
	TenantName   string
	URL          string
	Region       string
	TenantID     string
	Surface      string
	Type         string
	ClientID     string
	Username     string
	Password     string
	ClientSecret string
	SkipConfirm  bool
	Test         bool
	NoTest       bool
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
	interactive := StdinIsTerminal() && !opts.SkipConfirm
	res := &CredAddResult{}

	tenantName := strings.TrimSpace(opts.TenantName)
	if tenantName == "" && interactive {
		line, err := PromptSecretLine("Tenant name")
		if err != nil {
			return nil, err
		}
		tenantName = strings.TrimSpace(line)
	}
	if tenantName == "" {
		return nil, fmt.Errorf("tenant name required — pass --tenant=<name> (or run without -y in a terminal)")
	}
	res.TenantName = tenantName

	tenant, tenantExists := cfg.Tenants[tenantName]
	if !tenantExists {
		url := strings.TrimSpace(opts.URL)
		region := strings.TrimSpace(opts.Region)
		tid := strings.TrimSpace(opts.TenantID)
		if interactive {
			if url == "" {
				line, err := PromptSecretLine("Jamf Pro instance URL (blank if platform-only)")
				if err != nil {
					return nil, err
				}
				url = strings.TrimSpace(line)
			}
			if region == "" {
				line, err := PromptSecretLine("Platform region (us|eu|apac, blank to skip)")
				if err != nil {
					return nil, err
				}
				region = strings.TrimSpace(line)
			}
			if region != "" && tid == "" {
				line, err := PromptSecretLine("Platform tenant id")
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
		tenant = TenantConfig{URL: url, Region: region, TenantID: tid}
		if err := tenant.Validate(); err != nil {
			return nil, fmt.Errorf("tenant: %w", err)
		}
		if cfg.Tenants == nil {
			cfg.Tenants = map[string]TenantConfig{}
		}
		cfg.Tenants[tenantName] = tenant
		res.CreatedTenant = true
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
		return nil, fmt.Errorf("type=platform requires the tenant to have region + tenantId")
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

	auth := AuthEntry{
		Type:    typ,
		Tenants: []string{tenantName},
		Access:  AccessFull,
	}
	if prev, ok := cfg.Auths[authName]; ok {
		auth = prev
		auth.Type = typ
		auth.Surface = ""
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
			tid = tenant.EffectiveTenantID()
		}
		if rec := LoadCaps(authName, tenantName, surface, tid); rec != nil {
			res.Live = rec.Live
		}
	}
	return res, nil
}
