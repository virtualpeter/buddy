package script

import (
	"testing"

	"buddy/internal/buddy"

	"go.starlark.net/starlark"
)

func TestPageOptsFromArgsOmitPageMeansAll(t *testing.T) {
	opts, err := pageOptsFromArgs(starlark.None, starlark.Int{})
	if err != nil {
		t.Fatal(err)
	}
	if opts.Page != buddy.PageAll {
		t.Fatalf("page = %d, want PageAll", opts.Page)
	}
}

func TestPageOptsFromArgsSinglePage(t *testing.T) {
	opts, err := pageOptsFromArgs(starlark.MakeInt(2), starlark.MakeInt(50))
	if err != nil {
		t.Fatal(err)
	}
	if opts.Page != 2 || opts.PageSize != 50 {
		t.Fatalf("got %+v", opts)
	}
}

func TestPageOptsFromArgsRejectsNegative(t *testing.T) {
	_, err := pageOptsFromArgs(starlark.MakeInt(-1), starlark.Int{})
	if err == nil {
		t.Fatal("expected error for page < 0")
	}
}
