package secrets

import (
	"os"
	"strings"
)

// EnvVaultUnlock selects how the signed-macOS Keychain DEK is protected.
//
//	presence (default, empty) — SecAccessControl userPresence; Touch ID/passcode on read
//	after-first-unlock        — legacy AfterFirstUnlockThisDeviceOnly, no presence ACL (CI)
const EnvVaultUnlock = "BUDDY_VAULT_UNLOCK"

const (
	vaultUnlockPresence         = "presence"
	vaultUnlockAfterFirstUnlock = "after-first-unlock"
)

// VaultUnlockMode returns the normalized unlock mode from the environment.
func VaultUnlockMode() string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(EnvVaultUnlock))) {
	case vaultUnlockAfterFirstUnlock, "afterfirstunlock", "legacy":
		return vaultUnlockAfterFirstUnlock
	default:
		return vaultUnlockPresence
	}
}

// VaultUnlockUsesPresence reports whether the DEK should use userPresence ACL
// (signed secitem Darwin only; other backends ignore this).
func VaultUnlockUsesPresence() bool {
	return VaultUnlockMode() == vaultUnlockPresence
}

// DefaultUnlockReason is used when UnlockSession is called with an empty reason.
const DefaultUnlockReason = "Unlock Buddy vault"
