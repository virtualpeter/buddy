package script

import (
	"log/slog"
	"testing"

	"buddy/internal/buddy"
)

func TestUUIDIs(t *testing.T) {
	s, err := NewSession("", buddy.Opts{SkipConfirm: true}, slog.Default(), false)
	if err != nil {
		t.Fatal(err)
	}
	res := s.Eval("uuid_is", `
uuid_is(uuid_new()) and not uuid_is("com.apple.foo") and not uuid_is("")
`)
	if res.Error != "" {
		t.Fatalf("eval: %#v", res)
	}
	if res.Result != "true" {
		t.Fatalf("want true, got %#v", res)
	}
}
