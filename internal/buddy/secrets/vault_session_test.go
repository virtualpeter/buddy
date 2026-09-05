package secrets

import (
	"path/filepath"
	"sync/atomic"
	"testing"
)

type countingMaster struct {
	MemoryMaster
	gets atomic.Int32
}

func (c *countingMaster) Get() (string, error) {
	c.gets.Add(1)
	return c.MemoryMaster.Get()
}

func TestVaultSessionUnlockCachesDEK(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.enc")
	dek := &countingMaster{}
	store := NewVaultStore(path, dek)
	if err := store.Set("sandbox", KindPassword, "pw"); err != nil {
		t.Fatal(err)
	}
	if !store.SessionUnlocked() {
		t.Fatal("expected session after Set")
	}
	getsAfterSet := dek.gets.Load()

	store.LockSession()
	if store.SessionUnlocked() {
		t.Fatal("expected locked session")
	}

	if err := store.UnlockSession("test unlock"); err != nil {
		t.Fatal(err)
	}
	if !store.SessionUnlocked() {
		t.Fatal("expected unlocked session")
	}
	getsAfterUnlock := dek.gets.Load()
	if getsAfterUnlock <= getsAfterSet {
		t.Fatalf("UnlockSession should fetch DEK; gets %d → %d", getsAfterSet, getsAfterUnlock)
	}

	for i := 0; i < 3; i++ {
		got, err := store.Get("sandbox", KindPassword)
		if err != nil || got != "pw" {
			t.Fatalf("Get = %q err=%v", got, err)
		}
	}
	if dek.gets.Load() != getsAfterUnlock {
		t.Fatalf("session should avoid further DEK Gets; got %d want %d", dek.gets.Load(), getsAfterUnlock)
	}

	store.LockSession()
	got, err := store.Get("sandbox", KindPassword)
	if err != nil || got != "pw" {
		t.Fatalf("lazy unlock Get = %q err=%v", got, err)
	}
	if dek.gets.Load() <= getsAfterUnlock {
		t.Fatal("lazy unlock after LockSession should fetch DEK again")
	}
}

func TestVaultUnlockSessionMissingDEKOK(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.enc")
	store := NewVaultStore(path, &MemoryMaster{})
	if err := store.UnlockSession(""); err != nil {
		t.Fatalf("optional unlock with no DEK: %v", err)
	}
	if store.SessionUnlocked() {
		t.Fatal("no DEK should leave session locked")
	}
}

func TestVaultUnlockMode(t *testing.T) {
	t.Setenv(EnvVaultUnlock, "")
	if !VaultUnlockUsesPresence() {
		t.Fatal("default should use presence")
	}
	t.Setenv(EnvVaultUnlock, "after-first-unlock")
	if VaultUnlockUsesPresence() {
		t.Fatal("after-first-unlock should disable presence")
	}
	if VaultUnlockMode() != vaultUnlockAfterFirstUnlock {
		t.Fatalf("mode = %q", VaultUnlockMode())
	}
	t.Setenv(EnvVaultUnlock, "legacy")
	if VaultUnlockUsesPresence() {
		t.Fatal("legacy alias should disable presence")
	}
}
