package buddy

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ImportQuickstart merges a secrets-free identity document (buddy YAML or ACS
// plist) into cfg and saves. Auths in the file are ignored.
func ImportQuickstart(cfg *Config, path string) error {
	if cfg == nil {
		return fmt.Errorf("internal error: config is nil")
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("quickstart path is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	imported, err := parseQuickstart(data)
	if err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	imported.Auths = nil
	if len(imported.Tenants) == 0 && len(imported.Orgs) == 0 {
		return fmt.Errorf("%s has no contexts: or orgs: (identity only; no secrets)", path)
	}
	merged := overlayConfig(cfg, imported)
	if merged == nil {
		merged = imported
	}
	if err := expandOrgs(merged); err != nil {
		return err
	}
	cfg.Tenant = merged.Tenant
	cfg.Tenants = merged.Tenants
	cfg.Orgs = merged.Orgs
	cfg.Channels = merged.Channels
	cfg.Features = merged.Features
	if strings.TrimSpace(cfg.Tenant.Default) == "" {
		if n := firstSortedTenantName(cfg); n != "" {
			cfg.Tenant.Default = n
		}
	}
	if err := cfg.ValidateTenantSelectionAndChannels(); err != nil {
		return err
	}
	if err := SaveConfig(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	return nil
}

func parseQuickstart(data []byte) (*Config, error) {
	if looksLikePlist(data) {
		return configFromManagedPlist(data)
	}
	if err := rejectRemovedConfigKeys(data); err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func looksLikePlist(data []byte) bool {
	s := bytes.TrimSpace(data)
	return bytes.HasPrefix(s, []byte("<?xml")) ||
		bytes.HasPrefix(s, []byte("<plist")) ||
		bytes.HasPrefix(s, []byte("bplist"))
}
