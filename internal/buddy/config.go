package buddy

import (
	"bytes"
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

// Access levels for an auth entry.
const (
	AccessFull     = "full"
	AccessReadonly = "readonly"
)

// Surface values for a resolved session / tenant identity (not stored on auths).
const (
	SurfacePlatform = "platform"
	SurfaceInstance = "instance"
)

// Auth types. Platform oauth is type platform (not oauth + surface: platform).
// command remains an instance token helper.
const (
	AuthTypeCommand  = "command"
	AuthTypeBasic    = "basic"
	AuthTypeOAuth    = "oauth"    // instance OAuth client
	AuthTypePlatform = "platform" // Platform API Gateway OAuth client
)

// NormalizeAuthType lowercases and trims an auth type string.
func NormalizeAuthType(typ string) string {
	return strings.ToLower(strings.TrimSpace(typ))
}

// AuthTypeUsesClientCredentials reports oauth or platform (both use clientId + secret).
func AuthTypeUsesClientCredentials(typ string) bool {
	t := NormalizeAuthType(typ)
	return t == AuthTypeOAuth || t == AuthTypePlatform
}

// AuthMechanismType maps stored auth type onto the token layer (platform → oauth).
func AuthMechanismType(typ string) string {
	if NormalizeAuthType(typ) == AuthTypePlatform {
		return AuthTypeOAuth
	}
	return NormalizeAuthType(typ)
}

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

// TenantConfig is a named workplace (a buddy context): an organization (no
// region), a Platform environment (region + id), and/or a Jamf Pro instance.
// Account environment id (id / environmentId) and jamfpro.tenantId may both be
// set — they are different objects. Optional nested orgs: YAML expands into
// this flat map; child org: is derived from the tree when present.
type TenantConfig struct {
	Org           string   `yaml:"org,omitempty"`            // parent organization workplace (no region)
	Region        string   `yaml:"region,omitempty"`         // Platform API Gateway region (us|eu|apac|…); omit on org workplaces
	URL           string   `yaml:"url,omitempty"`            // Instance URL (legacy flat; prefer jamfpro.url)
	EnvironmentID string   `yaml:"environmentId,omitempty"`  // Account environment UUID (also YAML id:)
	TenantID      string   `yaml:"tenantId,omitempty"`       // Jamf Pro product tenant UUID (also jamfpro.tenantId)
	DenyBuiltins  []string `yaml:"deny_builtins,omitempty"`  // exact names, trailing-* globs, or "*"
	AllowBuiltins []string `yaml:"allow_builtins,omitempty"` // override this tenant's deny (allow follows deny)
}

// UnmarshalYAML accepts deny_builtins / allow_builtins as a sequence or a single scalar.
func (t *TenantConfig) UnmarshalYAML(value *yaml.Node) error {
	if t == nil {
		return fmt.Errorf("tenant config is nil")
	}
	if value == nil || value.Kind == yaml.ScalarNode && (value.Tag == "!!null" || strings.TrimSpace(value.Value) == "") {
		*t = TenantConfig{}
		return nil
	}
	var raw struct {
		Org           string `yaml:"org"`
		Organization  string `yaml:"organization"` // alias of org:
		Region        string `yaml:"region"`
		URL           string `yaml:"url"`
		ID            string `yaml:"id"` // GA: Account environment UUID
		EnvironmentID string `yaml:"environmentId"`
		TenantID      string `yaml:"tenantId"`
		JamfPro       struct {
			URL      string `yaml:"url"`
			TenantID string `yaml:"tenantId"`
		} `yaml:"jamfpro"`
		DenyBuiltins  yaml.Node `yaml:"deny_builtins"`
		AllowBuiltins yaml.Node `yaml:"allow_builtins"`
	}
	if err := value.Decode(&raw); err != nil {
		return err
	}
	deny, err := decodeBuiltinPatternList(&raw.DenyBuiltins)
	if err != nil {
		return fmt.Errorf("deny_builtins: %w", err)
	}
	allow, err := decodeBuiltinPatternList(&raw.AllowBuiltins)
	if err != nil {
		return fmt.Errorf("allow_builtins: %w", err)
	}
	t.Org = strings.TrimSpace(raw.Org)
	if t.Org == "" {
		t.Org = strings.TrimSpace(raw.Organization)
	}
	t.Region = raw.Region
	t.URL = strings.TrimSpace(raw.URL)
	if t.URL == "" {
		t.URL = strings.TrimSpace(raw.JamfPro.URL)
	}
	t.EnvironmentID = strings.TrimSpace(raw.EnvironmentID)
	if t.EnvironmentID == "" {
		t.EnvironmentID = strings.TrimSpace(raw.ID)
	}
	t.TenantID = strings.TrimSpace(raw.TenantID)
	if t.TenantID == "" {
		t.TenantID = strings.TrimSpace(raw.JamfPro.TenantID)
	}
	t.DenyBuiltins = deny
	t.AllowBuiltins = allow
	return nil
}

// AuthEntry is a secret-bearing principal linked to one or more tenants.
type AuthEntry struct {
	Type          string   `yaml:"type"`
	RunCommand    string   `yaml:"runCommand,omitempty"`
	Username      string   `yaml:"username,omitempty"`
	Password      string   `yaml:"password,omitempty"`
	ClientID      string   `yaml:"clientId,omitempty"`
	ClientSecret  string   `yaml:"clientSecret,omitempty"`
	Tenants       []string `yaml:"tenants"`                  // dual-read; prefer contexts: / jamfpro:
	Contexts      []string `yaml:"contexts,omitempty"`       // platform auth allowlist (environment-scoped)
	JamfPro       string   `yaml:"jamfpro,omitempty"`        // instance (and tenant-scoped platform) product link
	Scope         string   `yaml:"scope,omitempty"`          // platform: organization | environment | tenant
	Access        string   `yaml:"access,omitempty"`         // readonly | full (default full)
	Surface       string   `yaml:"surface,omitempty"`        // legacy; folded into type on load
	Channels      []string `yaml:"channels,omitempty"`       // buddy clients cli|mcp|ide; empty = all
	DenyBuiltins  []string `yaml:"deny_builtins,omitempty"`  // exact names, trailing-* globs, or "*"
	AllowBuiltins []string `yaml:"allow_builtins,omitempty"` // override this auth's deny (allow follows deny)
}

// UnmarshalYAML accepts deny_builtins / allow_builtins as a sequence or a single scalar.
func (a *AuthEntry) UnmarshalYAML(value *yaml.Node) error {
	if a == nil {
		return fmt.Errorf("auth entry is nil")
	}
	var raw struct {
		Type          string    `yaml:"type"`
		RunCommand    string    `yaml:"runCommand"`
		Username      string    `yaml:"username"`
		Password      string    `yaml:"password"`
		ClientID      string    `yaml:"clientId"`
		ClientSecret  string    `yaml:"clientSecret"`
		Tenants       []string  `yaml:"tenants"`
		Contexts      []string  `yaml:"contexts"`
		JamfPro       string    `yaml:"jamfpro"`
		Scope         string    `yaml:"scope"`
		Access        string    `yaml:"access"`
		Surface       string    `yaml:"surface"`
		Channels      []string  `yaml:"channels"`
		DenyBuiltins  yaml.Node `yaml:"deny_builtins"`
		AllowBuiltins yaml.Node `yaml:"allow_builtins"`
	}
	if err := value.Decode(&raw); err != nil {
		return err
	}
	deny, err := decodeBuiltinPatternList(&raw.DenyBuiltins)
	if err != nil {
		return fmt.Errorf("deny_builtins: %w", err)
	}
	allow, err := decodeBuiltinPatternList(&raw.AllowBuiltins)
	if err != nil {
		return fmt.Errorf("allow_builtins: %w", err)
	}
	a.Type = raw.Type
	a.RunCommand = raw.RunCommand
	a.Username = raw.Username
	a.Password = raw.Password
	a.ClientID = raw.ClientID
	a.ClientSecret = raw.ClientSecret
	a.Tenants = mergeAuthLinks(raw.Tenants, raw.Contexts, raw.JamfPro)
	a.Contexts = append([]string(nil), raw.Contexts...)
	a.JamfPro = strings.TrimSpace(raw.JamfPro)
	a.Scope = strings.TrimSpace(raw.Scope)
	a.Access = raw.Access
	a.Surface = raw.Surface
	a.Channels = raw.Channels
	a.DenyBuiltins = deny
	a.AllowBuiltins = allow
	return nil
}

func mergeAuthLinks(tenants, contexts []string, jamfpro string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}
	for _, t := range tenants {
		add(t)
	}
	for _, t := range contexts {
		add(t)
	}
	add(jamfpro)
	return out
}

