package buddy

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultTenant is the default tenant name used when none is configured.
const DefaultTenant = "default"

// Deprecated: DefaultCred is an alias of DefaultTenant for older call sites.
const DefaultCred = DefaultTenant

// Access levels for an auth entry.
const (
	AccessFull     = "full"
	AccessReadonly = "readonly"
)

// Surface hints for an auth entry.
const (
	SurfaceAny      = "any"
	SurfacePlatform = "platform"
	SurfaceInstance = "instance"
)

// AuthConfig holds the authentication method fields (type + secrets/identifiers).
type AuthConfig struct {
	Type         string `yaml:"type"`                   // "command", "basic", or "oauth"
	RunCommand   string `yaml:"runCommand,omitempty"`   // type: command
	Username     string `yaml:"username,omitempty"`     // type: basic
	Password     string `yaml:"password,omitempty"`     // type: basic
	ClientID     string `yaml:"clientId,omitempty"`     // type: oauth
	ClientSecret string `yaml:"clientSecret,omitempty"` // type: oauth
	// TenantID is legacy-only (mis-indented under auth in old credentials:).
	TenantID string `yaml:"tenantId,omitempty"`
}

// MarshalYAML emits only fields relevant to auth.type.
func (a AuthConfig) MarshalYAML() (interface{}, error) {
	n := &yaml.Node{Kind: yaml.MappingNode}
	add := func(key, val string, force bool) {
		if !force && val == "" {
			return
		}
		n.Content = append(n.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: val},
		)
	}
	add("type", a.Type, true)
	switch a.Type {
	case "command":
		add("runCommand", a.RunCommand, false)
	case "basic":
		add("username", a.Username, false)
		add("password", a.Password, false)
	case "oauth":
		add("clientId", a.ClientID, false)
		add("clientSecret", a.ClientSecret, false)
	default:
		add("runCommand", a.RunCommand, false)
		add("username", a.Username, false)
		add("password", a.Password, false)
		add("clientId", a.ClientID, false)
		add("clientSecret", a.ClientSecret, false)
	}
	return n, nil
}

// TenantConfig is a Jamf environment: instance URL and/or Platform gateway identity.
type TenantConfig struct {
	Region   string `yaml:"region,omitempty"`   // Platform API Gateway region (us|eu|apac|…)
	URL      string `yaml:"url,omitempty"`      // Instance or gateway URL; optional when region is set
	TenantID string `yaml:"tenantId,omitempty"` // Platform API Gateway tenant id
}

// AuthEntry is a secret-bearing principal linked to one or more tenants.
type AuthEntry struct {
	Type         string   `yaml:"type"`
	RunCommand   string   `yaml:"runCommand,omitempty"`
	Username     string   `yaml:"username,omitempty"`
	Password     string   `yaml:"password,omitempty"`
	ClientID     string   `yaml:"clientId,omitempty"`
	ClientSecret string   `yaml:"clientSecret,omitempty"`
	Tenants      []string `yaml:"tenants"`
	Access       string   `yaml:"access,omitempty"`  // readonly | full (default full)
	Surface      string   `yaml:"surface,omitempty"` // platform | instance | any (default any)
}

// AuthConfig returns the secret/type block for this entry.
func (a AuthEntry) AuthConfig() AuthConfig {
	return AuthConfig{
		Type:         a.Type,
		RunCommand:   a.RunCommand,
		Username:     a.Username,
		Password:     a.Password,
		ClientID:     a.ClientID,
		ClientSecret: a.ClientSecret,
	}
}

// EffectiveAccess returns access, defaulting to full.
func (a AuthEntry) EffectiveAccess() string {
	switch strings.ToLower(strings.TrimSpace(a.Access)) {
	case AccessReadonly:
		return AccessReadonly
	default:
		return AccessFull
	}
}

// EffectiveSurface returns surface, defaulting to any.
func (a AuthEntry) EffectiveSurface() string {
	switch strings.ToLower(strings.TrimSpace(a.Surface)) {
	case SurfacePlatform, SurfaceInstance:
		return strings.ToLower(strings.TrimSpace(a.Surface))
	default:
		return SurfaceAny
	}
}

// LinkedTo reports whether this auth lists tenantName.
func (a AuthEntry) LinkedTo(tenantName string) bool {
	for _, t := range a.Tenants {
		if t == tenantName {
			return true
		}
	}
	return false
}

