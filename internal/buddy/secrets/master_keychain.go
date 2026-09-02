package secrets

import "fmt"

// KeychainMaster stores the recovery passphrase as one Keychain generic-password.
type KeychainMaster struct{}

func (KeychainMaster) Description() string {
	return keychainMasterDescription()
}

func (KeychainMaster) Get() (string, error) {
	return keychainMasterGet()
}

func (KeychainMaster) Set(passphrase string) error {
	if passphrase == "" {
		return fmt.Errorf("passphrase must not be empty")
	}
	return keychainMasterSet(passphrase)
}

func (KeychainMaster) Delete() error {
	return keychainMasterDelete()
}

func (KeychainMaster) Has() (bool, error) {
	_, err := keychainMasterGet()
	if err != nil {
		return false, nil
	}
	return true, nil
}

// KeychainDEK stores the vault data key as one Keychain generic-password.
type KeychainDEK struct{}

func (KeychainDEK) Description() string {
	return keychainDEKDescription()
}

func (KeychainDEK) Get() (string, error) {
	return keychainDEKGet()
}

func (KeychainDEK) Set(value string) error {
	if value == "" {
		return fmt.Errorf("vault key must not be empty")
	}
	return keychainDEKSet(value)
}

func (KeychainDEK) Delete() error {
	return keychainDEKDelete()
}

func (KeychainDEK) Has() (bool, error) {
	_, err := keychainDEKGet()
	if err != nil {
		return false, nil
	}
	return true, nil
}