// AuthConfig returns the secret/type block for this entry.
// type platform is oauth client-credentials against the gateway.
func (a AuthEntry) AuthConfig() AuthConfig {
	return AuthConfig{
		Type:         AuthMechanismType(a.Type),
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

// EffectiveSurface is instance unless type is platform (or legacy surface: platform).
func (a AuthEntry) EffectiveSurface() string {
	if NormalizeAuthType(a.Type) == AuthTypePlatform {
		return SurfacePlatform
	}
	if strings.EqualFold(strings.TrimSpace(a.Surface), SurfacePlatform) {
		return SurfacePlatform
	}
	return SurfaceInstance
}

// PlatformScopeOrganization / Environment / Tenant are Jamf Account integration scopes.
const (
	PlatformScopeOrganization = "organization"
	PlatformScopeEnvironment  = "environment"
	PlatformScopeTenant       = "tenant"
)

// EffectiveScope returns the platform auth scope (organization, environment, tenant).
// Omitted scope on type: platform defaults to environment (GA). Instance auths return "".
func (a AuthEntry) EffectiveScope() string {
	s := strings.ToLower(strings.TrimSpace(a.Scope))
	switch s {
	case "org", "organisation", PlatformScopeOrganization:
		return PlatformScopeOrganization
	case "env", PlatformScopeEnvironment:
		return PlatformScopeEnvironment
	case PlatformScopeTenant:
		return PlatformScopeTenant
	case "":
		if NormalizeAuthType(a.Type) == AuthTypePlatform {
			return PlatformScopeEnvironment
		}
		return ""
	default:
		return s
	}
}

// IsOrganizationScope reports an explicit organization (org / organisation) platform scope.
func IsOrganizationScope(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "org", "organisation", PlatformScopeOrganization:
		return true
	default:
		return false
	}
}

// UsesClientCredentials reports oauth or platform.
func (a AuthEntry) UsesClientCredentials() bool {
	return AuthTypeUsesClientCredentials(a.Type)
}

// LinkedTo reports whether this auth is attached to the named context.
func (a AuthEntry) LinkedTo(tenantName string) bool {
	if strings.TrimSpace(a.JamfPro) == tenantName {
		return true
	}
	for _, t := range a.Tenants {
		if t == tenantName {
			return true
		}
	}
	for _, t := range a.Contexts {
		if t == tenantName {
			return true
		}
	}
	return false
}

// ChannelAllowed reports whether this auth may be used on channel.
// Empty Channels means all buddy clients (cli/mcp/ide).
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
		if p == "serve" || p == "notebook" {
			return nil, fmt.Errorf("channel %q was removed; use ide (or all)", p)
		}
		if !isBuddyChannel(p) {
			return nil, fmt.Errorf("unknown channel %q (want cli, mcp, ide, or all)", p)
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
	Region        string     `yaml:"region,omitempty"`
	URL           string     `yaml:"url,omitempty"`
	EnvironmentID string     `yaml:"environmentId,omitempty"`
	TenantID      string     `yaml:"tenantId,omitempty"`
	Auth          AuthConfig `yaml:"auth"`
}

// EffectiveEnvironmentID returns the Platform environment id.
func (c CredentialConfig) EffectiveEnvironmentID() string {
	return strings.TrimSpace(c.EnvironmentID)
}

// EffectiveTenantID returns the Platform tenant id.
func (c CredentialConfig) EffectiveTenantID() string {
	if t := strings.TrimSpace(c.TenantID); t != "" {
		return t
	}
	return strings.TrimSpace(c.Auth.TenantID)
}

// PlatformScopeKey is the cache/caps identity for a platform session:
// environment id, tenant id, or "org" when neither is set.
func (c CredentialConfig) PlatformScopeKey() string {
	if e := c.EffectiveEnvironmentID(); e != "" {
		return e
	}
	if t := c.EffectiveTenantID(); t != "" {
		return t
	}
	if c.IsPlatform() {
		return "org"
	}
	return ""
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
	return c.EffectiveEnvironmentID() != "" ||
		c.EffectiveTenantID() != "" ||
		strings.TrimSpace(c.Region) != "" ||
		LooksLikePlatformGateway(c.URL)
}

// UsesPlatformAuth is true when OAuth should use gateway token endpoints.
func (c CredentialConfig) UsesPlatformAuth() bool {
	return c.IsPlatform()
}

// HasScopeID reports whether this tenant has an environment or tenant scope id.
func (t TenantConfig) HasScopeID() bool {
	return t.EffectiveEnvironmentID() != "" || t.EffectiveTenantID() != ""
}

func (t TenantConfig) hasNonGatewayURL() bool {
	u := strings.TrimSpace(t.URL)
	if u == "" || LooksLikePlatformGateway(u) || LooksLikeRetiredPlatformGateway(u) {
		return false
	}
	return true
}

// HasInstanceURL reports whether an instance (non-gateway) URL is configured.
// When a scope id is set without a region, url is treated as a gateway/proxy base
// (not an instance host) so test servers and custom proxies work.
func (t TenantConfig) HasInstanceURL() bool {
	if !t.hasNonGatewayURL() {
		return false
	}
	if t.HasScopeID() && t.EffectiveRegion() == "" {
		return false
	}
	return true
}

// HasPlatform reports whether this tenant has Platform gateway identity.
func (t TenantConfig) HasPlatform() bool {
	if t.IsOrganizationWorkplace() {
		return true
	}
	if t.HasScopeID() {
		return true
	}
	if LooksLikePlatformGateway(t.URL) {
		return true
	}
	if strings.TrimSpace(t.Region) != "" {
		return true
	}
	return false
}

// IsOrganizationWorkplace reports a named org context: no region, no environment
// or product tenant id, no instance URL. Jamf Account orgs sit above environments.
// Tokens go to us.api.jamfcloud.com with no environment/tenant header; YAML has
// no region. Parent org: on a child must not be set here.
func (t TenantConfig) IsOrganizationWorkplace() bool {
	if strings.TrimSpace(t.Org) != "" {
		return false
	}
	if t.HasScopeID() {
		return false
	}
	if strings.TrimSpace(t.Region) != "" {
		return false
	}
	if t.HasInstanceURL() {
		return false
	}
	u := strings.TrimRight(strings.TrimSpace(t.URL), "/")
	if u == "" || LooksLikeOrgPlatformGateway(u) {
		return true
	}
	return strings.EqualFold(RegionFromGatewayURL(u), OrgPlatformRegion)
}

// WorkplaceKind is the config-UI label for this context: organization,
// environment, tenant (Jamf product tenant), or instance.
func (t TenantConfig) WorkplaceKind() string {
	switch {
	case t.IsOrganizationWorkplace():
		return "organization"
	case strings.TrimSpace(t.EffectiveTenantID()) != "":
		return "tenant"
	case strings.TrimSpace(t.EffectiveEnvironmentID()) != "":
		return "environment"
	case t.HasPlatform() && !t.HasInstanceURL():
		return "environment"
	default:
		return "instance"
	}
}

// FormatWorkplaceKind is the cred-list / cred-status label for a context
// (product-tenant instead of the internal "tenant" kind).
func FormatWorkplaceKind(t TenantConfig) string {
	if t.WorkplaceKind() == "tenant" {
		return "product-tenant"
	}
	return t.WorkplaceKind()
}

// WorkplaceTreeRow is one context in org → environment → product-tenant order.
type WorkplaceTreeRow struct {
	Name        string // --context name
	Kind        string // organization | environment | product-tenant | instance
	Org         string // organization workplace (parent, or this name)
	Environment string // environment workplace (parent, or this name)
}

// WorkplaceTreeOrder returns context names org-first, then each environment,
// then that environment's product tenants, then the next environment. Instance
// workplaces (no Account tree) follow, sorted by name.
func WorkplaceTreeOrder(tenants map[string]TenantConfig) []string {
	rows := WorkplaceTree(tenants)
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Name)
	}
	return out
}

