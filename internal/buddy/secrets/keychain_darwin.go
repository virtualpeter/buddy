//go:build darwin && !secitem

package secrets

import (
	"fmt"
	"os/exec"
	"strings"
)

const keychainService = keychainServiceCLI

// KeychainStore stores secrets in the macOS Keychain via the security(1) CLI.
type KeychainStore struct{}

// NewKeychainStore creates a Keychain-backed secret store.
func NewKeychainStore() *KeychainStore {
	return &KeychainStore{}
}

func (k *KeychainStore) Description() string {
	return fmt.Sprintf("macOS Keychain (service %q, security CLI)", keychainService)
}

func (k *KeychainStore) Get(credName string, kind Kind) (string, error) {
	account := keychainAccount(credName, kind)
	cmd := exec.Command("security", "find-generic-password", "-s", keychainService, "-a", account, "-w")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			msg := strings.TrimSpace(string(exitErr.Stderr))
			if strings.Contains(msg, "could not be found") || strings.Contains(msg, "The specified item could not be found") {
				return "", fmt.Errorf("secret %q not found in keychain for credential %q", kind, credName)
			}
			return "", fmt.Errorf("keychain lookup: %s", msg)
		}
		return "", fmt.Errorf("keychain lookup: %w", err)
	}
	val := strings.TrimSpace(string(output))
	if val == "" {
		return "", fmt.Errorf("secret %q not found in keychain for credential %q", kind, credName)
	}
	return val, nil
}

func (k *KeychainStore) Set(credName string, kind Kind, value string) error {
	account := keychainAccount(credName, kind)
	cmd, err := securityAddInteractiveCmd(keychainService, account, value)
	if err != nil {
		return err
	}
	if _, err := cmd.CombinedOutput(); err != nil {
		// Do not include command output or stdin (may contain the secret).
		return fmt.Errorf("keychain store: %w", err)
	}
	return nil
}

func (k *KeychainStore) Delete(credName string, kind Kind) error {
	account := keychainAccount(credName, kind)
	cmd := exec.Command("security", "delete-generic-password", "-s", keychainService, "-a", account)
	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if strings.Contains(msg, "could not be found") {
			return nil
		}
		return fmt.Errorf("keychain delete: %w: %s", err, msg)
	}
	return nil
}

func (k *KeychainStore) Has(credName string, kind Kind) (bool, error) {
	_, err := k.Get(credName, kind)
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (k *KeychainStore) List() ([]Entry, error) {
	cmd := exec.Command("security", "dump-keychain")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// dump-keychain often writes to stderr even on success; still try parse.
		if len(output) == 0 {
			return nil, fmt.Errorf("keychain list: %w", err)
		}
	}
	return parseKeychainDumpForService(string(output), keychainService), nil
}
