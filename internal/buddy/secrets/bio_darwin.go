//go:build darwin && secitem

package secrets

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework LocalAuthentication -framework Foundation
#include <stdlib.h>
#import <LocalAuthentication/LocalAuthentication.h>
#import <Foundation/Foundation.h>

// buddy_bio_confirm prompts with LAPolicyDeviceOwnerAuthentication (Touch ID /
// passcode). Used when SecAccessControl userPresence cannot be stored on a bare
// Developer ID CLI (errSecMissingEntitlement / no keychain-access-groups).
int buddy_bio_confirm(const char *reason) {
	@autoreleasepool {
		LAContext *ctx = [[LAContext alloc] init];
		NSError *authErr = nil;
		if (![ctx canEvaluatePolicy:LAPolicyDeviceOwnerAuthentication error:&authErr]) {
			return 0;
		}
		dispatch_semaphore_t sem = dispatch_semaphore_create(0);
		__block int result = 0;
		NSString *r = @"Unlock Buddy vault";
		if (reason != NULL && reason[0] != 0) {
			r = [NSString stringWithUTF8String:reason];
		}
		[ctx evaluatePolicy:LAPolicyDeviceOwnerAuthentication localizedReason:r reply:^(BOOL success, NSError *error) {
			result = success ? 1 : -1;
			dispatch_semaphore_signal(sem);
		}];
		dispatch_semaphore_wait(sem, DISPATCH_TIME_FOREVER);
		return result;
	}
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// confirmDeviceOwner shows Touch ID / device passcode via LocalAuthentication.
// Prefer SecAccessControl when the platform allows it; this is the fallback for
// Developer ID CLI binaries that cannot write userPresence Keychain items.
func confirmDeviceOwner(reason string) error {
	cReason := C.CString(reason)
	defer C.free(unsafe.Pointer(cReason))
	st := int(C.buddy_bio_confirm(cReason))
	if st < 0 {
		return fmt.Errorf("device owner authentication failed or cancelled")
	}
	return nil
}
