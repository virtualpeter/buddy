package buddy

import (
	"sort"
	"strings"
)

// AuthSummary is a public view of an auth entry for UI / MCP session listing.
type AuthSummary struct {
	Name    string   `json:"name"`
	Tenants []string `json:"tenants"`
	Access  string   `json:"access,omitempty"`
	Surface string   `json:"surface,omitempty"`
	Type    string   `json:"type,omitempty"`
}

// SessionSelection describes the active tenant/auth pin and available choices
// for a buddy client channel (cli|mcp|ide|notebook).
type SessionSelection struct {
	Channel        string        `json:"channel"`
	Tenant         string        `json:"tenant"`
	Auth           string        `json:"auth"` // pin; empty means auto-pick
	Tenants        []string      `json:"tenants"`
	Auths          []AuthSummary `json:"auths"`
	ChannelAccess  string        `json:"channel_access,omitempty"`
	ChannelTenant  string        `json:"channel_tenant,omitempty"`
	ChannelTenants []string      `json:"channel_tenants,omitempty"`
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
		Channel: channel,
		Tenant:  tenant,
		Auth:    authPin,
		Tenants: []string{},
		Auths:   []AuthSummary{},
	}
	if cfg == nil {
		return out
	}
	pol := cfg.ChannelPolicy(channel)
	out.ChannelAccess = pol.EffectiveAccess()
	out.ChannelTenant = strings.TrimSpace(pol.Tenant)
	out.ChannelTenants = append([]string(nil), pol.Tenants...)

	if tenant == "" {
		tenant = cfg.DefaultTenant(channel)
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
			Name:    name,
			Tenants: ts,
			Access:  a.EffectiveAccess(),
			Surface: a.EffectiveSurface(),
			Type:    a.Type,
		})
	}
	if authPin != "" && !cfg.AuthAllowed(channel, authPin) {
		out.Auth = ""
	}
	if tenant != "" && !cfg.TenantAllowedOnChannel(channel, tenant) {
		out.Tenant = cfg.DefaultTenant(channel)
	}

	credNames := make([]string, 0, len(out.Auths))
	for _, a := range out.Auths {
		credNames = append(credNames, a.Name)
	}
	out.Active = out.Auth
	out.Creds = credNames
	return out
}

// ToMap returns the /api/creds JSON shape.
func (s SessionSelection) ToMap() map[string]any {
	return map[string]any{
		"channel":         s.Channel,
		"tenant":          s.Tenant,
		"auth":            s.Auth,
		"tenants":         s.Tenants,
		"auths":           s.Auths,
		"channel_access":  s.ChannelAccess,
		"channel_tenant":  s.ChannelTenant,
		"channel_tenants": s.ChannelTenants,
		"active":          s.Active,
		"creds":           s.Creds,
	}
}

// InfoMap returns a channel-aware snapshot including binary version (MCP buddy_info).
func (s SessionSelection) InfoMap(version string) map[string]any {
	if version == "" {
		version = "dev"
	}
	return map[string]any{
		"version":         version,
		"channel":         s.Channel,
		"tenant":          s.Tenant,
		"auth":            s.Auth,
		"tenants":         s.Tenants,
		"auths":           s.Auths,
		"channel_access":  s.ChannelAccess,
		"channel_tenant":  s.ChannelTenant,
		"channel_tenants": s.ChannelTenants,
	}
}
