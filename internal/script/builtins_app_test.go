package script

import (
	"strings"
	"testing"

	"go.starlark.net/starlark"
)

func TestAppBuiltinsRegistered(t *testing.T) {
	dict := makeBuiltins("", nil, false)
	for _, name := range []string{
		"apps", "app", "app_scan", "app_import", "app_cache", "app_cache_refresh",
		"app_upgrade", "app_reconcile",
	} {
		if _, ok := dict[name]; !ok {
			t.Errorf("missing builtin %s", name)
		}
	}
}

func TestAppImportRequiresNameAndSSID(t *testing.T) {
	dict := makeBuiltins("", nil, false)
	fn := dict["app_import"].(*starlark.Builtin)
	th := &starlark.Thread{Name: "t"}
	_, err := starlark.Call(th, fn, nil, nil)
	if err == nil {
		t.Fatal("expected error when name and ss_policy_id are omitted")
	}
	if !strings.Contains(err.Error(), "name") && !strings.Contains(err.Error(), "ss_policy_id") {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = starlark.Call(th, fn, starlark.Tuple{starlark.String("santa"), starlark.MakeInt(0)}, nil)
	if err == nil || !strings.Contains(err.Error(), "ss_policy_id") {
		t.Fatalf("want ss_policy_id error, got %v", err)
	}
}
