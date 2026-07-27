package buddy

import (
	"testing"

	"buddy/internal/buddy/secrets"
)

func TestOrphanCredentialSecrets(t *testing.T) {
	mock := &mockSecretStore{values: map[string]string{
		"alive/password":       "pw",
		"gone/client-secret":   "cs",
		"gone/password":        "pw2",
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