// WorkplaceTree returns display rows in tree order (no Account UUIDs).
func WorkplaceTree(tenants map[string]TenantConfig) []WorkplaceTreeRow {
	if len(tenants) == 0 {
		return nil
	}
	kindOf := func(name string) string {
		return tenants[name].WorkplaceKind()
	}
	var orgs, envs, product, inst, other []string
	for n := range tenants {
		switch kindOf(n) {
		case "organization":
			orgs = append(orgs, n)
		case "environment":
			envs = append(envs, n)
		case "tenant":
			product = append(product, n)
		case "instance":
			inst = append(inst, n)
		default:
			other = append(other, n)
		}
	}
	sort.Strings(orgs)
	sort.Strings(envs)
	sort.Strings(product)
	sort.Strings(inst)
	sort.Strings(other)

	parentEnv := map[string]string{}
	tenantsByEnv := map[string][]string{}
	var orphanProduct []string
	for _, n := range product {
		p := parentEnvironmentName(tenants, n, tenants[n])
		parentEnv[n] = p
		if p != "" {
			tenantsByEnv[p] = append(tenantsByEnv[p], n)
		} else {
			orphanProduct = append(orphanProduct, n)
		}
	}
	for env := range tenantsByEnv {
		sort.Strings(tenantsByEnv[env])
	}

	envsByOrg := map[string][]string{}
	var rootEnvs []string
	for _, n := range envs {
		org := strings.TrimSpace(tenants[n].Org)
		if org == "" {
			rootEnvs = append(rootEnvs, n)
			continue
		}
		envsByOrg[org] = append(envsByOrg[org], n)
	}
	for org := range envsByOrg {
		sort.Strings(envsByOrg[org])
	}

	orgSet := map[string]struct{}{}
	for _, n := range orgs {
		orgSet[n] = struct{}{}
	}
	for org := range envsByOrg {
		orgSet[org] = struct{}{}
	}
	for _, n := range orphanProduct {
		if o := strings.TrimSpace(tenants[n].Org); o != "" {
			orgSet[o] = struct{}{}
		}
	}
	orgNames := make([]string, 0, len(orgSet))
	for n := range orgSet {
		orgNames = append(orgNames, n)
	}
	sort.Strings(orgNames)

	emitted := map[string]bool{}
	var order []string
	emit := func(name string) {
		if name == "" || emitted[name] {
			return
		}
		if _, ok := tenants[name]; !ok {
			return
		}
		emitted[name] = true
		order = append(order, name)
	}
	for _, org := range orgNames {
		if t, ok := tenants[org]; ok && t.WorkplaceKind() == "organization" {
			emit(org)
		}
		for _, env := range envsByOrg[org] {
			emit(env)
			for _, n := range tenantsByEnv[env] {
				emit(n)
			}
		}
		for _, n := range orphanProduct {
			if strings.TrimSpace(tenants[n].Org) == org {
				emit(n)
			}
		}
	}
	for _, env := range rootEnvs {
		emit(env)
		for _, n := range tenantsByEnv[env] {
			emit(n)
		}
	}
	for _, n := range orphanProduct {
		emit(n)
	}
	for _, n := range inst {
		emit(n)
	}
	for _, n := range other {
		emit(n)
	}
	rest := make([]string, 0, len(tenants))
	for n := range tenants {
		if !emitted[n] {
			rest = append(rest, n)
		}
	}
	sort.Strings(rest)
	for _, n := range rest {
		emit(n)
	}

	rows := make([]WorkplaceTreeRow, 0, len(order))
	for _, name := range order {
		t := tenants[name]
		row := WorkplaceTreeRow{Name: name, Kind: FormatWorkplaceKind(t)}
		switch t.WorkplaceKind() {
		case "organization":
			row.Org = name
		case "environment":
			row.Org = strings.TrimSpace(t.Org)
			row.Environment = name
		case "tenant":
			row.Org = strings.TrimSpace(t.Org)
			row.Environment = parentEnv[name]
		default:
			row.Org = strings.TrimSpace(t.Org)
		}
		rows = append(rows, row)
	}
	return rows
}

func parentEnvironmentName(tenants map[string]TenantConfig, name string, t TenantConfig) string {
	if t.WorkplaceKind() != "tenant" {
		return ""
	}
	eid := t.EffectiveEnvironmentID()
	if eid == "" {
		return ""
	}
	org := strings.TrimSpace(t.Org)
	var matches []string
	for n, o := range tenants {
		if n == name || o.WorkplaceKind() != "environment" {
			continue
		}
		if o.EffectiveEnvironmentID() != eid {
			continue
		}
		if org != "" && strings.TrimSpace(o.Org) != "" && strings.TrimSpace(o.Org) != org {
			continue
		}
		matches = append(matches, n)
	}
	sort.Strings(matches)
	if len(matches) == 0 {
		return ""
	}
	return matches[0]
}

// ProductTenantsUnderEnvironment returns Jamf product-tenant workplace names
// whose parent environment is envName, in tree order.
func ProductTenantsUnderEnvironment(tenants map[string]TenantConfig, envName string) []string {
	envName = strings.TrimSpace(envName)
	if envName == "" || tenants[envName].WorkplaceKind() != "environment" {
		return nil
	}
	var out []string
	for _, name := range WorkplaceTreeOrder(tenants) {
		t := tenants[name]
		if parentEnvironmentName(tenants, name, t) == envName {
			out = append(out, name)
		}
	}
	return out
}

// OrderNamesByWorkplaceTree sorts names using WorkplaceTreeOrder (unknown names last).
func OrderNamesByWorkplaceTree(tenants map[string]TenantConfig, names []string) []string {
	rank := make(map[string]int, len(tenants))
	for i, n := range WorkplaceTreeOrder(tenants) {
		rank[n] = i
	}
	out := append([]string{}, names...)
	sort.SliceStable(out, func(i, j int) bool {
		ri, okI := rank[out[i]]
		rj, okJ := rank[out[j]]
		if okI && okJ {
			return ri < rj
		}
		if okI {
			return true
		}
		if okJ {
			return false
		}
		return out[i] < out[j]
	})
	return out
}

// FormatWorkplaceAccount is the remainder of a cred-list row: instance URL,
// or the Account header the workplace implies.
func FormatWorkplaceAccount(t TenantConfig) string {
	var parts []string
	switch t.WorkplaceKind() {
	case "organization":
		parts = append(parts, "no X-Environment-Id/X-Tenant-Id")
		if gw := t.GatewayBaseURL(); gw != "" {
			parts = append(parts, gw)
		}
	case "environment":
		if e := t.EffectiveEnvironmentID(); e != "" {
			parts = append(parts, "X-Environment-Id="+e)
		}
	case "tenant":
		if e := t.EffectiveEnvironmentID(); e != "" {
			parts = append(parts, "X-Environment-Id="+e)
		}
		if id := t.EffectiveTenantID(); id != "" {
			parts = append(parts, "X-Tenant-Id="+id)
		}
		if u := t.InstanceBaseURL(); u != "" {
			parts = append(parts, u)
		}
	default:
		if u := t.InstanceBaseURL(); u != "" {
			return u
		}
		return "-"
	}
	if o := strings.TrimSpace(t.Org); o != "" {
		parts = append(parts, "org="+o)
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, "  ")
}

// FormatAuthScope is "-" for instance auths, else organization|environment|tenant.
func FormatAuthScope(a AuthEntry) string {
	if a.EffectiveSurface() != SurfacePlatform {
		return "-"
	}
	s := a.EffectiveScope()
	if s == "" {
		return "-"
	}
	return s
}

// FormatAuthScopeDetail is a one-line reminder of which Account header that scope sends.
func FormatAuthScopeDetail(a AuthEntry) string {
	switch FormatAuthScope(a) {
	case PlatformScopeOrganization:
		return "no X-Environment-Id / X-Tenant-Id (link an organization workplace)"
	case PlatformScopeEnvironment:
		return "sends X-Environment-Id from the linked environment workplace"
	case PlatformScopeTenant:
		return "sends X-Tenant-Id from the linked product-tenant workplace"
	default:
		return ""
	}
}

// EffectiveEnvironmentID returns the platform environment id.
func (t TenantConfig) EffectiveEnvironmentID() string {
	return strings.TrimSpace(t.EnvironmentID)
}

// EffectiveTenantID returns the platform tenant id.
func (t TenantConfig) EffectiveTenantID() string {
	return strings.TrimSpace(t.TenantID)
}

