//go:build !darwin

package secrets

// ActiveKeychainService is empty off macOS.
func ActiveKeychainService() string {
	return ""
}

// ListKeychainService is a no-op off macOS.
func ListKeychainService(string) ([]Entry, error) {
	return nil, nil
}

// EraseKeychainService is a no-op off macOS.
func EraseKeychainService(string) (int, error) {
	return 0, nil
}
