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