// PlatformScopeKey is environment id, tenant id, or "org" for a platform tenant.
func (t TenantConfig) PlatformScopeKey() string {
	if e := t.EffectiveEnvironmentID(); e != "" {
		return e
	}
	if tid := t.EffectiveTenantID(); tid != "" {
		return tid
	}
	if t.HasPlatform() {
		return "org"
	}
	return ""
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
// when a scope id is set (proxy / test servers). Retired apigw hosts are never returned.
func (t TenantConfig) GatewayBaseURL() string {
	if LooksLikeRetiredPlatformGateway(t.URL) {
		return ""
	}
	if t.IsOrganizationWorkplace() || LooksLikeOrgPlatformGateway(t.URL) {
		return orgPlatformGatewayBase()
	}
	if r := t.EffectiveRegion(); r != "" {
		if norm, err := NormalizePlatformRegion(r); err == nil {
			def := PlatformGatewayURL(norm)
			// Explicit non-default url (proxy) wins when present.
			if u := strings.TrimRight(strings.TrimSpace(t.URL), "/"); u != "" && u != def && !t.HasInstanceURL() && !LooksLikeRetiredPlatformGateway(u) {
				return u
			}
			return def
		}
	}
	if LooksLikePlatformGateway(t.URL) {
		return strings.TrimRight(strings.TrimSpace(t.URL), "/")
	}
	if t.HasPlatform() {
		if u := strings.TrimRight(strings.TrimSpace(t.URL), "/"); u != "" && !t.HasInstanceURL() && !LooksLikeRetiredPlatformGateway(u) {
			return u
		}
		// Hybrid tenant: instance url + scope id needs region for gateway host.
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

// OrgPlatformRegion is the gateway region for organization-scoped Account
// clients. There is no separate org hostname (api.jamfcloud.com does not
// resolve); org traffic uses the US gateway with no environment/tenant header.
// Org workplaces still omit region in YAML.
const OrgPlatformRegion = "us"

// OrgPlatformGatewayURL is the organization-scope GA token/API host.
const OrgPlatformGatewayURL = "https://" + OrgPlatformRegion + ".api.jamfcloud.com"

// orgPlatformGatewayURLForTest, when set, replaces OrgPlatformGatewayURL in
// compose (httptest of Account APIs such as licensing).
var orgPlatformGatewayURLForTest string

func orgPlatformGatewayBase() string {
	if s := strings.TrimRight(strings.TrimSpace(orgPlatformGatewayURLForTest), "/"); s != "" {
		return s
	}
	return OrgPlatformGatewayURL
}

// SetOrgPlatformGatewayURLForTest redirects organization compose to u (usually
// an httptest server). Restore with the returned func.
func SetOrgPlatformGatewayURLForTest(u string) func() {
	prev := orgPlatformGatewayURLForTest
	orgPlatformGatewayURLForTest = u
	return func() { orgPlatformGatewayURLForTest = prev }
}

// PlatformGatewayURL builds the regional GA gateway base URL for region.
func PlatformGatewayURL(region string) string {
	return "https://" + strings.ToLower(strings.TrimSpace(region)) + ".api.jamfcloud.com"
}

func gatewayHost(rawURL string) string {
	u := strings.ToLower(strings.TrimSpace(rawURL))
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	if i := strings.IndexByte(u, '/'); i >= 0 {
		u = u[:i]
	}
	return u
}

// RegionFromGatewayURL returns the region subdomain for a GA gateway URL
// (https://{region}.api.jamfcloud.com), or "". A sample host api.jamfcloud.com
// (no subdomain) does not resolve in DNS; it is ignored here (paste maps it
// to organization-against-US separately).
func RegionFromGatewayURL(rawURL string) string {
	h := gatewayHost(rawURL)
	if h == "api.jamfcloud.com" {
		return ""
	}
	const suffix = ".api.jamfcloud.com"
	if !strings.HasSuffix(h, suffix) {
		return ""
	}
	return strings.TrimSuffix(h, suffix)
}

// LooksLikeOrgPlatformGateway reports the non-resolving host that appears in
// some Jamf curl samples. Organization traffic uses us.api.jamfcloud.com with
// no scope header; that US host is also used for environment clients.
func LooksLikeOrgPlatformGateway(rawURL string) bool {
	return gatewayHost(rawURL) == "api.jamfcloud.com"
}

// LooksLikeRetiredPlatformGateway reports the public-beta host (*.apigw.jamf.com).
func LooksLikeRetiredPlatformGateway(rawURL string) bool {
	h := gatewayHost(rawURL)
	return h == "apigw.jamf.com" || strings.HasSuffix(h, ".apigw.jamf.com")
}

// LooksLikePlatformGateway reports whether url is a GA Jamf Platform API Gateway host.
func LooksLikePlatformGateway(rawURL string) bool {
	return RegionFromGatewayURL(rawURL) != "" || LooksLikeOrgPlatformGateway(rawURL)
}

func retiredPlatformGatewayError(rawURL string) error {
	region := "us"
	h := gatewayHost(rawURL)
	const suffix = ".apigw.jamf.com"
	if strings.HasSuffix(h, suffix) {
		if r := strings.TrimSuffix(h, suffix); r != "" {
			region = r
		}
	}
	shown := strings.TrimRight(strings.TrimSpace(rawURL), "/")
	if shown == "" {
		shown = h
	}
	return fmt.Errorf("%s is the retired Jamf Platform gateway and does not serve the GA API paths. Set url: %s (or region: %s); see 'buddy cred help'", shown, PlatformGatewayURL(region), region)
}

func bothPlatformScopeIDsError(environmentID, tenantID string) error {
	return fmt.Errorf("environmentId %q and tenantId %q are mutually exclusive; an API integration is created at one level", strings.TrimSpace(environmentID), strings.TrimSpace(tenantID))
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
	if LooksLikeRetiredPlatformGateway(t.URL) {
		return retiredPlatformGatewayError(t.URL)
	}
	hasInstance := t.HasInstanceURL()
	hasPlatform := t.HasPlatform()
	if t.IsOrganizationWorkplace() {
		if _, _, err := normalizeDenyAllowLists(t.DenyBuiltins, t.AllowBuiltins); err != nil {
			return err
		}
		return nil
	}
	if !hasInstance && !hasPlatform {
		return fmt.Errorf("tenant requires url (instance), region (environment/tenant Platform), or an organization workplace (no region); see 'buddy cred help'")
	}
	if hasPlatform {
		if t.GatewayBaseURL() == "" && t.EffectiveRegion() == "" && strings.TrimSpace(t.URL) == "" {
			return fmt.Errorf("Platform API Gateway tenants require region (e.g. us) or a gateway url")
		}
		if t.GatewayBaseURL() == "" && t.HasInstanceURL() && t.EffectiveRegion() == "" {
			return fmt.Errorf("Platform API Gateway tenants require region (e.g. us) when url is an instance host")
		}
		if t.HasInstanceURL() && !t.HasScopeID() {
			return fmt.Errorf("hybrid instance+platform tenants require environmentId (preferred) or tenantId; see 'buddy cred help'")
		}
	}
	if _, _, err := normalizeDenyAllowLists(t.DenyBuiltins, t.AllowBuiltins); err != nil {
		return err
	}
	return nil
}

// Validate checks auth entry fields.
func (a AuthEntry) Validate() error {
	switch NormalizeAuthType(a.Type) {
	case AuthTypeCommand, AuthTypeBasic, AuthTypeOAuth, AuthTypePlatform:
	case "":
		return fmt.Errorf("auth type is required (command, basic, oauth, or platform)")
	default:
		return fmt.Errorf("unknown auth type %q (supported: command, basic, oauth, platform)", a.Type)
	}
	if NormalizeAuthType(a.Type) == AuthTypePlatform {
		switch a.EffectiveScope() {
		case PlatformScopeOrganization, PlatformScopeEnvironment, PlatformScopeTenant:
		default:
			return fmt.Errorf("platform auth scope must be organization, environment, or tenant (got %q)", a.Scope)
		}
	}
	if len(a.Tenants) == 0 && a.EffectiveScope() != PlatformScopeOrganization {
		return fmt.Errorf("auth must link a context (jamfpro:, contexts:, or tenants:)")
	}
	switch a.EffectiveAccess() {
	case AccessFull, AccessReadonly:
	default:
		return fmt.Errorf("access must be %q or %q", AccessFull, AccessReadonly)
	}
	if surf := strings.ToLower(strings.TrimSpace(a.Surface)); surf != "" && surf != SurfaceInstance && surf != SurfacePlatform && surf != "any" {
		return fmt.Errorf("legacy auth surface must be %q or %q (got %q); prefer type: platform", SurfaceInstance, SurfacePlatform, a.Surface)
	}
	if a.EffectiveSurface() == SurfacePlatform && !a.UsesClientCredentials() {
		return fmt.Errorf("platform surface requires type %q (got %q)", AuthTypePlatform, a.Type)
	}
	if _, _, err := normalizeDenyAllowLists(a.DenyBuiltins, a.AllowBuiltins); err != nil {
		return err
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
			return fmt.Errorf("url is required (or set region for Platform API Gateway); see 'buddy cred help'")
		}
		if LooksLikeRetiredPlatformGateway(c.URL) {
			return retiredPlatformGatewayError(c.URL)
		}
		return nil
	}
	if LooksLikeRetiredPlatformGateway(c.URL) {
		return retiredPlatformGatewayError(c.URL)
	}
	if eid, tid := c.EffectiveEnvironmentID(), c.EffectiveTenantID(); eid != "" && tid != "" {
		return bothPlatformScopeIDsError(eid, tid)
	}
	if c.Auth.Type != AuthTypeOAuth {
		return fmt.Errorf("Platform API Gateway credentials require oauth client credentials (got auth.type=%q); basic and command are instance-only", c.Auth.Type)
	}
	if c.EffectiveBaseURL() == "" {
		return fmt.Errorf("Platform API Gateway credentials require region (e.g. us) or url (e.g. https://us.api.jamfcloud.com)")
	}
	return nil
}

// TenantSelection holds the global default tenant name.
type TenantSelection struct {
	Default string `yaml:"default"`
}

// UnmarshalYAML accepts a mapping with default only (`context:` / dual-read `tenant:`).
func (t *TenantSelection) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		return nil
	}
	if value.Kind == yaml.ScalarNode {
		return fmt.Errorf("context/tenant: must be a mapping with default: (not a bare string)")
	}
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("context/tenant: must be a mapping with default:")
	}
	for i := 0; i+1 < len(value.Content); i += 2 {
		k := value.Content[i].Value
		v := strings.TrimSpace(value.Content[i+1].Value)
		switch k {
		case "default":
			t.Default = v
		case "cli", "mcp", "ide":
			return fmt.Errorf("context/tenant.%s is no longer supported; use channels.%s.context (or channels.%s.tenant)", k, k, k)
		case "notebook", "serve":
			return fmt.Errorf("context/tenant.%s is no longer supported; use channels.ide.context", k)
		default:
			return fmt.Errorf("context/tenant: unknown key %q (want default only)", k)
		}
	}
	return nil
}

// EnvClassicInstanceOnly overrides features.classic_instance_only when set
// (1/true/yes/on or 0/false/no/off). Omit to use YAML, then default false.
const EnvClassicInstanceOnly = "BUDDY_CLASSIC_INSTANCE_ONLY"

