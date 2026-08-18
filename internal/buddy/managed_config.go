package buddy

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
	"howett.net/plist"
)

const (
	// ManagedPrefsDomain is the macOS preference domain for org-provided buddy
	// config (Jamf Application & Custom Settings / MCX). Same reverse-DNS as the
	// signed binary identifier and Keychain service.
	ManagedPrefsDomain = "virtualpete.buddy.cli"

	// EnvManagedPlist, when set, is the only managed plist loaded (tests and
	// non-Darwin drop-in). It replaces Darwin Managed Preferences discovery.
	EnvManagedPlist = "BUDDY_MANAGED_PLIST"
)

// HasManagedPrefs reports whether this config was merged with managed preferences.
func (cfg *Config) HasManagedPrefs() bool {
	if cfg == nil || cfg.mdm == nil {
		return false
	}
	m := cfg.mdm
	return strings.TrimSpace(m.Tenant.Default) != "" || len(m.Tenants) > 0 || len(m.Auths) > 0 || len(m.Channels) > 0
}

// TenantFromManagedPrefs reports whether name is supplied by managed preferences.
func (cfg *Config) TenantFromManagedPrefs(name string) bool {
	if cfg == nil || cfg.mdm == nil || cfg.mdm.Tenants == nil {
		return false
	}
	_, ok := cfg.mdm.Tenants[name]
	return ok
}

// AuthFromManagedPrefs reports whether name is supplied by managed preferences.
func (cfg *Config) AuthFromManagedPrefs(name string) bool {
	if cfg == nil || cfg.mdm == nil || cfg.mdm.Auths == nil {
		return false
	}
	_, ok := cfg.mdm.Auths[name]
	return ok
}

// ManagedPrefsSummary is a short line for `buddy cred list` (empty if none).
func (cfg *Config) ManagedPrefsSummary() string {
	if !cfg.HasManagedPrefs() {
		return ""
	}
	var parts []string
	if names := sortedMapKeys(cfg.mdm.Tenants); len(names) > 0 {
		parts = append(parts, "tenants "+strings.Join(names, ", "))
	}
	if names := sortedMapKeys(cfg.mdm.Auths); len(names) > 0 {
		parts = append(parts, "auths "+strings.Join(names, ", "))
	}
	if names := sortedMapKeys(cfg.mdm.Channels); len(names) > 0 {
		parts = append(parts, "channels "+strings.Join(names, ", "))
	}
	if d := strings.TrimSpace(cfg.mdm.Tenant.Default); d != "" {
		parts = append(parts, "default "+d)
	}
	if len(parts) == 0 {
		return ""
	}
	return "managed: " + strings.Join(parts, "; ")
}

func sortedMapKeys[T any](m map[string]T) []string {
	if len(m) == 0 {
		return nil
	}
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func loadManagedConfig() (*Config, bool, error) {
	if p := strings.TrimSpace(os.Getenv(EnvManagedPlist)); p != "" {
		c, err := loadManagedPlistFile(p)
		if err != nil && os.IsNotExist(err) {
			return nil, false, nil
		}
		if err != nil {
			return nil, false, err
		}
		return c, managedAuthsHaveSecrets(c), nil
	}
	if runtime.GOOS != "darwin" {
		return nil, false, nil
	}
	var paths []string
	paths = append(paths, filepath.Join("/Library/Managed Preferences", ManagedPrefsDomain+".plist"))
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		paths = append(paths, filepath.Join(home, "Library/Managed Preferences", ManagedPrefsDomain+".plist"))
	}
	var merged *Config
	for _, p := range paths {
		c, err := loadManagedPlistFile(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, false, err
		}
		merged = overlayConfig(merged, c)
	}
	return merged, managedAuthsHaveSecrets(merged), nil
}

func loadManagedPlistFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg, err := configFromManagedPlist(data)
	if err != nil {
		return nil, fmt.Errorf("managed preferences %s: %w", path, err)
	}
	return cfg, nil
}

func configFromManagedPlist(data []byte) (*Config, error) {
	var raw map[string]any
	if _, err := plist.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse plist: %w", err)
	}
	raw = unwrapManagedPlist(raw)
	converted, ok := plistToYAMLValue(raw).(map[string]any)
	if !ok || converted == nil {
		converted = map[string]any{}
	}
	yamlBytes, err := yaml.Marshal(converted)
	if err != nil {
		return nil, fmt.Errorf("encode managed prefs: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(yamlBytes, &cfg); err != nil {
		return nil, fmt.Errorf("decode managed prefs: %w", err)
	}
	cfg.Cred = ""
	cfg.Credentials = nil
	cfg.Defaults = nil
	cfg.AllowAuths = nil
	normalizeAuthChannels(&cfg)
	return &cfg, nil
}

func unwrapManagedPlist(raw map[string]any) map[string]any {
	if raw == nil {
		return nil
	}
	if hasManagedConfigKeys(raw) {
		return raw
	}
	if mcx, ok := raw["mcx_preference_settings"].(map[string]any); ok {
		return unwrapManagedPlist(mcx)
	}
	for _, key := range []string{"Forced", "forced"} {
		arr, ok := raw[key].([]any)
		if !ok || len(arr) == 0 {
			continue
		}
		if d, ok := arr[0].(map[string]any); ok {
			return unwrapManagedPlist(d)
		}
	}
	return raw
}