// CredentialConfig is a resolved tenant+auth view used by the HTTP/auth stack.
// It is not stored in the new YAML shape; ResolveSession / ResolveCredential build it.
type CredentialConfig struct {
	Region   string     `yaml:"region,omitempty"`
	URL      string     `yaml:"url,omitempty"`
	TenantID string     `yaml:"tenantId,omitempty"`
	Auth     AuthConfig `yaml:"auth"`
}

// EffectiveTenantID returns the Platform tenant id.
func (c CredentialConfig) EffectiveTenantID() string {
	if t := strings.TrimSpace(c.TenantID); t != "" {
		return t
	}
	return strings.TrimSpace(c.Auth.TenantID)
}

// EffectiveRegion returns the configured region, or infers it from a gateway URL.
func (c CredentialConfig) EffectiveRegion() string {
	if r := strings.TrimSpace(c.Region); r != "" {
		return strings.ToLower(r)
	}
	return RegionFromGatewayURL(c.URL)
}

// EffectiveBaseURL returns the API base URL for this resolved credential.
// When the tenant has both an instance URL and platform identity, PreferPlatform
// on the session chooses gateway vs instance — callers that need a specific
// surface should set URL appropriately via ComposeCredential.
func (c CredentialConfig) EffectiveBaseURL() string {
	if u := strings.TrimRight(strings.TrimSpace(c.URL), "/"); u != "" {
		return u
	}
	if r := strings.TrimSpace(c.Region); r != "" {
		if norm, err := NormalizePlatformRegion(r); err == nil {
			return PlatformGatewayURL(norm)
		}
	}
	return ""
}

// IsPlatform returns true when this resolved credential targets the Platform gateway.
func (c CredentialConfig) IsPlatform() bool {
	return c.EffectiveTenantID() != "" ||
		strings.TrimSpace(c.Region) != "" ||
		LooksLikePlatformGateway(c.URL)
}

// UsesPlatformAuth is true when OAuth should use gateway token endpoints.
func (c CredentialConfig) UsesPlatformAuth() bool {
	return c.IsPlatform()
}

// HasInstanceURL reports whether an instance (non-gateway) URL is configured.
// When tenantId is set without a region, url is treated as a gateway/proxy base
// (not an instance host) so test servers and custom proxies work.
func (t TenantConfig) HasInstanceURL() bool {
	u := strings.TrimSpace(t.URL)
	if u == "" || LooksLikePlatformGateway(u) {
		return false
	}
	if strings.TrimSpace(t.TenantID) != "" && t.EffectiveRegion() == "" {
		return false
	}
	return true
}

// HasPlatform reports whether this tenant has Platform gateway identity.
func (t TenantConfig) HasPlatform() bool {
	return strings.TrimSpace(t.TenantID) != ""
}

// EffectiveTenantID returns the platform tenant id.
func (t TenantConfig) EffectiveTenantID() string {
	return strings.TrimSpace(t.TenantID)
}

// EffectiveRegion returns region or infers from gateway URL.
func (t TenantConfig) EffectiveRegion() string {
	if r := strings.TrimSpace(t.Region); r != "" {
		return strings.ToLower(r)
	}
	return RegionFromGatewayURL(t.URL)
}

// GatewayBaseURL returns the platform gateway base URL, or "".
// Prefer regional default; otherwise a gateway-looking url; otherwise any url
// when tenantId is set (proxy / test servers).
func (t TenantConfig) GatewayBaseURL() string {
	if r := t.EffectiveRegion(); r != "" {
		if norm, err := NormalizePlatformRegion(r); err == nil {
			def := PlatformGatewayURL(norm)
			// Explicit non-default url (proxy) wins when present.
			if u := strings.TrimRight(strings.TrimSpace(t.URL), "/"); u != "" && u != def && !t.HasInstanceURL() {
				return u
			}
			return def
		}
	}
	if LooksLikePlatformGateway(t.URL) {
		return strings.TrimRight(strings.TrimSpace(t.URL), "/")
	}
	if t.HasPlatform() {
		if u := strings.TrimRight(strings.TrimSpace(t.URL), "/"); u != "" && !t.HasInstanceURL() {
			return u
		}
		// Hybrid tenant: instance url + tenantId needs region for gateway host.
	}
	return ""
}