// FeatureFlags are optional buddy behavior toggles (omitted keys use defaults).
type FeatureFlags struct {
	// ClassicInstanceOnly routes Classic /JSSResource calls to an instance auth.
	// Default false: Classic follows a platform pin / auto-pick
	// (/proclassic/… with /JSSResource stripped). Set true
	// (or BUDDY_CLASSIC_INSTANCE_ONLY=true) to force Classic onto instance.
	// Gateway 403 then means the OAuth client lacks Classic scopes.
	//
	// This flag does not apply to ACS (/api/config-profiles/…) or the
	// undocumented Pro policy list (GET /api/v1/policies). Those are always
	// instance — they are not published on the Platform gateway.
	ClassicInstanceOnly *bool `yaml:"classic_instance_only,omitempty"`

	// UseAgent takes workplaces and auths from the macOS Buddy Agent instead of
	// (strictly: underneath) this file. The Agent owns those secrets, so buddy
	// never opens its own vault for them. Default false; BUDDY_USE_AGENT wins.
	UseAgent *bool `yaml:"use_agent,omitempty"`

	// ForceAgent requires the Buddy Agent and ignores workplaces, auths,
	// channels, orgs, and other features from config.yaml. Managed preferences
	// still overlay the Agent document (user file does not). Default false;
	// BUDDY_FORCE_AGENT wins. MDM force_agent: true cannot be turned off in YAML.
	ForceAgent *bool `yaml:"force_agent,omitempty"`
}

// ClassicInstanceOnly reports whether Classic API routing must use an
// instance auth. Env wins over YAML; omitted means false.
func (cfg *Config) ClassicInstanceOnly() bool {
	if v, ok := parseBoolishEnv(EnvClassicInstanceOnly); ok {
		return v
	}
	if cfg != nil && cfg.Features.ClassicInstanceOnly != nil {
		return *cfg.Features.ClassicInstanceOnly
	}
	return false
}

// EnvUseAgent overrides features.use_agent (1/true/yes/on or 0/false/no/off).
const EnvUseAgent = "BUDDY_USE_AGENT"

// EnvForceAgent overrides features.force_agent (1/true/yes/on or 0/false/no/off).
const EnvForceAgent = "BUDDY_FORCE_AGENT"

// ForceAgent reports whether the Buddy Agent is required and config.yaml
// (except this flag) is ignored. Env wins; MDM true is sticky over user YAML.
func (cfg *Config) ForceAgent() bool {
	if v, ok := parseBoolishEnv(EnvForceAgent); ok {
		return v
	}
	if cfg != nil && cfg.Features.ForceAgent != nil {
		return *cfg.Features.ForceAgent
	}
	return false
}

// UseAgent reports whether workplaces, auths, and secrets come from the macOS
// Buddy Agent. ForceAgent implies true. Env wins over YAML; omitted means false
// unless this load already attached an Agent layer (empty config.yaml on a
// running Agent).
func (cfg *Config) UseAgent() bool {
	if cfg.ForceAgent() {
		return true
	}
	if v, ok := parseBoolishEnv(EnvUseAgent); ok {
		return v
	}
	if cfg != nil && cfg.Features.UseAgent != nil {
		return *cfg.Features.UseAgent
	}
	return cfg != nil && cfg.agent != nil
}

func parseBoolishEnv(key string) (value, ok bool) {
	s := strings.TrimSpace(os.Getenv(key))
	if s == "" {
		return false, false
	}
	switch strings.ToLower(s) {
	case "1", "true", "yes", "on":
		return true, true
	case "0", "false", "no", "off":
		return false, true
	default:
		return false, false
	}
}

func boolPtr(v bool) *bool { return &v }

func cloneBoolPtr(p *bool) *bool {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func boolPtrEqual(a, b *bool) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// Config holds buddy CLI configuration (tenants + auths + channel policies).
type Config struct {
	Tenant   TenantSelection          `yaml:"tenant"`
	Tenants  map[string]TenantConfig  `yaml:"tenants"`
	Orgs     map[string]OrgConfig     `yaml:"orgs,omitempty"` // optional Jamf tree; expands into Tenants
	Auths    map[string]AuthEntry     `yaml:"auths"`
	Channels map[string]ChannelPolicy `yaml:"channels,omitempty"`
	Features FeatureFlags             `yaml:"features,omitempty"`

	// user is the YAML file as loaded (after migrate). mdm is managed prefs and
	// agent is the Buddy Agent's document (identity/policy; not features.use_agent).
	// All three are
	// omitted from YAML; SaveConfig writes only the user overlay when either
	// base layer is present, so neither gets baked into config.yaml.
	user  *Config
	mdm   *Config
	agent *Config
}

// UnmarshalYAML dual-reads context:/contexts: (canonical) and tenant:/tenants:.
func (c *Config) UnmarshalYAML(value *yaml.Node) error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}
	var raw struct {
		Tenant   TenantSelection          `yaml:"tenant"`
		Context  TenantSelection          `yaml:"context"`
		Tenants  map[string]TenantConfig  `yaml:"tenants"`
		Contexts map[string]TenantConfig  `yaml:"contexts"`
		Orgs     map[string]OrgConfig     `yaml:"orgs"`
		Auths    map[string]AuthEntry     `yaml:"auths"`
		Channels map[string]ChannelPolicy `yaml:"channels"`
		Features FeatureFlags             `yaml:"features"`
	}
	if err := value.Decode(&raw); err != nil {
		return err
	}
	c.Tenant = raw.Tenant
	if d := strings.TrimSpace(raw.Context.Default); d != "" {
		c.Tenant.Default = d
	}
	c.Tenants = mergeWorkplaceMaps(raw.Tenants, raw.Contexts)
	c.Orgs = raw.Orgs
	c.Auths = raw.Auths
	c.Channels = raw.Channels
	c.Features = raw.Features
	return expandOrgs(c)
}

