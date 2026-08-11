package buddy

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"buddy/internal/buddy/secrets"

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
	Access       string   `yaml:"access,omitempty"`   // readonly | full (default full)
	Surface      string   `yaml:"surface,omitempty"`  // platform | instance | any (default any)
	Channels     []string `yaml:"channels,omitempty"` // buddy clients cli|mcp|serve; empty = all
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

// ChannelAllowed reports whether this auth may be used on channel.
// Empty Channels means all buddy clients (cli/mcp/ide/notebook).
func (a AuthEntry) ChannelAllowed(channel string) bool {
	channel = normalizeBuddyChannel(channel)
	if channel == "" {
		return true
	}
	list := a.normalizedChannels()
	if len(list) == 0 {
		return true
	}
	for _, c := range list {
		if c == channel {
			return true
		}
	}
	return false
}

func (a AuthEntry) normalizedChannels() []string {
	if len(a.Channels) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, c := range a.Channels {
		c = normalizeBuddyChannel(c)
		if !isBuddyChannel(c) || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	sort.Strings(out)
	if len(out) == len(AllBuddyChannels()) {
		return nil // all = unrestricted
	}
	return out
}

// ParseAuthChannels parses a comma-separated channels list.
// "all" or empty clears restriction (nil). Invalid names return an error.
func ParseAuthChannels(s string) ([]string, error) {
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, "all") {
		return nil, nil
	}
	seen := map[string]bool{}
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		if p == "serve" {
			return nil, fmt.Errorf("channel %q was removed; use ide and/or notebook (or all)", p)
		}
		if !isBuddyChannel(p) {
			return nil, fmt.Errorf("unknown channel %q (want cli, mcp, ide, notebook, or all)", p)
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	sort.Strings(out)
	if len(out) == len(AllBuddyChannels()) {
		return nil, nil
	}
	return out, nil
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

// TenantSelection holds the global default tenant name.
type TenantSelection struct {
	Default string `yaml:"default"`
}

// UnmarshalYAML accepts either a string (legacy `tenant: name`) or a mapping with default only.
func (t *TenantSelection) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		return nil
	}
	if value.Kind == yaml.ScalarNode {
		t.Default = strings.TrimSpace(value.Value)
		return nil
	}
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("tenant: must be a string or mapping")
	}
	for i := 0; i+1 < len(value.Content); i += 2 {
		k := value.Content[i].Value
		v := strings.TrimSpace(value.Content[i+1].Value)
		switch k {
		case "default":
			t.Default = v
		case "cli", "mcp", "ide", "notebook":
			return fmt.Errorf("tenant.%s is no longer supported; use channels.%s.tenant", k, k)
		case "serve":
			return fmt.Errorf("tenant.serve is no longer supported; use channels.notebook.tenant and/or channels.ide.tenant")
		default:
			return fmt.Errorf("tenant: unknown key %q (want default only)", k)
		}
	}
	return nil
}

// Config holds buddy CLI configuration (tenants + auths + channel policies).
type Config struct {
	Tenant   TenantSelection          `yaml:"tenant"`
	Tenants  map[string]TenantConfig  `yaml:"tenants"`
	Auths    map[string]AuthEntry     `yaml:"auths"`
	Channels map[string]ChannelPolicy `yaml:"channels,omitempty"`

	// Legacy fields — populated only while unmarshaling old configs; cleared after migrate.
	Cred        string                      `yaml:"cred,omitempty"`
	Credentials map[string]CredentialConfig `yaml:"credentials,omitempty"`
	Defaults    map[string]string           `yaml:"defaults,omitempty"`    // migrated away
	AllowAuths  map[string][]string         `yaml:"allow_auths,omitempty"` // migrated away
}

