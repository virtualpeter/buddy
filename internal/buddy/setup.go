package buddy

import (
	"fmt"
	"sort"
	"strings"

	"buddy/internal/buddy/secrets"
)

func defaultOAuthAuthName(tenantName string) string {
	t := strings.TrimSpace(tenantName)
	if t != "" && t != DefaultTenant {
		return t + "-oauth"
	}
	return "buddy-oauth"
}

// SetupSecretsResult is returned by SetupSecrets.
type SetupSecretsResult struct {
	CreatedAuths  []string
	StoredSecrets []string
	UpdatedAuths  []string // clientId / username filled in
	Missing       []string
}

// SetupSecretsOpts controls SetupSecrets. Empty secret/identity fields are
// prompted when stdin is a TTY; non-empty values are used as-is (tests).
type SetupSecretsOpts struct {
	SkipConfirm  bool
	Type         string // oauth (default) or basic, when creating an auth
	AuthName     string
	ClientID     string
	ClientSecret string
	Username     string
	Password     string
}

// SetupSecrets creates missing auths for tenants that have none, fills
// instance/platform coverage gaps, then stores missing passwords / client secrets.
func SetupSecrets(cfg *Config, opts SetupSecretsOpts) (*SetupSecretsResult, error) {
	if cfg == nil {
		return nil, fmt.Errorf("internal error: config is nil")
	}
	interactive := StdinIsTerminal() && !opts.SkipConfirm
	res := &SetupSecretsResult{}

	tenantNames := make([]string, 0, len(cfg.Tenants))
	for n := range cfg.Tenants {
		tenantNames = append(tenantNames, n)
	}
	sort.Strings(tenantNames)

	changed := false
	for _, tn := range tenantNames {
		if len(cfg.AuthsForTenant(tn)) > 0 {
			continue
		}
		tenant := cfg.Tenants[tn]
		surface := SurfaceInstance
		if !tenant.HasInstanceURL() && tenant.HasPlatform() {
			surface = SurfacePlatform
		}
		created, err := setupSecretsCreateAuth(cfg, tn, surface, opts, interactive)
		if err != nil {
			return res, err
		}
		if created == "" {
			res.Missing = append(res.Missing, fmt.Sprintf("tenant %q has no auth — run 'buddy setup secrets' in a terminal or 'buddy cred add'", tn))
			continue
		}
		res.CreatedAuths = append(res.CreatedAuths, created)
		changed = true
	}

	for _, tn := range tenantNames {
		tenant := cfg.Tenants[tn]
		for _, surface := range tenantSurfaces(tenant) {
			if surface != SurfaceInstance && surface != SurfacePlatform {
				continue
			}
			if tenantHasCoveringAuth(cfg, tn, surface) {
				continue
			}
			if len(cfg.AuthsForTenant(tn)) == 0 {
				continue // already reported as "has no auth"
			}
			created, err := setupSecretsCreateAuth(cfg, tn, surface, opts, interactive)
			if err != nil {
				return res, err
			}
			if created == "" {
				res.Missing = append(res.Missing, fmt.Sprintf("tenant %q has no %s-capable auth — run 'buddy setup secrets' in a terminal or 'buddy cred add'", tn, surface))
				continue
			}
			res.CreatedAuths = append(res.CreatedAuths, created)
			changed = true
		}
	}

	authNames := make([]string, 0, len(cfg.Auths))
	for n := range cfg.Auths {
		authNames = append(authNames, n)
	}
	sort.Strings(authNames)

	for _, name := range authNames {
		updated, stored, miss, err := setupSecretsFillAuth(cfg, name, opts, interactive)
		if err != nil {
			return res, err
		}
		if updated {
			res.UpdatedAuths = append(res.UpdatedAuths, name)
			changed = true
		}
		if stored != "" {
			res.StoredSecrets = append(res.StoredSecrets, stored)
		}
		if miss != "" {
			res.Missing = append(res.Missing, miss)
		}
	}

	if changed {
		if err := SaveConfig(cfg); err != nil {
			return res, fmt.Errorf("save config: %w", err)
		}
	}
	if len(res.Missing) > 0 {
		return res, fmt.Errorf("%s", strings.Join(res.Missing, "\n"))
	}
	return res, nil
}