// InstanceBaseURL returns the instance URL when present and not a gateway host.
func (t TenantConfig) InstanceBaseURL() string {
	if t.HasInstanceURL() {
		return strings.TrimRight(strings.TrimSpace(t.URL), "/")
	}
	return ""
}

// NormalizePlatformRegion lowercases and validates a gateway region code.
func NormalizePlatformRegion(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return "", fmt.Errorf("region is empty")
	}
	for _, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-'
		if !ok {
			return "", fmt.Errorf("invalid Platform API Gateway region %q (use us, eu, apac, …)", s)
		}
	}
	if s[0] < 'a' || s[0] > 'z' {
		return "", fmt.Errorf("invalid Platform API Gateway region %q (use us, eu, apac, …)", s)
	}
	return s, nil
}

// PlatformGatewayURL builds the regional gateway base URL for region.
func PlatformGatewayURL(region string) string {
	return "https://" + strings.ToLower(strings.TrimSpace(region)) + ".apigw.jamf.com"
}

// RegionFromGatewayURL returns the region subdomain for a *.apigw.jamf.com URL, or "".
func RegionFromGatewayURL(rawURL string) string {
	u := strings.ToLower(strings.TrimSpace(rawURL))
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	if i := strings.IndexByte(u, '/'); i >= 0 {
		u = u[:i]
	}
	const suffix = ".apigw.jamf.com"
	if !strings.HasSuffix(u, suffix) {
		return ""
	}
	return strings.TrimSuffix(u, suffix)
}

// LooksLikePlatformGateway reports whether url is a Jamf Platform API Gateway host.
func LooksLikePlatformGateway(rawURL string) bool {
	return RegionFromGatewayURL(rawURL) != ""
}

// LooksLikeTenantUUID reports whether s looks like a hyphenated UUID.
func LooksLikeTenantUUID(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) != 36 {
		return false
	}
	parts := strings.Split(s, "-")
	if len(parts) != 5 {
		return false
	}
	if len(parts[0]) != 8 || len(parts[1]) != 4 || len(parts[2]) != 4 || len(parts[3]) != 4 || len(parts[4]) != 12 {
		return false
	}
	for _, p := range parts {
		for _, r := range p {
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
				return false
			}
		}
	}
	return true
}

// Validate checks tenant fields for consistency.
func (t TenantConfig) Validate() error {
	if r := strings.TrimSpace(t.Region); r != "" {
		if _, err := NormalizePlatformRegion(r); err != nil {
			return err
		}
	}
	hasInstance := t.HasInstanceURL()
	hasPlatform := strings.TrimSpace(t.TenantID) != ""
	if !hasInstance && !hasPlatform {
		if LooksLikePlatformGateway(t.URL) || strings.TrimSpace(t.Region) != "" {
			return fmt.Errorf("Platform API Gateway tenants require tenantId; see 'buddy cred help'")
		}
		return fmt.Errorf("tenant requires url (instance) and/or region+tenantId (platform); see 'buddy cred help'")
	}
	if hasPlatform {
		if t.GatewayBaseURL() == "" && t.EffectiveRegion() == "" && strings.TrimSpace(t.URL) == "" {
			return fmt.Errorf("Platform API Gateway tenants require region (e.g. us) or a gateway url")
		}
		if t.GatewayBaseURL() == "" && t.HasInstanceURL() && t.EffectiveRegion() == "" {
			return fmt.Errorf("Platform API Gateway tenants require region (e.g. us) when url is an instance host")
		}
	}
	return nil
}

// Validate checks auth entry fields.
func (a AuthEntry) Validate() error {
	switch a.Type {
	case "command", "basic", "oauth":
	case "":
		return fmt.Errorf("auth type is required (command, basic, or oauth)")
	default:
		return fmt.Errorf("unknown auth type %q (supported: command, basic, oauth)", a.Type)
	}
	if len(a.Tenants) == 0 {
		return fmt.Errorf("auth must list at least one tenant under tenants:")
	}
	switch a.EffectiveAccess() {
	case AccessFull, AccessReadonly:
	default:
		return fmt.Errorf("access must be %q or %q", AccessFull, AccessReadonly)
	}
	switch a.EffectiveSurface() {
	case SurfaceAny, SurfacePlatform, SurfaceInstance:
	default:
		return fmt.Errorf("surface must be %q, %q, or %q", SurfaceAny, SurfacePlatform, SurfaceInstance)
	}
	if a.EffectiveSurface() == SurfacePlatform && a.Type != "oauth" {
		return fmt.Errorf("surface=platform requires auth type oauth (got %q)", a.Type)
	}
	return nil
}

