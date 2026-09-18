package secrets

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVaultStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.enc")
	dek := &MemoryMaster{}
	store := NewVaultStore(path, dek)

	if err := store.Set("sandbox", KindClientSecret, "s3cret"); err != nil {
		t.Fatal(err)
	}
	if _, ok := ParseDEK(dek.Phrase); !ok {
		t.Fatalf("expected DEK after Set, got %q", dek.Phrase)
	}
	got, err := store.Get("sandbox", KindClientSecret)
	if err != nil {
		t.Fatal(err)
	}
	if got != "s3cret" {
		t.Fatalf("Get = %q", got)
	}
	if err := store.Set("sandbox", KindPassword, "pw"); err != nil {
		t.Fatal(err)
	}
	list, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("list %+v", list)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "s3cret") {
		t.Fatal("secret in plaintext vault file")
	}
	env, err := ParseEnvelope(raw)
	if err != nil {
		t.Fatal(err)
	}
	if env.V != EnvelopeVersionRaw || env.KDF != EnvelopeKDFRaw {
		t.Fatalf("envelope %+v", env)
	}

	store2 := NewVaultStore(path, dek)
	got, err = store2.Get("sandbox", KindClientSecret)
	if err != nil || got != "s3cret" {
		t.Fatalf("reopen Get = %q err=%v", got, err)
	}

	if err := store2.Delete("sandbox", KindClientSecret); err != nil {
		t.Fatal(err)
	}
	has, err := store2.Has("sandbox", KindClientSecret)
	if err != nil || has {
		t.Fatalf("after delete Has client-secret = %v", has)
	}
	pw, err := store2.Get("sandbox", KindPassword)
	if err != nil || pw != "pw" {
		t.Fatalf("password = %q err=%v", pw, err)
	}
}

func TestVaultStoreWrongPassphrase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.enc")
	dek := &MemoryMaster{}
	store := NewVaultStore(path, dek)
	if err := store.Set("a", KindPassword, "pw"); err != nil {
		t.Fatal(err)
	}
	other := NewVaultStore(path, &MemoryMaster{Phrase: "wrong"})
	_, err := other.Get("a", KindPassword)
	if err == nil || (!strings.Contains(err.Error(), "vault key") && !strings.Contains(err.Error(), "wrong key")) {
		t.Fatalf("err = %v", err)
	}
}

func TestVaultRejectsBackupEnvelope(t *testing.T) {
	prev := EnvelopeIter
	EnvelopeIter = 10_000
	t.Cleanup(func() { EnvelopeIter = prev })

	env, err := EncryptEnvelope([]byte(`{"hello":"world"}`), "pw", KindCredBackup, AADCredBackup)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "secrets.enc")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewVaultStore(path, &MemoryMaster{Phrase: "pw"})
	_, err = store.Get("a", KindPassword)
	if err == nil || !strings.Contains(err.Error(), "not a buddy vault") {
		t.Fatalf("err = %v", err)
	}
}

func TestVaultSetPassphraseRotate(t *testing.T) {
	prev := EnvelopeIter
	EnvelopeIter = 10_000
	t.Cleanup(func() { EnvelopeIter = prev })
	t.Setenv(EnvVaultPassphrase, "")

	path := filepath.Join(t.TempDir(), "secrets.enc")
	dek := &MemoryMaster{}
	recovery := &MemoryMaster{Phrase: "old"}
	store := NewVaultStore(path, dek).WithRecovery(recovery)
	if err := store.Set("a", KindPassword, "pw"); err != nil {
		t.Fatal(err)
	}
	dekCopy := dek.Phrase
	if err := store.SetPassphrase("new"); err != nil {
		t.Fatal(err)
	}
	if recovery.Phrase != "" {
		t.Fatalf("recovery must not stay machine-stored, got %q", recovery.Phrase)
	}
	if dek.Phrase != dekCopy {
		t.Fatal("SetPassphrase must not rotate the DEK")
	}
	store2 := NewVaultStore(path, dek)
	got, err := store2.Get("a", KindPassword)
	if err != nil || got != "pw" {
		t.Fatalf("Get = %q err=%v", got, err)
	}
	empty := NewVaultStore(path, &MemoryMaster{})
	if err := empty.SetPassphrase("new"); err != nil {
		t.Fatalf("typed recovery unwrap: %v", err)
	}
	got, err = empty.Get("a", KindPassword)
	if err != nil || got != "pw" {
		t.Fatalf("unwrap Get = %q err=%v", got, err)
	}
	_, err = NewVaultStore(path, &MemoryMaster{}).WithRecovery(&MemoryMaster{Phrase: "old"}).Get("a", KindPassword)
	if err == nil {
		t.Fatal("old recovery phrase / stored master must not unlock")
	}
}

