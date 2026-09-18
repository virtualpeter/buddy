//go:build !darwin || !secitem

package secrets

func confirmDeviceOwner(reason string) error {
	if confirmDeviceOwnerHook != nil {
		return confirmDeviceOwnerHook(reason)
	}
	return nil
}