func tenantHasCoveringAuth(cfg *Config, tenantName, surface string) bool {
	if cfg == nil {
		return false
	}
	tenant := cfg.Tenants[tenantName]
	for _, name := range cfg.AuthsForTenant(tenantName) {
		auth, err := cfg.ResolveAuth(name)
		if err != nil {
			continue
		}
		if AuthCoversCell(auth, tenant, surface) {
			return true
		}
	}
	return false
}

func defaultAuthNameForSurface(tenantName, surface, typ string) string {
	if surface == SurfacePlatform || NormalizeAuthType(typ) == AuthTypePlatform {
		t := strings.TrimSpace(tenantName)
		if t == "" {
			t = "buddy"
		}
		return t + "-platform"
	}
	if typ == "oauth" {
		return defaultOAuthAuthName(tenantName)
	}
	t := strings.TrimSpace(tenantName)
	if t == "" {
		t = "buddy"
	}
	return t + "-admin"
}

func setupSecretsCreateAuth(cfg *Config, tenantName, surface string, opts SetupSecretsOpts, interactive bool) (string, error) {
	typ := strings.ToLower(strings.TrimSpace(opts.Type))
	if typ == "" {
		typ = "oauth"
	}
	if surface == SurfacePlatform {
		typ = AuthTypePlatform
	}
	if interactive && opts.Type == "" && surface != SurfacePlatform {
		line, err := PromptSecretLine(fmt.Sprintf("Tenant %q has no %s auth. Type [oauth]", tenantName, surface))
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(line) != "" {
			typ = NormalizeAuthType(line)
		}
	}
	switch typ {
	case AuthTypeOAuth, AuthTypeBasic, AuthTypePlatform:
	default:
		return "", fmt.Errorf("auth type must be oauth, basic, or platform (got %q)", typ)
	}
	if surface == SurfacePlatform {
		typ = AuthTypePlatform
	}

	defName := defaultAuthNameForSurface(tenantName, surface, typ)
	authName := strings.TrimSpace(opts.AuthName)
	if authName == "" {
		if !interactive {
			if opts.ClientID == "" && opts.Username == "" {
				return "", nil
			}
			authName = defName
		} else {
			line, err := PromptSecretLine(fmt.Sprintf("Auth name [%s]", defName))
			if err != nil {
				return "", err
			}
			if strings.TrimSpace(line) != "" {
				authName = strings.TrimSpace(line)
			} else {
				authName = defName
			}
		}
	}
	if authName == "" {
		return "", nil
	}
	if _, exists := cfg.Auths[authName]; exists {
		return "", fmt.Errorf("auth %q already exists — pick another name or run 'buddy cred set auth'", authName)
	}

	auth := AuthEntry{
		Type:    typ,
		Tenants: []string{tenantName},
		Access:  AccessFull,
	}
	switch typ {
	case AuthTypeOAuth, AuthTypePlatform:
		cid := strings.TrimSpace(opts.ClientID)
		if cid == "" {
			if !interactive {
				return "", nil
			}
			line, err := PromptSecretLine("OAuth client ID")
			if err != nil {
				return "", err
			}
			cid = strings.TrimSpace(line)
		}
		if cid == "" {
			return "", fmt.Errorf("client ID is required for oauth auth %q", authName)
		}
		auth.ClientID = cid
		secret := opts.ClientSecret
		if secret == "" {
			if !interactive {
				// create stub; secret pass will report missing
			} else {
				var err error
				secret, err = PromptSecret(fmt.Sprintf("Client secret for %s", authName))
				if err != nil {
					return "", err
				}
			}
		}
		if strings.TrimSpace(secret) != "" {
			if err := SetCredentialSecret(authName, secrets.KindClientSecret, secret); err != nil {
				return "", fmt.Errorf("store client secret: %w", err)
			}
		}
	case "basic":
		user := strings.TrimSpace(opts.Username)
		if user == "" {
			if !interactive {
				return "", nil
			}
			line, err := PromptSecretLine("Jamf username")
			if err != nil {
				return "", err
			}
			user = strings.TrimSpace(line)
		}
		if user == "" {
			return "", fmt.Errorf("username is required for basic auth %q", authName)
		}
		auth.Username = user
		password := opts.Password
		if password == "" && interactive {
			var err error
			password, err = PromptSecret(fmt.Sprintf("Password for %s", user))
			if err != nil {
				return "", err
			}
		}
		if strings.TrimSpace(password) != "" {
			if err := SetCredentialSecret(authName, secrets.KindPassword, password); err != nil {
				return "", fmt.Errorf("store password: %w", err)
			}
		}
	}
	if err := auth.Validate(); err != nil {
		return "", fmt.Errorf("auth: %w", err)
	}
	if cfg.Auths == nil {
		cfg.Auths = map[string]AuthEntry{}
	}
	cfg.Auths[authName] = auth
	return authName, nil
}