// legacyConfigFile is used to detect and migrate older config shapes.
type legacyConfigFile struct {
	Cred        string                      `yaml:"cred"`
	Credentials map[string]CredentialConfig `yaml:"credentials"`
	Tenant      TenantSelection             `yaml:"tenant"`
	Tenants     map[string]TenantConfig     `yaml:"tenants"`
	Auths       map[string]AuthEntry        `yaml:"auths"`
	Channels    map[string]ChannelPolicy    `yaml:"channels"`
	Defaults    map[string]string           `yaml:"defaults"`
	AllowAuths  map[string][]string         `yaml:"allow_auths"`
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

// DefaultTenant returns the selected tenant for channel (cli|mcp|ide|notebook).
func (cfg *Config) DefaultTenant(channel string) string {
	if cfg == nil {
		return DefaultTenant
	}
	channel = normalizeBuddyChannel(channel)
	if p := cfg.ChannelPolicy(channel); strings.TrimSpace(p.Tenant) != "" {
		return strings.TrimSpace(p.Tenant)
	}
	if s := strings.TrimSpace(cfg.Tenant.Default); s != "" {
		return s
	}
	return DefaultTenant
}

// SetDefaultTenant sets tenant.default.
func (cfg *Config) SetDefaultTenant(name string) {
	if cfg == nil {
		return
	}
	cfg.Tenant.Default = strings.TrimSpace(name)
}

// AuthAllowed reports whether authName may be used on channel (via auth.channels).
func (cfg *Config) AuthAllowed(channel, authName string) bool {
	if cfg == nil || authName == "" {
		return true
	}
	a, ok := cfg.Auths[authName]
	if !ok {
		return true
	}
	return a.ChannelAllowed(channel)
}

// FilterAuths returns names allowed on channel (preserves order).
func (cfg *Config) FilterAuths(channel string, names []string) []string {
	if cfg == nil || len(names) == 0 {
		return names
	}
	out := make([]string, 0, len(names))
	for _, n := range names {
		if cfg.AuthAllowed(channel, n) {
			out = append(out, n)
		}
	}
	return out
}

// ValidateTenantSelectionAndChannels checks tenant:, channels:, and auth channels.
func (cfg *Config) ValidateTenantSelectionAndChannels() error {
	if cfg == nil {
		return nil
	}
	checkTenantName := func(where, name string) error {
		name = strings.TrimSpace(name)
		if name == "" {
			return nil
		}
		if _, ok := cfg.Tenants[name]; !ok {
			return fmt.Errorf("%s: unknown tenant %q", where, name)
		}
		return nil
	}
	if len(cfg.Tenants) > 0 && strings.TrimSpace(cfg.Tenant.Default) == "" {
		return fmt.Errorf("tenant.default is required when tenants are configured")
	}
	if err := checkTenantName("tenant.default", cfg.Tenant.Default); err != nil {
		return err
	}
	for ch, p := range cfg.Channels {
		ch = strings.ToLower(strings.TrimSpace(ch))
		if ch == "serve" {
			return fmt.Errorf("channels.serve is no longer supported; use channels.ide and/or channels.notebook")
		}
		if !isBuddyChannel(ch) {
			return fmt.Errorf("channels: unknown channel %q (want cli, mcp, ide, or notebook)", ch)
		}
		switch strings.ToLower(strings.TrimSpace(p.Access)) {
		case "", AccessFull, AccessReadonly:
		default:
			return fmt.Errorf("channels.%s.access must be %q or %q", ch, AccessFull, AccessReadonly)
		}
		if err := checkTenantName("channels."+ch+".tenant", p.Tenant); err != nil {
			return err
		}
		for _, tn := range p.Tenants {
			if err := checkTenantName("channels."+ch+".tenants", tn); err != nil {
				return err
			}
		}
		normalized, err := NormalizeDenyBuiltins(p.DenyBuiltins)
		if err != nil {
			return fmt.Errorf("channels.%s.deny_builtins: %w", ch, err)
		}
		p.DenyBuiltins = normalized
		cfg.Channels[ch] = p
	}
	for name, a := range cfg.Auths {
		for _, c := range a.Channels {
			c = strings.ToLower(strings.TrimSpace(c))
			if c == "" {
				continue
			}
			if c == "serve" {
				return fmt.Errorf("auth %q: channel %q was removed; use ide and/or notebook", name, c)
			}
			if !isBuddyChannel(c) {
				return fmt.Errorf("auth %q: unknown channel %q (want cli, mcp, ide, or notebook)", name, c)
			}
		}
	}
	return nil
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
	dir, err := secrets.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// LoadConfig loads config from the platform config dir (see secrets.ConfigDir).
// Older shapes (credentials:, string tenant:, defaults:, allow_auths:) migrate in-memory and rewrite on disk.
func LoadConfig() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, fmt.Errorf("resolve config path: %w", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{Tenant: TenantSelection{Default: DefaultTenant}}, nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	// Detect string-form tenant: before typed unmarshal (TenantSelection accepts both).
	stringTenant := tenantKeyIsString(data)

	var raw legacyConfigFile
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	cfg := &Config{
		Tenant:     raw.Tenant,
		Tenants:    raw.Tenants,
		Auths:      raw.Auths,
		Channels:   raw.Channels,
		Defaults:   raw.Defaults,
		AllowAuths: raw.AllowAuths,
	}
	var notes []string
	if len(raw.Credentials) > 0 && len(raw.Tenants) == 0 {
		cfg, err = migrateCredentialsToTenantsAuths(&raw)
		if err != nil {
			return nil, fmt.Errorf("migrate config %s: %w", path, err)
		}
		cfg.Defaults = raw.Defaults
		cfg.AllowAuths = raw.AllowAuths
		notes = append(notes, "credentials: to tenants:/auths:")
	}
	if migrated := migrateLegacyTenantDefaultsAllow(cfg, stringTenant); migrated {
		notes = append(notes, "tenant:/defaults:/allow_auths: to tenant: object + auth channels")
	}
	if strings.TrimSpace(cfg.Tenant.Default) == "" {
		if raw.Cred != "" && cfg.Tenants != nil {
			if _, ok := cfg.Tenants[raw.Cred]; ok {
				cfg.Tenant.Default = raw.Cred
			} else if a, ok := cfg.Auths[raw.Cred]; ok && len(a.Tenants) > 0 {
				cfg.Tenant.Default = a.Tenants[0]
			}
		}
	}
	if strings.TrimSpace(cfg.Tenant.Default) == "" {
		cfg.Tenant.Default = DefaultTenant
	}
	cfg.Cred = ""
	cfg.Credentials = nil
	cfg.Defaults = nil
	cfg.AllowAuths = nil
	normalizeAuthChannels(cfg)

	if err := cfg.ValidateTenantSelectionAndChannels(); err != nil {
		return nil, fmt.Errorf("config %s: %w", path, err)
	}

	if len(notes) > 0 {
		if err := SaveConfig(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "warning: migrated config in memory (%s) but could not rewrite %s: %v\n", strings.Join(notes, "; "), path, err)
			fmt.Fprintf(os.Stderr, "note: fix permissions and re-run buddy (or 'buddy cred use') so the new shape is persisted\n")
		} else {
			fmt.Fprintf(os.Stderr, "note: migrated config.yaml (%s); see 'buddy cred help'\n", strings.Join(notes, "; "))
		}
	}
	return cfg, nil
}

func tenantKeyIsString(data []byte) bool {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return false
	}
	root := &doc
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		root = doc.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "tenant" {
			return root.Content[i+1].Kind == yaml.ScalarNode
		}
	}
	return false
}

