//go:build darwin

package secrets

import (
	"fmt"
	"os/exec"
	"strings"
)

func dumpKeychainOutput() (string, error) {
	cmd := exec.Command("security", "dump-keychain")
	output, err := cmd.CombinedOutput()
	if err != nil && len(output) == 0 {
		return "", fmt.Errorf("keychain list: %w", err)
	}
	return string(output), nil
}

// ActiveKeychainService is the Keychain service this binary uses.
func ActiveKeychainService() string {
	return keychainService
}

// ListKeychainService returns buddy secret identities stored under service (no values).
func ListKeychainService(service string) ([]Entry, error) {
	dump, err := dumpKeychainOutput()
	if err != nil {
		return nil, err
	}
	return parseKeychainDumpForService(dump, service), nil
}

func deleteKeychainAccount(service, account string) error {
	cmd := exec.Command("security", "delete-generic-password", "-s", service, "-a", account)
	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if strings.Contains(msg, "could not be found") {
			return nil
		}
		if msg != "" {
			return fmt.Errorf("keychain delete %s %s: %w: %s", service, account, err, msg)
		}
		return fmt.Errorf("keychain delete %s %s: %w", service, account, err)
	}
	return nil
}

// EraseKeychainService deletes every buddy generic-password item for service.
func EraseKeychainService(service string) (int, error) {
	entries, err := ListKeychainService(service)
	if err != nil {
		return 0, err
	}
	deleted := 0
	var errs []error
	for _, e := range entries {
		if err := deleteKeychainAccount(service, keychainAccount(e.AuthName, e.Kind)); err != nil {
			errs = append(errs, err)
			continue
		}
		deleted++
	}
	if len(errs) > 0 {
		return deleted, fmt.Errorf("deleted %d, %d failed: %v", deleted, len(errs), errs)
	}
	return deleted, nil
}
