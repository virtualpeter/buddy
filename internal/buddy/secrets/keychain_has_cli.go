//go:build darwin && !secitem

package secrets

func (KeychainMaster) Has() (bool, error) {
	_, err := keychainMasterGet()
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (KeychainDEK) Has() (bool, error) {
	_, err := keychainDEKGet()
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (KeychainTokenWrap) Has() (bool, error) {
	_, err := keychainTokenWrapGet()
	if err != nil {
		return false, nil
	}
	return true, nil
}