func setupSecretsFillAuth(cfg *Config, authName string, opts SetupSecretsOpts, interactive bool) (updated bool, stored string, missing string, err error) {
	auth, ok := cfg.Auths[authName]
	if !ok {
		return false, "", "", nil
	}
	switch auth.Type {
	case AuthTypeCommand:
		return false, "", "", nil
	case AuthTypeOAuth, AuthTypePlatform:
		if strings.TrimSpace(auth.ClientID) == "" {
			cid := strings.TrimSpace(opts.ClientID)
			if cid == "" && interactive {
				line, err := PromptSecretLine(fmt.Sprintf("OAuth client ID for auth %q", authName))
				if err != nil {
					return false, "", "", err
				}
				cid = strings.TrimSpace(line)
			}
			if cid == "" {
				return false, "", fmt.Sprintf("auth %q is oauth with no clientId — run 'buddy cred set auth %s --client-id=…'", authName, authName), nil
			}
			auth.ClientID = cid
			cfg.Auths[authName] = auth
			updated = true
		}
		st, err := CredentialSecretStatusFor(cfg, authName)
		if err != nil {
			return updated, "", "", err
		}
		if st.ClientSecretInConfig || st.ClientSecretInStore {
			return updated, "", "", nil
		}
		secret := opts.ClientSecret
		if secret == "" && interactive {
			secret, err = PromptSecret(fmt.Sprintf("Client secret for %s", authName))
			if err != nil {
				return updated, "", "", err
			}
		}
		if strings.TrimSpace(secret) == "" {
			return updated, "", fmt.Sprintf("auth %q is missing a client secret — run 'buddy cred set secret %s --client-secret'", authName, authName), nil
		}
		if err := SetCredentialSecret(authName, secrets.KindClientSecret, secret); err != nil {
			return updated, "", "", fmt.Errorf("store client secret: %w", err)
		}
		return updated, authName + " client-secret", "", nil
	case "basic":
		if strings.TrimSpace(auth.Username) == "" {
			user := strings.TrimSpace(opts.Username)
			if user == "" && interactive {
				line, err := PromptSecretLine(fmt.Sprintf("Jamf username for auth %q", authName))
				if err != nil {
					return false, "", "", err
				}
				user = strings.TrimSpace(line)
			}
			if user == "" {
				return false, "", fmt.Sprintf("auth %q is basic with no username — run 'buddy cred set auth %s --username=…'", authName, authName), nil
			}
			auth.Username = user
			cfg.Auths[authName] = auth
			updated = true
		}
		st, err := CredentialSecretStatusFor(cfg, authName)
		if err != nil {
			return updated, "", "", err
		}
		if st.PasswordInConfig || st.PasswordInStore {
			return updated, "", "", nil
		}
		password := opts.Password
		if password == "" && interactive {
			password, err = PromptSecret(fmt.Sprintf("Password for %s", auth.Username))
			if err != nil {
				return updated, "", "", err
			}
		}
		if strings.TrimSpace(password) == "" {
			return updated, "", fmt.Sprintf("auth %q is missing a password — run 'buddy cred set secret %s'", authName, authName), nil
		}
		if err := SetCredentialSecret(authName, secrets.KindPassword, password); err != nil {
			return updated, "", "", fmt.Errorf("store password: %w", err)
		}
		return updated, authName + " password", "", nil
	default:
		return false, "", "", nil
	}
}
