package serve

import (
	"context"
	"log/slog"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

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

func TestRunDoesNotUnlockVault(t *testing.T) {
	dek := &countingDEK{}
	store := secrets.NewVaultStore(filepath.Join(t.TempDir(), "secrets.enc"), dek)
	if err := store.Set("sandbox", secrets.KindPassword, "pw"); err != nil {
		t.Fatal(err)
	}
	store.LockSession()
	gets := dek.gets.Load()
	secrets.SetStore(store)
	t.Cleanup(func() { secrets.SetStore(nil) })

	t.Setenv("BUDDY_UPDATE_CHECK_PATH", filepath.Join(t.TempDir(), "update-check.json"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, Options{
			Addr:   "127.0.0.1:0",
			Logger: slog.New(slog.DiscardHandler),
		})
	}()
	select {
	case err := <-errCh:
		t.Fatalf("serve exited before listen: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	if dek.gets.Load() != gets {
		t.Fatalf("serve start fetched DEK; gets %d → %d", gets, dek.gets.Load())
	}
	if store.SessionUnlocked() {
		t.Fatal("serve start should leave vault locked")
	}
	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("serve shutdown: %v", err)
	}
}
