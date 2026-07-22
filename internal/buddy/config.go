package buddy

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultCred is the default credential name used when --user is not specified.
const DefaultCred = "default"

// AuthConfig holds the authentication method configuration for a credential.
type AuthConfig struct {
	Type         string `yaml:"type"`                   // "command", "basic", or "oauth"
	RunCommand   string `yaml:"runCommand,omitempty"`   // type: command — stdout of this command is used as Bearer token
	Username     string `yaml:"username,omitempty"`     // type: basic
	Password     string `yaml:"password,omitempty"`     // type: basic
	ClientID     string `yaml:"clientId,omitempty"`     // type: oauth
	ClientSecret string `yaml:"clientSecret,omitempty"` // type: oauth
	// TenantID is accepted here only for mis-indented configs (should be on CredentialConfig).
	TenantID string `yaml:"tenantId,omitempty"`
}

// MarshalYAML emits only fields relevant to auth.type so SaveConfig does not
// write empty runCommand/username/… keys for every credential.
func (a AuthConfig) MarshalYAML() (interface{}, error) {
	n := &yaml.Node{Kind: yaml.MappingNode}
	add := func(key, val string, force bool) {
		if !force && val == "" {
			return
		}
		n.Content = append(n.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: val},
		)
	}
	add("type", a.Type, true)
	switch a.Type {
	case "command":
		add("runCommand", a.RunCommand, false)
	case "basic":
		add("username", a.Username, false)
		add("password", a.Password, false)
	case "oauth":
		add("clientId", a.ClientID, false)
		add("clientSecret", a.ClientSecret, false)
	default:
		add("runCommand", a.RunCommand, false)
		add("username", a.Username, false)
		add("password", a.Password, false)
		add("clientId", a.ClientID, false)
		add("clientSecret", a.ClientSecret, false)
	}
	// Misplaced auth.tenantId is migrated to CredentialConfig by normalizeConfigForSave.
	return n, nil
}

// CredentialConfig holds the URL and auth configuration for a named credential.
type CredentialConfig struct {
	Region   string     `yaml:"region,omitempty"`   // Platform API Gateway region (us|eu|apac|…); implies platform mode
	URL      string     `yaml:"url,omitempty"`      // Instance or gateway URL; optional when region is set
	TenantID string     `yaml:"tenantId,omitempty"` // Platform API Gateway tenant id; required in platform mode
	Auth     AuthConfig `yaml:"auth"`
}

// EffectiveTenantID returns the Platform tenant id from the credential root,
// or from auth.tenantId if the user misplaced it under auth: (common YAML mistake).
func (c CredentialConfig) EffectiveTenantID() string {
	if t := strings.TrimSpace(c.TenantID); t != "" {
		return t
	}
	return strings.TrimSpace(c.Auth.TenantID)
}

// EffectiveRegion returns the configured region, or infers it from a gateway URL.
func (c CredentialConfig) EffectiveRegion() string {
	if r := strings.TrimSpace(c.Region); r != "" {
		return strings.ToLower(r)
	}
	return RegionFromGatewayURL(c.URL)
}

// EffectiveBaseURL returns the API base URL: explicit url if set, otherwise
// https://{region}.apigw.jamf.com when region is set.
func (c CredentialConfig) EffectiveBaseURL() string {
	if u := strings.TrimRight(strings.TrimSpace(c.URL), "/"); u != "" {
		return u
	}
	if r := strings.TrimSpace(c.Region); r != "" {
		if norm, err := NormalizePlatformRegion(r); err == nil {
			return PlatformGatewayURL(norm)
		}
	}
	return ""
}

// IsPlatform returns true when this credential targets the Jamf Platform API Gateway.
func (c CredentialConfig) IsPlatform() bool {
	return c.EffectiveTenantID() != "" ||
		strings.TrimSpace(c.Region) != "" ||
		LooksLikePlatformGateway(c.URL)
}

// NormalizePlatformRegion lowercases and validates a gateway region code.
func NormalizePlatformRegion(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return "", fmt.Errorf("region is empty")
	}
	for _, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-'
		if !ok {
			return "", fmt.Errorf("invalid Platform API Gateway region %q (use us, eu, apac, …)", s)
		}
	}
	if s[0] < 'a' || s[0] > 'z' {
		return "", fmt.Errorf("invalid Platform API Gateway region %q (use us, eu, apac, …)", s)
	}
	return s, nil
}

