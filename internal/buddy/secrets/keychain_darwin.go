//go:build darwin

package secrets

import (
	"fmt"
	"os/exec"
	"strings"
)

const keychainService = "buddy"

// KeychainStore stores secrets in the macOS Keychain via the security(1) CLI.
type KeychainStore struct{}

// NewKeychainStore creates a Keychain-backed secret store.
func NewKeychainStore() *KeychainStore {
	return &KeychainStore{}
}

func (k *KeychainStore) Description() string {
	return fmt.Sprintf("macOS Keychain (service %q)", keychainService)
}

func keychainAccount(credName string, kind Kind) string {
	return fmt.Sprintf("%s/%s", credName, kind)
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
	cmd := exec.Command("security", "add-generic-password", "-s", keychainService, "-a", account, "-w", value, "-U")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("keychain store: %w: %s", err, strings.TrimSpace(string(output)))
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
			return nil, fmt.Errorf("keychain list: %w: %s", err, strings.TrimSpace(string(output)))
		}
	}
	return parseKeychainDump(string(output)), nil
}

// parseKeychainDump extracts buddy generic-password accounts (auth/kind).
func parseKeychainDump(dump string) []Entry {
	var out []Entry
	seen := map[string]struct{}{}
	var acct string
	var isBuddy bool
	inGenp := false
	flush := func() {
		if inGenp && isBuddy && acct != "" {
			parts := strings.SplitN(acct, "/", 2)
			if len(parts) == 2 {
				if kind, err := ParseKind(parts[1]); err == nil {
					key := parts[0] + "/" + kind.String()
					if _, ok := seen[key]; !ok {
						seen[key] = struct{}{}
						out = append(out, Entry{AuthName: parts[0], Kind: kind})
					}
				}
			}
		}
		acct = ""
		isBuddy = false
		inGenp = false
	}
	for _, line := range strings.Split(dump, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "class:") {
			flush()
			inGenp = strings.Contains(trim, `"genp"`)
			continue
		}
		if !inGenp {
			continue
		}
		if strings.Contains(trim, `"svce"<blob>="buddy"`) || strings.Contains(trim, `0x00000007 <blob>="buddy"`) {
			isBuddy = true
		}
		const acctPrefix = `"acct"<blob>="`
		if i := strings.Index(trim, acctPrefix); i >= 0 {
			rest := trim[i+len(acctPrefix):]
			if j := strings.Index(rest, `"`); j >= 0 {
				acct = rest[:j]
			}
		}
	}
	flush()
	return out
}