// migrateLegacyTenantDefaultsAllow folds string tenant / defaults / allow_auths into the new shape.
// Returns true when a rewrite is needed.
func migrateLegacyTenantDefaultsAllow(cfg *Config, stringTenant bool) bool {
	if cfg == nil {
		return false
	}
	changed := stringTenant || len(cfg.Defaults) > 0 || len(cfg.AllowAuths) > 0
	if !changed {
		return false
	}
	if len(cfg.Defaults) > 0 {
		if cfg.Channels == nil {
			cfg.Channels = map[string]ChannelPolicy{}
		}
		for _, ch := range []string{ChannelCLI, ChannelMCP, ChannelIDE, ChannelNotebook} {
			if v := strings.TrimSpace(cfg.Defaults[ch]); v != "" {
				p := cfg.Channels[ch]
				if strings.TrimSpace(p.Tenant) == "" {
					p.Tenant = v
					cfg.Channels[ch] = p
				}
			}
		}
		// Legacy defaults.serve → notebook + ide when unset.
		if v := strings.TrimSpace(cfg.Defaults["serve"]); v != "" {
			for _, ch := range []string{ChannelNotebook, ChannelIDE} {
				p := cfg.Channels[ch]
				if strings.TrimSpace(p.Tenant) == "" {
					p.Tenant = v
					cfg.Channels[ch] = p
				}
			}
		}
	}
	if len(cfg.AllowAuths) > 0 {
		applyAllowAuthsToChannels(cfg, cfg.AllowAuths)
	}
	cfg.Defaults = nil
	cfg.AllowAuths = nil
	return true
}