// PlatformGatewayURL builds the regional gateway base URL for region.
func PlatformGatewayURL(region string) string {
	return "https://" + strings.ToLower(strings.TrimSpace(region)) + ".apigw.jamf.com"
}

// RegionFromGatewayURL returns the region subdomain for a *.apigw.jamf.com URL, or "".
func RegionFromGatewayURL(rawURL string) string {
	u := strings.ToLower(strings.TrimSpace(rawURL))
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	if i := strings.IndexByte(u, '/'); i >= 0 {
		u = u[:i]
	}
	const suffix = ".apigw.jamf.com"
	if !strings.HasSuffix(u, suffix) {
		return ""
	}
	return strings.TrimSuffix(u, suffix)
}

// LooksLikePlatformGateway reports whether url is a Jamf Platform API Gateway host
// (e.g. https://us.apigw.jamf.com), independent of tenantId.
func LooksLikePlatformGateway(rawURL string) bool {
	return RegionFromGatewayURL(rawURL) != ""
}

// UsesPlatformAuth is true when OAuth should use gateway token endpoints
// (/auth/token) rather than instance /api/oauth/token.
func (c CredentialConfig) UsesPlatformAuth() bool {
	return c.IsPlatform()
}

// LooksLikeTenantUUID reports whether s looks like a hyphenated UUID.
// Tenant IDs are opaque strings; this is only used for diagnostics, not validation.
func LooksLikeTenantUUID(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) != 36 {
		return false
	}
	parts := strings.Split(s, "-")
	if len(parts) != 5 {
		return false
	}
	if len(parts[0]) != 8 || len(parts[1]) != 4 || len(parts[2]) != 4 || len(parts[3]) != 4 || len(parts[4]) != 12 {
		return false
	}
	for _, p := range parts {
		for _, r := range p {
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
				return false
			}
		}
	}
	return true
}

// Validate checks credential fields for consistency (especially Platform API Gateway mode).
func (c CredentialConfig) Validate() error {
	if r := strings.TrimSpace(c.Region); r != "" {
		if _, err := NormalizePlatformRegion(r); err != nil {
			return err
		}
	}
	if !c.IsPlatform() {
		if strings.TrimSpace(c.URL) == "" {
			return fmt.Errorf("url is required (or set region + tenantId for Platform API Gateway); see 'buddy cred help'")
		}
		return nil
	}
	if c.EffectiveTenantID() == "" {
		return fmt.Errorf("Platform API Gateway credentials require tenantId (from Jamf Account → Integrations) as a sibling of url/auth/region — not under auth:; see 'buddy cred help'")
	}
	if c.Auth.Type != "oauth" {
		return fmt.Errorf("Platform API Gateway credentials require auth.type=oauth (got %q); basic and command auth are not supported", c.Auth.Type)
	}
	if c.EffectiveBaseURL() == "" {
		return fmt.Errorf("Platform API Gateway credentials require region (e.g. us) or url (e.g. https://us.apigw.jamf.com)")
	}
	return nil
}

// Config holds buddy CLI configuration.
type Config struct {
	Cred        string                      `yaml:"cred"`
	Credentials map[string]CredentialConfig `yaml:"credentials"`
}

// ResolveCredential looks up a named credential from the config.
// Returns a descriptive error if the credential is not found.
func (cfg *Config) ResolveCredential(credName string) (CredentialConfig, error) {
	if cfg.Credentials == nil {
		return CredentialConfig{}, fmt.Errorf(
			"no credentials configured — run 'buddy cred add' to add one, or see 'buddy cred help'",
		)
	}
	cred, ok := cfg.Credentials[credName]
	if !ok {
		return CredentialConfig{}, fmt.Errorf(
			"credential %q not found in config — run 'buddy cred list' to see available credentials",
			credName,
		)
	}
	if err := cred.Validate(); err != nil {
		return CredentialConfig{}, fmt.Errorf("credential %q: %w", credName, err)
	}
	return cred, nil
}

