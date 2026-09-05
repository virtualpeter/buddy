//go:build darwin && secitem

package secrets

import (
	"fmt"
	"testing"
)

func TestKeychainSecItemDescription(t *testing.T) {
	d := NewKeychainStore().Description()
	want := fmt.Sprintf("macOS Keychain (service %q, Security.framework)", keychainServiceSigned)
	if d != want {
		t.Fatalf("description: %s", d)
	}
}

// Manual check (signed binary, interactive Mac): after `make install-signed`,
// `buddy mcp` or a vault-backed command (e.g. `buddy policy list`) should prompt
// once for Touch ID / passcode to release the DEK, then reuse the in-process
// session. Set BUDDY_VAULT_UNLOCK=after-first-unlock to skip ACL for CI.
// Presence plumbing is covered by C helpers; full Keychain UI cannot run in go test.
