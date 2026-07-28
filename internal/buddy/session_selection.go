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
// for a buddy client channel (cli|mcp|serve|ide).
type SessionSelection struct {
	Channel   string        `json:"channel"`
	Tenant    string        `json:"tenant"`
	Auth      string        `json:"auth"` // pin; empty means auto-pick
	Tenants   []string      `json:"tenants"`
	Auths     []AuthSummary `json:"auths"`
	IDETenant string        `json:"ide_tenant,omitempty"`
	// Legacy keys kept for /api/creds compatibility (auth pin / auth names).
	Active string   `json:"active,omitempty"`
	Creds  []string `json:"creds,omitempty"`
}

// SessionSelectionFor builds a channel-aware snapshot of tenants and auths.
// defaultTenant / defaultAuth override the process ActiveSelection when non-empty
// (as used by buddy serve options). Auth channel filtering treats ide as serve.
func SessionSelectionFor(channel, defaultTenant, defaultAuth string) SessionSelection {
	tenantChannel := strings.ToLower(strings.TrimSpace(channel))
	if tenantChannel == "" {
		tenantChannel = ChannelCLI
	}
	authChannel := normalizeBuddyChannel(tenantChannel)

	cfg := GetActiveConfig()
	tenant, authPin := ActiveSelection()
	if defaultTenant != "" {
		tenant = defaultTenant
	}
	if defaultAuth != "" {
		authPin = defaultAuth
	}

	out := SessionSelection{
		Channel: tenantChannel,
		Tenant:  tenant,
		Auth:    authPin,
		Tenants: []string{},
		Auths:   []AuthSummary{},
	}
	if cfg == nil {
		return out
	}
	if tenant == "" {
		tenant = cfg.DefaultTenant(tenantChannel)
		out.Tenant = tenant
	}
	out.IDETenant = cfg.DefaultTenant(ChannelIDE)

	for name := range cfg.Tenants {
		out.Tenants = append(out.Tenants, name)
	}
	sort.Strings(out.Tenants)

	authNames := make([]string, 0, len(cfg.Auths))
	for name := range cfg.Auths {
		authNames = append(authNames, name)
	}
	sort.Strings(authNames)
	authNames = cfg.FilterAuths(authChannel, authNames)
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
	if authPin != "" && !cfg.AuthAllowed(authChannel, authPin) {
		out.Auth = ""
	}

	credNames := make([]string, 0, len(out.Auths))
	for _, a := range out.Auths {
		credNames = append(credNames, a.Name)
	}
	out.Active = out.Auth
	out.Creds = credNames
	return out
}

// ToMap returns the legacy /api/creds JSON shape (without channel).
func (s SessionSelection) ToMap() map[string]any {
	return map[string]any{
		"tenant":     s.Tenant,
		"ide_tenant": s.IDETenant,
		"auth":       s.Auth,
		"tenants":    s.Tenants,
		"auths":      s.Auths,
		"active":     s.Active,
		"creds":      s.Creds,
	}
}

// InfoMap returns a channel-aware snapshot including binary version (MCP buddy_info).
func (s SessionSelection) InfoMap(version string) map[string]any {
	if version == "" {
		version = "dev"
	}
	return map[string]any{
		"version": version,
		"channel": s.Channel,
		"tenant":  s.Tenant,
		"auth":    s.Auth,
		"tenants": s.Tenants,
		"auths":   s.Auths,
	}
}
