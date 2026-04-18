package buddy

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// DefaultCred is the default credential name used when --cred is not specified.
const DefaultCred = "default"

// AuthConfig holds the authentication method configuration for a credential.
type AuthConfig struct {
	Type         string `yaml:"type"`         // "command", "basic", or "oauth"
	RunCommand   string `yaml:"runCommand"`   // type: command — stdout of this command is used as Bearer token
	Username     string `yaml:"username"`     // type: basic
	Password     string `yaml:"password"`     // type: basic
	ClientID     string `yaml:"clientId"`     // type: oauth
	ClientSecret string `yaml:"clientSecret"` // type: oauth
}

// CredentialConfig holds the URL and auth configuration for a named credential.
type CredentialConfig struct {
	URL  string     `yaml:"url"`
	Auth AuthConfig `yaml:"auth"`
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
			"no credentials configured — run 'buddy config add' to add one, or see 'buddy config help'",
		)
	}
	cred, ok := cfg.Credentials[credName]
	if !ok {
		return CredentialConfig{}, fmt.Errorf(
			"credential %q not found in config — run 'buddy config list' to see available credentials",
			credName,
		)
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

// SaveConfig writes the config to ~/.config/buddy/config.yaml (or XDG_CONFIG_HOME).
// Parent directories are created if they do not exist.
// Note: any comments in the existing file will not be preserved.
func SaveConfig(cfg *Config) error {
	path, err := configPath()
	if err != nil {
		return fmt.Errorf("resolve config path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}
	return nil
}
