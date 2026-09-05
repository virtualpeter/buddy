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

// KeychainTokenWrap stores the token-cache AES wrap key (never userPresence).
type KeychainTokenWrap struct{}

func (KeychainTokenWrap) Description() string {
	return keychainTokenWrapDescription()
}

func (KeychainTokenWrap) Get() (string, error) {
	return keychainTokenWrapGet()
}

func (KeychainTokenWrap) Set(value string) error {
	if value == "" {
		return fmt.Errorf("token wrap key must not be empty")
	}
	return keychainTokenWrapSet(value)
}

func (KeychainTokenWrap) Delete() error {
	return keychainTokenWrapDelete()
}