func applyAllowAuthsToChannels(cfg *Config, allow map[string][]string) {
	if cfg == nil || len(allow) == 0 {
		return
	}
	surfaces := AllBuddyChannels()
	allowSets := map[string]map[string]bool{}
	for _, s := range surfaces {
		list := allow[s]
		if s == ChannelNotebook || s == ChannelIDE {
			// Legacy allow_auths.serve applies to both notebook and ide.
			if len(list) == 0 {
				list = allow["serve"]
			}
		}
		if len(list) == 0 {
			continue // unrestricted for this surface
		}
		set := map[string]bool{}
		for _, n := range list {
			n = strings.TrimSpace(n)
			if n != "" {
				set[n] = true
			}
		}
		allowSets[s] = set
	}
	for name, a := range cfg.Auths {
		var kept []string
		for _, s := range surfaces {
			set, restricted := allowSets[s]
			if !restricted || set[name] {
				kept = append(kept, s)
			}
		}
		if len(kept) == len(surfaces) {
			a.Channels = nil
		} else {
			a.Channels = kept
		}
		cfg.Auths[name] = a
	}
}

func normalizeAuthChannels(cfg *Config) {
	if cfg == nil {
		return
	}
	for name, a := range cfg.Auths {
		a.Channels = a.normalizedChannels()
		cfg.Auths[name] = a
	}
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
		return &Config{Tenant: TenantSelection{Default: DefaultTenant}, Tenants: map[string]TenantConfig{}, Auths: map[string]AuthEntry{}}
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
			cfg.Tenant = TenantSelection{Default: a.Tenants[0]}
		} else if _, ok := cfg.Tenants[raw.Cred]; ok {
			cfg.Tenant = TenantSelection{Default: raw.Cred}
		}
	}
	if strings.TrimSpace(cfg.Tenant.Default) == "" {
		cfg.Tenant = TenantSelection{Default: DefaultTenant}
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
	cfg.Defaults = nil
	cfg.AllowAuths = nil
	if strings.TrimSpace(cfg.Tenant.Default) == "" {
		cfg.Tenant.Default = DefaultTenant
	}
	normalizeAuthChannels(cfg)
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
		if ch := a.normalizedChannels(); len(ch) > 0 {
			seq := &yaml.Node{Kind: yaml.SequenceNode}
			for _, c := range ch {
				seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: c})
			}
			entry.Content = append(entry.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "channels"},
				seq,
			)
		}
		authsNode.Content = append(authsNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: name},
			entry,
		)
	}

	tenantNode := &yaml.Node{Kind: yaml.MappingNode}
	tenantNode.Content = append(tenantNode.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "default"},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(cfg.Tenant.Default)},
	)

	root := &yaml.Node{Kind: yaml.MappingNode}
	root.Content = append(root.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "tenant"},
		tenantNode,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "tenants"},
		tenantsNode,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "auths"},
		authsNode,
	)

	if len(cfg.Channels) > 0 {
		chNames := make([]string, 0, len(cfg.Channels))
		for n := range cfg.Channels {
			chNames = append(chNames, n)
		}
		sort.Strings(chNames)
		channelsNode := &yaml.Node{Kind: yaml.MappingNode}
		for _, name := range chNames {
			p := cfg.Channels[name]
			entry := &yaml.Node{Kind: yaml.MappingNode}
			if a := p.EffectiveAccess(); a == AccessReadonly {
				entry.Content = append(entry.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "access"},
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: AccessReadonly},
				)
			}
			if t := strings.TrimSpace(p.Tenant); t != "" {
				entry.Content = append(entry.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "tenant"},
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: t},
				)
			}
			if len(p.Tenants) > 0 {
				seq := &yaml.Node{Kind: yaml.SequenceNode}
				for _, tn := range p.Tenants {
					seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: tn})
				}
				entry.Content = append(entry.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "tenants"},
					seq,
				)
			}
			if len(entry.Content) == 0 {
				continue
			}
			channelsNode.Content = append(channelsNode.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: name},
				entry,
			)
		}
		if len(channelsNode.Content) > 0 {
			root.Content = append(root.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "channels"},
				channelsNode,
			)
		}
	}

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
