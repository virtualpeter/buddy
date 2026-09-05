package secrets

// presenceDEKStore is implemented by Keychain DEK backends that can prompt
// via SecAccessControl userPresence (signed secitem Darwin).
type presenceDEKStore interface {
	GetPresence(reason string) (string, error)
	SetPresence(value, reason string) error
}

// presenceMasterStore is the recovery-passphrase counterpart (optional ACL).
type presenceMasterStore interface {
	GetPresence(reason string) (string, error)
	SetPresence(value, reason string) error
}
