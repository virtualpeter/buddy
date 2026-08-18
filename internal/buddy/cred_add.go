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

	surface := strings.ToLower(strings.TrimSpace(opts.Surface))
	if surface == "" && interactive {
		def := SurfaceAny
		if tenant.HasInstanceURL() && !tenant.HasPlatform() {
			def = SurfaceInstance
		} else if tenant.HasPlatform() && !tenant.HasInstanceURL() {
			def = SurfacePlatform
		}
		line, err := PromptSecretLine(fmt.Sprintf("Auth surface any|instance|platform [%s]", def))
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(line) != "" {
			surface = strings.ToLower(strings.TrimSpace(line))
		} else {
			surface = def
		}
	}
	if surface == "" {
		if tenant.HasPlatform() && !tenant.HasInstanceURL() {
			surface = SurfacePlatform
		} else if tenant.HasInstanceURL() && !tenant.HasPlatform() {
			surface = SurfaceInstance
		} else {
			surface = SurfaceAny
		}
	}
	switch surface {
	case SurfaceAny, SurfaceInstance, SurfacePlatform:
	default:
		return nil, fmt.Errorf("surface must be any, instance, or platform (got %q)", surface)
	}

	typ := strings.ToLower(strings.TrimSpace(opts.Type))
	if typ == "" {
		typ = "oauth"
	}
	if interactive && opts.Type == "" && surface != SurfacePlatform {
		line, err := PromptSecretLine("Auth type oauth|basic [oauth]")
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(line) != "" {
			typ = strings.ToLower(strings.TrimSpace(line))
		}
	}
	if surface == SurfacePlatform {
		typ = "oauth"
	}
	switch typ {
	case "oauth", "basic":
	default:
		return nil, fmt.Errorf("auth type must be oauth or basic (got %q)", typ)
	}

	authName := strings.TrimSpace(opts.AuthName)
	defName := defaultAuthNameForSurface(tenantName, surface, typ)
	if surface == SurfaceAny && typ == "oauth" {
		defName = defaultOAuthAuthName(tenantName)
	}
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
		Surface: surface,
	}
	if prev, ok := cfg.Auths[authName]; ok {
		auth = prev
		auth.Type = typ
		auth.Surface = surface
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
	case "oauth":
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
		wantSurf := surface
		if wantSurf == SurfaceAny {
			wantSurf = ""
		}
		err := WithSessionOverrides(SessionOverrides{Tenant: tenantName, Auth: authName, Surface: wantSurf}, func() error {
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
		testSurf := surface
		if testSurf == SurfaceAny {
			if tenant.HasInstanceURL() {
				testSurf = SurfaceInstance
			} else {
				testSurf = SurfacePlatform
			}
		}
		if rec := LoadCaps(authName, tenantName, testSurf, tid); rec != nil {
			res.Live = rec.Live
		}
	}
	return res, nil
}
