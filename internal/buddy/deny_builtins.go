package buddy

import (
	"fmt"
	"strings"
)

// DefaultMCPDenyBuiltin is always included in the MCP channel deny list unless
// allow_builtins unblocks it at the channel layer. CLI and IDE have no default deny.
const DefaultMCPDenyBuiltin = "jamf_*"

// NormalizeDenyBuiltins trims, dedupes, and validates deny_builtins patterns.
// Patterns are exact names, a trailing-* prefix glob (e.g. jamf_*), or bare "*"
// (deny everything at this layer; pair with allow_builtins for an allowlist).
func NormalizeDenyBuiltins(patterns []string) ([]string, error) {
	return normalizeBuiltinPatterns(patterns, "deny_builtins")
}

// NormalizeAllowBuiltins trims, dedupes, and validates allow_builtins patterns.
// Same globs as deny, including bare "*" (unblocks every denied builtin at this layer).
func NormalizeAllowBuiltins(patterns []string) ([]string, error) {
	return normalizeBuiltinPatterns(patterns, "allow_builtins")
}

func normalizeBuiltinPatterns(patterns []string, field string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.Contains(p, "*") {
			if !strings.HasSuffix(p, "*") || strings.Count(p, "*") != 1 {
				return nil, fmt.Errorf("%s pattern %q: only a trailing * prefix glob is supported (e.g. jamf_* or *)", field, p)
			}
		}
		if seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out, nil
}

func normalizeDenyAllowLists(deny, allow []string) (d, a []string, err error) {
	d, err = NormalizeDenyBuiltins(deny)
	if err != nil {
		return nil, nil, fmt.Errorf("deny_builtins: %w", err)
	}
	a, err = NormalizeAllowBuiltins(allow)
	if err != nil {
		return nil, nil, fmt.Errorf("allow_builtins: %w", err)
	}
	return d, a, nil
}

// BuiltinNameDenied reports whether name matches any deny_builtins pattern.
// Bare "*" matches every name.
func BuiltinNameDenied(patterns []string, name string) bool {
	return builtinPatternMatches(patterns, name)
}

// BuiltinNameAllowed reports whether name matches any allow_builtins pattern.
// Bare "*" matches every name.
func BuiltinNameAllowed(patterns []string, name string) bool {
	return builtinPatternMatches(patterns, name)
}

// BuiltinNameBlocked reports whether name is denied after that layer's allow_builtins.
func BuiltinNameBlocked(deny, allow []string, name string) bool {
	if !BuiltinNameDenied(deny, name) {
		return false
	}
	return !BuiltinNameAllowed(allow, name)
}

func builtinPatternMatches(patterns []string, name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || len(patterns) == 0 {
		return false
	}
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if p == "*" {
			return true
		}
		if strings.HasSuffix(p, "*") && strings.Count(p, "*") == 1 {
			prefix := strings.TrimSuffix(p, "*")
			if prefix != "" && strings.HasPrefix(name, prefix) {
				return true
			}
			continue
		}
		if p == name {
			return true
		}
	}
	return false
}

// ErrBuiltinDenied returns the Starlark/MCP error for a blocked builtin on channel.
func ErrBuiltinDenied(channel, name string) error {
	channel = normalizeBuddyChannel(channel)
	if channel == "" {
		channel = ChannelCLI
	}
	return ErrBuiltinDeniedAt("channels."+channel, name)
}

// ErrBuiltinDeniedAt names the config path that blocked the builtin (channels.mcp, tenants.prod, auths.ro).
func ErrBuiltinDeniedAt(scope, name string) error {
	return fmt.Errorf("builtin %q is blocked by %s.deny_builtins; allow_builtins can override at that layer — use a named builtin or docs, do not invent HTTP", name, scope)
}

// EffectiveDenyBuiltins returns deny patterns for channel, including the MCP
// default jamf_* (union with any YAML deny_builtins). CLI/IDE have no default.
func EffectiveDenyBuiltins(cfg *Config, channel string) []string {
	channel = normalizeBuddyChannel(channel)
	if channel == "" {
		channel = ChannelCLI
	}
	var raw []string
	if channel == ChannelMCP {
		raw = append(raw, DefaultMCPDenyBuiltin)
	}
	if cfg != nil {
		raw = append(raw, cfg.ChannelPolicy(channel).DenyBuiltins...)
	}
	out, err := NormalizeDenyBuiltins(raw)
	if err != nil {
		return raw
	}
	return out
}

// EffectiveAllowBuiltins returns configured allow_builtins for channel.
func EffectiveAllowBuiltins(cfg *Config, channel string) []string {
	if cfg == nil {
		return nil
	}
	return append([]string(nil), cfg.ChannelPolicy(channel).AllowBuiltins...)
}

// ActiveChannelDenyBuiltins returns effective deny_builtins for the process channel.
func ActiveChannelDenyBuiltins() []string {
	return EffectiveDenyBuiltins(GetActiveConfig(), ActiveChannel())
}

// ActiveChannelAllowBuiltins returns allow_builtins for the process channel.
func ActiveChannelAllowBuiltins() []string {
	return EffectiveAllowBuiltins(GetActiveConfig(), ActiveChannel())
}

// CheckBuiltinAllowed reports whether name is blocked on the resolved session.
// tenantKw/authKw override process defaults when non-empty (builtin kwargs).
// toTenant/toAuth, when set, are also checked (copy/migrate dest).
func CheckBuiltinAllowed(name, tenantKw, authKw, toTenant, toAuth string) error {
	cfg := GetActiveConfig()
	channel := ActiveChannel()
	tenant, auth := ActiveSelection()
	if strings.TrimSpace(tenantKw) != "" {
		tenant = strings.TrimSpace(tenantKw)
	}
	if strings.TrimSpace(authKw) != "" {
		auth = strings.TrimSpace(authKw)
	}
	if tenant == "" && cfg != nil {
		tenant = cfg.DefaultTenant(channel)
	}
	if err := checkBuiltinLayers(cfg, channel, tenant, auth, name); err != nil {
		return err
	}
	if strings.TrimSpace(toTenant) == "" && strings.TrimSpace(toAuth) == "" {
		return nil
	}
	dt := strings.TrimSpace(toTenant)
	da := strings.TrimSpace(toAuth)
	if dt == "" {
		dt = tenant
	}
	if da == "" {
		da = auth
	}
	return checkBuiltinLayers(cfg, channel, dt, da, name)
}

func checkBuiltinLayers(cfg *Config, channel, tenant, auth, name string) error {
	deny := EffectiveDenyBuiltins(cfg, channel)
	allow := EffectiveAllowBuiltins(cfg, channel)
	if BuiltinNameBlocked(deny, allow, name) {
		ch := normalizeBuddyChannel(channel)
		if ch == "" {
			ch = ChannelCLI
		}
		return ErrBuiltinDeniedAt("channels."+ch, name)
	}
	if cfg == nil {
		return nil
	}
	if tenant != "" {
		if t, ok := cfg.Tenants[tenant]; ok {
			if BuiltinNameBlocked(t.DenyBuiltins, t.AllowBuiltins, name) {
				return ErrBuiltinDeniedAt("tenants."+tenant, name)
			}
		}
	}
	if auth != "" {
		if a, ok := cfg.Auths[auth]; ok {
			if BuiltinNameBlocked(a.DenyBuiltins, a.AllowBuiltins, name) {
				return ErrBuiltinDeniedAt("auths."+auth, name)
			}
		}
	}
	return nil
}