func hasManagedConfigKeys(raw map[string]any) bool {
	for _, k := range []string{"tenant", "tenants", "auths", "channels"} {
		if _, ok := raw[k]; ok {
			return true
		}
	}
	return false
}

func plistToYAMLValue(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[k] = plistToYAMLValue(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = plistToYAMLValue(val)
		}
		return out
	case []byte:
		return string(x)
	case uint64, uint32, uint16, uint8, uint, int64, int32, int16, int8, int, float64, float32:
		return fmt.Sprint(x)
	default:
		return x
	}
}

func managedAuthsHaveSecrets(cfg *Config) bool {
	if cfg == nil {
		return false
	}
	for _, a := range cfg.Auths {
		if a.Password != "" || a.ClientSecret != "" {
			return true
		}
	}
	return false
}

func overlayConfig(base, overlay *Config) *Config {
	if base == nil && overlay == nil {
		return &Config{}
	}
	if overlay == nil {
		return cloneConfig(base)
	}
	if base == nil {
		return cloneConfig(overlay)
	}
	out := cloneConfig(base)
	if d := strings.TrimSpace(overlay.Tenant.Default); d != "" {
		out.Tenant.Default = d
	}
	if len(overlay.Tenants) > 0 {
		if out.Tenants == nil {
			out.Tenants = map[string]TenantConfig{}
		}
		for name, t := range overlay.Tenants {
			out.Tenants[name] = cloneTenantConfig(t)
		}
	}
	if len(overlay.Auths) > 0 {
		if out.Auths == nil {
			out.Auths = map[string]AuthEntry{}
		}
		for name, a := range overlay.Auths {
			out.Auths[name] = cloneAuthEntry(a)
		}
	}
	if len(overlay.Channels) > 0 {
		if out.Channels == nil {
			out.Channels = map[string]ChannelPolicy{}
		}
		for name, p := range overlay.Channels {
			out.Channels[name] = cloneChannelPolicy(p)
		}
	}
	if overlay.Features.ClassicInstanceOnly != nil {
		out.Features.ClassicInstanceOnly = cloneBoolPtr(overlay.Features.ClassicInstanceOnly)
	}
	return out
}

func cloneConfig(c *Config) *Config {
	if c == nil {
		return nil
	}
	out := &Config{Tenant: c.Tenant}
	if c.Tenants != nil {
		out.Tenants = make(map[string]TenantConfig, len(c.Tenants))
		for k, v := range c.Tenants {
			out.Tenants[k] = cloneTenantConfig(v)
		}
	}
	if c.Auths != nil {
		out.Auths = make(map[string]AuthEntry, len(c.Auths))
		for k, v := range c.Auths {
			out.Auths[k] = cloneAuthEntry(v)
		}
	}
	if c.Channels != nil {
		out.Channels = make(map[string]ChannelPolicy, len(c.Channels))
		for k, v := range c.Channels {
			out.Channels[k] = cloneChannelPolicy(v)
		}
	}
	out.Features.ClassicInstanceOnly = cloneBoolPtr(c.Features.ClassicInstanceOnly)
	return out
}

func cloneAuthEntry(a AuthEntry) AuthEntry {
	a.Tenants = append([]string(nil), a.Tenants...)
	a.Channels = append([]string(nil), a.Channels...)
	a.DenyBuiltins = append([]string(nil), a.DenyBuiltins...)
	a.AllowBuiltins = append([]string(nil), a.AllowBuiltins...)
	return a
}

func cloneTenantConfig(t TenantConfig) TenantConfig {
	t.DenyBuiltins = append([]string(nil), t.DenyBuiltins...)
	t.AllowBuiltins = append([]string(nil), t.AllowBuiltins...)
	return t
}

func cloneChannelPolicy(p ChannelPolicy) ChannelPolicy {
	p.Tenants = append([]string(nil), p.Tenants...)
	p.DenyBuiltins = append([]string(nil), p.DenyBuiltins...)
	p.AllowBuiltins = append([]string(nil), p.AllowBuiltins...)
	return p
}

