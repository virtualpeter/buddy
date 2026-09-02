package secrets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	store, err := NewFileStore()
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Set("sandbox", KindClientSecret, "top-secret"); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get("sandbox", KindClientSecret)
	if err != nil {
		t.Fatal(err)
	}
	if got != "top-secret" {
		t.Fatalf("Get = %q", got)
	}
	has, err := store.Has("sandbox", KindClientSecret)
	if err != nil || !has {
		t.Fatalf("Has = %v err=%v", has, err)
	}

	if err := store.Delete("sandbox", KindClientSecret); err != nil {
		t.Fatal(err)
	}
	has, err = store.Has("sandbox", KindClientSecret)
	if err != nil || has {
		t.Fatalf("after delete Has = %v", has)
	}

	path := filepath.Join(dir, "buddy", "secrets", "sandbox.json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected secrets file removed, stat err=%v", err)
	}
}

func TestFileStorePasswordAndClientSecret(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	store, err := NewFileStore()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Set("dev", KindPassword, "pw"); err != nil {
		t.Fatal(err)
	}
	if err := store.Set("dev", KindClientSecret, "sec"); err != nil {
		t.Fatal(err)
	}
	pw, err := store.Get("dev", KindPassword)
	if err != nil || pw != "pw" {
		t.Fatalf("password = %q err=%v", pw, err)
	}
	sec, err := store.Get("dev", KindClientSecret)
	if err != nil || sec != "sec" {
		t.Fatalf("client secret = %q err=%v", sec, err)
	}
}

func TestDefaultStoreFileOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("BUDDY_SECRET_STORE", "file")
	t.Setenv(EnvVaultPassphrase, "test-pass")
	SetStore(nil)
	SetMasterStore(nil)
	SetDEKStore(nil)

	store, err := DefaultStore()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := store.(*VaultStore); !ok {
		t.Fatalf("expected VaultStore, got %T", store)
	}
	kind, _, err := StoreInfo()
	if err != nil {
		t.Fatal(err)
	}
	if kind != "vault" {
		t.Fatalf("kind = %q", kind)
	}
}
