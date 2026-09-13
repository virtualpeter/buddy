package buddy

import (
	"testing"
)

func TestDefaultOAuthAuthName(t *testing.T) {
	if got := defaultOAuthAuthName("sandbox"); got != "sandbox-oauth" {
		t.Fatalf("got %q", got)
	}
	if got := defaultOAuthAuthName(DefaultTenant); got != "buddy-oauth" {
		t.Fatalf("got %q", got)
	}
	if got := defaultOAuthAuthName(""); got != "buddy-oauth" {
		t.Fatalf("got %q", got)
	}
}
