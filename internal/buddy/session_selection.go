package buddy

import (
	"sort"
	"strings"
)

// AuthSummary is a public view of an auth entry for UI / MCP session listing.
type AuthSummary struct {
	Name     string   `json:"name"`
	Contexts []string `json:"contexts"`
	Tenants  []string `json:"tenants"` // dual-read alias of contexts
	Access   string   `json:"access,omitempty"`
	Surface  string   `json:"surface,omitempty"`
	Type     string   `json:"type,omitempty"`
}

// SessionSelection describes the active context/auth pin and available choices
// for a buddy client channel (cli|mcp|ide).
type SessionSelection struct {
	Channel             string        `json:"channel"`
	Context             string        `json:"context"`
	Tenant              string        `json:"tenant"` // dual-read alias of context
	Auth                string        `json:"auth"`   // pin; empty means auto-pick
	Contexts            []string      `json:"contexts"`
	Tenants             []string      `json:"tenants"` // dual-read alias of contexts
	Auths               []AuthSummary `json:"auths"`
	ChannelAccess       string        `json:"channel_access,omitempty"`
	ChannelContext      string        `json:"channel_context,omitempty"`
	ChannelTenant       string        `json:"channel_tenant,omitempty"` // dual-read alias of channel_context
	ChannelContexts     []string      `json:"channel_contexts,omitempty"`
	ChannelTenants      []string      `json:"channel_tenants,omitempty"` // dual-read alias of channel_contexts
	DenyBuiltins        []string      `json:"deny_builtins,omitempty"`  // channel effective (MCP includes default jamf_*)
	AllowBuiltins       []string      `json:"allow_builtins,omitempty"` // channel allow
	TenantDenyBuiltins  []string      `json:"tenant_deny_builtins,omitempty"`
	TenantAllowBuiltins []string      `json:"tenant_allow_builtins,omitempty"`
	AuthDenyBuiltins    []string      `json:"auth_deny_builtins,omitempty"`
	AuthAllowBuiltins   []string      `json:"auth_allow_builtins,omitempty"`
	// Legacy keys kept for /api/creds compatibility (auth pin / auth names).
	Active string   `json:"active,omitempty"`
	Creds  []string `json:"creds,omitempty"`
}

// SessionSelectionFor builds a channel-aware snapshot of tenants and auths.
// defaultTenant / defaultAuth override the process ActiveSelection when non-empty
// (as used by buddy serve options).
func SessionSelectionFor(channel, defaultTenant, defaultAuth string) SessionSelection {
	channel = normalizeBuddyChannel(channel)
	if channel == "" {
		channel = ChannelCLI
	}

	cfg := GetActiveConfig()
	tenant, authPin := ActiveSelection()
	if defaultTenant != "" {
		tenant = defaultTenant
	}
	if defaultAuth != "" {
		authPin = defaultAuth
	}

	out := SessionSelection{
		Channel:  channel,
		Context:  tenant,
		Tenant:   tenant,
		Auth:     authPin,
		Contexts: []string{},
		Tenants:  []string{},
		Auths:    []AuthSummary{},
	}
	out.DenyBuiltins = EffectiveDenyBuiltins(cfg, channel)
	out.AllowBuiltins = EffectiveAllowBuiltins(cfg, channel)
	if cfg == nil {
		return out
	}
	pol := cfg.ChannelPolicy(channel)
	out.ChannelAccess = pol.EffectiveAccess()
	out.ChannelTenant = strings.TrimSpace(pol.Tenant)
	out.ChannelTenants = append([]string(nil), pol.Tenants...)

	if tenant != "" && cfg.Tenants != nil {
		if t, ok := cfg.Tenants[tenant]; ok {
			out.TenantDenyBuiltins = append([]string(nil), t.DenyBuiltins...)
			out.TenantAllowBuiltins = append([]string(nil), t.AllowBuiltins...)
		}
	}
	if authPin != "" && cfg.Auths != nil {
		if a, ok := cfg.Auths[authPin]; ok {
			out.AuthDenyBuiltins = append([]string(nil), a.DenyBuiltins...)
			out.AuthAllowBuiltins = append([]string(nil), a.AllowBuiltins...)
		}
	}

	if tenant == "" {
		tenant = cfg.DefaultTenant(channel)
		out.Context = tenant
		out.Tenant = tenant
	}

	for name := range cfg.Tenants {
		if cfg.TenantAllowedOnChannel(channel, name) {
			out.Tenants = append(out.Tenants, name)
		}
	}
	sort.Strings(out.Tenants)

	authNames := make([]string, 0, len(cfg.Auths))
	for name := range cfg.Auths {
		authNames = append(authNames, name)
	}
	sort.Strings(authNames)
	authNames = cfg.FilterAuths(channel, authNames)
	for _, name := range authNames {
		a := cfg.Auths[name]
		ts := append([]string(nil), a.Tenants...)
		sort.Strings(ts)
		out.Auths = append(out.Auths, AuthSummary{
			Name:     name,
			Contexts: ts,
			Tenants:  ts,
			Access:   a.EffectiveAccess(),
			Surface:  a.EffectiveSurface(),
			Type:     a.Type,
		})
	}
	if authPin != "" && !cfg.AuthAllowed(channel, authPin) {
		out.Auth = ""
	}
	if tenant != "" && !cfg.TenantAllowedOnChannel(channel, tenant) {
		out.Context = cfg.DefaultTenant(channel)
		out.Tenant = out.Context
		tenant = out.Tenant
	}
	if authPin != "" && out.Auth == "" {
		authPin = ""
	}
	if t, ok := cfg.Tenants[tenant]; ok {
		out.TenantDenyBuiltins = append([]string(nil), t.DenyBuiltins...)
		out.TenantAllowBuiltins = append([]string(nil), t.AllowBuiltins...)
	}
	if authPin != "" {
		if a, ok := cfg.Auths[authPin]; ok {
			out.AuthDenyBuiltins = append([]string(nil), a.DenyBuiltins...)
			out.AuthAllowBuiltins = append([]string(nil), a.AllowBuiltins...)
		}
	}

	credNames := make([]string, 0, len(out.Auths))
	for _, a := range out.Auths {
		credNames = append(credNames, a.Name)
	}
	out.Active = out.Auth
	out.Creds = credNames
	out.Context = out.Tenant
	out.Contexts = out.Tenants
	out.ChannelContext = out.ChannelTenant
	out.ChannelContexts = out.ChannelTenants
	return out
}

