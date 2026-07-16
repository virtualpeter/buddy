package secrets

import (
	"os"
	"path/filepath"
)

// ConfigDir returns the buddy config directory (~/.config/buddy or XDG_CONFIG_HOME/buddy).
func ConfigDir() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "buddy"), nil
}

// SecretsDir returns the directory for file-backed secrets.
func SecretsDir() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "secrets"), nil
}

func secretFilePath(credName string) (string, error) {
	dir, err := SecretsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, credName+".json"), nil
}