// Validate checks a resolved credential (tenant+auth view).
func (c CredentialConfig) Validate() error {
	if r := strings.TrimSpace(c.Region); r != "" {
		if _, err := NormalizePlatformRegion(r); err != nil {
			return err
		}
	}
	if !c.IsPlatform() {
		if strings.TrimSpace(c.URL) == "" {
			return fmt.Errorf("url is required (or set region + tenantId for Platform API Gateway); see 'buddy cred help'")
		}
		return nil
	}
	if c.EffectiveTenantID() == "" {
		return fmt.Errorf("Platform API Gateway credentials require tenantId; see 'buddy cred help'")
	}
	if c.Auth.Type != "oauth" {
		return fmt.Errorf("Platform API Gateway credentials require auth.type=oauth (got %q); basic and command auth are not supported", c.Auth.Type)
	}
	if c.EffectiveBaseURL() == "" {
		return fmt.Errorf("Platform API Gateway credentials require region (e.g. us) or url (e.g. https://us.apigw.jamf.com)")
	}
	return nil
}

// Config holds buddy CLI configuration (tenants + auths).
type Config struct {
	Tenant  string                  `yaml:"tenant"`
	Tenants map[string]TenantConfig `yaml:"tenants"`
	Auths   map[string]AuthEntry    `yaml:"auths"`

	// Legacy fields — populated only while unmarshaling old configs; cleared after migrate.
	Cred        string                      `yaml:"cred,omitempty"`
	Credentials map[string]CredentialConfig `yaml:"credentials,omitempty"`
}

// legacyConfigFile is used to detect and migrate the pre-tenant credentials: shape.
type legacyConfigFile struct {
	Cred        string                      `yaml:"cred"`
	Credentials map[string]CredentialConfig `yaml:"credentials"`
	Tenant      string                      `yaml:"tenant"`
	Tenants     map[string]TenantConfig     `yaml:"tenants"`
	Auths       map[string]AuthEntry        `yaml:"auths"`
}

// ComposeCredential builds a resolved CredentialConfig for the given tenant+auth
// and desired surface (platform / instance / any).
func ComposeCredential(tenant TenantConfig, auth AuthEntry, surface string) CredentialConfig {
	surface = strings.ToLower(strings.TrimSpace(surface))
	if surface == "" {
		surface = auth.EffectiveSurface()
	}
	cred := CredentialConfig{Auth: auth.AuthConfig()}
	switch surface {
	case SurfacePlatform:
		cred.TenantID = tenant.EffectiveTenantID()
		cred.Region = tenant.EffectiveRegion()
		if gw := tenant.GatewayBaseURL(); gw != "" {
			cred.URL = gw
		}
	case SurfaceInstance:
		cred.URL = tenant.InstanceBaseURL()
		if cred.URL == "" && !LooksLikePlatformGateway(tenant.URL) {
			cred.URL = strings.TrimRight(strings.TrimSpace(tenant.URL), "/")
		}
	default: // any — prefer platform when tenant has it and auth allows, else instance
		if tenant.HasPlatform() && auth.EffectiveSurface() != SurfaceInstance {
			cred.TenantID = tenant.EffectiveTenantID()
			cred.Region = tenant.EffectiveRegion()
			if gw := tenant.GatewayBaseURL(); gw != "" {
				cred.URL = gw
			} else if tenant.HasInstanceURL() {
				cred.URL = tenant.InstanceBaseURL()
			}
		} else {
			cred.URL = tenant.InstanceBaseURL()
			if cred.URL == "" {
				cred.TenantID = tenant.EffectiveTenantID()
				cred.Region = tenant.EffectiveRegion()
				cred.URL = tenant.GatewayBaseURL()
			}
		}
	}
	return cred
}

// ResolveTenant looks up a named tenant.
func (cfg *Config) ResolveTenant(name string) (TenantConfig, error) {
	if cfg.Tenants == nil {
		return TenantConfig{}, fmt.Errorf("no tenants configured — run 'buddy cred set tenant' (see 'buddy cred help')")
	}
	t, ok := cfg.Tenants[name]
	if !ok {
		return TenantConfig{}, fmt.Errorf("tenant %q not found — run 'buddy cred list' to see available tenants", name)
	}
	if err := t.Validate(); err != nil {
		return TenantConfig{}, fmt.Errorf("tenant %q: %w", name, err)
	}
	return t, nil
}

