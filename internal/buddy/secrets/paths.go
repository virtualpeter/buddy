package secrets

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// ConfigDir returns the buddy config directory.
//
// Resolution order:
//  1. $XDG_CONFIG_HOME/buddy (all platforms, when set)
//  2. Windows: %APPDATA%\buddy (Roaming), else os.UserConfigDir()/buddy
//  3. macOS / Linux / others: ~/.config/buddy
//
// macOS deliberately uses ~/.config (XDG-style), not ~/Library/Application Support,
// so existing installs keep working.
func ConfigDir() (string, error) {
	home, _ := os.UserHomeDir()
	userCfg, _ := os.UserConfigDir()
	return resolveConfigDir(runtime.GOOS, os.Getenv("XDG_CONFIG_HOME"), os.Getenv("APPDATA"), home, userCfg)
}

func resolveConfigDir(goos, xdg, appData, home, userConfigDir string) (string, error) {
	if xdg != "" {
		return filepath.Join(xdg, "buddy"), nil
	}
	if goos == "windows" {
		if appData != "" {
			return filepath.Join(appData, "buddy"), nil
		}
		if userConfigDir != "" {
			return filepath.Join(userConfigDir, "buddy"), nil
		}
		return "", fmt.Errorf("resolve config dir: APPDATA and UserConfigDir unavailable")
	}
	if home == "" {
		return "", fmt.Errorf("resolve config dir: home directory unavailable")
	}
	return filepath.Join(home, ".config", "buddy"), nil
}

// SecretsDir returns the directory for file-backed secrets.
func SecretsDir() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "secrets"), nil
}

// TokensDir returns the directory for on-disk bearer token cache files.
func TokensDir() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "tokens"), nil
}

// PolicyCacheDir returns the root directory for on-disk Classic policy payload caches.
func PolicyCacheDir() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "policy-cache"), nil
}

func secretFilePath(credName string) (string, error) {
	dir, err := SecretsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, credName+".json"), nil
}
