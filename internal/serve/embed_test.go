package serve

import (
	"net/http/httptest"
	"testing"
)

func TestRequestEmbed(t *testing.T) {
	SetDefaultEmbed(false)
	t.Cleanup(func() { SetDefaultEmbed(false) })

	req := httptest.NewRequest("GET", "/docs/?embed=1", nil)
	if !RequestEmbed(req) {
		t.Fatal("?embed=1 should enable embed")
	}
	req = httptest.NewRequest("GET", "/docs/?embed=0", nil)
	if RequestEmbed(req) {
		t.Fatal("?embed=0 should disable embed")
	}

	SetDefaultEmbed(true)
	req = httptest.NewRequest("GET", "/docs/", nil)
	if !RequestEmbed(req) {
		t.Fatal("--embed default should enable embed")
	}
	req = httptest.NewRequest("GET", "/docs/?embed=0", nil)
	if RequestEmbed(req) {
		t.Fatal("?embed=0 should override --embed default")
	}
}
