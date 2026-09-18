package secrets

import "sync"

var (
	deviceOwnerMu sync.Mutex
	deviceOwnerOK bool
)

// confirmDeviceOwnerHook overrides confirmDeviceOwner in tests (all platforms).
var confirmDeviceOwnerHook func(reason string) error

func deviceOwnerAlreadyConfirmed() bool {
	deviceOwnerMu.Lock()
	defer deviceOwnerMu.Unlock()
	return deviceOwnerOK
}

func markDeviceOwnerConfirmed() {
	deviceOwnerMu.Lock()
	deviceOwnerOK = true
	deviceOwnerMu.Unlock()
}

func resetDeviceOwnerConfirm() {
	deviceOwnerMu.Lock()
	deviceOwnerOK = false
	deviceOwnerMu.Unlock()
}