func mergeWorkplaceMaps(legacy, contexts map[string]TenantConfig) map[string]TenantConfig {
	if len(legacy) == 0 && len(contexts) == 0 {
		return nil
	}
	out := make(map[string]TenantConfig, len(legacy)+len(contexts))
	for k, v := range legacy {
		out[k] = v
	}
	for k, v := range contexts {
		out[k] = v
	}
	return out
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
// and desired surface (platform or instance).
func ComposeCredential(tenant TenantConfig, auth AuthEntry, surface string) CredentialConfig {
	surface = strings.ToLower(strings.TrimSpace(surface))
	if surface == "" {
		surface = auth.EffectiveSurface()
	}
	cred := CredentialConfig{Auth: auth.AuthConfig()}
	switch surface {
	case SurfacePlatform:
		switch composePlatformScope(auth, tenant) {
		case PlatformScopeOrganization:
			cred.URL = orgPlatformGatewayBase()
			cred.Region = OrgPlatformRegion
		case PlatformScopeTenant:
			cred.TenantID = tenant.EffectiveTenantID()
			cred.Region = tenant.EffectiveRegion()
			if gw := tenant.GatewayBaseURL(); gw != "" {
				cred.URL = gw
			}
		default:
			cred.EnvironmentID = tenant.EffectiveEnvironmentID()
			cred.Region = tenant.EffectiveRegion()
			if gw := tenant.GatewayBaseURL(); gw != "" {
				cred.URL = gw
			}
		}
	default: // instance
		cred.URL = tenant.InstanceBaseURL()
		if cred.URL == "" && !LooksLikePlatformGateway(tenant.URL) {
			cred.URL = strings.TrimRight(strings.TrimSpace(tenant.URL), "/")
		}
	}
	return cred
}

// composePlatformScope picks which Account header a platform auth sends.
// Explicit auth.scope wins. Omitted scope infers from the context (beta YAML
// that only had tenantId still sends X-Tenant-Id).
func composePlatformScope(auth AuthEntry, tenant TenantConfig) string {
	if strings.TrimSpace(auth.Scope) != "" {
		return auth.EffectiveScope()
	}
	if tenant.EffectiveEnvironmentID() != "" {
		return PlatformScopeEnvironment
	}
	if tenant.EffectiveTenantID() != "" {
		return PlatformScopeTenant
	}
	return PlatformScopeOrganization
}

// ResolveTenant looks up a named tenant.
func (cfg *Config) ResolveTenant(name string) (TenantConfig, error) {
	if cfg.Tenants == nil {
		return TenantConfig{}, fmt.Errorf("no tenants configured — on macOS add a workplace in Buddy Agent; otherwise 'buddy cred add' or 'buddy setup' (see 'buddy setup help')")
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

// AuthsForTenant returns auth names that may be used on tenantName (unsorted).
// Organization-scoped auths only appear on organization workplaces.
func (cfg *Config) AuthsForTenant(tenantName string) []string {
	if cfg == nil {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		out = append(out, name)
	}
	tenant, hasTenant := cfg.Tenants[tenantName]
	for name, a := range cfg.Auths {
		if !a.LinkedTo(tenantName) {
			continue
		}
		if hasTenant && !AuthFitsWorkplace(a, tenant) {
			continue
		}
		add(name)
	}
	return out
}

// AuthAvailableOnTenant reports whether auth may be used with tenantName:
// a direct jamfpro:/contexts: link, and organization-scoped auths only on
// an organization workplace.
func (cfg *Config) AuthAvailableOnTenant(auth AuthEntry, tenantName string) bool {
	if !auth.LinkedTo(tenantName) {
		return false
	}
	if cfg == nil {
		return auth.EffectiveScope() != PlatformScopeOrganization
	}
	t, ok := cfg.Tenants[tenantName]
	if !ok {
		return auth.EffectiveScope() != PlatformScopeOrganization
	}
	return AuthFitsWorkplace(auth, t)
}

// DefaultTenant returns the selected tenant for channel (cli|mcp|ide).
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
			return fmt.Errorf("%s: unknown context %q", where, name)
		}
		return nil
	}
	if len(cfg.Tenants) > 0 && strings.TrimSpace(cfg.Tenant.Default) == "" {
		return fmt.Errorf("context.default is required when contexts are configured")
	}
	if err := checkTenantName("context.default", cfg.Tenant.Default); err != nil {
		return err
	}
	for ch, p := range cfg.Channels {
		ch = strings.ToLower(strings.TrimSpace(ch))
		if ch == "serve" || ch == "notebook" {
			return fmt.Errorf("channels.%s is no longer supported; use channels.ide", ch)
		}
		if !isBuddyChannel(ch) {
			return fmt.Errorf("channels: unknown channel %q (want cli, mcp, or ide)", ch)
		}
		switch strings.ToLower(strings.TrimSpace(p.Access)) {
		case "", AccessFull, AccessReadonly:
		default:
			return fmt.Errorf("channels.%s.access must be %q or %q", ch, AccessFull, AccessReadonly)
		}
		if err := checkTenantName("channels."+ch+".context", p.Tenant); err != nil {
			return err
		}
		for _, tn := range p.Tenants {
			if err := checkTenantName("channels."+ch+".contexts", tn); err != nil {
				return err
			}
		}
		normalized, err := NormalizeDenyBuiltins(p.DenyBuiltins)
		if err != nil {
			return fmt.Errorf("channels.%s.deny_builtins: %w", ch, err)
		}
		p.DenyBuiltins = normalized
		allow, err := NormalizeAllowBuiltins(p.AllowBuiltins)
		if err != nil {
			return fmt.Errorf("channels.%s.allow_builtins: %w", ch, err)
		}
		p.AllowBuiltins = allow
		cfg.Channels[ch] = p
	}
	for name, t := range cfg.Tenants {
		deny, allow, err := normalizeDenyAllowLists(t.DenyBuiltins, t.AllowBuiltins)
		if err != nil {
			return fmt.Errorf("contexts.%s: %w", name, err)
		}
		t.DenyBuiltins = deny
		t.AllowBuiltins = allow
		cfg.Tenants[name] = t
		if err := cfg.validateContextOrg(name, t); err != nil {
			return err
		}
	}
	for name, a := range cfg.Auths {
		deny, allow, err := normalizeDenyAllowLists(a.DenyBuiltins, a.AllowBuiltins)
		if err != nil {
			return fmt.Errorf("auths.%s: %w", name, err)
		}
		a.DenyBuiltins = deny
		a.AllowBuiltins = allow
		cfg.Auths[name] = a
		for _, c := range a.Channels {
			c = strings.ToLower(strings.TrimSpace(c))
			if c == "" {
				continue
			}
			if c == "serve" || c == "notebook" {
				return fmt.Errorf("auth %q: channel %q was removed; use ide", name, c)
			}
			if !isBuddyChannel(c) {
				return fmt.Errorf("auth %q: unknown channel %q (want cli, mcp, or ide)", name, c)
			}
		}
	}
	return nil
}

func (cfg *Config) validateContextOrg(name string, t TenantConfig) error {
	parent := strings.TrimSpace(t.Org)
	if parent == "" {
		return nil
	}
	if parent == name {
		return fmt.Errorf("contexts.%s.org cannot be itself", name)
	}
	if cfg == nil || cfg.Tenants == nil {
		return fmt.Errorf("contexts.%s.org: unknown context %q", name, parent)
	}
	p, ok := cfg.Tenants[parent]
	if !ok {
		return fmt.Errorf("contexts.%s.org: unknown context %q", name, parent)
	}
	if !p.IsOrganizationWorkplace() {
		return fmt.Errorf("contexts.%s.org %q must be an organization workplace (no region, no environment id)", name, parent)
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

// ConfigNeedsSetup reports whether cfg has no tenants after LoadConfig merge
// (missing config.yaml, empty file, or tenants map absent). False when MDM
// managed preferences or a discovered Buddy Agent already supply tenants.
func ConfigNeedsSetup(cfg *Config) bool {
	return cfg == nil || len(cfg.Tenants) == 0
}

// ConfigFileExists reports whether ~/.config/buddy/config.yaml (or XDG equivalent) exists.
func ConfigFileExists() (bool, error) {
	path, err := configPath()
	if err != nil {
		return false, err
	}
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return !fi.IsDir(), nil
}

// LoadConfig loads config from the platform config dir (see secrets.ConfigDir).
// Requires contexts:/auths: (dual-read tenants:). Legacy credentials:/defaults:/allow_auths:
// and a bare string tenant:/context: are rejected. Auth surface: still folds into type on load.
// macOS managed preferences (and BUDDY_MANAGED_PLIST) overlay as org defaults; user YAML wins per name.
func LoadConfig() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, fmt.Errorf("resolve config path: %w", err)
	}
	var user *Config
	var notes []string
	userExists := false
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read config %s: %w", path, err)
		}
		user = &Config{}
	} else {
		userExists = true
		var err2 error
		user, notes, err2 = parseAndMigrateUserYAML(data)
		if err2 != nil {
			return nil, wrapConfigParseError(path, err2)
		}
	}

	mdm, hadSecrets, err := loadManagedConfig()
	if err != nil {
		return nil, err
	}
	if hadSecrets {
		fmt.Fprintf(os.Stderr, "warning: managed preferences include inline secrets; the plist is typically world-readable — prefer 'buddy setup secrets' (or 'buddy cred set secret')\n")
	}

	// features.use_agent / force_agent: the Buddy Agent layer sits beneath
	// managed prefs. force_agent skips the user file (MDM still applies).
	// Dial when asked, or when config.yaml is empty/missing on a platform
	// that has an Agent — do not persist the flag on discover.
	var agentCfg *Config
	forceAgent := layersForceAgent(mdm, user)
	if forceAgent || overlayConfig(mdm, user).UseAgent() {
		agentCfg, err = loadAgentConfig()
		if err != nil {
			return nil, err
		}
	} else if shouldDiscoverAgent(user, mdm) {
		agentCfg, err = tryLoadAgentConfig()
		if err != nil {
			return nil, err
		}
	}

	workplaceFallback := mdm
	if !forceAgent {
		workplaceFallback = overlayConfig(mdm, user)
	}
	if agentCfg != nil && !agentHasWorkplaces(agentCfg) &&
		!agentHasWorkplaces(workplaceFallback) {
		return nil, agentNeedsConfigError()
	}

	var merged *Config
	if forceAgent {
		merged = overlayConfig(agentCfg, mdm)
		if merged == nil {
			merged = &Config{}
		}
		if (merged.Features.ForceAgent == nil || !*merged.Features.ForceAgent) &&
			user != nil && user.Features.ForceAgent != nil && *user.Features.ForceAgent {
			merged.Features.ForceAgent = boolPtr(true)
		}
	} else {
		merged = overlayConfig(overlayConfig(agentCfg, mdm), user)
	}
	if merged == nil {
		merged = &Config{}
	}
	merged.user = cloneConfig(user)
	merged.mdm = cloneConfig(mdm)
	merged.agent = cloneConfig(agentCfg)
	if err := expandOrgs(merged); err != nil {
		src := path
		if !userExists && mdm != nil {
			src = "managed preferences"
		}
		return nil, fmt.Errorf("config %s: %w", src, err)
	}
	migrateAuthSurfaceIntoType(merged)

	if strings.TrimSpace(merged.Tenant.Default) == "" {
		if n := firstSortedTenantName(merged); n != "" {
			merged.Tenant.Default = n
		} else if len(merged.Tenants) == 0 && len(merged.Auths) == 0 && len(merged.Channels) == 0 {
			// Placeholder only on a truly empty file; inventing "default" when
			// the Agent served policy-only channels would fail validation.
			merged.Tenant.Default = DefaultTenant
		}
	}
	normalizeAuthChannels(merged)

	if len(merged.Tenants) > 0 || len(merged.Auths) > 0 || len(merged.Channels) > 0 {
		src := path
		if !userExists && mdm != nil {
			src = "managed preferences"
		}
		if err := merged.ValidateTenantSelectionAndChannels(); err != nil {
			return nil, fmt.Errorf("config %s: %w", src, err)
		}
	}

	if userExists && len(notes) > 0 {
		if err := SaveConfig(merged); err != nil {
			fmt.Fprintf(os.Stderr, "warning: migrated config in memory (%s) but could not rewrite %s: %v\n", strings.Join(notes, "; "), path, err)
			fmt.Fprintf(os.Stderr, "note: fix permissions and re-run buddy (or 'buddy cred use') so the new shape is persisted\n")
		} else {
			fmt.Fprintf(os.Stderr, "note: migrated config.yaml (%s); see 'buddy cred help'\n", strings.Join(notes, "; "))
		}
	}
	return merged, nil
}

