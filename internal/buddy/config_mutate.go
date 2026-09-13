package buddy

import (
	"fmt"
	"sort"
	"strings"
)

// SetTenantOpts controls SetTenantMetadata.
type SetTenantOpts struct {
	URL           string
	Region        string
	Org           string
	EnvironmentID string
	TenantID      string
	DenyBuiltins  []string
	AllowBuiltins []string
	SetDeny       bool // when true, DenyBuiltins replaces previous (even if empty)
	SetAllow      bool
	SetOrg        bool // when true, Org replaces previous (even if empty)
	Overwrite     bool // when false and tenant exists, return ErrExists
}

// ErrExists is returned when Overwrite is false and the named entry already exists.
var ErrExists = fmt.Errorf("already exists")

// SetTenantMetadata creates or updates a tenant (url / region / scope ids / builtins).
// Does not prompt. Caller must SaveConfig.
func SetTenantMetadata(cfg *Config, name string, opts SetTenantOpts) (created bool, err error) {
	if cfg == nil {
		return false, fmt.Errorf("config is nil")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return false, fmt.Errorf("tenant name required")
	}
	prev, existing := cfg.Tenants[name]
	if existing && !opts.Overwrite {
		return false, fmt.Errorf("tenant %q: %w", name, ErrExists)
	}
	t := TenantConfig{
		URL:           strings.TrimSpace(opts.URL),
		Region:        strings.TrimSpace(opts.Region),
		Org:           strings.TrimSpace(opts.Org),
		EnvironmentID: strings.TrimSpace(opts.EnvironmentID),
		TenantID:      strings.TrimSpace(opts.TenantID),
		DenyBuiltins:  opts.DenyBuiltins,
		AllowBuiltins: opts.AllowBuiltins,
	}
	if existing {
		if t.URL == "" {
			t.URL = prev.URL
		}
		if t.Region == "" {
			t.Region = prev.Region
		}
		if !opts.SetOrg {
			t.Org = prev.Org
		}
		if t.EnvironmentID == "" {
			t.EnvironmentID = prev.EnvironmentID
		}
		if t.TenantID == "" {
			t.TenantID = prev.TenantID
		}
		if !opts.SetDeny {
			t.DenyBuiltins = prev.DenyBuiltins
		}
		if !opts.SetAllow {
			t.AllowBuiltins = prev.AllowBuiltins
		}
	}
	if t.Region != "" {
		norm, err := NormalizePlatformRegion(t.Region)
		if err != nil {
			return false, err
		}
		t.Region = norm
	}
	if err := t.Validate(); err != nil {
		return false, err
	}
	if err := cfg.validateContextOrg(name, t); err != nil {
		return false, err
	}
	if cfg.Tenants == nil {
		cfg.Tenants = map[string]TenantConfig{}
	}
	cfg.Tenants[name] = t
	if strings.TrimSpace(cfg.Tenant.Default) == "" {
		cfg.SetDefaultTenant(name)
	}
	return !existing, nil
}

// SetAuthMetadataOpts controls SetAuthMetadata (no vault secrets).
type SetAuthMetadataOpts struct {
	Type          string
	Username      string
	ClientID      string
	RunCommand    string
	Tenants       []string
	JamfPro       string
	Contexts      []string
	Scope         string
	Access        string
	Channels      []string
	ChannelsSet   bool
	DenyBuiltins  []string
	AllowBuiltins []string
	SetDeny       bool
	SetAllow      bool
	Overwrite     bool
}

