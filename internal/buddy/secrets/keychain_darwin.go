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

func keychainMasterDescription() string {
	return fmt.Sprintf("macOS Keychain (service %q, account %q, security CLI)", keychainService, masterKeychainAccount)
}

func keychainDEKDescription() string {
	return fmt.Sprintf("macOS Keychain (service %q, account %q, security CLI)", keychainService, dekKeychainAccount)
}

func keychainLookup(service, account string) (string, error) {
	cmd := exec.Command("security", "find-generic-password", "-s", service, "-a", account, "-w")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			msg := strings.TrimSpace(string(exitErr.Stderr))
			if strings.Contains(msg, "could not be found") || strings.Contains(msg, "The specified item could not be found") {
				return "", fmt.Errorf("keychain item %q not found", account)
			}
			return "", fmt.Errorf("keychain lookup: %s", msg)
		}
		return "", fmt.Errorf("keychain lookup: %w", err)
	}
	val := strings.TrimSpace(string(output))
	if val == "" {
		return "", fmt.Errorf("keychain item %q not found", account)
	}
	return val, nil
}

func keychainStoreValue(service, account, value string) error {
	cmd, err := securityAddInteractiveCmd(service, account, value)
	if err != nil {
		return err
	}
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("keychain store: %w", err)
	}
	return nil
}

func keychainMasterGet() (string, error) {
	v, err := keychainLookup(keychainService, masterKeychainAccount)
	if err != nil {
		return "", fmt.Errorf("recovery passphrase not set — run 'buddy cred set passphrase': %w", err)
	}
	return v, nil
}

func keychainMasterSet(value string) error {
	return keychainStoreValue(keychainService, masterKeychainAccount, value)
}

func keychainMasterDelete() error {
	return deleteKeychainAccount(keychainService, masterKeychainAccount)
}

func keychainDEKGet() (string, error) {
	v, err := keychainLookup(keychainService, dekKeychainAccount)
	if err != nil {
		return "", fmt.Errorf("vault key not set: %w", err)
	}
	return v, nil
}

func keychainDEKSet(value string) error {
	return keychainStoreValue(keychainService, dekKeychainAccount, value)
}

func keychainDEKDelete() error {
	return deleteKeychainAccount(keychainService, dekKeychainAccount)
}

func (k *KeychainStore) Get(credName string, kind Kind) (string, error) {
	account := keychainAccount(credName, kind)
	val, err := keychainLookup(keychainService, account)
	if err != nil {
		return "", fmt.Errorf("secret %q not found in keychain for credential %q", kind, credName)
	}
	return val, nil
}

func (k *KeychainStore) Set(credName string, kind Kind, value string) error {
	return keychainStoreValue(keychainService, keychainAccount(credName, kind), value)
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
	dump, err := dumpKeychainOutput()
	if err != nil {
		return nil, err
	}
	return parseKeychainDumpForService(dump, keychainService), nil
}