// ToMap returns the /api/creds JSON shape.
func (s SessionSelection) ToMap() map[string]any {
	m := map[string]any{
		"channel":          s.Channel,
		"context":          s.Context,
		"tenant":           s.Tenant,
		"auth":             s.Auth,
		"contexts":         s.Contexts,
		"tenants":          s.Tenants,
		"auths":            s.Auths,
		"channel_access":   s.ChannelAccess,
		"channel_context":  s.ChannelContext,
		"channel_tenant":   s.ChannelTenant,
		"channel_contexts": s.ChannelContexts,
		"channel_tenants":  s.ChannelTenants,
		"deny_builtins":    s.DenyBuiltins,
		"active":           s.Active,
		"creds":            s.Creds,
	}
	if len(s.AllowBuiltins) > 0 {
		m["allow_builtins"] = s.AllowBuiltins
	}
	if len(s.TenantDenyBuiltins) > 0 {
		m["tenant_deny_builtins"] = s.TenantDenyBuiltins
	}
	if len(s.TenantAllowBuiltins) > 0 {
		m["tenant_allow_builtins"] = s.TenantAllowBuiltins
	}
	if len(s.AuthDenyBuiltins) > 0 {
		m["auth_deny_builtins"] = s.AuthDenyBuiltins
	}
	if len(s.AuthAllowBuiltins) > 0 {
		m["auth_allow_builtins"] = s.AuthAllowBuiltins
	}
	return m
}

// InfoMap returns a channel-aware snapshot including binary version (MCP buddy_info).
func (s SessionSelection) InfoMap(version string) map[string]any {
	if version == "" {
		version = "dev"
	}
	m := map[string]any{
		"version":          version,
		"channel":          s.Channel,
		"context":          s.Context,
		"tenant":           s.Tenant,
		"auth":             s.Auth,
		"contexts":         s.Contexts,
		"tenants":          s.Tenants,
		"auths":            s.Auths,
		"channel_access":   s.ChannelAccess,
		"channel_context":  s.ChannelContext,
		"channel_tenant":   s.ChannelTenant,
		"channel_contexts": s.ChannelContexts,
		"deny_builtins":    s.DenyBuiltins,
	}
	if len(s.AllowBuiltins) > 0 {
		m["allow_builtins"] = s.AllowBuiltins
	}
	if len(s.TenantDenyBuiltins) > 0 {
		m["tenant_deny_builtins"] = s.TenantDenyBuiltins
	}
	if len(s.TenantAllowBuiltins) > 0 {
		m["tenant_allow_builtins"] = s.TenantAllowBuiltins
	}
	if len(s.AuthDenyBuiltins) > 0 {
		m["auth_deny_builtins"] = s.AuthDenyBuiltins
	}
	if len(s.AuthAllowBuiltins) > 0 {
		m["auth_allow_builtins"] = s.AuthAllowBuiltins
	}
	return m
}
