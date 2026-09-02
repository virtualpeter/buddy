//go:build !darwin || !secitem

package secrets

func confirmDeviceOwner(string) error { return nil }
