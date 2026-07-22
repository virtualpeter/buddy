package script

import (
	"log/slog"
	"strings"
	"sync"
	"testing"

	"go.starlark.net/starlark"
)

func TestV3BuiltinNamesPresent(t *testing.T) {
	dict := makeBuiltins("default", slog.Default(), false)
	for _, name := range []string{
		"policies", "policy", "policy_xml", "policy_set",
		"packages", "package", "scripts", "script",
		"computers", "computer", "mdm_commands",
		"profiles", "profile", "profile_xml",
		"smart_groups", "static_groups", "smart_group", "static_group",
		"roles", "role", "role_privileges", "clients", "client",
		"blueprints", "blueprint", "blueprint_create", "blueprint_set",
		"blueprint_delete", "blueprint_deploy", "blueprint_undeploy",
		"blueprint_status", "blueprint_components", "blueprint_component",
		"auth", "confirm", "error", "exit",
		// Deprecated aliases still registered during migration window.
		"policy_find_ids", "package_list", "computer_find_ids", "group_list_smart",
	} {
		if _, ok := dict[name]; !ok {
			t.Errorf("missing builtin %q", name)
		}
	}
}

func TestXorIDOrKey(t *testing.T) {
	if err := xorIDOrKey("policy", true, true, "trigger"); err == nil {
		t.Fatal("expected error for both")
	}
	if err := xorIDOrKey("policy", false, false, "trigger"); err == nil {
		t.Fatal("expected error for neither")
	}
	if err := xorIDOrKey("policy", true, false, "trigger"); err != nil {
		t.Fatal(err)
	}
}

func TestStringListFromStarlark(t *testing.T) {
	got, err := stringListFromStarlark(starlark.String("a"), "keywords")
	if err != nil || len(got) != 1 || got[0] != "a" {
		t.Fatalf("got %v err %v", got, err)
	}
	list := starlark.NewList([]starlark.Value{starlark.String("x"), starlark.String("y")})
	got, err = stringListFromStarlark(list, "keywords")
	if err != nil || len(got) != 2 {
		t.Fatalf("got %v err %v", got, err)
	}
}

func TestPolicyBuiltinRejectsAmbiguous(t *testing.T) {
	fn := policyBuiltin("default", slog.Default())
	thread := &starlark.Thread{Name: "t"}
	kwargs := []starlark.Tuple{
		{starlark.String("id"), starlark.MakeInt(1)},
		{starlark.String("trigger"), starlark.String("x")},
	}
	_, err := fn(thread, nil, nil, kwargs)
	if err == nil || !strings.Contains(err.Error(), "not both") {
		t.Fatalf("expected ambiguous error, got %v", err)
	}
}

func TestDeprecateWarnsOnce(t *testing.T) {
	deprecOnce = sync.Map{}
	calls := 0
	inner := func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
		calls++
		return starlark.None, nil
	}
	wrapped := deprecate("old_fn", "new_fn", inner)
	thread := &starlark.Thread{Name: "t"}
	_, _ = wrapped(thread, nil, nil, nil)
	_, _ = wrapped(thread, nil, nil, nil)
	if calls != 2 {
		t.Fatalf("inner calls = %d, want 2", calls)
	}
}
