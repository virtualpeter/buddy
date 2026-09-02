package secrets

import (
	"fmt"
	"os/exec"
	"strings"
)

const passPrefix = "buddy"

// PassStore stores secrets using the pass(1) password manager.
type PassStore struct{}

// NewPassStore creates a pass-backed secret store.
func NewPassStore() (*PassStore, error) {
	if _, err := exec.LookPath("pass"); err != nil {
		return nil, fmt.Errorf("pass command not found in PATH")
	}
	return &PassStore{}, nil
}

func (p *PassStore) Description() string {
	return passPrefix + "/<credential>/<secret>"
}

func passEntry(credName string, kind Kind) string {
	return fmt.Sprintf("%s/%s/%s", passPrefix, credName, kind)
}

func (p *PassStore) Get(credName string, kind Kind) (string, error) {
	entry := passEntry(credName, kind)
	cmd := exec.Command("pass", "show", entry)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			msg := strings.TrimSpace(string(exitErr.Stderr))
			if msg == "" {
				msg = "pass show failed"
			}
			return "", fmt.Errorf("secret %q not found in pass store for credential %q: %s", kind, credName, msg)
		}
		return "", fmt.Errorf("pass show: %w", err)
	}
	val := strings.TrimSpace(string(output))
	if val == "" {
		return "", fmt.Errorf("secret %q not found in pass store for credential %q", kind, credName)
	}
	return val, nil
}

func (p *PassStore) Set(credName string, kind Kind, value string) error {
	entry := passEntry(credName, kind)
	cmd := exec.Command("pass", "insert", "-f", "-m", entry)
	cmd.Stdin = strings.NewReader(value)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pass insert %s: %w: %s", entry, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (p *PassStore) Delete(credName string, kind Kind) error {
	entry := passEntry(credName, kind)
	cmd := exec.Command("pass", "rm", "-f", entry)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pass rm %s: %w: %s", entry, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (p *PassStore) Has(credName string, kind Kind) (bool, error) {
	_, err := p.Get(credName, kind)
	if err != nil {
		return false, nil
	}
	return true, nil
}
