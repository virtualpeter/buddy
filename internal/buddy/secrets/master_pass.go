package secrets

import (
	"fmt"
	"os/exec"
	"strings"
)

const (
	passMasterEntry = passPrefix + "/master"
	passDEKEntry    = passPrefix + "/dek"
)

// PassMaster stores the recovery passphrase in pass(1) at buddy/master.
type PassMaster struct {
	dek bool
}

// NewPassMaster creates a pass-backed recovery-passphrase store.
func NewPassMaster() (*PassMaster, error) {
	if _, err := exec.LookPath("pass"); err != nil {
		return nil, fmt.Errorf("pass command not found in PATH")
	}
	return &PassMaster{}, nil
}

// NewPassDEK creates a pass-backed vault data-key store (buddy/dek).
func NewPassDEK() (*PassMaster, error) {
	if _, err := exec.LookPath("pass"); err != nil {
		return nil, fmt.Errorf("pass command not found in PATH")
	}
	return &PassMaster{dek: true}, nil
}

func (p *PassMaster) entry() string {
	if p != nil && p.dek {
		return passDEKEntry
	}
	return passMasterEntry
}

func (p *PassMaster) Description() string {
	return p.entry()
}

func (p *PassMaster) Get() (string, error) {
	entry := p.entry()
	cmd := exec.Command("pass", "show", entry)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			msg := strings.TrimSpace(string(exitErr.Stderr))
			if msg == "" {
				msg = "pass show failed"
			}
			return "", fmt.Errorf("vault key not found in pass store: %s", msg)
		}
		return "", fmt.Errorf("pass show: %w", err)
	}
	val := strings.TrimSpace(string(output))
	if val == "" {
		return "", fmt.Errorf("vault key not set — run 'buddy cred set passphrase'")
	}
	return val, nil
}

func (p *PassMaster) Set(passphrase string) error {
	if strings.TrimSpace(passphrase) == "" {
		return fmt.Errorf("value must not be empty")
	}
	entry := p.entry()
	cmd := exec.Command("pass", "insert", "-f", "-m", entry)
	cmd.Stdin = strings.NewReader(passphrase)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pass insert %s: %w: %s", entry, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (p *PassMaster) Delete() error {
	entry := p.entry()
	cmd := exec.Command("pass", "rm", "-f", entry)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pass rm %s: %w: %s", entry, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (p *PassMaster) Has() (bool, error) {
	_, err := p.Get()
	if err != nil {
		return false, nil
	}
	return true, nil
}
