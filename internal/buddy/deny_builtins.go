package buddy

import (
	"fmt"
	"strings"
)

// NormalizeDenyBuiltins trims, dedupes, and validates deny_builtins patterns.
// Patterns are exact builtin names, or a single trailing "*" prefix glob (e.g. jamf_*).
func NormalizeDenyBuiltins(patterns []string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.Contains(p, "*") {
			if !strings.HasSuffix(p, "*") || strings.Count(p, "*") != 1 {
				return nil, fmt.Errorf("deny_builtins pattern %q: only a trailing * prefix glob is supported (e.g. jamf_*)", p)
			}
			if p == "*" {
				return nil, fmt.Errorf("deny_builtins pattern \"*\" is not allowed (too broad)")
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

// BuiltinNameDenied reports whether name matches any deny_builtins pattern.
func BuiltinNameDenied(patterns []string, name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || len(patterns) == 0 {
		return false
	}
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
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
	return fmt.Errorf("builtin %q is blocked for channel %s (channels.%s.deny_builtins); use a named builtin or docs — do not invent HTTP", name, channel, channel)
}

// ActiveChannelDenyBuiltins returns deny_builtins for the process active channel.
func ActiveChannelDenyBuiltins() []string {
	cfg := GetActiveConfig()
	if cfg == nil {
		return nil
	}
	return append([]string(nil), cfg.ChannelPolicy(ActiveChannel()).DenyBuiltins...)
}
