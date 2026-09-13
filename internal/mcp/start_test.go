package mcp

import (
	"log/slog"
	"path/filepath"
	"sync/atomic"
	"testing"

	"buddy/internal/buddy"
	"buddy/internal/buddy/secrets"
)

type countingDEK struct {
	secrets.MemoryMaster
	gets atomic.Int32
}

func (c *countingDEK) Get() (string, error) {
	c.gets.Add(1)
	return c.MemoryMaster.Get()
}

func TestNewServerDoesNotUnlockVault(t *testing.T) {
	dek := &countingDEK{}
	store := secrets.NewVaultStore(filepath.Join(t.TempDir(), "secrets.enc"), dek)
	if err := store.Set("sandbox", secrets.KindPassword, "pw"); err != nil {
		t.Fatal(err)
	}
	store.LockSession()
	gets := dek.gets.Load()
	secrets.SetStore(store)
	t.Cleanup(func() { secrets.SetStore(nil) })

	srv := newServer("", buddy.Opts{}, slog.Default(), "test")
	if srv == nil {
		t.Fatal("newServer returned nil")
	}
	if dek.gets.Load() != gets {
		t.Fatalf("MCP start fetched DEK; gets %d → %d", gets, dek.gets.Load())
	}
	if store.SessionUnlocked() {
		t.Fatal("MCP start should leave vault locked")
	}
}