func lastGoodConfigPath(path string) string {
	return path + ".ok"
}

func rewriteLeadingYAMLTabs(data []byte) []byte {
	if !bytes.Contains(data, []byte{'\t'}) {
		return data
	}
	lines := bytes.Split(data, []byte("\n"))
	changed := false
	for i, line := range lines {
		n := 0
		for n < len(line) && line[n] == '\t' {
			n++
		}
		if n == 0 {
			continue
		}
		out := make([]byte, 0, len(line)-n+n*2)
		for j := 0; j < n; j++ {
			out = append(out, ' ', ' ')
		}
		out = append(out, line[n:]...)
		lines[i] = out
		changed = true
	}
	if !changed {
		return data
	}
	return bytes.Join(lines, []byte("\n"))
}

func wrapConfigParseError(path string, err error) error {
	last := lastGoodConfigPath(path)
	if fi, statErr := os.Stat(last); statErr == nil && !fi.IsDir() {
		return fmt.Errorf("parse config %s: %w (last good: %s — run 'buddy cred restore-config')", path, err, last)
	}
	return fmt.Errorf("parse config %s: %w", path, err)
}

// IsConfigParseError reports a YAML/parse failure of config.yaml (not validation of a loaded file).
func IsConfigParseError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "parse config ") || strings.Contains(s, "yaml:")
}

// RestoreLastGoodConfig copies config.yaml.ok over config.yaml.
func RestoreLastGoodConfig() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	last := lastGoodConfigPath(path)
	data, err := os.ReadFile(last)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no last-good config at %s", last)
		}
		return err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}
	return nil
}

func snapshotLastGoodConfig(path string, existing []byte) {
	if len(bytes.TrimSpace(existing)) == 0 {
		return
	}
	var tmp Config
	if err := yaml.Unmarshal(existing, &tmp); err != nil {
		return
	}
	_ = os.WriteFile(lastGoodConfigPath(path), existing, 0o600)
}

func parseAndMigrateUserYAML(data []byte) (*Config, []string, error) {
	if err := rejectRemovedConfigKeys(data); err != nil {
		return nil, nil, err
	}
	var cfg Config
	var notes []string
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		if bytes.Contains(data, []byte{'\t'}) {
			rewritten := rewriteLeadingYAMLTabs(data)
			if err2 := yaml.Unmarshal(rewritten, &cfg); err2 != nil {
				return nil, nil, err
			}
			notes = append(notes, "leading tabs → spaces")
			data = rewritten
		} else {
			return nil, nil, err
		}
	}
	if migrateAuthSurfaceIntoType(&cfg) {
		notes = append(notes, "auth surface into type (platform oauth → type: platform)")
	}
	if configYAMLUsesLegacyWorkplaceKeys(data) {
		notes = append(notes, "tenant/tenants keys → context/contexts")
	}
	folded, err := foldFlatIntoOrgs(&cfg)
	if err != nil {
		return nil, nil, err
	}
	if folded {
		notes = append(notes, "flat contexts → orgs tree")
	}
	normalizeAuthChannels(&cfg)
	return &cfg, notes, nil
}

// configYAMLUsesLegacyWorkplaceKeys reports tenant:/tenants: (top-level),
// auths.*.tenants, or channels.*.tenant/tenants. tenantId is not a workplace key.
func configYAMLUsesLegacyWorkplaceKeys(data []byte) bool {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return false
	}
	m := yamlMappingRoot(&root)
	if m == nil {
		return false
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		k := m.Content[i].Value
		v := m.Content[i+1]
		switch k {
		case "tenant", "tenants":
			return true
		case "auths", "channels":
			if nestedMappingsHaveKey(v, "tenant", "tenants") {
				return true
			}
		}
	}
	return false
}

func yamlMappingRoot(n *yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}
	if n.Kind == yaml.DocumentNode && len(n.Content) > 0 {
		n = n.Content[0]
	}
	if n.Kind != yaml.MappingNode {
		return nil
	}
	return n
}

func nestedMappingsHaveKey(n *yaml.Node, keys ...string) bool {
	if n == nil || n.Kind != yaml.MappingNode {
		return false
	}
	want := make(map[string]bool, len(keys))
	for _, k := range keys {
		want[k] = true
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		entry := n.Content[i+1]
		if entry == nil || entry.Kind != yaml.MappingNode {
			continue
		}
		for j := 0; j+1 < len(entry.Content); j += 2 {
			if want[entry.Content[j].Value] {
				return true
			}
		}
	}
	return false
}

func rejectRemovedConfigKeys(data []byte) error {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil || raw == nil {
		return nil
	}
	switch {
	case raw["credentials"] != nil:
		return fmt.Errorf("credentials: was removed; use contexts: and auths: (see 'buddy cred help')")
	case raw["defaults"] != nil:
		return fmt.Errorf("defaults: was removed; use channels.<cli|mcp|ide>.context")
	case raw["allow_auths"] != nil:
		return fmt.Errorf("allow_auths: was removed; set channels: on each auth")
	case raw["cred"] != nil:
		return fmt.Errorf("cred: was removed; use context.default")
	}
	return nil
}

