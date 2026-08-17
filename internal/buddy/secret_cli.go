package buddy

import (
	"fmt"
	"io"
	"os"

	"buddy/internal/buddy/secrets"
)

// SetCredentialSecrets stores password and/or client secret for a credential.
func SetCredentialSecrets(credName, password, clientSecret string) error {
	if password == "" && clientSecret == "" {
		return fmt.Errorf("provide --password and/or --client-secret")
	}
	if password != "" {
		if err := SetCredentialSecret(credName, secrets.KindPassword, password); err != nil {
			return err
		}
	}
	if clientSecret != "" {
		if err := SetCredentialSecret(credName, secrets.KindClientSecret, clientSecret); err != nil {
			return err
		}
	}
	return nil
}

// PromptSecretLine reads a secret from stdin with a label (echoed).
func PromptSecretLine(label string) (string, error) {
	return secrets.PromptLine(label)
}

// PromptSecret reads a secret from stdin without echoing it (when stdin is a TTY).
func PromptSecret(label string) (string, error) {
	return secrets.PromptSecret(label)
}

// StdinIsTerminal reports whether stdin is an interactive terminal.
func StdinIsTerminal() bool {
	return secrets.StdinIsTerminal()
}

// ReadSecretFromStdin reads a secret value from stdin.
func ReadSecretFromStdin() (string, error) {
	return secrets.ReadAllFromReader(os.Stdin)
}

// SecretStoreAvailable reports whether a platform secret store can be used.
func SecretStoreAvailable() bool {
	_, err := secrets.DefaultStore()
	return err == nil
}

// CredentialSecretStatus describes stored secrets for a credential.
type CredentialSecretStatus struct {
	PasswordInConfig     bool
	ClientSecretInConfig bool
	PasswordInStore      bool
	ClientSecretInStore  bool
}

// CredentialSecretStatusFor returns secret storage status for an auth name.
func CredentialSecretStatusFor(cfg *Config, authName string) (CredentialSecretStatus, error) {
	auth, err := cfg.ResolveAuth(authName)
	if err != nil {
		return CredentialSecretStatus{}, err
	}
	st := CredentialSecretStatus{
		PasswordInConfig:     auth.Password != "",
		ClientSecretInConfig: auth.ClientSecret != "",
	}
	if pw, err := HasCredentialSecret(authName, secrets.KindPassword); err == nil {
		st.PasswordInStore = pw
	}
	if cs, err := HasCredentialSecret(authName, secrets.KindClientSecret); err == nil {
		st.ClientSecretInStore = cs
	}
	return st, nil
}

// PrintCredentialSecretStatus prints secret status for an auth.
func PrintCredentialSecretStatus(cfg *Config, authName string, w io.Writer) error {
	st, err := CredentialSecretStatusFor(cfg, authName)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "auth: %s\n", authName)
	fmt.Fprintf(w, "  password in config:       %t\n", st.PasswordInConfig)
	fmt.Fprintf(w, "  password in store:        %t\n", st.PasswordInStore)
	fmt.Fprintf(w, "  clientSecret in config:   %t\n", st.ClientSecretInConfig)
	fmt.Fprintf(w, "  clientSecret in store:    %t\n", st.ClientSecretInStore)
	return nil
}

// DeleteCredentialSecrets removes secrets from the platform store.
func DeleteCredentialSecrets(credName string, password, clientSecret bool) error {
	if !password && !clientSecret {
		return fmt.Errorf("specify --password and/or --client-secret to delete")
	}
	if password {
		if err := DeleteCredentialSecret(credName, secrets.KindPassword); err != nil {
			return err
		}
	}
	if clientSecret {
		if err := DeleteCredentialSecret(credName, secrets.KindClientSecret); err != nil {
			return err
		}
	}
	return nil
}

// ListCredentialSecrets lists secret identities in the platform store (no values).
func ListCredentialSecrets() ([]secrets.Entry, error) {
	store, err := secrets.DefaultStore()
	if err != nil {
		return nil, err
	}
	return store.List()
}

// OrphanCredentialSecrets returns store entries whose auth name is not in cfg.Auths.
func OrphanCredentialSecrets(cfg *Config) ([]secrets.Entry, error) {
	entries, err := ListCredentialSecrets()
	if err != nil {
		return nil, err
	}
	var orphans []secrets.Entry
	for _, e := range entries {
		if cfg == nil || cfg.Auths == nil {
			orphans = append(orphans, e)
			continue
		}
		if _, ok := cfg.Auths[e.AuthName]; !ok {
			orphans = append(orphans, e)
		}
	}
	return orphans, nil
}

// KeychainSecretGroup is buddy items under one Keychain service (identities only).
type KeychainSecretGroup struct {
	Service string
	Entries []secrets.Entry
}

func secretStoreOverridden() bool {
	kind, _, _ := secrets.StoreInfo()
	return kind == "override"
}

// ListBuddyKeychainSecrets lists items under both buddy Keychain services on macOS.
// Empty when the store is overridden (tests) or not on Darwin.
func ListBuddyKeychainSecrets() ([]KeychainSecretGroup, error) {
	if secretStoreOverridden() {
		return nil, nil
	}
	var out []KeychainSecretGroup
	for _, svc := range secrets.BuddyKeychainServices() {
		entries, err := secrets.ListKeychainService(svc)
		if err != nil {
			return nil, err
		}
		if len(entries) == 0 {
			continue
		}
		out = append(out, KeychainSecretGroup{Service: svc, Entries: entries})
	}
	return out, nil
}

// EraseCredentialSecrets deletes every secret in the active store.
// On macOS it also deletes leftover items from both buddy Keychain services
// (CLI "buddy" and signed "virtualpete.buddy.cli").
func EraseCredentialSecrets() (storeDeleted, keychainDeleted int, err error) {
	store, err := secrets.DefaultStore()
	if err != nil {
		return 0, 0, err
	}
	entries, err := store.List()
	if err != nil {
		return 0, 0, err
	}
	for _, e := range entries {
		if err := store.Delete(e.AuthName, e.Kind); err != nil {
			return storeDeleted, 0, fmt.Errorf("delete %s for %q: %w", e.Kind, e.AuthName, err)
		}
		storeDeleted++
	}
	if secretStoreOverridden() {
		return storeDeleted, 0, nil
	}
	for _, svc := range secrets.BuddyKeychainServices() {
		n, err := secrets.EraseKeychainService(svc)
		keychainDeleted += n
		if err != nil {
			return storeDeleted, keychainDeleted, err
		}
	}
	return storeDeleted, keychainDeleted, nil
}

// ActiveKeychainService is the Keychain service this binary uses (empty off macOS).
func ActiveKeychainService() string {
	return secrets.ActiveKeychainService()
}
