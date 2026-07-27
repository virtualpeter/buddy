package secrets

import "testing"

func TestFileStoreList(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	store, err := NewFileStore()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Set("a", KindPassword, "pw"); err != nil {
		t.Fatal(err)
	}
	if err := store.Set("a", KindClientSecret, "cs"); err != nil {
		t.Fatal(err)
	}
	if err := store.Set("b", KindPassword, "pw2"); err != nil {
		t.Fatal(err)
	}
	list, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Fatalf("got %d entries: %+v", len(list), list)
	}
}

func TestParseKeychainDump(t *testing.T) {
	dump := `
class: "genp"
attributes:
    "svce"<blob>="buddy"
    "acct"<blob>="sandbox-admin/password"
class: "genp"
attributes:
    "svce"<blob>="other"
    "acct"<blob>="x/password"
class: "genp"
attributes:
    "acct"<blob>="oauth1/client-secret"
    "svce"<blob>="buddy"
`
	got := parseKeychainDump(dump)
	if len(got) != 2 {
		t.Fatalf("got %+v", got)
	}
}