// migrateAuthSurfaceIntoType folds legacy auth surface into type:
//   - surface: any / instance → dropped (instance is the default)
//   - type: oauth + surface: platform → type: platform
func migrateAuthSurfaceIntoType(cfg *Config) bool {
	if cfg == nil {
		return false
	}
	changed := false
	for name, a := range cfg.Auths {
		surf := strings.ToLower(strings.TrimSpace(a.Surface))
		typ := NormalizeAuthType(a.Type)
		next := a
		if surf == SurfacePlatform && (typ == AuthTypeOAuth || typ == "") {
			next.Type = AuthTypePlatform
			typ = AuthTypePlatform
		}
		if surf != "" {
			next.Surface = ""
		}
		if next.Type != a.Type || next.Surface != a.Surface {
			cfg.Auths[name] = next
			changed = true
		}
	}
	return changed
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

// ConfigFromCredentialMap builds a tenants/auths config from a flat
// name→CredentialConfig map (test helper). defaultName sets Config.Tenant
// when it matches a tenant or auth.
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
// Unique host identities (instance URL and/or platform scope) become tenants;
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
				Region:        c.Region,
				URL:           c.URL,
				EnvironmentID: c.EnvironmentID,
				TenantID:      c.TenantID,
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
		typ := c.Auth.Type
		if c.IsPlatform() && (typ == "" || typ == AuthTypeOAuth) {
			typ = AuthTypePlatform
		}
		cfg.Auths[name] = AuthEntry{
			Type:         typ,
			RunCommand:   c.Auth.RunCommand,
			Username:     c.Auth.Username,
			Password:     c.Auth.Password,
			ClientID:     c.Auth.ClientID,
			ClientSecret: c.Auth.ClientSecret,
			Tenants:      []string{tenantName},
			Access:       AccessFull,
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
	eid := strings.TrimSpace(c.EnvironmentID)
	tid := strings.TrimSpace(c.TenantID)
	if tid == "" {
		tid = strings.TrimSpace(c.Auth.TenantID)
	}
	url := strings.TrimRight(strings.ToLower(strings.TrimSpace(c.URL)), "/")
	region := strings.ToLower(strings.TrimSpace(c.Region))
	if region == "" {
		region = RegionFromGatewayURL(c.URL)
	}
	if eid != "" {
		return "platform:" + region + ":env:" + eid
	}
	if tid != "" {
		return "platform:" + region + ":" + tid
	}
	if region != "" || LooksLikePlatformGateway(c.URL) {
		return "platform:" + region + ":org"
	}
	return "instance:" + url
}

func normalizeConfigForSave(cfg *Config) {
	if cfg == nil {
		return
	}
	normalizeAuthChannels(cfg)
	migrateAuthSurfaceIntoType(cfg)
	for name, t := range cfg.Tenants {
		if t.IsOrganizationWorkplace() {
			t.URL = ""
			t.Region = ""
			t.Org = strings.TrimSpace(t.Org)
			cfg.Tenants[name] = t
			continue
		}
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
		t.Org = strings.TrimSpace(t.Org)
		cfg.Tenants[name] = t
	}
	for name, a := range cfg.Auths {
		if a.Access == AccessFull {
			a.Access = ""
		}
		a.Surface = ""
		normalizeAuthLinksForSave(&a)
		if NormalizeAuthType(a.Type) == AuthTypePlatform && strings.TrimSpace(a.Scope) == "" {
			a.Scope = a.EffectiveScope()
		}
		cfg.Auths[name] = a
	}
}

func normalizeAuthLinksForSave(a *AuthEntry) {
	if a == nil {
		return
	}
	if a.JamfPro == "" && len(a.Contexts) == 0 && len(a.Tenants) > 0 {
		switch {
		case NormalizeAuthType(a.Type) == AuthTypePlatform && a.EffectiveScope() == PlatformScopeTenant:
			a.JamfPro = a.Tenants[0]
		case NormalizeAuthType(a.Type) == AuthTypePlatform:
			a.Contexts = append([]string(nil), a.Tenants...)
		case len(a.Tenants) == 1:
			a.JamfPro = a.Tenants[0]
		default:
			a.Contexts = append([]string(nil), a.Tenants...)
		}
	}
	if a.JamfPro != "" || len(a.Contexts) > 0 {
		a.Tenants = mergeAuthLinks(a.Tenants, a.Contexts, a.JamfPro)
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

func yamlWorkplaceKeyOrder(existing []byte) []string {
	if order := yamlMapKeyOrder(existing, "contexts"); len(order) > 0 {
		return order
	}
	if order := yamlMapKeyOrder(existing, "tenants"); len(order) > 0 {
		return order
	}
	return yamlMapKeyOrder(existing, "credentials")
}

func marshalConfig(cfg *Config, existing []byte) ([]byte, error) {
	return marshalConfigWith(cfg, existing, marshalConfigOptions{fillDefault: true})
}

// appendBuiltinPatternsYAML writes deny_builtins / allow_builtins as a sequence.
// Quote "*" so YAML does not treat it as an alias.
func appendBuiltinPatternsYAML(entry *yaml.Node, key string, patterns []string) {
	if len(patterns) == 0 {
		return
	}
	seq := &yaml.Node{Kind: yaml.SequenceNode}
	for _, p := range patterns {
		n := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: p}
		if p == "*" {
			n.Style = yaml.DoubleQuotedStyle
		}
		seq.Content = append(seq.Content, n)
	}
	entry.Content = append(entry.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		seq,
	)
}

type marshalConfigOptions struct {
	fillDefault   bool
	omitEmptyMaps bool
}

func marshalConfigWith(cfg *Config, existing []byte, opts marshalConfigOptions) ([]byte, error) {
	if cfg == nil {
		cfg = &Config{}
	}
	normalizeConfigForSave(cfg)
	if opts.fillDefault && strings.TrimSpace(cfg.Tenant.Default) == "" {
		cfg.Tenant.Default = DefaultTenant
	}

	tenantSet := make(map[string]struct{}, len(cfg.Tenants))
	for n := range cfg.Tenants {
		tenantSet[n] = struct{}{}
	}
	implied, _ := workplacesFromOrgs(cfg.Orgs)
	if len(implied) > 0 {
		for n, t := range cfg.Tenants {
			if src, ok := implied[n]; ok && workplaceIdentityEqual(t, src) {
				delete(tenantSet, n)
			}
		}
	}
	authSet := make(map[string]struct{}, len(cfg.Auths))
	for n := range cfg.Auths {
		authSet[n] = struct{}{}
	}
	tenantOrder := mapKeyOrder(yamlWorkplaceKeyOrder(existing), tenantSet)
	authOrder := mapKeyOrder(yamlMapKeyOrder(existing, "auths"), authSet)
	if len(authOrder) == 0 {
		authOrder = mapKeyOrder(yamlMapKeyOrder(existing, "credentials"), authSet)
	}

	tenantsNode := &yaml.Node{Kind: yaml.MappingNode}
	for _, name := range tenantOrder {
		t := cfg.Tenants[name]
		entry := &yaml.Node{Kind: yaml.MappingNode}
		addTenantStr := func(k, v string) {
			if v == "" {
				return
			}
			entry.Content = append(entry.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k},
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v},
			)
		}
		addTenantStr("org", t.Org)
		addTenantStr("region", t.Region)
		addTenantStr("id", t.EnvironmentID)
		if t.URL != "" || t.TenantID != "" {
			jamf := &yaml.Node{Kind: yaml.MappingNode}
			addJamf := func(k, v string) {
				if v == "" {
					return
				}
				jamf.Content = append(jamf.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k},
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v},
				)
			}
			addJamf("url", t.URL)
			addJamf("tenantId", t.TenantID)
			if len(jamf.Content) > 0 {
				entry.Content = append(entry.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "jamfpro"},
					jamf,
				)
			}
		}
		appendBuiltinPatternsYAML(entry, "deny_builtins", t.DenyBuiltins)
		appendBuiltinPatternsYAML(entry, "allow_builtins", t.AllowBuiltins)
		tenantsNode.Content = append(tenantsNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: name},
			entry,
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
		switch NormalizeAuthType(a.Type) {
		case AuthTypeCommand:
			addStr("runCommand", a.RunCommand, false)
		case AuthTypeBasic:
			addStr("username", a.Username, false)
			addStr("password", a.Password, false)
		case AuthTypeOAuth, AuthTypePlatform:
			addStr("clientId", a.ClientID, false)
			addStr("clientSecret", a.ClientSecret, false)
		}
		if jp := strings.TrimSpace(a.JamfPro); jp != "" {
			addStr("jamfpro", jp, true)
		}
		if len(a.Contexts) > 0 {
			seq := &yaml.Node{Kind: yaml.SequenceNode}
			for _, t := range a.Contexts {
				seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: t})
			}
			entry.Content = append(entry.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "contexts"},
				seq,
			)
		} else if strings.TrimSpace(a.JamfPro) == "" && len(a.Tenants) > 0 {
			seq := &yaml.Node{Kind: yaml.SequenceNode}
			for _, t := range a.Tenants {
				seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: t})
			}
			entry.Content = append(entry.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "contexts"},
				seq,
			)
		}
		if NormalizeAuthType(a.Type) == AuthTypePlatform {
			addStr("scope", a.EffectiveScope(), true)
		}
		if a.Access != "" && a.Access != AccessFull {
			addStr("access", a.Access, true)
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
		appendBuiltinPatternsYAML(entry, "deny_builtins", a.DenyBuiltins)
		appendBuiltinPatternsYAML(entry, "allow_builtins", a.AllowBuiltins)
		authsNode.Content = append(authsNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: name},
			entry,
		)
	}

	root := &yaml.Node{Kind: yaml.MappingNode}
	if d := strings.TrimSpace(cfg.Tenant.Default); d != "" {
		selNode := &yaml.Node{Kind: yaml.MappingNode}
		selNode.Content = append(selNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "default"},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: d},
		)
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "context"},
			selNode,
		)
	}

	if orgsNode := marshalOrgsNode(cfg.Orgs, existing); orgsNode != nil {
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "orgs"},
			orgsNode,
		)
	}

	writeTenants := len(tenantOrder) > 0 || (!opts.omitEmptyMaps && len(cfg.Orgs) == 0)
	writeAuths := len(authOrder) > 0 || !opts.omitEmptyMaps
	if writeTenants {
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "contexts"},
			tenantsNode,
		)
	}
	if writeAuths {
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "auths"},
			authsNode,
		)
	}

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
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "context"},
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: t},
				)
			}
			if len(p.Tenants) > 0 {
				seq := &yaml.Node{Kind: yaml.SequenceNode}
				for _, tn := range p.Tenants {
					seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: tn})
				}
				entry.Content = append(entry.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "contexts"},
					seq,
				)
			}
			appendBuiltinPatternsYAML(entry, "deny_builtins", p.DenyBuiltins)
			appendBuiltinPatternsYAML(entry, "allow_builtins", p.AllowBuiltins)
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

	feat := &yaml.Node{Kind: yaml.MappingNode}
	if cfg.Features.ClassicInstanceOnly != nil && *cfg.Features.ClassicInstanceOnly {
		feat.Content = append(feat.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "classic_instance_only"},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"},
		)
	}
	if cfg.Features.UseAgent != nil {
		val := "false"
		if *cfg.Features.UseAgent {
			val = "true"
		}
		feat.Content = append(feat.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "use_agent"},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: val},
		)
	}
	if cfg.Features.ForceAgent != nil {
		val := "false"
		if *cfg.Features.ForceAgent {
			val = "true"
		}
		feat.Content = append(feat.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "force_agent"},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: val},
		)
	}
	if len(feat.Content) > 0 {
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "features"},
			feat,
		)
	}

	return yaml.Marshal(root)
}

// SaveConfig writes the config (new tenants/auths shape only).
// When managed prefs contributed to cfg, only user overrides and local keys are written.
func SaveConfig(cfg *Config) error {
	path, err := configPath()
	if err != nil {
		return fmt.Errorf("resolve config path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	existing, _ := os.ReadFile(path)
	snapshotLastGoodConfig(path, existing)
	toWrite := cfg
	opts := marshalConfigOptions{fillDefault: true}
	hasBaseLayer := configBaseLayer(cfg) != nil
	if hasBaseLayer {
		toWrite = userOverlayForSave(cfg)
		opts = marshalConfigOptions{fillDefault: false, omitEmptyMaps: true}
	}
	if toWrite != nil {
		if _, err := foldFlatIntoOrgs(toWrite); err != nil {
			return fmt.Errorf("fold config into orgs: %w", err)
		}
		if err := expandOrgs(toWrite); err != nil {
			return fmt.Errorf("expand orgs: %w", err)
		}
		normalizeConfigForSave(toWrite)
	}
	data, err := marshalConfigWith(toWrite, existing, opts)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}
	if hasBaseLayer && toWrite != nil {
		cfg.user = cloneConfig(toWrite)
	}
	return nil
}
