package buddy

import (
	"strings"
	"testing"

	"buddy/internal/buddy/secrets"
)

type mockSecretStore struct {
	values map[string]string
}

func (m *mockSecretStore) key(credName string, kind secrets.Kind) string {
	return credName + "/" + kind.String()
}

func (m *mockSecretStore) Get(credName string, kind secrets.Kind) (string, error) {
	if v, ok := m.values[m.key(credName, kind)]; ok && v != "" {
		return v, nil
	}
	return "", secrets.ErrNotFound
}

func (m *mockSecretStore) Set(credName string, kind secrets.Kind, value string) error {
	if m.values == nil {
		m.values = make(map[string]string)
	}
	m.values[m.key(credName, kind)] = value
	return nil
}

func (m *mockSecretStore) Delete(credName string, kind secrets.Kind) error {
	delete(m.values, m.key(credName, kind))
	return nil
}

func (m *mockSecretStore) Has(credName string, kind secrets.Kind) (bool, error) {
	v, ok := m.values[m.key(credName, kind)]
	return ok && v != "", nil
}

func (m *mockSecretStore) List() ([]secrets.Entry, error) {
	var out []secrets.Entry
	for k := range m.values {
		parts := strings.SplitN(k, "/", 2)
		if len(parts) != 2 {
			continue
		}
		kind, err := secrets.ParseKind(parts[1])
		if err != nil {
			continue
		}
		out = append(out, secrets.Entry{AuthName: parts[0], Kind: kind})
	}
	return out, nil
}

func (m *mockSecretStore) Description() string { return "mock" }

func TestResolveAuthWithSecretsInline(t *testing.T) {
	cred := CredentialConfig{
		Auth: AuthConfig{Type: "oauth", ClientID: "id", ClientSecret: "inline"},
	}
	resolved, err := cred.ResolveAuthWithSecrets("sandbox")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.ClientSecret != "inline" {
		t.Fatalf("ClientSecret = %q", resolved.ClientSecret)
	}
}

func TestResolveAuthWithSecretsFromStore(t *testing.T) {
	mock := &mockSecretStore{values: map[string]string{
		"sandbox/client-secret": "from-store",
	}}
	secrets.SetStore(mock)
	t.Cleanup(func() { secrets.SetStore(nil) })

	cred := CredentialConfig{
		Auth: AuthConfig{Type: "oauth", ClientID: "id"},
	}
	resolved, err := cred.ResolveAuthWithSecrets("sandbox")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.ClientSecret != "from-store" {
		t.Fatalf("ClientSecret = %q", resolved.ClientSecret)
	}
}

func TestResolveAuthWithSecretsMissing(t *testing.T) {
	secrets.SetStore(&mockSecretStore{values: map[string]string{}})
	t.Cleanup(func() { secrets.SetStore(nil) })

	cred := CredentialConfig{
		Auth: AuthConfig{Type: "basic", Username: "admin"},
	}
	_, err := cred.ResolveAuthWithSecrets("missing")
	if err == nil {
		t.Fatal("expected error for missing password")
	}
}
