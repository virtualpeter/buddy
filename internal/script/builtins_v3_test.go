package script

import (
	"log/slog"
	"strings"
	"testing"

	"go.starlark.net/starlark"
)

// Removed in v4 — must not be registered in makeBuiltins.
var forbiddenLegacyBuiltinNames = []string{
	"policy_find_ids", "policy_get_id", "policy_get_enable", "policy_get_details", "policy_fetch_xml",
	"policy_set_enable", "policy_set_disable", "policy_set_trigger", "policy_set_name", "policy_set_frequency",
	"package_list", "package_find_id", "package_fetch",
	"script_list", "script_get_id", "script_fetch",
	"computer_find_ids", "computer_get", "computer_fetch",
	"mdm_command_find", "mdm_fetch_commands",
	"profile_list", "profile_find_id", "profile_fetch_xml",
	"group_list_smart", "group_list_static", "group_find_id", "group_fetch", "smart_group_fetch",
	"group_get_version", "group_set_version", "group_put",
	"role_list", "role_fetch_list", "role_fetch", "role_privileges_list",
	"client_fetch_list", "client_fetch",
}

func TestV3BuiltinNamesPresent(t *testing.T) {
	dict := makeBuiltins("default", slog.Default(), false)
	for _, name := range []string{
		"policies", "policy", "policy_xml", "policy_set",
		"packages", "package", "scripts", "script",
		"computers", "computer", "computer_detail", "mdm_commands",
		"profiles", "profile", "profile_xml",
		"smart_groups", "static_groups", "smart_group", "static_group",
		"roles", "role", "role_privileges", "clients", "client",
		"blueprints", "blueprint", "blueprint_create", "blueprint_set",
		"blueprint_delete", "blueprint_deploy", "blueprint_undeploy",
		"blueprint_status", "blueprint_components", "blueprint_component",
		"auth", "confirm", "error", "exit",
		"version_extract", "version_compare", "version_sort",
		"change_batch", "plan_change", "run_change",
	} {
		if _, ok := dict[name]; !ok {
			t.Errorf("missing builtin %q", name)
		}
	}
}

func TestForbiddenLegacyBuiltinNamesAbsent(t *testing.T) {
	dict := makeBuiltins("default", slog.Default(), false)
	if len(forbiddenLegacyBuiltinNames) != 38 {
		t.Fatalf("forbidden list length = %d, want 38", len(forbiddenLegacyBuiltinNames))
	}
	for _, name := range forbiddenLegacyBuiltinNames {
		if _, ok := dict[name]; ok {
			t.Errorf("legacy alias %q must not be registered", name)
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

func TestStarlarkDictSetIDIfMissing(t *testing.T) {
	d := new(starlark.Dict)
	_ = d.SetKey(starlark.String("name"), starlark.String("g"))
	got := starlarkDictSetIDIfMissing(d, "42")
	out, ok := got.(*starlark.Dict)
	if !ok {
		t.Fatalf("got %T", got)
	}
	id, found, _ := out.Get(starlark.String("id"))
	if !found || id != starlark.String("42") {
		t.Fatalf("id = %v found=%v", id, found)
	}
	withID := new(starlark.Dict)
	_ = withID.SetKey(starlark.String("id"), starlark.String("99"))
	if starlarkDictSetIDIfMissing(withID, "42") != withID {
		t.Fatal("expected existing id to be preserved")
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