// ResolveAuth looks up a named auth entry.
func (cfg *Config) ResolveAuth(name string) (AuthEntry, error) {
	if cfg.Auths == nil {
		return AuthEntry{}, fmt.Errorf("no auths configured — run 'buddy cred set auth' (see 'buddy cred help')")
	}
	a, ok := cfg.Auths[name]
	if !ok {
		return AuthEntry{}, fmt.Errorf("auth %q not found — run 'buddy cred list' to see available auths", name)
	}
	if err := a.Validate(); err != nil {
		return AuthEntry{}, fmt.Errorf("auth %q: %w", name, err)
	}
	for _, tn := range a.Tenants {
		if _, ok := cfg.Tenants[tn]; !ok {
			return AuthEntry{}, fmt.Errorf("auth %q references unknown tenant %q", name, tn)
		}
	}
	return a, nil
}

// AuthsForTenant returns auth names linked to tenantName (unsorted).
func (cfg *Config) AuthsForTenant(tenantName string) []string {
	var out []string
	for name, a := range cfg.Auths {
		if a.LinkedTo(tenantName) {
			out = append(out, name)
		}
	}
	return out
}

// ResolveCredential resolves an auth name (with optional active tenant) into a
// CredentialConfig view. Prefer ResolveSession for operation-aware picking.
func (cfg *Config) ResolveCredential(authName string) (CredentialConfig, error) {
	sess, err := cfg.ResolveSession("", authName, OpAny)
	if err != nil {
		return CredentialConfig{}, err
	}
	return sess.Credential, nil
}

// configPath returns the path to the buddy config file.
func configPath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "buddy", "config.yaml"), nil
}

// LoadConfig loads config from ~/.config/buddy/config.yaml (or XDG_CONFIG_HOME).
// Old credentials: configs are migrated in-memory to tenants/auths and rewritten on disk.
func LoadConfig() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, fmt.Errorf("resolve config path: %w", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{Tenant: DefaultTenant}, nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var raw legacyConfigFile
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	cfg := &Config{
		Tenant:  raw.Tenant,
		Tenants: raw.Tenants,
		Auths:   raw.Auths,
	}
	migrated := false
	if len(raw.Credentials) > 0 && len(raw.Tenants) == 0 {
		cfg, err = migrateCredentialsToTenantsAuths(&raw)
		if err != nil {
			return nil, fmt.Errorf("migrate config %s: %w", path, err)
		}
		migrated = true
	}
	if cfg.Tenant == "" {
		if raw.Cred != "" && cfg.Tenants != nil {
			if _, ok := cfg.Tenants[raw.Cred]; ok {
				cfg.Tenant = raw.Cred
			}
		}
	}
	if cfg.Tenant == "" {
		cfg.Tenant = DefaultTenant
	}
	cfg.Cred = ""
	cfg.Credentials = nil

	if migrated {
		if err := SaveConfig(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "warning: migrated credentials: to tenants:/auths: in memory but could not rewrite %s: %v\n", path, err)
			fmt.Fprintf(os.Stderr, "note: fix permissions and re-run buddy (or save via 'buddy cred use') so the new shape is persisted\n")
		} else {
			fmt.Fprintf(os.Stderr, "note: migrated config.yaml from credentials: to tenants:/auths: (breaking change; see 'buddy cred help')\n")
		}
	}
	return cfg, nil
}

// ConfigFromCredentialMap builds a tenants/auths config from a legacy-style
// name→CredentialConfig map (used by tests and migration). defaultName sets
// Config.Tenant when it matches a migrated tenant or auth.
func ConfigFromCredentialMap(defaultName string, creds map[string]CredentialConfig) *Config {
	cfg, err := migrateCredentialsToTenantsAuths(&legacyConfigFile{
		Cred:        defaultName,
		Credentials: creds,
	})
	if err != nil || cfg == nil {
		return &Config{Tenant: DefaultTenant, Tenants: map[string]TenantConfig{}, Auths: map[string]AuthEntry{}}
	}
	return cfg
}

