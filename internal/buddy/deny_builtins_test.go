package buddy

import (
	"strings"
	"testing"
)

func TestBuiltinNameDenied(t *testing.T) {
	patterns := []string{"jamf_*", "computer_erase", "uuid_new"}
	if !BuiltinNameDenied(patterns, "jamf_put") {
		t.Fatal("jamf_* should match jamf_put")
	}
	if !BuiltinNameDenied(patterns, "jamf_get") {
		t.Fatal("jamf_* should match jamf_get")
	}
	if !BuiltinNameDenied(patterns, "computer_erase") {
		t.Fatal("exact computer_erase")
	}
	if BuiltinNameDenied(patterns, "jamf") {
		t.Fatal("jamf alone should not match jamf_*")
	}
	if BuiltinNameDenied(patterns, "computer_reboot") {
		t.Fatal("computer_reboot should not match")
	}
	if BuiltinNameDenied(nil, "jamf_put") {
		t.Fatal("empty patterns deny nothing")
	}
}

func TestNormalizeDenyBuiltins(t *testing.T) {
	got, err := NormalizeDenyBuiltins([]string{" jamf_* ", "computer_erase", "jamf_*", ""})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "jamf_*" || got[1] != "computer_erase" {
		t.Fatalf("got %#v", got)
	}
	if _, err := NormalizeDenyBuiltins([]string{"*"}); err == nil {
		t.Fatal("expected error for bare *")
	}
	if _, err := NormalizeDenyBuiltins([]string{"jamf*put"}); err == nil {
		t.Fatal("expected error for mid-string *")
	}
}

func TestErrBuiltinDenied(t *testing.T) {
	err := ErrBuiltinDenied(ChannelMCP, "jamf_put")
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{"jamf_put", "mcp", "deny_builtins"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("msg %q missing %q", msg, want)
		}
	}
}
