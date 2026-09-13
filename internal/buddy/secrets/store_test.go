package secrets

import "testing"

func TestDefaultStoreNonNil(t *testing.T) {
	store, err := DefaultStore()
	if err != nil {
		t.Fatal(err)
	}
	if store == nil {
		t.Fatal("expected non-nil store")
	}
	if store.Description() == "" {
		t.Fatal("expected non-empty Description")
	}
}

func TestDefaultStoreReusesProcessInstance(t *testing.T) {
	SetStore(nil)
	a, err := DefaultStore()
	if err != nil {
		t.Fatal(err)
	}
	b, err := DefaultStore()
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatal("DefaultStore should reuse one VaultStore per process (one Touch ID unlock)")
	}
}