// SetAuthMetadata creates or updates an auth entry without touching vault secrets.
// Does not prompt. Caller must SaveConfig.
func SetAuthMetadata(cfg *Config, name string, opts SetAuthMetadataOpts) (created bool, err error) {
	if cfg == nil {
		return false, fmt.Errorf("config is nil")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return false, fmt.Errorf("auth name required")
	}
	prev, existing := cfg.Auths[name]
	if existing && !opts.Overwrite {
		return false, fmt.Errorf("auth %q: %w", name, ErrExists)
	}
	typ := NormalizeAuthType(opts.Type)
	if typ == "" && existing {
		typ = prev.Type
	}
	if typ == "" {
		return false, fmt.Errorf("auth type required (oauth, basic, platform, or command)")
	}
	entry := AuthEntry{
		Type:          typ,
		Tenants:       mergeAuthLinks(opts.Tenants, opts.Contexts, opts.JamfPro),
		JamfPro:       strings.TrimSpace(opts.JamfPro),
		Contexts:      append([]string{}, opts.Contexts...),
		Scope:         strings.TrimSpace(opts.Scope),
		Access:        strings.TrimSpace(opts.Access),
		Channels:      append([]string{}, opts.Channels...),
		DenyBuiltins:  opts.DenyBuiltins,
		AllowBuiltins: opts.AllowBuiltins,
		Username:      strings.TrimSpace(opts.Username),
		ClientID:      strings.TrimSpace(opts.ClientID),
		RunCommand:    strings.TrimSpace(opts.RunCommand),
	}
	if existing {
		if entry.Username == "" {
			entry.Username = prev.Username
		}
		if entry.ClientID == "" {
			entry.ClientID = prev.ClientID
		}
		if entry.RunCommand == "" {
			entry.RunCommand = prev.RunCommand
		}
		entry.Password = prev.Password
		entry.ClientSecret = prev.ClientSecret
		if len(entry.Tenants) == 0 {
			entry.Tenants = append([]string{}, prev.Tenants...)
			entry.JamfPro = prev.JamfPro
			entry.Contexts = append([]string{}, prev.Contexts...)
		}
		if entry.Scope == "" {
			entry.Scope = prev.Scope
		}
		if entry.Access == "" {
			entry.Access = prev.Access
		}
		if !opts.ChannelsSet {
			entry.Channels = prev.Channels
		}
		if !opts.SetDeny {
			entry.DenyBuiltins = prev.DenyBuiltins
		}
		if !opts.SetAllow {
			entry.AllowBuiltins = prev.AllowBuiltins
		}
	}
	if err := entry.Validate(); err != nil {
		return false, err
	}
	for _, tn := range entry.Tenants {
		if _, ok := cfg.Tenants[tn]; !ok {
			return false, fmt.Errorf("tenant %q not found — create it first", tn)
		}
	}
	if cfg.Auths == nil {
		cfg.Auths = map[string]AuthEntry{}
	}
	cfg.Auths[name] = entry
	return !existing, nil
}

// DeleteTenantResult summarizes cascade effects of DeleteTenantMetadata.
type DeleteTenantResult struct {
	DeletedAuths   []string
	UnlinkedAuths  []string
	ClearedDefault bool
}

// DeleteTenantMetadata removes a tenant, deletes sole-linked auths (+ secrets),
// and unlinks multi-tenant auths. Does not prompt. Caller must SaveConfig.
func DeleteTenantMetadata(cfg *Config, name string) (DeleteTenantResult, error) {
	var res DeleteTenantResult
	if cfg == nil {
		return res, fmt.Errorf("config is nil")
	}
	name = strings.TrimSpace(name)
	if _, ok := cfg.Tenants[name]; !ok {
		return res, fmt.Errorf("tenant %q not found", name)
	}
	var deleteAuths, unlinkAuths []string
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
	for _, an := range deleteAuths {
		if err := deleteAuthRecord(cfg, an); err != nil {
			return res, err
		}
	}
	res.DeletedAuths = deleteAuths
	for _, an := range unlinkAuths {
		a := cfg.Auths[an]
		if a.JamfPro == name {
			a.JamfPro = ""
		}
		var keptCtx []string
		for _, t := range a.Contexts {
			if t != name {
				keptCtx = append(keptCtx, t)
			}
		}
		a.Contexts = keptCtx
		var kept []string
		for _, t := range a.Tenants {
			if t != name {
				kept = append(kept, t)
			}
		}
		a.Tenants = kept
		cfg.Auths[an] = a
	}
	res.UnlinkedAuths = unlinkAuths
	delete(cfg.Tenants, name)
	if cfg.Tenant.Default == name {
		cfg.Tenant.Default = DefaultTenant
		res.ClearedDefault = true
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
	return res, nil
}

// DeleteAuthMetadata removes an auth and its vault secrets. Does not prompt.
// Caller must SaveConfig.
func DeleteAuthMetadata(cfg *Config, name string) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	name = strings.TrimSpace(name)
	if _, ok := cfg.Auths[name]; !ok {
		return fmt.Errorf("auth %q not found", name)
	}
	return deleteAuthRecord(cfg, name)
}

func deleteAuthRecord(cfg *Config, name string) error {
	delete(cfg.Auths, name)
	_ = DeleteCredentialSecrets(name, true, true) // store may be unavailable
	return nil
}

// SetSelectionOpts sets tenant.default and/or a channel default tenant.
type SetSelectionOpts struct {
	DefaultTenant string // empty = leave unchanged
	Channel       string // cli|mcp|ide; empty = only DefaultTenant
	ChannelTenant string // when Channel set: set channels.<ch>.tenant (empty clears)
}

// SetSelection updates default / per-channel tenant selection. Caller must SaveConfig.
func SetSelection(cfg *Config, opts SetSelectionOpts) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if name := strings.TrimSpace(opts.DefaultTenant); name != "" {
		if _, ok := cfg.Tenants[name]; !ok {
			return fmt.Errorf("context %q not found", name)
		}
		cfg.SetDefaultTenant(name)
	}
	ch := strings.TrimSpace(opts.Channel)
	if ch == "" {
		return nil
	}
	switch ch {
	case ChannelCLI, ChannelMCP, ChannelIDE:
	default:
		return fmt.Errorf("unknown channel %q (want cli, mcp, or ide)", ch)
	}
	tenant := strings.TrimSpace(opts.ChannelTenant)
	if tenant != "" {
		if _, ok := cfg.Tenants[tenant]; !ok {
			return fmt.Errorf("context %q not found", tenant)
		}
	}
	return cfg.SetChannelPolicy(ch, "", tenant, nil, false, true, false, nil, false, nil, false)
}

