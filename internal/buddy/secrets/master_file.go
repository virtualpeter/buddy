package secrets

import (
	"fmt"
	"os"
	"strings"
)

// FileMaster stores the recovery passphrase in ~/.config/buddy/master (mode 0600).
type FileMaster struct {
	path string
	dek  bool
}

// NewFileMaster creates a file-backed recovery-passphrase store.
func NewFileMaster() (*FileMaster, error) {
	return &FileMaster{}, nil
}

// NewFileDEK creates a file-backed vault data-key store (~/.config/buddy/dek).
func NewFileDEK() (*FileMaster, error) {
	return &FileMaster{dek: true}, nil
}

func (f *FileMaster) masterPath() (string, error) {
	if f != nil && f.path != "" {
		return f.path, nil
	}
	if f != nil && f.dek {
		return DekFilePath()
	}
	return MasterFilePath()
}

func (f *FileMaster) noun() string {
	if f != nil && f.dek {
		return "vault key"
	}
	return "recovery passphrase"
}

func (f *FileMaster) Description() string {
	path, err := f.masterPath()
	if err != nil {
		if f != nil && f.dek {
			return "file vault key (path unknown)"
		}
		return "file master (path unknown)"
	}
	return path
}

func (f *FileMaster) Get() (string, error) {
	path, err := f.masterPath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%s not set — run 'buddy cred set passphrase'", f.noun())
		}
		return "", fmt.Errorf("read %s: %w", f.noun(), err)
	}
	pass := strings.TrimSpace(string(data))
	if pass == "" {
		return "", fmt.Errorf("%s not set — run 'buddy cred set passphrase'", f.noun())
	}
	return pass, nil
}

func (f *FileMaster) Set(passphrase string) error {
	if strings.TrimSpace(passphrase) == "" {
		return fmt.Errorf("%s must not be empty", f.noun())
	}
	path, err := f.masterPath()
	if err != nil {
		return err
	}
	return writeFileAtomic0600(path, []byte(passphrase+"\n"))
}

func (f *FileMaster) Delete() error {
	path, err := f.masterPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", f.noun(), err)
	}
	return nil
}

func (f *FileMaster) Has() (bool, error) {
	_, err := f.Get()
	if err != nil {
		return false, nil
	}
	return true, nil
}
