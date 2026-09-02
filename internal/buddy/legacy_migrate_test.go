package buddy

import (
	"testing"

	"buddy/internal/buddy/secrets"
)

func TestMigrateLegacySecretsFromFileStore(t *testing.T) {
	prev := secrets.EnvelopeIter
	secrets.EnvelopeIter = 10_000
	t.Cleanup(func() { secrets.EnvelopeIter = prev })

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("BUDDY_SECRET_STORE", "file")
	t.Setenv(secrets.EnvVaultPassphrase, "")
	secrets.SetStore(nil)
	master := &secrets.MemoryMaster{Phrase: "vault-pass"}
	secrets.SetMasterStore(master)
	t.Cleanup(func() {
		secrets.SetStore(nil)
		secrets.SetMasterStore(nil)
		secrets.SetDEKStore(nil)
	})

	legacy, err := secrets.NewFileStore()
	if err != nil {
		t.Fatal(err)
	}
	if err := legacy.Set("sandbox", secrets.KindClientSecret, "legacy-secret"); err != nil {
		t.Fatal(err)
	}
	if err := legacy.Set("onprem", secrets.KindPassword, "legacy-pw"); err != nil {
		t.Fatal(err)
	}

	res, err := MigrateLegacySecrets(true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Copied != 2 || res.Deleted != 2 {
		t.Fatalf("migrate %+v", res)
	}

	ok, err := HasCredentialSecret("sandbox", secrets.KindClientSecret)
	if err != nil || !ok {
		t.Fatalf("vault missing sandbox: %v %v", ok, err)
	}
	store, err := secrets.DefaultStore()
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.Get("sandbox", secrets.KindClientSecret)
	if err != nil || got != "legacy-secret" {
		t.Fatalf("Get = %q err=%v", got, err)
	}
	left, err := legacy.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 0 {
		t.Fatalf("legacy leftover %+v", left)
	}
}
