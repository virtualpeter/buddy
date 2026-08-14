package buddy

import (
	"fmt"
	"strings"
)

// Buddy client channel names (distinct from Jamf auth surface).
const (
	ChannelCLI  = "cli"
	ChannelMCP  = "mcp"
	ChannelIDE  = "ide"
	// ChannelNotebook is a legacy name. LoadConfig migrates it onto ChannelIDE.
	ChannelNotebook = "notebook"
)

// AllBuddyChannels is the closed set of buddy client channels.
func AllBuddyChannels() []string {
	return []string{ChannelCLI, ChannelMCP, ChannelIDE}
}

// ChannelPolicy is per-client policy under top-level channels:.
type ChannelPolicy struct {
	Access       string   `yaml:"access,omitempty"`        // readonly | full (omit = full)
	Tenant       string   `yaml:"tenant,omitempty"`        // default tenant for this channel
	Tenants      []string `yaml:"tenants,omitempty"`       // optional allowlist
	DenyBuiltins []string `yaml:"deny_builtins,omitempty"` // exact names or trailing-* prefix globs
}

// EffectiveAccess returns channel access, defaulting to full.
func (p ChannelPolicy) EffectiveAccess() string {
	switch strings.ToLower(strings.TrimSpace(p.Access)) {
	case AccessReadonly:
		return AccessReadonly
	default:
		return AccessFull
	}
}

// ChannelPolicy returns the policy for channel (empty policy if unset).
func (cfg *Config) ChannelPolicy(channel string) ChannelPolicy {
	if cfg == nil || cfg.Channels == nil {
		return ChannelPolicy{}
	}
	channel = normalizeBuddyChannel(channel)
	if channel == "" {
		return ChannelPolicy{}
	}
	return cfg.Channels[channel]
}

// SetChannelPolicy merges fields into channels[channel] (creates map/entry as needed).
func (cfg *Config) SetChannelPolicy(channel string, access, tenant string, tenants []string, setAccess, setTenant, setTenants bool, denyBuiltins []string, setDenyBuiltins bool) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	channel = normalizeBuddyChannel(channel)
	if !isBuddyChannel(channel) {
		return fmt.Errorf("unknown channel %q (want cli, mcp, or ide)", channel)
	}
	if cfg.Channels == nil {
		cfg.Channels = map[string]ChannelPolicy{}
	}
	p := cfg.Channels[channel]
	if setAccess {
		a := strings.ToLower(strings.TrimSpace(access))
		switch a {
		case "", AccessFull:
			p.Access = ""
		case AccessReadonly:
			p.Access = AccessReadonly
		default:
			return fmt.Errorf("access must be %q or %q", AccessFull, AccessReadonly)
		}
	}
	if setTenant {
		p.Tenant = strings.TrimSpace(tenant)
	}
	if setTenants {
		p.Tenants = normalizeNameList(tenants)
	}
	if setDenyBuiltins {
		normalized, err := NormalizeDenyBuiltins(denyBuiltins)
		if err != nil {
			return err
		}
		if len(normalized) == 0 {
			p.DenyBuiltins = nil
		} else {
			p.DenyBuiltins = normalized
		}
	}
	cfg.Channels[channel] = p
	return nil
}

func normalizeNameList(names []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	return out
}

func isBuddyChannel(channel string) bool {
	switch channel {
	case ChannelCLI, ChannelMCP, ChannelIDE:
		return true
	default:
		return false
	}
}

func normalizeBuddyChannel(channel string) string {
	channel = strings.ToLower(strings.TrimSpace(channel))
	if channel == ChannelNotebook {
		return ChannelIDE
	}
	return channel
}

// TenantAllowedOnChannel reports whether tenantName may be used on channel.
// Empty channel tenants allowlist means any configured tenant.
func (cfg *Config) TenantAllowedOnChannel(channel, tenantName string) bool {
	if cfg == nil || tenantName == "" {
		return true
	}
	p := cfg.ChannelPolicy(channel)
	if len(p.Tenants) == 0 {
		return true
	}
	for _, t := range p.Tenants {
		if t == tenantName {
			return true
		}
	}
	return false
}

// ChannelAccessFor returns effective access clamp for the channel (full|readonly).
func (cfg *Config) ChannelAccessFor(channel string) string {
	return cfg.ChannelPolicy(channel).EffectiveAccess()
}

// EffectiveAccessForChannel is the most restrictive of auth access and channel policy.
func (cfg *Config) EffectiveAccessForChannel(auth AuthEntry, channel string) string {
	a := auth.EffectiveAccess()
	c := cfg.ChannelAccessFor(channel)
	if a == AccessReadonly || c == AccessReadonly {
		return AccessReadonly
	}
	return AccessFull
}