// configPath returns the path to the buddy config file.
// Uses XDG_CONFIG_HOME if set, otherwise ~/.config/buddy/config.yaml.
func configPath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "buddy", "config.yaml"), nil
}

// LoadConfig loads config from ~/.config/buddy/config.yaml (or XDG_CONFIG_HOME).
// Returns an error if the file cannot be read or parsed.
// Returns an empty config (with DefaultCred) if the file does not exist yet.
func LoadConfig() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, fmt.Errorf("resolve config path: %w", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{Cred: DefaultCred}, nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if cfg.Cred == "" {
		cfg.Cred = DefaultCred
	}
	return &cfg, nil
}

// normalizeConfigForSave moves misplaced auth.tenantId up to the credential root,
// infers region from gateway URLs, and drops url when it matches the regional default.
func normalizeConfigForSave(cfg *Config) {
	if cfg == nil || cfg.Credentials == nil {
		return
	}
	for name, c := range cfg.Credentials {
		if strings.TrimSpace(c.TenantID) == "" && strings.TrimSpace(c.Auth.TenantID) != "" {
			c.TenantID = strings.TrimSpace(c.Auth.TenantID)
		}
		c.Auth.TenantID = ""
		if strings.TrimSpace(c.Region) == "" {
			if inferred := RegionFromGatewayURL(c.URL); inferred != "" {
				c.Region = inferred
			}
		} else if norm, err := NormalizePlatformRegion(c.Region); err == nil {
			c.Region = norm
		}
		if c.Region != "" {
			def := PlatformGatewayURL(c.Region)
			if strings.TrimRight(strings.TrimSpace(c.URL), "/") == def {
				c.URL = ""
			}
		}
		cfg.Credentials[name] = c
	}
}

// credentialKeyOrder returns existing credential names in file order, then any
// new names alphabetically. Falls back to sorted names when existing is empty.
func credentialKeyOrder(cfg *Config, existing []byte) []string {
	var prefer []string
	if len(existing) > 0 {
		prefer = credentialNamesInYAMLOrder(existing)
	}
	seen := make(map[string]bool, len(prefer))
	out := make([]string, 0, len(cfg.Credentials))
	for _, name := range prefer {
		if _, ok := cfg.Credentials[name]; ok {
			out = append(out, name)
			seen[name] = true
		}
	}
	var rest []string
	for name := range cfg.Credentials {
		if !seen[name] {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	return append(out, rest...)
}

func credentialNamesInYAMLOrder(data []byte) []string {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil
	}
	root := &doc
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		root = doc.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value != "credentials" {
			continue
		}
		creds := root.Content[i+1]
		if creds.Kind != yaml.MappingNode {
			return nil
		}
		names := make([]string, 0, len(creds.Content)/2)
		for j := 0; j+1 < len(creds.Content); j += 2 {
			names = append(names, creds.Content[j].Value)
		}
		return names
	}
	return nil
}

func marshalConfig(cfg *Config, existing []byte) ([]byte, error) {
	normalizeConfigForSave(cfg)

	credOrder := credentialKeyOrder(cfg, existing)
	credsNode := &yaml.Node{Kind: yaml.MappingNode}
	for _, name := range credOrder {
		c := cfg.Credentials[name]
		var credNode yaml.Node
		if err := credNode.Encode(c); err != nil {
			return nil, err
		}
		credsNode.Content = append(credsNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: name},
			&credNode,
		)
	}

	root := &yaml.Node{Kind: yaml.MappingNode}
	root.Content = append(root.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "cred"},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: cfg.Cred},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "credentials"},
		credsNode,
	)
	return yaml.Marshal(root)
}

// SaveConfig writes the config to ~/.config/buddy/config.yaml (or XDG_CONFIG_HOME).
// Parent directories are created if they do not exist.
//
// The file is rewritten as a whole (comments are not preserved). Empty auth
// fields are omitted, and existing credential key order is kept when possible;
// newly added credentials are appended in alphabetical order.
func SaveConfig(cfg *Config) error {
	path, err := configPath()
	if err != nil {
		return fmt.Errorf("resolve config path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	existing, _ := os.ReadFile(path) // optional; used only for key order
	data, err := marshalConfig(cfg, existing)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}
	return nil
}