// migrateCredentialsToTenantsAuths converts the legacy flat credentials map.
// Unique host identities (instance URL and/or platform tenantId) become tenants;
// each old credential name becomes an auth linked to that tenant.
func migrateCredentialsToTenantsAuths(raw *legacyConfigFile) (*Config, error) {
	cfg := &Config{
		Tenants: map[string]TenantConfig{},
		Auths:   map[string]AuthEntry{},
	}
	// identity key → tenant name
	identityTenant := map[string]string{}

	names := make([]string, 0, len(raw.Credentials))
	for n := range raw.Credentials {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, name := range names {
		c := raw.Credentials[name]
		if strings.TrimSpace(c.TenantID) == "" && strings.TrimSpace(c.Auth.TenantID) != "" {
			c.TenantID = strings.TrimSpace(c.Auth.TenantID)
		}
		c.Auth.TenantID = ""
		if strings.TrimSpace(c.Region) == "" {
			if inferred := RegionFromGatewayURL(c.URL); inferred != "" {
				c.Region = inferred
			}
		}
		idKey := tenantIdentityKey(c)
		tenantName, ok := identityTenant[idKey]
		if !ok {
			tenantName = name
			// Avoid colliding with an already-created tenant under a different identity.
			for {
				if _, exists := cfg.Tenants[tenantName]; !exists {
					break
				}
				tenantName = name + "-tenant"
				if _, exists := cfg.Tenants[tenantName]; !exists {
					break
				}
				tenantName = fmt.Sprintf("%s-%d", name, len(cfg.Tenants)+1)
				break
			}
			tc := TenantConfig{
				Region:   c.Region,
				URL:      c.URL,
				TenantID: c.TenantID,
			}
			// Drop gateway URL when region is set (same as normalize).
			if tc.Region != "" {
				if norm, err := NormalizePlatformRegion(tc.Region); err == nil {
					tc.Region = norm
					if strings.TrimRight(strings.TrimSpace(tc.URL), "/") == PlatformGatewayURL(norm) {
						tc.URL = ""
					}
				}
			}
			cfg.Tenants[tenantName] = tc
			identityTenant[idKey] = tenantName
		}
		surface := SurfaceAny
		if c.IsPlatform() {
			surface = SurfacePlatform
		} else {
			surface = SurfaceInstance
		}
		cfg.Auths[name] = AuthEntry{
			Type:         c.Auth.Type,
			RunCommand:   c.Auth.RunCommand,
			Username:     c.Auth.Username,
			Password:     c.Auth.Password,
			ClientID:     c.Auth.ClientID,
			ClientSecret: c.Auth.ClientSecret,
			Tenants:      []string{tenantName},
			Access:       AccessFull,
			Surface:      surface,
		}
	}
	if raw.Cred != "" {
		if a, ok := cfg.Auths[raw.Cred]; ok && len(a.Tenants) > 0 {
			cfg.Tenant = a.Tenants[0]
		} else if _, ok := cfg.Tenants[raw.Cred]; ok {
			cfg.Tenant = raw.Cred
		}
	}
	if cfg.Tenant == "" {
		cfg.Tenant = DefaultTenant
	}
	return cfg, nil
}

func tenantIdentityKey(c CredentialConfig) string {
	tid := strings.TrimSpace(c.TenantID)
	if tid == "" {
		tid = strings.TrimSpace(c.Auth.TenantID)
	}
	url := strings.TrimRight(strings.ToLower(strings.TrimSpace(c.URL)), "/")
	region := strings.ToLower(strings.TrimSpace(c.Region))
	if region == "" {
		region = RegionFromGatewayURL(c.URL)
	}
	if tid != "" {
		return "platform:" + region + ":" + tid
	}
	return "instance:" + url
}

func normalizeConfigForSave(cfg *Config) {
	if cfg == nil {
		return
	}
	cfg.Cred = ""
	cfg.Credentials = nil
	for name, t := range cfg.Tenants {
		if strings.TrimSpace(t.Region) == "" {
			if inferred := RegionFromGatewayURL(t.URL); inferred != "" {
				t.Region = inferred
			}
		} else if norm, err := NormalizePlatformRegion(t.Region); err == nil {
			t.Region = norm
		}
		if t.Region != "" {
			def := PlatformGatewayURL(t.Region)
			if strings.TrimRight(strings.TrimSpace(t.URL), "/") == def {
				t.URL = ""
			}
		}
		cfg.Tenants[name] = t
	}
	for name, a := range cfg.Auths {
		if a.Access == AccessFull {
			a.Access = ""
		}
		if a.Surface == SurfaceAny {
			a.Surface = ""
		}
		cfg.Auths[name] = a
	}
}

func mapKeyOrder(prefer []string, m map[string]struct{}) []string {
	seen := make(map[string]bool, len(prefer))
	out := make([]string, 0, len(m))
	for _, name := range prefer {
		if _, ok := m[name]; ok {
			out = append(out, name)
			seen[name] = true
		}
	}
	var rest []string
	for name := range m {
		if !seen[name] {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	return append(out, rest...)
}

func yamlMapKeyOrder(data []byte, key string) []string {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil
	}
	root := &doc
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		root = doc.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value != key {
			continue
		}
		m := root.Content[i+1]
		if m.Kind != yaml.MappingNode {
			return nil
		}
		names := make([]string, 0, len(m.Content)/2)
		for j := 0; j+1 < len(m.Content); j += 2 {
			names = append(names, m.Content[j].Value)
		}
		return names
	}
	return nil
}

func marshalConfig(cfg *Config, existing []byte) ([]byte, error) {
	normalizeConfigForSave(cfg)

	tenantSet := make(map[string]struct{}, len(cfg.Tenants))
	for n := range cfg.Tenants {
		tenantSet[n] = struct{}{}
	}
	authSet := make(map[string]struct{}, len(cfg.Auths))
	for n := range cfg.Auths {
		authSet[n] = struct{}{}
	}
	tenantOrder := mapKeyOrder(yamlMapKeyOrder(existing, "tenants"), tenantSet)
	if len(tenantOrder) == 0 {
		tenantOrder = mapKeyOrder(yamlMapKeyOrder(existing, "credentials"), tenantSet)
	}
	authOrder := mapKeyOrder(yamlMapKeyOrder(existing, "auths"), authSet)
	if len(authOrder) == 0 {
		authOrder = mapKeyOrder(yamlMapKeyOrder(existing, "credentials"), authSet)
	}

	tenantsNode := &yaml.Node{Kind: yaml.MappingNode}
	for _, name := range tenantOrder {
		var n yaml.Node
		if err := n.Encode(cfg.Tenants[name]); err != nil {
			return nil, err
		}
		tenantsNode.Content = append(tenantsNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: name},
			&n,
		)
	}
	authsNode := &yaml.Node{Kind: yaml.MappingNode}
	for _, name := range authOrder {
		a := cfg.Auths[name]
		entry := &yaml.Node{Kind: yaml.MappingNode}
		addStr := func(k, v string, force bool) {
			if !force && v == "" {
				return
			}
			entry.Content = append(entry.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k},
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v},
			)
		}
		addStr("type", a.Type, true)
		switch a.Type {
		case "command":
			addStr("runCommand", a.RunCommand, false)
		case "basic":
			addStr("username", a.Username, false)
			addStr("password", a.Password, false)
		case "oauth":
			addStr("clientId", a.ClientID, false)
			addStr("clientSecret", a.ClientSecret, false)
		}
		tenantsSeq := &yaml.Node{Kind: yaml.SequenceNode}
		for _, t := range a.Tenants {
			tenantsSeq.Content = append(tenantsSeq.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: t})
		}
		entry.Content = append(entry.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "tenants"},
			tenantsSeq,
		)
		if a.Access != "" && a.Access != AccessFull {
			addStr("access", a.Access, true)
		}
		if a.Surface != "" && a.Surface != SurfaceAny {
			addStr("surface", a.Surface, true)
		}
		authsNode.Content = append(authsNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: name},
			entry,
		)
	}

	root := &yaml.Node{Kind: yaml.MappingNode}
	root.Content = append(root.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "tenant"},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: cfg.Tenant},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "tenants"},
		tenantsNode,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "auths"},
		authsNode,
	)
	return yaml.Marshal(root)
}

// SaveConfig writes the config (new tenants/auths shape only).
func SaveConfig(cfg *Config) error {
	path, err := configPath()
	if err != nil {
		return fmt.Errorf("resolve config path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	existing, _ := os.ReadFile(path)
	data, err := marshalConfig(cfg, existing)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}
	return nil
}
