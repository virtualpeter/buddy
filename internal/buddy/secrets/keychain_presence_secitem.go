//go:build darwin && secitem

package secrets

import "fmt"

func (KeychainDEK) GetPresence(reason string) (string, error) {
	return keychainDEKGetPresence(reason)
}

func (KeychainDEK) SetPresence(value, reason string) error {
	if value == "" {
		return fmt.Errorf("vault key must not be empty")
	}
	return keychainDEKSetPresence(value, reason)
}

func (KeychainDEK) Has() (bool, error) {
	return secitemHas(dekKeychainAccount)
}

func (KeychainMaster) GetPresence(reason string) (string, error) {
	_ = reason
	return keychainMasterGet()
}

func (KeychainMaster) SetPresence(value, reason string) error {
	_ = reason
	if value == "" {
		return fmt.Errorf("passphrase must not be empty")
	}
	return keychainMasterSet(value)
}

func (KeychainMaster) Has() (bool, error) {
	return secitemHas(masterKeychainAccount)
}

func (KeychainTokenWrap) Has() (bool, error) {
	return secitemHas(tokenWrapKeychainAccount)
}