// userOverlayForSave returns the subset of cfg that belongs in config.yaml when
// managed prefs are in play. Unmodified MDM keys are omitted so later profile
// updates still apply.
func userOverlayForSave(cfg *Config) *Config {
	if cfg == nil {
		return &Config{}
	}
	if cfg.mdm == nil {
		return cloneConfig(cfg)
	}
	out := &Config{}
	userDefault := ""
	if cfg.user != nil {
		userDefault = strings.TrimSpace(cfg.user.Tenant.Default)
	}
	mdmDefault := strings.TrimSpace(cfg.mdm.Tenant.Default)
	mergedDefault := strings.TrimSpace(cfg.Tenant.Default)
	if userDefault != "" {
		out.Tenant.Default = mergedDefault
	} else if mdmDefault != "" && mergedDefault != mdmDefault {
		out.Tenant.Default = mergedDefault
	}

	userTenants := map[string]struct{}{}
	if cfg.user != nil {
		for n := range cfg.user.Tenants {
			userTenants[n] = struct{}{}
		}
	}
	for name, t := range cfg.Tenants {
		_, inUser := userTenants[name]
		mdmT, inMDM := cfg.mdm.Tenants[name]
		if inUser || !inMDM || !tenantConfigEqual(t, mdmT) {
			if out.Tenants == nil {
				out.Tenants = map[string]TenantConfig{}
			}
			out.Tenants[name] = cloneTenantConfig(t)
		}
	}

	userAuths := map[string]struct{}{}
	if cfg.user != nil {
		for n := range cfg.user.Auths {
			userAuths[n] = struct{}{}
		}
	}
	for name, a := range cfg.Auths {
		_, inUser := userAuths[name]
		mdmA, inMDM := cfg.mdm.Auths[name]
		if inUser || !inMDM || !authEntryEqual(a, mdmA) {
			if out.Auths == nil {
				out.Auths = map[string]AuthEntry{}
			}
			out.Auths[name] = cloneAuthEntry(a)
		}
	}

	userCh := map[string]struct{}{}
	if cfg.user != nil {
		for n := range cfg.user.Channels {
			userCh[n] = struct{}{}
		}
	}
	for name, p := range cfg.Channels {
		_, inUser := userCh[name]
		mdmP, inMDM := cfg.mdm.Channels[name]
		if inUser || !inMDM || !channelPolicyEqual(p, mdmP) {
			if out.Channels == nil {
				out.Channels = map[string]ChannelPolicy{}
			}
			out.Channels[name] = cloneChannelPolicy(p)
		}
	}

	userFeat := FeatureFlags{}
	if cfg.user != nil {
		userFeat = cfg.user.Features
	}
	if !boolPtrEqual(cfg.Features.ClassicInstanceOnly, cfg.mdm.Features.ClassicInstanceOnly) ||
		userFeat.ClassicInstanceOnly != nil {
		out.Features.ClassicInstanceOnly = cloneBoolPtr(cfg.Features.ClassicInstanceOnly)
	}
	return out
}

func tenantConfigEqual(a, b TenantConfig) bool {
	return strings.TrimSpace(a.URL) == strings.TrimSpace(b.URL) &&
		strings.TrimSpace(a.Region) == strings.TrimSpace(b.Region) &&
		strings.TrimSpace(a.TenantID) == strings.TrimSpace(b.TenantID) &&
		stringSliceEqual(a.DenyBuiltins, b.DenyBuiltins) &&
		stringSliceEqual(a.AllowBuiltins, b.AllowBuiltins)
}

func authEntryEqual(a, b AuthEntry) bool {
	a.Password = ""
	b.Password = ""
	a.ClientSecret = ""
	b.ClientSecret = ""
	if a.Type != b.Type || a.RunCommand != b.RunCommand || a.Username != b.Username || a.ClientID != b.ClientID {
		return false
	}
	if a.EffectiveAccess() != b.EffectiveAccess() || a.EffectiveSurface() != b.EffectiveSurface() {
		return false
	}
	return stringSliceEqual(a.Tenants, b.Tenants) && stringSliceEqual(a.normalizedChannels(), b.normalizedChannels()) &&
		stringSliceEqual(a.DenyBuiltins, b.DenyBuiltins) && stringSliceEqual(a.AllowBuiltins, b.AllowBuiltins)
}

func channelPolicyEqual(a, b ChannelPolicy) bool {
	if a.EffectiveAccess() != b.EffectiveAccess() {
		return false
	}
	if strings.TrimSpace(a.Tenant) != strings.TrimSpace(b.Tenant) {
		return false
	}
	return stringSliceEqual(a.Tenants, b.Tenants) && stringSliceEqual(a.DenyBuiltins, b.DenyBuiltins) && stringSliceEqual(a.AllowBuiltins, b.AllowBuiltins)
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// MarshalManagedPlist encodes cfg as XML plist for Jamf ACS preference domain
// ManagedPrefsDomain. Auths are omitted (names, client IDs, and secrets are
// per-user). They are still accepted if present when loading managed prefs.
func MarshalManagedPlist(cfg *Config) ([]byte, error) {
	if cfg == nil {
		cfg = &Config{}
	}
	stripped := cloneConfig(cfg)
	stripped.Auths = nil
	data, err := marshalConfigWith(stripped, nil, marshalConfigOptions{fillDefault: strings.TrimSpace(stripped.Tenant.Default) != "", omitEmptyMaps: true})
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := plist.NewEncoder(&buf)
	enc.Indent("  ")
	if err := enc.Encode(raw); err != nil {
		return nil, err
	}
	out := buf.Bytes()
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	return out, nil
}

func firstSortedTenantName(cfg *Config) string {
	if cfg == nil || len(cfg.Tenants) == 0 {
		return ""
	}
	names := make([]string, 0, len(cfg.Tenants))
	for n := range cfg.Tenants {
		names = append(names, n)
	}
	sort.Strings(names)
	return names[0]
}
