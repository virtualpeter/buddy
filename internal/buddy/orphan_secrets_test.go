package buddy

import (
	"testing"

	"buddy/internal/buddy/secrets"
)

func TestOrphanCredentialSecrets(t *testing.T) {
	mock := &mockSecretStore{values: map[string]string{
		"alive/password":     "pw",
		"gone/client-secret": "cs",
		"gone/password":      "pw2",
	}}
	secrets.SetStore(mock)
	t.Cleanup(func() { secrets.SetStore(nil) })

	cfg := &Config{
		Auths: map[string]AuthEntry{
			"alive": {Type: "basic", Username: "u", Tenants: []string{"sandbox"}},
		},
	}
	orphans, err := OrphanCredentialSecrets(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(orphans) != 2 {
		t.Fatalf("got %d orphans %+v, want 2", len(orphans), orphans)
	}
	for _, e := range orphans {
		if e.AuthName != "gone" {
			t.Fatalf("unexpected orphan %+v", e)
		}
	}
}

func TestEraseCredentialSecrets(t *testing.T) {
	mock := &mockSecretStore{values: map[string]string{
		"alive/password":     "pw",
		"gone/client-secret": "cs",
	}}
	secrets.SetStore(mock)
	t.Cleanup(func() { secrets.SetStore(nil) })

	storeN, kcN, err := EraseCredentialSecrets()
	if err != nil {
		t.Fatal(err)
	}
	if storeN != 2 {
		t.Fatalf("storeDeleted=%d, want 2", storeN)
	}
	if kcN != 0 {
		t.Fatalf("keychainDeleted=%d, want 0 with override store", kcN)
	}
	left, err := mock.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 0 {
		t.Fatalf("leftover %+v", left)
	}

	storeN, kcN, err = EraseCredentialSecrets()
	if err != nil {
		t.Fatal(err)
	}
	if storeN != 0 || kcN != 0 {
		t.Fatalf("empty erase store=%d keychain=%d", storeN, kcN)
	}
}
