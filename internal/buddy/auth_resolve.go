package buddy

import (
	"fmt"

	"buddy/internal/buddy/secrets"
)

// ResolvedAuth holds auth fields with secrets resolved from inline config or secret store.
type ResolvedAuth struct {
	Type         string
	RunCommand   string
	Username     string
	Password     string
	ClientID     string
	ClientSecret string
}

// ResolveAuthWithSecrets returns auth ready for tokenauth, loading missing secrets from the platform store.
func (c CredentialConfig) ResolveAuthWithSecrets(credName string) (ResolvedAuth, error) {
	auth := c.Auth
	resolved := ResolvedAuth{
		Type:         auth.Type,
		RunCommand:   auth.RunCommand,
		Username:     auth.Username,
		Password:     auth.Password,
		ClientID:     auth.ClientID,
		ClientSecret: auth.ClientSecret,
	}

	switch auth.Type {
	case "command":
		return resolved, nil
	case "basic":
		if resolved.Password != "" {
			return resolved, nil
		}
		store, err := secrets.DefaultStore()
		if err != nil {
			return ResolvedAuth{}, err
		}
		pw, err := store.Get(credName, secrets.KindPassword)
		if err != nil {
			return ResolvedAuth{}, fmt.Errorf("password missing for auth %q — run 'buddy cred set secret %s' (or 'buddy cred migrate %s' to move an inline secret): %w", credName, credName, credName, err)
		}
		resolved.Password = pw
		return resolved, nil
	case "oauth":
		if resolved.ClientSecret != "" {
			return resolved, nil
		}
		store, err := secrets.DefaultStore()
		if err != nil {
			return ResolvedAuth{}, err
		}
		secret, err := store.Get(credName, secrets.KindClientSecret)
		if err != nil {
			return ResolvedAuth{}, fmt.Errorf("client secret missing for auth %q — run 'buddy cred set secret %s' (or 'buddy cred migrate %s' to move an inline secret): %w", credName, credName, credName, err)
		}
		resolved.ClientSecret = secret
		return resolved, nil
	default:
		return resolved, nil
	}
}

// Secret kind aliases for CLI callers.
const (
	SecretKindPassword     = secrets.KindPassword
	SecretKindClientSecret = secrets.KindClientSecret
)

// SetCredentialSecret stores a secret for a credential in the platform store.
func SetCredentialSecret(credName string, kind secrets.Kind, value string) error {
	store, err := secrets.DefaultStore()
	if err != nil {
		return err
	}
	return store.Set(credName, kind, value)
}

// DeleteCredentialSecret removes a secret from the platform store.
func DeleteCredentialSecret(credName string, kind secrets.Kind) error {
	store, err := secrets.DefaultStore()
	if err != nil {
		return err
	}
	return store.Delete(credName, kind)
}

// HasCredentialSecret reports whether a secret exists in the platform store.
func HasCredentialSecret(credName string, kind secrets.Kind) (bool, error) {
	store, err := secrets.DefaultStore()
	if err != nil {
		return false, err
	}
	return store.Has(credName, kind)
}

// SecretStoreInfo returns the active secret backend name and description.
func SecretStoreInfo() (kind, detail string, err error) {
	return secrets.StoreInfo()
}