// ConfigSnapshot is the JSON-friendly view for GET /api/config.
type ConfigSnapshot struct {
	NeedsSetup      bool              `json:"needs_setup"`
	ConfigFile      bool              `json:"config_file"` // user config.yaml exists
	DefaultContext  string            `json:"default_context"`
	DefaultTenant   string            `json:"default_tenant"`   // dual-read alias of default_context
	ChannelContexts map[string]string `json:"channel_contexts"` // cli/mcp/ide → effective default
	ChannelTenants  map[string]string `json:"channel_tenants"`  // dual-read alias of channel_contexts
	Contexts        []TenantSnapshot  `json:"contexts"`
	Tenants         []TenantSnapshot  `json:"tenants"` // dual-read alias of contexts
	Auths           []AuthSnapshot    `json:"auths"`
	Coverage        CoverageReport    `json:"coverage"`
}

// TenantSnapshot is one context row for the config UI.
type TenantSnapshot struct {
	Name          string `json:"name"`
	URL           string `json:"url,omitempty"`
	Region        string `json:"region,omitempty"`
	Org           string `json:"org,omitempty"`
	Env           string `json:"env,omitempty"` // parent environment workplace name (not UUID)
	EnvironmentID string `json:"environment_id,omitempty"`
	TenantID      string `json:"tenant_id,omitempty"` // Jamf Pro product tenant UUID
	Kind          string `json:"kind,omitempty"`      // organization | environment | product-tenant | instance
	Organization  bool   `json:"organization,omitempty"`
	Managed       bool   `json:"managed,omitempty"`
}

// AuthSnapshot is one auth row for the config UI (metadata + secret presence).
type AuthSnapshot struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Contexts []string `json:"contexts"`
	Tenants  []string `json:"tenants"` // dual-read alias of contexts
	Access   string   `json:"access"`
	Scope    string   `json:"scope,omitempty"` // platform: organization|environment|tenant
	Channels []string `json:"channels,omitempty"`
	Username string   `json:"username,omitempty"`
	ClientID string   `json:"client_id,omitempty"`
	Secret   string   `json:"secret"` // missing | stored | inline | n/a
	Managed  bool     `json:"managed,omitempty"`
}

// BuildConfigSnapshot builds the serve/API view of config (no secret values).
func BuildConfigSnapshot(cfg *Config) ConfigSnapshot {
	snap := ConfigSnapshot{
		NeedsSetup:      ConfigNeedsSetup(cfg),
		ChannelContexts: map[string]string{},
		ChannelTenants:  map[string]string{},
	}
	if exists, err := ConfigFileExists(); err == nil {
		snap.ConfigFile = exists
	}
	if cfg == nil {
		return snap
	}
	snap.DefaultContext = cfg.Tenant.Default
	snap.DefaultTenant = cfg.Tenant.Default
	for _, ch := range []string{ChannelCLI, ChannelMCP, ChannelIDE} {
		d := cfg.DefaultTenant(ch)
		snap.ChannelContexts[ch] = d
		snap.ChannelTenants[ch] = d
	}
	for _, row := range WorkplaceTree(cfg.Tenants) {
		t := cfg.Tenants[row.Name]
		org := row.Org
		if t.WorkplaceKind() == "organization" {
			org = row.Name
		}
		snap.Tenants = append(snap.Tenants, TenantSnapshot{
			Name:          row.Name,
			URL:           t.URL,
			Region:        t.Region,
			Org:           org,
			Env:           row.Environment,
			EnvironmentID: t.EnvironmentID,
			TenantID:      t.TenantID,
			Kind:          row.Kind,
			Organization:  t.IsOrganizationWorkplace(),
			Managed:       cfg.TenantFromManagedPrefs(row.Name),
		})
	}
	snap.Contexts = snap.Tenants
	anames := make([]string, 0, len(cfg.Auths))
	for n := range cfg.Auths {
		anames = append(anames, n)
	}
	sort.Strings(anames)
	for _, n := range anames {
		a := cfg.Auths[n]
		snap.Auths = append(snap.Auths, AuthSnapshot{
			Name:     n,
			Type:     a.Type,
			Contexts: append([]string{}, a.Tenants...),
			Tenants:  append([]string{}, a.Tenants...),
			Access:   a.EffectiveAccess(),
			Scope:    a.EffectiveScope(),
			Channels: append([]string{}, a.Channels...),
			Username: a.Username,
			ClientID: a.ClientID,
			Secret:   AuthSecretPresence(cfg, n, false),
			Managed:  cfg.AuthFromManagedPrefs(n),
		})
	}
	snap.Coverage = BuildCoverageReport(cfg)
	return snap
}