func TestVaultPreservesWrapWithoutStoredMaster(t *testing.T) {
	prev := EnvelopeIter
	EnvelopeIter = 10_000
	t.Cleanup(func() { EnvelopeIter = prev })
	t.Setenv(EnvVaultPassphrase, "")

	path := filepath.Join(t.TempDir(), "secrets.enc")
	dek := &MemoryMaster{}
	store := NewVaultStore(path, dek)
	if err := store.Set("a", KindPassword, "pw"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetPassphrase("held-by-user"); err != nil {
		t.Fatal(err)
	}
	raw1, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	env1, err := ParseEnvelope(raw1)
	if err != nil || env1.Wrap == nil {
		t.Fatalf("expected wrap after SetPassphrase: %+v err=%v", env1, err)
	}
	wrapJSON, _ := json.Marshal(env1.Wrap)

	if err := store.Set("a", KindClientSecret, "cs"); err != nil {
		t.Fatal(err)
	}
	raw2, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	env2, err := ParseEnvelope(raw2)
	if err != nil || env2.Wrap == nil {
		t.Fatalf("wrap must survive save without stored master: %+v", env2)
	}
	wrapJSON2, _ := json.Marshal(env2.Wrap)
	if string(wrapJSON) != string(wrapJSON2) {
		t.Fatal("existing wrap bytes must be preserved unchanged")
	}
}

func TestVaultMigratesV1ToDEK(t *testing.T) {
	prev := EnvelopeIter
	EnvelopeIter = 10_000
	t.Cleanup(func() { EnvelopeIter = prev })

	path := filepath.Join(t.TempDir(), "secrets.enc")
	plain, err := json.Marshal(vaultPayload{Secrets: map[string]secretFile{"a": {Password: "pw"}}})
	if err != nil {
		t.Fatal(err)
	}
	env, err := EncryptEnvelope(plain, "legacy", KindSecretVault, AADSecretVault)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	dek := &MemoryMaster{}
	rec := &MemoryMaster{Phrase: "legacy"}
	store := NewVaultStore(path, dek).WithRecovery(rec)
	got, err := store.Get("a", KindPassword)
	if err != nil || got != "pw" {
		t.Fatalf("Get = %q err=%v", got, err)
	}
	if _, ok := ParseDEK(dek.Phrase); !ok {
		t.Fatalf("expected migrated DEK, got %q", dek.Phrase)
	}
	raw2, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	env2, err := ParseEnvelope(raw2)
	if err != nil {
		t.Fatal(err)
	}
	if env2.V != EnvelopeVersionRaw || env2.KDF != EnvelopeKDFRaw {
		t.Fatalf("migrated envelope %+v", env2)
	}
	if env2.Wrap == nil {
		t.Fatal("expected recovery wrap after v1 migrate")
	}
}

func TestVaultErase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.enc")
	store := NewVaultStore(path, &MemoryMaster{})
	if err := store.Set("a", KindPassword, "x"); err != nil {
		t.Fatal(err)
	}
	if err := store.Set("b", KindClientSecret, "y"); err != nil {
		t.Fatal(err)
	}
	n, err := store.Erase()
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("erased %d", n)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("vault file still present: %v", err)
	}
}

func TestVaultEmptyListWithoutPassphrase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.enc")
	store := NewVaultStore(path, &MemoryMaster{})
	list, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("list %+v", list)
	}
}

func TestFileDEKRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	m, err := NewFileDEK()
	if err != nil {
		t.Fatal(err)
	}
	enc, err := EncodeDEK(mustDEK(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Set(enc); err != nil {
		t.Fatal(err)
	}
	got, err := m.Get()
	if err != nil || got != enc {
		t.Fatalf("Get = %q err=%v", got, err)
	}
	path, err := DekFilePath()
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(dir, "buddy", "dek") {
		t.Fatalf("DekFilePath = %q", path)
	}
}

func mustDEK(t *testing.T) []byte {
	t.Helper()
	key, err := GenerateDEK()
	if err != nil {
		t.Fatal(err)
	}
	return key
}
