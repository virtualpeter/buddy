package buddy

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Buddy client channel names (distinct from Jamf auth surface).
const (
	ChannelCLI = "cli"
	ChannelMCP = "mcp"
	ChannelIDE = "ide"
)

// AllBuddyChannels is the closed set of buddy client channels.
func AllBuddyChannels() []string {
	return []string{ChannelCLI, ChannelMCP, ChannelIDE}
}

// ChannelPolicy is per-client policy under top-level channels:.
type ChannelPolicy struct {
	Access        string   `yaml:"access,omitempty"`         // readonly | full (omit = full)
	Tenant        string   `yaml:"tenant,omitempty"`         // default tenant for this channel
	Tenants       []string `yaml:"tenants,omitempty"`        // optional allowlist
	DenyBuiltins  []string `yaml:"deny_builtins,omitempty"`  // exact names or trailing-* prefix globs; "*" deny-all
	AllowBuiltins []string `yaml:"allow_builtins,omitempty"` // override deny at this layer; ["*"] unblocks all
}

// UnmarshalYAML accepts deny_builtins / allow_builtins as a sequence (canonical)
// or a single scalar. Quote "*" so YAML does not treat it as an alias.
func (p *ChannelPolicy) UnmarshalYAML(value *yaml.Node) error {
	if p == nil {
		return fmt.Errorf("channel policy is nil")
	}
	var raw struct {
		Access        string    `yaml:"access"`
		Tenant        string    `yaml:"tenant"`
		Tenants       []string  `yaml:"tenants"`
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
	p.Access = raw.Access
	p.Tenant = raw.Tenant
	p.Tenants = raw.Tenants
	p.DenyBuiltins = deny
	p.AllowBuiltins = allow
	return nil
}

func decodeBuiltinPatternList(n *yaml.Node) ([]string, error) {
	if n == nil || n.Kind == 0 {
		return nil, nil
	}
	switch n.Kind {
	case yaml.ScalarNode:
		if n.Tag == "!!null" || n.Value == "" || n.Value == "~" || n.Value == "null" {
			return nil, nil
		}
		return []string{strings.TrimSpace(n.Value)}, nil
	case yaml.SequenceNode:
		var items []string
		if err := n.Decode(&items); err != nil {
			return nil, err
		}
		return items, nil
	default:
		return nil, fmt.Errorf("must be a string or a list of strings")
	}
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
func (cfg *Config) SetChannelPolicy(channel string, access, tenant string, tenants []string, setAccess, setTenant, setTenants bool, denyBuiltins []string, setDenyBuiltins bool, allowBuiltins []string, setAllowBuiltins bool) error {
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
	if setAllowBuiltins {
		normalized, err := NormalizeAllowBuiltins(allowBuiltins)
		if err != nil {
			return err
		}
		if len(normalized) == 0 {
			p.AllowBuiltins = nil
		} else {
			p.AllowBuiltins = normalized
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
	return strings.ToLower(strings.TrimSpace(channel))
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
